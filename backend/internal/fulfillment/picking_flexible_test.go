package fulfillment_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

// Scenario J & General Substitution: free compatible serialized ZMU performs atomic substitution when orderItemId is provided.
func TestPicking_FlexibleZMU_SubstitutionAndInvariants(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	var variantID, prodID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_variant_id, product_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID, &prodID)
	require.NoError(t, err)

	invItemID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 2, now(), now())
	`, invItemID, prodID, variantID, f.sellerID)
	require.NoError(t, err)

	alloc1UnitID, alloc1UnitCode := f.createUnitWithStatus(t, ctx, itemID, "warehouse")
	alloc2UnitID, alloc2UnitCode := f.createUnitWithStatus(t, ctx, itemID, "warehouse")
	freeUnitID, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	// Verify initial compatible units count in GetPickingOrder
	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)
	assert.Equal(t, 3, po.Items[0].CompatibleUnitsCount, "2 allocated unpicked + 1 free = 3 compatible units")

	// Verify compatible units read endpoint
	compatUnits, err := f.svc.GetCompatibleUnits(ctx, fulfillmentID, itemID)
	require.NoError(t, err)
	require.Len(t, compatUnits, 3)

	availMap := make(map[string]string)
	for _, u := range compatUnits {
		availMap[u.UnitCode] = u.Availability
	}
	assert.Equal(t, "allocated_to_current_item", availMap[alloc1UnitCode])
	assert.Equal(t, "allocated_to_current_item", availMap[alloc2UnitCode])
	assert.Equal(t, "free", availMap[freeUnitCode])

	// 1. Scan the FREE ZMU targeted to itemID -> ATOMIC SUBSTITUTION
	adminID := f.adminID
	res, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, freeUnitCode, &itemID)
	require.NoError(t, err)
	assert.Equal(t, "serialized", res.ScanResult.Type)
	assert.True(t, res.ScanResult.NewlyPicked)
	assert.True(t, res.ScanResult.Substituted)
	assert.False(t, res.ScanResult.AlreadyPicked)
	assert.Equal(t, 1, res.Item.PickedQuantity)
	assert.Equal(t, 1, res.Item.RemainingQuantity)

	// 2. Invariants Check:
	// A. Exactly one old allocation was released with reason 'picking_substitution'
	var releasedCount int
	var releaseReason string
	err = f.db.QueryRow(ctx, `
		SELECT count(*), coalesce(max(release_reason), '')
		FROM order_item_allocations
		WHERE order_item_id = $1 AND released_at IS NOT NULL
	`, itemID).Scan(&releasedCount, &releaseReason)
	require.NoError(t, err)
	assert.Equal(t, 1, releasedCount)
	assert.Equal(t, "picking_substitution", releaseReason)

	// B. Active allocation count remains exactly equal to item quantity (2)
	var activeCount int
	err = f.db.QueryRow(ctx, `
		SELECT count(*)
		FROM order_item_allocations
		WHERE order_item_id = $1 AND released_at IS NULL
	`, itemID).Scan(&activeCount)
	require.NoError(t, err)
	assert.Equal(t, 2, activeCount, "Active allocations count must strictly equal quantity")

	// C. Scanned free unit is now an active allocation with picked_at set
	var freeUnitPickedAt *time.Time
	var freeUnitReleasedAt *time.Time
	err = f.db.QueryRow(ctx, `
		SELECT picked_at, released_at
		FROM order_item_allocations
		WHERE inventory_unit_id = $1
	`, freeUnitID).Scan(&freeUnitPickedAt, &freeUnitReleasedAt)
	require.NoError(t, err)
	assert.NotNil(t, freeUnitPickedAt)
	assert.Nil(t, freeUnitReleasedAt)

	// D. Stock counters in inventory_items untouched (10 total, 2 reserved)
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1`, invItemID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 2, reservedStock)

	// E. Physical unit status in inventory_units remains 'warehouse'
	var status1, status2, statusFree string
	_ = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, alloc1UnitID).Scan(&status1)
	_ = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, alloc2UnitID).Scan(&status2)
	_ = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, freeUnitID).Scan(&statusFree)
	assert.Equal(t, "warehouse", status1)
	assert.Equal(t, "warehouse", status2)
	assert.Equal(t, "warehouse", statusFree)

	// F. Refresh read model proves persistence
	poRefreshed, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, 1, poRefreshed.Items[0].PickedQuantity)
	assert.Equal(t, 1, poRefreshed.Items[0].RemainingQuantity)
	assert.Equal(t, 2, poRefreshed.Items[0].CompatibleUnitsCount, "1 unpicked allocated + 1 remaining free = 2")

	// G. Re-scanning the newly substituted unit reports already picked idempotently
	resDup, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, freeUnitCode, &itemID)
	require.NoError(t, err)
	assert.True(t, resDup.ScanResult.AlreadyPicked)
	assert.False(t, resDup.ScanResult.NewlyPicked)
	assert.False(t, resDup.ScanResult.Substituted)

	// H. Pick remaining preallocated unit -> completion
	var remainingAllocCode string
	for _, u := range poRefreshed.Items[0].AllocatedUnits {
		if u.PickedAt == nil {
			remainingAllocCode = u.UnitCode
			break
		}
	}
	require.NotEmpty(t, remainingAllocCode)
	resFinal, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, remainingAllocCode, &itemID)
	require.NoError(t, err)
	assert.True(t, resFinal.ScanResult.NewlyPicked)
	assert.False(t, resFinal.ScanResult.Substituted)
	assert.True(t, resFinal.FulfillmentProgress.IsComplete)
}

