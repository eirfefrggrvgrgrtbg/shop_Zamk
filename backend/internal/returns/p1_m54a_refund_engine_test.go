package returns_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/go-chi/chi/v5"
)

// Helper to create a succeeded payment for an order
func createSucceededPayment(t *testing.T, fix *m51Fixture, orderID uuid.UUID, amountCents int64) uuid.UUID {
	t.Helper()
	testutil.AssertTestDatabase(t, fix.client.Pool)
	ctx := context.Background()
	payID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, 'tbank', $3, 'succeeded', $4, 'RUB', $5, now(), now())
	`, payID, orderID, "PAY-"+uuid.New().String()[:8], amountCents, "IDEM-"+uuid.New().String())
	require.NoError(t, err)
	return payID
}

// Helper to create serialized allocations and inventory units for an order item
func createSerializedAllocations(t *testing.T, fix *m51Fixture, orderID, orderItemID, variantID uuid.UUID, count int) ([]uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	supplyID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, supplyItemID, supplyID, variantID, count)
	require.NoError(t, err)

	var allocIDs []uuid.UUID
	var unitIDs []uuid.UUID

	for i := 1; i <= count; i++ {
		invUnitID := uuid.New()
		unitCode := fmt.Sprintf("ZMU-%s-%d", uuid.New().String()[:8], i)
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'shipped', now(), now())
		`, invUnitID, unitCode, variantID, supplyID, supplyItemID, i)
		require.NoError(t, err)
		unitIDs = append(unitIDs, invUnitID)

		allocID := uuid.New()
		nowTime := time.Now()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
			VALUES ($1, $2, $3, $4, now())
		`, allocID, orderItemID, invUnitID, nowTime)
		require.NoError(t, err)
		allocIDs = append(allocIDs, allocID)
	}

	return allocIDs, unitIDs
}

// ----------------------------------------------------------------------------
// 1. SERIALIZED REFUND TEST MATRIX
// ----------------------------------------------------------------------------

func TestM54A_SerializedRefundMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	t.Run("A_1restock_1damaged_1reject_yields_refundable_2", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		allocIDs, _ := createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, 3)
		createSucceededPayment(t, fix, tOrd.orderID, 3000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
			VALUES ($1, $2, $3, 3, 0, 0, 0, now())
		`, retItemID, retID, tOrd.orderItemID)
		require.NoError(t, err)

		// 1 restock, 1 damaged, 1 reject
		dispositions := []string{"restock", "damaged", "reject"}
		for i, allocID := range allocIDs {
			nowTime := time.Now()
			cond := "inspected"
			disp := dispositions[i]
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, now(), now())
			`, uuid.New(), retItemID, allocID, nowTime, cond, disp)
			require.NoError(t, err)
		}

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		require.NotNil(t, quote)
		assert.Equal(t, "serialized", quote.Items[0].Mode)
		assert.Equal(t, 3, quote.Items[0].RequestedQuantity)
		assert.Equal(t, 2, quote.Items[0].RefundableQuantity, "1 restock + 1 damaged = 2 refundable")
		assert.Equal(t, int64(2000), quote.Items[0].RefundCents)
		assert.Equal(t, int64(2000), quote.ProductsRefundCents)
		assert.Equal(t, int64(0), quote.DeliveryRefundCents)
		assert.Equal(t, int64(2000), quote.TotalRefundCents)
		assert.True(t, quote.CanRefund)
		assert.Nil(t, quote.BlockingReason)

		// Execute refund
		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		require.NotNil(t, ref)
		assert.Equal(t, int64(2000), ref.AmountCents)
		assert.Equal(t, "pending", ref.Status)
	})

	t.Run("B_1restock_1damaged_third_never_scanned_yields_refundable_2", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		allocIDs, _ := createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, 3)
		createSucceededPayment(t, fix, tOrd.orderID, 3000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
			VALUES ($1, $2, $3, 3, now())
		`, retItemID, retID, tOrd.orderItemID)
		require.NoError(t, err)

		// Scan only 2 units (1 restock, 1 damaged), third never scanned
		nowTime := time.Now()
		cond := "inspected"
		disp1 := "restock"
		disp2 := "damaged"
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		`, uuid.New(), retItemID, allocIDs[0], nowTime, cond, disp1)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		`, uuid.New(), retItemID, allocIDs[1], nowTime, cond, disp2)
		require.NoError(t, err)

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, 2, quote.Items[0].RefundableQuantity, "Unscanned unit must NOT be counted as refundable")
		assert.Equal(t, int64(2000), quote.TotalRefundCents)
		assert.True(t, quote.CanRefund)
	})

	t.Run("C_all_reject_yields_refundable_0_blocks_refund", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		allocIDs, _ := createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, 3)
		createSucceededPayment(t, fix, tOrd.orderID, 3000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
			VALUES ($1, $2, $3, 3, now())
		`, retItemID, retID, tOrd.orderItemID)
		require.NoError(t, err)

		// All 3 reject
		for _, allocID := range allocIDs {
			nowTime := time.Now()
			cond := "broken"
			disp := "reject"
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, now(), now())
			`, uuid.New(), retItemID, allocID, nowTime, cond, disp)
			require.NoError(t, err)
		}

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, 0, quote.Items[0].RefundableQuantity)
		assert.Equal(t, int64(0), quote.TotalRefundCents)
		assert.False(t, quote.CanRefund)
		require.NotNil(t, quote.BlockingReason)
		assert.Contains(t, *quote.BlockingReason, "Нет принятых товаров")

		// CreateRefund must fail
		_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		assert.ErrorIs(t, err, returns.ErrRefundNoEligibleItems)
	})

	t.Run("D_all_received_restock_damaged_restock_yields_refundable_3", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		allocIDs, _ := createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, 3)
		createSucceededPayment(t, fix, tOrd.orderID, 3000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
			VALUES ($1, $2, $3, 3, now())
		`, retItemID, retID, tOrd.orderItemID)
		require.NoError(t, err)

		disps := []string{"restock", "damaged", "restock"}
		for i, allocID := range allocIDs {
			nowTime := time.Now()
			disp := disps[i]
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 'ok', $5, now(), now())
			`, uuid.New(), retItemID, allocID, nowTime, disp)
			require.NoError(t, err)
		}

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, 3, quote.Items[0].RefundableQuantity)
		assert.Equal(t, int64(3000), quote.TotalRefundCents)
		assert.True(t, quote.CanRefund)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		assert.Equal(t, int64(3000), ref.AmountCents)
	})

	t.Run("E_serialized_dispositions_win_over_return_items_aggregate_fields", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		allocIDs, _ := createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, 3)
		createSucceededPayment(t, fix, tOrd.orderID, 3000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		// Intentionally set return_items aggregate values to 0 accepted, 0 damaged, 3 rejected
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
			VALUES ($1, $2, $3, 3, 0, 0, 3, now())
		`, retItemID, retID, tOrd.orderItemID)
		require.NoError(t, err)

		// BUT return_item_units has 2 restock + 1 damaged
		disps := []string{"restock", "restock", "damaged"}
		for i, allocID := range allocIDs {
			nowTime := time.Now()
			disp := disps[i]
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 'ok', $5, now(), now())
			`, uuid.New(), retItemID, allocID, nowTime, disp)
			require.NoError(t, err)
		}

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, "serialized", quote.Items[0].Mode)
		assert.Equal(t, 3, quote.Items[0].RefundableQuantity, "Serialized dispositions MUST win over return_items aggregate fields")
		assert.Equal(t, int64(3000), quote.TotalRefundCents)
		assert.True(t, quote.CanRefund)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		assert.Equal(t, int64(3000), ref.AmountCents)
	})
}

