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

func TestAdminMarketingAnalyticsRouter_RBAC(t *testing.T) {
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
			VALUES ($1, $2, $3, 'Test User', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		require.NoError(t, err)
		return id
	}

	insertAdminWithPerms := func(userID uuid.UUID, perms []string) {
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, userID, "MarketingRole", perms)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	adminWithRead := insertUser("admin")
	insertAdminWithPerms(adminWithRead, []string{"marketing.campaigns.read"})
	adminWithReadToken := makeToken(adminWithRead, "admin")

	adminNoPerm := insertUser("admin")
	insertAdminWithPerms(adminNoPerm, []string{"inventory.read"})
	adminNoPermToken := makeToken(adminNoPerm, "admin")

	customerUser := insertUser("customer")
	customerToken := makeToken(customerUser, "customer")

	fromStr := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	toStr := time.Now().UTC().Format(time.RFC3339)
	endpoint := "/api/admin/marketing/analytics/overview?from=" + fromStr + "&to=" + toStr

	// Matrix AD: Read permission allowed
	t.Run("MatrixAD_ReadPermission_Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+adminWithReadToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Admin with marketing.campaigns.read should be allowed")
	})

	// Matrix AE: No permission denied
	t.Run("MatrixAE_NoPermission_Denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+adminNoPermToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code, "Admin without marketing.campaigns.read must be 403 Forbidden")

		// Customer denied
		reqCustomer := httptest.NewRequest(http.MethodGet, endpoint, nil)
		reqCustomer.Header.Set("Authorization", "Bearer "+customerToken)
		wCustomer := httptest.NewRecorder()
		r.ServeHTTP(wCustomer, reqCustomer)
		assert.Equal(t, http.StatusForbidden, wCustomer.Code, "Customer must be 403 Forbidden")

		// Unauthenticated denied
		reqUnauth := httptest.NewRequest(http.MethodGet, endpoint, nil)
		wUnauth := httptest.NewRecorder()
		r.ServeHTTP(wUnauth, reqUnauth)
		assert.Equal(t, http.StatusUnauthorized, wUnauth.Code, "Unauthenticated request must be 401 Unauthorized")
	})
}
