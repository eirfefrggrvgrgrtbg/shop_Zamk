package payments

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestProblemCode_PaidOrderWithoutSucceededPayment(t *testing.T) {
	client, svc := setupTestService(t)
	SetupFixture(t, client, "failed", "paid", 100000, false, "")
	
	payments, _, err := svc.ListAdminPayments(context.Background(), "", "", "", "", "", "", "PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, payments)
	assert.Equal(t, []PaymentProblem{{Code: "PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT", Severity: "critical"}}, payments[0].Problems)
}

func TestProblemCode_SucceededPaymentOrderNotPaid(t *testing.T) {
	client, svc := setupTestService(t)
	SetupFixture(t, client, "succeeded", "awaiting_payment", 100000, false, "")
	
	payments, _, err := svc.ListAdminPayments(context.Background(), "", "", "", "", "", "", "SUCCEEDED_PAYMENT_ORDER_NOT_PAID", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, payments)
	assert.Equal(t, []PaymentProblem{{Code: "SUCCEEDED_PAYMENT_ORDER_NOT_PAID", Severity: "critical"}}, payments[0].Problems)
}

func TestProblemCode_MultipleSucceededPayments(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	
	// Create second succeeded payment
	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, integration_mode, payment_method, created_at) VALUES ($1, $2, 'tbank', $3, 'succeeded', 100000, 'RUB', $4, 'mock', 'tpay', $5)", uuid.New(), fix.OrderID, uuid.New().String(), uuid.New().String(), time.Now())
	require.NoError(t, err)

	payments, _, err := svc.ListAdminPayments(context.Background(), "ORD-" + fix.OrderID.String()[:8], "", "", "", "", "", "MULTIPLE_SUCCEEDED_PAYMENTS", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments, 2)
	assert.Equal(t, []PaymentProblem{{Code: "MULTIPLE_SUCCEEDED_PAYMENTS", Severity: "critical"}}, payments[0].Problems)
	assert.Equal(t, []PaymentProblem{{Code: "MULTIPLE_SUCCEEDED_PAYMENTS", Severity: "critical"}}, payments[1].Problems)
}

func TestProblemCode_AmountMismatch(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	
	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, "UPDATE payments SET amount_cents = 50000 WHERE id = $1", fix.PaymentID)
	require.NoError(t, err)

	payments, _, err := svc.ListAdminPayments(context.Background(), "", "", "", "", "", "", "AMOUNT_MISMATCH", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, payments)
	assert.Equal(t, []PaymentProblem{{Code: "AMOUNT_MISMATCH", Severity: "warning"}}, payments[0].Problems)
}

func TestProblemCode_StuckPending(t *testing.T) {
	client, svc := setupTestService(t)
	SetupFixture(t, client, "pending", "awaiting_payment", 100000, true, "") // true -> old created_at
	
	payments, _, err := svc.ListAdminPayments(context.Background(), "", "", "", "", "", "", "STUCK_PENDING", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, payments)
	assert.Equal(t, []PaymentProblem{{Code: "STUCK_PENDING", Severity: "warning"}}, payments[0].Problems)
}

func TestProblemCode_InvalidWebhookSignature(t *testing.T) {
	client, svc := setupTestService(t)
	SetupFixture(t, client, "pending", "awaiting_payment", 100000, false, "INVALID_WEBHOOK_SIGNATURE")
	
	payments, _, err := svc.ListAdminPayments(context.Background(), "", "", "", "", "", "", "INVALID_WEBHOOK_SIGNATURE", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, payments)
	assert.Equal(t, []PaymentProblem{{Code: "INVALID_WEBHOOK_SIGNATURE", Severity: "warning"}}, payments[0].Problems)
}

func TestProblemCode_UnprocessedWebhook(t *testing.T) {
	client, svc := setupTestService(t)
	SetupFixture(t, client, "pending", "awaiting_payment", 100000, false, "UNPROCESSED_WEBHOOK")
	
	payments, _, err := svc.ListAdminPayments(context.Background(), "", "", "", "", "", "", "UNPROCESSED_WEBHOOK", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, payments)
	assert.Equal(t, []PaymentProblem{{Code: "UNPROCESSED_WEBHOOK", Severity: "warning"}}, payments[0].Problems)
}

