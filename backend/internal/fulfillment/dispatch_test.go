package fulfillment_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
)

func TestDispatch_Serialized_Success(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)

	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)

	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	// Create 2 allocations, both picked
	f.createAllocation(t, ctx, itemID, true)
	f.createAllocation(t, ctx, itemID, true)

	// Create a reservation in 'converted' status
	resID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, quantity, status, order_id, created_at, expires_at)
		SELECT $1, id, product_id, product_variant_id, 2, 'converted', $2, now(), now() + interval '1 hour'
		FROM inventory_items WHERE product_variant_id = $3
	`, resID, orderID, variantID)
	require.NoError(t, err)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, fulfillmentID, res.FulfillmentID)
	assert.Equal(t, orderID, res.OrderID)
	assert.Equal(t, "shipped", res.FulfillmentStatus)
	assert.Equal(t, "shipped", res.OrderStatus)
	assert.Equal(t, "shipped", res.ShipmentStatus)
	assert.NotEqual(t, uuid.Nil, res.ShipmentID)
	assert.WithinDuration(t, time.Now(), res.ShippedAt, 5*time.Second)

	// 1. Verify fulfillment status in DB
	var fStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", fStatus)

	// 2. Verify parent order status in DB
	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", oStatus)

	// 3. Verify stock was decremented by 2
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 3, reservedStock)

	// 4. Verify inventory units are now 'shipped'
	rows, err := f.db.Query(ctx, `
		SELECT u.status 
		FROM inventory_units u 
		JOIN order_item_allocations a ON a.inventory_unit_id = u.id 
		WHERE a.order_item_id = $1
	`, itemID)
	require.NoError(t, err)
	defer rows.Close()

	unitCount := 0
	for rows.Next() {
		var uStatus string
		require.NoError(t, rows.Scan(&uStatus))
		assert.Equal(t, "shipped", uStatus)
		unitCount++
	}
	assert.Equal(t, 2, unitCount)

	// 5. Verify allocation lineage: released_at remains NULL, picked_at preserved
	arows, err := f.db.Query(ctx, `SELECT picked_at, released_at FROM order_item_allocations WHERE order_item_id = $1`, itemID)
	require.NoError(t, err)
	defer arows.Close()

	for arows.Next() {
		var pickedAt *time.Time
		var releasedAt *time.Time
		require.NoError(t, arows.Scan(&pickedAt, &releasedAt))
		assert.NotNil(t, pickedAt)
		assert.Nil(t, releasedAt)
	}

	// 6. Verify reservation remains converted
	var resStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, resID).Scan(&resStatus)
	require.NoError(t, err)
	assert.Equal(t, "converted", resStatus)

	// 7. Verify shipment row
	var sStatus string
	var sShippedAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT status, shipped_at FROM shipments WHERE id = $1`, res.ShipmentID).Scan(&sStatus, &sShippedAt)
	require.NoError(t, err)
	assert.Equal(t, "shipped", sStatus)
	assert.NotNil(t, sShippedAt)

	// 8. Verify shipment_events row
	var eventCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'shipped'`, res.ShipmentID).Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount)
}

func TestDispatch_Legacy_Success(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 3, 3)

	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)

	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, "shipped", res.FulfillmentStatus)
	assert.Equal(t, "shipped", res.OrderStatus)

	// Verify aggregate stock decremented by 3
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 7, totalStock)
	assert.Equal(t, 2, reservedStock)
}

func TestDispatch_MultipleItems_SameInventoryItem_AggregatesCorrectly(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")

	// Create item 1 (quantity 2)
	itemID1 := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID1).Scan(&prodID, &variantID)
	require.NoError(t, err)

	// Create item 2 with the SAME product_id and product_variant_id (quantity 3)
	itemID2 := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Item 2', 'slug', 100, 3, 300, $6, 3)
	`, itemID2, orderID, prodID, variantID, f.sellerID, fulfillmentID)
	require.NoError(t, err)

	f.createInventoryItem(t, ctx, prodID, variantID, 15, 10)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Total quantity shipped = 2 + 3 = 5
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 5, reservedStock)
}

