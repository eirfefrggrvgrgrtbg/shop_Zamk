package fulfillment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type deliveryHoldFixture struct {
	client     *postgres.Client
	db         *pgxpool.Pool
	repo       *fulfillment.Repository
	ordersRepo *orders.Repository
	payoutRepo *payouts.Repository
	payoutSvc  *payouts.Service
	svc        *fulfillment.Service
	sellerID   uuid.UUID
	adminID    uuid.UUID
}

func setupDeliveryHoldFixture(t *testing.T, ctx context.Context) *deliveryHoldFixture {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = testutil.GetTestDatabaseURL()
	}
	require.NotEmpty(t, dbURL, "test database URL must not be empty")

	client, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, client.Pool)

	db := client.Pool
	repo := fulfillment.NewRepository(db)
	ordersRepo := orders.NewRepository(db)
	payoutRepo := payouts.NewRepository(db)
	payoutSvc := payouts.NewService(payoutRepo, client, nil, ordersRepo, nil, nil)
	svc := fulfillment.NewService(repo, ordersRepo, client, payoutSvc, nil)

	sellerID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Hold Test Seller', $2, $3, 'active', now(), now())
	`, sellerID, uuid.New().String(), uuid.New().String()+"@seller.com")
	require.NoError(t, err)

	adminID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Admin', $2, 'hash', 'admin', 'active', now(), now())
	`, adminID, uuid.New().String()+"@admin.com")
	require.NoError(t, err)

	return &deliveryHoldFixture{
		client:     client,
		db:         db,
		repo:       repo,
		ordersRepo: ordersRepo,
		payoutRepo: payoutRepo,
		payoutSvc:  payoutSvc,
		svc:        svc,
		sellerID:   sellerID,
		adminID:    adminID,
	}
}