func TestProblemCodes_MultipleProblemsReturnedWithoutDuplicates(t *testing.T) {
	client, svc := setupTestService(t)
	// succeeded, but order is awaiting_payment (SUCCEEDED_PAYMENT_ORDER_NOT_PAID)
	// amount is 50000 vs 100000 (AMOUNT_MISMATCH)
	// mock invalid webhook (INVALID_WEBHOOK_SIGNATURE)
	fix := SetupFixture(t, client, "succeeded", "awaiting_payment", 100000, false, "INVALID_WEBHOOK_SIGNATURE")
	
	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, "UPDATE payments SET amount_cents = 50000 WHERE id = $1", fix.PaymentID)
	require.NoError(t, err)

	payments, _, err := svc.ListAdminPayments(context.Background(), "ORD-" + fix.OrderID.String()[:8], "", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, payments)

	expected := []PaymentProblem{
		{Code: "SUCCEEDED_PAYMENT_ORDER_NOT_PAID", Severity: "critical"},
		{Code: "AMOUNT_MISMATCH", Severity: "warning"},
		{Code: "INVALID_WEBHOOK_SIGNATURE", Severity: "warning"},
	}
	assert.Equal(t, expected, payments[0].Problems)
}

func TestAdminPaymentsProblemFilters_Service(t *testing.T) {
	client, svc := setupTestService(t)
	// We want to test that hasProblem and problemCode=AMOUNT_MISMATCH works.
	// We'll create one payment with AMOUNT_MISMATCH, and another without problems.
	fixProblem := SetupFixture(t, client, "succeeded", "paid", 100000, false, "INVALID_WEBHOOK_SIGNATURE")
	fixClean := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	
	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, "UPDATE payments SET amount_cents = 50000 WHERE id = $1", fixProblem.PaymentID)
	require.NoError(t, err)

	// 1. hasProblem=true
	payments, _, err := svc.ListAdminPayments(ctx, "", "", "", "", "", "", "", "", "", 0, 0, true, "createdAt", "DESC", 100, 0)
	require.NoError(t, err)

	foundProblem := false
	for _, p := range payments {
		if p.PaymentID == fixProblem.PaymentID {
			foundProblem = true
			assert.NotEmpty(t, p.Problems)
		}
		if p.PaymentID == fixClean.PaymentID {
			t.Fatal("Clean payment should not be returned when hasProblem=true")
		}
	}
	assert.True(t, foundProblem, "Payment with problem should be returned")

	// 2. problemCode=AMOUNT_MISMATCH
	payments, _, err = svc.ListAdminPayments(ctx, "", "", "", "", "", "", "AMOUNT_MISMATCH", "", "", 0, 0, false, "createdAt", "DESC", 100, 0)
	require.NoError(t, err)

	foundProblemCode := false
	for _, p := range payments {
		if p.PaymentID == fixProblem.PaymentID {
			foundProblemCode = true
			// Validate that the array contains ALL problems, not just AMOUNT_MISMATCH
			expected := []PaymentProblem{
				{Code: "AMOUNT_MISMATCH", Severity: "warning"},
				{Code: "INVALID_WEBHOOK_SIGNATURE", Severity: "warning"},
			}
			assert.Equal(t, expected, p.Problems)
		}
		if p.PaymentID == fixClean.PaymentID {
			t.Fatal("Clean payment should not be returned when problemCode=AMOUNT_MISMATCH")
		}
	}
	assert.True(t, foundProblemCode, "Payment with AMOUNT_MISMATCH should be returned")
}

func TestRefundSummary_UsesOnlySucceededAsRefunded(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	pid := fix.PaymentID

	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 30000, 'pending', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 10000, 'succeeded', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 50000, 'failed', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)

	var proof struct {
		ID string
		PaymentID string
		RefundOrderID string
		PaymentOrderID string
		ReturnID *string
		Status string
		Amount int64
	}
	err = client.Pool.QueryRow(ctx, `
		SELECT r.id, r.payment_id, r.order_id, p.order_id, r.return_id, r.status, r.amount_cents
		FROM refunds r JOIN payments p ON p.id = r.payment_id
		WHERE r.payment_id = $1 LIMIT 1
	`, pid).Scan(&proof.ID, &proof.PaymentID, &proof.RefundOrderID, &proof.PaymentOrderID, &proof.ReturnID, &proof.Status, &proof.Amount)
	require.NoError(t, err)
	assert.Equal(t, proof.PaymentOrderID, proof.RefundOrderID)

	payments, _, err := svc.ListAdminPayments(ctx, pid.String(), "", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	assert.Equal(t, int64(100000), payments[0].PaidAmountCents)
	assert.Equal(t, int64(10000), payments[0].SucceededRefundedAmountCents)
	assert.Equal(t, int64(30000), payments[0].PendingRefundAmountCents)
	assert.Equal(t, int64(40000), payments[0].ReservedRefundAmountCents)
	assert.Equal(t, int64(90000), payments[0].NetAmountCents)
	assert.Equal(t, int64(60000), payments[0].AvailableToRefundCents)
	assert.Equal(t, "partial_pending", payments[0].RefundState)
}

