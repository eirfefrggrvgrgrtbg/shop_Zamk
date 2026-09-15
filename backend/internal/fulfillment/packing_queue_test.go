package fulfillment_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
)

func TestPackingQueue_FilteringAndExclusions(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	// 1. Ready to pack: serialized item fully picked
	o1ID, f1ID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	it1 := f.createOrderItem(t, ctx, o1ID, f1ID, 2, 0)
	f.createAllocation(t, ctx, it1, true)
	f.createAllocation(t, ctx, it1, true)

	// 2. Ready to pack: legacy item fully picked
	o2ID, f2ID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	_ = f.createOrderItem(t, ctx, o2ID, f2ID, 3, 3)

	// 3. Partially picked serialized (should be EXCLUDED)
	o3ID, f3ID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	it3 := f.createOrderItem(t, ctx, o3ID, f3ID, 2, 0)
	f.createAllocation(t, ctx, it3, true)
	f.createAllocation(t, ctx, it3, false) // unpicked

	// 4. Partially picked legacy (should be EXCLUDED)
	o4ID, f4ID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	_ = f.createOrderItem(t, ctx, o4ID, f4ID, 3, 2) // 2 of 3 picked

	// 5. Unpicked / Paid (should be EXCLUDED)
	o5ID, f5ID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	it5 := f.createOrderItem(t, ctx, o5ID, f5ID, 1, 0)
	f.createAllocation(t, ctx, it5, false)

	// 6. Already packed (should be EXCLUDED)
	o6ID, f6ID := f.createOrderAndFulfillment(t, ctx, "packed", "packed")
	it6 := f.createOrderItem(t, ctx, o6ID, f6ID, 1, 0)
	f.createAllocation(t, ctx, it6, true)

	// 7. Shipped (should be EXCLUDED)
	o7ID, f7ID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	it7 := f.createOrderItem(t, ctx, o7ID, f7ID, 1, 0)
	f.createAllocation(t, ctx, it7, true)

	// 8. Cancelled order (should be EXCLUDED)
	o8ID, f8ID := f.createOrderAndFulfillment(t, ctx, "cancelled", "assembling")
	it8 := f.createOrderItem(t, ctx, o8ID, f8ID, 1, 0)
	f.createAllocation(t, ctx, it8, true)

	// Query packing queue
	queue, err := f.svc.GetPackingQueue(ctx)
	require.NoError(t, err)

	// Build map of queue fulfillments
	queueMap := make(map[uuid.UUID]fulfillment.PackingQueueItem)
	for _, item := range queue {
		queueMap[item.FulfillmentID] = item
	}

	// Assertions:
	// Included items:
	assert.Contains(t, queueMap, f1ID, "serialized fully-picked fulfillment must be in packing queue")
	assert.Contains(t, queueMap, f2ID, "legacy fully-picked fulfillment must be in packing queue")

	// Excluded items:
	assert.NotContains(t, queueMap, f3ID, "partially picked serialized fulfillment must be excluded")
	assert.NotContains(t, queueMap, f4ID, "partially picked legacy fulfillment must be excluded")
	assert.NotContains(t, queueMap, f5ID, "unpicked paid fulfillment must be excluded")
	assert.NotContains(t, queueMap, f6ID, "already packed fulfillment must be excluded")
	assert.NotContains(t, queueMap, f7ID, "shipped fulfillment must be excluded")
	assert.NotContains(t, queueMap, f8ID, "cancelled order fulfillment must be excluded")

	// Verify details on f1ID
	f1Item := queueMap[f1ID]
	assert.Equal(t, f1ID, f1Item.FulfillmentID)
	assert.Equal(t, o1ID, f1Item.OrderID)
	assert.Equal(t, "assembling", f1Item.Status)
	assert.Equal(t, "assembling", f1Item.OrderStatus)
	assert.Equal(t, 1, f1Item.ItemsCount)
	assert.Equal(t, 2, f1Item.TotalQuantity)
	assert.Equal(t, 2, f1Item.PickedQuantity)
	assert.NotNil(t, f1Item.PickingCompletedAt)
	assert.False(t, f1Item.CreatedAt.IsZero())

	// Verify details on f2ID
	f2Item := queueMap[f2ID]
	assert.Equal(t, f2ID, f2Item.FulfillmentID)
	assert.Equal(t, o2ID, f2Item.OrderID)
	assert.Equal(t, 1, f2Item.ItemsCount)
	assert.Equal(t, 3, f2Item.TotalQuantity)
	assert.Equal(t, 3, f2Item.PickedQuantity)
}