func (f *deliveryHoldFixture) createShippedOrderWithItem(t *testing.T, ctx context.Context, priceCents int64, qty int) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	orderID := uuid.New()
	userID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Buyer', $2, 'hash', 'customer', 'active', now(), now())
	`, userID, uuid.New().String()+"@buyer.com")
	require.NoError(t, err)

	subtotal := priceCents * int64(qty)
	_, err = f.db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'shipped', $3, 'Test Customer', '+79991234567', 'cust@example.com', 'Test Address')
	`, orderID, userID, subtotal)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'shipped', $4, 900, $5)
	`, fulfillmentID, orderID, f.sellerID, subtotal, subtotal-(subtotal*900/10000))
	require.NoError(t, err)

	catID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Test Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Test Prod', $4, $5, 'published', now(), now())`, prodID, f.sellerID, catID, uuid.New().String(), priceCents)
	require.NoError(t, err)

	variantID := uuid.New()
	barcode := fmt.Sprintf("46%011d", rand.Int63n(90000000000)+10000000000)
	sku := fmt.Sprintf("SKU-%s", uuid.New().String()[:8])
	_, err = f.db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, true, now(), now())`, variantID, prodID, sku, uuid.New().String(), barcode, priceCents)
	require.NoError(t, err)

	orderItemID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Test Prod', 'slug', $6, $7, $8, $9, $7)
	`, orderItemID, orderID, prodID, variantID, f.sellerID, priceCents, qty, subtotal, fulfillmentID)
	require.NoError(t, err)

	shipmentID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 'CDEK', 'TRK-HOLD-001', 'https://cdek.ru/trk/001', now() - interval '1 hour', now(), now())
	`, shipmentID, orderID, fulfillmentID)
	require.NoError(t, err)

	return orderID, fulfillmentID, orderItemID, shipmentID
}

// Case A & B: Integration test + exact canonical timestamp relation test
// Deliver shipment through canonical service.
// Assert:
// - shipment.status == "delivered"
// - shipment.delivered_at IS NOT NULL
// - seller_ledger_entries row exists for fulfillment
// - entry.type == "seller_earning"
// - entry.available_at IS NOT NULL
// - entry.available_at == shipment.delivered_at + 14 days
func TestDeliverShipment_AtomicHoldTransition(t *testing.T) {
	ctx := context.Background()
	f := setupDeliveryHoldFixture(t, ctx)
	defer f.client.Close()

	priceCents := int64(3000)
	qty := 1
	orderID, fulfillmentID, orderItemID, shipmentID := f.createShippedOrderWithItem(t, ctx, priceCents, qty)

	comment := "Вручено покупателю курьером"
	res, err := f.svc.DeliverShipment(ctx, f.adminID, shipmentID, fulfillment.DeliverShipmentRequest{
		Comment: &comment,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "delivered", res.ShipmentStatus)
	assert.Equal(t, "delivered", res.FulfillmentStatus)
	assert.Equal(t, "delivered", res.OrderStatus)
	assert.Equal(t, fulfillmentID, res.FulfillmentID)
	assert.Equal(t, orderID, res.OrderID)
	assert.False(t, res.DeliveredAt.IsZero())

	// 1. Verify shipments row
	var shipStatus string
	var deliveredAt time.Time
	err = f.db.QueryRow(ctx, `SELECT status, delivered_at FROM shipments WHERE id = $1`, shipmentID).Scan(&shipStatus, &deliveredAt)
	require.NoError(t, err)
	assert.Equal(t, "delivered", shipStatus)
	assert.False(t, deliveredAt.IsZero(), "shipment delivered_at must NOT be null")

	// 2. Verify seller_ledger_entries rows for this order
	type ledgerRow struct {
		id          uuid.UUID
		entryType   string
		amountCents int64
		availableAt *time.Time
	}
	var entries []ledgerRow
	rows, err := f.db.Query(ctx, `
		SELECT id, type, amount_cents, available_at
		FROM seller_ledger_entries
		WHERE order_id = $1
		ORDER BY type ASC
	`, orderID)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var r ledgerRow
		require.NoError(t, rows.Scan(&r.id, &r.entryType, &r.amountCents, &r.availableAt))
		entries = append(entries, r)
	}
	require.NoError(t, rows.Err())

	// Expected entries: sale_gross, seller_earning, zamk_commission
	require.Len(t, entries, 3, "expected 3 ledger entries (gross, commission, earning)")

	var sellerEarning *ledgerRow
	for i := range entries {
		if entries[i].entryType == "seller_earning" {
			sellerEarning = &entries[i]
		}
	}
	require.NotNil(t, sellerEarning, "seller_earning entry must exist")

	// 3. Verify non-null hold contract
	require.NotNil(t, sellerEarning.availableAt, "seller_earning.available_at MUST NOT be null after delivery")

	// 4. Verify exact canonical timestamp relation (available_at == delivered_at + 14 days)
	expectedAvailableAt := deliveredAt.AddDate(0, 0, 14)
	assert.True(t, sellerEarning.availableAt.Equal(expectedAvailableAt),
		"seller_earning.available_at (%s) must equal delivered_at + 14 days (%s)",
		sellerEarning.availableAt.Format(time.RFC3339Nano),
		expectedAvailableAt.Format(time.RFC3339Nano),
	)

	// Verify orderItem linkage on the earning entry
	var earningOrderItemID *uuid.UUID
	err = f.db.QueryRow(ctx, `SELECT order_item_id FROM seller_ledger_entries WHERE id = $1`, sellerEarning.id).Scan(&earningOrderItemID)
	require.NoError(t, err)
	require.NotNil(t, earningOrderItemID)
	assert.Equal(t, orderItemID, *earningOrderItemID)
	assert.Equal(t, int64(2730), sellerEarning.amountCents) // 3000 - 9% (270) = 2730
}

// Case A & B via HTTP handler:
// POST /api/admin/shipments/{id}/deliver
func TestDeliverShipment_Handler_HoldTransition(t *testing.T) {
	ctx := context.Background()
	f := setupDeliveryHoldFixture(t, ctx)
	defer f.client.Close()

	handler := fulfillment.NewHandler(f.svc)
	priceCents := int64(5000)
	qty := 1
	orderID, _, _, shipmentID := f.createShippedOrderWithItem(t, ctx, priceCents, qty)

	comment := "Доставлено через ПВЗ"
	body, _ := json.Marshal(fulfillment.DeliverShipmentRequest{
		Comment: &comment,
	})
	req := httptest.NewRequest("POST", "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	rCtx := chi.NewRouteContext()
	rCtx.URLParams.Add("id", shipmentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
	req = req.WithContext(context.WithValue(req.Context(), "userID", f.adminID))

	handler.DeliverShipment(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var res fulfillment.DeliveryResult
	err := json.NewDecoder(rec.Body).Decode(&res)
	require.NoError(t, err)
	assert.Equal(t, "delivered", res.ShipmentStatus)
	assert.Equal(t, "delivered", res.FulfillmentStatus)
	assert.Equal(t, "delivered", res.OrderStatus)

	// Check DB
	var deliveredAt time.Time
	err = f.db.QueryRow(ctx, `SELECT delivered_at FROM shipments WHERE id = $1`, shipmentID).Scan(&deliveredAt)
	require.NoError(t, err)

	var availableAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT available_at FROM seller_ledger_entries WHERE order_id = $1 AND type = 'seller_earning'`, orderID).Scan(&availableAt)
	require.NoError(t, err)
	require.NotNil(t, availableAt)
	assert.True(t, availableAt.Equal(deliveredAt.AddDate(0, 0, 14)))
}

