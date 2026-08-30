package router_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestTimelineEndpoints_RBAC(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	defer pgClient.Close()

	// Strict DB Safety Guard
	testutil.AssertTestDatabase(t, pgClient.Pool)

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

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

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	// Fixture IDs
	customerID := uuid.New()
	sellerUserID := uuid.New()
	adminWithoutPermsID := uuid.New()
	adminWithOrdersPermID := uuid.New()
	adminWithReturnsPermID := uuid.New()

	roleNoPermsID := uuid.New()
	roleOrdersID := uuid.New()
	roleReturnsID := uuid.New()

	orderID := uuid.New()
	returnID := uuid.New()
	sellerID := uuid.New()

	cleanup := func() {
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", returnID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE order_id = $1", orderID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = ANY($1)", []uuid.UUID{adminWithoutPermsID, adminWithOrdersPermID, adminWithReturnsPermID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = ANY($1)", []uuid.UUID{roleNoPermsID, roleOrdersID, roleReturnsID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = ANY($1)", []uuid.UUID{roleNoPermsID, roleOrdersID, roleReturnsID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{customerID, sellerUserID, adminWithoutPermsID, adminWithOrdersPermID, adminWithReturnsPermID})
	}

	cleanup()
	t.Cleanup(func() {
		cleanup()
	})

	// 1. Insert Users
	insertUser := func(id uuid.UUID, email, role string) {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
			VALUES ($1, 'User', $2, 'hash', $3, 'active', false, now(), now())
		`, id, email, role)
		require.NoError(t, err)
	}

	custEmail := fmt.Sprintf("cust_tl_%s@zamk.local", customerID.String()[:8])
	sellerEmail := fmt.Sprintf("seller_tl_%s@zamk.local", sellerUserID.String()[:8])
	adminNoPermEmail := fmt.Sprintf("admin_noperm_%s@zamk.local", adminWithoutPermsID.String()[:8])
	adminOrdersEmail := fmt.Sprintf("admin_orders_%s@zamk.local", adminWithOrdersPermID.String()[:8])
	adminReturnsEmail := fmt.Sprintf("admin_returns_%s@zamk.local", adminWithReturnsPermID.String()[:8])

	insertUser(customerID, custEmail, "customer")
	insertUser(sellerUserID, sellerEmail, "seller")
	insertUser(adminWithoutPermsID, adminNoPermEmail, "admin")
	insertUser(adminWithOrdersPermID, adminOrdersEmail, "admin")
	insertUser(adminWithReturnsPermID, adminReturnsEmail, "admin")

	// 2. Roles & Permissions
	insertRoleWithPerm := func(roleID uuid.UUID, code string, userID uuid.UUID, perm string) {
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'Role')`, roleID, code)
		require.NoError(t, err)
		if perm != "" {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, perm)
			require.NoError(t, err)
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		require.NoError(t, err)
	}

	insertRoleWithPerm(roleNoPermsID, fmt.Sprintf("role_noperm_%s", roleNoPermsID.String()[:8]), adminWithoutPermsID, "")
	insertRoleWithPerm(roleOrdersID, fmt.Sprintf("role_orders_%s", roleOrdersID.String()[:8]), adminWithOrdersPermID, "orders.read")
	insertRoleWithPerm(roleReturnsID, fmt.Sprintf("role_returns_%s", roleReturnsID.String()[:8]), adminWithReturnsPermID, "returns.read")

	// 3. Create minimal target Order & Return
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'RBAC Seller', $2, 'desc', 'rbac@test.local', '123', 'active', now(), now())
	`, sellerID, fmt.Sprintf("rbac-seller-%s", sellerID.String()[:8]))
	require.NoError(t, err)

	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'paid', 1000, 'RUB', 'Cust', '123', 'c@b.a', 'Addr', now(), now())
	`, orderID, customerID, ordNumber)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'packed', 1000, 100, 900, now(), now())
	`, uuid.New(), orderID, sellerID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at)
		VALUES ($1, $2, (SELECT id FROM order_fulfillments WHERE order_id = $2 LIMIT 1), $3, 'requested', 'reason', 'comment', now(), now())
	`, returnID, orderID, customerID)
	require.NoError(t, err)

	generateToken := func(userID uuid.UUID, email, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, email, role)
		require.NoError(t, err)
		return tok
	}

	custToken := generateToken(customerID, custEmail, "customer")
	sellerToken := generateToken(sellerUserID, sellerEmail, "seller")
	adminNoPermToken := generateToken(adminWithoutPermsID, adminNoPermEmail, "admin")
	adminOrdersToken := generateToken(adminWithOrdersPermID, adminOrdersEmail, "admin")
	adminReturnsToken := generateToken(adminWithReturnsPermID, adminReturnsEmail, "admin")

	execReq := func(url, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	ordersURL := fmt.Sprintf("/api/admin/orders/%s/timeline", orderID.String())
	returnsURL := fmt.Sprintf("/api/admin/returns/%s/timeline", returnID.String())

	// --- Order Timeline RBAC ---
	t.Run("OrderTimeline_RBAC", func(t *testing.T) {
		// 1. Unauthenticated -> 401
		res := execReq(ordersURL, "")
		assert.Equal(t, http.StatusUnauthorized, res.Code)

		// 2. Customer -> 403
		res = execReq(ordersURL, custToken)
		assert.Equal(t, http.StatusForbidden, res.Code)

		// 3. Seller -> 403
		res = execReq(ordersURL, sellerToken)
		assert.Equal(t, http.StatusForbidden, res.Code)

		// 4. Admin without orders.read -> 403
		res = execReq(ordersURL, adminNoPermToken)
		assert.Equal(t, http.StatusForbidden, res.Code)

		// 5. Admin with orders.read -> 200
		res = execReq(ordersURL, adminOrdersToken)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Contains(t, res.Body.String(), ordNumber)
	})

	// --- Return Timeline RBAC ---
	t.Run("ReturnTimeline_RBAC", func(t *testing.T) {
		// 1. Unauthenticated -> 401
		res := execReq(returnsURL, "")
		assert.Equal(t, http.StatusUnauthorized, res.Code)

		// 2. Customer -> 403
		res = execReq(returnsURL, custToken)
		assert.Equal(t, http.StatusForbidden, res.Code)

		// 3. Seller -> 403
		res = execReq(returnsURL, sellerToken)
		assert.Equal(t, http.StatusForbidden, res.Code)

		// 4. Admin without returns.read -> 403
		res = execReq(returnsURL, adminNoPermToken)
		assert.Equal(t, http.StatusForbidden, res.Code)

		// 5. Admin with returns.read -> 200
		res = execReq(returnsURL, adminReturnsToken)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Contains(t, res.Body.String(), ordNumber)
	})
}
