package fulfillment_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
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
	assert.False(t, res.ScanResult.AlreadyPicked)
	assert.Equal(t, 1, res.Item.PickedQuantity)
	assert.Equal(t, 1, res.Item.RemainingQuantity)
	assert.False(t, res.FulfillmentProgress.IsComplete)

	// Check transition to assembling
	f_db, err := f.svc.GetAdminFulfillment(ctx, fulfillmentID)
	require.NoError(t, err)
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
	assert.Equal(t, 0, res3.Item.RemainingQuantity)
	assert.True(t, res3.FulfillmentProgress.IsComplete)

	// Fulfillment remains assembling, NOT packed or shipped
	f_dbFinal, err := f.svc.GetAdminFulfillment(ctx, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, "assembling", f_dbFinal.Status)
}

func TestPickingScan_CoherentEligibility(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	invalidPairs := []struct {
		orderStatus string
		fulfStatus  string
	}{
		{"cancelled", "paid"},
		{"awaiting_payment", "paid"},
		{"packed", "paid"},
		{"shipped", "paid"},
		{"delivered", "paid"},
		{"paid", "awaiting_payment"},
		{"paid", "packed"},
		{"paid", "shipped"},
		{"paid", "delivered"},
		{"paid", "cancelled"},
		{"paid", "returned"},
		{"paid", "refunded"},
	}

	for _, pair := range invalidPairs {
		t.Run(pair.orderStatus+"_"+pair.fulfStatus, func(t *testing.T) {
			orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, pair.orderStatus, pair.fulfStatus)
			itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
			f.createAllocation(t, ctx, itemID, false)

			po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
			assert.Nil(t, po)
			assert.ErrorIs(t, err, fulfillment.ErrPickingNotAllowed)

			var unitCode string
			err = f.db.QueryRow(ctx, `
				SELECT u.unit_code
				FROM inventory_units u
				JOIN order_item_allocations a ON a.inventory_unit_id = u.id
				WHERE a.order_item_id = $1
			`, itemID).Scan(&unitCode)
			require.NoError(t, err)

			res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode)
			assert.Nil(t, res)
			assert.ErrorIs(t, err, fulfillment.ErrPickingNotAllowed)
		})
	}

	allowedPairs := []struct {
		orderStatus string
		fulfStatus  string
	}{
		{"paid", "paid"},
		{"assembling", "paid"},
		{"paid", "assembling"},
		{"assembling", "assembling"},
	}

	for _, pair := range allowedPairs {
		t.Run("allowed_"+pair.orderStatus+"_"+pair.fulfStatus, func(t *testing.T) {
			orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, pair.orderStatus, pair.fulfStatus)
			itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
			f.createAllocation(t, ctx, itemID, false)

			po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
			require.NoError(t, err)
			assert.NotNil(t, po)

			var unitCode string
			err = f.db.QueryRow(ctx, `
				SELECT u.unit_code
				FROM inventory_units u
				JOIN order_item_allocations a ON a.inventory_unit_id = u.id
				WHERE a.order_item_id = $1
			`, itemID).Scan(&unitCode)
			require.NoError(t, err)

			res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode)
			require.NoError(t, err)
			assert.NotNil(t, res)
		})
	}
}

