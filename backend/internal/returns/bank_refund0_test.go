package returns_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

// BANK.REFUND.0: bank refund attempts against the in-memory FakeRefundProvider (no network).

type bankRefundOrder struct {
	ord               testOrder
	paymentID         uuid.UUID
	providerPaymentID string
}

// newBankRefundOrder: a delivered order of qty units at 1000 each, fully paid, with the seller earning available.
func newBankRefundOrder(t *testing.T, fix *m51Fixture, qty int) bankRefundOrder {
	t.Helper()
	ctx := context.Background()
	ord := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), qty)
	payID := createSucceededPayment(t, fix, ord.orderID, int64(qty*1000))
	var ppid string
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT provider_payment_id FROM payments WHERE id = $1`, payID).Scan(&ppid))
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', $5, 'RUB', now() - interval '1 minute', '{}', now())
	`, uuid.New(), fix.sellerAID, ord.orderID, ord.orderItemID, int64(qty*1000))
	require.NoError(t, err)
	return bankRefundOrder{ord: ord, paymentID: payID, providerPaymentID: ppid}
}

// newReceivedReturn: a return of accepted units of the order, received at the warehouse.
func newReceivedReturn(t *testing.T, fix *m51Fixture, o bankRefundOrder, accepted int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	retID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, o.ord.orderID, o.ord.fulfillmentID, fix.userID)
	require.NoError(t, err)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, $4, $4, now())
	`, uuid.New(), retID, o.ord.orderItemID, accepted)
	require.NoError(t, err)
	return retID
}

type refundRowState struct {
	Status        string
	SubmitOutcome string
	Key           *string
	Attempts      int
	ReviewReason  *string
	RetryOf       *uuid.UUID
	FailureCode   *string
}

func readRefundRow(t *testing.T, fix *m51Fixture, refundID uuid.UUID) refundRowState {
	t.Helper()
	var r refundRowState
	require.NoError(t, fix.client.Pool.QueryRow(context.Background(), `
		SELECT status, submit_outcome, idempotency_key, submit_attempts, review_reason, retry_of_refund_id, failure_code
		FROM refunds WHERE id = $1
	`, refundID).Scan(&r.Status, &r.SubmitOutcome, &r.Key, &r.Attempts, &r.ReviewReason, &r.RetryOf, &r.FailureCode))
	return r
}

func returnStatus(t *testing.T, fix *m51Fixture, retID uuid.UUID) string {
	t.Helper()
	var s string
	require.NoError(t, fix.client.Pool.QueryRow(context.Background(), `SELECT status FROM returns WHERE id = $1`, retID).Scan(&s))
	return s
}

// returnDeductions counts and sums the authoritative return deductions of an order (not the payout holds).
func returnDeductions(t *testing.T, fix *m51Fixture, orderID uuid.UUID) (int, int64) {
	t.Helper()
	var n int
	var sum int64
	require.NoError(t, fix.client.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*), COALESCE(SUM(amount_cents), 0) FROM seller_ledger_entries
		WHERE order_id = $1 AND type = 'adjustment'
		  AND COALESCE(metadata->>'reason', '') NOT IN ('return_hold', 'return_hold_release')
	`, orderID).Scan(&n, &sum))
	return n, sum
}

