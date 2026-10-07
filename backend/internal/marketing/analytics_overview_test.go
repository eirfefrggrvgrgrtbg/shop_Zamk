package marketing_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// TestAnalyticsOverview_Matrix tests marketing overview data according to Matrix A-AE.
func TestAnalyticsOverview_Matrix(t *testing.T) {
	ctx := context.Background()
	dbURL := testutil.GetTestDatabaseURL()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer db.Close()

	// Clear out relevant tables to ensure clean state
	_, err = db.Exec(ctx, `TRUNCATE users, sellers, categories, brands, products, product_variants, marketing_campaigns, analytics_sessions, behavioral_events, orders, order_fulfillments, payments, order_items, returns, return_items, refunds CASCADE`)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	startOfRange := now.Add(-24 * time.Hour) // range: [now-24h, now)
	prevStart := startOfRange.Add(-24 * time.Hour)

	// Setup Base Catalog Fixtures
	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', 'cat', $2, $2)`, catID, prevStart)
	require.NoError(t, err)

	brandID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO brands (id, name, slug, created_at, updated_at) VALUES ($1, 'Brand', 'brand', $2, $2)`, brandID, prevStart)
	require.NoError(t, err)

	sellerUserID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Seller', 'seller@test.com', '79991234568', 'hash', 'seller', 'active', false, $2, $2)`, sellerUserID, prevStart)
	require.NoError(t, err)

	sellerID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO sellers (id, status, created_at, updated_at) VALUES ($1, 'active', $2, $2)`, sellerID, prevStart)
	require.NoError(t, err)

	productID := uuid.New()
	variantID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Product', 'product', 'published', 'Desc', 1250, $5, $5)`, productID, sellerID, catID, brandID, prevStart)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'SKU-1', 1250, $3, $3)`, variantID, productID, prevStart)
	require.NoError(t, err)

	// Matrix X: planned budget in campaign model
	campaignID := uuid.New()
	plannedBudgetCents := int64(500000)
	_, err = db.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, title, campaign_type, purpose, seller_id, starts_at, ends_at, funding_mode, status, discount_type, seller_discount_bps, planned_budget_cents)
		VALUES ($1, 'Campaign Alpha', 'discount', 'advertising', NULL, $2, $3, 'seller', 'active', 'percent', 1000, $4)
	`, campaignID, prevStart, now.Add(24*time.Hour), plannedBudgetCents)
	require.NoError(t, err)

	// User 1 (will be a Repeat customer in current range, because they ordered in prev range)
	user1ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Customer 1', 'c1@test.com', '79991111111', 'hash', 'customer', 'active', false, $2, $2)`, user1ID, prevStart)
	require.NoError(t, err)

	// User 2 (will be a New customer in current range)
	user2ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Customer 2', 'c2@test.com', '79992222222', 'hash', 'customer', 'active', false, $2, $2)`, user2ID, startOfRange)
	require.NoError(t, err)

	// 1. Prior period order for user1 (makes user1 a repeat customer in current range)
	orderPrevID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 1000, 'RUB', 'C1', '79991111111', 'c1@test.com', 'Addr', $3, $3)`, orderPrevID, user1ID, prevStart.Add(2*time.Hour))
	require.NoError(t, err)
	fulPrevID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 1000, 1000, 900, $4, $4)`, fulPrevID, orderPrevID, sellerID, prevStart.Add(2*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'Item Prev', 'item-prev', 1000, 1, 1000, $7)`, uuid.New(), orderPrevID, fulPrevID, productID, variantID, sellerID, prevStart.Add(2*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 1000, 'RUB', 'P-PREV', 'idem-prev', 'card', 'mock', 'pending', $3, $3, $3)`, uuid.New(), orderPrevID, prevStart.Add(2*time.Hour))
	require.NoError(t, err)

	// Prior period session
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, started_at, last_seen_at, source) VALUES ($1, $2, $3, $3, 'organic')`, uuid.New(), uuid.New(), prevStart.Add(time.Hour))
	require.NoError(t, err)

	// 2. Current period session 1: Campaign visit + direct source test
	session1ID := uuid.New()
	visitor1ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, started_at, last_seen_at, campaign_id, source) VALUES ($1, $2, $3, $3, $4, 'vk')`, session1ID, visitor1ID, startOfRange.Add(time.Hour), campaignID)
	require.NoError(t, err)

	// Current period session 2: Direct source (Matrix P)
	session2ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, started_at, last_seen_at, source) VALUES ($1, $2, $3, $3, 'direct')`, session2ID, uuid.New(), startOfRange.Add(2*time.Hour))
	require.NoError(t, err)

	// Current period session 3: Missing/unattributed source (Matrix Q)
	session3ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, started_at, last_seen_at, source) VALUES ($1, $2, $3, $3, 'missing')`, session3ID, uuid.New(), startOfRange.Add(3*time.Hour))
	require.NoError(t, err)

	// Matrix T: Pageviews do not inflate visits (Insert 20 behavioral page_view events for session 1)
	for i := 0; i < 20; i++ {
		_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, session_id, visitor_id, event_type, source, occurred_at) VALUES ($1, $2, $3, 'page_view', 'client', $4)`, uuid.New(), session1ID, visitor1ID, startOfRange.Add(time.Hour))
		require.NoError(t, err)
	}

	// 3. Paid Order 1 in current range (User 1 - repeat customer, 2 sold units, amount 2500, attributed to Campaign Alpha)
	order1ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 2500, 'RUB', 'C1', '79991111111', 'c1@test.com', 'Addr', $3, $3)`, order1ID, user1ID, startOfRange.Add(4*time.Hour))
	require.NoError(t, err)
	ful1ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 2500, 1000, 2000, $4, $4)`, ful1ID, order1ID, sellerID, startOfRange.Add(4*time.Hour))
	require.NoError(t, err)
	order1ItemID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'Item 1', 'item-1', 1250, 2, 2500, $7)`, order1ItemID, order1ID, ful1ID, productID, variantID, sellerID, startOfRange.Add(4*time.Hour))
	require.NoError(t, err)

	// Matrix F: Payment retry fixture - 1 failed attempt, 1 succeeded attempt for Order 1
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, failed_at) VALUES ($1, $2, 'failed', 'tbank', 2500, 'RUB', 'P-FAIL', 'idem-f', 'card', 'mock', 'failed', $3, $3, $3)`, uuid.New(), order1ID, startOfRange.Add(4*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 2500, 'RUB', 'P-1', 'idem-1', 'card', 'mock', 'pending', $3, $3, $3)`, uuid.New(), order1ID, startOfRange.Add(4*time.Hour))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO order_attributions (order_id, session_id, visitor_id, campaign_id, source, attributed_at) VALUES ($1, $2, $3, $4, 'vk', $5)`, order1ID, session1ID, visitor1ID, campaignID, startOfRange.Add(4*time.Hour))
	require.NoError(t, err)

	// 4. Paid Order 2 in current range (User 2 - new customer, 1 unit, amount 1500)
	order2ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 1500, 'RUB', 'C2', '79992222222', 'c2@test.com', 'Addr', $3, $3)`, order2ID, user2ID, startOfRange.Add(6*time.Hour))
	require.NoError(t, err)
	ful2ID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 1500, 1000, 1200, $4, $4)`, ful2ID, order2ID, sellerID, startOfRange.Add(6*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'Item 2', 'item-2', 1500, 1, 1500, $7)`, uuid.New(), order2ID, ful2ID, productID, variantID, sellerID, startOfRange.Add(6*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 1500, 'RUB', 'P-2', 'idem-2', 'card', 'mock', 'pending', $3, $3, $3)`, uuid.New(), order2ID, startOfRange.Add(6*time.Hour))
	require.NoError(t, err)
	// Matrix P: Attributed to Direct
	_, err = db.Exec(ctx, `INSERT INTO order_attributions (order_id, session_id, source, attributed_at) VALUES ($1, $2, 'direct', $3)`, order2ID, session2ID, startOfRange.Add(6*time.Hour))
	require.NoError(t, err)

	// Matrix N fixture: User 2 makes a SECOND paid order in the same period (amount 1000)
	order2SecondID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 1000, 'RUB', 'C2', '79992222222', 'c2@test.com', 'Addr', $3, $3)`, order2SecondID, user2ID, startOfRange.Add(7*time.Hour))
	require.NoError(t, err)
	ful2SecondID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 1000, 1000, 800, $4, $4)`, ful2SecondID, order2SecondID, sellerID, startOfRange.Add(7*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'Item 2B', 'item-2b', 1000, 1, 1000, $7)`, uuid.New(), order2SecondID, ful2SecondID, productID, variantID, sellerID, startOfRange.Add(7*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 1000, 'RUB', 'P-2B', 'idem-2b', 'card', 'mock', 'pending', $3, $3, $3)`, uuid.New(), order2SecondID, startOfRange.Add(7*time.Hour))
	require.NoError(t, err)

	// Matrix D: Unpaid order fixture (status 'pending', payment 'pending') -> Must be excluded
	unpaidOrderID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'awaiting_payment', 5000, 'RUB', 'C1', '79991111111', 'c1@test.com', 'Addr', $3, $3)`, unpaidOrderID, user1ID, startOfRange.Add(8*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at) VALUES ($1, $2, 'pending', 'tbank', 5000, 'RUB', 'P-PEND', 'idem-pend', 'card', 'mock', 'pending', $3, $3)`, uuid.New(), unpaidOrderID, startOfRange.Add(8*time.Hour))
	require.NoError(t, err)

	// Matrix E: Cancelled order fixture (status 'cancelled') -> Must be excluded
	cancelledOrderID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'cancelled', 3000, 'RUB', 'C1', '79991111111', 'c1@test.com', 'Addr', $3, $3)`, cancelledOrderID, user1ID, startOfRange.Add(9*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 3000, 'RUB', 'P-CANC', 'idem-canc', 'card', 'mock', 'pending', $3, $3, $3)`, uuid.New(), cancelledOrderID, startOfRange.Add(9*time.Hour))
	require.NoError(t, err)

	// Matrix AB: Order created earlier before from, but payment succeeded inside [from, to)
	orderCreatedEarlierID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 2000, 'RUB', 'C1', '79991111111', 'c1@test.com', 'Addr', $3, $3)`, orderCreatedEarlierID, user1ID, prevStart.Add(2*time.Hour))
	require.NoError(t, err)
	fulCreatedEarlierID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 2000, 1000, 1700, $4, $4)`, fulCreatedEarlierID, orderCreatedEarlierID, sellerID, prevStart.Add(2*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'Item Early', 'item-early', 2000, 1, 2000, $7)`, uuid.New(), orderCreatedEarlierID, fulCreatedEarlierID, productID, variantID, sellerID, prevStart.Add(2*time.Hour))
	require.NoError(t, err)
	// Payment happened inside current range!
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 2000, 'RUB', 'P-EARLY', 'idem-early', 'card', 'mock', 'pending', $3, $3, $4)`, uuid.New(), orderCreatedEarlierID, prevStart.Add(2*time.Hour), startOfRange.Add(10*time.Hour))
	require.NoError(t, err)

	// Matrix O: Canonical completed return (status 'completed')
	retID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at, completed_at) VALUES ($1, $2, $3, $4, 'completed', 'defective', $5, $5, $5)`, retID, order1ID, ful1ID, user1ID, startOfRange.Add(11*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at) VALUES ($1, $2, $3, 1, 1, $4)`, uuid.New(), retID, order1ItemID, startOfRange.Add(11*time.Hour))
	require.NoError(t, err)
	// Non-completed return (status 'requested') -> Must not count in completed returns
	retPendingID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, $4, 'requested', 'size', $5, $5)`, retPendingID, order1ID, ful1ID, user1ID, startOfRange.Add(11*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at) VALUES ($1, $2, $3, 1, 0, $4)`, uuid.New(), retPendingID, order1ItemID, startOfRange.Add(11*time.Hour))
	require.NoError(t, err)

	// Refund for completed return
	_, err = db.Exec(ctx, `INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, created_at, updated_at, processed_at) VALUES ($1, $2, $3, 'succeeded', 1250, 'RUB', $4, $4, $4)`, uuid.New(), retID, order1ID, startOfRange.Add(12*time.Hour))
	require.NoError(t, err)

	repo := marketing.NewAnalyticsRepository(db)
	svc := marketing.NewAnalyticsService(repo)
	handler := marketing.NewHandler(&marketing.Service{
		Analytics: svc,
	}, slog.Default())

	// Matrix A: Empty period
	t.Run("MatrixA_EmptyPeriod", func(t *testing.T) {
		emptyFrom := now.Add(-300 * 24 * time.Hour)
		emptyTo := emptyFrom.Add(24 * time.Hour)
		snap, err := repo.GetOverviewMetrics(ctx, emptyFrom, emptyTo)
		require.NoError(t, err)
		assert.Equal(t, 0, snap.Visits)
		assert.Equal(t, 0, snap.PaidOrders)
		assert.Equal(t, int64(0), snap.RevenueCents)
		assert.Equal(t, 0, snap.SoldUnits)
		assert.Equal(t, int64(0), snap.AovCents)
		assert.Equal(t, 0, snap.ConversionRateBps)
		assert.Equal(t, 0, snap.ReturnsCount)
	})

	// Matrix B: Visits-only
	t.Run("MatrixB_VisitsOnly", func(t *testing.T) {
		vFrom := prevStart.Add(-24 * time.Hour)
		vTo := vFrom.Add(time.Hour)
		_, err := db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, started_at, last_seen_at) VALUES ($1, $2, $3, $3)`, uuid.New(), uuid.New(), vFrom.Add(10*time.Minute))
		require.NoError(t, err)
		snap, err := repo.GetOverviewMetrics(ctx, vFrom, vTo)
		require.NoError(t, err)
		assert.Equal(t, 1, snap.Visits)
		assert.Equal(t, 0, snap.PaidOrders)
		assert.Equal(t, int64(0), snap.RevenueCents)
		assert.Equal(t, 0, snap.ConversionRateBps)
	})

	// Matrix C & D & E & F & G & H: Paid orders, unpaid excluded, cancelled excluded, retry safety, sold units, canonical revenue
	t.Run("MatrixC_to_H_PaidOrders_Revenue_SoldUnits", func(t *testing.T) {
		snap, err := repo.GetOverviewMetrics(ctx, startOfRange, now)
		require.NoError(t, err)

		// Paid orders count: Order 1 (2500) + Order 2 (1500) + Order 2B (1000) + Order Created Earlier (2000) = 4 paid orders
		// Unpaid (5000) is excluded (Matrix D)
		// Cancelled (3000) is excluded (Matrix E)
		// Payment retry on Order 1 does not double count (Matrix F)
		assert.Equal(t, 4, snap.PaidOrders, "Matrix C & D & E & F: Exactly 4 valid paid orders counted")

		// Canonical gross revenue field = 2500 + 1500 + 1000 + 2000 = 7000 cents (Matrix H)
		assert.Equal(t, int64(7000), snap.RevenueCents, "Matrix H: Canonical customer-paid gross revenue")

		// Sold units: Order 1 (2) + Order 2 (1) + Order 2B (1) + Order Early (1) = 5 units (Matrix G)
		assert.Equal(t, 5, snap.SoldUnits, "Matrix G: Exactly 5 sold units belonging to paid orders")
	})

	// Matrix I: AOV (Average Order Value)
	t.Run("MatrixI_AOV", func(t *testing.T) {
		snap, err := repo.GetOverviewMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		// 7000 cents / 4 paid orders = 1750 cents
		assert.Equal(t, int64(1750), snap.AovCents, "Matrix I: AOV = Revenue / PaidOrders")
	})

	// Matrix J: Conversion & Matrix T: Pageviews do not inflate visits
	t.Run("MatrixJ_and_T_Visits_and_Conversion", func(t *testing.T) {
		snap, err := repo.GetOverviewMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		// Sessions in range: session 1, session 2, session 3 = 3 visits (20 pageviews in session 1 do NOT increase visits)
		assert.Equal(t, 3, snap.Visits, "Matrix T: 3 sessions = 3 visits despite 20 pageviews")
		// Conversion: 4 orders / 3 visits = >100% or 13333 bps
		assert.Equal(t, 13333, snap.ConversionRateBps, "Matrix J: Conversion calculated as paidOrders / visits * 10000 bps")
	})

	// Matrix K: Zero denominator
	t.Run("MatrixK_ZeroDenominator", func(t *testing.T) {
		emptyFrom := now.Add(-500 * 24 * time.Hour)
		emptyTo := emptyFrom.Add(time.Hour)
		snap, err := repo.GetOverviewMetrics(ctx, emptyFrom, emptyTo)
		require.NoError(t, err)
		assert.Equal(t, 0, snap.ConversionRateBps, "Zero denominator produces 0 bps, never panic or NaN")
		assert.Equal(t, int64(0), snap.AovCents)
	})

	// Matrix L & M & N: New customer, repeat customer, multiple period orders counted once
	t.Run("MatrixL_M_N_CustomerClassification", func(t *testing.T) {
		snap, err := repo.GetOverviewMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		// User 1 ordered in previous range -> Repeat customer (1)
		// User 2 ordered twice in current range -> New customer (1), NOT counted twice (Matrix N)
		assert.Equal(t, 1, snap.RepeatCustomers, "Matrix M: User 1 is a repeat customer")
		assert.Equal(t, 1, snap.NewCustomers, "Matrix L & N: User 2 is a new customer counted once despite multiple orders")
	})

	// Matrix O: Canonical completed returns
	t.Run("MatrixO_CompletedReturns", func(t *testing.T) {
		snap, err := repo.GetOverviewMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		assert.Equal(t, 1, snap.ReturnsCount, "Matrix O: Only completed return counted")
		assert.Equal(t, 1, snap.ReturnedUnits, "Matrix O: Only accepted returned units counted")
		assert.Equal(t, int64(1250), snap.ReturnedAmountCents, "Matrix O: Refunded amount")
	})

	// Matrix P & Q & R: Direct vs Missing vs Source attribution
	t.Run("MatrixP_Q_R_SourceAttribution", func(t *testing.T) {
		srcs, err := repo.GetSourceMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		sourceMap := make(map[string]marketing.RawSourceMetrics)
		for _, s := range srcs {
			sourceMap[s.Source] = s
		}

		// Direct bucket exists (Matrix P)
		assert.Contains(t, sourceMap, "Direct")
		assert.Equal(t, 1, sourceMap["Direct"].Visits)
		assert.Equal(t, 1, sourceMap["Direct"].PaidOrders) // Order 2 attributed to Direct

		// Unattributed bucket exists and is separate (Matrix Q)
		assert.Contains(t, sourceMap, "Unattributed")
		assert.Equal(t, 1, sourceMap["Unattributed"].Visits)

		// 'vk' source bucket exists (Matrix R)
		assert.Contains(t, sourceMap, "vk")
		assert.Equal(t, 1, sourceMap["vk"].Visits)
		assert.Equal(t, 1, sourceMap["vk"].PaidOrders)
		assert.Equal(t, int64(2500), sourceMap["vk"].RevenueCents)
	})

	// Matrix S & U & V & W: Campaign metrics
	t.Run("MatrixS_U_V_W_CampaignMetrics", func(t *testing.T) {
		cmps, err := repo.GetCampaignMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		var campAlpha *marketing.RawCampaignMetrics
		for i := range cmps {
			if cmps[i].CampaignID != nil && *cmps[i].CampaignID == campaignID {
				campAlpha = &cmps[i]
				break
			}
		}
		require.NotNil(t, campAlpha, "Matrix S: Campaign Alpha found in campaign metrics")
		assert.Equal(t, "Campaign Alpha", campAlpha.Name)
		assert.Equal(t, 1, campAlpha.Visits, "Matrix U: 1 campaign session")
		assert.Equal(t, 1, campAlpha.PaidOrders, "Matrix V: 1 campaign paid order")
		assert.Equal(t, int64(2500), campAlpha.RevenueCents, "Matrix W: 2500 revenue cents")
	})

	// Matrix Y & Z: Previous period and duration equality
	t.Run("MatrixY_Z_PreviousPeriodService", func(t *testing.T) {
		res, err := svc.GetOverview(ctx, startOfRange, now)
		require.NoError(t, err)
		assert.Equal(t, 4, res.Current.PaidOrders)
		assert.Equal(t, 1, res.Previous.PaidOrders) // 1 order in previous period
		assert.Equal(t, int64(1000), res.Previous.RevenueCents)
	})

	// Matrix AA: Custom half-open boundary [from, to)
	t.Run("MatrixAA_HalfOpenBoundary", func(t *testing.T) {
		// Event exactly at boundary 'now' must be excluded
		exactBoundaryID := uuid.New()
		_, err := db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, started_at, last_seen_at) VALUES ($1, $2, $3, $3)`, exactBoundaryID, uuid.New(), now)
		require.NoError(t, err)

		snap, err := repo.GetOverviewMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		assert.Equal(t, 3, snap.Visits, "Matrix AA: Session at exact 'to' boundary must NOT be included in [from, to)")
	})

	// Matrix AB: Created earlier / paid in range
	t.Run("MatrixAB_CreatedEarlier_PaidInRange", func(t *testing.T) {
		// orderCreatedEarlierID had created_at in prevStart, but paid_at in startOfRange.
		// It is already included in snap.PaidOrders = 4.
		snap, err := repo.GetOverviewMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		assert.True(t, snap.PaidOrders >= 4, "Matrix AB: Order paid in range is counted regardless of creation date")
	})

	// Matrix AC: Historical attribution immutable after campaign rename
	t.Run("MatrixAC_HistoricalAttributionImmutable", func(t *testing.T) {
		// Rename campaign
		_, err := db.Exec(ctx, `UPDATE marketing_campaigns SET title = 'Renamed Campaign' WHERE id = $1`, campaignID)
		require.NoError(t, err)

		cmps, err := repo.GetCampaignMetrics(ctx, startOfRange, now)
		require.NoError(t, err)
		var campRenamed *marketing.RawCampaignMetrics
		for i := range cmps {
			if cmps[i].CampaignID != nil && *cmps[i].CampaignID == campaignID {
				campRenamed = &cmps[i]
				break
			}
		}
		require.NotNil(t, campRenamed)
		assert.Equal(t, "Renamed Campaign", campRenamed.Name)
		assert.Equal(t, 1, campRenamed.PaidOrders, "Matrix AC: Attribution remains intact via campaign_id")
		assert.Equal(t, int64(2500), campRenamed.RevenueCents)
	})

	// Matrix AD: HTTP handler returns 200 OK
	t.Run("MatrixAD_HTTPHandler", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/marketing/analytics/overview?from="+startOfRange.Format(time.RFC3339)+"&to="+now.Format(time.RFC3339), nil)
		rec := httptest.NewRecorder()
		handler.GetAnalyticsOverview(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var res marketing.AnalyticsOverviewResponse
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		require.NoError(t, err)
		assert.Equal(t, 4, res.Current.PaidOrders)
	})
}
