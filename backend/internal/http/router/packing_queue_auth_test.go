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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestPackingQueue_RouterAuth_CapabilityContract(t *testing.T) {
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
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, userID, "PackingQueueRole", perms)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	// 1. Staff with warehouse.packing only
	packingUserID := insertUser("admin")
	insertAdminWithPerms(packingUserID, []string{"warehouse.packing"})
	packingToken := makeToken(packingUserID, "admin")

	// 2. Staff with orders.read only
	ordersReadUserID := insertUser("admin")
	insertAdminWithPerms(ordersReadUserID, []string{"orders.read"})
	ordersReadToken := makeToken(ordersReadUserID, "admin")

	// 3. Staff with irrelevant role (returns.read only)
	irrelevantUserID := insertUser("admin")
	insertAdminWithPerms(irrelevantUserID, []string{"returns.read"})
	irrelevantToken := makeToken(irrelevantUserID, "admin")

	// Test A: warehouse.packing accesses GET /api/admin/fulfillments/packing -> 200 OK
	t.Run("warehouse.packing accesses packing queue -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/fulfillments/packing", nil)
		req.Header.Set("Authorization", "Bearer "+packingToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var items []interface{}
		err := json.NewDecoder(w.Body).Decode(&items)
		require.NoError(t, err)
	})

	// Test B: orders.read accesses GET /api/admin/fulfillments/packing -> 200 OK
	t.Run("orders.read accesses packing queue -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/fulfillments/packing", nil)
		req.Header.Set("Authorization", "Bearer "+ordersReadToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test C: Unauthenticated -> 401 Unauthorized
	t.Run("unauthenticated accesses packing queue -> 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/fulfillments/packing", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Test D: Staff with returns.read only -> 403 Forbidden
	t.Run("irrelevant permission accesses packing queue -> 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/fulfillments/packing", nil)
		req.Header.Set("Authorization", "Bearer "+irrelevantToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