// ----------------------------------------------------------------------------
// 2. LEGACY REFUND TEST MATRIX
// ----------------------------------------------------------------------------

func TestM54A_LegacyRefundMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	t.Run("legacy_accepted2_damaged1_rejected1_requested5_yields_refundable3", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
		// NO serialized allocations created -> purely legacy
		createSucceededPayment(t, fix, tOrd.orderID, 5000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
			VALUES ($1, $2, $3, 5, 2, 1, 1, now())
		`, retItemID, retID, tOrd.orderItemID)
		require.NoError(t, err)

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, "legacy", quote.Items[0].Mode)
		assert.Equal(t, 5, quote.Items[0].RequestedQuantity)
		assert.Equal(t, 3, quote.Items[0].RefundableQuantity, "Legacy accepted (2) + damaged (1) = 3")
		assert.Equal(t, int64(3000), quote.TotalRefundCents)
		assert.True(t, quote.CanRefund)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		assert.Equal(t, int64(3000), ref.AmountCents)
	})
}

// ----------------------------------------------------------------------------
// 3. STATUS GATE MATRIX
// ----------------------------------------------------------------------------

func TestM54A_StatusGateMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	statuses := []struct {
		status         string
		canRefund      bool
		expectedErr    error
		isIdempotentOk bool
	}{
		{status: "requested", canRefund: false, expectedErr: returns.ErrReturnNotReceived},
		{status: "needs_info", canRefund: false, expectedErr: returns.ErrReturnNotReceived},
		{status: "approved", canRefund: false, expectedErr: returns.ErrReturnNotReceived},
		{status: "receiving", canRefund: false, expectedErr: returns.ErrReturnNotReceived},
		{status: "rejected", canRefund: false, expectedErr: returns.ErrReturnRejected},
		{status: "cancelled", canRefund: false, expectedErr: returns.ErrReturnRejected},
		{status: "completed", canRefund: false, expectedErr: returns.ErrReturnAlreadyRefunded},
		{status: "item_received", canRefund: true, expectedErr: nil},
	}

	for _, tc := range statuses {
		t.Run("status_"+tc.status, func(t *testing.T) {
			tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
			createSucceededPayment(t, fix, tOrd.orderID, 1000)

			retID := uuid.New()
			_, err := fix.client.Pool.Exec(ctx, `
				INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 'defective', now(), now())
			`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID, tc.status)
			require.NoError(t, err)

			retItemID := uuid.New()
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
				VALUES ($1, $2, $3, 1, 1, now())
			`, retItemID, retID, tOrd.orderItemID)
			require.NoError(t, err)

			quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
			require.NoError(t, err)
			assert.Equal(t, tc.canRefund, quote.CanRefund, "CanRefund mismatch for status %s", tc.status)
			if !tc.canRefund {
				assert.NotNil(t, quote.BlockingReason)
			}

			ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr, "Expected error for status %s", tc.status)
				assert.Nil(t, ref)
			} else {
				require.NoError(t, err)
				require.NotNil(t, ref)
				assert.Equal(t, int64(1000), ref.AmountCents)
			}
		})
	}

	t.Run("status_refunded_is_idempotent", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		payID := createSucceededPayment(t, fix, tOrd.orderID, 1000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'refunded', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		existingRefundID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'pending', 1000, 'RUB', now(), now())
		`, existingRefundID, retID, payID, tOrd.orderID)
		require.NoError(t, err)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		require.NotNil(t, ref)
		assert.Equal(t, existingRefundID, ref.ID, "Repeated refund call must idempotently return the existing refund")
	})
}

// ----------------------------------------------------------------------------
// 4. PAYMENT LINK & AMBIGUITY MATRIX
// ----------------------------------------------------------------------------

func TestM54A_PaymentAmbiguityMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	t.Run("0_succeeded_payments_blocks_refund", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		// No payments inserted

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.False(t, quote.CanRefund)
		require.NotNil(t, quote.BlockingReason)
		assert.Contains(t, *quote.BlockingReason, "найдена")

		_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		assert.ErrorIs(t, err, payments.ErrPaymentNotFound)
	})

	t.Run("1_succeeded_payment_allows_refund", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPayment(t, fix, tOrd.orderID, 1000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.True(t, quote.CanRefund)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		assert.Equal(t, int64(1000), ref.AmountCents)
	})

	t.Run("2_succeeded_payments_fails_closed_with_ambiguous_funding", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		// Insert TWO succeeded payments for the same order
		createSucceededPayment(t, fix, tOrd.orderID, 500)
		createSucceededPayment(t, fix, tOrd.orderID, 500)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.False(t, quote.CanRefund)
		require.NotNil(t, quote.BlockingReason)
		assert.Contains(t, *quote.BlockingReason, "Неоднознач")

		_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		assert.ErrorIs(t, err, payments.ErrAmbiguousFundingPayment)
	})
}

// ----------------------------------------------------------------------------
// 5. CONCURRENCY & IDEMPOTENCY
// ----------------------------------------------------------------------------

func TestM54A_ConcurrencyAndIdempotency(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)

	retID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 1, 1, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	// Launch two concurrent refund attempts
	var wg sync.WaitGroup
	results := make([]*returns.Refund, 2)
	errs := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			results[idx], errs[idx] = fix.svc.CreateRefund(context.Background(), fix.userID, retID, returns.CreateRefundRequest{})
		}()
	}
	wg.Wait()

	// Both callers should get valid results without errors or deadlock
	for i := 0; i < 2; i++ {
		require.NoError(t, errs[i])
		require.NotNil(t, results[i])
		assert.Equal(t, int64(1000), results[i].AmountCents)
	}

	// Verify DB state: EXACTLY ONE refund record created in DB
	var refundCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&refundCount)
	require.NoError(t, err)
	assert.Equal(t, 1, refundCount, "Exactly one refund row must exist in DB")

	// Verify Return status is item_received while refund is pending
	var status string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "item_received", status)
}

// ----------------------------------------------------------------------------
// 6. SELLER FINANCE ATOMICITY
// ----------------------------------------------------------------------------

func TestM54A_SellerFinanceAtomicity(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	t.Run("serialized_reject_unreceived_not_deducted_from_seller_finance", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		allocIDs, _ := createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, 3)
		createSucceededPayment(t, fix, tOrd.orderID, 3000)

		// Create seller earning entry of 3000 cents for this order item
		earningID := uuid.New()
		nowTime := time.Now()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'seller_earning', 3000, 'RUB', $5, '{}', now())
		`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID, nowTime)
		require.NoError(t, err)

		retID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
			VALUES ($1, $2, $3, 3, now())
		`, retItemID, retID, tOrd.orderItemID)
		require.NoError(t, err)

		// 1 restock, 1 damaged, 1 reject
		disps := []string{"restock", "damaged", "reject"}
		for i, allocID := range allocIDs {
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
				VALUES ($1, $2, $3, now(), 'inspected', $4, now(), now())
			`, uuid.New(), retItemID, allocID, disps[i])
			require.NoError(t, err)
		}

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		assert.Equal(t, int64(2000), ref.AmountCents)

		// Process refund success to trigger seller deduction
		err = fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now())
		require.NoError(t, err)

		// Check seller finance deduction amount in seller_ledger_entries: must be exactly -2000 (for 2 refundable units), NOT -3000
		var adjustmentAmount int64
		err = fix.client.Pool.QueryRow(ctx, `
			SELECT amount_cents FROM seller_ledger_entries
			WHERE order_id = $1 AND type = 'adjustment'
		`, tOrd.orderID).Scan(&adjustmentAmount)
		require.NoError(t, err)
		assert.Equal(t, int64(-2000), adjustmentAmount, "Seller deduction must be strictly proportional to actual refundable quantity")
	})

	t.Run("failed_payment_reservation_leaves_seller_finance_unchanged", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		// Succeeded payment is for only 100 cents (less than refundable 1000 cents -> will trigger ErrRefundExceedsPaid)
		createSucceededPayment(t, fix, tOrd.orderID, 100)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		assert.ErrorIs(t, err, payments.ErrRefundExceedsPaid)

		// Verify Return status is still item_received (rollback worked!)
		var retStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", retStatus)

		// Verify 0 refunds created
		var refundCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&refundCount)
		require.NoError(t, err)
		assert.Equal(t, 0, refundCount)

		// Verify 0 seller ledger adjustments created
		var adjCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCount)
		require.NoError(t, err)
		assert.Equal(t, 0, adjCount)
	})
}