func refundCountForReturn(t *testing.T, fix *m51Fixture, retID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, fix.client.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM refunds WHERE return_id = $1`, retID).Scan(&n))
	return n
}

func withFakeRefundProvider(fix *m51Fixture) *payments.FakeRefundProvider {
	fake := payments.NewFakeRefundProvider()
	fix.svc.WithRefundProvider(fake)
	return fake
}

// A + B: reserved -> accepted (no financial success) -> confirmed by the provider -> succeeded, exactly once.
func TestBankRefund0_A_B_AcceptedThenConfirmed(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 3)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "pending", row.Status)
	assert.Equal(t, "not_sent", row.SubmitOutcome)
	require.NotNil(t, row.Key, "the idempotency key is fixed with the reservation, before any provider call")
	key := *row.Key

	ref, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", ref.Status)
	row = readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "accepted", row.SubmitOutcome)
	assert.Equal(t, key, *row.Key, "the key never changes")
	assert.Equal(t, 1, fake.SubmitCalls(key))

	// B: accepted is not a refund: still reserved, return open, no seller deduction.
	ref, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", ref.Status)
	assert.Equal(t, "item_received", returnStatus(t, fix, retID))
	n, _ := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 0, n)
	quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), quote.PendingRefundCents)
	assert.Equal(t, int64(0), quote.SucceededRefundedCents)

	// A: the provider's attributed confirmation settles it.
	require.NoError(t, fake.Confirm(key))
	ref, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", ref.Status)
	assert.NotNil(t, ref.ProcessedAt)
	assert.Equal(t, "confirmed", readRefundRow(t, fix, ref.ID).SubmitOutcome)
	assert.Equal(t, "refunded", returnStatus(t, fix, retID))
	n, sum := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 1, n)
	assert.Equal(t, int64(-3000), sum)
	assert.Equal(t, int64(3000), fake.RefundedCents(o.providerPaymentID))
	assert.Equal(t, 1, fake.SubmitCalls(key), "settling never sends again")
}

// C: a timeout is 'unknown', never 'failed'; nothing but the bank result may settle it.
func TestBankRefund0_C_TimeoutIsUnknown(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 3)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	fake.QueueSubmit(payments.FakeSubmitTimeout)
	ref, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", ref.Status)
	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "unknown", row.SubmitOutcome)
	assert.Nil(t, row.FailureCode)
	assert.Equal(t, "item_received", returnStatus(t, fix, retID))
	n, _ := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 0, n)

	// The amount stays reserved.
	quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), quote.PendingRefundCents)
	assert.False(t, quote.CanRefund)

	// Neither the canonical success / failure nor the simulator may settle an attempt handed to the bank.
	err = fix.client.RunInTx(ctx, func(tx pgx.Tx) error { return fix.svc.ProcessRefundFailureTx(ctx, tx, ref.ID) })
	assert.ErrorIs(t, err, returns.ErrRefundAwaitingBankConfirmation)
	err = fix.client.RunInTx(ctx, func(tx pgx.Tx) error { return fix.svc.ProcessRefundSuccessTx(ctx, tx, ref.ID, time.Now()) })
	assert.ErrorIs(t, err, returns.ErrRefundAwaitingBankConfirmation)
	_, err = fix.svc.SimulateRefundSuccess(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrRefundAwaitingBankConfirmation)
	_, err = fix.svc.SimulateRefundFailure(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrRefundAwaitingBankConfirmation)
	assert.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)

	// The provider has no record of the request: still unknown, not failed.
	ref, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", ref.Status)
	assert.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)

	// A provider error while checking changes nothing either.
	fake.FailStateQueries(errors.New("provider unavailable"))
	_, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	assert.Error(t, err)
	assert.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)
}

// D: repeating after a timeout or a lost answer creates no second refund and no second provider operation.
func TestBankRefund0_D_RetryAfterUnknownNeverSendsAgain(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 3)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	key := *readRefundRow(t, fix, ref.ID).Key
	fake.QueueSubmit(payments.FakeSubmitLostResponse) // the provider executes it, the answer is lost
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)

	for i := 0; i < 3; i++ {
		again, err := fix.svc.SubmitRefund(ctx, ref.ID)
		require.NoError(t, err)
		assert.Equal(t, ref.ID, again.ID)
		recreated, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		assert.Equal(t, ref.ID, recreated.ID, "the same local action returns the existing operation")
	}
	assert.Equal(t, 1, fake.SubmitCalls(key), "an unknown outcome is never resent")
	assert.Equal(t, 1, fake.Operations(o.providerPaymentID))
	assert.Equal(t, 1, refundCountForReturn(t, fix, retID))
	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, 1, row.Attempts)
	assert.Equal(t, key, *row.Key)

	// Reconciliation learns the provider holds this request, then confirms it.
	_, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "accepted", readRefundRow(t, fix, ref.ID).SubmitOutcome)
	require.NoError(t, fake.Confirm(key))
	ref, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", ref.Status)
	assert.Equal(t, 1, fake.SubmitCalls(key))
}

// E: a repeated confirmation (and a repeated canonical success) is a no-op on the ledger.
func TestBankRefund0_E_RepeatedConfirmationIsIdempotent(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 3)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	require.NoError(t, fake.Confirm(*readRefundRow(t, fix, ref.ID).Key))

	for i := 0; i < 3; i++ {
		got, err := fix.svc.ReconcileRefund(ctx, ref.ID)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", got.Status)
	}
	require.NoError(t, fix.client.RunInTx(ctx, func(tx pgx.Tx) error {
		return fix.svc.ProcessRefundSuccessTx(ctx, tx, ref.ID, time.Now())
	}))
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	_, err = fix.svc.SimulateRefundSuccess(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrNoPendingRefund)

	n, sum := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 1, n, "the deduction is written once")
	assert.Equal(t, int64(-3000), sum)
	assert.Equal(t, "confirmed", readRefundRow(t, fix, ref.ID).SubmitOutcome)
}

// F: concurrent refund requests never reserve more than was paid.
func TestBankRefund0_F_ConcurrentRequestsStayWithinPaid(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	withFakeRefundProvider(fix)

	t.Run("same_return", func(t *testing.T) {
		o := newBankRefundOrder(t, fix, 3)
		retID := newReceivedReturn(t, fix, o, 3)
		var wg sync.WaitGroup
		ids := make([]uuid.UUID, 6)
		for i := range ids {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
				if assert.NoError(t, err) {
					ids[i] = ref.ID
				}
			}(i)
		}
		wg.Wait()
		for _, id := range ids {
			assert.Equal(t, ids[0], id)
		}
		assert.Equal(t, 1, refundCountForReturn(t, fix, retID))
	})

	t.Run("two_returns_of_one_order", func(t *testing.T) {
		o := newBankRefundOrder(t, fix, 3)
		ret1 := newReceivedReturn(t, fix, o, 2)
		ret2 := newReceivedReturn(t, fix, o, 2) // 4 units claimed of 3 paid
		var wg sync.WaitGroup
		for _, r := range []uuid.UUID{ret1, ret2} {
			wg.Add(1)
			go func(r uuid.UUID) {
				defer wg.Done()
				_, _ = fix.svc.CreateRefund(ctx, fix.userID, r, returns.CreateRefundRequest{})
			}(r)
		}
		wg.Wait()
		var reserved int64
		require.NoError(t, fix.client.Pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount_cents), 0) FROM refunds WHERE payment_id = $1 AND status IN ('pending', 'processing', 'succeeded')
		`, o.paymentID).Scan(&reserved))
		assert.LessOrEqual(t, reserved, int64(3000))
		assert.Greater(t, reserved, int64(0))
	})
}