func TestPackingQueue_DTOLeastPrivilege_NoPIIOrFinancialFields(t *testing.T) {
	item := fulfillment.PackingQueueItem{
		FulfillmentID:      uuid.New(),
		OrderID:            uuid.New(),
		OrderNumber:        "ORD-100",
		Status:             "assembling",
		OrderStatus:        "assembling",
		CreatedAt:          time.Now(),
		PickingCompletedAt: nil,
		ItemsCount:         2,
		TotalQuantity:      4,
		PickedQuantity:     4,
	}

	bytes, err := json.Marshal(item)
	require.NoError(t, err)

	var jsonMap map[string]interface{}
	err = json.Unmarshal(bytes, &jsonMap)
	require.NoError(t, err)

	// Critical check: verify forbidden commercial & PII fields do not exist in DTO
	forbiddenFields := []string{
		"subtotalCents",
		"subtotal_cents",
		"commissionBps",
		"commission_bps",
		"sellerAmountCents",
		"seller_amount_cents",
		"unitPriceCents",
		"unit_price_cents",
		"lineTotalCents",
		"line_total_cents",
		"customerPhone",
		"customer_phone",
		"deliveryAddress",
		"delivery_address",
		"priceCents",
		"price_cents",
		"payoutCents",
	}

	for _, field := range forbiddenFields {
		_, exists := jsonMap[field]
		assert.False(t, exists, "DTO must not expose forbidden field %q", field)
	}

	// Verify required operational fields exist
	assert.Contains(t, jsonMap, "fulfillmentId")
	assert.Contains(t, jsonMap, "orderId")
	assert.Contains(t, jsonMap, "orderNumber")
	assert.Contains(t, jsonMap, "status")
	assert.Contains(t, jsonMap, "orderStatus")
	assert.Contains(t, jsonMap, "itemsCount")
	assert.Contains(t, jsonMap, "totalQuantity")
	assert.Contains(t, jsonMap, "pickedQuantity")
}

func TestPackingQueue_Classification_SerializedVsLegacy(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	// Case A. SERIALIZED READY:
	// quantity = 2, active allocations = 2, both picked_at != NULL, order_items.picked_quantity = 0
	// Expected: INCLUDED
	oA, fA := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itA := f.createOrderItem(t, ctx, oA, fA, 2, 0) // picked_quantity strictly 0
	f.createAllocation(t, ctx, itA, true)
	f.createAllocation(t, ctx, itA, true)

	// Case B. SERIALIZED PARTIAL:
	// quantity = 2, active allocations = 2, only 1 picked_at != NULL
	// Expected: EXCLUDED
	oB, fB := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itB := f.createOrderItem(t, ctx, oB, fB, 2, 0)
	f.createAllocation(t, ctx, itB, true)
	f.createAllocation(t, ctx, itB, false)

	// Case C. SERIALIZED INVALID PARTIAL ALLOCATION:
	// quantity = 2, active allocations = 1 (count < quantity)
	// Expected: EXCLUDED
	oC, fC := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itC := f.createOrderItem(t, ctx, oC, fC, 2, 0)
	f.createAllocation(t, ctx, itC, true) // only 1 allocation created

	// Case C2. SERIALIZED INVALID OVER-ALLOCATION:
	// quantity = 1, active allocations = 2 (count > quantity)
	// Expected: EXCLUDED
	oC2, fC2 := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	itC2 := f.createOrderItem(t, ctx, oC2, fC2, 1, 0)
	f.createAllocation(t, ctx, itC2, true)
	f.createAllocation(t, ctx, itC2, true)

	// Case D. LEGACY READY:
	// active allocations = 0, picked_quantity == quantity
	// Expected: INCLUDED
	oD, fD := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	_ = f.createOrderItem(t, ctx, oD, fD, 3, 3)

	// Case E. LEGACY PARTIAL:
	// active allocations = 0, picked_quantity < quantity
	// Expected: EXCLUDED
	oE, fE := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	_ = f.createOrderItem(t, ctx, oE, fE, 3, 2)

	// Query packing queue
	queue, err := f.svc.GetPackingQueue(ctx)
	require.NoError(t, err)

	queueMap := make(map[uuid.UUID]fulfillment.PackingQueueItem)
	for _, item := range queue {
		queueMap[item.FulfillmentID] = item
	}

	// Assertions for Case A through E:
	assert.Contains(t, queueMap, fA, "Case A: SERIALIZED READY must be INCLUDED in packing queue even when order_items.picked_quantity is 0")
	itemA := queueMap[fA]
	assert.Equal(t, 2, itemA.TotalQuantity)
	assert.Equal(t, 2, itemA.PickedQuantity, "PickedQuantity must reflect picked allocations count (2) not table picked_quantity (0)")

	assert.NotContains(t, queueMap, fB, "Case B: SERIALIZED PARTIAL must be EXCLUDED from packing queue")
	assert.NotContains(t, queueMap, fC, "Case C: SERIALIZED INVALID PARTIAL ALLOCATION must be EXCLUDED from packing queue")
	assert.NotContains(t, queueMap, fC2, "Case C2: SERIALIZED INVALID OVER-ALLOCATION must be EXCLUDED from packing queue")

	assert.Contains(t, queueMap, fD, "Case D: LEGACY READY must be INCLUDED in packing queue")
	itemD := queueMap[fD]
	assert.Equal(t, 3, itemD.TotalQuantity)
	assert.Equal(t, 3, itemD.PickedQuantity)

	assert.NotContains(t, queueMap, fE, "Case E: LEGACY PARTIAL must be EXCLUDED from packing queue")
}