// Section 1: Free ZMU scan WITHOUT orderItemId must be rejected with ErrOrderItemRequiredForSubstitution and zero mutation.
func TestPicking_FlexibleZMU_FreeZMU_WithoutOrderItemID_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemA := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	itemB := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	var variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemA).Scan(&variantID)
	require.NoError(t, err)

	// Both items share the same variant
	_, err = f.db.Exec(ctx, `UPDATE order_items SET product_variant_id = $1 WHERE id = $2`, variantID, itemB)
	require.NoError(t, err)

	_, allocCodeA := f.createUnitWithStatus(t, ctx, itemA, "warehouse")
	_, allocCodeB := f.createUnitWithStatus(t, ctx, itemB, "warehouse")

	// Create 1 free warehouse ZMU of the same variant
	freeUnitID, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	// 1. Scan free ZMU WITHOUT orderItemId -> MUST BE REJECTED
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, freeUnitCode, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrOrderItemRequiredForSubstitution)

	// Invariant Checks:
	// A. Zero allocations released for these items
	var releasedCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE order_item_id IN ($1, $2) AND released_at IS NOT NULL`, itemA, itemB).Scan(&releasedCount)
	require.NoError(t, err)
	assert.Equal(t, 0, releasedCount, "Zero allocations must be released")

	// B. Zero replacement allocations created
	var freeUnitAllocations int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1`, freeUnitID).Scan(&freeUnitAllocations)
	require.NoError(t, err)
	assert.Equal(t, 0, freeUnitAllocations, "Free unit must not be allocated")

	// C. Zero picking progress change
	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	for _, it := range po.Items {
		assert.Equal(t, 0, it.PickedQuantity, "Picked quantity must remain 0")
		assert.Equal(t, 1, it.RemainingQuantity, "Remaining quantity must remain 1")
	}

	// 2. Regression: scanning preallocated ZMU without orderItemId still works safely
	resA, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, allocCodeA, nil)
	require.NoError(t, err)
	assert.True(t, resA.ScanResult.NewlyPicked)
	assert.False(t, resA.ScanResult.Substituted)
	assert.Equal(t, itemA, resA.ScanResult.OrderItemID)

	_ = allocCodeB
}