// G: two partial refunds of one payment never confirm each other; an unattributable answer goes to review.
func TestBankRefund0_G_PartialRefundsAreAttributedSeparately(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	ret1 := newReceivedReturn(t, fix, o, 1)
	ret2 := newReceivedReturn(t, fix, o, 1)

	ref1, err := fix.svc.CreateRefund(ctx, fix.userID, ret1, returns.CreateRefundRequest{})
	require.NoError(t, err)
	ref2, err := fix.svc.CreateRefund(ctx, fix.userID, ret2, returns.CreateRefundRequest{})
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, ref1.ID)
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, ref2.ID)
	require.NoError(t, err)
	key1 := *readRefundRow(t, fix, ref1.ID).Key
	key2 := *readRefundRow(t, fix, ref2.ID).Key
	require.NotEqual(t, key1, key2)
	assert.Equal(t, 2, fake.Operations(o.providerPaymentID))

	// Only the first is refunded by the provider.
	require.NoError(t, fake.Confirm(key1))
	got2, err := fix.svc.ReconcileRefund(ctx, ref2.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", got2.Status, "the other refund's confirmation does not settle this one")
	got1, err := fix.svc.ReconcileRefund(ctx, ref1.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", got1.Status)

	// A contradictory answer for the second: success, but attributed to the first attempt.
	fake.OverrideState(key2, payments.RefundStateResult{
		Outcome:           payments.RefundStateSucceeded,
		ProviderPaymentID: o.providerPaymentID,
		IdempotencyKey:    key1,
		AmountCents:       1000,
	})
	got2, err = fix.svc.ReconcileRefund(ctx, ref2.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", got2.Status)
	row2 := readRefundRow(t, fix, ref2.ID)
	assert.Equal(t, "accepted", row2.SubmitOutcome)
	require.NotNil(t, row2.ReviewReason)
	assert.Equal(t, "item_received", returnStatus(t, fix, ret2))
	_, err = fix.svc.ReconcileRefund(ctx, ref2.ID)
	assert.ErrorIs(t, err, returns.ErrRefundUnderReview, "a refund under review is not settled automatically")

	n, sum := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 1, n, "only the confirmed partial refund is deducted")
	assert.Equal(t, int64(-1000), sum)
}

// G (amount): the right key with another amount is not a confirmation either.
func TestBankRefund0_G_AmountMismatchGoesToReview(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 2)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	key := *readRefundRow(t, fix, ref.ID).Key
	fake.OverrideState(key, payments.RefundStateResult{
		Outcome: payments.RefundStateSucceeded, ProviderPaymentID: o.providerPaymentID, IdempotencyKey: key, AmountCents: 3000,
	})
	got, err := fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", got.Status)
	assert.NotNil(t, readRefundRow(t, fix, ref.ID).ReviewReason)
	n, _ := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 0, n)
}

