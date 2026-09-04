package router_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestReconciliationEndpoints_RoutingAndRBAC(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	defer pgClient.Close()

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

	adminNoPermID := uuid.New()
	adminWithAdjustPermID := uuid.New()
	roleNoPermID := uuid.New()
	roleAdjustID := uuid.New()

	pfx := "recon_rb_" + uuid.NewString()[:8]

	cleanup := func() {
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = ANY($1)", []uuid.UUID{adminNoPermID, adminWithAdjustPermID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = ANY($1)", []uuid.UUID{roleNoPermID, roleAdjustID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = ANY($1)", []uuid.UUID{roleNoPermID, roleAdjustID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{adminNoPermID, adminWithAdjustPermID})
	}
	cleanup()
	t.Cleanup(cleanup)

	// Users
	insertUser := func(id uuid.UUID, email, role string) {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
			VALUES ($1, 'User', $2, 'hash', $3, 'active', false, now(), now())
		`, id, email, role)
		require.NoError(t, err)
	}

	insertUser(adminNoPermID, pfx+"_noperm@zamk.local", "admin")
	insertUser(adminWithAdjustPermID, pfx+"_adjust@zamk.local", "admin")

	// Roles
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

	insertRoleWithPerm(roleNoPermID, pfx+"_role_noperm", adminNoPermID, "")
	insertRoleWithPerm(roleAdjustID, pfx+"_role_adjust", adminWithAdjustPermID, "inventory.adjust")

	tokenNoPerm, err := tokenService.GenerateAccessToken(adminNoPermID, pfx+"_noperm@zamk.local", "admin")
	require.NoError(t, err)
	tokenAdjust, err := tokenService.GenerateAccessToken(adminWithAdjustPermID, pfx+"_adjust@zamk.local", "admin")
	require.NoError(t, err)

	// 1. Unauthenticated -> 401
	{
		body, _ := json.Marshal(map[string]interface{}{"variantId": uuid.New()})
		req := httptest.NewRequest("POST", "/api/admin/inventory/reconciliations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}

	// 2. Authenticated but missing inventory.adjust -> 403
	{
		body, _ := json.Marshal(map[string]interface{}{"variantId": uuid.New()})
		req := httptest.NewRequest("POST", "/api/admin/inventory/reconciliations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenNoPerm))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}

	// 3. Authenticated with inventory.adjust but empty variantId -> 400
	{
		body, _ := json.Marshal(map[string]interface{}{"variantId": uuid.Nil})
		req := httptest.NewRequest("POST", "/api/admin/inventory/reconciliations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenAdjust))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}

	// 4. Authenticated with inventory.adjust but nonexistent variant -> 404
	{
		body, _ := json.Marshal(map[string]interface{}{"variantId": uuid.New()})
		req := httptest.NewRequest("POST", "/api/admin/inventory/reconciliations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenAdjust))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}
}
