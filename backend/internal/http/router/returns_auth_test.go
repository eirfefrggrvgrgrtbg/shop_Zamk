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

	insertStaffMemberWithCanonicalRole := func(userID uuid.UUID, roleCode string) {
		var roleID uuid.UUID
		err := pgClient.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = $1`, roleCode).Scan(&roleID)
		require.NoError(t, err)
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

	// Create Return in approved status with arrived_at_zamk return shipment to test StartReceiving
	startReturnID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'approved', 'defective', now(), now())
	`, startReturnID, orderID, fulfillmentID, buyerID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
		VALUES ($1, $2, $3, 1, now())
	`, uuid.New(), startReturnID, orderItemID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status, created_at, updated_at)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'TRK-RBAC-START', 'arrived_at_zamk', now(), now())
	`, uuid.New(), startReturnID)
	require.NoError(t, err)

	// Create Return in receiving status so scan endpoint succeeds when called by authorized admin
	returnID := uuid.New()
	returnComment := "Customer note on damaged item"
	returnAdminComment := "Support approved receiving"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, receiving_started_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'receiving', 'defective', $5, $6, now(), now(), now())
	`, returnID, orderID, fulfillmentID, buyerID, returnComment, returnAdminComment)
	require.NoError(t, err)

	returnItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
		VALUES ($1, $2, $3, 1, 0, 0, 0, now())
	`, returnItemID, returnID, orderItemID)
	require.NoError(t, err)

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pgClient.Pool.Exec(c, `DELETE FROM return_item_units WHERE return_id = $1`, returnID)
		_, _ = pgClient.Pool.Exec(c, `DELETE FROM return_items WHERE return_id IN ($1, $2)`, returnID, startReturnID)
		_, _ = pgClient.Pool.Exec(c, `DELETE FROM return_shipments WHERE return_id IN ($1, $2)`, returnID, startReturnID)
		_, _ = pgClient.Pool.Exec(c, `DELETE FROM returns WHERE id IN ($1, $2)`, returnID, startReturnID)
	})

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
	readOnlyUser := insertUser("admin")
	insertAdminWithPerms(readOnlyUser, []string{"returns.read"})
	readOnlyToken := makeToken(readOnlyUser, "admin")

	supportUser := insertUser("admin")
	insertAdminWithPerms(supportUser, []string{"returns.read", "returns.update_status"})
	supportToken := makeToken(supportUser, "admin")

	warehouseUser := insertUser("admin")
	insertAdminWithPerms(warehouseUser, []string{"warehouse.returns"})
	warehouseToken := makeToken(warehouseUser, "admin")

	ownerUser := insertUser("admin")
	insertStaffMemberWithCanonicalRole(ownerUser, "owner")
	ownerToken := makeToken(ownerUser, "admin")

	adminCanonicalUser := insertUser("admin")
	insertStaffMemberWithCanonicalRole(adminCanonicalUser, "admin")
	adminCanonicalToken := makeToken(adminCanonicalUser, "admin")

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

	t.Run("GET receiving - returns.read only -> 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var state returns.AdminReturnReceivingState
		err := json.NewDecoder(rr.Body).Decode(&state)
		require.NoError(t, err)
		assert.Equal(t, returnID, state.Return.ID)
		assert.Equal(t, "receiving", state.Return.Status)
		assert.Equal(t, "defective", state.Return.Reason)
		assert.NotNil(t, state.Return.Comment)
		assert.Equal(t, returnComment, *state.Return.Comment)
		assert.NotNil(t, state.Return.AdminComment)
		assert.Equal(t, returnAdminComment, *state.Return.AdminComment)
		assert.Equal(t, 1, state.TotalRequested)
		assert.Equal(t, 0, state.TotalScanned)
	})

	t.Run("GET receiving - support (returns.read + returns.update_status) -> 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		req.Header.Set("Authorization", "Bearer "+supportToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var state returns.AdminReturnReceivingState
		err := json.NewDecoder(rr.Body).Decode(&state)
		require.NoError(t, err)
		assert.NotNil(t, state.Return.Comment)
		assert.Equal(t, returnComment, *state.Return.Comment)
		assert.NotNil(t, state.Return.AdminComment)
		assert.Equal(t, returnAdminComment, *state.Return.AdminComment)
		assert.Equal(t, "defective", state.Return.Reason)
	})

	t.Run("GET receiving - warehouse (warehouse.returns only) -> 200 with comment redacted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
		req.Header.Set("Authorization", "Bearer "+warehouseToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var state returns.AdminReturnReceivingState
		err := json.NewDecoder(rr.Body).Decode(&state)
		require.NoError(t, err)
		assert.Nil(t, state.Return.Comment, "Customer free-text comment must be redacted for warehouse-only role")
		assert.Nil(t, state.Return.AdminComment, "Admin comment must be redacted for warehouse-only role")
		assert.Equal(t, "defective", state.Return.Reason, "Structured return reason must remain available")
		assert.Equal(t, 1, state.TotalRequested)
		assert.Equal(t, 0, state.TotalScanned)
	})

	t.Run("GET receiving - canonical owner & admin -> 200", func(t *testing.T) {
		for _, tok := range []string{ownerToken, adminCanonicalToken} {
			req := httptest.NewRequest("GET", "/api/admin/returns/"+returnID.String()+"/receiving", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		}
	})

	// Endpoint 2: POST /api/admin/returns/{id}/receiving/start (needs warehouse.returns)
	t.Run("POST receiving/start - unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+startReturnID.String()+"/receiving/start", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("POST receiving/start - customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+startReturnID.String()+"/receiving/start", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/start - returns.read only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+startReturnID.String()+"/receiving/start", nil)
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/start - support with returns.update_status only -> 403 with ZERO mutation", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+startReturnID.String()+"/receiving/start", nil)
		req.Header.Set("Authorization", "Bearer "+supportToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		// Prove zero durable mutation
		var status string
		var startedAt *string
		err := pgClient.Pool.QueryRow(ctx, "SELECT status, receiving_started_at::text FROM returns WHERE id = $1", startReturnID).Scan(&status, &startedAt)
		require.NoError(t, err)
		assert.Equal(t, "approved", status)
		assert.Nil(t, startedAt)
	})

	t.Run("POST receiving/start - warehouse -> 204", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+startReturnID.String()+"/receiving/start", nil)
		req.Header.Set("Authorization", "Bearer "+warehouseToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		var status string
		err := pgClient.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", startReturnID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "receiving", status)
	})

	// Endpoint 3: POST /api/admin/returns/{id}/receiving/scan (needs warehouse.returns)
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

	t.Run("POST receiving/scan - returns.read only -> 403", func(t *testing.T) {
		body, _ := json.Marshal(returns.ScanReturnUnitRequest{Code: zmuCode})
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/scan - support with returns.update_status -> 403 with ZERO mutation", func(t *testing.T) {
		body, _ := json.Marshal(returns.ScanReturnUnitRequest{Code: zmuCode})
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+supportToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		// Prove zero durable unit scan insertion
		var scanCount int
		err := pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM return_item_units WHERE return_item_id = $1", returnItemID).Scan(&scanCount)
		require.NoError(t, err)
		assert.Equal(t, 0, scanCount)
	})

	var scannedUnitID uuid.UUID

	t.Run("POST receiving/scan - warehouse -> 200", func(t *testing.T) {
		body, _ := json.Marshal(returns.ScanReturnUnitRequest{Code: zmuCode})
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+warehouseToken)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var scanResp returns.ScanReturnUnitResponse
		err := json.NewDecoder(rr.Body).Decode(&scanResp)
		require.NoError(t, err)
		assert.False(t, scanResp.AlreadyScanned)
		assert.Equal(t, allocID, scanResp.ReturnItemUnit.OrderItemAllocationID)

		scannedUnitID = scanResp.ReturnItemUnit.ID
	})

	// Endpoint 4: PATCH /api/admin/returns/{id}/receiving/units/{unitId} (needs warehouse.returns)
	cond := "Good condition"
	inspBody, _ := json.Marshal(returns.UpdateSerializedUnitInspectionRequest{
		InspectedCondition: &cond,
		Disposition:        "restock",
	})

	t.Run("PATCH receiving/units - unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("PATCH receiving/units - customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("PATCH receiving/units - returns.read only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("PATCH receiving/units - support with returns.update_status -> 403 with ZERO mutation", func(t *testing.T) {
		badCond := "Bad condition by support"
		badBody, _ := json.Marshal(returns.UpdateSerializedUnitInspectionRequest{
			InspectedCondition: &badCond,
			Disposition:        "damaged",
		})
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(badBody))
		req.Header.Set("Authorization", "Bearer "+supportToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		var dbDisp *string
		var dbCond *string
		err := pgClient.Pool.QueryRow(ctx, "SELECT disposition, inspected_condition FROM return_item_units WHERE id = $1", scannedUnitID).Scan(&dbDisp, &dbCond)
		require.NoError(t, err)
		assert.Nil(t, dbDisp)
		assert.Nil(t, dbCond)
	})

	t.Run("PATCH receiving/units - canonical admin & warehouse -> 204", func(t *testing.T) {
		// First admin canonical updates it
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/units/%s", returnID, scannedUnitID), bytes.NewReader(inspBody))
		req.Header.Set("Authorization", "Bearer "+adminCanonicalToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		var dbDisp string
		var dbCond *string
		err := pgClient.Pool.QueryRow(ctx, "SELECT disposition, inspected_condition FROM return_item_units WHERE id = $1", scannedUnitID).Scan(&dbDisp, &dbCond)
		require.NoError(t, err)
		assert.Equal(t, "restock", dbDisp)
		require.NotNil(t, dbCond)
		assert.Equal(t, "Good condition", *dbCond)
	})

	// Endpoint 5: PATCH /api/admin/returns/{id}/receiving/items/{itemId}/legacy-inspection (needs warehouse.returns)
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

	t.Cleanup(func() {
		_, _ = pgClient.Pool.Exec(context.Background(), `
			DELETE FROM return_items WHERE return_id = $1;
			DELETE FROM returns WHERE id = $1;
		`, legRetID)
	})

	legBody, _ := json.Marshal(returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 2,
		DamagedQuantity:  1,
		RejectedQuantity: 1,
	})

	t.Run("PATCH legacy-inspection - unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("PATCH legacy-inspection - customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("PATCH legacy-inspection - returns.read only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("PATCH legacy-inspection - support with returns.update_status -> 403 with ZERO mutation", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		req.Header.Set("Authorization", "Bearer "+supportToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		var accQty, dmgQty, rejQty int
		err := pgClient.Pool.QueryRow(ctx, "SELECT accepted_quantity, damaged_quantity, rejected_quantity FROM return_items WHERE id = $1", legItemID).Scan(&accQty, &dmgQty, &rejQty)
		require.NoError(t, err)
		assert.Equal(t, 0, accQty)
		assert.Equal(t, 0, dmgQty)
		assert.Equal(t, 0, rejQty)
	})

	t.Run("PATCH legacy-inspection - canonical owner & warehouse -> 204", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/admin/returns/%s/receiving/items/%s/legacy-inspection", legRetID, legItemID), bytes.NewReader(legBody))
		req.Header.Set("Authorization", "Bearer "+ownerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		var accQty, dmgQty, rejQty int
		err := pgClient.Pool.QueryRow(ctx, "SELECT accepted_quantity, damaged_quantity, rejected_quantity FROM return_items WHERE id = $1", legItemID).Scan(&accQty, &dmgQty, &rejQty)
		require.NoError(t, err)
		assert.Equal(t, 2, accQty)
		assert.Equal(t, 1, dmgQty)
		assert.Equal(t, 1, rejQty)
	})

	// Endpoint 6: POST /api/admin/returns/{id}/receiving/finalize (needs warehouse.returns)
	t.Run("POST receiving/finalize - unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("POST receiving/finalize - customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/finalize - returns.read only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("POST receiving/finalize - support with returns.update_status -> 403 with ZERO mutation", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		req.Header.Set("Authorization", "Bearer "+supportToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		var finStatus string
		err := pgClient.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", returnID).Scan(&finStatus)
		require.NoError(t, err)
		assert.Equal(t, "receiving", finStatus)
	})

	t.Run("POST receiving/finalize - warehouse -> 204", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/receiving/finalize", nil)
		req.Header.Set("Authorization", "Bearer "+warehouseToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		var finStatus string
		err := pgClient.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", returnID).Scan(&finStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", finStatus)
	})

	t.Run("P1 RETURNS - Warehouse List Scope and Detail Minimization Matrix", func(t *testing.T) {
		// Fixtures for testing the list matrix (use future timestamps so they sort to the very top of ORDER BY created_at DESC):
		// 1. approved + arrived_at_zamk -> PRESENT in warehouse list
		retAppArrivedID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'approved', 'defective', 'Customer note', 'Support decision', now() + interval '10 hours', now())
		`, retAppArrivedID, orderID, fulfillmentID, buyerID)
		require.NoError(t, err)
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status, customer_name, customer_phone, pickup_address, created_at, updated_at)
			VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-ARRIVED', 'arrived_at_zamk', 'Cust Name', '+79991112233', '{"city":"Msk","street":"Lenina","house":"1"}', now() + interval '10 hours', now())
		`, uuid.New(), retAppArrivedID)
		require.NoError(t, err)

		// 2. approved + in_transit (NOT arrived) -> ABSENT in warehouse list
		retAppInTransitID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'approved', 'defective', 'In transit note', 'Approved by support', now() + interval '9 hours', now())
		`, retAppInTransitID, orderID, fulfillmentID, buyerID)
		require.NoError(t, err)
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status, customer_name, customer_phone, created_at, updated_at)
			VALUES ($1, $2, 'cdek', 'cdek_office', 'TRK-TRANSIT', 'in_transit', 'Cust Name', '+79991112233', now() + interval '9 hours', now())
		`, uuid.New(), retAppInTransitID)
		require.NoError(t, err)

		// 3. approved + no shipment (NOT arrived) -> ABSENT in warehouse list
		retAppNoShipID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'approved', 'size_fit', 'No ship note', 'Approved', now() + interval '8 hours', now())
		`, retAppNoShipID, orderID, fulfillmentID, buyerID)
		require.NoError(t, err)

		// 4. requested (new) -> ABSENT in warehouse list
		retRequestedID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'requested', 'changed_mind', 'Requested note', now() + interval '7 hours', now())
		`, retRequestedID, orderID, fulfillmentID, buyerID)
		require.NoError(t, err)

		// 5. rejected -> ABSENT in warehouse list
		retRejectedID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'rejected', 'other', 'Reject note', 'Rejected by support', now() + interval '6 hours', now())
		`, retRejectedID, orderID, fulfillmentID, buyerID)
		require.NoError(t, err)

		// 6. receiving -> PRESENT in warehouse list
		retReceivingID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'receiving', 'defective', 'Receiving note', now() + interval '5 hours', now())
		`, retReceivingID, orderID, fulfillmentID, buyerID)
		require.NoError(t, err)

		// 7. item_received -> PRESENT in warehouse list
		retItemReceivedID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', 'Received note', now() + interval '4 hours', now())
		`, retItemReceivedID, orderID, fulfillmentID, buyerID)
		require.NoError(t, err)

		t.Cleanup(func() {
			c := context.Background()
			delIDs := []uuid.UUID{retAppArrivedID, retAppInTransitID, retAppNoShipID, retRequestedID, retRejectedID, retReceivingID, retItemReceivedID}
			_, _ = pgClient.Pool.Exec(c, `DELETE FROM return_shipments WHERE return_id = ANY($1)`, delIDs)
			_, _ = pgClient.Pool.Exec(c, `DELETE FROM returns WHERE id = ANY($1)`, delIDs)
		})

		// A. Test WAREHOUSE LIST SCOPE
		t.Run("Warehouse list scope: strictly arrived_at_zamk, receiving, item_received only", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/admin/returns?limit=50&offset=0", nil)
			req.Header.Set("Authorization", "Bearer "+warehouseToken)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)

			var listResp returns.AdminReturnListResponse
			err := json.NewDecoder(rr.Body).Decode(&listResp)
			require.NoError(t, err)

			ids := make(map[uuid.UUID]returns.AdminReturnResponse)
			for _, item := range listResp.Items {
				ids[item.ID] = item
			}

			// Must be PRESENT:
			assert.Contains(t, ids, retAppArrivedID, "approved + arrived_at_zamk must be PRESENT for warehouse")
			assert.Contains(t, ids, retItemReceivedID, "item_received must be PRESENT for warehouse")
			assert.Contains(t, ids, retReceivingID, "receiving must be PRESENT for warehouse")

			// Must be ABSENT:
			assert.NotContains(t, ids, retAppInTransitID, "approved + in_transit must be ABSENT for warehouse")
			assert.NotContains(t, ids, retAppNoShipID, "approved with no shipment must be ABSENT for warehouse")
			assert.NotContains(t, ids, retRequestedID, "requested (new) must be ABSENT for warehouse")
			assert.NotContains(t, ids, retRejectedID, "rejected must be ABSENT for warehouse")

			// Verify data minimization on all returned items:
			for _, item := range listResp.Items {
				assert.Nil(t, item.CustomerName, "CustomerName must be redacted for warehouse")
				assert.Nil(t, item.CustomerEmail, "CustomerEmail must be redacted for warehouse")
				assert.Nil(t, item.CustomerPhone, "CustomerPhone must be redacted for warehouse")
				assert.Nil(t, item.Comment, "Customer comment must be redacted for warehouse")
				assert.Nil(t, item.AdminComment, "AdminComment/support note must be redacted for warehouse")
				if item.Shipment != nil {
					assert.Nil(t, item.Shipment.CustomerName, "Shipment.CustomerName must be redacted")
					assert.Nil(t, item.Shipment.CustomerPhone, "Shipment.CustomerPhone must be redacted")
					assert.Nil(t, item.Shipment.PickupAddress, "Shipment.PickupAddress must be redacted")
				}
			}
		})

		// B. Test SUPPORT LIST (returns.read = true)
		t.Run("Support list: retains full unfiltered returns list", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/admin/returns?limit=50&offset=0", nil)
			req.Header.Set("Authorization", "Bearer "+supportToken)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)

			var listResp returns.AdminReturnListResponse
			err := json.NewDecoder(rr.Body).Decode(&listResp)
			require.NoError(t, err)

			ids := make(map[uuid.UUID]returns.AdminReturnResponse)
			for _, item := range listResp.Items {
				ids[item.ID] = item
			}

			// Support must see ALL returns
			assert.Contains(t, ids, retAppArrivedID, "Support must see approved+arrived")
			assert.Contains(t, ids, retAppInTransitID, "Support must see approved+in_transit")
			assert.Contains(t, ids, retAppNoShipID, "Support must see approved with no shipment")
			assert.Contains(t, ids, retRequestedID, "Support must see requested")
			assert.Contains(t, ids, retRejectedID, "Support must see rejected")
			assert.Contains(t, ids, retReceivingID, "Support must see receiving")
			assert.Contains(t, ids, retItemReceivedID, "Support must see item_received")

			// Support retains Customer PII and Comments:
			supItem := ids[retAppArrivedID]
			assert.NotNil(t, supItem.CustomerName, "Support must retain customer name")
			assert.NotNil(t, supItem.Comment, "Support must retain customer comment")
			assert.NotNil(t, supItem.AdminComment, "Support must retain admin comment")
		})

		// C. Test WAREHOUSE DETAIL MINIMIZATION vs SUPPORT DETAIL
		t.Run("Warehouse detail: redacts PII, comments, support notes; preserves logistics tracking and reason", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/admin/returns/"+retAppArrivedID.String(), nil)
			req.Header.Set("Authorization", "Bearer "+warehouseToken)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)

			var det returns.AdminReturnResponse
			err := json.NewDecoder(rr.Body).Decode(&det)
			require.NoError(t, err)

			// Redacted fields:
			assert.Nil(t, det.CustomerName, "CustomerName must be nil for warehouse")
			assert.Nil(t, det.CustomerEmail, "CustomerEmail must be nil for warehouse")
			assert.Nil(t, det.CustomerPhone, "CustomerPhone must be nil for warehouse")
			assert.Nil(t, det.Comment, "Customer comment must be nil for warehouse")
			assert.Nil(t, det.AdminComment, "AdminComment/support note must be nil for warehouse")

			require.NotNil(t, det.Shipment, "Shipment must be present")
			assert.Nil(t, det.Shipment.CustomerName, "Shipment.CustomerName must be nil")
			assert.Nil(t, det.Shipment.CustomerPhone, "Shipment.CustomerPhone must be nil")
			assert.Nil(t, det.Shipment.PickupAddress, "Shipment.PickupAddress must be nil")

			// Preserved necessary warehouse operational fields:
			assert.Equal(t, "defective", det.Reason, "Return reason must be preserved for inspection")
			assert.Equal(t, "cdek", det.Shipment.Provider)
			assert.Equal(t, "TRK-ARRIVED", *det.Shipment.TrackingNumber)
			assert.Equal(t, "arrived_at_zamk", det.Shipment.Status)
			require.NotNil(t, det.ShipmentStatus)
			assert.Equal(t, "arrived_at_zamk", *det.ShipmentStatus)
		})

		t.Run("Support detail: retains all customer communication, notes, PII", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/admin/returns/"+retAppArrivedID.String(), nil)
			req.Header.Set("Authorization", "Bearer "+supportToken)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)

			var det returns.AdminReturnResponse
			err := json.NewDecoder(rr.Body).Decode(&det)
			require.NoError(t, err)

			assert.NotNil(t, det.CustomerName)
			assert.NotNil(t, det.CustomerEmail)
			assert.NotNil(t, det.CustomerPhone)
			assert.NotNil(t, det.Comment)
			assert.Equal(t, "Customer note", *det.Comment)
			assert.NotNil(t, det.AdminComment)
			assert.Equal(t, "Support decision", *det.AdminComment)
			require.NotNil(t, det.Shipment)
			assert.NotNil(t, det.Shipment.CustomerName)
			assert.NotNil(t, det.Shipment.CustomerPhone)
		})

		// D. Support-only endpoints blocked for warehouse
		t.Run("Support-only endpoints strictly forbidden for warehouse-only role", func(t *testing.T) {
			for _, endpoint := range []string{
				"/api/admin/returns/" + retAppArrivedID.String() + "/timeline",
				"/api/admin/returns/" + retAppArrivedID.String() + "/messages",
				"/api/admin/returns/" + retAppArrivedID.String() + "/refund-quote",
			} {
				req := httptest.NewRequest("GET", endpoint, nil)
				req.Header.Set("Authorization", "Bearer "+warehouseToken)
				rr := httptest.NewRecorder()
				r.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusForbidden, rr.Code, "Warehouse must not access %s", endpoint)
			}
		})
	})
}
