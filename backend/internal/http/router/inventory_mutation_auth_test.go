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
	"time"

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

func TestInventoryMutationAuthAndBoundary(t *testing.T) {
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

	// Fixture IDs
	sellerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	varID := uuid.New()
	inventoryItemID := uuid.New()

	adminNoPermID := uuid.New()
	adminAdjustID := uuid.New()
	adminReceiptID := uuid.New()
	adminWriteOffID := uuid.New()

	roleNoPermID := uuid.New()
	roleAdjustID := uuid.New()
	roleReceiptID := uuid.New()
	roleWriteOffID := uuid.New()

	pfx := "inv_auth_" + uuid.NewString()[:8]

	cleanup := func() {
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM audit_logs WHERE actor_user_id = ANY($1)", []uuid.UUID{adminNoPermID, adminAdjustID, adminReceiptID, adminWriteOffID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM stock_movements WHERE product_variant_id = $1 OR inventory_item_id = $2", varID, inventoryItemID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE id = $1 OR product_variant_id = $2", inventoryItemID, varID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE id = $1", varID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = $1", prodID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", catID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = ANY($1)", []uuid.UUID{adminNoPermID, adminAdjustID, adminReceiptID, adminWriteOffID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = ANY($1)", []uuid.UUID{roleNoPermID, roleAdjustID, roleReceiptID, roleWriteOffID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = ANY($1)", []uuid.UUID{roleNoPermID, roleAdjustID, roleReceiptID, roleWriteOffID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{adminNoPermID, adminAdjustID, adminReceiptID, adminWriteOffID})
	}
	cleanup()
	t.Cleanup(cleanup)

	// Setup Seller, Category, Product, Variant, Inventory Item
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Inv Brand', $2, $3, 'active', now(), now())
	`, sellerID, pfx+"_brand", pfx+"@brand.local")
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Inv Cat', $2, now(), now())
	`, catID, pfx+"_cat")
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Inv Prod', $4, 1500, 'published', now(), now())
	`, prodID, sellerID, catID, pfx+"_prod")
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1500, true, now(), now())
	`, varID, prodID, pfx+"_sku", pfx+"_ssku", fmt.Sprintf("98%011d", time.Now().UnixNano()%100000000000))
	require.NoError(t, err)

	// Initial inventory: total_stock = 20, reserved_stock = 5
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 20, 5, now(), now())
	`, inventoryItemID, prodID, varID, sellerID)
	require.NoError(t, err)

	// Helper to insert user & staff role with permissions
	setupAdmin := func(userID, roleID uuid.UUID, email, roleCode, perm string) string {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
			VALUES ($1, 'Staff Admin', $2, 'hash', 'admin', 'active', false, now(), now())
		`, userID, email)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO staff_roles (id, code, name)
			VALUES ($1, $2, 'Role')
		`, roleID, roleCode)
		require.NoError(t, err)

		if perm != "" {
			_, err = pgClient.Pool.Exec(ctx, `
				INSERT INTO staff_role_permissions (role_id, permission)
				VALUES ($1, $2)
			`, roleID, perm)
			require.NoError(t, err)
		}

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO staff_members (user_id, staff_role_id, status)
			VALUES ($1, $2, 'active')
		`, userID, roleID)
		require.NoError(t, err)

		token, err := tokenService.GenerateAccessToken(userID, email, "admin")
		require.NoError(t, err)
		return token
	}

	tokenNoPerm := setupAdmin(adminNoPermID, roleNoPermID, pfx+"_noperm@zamk.local", pfx+"_role_noperm", "")
	tokenAdjust := setupAdmin(adminAdjustID, roleAdjustID, pfx+"_adjust@zamk.local", pfx+"_role_adjust", "inventory.adjust")
	tokenReceipt := setupAdmin(adminReceiptID, roleReceiptID, pfx+"_receipt@zamk.local", pfx+"_role_receipt", "inventory.receipt")
	tokenWriteOff := setupAdmin(adminWriteOffID, roleWriteOffID, pfx+"_writeoff@zamk.local", pfx+"_role_writeoff", "inventory.write_off")

	// =========================================================================
	// 1. Obsolete Generic Endpoint: POST /api/admin/inventory/{id}/adjust
	// Must return 404/405 and cause zero mutations in DB
	// =========================================================================
	t.Run("Obsolete generic adjust endpoint is completely removed", func(t *testing.T) {
		obsoletePath := fmt.Sprintf("/api/admin/inventory/%s/adjust", inventoryItemID)

		// Test A: Calling with inventory.adjust attempting receipt bypass
		receiptBypassBody, _ := json.Marshal(map[string]any{
			"type":     "receipt",
			"quantity": 100,
			"reason":   "Attempted receipt bypass via obsolete endpoint",
		})
		req := httptest.NewRequest(http.MethodPost, obsoletePath, bytes.NewReader(receiptBypassBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenAdjust)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed,
			"Obsolete endpoint must return 404 or 405, got %d", w.Code)

		// Test B: Calling with inventory.adjust attempting write-off bypass
		writeOffBypassBody, _ := json.Marshal(map[string]any{
			"type":     "write_off",
			"quantity": 10,
			"reason":   "Attempted write-off bypass via obsolete endpoint",
		})
		req = httptest.NewRequest(http.MethodPost, obsoletePath, bytes.NewReader(writeOffBypassBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenAdjust)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed,
			"Obsolete endpoint must return 404 or 405, got %d", w.Code)

		// Test C: Calling with inventory.receipt token
		req = httptest.NewRequest(http.MethodPost, obsoletePath, bytes.NewReader(receiptBypassBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenReceipt)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed,
			"Obsolete endpoint must return 404 or 405, got %d", w.Code)

		// Test D: Calling unauthenticated
		req = httptest.NewRequest(http.MethodPost, obsoletePath, bytes.NewReader(receiptBypassBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed || w.Code == http.StatusUnauthorized,
			"Obsolete endpoint must not route successfully, got %d", w.Code)

		// Verification: DB state MUST be pristine (total_stock=20, reserved_stock=5, 0 stock movements)
		var curTotal, curReserved int
		err := pgClient.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", inventoryItemID).Scan(&curTotal, &curReserved)
		require.NoError(t, err)
		assert.Equal(t, 20, curTotal, "total_stock must not change after calls to removed endpoint")
		assert.Equal(t, 5, curReserved, "reserved_stock must not change after calls to removed endpoint")

		var movCount int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE product_variant_id = $1", varID).Scan(&movCount)
		require.NoError(t, err)
		assert.Equal(t, 0, movCount, "zero stock_movements must exist after calls to removed endpoint")
	})

	// =========================================================================
	// 2. Dedicated Adjustment Endpoint: POST /api/admin/inventory/adjustments
	// Requires: inventory.adjust
	// =========================================================================
	t.Run("Dedicated adjustments endpoint enforces inventory.adjust permission", func(t *testing.T) {
		adjustBody, _ := json.Marshal(map[string]any{
			"productVariantId": varID,
			"quantity":         7,
			"reason":           "Physical inventory cycle recount",
		})

		// Unauthenticated -> 401
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/adjustments", bytes.NewReader(adjustBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		// Authenticated without inventory.adjust -> 403
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/adjustments", bytes.NewReader(adjustBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenNoPerm)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code)
		}

		// Authenticated with inventory.receipt only -> 403
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/adjustments", bytes.NewReader(adjustBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenReceipt)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code)
		}

		// Authenticated with inventory.adjust -> 200 OK
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/adjustments", bytes.NewReader(adjustBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenAdjust)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			// DB verification: total_stock was 20, added 7 -> 27
			var curTotal int
			err := pgClient.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", inventoryItemID).Scan(&curTotal)
			require.NoError(t, err)
			assert.Equal(t, 27, curTotal)

			// Stock movement verification
			var movType string
			var movQty int
			var movActor *uuid.UUID
			err = pgClient.Pool.QueryRow(ctx, "SELECT type, quantity, actor_user_id FROM stock_movements WHERE product_variant_id = $1 ORDER BY created_at DESC LIMIT 1", varID).
				Scan(&movType, &movQty, &movActor)
			require.NoError(t, err)
			assert.Equal(t, "adjustment", movType)
			assert.Equal(t, 7, movQty)
			require.NotNil(t, movActor)
			assert.Equal(t, adminAdjustID, *movActor)
		}
	})

	// =========================================================================
	// 3. Dedicated Receipts Endpoint: POST /api/admin/inventory/receipts
	// Requires: inventory.receipt
	// =========================================================================
	t.Run("Dedicated receipts endpoint enforces inventory.receipt permission", func(t *testing.T) {
		reason := "Supplier delivery Batch-A"
		receiptBody, _ := json.Marshal(map[string]any{
			"productVariantId": varID,
			"quantity":         10,
			"reason":           reason,
		})

		// Unauthenticated -> 401
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/receipts", bytes.NewReader(receiptBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		// Authenticated without inventory.receipt (roleNoPerm) -> 403
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/receipts", bytes.NewReader(receiptBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenNoPerm)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code)
		}

		// Authenticated with inventory.adjust only -> 403 (verifying P0 leak is closed: adjust cannot do receipt!)
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/receipts", bytes.NewReader(receiptBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenAdjust)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code, "inventory.adjust alone MUST NOT allow receiving stock")
		}

		// Authenticated with inventory.receipt -> 200 OK
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/receipts", bytes.NewReader(receiptBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenReceipt)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			// DB verification: total_stock was 27, received 10 -> 37
			var curTotal int
			err := pgClient.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", inventoryItemID).Scan(&curTotal)
			require.NoError(t, err)
			assert.Equal(t, 37, curTotal)

			// Stock movement verification
			var movType string
			var movQty int
			var movActor *uuid.UUID
			err = pgClient.Pool.QueryRow(ctx, "SELECT type, quantity, actor_user_id FROM stock_movements WHERE product_variant_id = $1 ORDER BY created_at DESC LIMIT 1", varID).
				Scan(&movType, &movQty, &movActor)
			require.NoError(t, err)
			assert.Equal(t, "receipt", movType)
			assert.Equal(t, 10, movQty)
			require.NotNil(t, movActor)
			assert.Equal(t, adminReceiptID, *movActor)
		}
	})

	// =========================================================================
	// 4. Dedicated Write-Offs Endpoint: POST /api/admin/inventory/write-offs
	// Requires: inventory.write_off
	// =========================================================================
	t.Run("Dedicated write-offs endpoint enforces inventory.write_off permission", func(t *testing.T) {
		writeOffBody, _ := json.Marshal(map[string]any{
			"productVariantId": varID,
			"quantity":         3,
			"reason":           "Damaged in warehouse storage",
		})

		// Unauthenticated -> 401
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/write-offs", bytes.NewReader(writeOffBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		// Authenticated without inventory.write_off (roleNoPerm) -> 403
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/write-offs", bytes.NewReader(writeOffBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenNoPerm)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code)
		}

		// Authenticated with inventory.adjust only -> 403 (verifying P0 leak is closed: adjust cannot write-off!)
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/write-offs", bytes.NewReader(writeOffBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenAdjust)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code, "inventory.adjust alone MUST NOT allow writing off stock")
		}

		// Authenticated with inventory.write_off -> 200 OK
		{
			req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/write-offs", bytes.NewReader(writeOffBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenWriteOff)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			// DB verification: total_stock was 37, written off 3 -> 34
			var curTotal int
			err := pgClient.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", inventoryItemID).Scan(&curTotal)
			require.NoError(t, err)
			assert.Equal(t, 34, curTotal)

			// Stock movement verification
			var movType string
			var movQty int
			var movActor *uuid.UUID
			err = pgClient.Pool.QueryRow(ctx, "SELECT type, quantity, actor_user_id FROM stock_movements WHERE product_variant_id = $1 ORDER BY created_at DESC LIMIT 1", varID).
				Scan(&movType, &movQty, &movActor)
			require.NoError(t, err)
			assert.Equal(t, "write_off", movType)
			assert.Equal(t, 3, movQty)
			require.NotNil(t, movActor)
			assert.Equal(t, adminWriteOffID, *movActor)
		}
	})
}
