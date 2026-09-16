package router_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func setupPersonalizationRouterTestEnv(t *testing.T) (context.Context, http.Handler, func(), *postgres.Client, *auth.TokenService) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping router integration test")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://127.0.0.1:3000"},
		},
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
	}

	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)

	var dbName string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "tests must strictly run against zamk_test")

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)

	cleanup := func() {
		cancel()
		redisClient.Close()
		pgClient.Close()
	}

	return ctx, r, cleanup, pgClient, tokenService
}

func TestPersonalization_CustomerProductViewRoute(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	// Fixtures
	customerAID := uuid.New()
	customerBID := uuid.New()
	sellerUserID := uuid.New()
	adminUserID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()
	publishedProdID := uuid.New()
	pubVariantID := uuid.New()
	zeroStockProdID := uuid.New()
	zeroStockVariantID := uuid.New()
	unpublishedProdID := uuid.New()
	unpubVariantID := uuid.New()

	now := time.Now()

	createTestUser := func(id uuid.UUID, role string) string {
		email := fmt.Sprintf("%s-%s@zamk.local", role, id.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test User', 'hash', $4, 'active', now(), now())
		`, id, email, "+7999"+id.String()[:7], role)
		require.NoError(t, err)

		tok, err := tokenService.GenerateAccessToken(id, email, role)
		require.NoError(t, err)
		return tok
	}

	tokenCustomerA := createTestUser(customerAID, "customer")
	tokenCustomerB := createTestUser(customerBID, "customer")
	tokenSeller := createTestUser(sellerUserID, "seller")
	tokenAdmin := createTestUser(adminUserID, "admin")

	// Create seller
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'View Test Brand', $2, $3, 'active', now(), now())
	`, sellerID, "view-brand-"+sellerID.String()[:8], "view-"+sellerID.String()[:8]+"@test.local")
	require.NoError(t, err)

	// Create category
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'View Category', $2, true, now(), now())
	`, catID, "view-cat-"+catID.String()[:8])
	require.NoError(t, err)

	// Create published product with free stock = 5 >= 2
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Published Coat', $4, 150000, 'RUB', 'published', $5, $5, $5, $5, $5)
	`, publishedProdID, sellerID, catID, "pub-coat-"+publishedProdID.String()[:8], now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, 150000, true, $5, $5)
	`, pubVariantID, publishedProdID, "SKU-"+pubVariantID.String()[:8], "BC-"+pubVariantID.String()[:8], now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 5, 0, now(), now())
	`, uuid.New(), publishedProdID, pubVariantID, sellerID)
	require.NoError(t, err)

	// Create published product with 0 stock (free stock < 2 -> inaccessible)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Zero Stock Coat', $4, 150000, 'RUB', 'published', $5, $5, $5, $5, $5)
	`, zeroStockProdID, sellerID, catID, "zero-coat-"+zeroStockProdID.String()[:8], now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, 150000, true, $5, $5)
	`, zeroStockVariantID, zeroStockProdID, "SKU-"+zeroStockVariantID.String()[:8], "BC-"+zeroStockVariantID.String()[:8], now)
	require.NoError(t, err)

	// Create unpublished product (pending_moderation)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			submitted_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Pending Coat', $4, 150000, 'RUB', 'pending_moderation', $5, $5, $5)
	`, unpublishedProdID, sellerID, catID, "pending-coat-"+unpublishedProdID.String()[:8], now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, 150000, true, $5, $5)
	`, unpubVariantID, unpublishedProdID, "SKU-"+unpubVariantID.String()[:8], "BC-"+unpubVariantID.String()[:8], now)
	require.NoError(t, err)

	// Clean up all test fixtures afterwards
	defer func() {
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM customer_product_views WHERE user_id IN ($1, $2)", customerAID, customerBID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id IN ($1, $2, $3)", publishedProdID, zeroStockProdID, unpublishedProdID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id IN ($1, $2, $3)", publishedProdID, zeroStockProdID, unpublishedProdID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id IN ($1, $2, $3)", publishedProdID, zeroStockProdID, unpublishedProdID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", catID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2, $3, $4)", customerAID, customerBID, sellerUserID, adminUserID)
	}()

	sendView := func(token, productID string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer/products/%s/view", productID), nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Result()
	}

	t.Run("D. unauthenticated request -> 401", func(t *testing.T) {
		res := sendView("", publishedProdID.String())
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	})

	t.Run("D2. non-customer role -> 403", func(t *testing.T) {
		resSeller := sendView(tokenSeller, publishedProdID.String())
		assert.Equal(t, http.StatusForbidden, resSeller.StatusCode)

		resAdmin := sendView(tokenAdmin, publishedProdID.String())
		assert.Equal(t, http.StatusForbidden, resAdmin.StatusCode)
	})

	t.Run("E. invalid product id -> 400 Bad Request", func(t *testing.T) {
		res := sendView(tokenCustomerA, "not-a-valid-uuid")
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("F. nonexistent product -> 404 Not Found, no view row created", func(t *testing.T) {
		fakeID := uuid.New().String()
		res := sendView(tokenCustomerA, fakeID)
		assert.Equal(t, http.StatusNotFound, res.StatusCode)

		var count int
		err := pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, fakeID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("G1. unpublished product -> 404 Not Found, no view row created", func(t *testing.T) {
		res := sendView(tokenCustomerA, unpublishedProdID.String())
		assert.Equal(t, http.StatusNotFound, res.StatusCode)

		var count int
		err := pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, unpublishedProdID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("G2. zero-stock product -> 404 Not Found, no view row created", func(t *testing.T) {
		res := sendView(tokenCustomerA, zeroStockProdID.String())
		assert.Equal(t, http.StatusNotFound, res.StatusCode)

		var count int
		err := pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, zeroStockProdID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("A. Customer A views Product X -> 200 OK, view_count = 1", func(t *testing.T) {
		res := sendView(tokenCustomerA, publishedProdID.String())
		assert.Equal(t, http.StatusOK, res.StatusCode)

		var viewCount int64
		var lastViewed time.Time
		err := pgClient.Pool.QueryRow(ctx, "SELECT view_count, last_viewed_at FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, publishedProdID).Scan(&viewCount, &lastViewed)
		require.NoError(t, err)
		assert.Equal(t, int64(1), viewCount)
		assert.False(t, lastViewed.IsZero())
	})

	t.Run("B. Customer A views Product X again -> 200 OK, view_count = 2, last_viewed_at updated", func(t *testing.T) {
		var firstViewed time.Time
		err := pgClient.Pool.QueryRow(ctx, "SELECT last_viewed_at FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, publishedProdID).Scan(&firstViewed)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		res := sendView(tokenCustomerA, publishedProdID.String())
		assert.Equal(t, http.StatusOK, res.StatusCode)

		var viewCount int64
		var secondViewed time.Time
		err = pgClient.Pool.QueryRow(ctx, "SELECT view_count, last_viewed_at FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, publishedProdID).Scan(&viewCount, &secondViewed)
		require.NoError(t, err)
		assert.Equal(t, int64(2), viewCount)
		assert.True(t, secondViewed.After(firstViewed) || secondViewed.Equal(firstViewed))

		// Ensure strictly one row exists for A/X
		var count int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, publishedProdID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("C. Customer B views same Product X -> separate B/X row, no cross-user overwrite", func(t *testing.T) {
		res := sendView(tokenCustomerB, publishedProdID.String())
		assert.Equal(t, http.StatusOK, res.StatusCode)

		var viewCountB int64
		err := pgClient.Pool.QueryRow(ctx, "SELECT view_count FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerBID, publishedProdID).Scan(&viewCountB)
		require.NoError(t, err)
		assert.Equal(t, int64(1), viewCountB)

		// Customer A row untouched
		var viewCountA int64
		err = pgClient.Pool.QueryRow(ctx, "SELECT view_count FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, publishedProdID).Scan(&viewCountA)
		require.NoError(t, err)
		assert.Equal(t, int64(2), viewCountA)
	})

	t.Run("H. concurrent repeated calls do not create duplicate rows", func(t *testing.T) {
		concurrency := 8
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				_ = sendView(tokenCustomerA, publishedProdID.String())
			}()
		}
		wg.Wait()

		var totalRows int
		err := pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, publishedProdID).Scan(&totalRows)
		require.NoError(t, err)
		assert.Equal(t, 1, totalRows)

		var finalCount int64
		err = pgClient.Pool.QueryRow(ctx, "SELECT view_count FROM customer_product_views WHERE user_id = $1 AND product_id = $2", customerAID, publishedProdID).Scan(&finalCount)
		require.NoError(t, err)
		assert.Equal(t, int64(2+concurrency), finalCount)
	})

	t.Run("I. recording a view does NOT mutate product status, stock, or order/cart state", func(t *testing.T) {
		// Snapshot state before
		var statusBefore string
		err := pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", publishedProdID).Scan(&statusBefore)
		require.NoError(t, err)

		var totalStockBefore, reservedStockBefore int
		err = pgClient.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_id = $1", publishedProdID).Scan(&totalStockBefore, &reservedStockBefore)
		require.NoError(t, err)

		var cartItemCountBefore int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM cart_items ci JOIN carts c ON ci.cart_id = c.id WHERE c.user_id = $1", customerAID).Scan(&cartItemCountBefore)
		require.NoError(t, err)

		var orderCountBefore int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE user_id = $1", customerAID).Scan(&orderCountBefore)
		require.NoError(t, err)

		// Record view
		res := sendView(tokenCustomerA, publishedProdID.String())
		assert.Equal(t, http.StatusOK, res.StatusCode)

		// Verify state after
		var statusAfter string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", publishedProdID).Scan(&statusAfter)
		require.NoError(t, err)
		assert.Equal(t, statusBefore, statusAfter)

		var totalStockAfter, reservedStockAfter int
		err = pgClient.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_id = $1", publishedProdID).Scan(&totalStockAfter, &reservedStockAfter)
		require.NoError(t, err)
		assert.Equal(t, totalStockBefore, totalStockAfter)
		assert.Equal(t, reservedStockBefore, reservedStockAfter)

		var cartItemCountAfter int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM cart_items ci JOIN carts c ON ci.cart_id = c.id WHERE c.user_id = $1", customerAID).Scan(&cartItemCountAfter)
		require.NoError(t, err)
		assert.Equal(t, cartItemCountBefore, cartItemCountAfter)

		var orderCountAfter int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE user_id = $1", customerAID).Scan(&orderCountAfter)
		require.NoError(t, err)
		assert.Equal(t, orderCountBefore, orderCountAfter)
	})

	// GetRecentlyViewedProducts auth tests
	getRecent := func(token string) *http.Response {
		req := httptest.NewRequest("GET", "/api/customer/products/recently-viewed", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Result()
	}

	t.Run("J. recently viewed: anonymous -> 401", func(t *testing.T) {
		res := getRecent("")
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	})

	t.Run("K. recently viewed: seller -> 403", func(t *testing.T) {
		res := getRecent(tokenSeller)
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("L. recently viewed: admin -> 403", func(t *testing.T) {
		res := getRecent(tokenAdmin)
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("M. recently viewed: customer -> 200", func(t *testing.T) {
		res := getRecent(tokenCustomerA)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	// Similar products public route tests
	getSimilar := func(productID string, token string) *http.Response {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/public/products/%s/similar", productID), nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Result()
	}

	t.Run("N. similar products: anonymous -> 200", func(t *testing.T) {
		res := getSimilar(publishedProdID.String(), "")
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("O. similar products: customer -> 200", func(t *testing.T) {
		res := getSimilar(publishedProdID.String(), tokenCustomerA)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("P. similar products: seller -> 200", func(t *testing.T) {
		res := getSimilar(publishedProdID.String(), tokenSeller)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("Q. similar products: admin -> 200", func(t *testing.T) {
		res := getSimilar(publishedProdID.String(), tokenAdmin)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("R. similar products: invalid UUID -> 400", func(t *testing.T) {
		res := getSimilar("not-a-valid-uuid", "")
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("S. similar products: nonexistent product -> 404", func(t *testing.T) {
		res := getSimilar(uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("T. similar products: source with free stock = 1 -> 404, free stock = 2 -> 200", func(t *testing.T) {
		// Stock = 1 (free < 2 -> 404)
		_, err := pgClient.Pool.Exec(ctx, "UPDATE inventory_items SET total_stock = 1 WHERE product_id = $1", publishedProdID)
		require.NoError(t, err)

		res := getSimilar(publishedProdID.String(), "")
		assert.Equal(t, http.StatusNotFound, res.StatusCode)

		// Stock = 2 (free >= 2 -> 200)
		_, err = pgClient.Pool.Exec(ctx, "UPDATE inventory_items SET total_stock = 2 WHERE product_id = $1", publishedProdID)
		require.NoError(t, err)

		res2 := getSimilar(publishedProdID.String(), "")
		assert.Equal(t, http.StatusOK, res2.StatusCode)

		// Restore original stock
		_, err = pgClient.Pool.Exec(ctx, "UPDATE inventory_items SET total_stock = 5 WHERE product_id = $1", publishedProdID)
		require.NoError(t, err)
	})

	// For you candidate engine customer route auth tests
	getForYou := func(token string, query ...string) *http.Response {
		url := "/api/customer/products/for-you"
		if len(query) > 0 && query[0] != "" {
			url += "?" + query[0]
		}
		req := httptest.NewRequest("GET", url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Result()
	}

	t.Run("U. for you: anonymous -> 401", func(t *testing.T) {
		res := getForYou("")
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	})

	t.Run("V. for you: seller -> 403", func(t *testing.T) {
		res := getForYou(tokenSeller)
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("W. for you: admin -> 403", func(t *testing.T) {
		res := getForYou(tokenAdmin)
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("X. for you: customer -> 200", func(t *testing.T) {
		res := getForYou(tokenCustomerA)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("Y. for you: customer_id in query is ignored", func(t *testing.T) {
		res := getForYou(tokenCustomerA, "customer_id="+uuid.New().String())
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})
}
