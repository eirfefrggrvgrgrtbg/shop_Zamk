package router_test

import (
	"context"
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

func TestCustomerOrderCancelAuthorization(t *testing.T) {
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
		Auth:   config.AuthConfig{},
		App:    config.AppConfig{Env: "test"},
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
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
			VALUES ($1, $2, $3, 'Test User', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		require.NoError(t, err)
		return id
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	ownerCustomerID := insertUser("customer")
	otherCustomerID := insertUser("customer")
	sellerUserID := insertUser("seller")
	adminUserID := insertUser("admin")

	ownerToken := makeToken(ownerCustomerID, "customer")
	otherCustomerToken := makeToken(otherCustomerID, "customer")
	sellerToken := makeToken(sellerUserID, "seller")
	adminToken := makeToken(adminUserID, "admin")

	// Setup valid order in awaiting_payment for ownerCustomer
	orderID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'awaiting_payment', 1300000, 'RUB', 'Customer Owner', '+79991234567', 'cust@test.local', 'Test Address', NOW(), NOW())
	`, orderID, ownerCustomerID)
	require.NoError(t, err)

	cancelURL := "/api/customer/orders/" + orderID.String() + "/cancel"

	// 1. Unauthenticated request -> 401 Unauthorized
	t.Run("unauthenticated request is rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, cancelURL, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// 2. Seller JWT -> 403 Forbidden
	t.Run("seller JWT is rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, cancelURL, nil)
		req.Header.Set("Authorization", "Bearer "+sellerToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// 3. Admin JWT -> 403 Forbidden
	t.Run("admin JWT is rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, cancelURL, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// 4. Another customer JWT -> 404 Not Found (not owner)
	t.Run("non-owner customer JWT is rejected with 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, cancelURL, nil)
		req.Header.Set("Authorization", "Bearer "+otherCustomerToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	// 5. Owning customer JWT -> 204 No Content and successfully cancels order
	t.Run("owning customer JWT is allowed and cancels order", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, cancelURL, nil)
		req.Header.Set("Authorization", "Bearer "+ownerToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		var status string
		var cancelledAt *time.Time
		err := pgClient.Pool.QueryRow(ctx, `SELECT status, cancelled_at FROM orders WHERE id = $1`, orderID).Scan(&status, &cancelledAt)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", status)
		assert.NotNil(t, cancelledAt)
	})
}
