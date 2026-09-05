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

func TestAdminDispatchRouter(t *testing.T) {
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
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'DispatchRole')`, roleID, code)
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

	// 1. Prepare data
	sellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Dispatch Brand', $2, $3, 'active', now(), now())
	`, sellerID, uuid.New().String(), uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	orderID := uuid.New()
	buyerID := insertUser("customer")
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'packed', 1000, 'N', 'P', 'E', 'A')
	`, orderID, buyerID)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'packed', 1000, 900, 900)
	`, fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	catID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 1000, 'published', now(), now())`, prodID, sellerID, catID, uuid.New().String())
	require.NoError(t, err)

	varID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())`, varID, prodID, uuid.New().String(), uuid.New().String(), uuid.New().String())
	require.NoError(t, err)

	// Create inventory_items with total_stock=10, reserved_stock=5
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 5, now(), now())
	`, uuid.New(), prodID, varID, sellerID)
	require.NoError(t, err)

	itemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Title', 'slug', 1000, 1, 1000, $6, 1)
	`, itemID, orderID, prodID, varID, sellerID, fulfillmentID)
	require.NoError(t, err)

	// Create users
	adminWithDispatchPerm := insertUser("admin")
	insertAdminWithPerms(adminWithDispatchPerm, []string{"warehouse.dispatch"})
	adminWithDispatchToken := makeToken(adminWithDispatchPerm, "admin")

	adminWithUpdatePerm := insertUser("admin")
	insertAdminWithPerms(adminWithUpdatePerm, []string{"orders.update_status"})
	adminWithUpdateToken := makeToken(adminWithUpdatePerm, "admin")

	adminWithReadOnly := insertUser("admin")
	insertAdminWithPerms(adminWithReadOnly, []string{"orders.read"})
	adminReadOnlyToken := makeToken(adminWithReadOnly, "admin")

	sellerUser := insertUser("seller")
	sellerToken := makeToken(sellerUser, "seller")

	customerUser := insertUser("customer")
	customerToken := makeToken(customerUser, "customer")

	// 1. Unauthenticated -> 401
	t.Run("unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	// 2. Customer -> 403
	t.Run("customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 3. Seller -> 403
	t.Run("seller -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+sellerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 4. Admin with only orders.read -> 403
	t.Run("admin with orders.read only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+adminReadOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 4.1 Admin with only orders.update_status -> 403 (does not automatically gain physical capabilities)
	t.Run("admin with orders.update_status only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithUpdateToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 5. Invalid UUID -> 400
	t.Run("invalid uuid -> 400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/not-a-uuid/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithDispatchToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	// 6. Non-existent fulfillment -> 404
	t.Run("non-existent fulfillment -> 404", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+uuid.New().String()+"/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithDispatchToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	// 7. Successful dispatch with warehouse.dispatch -> 200 OK
	t.Run("dispatch success with warehouse.dispatch -> 200", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithDispatchToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res fulfillment.DispatchResult
		err := json.NewDecoder(rr.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, fulfillmentID, res.FulfillmentID)
		assert.Equal(t, "shipped", res.FulfillmentStatus)
		assert.Equal(t, "shipped", res.OrderStatus)
	})

	// 8. Already shipped -> 409 dispatch_not_allowed
	t.Run("already shipped -> 409 dispatch_not_allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithDispatchToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(rr.Body).Decode(&res)
		assert.Equal(t, "dispatch_not_allowed", res.Error.Code)
	})
}
