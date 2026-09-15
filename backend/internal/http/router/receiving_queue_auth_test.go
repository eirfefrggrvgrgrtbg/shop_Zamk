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

func TestReceivingQueue_RouterAuth_CapabilityContract(t *testing.T) {
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
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, userID, "ReceivingQueueRole", perms)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	// 1. Staff with inventory.receipt
	receiptUserID := insertUser("admin")
	insertAdminWithPerms(receiptUserID, []string{"inventory.receipt"})
	receiptToken := makeToken(receiptUserID, "admin")

	// 2. Staff with inventory.read only (read-only inventory cannot perform supply receiving)
	readOnlyUserID := insertUser("admin")
	insertAdminWithPerms(readOnlyUserID, []string{"inventory.read"})
	readOnlyToken := makeToken(readOnlyUserID, "admin")

	// 3. Staff with orders.read only
	ordersReadUserID := insertUser("admin")
	insertAdminWithPerms(ordersReadUserID, []string{"orders.read"})
	ordersReadToken := makeToken(ordersReadUserID, "admin")

	// Test A: inventory.receipt accesses GET /api/admin/receiving/queue -> 200 OK
	t.Run("inventory.receipt accesses receiving queue -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/receiving/queue", nil)
		req.Header.Set("Authorization", "Bearer "+receiptToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var items []interface{}
		err := json.NewDecoder(w.Body).Decode(&items)
		require.NoError(t, err)
	})

	// Test B: Unauthenticated -> 401 Unauthorized
	t.Run("unauthenticated accesses receiving queue -> 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/receiving/queue", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Test C: Staff with inventory.read only -> 403 Forbidden
	t.Run("inventory.read only accesses receiving queue -> 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/receiving/queue", nil)
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	// Test D: Staff with orders.read only -> 403 Forbidden
	t.Run("orders.read only accesses receiving queue -> 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/receiving/queue", nil)
		req.Header.Set("Authorization", "Bearer "+ordersReadToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