// ----------------------------------------------------------------------------
// 7. NO PHYSICAL SIDE EFFECTS
// ----------------------------------------------------------------------------

func TestM54A_NoPhysicalSideEffects(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	allocIDs, unitIDs := createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, 2)
	createSucceededPayment(t, fix, tOrd.orderID, 2000)

	// Insert inventory stock record
	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 50, 5)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	retItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 2, 2, now())
	`, retItemID, retID, tOrd.orderItemID)
	require.NoError(t, err)

	for _, allocID := range allocIDs {
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
			VALUES ($1, $2, $3, now(), 'inspected', 'restock', now(), now())
		`, uuid.New(), retItemID, allocID)
		require.NoError(t, err)
	}

	// 1. Run Quote
	_, err = fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)

	// 2. Run CreateRefund
	_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	// Assert stock values are unchanged
	var totalStock, reservedStock int
	err = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 50, totalStock, "total_stock must NOT be changed by refund")
	assert.Equal(t, 5, reservedStock, "reserved_stock must NOT be changed by refund")

	// Assert unit statuses are unchanged
	for _, uID := range unitIDs {
		var uStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", uStatus, "inventory_units status must NOT be changed by refund")
	}
}

// ----------------------------------------------------------------------------
// 8. HISTORICAL PRICE BASIS
// ----------------------------------------------------------------------------