// Section 1: Legacy code without orderItemId regression.
func TestPicking_Legacy_WithoutOrderItemID_Accepted(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	legacyItem := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	var sku string
	err := f.db.QueryRow(ctx, `SELECT pv.sku FROM order_items oi JOIN product_variants pv ON pv.id = oi.product_variant_id WHERE oi.id = $1`, legacyItem).Scan(&sku)
	require.NoError(t, err)

	// Scan legacy SKU without orderItemId -> works
	res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, sku, nil)
	require.NoError(t, err)
	assert.Equal(t, "legacy", res.ScanResult.Type)
	assert.True(t, res.ScanResult.NewlyPicked)
	assert.Equal(t, legacyItem, res.ScanResult.OrderItemID)
}

// Scenario A: same fulfillment, same variant, TWO different order items.
// Request scoped to item B -> only item B allocation substituted! Item A remains untouched.
func TestPicking_FlexibleZMU_SameVariantTwoItems_ScopedToItemB(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemA := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	itemB := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	var variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemA).Scan(&variantID)
	require.NoError(t, err)

	// Make item B use the exact same variant as item A
	_, err = f.db.Exec(ctx, `UPDATE order_items SET product_variant_id = $1 WHERE id = $2`, variantID, itemB)
	require.NoError(t, err)

	// Allocate 1 unit to Item A and 1 unit to Item B
	_, unitCodeA := f.createUnitWithStatus(t, ctx, itemA, "warehouse")
	_, unitCodeB := f.createUnitWithStatus(t, ctx, itemB, "warehouse")

	// Create a free warehouse unit of the same variant
	freeUnitID, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	// Scan free ZMU specifically for Item B
	res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, freeUnitCode, &itemB)
	require.NoError(t, err)
	assert.Equal(t, itemB, res.ScanResult.OrderItemID)
	assert.True(t, res.ScanResult.Substituted)
	assert.True(t, res.ScanResult.NewlyPicked)

	// Verify Item B allocation was substituted
	var itemBAllocCount, itemBReleasedCount int
	err = f.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE released_at IS NULL),
		       count(*) FILTER (WHERE released_at IS NOT NULL AND release_reason = 'picking_substitution')
		FROM order_item_allocations
		WHERE order_item_id = $1
	`, itemB).Scan(&itemBAllocCount, &itemBReleasedCount)
	require.NoError(t, err)
	assert.Equal(t, 1, itemBAllocCount, "Item B still has exactly 1 active allocation")
	assert.Equal(t, 1, itemBReleasedCount, "Item B had exactly 1 released allocation")

	// Verify Item A was NOT touched!
	var itemAAllocCount, itemAReleasedCount int
	var itemAPickedAt *time.Time
	err = f.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE released_at IS NULL),
		       count(*) FILTER (WHERE released_at IS NOT NULL),
		       max(picked_at)
		FROM order_item_allocations
		WHERE order_item_id = $1
	`, itemA).Scan(&itemAAllocCount, &itemAReleasedCount, &itemAPickedAt)
	require.NoError(t, err)
	assert.Equal(t, 1, itemAAllocCount, "Item A allocation count unchanged")
	assert.Equal(t, 0, itemAReleasedCount, "Item A was never released")
	assert.Nil(t, itemAPickedAt, "Item A was not picked")

	// Scanned unit is allocated to Item B, not Item A
	var actualAllocItemID uuid.UUID
	err = f.db.QueryRow(ctx, `SELECT order_item_id FROM order_item_allocations WHERE inventory_unit_id = $1 AND released_at IS NULL`, freeUnitID).Scan(&actualAllocItemID)
	require.NoError(t, err)
	assert.Equal(t, itemB, actualAllocItemID)

	_ = unitCodeA
	_ = unitCodeB
}

