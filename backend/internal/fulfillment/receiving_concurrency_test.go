package fulfillment_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type receivingTestMockPayoutsService struct{}

func (m *receivingTestMockPayoutsService) CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error {
	return nil
}

func seedTestFulfillment(ctx context.Context, db *postgres.Client) (uuid.UUID, uuid.UUID, string, error) {
	staffID := uuid.MustParse("99999999-9999-4999-8999-999999999999")
	sellerID := uuid.MustParse("11111111-1111-4111-8111-000000000001")
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()

	orderItemID := uuid.New()

	// Cleanup existing test artifacts if any
	_, _ = db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, staffID)
	_, _ = db.Pool.Exec(ctx, `DELETE FROM sellers WHERE id = $1`, sellerID)

	// User & Seller
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name, first_name, last_name)
		VALUES ($1, 'teststaff@zamk.local', 'hash', 'admin', 'Test Staff', 'Test', 'Staff')
		ON CONFLICT (id) DO NOTHING
	`, staffID)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'Test Brand', $2, 'testbrand@zamk.local', 'active')
		ON CONFLICT (id) DO NOTHING
	`, sellerID, "test-brand-"+uuid.New().String()[:8])
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	// Product & Variant
	productSlug := "parallel-test-" + uuid.New().String()[:8]
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, currency, status)
		VALUES ($1, $2, 'Parallel Test Product', $3, 10000, 'RUB', 'published')
		ON CONFLICT (id) DO NOTHING
	`, productID, sellerID, productSlug)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode, price_cents)
		VALUES ($1, $2, 'PARALLEL-SKU-01', $3, 10000)
		ON CONFLICT (id) DO NOTHING
	`, variantID, productID, variantID.String()[:13])
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	// Order & Fulfillment
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'paid', 10000, 'RUB', 'Test Customer', '+79990000000', 'cust@test.com', 'Test Address')
	`, orderID, staffID)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, receiving_code)
		VALUES ($1, $2, $3, 'packed', 10000, 1500, 8500, $4)
	`, fulfillmentID, orderID, sellerID, "FUL-PARALLEL-"+fulfillmentID.String()[:8])
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Parallel Test Product', 'parallel-test', 'PARALLEL-SKU-01', 10000, 1, 10000)
	`, orderItemID, orderID, fulfillmentID, productID, variantID, sellerID)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	return fulfillmentID, staffID, variantID.String()[:13], nil
}

func TestParallelConfirmReceiving_CreatesExactlyOneShipment(t *testing.T) {
	dbURL := testutil.GetTestDatabaseURL()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err, "Failed to connect to postgres test db")
	defer db.Close()

	testutil.AssertTestDatabase(t, db.Pool)

	fulfillmentID, staffID, barcode, err := seedTestFulfillment(ctx, db)
	require.NoError(t, err)

	repo := fulfillment.NewRepository(db.Pool)
	ordersRepo := orders.NewRepository(db.Pool)
	svc := fulfillment.NewService(repo, ordersRepo, db, &receivingTestMockPayoutsService{}, nil)

	// Ensure active receiving session exists
	sess, err := svc.StartReceivingSession(ctx, &staffID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, sess)

	// Scan item to 100% match
	scannedSess, err := svc.ScanReceivingItem(ctx, fulfillmentID, fulfillment.ScanItemRequest{
		Barcode:         barcode,
		ExpectedVersion: sess.Version,
		IdempotencyKey:  "test-idemp-001",
	})
	require.NoError(t, err)
	require.True(t, scannedSess.CanConfirm, "Session must be ready to confirm")

	// Run 2 parallel ConfirmReceiving calls
	var wg sync.WaitGroup
	wg.Add(2)

	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func(index int) {
			defer wg.Done()
			_, err := svc.ConfirmReceiving(ctx, staffID, fulfillmentID, fulfillment.ConfirmReceivingRequest{
				SessionID:       scannedSess.ID.String(),
				ExpectedVersion: scannedSess.Version,
				IdempotencyKey:  "confirm-idemp-key",
			})
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	var successCount, errCount int
	for err := range results {
		if err == nil {
			successCount++
		} else {
			t.Logf("Confirm error result: %v", err)
			errCount++
		}
	}

	// Assert exactly 1 call succeeded
	assert.Equal(t, 1, successCount, "Exactly 1 confirm call should succeed")
	assert.Equal(t, 1, errCount, "Exactly 1 confirm call should fail due to concurrency lock / status change")

	// Assert DB count of shipments for this fulfillment ID is exactly 1
	var shipmentCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM shipments WHERE fulfillment_id = $1", fulfillmentID).Scan(&shipmentCount)
	require.NoError(t, err)
	assert.Equal(t, 1, shipmentCount, "DB must contain EXACTLY 1 shipment for this fulfillment")
}