func TestM54A_HistoricalPriceBasis(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1) // OrderItem price = 1000
	createSucceededPayment(t, fix, tOrd.orderID, 1000)

	// Drastically change current product and variant price in catalog to 99999
	_, err := fix.client.Pool.Exec(ctx, "UPDATE products SET price_cents = 99999 WHERE id = $1", fix.prodAID)
	require.NoError(t, err)
	_, err = fix.client.Pool.Exec(ctx, "UPDATE product_variants SET price_cents = 99999 WHERE id = $1", fix.varAID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 1, 1, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), quote.Items[0].UnitPriceCents, "Quote MUST use historical OrderItem purchase price")
	assert.Equal(t, int64(1000), quote.TotalRefundCents)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(1000), ref.AmountCents, "Created refund MUST use historical OrderItem purchase price")
}

// ----------------------------------------------------------------------------
// 9. DELIVERY POLICY ZERO
// ----------------------------------------------------------------------------

func TestM54A_DeliveryPolicyZero(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	// Set order delivery_price_cents to 5000
	_, err := fix.client.Pool.Exec(ctx, "UPDATE orders SET delivery_price_cents = 5000 WHERE id = $1", tOrd.orderID)
	require.NoError(t, err)
	createSucceededPayment(t, fix, tOrd.orderID, 6000)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 1, 1, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), quote.DeliveryRefundCents, "Delivery refund policy for M5.4A is strictly 0")
	assert.Equal(t, int64(1000), quote.ProductsRefundCents)
	assert.Equal(t, int64(1000), quote.TotalRefundCents)
}

// ----------------------------------------------------------------------------
// 10. RBAC HTTP INTEGRATION TESTS
// ----------------------------------------------------------------------------

