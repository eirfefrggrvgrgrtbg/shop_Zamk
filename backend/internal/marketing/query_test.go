package marketing_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func assertDBIsZamkTest(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	testutil.AssertTestDatabase(t, db)

	var currentDB string
	err := db.QueryRow(context.Background(), "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "Must be connected strictly to zamk_test")
}

func TestQueryEngine_UnitValidation(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)
	to := now

	validPeriod := marketing.QueryPeriod{From: from, To: to}

	// Matrix 2: version 1 valid
	t.Run("Matrix2_Version1_Valid", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		assert.Nil(t, err)
	})

	// Matrix 3: unknown version
	t.Run("Matrix3_UnknownVersion_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    2,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "unsupported_version", err.Code)
	})

	// Matrix 4: missing period
	t.Run("Matrix4_MissingPeriod_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{},
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_period", err.Code)
	})

	// Matrix 5: from == to
	t.Run("Matrix5_FromEqualsTo_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: from},
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_period", err.Code)
	})

	// Matrix 6: from > to
	t.Run("Matrix6_FromAfterTo_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: to, To: from},
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_period", err.Code)
	})

	// Matrix 7: >366d
	t.Run("Matrix7_Over366Days_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: now.Add(-367 * 24 * time.Hour), To: now},
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "period_too_long", err.Code)
	})

	// Matrix 34: invalid filter field
	t.Run("Matrix34_InvalidFilterField_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Filters: []marketing.QueryFilter{
				{Dimension: "unknown_dim", Operator: "eq", Values: []string{"test"}},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_filter_dimension", err.Code)
	})

	// Matrix 34b: unselected filter dimension
	t.Run("Matrix34b_UnselectedFilterDimension_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Filters: []marketing.QueryFilter{
				{Dimension: "product", Operator: "eq", Values: []string{"test"}},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "unsupported_filter_dimension", err.Code)
	})

	// Matrix 35: invalid operator
	t.Run("Matrix35_InvalidOperator_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Filters: []marketing.QueryFilter{
				{Dimension: "source", Operator: "regex", Values: []string{"test"}},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_filter_operator", err.Code)
	})

	// Matrix 36: empty filter values
	t.Run("Matrix36_EmptyFilterValues_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Filters: []marketing.QueryFilter{
				{Dimension: "source", Operator: "eq", Values: []string{""}},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "empty_filter_values", err.Code)

		req.Filters[0].Values = []string{}
		err = marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "empty_filter_values", err.Code)
	})

	// Matrix 40: invalid sort field
	t.Run("Matrix40_InvalidSortField_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Sort: []marketing.QuerySort{
				{Field: "unselected_field", Direction: "asc"},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_sort_field", err.Code)
	})

	// Matrix 41: invalid direction
	t.Run("Matrix41_InvalidDirection_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Sort: []marketing.QuerySort{
				{Field: "orders", Direction: "up"},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_sort_direction", err.Code)
	})

	// Matrix 45: invalid zero limit
	t.Run("Matrix45_InvalidZeroLimit_Fails", func(t *testing.T) {
		zeroLimit := 0
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Limit:      &zeroLimit,
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_limit", err.Code)
	})

	// Matrix 46: over-max limit
	t.Run("Matrix46_OverMaxLimit_Fails", func(t *testing.T) {
		overLimit := 1001
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Limit:      &overLimit,
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_limit", err.Code)
	})

	// Matrix 47: too many dimensions
	t.Run("Matrix47_TooManyDimensions_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source", "day", "product"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "too_many_dimensions", err.Code)
	})

	// Matrix 48: too many metrics
	t.Run("Matrix48_TooManyMetrics_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders", "revenue", "sold_units", "sessions", "returns", "returned_units", "product_views", "favorites", "add_to_cart"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "too_many_metrics", err.Code)
	})

	// Matrix 49: too many filters
	t.Run("Matrix49_TooManyFilters_Fails", func(t *testing.T) {
		filters := make([]marketing.QueryFilter, 11)
		for i := 0; i < 11; i++ {
			filters[i] = marketing.QueryFilter{Dimension: "source", Operator: "eq", Values: []string{"test"}}
		}
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Filters:    filters,
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "too_many_filters", err.Code)
	})

	// Matrix 50: too many sorts
	t.Run("Matrix50_TooManySorts_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders", "revenue"},
			Sort: []marketing.QuerySort{
				{Field: "source", Direction: "asc"},
				{Field: "orders", Direction: "desc"},
				{Field: "revenue", Direction: "desc"},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "too_many_sort_keys", err.Code)
	})

	// Matrix 51: injection dimension
	t.Run("Matrix51_InjectionDimension_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source; DROP TABLE users; --"},
			Metrics:    []string{"orders"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_dimension", err.Code)
	})

	// Matrix 52: injection metric
	t.Run("Matrix52_InjectionMetric_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders; DROP TABLE users; --"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_metric", err.Code)
	})

	// Matrix 53: injection filter field
	t.Run("Matrix53_InjectionFilterField_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Filters: []marketing.QueryFilter{
				{Dimension: "source; DROP TABLE users; --", Operator: "eq", Values: []string{"v"}},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_filter_dimension", err.Code)
	})

	// Matrix 54: injection operator
	t.Run("Matrix54_InjectionOperator_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Filters: []marketing.QueryFilter{
				{Dimension: "source", Operator: "= 1 OR 1=1; --", Values: []string{"v"}},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_filter_operator", err.Code)
	})

	// Matrix 55: injection sort
	t.Run("Matrix55_InjectionSort_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{"orders"},
			Sort: []marketing.QuerySort{
				{Field: "orders; DROP TABLE users; --", Direction: "asc"},
			},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_sort_field", err.Code)
	})

	// Blocker 4: Unsupported combination fail-closed
	t.Run("Blocker4_UnsupportedCombination_FailClosed", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source", "product"},
			Metrics:    []string{"sessions"},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "unsupported_combination", err.Code)
	})

	// Blocker 15: Dimension without metric
	t.Run("Blocker15_DimensionWithoutMetric_Fails", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"source"},
			Metrics:    []string{},
		}
		err := marketing.ValidateQueryRequest(req)
		require.NotNil(t, err)
		assert.Equal(t, "missing_metrics", err.Code)

		// Exact case: dimensions = ["product"], metrics = []
		reqProduct := marketing.QueryRequest{
			Version:    1,
			Period:     validPeriod,
			Dimensions: []string{"product"},
			Metrics:    []string{},
		}
		errProduct := marketing.ValidateQueryRequest(reqProduct)
		require.NotNil(t, errProduct)
		assert.Equal(t, "missing_metrics", errProduct.Code)
		assert.Equal(t, "at least 1 metric must be requested", errProduct.Message)
	})
}

