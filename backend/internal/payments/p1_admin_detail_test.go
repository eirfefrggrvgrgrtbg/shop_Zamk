package payments

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminPaymentDetail(t *testing.T) {
	ctx := context.Background()
	client, svc, _ := setupTestDB(t)
	if client == nil {
		t.Skip("skipping test; db not available")
	}
	defer client.Close()

	_, orderID := createTestUserAndOrder(t, client.Pool, 10000)

	// Payment 1: Failed
	p1ID := uuid.New()
	_, err := client.Pool.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, payment_method, amount_cents, currency, payment_number, created_at, updated_at, failed_at, idempotency_key) 
		VALUES ($1, $2, 'failed', 'tbank', 'sbp', 10000, 'RUB', $3, now() - interval '2 hours', now(), now() - interval '1 hour', $4)`, p1ID, orderID, "PAY-"+uuid.New().String()[:6], uuid.New().String())
	require.NoError(t, err)

	// Payment 2: Succeeded
	p2ID := uuid.New()
	p2Num := "PAY-" + uuid.New().String()[:6]
	_, err = client.Pool.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, payment_method, amount_cents, currency, payment_number, created_at, updated_at, paid_at, idempotency_key) 
		VALUES ($1, $2, 'succeeded', 'tbank', 'card', 10000, 'RUB', $3, now() - interval '1 hour', now(), now(), $4)`, p2ID, orderID, p2Num, uuid.New().String())
	require.NoError(t, err)

	// Refunds on Payment 2 (one succeeded, one pending, one failed)
	_, err = client.Pool.Exec(ctx, `INSERT INTO refunds (id, order_id, payment_id, status, amount_cents, created_at, updated_at) VALUES (gen_random_uuid(), $1, $2, 'succeeded', 2000, now(), now())`, orderID, p2ID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, `INSERT INTO refunds (id, order_id, payment_id, status, amount_cents, created_at, updated_at) VALUES (gen_random_uuid(), $1, $2, 'pending', 1000, now(), now())`, orderID, p2ID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, `INSERT INTO refunds (id, order_id, payment_id, status, amount_cents, created_at, updated_at) VALUES (gen_random_uuid(), $1, $2, 'failed', 500, now(), now())`, orderID, p2ID)
	require.NoError(t, err)

	// Events on Payment 2
	_, err = client.Pool.Exec(ctx, `INSERT INTO payment_events (id, payment_id, provider, event_type, signature_valid, raw_payload, created_at, event_key) VALUES (gen_random_uuid(), $1, 'tbank', 'AUTHORIZED', true, '{"Status":"AUTHORIZED","Amount":10000}', now(), $2)`, p2ID, uuid.New().String())
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, `INSERT INTO payment_events (id, payment_id, provider, event_type, signature_valid, raw_payload, created_at, event_key) VALUES (gen_random_uuid(), $1, 'tbank', 'CONFIRMED', false, '{"Status":"CONFIRMED"}', now(), $2)`, p2ID, uuid.New().String())
	require.NoError(t, err)

	detail, err := svc.GetAdminPaymentDetail(ctx, p2ID)
	require.NoError(t, err)

	// Verify Payment
	assert.Equal(t, p2ID, detail.Payment.PaymentID)
	assert.Equal(t, p2Num, detail.Payment.PaymentNumber)
	assert.Equal(t, "succeeded", detail.Payment.Status)
	assert.Equal(t, 2, detail.Payment.AttemptsCount)
	assert.Equal(t, 2, detail.Payment.AttemptNumber) // second attempt

	// Verify Refunds summary
	assert.Equal(t, int64(2000), detail.Payment.SucceededRefundedAmountCents)
	assert.Equal(t, int64(1000), detail.Payment.PendingRefundAmountCents)
	
	// Failed refund is not in pending or succeeded
	
	// Verify Refunds list
	assert.Len(t, detail.Refunds, 3)

	// Verify Attempts list
	assert.Len(t, detail.Attempts, 2)
	assert.Equal(t, 1, detail.Attempts[0].AttemptNumber)
	assert.Equal(t, "failed", detail.Attempts[0].Status)
	assert.Equal(t, 2, detail.Attempts[1].AttemptNumber)
	assert.Equal(t, "succeeded", detail.Attempts[1].Status)

	// Verify Events list
	assert.Len(t, detail.ProviderEvents, 2)
	assert.False(t, detail.ProviderEvents[0].SignatureValid) // The latest one
	assert.True(t, detail.ProviderEvents[1].SignatureValid)

	// Verify Problems
	hasOrderNotPaid := false
	hasInvalidWebhook := false
	hasUnprocessedWebhook := false
	for _, p := range detail.Problems {
		if p.Code == "SUCCEEDED_PAYMENT_ORDER_NOT_PAID" {
			hasOrderNotPaid = true
		}
		if p.Code == "INVALID_WEBHOOK_SIGNATURE" {
			hasInvalidWebhook = true
		}
		if p.Code == "UNPROCESSED_WEBHOOK" {
			hasUnprocessedWebhook = true
		}
	}
	assert.True(t, hasOrderNotPaid, "Missing SUCCEEDED_PAYMENT_ORDER_NOT_PAID problem")
	assert.True(t, hasInvalidWebhook, "Missing INVALID_WEBHOOK_SIGNATURE problem")
	assert.True(t, hasUnprocessedWebhook, "Missing UNPROCESSED_WEBHOOK problem")
}