func TestM54A_RBAC_HTTP_Proof(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	handler := returns.NewHandler(fix.svc)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)

	retID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 1, 1, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	adminUserID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash) VALUES ($1, 'Admin', $2, 'hash')", adminUserID, "admin_"+uuid.New().String()+"@test.com")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = fix.client.Pool.Exec(ctx, "DELETE FROM refunds WHERE return_id = $1", retID)
		_, _ = fix.client.Pool.Exec(ctx, "DELETE FROM return_items WHERE return_id = $1", retID)
		_, _ = fix.client.Pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", retID)
		_, _ = fix.client.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", adminUserID)
	})

	t.Run("quote_endpoint_returns_read_authorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+retID.String()+"/refund-quote", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", retID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = req.WithContext(context.WithValue(req.Context(), "userID", adminUserID))

		rec := httptest.NewRecorder()
		handler.GetAdminRefundQuote(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var quote returns.ReturnRefundQuote
		err := json.NewDecoder(rec.Body).Decode(&quote)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), quote.TotalRefundCents)
		assert.True(t, quote.CanRefund)
	})

	t.Run("quote_endpoint_unauthorized_when_no_user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+retID.String()+"/refund-quote", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", retID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		handler.GetAdminRefundQuote(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("refund_execution_endpoint_authorized", func(t *testing.T) {
		reqBody := `{"reason":"test refund"}`
		req := httptest.NewRequest("POST", "/api/admin/returns/"+retID.String()+"/refund", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", retID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = req.WithContext(context.WithValue(req.Context(), "userID", adminUserID))

		rec := httptest.NewRecorder()
		handler.CreateAdminRefund(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var ref returns.Refund
		err := json.NewDecoder(rec.Body).Decode(&ref)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), ref.AmountCents)
	})
}

// ----------------------------------------------------------------------------
// 11. ALLOCATION INVARIANT MATRIX (Q vs A)
// ----------------------------------------------------------------------------

func TestM54A_AllocationInvariantMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	cases := []struct {
		name         string
		orderQty     int
		allocCount   int
		expectMode   string
		expectBlocked bool
	}{
		{name: "Q3_A1_partial_allocation_blocked", orderQty: 3, allocCount: 1, expectBlocked: true},
		{name: "Q3_A2_partial_allocation_blocked", orderQty: 3, allocCount: 2, expectBlocked: true},
		{name: "Q3_A4_excess_allocation_blocked", orderQty: 3, allocCount: 4, expectBlocked: true},
		{name: "Q3_A3_exact_allocation_serialized", orderQty: 3, allocCount: 3, expectMode: "serialized", expectBlocked: false},
		{name: "Q3_A0_zero_allocation_legacy", orderQty: 3, allocCount: 0, expectMode: "legacy", expectBlocked: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), tc.orderQty)
			var allocIDs []uuid.UUID
			if tc.allocCount > 0 {
				allocIDs, _ = createSerializedAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, tc.allocCount)
			}
			createSucceededPayment(t, fix, tOrd.orderID, int64(tc.orderQty*1000))

			retID := uuid.New()
			_, err := fix.client.Pool.Exec(ctx, `
				INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
			`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
			require.NoError(t, err)

			retItemID := uuid.New()
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
				VALUES ($1, $2, $3, $4, 1, 0, 0, now())
			`, retItemID, retID, tOrd.orderItemID, tc.orderQty)
			require.NoError(t, err)

			if len(allocIDs) > 0 {
				for _, allocID := range allocIDs {
					_, err = fix.client.Pool.Exec(ctx, `
						INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at)
						VALUES ($1, $2, $3, now(), 'inspected', 'restock', now(), now())
					`, uuid.New(), retItemID, allocID)
					require.NoError(t, err)
				}
			}

			quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
			require.NoError(t, err)
			require.NotNil(t, quote)

			if tc.expectBlocked {
				assert.False(t, quote.CanRefund, "Partial or excess allocation must be blocked")
				require.NotNil(t, quote.BlockingReason)
				assert.Contains(t, *quote.BlockingReason, "Несогласованное состояние резервирования")

				// Execution must reject with ErrRefundAllocationInvariant
				_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
				assert.ErrorIs(t, err, returns.ErrRefundAllocationInvariant)
			} else {
				assert.True(t, quote.CanRefund)
				assert.Nil(t, quote.BlockingReason)
				assert.Equal(t, tc.expectMode, quote.Items[0].Mode)

				ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
				require.NoError(t, err)
				require.NotNil(t, ref)
				assert.Equal(t, "pending", ref.Status)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// 12. REFUND LIFECYCLE SEMANTICS (Pending Reservation -> Succeeded / Failed)
// ----------------------------------------------------------------------------

func TestM54A_RefundLifecycleSemantics(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	t.Run("payment_reservation_failure_leaves_return_item_received", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		// Payment is for 100 cents, refund requires 1000 -> ErrRefundExceedsPaid
		createSucceededPayment(t, fix, tOrd.orderID, 100)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		assert.ErrorIs(t, err, payments.ErrRefundExceedsPaid)

		var retStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", retStatus, "Return must remain item_received when reservation fails")

		var refundCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&refundCount)
		require.NoError(t, err)
		assert.Equal(t, 0, refundCount)
	})

	t.Run("pending_refund_leaves_return_item_received_and_blocks_quote", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPayment(t, fix, tOrd.orderID, 1000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		require.NotNil(t, ref)
		assert.Equal(t, "pending", ref.Status)

		// 1. Assert Return status in DB remains 'item_received' (NOT falsely 'refunded')
		var retStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", retStatus, "Return must remain item_received while refund is pending")

		// 2. Assert Quote blocks subsequent refund attempts with human-friendly message
		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.False(t, quote.CanRefund)
		require.NotNil(t, quote.BlockingReason)
		assert.Contains(t, *quote.BlockingReason, "уже зарезервирован")

		// 3. Assert Timeline has ZERO return.refunded events (since processed_at is null)
		tl, err := fix.svc.GetAdminTimeline(ctx, retID)
		require.NoError(t, err)
		for _, ev := range tl.Events {
			assert.NotEqual(t, "return.refunded", ev.Type, "return.refunded must NOT be emitted while refund is pending")
		}
	})

	t.Run("successful_refund_transitions_return_and_emits_timeline_and_debits_seller", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPayment(t, fix, tOrd.orderID, 1000)

		// Create seller earning entry
		earningID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'seller_earning', 1000, 'RUB', now(), '{}', now())
		`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
		require.NoError(t, err)

		retID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)

		// Initially 0 seller deductions
		var adjCountBefore int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCountBefore)
		require.NoError(t, err)
		assert.Equal(t, 0, adjCountBefore)

		// Durably process refund as succeeded
		processedTime := time.Now().Add(2 * time.Minute).Truncate(time.Microsecond)
		err = fix.svc.ProcessRefundSuccess(ctx, ref.ID, processedTime)
		require.NoError(t, err)

		// 1. Assert refund row has status=succeeded and processed_at populated
		var dbStatus string
		var dbProcessedAt *time.Time
		err = fix.client.Pool.QueryRow(ctx, "SELECT status, processed_at FROM refunds WHERE id = $1", ref.ID).Scan(&dbStatus, &dbProcessedAt)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", dbStatus)
		require.NotNil(t, dbProcessedAt)
		assert.Equal(t, processedTime.Unix(), dbProcessedAt.Unix())

		// 2. Assert Return status is now 'refunded'
		var retStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "refunded", retStatus)

		// 3. Assert seller finance deduction of -1000 was created
		var adjAmount int64
		err = fix.client.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjAmount)
		require.NoError(t, err)
		assert.Equal(t, int64(-1000), adjAmount)

		// 4. Assert Timeline emits return.refunded event with exact occurredAt == processedAt
		tl, err := fix.svc.GetAdminTimeline(ctx, retID)
		require.NoError(t, err)
		var foundRefundEvent bool
		for _, ev := range tl.Events {
			if ev.Type == "return.refunded" {
				foundRefundEvent = true
				assert.Equal(t, processedTime.Unix(), ev.OccurredAt.Unix())
			}
		}
		assert.True(t, foundRefundEvent, "Universal timeline MUST emit return.refunded once refund is succeeded with processed_at")
	})

	t.Run("failed_refund_keeps_return_item_received_and_no_seller_debit", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPayment(t, fix, tOrd.orderID, 1000)

		retID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)

		// Process refund failure
		err = fix.svc.ProcessRefundFailure(ctx, ref.ID)
		require.NoError(t, err)

		// 1. Assert refund row status is failed
		var dbStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM refunds WHERE id = $1", ref.ID).Scan(&dbStatus)
		require.NoError(t, err)
		assert.Equal(t, "failed", dbStatus)

		// 2. Assert Return remains item_received
		var retStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", retStatus)

		// 3. Assert 0 seller deductions created
		var adjCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCount)
		require.NoError(t, err)
		assert.Equal(t, 0, adjCount)

		// 4. Assert Timeline does NOT emit return.refunded
		tl, err := fix.svc.GetAdminTimeline(ctx, retID)
		require.NoError(t, err)
		for _, ev := range tl.Events {
			assert.NotEqual(t, "return.refunded", ev.Type)
		}
	})
}

// ----------------------------------------------------------------------------
// 13. COMPLETION IDEMPOTENCY & PROCESSED_AT IMMUTABILITY
// ----------------------------------------------------------------------------