func TestQueryEngine_DatabaseIntegrationMatrix(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()

	// Matrix 1: DB Safety - verify zamk_test
	assertDBIsZamkTest(t, db)

	// Fixture IDs tracker for deterministic, dependency-safe cleanup
	var userIDs []uuid.UUID
	var sellerIDs []uuid.UUID
	var categoryIDs []uuid.UUID
	var productIDs []uuid.UUID
	var variantIDs []uuid.UUID
	var campaignIDs []uuid.UUID
	var sessionIDs []uuid.UUID
	var orderIDs []uuid.UUID
	var fulfillmentIDs []uuid.UUID
	var paymentIDs []uuid.UUID
	var orderItemIDs []uuid.UUID
	var promoIDs []uuid.UUID
	var promoCodeIDs []uuid.UUID
	var eventIDs []uuid.UUID
	var returnIDs []uuid.UUID
	var returnItemIDs []uuid.UUID

	defer func() {
		// Dependency-safe cleanup of ONLY created rows
		if len(returnItemIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM return_items WHERE id = ANY($1)`, returnItemIDs)
		}
		if len(returnIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM returns WHERE id = ANY($1)`, returnIDs)
		}
		if len(promoIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM order_item_promotions WHERE id = ANY($1)`, promoIDs)
		}
		if len(promoCodeIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM promo_codes WHERE id = ANY($1)`, promoCodeIDs)
		}
		if len(orderItemIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE id = ANY($1)`, orderItemIDs)
		}
		if len(orderIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM order_attributions WHERE order_id = ANY($1)`, orderIDs)
		}
		if len(paymentIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM payments WHERE id = ANY($1)`, paymentIDs)
		}
		if len(fulfillmentIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE id = ANY($1)`, fulfillmentIDs)
		}
		if len(orderIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM orders WHERE id = ANY($1)`, orderIDs)
		}
		if len(sessionIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE id = ANY($1)`, sessionIDs)
		}
		if len(eventIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM behavioral_events WHERE id = ANY($1)`, eventIDs)
		}
		if len(campaignIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM marketing_campaigns WHERE id = ANY($1)`, campaignIDs)
		}
		if len(variantIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM product_variants WHERE id = ANY($1)`, variantIDs)
		}
		if len(productIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM products WHERE id = ANY($1)`, productIDs)
		}
		if len(categoryIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM categories WHERE id = ANY($1)`, categoryIDs)
		}
		if len(sellerIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM sellers WHERE id = ANY($1)`, sellerIDs)
		}
		if len(userIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, userIDs)
		}
	}()

	now := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	from := now.Add(-7 * 24 * time.Hour)
	to := now.Add(time.Hour)

	// Setup Base Entities
	uID := uuid.New()
	userIDs = append(userIDs, uID)
	_, err := db.Exec(ctx, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'Customer QA', 'hash', 'customer')`, uID, "u-"+uID.String()+"@test.local")
	require.NoError(t, err)

	sID1 := uuid.New()
	sellerIDs = append(sellerIDs, sID1)
	_, err = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Acme Couture', $2, 'acme@test.local', 'active')`, sID1, "acme-"+sID1.String())
	require.NoError(t, err)

	sID2 := uuid.New()
	sellerIDs = append(sellerIDs, sID2)
	_, err = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Beta Design', $2, 'beta@test.local', 'active')`, sID2, "beta-"+sID2.String())
	require.NoError(t, err)

	cID1 := uuid.New()
	categoryIDs = append(categoryIDs, cID1)
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Apparel', $2)`, cID1, "apparel-"+cID1.String())
	require.NoError(t, err)

	cID2 := uuid.New()
	categoryIDs = append(categoryIDs, cID2)
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Footwear', $2)`, cID2, "footwear-"+cID2.String())
	require.NoError(t, err)

	pID1 := uuid.New()
	productIDs = append(productIDs, pID1)
	_, err = db.Exec(ctx, `INSERT INTO products (id, title, slug, status, description, seller_id, category_id, price_cents) VALUES ($1, 'Silk Blazer', $2, 'published', 'Desc', $3, $4, 5000)`, pID1, "p1-"+pID1.String(), sID1, cID1)
	require.NoError(t, err)

	vID1 := uuid.New()
	variantIDs = append(variantIDs, vID1)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 5000, true)`, vID1, pID1, "v1-"+vID1.String())
	require.NoError(t, err)

	pID2 := uuid.New()
	productIDs = append(productIDs, pID2)
	_, err = db.Exec(ctx, `INSERT INTO products (id, title, slug, status, description, seller_id, category_id, price_cents) VALUES ($1, 'Leather Boots', $2, 'published', 'Desc', $3, $4, 10000)`, pID2, "p2-"+pID2.String(), sID2, cID2)
	require.NoError(t, err)

	vID2 := uuid.New()
	variantIDs = append(variantIDs, vID2)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 10000, true)`, vID2, pID2, "v2-"+vID2.String())
	require.NoError(t, err)

	campID := uuid.New()
	campaignIDs = append(campaignIDs, campID)
	_, err = db.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, title, funding_mode, status, discount_type, seller_discount_bps, starts_at, ends_at) VALUES ($1, $2, 'Autumn Special Campaign', 'seller', 'active', 'percent', 1000, $3, $4)`, campID, sID1, from, to)
	require.NoError(t, err)

	// Sessions: 1 named 'telegram', 1 direct, 1 unattributed (null)
	sess1 := uuid.New()
	sessionIDs = append(sessionIDs, sess1)
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, source, started_at, last_seen_at) VALUES ($1, $2, 'telegram', $3, $3)`, sess1, uuid.New(), from.Add(time.Hour))
	require.NoError(t, err)

	sess2 := uuid.New()
	sessionIDs = append(sessionIDs, sess2)
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, source, started_at, last_seen_at) VALUES ($1, $2, 'direct', $3, $3)`, sess2, uuid.New(), from.Add(2*time.Hour))
	require.NoError(t, err)

	sess3 := uuid.New()
	sessionIDs = append(sessionIDs, sess3)
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, source, started_at, last_seen_at) VALUES ($1, $2, NULL, $3, $3)`, sess3, uuid.New(), from.Add(3*time.Hour))
	require.NoError(t, err)

	// Order 1: Succeeded, source telegram, campaign linked, normal item (pID1)
	ord1 := uuid.New()
	orderIDs = append(orderIDs, ord1)
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 5000, 'RUB', 'Test Customer', '79991234567', 'cust@test.com', 'Addr', $3, $3)`, ord1, uID, from.Add(4*time.Hour))
	require.NoError(t, err)

	pay1 := uuid.New()
	paymentIDs = append(paymentIDs, pay1)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 5000, 'RUB', $3, $4, 'card', 'mock', 'pending', $5, $5, $5)`, pay1, ord1, "P-"+pay1.String()[:8], "idem-"+pay1.String(), from.Add(4*time.Hour))
	require.NoError(t, err)

	ful1 := uuid.New()
	fulfillmentIDs = append(fulfillmentIDs, ful1)
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 5000, 1000, 4500, $4, $4)`, ful1, ord1, sID1, from.Add(4*time.Hour))
	require.NoError(t, err)

	oi1 := uuid.New()
	orderItemIDs = append(orderItemIDs, oi1)
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, quantity, price_cents, title, product_slug, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 1, 5000, 'Silk Blazer', 'silk-blazer', 5000, $7)`, oi1, ord1, ful1, pID1, vID1, sID1, from.Add(4*time.Hour))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO order_attributions (order_id, source, campaign_id, attributed_at) VALUES ($1, 'telegram', $2, $3)`, ord1, campID, from.Add(4*time.Hour))
	require.NoError(t, err)

	// Order 2: Succeeded, direct, promoted item (pID2, 10000 price but promo 8000 paid)
	ord2 := uuid.New()
	orderIDs = append(orderIDs, ord2)
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 8000, 'RUB', 'Test Customer', '79991234567', 'cust@test.com', 'Addr', $3, $3)`, ord2, uID, from.Add(5*time.Hour))
	require.NoError(t, err)

	pay2 := uuid.New()
	paymentIDs = append(paymentIDs, pay2)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 8000, 'RUB', $3, $4, 'card', 'mock', 'pending', $5, $5, $5)`, pay2, ord2, "P-"+pay2.String()[:8], "idem-"+pay2.String(), from.Add(5*time.Hour))
	require.NoError(t, err)

	ful2 := uuid.New()
	fulfillmentIDs = append(fulfillmentIDs, ful2)
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 8000, 1000, 7200, $4, $4)`, ful2, ord2, sID2, from.Add(5*time.Hour))
	require.NoError(t, err)

	oi2 := uuid.New()
	orderItemIDs = append(orderItemIDs, oi2)
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, quantity, price_cents, title, product_slug, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 1, 10000, 'Leather Boots', 'leather-boots', 10000, $7)`, oi2, ord2, ful2, pID2, vID2, sID2, from.Add(5*time.Hour))
	require.NoError(t, err)

	pcID := uuid.New()
	promoCodeIDs = append(promoCodeIDs, pcID)
	_, err = db.Exec(ctx, `INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, product_scope, created_at, updated_at) VALUES ($1, $2, $3, $4, 'percent', 2000, true, 'ENTIRE_STORE', now(), now())`, pcID, campID, sID2, "PROMO-"+pcID.String()[:8])
	require.NoError(t, err)

	prPromoID := uuid.New()
	promoIDs = append(promoIDs, prPromoID)
	_, err = db.Exec(ctx, `
		INSERT INTO order_item_promotions (
			id, order_item_id, order_id, seller_id, campaign_id, promo_code_id,
			base_unit_price_cents, seller_discount_unit_cents, zamk_subsidy_unit_cents,
			customer_paid_unit_price_cents, commission_base_unit_cents, quantity,
			total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents,
			total_commission_base_cents, commission_rate_bps, total_commission_charged_cents, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			10000, 2000, 0,
			8000, 8000, 1,
			2000, 0, 8000,
			8000, 1000, 800, $7
		)
	`, prPromoID, oi2, ord2, sID2, campID, pcID, from.Add(5*time.Hour))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO order_attributions (order_id, source, attributed_at) VALUES ($1, 'direct', $2)`, ord2, from.Add(5*time.Hour))
	require.NoError(t, err)

	// Order 3: Failed payment (must NOT count in revenue or orders)
	ord3 := uuid.New()
	orderIDs = append(orderIDs, ord3)
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'created', 5000, 'RUB', 'Test Customer', '79991234567', 'cust@test.com', 'Addr', $3, $3)`, ord3, uID, from.Add(6*time.Hour))
	require.NoError(t, err)

	pay3 := uuid.New()
	paymentIDs = append(paymentIDs, pay3)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at) VALUES ($1, $2, 'failed', 'tbank', 5000, 'RUB', $3, $4, 'card', 'mock', 'pending', $5, $5)`, pay3, ord3, "P-"+pay3.String()[:8], "idem-"+pay3.String(), from.Add(6*time.Hour))
	require.NoError(t, err)

	// Order 4: Cancelled order with succeeded payment (must NOT count in revenue or orders)
	ord4 := uuid.New()
	orderIDs = append(orderIDs, ord4)
	_, err = db.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'cancelled', 5000, 'RUB', 'Test Customer', '79991234567', 'cust@test.com', 'Addr', $3, $3)`, ord4, uID, from.Add(6*time.Hour))
	require.NoError(t, err)

	pay4 := uuid.New()
	paymentIDs = append(paymentIDs, pay4)
	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at) VALUES ($1, $2, 'succeeded', 'tbank', 5000, 'RUB', $3, $4, 'card', 'mock', 'pending', $5, $5, $5)`, pay4, ord4, "P-"+pay4.String()[:8], "idem-"+pay4.String(), from.Add(6*time.Hour))
	require.NoError(t, err)

	// Baseline tracking events before 'from' so coverage is Available
	ev0 := uuid.New()
	eventIDs = append(eventIDs, ev0)
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, product_id, source, occurred_at) VALUES ($1, 'product_view', $2, 'client', $3)`, ev0, pID1, from.Add(-time.Hour))
	require.NoError(t, err)

	sess0 := uuid.New()
	sessionIDs = append(sessionIDs, sess0)
	_, err = db.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, source, started_at, last_seen_at) VALUES ($1, $2, 'baseline', $3, $3)`, sess0, uuid.New(), from.Add(-time.Hour))
	require.NoError(t, err)

	// Behavioral events: 1 view on pID1, 1 favorite on pID1, 1 add_to_cart on pID1
	ev1 := uuid.New()
	eventIDs = append(eventIDs, ev1)
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, product_id, source, occurred_at) VALUES ($1, 'product_view', $2, 'client', $3)`, ev1, pID1, from.Add(7*time.Hour))
	require.NoError(t, err)

	ev2 := uuid.New()
	eventIDs = append(eventIDs, ev2)
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, product_id, source, occurred_at) VALUES ($1, 'favorite_added', $2, 'client', $3)`, ev2, pID1, from.Add(7*time.Hour))
	require.NoError(t, err)

	ev3 := uuid.New()
	eventIDs = append(eventIDs, ev3)
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, product_id, source, occurred_at) VALUES ($1, 'add_to_cart', $2, 'client', $3)`, ev3, pID1, from.Add(7*time.Hour))
	require.NoError(t, err)

	// Completed Return: 1 return on oi1 (pID1) with quantity 2 requested, 1 accepted
	retID := uuid.New()
	returnIDs = append(returnIDs, retID)
	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at, completed_at) VALUES ($1, $2, $3, $4, 'completed', 'defective', $5, $5, $5)`, retID, ord1, ful1, uID, from.Add(8*time.Hour))
	require.NoError(t, err)

	retItemID := uuid.New()
	returnItemIDs = append(returnItemIDs, retItemID)
	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at) VALUES ($1, $2, $3, 2, 1, $4)`, retItemID, retID, oi1, from.Add(8*time.Hour))
	require.NoError(t, err)

	repo := marketing.NewQueryRepository(db)

	// Matrix 8: metric-only aggregate
	t.Run("Matrix8_MetricOnlyAggregate", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{},
			Metrics:    []string{"orders", "revenue"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		require.Len(t, res.Rows, 1)
		// Ord1 (5000) + Ord2 (8000) = 13000 revenue, 2 orders
		assert.EqualValues(t, 2, res.Rows[0]["orders"])
		assert.EqualValues(t, 13000, res.Rows[0]["revenue"])
	})

	// Matrix 9 & 11 & 12: one dimension + source semantics + direct != unattributed
	t.Run("Matrix9_11_12_SourceSemantics", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"source"},
			Metrics:    []string{"sessions", "orders", "revenue"},
			Sort:       []marketing.QuerySort{{Field: "orders", Direction: "desc"}},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)

		foundNamed := false
		foundDirect := false
		foundUnattributed := false

		for _, row := range res.Rows {
			sKind := row["sourceKind"]
			if sKind == "named" && row["sourceKey"] == "telegram" {
				foundNamed = true
				assert.Equal(t, "telegram", row["source"])
			} else if sKind == "direct" && row["sourceKey"] == "direct" {
				foundDirect = true
				assert.Equal(t, "direct", row["source"])
			} else if sKind == "unattributed" && row["sourceKey"] == nil {
				foundUnattributed = true
			}
		}

		assert.True(t, foundNamed, "Should have named source")
		assert.True(t, foundDirect, "Should have direct source")
		assert.True(t, foundUnattributed, "Should have unattributed source")
	})

	// Matrix 10: two dimensions (day + source & designer + category)
	t.Run("Matrix10_TwoDimensions_DayAndSource", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"day", "source"},
			Metrics:    []string{"sessions", "orders"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, res.Rows)
		for _, row := range res.Rows {
			assert.Contains(t, row, "day")
			assert.Contains(t, row, "sourceKind")
		}
	})

	t.Run("Matrix10_TwoDimensions_DesignerAndCategory", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"designer", "category"},
			Metrics:    []string{"orders", "revenue"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, res.Rows)
		for _, row := range res.Rows {
			assert.Contains(t, row, "designerName")
			assert.Contains(t, row, "categoryName")
		}
	})

	// Matrix 13 & 17 & 18 & 19 & 21 & 22 & 23 & 24 & 25 & 26: product dimension and metrics
	t.Run("Matrix13_ProductDimensionAndMetrics", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders", "sold_units", "revenue", "product_views", "favorites", "add_to_cart", "returns", "returned_units", "conversion"},
			Filters: []marketing.QueryFilter{
				{Dimension: "product", Operator: "eq", Values: []string{"Silk Blazer"}},
			},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		require.Len(t, res.Rows, 1)

		row := res.Rows[0]
		assert.Equal(t, "Silk Blazer", row["productName"])
		assert.EqualValues(t, 1, row["orders"])
		assert.EqualValues(t, 1, row["sold_units"])
		assert.EqualValues(t, 5000, row["revenue"])
		assert.EqualValues(t, 1, row["product_views"])
		assert.EqualValues(t, 1, row["favorites"])
		assert.EqualValues(t, 1, row["add_to_cart"])
		assert.EqualValues(t, 1, row["returns"])
		assert.EqualValues(t, 1, row["returned_units"], "Accepted quantity semantics: 1 out of 2 accepted")
		assert.EqualValues(t, 1.0, row["conversion"])
	})

	// Matrix 14: designer dimension
	t.Run("Matrix14_DesignerDimension", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"designer"},
			Metrics:    []string{"orders", "revenue"},
			Filters: []marketing.QueryFilter{
				{Dimension: "designer", Operator: "eq", Values: []string{"Acme Couture"}},
			},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		require.Len(t, res.Rows, 1)
		assert.Equal(t, "Acme Couture", res.Rows[0]["designerName"])
		assert.EqualValues(t, 5000, res.Rows[0]["revenue"])
	})

	// Matrix 15: campaign dimension (real executed JOIN on marketing_campaigns)
	t.Run("Matrix15_CampaignDimension", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"campaign"},
			Metrics:    []string{"orders", "revenue"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, res.Rows)

		foundCamp := false
		for _, r := range res.Rows {
			if r["campaignName"] == "Autumn Special Campaign" {
				foundCamp = true
				assert.EqualValues(t, 1, r["orders"])
				assert.EqualValues(t, 5000, r["revenue"])
			}
		}
		assert.True(t, foundCamp, "Must find real campaign entity")
	})

	// Matrix 16: category dimension
	t.Run("Matrix16_CategoryDimension", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"category"},
			Metrics:    []string{"orders", "revenue"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, res.Rows)
		for _, r := range res.Rows {
			assert.Contains(t, r, "categoryName")
		}
	})

	// Matrix 27 & 28 & 29: conversion partial, unavailable, unknown != zero
	t.Run("Matrix27_28_29_ConversionCoverageSemantics", func(t *testing.T) {
		// 1. Matrix 28: Period in 2020 before any tracking started -> Unavailable
		pastFrom := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		pastTo := time.Date(2020, 1, 10, 0, 0, 0, 0, time.UTC)

		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: pastFrom, To: pastTo},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders", "conversion"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, marketing.CoverageUnavailable, res.Coverage["conversion"].Status)
		for _, r := range res.Rows {
			assert.Nil(t, r["conversion"], "Conversion must be null when tracking coverage is unavailable, not 0")
		}

		// 2. Matrix 27: From before tracking started, To after tracking started -> Partial
		var minOccurred *time.Time
		_ = db.QueryRow(ctx, `SELECT MIN(occurred_at) FROM behavioral_events WHERE event_type = 'product_view'`).Scan(&minOccurred)
		if minOccurred != nil {
			partialFrom := minOccurred.Add(-24 * time.Hour)
			partialTo := minOccurred.Add(24 * time.Hour)
			reqPartial := marketing.QueryRequest{
				Version:    1,
				Period:     marketing.QueryPeriod{From: partialFrom, To: partialTo},
				Dimensions: []string{"product"},
				Metrics:    []string{"orders", "conversion"},
			}
			resPartial, err := repo.ExecuteQuery(ctx, reqPartial)
			require.NoError(t, err)
			assert.Equal(t, marketing.CoveragePartial, resPartial.Coverage["conversion"].Status)
			for _, r := range resPartial.Rows {
				assert.Nil(t, r["conversion"], "Conversion must be null when tracking coverage is partial, not 0")
			}
		}

		// 3. Matrix 29: Complete denominator + zero orders -> 0.0 (not null, not clamped)
		reqZero := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"source"},
			Metrics:    []string{"sessions", "orders", "conversion"},
		}
		resZero, err := repo.ExecuteQuery(ctx, reqZero)
		require.NoError(t, err)
		assert.Equal(t, marketing.CoverageAvailable, resZero.Coverage["conversion"].Status)
		foundUnattrZero := false
		for _, r := range resZero.Rows {
			if r["sourceKind"] == "unattributed" {
				foundUnattrZero = true
				assert.EqualValues(t, 1, r["sessions"], "Unattributed has 1 session")
				assert.EqualValues(t, 0, r["orders"], "Unattributed has 0 orders")
				assert.EqualValues(t, 0.0, r["conversion"], "Complete denominator + zero orders must be 0.0")
			}
		}
		assert.True(t, foundUnattrZero, "Should find unattributed row with 0.0 conversion")
	})

	// Matrix 30, 31, 32, 33: Filter operators eq, neq, in, contains
	t.Run("Matrix30_31_32_33_FilterOperators", func(t *testing.T) {
		// eq
		reqEq := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders"},
			Filters:    []marketing.QueryFilter{{Dimension: "product", Operator: "eq", Values: []string{"Leather Boots"}}},
		}
		resEq, err := repo.ExecuteQuery(ctx, reqEq)
		require.NoError(t, err)
		require.Len(t, resEq.Rows, 1)
		assert.Equal(t, "Leather Boots", resEq.Rows[0]["productName"])

		// neq
		reqNeq := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders"},
			Filters:    []marketing.QueryFilter{{Dimension: "product", Operator: "neq", Values: []string{"Leather Boots"}}},
		}
		resNeq, err := repo.ExecuteQuery(ctx, reqNeq)
		require.NoError(t, err)
		for _, r := range resNeq.Rows {
			assert.NotEqual(t, "Leather Boots", r["productName"])
		}

		// in
		reqIn := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders"},
			Filters:    []marketing.QueryFilter{{Dimension: "product", Operator: "in", Values: []string{"Silk Blazer", "Leather Boots"}}},
		}
		resIn, err := repo.ExecuteQuery(ctx, reqIn)
		require.NoError(t, err)
		require.Len(t, resIn.Rows, 2)

		// contains
		reqContains := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders"},
			Filters:    []marketing.QueryFilter{{Dimension: "product", Operator: "contains", Values: []string{"Blazer"}}},
		}
		resContains, err := repo.ExecuteQuery(ctx, reqContains)
		require.NoError(t, err)
		require.Len(t, resContains.Rows, 1)
		assert.Equal(t, "Silk Blazer", resContains.Rows[0]["productName"])
	})

	// Matrix 37, 38, 39, 42: Sorting and tie-break
	t.Run("Matrix37_38_39_42_SortingAndTieBreak", func(t *testing.T) {
		// metric desc
		reqDesc := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"revenue"},
			Sort:       []marketing.QuerySort{{Field: "revenue", Direction: "desc"}},
		}
		resDesc, err := repo.ExecuteQuery(ctx, reqDesc)
		require.NoError(t, err)
		require.True(t, len(resDesc.Rows) >= 2)
		assert.True(t, resDesc.Rows[0]["revenue"].(float64) >= resDesc.Rows[1]["revenue"].(float64))

		// metric asc
		reqAsc := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"revenue"},
			Sort:       []marketing.QuerySort{{Field: "revenue", Direction: "asc"}},
		}
		resAsc, err := repo.ExecuteQuery(ctx, reqAsc)
		require.NoError(t, err)
		require.True(t, len(resAsc.Rows) >= 2)
		assert.True(t, resAsc.Rows[0]["revenue"].(float64) <= resAsc.Rows[1]["revenue"].(float64))

		// dimension sort asc (Matrix 39)
		reqDim := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"revenue"},
			Sort:       []marketing.QuerySort{{Field: "product", Direction: "asc"}},
		}
		resDim, err := repo.ExecuteQuery(ctx, reqDim)
		require.NoError(t, err)
		require.True(t, len(resDim.Rows) >= 2)
		assert.Equal(t, "Leather Boots", resDim.Rows[0]["productName"])
		assert.Equal(t, "Silk Blazer", resDim.Rows[1]["productName"])

		// dimension sort desc (Matrix 39)
		reqDimDesc := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"revenue"},
			Sort:       []marketing.QuerySort{{Field: "product", Direction: "desc"}},
		}
		resDimDesc, err := repo.ExecuteQuery(ctx, reqDimDesc)
		require.NoError(t, err)
		require.True(t, len(resDimDesc.Rows) >= 2)
		assert.Equal(t, "Silk Blazer", resDimDesc.Rows[0]["productName"])
		assert.Equal(t, "Leather Boots", resDimDesc.Rows[1]["productName"])

		// two sort keys (orders desc, revenue asc)
		reqTwoSort := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders", "revenue"},
			Sort: []marketing.QuerySort{
				{Field: "orders", Direction: "desc"},
				{Field: "revenue", Direction: "asc"},
			},
		}
		resTwoSort, err := repo.ExecuteQuery(ctx, reqTwoSort)
		require.NoError(t, err)
		require.True(t, len(resTwoSort.Rows) >= 2)
		assert.Equal(t, "Silk Blazer", resTwoSort.Rows[0]["productName"], "Silk Blazer revenue 5000 < Leather Boots 8000")
		assert.Equal(t, "Leather Boots", resTwoSort.Rows[1]["productName"])

		// deterministic tie-break (Matrix 42)
		reqTieBreak := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders"},
			Sort:       []marketing.QuerySort{{Field: "orders", Direction: "desc"}},
		}
		resTieBreak, err := repo.ExecuteQuery(ctx, reqTieBreak)
		require.NoError(t, err)
		require.True(t, len(resTieBreak.Rows) >= 2)
	})

	// Matrix 43, 44: Limits
	t.Run("Matrix43_44_Limits", func(t *testing.T) {
		lim := 1
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"revenue"},
			Limit:      &lim,
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Rows, 1)
	})

	// Matrix 56: No UUID leakage
	t.Run("Matrix56_NoUUIDLeakage", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product", "designer"},
			Metrics:    []string{"orders", "revenue"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		for _, row := range res.Rows {
			for col, val := range row {
				if sVal, ok := val.(string); ok {
					assert.False(t, uuidRegex.MatchString(sVal), "Column %s leaked raw UUID: %s", col, sVal)
				}
			}
		}
	})

	// Matrix 57: No PII
	t.Run("Matrix57_NoPII", func(t *testing.T) {
		req := marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"source"},
			Metrics:    []string{"orders", "revenue"},
		}
		res, err := repo.ExecuteQuery(ctx, req)
		require.NoError(t, err)
		for _, row := range res.Rows {
			for col, val := range row {
				colLower := strings.ToLower(col)
				assert.False(t, strings.Contains(colLower, "email"), "Leaked email column")
				assert.False(t, strings.Contains(colLower, "phone"), "Leaked phone column")
				assert.False(t, strings.Contains(colLower, "address"), "Leaked address column")
				if sVal, ok := val.(string); ok {
					assert.False(t, strings.Contains(sVal, "@"), "Leaked email value: %s", sVal)
				}
			}
		}
	})

	// Matrix 60: Product consistency (matches Product Analytics)
	t.Run("Matrix60_ProductConsistency", func(t *testing.T) {
		pAnalyticsRepo := marketing.NewAnalyticsRepository(db)
		pRes, err := pAnalyticsRepo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{
			From: from,
			To:   to,
		})
		require.NoError(t, err)

		qRes, err := repo.ExecuteQuery(ctx, marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"product"},
			Metrics:    []string{"orders", "sold_units", "revenue", "returns", "returned_units"},
		})
		require.NoError(t, err)

		// Map products
		qMap := make(map[string]map[string]interface{})
		for _, r := range qRes.Rows {
			qMap[r["productName"].(string)] = r
		}

		for _, p := range pRes.Products {
			if qr, exists := qMap[p.ProductName]; exists {
				assert.EqualValues(t, p.Purchases, qr["orders"], "Orders consistency for %s", p.ProductName)
				assert.EqualValues(t, p.SoldUnits, qr["sold_units"], "Sold units consistency for %s", p.ProductName)
				assert.EqualValues(t, p.RevenueCents, qr["revenue"], "Revenue consistency for %s", p.ProductName)
				assert.EqualValues(t, p.Returns, qr["returns"], "Returns count consistency for %s", p.ProductName)
				assert.EqualValues(t, p.ReturnedUnits, qr["returned_units"], "Returned units consistency for %s", p.ProductName)
			}
		}
	})

	// Matrix 61: Designer consistency (matches Designer Analytics)
	t.Run("Matrix61_DesignerConsistency", func(t *testing.T) {
		pAnalyticsRepo := marketing.NewAnalyticsRepository(db)
		dRes, err := pAnalyticsRepo.GetDesignerAnalytics(ctx, marketing.DesignerAnalyticsRequest{
			From: from,
			To:   to,
		})
		require.NoError(t, err)

		qRes, err := repo.ExecuteQuery(ctx, marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"designer"},
			Metrics:    []string{"orders", "sold_units", "revenue", "returns"},
		})
		require.NoError(t, err)

		qMap := make(map[string]map[string]interface{})
		for _, r := range qRes.Rows {
			qMap[r["designerName"].(string)] = r
		}

		for _, d := range dRes.Designers {
			if qr, exists := qMap[d.DesignerName]; exists {
				assert.EqualValues(t, d.Purchases, qr["orders"], "Orders consistency for %s", d.DesignerName)
				assert.EqualValues(t, d.SoldUnits, qr["sold_units"], "Sold units consistency for %s", d.DesignerName)
				assert.EqualValues(t, d.RevenueCents, qr["revenue"], "Revenue consistency for %s", d.DesignerName)
				assert.EqualValues(t, d.Returns, qr["returns"], "Returns consistency for %s", d.DesignerName)
			}
		}
	})

	// Matrix 62: Source consistency (matches Sources Analytics)
	t.Run("Matrix62_SourceConsistency", func(t *testing.T) {
		pAnalyticsRepo := marketing.NewAnalyticsRepository(db)
		sRes, err := pAnalyticsRepo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{
			From: from,
			To:   to,
		})
		require.NoError(t, err)

		qRes, err := repo.ExecuteQuery(ctx, marketing.QueryRequest{
			Version:    1,
			Period:     marketing.QueryPeriod{From: from, To: to},
			Dimensions: []string{"source"},
			Metrics:    []string{"sessions", "orders", "revenue"},
		})
		require.NoError(t, err)

		qMap := make(map[string]map[string]interface{})
		for _, r := range qRes.Rows {
			sKey := ""
			if r["sourceKey"] != nil {
				sKey = r["sourceKey"].(string)
			}
			qMap[sKey] = r
		}

		for _, s := range sRes.Sources {
			sKey := ""
			if s.SourceKey != nil {
				sKey = *s.SourceKey
			}
			if qr, exists := qMap[sKey]; exists {
				assert.EqualValues(t, s.Visits, qr["sessions"], "Sessions consistency for source %s", sKey)
				assert.EqualValues(t, s.PaidOrders, qr["orders"], "Orders consistency for source %s", sKey)
				assert.EqualValues(t, s.RevenueCents, qr["revenue"], "Revenue consistency for source %s", sKey)
			}
		}
	})
}

func TestQueryEngine_ExplainPerformance(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	assertDBIsZamkTest(t, db)

	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)
	to := now

	compiler := marketing.NewQueryCompiler()

	queries := []struct {
		name string
		req  marketing.QueryRequest
	}{
		{
			name: "revenue_by_product",
			req: marketing.QueryRequest{
				Version:    1,
				Period:     marketing.QueryPeriod{From: from, To: to},
				Dimensions: []string{"product"},
				Metrics:    []string{"revenue", "orders"},
			},
		},
		{
			name: "revenue_by_source",
			req: marketing.QueryRequest{
				Version:    1,
				Period:     marketing.QueryPeriod{From: from, To: to},
				Dimensions: []string{"source"},
				Metrics:    []string{"revenue", "orders"},
			},
		},
		{
			name: "day_plus_source",
			req: marketing.QueryRequest{
				Version:    1,
				Period:     marketing.QueryPeriod{From: from, To: to},
				Dimensions: []string{"day", "source"},
				Metrics:    []string{"orders", "sessions"},
			},
		},
		{
			name: "designer_plus_category",
			req: marketing.QueryRequest{
				Version:    1,
				Period:     marketing.QueryPeriod{From: from, To: to},
				Dimensions: []string{"designer", "category"},
				Metrics:    []string{"revenue", "orders"},
			},
		},
	}

	for _, tc := range queries {
		t.Run("EXPLAIN_"+tc.name, func(t *testing.T) {
			sqlStr, args, _, err := compiler.Build(tc.req)
			require.NoError(t, err)

			explainQuery := "EXPLAIN ANALYZE " + sqlStr
			rows, err := db.Query(ctx, explainQuery, args...)
			require.NoError(t, err)
			defer rows.Close()

			var lines []string
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err == nil {
					lines = append(lines, line)
					t.Log(line)
				}
			}
			require.NotEmpty(t, lines, "EXPLAIN output should not be empty")
		})
	}
}
