package router_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestSellerInventory_WireForecast(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping wire integration test in short mode")
	}

	ctx := context.Background()
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()

	// Safe DB check
	var dbName string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "wire integration test must run strictly against zamk_test")

	var (
		createdUserIDs    []uuid.UUID
		createdSellerIDs  []uuid.UUID
		createdProductIDs []uuid.UUID
		createdVariantIDs []uuid.UUID
	)

	t.Cleanup(func() {
		if len(createdVariantIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM variant_stock_forecasts WHERE product_variant_id = ANY($1)", createdVariantIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE id = ANY($1)", createdVariantIDs)
		}
		if len(createdProductIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProductIDs)
		}
		if len(createdSellerIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM seller_users WHERE seller_id = ANY($1)", createdSellerIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
		}
		if len(createdUserIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", createdUserIDs)
		}
	})

	// Build router
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

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	router, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	pfx := uuid.New().String()[:8]
	userAID := uuid.New()
	sellerAID := uuid.New()
	createdUserIDs = append(createdUserIDs, userAID)
	createdSellerIDs = append(createdSellerIDs, sellerAID)

	emailA := "seller_wire_a_" + pfx + "@zamk.local"
	phoneA := "7999" + pfx[:7]

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Seller A User', 'hash', 'seller', 'active', NOW(), NOW())
	`, userAID, emailA, phoneA)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Brand Wire A', $2, $3, 'active', NOW(), NOW())
	`, sellerAID, "brand-wire-a-"+pfx, emailA)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role)
		VALUES ($1, $2, $3, 'owner')
	`, uuid.New(), sellerAID, userAID)
	require.NoError(t, err)

	// Resolve / Insert Color (Красный)
	var redColorID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM colors WHERE name_ru = 'Красный' LIMIT 1").Scan(&redColorID)
	if err != nil {
		redColorID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active)
			VALUES ($1, 'red', 'Красный', '#FF0000', 1, true)
		`, redColorID)
		require.NoError(t, err)
	}

	// Resolve / Insert Size System & Size Value (L)
	var sizeSystemID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM size_systems LIMIT 1").Scan(&sizeSystemID)
	if err != nil {
		sizeSystemID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO size_systems (id, code, name, is_active)
			VALUES ($1, 'INT', 'International', true)
		`, sizeSystemID)
		require.NoError(t, err)
	}

	var sizeLID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM size_values WHERE size_system_id = $1 AND value = 'L' LIMIT 1", sizeSystemID).Scan(&sizeLID)
	if err != nil {
		sizeLID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO size_values (id, size_system_id, value, sort_order, is_active)
			VALUES ($1, $2, 'L', 1, true)
		`, sizeLID, sizeSystemID)
		require.NoError(t, err)
	}

	// 2. Seed Product A for Seller A with 2 variants:
	// A1: Modern dictionary variant (color_id, size_value_id, seller_sku; legacy color/size/sku NULL)
	// A2: Legacy variant (legacy size, color, sku)
	productAID := uuid.New()
	variantA1ID := uuid.New()
	variantA2ID := uuid.New()
	createdProductIDs = append(createdProductIDs, productAID)
	createdVariantIDs = append(createdVariantIDs, variantA1ID, variantA2ID)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id, created_at, updated_at)
		VALUES ($1, 'Wire Test Product A', $2, 5000, 'published', $3, NOW(), NOW())
	`, productAID, "prod-wire-a-"+productAID.String()[:8], sellerAID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, color_id, size_value_id, seller_sku, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'SKU-WIRE-A1', true, NOW(), NOW())
	`, variantA1ID, productAID, redColorID, sizeLID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, size, color, sku, is_active, created_at, updated_at)
		VALUES ($1, $2, 'M', 'Синий', 'SKU-WIRE-A2', true, NOW(), NOW())
	`, variantA2ID, productAID)
	require.NoError(t, err)

	// Inventory items for A1 and A2
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 2, NOW(), NOW())
	`, uuid.New(), productAID, variantA1ID, sellerAID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 5, 0, NOW(), NOW())
	`, uuid.New(), productAID, variantA2ID, sellerAID)
	require.NoError(t, err)

	// Insert forecast snapshot ONLY for Variant A1
	nowTime := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO variant_stock_forecasts (product_variant_id, state, days_of_cover, calculated_at)
		VALUES ($1, 'warning', 9.1, $2)
	`, variantA1ID, nowTime)
	require.NoError(t, err)

	// 3. Seed Seller B (Isolation check)
	userBID := uuid.New()
	sellerBID := uuid.New()
	productBID := uuid.New()
	variantB1ID := uuid.New()
	createdUserIDs = append(createdUserIDs, userBID)
	createdSellerIDs = append(createdSellerIDs, sellerBID)
	createdProductIDs = append(createdProductIDs, productBID)
	createdVariantIDs = append(createdVariantIDs, variantB1ID)

	pfxB := uuid.New().String()[:8]
	emailB := "seller_wire_b_" + pfxB + "@zamk.local"
	phoneB := "7998" + pfxB[:7]

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Seller B User', 'hash', 'seller', 'active', NOW(), NOW())
	`, userBID, emailB, phoneB)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Brand Wire B', $2, $3, 'active', NOW(), NOW())
	`, sellerBID, "brand-wire-b-"+pfxB, emailB)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role)
		VALUES ($1, $2, $3, 'owner')
	`, uuid.New(), sellerBID, userBID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id, created_at, updated_at)
		VALUES ($1, 'Wire Test Product B', $2, 3000, 'published', $3, NOW(), NOW())
	`, productBID, "prod-wire-b-"+productBID.String()[:8], sellerBID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, is_active, created_at, updated_at)
		VALUES ($1, $2, 'SKU-WIRE-B1', true, NOW(), NOW())
	`, variantB1ID, productBID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 20, 0, NOW(), NOW())
	`, uuid.New(), productBID, variantB1ID, sellerBID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO variant_stock_forecasts (product_variant_id, state, days_of_cover, calculated_at)
		VALUES ($1, 'critical', 3.5, $2)
	`, variantB1ID, nowTime)
	require.NoError(t, err)

	// 4. Generate JWT access token for Seller A
	token, err := tokenService.GenerateAccessToken(userAID, "seller_wire_a@zamk.local", "seller")
	require.NoError(t, err)

	// 5. Execute HTTP request to GET /api/seller/inventory
	req := httptest.NewRequest("GET", "/api/seller/inventory", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// 6. Assert HTTP 200 OK
	require.Equal(t, http.StatusOK, rec.Code, "GET /api/seller/inventory must return 200 OK")

	// 7. Verify Wire JSON (Raw map inspection to prove JSON serialization on the wire)
	var rawWire map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &rawWire)
	require.NoError(t, err, "response body must be valid JSON")

	rawItems, ok := rawWire["items"].([]interface{})
	require.True(t, ok, "response must have 'items' array")
	require.Equal(t, 2, len(rawItems), "Seller A must see exactly 2 items")

	var rawItemA1, rawItemA2 map[string]interface{}
	for _, it := range rawItems {
		itemMap := it.(map[string]interface{})
		vID := itemMap["variantId"].(string)
		if vID == variantA1ID.String() {
			rawItemA1 = itemMap
		} else if vID == variantA2ID.String() {
			rawItemA2 = itemMap
		} else if vID == variantB1ID.String() {
			t.Fatalf("Seller B variant %s appeared in Seller A inventory wire response (TENANT LEAK)", variantB1ID)
		}
	}

	// Assert Variant A1 (Modern) identity & forecast
	require.Equal(t, "SKU-WIRE-A1", rawItemA1["sku"])
	optA1, ok := rawItemA1["optionValues"].(map[string]interface{})
	require.True(t, ok, "Variant A1 must have optionValues map")
	require.Equal(t, "Красный", optA1["Цвет"])
	require.Equal(t, "L", optA1["Размер"])

	// Assert Variant A1 wire forecast shape:
	// "forecast": { "state": "warning", "daysOfCover": 9.1, "calculatedAt": "..." }
	require.NotNil(t, rawItemA1["forecast"], "Variant A1 must include 'forecast' object in wire JSON")
	forecastMap, ok := rawItemA1["forecast"].(map[string]interface{})
	require.True(t, ok, "forecast must be a JSON object")
	require.Equal(t, "warning", forecastMap["state"], "wire forecast.state must be 'warning'")
	require.Equal(t, 9.1, forecastMap["daysOfCover"], "wire forecast.daysOfCover must be 9.1")
	require.NotEmpty(t, forecastMap["calculatedAt"], "wire forecast.calculatedAt must not be empty")

	// Assert Variant A2 (Legacy) identity & forecast
	require.Equal(t, "SKU-WIRE-A2", rawItemA2["sku"])
	optA2, ok := rawItemA2["optionValues"].(map[string]interface{})
	require.True(t, ok, "Variant A2 must have optionValues map")
	require.Equal(t, "Синий", optA2["Цвет"])
	require.Equal(t, "M", optA2["Размер"])

	// Assert Variant A2 wire forecast is absent or nil
	require.Nil(t, rawItemA2["forecast"], "Variant A2 without snapshot must have forecast omitted or null")

	// 8. Also verify unmarshaling into typed DTO struct
	var typedResp inventory.SellerInventoryListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &typedResp)
	require.NoError(t, err)
	require.Equal(t, 2, typedResp.TotalCount)
	require.Equal(t, 2, len(typedResp.Items))

	for _, item := range typedResp.Items {
		if item.VariantID == variantA1ID {
			require.Equal(t, "SKU-WIRE-A1", item.SKU)
			require.Equal(t, "Красный", item.OptionValues["Цвет"])
			require.Equal(t, "L", item.OptionValues["Размер"])
			require.NotNil(t, item.Forecast)
			require.Equal(t, "warning", item.Forecast.State)
			require.NotNil(t, item.Forecast.DaysOfCover)
			require.Equal(t, 9.1, *item.Forecast.DaysOfCover)
			require.True(t, item.Forecast.CalculatedAt.Equal(nowTime))
		} else if item.VariantID == variantA2ID {
			require.Equal(t, "SKU-WIRE-A2", item.SKU)
			require.Equal(t, "Синий", item.OptionValues["Цвет"])
			require.Equal(t, "M", item.OptionValues["Размер"])
			require.Nil(t, item.Forecast)
		}
	}
}