func TestPickingScan_InvalidScanKeepsPaidStatus(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	f.createAllocation(t, ctx, itemID, false)
	f.createAllocation(t, ctx, itemID, false)

	// Another order with allocation (foreign ZMU)
	orderID2, fulfillmentID2 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID2 := f.createOrderItem(t, ctx, orderID2, fulfillmentID2, 1, 0)
	f.createAllocation(t, ctx, itemID2, false)
	var foreignUnitCode string
	_ = f.db.QueryRow(ctx, `SELECT u.unit_code FROM inventory_units u JOIN order_item_allocations a ON a.inventory_unit_id = u.id WHERE a.order_item_id = $1`, itemID2).Scan(&foreignUnitCode)

	// Unallocated ZMU
	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)
	_, unallocatedUnitCode := f.createUnallocatedUnit(t, ctx, variantID)

	// Non-warehouse ZMU
	orderID3, fulfillmentID3 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID3 := f.createOrderItem(t, ctx, orderID3, fulfillmentID3, 1, 0)
	_, damagedUnitCode := f.createUnitWithStatus(t, ctx, itemID3, "damaged")

	// Barcode for serialized item
	var variantBarcode string
	_ = f.db.QueryRow(ctx, `SELECT pv.barcode FROM product_variants pv WHERE pv.id = $1`, variantID).Scan(&variantBarcode)

	adminID := f.adminID

	assertPaidState := func() {
		var oStatus, fStatus string
		_ = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
		_ = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
		assert.Equal(t, "paid", oStatus, "order status must remain paid")
		assert.Equal(t, "paid", fStatus, "fulfillment status must remain paid")

		var pickedCount int
		_ = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE order_item_id = $1 AND picked_at IS NOT NULL`, itemID).Scan(&pickedCount)
		assert.Equal(t, 0, pickedCount, "picked allocations must be 0")
	}

	// 1. Unknown code
	_, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, "UNKNOWN_CODE_9999")
	assert.ErrorIs(t, err, fulfillment.ErrCodeNotFound)
	assertPaidState()

	// 2. Foreign ZMU
	_, err = f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, foreignUnitCode)
	assert.ErrorIs(t, err, fulfillment.ErrUnitAllocatedToOtherOrder)
	assertPaidState()

	// 3. Unallocated ZMU
	_, err = f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, unallocatedUnitCode)
	assert.ErrorIs(t, err, fulfillment.ErrUnitNotAllocatedToFulfillment)
	assertPaidState()

	// 4. Non-warehouse ZMU (scanned on fulfillment3)
	_, err = f.svc.ScanPickingCode(ctx, adminID, fulfillmentID3, damagedUnitCode)
	assert.ErrorIs(t, err, fulfillment.ErrUnitNotInWarehouse)
	var f3Status string
	_ = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID3).Scan(&f3Status)
	assert.Equal(t, "paid", f3Status)

	// 5. Barcode for serialized item
	_, err = f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, variantBarcode)
	assert.ErrorIs(t, err, fulfillment.ErrCannotPickSerializedWithBarcode)
	assertPaidState()
}

func TestPickingScan_UnallocatedZMU(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	var variantID uuid.UUID
	_ = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)

	unitID, unitCode := f.createUnallocatedUnit(t, ctx, variantID)

	_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode)
	require.ErrorIs(t, err, fulfillment.ErrUnitNotAllocatedToFulfillment)

	// Verify unit is still unallocated and status is warehouse
	var allocCount int
	_ = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1`, unitID).Scan(&allocCount)
	assert.Equal(t, 0, allocCount)

	var uStatus string
	_ = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, unitID).Scan(&uStatus)
	assert.Equal(t, "warehouse", uStatus)
}

func TestPickingScan_NonWarehouseZMU(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	unitID, unitCode := f.createUnitWithStatus(t, ctx, itemID, "damaged")

	_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode)
	require.ErrorIs(t, err, fulfillment.ErrUnitNotInWarehouse)

	// Verify picked_at stays NULL
	var pickedAt interface{}
	_ = f.db.QueryRow(ctx, `SELECT picked_at FROM order_item_allocations WHERE inventory_unit_id = $1`, unitID).Scan(&pickedAt)
	assert.Nil(t, pickedAt)

	// Verify fulfillment remains paid
	var fStatus string
	_ = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	assert.Equal(t, "paid", fStatus)
}

func TestPickingScan_PartialAllocationRollback(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	// Item quantity = 2, but only 1 allocation
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	f.createAllocation(t, ctx, itemID, false)

	var unitID uuid.UUID
	var unitCode string
	err := f.db.QueryRow(ctx, `
		SELECT u.id, u.unit_code
		FROM inventory_units u
		JOIN order_item_allocations a ON a.inventory_unit_id = u.id
		WHERE a.order_item_id = $1
	`, itemID).Scan(&unitID, &unitCode)
	require.NoError(t, err)

	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode)
	require.ErrorIs(t, err, fulfillment.ErrInvariantViolation)

	// Verify whole transaction rolled back: picked_at remains NULL
	var pickedAt interface{}
	_ = f.db.QueryRow(ctx, `SELECT picked_at FROM order_item_allocations WHERE inventory_unit_id = $1`, unitID).Scan(&pickedAt)
	assert.Nil(t, pickedAt, "picked_at must remain NULL after rollback")

	// Verify fulfillment remains paid
	var fStatus string
	_ = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	assert.Equal(t, "paid", fStatus)
}

