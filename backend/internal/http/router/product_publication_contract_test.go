package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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

func setupProductPublicationTestEnv(t *testing.T) (context.Context, http.Handler, func(), *postgres.Client, *auth.TokenService) {
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

func TestProductPublicationContract(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupProductPublicationTestEnv(t)
	defer cleanup()

	// 1. Create Admin user with moderation and receiving permissions
	adminID := uuid.New()
	adminEmail := "admin-pub-" + adminID.String()[:8] + "@test.com"
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Admin User', 'hash', 'admin', 'active', now(), now())
	`, adminID, adminEmail, "+7999"+adminID.String()[:7])
	require.NoError(t, err)

	roleID := uuid.New()
	code := roleID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'PubRole')`, roleID, code)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_role_permissions (role_id, permission)
		VALUES ($1, 'products.approve'), ($1, 'products.reject'), ($1, 'inventory.receipt')
	`, roleID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, adminID, roleID)
	require.NoError(t, err)

	adminToken, err := tokenService.GenerateAccessToken(adminID, adminEmail, "admin")
	require.NoError(t, err)

	// 2. Create Customer user
	customerID := uuid.New()
	customerEmail := "customer-pub-" + customerID.String()[:8] + "@test.com"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Customer User', 'hash', 'customer', 'active', now(), now())
	`, customerID, customerEmail, "+7998"+customerID.String()[:7])
	require.NoError(t, err)

	customerToken, err := tokenService.GenerateAccessToken(customerID, customerEmail, "customer")
	require.NoError(t, err)

	// 3. Create Active Seller
	sellerID := uuid.New()
	sellerSlug := "pub-seller-" + sellerID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Publication Test Brand', $2, $3, 'active', now(), now())
	`, sellerID, sellerSlug, sellerSlug+"@test.com")
	require.NoError(t, err)

	// 4. Create Category
	catID := uuid.New()
	catSlug := "pub-cat-" + catID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Pub Category', $2, true, now(), now())
	`, catID, catSlug)
	require.NoError(t, err)

	// 5. Ensure Delivery Method exists for order creation
	var deliveryMethodID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM delivery_methods WHERE is_active = true LIMIT 1").Scan(&deliveryMethodID)
	if err != nil {
		deliveryMethodID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
			VALUES ($1, 'pub-standard-delivery', 'Standard Delivery', 50000, true, now(), now())
		`, deliveryMethodID)
		require.NoError(t, err)
	}

	// Helper to insert a valid product in pending_moderation
	createPendingProduct := func(title, slug string, sID uuid.UUID) (uuid.UUID, uuid.UUID) {
		prodID := uuid.New()
		variantID := uuid.New()
		now := time.Now()

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				submitted_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, 500000, 'RUB', 'pending_moderation', $6, $6, $6)
		`, prodID, sID, catID, title, slug, now)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (
				id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at
			) VALUES ($1, $2, $3, $3, $4, 500000, true, $5, $5)
		`, variantID, prodID, "SKU-"+variantID.String()[:8], "BC-"+variantID.String()[:8], now)
		require.NoError(t, err)

		return prodID, variantID
	}

	t.Run("A. approved/published + free=0 -> hidden, PDP unavailable", func(t *testing.T) {
		prodID, _ := createPendingProduct("Zero Stock Product", "zero-stock-"+uuid.New().String()[:8], sellerID)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		catReq := httptest.NewRequest(http.MethodGet, "/api/public/products", nil)
		catRec := httptest.NewRecorder()
		r.ServeHTTP(catRec, catReq)
		var catResp struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		}
		err := json.NewDecoder(catRec.Body).Decode(&catResp)
		require.NoError(t, err)
		for _, item := range catResp.Items {
			assert.NotEqual(t, prodID, item.ID, "0-stock product must be hidden from catalog")
		}

		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRec.Code, "0-stock product PDP must be 404")
	})

	t.Run("B. free=1 -> hidden", func(t *testing.T) {
		prodID, variantID := createPendingProduct("1 Stock Product", "one-stock-"+uuid.New().String()[:8], sellerID)
		_, err := pgClient.Pool.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 1, 0, now(), now())", uuid.New(), prodID, variantID, sellerID)
		require.NoError(t, err)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRec.Code, "1-stock product PDP must be 404")
	})

	t.Run("C. free=2 -> visible", func(t *testing.T) {
		prodID, variantID := createPendingProduct("2 Stock Product", "two-stock-"+uuid.New().String()[:8], sellerID)
		_, err := pgClient.Pool.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 2, 0, now(), now())", uuid.New(), prodID, variantID, sellerID)
		require.NoError(t, err)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusOK, pdpRec.Code, "2-stock product PDP must be 200")
	})

	t.Run("D. free=3 -> visible", func(t *testing.T) {
		prodID, variantID := createPendingProduct("3 Stock Product", "three-stock-"+uuid.New().String()[:8], sellerID)
		_, err := pgClient.Pool.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 3, 0, now(), now())", uuid.New(), prodID, variantID, sellerID)
		require.NoError(t, err)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusOK, pdpRec.Code, "3-stock product PDP must be 200")
	})

	t.Run("E. real reservation 2 -> 1 -> hidden without changing moderation approval", func(t *testing.T) {
		prodID, variantID := createPendingProduct("Reservation Test Product", "res-stock-"+uuid.New().String()[:8], sellerID)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 2, 0, now(), now())
		`, uuid.New(), prodID, variantID, sellerID)
		require.NoError(t, err)

		// Admin approves product
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// 1. Initial state: free=2, PDP returns 200
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusOK, pdpRec.Code, "Product with 2 free units must be visible")

		// Count moderation logs before reservation
		var modLogCountBefore int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_moderation_logs WHERE product_id = $1", prodID).Scan(&modLogCountBefore)
		require.NoError(t, err)

		// 2. Customer adds 1 unit to cart via real endpoint
		cartBody, _ := json.Marshal(map[string]any{
			"productId":        prodID.String(),
			"productVariantId": variantID.String(),
			"quantity":         1,
		})
		cartReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReq.Header.Set("Authorization", "Bearer "+customerToken)
		cartReq.Header.Set("Content-Type", "application/json")
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		assert.Equal(t, http.StatusCreated, cartRec.Code)

		// 3. Customer places order via real endpoint (creates real reservation in inventory)
		orderBody, _ := json.Marshal(map[string]any{
			"customerName":     "Test Customer",
			"customerPhone":    "+79981234567",
			"customerEmail":    customerEmail,
			"deliveryAddress":  "Test Street 1",
			"deliveryMethodId": deliveryMethodID,
		})
		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(orderBody))
		orderReq.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		assert.Equal(t, http.StatusCreated, orderRec.Code)

		// Verify DB reserved_stock is now 1 (canonical free = 2 - 1 = 1)
		var totalStock, reservedStock int
		err = pgClient.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalStock, &reservedStock)
		require.NoError(t, err)
		assert.Equal(t, 2, totalStock)
		assert.Equal(t, 1, reservedStock, "real reservation must increment reserved_stock to 1")

		// 4. Product must be hidden from catalog and PDP
		pdpReq2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec2 := httptest.NewRecorder()
		r.ServeHTTP(pdpRec2, pdpReq2)
		assert.Equal(t, http.StatusNotFound, pdpRec2.Code, "Product with free=1 must be hidden from PDP")

		catReq := httptest.NewRequest(http.MethodGet, "/api/public/products", nil)
		catRec := httptest.NewRecorder()
		r.ServeHTTP(catRec, catReq)
		var catResp struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		}
		json.NewDecoder(catRec.Body).Decode(&catResp)
		for _, item := range catResp.Items {
			assert.NotEqual(t, prodID, item.ID, "Product with free=1 must be hidden from catalog")
		}

		// 5. Verify product status in DB remains strictly 'published'
		var status string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", prodID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "published", status, "Product status must not mutate during stock shortage")

		// 6. Verify no new moderation logs were created
		var modLogCountAfter int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_moderation_logs WHERE product_id = $1", prodID).Scan(&modLogCountAfter)
		require.NoError(t, err)
		assert.Equal(t, modLogCountBefore, modLogCountAfter, "Stock change must not create moderation history")
	})

	t.Run("F. real cancellation 1 -> 2 -> visible automatically without republish", func(t *testing.T) {
		prodID, variantID := createPendingProduct("Cancel Test Product", "cancel-stock-"+uuid.New().String()[:8], sellerID)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 2, 0, now(), now())
		`, uuid.New(), prodID, variantID, sellerID)
		require.NoError(t, err)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// Customer puts in cart and creates order
		cartBody, _ := json.Marshal(map[string]any{
			"productId":        prodID.String(),
			"productVariantId": variantID.String(),
			"quantity":         1,
		})
		cartReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReq.Header.Set("Authorization", "Bearer "+customerToken)
		cartReq.Header.Set("Content-Type", "application/json")
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		require.Equal(t, http.StatusCreated, cartRec.Code)

		orderBody, _ := json.Marshal(map[string]any{
			"customerName":     "Cancel Customer",
			"customerPhone":    "+79981234568",
			"customerEmail":    customerEmail,
			"deliveryAddress":  "Test Street 2",
			"deliveryMethodId": deliveryMethodID,
		})
		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(orderBody))
		orderReq.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		require.Equal(t, http.StatusCreated, orderRec.Code)

		var orderCreated struct {
			ID uuid.UUID `json:"id"`
		}
		err = json.NewDecoder(orderRec.Body).Decode(&orderCreated)
		require.NoError(t, err)

		// Product is now hidden (free = 1)
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRec.Code)

		// Customer cancels order via real production API
		cancelReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer/orders/%s/cancel", orderCreated.ID), bytes.NewReader([]byte(`{"reason":"not_needed"}`)))
		cancelReq.Header.Set("Authorization", "Bearer "+customerToken)
		cancelReq.Header.Set("Content-Type", "application/json")
		cancelRec := httptest.NewRecorder()
		r.ServeHTTP(cancelRec, cancelReq)
		assert.Equal(t, http.StatusNoContent, cancelRec.Code)

		// Reserved stock is freed, canonical free is 2 again
		var reservedStock int
		err = pgClient.Pool.QueryRow(ctx, "SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&reservedStock)
		require.NoError(t, err)
		assert.Equal(t, 0, reservedStock, "order cancellation must release reserved_stock to 0")

		// Product automatically becomes visible again on PDP
		pdpReq2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec2 := httptest.NewRecorder()
		r.ServeHTTP(pdpRec2, pdpReq2)
		assert.Equal(t, http.StatusOK, pdpRec2.Code, "Product must automatically reappear on PDP after order cancellation")
	})

	t.Run("G. real FBO receiving 1 -> 2 -> visible automatically without republish", func(t *testing.T) {
		prodID, variantID := createPendingProduct("Receiving Test Product", "rec-stock-"+uuid.New().String()[:8], sellerID)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 1, 0, now(), now())
		`, uuid.New(), prodID, variantID, sellerID)
		require.NoError(t, err)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// Initially hidden at free=1
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRec.Code, "Initially hidden at free=1")

		// Set up incoming supply for 1 unit
		supplyID := uuid.New()
		qrToken := "QR-" + supplyID.String()[:12]
		supplyNumber := "SUP-" + supplyID.String()[:8]
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, seller_id, status, supply_number, qr_token, handoff_method, created_at, updated_at)
			VALUES ($1, $2, 'shipped_by_seller', $3, $4, 'pickup', now(), now())
		`, supplyID, sellerID, supplyNumber, qrToken)
		require.NoError(t, err)

		supplyItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 1, now(), now())
		`, supplyItemID, supplyID, variantID)
		require.NoError(t, err)

		// 1. Mark supply arrived via API
		arriveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/%s/arrive", supplyID), nil)
		arriveReq.Header.Set("Authorization", "Bearer "+adminToken)
		arriveRec := httptest.NewRecorder()
		r.ServeHTTP(arriveRec, arriveReq)
		assert.Equal(t, http.StatusNoContent, arriveRec.Code)

		// 2. Start receiving session via API
		startBody, _ := json.Marshal(map[string]any{"qr_token": qrToken})
		startReq := httptest.NewRequest(http.MethodPost, "/api/admin/receiving/sessions", bytes.NewReader(startBody))
		startReq.Header.Set("Authorization", "Bearer "+adminToken)
		startReq.Header.Set("Content-Type", "application/json")
		startRec := httptest.NewRecorder()
		r.ServeHTTP(startRec, startReq)
		assert.Equal(t, http.StatusOK, startRec.Code)

		var sessionResp struct {
			ID    uuid.UUID `json:"id"`
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
		}
		err = json.NewDecoder(startRec.Body).Decode(&sessionResp)
		require.NoError(t, err)
		require.NotEmpty(t, sessionResp.Items)

		// 3. Scan 1 unit via API
		scanBody, _ := json.Marshal(map[string]any{
			"variantId": variantID.String(),
			"quantity":  1,
		})
		scanReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/sessions/%s/scan", sessionResp.ID), bytes.NewReader(scanBody))
		scanReq.Header.Set("Authorization", "Bearer "+adminToken)
		scanReq.Header.Set("Content-Type", "application/json")
		scanRec := httptest.NewRecorder()
		r.ServeHTTP(scanRec, scanReq)
		assert.Equal(t, http.StatusNoContent, scanRec.Code)

		// 4. Finalize receiving session via API
		finalizeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/sessions/%s/finalize", sessionResp.ID), bytes.NewReader([]byte(`{}`)))
		finalizeReq.Header.Set("Authorization", "Bearer "+adminToken)
		finalizeReq.Header.Set("Content-Type", "application/json")
		finalizeRec := httptest.NewRecorder()
		r.ServeHTTP(finalizeRec, finalizeReq)
		assert.Equal(t, http.StatusNoContent, finalizeRec.Code)

		// Verify DB total_stock is now 2 (free = 2)
		var totalStock int
		err = pgClient.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalStock)
		require.NoError(t, err)
		assert.Equal(t, 2, totalStock, "receiving must update inventory total_stock to 2")

		// 5. Product automatically becomes visible on PDP
		pdpReq2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec2 := httptest.NewRecorder()
		r.ServeHTTP(pdpRec2, pdpReq2)
		assert.Equal(t, http.StatusOK, pdpRec2.Code, "Product must automatically appear on PDP after receiving reaches free=2")

		// Status remains 'published' without new moderation logs
		var status string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", prodID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "published", status)
	})

	t.Run("H. serialized allocated units excluded and no double counting", func(t *testing.T) {
		prodID, variantID := createPendingProduct("Serialized Test Product", "ser-stock-"+uuid.New().String()[:8], sellerID)

		// Mixed inventory: 1 serialized unit in warehouse + 1 legacy unit in inventory_items
		// Total stock = 2, reserved = 0 -> Free = 2
		invID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 2, 0, now(), now())
		`, invID, prodID, variantID, sellerID)
		require.NoError(t, err)

		supplyID := uuid.New()
		supplyItemID := uuid.New()
		supplyNumber := "SUP-SER-" + supplyID.String()[:8]
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
			VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
		`, supplyID, sellerID, supplyNumber)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 1, now(), now())
		`, supplyItemID, supplyID, variantID)
		require.NoError(t, err)

		unitID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 1, 'warehouse', now(), now())
		`, unitID, "ZMU-"+unitID.String()[:8], variantID, supplyID, supplyItemID)
		require.NoError(t, err)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// 1. Initially visible at free=2
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusOK, pdpRec.Code, "Initially visible with 2 free units")

		// 2. Customer creates order for 1 unit
		cartBody, _ := json.Marshal(map[string]any{
			"productId":        prodID.String(),
			"productVariantId": variantID.String(),
			"quantity":         1,
		})
		cartReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReq.Header.Set("Authorization", "Bearer "+customerToken)
		cartReq.Header.Set("Content-Type", "application/json")
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		require.Equal(t, http.StatusCreated, cartRec.Code)

		orderBody, _ := json.Marshal(map[string]any{
			"customerName":     "Serialized Customer",
			"customerPhone":    "+79981234569",
			"customerEmail":    customerEmail,
			"deliveryAddress":  "Serialized Lane 1",
			"deliveryMethodId": deliveryMethodID,
		})
		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(orderBody))
		orderReq.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		require.Equal(t, http.StatusCreated, orderRec.Code)

		var orderCreated struct {
			ID uuid.UUID `json:"id"`
		}
		json.NewDecoder(orderRec.Body).Decode(&orderCreated)

		// Verify the warehouse serialized unit was allocated
		var allocCount int
		err = pgClient.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM order_item_allocations
			WHERE inventory_unit_id = $1 AND released_at IS NULL
		`, unitID).Scan(&allocCount)
		require.NoError(t, err)
		assert.Equal(t, 1, allocCount, "Serialized warehouse unit must be allocated to the active order")

		// Verify canonical free stock is now 1: total_stock=2, reserved_stock=1
		var tot, res int
		err = pgClient.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invID).Scan(&tot, &res)
		require.NoError(t, err)
		assert.Equal(t, 2, tot)
		assert.Equal(t, 1, res)

		// 3. Product must be hidden from PDP (free=1 < 2)
		pdpReq2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec2 := httptest.NewRecorder()
		r.ServeHTTP(pdpRec2, pdpReq2)
		assert.Equal(t, http.StatusNotFound, pdpRec2.Code, "Product must be hidden when free units drop to 1")

		// 4. Cancel order: allocation released, product becomes visible again
		cancelReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer/orders/%s/cancel", orderCreated.ID), bytes.NewReader([]byte(`{"reason":"not_needed"}`)))
		cancelReq.Header.Set("Authorization", "Bearer "+customerToken)
		cancelReq.Header.Set("Content-Type", "application/json")
		cancelRec := httptest.NewRecorder()
		r.ServeHTTP(cancelRec, cancelReq)
		assert.Equal(t, http.StatusNoContent, cancelRec.Code)

		// Allocation must be released
		var activeAllocs int
		err = pgClient.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM order_item_allocations
			WHERE inventory_unit_id = $1 AND released_at IS NULL
		`, unitID).Scan(&activeAllocs)
		require.NoError(t, err)
		assert.Equal(t, 0, activeAllocs, "Allocation must be released")

		// Product is visible again
		pdpReq3 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec3 := httptest.NewRecorder()
		r.ServeHTTP(pdpRec3, pdpReq3)
		assert.Equal(t, http.StatusOK, pdpRec3.Code, "Product must be visible again after allocation release")
	})

	t.Run("I. inactive seller -> hidden regardless of stock", func(t *testing.T) {
		inactiveSellerID := uuid.New()
		inactiveSlug := "inactive-seller-" + inactiveSellerID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Inactive', $2, $3, 'blocked', now(), now())`, inactiveSellerID, inactiveSlug, inactiveSlug+"@test.com")
		require.NoError(t, err)

		prodID, variantID := createPendingProduct("Inactive Prod", "inact-prod-"+uuid.New().String()[:8], inactiveSellerID)
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 5, 0, now(), now())`, uuid.New(), prodID, variantID, inactiveSellerID)
		require.NoError(t, err)

		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approve suspended"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)

		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRec.Code, "Inactive seller product must be 404")
	})
}
