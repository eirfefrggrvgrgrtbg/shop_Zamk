package fulfillment_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
)

func TestPickingScan_Serialized(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	// Create 2 allocations (serialized mode)
	f.createAllocation(t, ctx, itemID, false)
	f.createAllocation(t, ctx, itemID, false)

	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)
	allocs := po.Items[0].AllocatedUnits
	require.Len(t, allocs, 2)

	// A. Scan first ZMU
	adminID := f.adminID
	res, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, allocs[0].UnitCode)
	require.NoError(t, err)
	assert.True(t, res.ScanResult.NewlyPicked)
	assert.Equal(t, 1, res.Item.PickedQuantity)
	assert.False(t, res.FulfillmentProgress.IsComplete)

	// Check transition to assembling
	f_db, _ := f.svc.GetAdminFulfillment(ctx, fulfillmentID)
	assert.Equal(t, "assembling", f_db.Status)

	// B. Duplicate scan
	res2, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, allocs[0].UnitCode)
	require.NoError(t, err)
	assert.False(t, res2.ScanResult.NewlyPicked)
	assert.True(t, res2.ScanResult.AlreadyPicked)
	assert.Equal(t, 1, res2.Item.PickedQuantity)

	// C. Scan second ZMU
	res3, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, allocs[1].UnitCode)
	require.NoError(t, err)
	assert.True(t, res3.ScanResult.NewlyPicked)
	assert.Equal(t, 2, res3.Item.PickedQuantity)
	assert.True(t, res3.FulfillmentProgress.IsComplete)
}

func TestPickingScan_ForeignZMU(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID1, fulfillmentID1 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID1 := f.createOrderItem(t, ctx, orderID1, fulfillmentID1, 1, 0)
	f.createAllocation(t, ctx, itemID1, false)

	orderID2, fulfillmentID2 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	_ = f.createOrderItem(t, ctx, orderID2, fulfillmentID2, 1, 0) // some other order

	po1, _ := f.svc.GetPickingOrder(ctx, fulfillmentID1)
	unitCode := po1.Items[0].AllocatedUnits[0].UnitCode

	adminID := f.adminID
	_, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID2, unitCode)
	require.ErrorIs(t, err, fulfillment.ErrUnitAllocatedToOtherOrder)
}

func TestPickingScan_Legacy(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	// Legacy item (no allocations)
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	// Get barcode for item
	var barcode string
	err := f.db.QueryRow(ctx, `
		SELECT pv.barcode FROM product_variants pv 
		JOIN order_items oi ON oi.product_variant_id = pv.id
		WHERE oi.id = $1
	`, itemID).Scan(&barcode)
	require.NoError(t, err)

	adminID := f.adminID
	// Scan 1
	res1, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, barcode)
	require.NoError(t, err)
	assert.True(t, res1.ScanResult.NewlyPicked)
	assert.Equal(t, 1, res1.Item.PickedQuantity)

	// Scan 2
	res2, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, barcode)
	require.NoError(t, err)
	assert.True(t, res2.ScanResult.NewlyPicked)
	assert.Equal(t, 2, res2.Item.PickedQuantity)

	// Scan 3 (overpick)
	res3, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, barcode)
	require.NoError(t, err)
	assert.False(t, res3.ScanResult.NewlyPicked)
	assert.True(t, res3.ScanResult.AlreadyComplete)
	assert.Equal(t, 2, res3.Item.PickedQuantity)
}

func TestPickingScan_SerializedConcurrency(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, false)

	po, _ := f.svc.GetPickingOrder(ctx, fulfillmentID)
	unitCode := po.Items[0].AllocatedUnits[0].UnitCode
	adminID := f.adminID

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, _ = f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, unitCode)
		}()
	}
	wg.Wait()

	poFinal, _ := f.svc.GetPickingOrder(ctx, fulfillmentID)
	assert.Equal(t, 1, poFinal.Items[0].PickedQuantity)
}

func TestPickingScan_LegacyConcurrency(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 5, 0)
	var barcode string
	_ = f.db.QueryRow(ctx, `SELECT pv.barcode FROM product_variants pv JOIN order_items oi ON oi.product_variant_id = pv.id WHERE oi.id = $1`, itemID).Scan(&barcode)

	adminID := f.adminID

	// Try scanning 10 times concurrently for quantity 5
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, barcode)
		}()
	}
	wg.Wait()

	poFinal, _ := f.svc.GetPickingOrder(ctx, fulfillmentID)
	assert.Equal(t, 5, poFinal.Items[0].PickedQuantity, "should never exceed quantity 5")
}
