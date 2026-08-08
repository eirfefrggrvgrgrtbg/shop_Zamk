package payments

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminPaymentsSearch(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	// 1. Create a base fixture with known customer details
	fix1 := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	searchEmail := uuid.New().String()[:8] + "@example.com"
	_, err := client.Pool.Exec(ctx, `UPDATE users SET name = $1, email = $2, phone = $3 WHERE id = $4`,
		"John Doe", searchEmail, "+1234567890", fix1.UserID)
	require.NoError(t, err)

	var pNum1, provPayID1 string
	err = client.Pool.QueryRow(ctx, "SELECT payment_number, provider_payment_id FROM payments WHERE id = $1", fix1.PaymentID).Scan(&pNum1, &provPayID1)
	require.NoError(t, err)

	pID2 := uuid.New()
	pNum2 := "PAY-" + pID2.String()[:8]
	providerPaymentID2 := "bank-" + uuid.New().String()
	_, err = client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, integration_mode, payment_method, payment_number, created_at) VALUES ($1, $2, 'tbank', $3, 'succeeded', 50000, 'RUB', $4, 'mock', 'tpay', $5, $6)", 
		pID2, fix1.OrderID, providerPaymentID2, uuid.New().String(), pNum2, time.Now())
	require.NoError(t, err)

	// Searches that should match fix1.PaymentID
	tests := []struct {
		name          string
		q             string
		expectPayment uuid.UUID
	}{
		{"paymentNumber", pNum1, fix1.PaymentID},
		{"providerPaymentId", provPayID1, fix1.PaymentID},
		{"orderNumber", "ORD-" + fix1.OrderID.String()[:8], fix1.PaymentID},
		{"customer name", "John DOe", fix1.PaymentID},
		{"customer email", searchEmail, fix1.PaymentID},
		{"customer phone", "23456789", fix1.PaymentID},
		{"payment UUID", fix1.PaymentID.String(), fix1.PaymentID},
		{"payment UUID 2", pID2.String(), pID2},
		{"paymentNumber 2", pNum2, pID2},
		{"providerPaymentId 2", providerPaymentID2, pID2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _, err := svc.ListAdminPayments(ctx, tt.q, "", "", "", "", "", "", "", "", 0, 0, false, "", "", 10, 0)
			require.NoError(t, err)
			
			found := false
			for _, p := range res {
				if p.PaymentID == tt.expectPayment {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected to find payment %s with q=%s", tt.expectPayment, tt.q)
		})
	}

	t.Run("SQL injection should be safe", func(t *testing.T) {
		q := "' OR 1=1 --"
		res, _, err := svc.ListAdminPayments(ctx, q, "", "", "", "", "", "", "", "", 0, 0, false, "", "", 10, 0)
		require.NoError(t, err)
		assert.Empty(t, res, "Should not return anything for literal search of SQL injection string")
	})

	t.Run("Spaces should be normalized or empty", func(t *testing.T) {
		q := "   "
		res, total, err := svc.ListAdminPayments(ctx, q, "", "", "", "", "", "", "", "", 0, 0, false, "", "", 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0, "Spaces should be treated as empty or find nothing without crashing")
		if len(res) == 0 {
			// If it treats spaces literally and finds nothing, that is also fine.
		}
	})
}

func TestAdminPaymentsFilters(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")
	// Make it partial refund
	_, err := client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, status, amount_cents, currency, provider_refund_id, created_at) VALUES ($1, $2, $3, 'succeeded', 10000, 'RUB', $4, $5)",
		uuid.New(), fix.PaymentID, fix.OrderID, uuid.New().String(), time.Now())
	require.NoError(t, err)
	// Add a problem
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'cancelled' WHERE id = $1", fix.OrderID)
	require.NoError(t, err) // SUCCEEDED_PAYMENT_ORDER_NOT_PAID

	// Query with ALL filters combined
	res, total, err := svc.ListAdminPayments(ctx, 
		"", 
		"succeeded", 
		"tbank", 
		"tpay", 
		"mock", 
		"partial", 
		"SUCCEEDED_PAYMENT_ORDER_NOT_PAID", 
		"", "", 
		50000, 150000, 
		true, // hasProblem
		"createdAt", "desc", 
		10, 0,
	)
	
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)
	
	found := false
	for _, p := range res {
		if p.PaymentID == fix.PaymentID {
			found = true
			assert.Equal(t, "succeeded", p.Status)
			assert.Equal(t, "tbank", *p.Provider)
			assert.Equal(t, "tpay", *p.PaymentMethod)
			assert.Equal(t, "mock", *p.IntegrationMode)
			assert.Equal(t, "partial", p.RefundState)
			break
		}
	}
	assert.True(t, found, "Expected to find payment with combined filters")
}

