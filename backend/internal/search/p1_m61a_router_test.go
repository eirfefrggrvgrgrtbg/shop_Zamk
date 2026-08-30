package search_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/search"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

func TestM61A_RouterRBAC(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	defer pgClient.Close()

	// 1. Strict DB Safety Guard
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
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	// Clean up only our test data by unique prefix/keys
	cleanup := func() {
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM supply_receiving_scans WHERE inventory_unit_id IN (SELECT id FROM inventory_units WHERE unit_code LIKE 'ZMU-98%')`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM inventory_units WHERE unit_code LIKE 'ZMU-98%'`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM seller_supply_items WHERE supply_id IN (SELECT id FROM seller_supplies WHERE supply_number LIKE 'SUP-98%')`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM seller_supplies WHERE supply_number LIKE 'SUP-98%'`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM returns WHERE reason = 'm61a_router_test'`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM order_fulfillments WHERE order_id IN (SELECT id FROM orders WHERE order_number LIKE 'ORD-98%')`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM orders WHERE order_number LIKE 'ORD-98%'`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE slug LIKE 'm61a-rtr-%') OR barcode LIKE 'ZMK-98%'`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM products WHERE slug LIKE 'm61a-rtr-%'`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM staff_members WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'm61a_rtr_%')`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM staff_role_permissions WHERE role_id IN (SELECT id FROM staff_roles WHERE code LIKE 'm61a_rtr_%')`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM staff_roles WHERE code LIKE 'm61a_rtr_%'`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM sellers WHERE slug LIKE 'm61a-rtr-%' OR id IN (SELECT id FROM users WHERE email LIKE 'm61a_rtr_%')`)
		_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'm61a_rtr_%'`)
	}
	cleanup()
	defer cleanup()

	// 2. Self-Contained Minimal Search Fixtures
	sellerUserID := uuid.New()
	customerUserID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	unitID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	return1ID := uuid.New()
	return2ID := uuid.New()

	fixtureORD := "ORD-980001"
	fixtureZMU := "ZMU-98765432ABCDEFGH"
	fixtureZMK := "ZMK-9801"

	// Seller & Brand
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'M61A RTR Seller', 'm61a_rtr_seller@test.com', 'hash', 'seller', NOW(), NOW())
	`, sellerUserID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, status, created_at, updated_at)
		VALUES ($1, 'M61A RTR Brand', 'm61a-rtr-brand', 'active', NOW(), NOW())
	`, sellerUserID)
	require.NoError(t, err)

	// Customer
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'M61A RTR Customer', 'm61a_rtr_customer@test.com', 'hash', 'customer', NOW(), NOW())
	`, customerUserID)
	require.NoError(t, err)

	custToken, err := tokenService.GenerateAccessToken(customerUserID, "m61a_rtr_customer@test.com", users.RoleCustomer)
	require.NoError(t, err)

	// Product & Variant
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, title, slug, status, price_cents, average_rating, seller_id, created_at, updated_at)
		VALUES ($1, 'M61A RTR Test Product', 'm61a-rtr-prod', 'published', 10000, 0, $2, NOW(), NOW())
	`, productID, sellerUserID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, is_active, barcode, seller_sku, created_at, updated_at)
		VALUES ($1, $2, true, $3, 'SKU-9801-X', NOW(), NOW())
	`, variantID, productID, fixtureZMK)
	require.NoError(t, err)

	// Supply & Inventory Unit (ZMU)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', 'SUP-9801', 'courier', NOW(), NOW())
	`, supplyID, sellerUserID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 1, 1, 0, 0, 0, NOW(), NOW())
	`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 1, NOW(), NOW())
	`, unitID, variantID, fixtureZMU, supplyID, supplyItemID)
	require.NoError(t, err)

	// Order
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, order_number, user_id, status, customer_name, customer_email, customer_phone, delivery_address, total_price_cents, currency, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 'M61A RTR Buyer', 'm61a_rtr_customer@test.com', '+79998887766', 'Address 1', 10000, 'RUB', NOW(), NOW())
	`, orderID, fixtureORD, customerUserID)
	require.NoError(t, err)

	// Fulfillment
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', NOW(), NOW())
	`, fulfillmentID, orderID, sellerUserID)
	require.NoError(t, err)

	// TWO Returns for the same Order
	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'rejected', 'm61a_router_test', $5, $5)
	`, return1ID, orderID, fulfillmentID, customerUserID, t1)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'needs_info', 'm61a_router_test', $5, $5)
	`, return2ID, orderID, fulfillmentID, customerUserID, t2)
	require.NoError(t, err)

	// 3. Setup Staff Roles & Users for RBAC Tests

	// A. Admin user WITHOUT staff membership
	adminNoStaffID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Admin No Staff', 'm61a_rtr_nostaff@test.com', 'hash', 'admin', NOW(), NOW())
	`, adminNoStaffID)
	require.NoError(t, err)
	adminNoStaffToken, err := tokenService.GenerateAccessToken(adminNoStaffID, "m61a_rtr_nostaff@test.com", users.RoleAdmin)
	require.NoError(t, err)

	// B. Inactive/Suspended staff member
	var ownerRoleID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner'`).Scan(&ownerRoleID)
	require.NoError(t, err)

	suspendedAdminID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Suspended Admin', 'm61a_rtr_suspended@test.com', 'hash', 'admin', NOW(), NOW())
	`, suspendedAdminID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'blocked', NOW(), NOW())
	`, suspendedAdminID, ownerRoleID)
	require.NoError(t, err)
	suspendedToken, err := tokenService.GenerateAccessToken(suspendedAdminID, "m61a_rtr_suspended@test.com", users.RoleAdmin)
	require.NoError(t, err)

	// C. Active staff member with ZERO relevant search permissions
	zeroPermsRoleID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, description, is_system, created_at, updated_at)
		VALUES ($1, 'm61a_rtr_zero', 'Zero Perms Role', 'Testing', false, NOW(), NOW())
	`, zeroPermsRoleID)
	require.NoError(t, err)

	zeroPermsAdminID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Zero Perms Admin', 'm61a_rtr_zero@test.com', 'hash', 'admin', NOW(), NOW())
	`, zeroPermsAdminID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, zeroPermsAdminID, zeroPermsRoleID)
	require.NoError(t, err)
	zeroPermsToken, err := tokenService.GenerateAccessToken(zeroPermsAdminID, "m61a_rtr_zero@test.com", users.RoleAdmin)
	require.NoError(t, err)

	// D. Active staff member with ONLY orders.read
	ordersOnlyRoleID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, description, is_system, created_at, updated_at)
		VALUES ($1, 'm61a_rtr_orders', 'Orders Only', 'Testing', false, NOW(), NOW())
	`, ordersOnlyRoleID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_role_permissions (role_id, permission)
		VALUES ($1, 'orders.read')
	`, ordersOnlyRoleID)
	require.NoError(t, err)

	ordersOnlyAdminID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Orders Only Admin', 'm61a_rtr_orders@test.com', 'hash', 'admin', NOW(), NOW())
	`, ordersOnlyAdminID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, ordersOnlyAdminID, ordersOnlyRoleID)
	require.NoError(t, err)
	ordersOnlyToken, err := tokenService.GenerateAccessToken(ordersOnlyAdminID, "m61a_rtr_orders@test.com", users.RoleAdmin)
	require.NoError(t, err)

	// E. Active staff member with ONLY returns.read
	returnsOnlyRoleID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, description, is_system, created_at, updated_at)
		VALUES ($1, 'm61a_rtr_returns', 'Returns Only', 'Testing', false, NOW(), NOW())
	`, returnsOnlyRoleID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_role_permissions (role_id, permission)
		VALUES ($1, 'returns.read')
	`, returnsOnlyRoleID)
	require.NoError(t, err)

	returnsOnlyAdminID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Returns Only Admin', 'm61a_rtr_returns@test.com', 'hash', 'admin', NOW(), NOW())
	`, returnsOnlyAdminID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, returnsOnlyAdminID, returnsOnlyRoleID)
	require.NoError(t, err)
	returnsOnlyToken, err := tokenService.GenerateAccessToken(returnsOnlyAdminID, "m61a_rtr_returns@test.com", users.RoleAdmin)
	require.NoError(t, err)

	// F. Active staff member with ONLY inventory.read
	inventoryOnlyRoleID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, description, is_system, created_at, updated_at)
		VALUES ($1, 'm61a_rtr_inventory', 'Inventory Only', 'Testing', false, NOW(), NOW())
	`, inventoryOnlyRoleID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_role_permissions (role_id, permission)
		VALUES ($1, 'inventory.read')
	`, inventoryOnlyRoleID)
	require.NoError(t, err)

	inventoryOnlyAdminID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Inventory Only Admin', 'm61a_rtr_inv@test.com', 'hash', 'admin', NOW(), NOW())
	`, inventoryOnlyAdminID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, inventoryOnlyAdminID, inventoryOnlyRoleID)
	require.NoError(t, err)
	inventoryOnlyToken, err := tokenService.GenerateAccessToken(inventoryOnlyAdminID, "m61a_rtr_inv@test.com", users.RoleAdmin)
	require.NoError(t, err)

	// G. Full Owner Admin
	ownerAdminID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Owner Admin', 'm61a_rtr_owner@test.com', 'hash', 'admin', NOW(), NOW())
	`, ownerAdminID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, ownerAdminID, ownerRoleID)
	require.NoError(t, err)
	ownerToken, err := tokenService.GenerateAccessToken(ownerAdminID, "m61a_rtr_owner@test.com", users.RoleAdmin)
	require.NoError(t, err)

	// 4. Test Scenarios

	t.Run("Unauthenticated request returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q=test", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Customer role request returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q=test", nil)
		req.Header.Set("Authorization", "Bearer "+custToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Admin-role user WITHOUT staff membership returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q=test", nil)
		req.Header.Set("Authorization", "Bearer "+adminNoStaffToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Inactive/Suspended staff member returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q=test", nil)
		req.Header.Set("Authorization", "Bearer "+suspendedToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Active staff member with ZERO relevant search permissions returns 200 with empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+fixtureORD, nil)
		req.Header.Set("Authorization", "Bearer "+zeroPermsToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp search.GlobalSearchResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Results)
	})

	t.Run("HTTP partial RBAC: orders.read sees Order but NOT Returns", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+fixtureORD, nil)
		req.Header.Set("Authorization", "Bearer "+ordersOnlyToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp search.GlobalSearchResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Must non-vacuously find exactly the 1 Order
		require.Len(t, resp.Results, 1, "Must find exactly 1 result for orders.read")
		assert.Equal(t, search.ResultTypeOrder, resp.Results[0].Type)
		assert.Equal(t, fixtureORD, resp.Results[0].CanonicalIdentifier)
		assert.Equal(t, "/orders/"+orderID.String(), resp.Results[0].NavigationTarget)

		// Explicitly assert that no other domains are exposed
		for _, item := range resp.Results {
			assert.NotEqual(t, search.ResultTypeReturn, item.Type)
			assert.NotEqual(t, search.ResultTypeCustomer, item.Type)
			assert.NotEqual(t, search.ResultTypeInventoryUnit, item.Type)
			assert.NotEqual(t, search.ResultTypeProduct, item.Type)
			assert.NotEqual(t, search.ResultTypeProductVariant, item.Type)
		}
	})

	t.Run("HTTP partial RBAC: returns.read sees both Returns but NOT Order", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+fixtureORD, nil)
		req.Header.Set("Authorization", "Bearer "+returnsOnlyToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp search.GlobalSearchResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Must find exactly the 2 distinct Returns
		require.Len(t, resp.Results, 2, "Must find exactly 2 return results for returns.read")
		assert.Equal(t, search.ResultTypeReturn, resp.Results[0].Type)
		assert.Equal(t, search.ResultTypeReturn, resp.Results[1].Type)
		assert.Equal(t, fixtureORD, resp.Results[0].CanonicalIdentifier)
		assert.Equal(t, fixtureORD, resp.Results[1].CanonicalIdentifier)
		assert.NotEqual(t, resp.Results[0].ID, resp.Results[1].ID, "Returns must have distinct result IDs")

		// Order result MUST NOT be exposed
		for _, item := range resp.Results {
			assert.NotEqual(t, search.ResultTypeOrder, item.Type)
		}
	})

	t.Run("HTTP partial RBAC: inventory.read sees canonical ZMU but NOT Order", func(t *testing.T) {
		// Programmatic verification of canonical ZMU format
		require.True(t, strings.HasPrefix(fixtureZMU, "ZMU-"), "ZMU must start with ZMU-")
		suffix := strings.TrimPrefix(fixtureZMU, "ZMU-")
		assert.Len(t, suffix, 16, "ZMU suffix must be exactly 16 characters")
		const canonicalAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
		for _, c := range suffix {
			assert.True(t, strings.ContainsRune(canonicalAlphabet, c), "Character %c must belong to canonical Crockford Base32 alphabet (no 0, 1, I, L, O, U)", c)
		}

		// 1. Query exact uppercase ZMU fixture
		reqZMU := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+fixtureZMU, nil)
		reqZMU.Header.Set("Authorization", "Bearer "+inventoryOnlyToken)
		recZMU := httptest.NewRecorder()
		r.ServeHTTP(recZMU, reqZMU)
		assert.Equal(t, http.StatusOK, recZMU.Code)

		var respZMU search.GlobalSearchResponse
		err := json.Unmarshal(recZMU.Body.Bytes(), &respZMU)
		require.NoError(t, err)
		require.Len(t, respZMU.Results, 1, "Must find exactly 1 ZMU unit result")
		assert.Equal(t, search.ResultTypeInventoryUnit, respZMU.Results[0].Type)
		assert.Equal(t, fixtureZMU, respZMU.Results[0].CanonicalIdentifier)
		assert.Equal(t, "/inventory", respZMU.Results[0].NavigationTarget)

		// 2. Query lowercase ZMU -> must resolve to stored canonical uppercase ZMU
		reqZMULower := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+strings.ToLower(fixtureZMU), nil)
		reqZMULower.Header.Set("Authorization", "Bearer "+inventoryOnlyToken)
		recZMULower := httptest.NewRecorder()
		r.ServeHTTP(recZMULower, reqZMULower)
		assert.Equal(t, http.StatusOK, recZMULower.Code)

		var respZMULower search.GlobalSearchResponse
		err = json.Unmarshal(recZMULower.Body.Bytes(), &respZMULower)
		require.NoError(t, err)
		require.Len(t, respZMULower.Results, 1)
		assert.Equal(t, fixtureZMU, respZMULower.Results[0].CanonicalIdentifier, "Lowercase ZMU search must return stored canonical uppercase ZMU")

		// 3. Query fixture ORD -> must be empty for inventory.read
		reqORD := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+fixtureORD, nil)
		reqORD.Header.Set("Authorization", "Bearer "+inventoryOnlyToken)
		recORD := httptest.NewRecorder()
		r.ServeHTTP(recORD, reqORD)
		assert.Equal(t, http.StatusOK, recORD.Code)

		var respORD search.GlobalSearchResponse
		err = json.Unmarshal(recORD.Body.Bytes(), &respORD)
		require.NoError(t, err)
		assert.Empty(t, respORD.Results, "inventory.read only user must not see order or return results")
	})

	t.Run("Short query returns 400 Bad Request with error code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q=a", nil)
		req.Header.Set("Authorization", "Bearer "+ownerToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var errResp map[string]map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "query_too_short", errResp["error"]["code"])
	})

	t.Run("Fail-Closed: Handler with nil staffSvc returns 500 internal_error and exposes zero data", func(t *testing.T) {
		h := search.NewHandler(search.NewService(search.NewRepository(pgClient.Pool)), nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+fixtureORD, nil)
		reqCtx := context.WithValue(req.Context(), "userID", ownerAdminID)
		req = req.WithContext(reqCtx)

		rec := httptest.NewRecorder()
		h.HandleGlobalSearch(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		var errResp map[string]map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "internal_error", errResp["error"]["code"])
		assert.Equal(t, "Internal server error", errResp["error"]["message"])
		assert.NotContains(t, rec.Body.String(), "ORD-")
	})

	t.Run("Internal database error does not leak SQL error details", func(t *testing.T) {
		closedPool, err := pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		closedPool.Close()

		userRepo := users.NewRepository(pgClient.Pool)
		staffRepo := staff.NewRepository(pgClient.Pool)
		staffSvc := staff.NewService(staffRepo, userRepo, pgClient)

		h := search.NewHandler(search.NewService(search.NewRepository(closedPool)), staffSvc, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/search?q="+fixtureORD, nil)
		reqCtx := context.WithValue(req.Context(), "userID", ownerAdminID)
		req = req.WithContext(reqCtx)

		rec := httptest.NewRecorder()
		h.HandleGlobalSearch(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		var errResp map[string]map[string]string
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "internal_error", errResp["error"]["code"])
		assert.Equal(t, "Internal server error", errResp["error"]["message"])
		assert.NotContains(t, rec.Body.String(), "closed pool")
		assert.NotContains(t, rec.Body.String(), "SELECT")
		assert.NotContains(t, rec.Body.String(), "postgres")
	})
}
