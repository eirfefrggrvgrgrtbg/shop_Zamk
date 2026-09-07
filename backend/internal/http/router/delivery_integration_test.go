package router_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"

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

type fixtureCleaner struct {
	client         *postgres.Client
	refundIDs      []uuid.UUID
	returnItemIDs  []uuid.UUID
	returnIDs      []uuid.UUID
	paymentIDs     []uuid.UUID
	ledgerItemIDs  []uuid.UUID
	shipmentIDs    []uuid.UUID
	orderItemIDs   []uuid.UUID
	fulfillmentIDs []uuid.UUID
	variantIDs     []uuid.UUID
	productIDs     []uuid.UUID
	categoryIDs    []uuid.UUID
	orderIDs       []uuid.UUID
	roleIDs        []uuid.UUID
	sellerIDs      []uuid.UUID
	userIDs        []uuid.UUID
}

func (fc *fixtureCleaner) execDelete(t *testing.T, ctx context.Context, entityName, query string, ids []uuid.UUID) {
	if len(ids) == 0 {
		return
	}
	_, err := fc.client.Pool.Exec(ctx, query, ids)
	if err != nil {
		t.Errorf("cleanup: failed to delete %s (ids: %v): %v", entityName, ids, err)
	}
}

func (fc *fixtureCleaner) Clean(t *testing.T, ctx context.Context) {
	// Delete strictly in reverse foreign-key dependency order
	// 1. Refunds (references returns, orders, payments)
	fc.execDelete(t, ctx, "refunds", "DELETE FROM refunds WHERE id = ANY($1)", fc.refundIDs)

	// 2. Return items (references returns, order_items)
	fc.execDelete(t, ctx, "return_items", "DELETE FROM return_items WHERE id = ANY($1)", fc.returnItemIDs)

	// 3. Returns (references orders, order_fulfillments, users)
	fc.execDelete(t, ctx, "returns", "DELETE FROM returns WHERE id = ANY($1)", fc.returnIDs)

	// 4. Seller ledger entries (references order_items, orders, sellers)
	fc.execDelete(t, ctx, "seller_ledger_entries", "DELETE FROM seller_ledger_entries WHERE order_item_id = ANY($1)", fc.ledgerItemIDs)

	// 5. Shipment events (references shipments)
	fc.execDelete(t, ctx, "shipment_events", "DELETE FROM shipment_events WHERE shipment_id = ANY($1)", fc.shipmentIDs)

	// 6. Shipments (references orders, order_fulfillments)
	fc.execDelete(t, ctx, "shipments", "DELETE FROM shipments WHERE id = ANY($1)", fc.shipmentIDs)

	// 7. Order items (references orders, order_fulfillments, products, product_variants, sellers)
	fc.execDelete(t, ctx, "order_items", "DELETE FROM order_items WHERE id = ANY($1)", fc.orderItemIDs)

	// 8. Order fulfillments (references orders, sellers)
	fc.execDelete(t, ctx, "order_fulfillments", "DELETE FROM order_fulfillments WHERE id = ANY($1)", fc.fulfillmentIDs)

	// 9. Order status history (references orders)
	fc.execDelete(t, ctx, "order_status_history", "DELETE FROM order_status_history WHERE order_id = ANY($1)", fc.orderIDs)

	// 10. Payments (references orders)
	fc.execDelete(t, ctx, "payments", "DELETE FROM payments WHERE id = ANY($1)", fc.paymentIDs)

	// 11. Orders (references users)
	fc.execDelete(t, ctx, "orders", "DELETE FROM orders WHERE id = ANY($1)", fc.orderIDs)

	// 12. Product variants (references products)
	fc.execDelete(t, ctx, "product_variants", "DELETE FROM product_variants WHERE id = ANY($1)", fc.variantIDs)

	// 13. Products (references categories, sellers)
	fc.execDelete(t, ctx, "products", "DELETE FROM products WHERE id = ANY($1)", fc.productIDs)

	// 14. Categories
	fc.execDelete(t, ctx, "categories", "DELETE FROM categories WHERE id = ANY($1)", fc.categoryIDs)

	// 15. Staff RBAC (members -> permissions -> roles)
	if len(fc.roleIDs) > 0 {
		_, err := fc.client.Pool.Exec(ctx, "DELETE FROM staff_members WHERE staff_role_id = ANY($1)", fc.roleIDs)
		if err != nil {
			t.Errorf("cleanup: failed to delete staff_members: %v", err)
		}
		_, err = fc.client.Pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = ANY($1)", fc.roleIDs)
		if err != nil {
			t.Errorf("cleanup: failed to delete staff_role_permissions: %v", err)
		}
		_, err = fc.client.Pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = ANY($1)", fc.roleIDs)
		if err != nil {
			t.Errorf("cleanup: failed to delete staff_roles: %v", err)
		}
	}

	// 16. Notifications (references users, sellers)
	if len(fc.userIDs) > 0 || len(fc.sellerIDs) > 0 {
		_, err := fc.client.Pool.Exec(ctx, "DELETE FROM notifications WHERE recipient_user_id = ANY($1) OR recipient_seller_id = ANY($2)", fc.userIDs, fc.sellerIDs)
		if err != nil {
			t.Errorf("cleanup: failed to delete notifications: %v", err)
		}
	}

	// 17. Sellers
	fc.execDelete(t, ctx, "sellers", "DELETE FROM sellers WHERE id = ANY($1)", fc.sellerIDs)

	// 18. Users
	fc.execDelete(t, ctx, "users", "DELETE FROM users WHERE id = ANY($1)", fc.userIDs)
}