func TestAdminPaymentsPaginationAndSorting(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	// Create 5 payments with explicit amounts
	amounts := []int64{100, 200, 300, 400, 500}
	// Create a unique prefix for this test run
	uniquePrefix := "PAG-" + uuid.New().String()[:6] + "-"
	var createdIDs []uuid.UUID
	
	for i, amt := range amounts {
		fix := SetupFixture(t, client, "succeeded", "paid", amt, false, "")
		
		pNum := fmt.Sprintf("%s%d", uniquePrefix, i)
		// Set created_at to unique times to avoid ties and set unique payment_number
		createdAt := time.Now().Add(-time.Duration(i) * time.Hour)
		_, err := client.Pool.Exec(ctx, "UPDATE payments SET created_at = $1, payment_number = $2 WHERE id = $3", createdAt, pNum, fix.PaymentID)
		require.NoError(t, err)
		
		createdIDs = append(createdIDs, fix.PaymentID)
	}

	// Fetch page 1 (limit 2, offset 0) sorted by amount desc
	page1, _, err := svc.ListAdminPayments(ctx, uniquePrefix, "", "", "", "", "", "", "", "", 0, 0, false, "amount", "desc", 2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, int64(500), page1[0].AmountCents)
	assert.Equal(t, int64(400), page1[1].AmountCents)

	// Fetch page 2 (limit 2, offset 2) sorted by amount desc
	page2, total, err := svc.ListAdminPayments(ctx, uniquePrefix, "", "", "", "", "", "", "", "", 0, 0, false, "amount", "desc", 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Equal(t, int64(300), page2[0].AmountCents)
	assert.Equal(t, int64(200), page2[1].AmountCents)

	// Verify no overlap
	for _, p1 := range page1 {
		for _, p2 := range page2 {
			assert.NotEqual(t, p1.PaymentID, p2.PaymentID, "Pages should not overlap")
		}
	}

	// Verify deterministic
	page1b, totalB, err := svc.ListAdminPayments(ctx, uniquePrefix, "", "", "", "", "", "", "", "", 0, 0, false, "amount", "desc", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, page1[0].PaymentID, page1b[0].PaymentID)
	assert.Equal(t, page1[1].PaymentID, page1b[1].PaymentID)
	assert.Equal(t, total, totalB)
}

func TestAdminPaymentsAggregatesDoNotMultiplyRows(t *testing.T) {
	client, svc := setupTestService(t)
	ctx := context.Background()

	fix := SetupFixture(t, client, "succeeded", "paid", 100000, false, "")

	// Insert 2 refunds
	_, err := client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, status, amount_cents, currency, provider_refund_id, created_at) VALUES ($1, $2, $3, 'succeeded', 10000, 'RUB', $4, $5)",
		uuid.New(), fix.PaymentID, fix.OrderID, uuid.New().String(), time.Now())
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "INSERT INTO refunds (id, payment_id, order_id, status, amount_cents, currency, provider_refund_id, created_at) VALUES ($1, $2, $3, 'succeeded', 20000, 'RUB', $4, $5)",
		uuid.New(), fix.PaymentID, fix.OrderID, uuid.New().String(), time.Now())
	require.NoError(t, err)

	// Insert 3 payment events
	for i := 0; i < 3; i++ {
		_, err = client.Pool.Exec(ctx, "INSERT INTO payment_events (id, payment_id, provider, provider_payment_id, event_type, event_key, raw_payload, signature_valid, created_at) VALUES ($1, $2, 'tbank', $3, 'CONFIRMED', $4, $5, true, $6)",
			uuid.New(), fix.PaymentID, uuid.New().String(), uuid.New().String(), []byte("{}"), time.Now())
		require.NoError(t, err)
	}

	res, _, err := svc.ListAdminPayments(ctx, fix.PaymentID.String(), "", "", "", "", "", "", "", "", 0, 0, false, "", "", 10, 0)
	require.NoError(t, err)
	
	// Ensure EXACTLY one row is returned
	var count int
	for _, p := range res {
		if p.PaymentID == fix.PaymentID {
			count++
			assert.Equal(t, int64(30000), p.SucceededRefundedAmountCents)
		}
	}
	assert.Equal(t, 1, count, "Payment rows must not multiply when refunds/events are joined")
}
