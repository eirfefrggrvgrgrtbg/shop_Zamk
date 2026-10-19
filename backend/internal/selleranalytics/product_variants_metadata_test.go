package selleranalytics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAnalytics_VariantMetadata(t *testing.T) {
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

	catID := uuid.New()
	_, err = db.Exec(context.Background(), "INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat VM', $2) ON CONFLICT DO NOTHING", catID, catID.String())
	require.NoError(t, err)

	// Fetch canonical color and size from seed
	var colorBlackID uuid.UUID
	err = db.QueryRow(context.Background(), "SELECT id FROM colors WHERE code = 'BLACK' LIMIT 1").Scan(&colorBlackID)
	require.NoError(t, err)

	var sizeMID uuid.UUID
	err = db.QueryRow(context.Background(), "SELECT id FROM size_values WHERE value = 'M' LIMIT 1").Scan(&sizeMID)
	if err != nil {
		// Fallback to any active size value if 'M' not found
		err = db.QueryRow(context.Background(), "SELECT id FROM size_values LIMIT 1").Scan(&sizeMID)
		require.NoError(t, err)
	}

	now := time.Now().UTC()
	from := now.Add(-30 * 24 * time.Hour)
	to := now.Add(24 * time.Hour)
	fromStr := url.QueryEscape(from.Format(time.RFC3339))
	toStr := url.QueryEscape(to.Format(time.RFC3339))
	tz := "Europe/Moscow"

	// -------------------------------------------------------------
	// Scenario A: Modern variant with seller_sku, color_id, size_value_id,
	// NULL legacy fields and zero sales.
	// -------------------------------------------------------------
	t.Run("Scenario A - Modern variant with zero sales resolves readable size/color and seller_sku", func(t *testing.T) {
		pID := uuid.New()
		vID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Modern Hoodie', $4, 'published', 122200)", pID, sellerA, catID, pID.String())
		require.NoError(t, err)

		// Legacy size/color/sku are explicitly NULL; modern seller_sku, color_id, size_value_id populated
		_, err = db.Exec(context.Background(), `
			INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active)
			VALUES ($1, $2, 'SKU-4444-MODERN-A', $3, $4, 122200, true)
		`, vID, pID, colorBlackID, sizeMID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 10, 0)", uuid.New(), pID, vID, sellerA)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pID, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 1)
		v := resp.Variants[0]
		assert.Equal(t, "SKU-4444-MODERN-A", v.SKU, "Seller SKU must be resolved from pv.seller_sku")
		assert.Contains(t, v.DisplayName, "Черный", "Display name must resolve color from colors.name_ru")
		assert.Equal(t, int64(0), v.GrossSalesCents, "Gross sales must be 0 for unsold variant")
		assert.Equal(t, 0, v.UnitsSold, "Units sold must be 0")
		assert.Equal(t, 10, v.AvailableStock, "Available stock must be 10")
		assert.NotEqual(t, vID.String(), v.DisplayName, "Unsold modern variant must NOT fall back to raw UUID")
	})

	// -------------------------------------------------------------
	// Scenario B: Modern variant with delivered sales.
	// -------------------------------------------------------------
	t.Run("Scenario B - Modern variant with delivered sales", func(t *testing.T) {
		pID := uuid.New()
		vID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Delivered Modern Item', $4, 'published', 150000)", pID, sellerA, catID, pID.String())
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), `
			INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active)
			VALUES ($1, $2, 'SKU-4444-MODERN-B', $3, $4, 150000, true)
		`, vID, pID, colorBlackID, sizeMID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 25, 0)", uuid.New(), pID, vID, sellerA)
		require.NoError(t, err)

		// Delivered sale: qty = 2, gross = 300,000 cents
		insertOrderWithLedger(t, db, sellerA, pID, vID, 300000, 24000, 276000, 2, true, now.Add(-5*24*time.Hour))

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pID, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 1)
		v := resp.Variants[0]
		assert.Equal(t, "SKU-4444-MODERN-B", v.SKU)
		assert.Contains(t, v.DisplayName, "Черный")
		assert.Equal(t, int64(300000), v.GrossSalesCents)
		assert.Equal(t, 2, v.UnitsSold)
		assert.Equal(t, 25, v.AvailableStock)
	})

	// -------------------------------------------------------------
	// Scenario C: Legacy variant using pv.size, pv.color and pv.sku.
	// -------------------------------------------------------------
	t.Run("Scenario C - Legacy variant backward compatibility", func(t *testing.T) {
		pID := uuid.New()
		vID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Legacy Item', $4, 'published', 100000)", pID, sellerA, catID, pID.String())
		require.NoError(t, err)

		// Legacy textual columns populated, modern IDs are NULL
		_, err = db.Exec(context.Background(), `
			INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active)
			VALUES ($1, $2, 'SKU-LEGACY-C', 'L', 'Белый', 100000, true)
		`, vID, pID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 15, 0)", uuid.New(), pID, vID, sellerA)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pID, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 1)
		v := resp.Variants[0]
		assert.Equal(t, "SKU-LEGACY-C", v.SKU, "Legacy SKU must be preserved")
		assert.Equal(t, "L Белый", v.DisplayName, "Legacy size and color must be combined")
	})

	// -------------------------------------------------------------
	// Scenario D: Variant missing catalog metadata but with order_items snapshot.
	// -------------------------------------------------------------
	t.Run("Scenario D - Variant fallback to order_items snapshot", func(t *testing.T) {
		pID := uuid.New()
		vID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Snapshot Fallback Item', $4, 'published', 80000)", pID, sellerA, catID, pID.String())
		require.NoError(t, err)

		// Variant in catalog has NO size, color, or sku
		_, err = db.Exec(context.Background(), `
			INSERT INTO product_variants (id, product_id, price_cents, is_active)
			VALUES ($1, $2, 80000, true)
		`, vID, pID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 5, 0)", uuid.New(), pID, vID, sellerA)
		require.NoError(t, err)

		// Create order item with historical snapshots
		oID := uuid.New()
		cID := insertCustomer(t, db)
		_, err = db.Exec(context.Background(), "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', 'D Cust', '123', 'd@d.c', 'addr')", oID, cID)
		require.NoError(t, err)
		fID := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 80000, 800, 73600, $4)", fID, oID, sellerA, now)
		require.NoError(t, err)
		oiID := uuid.New()
		_, err = db.Exec(context.Background(), `
			INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, price_cents, quantity, subtotal_price_cents, order_fulfillment_id)
			VALUES ($1, $2, $3, $4, $5, 'Snap Item', 'slug', 'XL', 'Красный', 'SKU-SNAP-D', 80000, 1, 80000, $6)
		`, oiID, oID, pID, vID, sellerA, fID)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 80000, $5)", uuid.New(), sellerA, oID, oiID, now)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pID, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 1)
		v := resp.Variants[0]
		assert.Equal(t, "SKU-SNAP-D", v.SKU, "SKU must fall back to order item snapshot")
		assert.Equal(t, "XL Красный", v.DisplayName, "Display name must fall back to order item size and color snapshot")
	})

	// -------------------------------------------------------------
	// Scenario E: Variant genuinely missing all metadata.
	// -------------------------------------------------------------
	t.Run("Scenario E - Variant genuinely missing all metadata safely falls back to UUID", func(t *testing.T) {
		pID := uuid.New()
		vID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Bare Item', $4, 'published', 50000)", pID, sellerA, catID, pID.String())
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, price_cents, is_active) VALUES ($1, $2, 50000, true)", vID, pID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 2, 0)", uuid.New(), pID, vID, sellerA)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pID, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 1)
		v := resp.Variants[0]
		assert.Equal(t, "", v.SKU, "SKU must be empty when none exists")
		assert.Equal(t, vID.String(), v.DisplayName, "When completely devoid of attributes, safe fallback is variant UUID")
	})

	// -------------------------------------------------------------
	// Scenario F: Multi-variant product produces exactly one row per variant.
	// -------------------------------------------------------------
	t.Run("Scenario F - Multi-variant product has no duplicated rows from joins", func(t *testing.T) {
		pID := uuid.New()
		v1 := uuid.New()
		v2 := uuid.New()
		v3 := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Multi Item', $4, 'published', 100000)", pID, sellerA, catID, pID.String())
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active) VALUES ($1, $2, 'SKU-F1', $3, $4, 100000, true)", v1, pID, colorBlackID, sizeMID)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-F2', 'S', 'Синий', 100000, true)", v2, pID)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, seller_sku, price_cents, is_active) VALUES ($1, $2, 'SKU-F3', 100000, true)", v3, pID)
		require.NoError(t, err)

		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 5, 0)", uuid.New(), pID, v1, sellerA)
		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 7, 0)", uuid.New(), pID, v2, sellerA)
		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 9, 0)", uuid.New(), pID, v3, sellerA)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pID, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 3, "Must have exactly 3 variants without duplicate rows from joins")
	})

	// -------------------------------------------------------------
	// Scenario G: Archived/inactive variant is still included.
	// -------------------------------------------------------------
	t.Run("Scenario G - Archived variant with sales remains visible", func(t *testing.T) {
		pID := uuid.New()
		vID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Archived Variant Item', $4, 'published', 120000)", pID, sellerA, catID, pID.String())
		require.NoError(t, err)

		// is_active = false
		_, err = db.Exec(context.Background(), `
			INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active)
			VALUES ($1, $2, 'SKU-ARCHIVED', $3, $4, 120000, false)
		`, vID, pID, colorBlackID, sizeMID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 0, 0)", uuid.New(), pID, vID, sellerA)
		require.NoError(t, err)

		insertOrderWithLedger(t, db, sellerA, pID, vID, 120000, 9600, 110400, 1, true, now.Add(-2*24*time.Hour))

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pID, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 1, "Archived variant must not be filtered out")
		assert.Equal(t, "SKU-ARCHIVED", resp.Variants[0].SKU)
		assert.Equal(t, 1, resp.Variants[0].UnitsSold)
		assert.Equal(t, int64(120000), resp.Variants[0].GrossSalesCents)
	})

	// -------------------------------------------------------------
	// Scenario H: Product with zero sales and positive stock.
	// -------------------------------------------------------------
	t.Run("Scenario H - Product with zero sales and positive stock", func(t *testing.T) {
		pH := uuid.New()
		vH1 := uuid.New()
		vH2 := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Stock Only Item', $4, 'published', 200000)", pH, sellerA, catID, pH.String())
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active) VALUES ($1, $2, 'SKU-H1', $3, $4, 200000, true)", vH1, pH, colorBlackID, sizeMID)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-H2', 'XL', 'Серый', 200000, true)", vH2, pH)
		require.NoError(t, err)

		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 15, 0)", uuid.New(), pH, vH1, sellerA)
		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 25, 0)", uuid.New(), pH, vH2, sellerA)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pH, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, 40, resp.CurrentAvailableStock)
		assert.Equal(t, int64(0), resp.GrossSales.CurrentCents)
		require.Len(t, resp.Variants, 2)
		assert.Equal(t, 15, resp.Variants[0].AvailableStock)
		assert.Equal(t, 25, resp.Variants[1].AvailableStock)
	})

	// -------------------------------------------------------------
	// Scenario I: Product with genuine zero stock.
	// -------------------------------------------------------------
	t.Run("Scenario I - Genuine zero stock variant", func(t *testing.T) {
		pI := uuid.New()
		vI := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Zero Stock Item', $4, 'published', 100000)", pI, sellerA, catID, pI.String())
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active) VALUES ($1, $2, 'SKU-I', $3, $4, 100000, true)", vI, pI, colorBlackID, sizeMID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 0, 0)", uuid.New(), pI, vI, sellerA)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pI, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Variants, 1)
		assert.Equal(t, 0, resp.Variants[0].AvailableStock)
		assert.Equal(t, "out_of_stock", resp.Variants[0].StockCoverageState)
	})

	// -------------------------------------------------------------
	// Scenario J: Cross-seller access control.
	// -------------------------------------------------------------
	t.Run("Scenario J - Foreign product access is safely rejected", func(t *testing.T) {
		pB := uuid.New()
		vB := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Seller B Private', $4, 'published', 999000)", pB, sellerB, catID, pB.String())
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active) VALUES ($1, $2, 'SKU-SECRET-B', $3, $4, 999000, true)", vB, pB, colorBlackID, sizeMID)
		require.NoError(t, err)

		// Seller A tries to request Seller B's product
		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pB, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code, "Foreign product must return 404")

		// Direct repository invocation
		vars, err := repo.GetVariantsPerformance(context.Background(), sellerA, pB, from, to)
		require.NoError(t, err)
		assert.Empty(t, vars, "Direct repository query for foreign product must return empty slice")
	})

	// -------------------------------------------------------------
	// Scenario K: Full reconciliation of variant revenue, units and stock against product totals.
	// -------------------------------------------------------------
	t.Run("Scenario K - Reconciliation of variants against product totals", func(t *testing.T) {
		pK := uuid.New()
		vK1 := uuid.New()
		vK2 := uuid.New()
		vK3 := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Reconcile Item', $4, 'published', 100000)", pK, sellerA, catID, pK.String())
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, seller_sku, color_id, size_value_id, price_cents, is_active) VALUES ($1, $2, 'SKU-K1', $3, $4, 100000, true)", vK1, pK, colorBlackID, sizeMID)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-K2', 'S', 'Черный', 150000, true)", vK2, pK)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, seller_sku, price_cents, is_active) VALUES ($1, $2, 'SKU-K3', 200000, true)", vK3, pK)
		require.NoError(t, err)

		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 10, 0)", uuid.New(), pK, vK1, sellerA)
		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 20, 0)", uuid.New(), pK, vK2, sellerA)
		db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 30, 0)", uuid.New(), pK, vK3, sellerA)

		// Sales on vK1 (qty 1, 100,000) and vK2 (qty 2, 300,000). vK3 has 0 sales.
		insertOrderWithLedger(t, db, sellerA, pK, vK1, 100000, 8000, 92000, 1, true, now.Add(-3*24*time.Hour))
		insertOrderWithLedger(t, db, sellerA, pK, vK2, 300000, 24000, 276000, 2, true, now.Add(-2*24*time.Hour))

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pK, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		var sumVariantGross int64
		var sumVariantUnits int
		var sumVariantStock int
		for _, v := range resp.Variants {
			sumVariantGross += v.GrossSalesCents
			sumVariantUnits += v.UnitsSold
			sumVariantStock += v.AvailableStock
		}

		assert.Equal(t, resp.GrossSales.CurrentCents, sumVariantGross, "SUM(variant.grossSalesCents) must equal Product.GrossSales.CurrentCents")
		assert.Equal(t, resp.UnitsSold.Current, sumVariantUnits, "SUM(variant.unitsSold) must equal Product.UnitsSold.Current")
		assert.Equal(t, resp.CurrentAvailableStock, sumVariantStock, "SUM(variant.availableStock) must equal Product.CurrentAvailableStock")

		assert.Equal(t, int64(400000), sumVariantGross)
		assert.Equal(t, 3, sumVariantUnits)
		assert.Equal(t, 60, sumVariantStock)
	})
}
