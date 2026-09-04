package fulfillment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountActionablePicking_MandatoryCases(t *testing.T) {
	ctx := context.Background()

	// 1. paid + 0/1 picked -> count 1
	t.Run("paid_0_of_1_picked", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		before, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)

		orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
		itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
		f.createAllocation(t, ctx, itemID, false)

		actionable, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.True(t, actionable, "paid order with 0/1 picked unit must be actionable")

		after, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, after-before, "actionable count delta must increase by 1")
	})

	// 2. assembling + 1/2 picked -> count 1
	t.Run("assembling_1_of_2_picked", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		before, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)

		orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
		itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
		f.createAllocation(t, ctx, itemID, true)  // 1 picked
		f.createAllocation(t, ctx, itemID, false) // 1 unpicked

		actionable, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.True(t, actionable, "assembling order with 1/2 picked units must be actionable")

		after, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, after-before, "actionable count delta must increase by 1")
	})

	// 3. assembling + 1/1 picked -> count 0
	t.Run("assembling_1_of_1_picked", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		before, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)

		orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
		itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
		f.createAllocation(t, ctx, itemID, true) // fully picked

		actionable, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.False(t, actionable, "assembling order with 1/1 picked units must not be actionable for picking")

		after, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, after-before, "actionable count delta must be 0 for fully picked assembling order")
	})

	// 4. awaiting_payment -> count 0
	t.Run("awaiting_payment", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		before, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)

		orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "awaiting_payment", "awaiting_payment")
		itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
		f.createAllocation(t, ctx, itemID, false)

		actionable, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.False(t, actionable, "awaiting_payment order must not be actionable for picking")

		after, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, after-before)
	})

	// 5. cancelled/refunded/terminal invalid order -> count 0
	t.Run("terminal_orders", func(t *testing.T) {
		terminalStatuses := []struct {
			orderStatus string
			fulfStatus  string
		}{
			{"cancelled", "paid"},
			{"paid", "cancelled"},
			{"paid", "refunded"},
			{"packed", "packed"},
			{"shipped", "shipped"},
			{"delivered", "delivered"},
		}

		for _, ts := range terminalStatuses {
			t.Run(ts.orderStatus+"_"+ts.fulfStatus, func(t *testing.T) {
				f := setupPickingFixture(t, ctx)
				defer f.db.Close()

				before, err := f.svc.CountActionablePicking(ctx)
				require.NoError(t, err)

				orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, ts.orderStatus, ts.fulfStatus)
				itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
				f.createAllocation(t, ctx, itemID, false)

				actionable, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
				require.NoError(t, err)
				assert.False(t, actionable, "terminal status %s/%s must not be actionable for picking", ts.orderStatus, ts.fulfStatus)

				after, err := f.svc.CountActionablePicking(ctx)
				require.NoError(t, err)
				assert.Equal(t, 0, after-before)
			})
		}
	})

	// 6. serialized fully picked -> count 0
	t.Run("serialized_fully_picked", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		before, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)

		orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
		itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
		f.createAllocation(t, ctx, itemID, true)
		f.createAllocation(t, ctx, itemID, true)

		actionable, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.False(t, actionable, "serialized item with all units picked must not be actionable")

		after, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, after-before)
	})

	// 7. legacy fully picked -> count 0
	t.Run("legacy_fully_picked", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		before, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)

		orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
		_ = f.createOrderItem(t, ctx, orderID, fulfillmentID, 3, 3) // 0 allocs, 3/3 picked

		actionable, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.False(t, actionable, "legacy item with picked_quantity == quantity must not be actionable")

		after, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, after-before)
	})

	// 8. Last scan completion changes actionable count from 1 to 0 on authoritative reload
	t.Run("last_scan_decrements_actionable_count", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
		itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
		_, unitCode := f.createUnitWithStatus(t, ctx, itemID, "warehouse")

		// Before scan: must be actionable
		actionableBefore, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.True(t, actionableBefore)

		countBefore, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)

		// Perform last scan
		res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode, nil)
		require.NoError(t, err)
		assert.True(t, res.FulfillmentProgress.IsComplete)

		// After scan: on authoritative reload must NOT be actionable and count must decrement by 1
		actionableAfter, err := f.svc.IsFulfillmentActionablePicking(ctx, fulfillmentID)
		require.NoError(t, err)
		assert.False(t, actionableAfter, "fulfillment must not be actionable after last scan completes")

		countAfter, err := f.svc.CountActionablePicking(ctx)
		require.NoError(t, err)
		assert.Equal(t, -1, countAfter-countBefore, "count must decrement by 1 after last scan")
	})
}
