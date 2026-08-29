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
)

func TestPacking_FullyPickedSerialized_Success(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	// Create 2 allocations, both already picked
	f.createAllocation(t, ctx, itemID, true)
	f.createAllocation(t, ctx, itemID, true)

	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, fulfillmentID, res.FulfillmentID)
	assert.Equal(t, orderID, res.OrderID)
	assert.Equal(t, "packed", res.FulfillmentStatus)
	assert.Equal(t, "packed", res.OrderStatus)
	assert.WithinDuration(t, time.Now(), res.PackedAt, 5*time.Second)

	// Verify database persistence
	var fStatus string
	var fPackedAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT status, packed_at FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus, &fPackedAt)
	require.NoError(t, err)
	assert.Equal(t, "packed", fStatus)
	require.NotNil(t, fPackedAt)
	assert.WithinDuration(t, time.Now(), *fPackedAt, 5*time.Second)

	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "packed", oStatus)
}

func TestPacking_FullyPickedLegacy_Success(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	_ = f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2) // picked_quantity = 2 of 2, allocations = 0

	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, fulfillmentID, res.FulfillmentID)
	assert.Equal(t, orderID, res.OrderID)
	assert.Equal(t, "packed", res.FulfillmentStatus)
	assert.Equal(t, "packed", res.OrderStatus)
	assert.WithinDuration(t, time.Now(), res.PackedAt, 5*time.Second)

	// Verify database persistence
	var fStatus string
	var fPackedAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT status, packed_at FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus, &fPackedAt)
	require.NoError(t, err)
	assert.Equal(t, "packed", fStatus)
	require.NotNil(t, fPackedAt)

	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "packed", oStatus)
}

func TestPacking_Serialized_UnpickedZMU_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	// Create 2 allocations: 1 picked, 1 unpicked
	f.createAllocation(t, ctx, itemID, true)
	f.createAllocation(t, ctx, itemID, false)

	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFullyPicked)

	// Assert no mutation occurred
	var fStatus string
	var fPackedAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT status, packed_at FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus, &fPackedAt)
	require.NoError(t, err)
	assert.Equal(t, "assembling", fStatus)
	assert.Nil(t, fPackedAt)

	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "assembling", oStatus)
}

func TestPacking_Legacy_PartiallyPicked_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	_ = f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 1) // picked 1 of 2

	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFullyPicked)

	// Assert no mutation occurred
	var fStatus string
	var fPackedAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT status, packed_at FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus, &fPackedAt)
	require.NoError(t, err)
	assert.Equal(t, "assembling", fStatus)
	assert.Nil(t, fPackedAt)

	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "assembling", oStatus)
}

func TestPacking_PartialAllocation_InvariantViolation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0) // quantity = 2

	// Only 1 allocation created (0 < A < Q)
	f.createAllocation(t, ctx, itemID, true)

	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, fulfillment.ErrInvariantViolation)

	// Assert no mutation occurred
	var fStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "assembling", fStatus)
}

func TestPacking_ExcessAllocation_InvariantViolation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0) // quantity = 1

	// 2 allocations created (A > Q)
	f.createAllocation(t, ctx, itemID, true)
	f.createAllocation(t, ctx, itemID, true)

	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, fulfillment.ErrInvariantViolation)

	// Assert no mutation occurred
	var fStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "assembling", fStatus)
}

func TestPacking_WrongFulfillmentStatus_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	wrongStatuses := []string{"paid", "packed", "shipped", "delivered", "cancelled"}

	for _, wrongStatus := range wrongStatuses {
		t.Run("fulfillment_"+wrongStatus, func(t *testing.T) {
			orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", wrongStatus)
			itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
			f.createAllocation(t, ctx, itemID, true)

			res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
			assert.Nil(t, res)
			assert.ErrorIs(t, err, fulfillment.ErrPackingNotAllowed)
		})
	}
}

func TestPacking_IncoherentParentOrder_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	incoherentOrderStatuses := []string{"paid", "packed", "shipped", "delivered", "cancelled", "awaiting_payment"}

	for _, orderStatus := range incoherentOrderStatuses {
		t.Run("order_"+orderStatus, func(t *testing.T) {
			orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, orderStatus, "assembling")
			itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
			f.createAllocation(t, ctx, itemID, true)

			res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
			assert.Nil(t, res)
			assert.ErrorIs(t, err, fulfillment.ErrPackingNotAllowed)
		})
	}
}