func TestM54A_SuccessIdempotencyAndImmutability(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)

	earningID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 1000, 'RUB', now(), '{}', now())
	`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 1, 1, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	t1 := time.Now().Add(1 * time.Minute).Truncate(time.Microsecond)
	err = fix.svc.ProcessRefundSuccess(ctx, ref.ID, t1)
	require.NoError(t, err)

	// Second success callback with different timestamp t2
	t2 := time.Now().Add(5 * time.Minute).Truncate(time.Microsecond)
	err = fix.svc.ProcessRefundSuccess(ctx, ref.ID, t2)
	require.NoError(t, err, "Duplicate success callback must be safe and idempotent")

	// 1. Assert exactly 1 refund row in DB
	var refCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&refCount)
	require.NoError(t, err)
	assert.Equal(t, 1, refCount, "Duplicate callback must NOT insert another refund row")

	// 2. Assert processed_at was NOT overwritten (remains t1)
	var dbStatus string
	var dbProcessedAt *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT status, processed_at FROM refunds WHERE id = $1", ref.ID).Scan(&dbStatus, &dbProcessedAt)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", dbStatus)
	require.NotNil(t, dbProcessedAt)
	assert.Equal(t, t1.Unix(), dbProcessedAt.Unix(), "processed_at must be immutable and NOT replaced by duplicate callback")

	// 3. Assert exactly ONE seller ledger deduction (no double debit)
	var adjCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCount)
	require.NoError(t, err)
	assert.Equal(t, 1, adjCount, "Must have exactly ONE seller deduction ledger entry")

	var adjAmount int64
	err = fix.client.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjAmount)
	require.NoError(t, err)
	assert.Equal(t, int64(-1000), adjAmount)
}

// ----------------------------------------------------------------------------
// 14. CONCURRENT SUCCESS CALLBACKS (NO DOUBLE DEBIT)
// ----------------------------------------------------------------------------

func TestM54A_ConcurrentSuccessCallbacks(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)

	earningID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 1000, 'RUB', now(), '{}', now())
	`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 1, 1, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	processedTime := time.Now().Truncate(time.Microsecond)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			errs[idx] = fix.svc.ProcessRefundSuccess(context.Background(), ref.ID, processedTime)
		}()
	}
	wg.Wait()

	for i := 0; i < 2; i++ {
		require.NoError(t, errs[i], "Concurrent success callback must not error or deadlock")
	}

	// 1. Assert exactly 1 refund row in DB, status = succeeded
	var refCount int
	var status string
	var dbProcessedAt *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*), MAX(status), MAX(processed_at) FROM refunds WHERE id = $1", ref.ID).Scan(&refCount, &status, &dbProcessedAt)
	require.NoError(t, err)
	assert.Equal(t, 1, refCount)
	assert.Equal(t, "succeeded", status)
	require.NotNil(t, dbProcessedAt)

	// 2. Assert Return.status = refunded
	var retStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
	require.NoError(t, err)
	assert.Equal(t, "refunded", retStatus)

	// 3. Assert exactly ONE seller deduction ledger entry
	var adjCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCount)
	require.NoError(t, err)
	assert.Equal(t, 1, adjCount, "Must have exactly ONE seller deduction under concurrent callbacks")

	var adjAmount int64
	err = fix.client.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjAmount)
	require.NoError(t, err)
	assert.Equal(t, int64(-1000), adjAmount)
}

// ----------------------------------------------------------------------------
// 15. SUCCESS VS FAILURE RACE & TERMINAL STATE PROTECTION
// ----------------------------------------------------------------------------

