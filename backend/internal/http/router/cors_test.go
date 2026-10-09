package router_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestCORSPreflightAllowsIdempotencyKey(t *testing.T) {
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
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://127.0.0.1:3000", "http://localhost:3000"},
		},
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

	req := httptest.NewRequest(http.MethodOptions, "/api/customer/orders", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type, idempotency-key")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://127.0.0.1:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))

	allowHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	require.NotEmpty(t, allowHeaders)
	assert.True(t, strings.Contains(allowHeaders, "Idempotency-Key"), "Access-Control-Allow-Headers should contain Idempotency-Key, got: %s", allowHeaders)
	assert.True(t, strings.Contains(allowHeaders, "X-Zamk-App"), "Access-Control-Allow-Headers should contain X-Zamk-App, got: %s", allowHeaders)
	assert.Equal(t, "Content-Disposition", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORSExposesContentDispositionOnExport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	adminOrigin := "http://localhost:3002"
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{adminOrigin, "http://127.0.0.1:3002"},
		},
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

	exportEndpoint := "/api/admin/marketing/analytics/export"

	// 1. Preflight OPTIONS on Export endpoint
	t.Run("Preflight_OPTIONS_Exposes_Headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, exportEndpoint, nil)
		req.Header.Set("Origin", adminOrigin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "content-type, authorization")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, adminOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "Content-Disposition", rec.Header().Get("Access-Control-Expose-Headers"))
	})

	// Setup authorized user
	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)
	adminID := uuid.New()
	phone := "7999" + adminID.String()[:7]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Admin User', 'hash', 'admin', 'active', NOW(), NOW())
	`, adminID, adminID.String()+"@test.com", phone)
	require.NoError(t, err)

	_, err = setupRouterStaffWithPermissions(ctx, pgClient.Pool, adminID, "MarketingRole", []string{"marketing.campaigns.read"})
	require.NoError(t, err)

	adminToken, err := tokenService.GenerateAccessToken(adminID, adminID.String()+"@test.com", "admin")
	require.NoError(t, err)

	fromStr := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	toStr := time.Now().UTC().Format(time.RFC3339)

	// 2. Actual CSV Export with Origin header
	t.Run("CSV_Export_Has_Expose_Headers_And_Content_Disposition", func(t *testing.T) {
		bodyJSON := `{"format": "csv", "query": {"version": 1, "period": {"from": "` + fromStr + `", "to": "` + toStr + `"}, "dimensions": ["source"], "metrics": ["orders"]}}`
		req := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		req.Header.Set("Origin", adminOrigin)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, adminOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "Content-Disposition", rec.Header().Get("Access-Control-Expose-Headers"))
		assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))

		contentDisp := rec.Header().Get("Content-Disposition")
		assert.NotEmpty(t, contentDisp)
		assert.True(t, strings.HasPrefix(contentDisp, `attachment; filename="zamk-marketing-report-`), "Expected Content-Disposition to start with attachment; filename=\"zamk-marketing-report-, got: %s", contentDisp)
		assert.True(t, strings.HasSuffix(contentDisp, `.csv"`), "Expected Content-Disposition to end with .csv\", got: %s", contentDisp)
	})

	// 3. Actual XLSX Export with Origin header
	t.Run("XLSX_Export_Has_Expose_Headers_And_Content_Disposition", func(t *testing.T) {
		bodyJSON := `{"format": "xlsx", "query": {"version": 1, "period": {"from": "` + fromStr + `", "to": "` + toStr + `"}, "dimensions": ["source"], "metrics": ["orders"]}}`
		req := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		req.Header.Set("Origin", adminOrigin)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, adminOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "Content-Disposition", rec.Header().Get("Access-Control-Expose-Headers"))
		assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-Type"))

		contentDisp := rec.Header().Get("Content-Disposition")
		assert.NotEmpty(t, contentDisp)
		assert.True(t, strings.HasPrefix(contentDisp, `attachment; filename="zamk-marketing-report-`), "Expected Content-Disposition to start with attachment; filename=\"zamk-marketing-report-, got: %s", contentDisp)
		assert.True(t, strings.HasSuffix(contentDisp, `.xlsx"`), "Expected Content-Disposition to end with .xlsx\", got: %s", contentDisp)
	})

	// 4. Disallowed Origin does not receive CORS headers
	t.Run("Disallowed_Origin_Does_Not_Receive_CORS_Headers", func(t *testing.T) {
		bodyJSON := `{"format": "csv", "query": {"version": 1, "period": {"from": "` + fromStr + `", "to": "` + toStr + `"}, "dimensions": ["source"], "metrics": ["orders"]}}`
		req := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		req.Header.Set("Origin", "http://malicious.evil.com")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, rec.Header().Get("Access-Control-Expose-Headers"))
	})
}
