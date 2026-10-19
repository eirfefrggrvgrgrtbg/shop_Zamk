package selleranalytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAnalytics_SortingAndPagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)
	handler := NewHandler(svc, repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sellerA := insertSeller(t, db)
	sellerB := insertSeller(t, db)

	var userA uuid.UUID
	err := db.QueryRow(context.Background(), "SELECT user_id FROM seller_users WHERE seller_id = $1", sellerA).Scan(&userA)
	require.NoError(t, err)

	now := time.Now().UTC()
	fromStr := now.Add(-24 * time.Hour).Format(time.RFC3339)
	toStr := now.Add(24 * time.Hour).Format(time.RFC3339)
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)
	tz := "Europe/Moscow"

	catID := uuid.New()
	db.Exec(context.Background(), "INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat Sorting', $2) ON CONFLICT DO NOTHING", catID, catID.String())

	// Create 3 Products for Seller A
	// P1: Gross 300,000 | Orders 2 | Units 2 | Stock 50 | Returns 0 (0%)
	p1 := uuid.New()
	v1 := uuid.New()
	db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Product Alpha', $4, 'published', 150000)", p1, sellerA, catID, p1.String())
	db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-P1', 'M', 'Black', 150000, true)", v1, p1)
	db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 50, 0)", uuid.New(), p1, v1, sellerA)
	insertOrderWithLedger(t, db, sellerA, p1, v1, 150000, 12000, 138000, 1, true, now)
	insertOrderWithLedger(t, db, sellerA, p1, v1, 150000, 12000, 138000, 1, true, now)

	// P2: Gross 100,000 | Orders 1 | Units 1 | Stock 10 | Returns 1 (100%)
	p2 := uuid.New()
	v2 := uuid.New()
	db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Product Beta', $4, 'published', 100000)", p2, sellerA, catID, p2.String())
	db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-P2', 'S', 'White', 100000, true)", v2, p2)
	db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 10, 0)", uuid.New(), p2, v2, sellerA)
	o2, oi2 := insertOrderWithLedger(t, db, sellerA, p2, v2, 100000, 8000, 92000, 1, true, now)
	var cID2 uuid.UUID
	db.QueryRow(context.Background(), "SELECT user_id FROM orders WHERE id = $1", o2).Scan(&cID2)
	insertReturn(t, db, o2, oi2, cID2, "completed", 1, 100000, now)

	// P3: Gross 100,000 | Orders 1 | Units 1 | Stock 80 | Returns 0 (0%) -> Same Gross/Orders/Units as P2, but different stock and return rate
	p3 := uuid.New()
	v3 := uuid.New()
	db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Product Gamma', $4, 'published', 100000)", p3, sellerA, catID, p3.String())
	db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-P3', 'L', 'Blue', 100000, true)", v3, p3)
	db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 80, 0)", uuid.New(), p3, v3, sellerA)
	insertOrderWithLedger(t, db, sellerA, p3, v3, 100000, 8000, 92000, 1, true, now)

	// Product 4 for Seller B -> must NEVER leak into Seller A's products list
	pB := uuid.New()
	vB := uuid.New()
	db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Seller B Secret', $4, 'published', 999000)", pB, sellerB, catID, pB.String())
	db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, 'SKU-B', 999000, true)", vB, pB)
	insertOrderWithLedger(t, db, sellerB, pB, vB, 999000, 50000, 949000, 1, true, now)

	// 1. Exact ordering by gross_sales desc
	t.Run("Sort by gross_sales desc", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=gross_sales&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductsResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Items, 3)
		assert.Equal(t, p1.String(), resp.Items[0].ProductID, "Alpha must be first with 300,000 cents")
		assert.Equal(t, int64(300000), resp.Items[0].GrossSalesCents)

		// Items 1 and 2 must have 100,000 cents and be stably ordered
		assert.Equal(t, int64(100000), resp.Items[1].GrossSalesCents)
		assert.Equal(t, int64(100000), resp.Items[2].GrossSalesCents)

		// Verify Seller B's product is NOT present (seller isolation)
		for _, item := range resp.Items {
			assert.NotEqual(t, pB.String(), item.ProductID)
		}
	})

	// 2. Exact ordering by stock desc
	t.Run("Sort by stock desc", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=stock&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductsResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Items, 3)
		assert.Equal(t, p3.String(), resp.Items[0].ProductID, "Gamma has stock 80")
		assert.Equal(t, 80, resp.Items[0].AvailableStock)
		assert.Equal(t, p1.String(), resp.Items[1].ProductID, "Alpha has stock 50")
		assert.Equal(t, 50, resp.Items[1].AvailableStock)
		assert.Equal(t, p2.String(), resp.Items[2].ProductID, "Beta has stock 10")
		assert.Equal(t, 10, resp.Items[2].AvailableStock)
	})

	// 3. Exact ordering by returns desc
	t.Run("Sort by returns desc", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=returns&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductsResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Items, 3)
		assert.Equal(t, p2.String(), resp.Items[0].ProductID, "Beta has 100% return rate")
		require.NotNil(t, resp.Items[0].ReturnRatePercent)
		assert.Equal(t, 100.0, *resp.Items[0].ReturnRatePercent)
	})

	// 4. Exact ordering by orders desc
	t.Run("Sort by orders desc", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=orders&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductsResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Items, 3)
		assert.Equal(t, p1.String(), resp.Items[0].ProductID, "Alpha has 2 orders")
		assert.Equal(t, 2, resp.Items[0].OrdersCount)
	})

	// 5. Exact ordering by units desc
	t.Run("Sort by units desc", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=units&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductsResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Items, 3)
		assert.Equal(t, p1.String(), resp.Items[0].ProductID, "Alpha has 2 units sold")
		assert.Equal(t, 2, resp.Items[0].UnitsSold)
	})

	// 6. Pagination stability and deterministic tie-breaking across pages
	t.Run("Pagination stability with limit=1 slices", func(t *testing.T) {
		// Slicing through all 3 items one by one
		var pageItems []string
		for offset := 0; offset < 3; offset++ {
			req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=gross_sales&order=desc&limit=1&offset="+string(rune('0'+offset)), nil)
			req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			var resp ProductsResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			require.Len(t, resp.Items, 1)
			assert.Equal(t, 3, resp.TotalCount)
			pageItems = append(pageItems, resp.Items[0].ProductID)
		}

		// Ensure all 3 distinct products were returned without duplicates
		assert.Len(t, pageItems, 3)
		assert.Equal(t, p1.String(), pageItems[0])
		assert.NotEqual(t, pageItems[1], pageItems[2])
		seen := make(map[string]bool)
		for _, id := range pageItems {
			assert.False(t, seen[id], "Pagination must not produce duplicates due to unstable ties")
			seen[id] = true
		}
	})

	// 7. CamelCase alias compatibility
	t.Run("CamelCase sort key aliases", func(t *testing.T) {
		// grossSalesCents -> gross_sales
		req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=grossSalesCents&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductsResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, p1.String(), resp.Items[0].ProductID)

		// availableStock -> stock
		req = httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=availableStock&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, p3.String(), resp.Items[0].ProductID)
	})

	// 8. Invalid sort key fallback to gross_sales
	t.Run("Invalid sort key fallback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products?from="+fromStr+"&to="+toStr+"&sort=non_existent_column&order=desc", nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductsResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, p1.String(), resp.Items[0].ProductID, "Invalid sort must safely fallback to gross_sales")
	})

	// 9. Zero-stock fallback regression verification
	t.Run("Zero stock regression: genuinely out-of-stock owned product displays 0 available units", func(t *testing.T) {
		pZeroStock := uuid.New()
		vZeroStock := uuid.New()
		db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Zero Stock Item', $4, 'published', 50000)", pZeroStock, sellerA, catID, pZeroStock.String())
		db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, 'SKU-ZERO-STOCK', 50000, true)", vZeroStock, pZeroStock)
		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 0, 0)", uuid.New(), pZeroStock, vZeroStock, sellerA)

		// Detail endpoint
		detail, err := svc.GetProductDetail(context.Background(), sellerA, pZeroStock, from, to, tz)
		require.NoError(t, err)
		assert.Equal(t, 0, detail.CurrentAvailableStock, "Genuinely zero stock product must have CurrentAvailableStock == 0")
		assert.Equal(t, 0, detail.UnitsSold.Current)
		assert.Equal(t, int64(0), detail.GrossSales.CurrentCents)
		assert.Len(t, detail.Variants, 1)
		assert.Equal(t, 0, detail.Variants[0].AvailableStock)
	})
}
