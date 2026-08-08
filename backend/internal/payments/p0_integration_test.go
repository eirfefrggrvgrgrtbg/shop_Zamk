package payments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"


	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
)

func TestCreatePayment_UsesDatabasePaymentNumber(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)

	resp, err := svc.CreatePayment(ctx, userID, orderID, "tpay")
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	if !strings.HasPrefix(resp.PaymentNumber, "PAY-") {
		t.Errorf("expected PaymentNumber to start with PAY-, got %s", resp.PaymentNumber)
	}
	if len(resp.PaymentNumber) != 10 {
		t.Errorf("expected PaymentNumber length 10 (PAY-000000), got %d (%s)", len(resp.PaymentNumber), resp.PaymentNumber)
	}
}

func TestConcurrentCreatePayment_ReturnsSingleActivePayment(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)

	var wg sync.WaitGroup
	results := make([]*CreatePaymentResponse, 10)
	errs := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = svc.CreatePayment(ctx, userID, orderID, "tpay")
		}(i)
	}
	wg.Wait()

	var successCount int
	var firstPaymentID uuid.UUID
	for i := 0; i < 10; i++ {
		if errs[i] == nil && results[i] != nil {
			successCount++
			if firstPaymentID == uuid.Nil {
				firstPaymentID = results[i].PaymentID
			} else if firstPaymentID != results[i].PaymentID {
				t.Errorf("got different payment IDs: %v and %v", firstPaymentID, results[i].PaymentID)
			}
		}
	}

	if successCount != 10 {
		t.Errorf("expected all 10 to succeed and return the same payment, but got %d successes", successCount)
	}
}

