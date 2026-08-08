package payments

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

func setupTestDB(t *testing.T) (*postgres.Client, *Service, *orders.Repository) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping DB integration test")
	}

	ctx := context.Background()
	client, err := postgres.NewClient(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	paymentsRepo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	invRepo := inventory.NewRepository(client.Pool)
	invSvc := inventory.NewService(invRepo, nil, client)

	tbankProvider := NewTBankProvider("STUB", "STUB", "", "", "", true, "O", "mock")
	notifRepo := notifications.NewRepository(client)
	cfg := &config.Config{
		App: config.AppConfig{PaymentStuckPendingMinutes: 30},
	}
	notifSvc := notifications.NewService(notifRepo, nil, nil)

	svc := NewService(paymentsRepo, ordersRepo, invSvc, tbankProvider, client, notifSvc, cfg)

	return client, svc, ordersRepo
}

func createTestUserAndOrder(t *testing.T, pool *pgxpool.Pool, amountCents int64) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	userID := uuid.New()
	email := "test-" + userID.String() + "@example.com"

	_, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'Test User', 'hash', 'customer')`, userID, email)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	orderID := uuid.New()
	orderNumber := "ORD-" + orderID.String()[:8]
	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'awaiting_payment', $4, 'RUB', 'Test Address', 'Delivery', 0, 'Test User', $5, '+79000000000')
	`, orderID, userID, orderNumber, amountCents, email)
	if err != nil {
		t.Fatalf("failed to insert test order: %v", err)
	}

	sellerID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Test Seller', $2, 'seller@example.com', 'active')`, sellerID, "slug-"+sellerID.String())
	if err != nil {
		t.Fatalf("failed to insert test seller: %v", err)
	}

	fulfillmentID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'awaiting_payment', $4, 1500, $4)
	`, fulfillmentID, orderID, sellerID, amountCents)
	if err != nil {
		t.Fatalf("failed to insert test fulfillment: %v", err)
	}

	return userID, orderID
}

func TestMockPaymentCreation_RemainsPending(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 250000)

	resp, err := svc.CreatePayment(ctx, userID, orderID, "tpay")
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	// 1. Assert Response
	if resp.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", resp.Status)
	}
	if resp.IntegrationMode != "mock" {
		t.Errorf("expected integrationMode 'mock', got %s", resp.IntegrationMode)
	}
	if resp.PaymentMethod != "tpay" {
		t.Errorf("expected paymentMethod 'tpay', got %s", resp.PaymentMethod)
	}
	if resp.AmountCents != 250000 {
		t.Errorf("expected amountCents 250000, got %d", resp.AmountCents)
	}
	expectedURL := "/dev/payments/mock/" + resp.PaymentID.String()
	if resp.PaymentURL != expectedURL {
		t.Errorf("expected paymentURL %s, got %s", expectedURL, resp.PaymentURL)
	}

	// 2. Assert DB Row for Payment is pending
	var dbStatus string
	var dbAmount int64
	err = client.Pool.QueryRow(ctx, `SELECT status, amount_cents FROM payments WHERE id = $1`, resp.PaymentID).Scan(&dbStatus, &dbAmount)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}
	if dbStatus != "pending" {
		t.Errorf("expected payment DB status 'pending', got %s", dbStatus)
	}
	if dbAmount != 250000 {
		t.Errorf("expected payment DB amount 250000, got %d", dbAmount)
	}

	// 3. Assert DB Row for Order is awaiting_payment
	var orderStatus string
	err = client.Pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("failed to query order row: %v", err)
	}
	if orderStatus != "awaiting_payment" {
		t.Errorf("expected order DB status 'awaiting_payment', got %s", orderStatus)
	}
}

func TestMockConfirmation_SetsPaymentSucceededAndOrderPaid(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 300000)

	resp, err := svc.CreatePayment(ctx, userID, orderID, "tpay")
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	// Confirm mock payment
	err = svc.ProcessMockPaymentAction(ctx, resp.PaymentID, "confirm")
	if err != nil {
		t.Fatalf("failed to confirm mock payment: %v", err)
	}

	// Assert Payment is succeeded
	var paymentStatus string
	err = client.Pool.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, resp.PaymentID).Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}
	if paymentStatus != "succeeded" {
		t.Errorf("expected payment status 'succeeded' after confirmation, got %s", paymentStatus)
	}

	// Assert Order is paid
	var orderStatus string
	err = client.Pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("failed to query order row: %v", err)
	}
	if orderStatus != "paid" {
		t.Errorf("expected order status 'paid' after confirmation, got %s", orderStatus)
	}

	// Assert Payment Event logged
	var eventType string
	err = client.Pool.QueryRow(ctx, `SELECT event_type FROM payment_events WHERE payment_id = $1`, resp.PaymentID).Scan(&eventType)
	if err != nil {
		t.Fatalf("failed to query payment_events row: %v", err)
	}
	if eventType != "mock_confirm" {
		t.Errorf("expected event_type 'mock_confirm', got %s", eventType)
	}
}

func TestMockFailure_DoesNotSetOrderPaid(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 400000)

	resp, err := svc.CreatePayment(ctx, userID, orderID, "tpay")
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	// Reject mock payment
	err = svc.ProcessMockPaymentAction(ctx, resp.PaymentID, "reject")
	if err != nil {
		t.Fatalf("failed to reject mock payment: %v", err)
	}

	// Assert Payment is failed
	var paymentStatus string
	err = client.Pool.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, resp.PaymentID).Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}
	if paymentStatus != "failed" {
		t.Errorf("expected payment status 'failed' after rejection, got %s", paymentStatus)
	}

	// Assert Order remains awaiting_payment
	var orderStatus string
	err = client.Pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("failed to query order row: %v", err)
	}
	if orderStatus != "awaiting_payment" {
		t.Errorf("expected order status 'awaiting_payment' after rejection, got %s", orderStatus)
	}
}

func TestPaymentAmountComesFromOrder(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	defer client.Close()

	ctx := context.Background()
	orderAmount := int64(78950)
	userID, orderID := createTestUserAndOrder(t, client.Pool, orderAmount)

	resp, err := svc.CreatePayment(ctx, userID, orderID, "tpay")
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	if resp.AmountCents != orderAmount {
		t.Errorf("expected AmountCents to be %d, got %d", orderAmount, resp.AmountCents)
	}
}

func TestDuplicateCreatePayment_ReusesActivePayment(t *testing.T) {
	client, svc, _ := setupTestDB(t)
	defer client.Close()

	ctx := context.Background()
	userID, orderID := createTestUserAndOrder(t, client.Pool, 100000)

	resp1, err := svc.CreatePayment(ctx, userID, orderID, "tpay")
	if err != nil {
		t.Fatalf("failed to create first payment: %v", err)
	}

	resp2, err := svc.CreatePayment(ctx, userID, orderID, "tpay")
	if err != nil {
		t.Fatalf("failed to call duplicate create payment: %v", err)
	}

	if resp1.PaymentID != resp2.PaymentID {
		t.Errorf("expected duplicate CreatePayment to reuse PaymentID %s, got %s", resp1.PaymentID, resp2.PaymentID)
	}
}
