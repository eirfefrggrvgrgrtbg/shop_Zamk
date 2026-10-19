package selleranalytics

import (
	"context"
	"encoding/json"
	"errors"
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

func TestProductAnalytics_Timeseries(t *testing.T) {
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
	_, err = db.Exec(context.Background(), "INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat TS', $2) ON CONFLICT DO NOTHING", catID, catID.String())
	require.NoError(t, err)

	// Fixed test window in Europe/Moscow:
	// 2026-10-10 00:00:00 MSK (+03:00) to 2026-10-15 00:00:00 MSK (+03:00) (5 days: Oct 10, 11, 12, 13, 14)
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	fromMSK := time.Date(2026, 10, 10, 0, 0, 0, 0, loc)
	toMSK := time.Date(2026, 10, 15, 0, 0, 0, 0, loc)
	fromStr := url.QueryEscape(fromMSK.Format(time.RFC3339))
	toStr := url.QueryEscape(toMSK.Format(time.RFC3339))
	tz := "Europe/Moscow"

	// Helper to create product
	createProduct := func(sellerID uuid.UUID, title string, priceCents int64) (uuid.UUID, uuid.UUID) {
		pID := uuid.New()
		vID := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, $4, $5, 'published', $6)", pID, sellerID, catID, title, pID.String(), priceCents)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, $3, 'M', 'Black', $4, true)", vID, pID, "SKU-"+vID.String()[:8], priceCents)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 50, 0)", uuid.New(), pID, vID, sellerID)
		require.NoError(t, err)
		return pID, vID
	}

	// -------------------------------------------------------------
	// Scenario A: 1 delivered sale in period
	// -------------------------------------------------------------
	t.Run("Scenario A - 1 delivered sale in period", func(t *testing.T) {
		pA, vA := createProduct(sellerA, "Product Alpha", 150000)
		saleDate := time.Date(2026, 10, 11, 14, 0, 0, 0, loc) // Day 2
		insertOrderWithLedger(t, db, sellerA, pA, vA, 150000, 12000, 138000, 1, true, saleDate)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pA, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Timeseries, 5, "Must contain exactly 5 daily calendar buckets for 5-day period")
		assert.Equal(t, "2026-10-10", resp.Timeseries[0].Date)
		assert.Equal(t, int64(0), resp.Timeseries[0].GrossSalesCents)
		assert.Equal(t, 0, resp.Timeseries[0].UnitsSold)
		assert.Equal(t, 0, resp.Timeseries[0].OrdersCount)

		assert.Equal(t, "2026-10-11", resp.Timeseries[1].Date)
		assert.Equal(t, int64(150000), resp.Timeseries[1].GrossSalesCents)
		assert.Equal(t, 1, resp.Timeseries[1].UnitsSold)
		assert.Equal(t, 1, resp.Timeseries[1].OrdersCount)

		assert.Equal(t, "2026-10-12", resp.Timeseries[2].Date)
		assert.Equal(t, int64(0), resp.Timeseries[2].GrossSalesCents)
		assert.Equal(t, 0, resp.Timeseries[2].UnitsSold)
		assert.Equal(t, 0, resp.Timeseries[2].OrdersCount)
	})

	// -------------------------------------------------------------
	// Scenario B: Multiple deliveries across distinct calendar days
	// -------------------------------------------------------------
	t.Run("Scenario B - Multiple deliveries across distinct days", func(t *testing.T) {
		pB, vB := createProduct(sellerA, "Product Beta", 100000)
		day1 := time.Date(2026, 10, 10, 10, 0, 0, 0, loc)
		day3 := time.Date(2026, 10, 12, 16, 0, 0, 0, loc)
		insertOrderWithLedger(t, db, sellerA, pB, vB, 100000, 8000, 92000, 1, true, day1)
		insertOrderWithLedger(t, db, sellerA, pB, vB, 200000, 16000, 184000, 2, true, day3)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pB, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Timeseries, 5)
		assert.Equal(t, "2026-10-10", resp.Timeseries[0].Date)
		assert.Equal(t, int64(100000), resp.Timeseries[0].GrossSalesCents)
		assert.Equal(t, 1, resp.Timeseries[0].UnitsSold)
		assert.Equal(t, 1, resp.Timeseries[0].OrdersCount)

		assert.Equal(t, "2026-10-11", resp.Timeseries[1].Date)
		assert.Equal(t, int64(0), resp.Timeseries[1].GrossSalesCents)

		assert.Equal(t, "2026-10-12", resp.Timeseries[2].Date)
		assert.Equal(t, int64(200000), resp.Timeseries[2].GrossSalesCents)
		assert.Equal(t, 2, resp.Timeseries[2].UnitsSold)
		assert.Equal(t, 1, resp.Timeseries[2].OrdersCount)
	})

	// -------------------------------------------------------------
	// Scenario C: Multi-quantity order item (quantity = 3)
	// -------------------------------------------------------------
	t.Run("Scenario C - Multi-quantity order item", func(t *testing.T) {
		pC, vC := createProduct(sellerA, "Product Gamma", 100000)
		saleDate := time.Date(2026, 10, 13, 12, 0, 0, 0, loc)
		insertOrderWithLedger(t, db, sellerA, pC, vC, 300000, 24000, 276000, 3, true, saleDate)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pC, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Bucket 2026-10-13 is index 3
		assert.Equal(t, "2026-10-13", resp.Timeseries[3].Date)
		assert.Equal(t, int64(300000), resp.Timeseries[3].GrossSalesCents)
		assert.Equal(t, 3, resp.Timeseries[3].UnitsSold, "Units sold must be 3")
		assert.Equal(t, 1, resp.Timeseries[3].OrdersCount, "Orders count must be 1")
	})

	// -------------------------------------------------------------
	// Scenario D: Multi-variant order (variants v1 and v2 in same order)
	// -------------------------------------------------------------
	t.Run("Scenario D - Multi-variant order counted as single order", func(t *testing.T) {
		pD := uuid.New()
		vD1 := uuid.New()
		vD2 := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Product Delta', $4, 'published', 100000)", pD, sellerA, catID, pD.String())
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-D1', 'S', 'Red', 100000, true)", vD1, pD)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-D2', 'M', 'Red', 150000, true)", vD2, pD)
		require.NoError(t, err)

		saleDate := time.Date(2026, 10, 11, 15, 0, 0, 0, loc)
		oID := uuid.New()
		cID := insertCustomer(t, db)
		_, err = db.Exec(context.Background(), "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', 'D Customer', '123', 'd@d.c', 'addr')", oID, cID)
		require.NoError(t, err)

		fID := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 250000, 800, 230000, $4)", fID, oID, sellerA, saleDate)
		require.NoError(t, err)

		oi1 := uuid.New()
		oi2 := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Delta', 'slug', 100000, 1, 100000, $6)", oi1, oID, pD, vD1, sellerA, fID)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Delta', 'slug', 150000, 1, 150000, $6)", oi2, oID, pD, vD2, sellerA, fID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 100000, $5)", uuid.New(), sellerA, oID, oi1, saleDate)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 150000, $5)", uuid.New(), sellerA, oID, oi2, saleDate)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pD, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Bucket 2026-10-11
		b := resp.Timeseries[1]
		assert.Equal(t, "2026-10-11", b.Date)
		assert.Equal(t, int64(250000), b.GrossSalesCents)
		assert.Equal(t, 2, b.UnitsSold)
		assert.Equal(t, 1, b.OrdersCount, "Two variants in same order must count as 1 order, not 2")
	})

	// -------------------------------------------------------------
	// Scenario E: 2 distinct orders on same day
	// -------------------------------------------------------------
	t.Run("Scenario E - 2 distinct orders on same day", func(t *testing.T) {
		pE, vE := createProduct(sellerA, "Product Epsilon", 50000)
		saleDate := time.Date(2026, 10, 12, 10, 0, 0, 0, loc)
		insertOrderWithLedger(t, db, sellerA, pE, vE, 50000, 4000, 46000, 1, true, saleDate)
		insertOrderWithLedger(t, db, sellerA, pE, vE, 50000, 4000, 46000, 1, true, saleDate.Add(2*time.Hour))

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pE, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Bucket 2026-10-12 (index 2)
		b := resp.Timeseries[2]
		assert.Equal(t, "2026-10-12", b.Date)
		assert.Equal(t, int64(100000), b.GrossSalesCents)
		assert.Equal(t, 2, b.UnitsSold)
		assert.Equal(t, 2, b.OrdersCount, "Two distinct orders on same day must count as 2 orders")
	})

	// -------------------------------------------------------------
	// Scenario F: Paid but not delivered -> 0 sales in timeseries
	// -------------------------------------------------------------
	t.Run("Scenario F - Paid but not delivered excluded from delivered timeseries", func(t *testing.T) {
		pF, vF := createProduct(sellerA, "Product Zeta", 80000)
		saleDate := time.Date(2026, 10, 11, 10, 0, 0, 0, loc)
		// delivered = false -> no sale_gross ledger entry
		insertOrderWithLedger(t, db, sellerA, pF, vF, 80000, 6400, 73600, 1, false, saleDate)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pF, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		for _, b := range resp.Timeseries {
			assert.Equal(t, int64(0), b.GrossSalesCents)
			assert.Equal(t, 0, b.UnitsSold)
			assert.Equal(t, 0, b.OrdersCount)
		}
	})

	// -------------------------------------------------------------
	// Scenario G: Cancelled before delivery -> 0 sales in timeseries
	// -------------------------------------------------------------
	t.Run("Scenario G - Cancelled before delivery excluded from timeseries", func(t *testing.T) {
		pG, vG := createProduct(sellerA, "Product Eta", 90000)
		saleDate := time.Date(2026, 10, 11, 10, 0, 0, 0, loc)
		oID, _ := insertOrderWithLedger(t, db, sellerA, pG, vG, 90000, 7200, 82800, 1, false, saleDate)
		_, err := db.Exec(context.Background(), "UPDATE orders SET status = 'cancelled', cancelled_at = $1 WHERE id = $2", saleDate.Add(time.Hour), oID)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pG, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		for _, b := range resp.Timeseries {
			assert.Equal(t, int64(0), b.GrossSalesCents)
			assert.Equal(t, 0, b.UnitsSold)
			assert.Equal(t, 0, b.OrdersCount)
		}
	})

	// -------------------------------------------------------------
	// Scenario H: Foreign product -> HTTP 404, no timeseries data leaked
	// -------------------------------------------------------------
	t.Run("Scenario H - Foreign product returns 404 with no timeseries leak", func(t *testing.T) {
		pH, vH := createProduct(sellerB, "Seller B Secret", 999000)
		saleDate := time.Date(2026, 10, 11, 10, 0, 0, 0, loc)
		insertOrderWithLedger(t, db, sellerB, pH, vH, 999000, 50000, 949000, 1, true, saleDate)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pH, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA)) // Seller A requesting Seller B's product
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code, "Foreign product must return HTTP 404")

		// Also verify direct service boundary
		_, err := svc.GetProductDetail(context.Background(), sellerA, pH, fromMSK, toMSK, tz)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrProductNotFound))
	})

	// -------------------------------------------------------------
	// Scenario I: Owned product with no sales -> HTTP 200, all zero buckets
	// -------------------------------------------------------------
	t.Run("Scenario I - Owned product with zero sales returns 200 with zero buckets", func(t *testing.T) {
		pI, _ := createProduct(sellerA, "Product Iota Zero", 120000)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pI, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, pI.String(), resp.ProductID)
		assert.Equal(t, "Product Iota Zero", resp.Title)
		assert.Equal(t, int64(0), resp.GrossSales.CurrentCents)
		assert.Equal(t, 0, resp.UnitsSold.Current)
		require.Len(t, resp.Timeseries, 5)
		for _, b := range resp.Timeseries {
			assert.Equal(t, int64(0), b.GrossSalesCents)
			assert.Equal(t, 0, b.UnitsSold)
			assert.Equal(t, 0, b.OrdersCount)
		}
	})

	// -------------------------------------------------------------
	// Scenario J: Historical product price difference (ledger authoritative)
	// -------------------------------------------------------------
	t.Run("Scenario J - Historical product price difference ledger authoritative", func(t *testing.T) {
		pJ, vJ := createProduct(sellerA, "Product Kappa", 100000)
		saleDate := time.Date(2026, 10, 11, 10, 0, 0, 0, loc)
		// Delivered at historical price 100,000 cents
		insertOrderWithLedger(t, db, sellerA, pJ, vJ, 100000, 8000, 92000, 1, true, saleDate)

		// Later seller changes catalog price to 250,000 cents
		_, err := db.Exec(context.Background(), "UPDATE products SET price_cents = 250000 WHERE id = $1", pJ)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "UPDATE product_variants SET price_cents = 250000 WHERE id = $1", vJ)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pJ, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Timeseries must strictly reflect the historical ledger price 100,000, NOT 250,000
		b := resp.Timeseries[1]
		assert.Equal(t, int64(100000), b.GrossSalesCents)
		assert.Equal(t, int64(100000), resp.GrossSales.CurrentCents)
	})

	// -------------------------------------------------------------
	// Scenario K: Partial return after delivery
	// -------------------------------------------------------------
	t.Run("Scenario K - Partial return leaves delivered gross intact", func(t *testing.T) {
		pK, vK := createProduct(sellerA, "Product Lambda", 100000)
		saleDate := time.Date(2026, 10, 11, 10, 0, 0, 0, loc)
		retDate := time.Date(2026, 10, 13, 11, 0, 0, 0, loc)

		oID, oiID := insertOrderWithLedger(t, db, sellerA, pK, vK, 200000, 16000, 184000, 2, true, saleDate)
		var cID uuid.UUID
		db.QueryRow(context.Background(), "SELECT user_id FROM orders WHERE id = $1", oID).Scan(&cID)
		insertReturn(t, db, oID, oiID, cID, "completed", 1, 100000, retDate)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pK, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Sale bucket (Day 2: 2026-10-11)
		bSale := resp.Timeseries[1]
		assert.Equal(t, int64(200000), bSale.GrossSalesCents, "Gross sales remains delivered gross")
		assert.Equal(t, 2, bSale.UnitsSold)

		// Return bucket (Day 4: 2026-10-13)
		bRet := resp.Timeseries[3]
		assert.Equal(t, 1, bRet.ReturnedUnits, "Returned units tracked in return date bucket")
	})

	// -------------------------------------------------------------
	// Scenario L: Moscow timezone boundary
	// -------------------------------------------------------------
	t.Run("Scenario L - Moscow timezone boundary (21:30 UTC -> 00:30 MSK next day)", func(t *testing.T) {
		pL, vL := createProduct(sellerA, "Product Mu TZ", 120000)
		// Oct 10 21:30 UTC is Oct 11 00:30 MSK (UTC+3)
		utcTime := time.Date(2026, 10, 10, 21, 30, 0, 0, time.UTC)
		insertOrderWithLedger(t, db, sellerA, pL, vL, 120000, 9600, 110400, 1, true, utcTime)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pL, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Day 1: 2026-10-10 MSK -> 0 sales
		assert.Equal(t, "2026-10-10", resp.Timeseries[0].Date)
		assert.Equal(t, int64(0), resp.Timeseries[0].GrossSalesCents)

		// Day 2: 2026-10-11 MSK -> contains the sale!
		assert.Equal(t, "2026-10-11", resp.Timeseries[1].Date)
		assert.Equal(t, int64(120000), resp.Timeseries[1].GrossSalesCents)
		assert.Equal(t, 1, resp.Timeseries[1].UnitsSold)
		assert.Equal(t, 1, resp.Timeseries[1].OrdersCount)
	})

	// -------------------------------------------------------------
	// Scenario M: Mathematical reconciliation check
	// -------------------------------------------------------------
	t.Run("Scenario M - Mathematical reconciliation check", func(t *testing.T) {
		pM, vM := createProduct(sellerA, "Product Nu Reconcile", 110000)
		d1 := time.Date(2026, 10, 10, 10, 0, 0, 0, loc)
		d2 := time.Date(2026, 10, 12, 14, 0, 0, 0, loc)
		d3 := time.Date(2026, 10, 14, 18, 0, 0, 0, loc)

		insertOrderWithLedger(t, db, sellerA, pM, vM, 110000, 8800, 101200, 1, true, d1)
		insertOrderWithLedger(t, db, sellerA, pM, vM, 220000, 17600, 202400, 2, true, d2)
		insertOrderWithLedger(t, db, sellerA, pM, vM, 330000, 26400, 303600, 3, true, d3)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pM, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		var sumGross int64
		var sumUnits int
		var sumOrders int
		for _, b := range resp.Timeseries {
			sumGross += b.GrossSalesCents
			sumUnits += b.UnitsSold
			sumOrders += b.OrdersCount
		}

		assert.Equal(t, resp.GrossSales.CurrentCents, sumGross, "SUM(ts.grossSalesCents) must equal header GrossSales.CurrentCents")
		assert.Equal(t, resp.UnitsSold.Current, sumUnits, "SUM(ts.unitsSold) must equal header UnitsSold.Current")
		assert.Equal(t, resp.Orders.Current, sumOrders, "SUM(ts.ordersCount) must equal header Orders.Current")
		assert.Equal(t, int64(660000), sumGross)
		assert.Equal(t, 6, sumUnits)
		assert.Equal(t, 3, sumOrders)
	})

	// -------------------------------------------------------------
	// Scenario N: Multi-seller order isolation
	// -------------------------------------------------------------
	t.Run("Scenario N - Multi-seller order isolation", func(t *testing.T) {
		pN_A, vN_A := createProduct(sellerA, "Seller A Item", 100000)
		pN_B, vN_B := createProduct(sellerB, "Seller B Item", 300000)

		saleDate := time.Date(2026, 10, 11, 12, 0, 0, 0, loc)
		oID := uuid.New()
		cID := insertCustomer(t, db)
		_, err = db.Exec(context.Background(), "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', 'Multi Customer', '123', 'm@m.c', 'addr')", oID, cID)
		require.NoError(t, err)

		// Fulfillment for Seller A
		fA := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 100000, 800, 92000, $4)", fA, oID, sellerA, saleDate)
		require.NoError(t, err)
		oiA := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Seller A Item', 'slug', 100000, 1, 100000, $6)", oiA, oID, pN_A, vN_A, sellerA, fA)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 100000, $5)", uuid.New(), sellerA, oID, oiA, saleDate)
		require.NoError(t, err)

		// Fulfillment for Seller B
		fB := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 300000, 800, 276000, $4)", fB, oID, sellerB, saleDate)
		require.NoError(t, err)
		oiB := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Seller B Item', 'slug', 300000, 1, 300000, $6)", oiB, oID, pN_B, vN_B, sellerB, fB)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 300000, $5)", uuid.New(), sellerB, oID, oiB, saleDate)
		require.NoError(t, err)

		// Seller A queries Product A
		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pN_A, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Bucket 2026-10-11
		b := resp.Timeseries[1]
		assert.Equal(t, int64(100000), b.GrossSalesCents, "Product A timeseries must only include Seller A's 100,000 cents, NOT Seller B's 300,000")
		assert.Equal(t, 1, b.UnitsSold)
		assert.Equal(t, 1, b.OrdersCount)
	})

	// -------------------------------------------------------------
	// Scenario P: Delivered order reconciliation gate across midnight, multi-variant, and multi-order
	// -------------------------------------------------------------
	t.Run("Scenario P - Delivered order reconciliation gate across midnight and multi-variant", func(t *testing.T) {
		pP := uuid.New()
		vP1 := uuid.New()
		vP2 := uuid.New()
		_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Product Pi Gate', $4, 'published', 100000)", pP, sellerA, catID, pP.String())
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-P1', 'S', 'Black', 100000, true)", vP1, pP)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active) VALUES ($1, $2, 'SKU-P2', 'M', 'Black', 100000, true)", vP2, pP)
		require.NoError(t, err)

		// Order 1: Oct 11, 23:59:50 MSK (10 seconds before midnight) -> multi-variant (vP1 qty 1, vP2 qty 2)
		d1 := time.Date(2026, 10, 11, 23, 59, 50, 0, loc)
		oID1 := uuid.New()
		cID1 := insertCustomer(t, db)
		_, err = db.Exec(context.Background(), "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', 'Cust 1', '123', 'c1@c.c', 'addr')", oID1, cID1)
		require.NoError(t, err)
		f1 := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 300000, 800, 276000, $4)", f1, oID1, sellerA, d1)
		require.NoError(t, err)
		oi1_1 := uuid.New()
		oi1_2 := uuid.New()
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Pi', 'slug', 100000, 1, 100000, $6)", oi1_1, oID1, pP, vP1, sellerA, f1)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Pi', 'slug', 100000, 2, 200000, $6)", oi1_2, oID1, pP, vP2, sellerA, f1)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 100000, $5)", uuid.New(), sellerA, oID1, oi1_1, d1)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 200000, $5)", uuid.New(), sellerA, oID1, oi1_2, d1)
		require.NoError(t, err)

		// Order 2: Oct 12, 00:00:10 MSK (10 seconds after midnight) -> single variant (vP1 qty 1)
		d2 := time.Date(2026, 10, 12, 0, 0, 10, 0, loc)
		insertOrderWithLedger(t, db, sellerA, pP, vP1, 100000, 8000, 92000, 1, true, d2)

		// Order 3: Oct 13, 14:00:00 MSK -> single variant (vP2 qty 1)
		d3 := time.Date(2026, 10, 13, 14, 0, 0, 0, loc)
		insertOrderWithLedger(t, db, sellerA, pP, vP2, 100000, 8000, 92000, 1, true, d3)

		req := httptest.NewRequest("GET", fmt.Sprintf("/products/%s?from=%s&to=%s&tz=%s", pP, fromStr, toStr, tz), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userA))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProductDetailResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Len(t, resp.Timeseries, 5)

		// Day 1: 2026-10-10 -> 0
		assert.Equal(t, "2026-10-10", resp.Timeseries[0].Date)
		assert.Equal(t, int64(0), resp.Timeseries[0].GrossSalesCents)
		assert.Equal(t, 0, resp.Timeseries[0].UnitsSold)
		assert.Equal(t, 0, resp.Timeseries[0].OrdersCount)

		// Day 2: 2026-10-11 -> Order 1 (300,000 cents, 3 units, 1 distinct order)
		assert.Equal(t, "2026-10-11", resp.Timeseries[1].Date)
		assert.Equal(t, int64(300000), resp.Timeseries[1].GrossSalesCents)
		assert.Equal(t, 3, resp.Timeseries[1].UnitsSold)
		assert.Equal(t, 1, resp.Timeseries[1].OrdersCount, "Multi-variant order must count as 1 order on Oct 11")

		// Day 3: 2026-10-12 -> Order 2 (100,000 cents, 1 unit, 1 distinct order)
		assert.Equal(t, "2026-10-12", resp.Timeseries[2].Date)
		assert.Equal(t, int64(100000), resp.Timeseries[2].GrossSalesCents)
		assert.Equal(t, 1, resp.Timeseries[2].UnitsSold)
		assert.Equal(t, 1, resp.Timeseries[2].OrdersCount, "Across-midnight order must count as 1 order on Oct 12")

		// Day 4: 2026-10-13 -> Order 3 (100,000 cents, 1 unit, 1 distinct order)
		assert.Equal(t, "2026-10-13", resp.Timeseries[3].Date)
		assert.Equal(t, int64(100000), resp.Timeseries[3].GrossSalesCents)
		assert.Equal(t, 1, resp.Timeseries[3].UnitsSold)
		assert.Equal(t, 1, resp.Timeseries[3].OrdersCount)

		// Day 5: 2026-10-14 -> 0
		assert.Equal(t, "2026-10-14", resp.Timeseries[4].Date)
		assert.Equal(t, int64(0), resp.Timeseries[4].GrossSalesCents)

		// RECONCILIATION GATE: Sum of daily buckets must strictly equal header totals
		var sumDailyGross int64
		var sumDailyUnits int
		var sumDailyOrders int
		for _, b := range resp.Timeseries {
			sumDailyGross += b.GrossSalesCents
			sumDailyUnits += b.UnitsSold
			sumDailyOrders += b.OrdersCount
		}

		assert.Equal(t, int64(500000), sumDailyGross)
		assert.Equal(t, 5, sumDailyUnits)
		assert.Equal(t, 3, sumDailyOrders)

		assert.Equal(t, resp.GrossSales.CurrentCents, sumDailyGross, "SUM(daily.grossSalesCents) == GrossSales.CurrentCents")
		assert.Equal(t, resp.UnitsSold.Current, sumDailyUnits, "SUM(daily.unitsSold) == UnitsSold.Current")
		assert.Equal(t, resp.Orders.Current, sumDailyOrders, "SUM(daily.ordersCount) == Orders.Current")
	})
}
