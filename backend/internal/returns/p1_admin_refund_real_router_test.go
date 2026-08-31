package returns_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	goredis "github.com/redis/go-redis/v9"
)

type SetupData struct {
	Pool            *pgxpool.Pool
	Router          *chi.Mux
	TokenService    *auth.TokenService
	CustomerID      uuid.UUID
	SellerID        uuid.UUID
	AdminNoPermID   uuid.UUID
	AdminWithPermID uuid.UUID
	OrderID         uuid.UUID
	PaymentID       uuid.UUID
	ReturnID        uuid.UUID
	FulfillmentID   uuid.UUID
	OrderItemID     uuid.UUID
	ReturnItemID    uuid.UUID
	ProductID       uuid.UUID
	VariantID       uuid.UUID
	SellerProfileID uuid.UUID
	RoleNoPermID    uuid.UUID
	RoleWithPermID  uuid.UUID
}

func SetupRealRouterAuthFixture(t *testing.T) *SetupData {
	dbURL := testutil.GetTestDatabaseURL()

	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, pool)

	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-secret"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.RateLimit.Enabled = false
	cfg.Worker.ReturnWindowDays = 14

	pgClient := &postgres.Client{Pool: pool}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	dummyRedis := &redis.Client{Client: goredis.NewClient(&goredis.Options{})}

	router, cancel := app.BuildRouter(context.Background(), cfg, pgClient, dummyRedis, logger)
	t.Cleanup(cancel)
	t.Cleanup(func() { pool.Close() })

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)

	data := &SetupData{
		Pool:            pool,
		Router:          router,
		TokenService:    tokenService,
		CustomerID:      uuid.New(),
		SellerID:        uuid.New(),
		AdminNoPermID:   uuid.New(),
		AdminWithPermID: uuid.New(),
		OrderID:         uuid.New(),
		PaymentID:       uuid.New(),
		ReturnID:        uuid.New(),
		FulfillmentID:   uuid.New(),
		OrderItemID:     uuid.New(),
		ReturnItemID:    uuid.New(),
		ProductID:       uuid.New(),
		VariantID:       uuid.New(),
		SellerProfileID: uuid.New(),
	}

	ctx := context.Background()
	now := time.Now()

	// 1. Create Users
	usersToCreate := []struct {
		id   uuid.UUID
		role string
	}{
		{data.CustomerID, "customer"},
		{data.SellerID, "seller"},
		{data.AdminNoPermID, "admin"},
		{data.AdminWithPermID, "admin"},
	}

	for _, u := range usersToCreate {
		_, err := pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at) VALUES ($1, 'Test', $2, 'hash', $3, $4, $4)", u.id, u.id.String()+"@test.com", u.role, now)
		require.NoError(t, err)
	}

	// 2. Create Seller profile
	_, err = pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Test Seller', $2, 'test@test.com', 'active', $3, $3)", data.SellerProfileID, "slug-"+data.SellerProfileID.String(), now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role, created_at) VALUES ($1, $2, $3, 'owner', $4)", uuid.New(), data.SellerProfileID, data.SellerID, now)
	require.NoError(t, err)

	// 3. Create Staff Roles & Members
	data.RoleNoPermID = uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO staff_roles (id, code, name, description, is_system) VALUES ($1, $2, 'No Perm', '', false)", data.RoleNoPermID, "no_perm_"+uuid.New().String()[:8])
	require.NoError(t, err)

	data.RoleWithPermID = uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO staff_roles (id, code, name, description, is_system) VALUES ($1, $2, 'With Perm', '', false)", data.RoleWithPermID, "with_perm_"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, 'refunds.create')", data.RoleWithPermID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES ($1, $2, 'active', $3, $3)", data.AdminNoPermID, data.RoleNoPermID, now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES ($1, $2, 'active', $3, $3)", data.AdminWithPermID, data.RoleWithPermID, now)
	require.NoError(t, err)

	// 4. Create Order & Payment & Return domain
	_, err = pool.Exec(ctx, "INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at) VALUES ($1, $2, $3, 'awaiting_payment', 100000, 'RUB', 'test', 'Delivery', 0, 'test', 'test', 'test', $4, $4)", data.OrderID, data.CustomerID, "ORD-"+data.OrderID.String()[:8], now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO payments (id, order_id, amount_cents, currency, status, provider, idempotency_key, created_at, updated_at) VALUES ($1, $2, 100000, 'RUB', 'succeeded', 'tbank', $3, $4, $4)", data.PaymentID, data.OrderID, uuid.New().String(), now)
	require.NoError(t, err)

	// Product & Variant
	_, err = pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, status, price_cents, created_at, updated_at, slug, description, currency) VALUES ($1, $2, 'Test Product', 'published', 100000, $3, $3, $4, 'desc', 'RUB')", data.ProductID, data.SellerProfileID, now, "prod-"+data.ProductID.String()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'SKU', 100000, $3, $3)", data.VariantID, data.ProductID, now)
	require.NoError(t, err)

	// Fulfillment & Order Item
	_, err = pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'delivered', $4, $4)", data.FulfillmentID, data.OrderID, data.SellerProfileID, now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO order_items (id, order_id, product_variant_id, quantity, price_cents, subtotal_price_cents, seller_id, product_id, order_fulfillment_id, title, product_slug, created_at) VALUES ($1, $2, $3, 1, 100000, 100000, $4, $5, $6, 'Test Product', 'test-product', $7)", data.OrderItemID, data.OrderID, data.VariantID, data.SellerProfileID, data.ProductID, data.FulfillmentID, now)
	require.NoError(t, err)

	// Return
	var fulfillmentID uuid.UUID
	_ = pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1 LIMIT 1", data.OrderID).Scan(&fulfillmentID)
	if fulfillmentID == uuid.Nil {
		fulfillmentID = uuid.New()
		_, _ = pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'delivered')", fulfillmentID, data.OrderID, data.SellerID)
	}

	_, err = pool.Exec(ctx, "INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, $4, 'item_received', 'defective', $5, $5)", data.ReturnID, data.OrderID, fulfillmentID, data.CustomerID, now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO return_items (id, return_id, order_item_id, quantity, condition, accepted_quantity, created_at) VALUES ($1, $2, $3, 1, 'new', 1, $4)", data.ReturnItemID, data.ReturnID, data.OrderItemID, now)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		// Cleanup refunds
		pool.Exec(ctx, "DELETE FROM refunds WHERE return_id = $1", data.ReturnID)

		// Cleanup returns
		pool.Exec(ctx, "DELETE FROM return_items WHERE return_id = $1", data.ReturnID)
		pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", data.ReturnID)

		// Cleanup payments & events
		pool.Exec(ctx, "DELETE FROM payment_events WHERE payment_id = $1", data.PaymentID)
		pool.Exec(ctx, "DELETE FROM payments WHERE id = $1", data.PaymentID)

		// Cleanup order items & fulfillments & orders
		pool.Exec(ctx, "DELETE FROM order_items WHERE id = $1", data.OrderItemID)
		pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", data.FulfillmentID)
		pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", data.OrderID)

		// Cleanup products
		pool.Exec(ctx, "DELETE FROM product_variants WHERE id = $1", data.VariantID)
		pool.Exec(ctx, "DELETE FROM products WHERE id = $1", data.ProductID)

		// Cleanup sellers & staff
		pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", data.SellerProfileID)
		pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = $1 OR user_id = $2", data.AdminNoPermID, data.AdminWithPermID)
		pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = $1 OR role_id = $2", data.RoleNoPermID, data.RoleWithPermID)
		pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = $1 OR id = $2", data.RoleNoPermID, data.RoleWithPermID)

		// Cleanup users
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2, $3, $4)", data.CustomerID, data.SellerID, data.AdminNoPermID, data.AdminWithPermID)
	})

	return data
}

