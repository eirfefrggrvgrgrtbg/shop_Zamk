package payments

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func requireIsolatedTestDatabase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var dbName string
	err := pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&dbName)
	if err != nil {
		t.Fatalf("failed to determine current database: %v", err)
	}

	if !strings.Contains(dbName, "_test") {
		t.Fatalf("refusing to run destructive integration test against non-test database: %s", dbName)
	}
}

type PaymentTestFixture struct {
	UserID         uuid.UUID
	SellerID       uuid.UUID
	OrderID        uuid.UUID
	FulfillmentID  uuid.UUID
	PaymentID      uuid.UUID
	RefundIDs      []uuid.UUID
	EventIDs       []uuid.UUID
}

func setupTestService(t *testing.T) (*postgres.Client, *Service) {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping DB integration test")
	}

	ctx := context.Background()
	client, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)

	t.Cleanup(func() {
		client.Close()
	})

	requireIsolatedTestDatabase(t, client.Pool)

	repo := NewRepository(client.Pool)
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	svc := NewService(repo, orders.NewRepository(client.Pool), inventory.NewService(nil, nil, client), nil, client, notifications.NewService(nil, nil, nil), cfg)

	return client, svc
}

func SetupFixture(t *testing.T, client *postgres.Client, pStatus, oStatus string, pAmount int64, stuck bool, probCode string) PaymentTestFixture {
	t.Helper()
	ctx := context.Background()
	
	var fix PaymentTestFixture
	fix.UserID = uuid.New()
	fix.SellerID = uuid.New()
	fix.OrderID = uuid.New()
	fix.FulfillmentID = uuid.New()
	fix.PaymentID = uuid.New()

	email := "test-" + fix.UserID.String() + "@example.com"
	_, err := client.Pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'Test User', 'hash', 'customer')`, fix.UserID, email)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Test Seller', $2, 'seller@example.com', 'active')`, fix.SellerID, "slug-"+fix.SellerID.String())
	require.NoError(t, err)

	orderNumber := "ORD-" + fix.OrderID.String()[:8]
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, $4, $5, 'RUB', 'Test Address', 'Delivery', 0, 'Test User', $6, '+79000000000')
	`, fix.OrderID, fix.UserID, orderNumber, oStatus, pAmount, email)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'awaiting_payment', $4, 1500, $4)
	`, fix.FulfillmentID, fix.OrderID, fix.SellerID, pAmount)
	require.NoError(t, err)

	createdAt := time.Now()
	if stuck {
		createdAt = time.Now().Add(-2 * time.Hour)
	}

	_, err = client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, integration_mode, payment_method, created_at) VALUES ($1, $2, 'tbank', $3, $4, $5, 'RUB', $6, 'mock', 'tpay', $7)", fix.PaymentID, fix.OrderID, uuid.New().String(), pStatus, pAmount, uuid.New().String(), createdAt)
	require.NoError(t, err)

	if probCode == "INVALID_WEBHOOK_SIGNATURE" {
		evtID := uuid.New()
		fix.EventIDs = append(fix.EventIDs, evtID)
		_, err = client.Pool.Exec(ctx, "INSERT INTO payment_events (id, payment_id, provider, provider_payment_id, event_type, event_key, raw_payload, signature_valid, processed_at) VALUES ($1, $2, 'tbank', $3, 'AUTHORIZED', $4, '{}', false, now())", evtID, fix.PaymentID, uuid.New().String(), uuid.New().String())
		require.NoError(t, err)
	}
	if probCode == "UNPROCESSED_WEBHOOK" {
		evtID := uuid.New()
		fix.EventIDs = append(fix.EventIDs, evtID)
		_, err = client.Pool.Exec(ctx, "INSERT INTO payment_events (id, payment_id, provider, provider_payment_id, event_type, event_key, raw_payload, signature_valid, processed_at) VALUES ($1, $2, 'tbank', $3, 'AUTHORIZED', $4, '{}', true, NULL)", evtID, fix.PaymentID, uuid.New().String(), uuid.New().String())
		require.NoError(t, err)
	}
	if probCode == "MULTIPLE_SUCCEEDED_PAYMENTS" {
		pid2 := uuid.New()
		_, err = client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, integration_mode, payment_method, created_at) VALUES ($1, $2, 'tbank', $3, 'succeeded', $4, 'RUB', $5, 'mock', 'tpay', $6)", pid2, fix.OrderID, uuid.New().String(), pAmount, uuid.New().String(), createdAt)
		require.NoError(t, err)
		// we don't track pid2 in fix.PaymentID, but cleanup by order_id will catch it.
	}

	t.Cleanup(func() {
		if err := CleanupPaymentTestFixture(ctx, client.Pool, fix); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	})
	
	return fix
}

func CleanupPaymentTestFixture(ctx context.Context, pool *pgxpool.Pool, fix PaymentTestFixture) error {
	// Reverse FK order
	if _, err := pool.Exec(ctx, "DELETE FROM notifications WHERE recipient_user_id = $1 OR recipient_seller_id = $2", fix.UserID, fix.SellerID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM payment_events WHERE payment_id = $1", fix.PaymentID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM refunds WHERE payment_id = $1 OR order_id = $2", fix.PaymentID, fix.OrderID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM returns WHERE order_id = $1", fix.OrderID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM payments WHERE order_id = $1", fix.OrderID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM shipments WHERE fulfillment_id = $1", fix.FulfillmentID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE order_id = $1", fix.OrderID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM order_items WHERE order_id = $1", fix.OrderID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", fix.OrderID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", fix.SellerID); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", fix.UserID); err != nil {
		return err
	}
	return nil
}
