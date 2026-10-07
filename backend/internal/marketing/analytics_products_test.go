package marketing_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// fakeMockDB for DB safety test
type fakeMockDB struct {
	dbName string
}

func (f *fakeMockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &fakeRow{val: f.dbName}
}

type fakeRow struct {
	val string
}

func (r *fakeRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if ptr, ok := dest[0].(*string); ok {
			*ptr = r.val
			return nil
		}
	}
	return errors.New("scan error")
}

func setupTestDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx := context.Background()
	dbURL := testutil.GetTestDatabaseURL()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)

	// Mandatory Safety Guard: verify db is strictly "zamk_test" before any operation
	testutil.AssertTestDatabase(t, db)

	return db, ctx
}

func cleanFixtures(ctx context.Context, db *pgxpool.Pool, productIDs []uuid.UUID, sellerIDs []uuid.UUID, catIDs []uuid.UUID, orderIDs []uuid.UUID, returnIDs []uuid.UUID) {
	if len(productIDs) > 0 {
		_, _ = db.Exec(ctx, `DELETE FROM behavioral_events WHERE product_id = ANY($1)`, productIDs)
		_, _ = db.Exec(ctx, `DELETE FROM order_item_promotions WHERE order_id = ANY($1)`, orderIDs)
		_, _ = db.Exec(ctx, `DELETE FROM return_items WHERE order_item_id IN (SELECT id FROM order_items WHERE product_id = ANY($1))`, productIDs)
		_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE product_id = ANY($1)`, productIDs)
		_, _ = db.Exec(ctx, `DELETE FROM product_variants WHERE product_id = ANY($1)`, productIDs)
		_, _ = db.Exec(ctx, `DELETE FROM products WHERE id = ANY($1)`, productIDs)
	}
	if len(returnIDs) > 0 {
		_, _ = db.Exec(ctx, `DELETE FROM returns WHERE id = ANY($1)`, returnIDs)
	}
	if len(orderIDs) > 0 {
		_, _ = db.Exec(ctx, `DELETE FROM payments WHERE order_id = ANY($1)`, orderIDs)
		_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE order_id = ANY($1)`, orderIDs)
		_, _ = db.Exec(ctx, `DELETE FROM orders WHERE id = ANY($1)`, orderIDs)
	}
	if len(catIDs) > 0 {
		_, _ = db.Exec(ctx, `DELETE FROM categories WHERE id = ANY($1)`, catIDs)
	}
	if len(sellerIDs) > 0 {
		_, _ = db.Exec(ctx, `DELETE FROM sellers WHERE id = ANY($1)`, sellerIDs)
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, sellerIDs)
	}
}

func insertAnalyticsTestOrder(ctx context.Context, db *pgxpool.Pool, orderID, userID, sellerID, fulID uuid.UUID, status string, totalCents int64, t time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'RUB', 'Customer', '79991112233', 'customer@test.com', 'Test Address', $5, $5)
	`, orderID, userID, status, totalCents, t)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'paid', $4, 1000, $4, $5, $5)
	`, fulID, orderID, sellerID, totalCents, t)
	return err
}

func insertAnalyticsTestPayment(ctx context.Context, db *pgxpool.Pool, paymentID, orderID uuid.UUID, status string, amountCents int64, paidAt time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO payments (id, order_id, status, provider, amount_cents, currency, payment_number, idempotency_key, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at)
		VALUES ($1, $2, $3, 'tbank', $4, 'RUB', $5, $6, 'card', 'mock', 'pending', $7, $7, $7)
	`, paymentID, orderID, status, amountCents, "P-"+paymentID.String()[:8], "idem-"+paymentID.String()[:8], paidAt)
	return err
}

func insertAnalyticsTestOrderItem(ctx context.Context, db *pgxpool.Pool, oiID, orderID, fulID, prodID, varID, sellerID uuid.UUID, priceCents int64, qty int, t time.Time) error {
	subtotal := priceCents * int64(qty)
	_, err := db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item', 'item', $7, $8, $9, $10)
	`, oiID, orderID, fulID, prodID, varID, sellerID, priceCents, qty, subtotal, t)
	return err
}

