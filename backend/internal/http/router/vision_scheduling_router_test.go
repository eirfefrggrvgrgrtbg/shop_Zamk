package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

func setupVisionRouterTestEnv(t *testing.T, visionEnabled bool) (context.Context, http.Handler, func(), *postgres.Client, *auth.TokenService) {
	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://127.0.0.1:3000"},
		},
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
		Vision: config.VisionConfig{
			Enabled: visionEnabled,
		},
	}

	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)

	var dbName string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "tests must strictly run against zamk_test")

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)

	cleanup := func() {
		cancel()
		redisClient.Close()
		pgClient.Close()
	}

	return ctx, r, cleanup, pgClient, tokenService
}

func createRouterVisionTestFixture(t *testing.T, ctx context.Context, pgClient *postgres.Client, tokenService *auth.TokenService) (uuid.UUID, string, uuid.UUID) {
	t.Helper()

	sellerID := uuid.New()
	sellerSlug := "v-seller-" + sellerID.String()[:8]
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Vision Brand', $2, $3, 'active', now(), now())
	`, sellerID, sellerSlug, sellerSlug+"@test.com")
	require.NoError(t, err)

	sellerUserID := uuid.New()
	sellerUserEmail := "seller-v-" + sellerUserID.String()[:8] + "@test.com"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Seller User', 'hash', 'seller', 'active', now(), now())
	`, sellerUserID, sellerUserEmail, "+7991"+sellerUserID.String()[:7])
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, sellerUserID)
	require.NoError(t, err)

	sellerToken, err := tokenService.GenerateAccessToken(sellerUserID, sellerUserEmail, "seller")
	require.NoError(t, err)

	catID := uuid.New()
	catSlug := "v-cat-" + catID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Vision Category', $2, true, now(), now())
	`, catID, catSlug)
	require.NoError(t, err)

	prodID := uuid.New()
	prodSlug := "v-prod-" + prodID.String()[:8]
	now := time.Now()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, description, price_cents, currency, status,
			vision_content_version, created_at, updated_at
		) VALUES ($1, $2, $3, 'Vision Test Product', $4, 'Desc', 500000, 'RUB', 'draft', 0, $5, $5)
	`, prodID, sellerID, catID, prodSlug, now)
	require.NoError(t, err)

	// Add 3 images with crops
	for i := 1; i <= 3; i++ {
		imgID := uuid.New()
		objKey := fmt.Sprintf("products/%s/%s/%s.jpg", sellerID, prodID, imgID)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO product_images (id, product_id, image_url, object_key, rendition_url, rendition_object_key, crop_x, crop_y, crop_width, crop_height, sort_order, is_main, created_at)
			VALUES ($1, $2, $3, $4, $3, $4, 0.0, 0.0, 1.0, 1.0, $5, $6, now())
		`, imgID, prodID, "http://test/"+objKey, objKey, i, i == 1)
		require.NoError(t, err)
	}

	// Add 1 active variant with SKU
	varID := uuid.New()
	sku := "SKU-V-" + varID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, 500000, true, $5, $5)
	`, varID, prodID, sku, "BC-"+varID.String()[:8], now)
	require.NoError(t, err)

	return sellerID, sellerToken, prodID
}

func TestVisionScheduling_Router_VisionEnabledTrue(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupVisionRouterTestEnv(t, true)
	defer cleanup()

	_, sellerToken, prodID := createRouterVisionTestFixture(t, ctx, pgClient, tokenService)

	// Send POST /api/seller/products/{id}/submit-moderation
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/submit-moderation", prodID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+sellerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "Response must be 200 OK: %s", rec.Body.String())

	// Verify product status is pending_moderation
	var status string
	err := pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", prodID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "pending_moderation", status)

	// Verify product_vision_runs row is created in status pending
	var runID uuid.UUID
	var runStatus string
	var attempts int
	err = pgClient.Pool.QueryRow(ctx, "SELECT id, status, attempts FROM product_vision_runs WHERE product_id = $1", prodID).Scan(&runID, &runStatus, &attempts)
	require.NoError(t, err, "Vision run must be created when VISION_ENABLED=true")
	assert.Equal(t, vision.RunStatusPending, runStatus)
	assert.Equal(t, 0, attempts)
}

func TestVisionScheduling_Router_VisionEnabledFalse(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupVisionRouterTestEnv(t, false)
	defer cleanup()

	_, sellerToken, prodID := createRouterVisionTestFixture(t, ctx, pgClient, tokenService)

	// Send POST /api/seller/products/{id}/submit-moderation
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/seller/products/%s/submit-moderation", prodID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+sellerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "Response must be 200 OK: %s", rec.Body.String())

	// Verify product status is pending_moderation
	var status string
	err := pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", prodID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "pending_moderation", status)

	// Verify ZERO product_vision_runs created
	var count int
	err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", prodID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Zero vision runs must be scheduled when VISION_ENABLED=false")
}
