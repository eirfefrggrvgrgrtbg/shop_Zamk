package selleranalytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAnalytics_CrossSellerAccessControl(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)
	handler := NewHandler(svc, repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sellerA := insertSeller(t, db)
	sellerB := insertSeller(t, db)

	var userA, userB uuid.UUID
	err := db.QueryRow(context.Background(), "SELECT user_id FROM seller_users WHERE seller_id = $1", sellerA).Scan(&userA)
	require.NoError(t, err)
	err = db.QueryRow(context.Background(), "SELECT user_id FROM seller_users WHERE seller_id = $1", sellerB).Scan(&userB)
	require.NoError(t, err)

	now := time.Now().UTC()
	fromStr := now.Add(-24 * time.Hour).Format(time.RFC3339)
	toStr := now.Add(24 * time.Hour).Format(time.RFC3339)
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)
	tz := "Europe/Moscow"

	// Create Product A owned by Seller A with 2 variants
	pA := uuid.New()
	catID := uuid.New()
	db.Exec(context.Background(), "INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat A', $2) ON CONFLICT DO NOTHING", catID, catID.String())
	_, err = db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Product A', $4, 'published', 150000)", pA, sellerA, catID, pA.String())
	require.NoError(t, err)

	vA1 := uuid.New()
	vA2 := uuid.New()
	_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-A1', 'M', 'Black', 150000, true)", vA1, pA)
	require.NoError(t, err)
	_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-A2', 'L', 'White', 150000, true)", vA2, pA)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 50, 0)", uuid.New(), pA, vA1, sellerA)
	require.NoError(t, err)
	_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 30, 0)", uuid.New(), pA, vA2, sellerA)
	require.NoError(t, err)

	// Create Product B owned by Seller B with private SKU and secret variant details
	pB := uuid.New()
	_, err = db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Secret Product B', $4, 'published', 250000)", pB, sellerB, catID, pB.String())
	require.NoError(t, err)

	vB1 := uuid.New()
	_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SECRET-SKU-B1', 'XL', 'Red', 250000, true)", vB1, pB)
	require.NoError(t, err)
	_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 999, 0)", uuid.New(), pB, vB1, sellerB)
	require.NoError(t, err)

	// Add sales for both products
	insertOrderWithLedger(t, db, sellerA, pA, vA1, 150000, 12000, 138000, 1, true, now)
	insertOrderWithLedger(t, db, sellerB, pB, vB1, 250000, 20000, 230000, 1, true, now)

	// Scenario A: Seller A requests own product A -> HTTP 200, correct product ID and variants
	t.Run("Scenario A - Own product", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products/"+pA.String()+"?from="+fromStr+"&to="+toStr, nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, pA.String(), resp.ProductID)
		assert.Equal(t, "Product A", resp.Title)
		assert.Equal(t, int64(150000), resp.GrossSales.CurrentCents)
		assert.Equal(t, 1, resp.UnitsSold.Current)
		assert.Equal(t, 1, resp.Orders.Current)
		assert.Len(t, resp.Variants, 2)

		var skus []string
		for _, v := range resp.Variants {
			skus = append(skus, v.SKU)
		}
		assert.Contains(t, skus, "SKU-A1")
		assert.Contains(t, skus, "SKU-A2")
		assert.NotContains(t, skus, "SECRET-SKU-B1")
	})

	// Scenario B: Seller A requests product B (owned by Seller B) -> HTTP 404, no foreign metadata
	t.Run("Scenario B - Foreign product", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products/"+pB.String()+"?from="+fromStr+"&to="+toStr, nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		body := rec.Body.String()
		assert.NotContains(t, body, "Secret Product B")
		assert.NotContains(t, body, "SECRET-SKU-B1")
		assert.NotContains(t, body, vB1.String())
		assert.NotContains(t, body, "999") // foreign stock
	})

	// Scenario C: Nonexistent product ID -> HTTP 404, identical response to foreign product
	t.Run("Scenario C - Nonexistent product", func(t *testing.T) {
		nonexistentID := uuid.New()
		req := httptest.NewRequest("GET", "/products/"+nonexistentID.String()+"?from="+fromStr+"&to="+toStr, nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "Product not found")
	})

	// Scenario D: Malformed product ID -> HTTP 400
	t.Run("Scenario D - Invalid product ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products/not-a-valid-uuid?from="+fromStr+"&to="+toStr, nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Invalid product ID")
	})

	// Scenario E: Unauthenticated request -> HTTP 401
	t.Run("Scenario E - Unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products/"+pA.String()+"?from="+fromStr+"&to="+toStr, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Scenario F: Seller isolation with multi-seller orders
	t.Run("Scenario F - Multi-seller order isolation", func(t *testing.T) {
		// Create a shared customer order with 1 item from Seller A (Product A) and 1 item from Seller B (Product B)
		cID := insertCustomer(t, db)
		sharedOrderID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address, total_price_cents) VALUES ($1, $2, 'delivered', 'Customer Name', '+79990000000', 'cust@test.ru', 'Moscow', 400000)", sharedOrderID, cID)
		require.NoError(t, err)

		fA := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 150000, 800, 138000, $4)", fA, sharedOrderID, sellerA, now)
		require.NoError(t, err)

		oiA := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Product A', 'p-a', 150000, 1, 150000, $6)", oiA, sharedOrderID, pA, vA1, sellerA, fA)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 150000, $5)", uuid.New(), sellerA, sharedOrderID, oiA, now)
		require.NoError(t, err)

		fB := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 250000, 800, 230000, $4)", fB, sharedOrderID, sellerB, now)
		require.NoError(t, err)

		oiB := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Secret Product B', 'p-b', 250000, 1, 250000, $6)", oiB, sharedOrderID, pB, vB1, sellerB, fB)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 250000, $5)", uuid.New(), sellerB, sharedOrderID, oiB, now)
		require.NoError(t, err)

		// Request Product A detail
		req := httptest.NewRequest("GET", "/products/"+pA.String()+"?from="+fromStr+"&to="+toStr, nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		// Product A gross sales should NOT include Seller B's 250000
		assert.Equal(t, int64(300000), resp.GrossSales.CurrentCents) // 150000 from earlier + 150000 from shared order
		assert.Equal(t, 2, resp.UnitsSold.Current)
	})

	// Scenario G: Owned product with zero sales in period -> HTTP 200, ownership succeeds, catalog title populated
	t.Run("Scenario G - Owned product with zero sales", func(t *testing.T) {
		pZero := uuid.New()
		vZ := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Zero Sales Product', $4, 'published', 100000)", pZero, sellerA, catID, pZero.String())
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-ZERO-1', 'S', 'Blue', 100000, true)", vZ, pZero)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 25, 0)", uuid.New(), pZero, vZ, sellerA)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/products/"+pZero.String()+"?from="+fromStr+"&to="+toStr, nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "Must NOT return 404 for owned product with zero sales")
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, pZero.String(), resp.ProductID)
		assert.Equal(t, "Zero Sales Product", resp.Title)
		assert.Equal(t, int64(0), resp.GrossSales.CurrentCents)
		assert.Equal(t, 0, resp.UnitsSold.Current)
		assert.Equal(t, 25, resp.CurrentAvailableStock)
		assert.Len(t, resp.Variants, 1)
		assert.Equal(t, "SKU-ZERO-1", resp.Variants[0].SKU)
	})

	// Scenario H: Direct service boundary invocation -> returns ErrProductNotFound
	t.Run("Scenario H - Direct service boundary", func(t *testing.T) {
		_, err := svc.GetProductDetail(context.Background(), sellerA, pB, from, to, tz)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrProductNotFound), "Must return ErrProductNotFound when called directly on Service")
	})

	// Scenario I: Defense-in-depth at repository level
	t.Run("Scenario I - Repository GetVariantsPerformance scoped to seller", func(t *testing.T) {
		// Even if repository method is directly invoked with Seller A identity and Seller B's product ID
		variants, err := repo.GetVariantsPerformance(context.Background(), sellerA, pB, from, to)
		require.NoError(t, err)
		assert.Empty(t, variants, "Repository must return 0 variants for foreign product due to seller_id join")
	})
}
