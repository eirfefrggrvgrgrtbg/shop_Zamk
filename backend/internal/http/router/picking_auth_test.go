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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestAdminPickingScanRouter(t *testing.T) {
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
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'PickingRole')`, roleID, code)
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

	// Setup seller, product, variant, order, fulfillmen
	sellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Picking Seller', $2, $3, 'active', NOW(), NOW())
	`, sellerID, "seller-"+sellerID.String()[:8], sellerID.String()+"@seller.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 1000, 'published', now(), now())`, prodID, sellerID, catID, uuid.New().String())
	require.NoError(t, err)

	variantID := uuid.New()
	barcode := "BARCODE-" + variantID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())`, variantID, prodID, "SKU-"+variantID.String()[:8], "SSKU-"+variantID.String()[:8], barcode)
	require.NoError(t, err)

	buyerID := insertUser("customer")
	orderID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'paid', 1000, 'Buyer', 'Phone', 'Email', 'Addr')
	`, orderID, buyerID)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'paid', 1000, 900, 900)
	`, fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	itemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Prod Item', 'prod-slug', 100, 1, 100, $6, 0)
	`, itemID, orderID, prodID, variantID, sellerID, fulfillmentID)
	require.NoError(t, err)

	// Create supply and uni
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`, supplyID, sellerID, uuid.New().String()[:8])
	require.NoError(t, err)
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitID := uuid.New()
	unitCode := "ZMU-ROUTER-" + uuid.New().String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'warehouse')`, unitID, unitCode, variantID, supplyID, supplyItemID)
	require.NoError(t, err)

	allocID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at) VALUES ($1, $2, $3, NULL)`, allocID, itemID, unitID)
	require.NoError(t, err)

	adminID := insertUser("admin")
	insertAdminWithPerms(adminID, []string{"orders.read"})
	adminTok := makeToken(adminID, "admin")

	type errResponse struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	// 1. Unauthenticated -> 401
	t.Run("unauthenticated -> 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"code": unitCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	// 2. Customer user -> 403
	t.Run("customer -> 403", func(t *testing.T) {
		custTok := makeToken(buyerID, "customer")
		body, _ := json.Marshal(map[string]string{"code": unitCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+custTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 3. Admin without orders.read -> 403
	t.Run("admin without orders.read -> 403", func(t *testing.T) {
		adminNoPerm := insertUser("admin")
		insertAdminWithPerms(adminNoPerm, []string{"inventory.read"})
		noPermTok := makeToken(adminNoPerm, "admin")

		body, _ := json.Marshal(map[string]string{"code": unitCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+noPermTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 4. Domain error: unknown code -> 404 picking_code_not_found
	t.Run("unknown code -> 404 picking_code_not_found", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"code": "NON_EXISTENT_CODE_12345"})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "picking_code_not_found", res.Error.Code)
	})

	// 5. Domain error: cannot pick serialized with barcode -> 409 cannot_pick_serialized_with_barcode
	t.Run("serialized with barcode -> 409 cannot_pick_serialized_with_barcode", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"code": barcode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "cannot_pick_serialized_with_barcode", res.Error.Code)
	})

	// 6. Domain error: picking_not_allowed on cancelled order
	t.Run("picking_not_allowed -> 409", func(t *testing.T) {
		cancOrderID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
			VALUES ($1, $2, 'cancelled', 1000, 'Buyer', 'Phone', 'Email', 'Addr')
		`, cancOrderID, buyerID)
		require.NoError(t, err)

		cancFulfID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'paid', 1000, 900, 900)
		`, cancFulfID, cancOrderID, sellerID)
		require.NoError(t, err)

		body, _ := json.Marshal(map[string]string{"code": unitCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+cancFulfID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "picking_not_allowed", res.Error.Code)
	})

	// 7. Domain error: unit_not_allocated_to_fulfillment -> 409
	t.Run("unallocated unit -> 409 unit_not_allocated_to_fulfillment", func(t *testing.T) {
		unallocSupplyID := uuid.New()
		unallocSupplyItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`, unallocSupplyID, sellerID, uuid.New().String()[:8])
		require.NoError(t, err)
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())`, unallocSupplyItemID, unallocSupplyID, variantID)
		require.NoError(t, err)

		unallocUnitID := uuid.New()
		unallocCode := "ZMU-UNALLOC-" + unallocUnitID.String()[:8]
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'warehouse')`, unallocUnitID, unallocCode, variantID, unallocSupplyID, unallocSupplyItemID)
		require.NoError(t, err)

		body, _ := json.Marshal(map[string]string{"code": unallocCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "unit_not_allocated_to_fulfillment", res.Error.Code)
	})

	// 8. Domain error: unit_allocated_to_other_order -> 409
	t.Run("foreign unit -> 409 unit_allocated_to_other_order", func(t *testing.T) {
		otherOrderID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
			VALUES ($1, $2, 'paid', 1000, 'Buyer', 'Phone', 'Email', 'Addr')
		`, otherOrderID, buyerID)
		require.NoError(t, err)

		otherFulfID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'paid', 1000, 900, 900)
		`, otherFulfID, otherOrderID, sellerID)
		require.NoError(t, err)

		body, _ := json.Marshal(map[string]string{"code": unitCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+otherFulfID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "unit_allocated_to_other_order", res.Error.Code)
	})

	// 9. Domain error: unit_not_in_warehouse -> 409
	t.Run("non-warehouse unit -> 409 unit_not_in_warehouse", func(t *testing.T) {
		damOrderID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
			VALUES ($1, $2, 'paid', 1000, 'Buyer', 'Phone', 'Email', 'Addr')
		`, damOrderID, buyerID)
		require.NoError(t, err)

		damFulfID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'paid', 1000, 900, 900)
		`, damFulfID, damOrderID, sellerID)
		require.NoError(t, err)

		damItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
			VALUES ($1, $2, $3, $4, $5, 'Dam Item', 'prod-slug', 100, 1, 100, $6, 0)
		`, damItemID, damOrderID, prodID, variantID, sellerID, damFulfID)
		require.NoError(t, err)

		damagedUnitID := uuid.New()
		damagedCode := "ZMU-DAMAGED-" + damagedUnitID.String()[:8]
		damagedSupplyID := uuid.New()
		damagedSupplyItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`, damagedSupplyID, sellerID, uuid.New().String()[:8])
		require.NoError(t, err)
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())`, damagedSupplyItemID, damagedSupplyID, variantID)
		require.NoError(t, err)
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'damaged')`, damagedUnitID, damagedCode, variantID, damagedSupplyID, damagedSupplyItemID)
		require.NoError(t, err)

		damagedAllocID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at) VALUES ($1, $2, $3, NULL)`, damagedAllocID, damItemID, damagedUnitID)
		require.NoError(t, err)

		body, _ := json.Marshal(map[string]string{"code": damagedCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+damFulfID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "unit_not_in_warehouse", res.Error.Code)
	})

	// 10. Domain error: ambiguous_picking_code -> 409
	t.Run("ambiguous legacy code -> 409 ambiguous_picking_code and keeps paid state", func(t *testing.T) {
		ambigOrderID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
			VALUES ($1, $2, 'paid', 1000, 'Buyer', 'Phone', 'Email', 'Addr')
		`, ambigOrderID, buyerID)
		require.NoError(t, err)

		ambigFulfID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'paid', 1000, 900, 900)
		`, ambigFulfID, ambigOrderID, sellerID)
		require.NoError(t, err)

		ambigVariantID := uuid.New()
		ambigBarcode := "BARCODE-AMBIG-" + ambigVariantID.String()[:8]
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())`, ambigVariantID, prodID, "SKU-AMBIG-"+ambigVariantID.String()[:8], "SSKU-AMBIG-"+ambigVariantID.String()[:8], ambigBarcode)
		require.NoError(t, err)

		ambigItemID1 := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
			VALUES ($1, $2, $3, $4, $5, 'Prod Item 1', 'prod-slug', 100, 1, 100, $6, 0)
		`, ambigItemID1, ambigOrderID, prodID, ambigVariantID, sellerID, ambigFulfID)
		require.NoError(t, err)

		ambigItemID2 := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
			VALUES ($1, $2, $3, $4, $5, 'Prod Item 2', 'prod-slug', 100, 1, 100, $6, 0)
		`, ambigItemID2, ambigOrderID, prodID, ambigVariantID, sellerID, ambigFulfID)
		require.NoError(t, err)

		body, _ := json.Marshal(map[string]string{"code": ambigBarcode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+ambigFulfID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res errResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "ambiguous_picking_code", res.Error.Code)

		// Assert paid state and quantities unchanged
		var oStatus, fStatus string
		_ = pgClient.Pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, ambigOrderID).Scan(&oStatus)
		_ = pgClient.Pool.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, ambigFulfID).Scan(&fStatus)
		assert.Equal(t, "paid", oStatus, "order status must remain paid")
		assert.Equal(t, "paid", fStatus, "fulfillment status must remain paid")

		var p1, p2 int
		_ = pgClient.Pool.QueryRow(ctx, `SELECT picked_quantity FROM order_items WHERE id = $1`, ambigItemID1).Scan(&p1)
		_ = pgClient.Pool.QueryRow(ctx, `SELECT picked_quantity FROM order_items WHERE id = $1`, ambigItemID2).Scan(&p2)
		assert.Equal(t, 0, p1, "item 1 picked quantity unchanged")
		assert.Equal(t, 0, p2, "item 2 picked quantity unchanged")
	})

	// 11. Valid admin request -> 200 with PickingScanResult
	t.Run("valid admin scan -> 200", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"code": unitCode})
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res fulfillment.PickingScanResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, fulfillmentID, res.FulfillmentID)
		assert.Equal(t, "serialized", res.ScanResult.Type)
		assert.True(t, res.ScanResult.NewlyPicked)
		assert.Equal(t, 1, res.Item.PickedQuantity)
		assert.True(t, res.FulfillmentProgress.IsComplete)
	})
}