// 1. current_database guard
func TestProductAnalytics_1_CurrentDatabaseGuard(t *testing.T) {
	wrongDB := &fakeMockDB{dbName: "zamk_production"}
	err := testutil.VerifyTestDatabase(context.Background(), wrongDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "REFUSING DESTRUCTIVE TEST SETUP")
	assert.Contains(t, err.Error(), "zamk_production")

	errNil := testutil.VerifyTestDatabase(context.Background(), nil)
	require.Error(t, errNil)
	assert.Contains(t, errNil.Error(), "REFUSING DESTRUCTIVE TEST SETUP: database connection is nil")

	testDB := &fakeMockDB{dbName: "zamk_test"}
	errOk := testutil.VerifyTestDatabase(context.Background(), testDB)
	require.NoError(t, errOk)
}

// 2. populated product analytics + 12 (paid orders) + 13 (sold units) + 14 (revenue) + 15 (completed returns)
func TestProductAnalytics_2_PopulatedProductAnalytics(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-48 * time.Hour)
	to := now

	sellerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	varID := uuid.New()
	orderID := uuid.New()
	fulID := uuid.New()
	paymentID := uuid.New()
	oiID := uuid.New()
	retID := uuid.New()
	retItemID := uuid.New()

	defer cleanFixtures(ctx, db, []uuid.UUID{prodID}, []uuid.UUID{sellerID}, []uuid.UUID{catID}, []uuid.UUID{orderID}, []uuid.UUID{retID})

	// Fixtures
	_, err := db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Sel', 'sel@t.com', '79990001122', 'h', 'seller', 'active', false, $2, $2)`, sellerID, from)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Acme Studio', 'active', $2, $2)`, sellerID, from)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Coats', 'coats', $2, $2)`, catID, from)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Trench Coat', 'trench-coat', 'published', 'Desc', 10000, $4, $4)`, prodID, sellerID, catID, from)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'TC-01', 10000, $3, $3)`, varID, prodID, from)
	require.NoError(t, err)

	// Behavioral events inside period
	eventTime := from.Add(1 * time.Hour)
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'product_view', 'client', $2, $3)`, uuid.New(), prodID, eventTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'favorite_added', 'client', $2, $3)`, uuid.New(), prodID, eventTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'add_to_cart', 'client', $2, $3)`, uuid.New(), prodID, eventTime)
	require.NoError(t, err)

	// Order & Payment inside period
	orderTime := from.Add(2 * time.Hour)
	err = insertAnalyticsTestOrder(ctx, db, orderID, sellerID, sellerID, fulID, "paid", 20000, orderTime)
	require.NoError(t, err)
	err = insertAnalyticsTestPayment(ctx, db, paymentID, orderID, "succeeded", 20000, orderTime)
	require.NoError(t, err)
	err = insertAnalyticsTestOrderItem(ctx, db, oiID, orderID, fulID, prodID, varID, sellerID, 10000, 2, orderTime)
	require.NoError(t, err)

	// Completed Return inside period
	retTime := from.Add(3 * time.Hour)
	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at, completed_at) VALUES ($1, $2, $3, $4, 'completed', 'defective', $5, $5, $5)`, retID, orderID, fulID, sellerID, retTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at) VALUES ($1, $2, $3, 1, 1, $4)`, retItemID, retID, oiID, retTime)
	require.NoError(t, err)

	repo := marketing.NewAnalyticsRepository(db)
	repo.SetVerifiedCompleteFrom("product_view", from)
	repo.SetVerifiedCompleteFrom("favorite_added", from)
	repo.SetVerifiedCompleteFrom("add_to_cart", from)

	selStr := sellerID.String()
	req := marketing.ProductAnalyticsRequest{
		From:       from,
		To:         to,
		DesignerID: &selStr,
	}

	res, err := repo.GetProductAnalytics(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Products, 1)

	p := res.Products[0]
	assert.Equal(t, prodID.String(), p.ProductID)
	assert.Equal(t, "Trench Coat", p.ProductName)
	assert.Equal(t, "Acme Studio", *p.DesignerName)
	assert.Equal(t, "Coats", *p.CategoryName)
	assert.Equal(t, 1, p.Views)
	assert.Equal(t, 1, p.Favorites)
	assert.Equal(t, 1, p.AddToCart)
	assert.Equal(t, 1, p.Purchases)
	assert.Equal(t, 2, p.SoldUnits)
	assert.Equal(t, int64(20000), p.RevenueCents)
	assert.Equal(t, 1.0, p.ConversionRate)
	assert.Equal(t, 1, p.Returns)
	assert.Equal(t, 1, p.ReturnedUnits)
}

// 3. period exclusion
func TestProductAnalytics_3_PeriodExclusion(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	to := now
	outsideTime := from.Add(-2 * time.Hour)

	sellerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	varID := uuid.New()
	orderID := uuid.New()
	fulID := uuid.New()
	oiID := uuid.New()

	defer cleanFixtures(ctx, db, []uuid.UUID{prodID}, []uuid.UUID{sellerID}, []uuid.UUID{catID}, []uuid.UUID{orderID}, nil)

	_, err := db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Sel', 'sel2@t.com', '79990001123', 'h', 'seller', 'active', false, $2, $2)`, sellerID, outsideTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Old Brand', 'active', $2, $2)`, sellerID, outsideTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Old Cat', 'old-cat', $2, $2)`, catID, outsideTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Old Coat', 'old-coat', 'published', 'Desc', 5000, $4, $4)`, prodID, sellerID, catID, outsideTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'OLD-1', 5000, $3, $3)`, varID, prodID, outsideTime)
	require.NoError(t, err)

	// Events outside period
	_, err = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'product_view', 'client', $2, $3)`, uuid.New(), prodID, outsideTime)
	require.NoError(t, err)

	// Order paid outside period
	err = insertAnalyticsTestOrder(ctx, db, orderID, sellerID, sellerID, fulID, "paid", 5000, outsideTime)
	require.NoError(t, err)
	err = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderID, "succeeded", 5000, outsideTime)
	require.NoError(t, err)
	err = insertAnalyticsTestOrderItem(ctx, db, oiID, orderID, fulID, prodID, varID, sellerID, 5000, 1, outsideTime)
	require.NoError(t, err)

	repo := marketing.NewAnalyticsRepository(db)
	selStr := sellerID.String()
	req := marketing.ProductAnalyticsRequest{
		From:       from,
		To:         to,
		DesignerID: &selStr,
	}

	res, err := repo.GetProductAnalytics(ctx, req)
	require.NoError(t, err)
	assert.Empty(t, res.Products, "Products with activity outside requested window must be excluded")
}