func TestDuplicateWebhook_IsIdempotent(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)

	resp, _ := svc.CreatePayment(ctx, userID, orderID, "tpay")

	var providerPID string
	_ = client.Pool.QueryRow(ctx, "SELECT provider_payment_id FROM payments WHERE id = $1", resp.PaymentID).Scan(&providerPID)

	// Send duplicate with different JSON key order but same values
	payload1 := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"CONFIRMED"}`, providerPID, orderID.String()))
	payload2 := []byte(fmt.Sprintf(`{"Status":"CONFIRMED","Amount":1000,"OrderId":"%s","PaymentId":%s}`, orderID.String(), providerPID))

	err1 := svc.HandleWebhook(ctx, nil, payload1)
	if err1 != nil {
		t.Fatalf("expected no error on first webhook, got %v", err1)
	}

	err2 := svc.HandleWebhook(ctx, nil, payload2)
	if err2 != nil && !errors.Is(err2, ErrPaymentAlreadyProcessed) {
		t.Errorf("expected ErrPaymentAlreadyProcessed on duplicate webhook, got %v", err2)
	}

	// Verify only ONE event is in DB despite different JSON key order
	var count int
	_ = client.Pool.QueryRow(ctx, "SELECT count(*) FROM payment_events WHERE payment_id = $1", resp.PaymentID).Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 event, got %d", count)
	}
}

func TestAuthorizedThenConfirmed_AreDistinctEvents(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)

	resp, _ := svc.CreatePayment(ctx, userID, orderID, "tpay")

	var providerPID string
	_ = client.Pool.QueryRow(ctx, "SELECT provider_payment_id FROM payments WHERE id = $1", resp.PaymentID).Scan(&providerPID)

	payloadAuth := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"AUTHORIZED"}`, providerPID, orderID.String()))
	payloadConf := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"CONFIRMED"}`, providerPID, orderID.String()))

	if err := svc.HandleWebhook(ctx, nil, payloadAuth); err != nil {
		t.Fatalf("failed AUTHORIZED: %v", err)
	}
	
	p, _ := svc.repo.GetPaymentByID(ctx, resp.PaymentID)
	if p.Status != "succeeded" {
		t.Errorf("expected payment to be succeeded after AUTHORIZED, got %s", p.Status)
	}

	if err := svc.HandleWebhook(ctx, nil, payloadConf); err != nil {
		t.Fatalf("failed CONFIRMED: %v", err)
	}

	var count int
	_ = client.Pool.QueryRow(ctx, "SELECT count(*) FROM payment_events WHERE payment_id = $1", resp.PaymentID).Scan(&count)
	if count != 2 {
		t.Errorf("expected exactly 2 distinct events (AUTHORIZED and CONFIRMED), got %d", count)
	}
}

func TestLateAuthorized_DoesNotRegressConfirmedPayment(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)

	resp, _ := svc.CreatePayment(ctx, userID, orderID, "tpay")
	var providerPID string
	_ = client.Pool.QueryRow(ctx, "SELECT provider_payment_id FROM payments WHERE id = $1", resp.PaymentID).Scan(&providerPID)

	payloadConf := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"CONFIRMED"}`, providerPID, orderID.String()))
	payloadAuth := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"AUTHORIZED"}`, providerPID, orderID.String()))

	_ = svc.HandleWebhook(ctx, nil, payloadConf)
	
	// Send late AUTHORIZED
	err := svc.HandleWebhook(ctx, nil, payloadAuth)
	if err != nil {
		t.Errorf("expected nil for late AUTHORIZED, got %v", err)
	}

	p, _ := svc.repo.GetPaymentByID(ctx, resp.PaymentID)
	if p.Status != "succeeded" {
		t.Errorf("payment regressed from succeeded: %s", p.Status)
	}

	var count int
	_ = client.Pool.QueryRow(ctx, "SELECT count(*) FROM payment_events WHERE payment_id = $1", resp.PaymentID).Scan(&count)
	if count != 2 {
		t.Errorf("expected exactly 2 distinct events, got %d", count)
	}
}

func TestWebhookDoesNotRegressTerminalPayment(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)
	resp, _ := svc.CreatePayment(ctx, userID, orderID, "tpay")
	var providerPID string
	_ = client.Pool.QueryRow(ctx, "SELECT provider_payment_id FROM payments WHERE id = $1", resp.PaymentID).Scan(&providerPID)

	// Make it terminal
	payloadConf := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"CONFIRMED"}`, providerPID, orderID.String()))
	_ = svc.HandleWebhook(ctx, nil, payloadConf)

	statuses := []string{"NEW", "PENDING", "FAILED", "CANCELED"}
	for _, status := range statuses {
		payload := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"%s"}`, providerPID, orderID.String(), status))
		_ = svc.HandleWebhook(ctx, nil, payload)
		
		p, _ := svc.repo.GetPaymentByID(ctx, resp.PaymentID)
		if p.Status != "succeeded" {
			t.Errorf("terminal payment regressed by %s to %s", status, p.Status)
		}
	}
}

func TestInvalidWebhookHandler_ReturnsForbiddenWithoutOK(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)
	resp, _ := svc.CreatePayment(ctx, userID, orderID, "tpay")

	// strict provider
	strictProvider := NewTBankProvider("REAL_TERM", "REAL_PASS", "", "", "", true, "O", "hosted_form")
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	svcStrict := NewService(svc.repo, svc.ordersRepo, svc.inventorySvc, strictProvider, client, svc.notifSvc, cfg)

	h := NewHandler(svcStrict, "test")

	payload := []byte(fmt.Sprintf(`{"PaymentId":99999,"OrderId":"%s","Amount":1000,"Status":"CONFIRMED","Token":"wrong"}`, orderID.String()))

	req := httptest.NewRequest("POST", "/webhooks/tbank", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleTBankWebhook(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusForbidden {
		t.Errorf("expected HTTP 400 or 403 for invalid signature, got %d", res.StatusCode)
	}
	
	if w.Body.String() == "OK" {
		t.Errorf("expected response body NOT to be OK for invalid signature")
	}

	p, _ := svc.repo.GetPaymentByID(ctx, resp.PaymentID)
	if p.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", p.Status)
	}
}

func TestAdminCannotSetOrderPaidWithoutPaymentConfirmation(t *testing.T) {
	client, _, ordersRepo := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	_, orderID := createTestUserAndOrder(t, client.Pool, 1000)

	// Create orders service and verify we cannot set it to paid manually
	ordersSvc := orders.NewService(ordersRepo, nil, nil, client, nil)

	adminID := uuid.New()

	err := ordersSvc.UpdateOrderStatus(ctx, adminID, orderID, orders.UpdateOrderStatusRequest{
		Status: "paid",
	})

	if err != orders.ErrManualPaidNotAllowed {
		t.Errorf("expected ErrManualPaidNotAllowed, got %v", err)
	}
}

func TestEventKey_IncludesReceiptAndData(t *testing.T) {
	provider := NewTBankProvider("STUB", "STUB", "", "", "", true, "O", "hosted_form")
	ctx := context.Background()

	// 1. Same payload, different order -> same key
	p1 := []byte(`{"PaymentId":123,"Amount":100,"Status":"CONFIRMED","DATA":{"foo":"bar"},"Receipt":{"Email":"a@a.com"},"Token":"aaa"}`)
	p2 := []byte(`{"Token":"bbb","Status":"CONFIRMED","Receipt":{"Email":"a@a.com"},"DATA":{"foo":"bar"},"Amount":100,"PaymentId":123}`)
	
	e1, _ := provider.ParseWebhook(ctx, p1)
	e2, _ := provider.ParseWebhook(ctx, p2)
	if e1.EventKey != e2.EventKey {
		t.Errorf("expected same key for different order, got %s and %s", e1.EventKey, e2.EventKey)
	}

	// 2. Different DATA -> different key
	p3 := []byte(`{"PaymentId":123,"Amount":100,"Status":"CONFIRMED","DATA":{"foo":"baz"},"Receipt":{"Email":"a@a.com"},"Token":"aaa"}`)
	e3, _ := provider.ParseWebhook(ctx, p3)
	if e1.EventKey == e3.EventKey {
		t.Errorf("expected different key for different DATA")
	}

	// 3. Different Receipt -> different key
	p4 := []byte(`{"PaymentId":123,"Amount":100,"Status":"CONFIRMED","DATA":{"foo":"bar"},"Receipt":{"Email":"b@b.com"},"Token":"aaa"}`)
	e4, _ := provider.ParseWebhook(ctx, p4)
	if e1.EventKey == e4.EventKey {
		t.Errorf("expected different key for different Receipt")
	}
	
	// Check that Token is removed from raw payload
	if bytes.Contains(e1.RawPayload, []byte("Token")) {
		t.Errorf("Token was not removed from RawPayload")
	}
}

func TestPaymentEvent_PreservesProviderEventType(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 1000)

	resp, _ := svc.CreatePayment(ctx, userID, orderID, "tpay")
	var providerPID string
	_ = client.Pool.QueryRow(ctx, "SELECT provider_payment_id FROM payments WHERE id = $1", resp.PaymentID).Scan(&providerPID)

	payloadAuth := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"AUTHORIZED"}`, providerPID, orderID.String()))
	payloadConf := []byte(fmt.Sprintf(`{"PaymentId":%s,"OrderId":"%s","Amount":1000,"Status":"CONFIRMED"}`, providerPID, orderID.String()))

	_ = svc.HandleWebhook(ctx, nil, payloadAuth)
	_ = svc.HandleWebhook(ctx, nil, payloadConf)

	rows, _ := client.Pool.Query(ctx, "SELECT event_type FROM payment_events WHERE payment_id = $1 ORDER BY created_at", resp.PaymentID)
	defer rows.Close()

	var types []string
	for rows.Next() {
		var et string
		_ = rows.Scan(&et)
		types = append(types, et)
	}

	if len(types) != 2 || types[0] != "AUTHORIZED" || types[1] != "CONFIRMED" {
		t.Errorf("expected AUTHORIZED and CONFIRMED event_types, got %v", types)
	}
}
