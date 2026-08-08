package fulfillment_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

func discTestDBURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://zamk:zamk_password@127.0.0.1:5433/zamk?sslmode=disable"
}

// seedDiscFulfillment создаёт изолированный набор данных для тестов расхождения.
// Возвращает fulfillmentID, sellerID (sellers.id), staffID (users.id).
func seedDiscFulfillment(ctx context.Context, db *postgres.Client) (fulfillmentID, sellerID, staffID uuid.UUID, err error) {
	staffID = uuid.New()
	sellerID = uuid.New()
	orderID := uuid.New()
	fulfillmentID = uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	orderItemID := uuid.New()

	suffix := fulfillmentID.String()[:8]

	if _, err = db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name, first_name, last_name)
		VALUES ($1, $2, 'hash', 'admin', 'Disc Staff', 'Disc', 'Staff')
		ON CONFLICT (id) DO NOTHING
	`, staffID, "discstaff-"+suffix+"@zamk.local"); err != nil {
		return
	}
	if _, err = db.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (id) DO NOTHING
	`, sellerID,
		"Disc Brand "+suffix,
		"disc-brand-"+suffix,
		"disc-"+suffix+"@zamk.local",
	); err != nil {
		return
	}
	if _, err = db.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, currency, status)
		VALUES ($1, $2, 'Disc Product', $3, 10000, 'RUB', 'published')
	`, productID, sellerID, "disc-prod-"+suffix); err != nil {
		return
	}
	if _, err = db.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode, price_cents, is_active)
		VALUES ($1, $2, $3, '4609999900001', 10000, true)
	`, variantID, productID, "DISC-SKU-"+suffix); err != nil {
		return
	}
	if _, err = db.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'paid', 10000, 'RUB', 'Disc Cust', '+79990000002', 'c@t.com', 'Disc Addr')
	`, orderID, staffID); err != nil {
		return
	}
	if _, err = db.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, receiving_code)
		VALUES ($1, $2, $3, 'packed', 10000, 1500, 8500, $4)
	`, fulfillmentID, orderID, sellerID, "FUL-DISC-"+suffix); err != nil {
		return
	}
	if _, err = db.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Disc Product', $7, $8, 10000, 2, 20000)
	`, orderItemID, orderID, fulfillmentID, productID, variantID, sellerID,
		"disc-prod-"+suffix, "DISC-SKU-"+suffix,
	); err != nil {
		return
	}
	return
}

// ---------------------------------------------------------------------------
// TestRecordDiscrepancy_PersistsResultWithoutShipment
// ---------------------------------------------------------------------------
// Проверяет полный happy-path RecordDiscrepancy:
//  1. fulfillment.status → discrepancy
//  2. receiving_session.status → discrepancy, completed_at заполнен
//  3. discrepancy_reason и discrepancy_comment сохранены
//  4. Shipment НЕ создан
//  5. Уведомление создано с правильными FK-полями:
//     recipient_seller_id = fulfillment.seller_id
//     recipient_user_id   = NULL
//     recipient_kind      = seller
func TestRecordDiscrepancy_PersistsResultWithoutShipment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := postgres.NewClient(ctx, discTestDBURL())
	require.NoError(t, err, "connect to test db")
	defer db.Close()

	fulfillmentID, sellerID, staffID, err := seedDiscFulfillment(ctx, db)
	require.NoError(t, err, "seed test data")

	// Real notification service (writes to the same test DB)
	notifRepo := notifications.NewRepository(db)
	usersRepo := users.NewRepository(db.Pool)
	notifSvc := notifications.NewService(notifRepo, usersRepo, nil)

	repo := fulfillment.NewRepository(db.Pool)
	ordersRepo := orders.NewRepository(db.Pool)
	svc := fulfillment.NewService(repo, ordersRepo, db, &receivingTestMockPayoutsService{}, notifSvc)

	// Start active receiving session
	sess, err := svc.StartReceivingSession(ctx, &staffID, fulfillmentID)
	require.NoError(t, err, "start receiving session")
	require.NotNil(t, sess)
	require.Equal(t, "active", sess.Status)

	// Partial scan (1 of 2 items) → leaves a discrepancy
	_, err = svc.ScanReceivingItem(ctx, fulfillmentID, fulfillment.ScanItemRequest{
		Barcode:         "4609999900001",
		ExpectedVersion: sess.Version,
		IdempotencyKey:  "disc-scan-" + fulfillmentID.String(),
	})
	require.NoError(t, err, "scan one item")

	// ==== CALL UNDER TEST ====
	err = svc.RecordDiscrepancy(ctx, staffID, fulfillmentID, fulfillment.RecordDiscrepancyRequest{
		SessionID: sess.ID.String(),
		Reason:    "shortage",
		Comment:   "Расхождение состава при сканировании на хабе",
	})
	require.NoError(t, err, "RecordDiscrepancy must commit without error")

	// --- Assert fulfillment state ---
	var gotStatus, gotReason, gotComment string
	var gotDiscAt *time.Time
	err = db.Pool.QueryRow(ctx, `
		SELECT status,
		       COALESCE(discrepancy_reason, ''),
		       COALESCE(discrepancy_comment, ''),
		       discrepancy_at
		FROM order_fulfillments
		WHERE id = $1
	`, fulfillmentID).Scan(&gotStatus, &gotReason, &gotComment, &gotDiscAt)
	require.NoError(t, err, "read fulfillment row")
	assert.Equal(t, "discrepancy", gotStatus, "fulfillment.status must be discrepancy")
	assert.Equal(t, "shortage", gotReason, "discrepancy_reason")
	assert.Equal(t, "Расхождение состава при сканировании на хабе", gotComment, "discrepancy_comment")
	assert.NotNil(t, gotDiscAt, "discrepancy_at must be set")

	// --- Assert receiving session state ---
	var sessStatus string
	var sessCompletedAt *time.Time
	err = db.Pool.QueryRow(ctx, `
		SELECT status, completed_at
		FROM fulfillment_receiving_sessions
		WHERE id = $1
	`, sess.ID).Scan(&sessStatus, &sessCompletedAt)
	require.NoError(t, err, "read session row")
	assert.Equal(t, "discrepancy", sessStatus, "session.status must be discrepancy")
	assert.NotNil(t, sessCompletedAt, "session.completed_at must be set")

	// --- Assert no shipment was created ---
	var shipmentCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM shipments WHERE fulfillment_id = $1
	`, fulfillmentID).Scan(&shipmentCount)
	require.NoError(t, err, "count shipments")
	assert.Equal(t, 0, shipmentCount, "no shipment must be created for a discrepancy")

	// --- Assert seller notification FK correctness in DB ---
	var (
		dbRecipientUserID   *uuid.UUID
		dbRecipientSellerID *uuid.UUID
		dbRecipientKind     string
		dbEntityType        string
		dbEntityID          uuid.UUID
	)
	err = db.Pool.QueryRow(ctx, `
		SELECT recipient_user_id, recipient_seller_id, recipient_kind, entity_type, entity_id
		FROM notifications
		WHERE entity_id = $1 AND type = 'fulfillment_discrepancy'
		ORDER BY created_at DESC
		LIMIT 1
	`, fulfillmentID).Scan(
		&dbRecipientUserID,
		&dbRecipientSellerID,
		&dbRecipientKind,
		&dbEntityType,
		&dbEntityID,
	)
	require.NoError(t, err, "notification row must exist in DB")
	assert.Nil(t, dbRecipientUserID, "recipient_user_id must be NULL (not sellers.id)")
	require.NotNil(t, dbRecipientSellerID, "recipient_seller_id must be set")
	assert.Equal(t, sellerID, *dbRecipientSellerID, "recipient_seller_id must equal fulfillment.seller_id")
	assert.Equal(t, notifications.RecipientKindSeller, dbRecipientKind, "recipient_kind")
	assert.Equal(t, "fulfillment", dbEntityType, "entity_type")
	assert.Equal(t, fulfillmentID, dbEntityID, "entity_id")
}