func TestDispatch_ReusesExistingPendingShipment(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 1)

	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	// Create an existing pending shipment
	existingShipmentID := uuid.New()
	carrier := "CDEK"
	trackingNum := "TRACK-999"
	trackingURL := "https://cdek.ru/track/999"
	_, err = f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6, now(), now())
	`, existingShipmentID, orderID, fulfillmentID, carrier, trackingNum, trackingURL)
	require.NoError(t, err)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, existingShipmentID, res.ShipmentID)
	assert.Equal(t, "shipped", res.ShipmentStatus)

	// Verify only 1 shipment exists for this fulfillment
	var count int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulfillmentID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify carrier and tracking number preserved
	var c, tn, tu, s string
	err = f.db.QueryRow(ctx, `SELECT carrier, tracking_number, tracking_url, status FROM shipments WHERE id = $1`, existingShipmentID).Scan(&c, &tn, &tu, &s)
	require.NoError(t, err)
	assert.Equal(t, carrier, c)
	assert.Equal(t, trackingNum, tn)
	assert.Equal(t, trackingURL, tu)
	assert.Equal(t, "shipped", s)

	// Verify shipment_events record has from_status = 'pending', to_status = 'shipped'
	var fromStatus, toStatus string
	err = f.db.QueryRow(ctx, `SELECT from_status, to_status FROM shipment_events WHERE shipment_id = $1`, existingShipmentID).Scan(&fromStatus, &toStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending", fromStatus)
	assert.Equal(t, "shipped", toStatus)
}

func TestDispatch_RejectsContradictoryTerminalShipmentState(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 1)

	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	// Create an existing shipment that is already delivered
	existingShipmentID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now(), now())
	`, existingShipmentID, orderID, fulfillmentID)
	require.NoError(t, err)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrShipmentContradictoryState)
}

func TestDispatch_RejectsWrongFulfillmentStatus(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	// 1. Assembling fulfillment
	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 1)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)
}

func TestDispatch_RejectsUnpickedSerializedAllocation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	// 1 picked, 1 unpicked
	f.createAllocation(t, ctx, itemID, true)
	f.createAllocation(t, ctx, itemID, false)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFullyPicked)
}

func TestDispatch_RejectsWrongInventoryUnitState(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	// Create unit with 'damaged' status
	unitID, _ := f.createUnitWithStatus(t, ctx, itemID, "damaged")
	// Mark allocation as picked
	_, err = f.db.Exec(ctx, `UPDATE order_item_allocations SET picked_at = now() WHERE inventory_unit_id = $1`, unitID)
	require.NoError(t, err)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrInventoryUnitStateConflict)
}

func TestDispatch_RejectsPartialOrExcessAllocations(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	// Partial allocation: Q=2, A=1
	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)
	f.createAllocation(t, ctx, itemID, true)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrInvariantViolation)

	// Excess allocation: Q=1, A=2
	orderID2, fulfillmentID2 := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID2 := f.createOrderItem(t, ctx, orderID2, fulfillmentID2, 1, 0)
	var prodID2, variantID2 uuid.UUID
	err = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID2).Scan(&prodID2, &variantID2)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID2, variantID2, 10, 5)
	f.createAllocation(t, ctx, itemID2, true)
	f.createAllocation(t, ctx, itemID2, true)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID2)
	assert.ErrorIs(t, err, fulfillment.ErrInvariantViolation)
}

func TestDispatch_RejectsInsufficientTotalStock(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)

	// total_stock = 1, reserved_stock = 1, but shipped quantity = 2
	f.createInventoryItem(t, ctx, prodID, variantID, 1, 1)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrInsufficientTotalStock)

	// Check that fulfillment status is still 'packed'
	var status string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "packed", status)
}

func TestDispatch_RejectsInsufficientReservedStock(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)

	// total_stock = 10, but reserved_stock = 1 (shipped quantity = 2)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 1)

	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrInsufficientReservedStock)
}

