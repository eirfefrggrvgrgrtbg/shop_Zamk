package testlab_test

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

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func setupIntegration(t *testing.T) (*pgxpool.Pool, *postgres.Client, *redis.Client, *config.Config, http.Handler) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "secret",
			RefreshTokenSecret:    "secret2",
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{
			CookieDomain:   "localhost",
			CookieSecure:   false,
			CookieSameSite: "Lax",
		},
		App: config.AppConfig{
			Env: "test",
		},
	}

	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		pgClient.Close()
	})

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 1)
	if err != nil {
		t.Logf("redis connect failed, skipping test: %v", err)
		t.Skip("redis not available")
	}
	t.Cleanup(func() {
		redisClient.Close()
	})

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	router, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	t.Cleanup(func() {
		cancel()
	})

	return pgClient.Pool, pgClient, redisClient, cfg, router
}

func createAdminToken(t *testing.T, db *pgxpool.Pool, cfg *config.Config) (string, uuid.UUID) {
	adminID := uuid.New()
	_, err := db.Exec(context.Background(), "INSERT INTO users (id, name, email, password_hash, role, status) VALUES ($1, 'Test Lab Bootstrap Admin', $2, 'hash', 'admin', 'active')", adminID, adminID.String()+"@testlabbootstrap.zamk.ru")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM users WHERE id = $1", adminID)
	})

	roleID := uuid.New()
	_, err = db.Exec(context.Background(), "INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'TestLabBootstrapRole')", roleID, roleID.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM staff_roles WHERE id = $1", roleID)
	})

	_, err = db.Exec(context.Background(), "INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, 'testing.manage')", roleID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM staff_role_permissions WHERE role_id = $1", roleID)
	})

	_, err = db.Exec(context.Background(), "INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')", adminID, roleID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM staff_members WHERE user_id = $1 AND staff_role_id = $2", adminID, roleID)
	})

	ts := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, 15)
	accessToken, err := ts.GenerateAccessToken(adminID, adminID.String()+"@testlabbootstrap.zamk.ru", "admin")
	require.NoError(t, err)
	return accessToken, adminID
}

func TestProductBootstrap_BasicSales(t *testing.T) {
	pool, _, _, cfg, router := setupIntegration(t)

	token, _ := createAdminToken(t, pool, cfg)

	// Call test lab
	reqBody := `{"preset":"BASIC_SALES"}`
	req := httptest.NewRequest("POST", "/api/admin/testing/analytics/scenarios/apply", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Expected OK response, got body: %s", rr.Body.String())

	var result struct {
		RunId    string    `json:"runId"`
		SellerId uuid.UUID `json:"sellerId"`
		Expected struct {
			GrossSales struct {
				CurrentCents int `json:"currentCents"`
			} `json:"grossSales"`
			Orders struct {
				Current int `json:"current"`
			} `json:"orders"`
			UnitsSold struct {
				Current int `json:"current"`
			} `json:"unitsSold"`
			Commission struct {
				CurrentCents int `json:"currentCents"`
			} `json:"commission"`
			SellerEarningBeforeReturns struct {
				CurrentCents int `json:"currentCents"`
			} `json:"sellerEarningBeforeReturns"`
			ReturnDeductions struct {
				CurrentCents int `json:"currentCents"`
			} `json:"returnDeductions"`
			OtherAdjustments struct {
				CurrentCents int `json:"currentCents"`
			} `json:"otherAdjustments"`
			NetCommercialEarning struct {
				CurrentCents int `json:"currentCents"`
			} `json:"netCommercialEarning"`
		} `json:"expectedResult"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)

	// Expected Analytics Values
	require.Equal(t, 150000, result.Expected.GrossSales.CurrentCents)
	require.Equal(t, 1, result.Expected.Orders.Current)
	require.Equal(t, 1, result.Expected.UnitsSold.Current)
	
	// Ensure these match EXPECTED ANALYTICS VALUES
	require.Equal(t, 12000, result.Expected.Commission.CurrentCents)
	require.Equal(t, 138000, result.Expected.SellerEarningBeforeReturns.CurrentCents)
	require.Equal(t, 0, result.Expected.ReturnDeductions.CurrentCents)
	require.Equal(t, 0, result.Expected.OtherAdjustments.CurrentCents)
	require.Equal(t, 138000, result.Expected.NetCommercialEarning.CurrentCents)

	// DB Assertions for Product
	ctx := context.Background()
	var productID, pStatus, sID string
	err = pool.QueryRow(ctx, "SELECT id, status, seller_id FROM products WHERE title LIKE 'TestLab Canonical Product%' ORDER BY created_at DESC LIMIT 1").Scan(&productID, &pStatus, &sID)
	require.NoError(t, err)

	require.Equal(t, "published", pStatus, "Product must be published")
	require.Equal(t, result.SellerId.String(), sID, "Product must belong to the isolated seller")

	// DB Assertions for Variant
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM product_variants WHERE product_id = $1", productID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "Exactly 1 variant must exist")

	var vID string
	err = pool.QueryRow(ctx, "SELECT id FROM product_variants WHERE product_id = $1", productID).Scan(&vID)
	require.NoError(t, err)

	// Insert unrelated product for cleanup check
	unrelatedSellerID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status) VALUES ($1, 'Unrelated Brand', 'active')", unrelatedSellerID)
	require.NoError(t, err)
	unrelatedProductID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, status, slug, price_cents) VALUES ($1, $2, 'Unrelated', 'published', $3, 100)", unrelatedProductID, unrelatedSellerID, uuid.New().String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, 'unrel-sku', 100)", uuid.New(), unrelatedProductID)
	require.NoError(t, err)

	// Clean up
	cleanupReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/testing/analytics/scenarios/%s", result.RunId), nil)
	cleanupReq.Header.Set("Content-Type", "application/json")
	cleanupReq.Header.Set("Authorization", "Bearer "+token)
	
	cleanupRr := httptest.NewRecorder()
	router.ServeHTTP(cleanupRr, cleanupReq)
	require.Equal(t, http.StatusNoContent, cleanupRr.Code, "Cleanup failed: %s", cleanupRr.Body.String())

	// Check if product deleted
	err = pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", productID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "Product must be deleted after cleanup")

	// Check unrelated product remains
	err = pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", unrelatedProductID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "Unrelated product must remain after cleanup")
}