func TestAdminPaymentDetail_DoesNotLeakSensitivePayload(t *testing.T) {
	ctx := context.Background()
	client, svc, _ := setupTestDB(t)
	if client == nil {
		t.Skip("skipping test; db not available")
	}
	defer client.Close()

	_, orderID := createTestUserAndOrder(t, client.Pool, 10000)

	pID := uuid.New()
	pSecNum := "PAY-" + uuid.New().String()[:6]
	_, err := client.Pool.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, payment_method, amount_cents, currency, payment_number, created_at, updated_at, idempotency_key) 
		VALUES ($1, $2, 'pending', 'tbank', 'sbp', 10000, 'RUB', $3, now(), now(), $4)`, pID, orderID, pSecNum, uuid.New().String())
	require.NoError(t, err)

	// Insert an event with highly sensitive information
	sensitivePayload := `{
		"PaymentId": "test_id",
		"OrderId": "test_order",
		"Status": "CONFIRMED",
		"Success": true,
		"Amount": 10000,
		"Token": "SECRET_TOKEN_XYZ",
		"Receipt": {"email": "test@test.com", "phone": "+1234567"},
		"DATA": {"internal": "info"}
	}`
	
	_, err = client.Pool.Exec(ctx, `INSERT INTO payment_events (id, payment_id, provider, event_type, signature_valid, raw_payload, created_at, event_key) VALUES (gen_random_uuid(), $1, 'tbank', 'CONFIRMED', true, $2, now(), $3)`, pID, sensitivePayload, uuid.New().String())
	require.NoError(t, err)

	detail, err := svc.GetAdminPaymentDetail(ctx, pID)
	require.NoError(t, err)

	require.Len(t, detail.ProviderEvents, 1)
	event := detail.ProviderEvents[0]
	
	// Test the parsed summary object directly
	_, hasToken := event.SafePayloadSummary["Token"]
	assert.False(t, hasToken, "Token leaked in safe payload summary")
	
	_, hasReceipt := event.SafePayloadSummary["Receipt"]
	assert.False(t, hasReceipt, "Receipt leaked in safe payload summary")

	_, hasData := event.SafePayloadSummary["DATA"]
	assert.False(t, hasData, "DATA leaked in safe payload summary")
	
	_, hasEmail := event.SafePayloadSummary["email"]
	assert.False(t, hasEmail, "email leaked in safe payload summary")
	
	_, hasPhone := event.SafePayloadSummary["phone"]
	assert.False(t, hasPhone, "phone leaked in safe payload summary")

	// Ensure the allowed fields are still there
	assert.Equal(t, "test_id", event.SafePayloadSummary["PaymentId"])
	assert.Equal(t, "CONFIRMED", event.SafePayloadSummary["Status"])

	// Test the serialized JSON (how HTTP actually outputs it)
	jsonBytes, err := json.Marshal(detail)
	require.NoError(t, err)
	var res map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &res); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	t.Logf("REAL HTTP JSON:\n%s", string(jsonBytes))
	jsonString := string(jsonBytes)
	
	assert.NotContains(t, jsonString, "SECRET_TOKEN_XYZ")
	assert.NotContains(t, jsonString, "test@test.com")
	assert.NotContains(t, jsonString, "+1234567")
	assert.NotContains(t, jsonString, "\"Token\"")
	assert.NotContains(t, jsonString, "\"Receipt\"")
	assert.NotContains(t, jsonString, "\"DATA\"")
}