func TestDispatch_RepeatedDispatch_RejectedWithoutSecondDecrement(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	// First dispatch -> success
	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify stock is now 8, 3
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 3, reservedStock)

	// Second dispatch -> rejected
	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)

	// Verify stock was NOT decremented again
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 3, reservedStock)
}

func TestDispatch_Concurrent_Safe(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	var wg sync.WaitGroup
	concurrentAttempts := 5
	successCount := 0
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrentAttempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				errCount++
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, successCount, "exactly 1 dispatch should succeed")
	assert.Equal(t, 4, errCount, "4 concurrent dispatches should fail")

	// Verify stock decremented only once
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 3, reservedStock)

	// Verify only 1 shipment exists
	var count int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulfillmentID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify exactly 1 shipment_event with to_status = 'shipped'
	var eventCount int
	err = f.db.QueryRow(ctx, `
		SELECT count(*) 
		FROM shipment_events se 
		JOIN shipments s ON s.id = se.shipment_id 
		WHERE s.fulfillment_id = $1 AND se.to_status = 'shipped'
	`, fulfillmentID).Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount)
}

func TestDispatch_ParentOrderStatusMatrix(t *testing.T) {
	ctx := context.Background()

	// Case 1: Shipped + Assembling sibling -> Parent order is 'assembling'
	t.Run("Shipped + Assembling sibling -> Parent assembling", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		orderID, f1ID := f.createOrderAndFulfillment(t, ctx, "assembling", "packed")
		seller2ID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Seller 2', $2, $3, 'active', now(), now())
		`, seller2ID, uuid.New().String(), uuid.New().String()+"@ex.com")
		require.NoError(t, err)

		f2ID := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'assembling', 1000, 900, 900)
		`, f2ID, orderID, seller2ID)
		require.NoError(t, err)

		item1 := f.createOrderItem(t, ctx, orderID, f1ID, 1, 1)
		var pID, vID uuid.UUID
		_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, item1).Scan(&pID, &vID)
		f.createInventoryItem(t, ctx, pID, vID, 10, 5)

		res, err := f.svc.DispatchFulfillment(ctx, f.adminID, f1ID)
		require.NoError(t, err)
		assert.Equal(t, "shipped", res.FulfillmentStatus)
		assert.Equal(t, "assembling", res.OrderStatus)
	})

	// Case 2: Shipped + Packed sibling -> Parent order is 'packed'
	t.Run("Shipped + Packed sibling -> Parent packed", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		orderID, f1ID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
		seller2ID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Seller 2', $2, $3, 'active', now(), now())
		`, seller2ID, uuid.New().String(), uuid.New().String()+"@ex.com")
		require.NoError(t, err)

		f2ID := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'packed', 1000, 900, 900)
		`, f2ID, orderID, seller2ID)
		require.NoError(t, err)

		item1 := f.createOrderItem(t, ctx, orderID, f1ID, 1, 1)
		var pID, vID uuid.UUID
		_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, item1).Scan(&pID, &vID)
		f.createInventoryItem(t, ctx, pID, vID, 10, 5)

		res, err := f.svc.DispatchFulfillment(ctx, f.adminID, f1ID)
		require.NoError(t, err)
		assert.Equal(t, "shipped", res.FulfillmentStatus)
		assert.Equal(t, "packed", res.OrderStatus)
	})

	// Case 3: All Shipped (2 of 2) -> Parent order becomes 'shipped'
	t.Run("All Shipped -> Parent shipped", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		orderID, f1ID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
		seller2ID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Seller 2', $2, $3, 'active', now(), now())
		`, seller2ID, uuid.New().String(), uuid.New().String()+"@ex.com")
		require.NoError(t, err)

		f2ID := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'packed', 1000, 900, 900)
		`, f2ID, orderID, seller2ID)
		require.NoError(t, err)

		item1 := f.createOrderItem(t, ctx, orderID, f1ID, 1, 1)
		var pID1, vID1 uuid.UUID
		_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, item1).Scan(&pID1, &vID1)
		f.createInventoryItem(t, ctx, pID1, vID1, 10, 5)

		item2 := f.createOrderItem(t, ctx, orderID, f2ID, 1, 1)
		var pID2, vID2 uuid.UUID
		_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, item2).Scan(&pID2, &vID2)
		f.createInventoryItem(t, ctx, pID2, vID2, 10, 5)

		res1, err := f.svc.DispatchFulfillment(ctx, f.adminID, f1ID)
		require.NoError(t, err)
		assert.Equal(t, "shipped", res1.FulfillmentStatus)
		assert.Equal(t, "packed", res1.OrderStatus)

		res2, err := f.svc.DispatchFulfillment(ctx, f.adminID, f2ID)
		require.NoError(t, err)
		assert.Equal(t, "shipped", res2.FulfillmentStatus)
		assert.Equal(t, "shipped", res2.OrderStatus)
	})

	// Case 4: Shipped + Delivered sibling -> Parent order is 'shipped'
	t.Run("Shipped + Delivered sibling -> Parent shipped", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		orderID, f1ID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
		seller2ID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Seller 2', $2, $3, 'active', now(), now())
		`, seller2ID, uuid.New().String(), uuid.New().String()+"@ex.com")
		require.NoError(t, err)

		f2ID := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'delivered', 1000, 900, 900)
		`, f2ID, orderID, seller2ID)
		require.NoError(t, err)

		item1 := f.createOrderItem(t, ctx, orderID, f1ID, 1, 1)
		var pID, vID uuid.UUID
		_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, item1).Scan(&pID, &vID)
		f.createInventoryItem(t, ctx, pID, vID, 10, 5)

		res, err := f.svc.DispatchFulfillment(ctx, f.adminID, f1ID)
		require.NoError(t, err)
		assert.Equal(t, "shipped", res.FulfillmentStatus)
		assert.Equal(t, "shipped", res.OrderStatus)
	})
}

