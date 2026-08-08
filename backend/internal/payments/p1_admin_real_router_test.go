package payments_test

import (
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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	goredis "github.com/redis/go-redis/v9"
)

type SetupData struct {
	Pool            *pgxpool.Pool
	Router          *chi.Mux
	TokenService    *auth.TokenService
	CustomerID      uuid.UUID
	AdminNoPermID   uuid.UUID
	AdminWithPermID uuid.UUID
	RoleNoPermID    uuid.UUID
	RoleWithPermID  uuid.UUID
	OrderID         uuid.UUID
	CleanOrderID    uuid.UUID
	PaymentID       uuid.UUID // Payment with problems
	PaymentCleanID  uuid.UUID // Clean payment
}

func SetupRealRouterAuthFixture(t *testing.T) *SetupData {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)

	var dbName string
	err = pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Contains(t, dbName, "_test", "Must use a _test database")

	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-secret"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.RateLimit.Enabled = false
	cfg.App.PaymentStuckPendingMinutes = 30

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
		AdminNoPermID:   uuid.New(),
		AdminWithPermID: uuid.New(),
		OrderID:         uuid.New(),
		CleanOrderID:    uuid.New(),
		PaymentID:       uuid.New(),
		PaymentCleanID:  uuid.New(),
	}

	ctx := context.Background()
	now := time.Now()

	// 1. Create Users
	usersToCreate := []struct {
		id   uuid.UUID
		role string
	}{
		{data.CustomerID, "customer"},
		{data.AdminNoPermID, "admin"},
		{data.AdminWithPermID, "admin"},
	}

	for _, u := range usersToCreate {
		_, err := pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at) VALUES ($1, 'Test', $2, 'hash', $3, $4, $4)", u.id, u.id.String()+"@test.com", u.role, now)
		require.NoError(t, err)
	}

	// 2. Create Staff Roles & Members
	data.RoleNoPermID = uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO staff_roles (id, code, name, description, is_system) VALUES ($1, $2, 'No Perm', '', false)", data.RoleNoPermID, "no_perm_"+uuid.New().String()[:8])
	require.NoError(t, err)

	data.RoleWithPermID = uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO staff_roles (id, code, name, description, is_system) VALUES ($1, $2, 'With Perm', '', false)", data.RoleWithPermID, "with_perm_"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, 'payments.read')", data.RoleWithPermID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES ($1, $2, 'active', $3, $3)", data.AdminNoPermID, data.RoleNoPermID, now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES ($1, $2, 'active', $3, $3)", data.AdminWithPermID, data.RoleWithPermID, now)
	require.NoError(t, err)

	// 3. Create Orders
	_, err = pool.Exec(ctx, "INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 100000, 'RUB', 'test', 'Delivery', 0, 'test', 'test', 'test', $4, $4)", data.OrderID, data.CustomerID, "ORD-"+data.OrderID.String()[:8], now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 100000, 'RUB', 'test', 'Delivery', 0, 'test', 'test', 'test', $4, $4)", data.CleanOrderID, data.CustomerID, "ORD-"+data.CleanOrderID.String()[:8], now)
	require.NoError(t, err)

	// 4. Create Payments
	// Payment 1: AMOUNT_MISMATCH
	_, err = pool.Exec(ctx, "INSERT INTO payments (id, order_id, amount_cents, currency, status, provider, payment_method, integration_mode, idempotency_key, provider_payment_id, created_at, updated_at) VALUES ($1, $2, 50000, 'RUB', 'succeeded', 'tbank', 'tpay', 'mock', $3, $4, $5, $5)", data.PaymentID, data.OrderID, uuid.New().String(), uuid.New().String(), now)
	require.NoError(t, err)

	// Payment 2: Clean
	_, err = pool.Exec(ctx, "INSERT INTO payments (id, order_id, amount_cents, currency, status, provider, payment_method, integration_mode, idempotency_key, provider_payment_id, created_at, updated_at) VALUES ($1, $2, 100000, 'RUB', 'succeeded', 'tbank', 'tpay', 'mock', $3, $4, $5, $5)", data.PaymentCleanID, data.CleanOrderID, uuid.New().String(), uuid.New().String(), now)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM payments WHERE id IN ($1, $2)", data.PaymentID, data.PaymentCleanID)
		pool.Exec(ctx, "DELETE FROM orders WHERE id IN ($1, $2)", data.OrderID, data.CleanOrderID)
		pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = $1 OR user_id = $2", data.AdminNoPermID, data.AdminWithPermID)
		pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = $1 OR role_id = $2", data.RoleNoPermID, data.RoleWithPermID)
		pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = $1 OR id = $2", data.RoleNoPermID, data.RoleWithPermID)
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2, $3)", data.CustomerID, data.AdminNoPermID, data.AdminWithPermID)
	})

	return data
}

func TestAdminPaymentsProblemFilters(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)

	GenerateToken := func(userID uuid.UUID, role string) string {
		token, err := data.TokenService.GenerateAccessToken(userID, "test@test.com", role)
		require.NoError(t, err)
		return token
	}

	makeRequest := func(token string, query string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("GET", "/api/admin/payments?"+query, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		return rr
	}

	t.Run("customer_forbidden", func(t *testing.T) {
		token := GenerateToken(data.CustomerID, "customer")
		rr := makeRequest(token, "hasProblem=true")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("admin_no_perm_forbidden", func(t *testing.T) {
		token := GenerateToken(data.AdminNoPermID, "admin")
		rr := makeRequest(token, "hasProblem=true")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("admin_has_problem_true", func(t *testing.T) {
		token := GenerateToken(data.AdminWithPermID, "admin")
		rr := makeRequest(token, "hasProblem=true")
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp payments.AdminPaymentListResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)

		foundProblem := false
		for _, p := range resp.Items {
			if p.PaymentID == data.PaymentID {
				foundProblem = true
				assert.NotEmpty(t, p.Problems)
			}
			if p.PaymentID == data.PaymentCleanID {
				t.Fatal("Clean payment should not be returned when hasProblem=true")
			}
		}
		assert.True(t, foundProblem, "Payment with problem should be returned")
	})

	t.Run("admin_problem_code_filter", func(t *testing.T) {
		token := GenerateToken(data.AdminWithPermID, "admin")
		rr := makeRequest(token, "problemCode=AMOUNT_MISMATCH")
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp payments.AdminPaymentListResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)

		foundProblemCode := false
		for _, p := range resp.Items {
			t.Logf("Returned payment: %s, Order: %s, Problems: %+v", p.PaymentID, p.OrderNumber, p.Problems)
			if p.PaymentID == data.PaymentID {
				foundProblemCode = true
				assert.Equal(t, []payments.PaymentProblem{{Code: "AMOUNT_MISMATCH", Severity: "warning"}}, p.Problems)
			}
			if p.PaymentID == data.PaymentCleanID {
				t.Errorf("Clean payment should not be returned when probCode=AMOUNT_MISMATCH")
			}
		}
		assert.True(t, foundProblemCode, "Payment with AMOUNT_MISMATCH should be returned")
	})
}