func TestPacking_GuaranteesNoStockOrUnitMutation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)

	var variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)
	require.NoError(t, err)

	var prodID uuid.UUID
	err = f.db.QueryRow(ctx, `SELECT product_id FROM product_variants WHERE id = $1`, variantID).Scan(&prodID)
	require.NoError(t, err)

	invID := f.createInventoryItem(t, ctx, prodID, variantID, 10, 1)

	// Create physical unit and allocation
	unitID, _ := f.createUnitWithStatus(t, ctx, itemID, "warehouse")
	// Mark allocation picked
	_, err = f.db.Exec(ctx, `UPDATE order_item_allocations SET picked_at = now() WHERE inventory_unit_id = $1`, unitID)
	require.NoError(t, err)

	// Create reservation
	var buyerID uuid.UUID
	err = f.db.QueryRow(ctx, `SELECT user_id FROM orders WHERE id = $1`, orderID).Scan(&buyerID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, order_id, quantity, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 1, 'active', now() + interval '1 hour', now())
	`, resID, invID, prodID, variantID, buyerID, orderID)
	require.NoError(t, err)

	// Snapshot before packing
	var totalBefore, resBefore int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1`, invID).Scan(&totalBefore, &resBefore)
	require.NoError(t, err)

	var unitStatusBefore string
	err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, unitID).Scan(&unitStatusBefore)
	require.NoError(t, err)

	var resStatusBefore string
	err = f.db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, resID).Scan(&resStatusBefore)
	require.NoError(t, err)

	var allocPickedBefore *time.Time
	var allocReleasedBefore *time.Time
	err = f.db.QueryRow(ctx, `SELECT picked_at, released_at FROM order_item_allocations WHERE inventory_unit_id = $1`, unitID).Scan(&allocPickedBefore, &allocReleasedBefore)
	require.NoError(t, err)

	// Execute packing
	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "packed", res.FulfillmentStatus)

	// Verify post-pack guarantees
	var totalAfter, resAfter int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1`, invID).Scan(&totalAfter, &resAfter)
	require.NoError(t, err)
	assert.Equal(t, totalBefore, totalAfter, "total_stock must NOT be modified by packing")
	assert.Equal(t, resBefore, resAfter, "reserved_stock must NOT be modified by packing")

	var unitStatusAfter string
	err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, unitID).Scan(&unitStatusAfter)
	require.NoError(t, err)
	assert.Equal(t, unitStatusBefore, unitStatusAfter, "unit status must remain warehouse")
	assert.Equal(t, "warehouse", unitStatusAfter)

	var resStatusAfter string
	err = f.db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, resID).Scan(&resStatusAfter)
	require.NoError(t, err)
	assert.Equal(t, resStatusBefore, resStatusAfter, "reservation status must remain active")
	assert.Equal(t, "active", resStatusAfter)

	var allocPickedAfter *time.Time
	var allocReleasedAfter *time.Time
	err = f.db.QueryRow(ctx, `SELECT picked_at, released_at FROM order_item_allocations WHERE inventory_unit_id = $1`, unitID).Scan(&allocPickedAfter, &allocReleasedAfter)
	require.NoError(t, err)
	assert.Equal(t, allocPickedBefore, allocPickedAfter, "allocation picked_at must NOT change")
	assert.Nil(t, allocReleasedAfter, "allocation must NOT be released")
}

func TestPacking_GuaranteesNoShipmentCreated(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, true)

	res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	var shipmentCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipments WHERE fulfillment_id = $1 OR order_id = $2`, fulfillmentID, orderID).Scan(&shipmentCount)
	require.NoError(t, err)
	assert.Equal(t, 0, shipmentCount, "Packing must NOT create any shipment records")
}

