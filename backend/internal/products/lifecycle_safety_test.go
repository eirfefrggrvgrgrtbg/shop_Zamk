package products_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type lifecycleEnv struct {
	svc          *products.Service
	handler      *products.Handler
	router       chi.Router
	sellerUserID uuid.UUID
	sellerID     uuid.UUID
	brandID      uuid.UUID
	catID        uuid.UUID
	pool         *pgxpool.Pool
	dbClient     *postgres.Client
}

func setupLifecycleEnv(t *testing.T) *lifecycleEnv {
	t.Helper()
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Canonical DB Safety Guard: Must be zamk_test
	testutil.AssertTestDatabase(t, pool)
	var dbName string
	err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "integration tests MUST run against zamk_test")

	var sellerID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	var brandID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT brand_id FROM seller_brands WHERE seller_id = $1 AND is_primary = true", sellerID).Scan(&brandID)
	require.NoError(t, err)

	var catID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM categories LIMIT 1").Scan(&catID)
	require.NoError(t, err)

	handler := products.NewHandler(svc, nil)

	r := chi.NewRouter()
	r.Route("/api/seller/products", func(r chi.Router) {
		r.Delete("/{id}", handler.DeleteDraftProduct)
		r.Post("/{id}/archive", handler.ArchiveSellerProduct)
	})

	return &lifecycleEnv{
		svc:          svc,
		handler:      handler,
		router:       r,
		sellerUserID: sellerUserID,
		sellerID:     sellerID,
		brandID:      brandID,
		catID:        catID,
		pool:         pool,
		dbClient:     dbClient,
	}
}

func (env *lifecycleEnv) createProduct(t *testing.T, status string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	prodID := uuid.New()
	varID := uuid.New()
	slug := fmt.Sprintf("prod-%s", prodID)

	_, err := env.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency)
		VALUES ($1, $2, $3, $4, 'Lifecycle Test Product', $5, $6, 1500, 'RUB')
	`, prodID, env.sellerID, env.brandID, env.catID, slug, status)
	require.NoError(t, err)

	_, err = env.pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, is_active)
		VALUES ($1, $2, $3, true)
	`, varID, prodID, fmt.Sprintf("SKU-%s", varID.String()[:8]))
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = env.pool.Exec(cleanupCtx, "DELETE FROM product_variants WHERE product_id = $1", prodID)
		_, _ = env.pool.Exec(cleanupCtx, "DELETE FROM products WHERE id = $1", prodID)
	})

	return prodID, varID
}

func (env *lifecycleEnv) createForeignSellerUser(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	foreignUserID := uuid.New()
	foreignSellerID := uuid.New()

	_, err := env.pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Foreign User')", foreignUserID, fmt.Sprintf("for-%s@test.com", foreignUserID))
	require.NoError(t, err)

	_, err = env.pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Foreign Brand', $2, 'for@test.com', 'active')", foreignSellerID, fmt.Sprintf("for-%s", foreignUserID))
	require.NoError(t, err)

	_, err = env.pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), foreignSellerID, foreignUserID)
	require.NoError(t, err)

	t.Cleanup(func() {
		cCtx := context.Background()
		_, _ = env.pool.Exec(cCtx, "DELETE FROM seller_users WHERE user_id = $1", foreignUserID)
		_, _ = env.pool.Exec(cCtx, "DELETE FROM sellers WHERE id = $1", foreignSellerID)
		_, _ = env.pool.Exec(cCtx, "DELETE FROM users WHERE id = $1", foreignUserID)
	})

	return foreignUserID
}

