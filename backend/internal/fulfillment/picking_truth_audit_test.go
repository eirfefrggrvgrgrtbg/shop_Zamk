package fulfillment_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
)

func TestPickingAndInventoryTruthAudit_CasesA_to_E(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	invRepo := inventory.NewRepository(f.db)
	invSvc := inventory.NewService(invRepo, nil, nil)

	// Base Order & Fulfillment: Live paid/assembling
	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	var variantID, prodID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_variant_id, product_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID, &prodID)
	require.NoError(t, err)

	invItemID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 1, now(), now())
	`, invItemID, prodID, variantID, f.sellerID)
	require.NoError(t, err)

	// Unit 1 (Case A): warehouse ZMU + active allocation to live paid fulfillment => occupied, not free
	allocUnitID, allocUnitCode := f.createUnitWithStatus(t, ctx, itemID, "warehouse")
	_ = allocUnitID

	// Unit 2 (Case B): warehouse ZMU + no active allocation => free
	freeUnitID, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)
	_ = freeUnitID

	// Unit 3 (Case C): warehouse ZMU + stale active allocation tied to terminal (delivered) order
	terminalOrderID, terminalFulfillID := f.createOrderAndFulfillment(t, ctx, "delivered", "delivered")
	terminalItemID := f.createOrderItem(t, ctx, terminalOrderID, terminalFulfillID, 1, 1)

	staleUnitID, staleUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	staleAllocID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, released_at, created_at)
		VALUES ($1, $2, $3, now(), NULL, now() - interval '2 days')
	`, staleAllocID, terminalItemID, staleUnitID)
	require.NoError(t, err)

	// Verify Case E: Inventory Explorer free/occupied classification agrees with picking free-ZMU eligibility
	invDetail, err := invSvc.GetAdminInventoryItem(ctx, invItemID)
	require.NoError(t, err)
	require.NotNil(t, invDetail)

	// Total warehouse units: 3 (allocUnit, freeUnit, staleUnit)
	assert.Equal(t, 3, invDetail.Physical.Warehouse, "Total warehouse ZMUs must be 3")
	// Only the live allocation should be counted as live allocated (1)
	assert.Equal(t, 1, invDetail.Physical.Allocated, "Only live active allocation on live order must be allocated")
	// Physical free must be 2 (freeUnit + staleUnit which is eligible for picking)
	assert.Equal(t, 2, invDetail.Physical.Free, "Physical free must be 2 (unallocated + stale terminal allocation)")
	assert.Equal(t, 1, invDetail.Physical.StaleAllocated, "Stale allocation must be detected and reported")

	// Verify Case C: stale active allocation must NOT be reported as healthy normal occupied stock
	assert.Equal(t, "mixed", invDetail.AccountingMode, "Variant with stale allocation must remain mixed accounting mode")
	assert.Equal(t, "warning", invDetail.Health.Status, "Health status must be warning")
	assert.Contains(t, invDetail.Health.Issues, "stale_active_allocation", "Health issues must include stale_active_allocation")

	// Check Picking Compatible Units
	compatUnits, err := f.svc.GetCompatibleUnits(ctx, fulfillmentID, itemID)
	require.NoError(t, err)
	require.Len(t, compatUnits, 3)

	availMap := make(map[string]string)
	for _, u := range compatUnits {
		availMap[u.UnitCode] = u.Availability
	}

	// Case A: allocUnitCode is allocated to current item (occupied)
	assert.Equal(t, "allocated_to_current_item", availMap[allocUnitCode])
	// Case B: freeUnitCode is free
	assert.Equal(t, "free", availMap[freeUnitCode])
	// Case C: staleUnitCode is free (stale terminal allocation is ignored by picking)
	assert.Equal(t, "free", availMap[staleUnitCode])

	// Case E verification: Picking free candidates count == Inventory Explorer physical.free == 2
	freeCandidatesCount := 0
	for _, u := range compatUnits {
		if u.Availability == "free" {
			freeCandidatesCount++
		}
	}
	assert.Equal(t, invDetail.Physical.Free, freeCandidatesCount, "Inventory Explorer physicalFree must agree with picking free count")

	// Foreign allocation safety: create another live paid order and try to scan its unit
	foreignOrderID, foreignFulfillID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	foreignItemID := f.createOrderItem(t, ctx, foreignOrderID, foreignFulfillID, 1, 0)
	_, foreignUnitCode := f.createUnitWithStatus(t, ctx, foreignItemID, "warehouse")

	_, errForeign := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, foreignUnitCode, &itemID)
	assert.ErrorIs(t, errForeign, fulfillment.ErrUnitAllocatedToOtherOrder, "Scanning unit allocated to other live order must be rejected")

	// Pick staleUnitCode for current item -> should supersede stale allocation and succeed!
	res, errScan := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, staleUnitCode, &itemID)
	require.NoError(t, errScan, "Picking stale unit for current item must succeed")
	assert.True(t, res.ScanResult.Substituted)
	assert.True(t, res.ScanResult.NewlyPicked)

	// Verify old stale allocation is released with stale_allocation_superseded
	var staleReleasedAt *time.Time
	var staleReason *string
	err = f.db.QueryRow(ctx, `SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1`, staleAllocID).Scan(&staleReleasedAt, &staleReason)
	require.NoError(t, err)
	assert.NotNil(t, staleReleasedAt, "stale allocation must be marked released")
	assert.Equal(t, "stale_allocation_superseded", *staleReason)
}