func TestPacking_MultiFulfillment_ParentStatusRecalculation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	// Create order in assembling
	orderID := uuid.New()
	userID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Buyer', $2, 'hash', 'customer', 'active', now(), now())
	`, userID, uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'assembling', 2000, 'N', 'P', 'E', 'A')
	`, orderID, userID)
	require.NoError(t, err)

	// Fulfillment 1 (Seller 1)
	f1ID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'assembling', 1000, 900, 900)
	`, f1ID, orderID, f.sellerID)
	require.NoError(t, err)
	item1ID := f.createOrderItem(t, ctx, orderID, f1ID, 1, 0)
	f.createAllocation(t, ctx, item1ID, true)

	// Seller 2
	seller2ID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Packing Seller 2', $2, $3, 'active', now(), now())
	`, seller2ID, uuid.New().String(), uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat2', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prod2ID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod2', $4, 1000, 'published', now(), now())`, prod2ID, seller2ID, catID, uuid.New().String())
	require.NoError(t, err)

	var2ID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())`, var2ID, prod2ID, uuid.New().String(), uuid.New().String(), uuid.New().String())
	require.NoError(t, err)

	// Fulfillment 2 (Seller 2)
	f2ID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'assembling', 1000, 900, 900)
	`, f2ID, orderID, seller2ID)
	require.NoError(t, err)

	item2ID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Item 2', 'slug2', 100, 1, 100, $6, 0)
	`, item2ID, orderID, prod2ID, var2ID, seller2ID, f2ID)
	require.NoError(t, err)

	sup2ID := uuid.New()
	sup2ItemID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`, sup2ID, seller2ID, uuid.New().String()[:8])
	require.NoError(t, err)
	_, err = f.db.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())`, sup2ItemID, sup2ID, var2ID)
	require.NoError(t, err)

	unit2ID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'warehouse')`, unit2ID, uuid.New().String()[:12], var2ID, sup2ID, sup2ItemID)
	require.NoError(t, err)

	alloc2ID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at) VALUES ($1, $2, $3, now())`, alloc2ID, item2ID, unit2ID)
	require.NoError(t, err)

	// Step 1: Pack Fulfillment 1
	res1, err := f.svc.PackFulfillment(ctx, f.adminID, f1ID)
	require.NoError(t, err)
	assert.Equal(t, "packed", res1.FulfillmentStatus)
	assert.Equal(t, "assembling", res1.OrderStatus, "Parent order must remain assembling while F2 is still assembling")

	// Verify in DB
	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "assembling", oStatus)

	// Step 2: Pack Fulfillment 2
	res2, err := f.svc.PackFulfillment(ctx, f.adminID, f2ID)
	require.NoError(t, err)
	assert.Equal(t, "packed", res2.FulfillmentStatus)
	assert.Equal(t, "packed", res2.OrderStatus, "Parent order must become packed once all fulfillments are packed")

	// Verify in DB
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "packed", oStatus)
}

func TestPacking_ConcurrentAttempts_Consistent(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, true)

	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				errorCount++
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, 1, successCount, "Exactly one concurrent pack request must succeed")
	assert.Equal(t, concurrency-1, errorCount, "All other concurrent pack requests must be rejected")

	// State must be consistently packed
	var fStatus string
	err := f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "packed", fStatus)

	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "packed", oStatus)
}

func TestPacking_ConcurrentWithOrderStatusChange(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, true)

	var wg sync.WaitGroup
	wg.Add(2)

	var packErr error
	var cancelErr error

	// Goroutine 1: Pack fulfillment
	go func() {
		defer wg.Done()
		_, packErr = f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	}()

	// Goroutine 2: Concurrent order status update to cancelled
	go func() {
		defer wg.Done()
		tx, err := f.db.Begin(ctx)
		if err != nil {
			cancelErr = err
			return
		}
		defer tx.Rollback(ctx)

		var currStatus string
		err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&currStatus)
		if err != nil {
			cancelErr = err
			return
		}
		if currStatus != "assembling" {
			cancelErr = assert.AnError
			return
		}
		_, err = tx.Exec(ctx, `UPDATE orders SET status = 'cancelled', updated_at = now() WHERE id = $1`, orderID)
		if err != nil {
			cancelErr = err
			return
		}
		cancelErr = tx.Commit(ctx)
	}()

	wg.Wait()

	// Exactly one branch succeeded due to row locking
	var finalFStatus, finalOStatus string
	err := f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&finalFStatus)
	require.NoError(t, err)
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&finalOStatus)
	require.NoError(t, err)

	if packErr == nil {
		// Pack won: order and fulfillment are packed
		assert.Equal(t, "packed", finalFStatus)
		assert.Equal(t, "packed", finalOStatus)
		assert.Error(t, cancelErr)
	} else {
		// Cancel won: order is cancelled, fulfillment was NOT packed
		assert.Equal(t, "assembling", finalFStatus)
		assert.Equal(t, "cancelled", finalOStatus)
		assert.NoError(t, cancelErr)
		assert.ErrorIs(t, packErr, fulfillment.ErrPackingNotAllowed)
	}
}