func TestM54A_SuccessVsFailureRace(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	t.Run("CaseA_success_first_then_failure_callback_cannot_downgrade", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPayment(t, fix, tOrd.orderID, 1000)

		earningID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'seller_earning', 1000, 'RUB', now(), '{}', now())
		`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
		require.NoError(t, err)

		retID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)

		tSuccess := time.Now().Add(1 * time.Minute).Truncate(time.Microsecond)
		err = fix.svc.ProcessRefundSuccess(ctx, ref.ID, tSuccess)
		require.NoError(t, err)

		// Late failure callback arrives
		err = fix.svc.ProcessRefundFailure(ctx, ref.ID)
		require.NoError(t, err)

		// Assert refund remains succeeded and processed_at intact
		var dbStatus string
		var dbProcessedAt *time.Time
		err = fix.client.Pool.QueryRow(ctx, "SELECT status, processed_at FROM refunds WHERE id = $1", ref.ID).Scan(&dbStatus, &dbProcessedAt)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", dbStatus)
		require.NotNil(t, dbProcessedAt)
		assert.Equal(t, tSuccess.Unix(), dbProcessedAt.Unix())

		// Assert Return remains refunded
		var retStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "refunded", retStatus)

		// Assert exactly 1 seller deduction remains
		var adjCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCount)
		require.NoError(t, err)
		assert.Equal(t, 1, adjCount)
	})

	t.Run("CaseB_failure_first_then_success_callback_cannot_resurrect", func(t *testing.T) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPayment(t, fix, tOrd.orderID, 1000)

		earningID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'seller_earning', 1000, 'RUB', now(), '{}', now())
		`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
		require.NoError(t, err)

		retID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, now())
		`, uuid.New(), retID, tOrd.orderItemID)
		require.NoError(t, err)

		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)

		// Failure commits first
		err = fix.svc.ProcessRefundFailure(ctx, ref.ID)
		require.NoError(t, err)

		// Late success callback arrives for the same failed refund
		err = fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now())
		require.NoError(t, err)

		// Assert refund remains failed with processed_at = NULL
		var dbStatus string
		var dbProcessedAt *time.Time
		err = fix.client.Pool.QueryRow(ctx, "SELECT status, processed_at FROM refunds WHERE id = $1", ref.ID).Scan(&dbStatus, &dbProcessedAt)
		require.NoError(t, err)
		assert.Equal(t, "failed", dbStatus)
		assert.Nil(t, dbProcessedAt, "Failed refund must never have processed_at set")

		// Assert Return remains item_received
		var retStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", retStatus)

		// Assert ZERO seller deductions
		var adjCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCount)
		require.NoError(t, err)
		assert.Equal(t, 0, adjCount)
	})
}

// ----------------------------------------------------------------------------
// 16. FAILURE IDEMPOTENCY
// ----------------------------------------------------------------------------

func TestM54A_FailureIdempotency(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)

	retID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 1, 1, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	// First failure callback
	err = fix.svc.ProcessRefundFailure(ctx, ref.ID)
	require.NoError(t, err)

	// Second failure callback
	err = fix.svc.ProcessRefundFailure(ctx, ref.ID)
	require.NoError(t, err, "Duplicate failure callback must be safe and idempotent")

	var refCount int
	var dbStatus string
	var dbProcessedAt *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*), MAX(status), MAX(processed_at) FROM refunds WHERE id = $1", ref.ID).Scan(&refCount, &dbStatus, &dbProcessedAt)
	require.NoError(t, err)
	assert.Equal(t, 1, refCount)
	assert.Equal(t, "failed", dbStatus)
	assert.Nil(t, dbProcessedAt)

	var retStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
	require.NoError(t, err)
	assert.Equal(t, "item_received", retStatus)
}

// ----------------------------------------------------------------------------
// 17. RETRY AFTER FAILED REFUND (FRESH ROW & SINGLE SELLER DEBIT)
// ----------------------------------------------------------------------------

func TestM54A_RetryAfterFailedRefund(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	createSucceededPayment(t, fix, tOrd.orderID, 3000)

	earningID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 3000, 'RUB', now(), '{}', now())
	`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 3, 3, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	// Step 1: Initial refund attempt #1
	ref1, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.Equal(t, "pending", ref1.Status)

	// Step 2: Refund #1 fails
	err = fix.svc.ProcessRefundFailure(ctx, ref1.ID)
	require.NoError(t, err)

	// Step 3: Verify Quote is eligible again after failure
	quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.True(t, quote.CanRefund, "Quote MUST become eligible again after refund failure")
	assert.Nil(t, quote.BlockingReason)
	assert.Equal(t, int64(3000), quote.TotalRefundCents)

	// Step 4: Retry CreateRefund creates a NEW refund row #2
	ref2, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.NotEqual(t, ref1.ID, ref2.ID, "Retry MUST create a NEW refund row rather than mutating the failed row")
	assert.Equal(t, "pending", ref2.Status)

	// Verify DB has exactly 2 refund rows for this return (1 failed, 1 pending)
	var countFailed, countPending int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FILTER (WHERE status = 'failed'), COUNT(*) FILTER (WHERE status = 'pending') FROM refunds WHERE return_id = $1", retID).Scan(&countFailed, &countPending)
	require.NoError(t, err)
	assert.Equal(t, 1, countFailed)
	assert.Equal(t, 1, countPending)

	// Return remains item_received, seller deduction = 0
	var retStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
	require.NoError(t, err)
	assert.Equal(t, "item_received", retStatus)

	var adjCountBefore int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCountBefore)
	require.NoError(t, err)
	assert.Equal(t, 0, adjCountBefore)

	// Step 5: Refund #2 succeeds
	processedTime := time.Now().Add(3 * time.Minute).Truncate(time.Microsecond)
	err = fix.svc.ProcessRefundSuccess(ctx, ref2.ID, processedTime)
	require.NoError(t, err)

	// Assert final DB state: refund #2 succeeded, refund #1 failed, Return refunded, exactly ONE seller deduction
	var ref2Status string
	var ref2ProcessedAt *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT status, processed_at FROM refunds WHERE id = $1", ref2.ID).Scan(&ref2Status, &ref2ProcessedAt)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", ref2Status)
	require.NotNil(t, ref2ProcessedAt)
	assert.Equal(t, processedTime.Unix(), ref2ProcessedAt.Unix())

	var ref1Status string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM refunds WHERE id = $1", ref1.ID).Scan(&ref1Status)
	require.NoError(t, err)
	assert.Equal(t, "failed", ref1Status)

	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
	require.NoError(t, err)
	assert.Equal(t, "refunded", retStatus)

	var adjCountAfter int
	var adjAmount int64
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*), COALESCE(SUM(amount_cents), 0) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'adjustment'", tOrd.orderID).Scan(&adjCountAfter, &adjAmount)
	require.NoError(t, err)
	assert.Equal(t, 1, adjCountAfter, "Failed retry then succeeded retry must result in exactly ONE effective seller deduction")
	assert.Equal(t, int64(-3000), adjAmount)
}

// ----------------------------------------------------------------------------
// 18. PAYMENT CAPACITY RESTORATION AFTER FAILURE
// ----------------------------------------------------------------------------

func TestM54A_PaymentCapacityAfterFailure(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	payRepo := payments.NewRepository(fix.client.Pool)
	paySvc := payments.NewService(payRepo, fix.ordersRepo, nil, nil, fix.client, nil, nil)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 10) // OrderItem price = 1000 -> total 10000
	payID := createSucceededPayment(t, fix, tOrd.orderID, 10000)

	// 1. Initial refund attempt of 5000 fails
	err := paySvc.ReserveRefund(ctx, payID, 5000, "failed attempt", nil)
	require.NoError(t, err)

	// Mark it failed in DB
	_, err = fix.client.Pool.Exec(ctx, "UPDATE refunds SET status = 'failed' WHERE payment_id = $1 AND amount_cents = 5000", payID)
	require.NoError(t, err)

	// 2. Failed refund 5000 must NOT consume capacity: reserving 5000 again is allowed
	err = paySvc.ReserveRefund(ctx, payID, 5000, "retry attempt", nil)
	require.NoError(t, err, "Failed refund must not permanently consume payment capacity")

	// 3. Now 5000 is pending. Reserving 6000 (5000 pending + 6000 = 11000 > 10000) must be blocked
	err = paySvc.ReserveRefund(ctx, payID, 6000, "excess attempt", nil)
	assert.ErrorIs(t, err, payments.ErrRefundExceedsPaid)

	// 4. Mark the pending 5000 refund as succeeded
	_, err = fix.client.Pool.Exec(ctx, "UPDATE refunds SET status = 'succeeded', processed_at = now() WHERE payment_id = $1 AND status = 'pending'", payID)
	require.NoError(t, err)

	// 5. Now 5000 is succeeded. Reserving 6000 (5000 succeeded + 6000 = 11000 > 10000) must still be blocked
	err = paySvc.ReserveRefund(ctx, payID, 6000, "excess attempt 2", nil)
	assert.ErrorIs(t, err, payments.ErrRefundExceedsPaid)

	// 6. Reserving 5000 (remaining available 5000) is allowed
	err = paySvc.ReserveRefund(ctx, payID, 5000, "final exact attempt", nil)
	require.NoError(t, err)
}