// Case C: Idempotent / repeated delivery guardrails
// Attempting to redeliver already-delivered shipment must fail appropriately
// and NOT duplicate seller_earning entries or corrupt hold timestamps.
func TestDeliverShipment_IdempotentRepeatedDeliveryGuardrails(t *testing.T) {
	ctx := context.Background()
	f := setupDeliveryHoldFixture(t, ctx)
	defer f.client.Close()

	priceCents := int64(1500)
	qty := 1
	orderID, _, _, shipmentID := f.createShippedOrderWithItem(t, ctx, priceCents, qty)

	// First delivery
	res, err := f.svc.DeliverShipment(ctx, f.adminID, shipmentID, fulfillment.DeliverShipmentRequest{})
	require.NoError(t, err)
	require.NotNil(t, res)

	// Record baseline hold state
	var originalDeliveredAt time.Time
	err = f.db.QueryRow(ctx, `SELECT delivered_at FROM shipments WHERE id = $1`, shipmentID).Scan(&originalDeliveredAt)
	require.NoError(t, err)

	var originalEarningID uuid.UUID
	var originalAvailableAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT id, available_at FROM seller_ledger_entries WHERE order_id = $1 AND type = 'seller_earning'`, orderID).Scan(&originalEarningID, &originalAvailableAt)
	require.NoError(t, err)
	require.NotNil(t, originalAvailableAt)

	var originalCount int
	err = f.db.QueryRow(ctx, `SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1`, orderID).Scan(&originalCount)
	require.NoError(t, err)
	assert.Equal(t, 3, originalCount)

	// Second delivery attempt — MUST FAIL
	res2, err2 := f.svc.DeliverShipment(ctx, f.adminID, shipmentID, fulfillment.DeliverShipmentRequest{})
	assert.Error(t, err2)
	assert.Nil(t, res2)
	assert.True(t, errors.Is(err2, fulfillment.ErrShipmentAlreadyDelivered), "expected ErrShipmentAlreadyDelivered, got: %v", err2)

	// Verify hold timestamps and entries are NOT duplicated or corrupted
	var postDeliveredAt time.Time
	err = f.db.QueryRow(ctx, `SELECT delivered_at FROM shipments WHERE id = $1`, shipmentID).Scan(&postDeliveredAt)
	require.NoError(t, err)
	assert.True(t, originalDeliveredAt.Equal(postDeliveredAt), "shipment delivered_at must not be corrupted")

	var postCount int
	err = f.db.QueryRow(ctx, `SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1`, orderID).Scan(&postCount)
	require.NoError(t, err)
	assert.Equal(t, originalCount, postCount, "seller_ledger_entries count must not increase")

	var postEarningID uuid.UUID
	var postAvailableAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT id, available_at FROM seller_ledger_entries WHERE order_id = $1 AND type = 'seller_earning'`, orderID).Scan(&postEarningID, &postAvailableAt)
	require.NoError(t, err)
	assert.Equal(t, originalEarningID, postEarningID)
	assert.True(t, originalAvailableAt.Equal(*postAvailableAt), "available_at must not be modified by failed repeated delivery")
}

// Mock payouts service that simulates failure at specific integration points
type failingPayoutsMock struct {
	realPayouts            *payouts.Service
	failCreatePendingSales bool
	failMarkOrderDelivered bool
}

func (m *failingPayoutsMock) CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error {
	return m.realPayouts.CreatePendingSalesForOrder(ctx, orderID)
}

func (m *failingPayoutsMock) CreatePendingSalesForFulfillmentTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) error {
	if m.failCreatePendingSales {
		return errors.New("simulated failure in CreatePendingSalesForFulfillmentTx")
	}
	return m.realPayouts.CreatePendingSalesForFulfillmentTx(ctx, tx, fulfillmentID)
}