func (fc *fixtureCleaner) VerifyClean(t *testing.T, ctx context.Context) {
	// 1. Verify temporary failure constraint is absent
	var constraintCount int
	err := fc.client.Pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint
		WHERE conname = 'check_fail_seller_earning'
	`).Scan(&constraintCount)
	if err != nil {
		t.Errorf("verification: failed to query pg_constraint: %v", err)
	} else if constraintCount != 0 {
		t.Errorf("verification: expected temporary constraint check_fail_seller_earning to be absent, found %d", constraintCount)
	}

	// 2. Verify tracked fixture records are completely gone
	verifyZero := func(entityName, query string, ids []uuid.UUID) {
		if len(ids) == 0 {
			return
		}
		var count int
		err := fc.client.Pool.QueryRow(ctx, query, ids).Scan(&count)
		if err != nil {
			t.Errorf("verification: failed to check leftovers for %s: %v", entityName, err)
		} else if count != 0 {
			t.Errorf("verification: expected 0 leftovers for %s, found %d", entityName, count)
		}
	}

	verifyZero("refunds", "SELECT count(*) FROM refunds WHERE id = ANY($1)", fc.refundIDs)
	verifyZero("return_items", "SELECT count(*) FROM return_items WHERE id = ANY($1)", fc.returnItemIDs)
	verifyZero("returns", "SELECT count(*) FROM returns WHERE id = ANY($1)", fc.returnIDs)
	verifyZero("seller_ledger_entries", "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = ANY($1)", fc.ledgerItemIDs)
	verifyZero("shipments", "SELECT count(*) FROM shipments WHERE id = ANY($1)", fc.shipmentIDs)
	verifyZero("order_items", "SELECT count(*) FROM order_items WHERE id = ANY($1)", fc.orderItemIDs)
	verifyZero("order_fulfillments", "SELECT count(*) FROM order_fulfillments WHERE id = ANY($1)", fc.fulfillmentIDs)
	verifyZero("payments", "SELECT count(*) FROM payments WHERE id = ANY($1)", fc.paymentIDs)
	verifyZero("orders", "SELECT count(*) FROM orders WHERE id = ANY($1)", fc.orderIDs)
	verifyZero("product_variants", "SELECT count(*) FROM product_variants WHERE id = ANY($1)", fc.variantIDs)
	verifyZero("products", "SELECT count(*) FROM products WHERE id = ANY($1)", fc.productIDs)
	verifyZero("categories", "SELECT count(*) FROM categories WHERE id = ANY($1)", fc.categoryIDs)
	verifyZero("staff_roles", "SELECT count(*) FROM staff_roles WHERE id = ANY($1)", fc.roleIDs)
	verifyZero("sellers", "SELECT count(*) FROM sellers WHERE id = ANY($1)", fc.sellerIDs)
	verifyZero("users", "SELECT count(*) FROM users WHERE id = ANY($1)", fc.userIDs)
}

func TestDeliveryPayoutsIntegration(t *testing.T) {
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

	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	// 1. Test DB safety guard before any fixture writes
	testutil.AssertTestDatabase(t, pgClient.Pool)

	cleaner := &fixtureCleaner{client: pgClient}

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)

	// Coordinated, ordered teardown via t.Cleanup:
	// - Stop background workers / router cancel first
	// - Drop failure constraint if left behind
	// - Clean tracked fixtures while PostgreSQL connection pool is still open
	// - Verify tracked fixtures are gone and constraint is absent
	// - Close Redis and PostgreSQL clients last
	t.Cleanup(func() {
		if cancel != nil {
			cancel()
		}

		_, dropErr := pgClient.Pool.Exec(context.Background(), "ALTER TABLE seller_ledger_entries DROP CONSTRAINT IF EXISTS check_fail_seller_earning")
		if dropErr != nil {
			t.Errorf("cleanup: failed to drop temporary constraint check_fail_seller_earning: %v", dropErr)
		}

		cleaner.Clean(t, context.Background())
		cleaner.VerifyClean(t, context.Background())

		if redisClient != nil {
			_ = redisClient.Close()
		}
		if pgClient != nil {
			pgClient.Close()
		}
	})

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	insertAdminWithPerms := func(userID uuid.UUID, perms []string) uuid.UUID {
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'DeliveryRole')`, roleID, code)
		require.NoError(t, err)
		for _, p := range perms {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
			require.NoError(t, err)
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		require.NoError(t, err)
		cleaner.roleIDs = append(cleaner.roleIDs, roleID)
		return roleID
	}

	insertUser := func(role string) (uuid.UUID, string) {
		id := uuid.New()
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test Admin', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		require.NoError(t, err)
		cleaner.userIDs = append(cleaner.userIDs, id)
		token, _ := tokenService.GenerateAccessToken(id, id.String()+"@test.com", role)
		return id, token
	}

	adminID, adminToken := insertUser("admin")
	insertAdminWithPerms(adminID, []string{"shipments.update_status", "refunds.create", "refunds.read"})

	t.Run("canonical_DeliverShipment_creates_expected_ledger_entries_and_isolates_sellers", func(t *testing.T) {
		sellerID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Brand', $3, $2, 'active', now(), now())", sellerID, uuid.New().String()+"@seller.com", uuid.New().String())
		require.NoError(t, err)
		cleaner.sellerIDs = append(cleaner.sellerIDs, sellerID)

		sellerB := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Brand B', $3, $2, 'active', now(), now())", sellerB, uuid.New().String()+"@seller.com", uuid.New().String())
		require.NoError(t, err)
		cleaner.sellerIDs = append(cleaner.sellerIDs, sellerB)

		customerID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'Customer', 'hash', 'customer', 'active', now(), now())", customerID, uuid.New().String()+"@customer.com", "7999"+uuid.New().String()[:7])
		require.NoError(t, err)
		cleaner.userIDs = append(cleaner.userIDs, customerID)

		orderID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'shipped', 20000, 'N', 'P', 'E', 'A', now(), now())", orderID, customerID)
		require.NoError(t, err)
		cleaner.orderIDs = append(cleaner.orderIDs, orderID)

		fulfillmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', now(), now())", fulfillmentID, orderID, sellerID)
		require.NoError(t, err)
		cleaner.fulfillmentIDs = append(cleaner.fulfillmentIDs, fulfillmentID)

		fulfillmentB := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', now(), now())", fulfillmentB, orderID, sellerB)
		require.NoError(t, err)
		cleaner.fulfillmentIDs = append(cleaner.fulfillmentIDs, fulfillmentB)

		catID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())", catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.categoryIDs = append(cleaner.categoryIDs, catID)

		productID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 'Desc', 5000, 'published', now(), now())", productID, sellerID, catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.productIDs = append(cleaner.productIDs, productID)

		productB := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod B', $4, 'Desc', 5000, 'published', now(), now())", productB, sellerB, catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.productIDs = append(cleaner.productIDs, productB)

		variantID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, 5000, true, now(), now())", variantID, productID, uuid.New().String())
		require.NoError(t, err)
		cleaner.variantIDs = append(cleaner.variantIDs, variantID)

		variantB := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, 5000, true, now(), now())", variantB, productB, uuid.New().String())
		require.NoError(t, err)
		cleaner.variantIDs = append(cleaner.variantIDs, variantB)

		orderItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 5000, 2, 10000)", orderItemID, orderID, fulfillmentID, sellerID, productID, variantID)
		require.NoError(t, err)
		cleaner.orderItemIDs = append(cleaner.orderItemIDs, orderItemID)
		cleaner.ledgerItemIDs = append(cleaner.ledgerItemIDs, orderItemID)

		itemB := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, $6, 'Title B', 'slug-b', 5000, 2, 10000)", itemB, orderID, fulfillmentB, sellerB, productB, variantB)
		require.NoError(t, err)
		cleaner.orderItemIDs = append(cleaner.orderItemIDs, itemB)
		cleaner.ledgerItemIDs = append(cleaner.ledgerItemIDs, itemB)

		shipmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO shipments (id, order_id, fulfillment_id, status, tracking_number, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', 'TRK123', now(), now())", shipmentID, orderID, fulfillmentID)
		require.NoError(t, err)
		cleaner.shipmentIDs = append(cleaner.shipmentIDs, shipmentID)

		// 2. Perform Delivery via HTTP
		payload := `{"comment": "test delivery"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		// 3. Verify ledger entries (10000 kopecks = 100 RUB gross)
		var earningCents, commCents, grossCents int64
		err = pgClient.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'", orderItemID).Scan(&earningCents)
		require.NoError(t, err)
		err = pgClient.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'zamk_commission'", orderItemID).Scan(&commCents)
		require.NoError(t, err)
		err = pgClient.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'sale_gross'", orderItemID).Scan(&grossCents)
		require.NoError(t, err)

		assert.Equal(t, int64(10000), grossCents, "gross must be 10000 kopecks (100 RUB)")
		assert.Equal(t, int64(-900), commCents, "commission must be -900 kopecks (-9 RUB, 9%)")
		assert.Equal(t, int64(9100), earningCents, "seller earning must be 9100 kopecks (91 RUB)")

		// 4. Repeated delivery creates no duplicates
		rr2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodPost, "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewBufferString(payload))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(rr2, req2)
		require.Equal(t, http.StatusConflict, rr2.Code)

		var count int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'", orderItemID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "repeated delivery should not duplicate earnings")

		// 5. Undelivered seller B has 0 ledger entries
		var countB int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1", itemB).Scan(&countB)
		require.NoError(t, err)
		assert.Equal(t, 0, countB, "seller B should not be credited when seller A is delivered")
	})

	t.Run("accounting_failure_rolls_back_delivery_and_ledger", func(t *testing.T) {
		sellerID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Brand Fail', $3, $2, 'active', now(), now())", sellerID, uuid.New().String()+"@seller.com", uuid.New().String())
		require.NoError(t, err)
		cleaner.sellerIDs = append(cleaner.sellerIDs, sellerID)

		customerID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'Customer', 'hash', 'customer', 'active', now(), now())", customerID, uuid.New().String()+"@customer.com", "7999"+uuid.New().String()[:7])
		require.NoError(t, err)
		cleaner.userIDs = append(cleaner.userIDs, customerID)

		orderID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'shipped', 10000, 'N', 'P', 'E', 'A', now(), now())", orderID, customerID)
		require.NoError(t, err)
		cleaner.orderIDs = append(cleaner.orderIDs, orderID)

		fulfillmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', now(), now())", fulfillmentID, orderID, sellerID)
		require.NoError(t, err)
		cleaner.fulfillmentIDs = append(cleaner.fulfillmentIDs, fulfillmentID)

		catID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())", catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.categoryIDs = append(cleaner.categoryIDs, catID)

		productID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 'Desc', 10000, 'published', now(), now())", productID, sellerID, catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.productIDs = append(cleaner.productIDs, productID)

		variantID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, 10000, true, now(), now())", variantID, productID, uuid.New().String())
		require.NoError(t, err)
		cleaner.variantIDs = append(cleaner.variantIDs, variantID)

		orderItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 10000, 1, 10000)", orderItemID, orderID, fulfillmentID, sellerID, productID, variantID)
		require.NoError(t, err)
		cleaner.orderItemIDs = append(cleaner.orderItemIDs, orderItemID)
		cleaner.ledgerItemIDs = append(cleaner.ledgerItemIDs, orderItemID)

		shipmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO shipments (id, order_id, fulfillment_id, status, tracking_number, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', 'TRK-FAIL', now(), now())", shipmentID, orderID, fulfillmentID)
		require.NoError(t, err)
		cleaner.shipmentIDs = append(cleaner.shipmentIDs, shipmentID)

		// Inject controlled accounting failure: allow sale_gross and zamk_commission, but fail on seller_earning
		_, err = pgClient.Pool.Exec(ctx, "ALTER TABLE seller_ledger_entries ADD CONSTRAINT check_fail_seller_earning CHECK (type != 'seller_earning') NOT VALID")
		require.NoError(t, err)
		defer func() {
			_, _ = pgClient.Pool.Exec(context.Background(), "ALTER TABLE seller_ledger_entries DROP CONSTRAINT IF EXISTS check_fail_seller_earning")
		}()

		payload := `{"comment": "test delivery failure"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, "delivery must fail when accounting entry fails")

		// Assert rollback: no partial ledger entries
		var ledgerCount int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1", orderItemID).Scan(&ledgerCount)
		require.NoError(t, err)
		assert.Equal(t, 0, ledgerCount, "all partial ledger entries must be rolled back")

		// Assert statuses unchanged
		var sStatus string
		var deliveredAt *string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status, delivered_at::text FROM shipments WHERE id = $1", shipmentID).Scan(&sStatus, &deliveredAt)
		require.NoError(t, err)
		assert.Equal(t, "shipped", sStatus, "shipment status must remain shipped")
		assert.Nil(t, deliveredAt, "shipment delivered_at must remain nil")

		var fStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM order_fulfillments WHERE id = $1", fulfillmentID).Scan(&fStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", fStatus, "fulfillment status must remain shipped")

		var oStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", oStatus, "order status must remain shipped")

		// Drop the failing constraint
		_, err = pgClient.Pool.Exec(ctx, "ALTER TABLE seller_ledger_entries DROP CONSTRAINT check_fail_seller_earning")
		require.NoError(t, err)

		// Retry delivery successfully
		rr2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodPost, "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewBufferString(payload))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(rr2, req2)
		require.Equal(t, http.StatusOK, rr2.Code)

		// Verify exactly one complete set of ledger entries
		var totalEntries int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1", orderItemID).Scan(&totalEntries)
		require.NoError(t, err)
		assert.Equal(t, 3, totalEntries, "must contain exactly 3 entries: sale_gross, zamk_commission, seller_earning")

		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM shipments WHERE id = $1", shipmentID).Scan(&sStatus)
		require.NoError(t, err)
		assert.Equal(t, "delivered", sStatus)
	})

	t.Run("concurrent_delivery_requests", func(t *testing.T) {
		sellerID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Brand Conc', $3, $2, 'active', now(), now())", sellerID, uuid.New().String()+"@seller.com", uuid.New().String())
		require.NoError(t, err)
		cleaner.sellerIDs = append(cleaner.sellerIDs, sellerID)

		customerID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'Customer', 'hash', 'customer', 'active', now(), now())", customerID, uuid.New().String()+"@customer.com", "7999"+uuid.New().String()[:7])
		require.NoError(t, err)
		cleaner.userIDs = append(cleaner.userIDs, customerID)

		orderID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'shipped', 10000, 'N', 'P', 'E', 'A', now(), now())", orderID, customerID)
		require.NoError(t, err)
		cleaner.orderIDs = append(cleaner.orderIDs, orderID)

		fulfillmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', now(), now())", fulfillmentID, orderID, sellerID)
		require.NoError(t, err)
		cleaner.fulfillmentIDs = append(cleaner.fulfillmentIDs, fulfillmentID)

		catID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())", catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.categoryIDs = append(cleaner.categoryIDs, catID)

		productID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 'Desc', 10000, 'published', now(), now())", productID, sellerID, catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.productIDs = append(cleaner.productIDs, productID)

		variantID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, 10000, true, now(), now())", variantID, productID, uuid.New().String())
		require.NoError(t, err)
		cleaner.variantIDs = append(cleaner.variantIDs, variantID)

		orderItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 10000, 1, 10000)", orderItemID, orderID, fulfillmentID, sellerID, productID, variantID)
		require.NoError(t, err)
		cleaner.orderItemIDs = append(cleaner.orderItemIDs, orderItemID)
		cleaner.ledgerItemIDs = append(cleaner.ledgerItemIDs, orderItemID)

		shipmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO shipments (id, order_id, fulfillment_id, status, tracking_number, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', 'TRK-CONC', now(), now())", shipmentID, orderID, fulfillmentID)
		require.NoError(t, err)
		cleaner.shipmentIDs = append(cleaner.shipmentIDs, shipmentID)

		payload := `{"comment": "test concurrent delivery"}`
		var wg sync.WaitGroup
		wg.Add(2)
		codes := make([]int, 2)
		for i := 0; i < 2; i++ {
			go func(idx int) {
				defer wg.Done()
				rr := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewBufferString(payload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+adminToken)
				r.ServeHTTP(rr, req)
				codes[idx] = rr.Code
			}(i)
		}
		wg.Wait()

		sort.Ints(codes)
		assert.Equal(t, []int{http.StatusOK, http.StatusConflict}, codes, "concurrent delivery must produce exactly one 200 and one 409")

		var earningCount, commCount, grossCount int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'", orderItemID).Scan(&earningCount)
		require.NoError(t, err)
		assert.Equal(t, 1, earningCount)

		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'zamk_commission'", orderItemID).Scan(&commCount)
		require.NoError(t, err)
		assert.Equal(t, 1, commCount)

		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'sale_gross'", orderItemID).Scan(&grossCount)
		require.NoError(t, err)
		assert.Equal(t, 1, grossCount)
	})

	t.Run("connected_refund_deduction_after_delivery", func(t *testing.T) {
		sellerID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Brand Refund', $3, $2, 'active', now(), now())", sellerID, uuid.New().String()+"@seller.com", uuid.New().String())
		require.NoError(t, err)
		cleaner.sellerIDs = append(cleaner.sellerIDs, sellerID)

		customerID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'Customer', 'hash', 'customer', 'active', now(), now())", customerID, uuid.New().String()+"@customer.com", "7999"+uuid.New().String()[:7])
		require.NoError(t, err)
		cleaner.userIDs = append(cleaner.userIDs, customerID)

		orderID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'shipped', 10000, 'N', 'P', 'E', 'A', now(), now())", orderID, customerID)
		require.NoError(t, err)
		cleaner.orderIDs = append(cleaner.orderIDs, orderID)

		payID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, created_at, updated_at)
			VALUES ($1, $2, 'tbank', $3, 'succeeded', 10000, 'RUB', $4, now(), now())
		`, payID, orderID, "PAY-"+uuid.New().String()[:8], "IDEM-"+uuid.New().String()[:8])
		require.NoError(t, err)
		cleaner.paymentIDs = append(cleaner.paymentIDs, payID)

		fulfillmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', now(), now())", fulfillmentID, orderID, sellerID)
		require.NoError(t, err)
		cleaner.fulfillmentIDs = append(cleaner.fulfillmentIDs, fulfillmentID)

		catID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())", catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.categoryIDs = append(cleaner.categoryIDs, catID)

		productID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 'Desc', 10000, 'published', now(), now())", productID, sellerID, catID, uuid.New().String())
		require.NoError(t, err)
		cleaner.productIDs = append(cleaner.productIDs, productID)

		variantID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, 10000, true, now(), now())", variantID, productID, uuid.New().String())
		require.NoError(t, err)
		cleaner.variantIDs = append(cleaner.variantIDs, variantID)

		orderItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 10000, 1, 10000)", orderItemID, orderID, fulfillmentID, sellerID, productID, variantID)
		require.NoError(t, err)
		cleaner.orderItemIDs = append(cleaner.orderItemIDs, orderItemID)
		cleaner.ledgerItemIDs = append(cleaner.ledgerItemIDs, orderItemID)

		shipmentID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, "INSERT INTO shipments (id, order_id, fulfillment_id, status, tracking_number, created_at, updated_at) VALUES ($1, $2, $3, 'shipped', 'TRK-REFUND', now(), now())", shipmentID, orderID, fulfillmentID)
		require.NoError(t, err)
		cleaner.shipmentIDs = append(cleaner.shipmentIDs, shipmentID)

		// 1. Deliver shipment through canonical DeliverShipment endpoint
		payload := `{"comment": "delivery before return"}`
		reqDel := httptest.NewRequest(http.MethodPost, "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewBufferString(payload))
		reqDel.Header.Set("Content-Type", "application/json")
		reqDel.Header.Set("Authorization", "Bearer "+adminToken)

		rrDel := httptest.NewRecorder()
		r.ServeHTTP(rrDel, reqDel)
		require.Equal(t, http.StatusOK, rrDel.Code)

		// Confirm earning was created by delivery (not manually seeded!)
		var initialEarning int64
		err = pgClient.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'", orderItemID).Scan(&initialEarning)
		require.NoError(t, err)
		assert.Equal(t, int64(9100), initialEarning, "seller earning must be 9100 kopecks (91 RUB)")

		// 2. Customer return received at warehouse
		retID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
		`, retID, orderID, fulfillmentID, customerID)
		require.NoError(t, err)
		cleaner.returnIDs = append(cleaner.returnIDs, retID)

		retItemID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
			VALUES ($1, $2, $3, 1, 1, 0, 0, now())
		`, retItemID, retID, orderItemID)
		require.NoError(t, err)
		cleaner.returnItemIDs = append(cleaner.returnItemIDs, retItemID)

		// 3. Initiate Refund via API
		reqRefund := httptest.NewRequest(http.MethodPost, "/api/admin/returns/"+retID.String()+"/refund", bytes.NewBufferString("{}"))
		reqRefund.Header.Set("Content-Type", "application/json")
		reqRefund.Header.Set("Authorization", "Bearer "+adminToken)

		rrRefund := httptest.NewRecorder()
		r.ServeHTTP(rrRefund, reqRefund)
		require.Equal(t, http.StatusCreated, rrRefund.Code)

		// Track created refund for cleanup
		var refID uuid.UUID
		err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM refunds WHERE return_id = $1", retID).Scan(&refID)
		require.NoError(t, err)
		cleaner.refundIDs = append(cleaner.refundIDs, refID)

		// 4. Simulate refund success via canonical endpoint
		reqSim := httptest.NewRequest(http.MethodPost, "/api/admin/returns/"+retID.String()+"/simulate-refund-success", nil)
		reqSim.Header.Set("Authorization", "Bearer "+adminToken)

		rrSim := httptest.NewRecorder()
		r.ServeHTTP(rrSim, reqSim)
		require.Equal(t, http.StatusOK, rrSim.Code)

		// Assert return transitioned to 'refunded'
		var retStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&retStatus)
		require.NoError(t, err)
		assert.Equal(t, "refunded", retStatus)

		// Assert deduction created in seller ledger
		var deductionAmount int64
		err = pgClient.Pool.QueryRow(ctx, "SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment'", orderItemID).Scan(&deductionAmount)
		require.NoError(t, err)
		assert.Equal(t, int64(-9100), deductionAmount, "adjustment deduction must exactly reverse the 9100 kopecks (91 RUB) seller earning")

		// 5. Repeat success processing to confirm idempotent protection against duplicate deduction
		rrSim2 := httptest.NewRecorder()
		reqSim2 := httptest.NewRequest(http.MethodPost, "/api/admin/returns/"+retID.String()+"/simulate-refund-success", nil)
		reqSim2.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(rrSim2, reqSim2)
		assert.Equal(t, http.StatusNotFound, rrSim2.Code, "repeat refund simulation must return 404 no pending refund")

		var adjCount int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment'", orderItemID).Scan(&adjCount)
		require.NoError(t, err)
		assert.Equal(t, 1, adjCount, "repeated refund success processing must NOT create duplicate deduction")
	})
}
