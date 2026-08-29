package router_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
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
}