func TestPaymentAttempts_AreOrderedPerOrder(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "failed", "awaiting_payment", 100000, false, "")
	ctx := context.Background()

	p1 := fix.PaymentID
	p2 := uuid.New()
	// Ensure p2 > p1 for deterministic id ASC tie-breaker
	for p2.String() < p1.String() {
		p2 = uuid.New()
	}

	// Fetch original created_at to use identically
	var createdAt time.Time
	err := client.Pool.QueryRow(ctx, "SELECT created_at FROM payments WHERE id = $1", p1).Scan(&createdAt)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, integration_mode, payment_method, created_at) VALUES ($1, $2, 'tbank', $3, 'succeeded', 100000, 'RUB', $4, 'mock', 'tpay', $5)", p2, fix.OrderID, uuid.New().String(), uuid.New().String(), createdAt)
	require.NoError(t, err)

	payments, _, err := svc.ListAdminPayments(ctx, "ORD-" + fix.OrderID.String()[:8], "", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments, 2)

	for _, p := range payments {
		if p.PaymentID == p1 {
			assert.Equal(t, 1, p.AttemptNumber)
			assert.Equal(t, "failed", p.Status)
		} else if p.PaymentID == p2 {
			assert.Equal(t, 2, p.AttemptNumber)
			assert.Equal(t, "succeeded", p.Status)
		}
		assert.Equal(t, 2, p.AttemptsCount)
	}
}

func TestPaymentAttempts_FilterPreservesFullCount(t *testing.T) {
	client, svc := setupTestService(t)
	fix := SetupFixture(t, client, "failed", "paid", 100000, false, "")
	ctx := context.Background()

	p1 := fix.PaymentID
	p2 := uuid.New()
	for p2.String() < p1.String() {
		p2 = uuid.New()
	}

	var createdAt time.Time
	err := client.Pool.QueryRow(ctx, "SELECT created_at FROM payments WHERE id = $1", p1).Scan(&createdAt)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, integration_mode, payment_method, created_at) VALUES ($1, $2, 'tbank', $3, 'succeeded', 100000, 'RUB', $4, 'mock', 'tpay', $5)", p2, fix.OrderID, uuid.New().String(), uuid.New().String(), createdAt)
	require.NoError(t, err)

	// Filter by status=succeeded
	payments, _, err := svc.ListAdminPayments(ctx, "ORD-" + fix.OrderID.String()[:8], "succeeded", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	p := payments[0]
	assert.Equal(t, p2, p.PaymentID)
	assert.Equal(t, 2, p.AttemptNumber)
	assert.Equal(t, 2, p.AttemptsCount)
}

func TestPaymentAttempts_ArePartitionedByOrder(t *testing.T) {
	client, svc := setupTestService(t)
	fix1 := SetupFixture(t, client, "failed", "awaiting_payment", 100000, false, "")
	fix2 := SetupFixture(t, client, "failed", "awaiting_payment", 200000, false, "")
	ctx := context.Background()

	// Order 1 gets a second payment
	p1_2 := uuid.New()
	for p1_2.String() < fix1.PaymentID.String() {
		p1_2 = uuid.New()
	}
	_, err := client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, integration_mode, payment_method, created_at) VALUES ($1, $2, 'tbank', $3, 'succeeded', 100000, 'RUB', $4, 'mock', 'tpay', $5)", p1_2, fix1.OrderID, uuid.New().String(), uuid.New().String(), time.Now())
	require.NoError(t, err)

	// List Order 1
	payments1, _, err := svc.ListAdminPayments(ctx, "ORD-" + fix1.OrderID.String()[:8], "", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments1, 2)
	for _, p := range payments1 {
		assert.Equal(t, 2, p.AttemptsCount)
	}

	// List Order 2
	payments2, _, err := svc.ListAdminPayments(ctx, "ORD-" + fix2.OrderID.String()[:8], "", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments2, 1)
	assert.Equal(t, 1, payments2[0].AttemptNumber)
	assert.Equal(t, 1, payments2[0].AttemptsCount)
}

func TestRefundSummary_ReservesPendingRefund(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	pid := fix.PaymentID

	_, err := client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 30000, 'pending', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)

	var proof struct {
		RefundOrderID string
		PaymentOrderID string
	}
	err = client.Pool.QueryRow(ctx, "SELECT r.order_id, p.order_id FROM refunds r JOIN payments p ON p.id = r.payment_id WHERE r.payment_id = $1 LIMIT 1", pid).Scan(&proof.RefundOrderID, &proof.PaymentOrderID)
	require.NoError(t, err)
	assert.Equal(t, proof.PaymentOrderID, proof.RefundOrderID)

	payments, _, err := svc.ListAdminPayments(ctx, pid.String(), "", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	assert.Equal(t, int64(100000), payments[0].PaidAmountCents)
	assert.Equal(t, int64(0), payments[0].SucceededRefundedAmountCents)
	assert.Equal(t, int64(30000), payments[0].PendingRefundAmountCents)
	assert.Equal(t, int64(30000), payments[0].ReservedRefundAmountCents)
	assert.Equal(t, int64(100000), payments[0].NetAmountCents)
	assert.Equal(t, int64(70000), payments[0].AvailableToRefundCents)
	assert.Equal(t, "pending", payments[0].RefundState)
}

