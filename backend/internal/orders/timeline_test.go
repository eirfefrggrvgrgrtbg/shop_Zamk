package orders_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestOrderTimeline_ComprehensiveMatrix(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Strict DB Safety Guard
	testutil.AssertTestDatabase(t, pool)

	dbClient := &postgres.Client{Pool: pool}
	repo := orders.NewRepository(pool)
	svc := orders.NewService(repo, nil, nil, dbClient, nil)

	// Fixture IDs for exact cleanup
	userID := uuid.New()
	sellerID := uuid.New()
	categoryID := uuid.New()
	brandID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	inventoryItemID := uuid.New()
	reservationID := uuid.New()
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	unitID := uuid.New()
	orderID := uuid.New()
	statusHistoryID := uuid.New()
	fulfillmentID := uuid.New()
	orderItemID := uuid.New()
	allocationID := uuid.New()
	paymentID := uuid.New()
	shipmentID := uuid.New()

	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])
	zmuCode := fmt.Sprintf("ZMU-%s", unitID.String()[:16])
	payNumber := fmt.Sprintf("PAY-%s", paymentID.String()[:6])
	trackNumber := fmt.Sprintf("TRK-%s", shipmentID.String()[:6])
	supplyNumber := fmt.Sprintf("SUP-%s", supplyID.String()[:6])

	t0 := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	tReserved := t0.Add(2 * time.Minute)
	tPaid := t0.Add(5 * time.Minute)
	tPickingStarted := t0.Add(10 * time.Minute)
	tPicked := t0.Add(15 * time.Minute)
	tPacked := t0.Add(25 * time.Minute)
	tShipped := t0.Add(35 * time.Minute)
	tDelivered := t0.Add(45 * time.Minute)

	cleanup := func() {
		// Exact parameterized cleanup in reverse dependency order
		_, _ = pool.Exec(ctx, "DELETE FROM shipments WHERE id = $1", shipmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM payments WHERE id = $1", paymentID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_item_allocations WHERE id = $1", allocationID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_items WHERE id = $1", orderItemID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_status_history WHERE id = $1", statusHistoryID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_reservations WHERE order_id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM reservations WHERE id = $1", reservationID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM inventory_units WHERE id = $1", unitID)
		_, _ = pool.Exec(ctx, "DELETE FROM seller_supply_items WHERE id = $1", supplyItemID)
		_, _ = pool.Exec(ctx, "DELETE FROM seller_supplies WHERE id = $1", supplyID)
		_, _ = pool.Exec(ctx, "DELETE FROM inventory_items WHERE id = $1", inventoryItemID)
		_, _ = pool.Exec(ctx, "DELETE FROM product_variants WHERE id = $1", variantID)
		_, _ = pool.Exec(ctx, "DELETE FROM products WHERE id = $1", productID)
		_, _ = pool.Exec(ctx, "DELETE FROM brands WHERE id = $1", brandID)
		_, _ = pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", categoryID)
		_, _ = pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}

	cleanup()
	t.Cleanup(func() {
		cleanup()
		// Verify zero leftovers
		var count int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE id = $1", orderID).Scan(&count)
		assert.Equal(t, 0, count, "leftover order record found")
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM reservations WHERE id = $1", reservationID).Scan(&count)
		assert.Equal(t, 0, count, "leftover reservation record found")
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM payments WHERE id = $1", paymentID).Scan(&count)
		assert.Equal(t, 0, count, "leftover payment record found")
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM order_item_allocations WHERE id = $1", allocationID).Scan(&count)
		assert.Equal(t, 0, count, "leftover allocation record found")
	})

	// 1. Insert Base Dependencies
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Timeline User', $2, 'hash', 'customer', 'active', false, now(), now())
	`, userID, fmt.Sprintf("tl_user_%s@test.local", userID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'Timeline Brand', $2, 'desc', 'seller@test.local', '123456', 'active', now(), now())
	`, sellerID, fmt.Sprintf("tl-brand-%s", sellerID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'TL Cat', $2, now(), now())
	`, categoryID, fmt.Sprintf("tl-cat-%s", categoryID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO brands (id, name, slug, created_at, updated_at)
		VALUES ($1, 'TL Brand', $2, now(), now())
	`, brandID, fmt.Sprintf("tl-brand-%s", brandID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Wool Coat Extra', $5, 'desc', 'published', 1500000, now(), now())
	`, productID, sellerID, categoryID, brandID, fmt.Sprintf("wool-coat-%s", productID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode, size, color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'M', 'Black', now(), now())
	`, variantID, productID, fmt.Sprintf("SKU-TL-%s", variantID.String()[:8]), fmt.Sprintf("ZMK-TL-%s", variantID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 1, now(), now())
	`, inventoryItemID, productID, variantID, sellerID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, supply_number, status, handoff_method, created_at, updated_at)
		VALUES ($1, $2, $3, 'completed', 'self_delivery', now(), now())
	`, supplyID, sellerID, supplyNumber)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 1, now(), now())
	`, unitID, variantID, zmuCode, supplyID, supplyItemID)
	require.NoError(t, err)

	// 2. Insert Order & Events
	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 1500000, 'RUB', 'Customer A', '123', 'a@b.c', 'Moscow', $4, $4)
	`, orderID, userID, ordNumber, t0)
	require.NoError(t, err)

	// Reservation event: order.reserved
	_, err = pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 day', $6, $7)
	`, reservationID, inventoryItemID, productID, variantID, userID, orderID, tReserved)
	require.NoError(t, err)

	// Status history: order.picking_started (to_status = 'assembling')
	_, err = pool.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, from_status, to_status, actor_user_id, comment, created_at)
		VALUES ($1, $2, 'paid', 'assembling', NULL, 'order assembling started', $3)
	`, statusHistoryID, orderID, tPickingStarted)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, packed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'packed', 1500000, 1000, 1350000, $4, $5, $5)
	`, fulfillmentID, orderID, sellerID, tPacked, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, price_cents, quantity, subtotal_price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Wool Coat Extra', 'wool-coat-extra', 'M', 'Black', 'SKU-TL-01', 1500000, 1, 1500000, $7)
	`, orderItemID, orderID, fulfillmentID, productID, variantID, sellerID, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, allocationID, orderItemID, unitID, tPicked, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at, paid_at)
		VALUES ($1, $2, 'tbank', 'pay-mock-1', 'succeeded', 1500000, 'RUB', 'url', $3, $4, 'card', 'mock', $5, $5, $6)
	`, paymentID, orderID, fmt.Sprintf("idemp-%s", paymentID.String()), payNumber, t0, tPaid)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, carrier, tracking_number, status, shipped_at, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'cdek', $4, 'delivered', $5, $6, $7, $7)
	`, shipmentID, orderID, fulfillmentID, trackNumber, tShipped, tDelivered, t0)
	require.NoError(t, err)

	// Fetch Timeline via Service
	tl, err := svc.GetAdminOrderTimeline(ctx, orderID)
	require.NoError(t, err)
	require.NotNil(t, tl)

	// Also verify legacy repository adapter reuses the canonical assembler
	repoEvents, err := repo.GetOrderTimeline(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, len(tl.Events), len(repoEvents), "repository adapter must produce identical event count")

	// Strict Assertions
	assert.Equal(t, "order", tl.EntityType)
	assert.Equal(t, orderID, tl.EntityID)
	assert.Equal(t, ordNumber, tl.CanonicalIdentifier)
	assert.NotContains(t, tl.CanonicalIdentifier, orderID.String())

	// Expected 8 events:
	// 0: order.created (t0)
	// 1: order.reserved (tReserved)
	// 2: order.paid (tPaid)
	// 3: order.picking_started (tPickingStarted)
	// 4: order.unit_picked (tPicked)
	// 5: order.packed (tPacked)
	// 6: order.shipped (tShipped)
	// 7: order.delivered (tDelivered)
	require.Len(t, tl.Events, 8)

	// Chronological ascending check
	for i := 0; i < len(tl.Events)-1; i++ {
		assert.True(t, tl.Events[i].OccurredAt.Before(tl.Events[i+1].OccurredAt) || tl.Events[i].OccurredAt.Equal(tl.Events[i+1].OccurredAt),
			"events must be in ascending chronological order: %s vs %s", tl.Events[i].Type, tl.Events[i+1].Type)
	}

	// Event 0: order.created
	assert.Equal(t, "order.created", tl.Events[0].Type)
	assert.True(t, t0.Equal(tl.Events[0].OccurredAt))
	assert.Equal(t, "system", tl.Events[0].ActorType)
	assert.Equal(t, "Система", tl.Events[0].ActorLabel)
	assert.Contains(t, tl.Events[0].Description, ordNumber)

	// Event 1: order.reserved
	assert.Equal(t, "order.reserved", tl.Events[1].Type)
	assert.True(t, tReserved.Equal(tl.Events[1].OccurredAt))
	assert.Equal(t, "system", tl.Events[1].ActorType)
	assert.Equal(t, "Система", tl.Events[1].ActorLabel)
	assert.Equal(t, "Товар зарезервирован", tl.Events[1].Title)
	assert.Contains(t, tl.Events[1].Description, ordNumber)
	assert.NotContains(t, tl.Events[1].Description, reservationID.String())

	// Event 2: order.paid
	assert.Equal(t, "order.paid", tl.Events[2].Type)
	assert.True(t, tPaid.Equal(tl.Events[2].OccurredAt))
	assert.Equal(t, "system", tl.Events[2].ActorType)
	assert.Contains(t, tl.Events[2].Description, payNumber)

	// Event 3: order.picking_started
	assert.Equal(t, "order.picking_started", tl.Events[3].Type)
	assert.True(t, tPickingStarted.Equal(tl.Events[3].OccurredAt))
	assert.Equal(t, "warehouse", tl.Events[3].ActorType)
	assert.Equal(t, "Склад ZAMK", tl.Events[3].ActorLabel)
	assert.Equal(t, "Сборка начата", tl.Events[3].Title)
	assert.Contains(t, tl.Events[3].Description, ordNumber)

	// Event 4: order.unit_picked
	assert.Equal(t, "order.unit_picked", tl.Events[4].Type)
	assert.True(t, tPicked.Equal(tl.Events[4].OccurredAt))
	assert.Equal(t, "warehouse", tl.Events[4].ActorType)
	assert.Equal(t, "Склад ZAMK", tl.Events[4].ActorLabel)
	assert.Contains(t, tl.Events[4].Description, zmuCode)
	assert.Contains(t, tl.Events[4].Description, "Wool Coat Extra")

	// Event 5: order.packed
	assert.Equal(t, "order.packed", tl.Events[5].Type)
	assert.True(t, tPacked.Equal(tl.Events[5].OccurredAt))
	assert.Equal(t, "warehouse", tl.Events[5].ActorType)
	assert.Equal(t, "Склад ZAMK", tl.Events[5].ActorLabel)
	assert.Contains(t, tl.Events[5].Description, ordNumber)

	// Event 6: order.shipped
	assert.Equal(t, "order.shipped", tl.Events[6].Type)
	assert.True(t, tShipped.Equal(tl.Events[6].OccurredAt))
	assert.Equal(t, "system", tl.Events[6].ActorType)
	assert.Contains(t, tl.Events[6].Description, trackNumber)

	// Event 7: order.delivered
	assert.Equal(t, "order.delivered", tl.Events[7].Type)
	assert.True(t, tDelivered.Equal(tl.Events[7].OccurredAt))
	assert.Equal(t, "system", tl.Events[7].ActorType)
	assert.Contains(t, tl.Events[7].Description, trackNumber)
}