func TestPickingScan_SerializedItemViaBarcode(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, false)

	var barcode string
	_ = f.db.QueryRow(ctx, `SELECT pv.barcode FROM product_variants pv JOIN order_items oi ON oi.product_variant_id = pv.id WHERE oi.id = $1`, itemID).Scan(&barcode)

	_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, barcode)
	require.ErrorIs(t, err, fulfillment.ErrCannotPickSerializedWithBarcode)

	// Verify allocation picked_at remains NULL
	var pickedAt interface{}
	_ = f.db.QueryRow(ctx, `SELECT picked_at FROM order_item_allocations WHERE order_item_id = $1`, itemID).Scan(&pickedAt)
	assert.Nil(t, pickedAt)
}

func TestPickingScan_AmbiguousLegacyCode(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID1 := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	// Create second order item for the same variant
	var prodID, variantID uuid.UUID
	var barcode string
	err := f.db.QueryRow(ctx, `
		SELECT oi.product_id, oi.product_variant_id, pv.barcode
		FROM order_items oi
		JOIN product_variants pv ON pv.id = oi.product_variant_id
		WHERE oi.id = $1
	`, itemID1).Scan(&prodID, &variantID, &barcode)
	require.NoError(t, err)

	itemID2 := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Duplicate Variant Item', 'slug', 100, 1, 100, $6, 0)
	`, itemID2, orderID, prodID, variantID, f.sellerID, fulfillmentID)
	require.NoError(t, err)

	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, barcode)
	require.ErrorIs(t, err, fulfillment.ErrAmbiguousPickingCode)

	// Neither item incremented
	var p1, p2 int
	_ = f.db.QueryRow(ctx, `SELECT picked_quantity FROM order_items WHERE id = $1`, itemID1).Scan(&p1)
	_ = f.db.QueryRow(ctx, `SELECT picked_quantity FROM order_items WHERE id = $1`, itemID2).Scan(&p2)
	assert.Equal(t, 0, p1)
	assert.Equal(t, 0, p2)

	// Order and fulfillment remain paid
	var oStatus, fStatus string
	_ = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	_ = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	assert.Equal(t, "paid", oStatus, "order status must remain paid")
	assert.Equal(t, "paid", fStatus, "fulfillment status must remain paid")
}

func TestPickingScan_ForeignZMU_NoMutation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID1, fulfillmentID1 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID1 := f.createOrderItem(t, ctx, orderID1, fulfillmentID1, 1, 0)
	f.createAllocation(t, ctx, itemID1, false)

	orderID2, fulfillmentID2 := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	_ = f.createOrderItem(t, ctx, orderID2, fulfillmentID2, 1, 0)

	var unitCode string
	_ = f.db.QueryRow(ctx, `SELECT u.unit_code FROM inventory_units u JOIN order_item_allocations a ON a.inventory_unit_id = u.id WHERE a.order_item_id = $1`, itemID1).Scan(&unitCode)

	adminID := f.adminID
	_, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID2, unitCode)
	require.ErrorIs(t, err, fulfillment.ErrUnitAllocatedToOtherOrder)

	// Foreign allocation unchanged
	var pickedAt interface{}
	_ = f.db.QueryRow(ctx, `SELECT picked_at FROM order_item_allocations WHERE order_item_id = $1`, itemID1).Scan(&pickedAt)
	assert.Nil(t, pickedAt)

	// Current fulfillment status does NOT move to assembling
	var f2Status string
	_ = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID2).Scan(&f2Status)
	assert.Equal(t, "paid", f2Status)
}

func TestPickingScan_SerializedConcurrency(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, false)

	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	unitCode := po.Items[0].AllocatedUnits[0].UnitCode
	adminID := f.adminID

	type scanOutcome struct {
		res *fulfillment.PickingScanResult
		err error
	}
	outcomes := make([]scanOutcome, 2)

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		idx := i
		go func() {
			defer wg.Done()
			res, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, unitCode)
			outcomes[idx] = scanOutcome{res: res, err: err}
		}()
	}
	wg.Wait()

	// Both callers must succeed without error
	require.NoError(t, outcomes[0].err)
	require.NoError(t, outcomes[1].err)

	// Exactly one is NewlyPicked, the other is AlreadyPicked
	newlyPickedCount := 0
	alreadyPickedCount := 0
	for _, o := range outcomes {
		if o.res.ScanResult.NewlyPicked {
			newlyPickedCount++
		}
		if o.res.ScanResult.AlreadyPicked {
			alreadyPickedCount++
		}
	}
	assert.Equal(t, 1, newlyPickedCount, "exactly one caller gets newlyPicked")
	assert.Equal(t, 1, alreadyPickedCount, "exactly one caller gets alreadyPicked")

	poFinal, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, 1, poFinal.Items[0].PickedQuantity)
}

func TestPickingScan_LegacyConcurrency_LastUnit(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 3, 2) // picked 2 of 3
	var barcode string
	_ = f.db.QueryRow(ctx, `SELECT pv.barcode FROM product_variants pv JOIN order_items oi ON oi.product_variant_id = pv.id WHERE oi.id = $1`, itemID).Scan(&barcode)

	adminID := f.adminID

	type scanOutcome struct {
		res *fulfillment.PickingScanResult
		err error
	}
	outcomes := make([]scanOutcome, 2)

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		idx := i
		go func() {
			defer wg.Done()
			res, err := f.svc.ScanPickingCode(ctx, adminID, fulfillmentID, barcode)
			outcomes[idx] = scanOutcome{res: res, err: err}
		}()
	}
	wg.Wait()

	// Both callers must receive safe response without error
	require.NoError(t, outcomes[0].err)
	require.NoError(t, outcomes[1].err)

	newlyPickedCount := 0
	alreadyCompleteCount := 0
	for _, o := range outcomes {
		if o.res.ScanResult.NewlyPicked {
			newlyPickedCount++
		}
		if o.res.ScanResult.AlreadyComplete {
			alreadyCompleteCount++
		}
	}
	assert.Equal(t, 1, newlyPickedCount, "exactly one caller gets newlyPicked")
	assert.Equal(t, 1, alreadyCompleteCount, "exactly one caller gets alreadyComplete")

	poFinal, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, 3, poFinal.Items[0].PickedQuantity, "final picked_quantity must equal quantity 3, never 4")
}

func TestPickingScan_StockInvariants(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, false)

	var prodID, variantID, unitID uuid.UUID
	var unitCode string
	_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	_ = f.db.QueryRow(ctx, `SELECT u.id, u.unit_code FROM inventory_units u JOIN order_item_allocations a ON a.inventory_unit_id = u.id WHERE a.order_item_id = $1`, itemID).Scan(&unitID, &unitCode)

	invID := f.createInventoryItem(t, ctx, prodID, variantID, 10, 2)

	assertInvariants := func() {
		var uStatus string
		_ = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, unitID).Scan(&uStatus)
		assert.Equal(t, "warehouse", uStatus, "inventory_units.status must remain warehouse")

		var totalStock, reservedStock int
		_ = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1`, invID).Scan(&totalStock, &reservedStock)
		assert.Equal(t, 10, totalStock, "inventory_items.total_stock unchanged")
		assert.Equal(t, 2, reservedStock, "inventory_items.reserved_stock unchanged")

		var allocActive int
		_ = f.db.QueryRow(ctx, `SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1 AND released_at IS NULL`, unitID).Scan(&allocActive)
		assert.Equal(t, 1, allocActive, "active allocation remains active")

		var shipmentCount int
		_ = f.db.QueryRow(ctx, `SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulfillmentID).Scan(&shipmentCount)
		assert.Equal(t, 0, shipmentCount, "no shipment created")
	}

	// Before scan
	assertInvariants()

	// 1. Rejected scan (e.g. unknown code)
	_, _ = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, "UNKNOWN_CODE")
	assertInvariants()

	// 2. Successful scan
	res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode)
	require.NoError(t, err)
	assert.True(t, res.ScanResult.NewlyPicked)
	assertInvariants()

	// 3. Duplicate scan
	res2, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode)
	require.NoError(t, err)
	assert.True(t, res2.ScanResult.AlreadyPicked)
	assertInvariants()
}
