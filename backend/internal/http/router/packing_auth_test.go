package router_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestAdminPackingRouter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
	}
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	insertUser := func(role string) uuid.UUID {
		id := uuid.New()
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test Admin', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		require.NoError(t, err)
		return id
	}

	insertAdminWithPerms := func(userID uuid.UUID, perms []string) {
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'PackingRole')`, roleID, code)
		require.NoError(t, err)
		for _, p := range perms {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
			require.NoError(t, err)
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	// Setup seller, category, product, variant
	sellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Packing Seller', $2, $3, 'active', NOW(), NOW())
	`, sellerID, "seller-"+sellerID.String()[:8], sellerID.String()+"@seller.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 1000, 'published', now(), now())`, prodID, sellerID, catID, uuid.New().String())
	require.NoError(t, err)

	variantID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())`, variantID, prodID, "SKU-"+variantID.String()[:8], "SSKU-"+variantID.String()[:8], "BARCODE-"+variantID.String()[:8])
	require.NoError(t, err)

	buyerID := insertUser("customer")
	orderID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'assembling', 1000, 'Buyer', 'Phone', 'Email', 'Addr')
	`, orderID, buyerID)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'assembling', 1000, 900, 900)
	`, fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	itemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Prod Item', 'prod-slug', 100, 1, 100, $6, 0)
	`, itemID, orderID, prodID, variantID, sellerID, fulfillmentID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`, supplyID, sellerID, uuid.New().String()[:8])
	require.NoError(t, err)
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitID := uuid.New()
	unitCode := "ZMU-PACK-ROUTER-" + uuid.New().String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'warehouse')`, unitID, unitCode, variantID, supplyID, supplyItemID)
	require.NoError(t, err)

	allocID := uuid.New()
	// Unpicked initially
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at) VALUES ($1, $2, $3, NULL)`, allocID, itemID, unitID)
	require.NoError(t, err)

	adminReadID := insertUser("admin")
	insertAdminWithPerms(adminReadID, []string{"orders.read"})
	adminReadTok := makeToken(adminReadID, "admin")

	adminPackID := insertUser("admin")
	insertAdminWithPerms(adminPackID, []string{"warehouse.packing"})
	adminPackTok := makeToken(adminPackID, "admin")

	adminUpdateID := insertUser("admin")
	insertAdminWithPerms(adminUpdateID, []string{"orders.update_status"})
	adminUpdateTok := makeToken(adminUpdateID, "admin")

	type errResponse struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	// 1. Unauthenticated -> 401
	t.Run("unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	// 2. Customer user -> 403
	t.Run("customer -> 403", func(t *testing.T) {
		custTok := makeToken(buyerID, "customer")
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+custTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 3. Admin with orders.read only (read-only) -> 403 Forbidden
	t.Run("admin with orders.read only -> 403 forbidden", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+adminReadTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 4. Admin without permissions -> 403
	t.Run("admin without permissions -> 403", func(t *testing.T) {
		adminNoPerm := insertUser("admin")
		insertAdminWithPerms(adminNoPerm, []string{"inventory.read"})
		noPermTok := makeToken(adminNoPerm, "admin")

		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+noPermTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 4.1. orders.update_status only -> 403
	t.Run("orders.update_status only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+adminUpdateTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 5. Invalid fulfillment ID -> 400
	t.Run("invalid fulfillment ID -> 400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/not-a-uuid/pack", nil)
		req.Header.Set("Authorization", "Bearer "+adminPackTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	// 6. Non-existent fulfillment ID -> 404
	t.Run("non-existent fulfillment ID -> 404", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+uuid.New().String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+adminPackTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "fulfillment_not_found", res.Error.Code)
	})

	// 7. Unpicked allocation -> 409 fulfillment_not_fully_picked
	t.Run("unpicked allocation -> 409 fulfillment_not_fully_picked", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+adminPackTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "fulfillment_not_fully_picked", res.Error.Code)
	})

	// 8. Pick allocation and pack successfully with orders.update_status -> 200 OK
	t.Run("fully picked with orders.update_status -> 200 OK", func(t *testing.T) {
		_, err := pgClient.Pool.Exec(ctx, `UPDATE order_item_allocations SET picked_at = now() WHERE id = $1`, allocID)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+adminPackTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res fulfillment.PackResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, fulfillmentID, res.FulfillmentID)
		assert.Equal(t, orderID, res.OrderID)
		assert.Equal(t, "packed", res.FulfillmentStatus)
		assert.Equal(t, "packed", res.OrderStatus)
	})

	// 9. Already packed -> 409 packing_not_allowed
	t.Run("already packed -> 409 packing_not_allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/pack", nil)
		req.Header.Set("Authorization", "Bearer "+adminPackTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "packing_not_allowed", res.Error.Code)
	})
}