func TestLifecycleSafety_SafeDeleteMatrix(t *testing.T) {
	env := setupLifecycleEnv(t)
	ctx := context.Background()

	// A. fresh disposable draft => delete succeeds => 204 contract
	t.Run("A. fresh disposable draft -> 204", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusDraft)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)

		// Verify product is deleted
		var count int
		err := env.pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", prodID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	// B. draft with inventory_items => 409 product_not_disposable => preserved
	t.Run("B. draft with inventory_items -> 409 product_not_disposable", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusDraft)

		invID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
			VALUES ($1, $2, $3, $4, 10, 0)
		`, invID, prodID, varID, env.sellerID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = env.pool.Exec(context.Background(), "DELETE FROM inventory_items WHERE id = $1", invID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		errObj := body["error"].(map[string]any)
		require.Equal(t, "product_not_disposable", errObj["code"])

		// Product preserved
		var count int
		err = env.pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", prodID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	// C. draft with inventory_units/ZMU => blocked => units preserved
	t.Run("C. draft with inventory_units/ZMU -> 409 blocked, units preserved", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusDraft)

		supID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at)
			VALUES ($1, $2, $3, 'draft', 'delivery', now(), now())
		`, supID, fmt.Sprintf("SUP-%s", supID.String()[:8]), env.sellerID)
		require.NoError(t, err)

		supItemID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 5, 0, 0, 0, 0, now(), now())
		`, supItemID, supID, varID)
		require.NoError(t, err)

		unitID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 1, 'warehouse', now(), now())
		`, unitID, fmt.Sprintf("ZMU-%s", unitID.String()[:8]), varID, supID, supItemID)
		require.NoError(t, err)

		t.Cleanup(func() {
			cCtx := context.Background()
			_, _ = env.pool.Exec(cCtx, "DELETE FROM inventory_units WHERE id = $1", unitID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM seller_supply_items WHERE id = $1", supItemID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM seller_supplies WHERE id = $1", supID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)

		// Units preserved
		var count int
		err = env.pool.QueryRow(ctx, "SELECT count(*) FROM inventory_units WHERE id = $1", unitID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	// D. draft with seller_supply history => blocked
	t.Run("D. draft with seller_supply history -> blocked", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusDraft)

		supID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at)
			VALUES ($1, $2, $3, 'draft', 'delivery', now(), now())
		`, supID, fmt.Sprintf("SUP-%s", supID.String()[:8]), env.sellerID)
		require.NoError(t, err)

		supItemID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 5, 0, 0, 0, 0, now(), now())
		`, supItemID, supID, varID)
		require.NoError(t, err)

		t.Cleanup(func() {
			cCtx := context.Background()
			_, _ = env.pool.Exec(cCtx, "DELETE FROM seller_supply_items WHERE id = $1", supItemID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM seller_supplies WHERE id = $1", supID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		errObj := body["error"].(map[string]any)
		require.Equal(t, "product_not_disposable", errObj["code"])
	})

	// E. draft with order item => blocked
	t.Run("E. draft with order item -> blocked", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusDraft)

		orderID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
			VALUES ($1, $2, 'paid', 1500, 'RUB', 'Test Buyer', '+79991234567', 'buyer@test.com', 'Moscow', now(), now())
		`, orderID, env.sellerUserID)
		require.NoError(t, err)

		fulfillmentID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'paid', 1500, 1000, 1350, now(), now())
		`, fulfillmentID, orderID, env.sellerID)
		require.NoError(t, err)

		orderItemID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at)
			VALUES ($1, $2, $3, $4, $5, 'Order Prod', 'slug', 1500, 1, 1500, $6, now())
		`, orderItemID, orderID, prodID, varID, env.sellerID, fulfillmentID)
		require.NoError(t, err)

		t.Cleanup(func() {
			cCtx := context.Background()
			_, _ = env.pool.Exec(cCtx, "DELETE FROM order_items WHERE id = $1", orderItemID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM orders WHERE id = $1", orderID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	// F. draft with review => blocked
	t.Run("F. draft with review -> blocked", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusDraft)

		orderID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
			VALUES ($1, $2, 'paid', 1500, 'RUB', 'Buyer', '+79990000000', 'buyer@test.com', 'Moscow', now(), now())
		`, orderID, env.sellerUserID)
		require.NoError(t, err)

		fulfillmentID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'paid', 1500, 1000, 1350, now(), now())
		`, fulfillmentID, orderID, env.sellerID)
		require.NoError(t, err)

		orderItemID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at)
			VALUES ($1, $2, $3, $4, $5, 'Prod', 'slug', 1500, 1, 1500, $6, now())
		`, orderItemID, orderID, prodID, varID, env.sellerID, fulfillmentID)
		require.NoError(t, err)

		reviewID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO product_reviews (id, product_id, product_variant_id, order_id, order_item_id, user_id, seller_id, rating, comment, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 5, 'Great product', 'published', now(), now())
		`, reviewID, prodID, varID, orderID, orderItemID, env.sellerUserID, env.sellerID)
		require.NoError(t, err)

		t.Cleanup(func() {
			cCtx := context.Background()
			_, _ = env.pool.Exec(cCtx, "DELETE FROM product_reviews WHERE id = $1", reviewID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM order_items WHERE id = $1", orderItemID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM orders WHERE id = $1", orderID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	// G. draft with moderation/revision history => blocked
	t.Run("G. draft with moderation/revision history -> blocked", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusDraft)

		modLogID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO product_moderation_logs (id, product_id, to_status, comment, created_at)
			VALUES ($1, $2, 'pending_moderation', 'Submitted for review', now())
		`, modLogID, prodID)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _ = env.pool.Exec(context.Background(), "DELETE FROM product_moderation_logs WHERE id = $1", modLogID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	// H. draft with inventory reconciliation / stock history => blocked
	t.Run("H. draft with inventory reconciliation session -> blocked", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusDraft)

		recID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO inventory_reconciliation_sessions (id, product_variant_id, status, started_by, started_at)
			VALUES ($1, $2, 'in_progress', $3, now())
		`, recID, varID, env.sellerUserID)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _ = env.pool.Exec(context.Background(), "DELETE FROM inventory_reconciliation_sessions WHERE id = $1", recID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	// I. customer relations (favorites, views, cart) => blocked
	t.Run("I. customer relations -> blocked", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusDraft)

		favID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO customer_favorites (id, user_id, product_id, created_at)
			VALUES ($1, $2, $3, now())
		`, favID, env.sellerUserID, prodID)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _ = env.pool.Exec(context.Background(), "DELETE FROM customer_favorites WHERE id = $1", favID)
		})

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	// J. rejected => hard delete invalid source status (409 invalid_status)
	t.Run("J. rejected -> 409 invalid_status", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusRejected)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		errObj := body["error"].(map[string]any)
		require.Equal(t, "invalid_status", errObj["code"])
	})

	// K. published => hard delete invalid source status (409 invalid_status)
	t.Run("K. published -> 409 invalid_status", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusPublished)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		errObj := body["error"].(map[string]any)
		require.Equal(t, "invalid_status", errObj["code"])
	})

	// L. foreign seller => 404 not found
	t.Run("L. foreign seller -> 404 not_found", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusDraft)
		foreignUserID := env.createForeignSellerUser(t)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/seller/products/%s", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", foreignUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	// M. guard query failure => DELETE FAILS CLOSED => product preserved
	t.Run("M. guard failure -> fails closed, product preserved", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusDraft)

		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		err := env.svc.DeleteSellerDraftProduct(cancelledCtx, env.sellerUserID, prodID)
		require.Error(t, err)

		// Verify product row is preserved
		var count int
		err = env.pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", prodID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "product MUST be preserved when delete guard encounters error")
	})
}

