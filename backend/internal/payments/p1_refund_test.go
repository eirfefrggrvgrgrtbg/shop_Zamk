package payments

import (
	"context"
	"sync"
	"testing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefundReservationRejectsZero(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	ctx := context.Background()

	err := svc.ReserveRefund(ctx, fix.PaymentID, 0, "return item", nil)
	assert.ErrorIs(t, err, ErrInvalidRefundAmount)

	var count int
	client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE payment_id = $1", fix.PaymentID).Scan(&count)
	assert.Equal(t, 0, count)
}

func TestRefundReservationRejectsNegative(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	ctx := context.Background()

	err := svc.ReserveRefund(ctx, fix.PaymentID, -100, "return item", nil)
	assert.ErrorIs(t, err, ErrInvalidRefundAmount)

	var count int
	client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE payment_id = $1", fix.PaymentID).Scan(&count)
	assert.Equal(t, 0, count)
}

func TestRefundReservationRejectsAmountAboveAvailable(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	ctx := context.Background()

	err := svc.ReserveRefund(ctx, fix.PaymentID, 100001, "return item", nil)
	assert.ErrorIs(t, err, ErrRefundExceedsPaid)

	var count int
	client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE payment_id = $1", fix.PaymentID).Scan(&count)
	assert.Equal(t, 0, count)
}

func TestRefundReservationRequiresSucceededPayment(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	statuses := []string{"created", "pending", "failed", "cancelled"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			fix := SetupFixture(t, client, status, "awaiting_payment", 100000, false, "")
			
			err := svc.ReserveRefund(ctx, fix.PaymentID, 10000, "return item", nil)
			assert.ErrorIs(t, err, ErrInvalidPaymentStatus)

			var count int
			client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE payment_id = $1", fix.PaymentID).Scan(&count)
			assert.Equal(t, 0, count)

			var pStatus string
			client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", fix.PaymentID).Scan(&pStatus)
			assert.Equal(t, status, pStatus)

			var oStatus string
			client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", fix.OrderID).Scan(&oStatus)
			assert.Equal(t, "awaiting_payment", oStatus)
		})
	}
}

func TestRefundReservationRejectsMismatchedOrderAndPayment(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	fixA := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	fixB := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")

	returnID := uuid.New()
	_, err := client.Pool.Exec(ctx, "INSERT INTO returns (id, order_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, 'pending', 'defective', now(), now())", returnID, fixB.OrderID, fixB.UserID)
	require.NoError(t, err)

	// Now try to refund using fixA.PaymentID, but providing fixB returnID.
	// Wait, ReserveRefund only takes paymentID, amount, reason, returnID.
	// We need to check if payment ID matches order ID? Wait, if we use ReserveRefund directly, it doesn't know about Return's order ID inside ReserveRefund!
	// Ah! ReserveRefund uses `return_id` to insert the refund, but it uses the payment's `order_id`. So the return's order_id might mismatch the refund's order_id!
	// Does ReserveRefund validate that `return.order_id == payment.order_id`? The domain requires it!
	
	// Wait! the prompt says: "Reserve flow должен доказать: ... Refund.order_id = Return.order_id".
	// ReserveRefund signature: ReserveRefund(ctx context.Context, paymentID uuid.UUID, requestedAmountCents int64, reason string, returnID *uuid.UUID)
	// I should pass returnID to ReserveRefund and it MUST validate that the return belongs to the same order as the payment!
	// Wait, currently ReserveRefund might NOT validate this, meaning I might need to implement the validation inside ReserveRefund!
	// Or maybe the prompt means the HTTP handler validates it? "Нельзя создать Refund, используя Payment другого Order."
	
	// Let's first test the Repository validation (FK might reject it, but the DB FK is on Return, so `return_id REFERENCES returns(id)`).
	// If the DB has no trigger for `return.order_id == payment.order_id`, we must check it in code.
	
	err = svc.ReserveRefund(ctx, fixA.PaymentID, 1000, "reason", &returnID)
	assert.ErrorIs(t, err, ErrMismatchedOrderAndPayment)
}

func TestRefundReservationCountsPendingAndSucceededOnly(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	fix := SetupFixture(t, client, "succeeded", "paid", 1000000, false, "")
	pid := fix.PaymentID

	_, err := client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 200000, 'succeeded', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 300000, 'pending', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 400000, 'failed', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)

	// Available should be 1000000 - 200000 - 300000 = 500000
	err = svc.ReserveRefund(ctx, fix.PaymentID, 500001, "return item", nil)
	assert.ErrorIs(t, err, ErrRefundExceedsPaid)

	err = svc.ReserveRefund(ctx, fix.PaymentID, 500000, "return item", nil)
	assert.NoError(t, err)

	repo := NewRepository(client.Pool)
	client.RunInTx(ctx, func(tx pgx.Tx) error {
		summary, _ := repo.GetRefundSummaryTx(ctx, tx, fix.PaymentID)
		assert.Equal(t, int64(800000), summary.PendingAmountCents)
		assert.Equal(t, int64(200000), summary.SucceededAmountCents)
		return nil
	})
}

func TestConcurrentRefundReservation_PreventsOverRefund(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "succeeded", "paid", 1000000, false, "")
	ctx := context.Background()

	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	results := make(chan error, 2)

	startBarrier.Add(1)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startBarrier.Wait()
			err := svc.ReserveRefund(ctx, fix.PaymentID, 700000, "return item", nil)
			results <- err
		}(i)
	}

	startBarrier.Done()
	wg.Wait()
	close(results)

	successCount := 0
	failCount := 0

	for err := range results {
		if err == nil {
			successCount++
		} else if err == ErrRefundExceedsPaid {
			failCount++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, failCount)

	var count int
	client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE payment_id = $1", fix.PaymentID).Scan(&count)
	assert.Equal(t, 1, count)

	repo := NewRepository(client.Pool)
	client.RunInTx(ctx, func(tx pgx.Tx) error {
		summary, _ := repo.GetRefundSummaryTx(ctx, tx, fix.PaymentID)
		assert.Equal(t, int64(700000), summary.PendingAmountCents)
		return nil
	})
}