// H: a definitive rejection fails the attempt, releases the reservation, and a retry is linked to it.
func TestBankRefund0_H_DefinitiveRejection(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)

	t.Run("rejected_on_submit", func(t *testing.T) {
		o := newBankRefundOrder(t, fix, 3)
		retID := newReceivedReturn(t, fix, o, 3)
		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		fake.QueueSubmit(payments.FakeSubmitReject)
		ref, err = fix.svc.SubmitRefund(ctx, ref.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", ref.Status)
		row := readRefundRow(t, fix, ref.ID)
		assert.Equal(t, "rejected", row.SubmitOutcome)
		require.NotNil(t, row.FailureCode)
		assert.Equal(t, "item_received", returnStatus(t, fix, retID))
		n, _ := returnDeductions(t, fix, o.ord.orderID)
		assert.Equal(t, 0, n, "a failed refund deducts nothing")

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), quote.PendingRefundCents)
		assert.Equal(t, int64(3000), quote.RemainingRefundableCents)
		assert.True(t, quote.CanRefund)

		retry, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		assert.NotEqual(t, ref.ID, retry.ID)
		retryRow := readRefundRow(t, fix, retry.ID)
		require.NotNil(t, retryRow.RetryOf)
		assert.Equal(t, ref.ID, *retryRow.RetryOf, "the retry is linked to the failed attempt")
		assert.NotEqual(t, *row.Key, *retryRow.Key, "a new attempt has its own key")
		var reserved int64
		require.NoError(t, fix.client.Pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount_cents), 0) FROM refunds WHERE payment_id = $1 AND status IN ('pending', 'processing', 'succeeded')
		`, o.paymentID).Scan(&reserved))
		assert.Equal(t, int64(3000), reserved, "no double reservation")
	})

	t.Run("rejected_after_acceptance", func(t *testing.T) {
		o := newBankRefundOrder(t, fix, 3)
		retID := newReceivedReturn(t, fix, o, 3)
		ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		_, err = fix.svc.SubmitRefund(ctx, ref.ID)
		require.NoError(t, err)
		require.NoError(t, fake.Fail(*readRefundRow(t, fix, ref.ID).Key, "DECLINED"))
		ref, err = fix.svc.ReconcileRefund(ctx, ref.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", ref.Status)
		assert.Equal(t, "rejected", readRefundRow(t, fix, ref.ID).SubmitOutcome)
		assert.Equal(t, "item_received", returnStatus(t, fix, retID))
	})
}

// I: nothing beyond the paid amount can be reserved, unresolved bank attempts included.
func TestBankRefund0_I_CannotReserveMoreThanPaid(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	paySvc := payments.NewService(payments.NewRepository(fix.client.Pool), fix.ordersRepo, nil, nil, fix.client, nil, nil,
		&config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}})
	o := newBankRefundOrder(t, fix, 3)

	assert.ErrorIs(t, paySvc.ReserveRefund(ctx, o.paymentID, 3001, "over", nil), payments.ErrRefundExceedsPaid)

	retID := newReceivedReturn(t, fix, o, 3)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	fake.QueueSubmit(payments.FakeSubmitTimeout)
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	require.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)

	assert.ErrorIs(t, paySvc.ReserveRefund(ctx, o.paymentID, 1, "extra", nil), payments.ErrRefundExceedsPaid,
		"an unknown attempt keeps its amount reserved")
}

// J: a failure while applying a confirmation rolls back refund, return and ledger together.
func TestBankRefund0_J_RollbackLeavesNoPartialState(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 3)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	require.NoError(t, fake.Confirm(*readRefundRow(t, fix, ref.ID).Key))

	// Break the allocation invariant (1 allocation for 3 units) so the seller deduction fails mid-transaction.
	createSerializedAllocations(t, fix, o.ord.orderID, o.ord.orderItemID, fix.varAID, 1)
	_, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.ErrorIs(t, err, returns.ErrRefundAllocationInvariant)

	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "pending", row.Status)
	assert.Equal(t, "accepted", row.SubmitOutcome)
	assert.Equal(t, "item_received", returnStatus(t, fix, retID))
	n, _ := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 0, n)
	var processed *time.Time
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT processed_at FROM refunds WHERE id = $1`, ref.ID).Scan(&processed))
	assert.Nil(t, processed)
}