func TestLifecycleSafety_ArchiveMatrix(t *testing.T) {
	env := setupLifecycleEnv(t)
	ctx := context.Background()

	testArchive := func(t *testing.T, initialStatus string, expectedCode int, expectedErrCode string) uuid.UUID {
		t.Helper()
		prodID, _ := env.createProduct(t, initialStatus)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/archive", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, expectedCode, rec.Code)
		if expectedErrCode != "" {
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			errObj := body["error"].(map[string]any)
			require.Equal(t, expectedErrCode, errObj["code"])
		} else {
			var currentStatus string
			err := env.pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", prodID).Scan(&currentStatus)
			require.NoError(t, err)
			require.Equal(t, products.StatusArchived, currentStatus)
		}
		return prodID
	}

	// Allowed Transitions
	t.Run("N. draft -> archived", func(t *testing.T) {
		testArchive(t, products.StatusDraft, http.StatusOK, "")
	})

	t.Run("O. rejected -> archived", func(t *testing.T) {
		testArchive(t, products.StatusRejected, http.StatusOK, "")
	})

	t.Run("P. approved -> archived", func(t *testing.T) {
		testArchive(t, products.StatusApproved, http.StatusOK, "")
	})

	t.Run("Q. published -> archived", func(t *testing.T) {
		testArchive(t, products.StatusPublished, http.StatusOK, "")
	})

	t.Run("R. out_of_stock -> archived", func(t *testing.T) {
		testArchive(t, products.StatusOutOfStock, http.StatusOK, "")
	})

	// S. archived -> idempotent success
	t.Run("S. archived -> idempotent success", func(t *testing.T) {
		prodID := testArchive(t, products.StatusDraft, http.StatusOK, "")

		// Second archive call
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/archive", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	// Forbidden Transitions
	t.Run("T. pending_moderation -> 409 invalid_transition", func(t *testing.T) {
		testArchive(t, products.StatusPendingModeration, http.StatusConflict, "invalid_transition")
	})

	t.Run("U. in_review -> 409 invalid_transition", func(t *testing.T) {
		testArchive(t, products.StatusInReview, http.StatusConflict, "invalid_transition")
	})

	t.Run("V. hidden -> 409 invalid_transition", func(t *testing.T) {
		testArchive(t, products.StatusHidden, http.StatusConflict, "invalid_transition")
	})

	t.Run("W. blocked -> 409 invalid_transition", func(t *testing.T) {
		testArchive(t, products.StatusBlocked, http.StatusConflict, "invalid_transition")
	})

	// X. archive published product with ZMU => stock unchanged
	t.Run("X. archive published with ZMU -> stock unchanged", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusPublished)

		supID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at)
			VALUES ($1, $2, $3, 'draft', 'delivery', now(), now())
		`, supID, fmt.Sprintf("SUP-%s", supID.String()[:8]), env.sellerID)
		require.NoError(t, err)

		supItemID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 1, 1, 0, 0, 0, now(), now())
		`, supItemID, supID, varID)
		require.NoError(t, err)

		unitID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 1, 'warehouse', now(), now())
		`, unitID, fmt.Sprintf("ZMU-%s", unitID.String()[:8]), varID, supID, supItemID)
		require.NoError(t, err)

		t.Cleanup(func() {
			cCtx := context.Background()
			_, _ = env.pool.Exec(cCtx, "DELETE FROM inventory_units WHERE id = $1", unitID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM seller_supply_items WHERE id = $1", supItemID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM seller_supplies WHERE id = $1", supID)
		})

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/archive", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		// Verify ZMU status and count unchanged
		var unitStatus string
		err = env.pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unitID).Scan(&unitStatus)
		require.NoError(t, err)
		require.Equal(t, "warehouse", unitStatus)
	})

	// Y. archive product with orders/history => history unchanged
	t.Run("Y. archive product with orders/history -> history unchanged", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusPublished)

		orderID := uuid.New()
		_, err := env.pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
			VALUES ($1, $2, 'paid', 1500, 'RUB', 'Buyer', '+79990000000', 'buyer@test.com', 'Moscow', now(), now())
		`, orderID, env.sellerUserID)
		require.NoError(t, err)

		fulfillmentID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'paid', 1500, 1000, 1350, now(), now())
		`, fulfillmentID, orderID, env.sellerID)
		require.NoError(t, err)

		orderItemID := uuid.New()
		_, err = env.pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at)
			VALUES ($1, $2, $3, $4, $5, 'Prod', 'slug', 1500, 1, 1500, $6, now())
		`, orderItemID, orderID, prodID, varID, env.sellerID, fulfillmentID)
		require.NoError(t, err)

		t.Cleanup(func() {
			cCtx := context.Background()
			_, _ = env.pool.Exec(cCtx, "DELETE FROM order_items WHERE id = $1", orderItemID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
			_, _ = env.pool.Exec(cCtx, "DELETE FROM orders WHERE id = $1", orderID)
		})

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/archive", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		// Order item still exists
		var oCount int
		err = env.pool.QueryRow(ctx, "SELECT count(*) FROM order_items WHERE id = $1", orderItemID).Scan(&oCount)
		require.NoError(t, err)
		require.Equal(t, 1, oCount)
	})

	// Z. archive preserves variant IDs
	t.Run("Z. archive preserves variant IDs", func(t *testing.T) {
		prodID, varID := env.createProduct(t, products.StatusPublished)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/archive", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", env.sellerUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var vCount int
		err := env.pool.QueryRow(ctx, "SELECT count(*) FROM product_variants WHERE id = $1 AND product_id = $2", varID, prodID).Scan(&vCount)
		require.NoError(t, err)
		require.Equal(t, 1, vCount)
	})

	// AA. archived product: actualVisibility=false, storefrontUrl=nil
	t.Run("AA. visibility engine: actualVisibility=false, storefrontUrl=nil", func(t *testing.T) {
		activeSeller := "active"
		prod := &products.Product{
			ID:                  uuid.New(),
			Status:              products.StatusArchived,
			SellerStatus:        &activeSeller,
			ActiveVariantsCount: 1,
			PriceCents:          1000,
			AvailableStock:      10,
		}

		vis := products.CalculateActualVisibility(prod)
		require.False(t, vis.ActualVisibility)
		require.Contains(t, vis.VisibilityReasons, "product_archived")
	})

	// AB. foreign seller cannot archive
	t.Run("AB. foreign seller cannot archive -> 404 not_found", func(t *testing.T) {
		prodID, _ := env.createProduct(t, products.StatusDraft)
		foreignUserID := env.createForeignSellerUser(t)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/archive", prodID), nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", foreignUserID))
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}
