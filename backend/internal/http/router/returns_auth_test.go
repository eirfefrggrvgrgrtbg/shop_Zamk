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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

func TestAdminReturnsReceivingRouter_RBAC(t *testing.T) {
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
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'ReturnsRole')`, roleID, code)
		require.NoError(t, err)
		for _, p := range perms {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
			require.NoError(t, err)
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	// 1. Prepare fixture data
	sellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Returns Brand', $2, $3, 'active', now(), now())
	`, sellerID, uuid.New().String(), uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug)
		VALUES ($1, 'Cat', $2)
	`, catID, "cat-"+uuid.New().String()[:8])
	require.NoError(t, err)

	prodID := uuid.New()
	varID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status)
		VALUES ($1, $2, $3, 'Prod Title', $4, 1000, 'published')
	`, prodID, sellerID, catID, "prod-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents)
		VALUES ($1, $2, $3, 1000)
	`, varID, prodID, "SKU-"+uuid.New().String()[:8])
	require.NoError(t, err)

	buyerID := insertUser("customer")
	orderID := uuid.New()
	orderNum := fmt.Sprintf("ORD-RBAC-%s", uuid.New().String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, $3, 'delivered', 1000, 'N', 'P', 'E', 'A')
	`, orderID, buyerID, orderNum)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'delivered', 1000, 900, 900)
	`, fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	orderItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Prod Title', 'prod-slug', 1000, 1000, 1)
	`, orderItemID, orderID, fulfillmentID, sellerID, prodID, varID)
	require.NoError(t, err)

	shipmentID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '2 hours', now() - interval '1 hour', now(), now())
	`, shipmentID, orderID, fulfillmentID)
	require.NoError(t, err)

	// Create Return in receiving status so scan endpoint succeeds when called by authorized admin
	returnID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, receiving_started_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'receiving', 'defective', now(), now(), now())
	`, returnID, orderID, fulfillmentID, buyerID)
	require.NoError(t, err)

	returnItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
		VALUES ($1, $2, $3, 1, 0, 0, 0, now())
	`, returnItemID, returnID, orderItemID)
	require.NoError(t, err)

	// Create inventory, supply, unit, allocation
	invItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, prodID, varID, sellerID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, sellerID, "SUP-RBAC-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, varID)
	require.NoError(t, err)

	zmuCode := "ZMU-RBAC-" + uuid.New().String()[:8]
	invUnitID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, zmuCode, varID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, prodID, varID, buyerID, orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, now() - interval '2 hours', NULL)
	`, allocID, orderItemID, invUnitID, resID)
	require.NoError(t, err)

	// Roles and Tokens
	adminWithRead := insertUser("admin")
	insertAdminWithPerms(adminWithRead, []string{"returns.read"})
	adminReadToken := makeToken(adminWithRead, "admin")

	adminWithUpdate := insertUser("admin")
	insertAdminWithPerms(adminWithUpdate, []string{"returns.update_status"})
	adminUpdateToken := makeToken(adminWithUpdate, "admin")

	adminWithBoth := insertUser("admin")
	insertAdminWithPerms(adminWithBoth, []string{"returns.read", "returns.update_status"})
	adminBothToken := makeToken(adminWithBoth, "admin")

	sellerUser := insertUser("seller")
	sellerToken := makeToken(sellerUser, "seller")

	customerUser := insertUser("customer")
	customerToken := makeToken(customerUser, "customer")

	// Endpoint 1: GET /api/admin/returns/{id}/receiving (needs returns.read)
	t.Run("GET receiving - unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("GET receiving - customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("GET receiving - seller -> 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		req.Header.Set("Authorization", "Bearer "+sellerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("GET receiving - admin with returns.update_status only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		req.Header.Set("Authorization", "Bearer "+adminUpdateToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("GET receiving - admin with returns.read -> 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		req.Header.Set("Authorization", "Bearer "+adminReadToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var state returns.AdminReturnReceivingState
		err := json.NewDecoder(rr.Body).Decode(&state)
		require.NoError(t, err)
		assert.Equal(t, returnID, state.Return.ID)
		assert.Equal(t, "receiving", state.Return.Status)
		assert.Equal(t, 1, state.TotalRequested)
		assert.Equal(t, 0, state.TotalScanned)
	})

	// Endpoint 2: POST /api/admin/returns/{id}/receiving/start (needs returns.update_status)
	t.Run("POST receiving/start - unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/start", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("POST receiving/start - customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/start", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/start - admin with returns.read only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/start", nil)
		req.Header.Set("Authorization", "Bearer "+adminReadToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/start - admin with returns.update_status -> 204", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/start", nil)
		req.Header.Set("Authorization", "Bearer "+adminUpdateToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	// Endpoint 3: POST /api/admin/returns/{id}/receiving/scan (needs returns.update_status)
	t.Run("POST receiving/scan - unauthenticated -> 401", func(t *testing.T) {
		body, _ := json.Marshal(returns.ScanReturnUnitRequest{Code: zmuCode})
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/scan", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("POST receiving/scan - customer -> 403", func(t *testing.T) {
		body, _ := json.Marshal(returns.ScanReturnUnitRequest{Code: zmuCode})
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/scan - admin with returns.read only -> 403", func(t *testing.T) {
		body, _ := json.Marshal(returns.ScanReturnUnitRequest{Code: zmuCode})
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminReadToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/scan - admin with returns.update_status -> 200", func(t *testing.T) {
		body, _ := json.Marshal(returns.ScanReturnUnitRequest{Code: zmuCode})
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminBothToken)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var scanResp returns.ScanReturnUnitResponse
		err := json.NewDecoder(rr.Body).Decode(&scanResp)
		require.NoError(t, err)
		assert.False(t, scanResp.AlreadyScanned)
		assert.Equal(t, allocID, scanResp.ReturnItemUnit.OrderItemAllocationID)

		scannedUnitID := scanResp.ReturnItemUnit.ID

		// Endpoint 4: PATCH /api/admin/returns/{id}/receiving/units/{unitId} (needs returns.update_status)
		cond := "Good condition"
		inspBody, _ := json.Marshal(returns.UpdateSerializedUnitInspectionRequest{
			InspectedCondition: &cond,
			Disposition:        "restock",
		})

		// 4A: unauthenticated -> 401
		req4A := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		rr4A := httptest.NewRecorder()
		r.ServeHTTP(rr4A, req4A)
		assert.Equal(t, http.StatusUnauthorized, rr4A.Code)

		// 4B: customer -> 403
		req4B := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		req4B.Header.Set("Authorization", "Bearer "+customerToken)
		rr4B := httptest.NewRecorder()
		r.ServeHTTP(rr4B, req4B)
		assert.Equal(t, http.StatusForbidden, rr4B.Code)

		// 4C: admin with returns.read only -> 403
		req4C := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		req4C.Header.Set("Authorization", "Bearer "+adminReadToken)
		rr4C := httptest.NewRecorder()
		r.ServeHTTP(rr4C, req4C)
		assert.Equal(t, http.StatusForbidden, rr4C.Code)

		// 4D: admin with returns.update_status -> 204
		req4D := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		req4D.Header.Set("Authorization", "Bearer "+adminUpdateToken)
		rr4D := httptest.NewRecorder()
		r.ServeHTTP(rr4D, req4D)
		assert.Equal(t, http.StatusNoContent, rr4D.Code)

		// Verify serialized inspection was actually persisted in DB
		var dbDisp string
		var dbCond *string
		err = pgClient.Pool.QueryRow(ctx, "SELECT disposition, inspected_condition FROM return_item_units WHERE id = $1", scannedUnitID).Scan(&dbDisp, &dbCond)
		require.NoError(t, err)
		assert.Equal(t, "restock", dbDisp)
		require.NotNil(t, dbCond)
		assert.Equal(t, "Good condition", *dbCond)

		// Endpoint 5: PATCH /api/admin/returns/{id}/receiving/items/{itemId}/legacy-inspection (needs returns.update_status)
		// Setup legacy return fixture
		legOrderID := uuid.New()
		legFID := uuid.New()
		legOIID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
			VALUES ($1, $2, $3, 'delivered', 5000, 'RUB', 'Addr', 'Method', 0, 'Name', 'Email', 'Phone')
		`, legOrderID, buyerID, "ORD-LEGAUTH-"+uuid.New().String()[:6])
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status)
			VALUES ($1, $2, $3, 'delivered')
		`, legFID, legOrderID, sellerID)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
			VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 1000, 5000, 5)
		`, legOIID, legOrderID, legFID, sellerID, prodID, varID)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'delivered', now() - interval '2 hours', now() - interval '1 hour', now(), now())
		`, uuid.New(), legOrderID, legFID)
		require.NoError(t, err)

		legRetID := uuid.New()
		legItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at, receiving_started_at)
			VALUES ($1, $2, $3, $4, 'receiving', 'defective', now(), now(), now())
		`, legRetID, legOrderID, legFID, buyerID)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
			VALUES ($1, $2, $3, 5, now())
		`, legItemID, legRetID, legOIID)
		require.NoError(t, err)

		legBody, _ := json.Marshal(returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 2,
			DamagedQuantity:  1,
			RejectedQuantity: 1,
		})

		// 5A: unauthenticated -> 401
		req5A := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		rr5A := httptest.NewRecorder()
		r.ServeHTTP(rr5A, req5A)
		assert.Equal(t, http.StatusUnauthorized, rr5A.Code)

		// 5B: customer -> 403
		req5B := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		req5B.Header.Set("Authorization", "Bearer "+customerToken)
		rr5B := httptest.NewRecorder()
		r.ServeHTTP(rr5B, req5B)
		assert.Equal(t, http.StatusForbidden, rr5B.Code)

		// 5C: admin with returns.read only -> 403
		req5C := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		req5C.Header.Set("Authorization", "Bearer "+adminReadToken)
		rr5C := httptest.NewRecorder()
		r.ServeHTTP(rr5C, req5C)
		assert.Equal(t, http.StatusForbidden, rr5C.Code)

		// 5D: admin with returns.update_status -> 204
		req5D := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		req5D.Header.Set("Authorization", "Bearer "+adminUpdateToken)
		rr5D := httptest.NewRecorder()
		r.ServeHTTP(rr5D, req5D)
		assert.Equal(t, http.StatusNoContent, rr5D.Code)

		// Verify legacy inspection was actually persisted in DB
		var accQty, dmgQty, rejQty int
		err = pgClient.Pool.QueryRow(ctx, "SELECT accepted_quantity, damaged_quantity, rejected_quantity FROM return_items WHERE id = $1", legItemID).Scan(&accQty, &dmgQty, &rejQty)
		require.NoError(t, err)
		assert.Equal(t, 2, accQty)
		assert.Equal(t, 1, dmgQty)
		assert.Equal(t, 1, rejQty)

		// Endpoint 6: POST /api/admin/returns/{id}/receiving/finalize (needs returns.update_status)
		// 6A: unauthenticated -> 401
		req6A := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		rr6A := httptest.NewRecorder()
		r.ServeHTTP(rr6A, req6A)
		assert.Equal(t, http.StatusUnauthorized, rr6A.Code)

		// 6B: customer -> 403
		req6B := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		req6B.Header.Set("Authorization", "Bearer "+customerToken)
		rr6B := httptest.NewRecorder()
		r.ServeHTTP(rr6B, req6B)
		assert.Equal(t, http.StatusForbidden, rr6B.Code)

		// 6C: admin with returns.read only -> 403
		req6C := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		req6C.Header.Set("Authorization", "Bearer "+adminReadToken)
		rr6C := httptest.NewRecorder()
		r.ServeHTTP(rr6C, req6C)
		assert.Equal(t, http.StatusForbidden, rr6C.Code)

		// 6D: admin with returns.update_status -> 204
		req6D := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		req6D.Header.Set("Authorization", "Bearer "+adminUpdateToken)
		rr6D := httptest.NewRecorder()
		r.ServeHTTP(rr6D, req6D)
		assert.Equal(t, http.StatusNoContent, rr6D.Code)

		// Verify return actually reached item_received in DB
		var finStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", returnID).Scan(&finStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", finStatus)
	})
}