// ----------------------------------------------------------------------------
// 19. M5.4B QUOTE CONTRACT & LATEST REFUND STATUS
// ----------------------------------------------------------------------------

func TestM54B_QuoteContractLatestRefundStatus(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-2*time.Hour), 3) // price = 1000 -> total 3000
	createSucceededPayment(t, fix, tOrd.orderID, 3000)

	earningID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 3000, 'RUB', now(), '{}', now())
	`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 3, 3, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	// 1. Initial state (no refunds): latestRefundStatus must be nil, canRefund = true
	quote1, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Nil(t, quote1.LatestRefundStatus, "Initial quote must have nil latestRefundStatus")
	assert.Nil(t, quote1.LatestRefundProcessedAt)
	assert.True(t, quote1.CanRefund)
	assert.Equal(t, int64(3000), quote1.TotalRefundCents)
	assert.Equal(t, int64(0), quote1.SucceededRefundedCents)
	assert.Equal(t, int64(0), quote1.PendingRefundCents)
	assert.Equal(t, int64(0), quote1.AlreadyRefundedCents)
	assert.Equal(t, int64(3000), quote1.RemainingRefundableCents)

	// 2. Create pending refund: latestRefundStatus must be "pending", canRefund = false
	ref1, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.Equal(t, "pending", ref1.Status)

	quote2, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, quote2.LatestRefundStatus)
	assert.Equal(t, "pending", *quote2.LatestRefundStatus)
	assert.False(t, quote2.CanRefund)
	assert.Equal(t, int64(3000), quote2.TotalRefundCents)
	assert.Equal(t, int64(0), quote2.SucceededRefundedCents, "Pending refund must NOT count as succeeded")
	assert.Equal(t, int64(3000), quote2.PendingRefundCents, "Pending refund must be reflected in PendingRefundCents")
	assert.Equal(t, int64(0), quote2.AlreadyRefundedCents, "AlreadyRefundedCents must be 0 when only pending")
	assert.Equal(t, int64(0), quote2.RemainingRefundableCents, "RemainingRefundableCents must be 0 while fully reserved in pending")

	// 3. Mark refund as failed: latestRefundStatus must be "failed", canRefund = true (allows retry)
	err = fix.svc.ProcessRefundFailure(ctx, ref1.ID)
	require.NoError(t, err)

	quote3, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, quote3.LatestRefundStatus)
	assert.Equal(t, "failed", *quote3.LatestRefundStatus)
	assert.True(t, quote3.CanRefund, "Failed refund must allow retry when conditions permit")
	assert.Equal(t, int64(3000), quote3.TotalRefundCents)
	assert.Equal(t, int64(0), quote3.SucceededRefundedCents)
	assert.Equal(t, int64(0), quote3.PendingRefundCents, "Failed refund must contribute 0 to pending")
	assert.Equal(t, int64(0), quote3.AlreadyRefundedCents)
	assert.Equal(t, int64(3000), quote3.RemainingRefundableCents, "Failed refund must restore remaining refundable capacity")

	// 4. Create retry refund (now pending): latestRefundStatus must be "pending", canRefund = false
	ref2, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.NotEqual(t, ref1.ID, ref2.ID)
	assert.Equal(t, "pending", ref2.Status)

	quote4, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, quote4.LatestRefundStatus)
	assert.Equal(t, "pending", *quote4.LatestRefundStatus)
	assert.False(t, quote4.CanRefund)
	assert.Equal(t, int64(3000), quote4.TotalRefundCents)
	assert.Equal(t, int64(0), quote4.SucceededRefundedCents)
	assert.Equal(t, int64(3000), quote4.PendingRefundCents)
	assert.Equal(t, int64(0), quote4.AlreadyRefundedCents)
	assert.Equal(t, int64(0), quote4.RemainingRefundableCents)

	// 5. Refund #2 succeeds: latestRefundStatus must be "succeeded", canRefund = false, processedAt populated
	procTime := time.Now().Truncate(time.Microsecond)
	err = fix.svc.ProcessRefundSuccess(ctx, ref2.ID, procTime)
	require.NoError(t, err)

	quote5, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, quote5.LatestRefundStatus)
	assert.Equal(t, "succeeded", *quote5.LatestRefundStatus)
	require.NotNil(t, quote5.LatestRefundProcessedAt)
	assert.Equal(t, procTime.Unix(), quote5.LatestRefundProcessedAt.Unix())
	assert.False(t, quote5.CanRefund)
	assert.Equal(t, int64(3000), quote5.TotalRefundCents)
	assert.Equal(t, int64(3000), quote5.SucceededRefundedCents, "Succeeded refund must be reflected in SucceededRefundedCents")
	assert.Equal(t, int64(0), quote5.PendingRefundCents)
	assert.Equal(t, int64(3000), quote5.AlreadyRefundedCents)
	assert.Equal(t, int64(0), quote5.RemainingRefundableCents)

	// 6. Old failed row + newer succeeded row: succeeded wins deterministically
	var totalRefundRows int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&totalRefundRows)
	require.NoError(t, err)
	assert.Equal(t, 2, totalRefundRows, "Must have 2 refund records (1 failed, 1 succeeded)")
	assert.Equal(t, "succeeded", *quote5.LatestRefundStatus)
}

