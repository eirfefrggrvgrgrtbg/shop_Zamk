package marketing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// TestAnalyticsTrend_CanonicalTruth verifies GetTrendMetrics against the canonical Marketing Overview definition.
// Required cases:
// A. one successful payment
// B. cancelled order excluded
// C. half-open [from, to) upper bound excluded
// D. missing dates zero-filled
// E. multiple days
// F. succeeded-payment retry / duplicate representation does NOT double-count canonical order
// G. sum(daily.paidOrders) == canonical Overview paidOrders for same range
// H. sum(daily.revenueCents) == canonical Overview revenueCents for same range
func TestAnalyticsTrend_CanonicalTruth(t *testing.T) {
	ctx := context.Background()
	dbURL := testutil.GetTestDatabaseURL()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer db.Close()

	// Clean database tables
	_, err = db.Exec(ctx, `TRUNCATE users, sellers, categories, brands, products, product_variants, orders, order_fulfillments, payments, order_items, analytics_sessions CASCADE`)
	require.NoError(t, err)

	// Base 5-day window: Day 1 (00:00 UTC) to Day 6 (00:00 UTC)
	// Day 1: 2026-06-01
	// Day 2: 2026-06-02
	// Day 3: 2026-06-03
	// Day 4: 2026-06-04
	// Day 5: 2026-06-05
	// Boundary 'to': 2026-06-06 00:00:00 UTC
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)

	// Setup Base Catalog Fixtures
	sellerUserID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Seller', 'seller@trend.com', '79991234568', 'hash', 'seller', 'active', false, $2, $2)`, sellerUserID, from)
	require.NoError(t, err)

	sellerID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO sellers (id, status, created_at, updated_at) VALUES ($1, 'active', $2, $2)`, sellerID, from)
	require.NoError(t, err)

	customerID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Customer', 'cust@trend.com', '79991111111', 'hash', 'customer', 'active', false, $2, $2)`, customerID, from)
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', 'cat', $2, $2)`, catID, from)
	require.NoError(t, err)

	brandID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO brands (id, name, slug, created_at, updated_at) VALUES ($1, 'Brand', 'brand', $2, $2)`, brandID, from)
	require.NoError(t, err)

	productID := uuid.New()
	variantID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Product', 'product', 'published', 'Desc', 1000, $5, $5)`, productID, sellerID, catID, brandID, from)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'SKU-1', 1000, $3, $3)`, variantID, productID, from)
	require.NoError(t, err)

	// Helper to insert an order with payment
	createOrder := func(orderID uuid.UUID, status string, amount int64, paidAt *time.Time, paymentStatus string) {
		createdAt := from.Add(time.Hour)
		if paidAt != nil {
			createdAt = *paidAt
		}
		_, err := db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'RUB', 'Customer', '79991111111', 'cust@trend.com', 'Address', $5, $5)`,
			orderID, customerID, status, amount, createdAt)
		require.NoError(t, err)

		fulID := uuid.New()
		_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'paid', $4, 1000, $4, $5, $5)`,
			fulID, orderID, sellerID, amount, createdAt)
		require.NoError(t, err)

		_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'Item', 'item', $7, 1, $7, $8)`,
			uuid.New(), orderID, fulID, productID, variantID, sellerID, amount, createdAt)
		require.NoError(t, err)

		if paidAt != nil {
			_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at)
				VALUES ($1, $2, $3, 'tbank', $4, 'RUB', $5, $6, 'card', 'mock', 'pending', $7, $7, $8)`,
				uuid.New(), orderID, paymentStatus, amount, "PAY-"+uuid.New().String()[:8], "IDEM-"+uuid.New().String()[:8], createdAt, *paidAt)
			require.NoError(t, err)
		}
	}

	// 1. Day 1 (2026-06-01): One normal successful payment (Case A) -> 1 order, 5000 cents
	day1PaidAt := from.Add(10 * time.Hour) // 2026-06-01 10:00 UTC
	orderDay1 := uuid.New()
	createOrder(orderDay1, "paid", 5000, &day1PaidAt, "succeeded")

	// 2. Day 2 (2026-06-02): NO paid orders -> Day 2 must be zero-filled (Case D)

	// 3. Day 3 (2026-06-03):
	// - Order A: Successful payment (3000 cents)
	// - Order B: Cancelled order (4000 cents) with succeeded payment -> Case B: cancelled order MUST be excluded!
	// - Order C: Succeeded-payment retry representation -> Case F: Order C has TWO succeeded payment rows (e.g. idempotent retry/duplicate representation), MUST count as 1 order, 2500 cents!
	day3PaidAt := from.Add(2*24*time.Hour + 8*time.Hour) // 2026-06-03 08:00 UTC
	orderDay3A := uuid.New()
	createOrder(orderDay3A, "paid", 3000, &day3PaidAt, "succeeded")

	orderDay3Cancelled := uuid.New()
	createOrder(orderDay3Cancelled, "cancelled", 4000, &day3PaidAt, "succeeded")

	orderDay3Retry := uuid.New()
	createOrder(orderDay3Retry, "paid", 2500, &day3PaidAt, "succeeded")
	// Add SECOND succeeded payment for the same order (duplicate / retry representation)
	retryPaidAt := day3PaidAt.Add(5 * time.Minute)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at)
		VALUES ($1, $2, 'succeeded', 'tbank', 2500, 'RUB', $3, $4, 'card', 'mock', 'pending', $5, $5, $5)`,
		uuid.New(), orderDay3Retry, "PAY-RETRY", "IDEM-RETRY", retryPaidAt)
	require.NoError(t, err)

	// 4. Day 4 (2026-06-04): NO paid orders -> Day 4 must be zero-filled (Case D)

	// 5. Day 5 (2026-06-05): Order D (10000 cents)
	day5PaidAt := from.Add(4*24*time.Hour + 14*time.Hour) // 2026-06-05 14:00 UTC
	orderDay5 := uuid.New()
	createOrder(orderDay5, "paid", 10000, &day5PaidAt, "succeeded")

	// 6. Upper boundary: Order paid exactly at `to` boundary (2026-06-06 00:00:00 UTC) -> Case C: MUST BE EXCLUDED from [from, to)!
	orderAtBoundary := uuid.New()
	createOrder(orderAtBoundary, "paid", 9999, &to, "succeeded")

	repo := marketing.NewAnalyticsRepository(db)

	trend, err := repo.GetTrendMetrics(ctx, from, to)
	require.NoError(t, err)

	// Verify length: 5 days (2026-06-01 through 2026-06-05)
	require.Len(t, trend, 5, "Case E: Exactly 5 daily buckets for 5-day range")

	// Expected daily data:
	// Day 1 (2026-06-01): 1 order, 5000 cents (Case A)
	assert.Equal(t, "2026-06-01", trend[0].Date)
	assert.Equal(t, 1, trend[0].PaidOrders, "Case A: Day 1 has 1 order")
	assert.Equal(t, int64(5000), trend[0].RevenueCents, "Case A: Day 1 has 5000 revenue")

	// Day 2 (2026-06-02): 0 orders, 0 cents (Case D zero-filled)
	assert.Equal(t, "2026-06-02", trend[1].Date)
	assert.Equal(t, 0, trend[1].PaidOrders, "Case D: Day 2 is zero-filled")
	assert.Equal(t, int64(0), trend[1].RevenueCents, "Case D: Day 2 has 0 revenue")

	// Day 3 (2026-06-03):
	// - OrderDay3A (3000)
	// - OrderDay3Cancelled (excluded! Case B)
	// - OrderDay3Retry (2500, duplicate payment not double-counted! Case F)
	// Total for Day 3: 2 orders, 5500 cents
	assert.Equal(t, "2026-06-03", trend[2].Date)
	assert.Equal(t, 2, trend[2].PaidOrders, "Case B & F: Day 3 excludes cancelled and deduplicates retry (2 orders)")
	assert.Equal(t, int64(5500), trend[2].RevenueCents, "Case B & F: Day 3 has 3000 + 2500 = 5500 revenue")

	// Day 4 (2026-06-04): 0 orders, 0 cents (Case D zero-filled)
	assert.Equal(t, "2026-06-04", trend[3].Date)
	assert.Equal(t, 0, trend[3].PaidOrders, "Case D: Day 4 is zero-filled")
	assert.Equal(t, int64(0), trend[3].RevenueCents, "Case D: Day 4 has 0 revenue")

	// Day 5 (2026-06-05): 1 order, 10000 cents
	assert.Equal(t, "2026-06-05", trend[4].Date)
	assert.Equal(t, 1, trend[4].PaidOrders, "Day 5 has 1 order")
	assert.Equal(t, int64(10000), trend[4].RevenueCents, "Day 5 has 10000 revenue")

	// Verify Case C: Order at exact `to` (2026-06-06 00:00) is NOT included in any day
	var totalTrendPaidOrders int
	var totalTrendRevenueCents int64
	for _, pt := range trend {
		totalTrendPaidOrders += pt.PaidOrders
		totalTrendRevenueCents += pt.RevenueCents
	}
	assert.Equal(t, 4, totalTrendPaidOrders, "Case C: 4 total paid orders (boundary order excluded)")
	assert.Equal(t, int64(20500), totalTrendRevenueCents, "Case C: 20500 total revenue (boundary order excluded)")

	// Verify Cases G & H: Canonical Overview equality
	overview, err := repo.GetOverviewMetrics(ctx, from, to)
	require.NoError(t, err)

	assert.Equal(t, overview.PaidOrders, totalTrendPaidOrders, "Case G: sum(daily.paidOrders) == canonical Overview paidOrders")
	assert.Equal(t, overview.RevenueCents, totalTrendRevenueCents, "Case H: sum(daily.revenueCents) == canonical Overview revenueCents")
}