func TestAdminRefundReserve_RealRouterAuth(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)

	// Helpers
	generateToken := func(userID uuid.UUID, role string) string {
		token, err := data.TokenService.GenerateAccessToken(userID, "test@test.com", role)
		require.NoError(t, err)
		return token
	}

	makeRequest := func(token string, amountCents int64) *httptest.ResponseRecorder {
		body := map[string]interface{}{
			"reason": "defective",
		}
		if amountCents > 0 {
			body["amount_cents"] = amountCents
		}

		bodyBytes, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/returns/"+data.ReturnID.String()+"/refund", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		rr := httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		return rr
	}

	verifyNoRefundCreated := func(t *testing.T) {
		var count int
		err := data.Pool.QueryRow(context.Background(), "SELECT count(*) FROM refunds WHERE return_id = $1", data.ReturnID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	}

	t.Run("no_token", func(t *testing.T) {
		rr := makeRequest("", 100000)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		verifyNoRefundCreated(t)
	})

	t.Run("invalid_token", func(t *testing.T) {
		rr := makeRequest("invalid.token.here", 100000)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		verifyNoRefundCreated(t)
	})

	t.Run("customer_forbidden", func(t *testing.T) {
		token := generateToken(data.CustomerID, "customer")
		rr := makeRequest(token, 100000)
		assert.Equal(t, http.StatusForbidden, rr.Code)
		verifyNoRefundCreated(t)
	})

	t.Run("seller_forbidden", func(t *testing.T) {
		token := generateToken(data.SellerID, "seller")
		rr := makeRequest(token, 100000)
		assert.Equal(t, http.StatusForbidden, rr.Code)
		verifyNoRefundCreated(t)
	})

	t.Run("admin_without_permission_forbidden", func(t *testing.T) {
		token := generateToken(data.AdminNoPermID, "admin")
		rr := makeRequest(token, 100000)
		assert.Equal(t, http.StatusForbidden, rr.Code)
		verifyNoRefundCreated(t)
	})

	t.Run("admin_success", func(t *testing.T) {
		defer data.Pool.Exec(context.Background(), "DELETE FROM refunds")
		token := generateToken(data.AdminWithPermID, "admin")

		bodyBytes, _ := json.Marshal(map[string]interface{}{
			"reason":       "defective",
			"amount_cents": 100000,
		})
		req, _ := http.NewRequest("POST", "/api/admin/returns/"+data.ReturnID.String()+"/refund", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)

		require.Contains(t, []int{http.StatusOK, http.StatusCreated}, rr.Code)

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.Equal(t, "pending", resp["status"])

		// Assert DB row
		var dbStatus, dbCurrency string
		var dbProviderRefundID *string
		var dbAmountCents int64
		var dbReturnID, dbPaymentID, dbOrderID, dbRefundID uuid.UUID

		err := data.Pool.QueryRow(context.Background(), `
			SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider_refund_id
			FROM refunds
			WHERE return_id = $1
		`, data.ReturnID).Scan(&dbRefundID, &dbReturnID, &dbPaymentID, &dbOrderID, &dbStatus, &dbAmountCents, &dbCurrency, &dbProviderRefundID)

		require.NoError(t, err)
		assert.Equal(t, data.ReturnID, dbReturnID)
		assert.Equal(t, data.PaymentID, dbPaymentID)
		assert.Equal(t, data.OrderID, dbOrderID)
		assert.Equal(t, "pending", dbStatus)
		assert.Equal(t, int64(100000), dbAmountCents)
		assert.Equal(t, "RUB", dbCurrency)
		assert.Nil(t, dbProviderRefundID)

		// Verify that Payment and Order did not change status
		var pStatus, oStatus string
		err = data.Pool.QueryRow(context.Background(), "SELECT status FROM payments WHERE id = $1", data.PaymentID).Scan(&pStatus)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", pStatus)

		err = data.Pool.QueryRow(context.Background(), "SELECT status FROM orders WHERE id = $1", data.OrderID).Scan(&oStatus)
		require.NoError(t, err)
		assert.Equal(t, "awaiting_payment", oStatus)
	})
}