// Scenario B: supplied orderItemId from another fulfillment -> rejected with zero mutation.
func TestPicking_FlexibleZMU_OrderItemFromOtherFulfillment_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID1, fulfillmentID1 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	_, fulfillmentID2 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemFromFulf1 := f.createOrderItem(t, ctx, orderID1, fulfillmentID1, 1, 0)

	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemFromFulf1).Scan(&variantID)
	_, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	// Try scanning in fulfillment 2 with orderItemId from fulfillment 1
	_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID2, freeUnitCode, &itemFromFulf1)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrItemNotInFulfillment)

	// Verify zero mutation occurred
	var allocCount int
	_ = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE order_item_id = $1`, itemFromFulf1).Scan(&allocCount)
	assert.Equal(t, 0, allocCount)
}

// Scenario C: supplied orderItemId wrong variant -> rejected with zero mutation.
func TestPicking_FlexibleZMU_WrongVariantForOrderItem_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createUnitWithStatus(t, ctx, itemID, "warehouse")

	// Create another variant and a free unit for that second variant
	wrongVariantID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at)
		VALUES ($1, (SELECT product_id FROM order_items WHERE id = $2), $3, 1000, true, now(), now())
	`, wrongVariantID, itemID, "SKU-WRONG-"+uuid.New().String()[:6])
	require.NoError(t, err)

	_, wrongVariantUnitCode := f.createUnallocatedUnit(t, ctx, wrongVariantID)

	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, wrongVariantUnitCode, &itemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrUnitVariantMismatch)

	// Zero mutation: no allocation released
	var releasedCount int
	_ = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE order_item_id = $1 AND released_at IS NOT NULL`, itemID).Scan(&releasedCount)
	assert.Equal(t, 0, releasedCount)
}

// Scenario D: compatible-units on legacy item -> rejected, never suggests serialized candidates.
func TestPicking_CompatibleUnits_LegacyItem_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	legacyItemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, legacyItemID).Scan(&variantID)
	f.createUnallocatedUnit(t, ctx, variantID)

	// Calling GetCompatibleUnits for a legacy item must return ErrItemNotSerialized
	units, err := f.svc.GetCompatibleUnits(ctx, fulfillmentID, legacyItemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrItemNotSerialized)
	assert.Nil(t, units)
}

// Scenario E / Sections 2 & 3: REAL serialized completion via allocation.picked_at != nil while oi.picked_quantity = 0.
func TestPicking_CompatibleUnits_CompletedSerializedItem_ViaPickedAt_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	// quantity = 1, picked_quantity = 0 (canonical serialized state!)
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	f.createUnitWithStatus(t, ctx, itemID, "warehouse")
	// Mark the active allocation as picked (picked_at = now())
	_, err := f.db.Exec(ctx, `UPDATE order_item_allocations SET picked_at = now() WHERE order_item_id = $1`, itemID)
	require.NoError(t, err)

	// oi.picked_quantity is explicitly 0
	var oiPickedQ int
	err = f.db.QueryRow(ctx, `SELECT picked_quantity FROM order_items WHERE id = $1`, itemID).Scan(&oiPickedQ)
	require.NoError(t, err)
	assert.Equal(t, 0, oiPickedQ, "oi.picked_quantity must be 0")

	// GetCompatibleUnits must reject with ErrItemAlreadyComplete based on allocation.picked_at
	units, err := f.svc.GetCompatibleUnits(ctx, fulfillmentID, itemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrItemAlreadyComplete)
	assert.Nil(t, units)

	// Scanning another free ZMU for this completed item must also be rejected
	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)
	_, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, freeUnitCode, &itemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrNoUnpickedAllocationForVariant)
}

// Sections 2 & 3: Partially picked serialized item (Q=2, A=2, 1 picked, 1 unpicked, oi.picked_quantity=0).
func TestPicking_CompatibleUnits_PartiallyPickedSerializedItem(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	// quantity = 2, oi.picked_quantity = 0
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	var variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)
	require.NoError(t, err)

	// Create 2 preallocated units
	pickedUnitID, _ := f.createUnitWithStatus(t, ctx, itemID, "warehouse")
	_, unpickedUnitCode := f.createUnitWithStatus(t, ctx, itemID, "warehouse")

	// Mark exactly one allocation as picked
	_, err = f.db.Exec(ctx, `UPDATE order_item_allocations SET picked_at = now() WHERE inventory_unit_id = $1`, pickedUnitID)
	require.NoError(t, err)

	// Create 1 free warehouse unit
	_, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	// GetCompatibleUnits should succeed and return:
	// - the unpicked allocated unit
	// - the free unit
	// and NEVER include the already picked unit!
	units, err := f.svc.GetCompatibleUnits(ctx, fulfillmentID, itemID)
	require.NoError(t, err)
	require.Len(t, units, 2)

	unitCodes := make(map[string]string)
	for _, u := range units {
		unitCodes[u.UnitCode] = u.Availability
	}
	assert.Equal(t, "allocated_to_current_item", unitCodes[unpickedUnitCode])
	assert.Equal(t, "free", unitCodes[freeUnitCode])

	// GetPickingOrder verification
	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)
	assert.Equal(t, 1, po.Items[0].PickedQuantity)
	assert.Equal(t, 1, po.Items[0].RemainingQuantity)
	assert.Equal(t, 2, po.Items[0].CompatibleUnitsCount, "1 unpicked allocated + 1 free = 2")
}

// Scenario F: compatible-units on terminal fulfillment/order -> not actionable.
func TestPicking_CompatibleUnits_TerminalFulfillment_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "delivered", "delivered")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createUnitWithStatus(t, ctx, itemID, "warehouse")

	units, err := f.svc.GetCompatibleUnits(ctx, fulfillmentID, itemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrPickingNotAllowed)
	assert.Nil(t, units)
}

// Scenario G: same-order-item concurrent scan of one free ZMU:
// Exactly one physical substitution; one release only; one replacement only; progress increments once; second is idempotent AlreadyPicked.
func TestPicking_FlexibleZMU_SameOrderItem_Concurrency(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createUnitWithStatus(t, ctx, itemID, "warehouse")

	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)
	_, sharedFreeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	var wg sync.WaitGroup
	wg.Add(2)

	var res1, res2 *fulfillment.PickingScanResult
	var err1, err2 error

	go func() {
		defer wg.Done()
		res1, err1 = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, sharedFreeUnitCode, &itemID)
	}()

	go func() {
		defer wg.Done()
		res2, err2 = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, sharedFreeUnitCode, &itemID)
	}()

	wg.Wait()

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Contract: exactly one request performs substitution and newly picks it;
	// the second request returns idempotent AlreadyPicked with NewlyPicked=false and Substituted=false.
	winnerCount := 0
	idempotentCount := 0

	if res1.ScanResult.NewlyPicked && res1.ScanResult.Substituted && !res1.ScanResult.AlreadyPicked {
		winnerCount++
	} else if !res1.ScanResult.NewlyPicked && !res1.ScanResult.Substituted && res1.ScanResult.AlreadyPicked {
		idempotentCount++
	}

	if res2.ScanResult.NewlyPicked && res2.ScanResult.Substituted && !res2.ScanResult.AlreadyPicked {
		winnerCount++
	} else if !res2.ScanResult.NewlyPicked && !res2.ScanResult.Substituted && res2.ScanResult.AlreadyPicked {
		idempotentCount++
	}

	assert.Equal(t, 1, winnerCount, "Exactly one worker must perform substitution and newly pick")
	assert.Equal(t, 1, idempotentCount, "The second worker must receive idempotent AlreadyPicked with no new mutation")

	// Verify database invariants:
	// Exactly one substitution release
	var releaseCount int
	err := f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE order_item_id = $1 AND release_reason = 'picking_substitution'`, itemID).Scan(&releaseCount)
	require.NoError(t, err)
	assert.Equal(t, 1, releaseCount, "Exactly one release must have occurred")

	// Exactly one active allocation
	var activeCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE order_item_id = $1 AND released_at IS NULL`, itemID).Scan(&activeCount)
	require.NoError(t, err)
	assert.Equal(t, 1, activeCount, "Active allocation count must strictly equal 1 (A == Q)")

	// Progress incremented exactly once
	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)
	assert.Equal(t, 1, po.Items[0].PickedQuantity, "Picked quantity must be incremented exactly once")
	assert.Equal(t, 0, po.Items[0].RemainingQuantity, "Remaining quantity must be 0")
	assert.True(t, po.Items[0].AllocatedUnits[0].PickedAt != nil, "Allocation must be picked")
}

// Scenario H: legacy same-variant free ZMU -> rejected; no conversion to serialized.
func TestPicking_FlexibleZMU_LegacyItem_RejectsZMUSubstitution(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	legacyItemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, legacyItemID).Scan(&variantID)
	_, freeUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	// Scanning free ZMU for legacy item is rejected
	_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, freeUnitCode, &legacyItemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrItemNotSerialized)

	// Invariant: zero allocations exist; item did NOT convert to serialized!
	var allocCount int
	_ = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE order_item_id = $1`, legacyItemID).Scan(&allocCount)
	assert.Equal(t, 0, allocCount, "Legacy item must remain unallocated")

	// Read model still classifies it as legacy
	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)
	assert.Equal(t, "legacy", po.Items[0].AllocationMode)
}

