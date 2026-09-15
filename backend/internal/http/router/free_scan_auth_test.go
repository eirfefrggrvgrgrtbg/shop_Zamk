package router_test

import (
	"bytes"
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

func TestFreeScan_CapabilitySplit_RouterAuth(t *testing.T) {
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
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, userID, "FreeScanRole", perms)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	// Setup seller, category, product, variant
	sellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'FS Brand', $2, $3, 'active', NOW(), NOW())
	`, sellerID, "seller-fs-"+sellerID.String()[:8], sellerID.String()+"@seller.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod FS', $4, 1000, 'published', now(), now())`, prodID, sellerID, catID, uuid.New().String())
	require.NoError(t, err)

	variantID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, 1000, true, now(), now())`, variantID, prodID, "SKU-FS-"+variantID.String()[:8], "BAR-FS-"+variantID.String()[:8])
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed_with_discrepancies', $3, 'pickup', now(), now())`, supplyID, sellerID, "SUP-FS-"+uuid.New().String()[:6])
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, missing_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, 0, 1, now(), now())`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitID := uuid.New()
	unitCode := "ZMU-FS-" + uuid.New().String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'expected')`, unitID, unitCode, variantID, supplyID, supplyItemID)
	require.NoError(t, err)

	// Admin tokens with different capabilities
	adminReadID := insertUser("admin")
	insertAdminWithPerms(adminReadID, []string{"inventory.read"})
	readOnlyTok := makeToken(adminReadID, "admin")

	adminReceiptID := insertUser("admin")
	insertAdminWithPerms(adminReceiptID, []string{"inventory.receipt"})
	receiptTok := makeToken(adminReceiptID, "admin")

	adminUnrelatedID := insertUser("admin")
	insertAdminWithPerms(adminUnrelatedID, []string{"orders.read"})
	unrelatedTok := makeToken(adminUnrelatedID, "admin")

	t.Run("GET /free-scan with inventory.read only -> allowed (200)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/receiving/free-scan?unitCode="+unitCode, nil)
		req.Header.Set("Authorization", "Bearer "+readOnlyTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, unitCode, resp["unitCode"])
		assert.Equal(t, "expected", resp["unitStatus"])
	})

	t.Run("GET /free-scan with inventory.receipt -> allowed (200)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/receiving/free-scan?unitCode="+unitCode, nil)
		req.Header.Set("Authorization", "Bearer "+receiptTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("GET /free-scan with unrelated capability orders.read -> forbidden (403)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/receiving/free-scan?unitCode="+unitCode, nil)
		req.Header.Set("Authorization", "Bearer "+unrelatedTok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST /free-scan/process with inventory.read only -> forbidden (403)", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"unitCode":  unitCode,
			"condition": "ok",
		})
		req := httptest.NewRequest("POST", "/api/admin/receiving/free-scan/process", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+readOnlyTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST /free-scan/process with unrelated orders.read -> forbidden (403)", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"unitCode":  unitCode,
			"condition": "ok",
		})
		req := httptest.NewRequest("POST", "/api/admin/receiving/free-scan/process", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+unrelatedTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST /free-scan/process with inventory.receipt -> allowed (200)", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"unitCode":  unitCode,
			"condition": "ok",
		})
		req := httptest.NewRequest("POST", "/api/admin/receiving/free-scan/process", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+receiptTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, unitCode, resp["unitCode"])
	})
}