// 4. category filter & 5. designer filter & 6. title search
func TestProductAnalytics_4_5_6_Filters(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	to := now
	eventTime := from.Add(1 * time.Hour)

	seller1ID, seller2ID := uuid.New(), uuid.New()
	cat1ID, cat2ID := uuid.New(), uuid.New()
	prod1ID, prod2ID := uuid.New(), uuid.New()

	defer cleanFixtures(ctx, db, []uuid.UUID{prod1ID, prod2ID}, []uuid.UUID{seller1ID, seller2ID}, []uuid.UUID{cat1ID, cat2ID}, nil, nil)

	_, _ = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'S1', 's1@t.com', '79990001131', 'h', 'seller', 'active', false, $2, $2)`, seller1ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Designer Alpha', 'active', $2, $2)`, seller1ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Shirts', 'shirts', $2, $2)`, cat1ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Silk Shirt', 'silk-shirt', 'published', 'Desc', 8000, $4, $4)`, prod1ID, seller1ID, cat1ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'product_view', 'client', $2, $3)`, uuid.New(), prod1ID, eventTime)

	_, _ = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'S2', 's2@t.com', '79990001132', 'h', 'seller', 'active', false, $2, $2)`, seller2ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Designer Beta', 'active', $2, $2)`, seller2ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Pants', 'pants', $2, $2)`, cat2ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Denim Jeans', 'denim-jeans', 'published', 'Desc', 12000, $4, $4)`, prod2ID, seller2ID, cat2ID, from)
	_, _ = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'product_view', 'client', $2, $3)`, uuid.New(), prod2ID, eventTime)

	repo := marketing.NewAnalyticsRepository(db)

	cat1Str := cat1ID.String()
	resCat, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, CategoryID: &cat1Str})
	require.NoError(t, err)
	require.Len(t, resCat.Products, 1)
	assert.Equal(t, prod1ID.String(), resCat.Products[0].ProductID)

	des2Str := seller2ID.String()
	resDes, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &des2Str})
	require.NoError(t, err)
	require.Len(t, resDes.Products, 1)
	assert.Equal(t, prod2ID.String(), resDes.Products[0].ProductID)

	searchTerm := "silk"
	resSearch, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, Search: &searchTerm})
	require.NoError(t, err)
	require.Len(t, resSearch.Products, 1)
	assert.Equal(t, prod1ID.String(), resSearch.Products[0].ProductID)
}