// K: while the bank result is open the return keeps holding the disputed seller earning (PAYOUT.SAFETY.1); the
// confirmation then deducts once and releases the hold.
func TestBankRefund0_K_OpenReturnKeepsHoldingEarning(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	payoutSvc := payouts.NewService(payouts.NewRepository(fix.client.Pool), fix.client, fix.returnsRepo, fix.ordersRepo,
		&config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}, fix.notifSvc)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 1)

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	fake.QueueSubmit(payments.FakeSubmitLostResponse) // executed by the provider, answer lost
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	require.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)

	batch, err := payoutSvc.CreatePayoutBatchForSeller(ctx, fix.sellerAID)
	require.NoError(t, err)
	assert.Equal(t, int64(2000), batch.AmountCents, "the disputed unit (1000) is held out of the batch")
	var holds int64
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM seller_ledger_entries
		WHERE seller_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold'
	`, fix.sellerAID).Scan(&holds))
	assert.Equal(t, int64(-1000), holds)
	n, _ := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 0, n, "no deduction while the bank result is unknown")

	// The provider holds the request; it is confirmed.
	_, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	require.NoError(t, fake.Confirm(*readRefundRow(t, fix, ref.ID).Key))
	got, err := fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", got.Status)

	n, sum := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 1, n)
	assert.Equal(t, int64(-1000), sum)
	var releases int64
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM seller_ledger_entries
		WHERE seller_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold_release'
	`, fix.sellerAID).Scan(&releases))
	assert.Equal(t, int64(1000), releases, "hold and release cancel out; the deduction counts once")
}

// Simulator gate: with a real terminal configured the simulator settles nothing.
func TestBankRefund0_SimulatorDisabledWithRealTerminal(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 3)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	staging := &config.Config{App: config.AppConfig{Env: "staging"}}
	staging.TBank.TerminalKey = "1234567890DEMO"
	staging.TBank.Password = "secret"
	require.False(t, staging.RefundSimulatorAllowed())
	fix.svc.WithRefundSimulator(staging.RefundSimulatorAllowed())
	t.Cleanup(func() { fix.svc.WithRefundSimulator(true) })

	_, err = fix.svc.SimulateRefundSuccess(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrRefundSimulatorDisabled)
	_, err = fix.svc.SimulateRefundFailure(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrRefundSimulatorDisabled)

	h := returns.NewHandler(fix.svc, "staging")
	r := chi.NewRouter()
	r.Post("/admin/returns/{id}/simulate-refund-success", h.SimulateRefundSuccess)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID.String()+"/simulate-refund-success", nil))
	assert.Equal(t, http.StatusForbidden, rr.Code)

	assert.Equal(t, "pending", readRefundRow(t, fix, ref.ID).Status)
	assert.Equal(t, "item_received", returnStatus(t, fix, retID))
}

// The schema guards a bank attempt: key / amount immutable, outcome consistent with status, final outcomes final.
func TestBankRefund0_SchemaGuards(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	retID := newReceivedReturn(t, fix, o, 3)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)

	for name, q := range map[string]string{
		"key_change":            `UPDATE refunds SET idempotency_key = 'other' WHERE id = $1`,
		"amount_change":         `UPDATE refunds SET amount_cents = amount_cents + 1 WHERE id = $1`,
		"back_to_not_sent":      `UPDATE refunds SET submit_outcome = 'not_sent' WHERE id = $1`,
		"succeeded_unconfirmed": `UPDATE refunds SET status = 'succeeded' WHERE id = $1`,
		"failed_unrejected":     `UPDATE refunds SET status = 'failed' WHERE id = $1`,
	} {
		_, err := fix.client.Pool.Exec(ctx, q, ref.ID)
		assert.Error(t, err, name)
	}
	assert.Equal(t, "accepted", readRefundRow(t, fix, ref.ID).SubmitOutcome)
}