func TestDispatch_GenericStatusUpdateCannotBypassPhysicalDispatch(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	shipmentID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', now(), now())
	`, shipmentID, orderID, fulfillmentID)
	require.NoError(t, err)

	// Attempt to set status to 'shipped' directly via generic status update
	err = f.svc.UpdateShipmentStatus(ctx, f.adminID, shipmentID, fulfillment.UpdateShipmentStatusRequest{
		Status: "shipped",
	})
	assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)

	// Attempt to set status to 'delivered' directly via generic status update
	err = f.svc.UpdateShipmentStatus(ctx, f.adminID, shipmentID, fulfillment.UpdateShipmentStatusRequest{
		Status: "delivered",
	})
	assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)
}

func TestDispatch_IncoherentParentOrder_Rejected(t *testing.T) {
	ctx := context.Background()

	incoherentStatuses := []string{
		"created",
		"awaiting_payment",
		"paid",
		"shipped",
		"delivered",
		"cancelled",
	}

	for _, invalidOrderStatus := range incoherentStatuses {
		t.Run("parent_"+invalidOrderStatus, func(t *testing.T) {
			f := setupPickingFixture(t, ctx)
			defer f.db.Close()

			orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, invalidOrderStatus, "packed")
			itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2)
			var prodID, variantID uuid.UUID
			err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
			require.NoError(t, err)
			f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

			_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
			assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)

			// Verify ZERO physical inventory or status mutations
			var fStatus string
			err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
			require.NoError(t, err)
			assert.Equal(t, "packed", fStatus)

			var oStatus string
			err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
			require.NoError(t, err)
			assert.Equal(t, invalidOrderStatus, oStatus)

			var totalStock, reservedStock int
			err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
			require.NoError(t, err)
			assert.Equal(t, 10, totalStock)
			assert.Equal(t, 5, reservedStock)
		})
	}
}

func TestDispatch_ShipmentStateAllowlist(t *testing.T) {
	ctx := context.Background()

	t.Run("allowed pre-dispatch states", func(t *testing.T) {
		allowedStates := []string{"pending", "assembling", "packed"}
		for _, st := range allowedStates {
			f := setupPickingFixture(t, ctx)
			defer f.db.Close()

			orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
			itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 1)
			var prodID, variantID uuid.UUID
			_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
			f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

			sID := uuid.New()
			_, err := f.db.Exec(ctx, `
				INSERT INTO shipments (id, order_id, fulfillment_id, status, created_at, updated_at)
				VALUES ($1, $2, $3, $4, now(), now())
			`, sID, orderID, fulfillmentID, st)
			require.NoError(t, err)

			res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
			require.NoError(t, err, "state %s should be allowed for pre-dispatch shipment", st)
			assert.Equal(t, sID, res.ShipmentID)
			assert.Equal(t, "shipped", res.ShipmentStatus)
		}
	})

	t.Run("rejected contradictory states", func(t *testing.T) {
		rejectedStates := []string{"shipped", "delivered", "cancelled", "failed"}
		for _, st := range rejectedStates {
			f := setupPickingFixture(t, ctx)
			defer f.db.Close()

			orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
			itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 1)
			var prodID, variantID uuid.UUID
			_ = f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
			f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

			sID := uuid.New()
			_, err := f.db.Exec(ctx, `
				INSERT INTO shipments (id, order_id, fulfillment_id, status, created_at, updated_at)
				VALUES ($1, $2, $3, $4, now(), now())
			`, sID, orderID, fulfillmentID, st)
			require.NoError(t, err)

			_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
			assert.ErrorIs(t, err, fulfillment.ErrShipmentContradictoryState, "state %s should be rejected", st)
		}
	})
}

func TestDispatch_ConcurrentWithOrderCancellation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	f.createAllocation(t, ctx, itemID, true)
	f.createAllocation(t, ctx, itemID, true)

	var wg sync.WaitGroup
	wg.Add(2)

	var dispatchErr error
	var cancelErr error

	// Goroutine 1: Dispatch
	go func() {
		defer wg.Done()
		_, dispatchErr = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	}()

	// Goroutine 2: Cancel order
	go func() {
		defer wg.Done()
		tx, err := f.db.Begin(ctx)
		if err != nil {
			cancelErr = err
			return
		}
		var oStatus string
		err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&oStatus)
		if err != nil {
			_ = tx.Rollback(ctx)
			cancelErr = err
			return
		}
		if oStatus == "shipped" || oStatus == "delivered" {
			_ = tx.Rollback(ctx)
			cancelErr = errors.New("cannot cancel shipped order")
			return
		}
		_, err = tx.Exec(ctx, `UPDATE orders SET status = 'cancelled', updated_at = now() WHERE id = $1`, orderID)
		if err != nil {
			_ = tx.Rollback(ctx)
			cancelErr = err
			return
		}
		_, err = tx.Exec(ctx, `UPDATE order_fulfillments SET status = 'cancelled', updated_at = now() WHERE id = $1`, fulfillmentID)
		if err != nil {
			_ = tx.Rollback(ctx)
			cancelErr = err
			return
		}
		cancelErr = tx.Commit(ctx)
	}()

	wg.Wait()

	// Invariant: Exactly one must succeed and determine the consistent state
	var finalFulfillmentStatus, finalOrderStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&finalFulfillmentStatus)
	require.NoError(t, err)
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&finalOrderStatus)
	require.NoError(t, err)

	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)

	if dispatchErr == nil {
		// Dispatch won
		assert.Equal(t, "shipped", finalFulfillmentStatus)
		assert.Equal(t, "shipped", finalOrderStatus)
		assert.Equal(t, 8, totalStock)
		assert.Equal(t, 3, reservedStock)
	} else {
		// Cancel won
		assert.ErrorIs(t, dispatchErr, fulfillment.ErrDispatchNotAllowed)
		assert.NoError(t, cancelErr)
		assert.Equal(t, "cancelled", finalFulfillmentStatus)
		assert.Equal(t, "cancelled", finalOrderStatus)
		assert.Equal(t, 10, totalStock, "no stock decrement on cancel")
		assert.Equal(t, 5, reservedStock, "no stock decrement on cancel")

		// Units must still be warehouse
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units u JOIN order_item_allocations a ON a.inventory_unit_id = u.id WHERE a.order_item_id = $1 LIMIT 1`, itemID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", uStatus)
	}
}