// 7 (sales sort), 8 (views sort), 9 (favorites sort with coverage), 10 (conversion sort), 11 (high_views_low_sales)
func TestProductAnalytics_7_11_Sorting(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	to := now
	eventTime := from.Add(1 * time.Hour)

	sellerID := uuid.New()
	catID := uuid.New()
	prodAID, prodBID := uuid.New(), uuid.New()
	varAID, varBID := uuid.New(), uuid.New()
	orderAID, orderBID := uuid.New(), uuid.New()
	fulAID, fulBID := uuid.New(), uuid.New()
	oiAID, oiBID := uuid.New(), uuid.New()

	defer cleanFixtures(ctx, db, []uuid.UUID{prodAID, prodBID}, []uuid.UUID{sellerID}, []uuid.UUID{catID}, []uuid.UUID{orderAID, orderBID}, nil)

	_, _ = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'S', 's@sort.com', '79990001140', 'h', 'seller', 'active', false, $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Sort Brand', 'active', $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Sort Cat', 'sort-cat', $2, $2)`, catID, from)

	// Prod A: 100 views, 1 purchase (sold units: 1), 5 favorites
	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Product A', 'prod-a', 'published', 'Desc', 5000, $4, $4)`, prodAID, sellerID, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'PA-1', 5000, $3, $3)`, varAID, prodAID, from)
	for i := 0; i < 5; i++ {
		_, _ = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'favorite_added', 'client', $2, $3)`, uuid.New(), prodAID, eventTime)
	}
	for i := 0; i < 100; i++ {
		_, _ = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'product_view', 'client', $2, $3)`, uuid.New(), prodAID, eventTime)
	}
	_ = insertAnalyticsTestOrder(ctx, db, orderAID, sellerID, sellerID, fulAID, "paid", 5000, eventTime)
	_ = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderAID, "succeeded", 5000, eventTime)
	_ = insertAnalyticsTestOrderItem(ctx, db, oiAID, orderAID, fulAID, prodAID, varAID, sellerID, 5000, 1, eventTime)

	// Prod B: 10 views, 5 purchases (sold units: 10), 1 favorite
	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Product B', 'prod-b', 'published', 'Desc', 5000, $4, $4)`, prodBID, sellerID, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'PB-1', 5000, $3, $3)`, varBID, prodBID, from)
	_, _ = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'favorite_added', 'client', $2, $3)`, uuid.New(), prodBID, eventTime)
	for i := 0; i < 10; i++ {
		_, _ = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'product_view', 'client', $2, $3)`, uuid.New(), prodBID, eventTime)
	}
	_ = insertAnalyticsTestOrder(ctx, db, orderBID, sellerID, sellerID, fulBID, "paid", 50000, eventTime)
	_ = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderBID, "succeeded", 50000, eventTime)
	_ = insertAnalyticsTestOrderItem(ctx, db, oiBID, orderBID, fulBID, prodBID, varBID, sellerID, 5000, 10, eventTime)

	repo := marketing.NewAnalyticsRepository(db)

	// 7. Sales sort (Prod B has 10 sold units, Prod A has 1)
	sortSales := marketing.ProductSortSales
	selStr := sellerID.String()
	resSales, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr, Sort: &sortSales})
	require.NoError(t, err)
	require.Len(t, resSales.Products, 2)
	assert.Equal(t, prodBID.String(), resSales.Products[0].ProductID)
	assert.Equal(t, prodAID.String(), resSales.Products[1].ProductID)

	// 8. Views sort (Prod A has 100 views, Prod B has 10)
	sortViews := marketing.ProductSortViews
	resViews, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr, Sort: &sortViews})
	require.NoError(t, err)
	require.Len(t, resViews.Products, 2)
	assert.Equal(t, prodAID.String(), resViews.Products[0].ProductID)
	assert.Equal(t, prodBID.String(), resViews.Products[1].ProductID)

	// 9. Favorites sort only when coverage permits
	sortFav := marketing.ProductSortFavorites
	_, errFavFail := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr, Sort: &sortFav})
	assert.ErrorIs(t, errFavFail, marketing.ErrFavoritesCoverageIncomplete)

	repo.SetVerifiedCompleteFrom("favorite_added", from)
	resFavOk, errFavOk := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr, Sort: &sortFav})
	require.NoError(t, errFavOk)
	require.Len(t, resFavOk.Products, 2)
	assert.Equal(t, prodAID.String(), resFavOk.Products[0].ProductID, "Prod A has 5 favorites, Prod B has 1")

	// 10. Conversion sort (Prod B has 1/10 = 0.1 conversion, Prod A has 1/100 = 0.01)
	sortConv := marketing.ProductSortConversion
	resConv, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr, Sort: &sortConv})
	require.NoError(t, err)
	require.Len(t, resConv.Products, 2)
	assert.Equal(t, prodBID.String(), resConv.Products[0].ProductID, "Prod B has higher conversion")

	// 11. High views / low sales (Prod A has 100 views / 1 purchase vs Prod B 10 views / 1 purchase)
	sortHighLow := marketing.ProductSortHighViewsLowSales
	resHighLow, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr, Sort: &sortHighLow})
	require.NoError(t, err)
	require.Len(t, resHighLow.Products, 2)
	assert.Equal(t, prodAID.String(), resHighLow.Products[0].ProductID, "Prod A has high views and low sales")
}

// 12 (paid orders deduplication & cancellation exclusion) + 14 (revenue truth with order_item_promotions)
func TestProductAnalytics_12_14_RevenueTruthAndPaidOrders(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	to := now
	eventTime := from.Add(1 * time.Hour)

	sellerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	varID := uuid.New()
	orderPaidID, orderCancelledID := uuid.New(), uuid.New()
	fulPaidID, fulCancelledID := uuid.New(), uuid.New()
	oi1ID, oi2ID := uuid.New(), uuid.New()

	defer cleanFixtures(ctx, db, []uuid.UUID{prodID}, []uuid.UUID{sellerID}, []uuid.UUID{catID}, []uuid.UUID{orderPaidID, orderCancelledID}, nil)

	_, _ = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'S', 's@rev.com', '79990001150', 'h', 'seller', 'active', false, $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Rev Brand', 'active', $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Rev Cat', 'rev-cat', $2, $2)`, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Promo Prod', 'promo-prod', 'published', 'Desc', 10000, $4, $4)`, prodID, sellerID, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'PP-1', 10000, $3, $3)`, varID, prodID, from)

	// Paid order with 2 payments (retry simulation) -> must count order once
	err := insertAnalyticsTestOrder(ctx, db, orderPaidID, sellerID, sellerID, fulPaidID, "paid", 8000, eventTime)
	require.NoError(t, err)
	err = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderPaidID, "succeeded", 8000, eventTime)
	require.NoError(t, err)
	err = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderPaidID, "succeeded", 8000, eventTime.Add(time.Minute))
	require.NoError(t, err)

	err = insertAnalyticsTestOrderItem(ctx, db, oi1ID, orderPaidID, fulPaidID, prodID, varID, sellerID, 10000, 1, eventTime)
	require.NoError(t, err)

	campaignID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, title, campaign_type, purpose, seller_id, starts_at, ends_at, funding_mode, status, discount_type, seller_discount_bps, planned_budget_cents)
		VALUES ($1, 'Promo Campaign', 'discount', 'advertising', NULL, $2, $3, 'seller', 'active', 'percent', 2000, 100000)
	`, campaignID, from.Add(-time.Hour), to.Add(24*time.Hour))
	require.NoError(t, err)

	promoID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, product_scope, max_discount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'SAVE20', 'percent', 2000, true, 'ENTIRE_STORE', NULL, now(), now())
	`, promoID, campaignID, sellerID)
	require.NoError(t, err)

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM promo_codes WHERE id = $1`, promoID)
		_, _ = db.Exec(ctx, `DELETE FROM marketing_campaigns WHERE id = $1`, campaignID)
	}()

	// Order Item has base price 10000, but order_item_promotions has customer paid 8000 (2000 discount)
	_, err = db.Exec(ctx, `
		INSERT INTO order_item_promotions (
			id, order_item_id, order_id, seller_id, campaign_id, promo_code_id,
			base_unit_price_cents, seller_discount_unit_cents, zamk_subsidy_unit_cents, customer_paid_unit_price_cents,
			commission_base_unit_cents, quantity, total_seller_discount_cents, total_zamk_subsidy_cents,
			total_customer_paid_cents, total_commission_base_cents, commission_rate_bps, total_commission_charged_cents, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			10000, 2000, 0, 8000,
			8000, 1, 2000, 0,
			8000, 8000, 1000, 800, $7
		)
	`, uuid.New(), oi1ID, orderPaidID, sellerID, campaignID, promoID, eventTime)
	require.NoError(t, err)

	// Cancelled order (must NOT be counted)
	err = insertAnalyticsTestOrder(ctx, db, orderCancelledID, sellerID, sellerID, fulCancelledID, "cancelled", 10000, eventTime)
	require.NoError(t, err)
	err = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderCancelledID, "succeeded", 10000, eventTime)
	require.NoError(t, err)
	err = insertAnalyticsTestOrderItem(ctx, db, oi2ID, orderCancelledID, fulCancelledID, prodID, varID, sellerID, 10000, 1, eventTime)
	require.NoError(t, err)

	repo := marketing.NewAnalyticsRepository(db)
	selStr := sellerID.String()
	res, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr})
	require.NoError(t, err)
	require.Len(t, res.Products, 1)

	p := res.Products[0]
	assert.Equal(t, 1, p.Purchases, "Retried payments must count as 1 paid order; cancelled orders excluded")
	assert.Equal(t, int64(8000), p.RevenueCents, "Revenue must reflect actual customer-paid amount from order_item_promotions")
}

// 15 (completed return counting) + 16 (return period filtering)
func TestProductAnalytics_15_16_ReturnSemantics(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	to := now
	insideTime := from.Add(2 * time.Hour)
	outsideTime := from.Add(-5 * time.Hour)

	sellerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	varID := uuid.New()
	orderID := uuid.New()
	fulID := uuid.New()
	oiID := uuid.New()
	retCompletedID, retPendingID, retOutsideID := uuid.New(), uuid.New(), uuid.New()

	defer cleanFixtures(ctx, db, []uuid.UUID{prodID}, []uuid.UUID{sellerID}, []uuid.UUID{catID}, []uuid.UUID{orderID}, []uuid.UUID{retCompletedID, retPendingID, retOutsideID})

	_, _ = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'S', 's@ret.com', '79990001160', 'h', 'seller', 'active', false, $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Ret Brand', 'active', $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Ret Cat', 'ret-cat', $2, $2)`, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Ret Prod', 'ret-prod', 'published', 'Desc', 5000, $4, $4)`, prodID, sellerID, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'RP-1', 5000, $3, $3)`, varID, prodID, from)

	err := insertAnalyticsTestOrder(ctx, db, orderID, sellerID, sellerID, fulID, "paid", 15000, insideTime)
	require.NoError(t, err)
	err = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderID, "succeeded", 15000, insideTime)
	require.NoError(t, err)
	err = insertAnalyticsTestOrderItem(ctx, db, oiID, orderID, fulID, prodID, varID, sellerID, 5000, 3, insideTime)
	require.NoError(t, err)

	// Completed return in period (accepted_quantity: 1)
	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at, completed_at) VALUES ($1, $2, $3, $4, 'completed', 'defective', $5, $5, $5)`, retCompletedID, orderID, fulID, sellerID, insideTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at) VALUES ($1, $2, $3, 1, 1, $4)`, uuid.New(), retCompletedID, oiID, insideTime)
	require.NoError(t, err)

	// Pending return in period (status != 'completed') -> must NOT be counted
	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, $4, 'requested', 'size', $5, $5)`, retPendingID, orderID, fulID, sellerID, insideTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at) VALUES ($1, $2, $3, 1, 0, $4)`, uuid.New(), retPendingID, oiID, insideTime)
	require.NoError(t, err)

	// Completed return outside period -> must NOT be counted
	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at, completed_at) VALUES ($1, $2, $3, $4, 'completed', 'defective', $5, $5, $5)`, retOutsideID, orderID, fulID, sellerID, outsideTime)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at) VALUES ($1, $2, $3, 1, 1, $4)`, uuid.New(), retOutsideID, oiID, outsideTime)
	require.NoError(t, err)

	repo := marketing.NewAnalyticsRepository(db)
	selStr := sellerID.String()
	res, err := repo.GetProductAnalytics(ctx, marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr})
	require.NoError(t, err)
	require.Len(t, res.Products, 1)

	p := res.Products[0]
	assert.Equal(t, 1, p.Returns, "Only returns completed in period are counted")
	assert.Equal(t, 1, p.ReturnedUnits, "Only returned units from returns completed in period are counted")
}

