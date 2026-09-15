package fulfillment_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
)

func TestDispatchQueue_FilteringAndExclusions(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	// 1. Ready to dispatch: status = packed, order = packed
	o1ID, f1ID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	it1 := f.createOrderItem(t, ctx, o1ID, f1ID, 2, 2)
	f.createAllocation(t, ctx, it1, true)
	f.createAllocation(t, ctx, it1, true)

	// 2. Ready to dispatch in multi-fulfillment: status = packed, parent order = assembling (other fulfillment assembling)
	o2ID, f2ID := f.createOrderAndFulfillment(t, ctx, "assembling", "packed")
	it2 := f.createOrderItem(t, ctx, o2ID, f2ID, 1, 1)
	f.createAllocation(t, ctx, it2, true)

	// 3. Assembling fulfillment (should be EXCLUDED)
	o3ID, f3ID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	it3 := f.createOrderItem(t, ctx, o3ID, f3ID, 2, 2)
	f.createAllocation(t, ctx, it3, true)
	f.createAllocation(t, ctx, it3, true)

	// 4. Paid fulfillment (should be EXCLUDED)
	o4ID, f4ID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	_ = f.createOrderItem(t, ctx, o4ID, f4ID, 1, 0)

	// 5. Already shipped fulfillment (should be EXCLUDED)
	o5ID, f5ID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	it5 := f.createOrderItem(t, ctx, o5ID, f5ID, 1, 1)
	f.createAllocation(t, ctx, it5, true)

	// 6. Delivered fulfillment (should be EXCLUDED)
	o6ID, f6ID := f.createOrderAndFulfillment(t, ctx, "delivered", "delivered")
	it6 := f.createOrderItem(t, ctx, o6ID, f6ID, 1, 1)
	f.createAllocation(t, ctx, it6, true)

	// 7. Cancelled order with packed fulfillment (should be EXCLUDED)
	o7ID, f7ID := f.createOrderAndFulfillment(t, ctx, "cancelled", "packed")
	it7 := f.createOrderItem(t, ctx, o7ID, f7ID, 1, 1)
	f.createAllocation(t, ctx, it7, true)

	// 8. Packed fulfillment with already shipped shipment (should be EXCLUDED)
	o8ID, f8ID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	it8 := f.createOrderItem(t, ctx, o8ID, f8ID, 1, 1)
	f.createAllocation(t, ctx, it8, true)
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now())
	`, uuid.New(), o8ID, f8ID)
	require.NoError(t, err)

	// Query dispatch queue
	queue, err := f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)

	queueMap := make(map[uuid.UUID]fulfillment.DispatchQueueItem)
	for _, item := range queue {
		queueMap[item.FulfillmentID] = item
	}

	// Assertions:
	// Included items:
	assert.Contains(t, queueMap, f1ID, "packed fulfillment with packed order must be in dispatch queue")
	assert.Contains(t, queueMap, f2ID, "packed fulfillment in assembling multi-fulfillment order must be in dispatch queue")

	// Excluded items:
	assert.NotContains(t, queueMap, f3ID, "assembling fulfillment must be excluded from dispatch queue")
	assert.NotContains(t, queueMap, f4ID, "paid fulfillment must be excluded from dispatch queue")
	assert.NotContains(t, queueMap, f5ID, "shipped fulfillment must be excluded from dispatch queue")
	assert.NotContains(t, queueMap, f6ID, "delivered fulfillment must be excluded from dispatch queue")
	assert.NotContains(t, queueMap, f7ID, "cancelled order fulfillment must be excluded from dispatch queue")
	assert.NotContains(t, queueMap, f8ID, "fulfillment with already shipped shipment must be excluded from dispatch queue")

	// Verify details on f1ID
	item1 := queueMap[f1ID]
	assert.Equal(t, f1ID, item1.FulfillmentID)
	assert.Equal(t, o1ID, item1.OrderID)
	assert.Equal(t, "packed", item1.Status)
	assert.Equal(t, "packed", item1.OrderStatus)
	assert.Equal(t, 1, item1.ItemsCount)
	assert.Equal(t, 2, item1.TotalQuantity)

	// Verify JSON serialization contains NO financial fields and NO customer PII
	rawJSON, err := json.Marshal(item1)
	require.NoError(t, err)
	jsonStr := string(rawJSON)
	assert.NotContains(t, jsonStr, "subtotalCents")
	assert.NotContains(t, jsonStr, "commissionBps")
	assert.NotContains(t, jsonStr, "sellerAmountCents")
	assert.NotContains(t, jsonStr, "unitPriceCents")
	assert.NotContains(t, jsonStr, "lineTotalCents")
	assert.NotContains(t, jsonStr, "customerName")
	assert.NotContains(t, jsonStr, "customerPhone")
	assert.NotContains(t, jsonStr, "deliveryAddress")
}

func TestDispatchQueue_LifecycleTransitions(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	// Create order and fulfillment in assembling state
	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	it := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, it).Scan(&prodID, &variantID)
	require.NoError(t, err)
	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)
	f.createAllocation(t, ctx, it, true)

	// 1. Initially assembling -> NOT in dispatch queue
	q1, err := f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)
	for _, item := range q1 {
		assert.NotEqual(t, fulfillmentID, item.FulfillmentID, "assembling fulfillment must not be in dispatch queue")
	}

	// 2. Pack fulfillment -> appears in dispatch queue
	packRes, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, "packed", packRes.FulfillmentStatus)

	q2, err := f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)
	var foundInQueue bool
	for _, item := range q2 {
		if item.FulfillmentID == fulfillmentID {
			foundInQueue = true
			assert.Equal(t, "packed", item.Status)
			assert.Equal(t, 1, item.TotalQuantity)
		}
	}
	assert.True(t, foundInQueue, "packed fulfillment must appear in dispatch queue")

	// 3. Dispatch fulfillment -> disappears from dispatch queue
	dispatchRes, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, "shipped", dispatchRes.FulfillmentStatus)

	q3, err := f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)
	for _, item := range q3 {
		assert.NotEqual(t, fulfillmentID, item.FulfillmentID, "dispatched fulfillment must disappear from dispatch queue")
	}
}