func (m *failingPayoutsMock) MarkOrderDeliveredTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, deliveredAt time.Time) error {
	if m.failMarkOrderDelivered {
		return errors.New("simulated failure in MarkOrderDeliveredTx")
	}
	return m.realPayouts.MarkOrderDeliveredTx(ctx, tx, orderID, deliveredAt)
}

// Case D: Finance failure atomicity test
// If payouts/hold step fails:
// - delivery transaction rolls back
// - shipment does NOT transition to delivered
// - orders table is NOT corrupted
// - no orphaned seller_ledger_entries rows remain
func TestDeliverShipment_FinanceFailureAtomicity(t *testing.T) {
	ctx := context.Background()
	f := setupDeliveryHoldFixture(t, ctx)
	defer f.client.Close()

	t.Run("fails when MarkOrderDeliveredTx fails -> full rollback", func(t *testing.T) {
		mockPayouts := &failingPayoutsMock{
			realPayouts:            f.payoutSvc,
			failMarkOrderDelivered: true,
		}
		serviceWithFailingPayouts := fulfillment.NewService(f.repo, f.ordersRepo, f.client, mockPayouts, nil)

		priceCents := int64(4500)
		qty := 1
		orderID, fulfillmentID, _, shipmentID := f.createShippedOrderWithItem(t, ctx, priceCents, qty)

		res, err := serviceWithFailingPayouts.DeliverShipment(ctx, f.adminID, shipmentID, fulfillment.DeliverShipmentRequest{})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "simulated failure in MarkOrderDeliveredTx")

		// 1. Shipment MUST remain 'shipped' and delivered_at MUST remain NULL
		var shipStatus string
		var deliveredAt *time.Time
		err = f.db.QueryRow(ctx, `SELECT status, delivered_at FROM shipments WHERE id = $1`, shipmentID).Scan(&shipStatus, &deliveredAt)
		require.NoError(t, err)
		assert.Equal(t, "shipped", shipStatus)
		assert.Nil(t, deliveredAt, "delivered_at must be rolled back to null")

		// 2. Fulfillment MUST remain 'shipped'
		var fulStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fulStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", fulStatus)

		// 3. Parent order status MUST remain 'shipped'
		var orderStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&orderStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", orderStatus)

		// 4. NO orphaned seller_ledger_entries rows must exist
		var ledgerCount int
		err = f.db.QueryRow(ctx, `SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1`, orderID).Scan(&ledgerCount)
		require.NoError(t, err)
		assert.Equal(t, 0, ledgerCount, "no orphaned seller_ledger_entries must remain after rollback")

		// 5. NO shipment_events for delivered
		var eventCount int
		err = f.db.QueryRow(ctx, `SELECT COUNT(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'delivered'`, shipmentID).Scan(&eventCount)
		require.NoError(t, err)
		assert.Equal(t, 0, eventCount, "no delivered shipment_events must remain after rollback")
	})

	t.Run("fails when CreatePendingSalesForFulfillmentTx fails -> full rollback", func(t *testing.T) {
		mockPayouts := &failingPayoutsMock{
			realPayouts:            f.payoutSvc,
			failCreatePendingSales: true,
		}
		serviceWithFailingPayouts := fulfillment.NewService(f.repo, f.ordersRepo, f.client, mockPayouts, nil)

		priceCents := int64(2500)
		qty := 1
		orderID, fulfillmentID, _, shipmentID := f.createShippedOrderWithItem(t, ctx, priceCents, qty)

		res, err := serviceWithFailingPayouts.DeliverShipment(ctx, f.adminID, shipmentID, fulfillment.DeliverShipmentRequest{})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "simulated failure in CreatePendingSalesForFulfillmentTx")

		// 1. Shipment MUST remain 'shipped'
		var shipStatus string
		var deliveredAt *time.Time
		err = f.db.QueryRow(ctx, `SELECT status, delivered_at FROM shipments WHERE id = $1`, shipmentID).Scan(&shipStatus, &deliveredAt)
		require.NoError(t, err)
		assert.Equal(t, "shipped", shipStatus)
		assert.Nil(t, deliveredAt)

		// 2. Fulfillment MUST remain 'shipped'
		var fulStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fulStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", fulStatus)

		// 3. Parent order status MUST remain 'shipped'
		var orderStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&orderStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", orderStatus)

		// 4. NO orphaned seller_ledger_entries rows
		var ledgerCount int
		err = f.db.QueryRow(ctx, `SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1`, orderID).Scan(&ledgerCount)
		require.NoError(t, err)
		assert.Equal(t, 0, ledgerCount)
	})
}