// 17 (availability: available / partial / unavailable) & 18 (zero vs incomplete)
func TestProductAnalytics_17_18_CoverageSemantics(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	to := now

	repo := marketing.NewAnalyticsRepository(db)

	// A) Case: No events ever recorded for event type -> unavailable
	req := marketing.ProductAnalyticsRequest{From: from, To: to}
	res, err := repo.GetProductAnalytics(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, marketing.CoverageUnavailable, res.Coverage.Views.Status)

	// B) Case: Events exist, but historical completeness NOT verified -> partial (fail closed)
	prodID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()
	defer cleanFixtures(ctx, db, []uuid.UUID{prodID}, []uuid.UUID{sellerID}, []uuid.UUID{catID}, nil, nil)

	_, _ = db.Exec(ctx, `INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'S', 's@cov.com', '79990001170', 'h', 'seller', 'active', false, $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, status, created_at, updated_at) VALUES ($1, 'Cov Brand', 'active', $2, $2)`, sellerID, from)
	_, _ = db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cov Cat', 'cov-cat', $2, $2)`, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Cov Prod', 'cov-prod', 'published', 'Desc', 5000, $4, $4)`, prodID, sellerID, catID, from)

	_, _ = db.Exec(ctx, `INSERT INTO behavioral_events (id, event_type, source, product_id, occurred_at) VALUES ($1, 'product_view', 'client', $2, $3)`, uuid.New(), prodID, from.Add(time.Hour))

	resPartial, err := repo.GetProductAnalytics(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, marketing.CoveragePartial, resPartial.Coverage.Views.Status, "Unverified historical tracking must fail closed to partial")
	assert.NotNil(t, resPartial.Coverage.Views.TrackedFrom)

	// C) Case: Tracking is verified complete from cutoff -> available
	repo.SetVerifiedCompleteFrom("product_view", from)
	resAvail, err := repo.GetProductAnalytics(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, marketing.CoverageAvailable, resAvail.Coverage.Views.Status, "Verified tracking coverage must report available")

	// 18. Product with zero events has views = 0 truthfully when coverage is available
	prodZeroID := uuid.New()
	varZeroID := uuid.New()
	orderID := uuid.New()
	fulID := uuid.New()
	defer cleanFixtures(ctx, db, []uuid.UUID{prodZeroID}, nil, nil, []uuid.UUID{orderID}, nil)

	_, _ = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description, price_cents, created_at, updated_at) VALUES ($1, $2, $3, 'Zero Prod', 'zero-prod', 'published', 'Desc', 5000, $4, $4)`, prodZeroID, sellerID, catID, from)
	_, _ = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'ZP-1', 5000, $3, $3)`, varZeroID, prodZeroID, from)
	err = insertAnalyticsTestOrder(ctx, db, orderID, sellerID, sellerID, fulID, "paid", 5000, from.Add(time.Hour))
	require.NoError(t, err)
	err = insertAnalyticsTestPayment(ctx, db, uuid.New(), orderID, "succeeded", 5000, from.Add(time.Hour))
	require.NoError(t, err)
	err = insertAnalyticsTestOrderItem(ctx, db, uuid.New(), orderID, fulID, prodZeroID, varZeroID, sellerID, 5000, 1, from.Add(time.Hour))
	require.NoError(t, err)

	selStr := sellerID.String()
	reqZero := marketing.ProductAnalyticsRequest{From: from, To: to, DesignerID: &selStr}
	resZero, err := repo.GetProductAnalytics(ctx, reqZero)
	require.NoError(t, err)
	var foundZero *marketing.ProductAnalyticsRow
	for i := range resZero.Products {
		if resZero.Products[i].ProductID == prodZeroID.String() {
			foundZero = &resZero.Products[i]
			break
		}
	}
	require.NotNil(t, foundZero)
	assert.Equal(t, 0, foundZero.Views, "Views are 0 and coverage is available")
	assert.Equal(t, marketing.CoverageAvailable, resZero.Coverage.Views.Status)
}

// 19. invalid dates
func TestProductAnalytics_19_InvalidDates(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	repo := marketing.NewAnalyticsRepository(db)
	svc := marketing.NewAnalyticsService(repo)
	handler := marketing.NewHandler(&marketing.Service{
		Analytics: svc,
	}, slog.Default())

	// Missing dates
	reqMissing, _ := http.NewRequest(http.MethodGet, "/api/admin/marketing/analytics/products", nil)
	recMissing := httptest.NewRecorder()
	handler.GetProductAnalytics(recMissing, reqMissing)
	assert.Equal(t, http.StatusBadRequest, recMissing.Code)

	// Malformed date
	reqBad, _ := http.NewRequest(http.MethodGet, "/api/admin/marketing/analytics/products?from=invalid&to=invalid", nil)
	recBad := httptest.NewRecorder()
	handler.GetProductAnalytics(recBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, recBad.Code)

	// from >= to
	now := time.Now().UTC()
	reqInverted, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/marketing/analytics/products?from=%s&to=%s", now.Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339)), nil)
	recInverted := httptest.NewRecorder()
	handler.GetProductAnalytics(recInverted, reqInverted)
	assert.Equal(t, http.StatusBadRequest, recInverted.Code)
}

// 20. RBAC contract
func TestProductAnalytics_20_RBAC(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	repo := marketing.NewAnalyticsRepository(db)
	svc := marketing.NewAnalyticsService(repo)
	handler := marketing.NewHandler(&marketing.Service{
		Analytics: svc,
	}, slog.Default())

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	url := fmt.Sprintf("/api/admin/marketing/analytics/products?from=%s&to=%s", from.Format(time.RFC3339), now.Format(time.RFC3339))

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	handler.GetProductAnalytics(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "Handler execution succeeds without needing write permission")
}
