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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func setupProductStockAlertsTestEnv(t *testing.T) (context.Context, http.Handler, func(), *postgres.Client, *auth.TokenService) {
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

func TestNTF2_ProductStockAlerts(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupProductStockAlertsTestEnv(t)
	defer cleanup()

	var (
		createdUserIDs      []uuid.UUID
		createdStaffRoleIDs []uuid.UUID
		createdSellerIDs    []uuid.UUID
		createdCategoryIDs  []uuid.UUID
		createdProductIDs   []uuid.UUID
		createdVariantIDs   []uuid.UUID
		createdSupplyIDs    []uuid.UUID
	)

	t.Cleanup(func() {
		var dbName string
		if err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err == nil && dbName == "zamk_test" {
			if len(createdSellerIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM notifications WHERE recipient_seller_id = ANY($1)", createdSellerIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM seller_status_history WHERE seller_id = ANY($1)", createdSellerIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM seller_users WHERE seller_id = ANY($1)", createdSellerIDs)
			}
			if len(createdVariantIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM order_item_allocations WHERE inventory_unit_id IN (SELECT id FROM inventory_units WHERE product_variant_id = ANY($1))", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM order_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM cart_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM reservations WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM stock_movements WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_units WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE id = ANY($1)", createdVariantIDs)
			}
			if len(createdProductIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_moderation_logs WHERE product_id = ANY($1)", createdProductIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProductIDs)
			}
			if len(createdSupplyIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM receiving_session_items WHERE session_id IN (SELECT id FROM receiving_sessions WHERE supply_id = ANY($1))", createdSupplyIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM receiving_sessions WHERE supply_id = ANY($1)", createdSupplyIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM seller_supply_items WHERE supply_id = ANY($1)", createdSupplyIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM seller_supplies WHERE id = ANY($1)", createdSupplyIDs)
			}

			if len(createdUserIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM orders WHERE customer_id = ANY($1)", createdUserIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = ANY($1)", createdUserIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", createdUserIDs)
			}
			if len(createdStaffRoleIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = ANY($1)", createdStaffRoleIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = ANY($1)", createdStaffRoleIDs)
			}
			if len(createdSellerIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
			}
			if len(createdCategoryIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCategoryIDs)
			}
		}
	})

	// 1. Create Admin user
	adminID := uuid.New()
	createdUserIDs = append(createdUserIDs, adminID)
	adminEmail := "admin-alert-" + adminID.String()[:8] + "@test.com"
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Admin User', 'hash', 'admin', 'active', now(), now())
	`, adminID, adminEmail, "+7999"+adminID.String()[:7])
	require.NoError(t, err)

	roleID := uuid.New()
	createdStaffRoleIDs = append(createdStaffRoleIDs, roleID)
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'AlertRole')`, roleID, roleID.String()[:8])
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_role_permissions (role_id, permission)
		VALUES ($1, 'products.approve'), ($1, 'products.reject'), ($1, 'inventory.receipt'), ($1, 'inventory.manage'), ($1, 'sellers.update_status'), ($1, 'sellers.read')
	`, roleID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, adminID, roleID)
	require.NoError(t, err)

	adminToken, err := tokenService.GenerateAccessToken(adminID, adminEmail, "admin")
	require.NoError(t, err)

	// 2. Create Seller Owner & Seller
	sellerOwnerID := uuid.New()
	createdUserIDs = append(createdUserIDs, sellerOwnerID)
	sellerOwnerEmail := "seller-owner-" + sellerOwnerID.String()[:8] + "@test.com"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Seller Owner', 'hash', 'seller', 'active', now(), now())
	`, sellerOwnerID, sellerOwnerEmail, "+7997"+sellerOwnerID.String()[:7])
	require.NoError(t, err)

	sellerID := uuid.New()
	createdSellerIDs = append(createdSellerIDs, sellerID)
	sellerSlug := "alert-seller-" + sellerID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Stock Alert Brand', $2, $3, 'active', now(), now())
	`, sellerID, sellerSlug, sellerSlug+"@test.com")
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, sellerOwnerID)
	require.NoError(t, err)

	sellerToken, err := tokenService.GenerateAccessToken(sellerOwnerID, sellerOwnerEmail, "seller")
	require.NoError(t, err)

	// 3. Create Customer
	customerID := uuid.New()
	createdUserIDs = append(createdUserIDs, customerID)
	customerEmail := "customer-alert-" + customerID.String()[:8] + "@test.com"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Customer User', 'hash', 'customer', 'active', now(), now())
	`, customerID, customerEmail, "+7996"+customerID.String()[:7])
	require.NoError(t, err)

	customerToken, err := tokenService.GenerateAccessToken(customerID, customerEmail, "customer")
	require.NoError(t, err)

	// 4. Create Category & Delivery Method
	catID := uuid.New()
	createdCategoryIDs = append(createdCategoryIDs, catID)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Alert Category', $2, true, now(), now())
	`, catID, "alert-cat-"+catID.String()[:8])
	require.NoError(t, err)

	var deliveryMethodID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM delivery_methods WHERE is_active = true LIMIT 1").Scan(&deliveryMethodID)
	if err != nil {
		deliveryMethodID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
			VALUES ($1, 'alert-delivery', 'Alert Delivery', 50000, true, now(), now())
		`, deliveryMethodID)
		require.NoError(t, err)
	}

	createProductFixture := func(title, slug, status string, priceCents int) (uuid.UUID, uuid.UUID) {
		prodID := uuid.New()
		variantID := uuid.New()
		createdProductIDs = append(createdProductIDs, prodID)
		createdVariantIDs = append(createdVariantIDs, variantID)
		now := time.Now()

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				submitted_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'RUB', $7, $8, $8, $8)
		`, prodID, sellerID, catID, title, slug, priceCents, status, now)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (
				id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at
			) VALUES ($1, $2, $3, $3, $4, $5, true, $6, $6)
		`, variantID, prodID, "SKU-"+variantID.String()[:8], "BC-"+variantID.String()[:8], priceCents, now)
		require.NoError(t, err)

		return prodID, variantID
	}


	getAlertsForProduct := func(prodID uuid.UUID) []notifications.Notification {
		dedupeKey := fmt.Sprintf("stock:critical:%s", prodID.String())
		rows, err := pgClient.Pool.Query(ctx, `
			SELECT id, recipient_seller_id, recipient_kind, type, title, body, kind, severity, status, dedupe_key, action_url, metadata, read_at, resolved_at, created_at
			FROM notifications
			WHERE recipient_seller_id = $1 AND dedupe_key = $2
			ORDER BY created_at ASC
		`, sellerID, dedupeKey)
		require.NoError(t, err)
		defer rows.Close()

		var res []notifications.Notification
		for rows.Next() {
			var n notifications.Notification
			err := rows.Scan(
				&n.ID, &n.RecipientSellerID, &n.RecipientKind, &n.Type, &n.Title, &n.Body, &n.Kind, &n.Severity, &n.Status, &n.DedupeKey, &n.ActionURL, &n.Metadata, &n.ReadAt, &n.ResolvedAt, &n.CreatedAt,
			)
			require.NoError(t, err)
			res = append(res, n)
		}
		return res
	}

	t.Run("A. PRODUCT NOT YET PUBLIC: draft/pending/rejected + free 0/1 -> no alert", func(t *testing.T) {
		// Draft product with free=0
		draftProdID, draftVarID := createProductFixture("Draft Product", "draft-"+uuid.New().String()[:8], "draft", 100000)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 0, 0, now(), now())
		`, uuid.New(), draftProdID, draftVarID, sellerID)
		require.NoError(t, err)

		alerts := getAlertsForProduct(draftProdID)
		assert.Empty(t, alerts, "draft product must not trigger critical stock alert")

		// Pending product with free=1
		pendingProdID, pendingVarID := createProductFixture("Pending Product", "pending-"+uuid.New().String()[:8], "pending_moderation", 100000)
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 1, 0, now(), now())
		`, uuid.New(), pendingProdID, pendingVarID, sellerID)
		require.NoError(t, err)

		alerts = getAlertsForProduct(pendingProdID)
		assert.Empty(t, alerts, "pending moderation product must not trigger critical stock alert")

		// Reject pending product via API
		rejectReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/reject", pendingProdID), bytes.NewReader([]byte(`{"comment":"Quality issues"}`)))
		rejectReq.Header.Set("Authorization", "Bearer "+adminToken)
		rejectReq.Header.Set("Content-Type", "application/json")
		rejectRec := httptest.NewRecorder()
		r.ServeHTTP(rejectRec, rejectReq)
		assert.Equal(t, http.StatusOK, rejectRec.Code)

		alerts = getAlertsForProduct(pendingProdID)
		assert.Empty(t, alerts, "rejected product must not trigger critical stock alert")
	})

	t.Run("B. PUBLISHED + FREE 2: visible, no active alert", func(t *testing.T) {
		prodID, varID := createProductFixture("Visible Product", "vis-"+uuid.New().String()[:8], "pending_moderation", 200000)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 2, 0, now(), now())
		`, uuid.New(), prodID, varID, sellerID)
		require.NoError(t, err)

		// Admin approves
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// Public PDP should be 200 OK
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusOK, pdpRec.Code)

		// No alert
		alerts := getAlertsForProduct(prodID)
		assert.Empty(t, alerts, "free=2 product must have no critical stock alert")
	})

	t.Run("C, D, E, G, I: Full lifecycle on real order reservation, cancellation, recurrence and status invariant", func(t *testing.T) {
		prodID, varID := createProductFixture("Lifecycle Product", "life-"+uuid.New().String()[:8], "pending_moderation", 150000)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 2, 0, now(), now())
		`, uuid.New(), prodID, varID, sellerID)
		require.NoError(t, err)

		// Approve product
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// --- PHASE C: Order 1 unit (free 2 -> 1) ---
		// Add to cart
		cartBody, _ := json.Marshal(map[string]any{
			"productId":        prodID.String(),
			"productVariantId": varID.String(),
			"quantity":         1,
		})
		cartReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReq.Header.Set("Authorization", "Bearer "+customerToken)
		cartReq.Header.Set("Content-Type", "application/json")
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		assert.Equal(t, http.StatusCreated, cartRec.Code)

		// Checkout order 1
		checkoutBody, _ := json.Marshal(map[string]any{
			"customerName":     "Test Customer",
			"customerPhone":    "+79981234567",
			"customerEmail":    customerEmail,
			"deliveryAddress":  "Test Street 1",
			"deliveryMethodId": deliveryMethodID,
		})
		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(checkoutBody))
		orderReq.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		assert.Equal(t, http.StatusCreated, orderRec.Code)

		var order1Resp struct {
			ID uuid.UUID `json:"id"`
		}
		err = json.NewDecoder(orderRec.Body).Decode(&order1Resp)
		require.NoError(t, err)

		// Storefront check: hidden on PDP
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRec.Code, "Product with free=1 must be hidden from storefront")

		// Alert check: exactly ONE active alert created
		alerts := getAlertsForProduct(prodID)
		require.Len(t, alerts, 1, "Exactly one alert must exist for product")
		alert1 := alerts[0]
		assert.Equal(t, "alert", alert1.Kind)
		assert.Equal(t, "critical", alert1.Severity)
		require.NotNil(t, alert1.Status)
		assert.Equal(t, "active", *alert1.Status)
		assert.Equal(t, "stock_critical_hidden", alert1.Type)
		assert.Equal(t, "Карточка скрыта из магазина", alert1.Title)
		assert.Equal(t, "Свободный остаток — 1 шт. Для показа товара в магазине необходимо минимум 2 свободные единицы.", alert1.Body)
		require.NotNil(t, alert1.ActionURL)
		assert.Equal(t, "/supplies/new", *alert1.ActionURL)
		assert.Nil(t, alert1.ResolvedAt)
		assert.Equal(t, prodID.String(), alert1.Metadata["productId"])
		assert.Equal(t, float64(1), alert1.Metadata["freeSellableStock"])
		assert.Equal(t, float64(2), alert1.Metadata["minimumRequiredStock"])

		// --- PHASE D: 1 -> 0 while alert active ---
		// Add 2nd unit to cart and checkout
		cartReq2 := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReq2.Header.Set("Authorization", "Bearer "+customerToken)
		cartReq2.Header.Set("Content-Type", "application/json")
		cartRec2 := httptest.NewRecorder()
		r.ServeHTTP(cartRec2, cartReq2)
		assert.Equal(t, http.StatusCreated, cartRec2.Code)

		orderReq2 := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(checkoutBody))
		orderReq2.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq2.Header.Set("Content-Type", "application/json")
		orderRec2 := httptest.NewRecorder()
		r.ServeHTTP(orderRec2, orderReq2)
		assert.Equal(t, http.StatusCreated, orderRec2.Code)

		var order2Resp struct {
			ID uuid.UUID `json:"id"`
		}
		err = json.NewDecoder(orderRec2.Body).Decode(&order2Resp)
		require.NoError(t, err)

		// Assert SAME alert ID remains (no duplicate alert)
		alertsD := getAlertsForProduct(prodID)
		require.Len(t, alertsD, 1, "Must still have exactly one alert (no duplicate)")
		alertD := alertsD[0]
		assert.Equal(t, alert1.ID, alertD.ID, "Alert ID must remain identical on 1 -> 0 transition")
		assert.Equal(t, "active", *alertD.Status)
		assert.Equal(t, "Свободного остатка нет. Карточка скрыта из магазина до пополнения товара.", alertD.Body)
		assert.Equal(t, float64(0), alertD.Metadata["freeSellableStock"])

		// --- PHASE E: Cancel orders releasing reservations back to >= 2 ---
		// Cancel order 1
		cancelReq1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer/orders/%s/cancel", order1Resp.ID), bytes.NewReader([]byte(`{"reason":"Customer changed mind"}`)))
		cancelReq1.Header.Set("Authorization", "Bearer "+customerToken)
		cancelReq1.Header.Set("Content-Type", "application/json")
		cancelRec1 := httptest.NewRecorder()
		r.ServeHTTP(cancelRec1, cancelReq1)
		assert.Equal(t, http.StatusNoContent, cancelRec1.Code)

		// Cancel order 2
		cancelReq2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer/orders/%s/cancel", order2Resp.ID), bytes.NewReader([]byte(`{"reason":"Customer changed mind"}`)))
		cancelReq2.Header.Set("Authorization", "Bearer "+customerToken)
		cancelReq2.Header.Set("Content-Type", "application/json")
		cancelRec2 := httptest.NewRecorder()
		r.ServeHTTP(cancelRec2, cancelReq2)
		assert.Equal(t, http.StatusNoContent, cancelRec2.Code)

		// Storefront check: product is visible again (200)
		pdpReqE := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRecE := httptest.NewRecorder()
		r.ServeHTTP(pdpRecE, pdpReqE)
		assert.Equal(t, http.StatusOK, pdpRecE.Code, "Product must be visible again after cancellation restores free=2")

		// Alert check: alert is resolved!
		alertsE := getAlertsForProduct(prodID)
		require.Len(t, alertsE, 1, "Old alert remains in history")
		assert.Equal(t, alert1.ID, alertsE[0].ID)
		assert.Equal(t, "resolved", *alertsE[0].Status)
		require.NotNil(t, alertsE[0].ResolvedAt)

		// --- PHASE G: Recurrence (2 -> 1 again creates a NEW active alert) ---
		cartReqG := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReqG.Header.Set("Authorization", "Bearer "+customerToken)
		cartReqG.Header.Set("Content-Type", "application/json")
		cartRecG := httptest.NewRecorder()
		r.ServeHTTP(cartRecG, cartReqG)
		assert.Equal(t, http.StatusCreated, cartRecG.Code)

		orderReqG := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(checkoutBody))
		orderReqG.Header.Set("Authorization", "Bearer "+customerToken)
		orderReqG.Header.Set("Content-Type", "application/json")
		orderRecG := httptest.NewRecorder()
		r.ServeHTTP(orderRecG, orderReqG)
		assert.Equal(t, http.StatusCreated, orderRecG.Code)

		alertsG := getAlertsForProduct(prodID)
		require.Len(t, alertsG, 2, "Must now have 2 notifications: 1 resolved historical and 1 new active")
		var activeAlert, resolvedAlert *notifications.Notification
		for i := range alertsG {
			if *alertsG[i].Status == "active" {
				activeAlert = &alertsG[i]
			} else if *alertsG[i].Status == "resolved" {
				resolvedAlert = &alertsG[i]
			}
		}
		require.NotNil(t, activeAlert, "New active alert must exist")
		require.NotNil(t, resolvedAlert, "Old resolved alert must exist")
		assert.NotEqual(t, resolvedAlert.ID, activeAlert.ID, "Recurrence must generate a NEW alert ID")
		assert.Equal(t, alert1.ID, resolvedAlert.ID)
		assert.Equal(t, "active", *activeAlert.Status)
		assert.Equal(t, "Свободный остаток — 1 шт. Для показа товара в магазине необходимо минимум 2 свободные единицы.", activeAlert.Body)

		// --- PHASE I: Status invariant ---
		var pStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", prodID).Scan(&pStatus)
		require.NoError(t, err)
		assert.Equal(t, "published", pStatus, "products.status must remain 'published' throughout stock transitions")

		var modLogCount int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_moderation_logs WHERE product_id = $1", prodID).Scan(&modLogCount)
		require.NoError(t, err)
		assert.Equal(t, 1, modLogCount, "Moderation history must not be created by stock alert transitions (only initial approve)")
	})

	t.Run("F. REAL FBO RECEIVING 1 -> 2: resolves active alert via real Supply receiving session", func(t *testing.T) {
		prodID, varID := createProductFixture("FBO Receiving Product", "rec-alert-"+uuid.New().String()[:8], "pending_moderation", 300000)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 1, 0, now(), now())
		`, uuid.New(), prodID, varID, sellerID)
		require.NoError(t, err)

		// Approve product -> free=1 -> alert created!
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// Product is hidden
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRec.Code)

		// Active alert exists
		alertsBefore := getAlertsForProduct(prodID)
		require.Len(t, alertsBefore, 1)
		assert.Equal(t, "active", *alertsBefore[0].Status)

		// Create real Supply for 1 unit
		supplyID := uuid.New()
		createdSupplyIDs = append(createdSupplyIDs, supplyID)
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
		`, supplyItemID, supplyID, varID)
		require.NoError(t, err)

		// 1. Arrive
		arriveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/%s/arrive", supplyID), nil)
		arriveReq.Header.Set("Authorization", "Bearer "+adminToken)
		arriveRec := httptest.NewRecorder()
		r.ServeHTTP(arriveRec, arriveReq)
		assert.Equal(t, http.StatusNoContent, arriveRec.Code)

		// 2. Start session
		startBody, _ := json.Marshal(map[string]any{"qr_token": qrToken})
		startReq := httptest.NewRequest(http.MethodPost, "/api/admin/receiving/sessions", bytes.NewReader(startBody))
		startReq.Header.Set("Authorization", "Bearer "+adminToken)
		startReq.Header.Set("Content-Type", "application/json")
		startRec := httptest.NewRecorder()
		r.ServeHTTP(startRec, startReq)
		assert.Equal(t, http.StatusOK, startRec.Code)

		var sessionResp struct {
			ID uuid.UUID `json:"id"`
		}
		err = json.NewDecoder(startRec.Body).Decode(&sessionResp)
		require.NoError(t, err)

		// 3. Scan 1 unit
		scanBody, _ := json.Marshal(map[string]any{
			"variantId": varID.String(),
			"quantity":  1,
		})
		scanReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/sessions/%s/scan", sessionResp.ID), bytes.NewReader(scanBody))
		scanReq.Header.Set("Authorization", "Bearer "+adminToken)
		scanReq.Header.Set("Content-Type", "application/json")
		scanRec := httptest.NewRecorder()
		r.ServeHTTP(scanRec, scanReq)
		assert.Equal(t, http.StatusNoContent, scanRec.Code)

		// 4. Finalize session
		finalizeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/sessions/%s/finalize", sessionResp.ID), bytes.NewReader([]byte(`{}`)))
		finalizeReq.Header.Set("Authorization", "Bearer "+adminToken)
		finalizeReq.Header.Set("Content-Type", "application/json")
		finalizeRec := httptest.NewRecorder()
		r.ServeHTTP(finalizeRec, finalizeReq)
		assert.Equal(t, http.StatusNoContent, finalizeRec.Code)

		// Verify PDP is visible (200)
		pdpRecAfter := httptest.NewRecorder()
		r.ServeHTTP(pdpRecAfter, pdpReq)
		assert.Equal(t, http.StatusOK, pdpRecAfter.Code, "Product must be visible on PDP after receiving reaches free=2")

		// Verify alert is resolved
		alertsAfter := getAlertsForProduct(prodID)
		require.Len(t, alertsAfter, 1)
		assert.Equal(t, "resolved", *alertsAfter[0].Status)
		assert.NotNil(t, alertsAfter[0].ResolvedAt)
	})

	t.Run("H. MULTI-VARIANT PRODUCT: VarA=1, VarB=1 (free=2) -> no alert; reserve VarA (free=1) -> ONE product-level alert", func(t *testing.T) {
		prodID := uuid.New()
		varA := uuid.New()
		varB := uuid.New()
		now := time.Now()

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				submitted_at, created_at, updated_at
			) VALUES ($1, $2, $3, 'Multi Variant Product', $4, 250000, 'RUB', 'pending_moderation', $5, $5, $5)
		`, prodID, sellerID, catID, "multi-"+prodID.String()[:8], now)
		require.NoError(t, err)

		skuA := "SKU-A-" + varA.String()[:8]
		skuB := "SKU-B-" + varB.String()[:8]
		bcA := "BC-A-" + varA.String()[:8]
		bcB := "BC-B-" + varB.String()[:8]

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES
				($1, $2, $3, $3, $4, 250000, true, $5, $5),
				($6, $2, $7, $7, $8, 250000, true, $5, $5)
		`, varA, prodID, skuA, bcA, now, varB, skuB, bcB)
		require.NoError(t, err)

		// Variant A: 1 unit, Variant B: 1 unit -> Total free = 2
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES
				($1, $2, $3, $4, 1, 0, now(), now()),
				($5, $2, $6, $4, 1, 0, now(), now())
		`, uuid.New(), prodID, varA, sellerID, uuid.New(), varB)
		require.NoError(t, err)

		// Admin approves
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// Product has free=2 -> visible, NO alert
		pdpReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		pdpRec := httptest.NewRecorder()
		r.ServeHTTP(pdpRec, pdpReq)
		assert.Equal(t, http.StatusOK, pdpRec.Code)

		alerts := getAlertsForProduct(prodID)
		assert.Empty(t, alerts, "Multi-variant product with total free=2 must have NO alerts")

		// Customer orders Variant A (quantity=1)
		cartBody, _ := json.Marshal(map[string]any{
			"productId":        prodID.String(),
			"productVariantId": varA.String(),
			"quantity":         1,
		})
		cartReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReq.Header.Set("Authorization", "Bearer "+customerToken)
		cartReq.Header.Set("Content-Type", "application/json")
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		assert.Equal(t, http.StatusCreated, cartRec.Code)

		checkoutBody, _ := json.Marshal(map[string]any{
			"customerName":     "Test Customer",
			"customerPhone":    "+79981234567",
			"customerEmail":    customerEmail,
			"deliveryAddress":  "Multi Street 1",
			"deliveryMethodId": deliveryMethodID,
		})
		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(checkoutBody))
		orderReq.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		assert.Equal(t, http.StatusCreated, orderRec.Code)

		// Now Variant A free=0, Variant B free=1 -> product free=1 < 2!
		// Product hidden on PDP
		pdpRecHidden := httptest.NewRecorder()
		r.ServeHTTP(pdpRecHidden, pdpReq)
		assert.Equal(t, http.StatusNotFound, pdpRecHidden.Code)

		// Exactly ONE product-level alert (NOT per-variant)
		alertsAfter := getAlertsForProduct(prodID)
		require.Len(t, alertsAfter, 1, "Must create exactly ONE product-level alert, never one per variant")
		assert.Equal(t, fmt.Sprintf("stock:critical:%s", prodID.String()), *alertsAfter[0].DedupeKey)
		assert.Equal(t, "active", *alertsAfter[0].Status)
		assert.Equal(t, float64(1), alertsAfter[0].Metadata["freeSellableStock"])
	})

	t.Run("J. SELLER NOTIFICATIONS API & READ != RESOLVED", func(t *testing.T) {
		// Fetch notifications via Seller API
		listReq := httptest.NewRequest(http.MethodGet, "/api/seller/notifications", nil)
		listReq.Header.Set("Authorization", "Bearer "+sellerToken)
		listRec := httptest.NewRecorder()
		r.ServeHTTP(listRec, listReq)
		assert.Equal(t, http.StatusOK, listRec.Code)

		var listResp struct {
			Items []struct {
				ID         uuid.UUID              `json:"id"`
				Kind       string                 `json:"kind"`
				Severity   string                 `json:"severity"`
				Status     *string                `json:"status"`
				Type       string                 `json:"type"`
				ActionURL  *string                `json:"actionUrl"`
				ReadAt     *time.Time             `json:"readAt"`
				ResolvedAt *time.Time             `json:"resolvedAt"`
				Metadata   map[string]interface{} `json:"metadata"`
			} `json:"items"`
			TotalCount int `json:"totalCount"`
		}
		err := json.NewDecoder(listRec.Body).Decode(&listResp)
		require.NoError(t, err)
		require.NotEmpty(t, listResp.Items)

		var activeAlertItem *struct {
			ID         uuid.UUID              `json:"id"`
			Kind       string                 `json:"kind"`
			Severity   string                 `json:"severity"`
			Status     *string                `json:"status"`
			Type       string                 `json:"type"`
			ActionURL  *string                `json:"actionUrl"`
			ReadAt     *time.Time             `json:"readAt"`
			ResolvedAt *time.Time             `json:"resolvedAt"`
			Metadata   map[string]interface{} `json:"metadata"`
		}
		for i := range listResp.Items {
			if listResp.Items[i].Kind == "alert" && listResp.Items[i].Status != nil && *listResp.Items[i].Status == "active" {
				activeAlertItem = &listResp.Items[i]
				break
			}
		}
		require.NotNil(t, activeAlertItem, "Seller must receive active critical alert via API")
		assert.Equal(t, "critical", activeAlertItem.Severity)
		assert.Equal(t, "/supplies/new", *activeAlertItem.ActionURL)
		assert.Nil(t, activeAlertItem.ReadAt)
		assert.Nil(t, activeAlertItem.ResolvedAt)

		// Mark active alert as read via API: POST /api/seller/notifications/:id/read
		readReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/notifications/%s/read", activeAlertItem.ID), nil)
		readReq.Header.Set("Authorization", "Bearer "+sellerToken)
		readRec := httptest.NewRecorder()
		r.ServeHTTP(readRec, readReq)
		assert.Equal(t, http.StatusOK, readRec.Code)

		// Query DB: read_at must be populated, but status must remain active! (READ != RESOLVED)
		var readAt *time.Time
		var status string
		err = pgClient.Pool.QueryRow(ctx, "SELECT read_at, status FROM notifications WHERE id = $1", activeAlertItem.ID).Scan(&readAt, &status)
		require.NoError(t, err)
		assert.NotNil(t, readAt, "read_at must be populated after mark read")
		assert.Equal(t, "active", status, "READ != RESOLVED: alert status must remain 'active' when marked read")
	})

	var sellerBlockedProdID uuid.UUID

	t.Run("K. SELLER ACTIVE -> INACTIVE (BLOCKED): resolves active critical alert via real production path", func(t *testing.T) {
		prodID, varID := createProductFixture("Seller Blocked Product", "seller-blocked-prod-"+uuid.New().String()[:8], "pending_moderation", 150000)
		sellerBlockedProdID = prodID
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 1, 0, now(), now())
		`, uuid.New(), prodID, varID, sellerID)
		require.NoError(t, err)

		// Admin approves product -> transitions to published, free=1 < 2 -> creates active critical alert!
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// Active alert must exist
		alertsBefore := getAlertsForProduct(prodID)
		require.Len(t, alertsBefore, 1)
		assert.Equal(t, "active", *alertsBefore[0].Status)

		// 2. Admin blocks seller via REAL production path: PATCH /api/admin/sellers/{id}/status
		blockReason := "Blocked due to policy audit"
		body, _ := json.Marshal(map[string]any{
			"status": "blocked",
			"reason": blockReason,
		})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/admin/sellers/%s/status", sellerID), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		// 3. Assert seller status is blocked in DB
		var sellerStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM sellers WHERE id = $1", sellerID).Scan(&sellerStatus)
		require.NoError(t, err)
		assert.Equal(t, "blocked", sellerStatus)

		// 4. Assert active alert is now RESOLVED!
		alertsAfter := getAlertsForProduct(prodID)
		require.Len(t, alertsAfter, 1)
		assert.Equal(t, "resolved", *alertsAfter[0].Status)
		assert.NotNil(t, alertsAfter[0].ResolvedAt)
	})

	t.Run("L. SELLER INACTIVE (BLOCKED) -> ACTIVE: critical alert appears automatically without inventory mutation", func(t *testing.T) {
		// Product from Subtest K still has free=1, but seller was blocked so alert was resolved
		// Verify no active alerts exist for sellerID
		var activeCount int
		err := pgClient.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM notifications
			WHERE recipient_seller_id = $1 AND kind = 'alert' AND status = 'active'
		`, sellerID).Scan(&activeCount)
		require.NoError(t, err)
		assert.Equal(t, 0, activeCount, "Blocked seller must have zero active alerts")

		// Admin activates seller via REAL production path: PATCH /api/admin/sellers/{id}/status
		body, _ := json.Marshal(map[string]any{
			"status": "active",
		})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/admin/sellers/%s/status", sellerID), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Assert seller status is active
		var sellerStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM sellers WHERE id = $1", sellerID).Scan(&sellerStatus)
		require.NoError(t, err)
		assert.Equal(t, "active", sellerStatus)

		// Assert active critical alert appears automatically WITHOUT any stock mutation!
		err = pgClient.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM notifications
			WHERE recipient_seller_id = $1 AND kind = 'alert' AND status = 'active'
		`, sellerID).Scan(&activeCount)
		require.NoError(t, err)
		assert.True(t, activeCount >= 1, "Active critical stock alert must reappear automatically upon seller activation")
	})

	t.Run("M. SELLER STATUS TRANSITIONS DO NOT MODIFY PRODUCT MODERATION STATUS OR HISTORY", func(t *testing.T) {
		// Verify published products of the seller remain published
		var pStatus string
		err := pgClient.Pool.QueryRow(ctx, `
			SELECT status FROM products WHERE id = $1
		`, sellerBlockedProdID).Scan(&pStatus)
		require.NoError(t, err)
		assert.Equal(t, "published", pStatus, "Product status must remain 'published' across seller status transitions")

		// Verify zero moderation logs were generated for seller status transitions on sellerBlockedProdID
		var modLogsCount int
		err = pgClient.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM product_moderation_logs
			WHERE product_id = $1
		`, sellerBlockedProdID).Scan(&modLogsCount)
		require.NoError(t, err)
		assert.Equal(t, 1, modLogsCount, "Moderation logs must not be created by seller status changes (only the initial approval log exists)")
	})


	t.Run("N. ONE ORDER RESERVATION CAUSES ONE LOGICAL STOCK-ALERT RECONCILIATION", func(t *testing.T) {
		prodID, varID := createProductFixture("Single Reconcile Product", "single-rec-"+uuid.New().String()[:8], "pending_moderation", 120000)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 2, 0, now(), now())
		`, uuid.New(), prodID, varID, sellerID)
		require.NoError(t, err)

		// Approve product (free=2 -> no alert)
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		// Reserve 1 unit via customer order (2 -> 1)
		cartBody, _ := json.Marshal(map[string]any{
			"productId":        prodID.String(),
			"productVariantId": varID.String(),
			"quantity":         1,
		})
		cartReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(cartBody))
		cartReq.Header.Set("Authorization", "Bearer "+customerToken)
		cartReq.Header.Set("Content-Type", "application/json")
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		assert.Equal(t, http.StatusCreated, cartRec.Code)

		checkoutBody, _ := json.Marshal(map[string]any{
			"customerName":     "Single Order",
			"customerPhone":    "+79998887766",
			"customerEmail":    customerEmail,
			"deliveryAddress":  "Test Street 1",
			"deliveryMethodId": deliveryMethodID,
		})
		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(checkoutBody))
		orderReq.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		assert.Equal(t, http.StatusCreated, orderRec.Code)

		// Assert exactly ONE notification row with this dedupeKey was created
		dedupeKey := fmt.Sprintf("stock:critical:%s", prodID.String())
		var count int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE dedupe_key = $1", dedupeKey).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "One reservation must create exactly one notification row")
	})

	t.Run("O. RECEIVING MULTIPLE VARIANTS OF ONE PRODUCT DEDUPLICATES RECONCILIATION", func(t *testing.T) {
		prodID := uuid.New()
		var1 := uuid.New()
		var2 := uuid.New()
		now := time.Now()

		createdProductIDs = append(createdProductIDs, prodID)
		createdVariantIDs = append(createdVariantIDs, var1, var2)

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, submitted_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'Multi-variant Receiving Product', $4, 200000, 'RUB', 'pending_moderation', $5, $5, $5)
		`, prodID, sellerID, catID, "multi-recv-"+prodID.String()[:8], now)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES
				($1, $2, $3, $3, $4, 200000, true, $5, $5),
				($6, $2, $7, $7, $8, 200000, true, $5, $5)
		`, var1, prodID, "SKU-V1-"+var1.String()[:8], "BC-V1-"+var1.String()[:8], now,
			var2, "SKU-V2-"+var2.String()[:8], "BC-V2-"+var2.String()[:8])
		require.NoError(t, err)

		// 0 stock initially -> Approve creates active critical alert (free=0)
		approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/moderation/products/%s/approve", prodID), bytes.NewReader([]byte(`{"comment":"Approved"}`)))
		approveReq.Header.Set("Authorization", "Bearer "+adminToken)
		approveReq.Header.Set("Content-Type", "application/json")
		approveRec := httptest.NewRecorder()
		r.ServeHTTP(approveRec, approveReq)
		assert.Equal(t, http.StatusOK, approveRec.Code)

		alertsBefore := getAlertsForProduct(prodID)
		require.Len(t, alertsBefore, 1)
		assert.Equal(t, "active", *alertsBefore[0].Status)

		// Create Supply with both variants
		supplyID := uuid.New()
		createdSupplyIDs = append(createdSupplyIDs, supplyID)
		qrToken := "QR-" + supplyID.String()[:12]
		supplyNumber := "SUP-" + supplyID.String()[:8]
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, seller_id, status, supply_number, qr_token, handoff_method, created_at, updated_at)
			VALUES ($1, $2, 'shipped_by_seller', $3, $4, 'pickup', now(), now())
		`, supplyID, sellerID, supplyNumber, qrToken)
		require.NoError(t, err)

		supplyItemID1 := uuid.New()
		supplyItemID2 := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES
				($1, $2, $3, 1, now(), now()),
				($4, $5, $6, 1, now(), now())
		`, supplyItemID1, supplyID, var1, supplyItemID2, supplyID, var2)
		require.NoError(t, err)


		// 1. Arrive supply
		arriveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/%s/arrive", supplyID), nil)
		arriveReq.Header.Set("Authorization", "Bearer "+adminToken)
		arriveRec := httptest.NewRecorder()
		r.ServeHTTP(arriveRec, arriveReq)
		assert.Equal(t, http.StatusNoContent, arriveRec.Code)

		// 2. Start session
		startBody, _ := json.Marshal(map[string]any{"qr_token": qrToken})
		startReq := httptest.NewRequest(http.MethodPost, "/api/admin/receiving/sessions", bytes.NewReader(startBody))
		startReq.Header.Set("Authorization", "Bearer "+adminToken)
		startReq.Header.Set("Content-Type", "application/json")
		startRec := httptest.NewRecorder()
		r.ServeHTTP(startRec, startReq)
		assert.Equal(t, http.StatusOK, startRec.Code)

		var sessionResp struct {
			ID uuid.UUID `json:"id"`
		}
		err = json.NewDecoder(startRec.Body).Decode(&sessionResp)
		require.NoError(t, err)

		// 3. Scan Variant 1
		scanBody1, _ := json.Marshal(map[string]any{"variantId": var1.String(), "quantity": 1})
		scanReq1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/sessions/%s/scan", sessionResp.ID), bytes.NewReader(scanBody1))
		scanReq1.Header.Set("Authorization", "Bearer "+adminToken)
		scanReq1.Header.Set("Content-Type", "application/json")
		scanRec1 := httptest.NewRecorder()
		r.ServeHTTP(scanRec1, scanReq1)
		assert.Equal(t, http.StatusNoContent, scanRec1.Code)

		// 4. Scan Variant 2
		scanBody2, _ := json.Marshal(map[string]any{"variantId": var2.String(), "quantity": 1})
		scanReq2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/sessions/%s/scan", sessionResp.ID), bytes.NewReader(scanBody2))
		scanReq2.Header.Set("Authorization", "Bearer "+adminToken)
		scanReq2.Header.Set("Content-Type", "application/json")
		scanRec2 := httptest.NewRecorder()
		r.ServeHTTP(scanRec2, scanReq2)
		assert.Equal(t, http.StatusNoContent, scanRec2.Code)

		// 5. Finalize session (reconciles with deduplicated product ID)
		finalizeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/receiving/sessions/%s/finalize", sessionResp.ID), bytes.NewReader([]byte(`{}`)))
		finalizeReq.Header.Set("Authorization", "Bearer "+adminToken)
		finalizeReq.Header.Set("Content-Type", "application/json")
		finalizeRec := httptest.NewRecorder()
		r.ServeHTTP(finalizeRec, finalizeReq)
		assert.Equal(t, http.StatusNoContent, finalizeRec.Code)

		// Alert must now be resolved (free stock = 1 + 1 = 2)
		alertsAfter := getAlertsForProduct(prodID)
		require.Len(t, alertsAfter, 1)
		assert.Equal(t, "resolved", *alertsAfter[0].Status)
		assert.NotNil(t, alertsAfter[0].ResolvedAt)
	})
}