// Scenario I: existing allocated serialized ZMU -> still works, and cross-item scan in same fulfillment is rejected.
func TestPicking_FlexibleZMU_AllocatedUnitBehavior(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemA := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	itemB := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	_, unitCodeA := f.createUnitWithStatus(t, ctx, itemA, "warehouse")
	_, unitCodeB := f.createUnitWithStatus(t, ctx, itemB, "warehouse")

	// 1. Scanning unit A for item B (same fulfillment, wrong order item) is rejected!
	_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCodeA, &itemB)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrUnitAllocatedToOtherOrderItem)

	// 2. Scanning unit A for item A works!
	res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCodeA, &itemA)
	require.NoError(t, err)
	assert.True(t, res.ScanResult.NewlyPicked)
	assert.False(t, res.ScanResult.Substituted)
	assert.Equal(t, itemA, res.ScanResult.OrderItemID)

	// 3. Scanning unit B for item B works!
	resB, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCodeB, &itemB)
	require.NoError(t, err)
	assert.True(t, resB.ScanResult.NewlyPicked)
	assert.False(t, resB.ScanResult.Substituted)
	assert.Equal(t, itemB, resB.ScanResult.OrderItemID)
}

// Rejections: non-warehouse and foreign units
func TestPicking_FlexibleZMU_Rejections(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createUnitWithStatus(t, ctx, itemID, "warehouse")

	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)

	adminID := f.adminID

	// 1. Foreign unit (allocated to another order)
	_, fulfillmentID2 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID2 := f.createOrderItem(t, ctx, orderID, fulfillmentID2, 1, 0)
	_, foreignUnitCode := f.createUnitWithStatus(t, ctx, itemID2, "warehouse")

	_, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, foreignUnitCode, &itemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrUnitAllocatedToOtherOrder)

	// 2. Damaged free unit
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, _ = f.db.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`, supplyID, f.sellerID, uuid.New().String()[:8])
	_, _ = f.db.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())`, supplyItemID, supplyID, variantID)
	damagedUnitID := uuid.New()
	damagedUnitCode, errGen := supplies.GenerateUnitCode()
	require.NoError(t, errGen)
	_, _ = f.db.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'damaged')`, damagedUnitID, damagedUnitCode, variantID, supplyID, supplyItemID)

	_, err = f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, damagedUnitCode, &itemID)
	require.Error(t, err)
	assert.ErrorIs(t, err, fulfillment.ErrUnitNotInWarehouse)
}