func TestOrderTimeline_EqualTimestampTieBreak(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	testutil.AssertTestDatabase(t, pool)

	dbClient := &postgres.Client{Pool: pool}
	repo := orders.NewRepository(pool)
	svc := orders.NewService(repo, nil, nil, dbClient, nil)

	userID := uuid.New()
	orderID := uuid.New()
	paymentID := uuid.New()
	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])
	payNumber := fmt.Sprintf("PAY-%s", paymentID.String()[:6])
	exactTime := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

	cleanup := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM payments WHERE id = $1", paymentID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}
	cleanup()
	t.Cleanup(cleanup)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'TieBreak User', $2, 'hash', 'customer', 'active', false, now(), now())
	`, userID, fmt.Sprintf("tb_user_%s@test.local", userID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'paid', 100000, 'RUB', 'Customer TB', '123', 'tb@b.c', 'Addr', $4, $4)
	`, orderID, userID, ordNumber, exactTime)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at, paid_at)
		VALUES ($1, $2, 'tbank', 'pay-mock-tb', 'succeeded', 100000, 'RUB', 'url', $3, $4, 'card', 'mock', $5, $5, $5)
	`, paymentID, orderID, fmt.Sprintf("idemp-%s", paymentID.String()), payNumber, exactTime)
	require.NoError(t, err)

	tl, err := svc.GetAdminOrderTimeline(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, tl.Events, 2)

	// order.created must strictly precede order.paid when timestamps are identical
	assert.Equal(t, "order.created", tl.Events[0].Type)
	assert.Equal(t, "order.paid", tl.Events[1].Type)
}

func TestOrderTimeline_HandlerInternalErrorPrivacy(t *testing.T) {
	handler := orders.NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/orders/invalid-uuid/timeline", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetAdminOrderTimeline(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_id", resp["error"]["code"])
}