func TestAvailableToRefund_DoesNotGoNegative(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	pid := fix.PaymentID

	_, err := client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 120000, 'succeeded', 'return', 'RUB', now(), now())", uuid.New(), pid, fix.OrderID)
	require.NoError(t, err)

	var proof struct {
		RefundOrderID string
		PaymentOrderID string
	}
	err = client.Pool.QueryRow(ctx, "SELECT r.order_id, p.order_id FROM refunds r JOIN payments p ON p.id = r.payment_id WHERE r.payment_id = $1 LIMIT 1", pid).Scan(&proof.RefundOrderID, &proof.PaymentOrderID)
	require.NoError(t, err)
	assert.Equal(t, proof.PaymentOrderID, proof.RefundOrderID)

	payments, _, err := svc.ListAdminPayments(ctx, pid.String(), "", "", "", "", "", "", "", "", 0, 0, false, "createdAt", "DESC", 10, 0)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	assert.Equal(t, int64(120000), payments[0].SucceededRefundedAmountCents)
	assert.Equal(t, int64(0), payments[0].NetAmountCents)
	assert.Equal(t, int64(0), payments[0].AvailableToRefundCents)
	assert.Equal(t, "full", payments[0].RefundState)
}

func TestPaymentFixtureCleanup_RemovesOnlyOwnGraph(t *testing.T) {
	client, _ := setupTestService(t)
	ctx := context.Background()

	fixA := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	fixB := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")

	_, err := client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) VALUES ($1, $2, $3, 1000, 'succeeded', 'return', 'RUB', now(), now())", uuid.New(), fixA.PaymentID, fixA.OrderID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "INSERT INTO payment_events (id, payment_id, provider, event_type, event_key, raw_payload, created_at) VALUES ($1, $2, 'tbank', 'custom', $3, '{}', now())", uuid.New(), fixA.PaymentID, uuid.New().String())
	require.NoError(t, err)

	err = CleanupPaymentTestFixture(ctx, client.Pool, fixA)
	require.NoError(t, err)

	checkCount := func(table string, fix PaymentTestFixture) int {
		var count int
		var query string
		switch table {
		case "users": query = "SELECT count(*) FROM users WHERE id = $1"
		case "sellers": query = "SELECT count(*) FROM sellers WHERE id = $1"
		case "orders": query = "SELECT count(*) FROM orders WHERE id = $1"
		case "payments": query = "SELECT count(*) FROM payments WHERE order_id = $1"
		case "order_fulfillments": query = "SELECT count(*) FROM order_fulfillments WHERE order_id = $1"
		case "refunds": query = "SELECT count(*) FROM refunds WHERE payment_id = $1"
		case "payment_events": query = "SELECT count(*) FROM payment_events WHERE payment_id = $1"
		}
		var param interface{} = fix.UserID
		if table == "sellers" { param = fix.SellerID }
		if table == "orders" || table == "payments" || table == "order_fulfillments" { param = fix.OrderID }
		if table == "refunds" || table == "payment_events" { param = fix.PaymentID }

		err = client.Pool.QueryRow(ctx, query, param).Scan(&count)
		require.NoError(t, err)
		return count
	}

	assert.Equal(t, 0, checkCount("users", fixA))
	assert.Equal(t, 0, checkCount("sellers", fixA))
	assert.Equal(t, 0, checkCount("orders", fixA))
	assert.Equal(t, 0, checkCount("payments", fixA))
	assert.Equal(t, 0, checkCount("order_fulfillments", fixA))
	assert.Equal(t, 0, checkCount("refunds", fixA))
	assert.Equal(t, 0, checkCount("payment_events", fixA))

	assert.Equal(t, 1, checkCount("users", fixB))
	assert.Equal(t, 1, checkCount("sellers", fixB))
	assert.Equal(t, 1, checkCount("orders", fixB))
	assert.Equal(t, 1, checkCount("payments", fixB))
	assert.Equal(t, 1, checkCount("order_fulfillments", fixB))

	err = CleanupPaymentTestFixture(ctx, client.Pool, fixB)
	require.NoError(t, err)

	assert.Equal(t, 0, checkCount("users", fixB))
	assert.Equal(t, 0, checkCount("orders", fixB))
}
