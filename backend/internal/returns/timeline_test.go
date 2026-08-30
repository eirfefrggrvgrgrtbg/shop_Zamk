package returns_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestReturnTimeline_ComprehensiveMatrix(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Strict DB Safety Guard
	testutil.AssertTestDatabase(t, pool)

	dbClient := &postgres.Client{Pool: pool}
	repo := returns.NewRepository(pool)
	svc := returns.NewService(repo, nil, nil, dbClient, nil, nil, 14, nil, nil, nil)

	// Fixture IDs
	customerUserID := uuid.New()
	adminUserID := uuid.New()
	sellerID := uuid.New()
	categoryID := uuid.New()
	brandID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	unitID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	orderItemID := uuid.New()
	allocationID := uuid.New()
	returnID := uuid.New()
	returnItemID := uuid.New()
	returnUnitID := uuid.New()
	returnShipmentID := uuid.New()
	refundID := uuid.New()
	msgInfoReqID := uuid.New()
	msgOrdinaryID := uuid.New()
	msgCustomerReplyID := uuid.New()
	msgExtraID := uuid.New()

	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])
	zmuCode := fmt.Sprintf("ZMU-%s", unitID.String()[:16])
	supplyNumber := fmt.Sprintf("SUP-%s", supplyID.String()[:6])

	t0 := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	tApproved := t0.Add(10 * time.Minute)
	tInfoReq := t0.Add(20 * time.Minute)
	tOrdinary := t0.Add(25 * time.Minute)
	tCustReply := t0.Add(30 * time.Minute)
	tExtraMsg := t0.Add(35 * time.Minute)
	tLogistics := t0.Add(38 * time.Minute)
	tReceivingStarted := t0.Add(40 * time.Minute)
	tScanned := t0.Add(50 * time.Minute)
	tRefunded := t0.Add(60 * time.Minute)

	cleanup := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM refunds WHERE id = $1", refundID)
		_, _ = pool.Exec(ctx, "DELETE FROM return_shipments WHERE id = $1", returnShipmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM return_messages WHERE id = ANY($1)", []uuid.UUID{msgInfoReqID, msgOrdinaryID, msgCustomerReplyID, msgExtraID})
		_, _ = pool.Exec(ctx, "DELETE FROM return_item_units WHERE id = $1", returnUnitID)
		_, _ = pool.Exec(ctx, "DELETE FROM return_items WHERE id = $1", returnItemID)
		_, _ = pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", returnID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_item_allocations WHERE id = $1", allocationID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_items WHERE id = $1", orderItemID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM inventory_units WHERE id = $1", unitID)
		_, _ = pool.Exec(ctx, "DELETE FROM seller_supply_items WHERE id = $1", supplyItemID)
		_, _ = pool.Exec(ctx, "DELETE FROM seller_supplies WHERE id = $1", supplyID)
		_, _ = pool.Exec(ctx, "DELETE FROM product_variants WHERE id = $1", variantID)
		_, _ = pool.Exec(ctx, "DELETE FROM products WHERE id = $1", productID)
		_, _ = pool.Exec(ctx, "DELETE FROM brands WHERE id = $1", brandID)
		_, _ = pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", categoryID)
		_, _ = pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{customerUserID, adminUserID})
	}

	cleanup()
	t.Cleanup(func() {
		cleanup()
		// Zero leftovers check
		var count int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM returns WHERE id = $1", returnID).Scan(&count)
		assert.Equal(t, 0, count, "leftover return record found")
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM return_shipments WHERE id = $1", returnShipmentID).Scan(&count)
		assert.Equal(t, 0, count, "leftover return_shipments record found")
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE id = $1", refundID).Scan(&count)
		assert.Equal(t, 0, count, "leftover refunds record found")
	})

	// 1. Dependencies
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES 
			($1, 'Cust Ret User', $3, 'hash', 'customer', 'active', false, now(), now()),
			($2, 'Admin Ret User', $4, 'hash', 'admin', 'active', false, now(), now())
	`, customerUserID, adminUserID, fmt.Sprintf("ret_cust_%s@test.local", customerUserID.String()[:8]), fmt.Sprintf("ret_admin_%s@test.local", adminUserID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'Ret Brand', $2, 'desc', 'seller_ret@test.local', '123456', 'active', now(), now())
	`, sellerID, fmt.Sprintf("ret-brand-%s", sellerID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Ret Cat', $2, now(), now())
	`, categoryID, fmt.Sprintf("ret-cat-%s", categoryID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO brands (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Ret Brand Name', $2, now(), now())
	`, brandID, fmt.Sprintf("ret-brand-name-%s", brandID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Dev Wool Coat', $5, 'desc', 'published', 2000000, now(), now())
	`, productID, sellerID, categoryID, brandID, fmt.Sprintf("dev-wool-coat-ret-%s", productID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode, size, color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'L', 'Navy', now(), now())
	`, variantID, productID, fmt.Sprintf("SKU-RET-%s", variantID.String()[:8]), fmt.Sprintf("ZMK-RET-%s", variantID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, supply_number, status, handoff_method, created_at, updated_at)
		VALUES ($1, $2, $3, 'completed', 'self_delivery', now(), now())
	`, supplyID, sellerID, supplyNumber)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', $4, $5, 1, now(), now())
	`, unitID, variantID, zmuCode, supplyID, supplyItemID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 2000000, 'RUB', 'Cust Ret', '123', 'cust_ret@b.c', 'Addr', $4, $4)
	`, orderID, customerUserID, ordNumber, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 2000000, 1000, 1800000, $4, $4)
	`, fulfillmentID, orderID, sellerID, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, price_cents, quantity, subtotal_price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Dev Wool Coat', 'dev-wool-coat-ret', 'L', 'Navy', 'SKU-RET-01', 2000000, 1, 2000000, $7)
	`, orderItemID, orderID, fulfillmentID, productID, variantID, sellerID, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, allocationID, orderItemID, unitID, t0, t0)
	require.NoError(t, err)

	// 2. Return record
	_, err = pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at, approved_at, receiving_started_at)
		VALUES ($1, $2, $3, $4, 'receiving', 'defective', 'Not fitting well', $5, $5, $6, $7)
	`, returnID, orderID, fulfillmentID, customerUserID, t0, tApproved, tReceivingStarted)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
		VALUES ($1, $2, $3, 1, $4)
	`, returnItemID, returnID, orderItemID, t0)
	require.NoError(t, err)

	// Return Logistics Selection: return.logistics_created (cdek_courier)
	_, err = pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status, created_at, updated_at)
		VALUES ($1, $2, 'cdek', 'cdek_courier', '1400123456', 'awaiting_handover', $3, $3)
	`, returnShipmentID, returnID, tLogistics)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, returnUnitID, returnItemID, allocationID, tScanned, t0)
	require.NoError(t, err)

	// Messages:
	// 1) Info request from Admin -> return.info_requested
	_, err = pool.Exec(ctx, `
		INSERT INTO return_messages (id, return_id, sender_user_id, sender_role, message_type, body, created_at)
		VALUES ($1, $2, $3, 'admin', 'info_request', 'Please provide photo evidence', $4)
	`, msgInfoReqID, returnID, adminUserID, tInfoReq)
	require.NoError(t, err)

	// 2) Ordinary internal admin message -> must NOT become a workflow event
	_, err = pool.Exec(ctx, `
		INSERT INTO return_messages (id, return_id, sender_user_id, sender_role, message_type, body, created_at)
		VALUES ($1, $2, $3, 'admin', 'message', 'Internal note', $4)
	`, msgOrdinaryID, returnID, adminUserID, tOrdinary)
	require.NoError(t, err)

	// 3) First customer response -> return.customer_replied
	_, err = pool.Exec(ctx, `
		INSERT INTO return_messages (id, return_id, sender_user_id, sender_role, message_type, body, created_at)
		VALUES ($1, $2, $3, 'customer', 'message', 'Here is the photo attached', $4)
	`, msgCustomerReplyID, returnID, customerUserID, tCustReply)
	require.NoError(t, err)

	// 4) Second customer message -> must NOT create a second customer_replied event
	_, err = pool.Exec(ctx, `
		INSERT INTO return_messages (id, return_id, sender_user_id, sender_role, message_type, body, created_at)
		VALUES ($1, $2, $3, 'customer', 'message', 'Also one more question', $4)
	`, msgExtraID, returnID, customerUserID, tExtraMsg)
	require.NoError(t, err)

	// Refund record: return.refunded
	_, err = pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'succeeded', 2000000, 'RUB', $4, $4, $4)
	`, refundID, returnID, orderID, tRefunded)
	require.NoError(t, err)

	// Fetch Timeline
	tl, err := svc.GetAdminTimeline(ctx, returnID)
	require.NoError(t, err)
	require.NotNil(t, tl)

	// Strict Assertions
	assert.Equal(t, "return", tl.EntityType)
	assert.Equal(t, returnID, tl.EntityID)
	assert.Equal(t, ordNumber, tl.CanonicalIdentifier)
	assert.NotContains(t, tl.CanonicalIdentifier, "Return ")
	assert.NotContains(t, tl.CanonicalIdentifier, returnID.String())

	// Expected 8 events:
	// 0: return.requested (t0)
	// 1: return.approved (tApproved)
	// 2: return.info_requested (tInfoReq)
	// 3: return.customer_replied (tCustReply)
	// 4: return.logistics_created (tLogistics)
	// 5: return.receiving_started (tReceivingStarted)
	// 6: return.unit_scanned (tScanned)
	// 7: return.refunded (tRefunded)
	require.Len(t, tl.Events, 8)

	// Ascending order check
	for i := 0; i < len(tl.Events)-1; i++ {
		assert.True(t, tl.Events[i].OccurredAt.Before(tl.Events[i+1].OccurredAt) || tl.Events[i].OccurredAt.Equal(tl.Events[i+1].OccurredAt))
	}

	// 0: return.requested
	assert.Equal(t, "return.requested", tl.Events[0].Type)
	assert.True(t, t0.Equal(tl.Events[0].OccurredAt))
	assert.Equal(t, "customer", tl.Events[0].ActorType)
	assert.Equal(t, "Покупатель", tl.Events[0].ActorLabel)
	assert.Contains(t, tl.Events[0].Description, ordNumber)

	// 1: return.approved
	assert.Equal(t, "return.approved", tl.Events[1].Type)
	assert.True(t, tApproved.Equal(tl.Events[1].OccurredAt))
	assert.Equal(t, "admin", tl.Events[1].ActorType)
	assert.Equal(t, "Администратор", tl.Events[1].ActorLabel)

	// 2: return.info_requested
	assert.Equal(t, "return.info_requested", tl.Events[2].Type)
	assert.True(t, tInfoReq.Equal(tl.Events[2].OccurredAt))
	assert.Equal(t, "admin", tl.Events[2].ActorType)
	assert.Equal(t, "Администратор", tl.Events[2].ActorLabel)

	// 3: return.customer_replied
	assert.Equal(t, "return.customer_replied", tl.Events[3].Type)
	assert.True(t, tCustReply.Equal(tl.Events[3].OccurredAt))
	assert.Equal(t, "customer", tl.Events[3].ActorType)
	assert.Equal(t, "Покупатель", tl.Events[3].ActorLabel)

	// 4: return.logistics_created (cdek_courier)
	assert.Equal(t, "return.logistics_created", tl.Events[4].Type)
	assert.True(t, tLogistics.Equal(tl.Events[4].OccurredAt))
	assert.Equal(t, "customer", tl.Events[4].ActorType)
	assert.Equal(t, "Покупатель", tl.Events[4].ActorLabel)
	assert.Equal(t, "Оформлена доставка возврата", tl.Events[4].Title)
	assert.Equal(t, "Способ возврата: СДЭК — курьер", tl.Events[4].Description)

	// 5: return.receiving_started
	assert.Equal(t, "return.receiving_started", tl.Events[5].Type)
	assert.True(t, tReceivingStarted.Equal(tl.Events[5].OccurredAt))
	assert.Equal(t, "warehouse", tl.Events[5].ActorType)
	assert.Equal(t, "Склад ZAMK", tl.Events[5].ActorLabel)

	// 6: return.unit_scanned
	assert.Equal(t, "return.unit_scanned", tl.Events[6].Type)
	assert.True(t, tScanned.Equal(tl.Events[6].OccurredAt))
	assert.Equal(t, "warehouse", tl.Events[6].ActorType)
	assert.Equal(t, "Склад ZAMK", tl.Events[6].ActorLabel)
	assert.Contains(t, tl.Events[6].Description, zmuCode)
	assert.Contains(t, tl.Events[6].Description, "Dev Wool Coat")

	// 7: return.refunded
	assert.Equal(t, "return.refunded", tl.Events[7].Type)
	assert.True(t, tRefunded.Equal(tl.Events[7].OccurredAt))
	assert.Equal(t, "system", tl.Events[7].ActorType)
	assert.Equal(t, "Система", tl.Events[7].ActorLabel)
	assert.Equal(t, "Возврат средств выполнен", tl.Events[7].Title)
	assert.Contains(t, tl.Events[7].Description, "20000 ₽")
}

func TestReturnTimeline_CDEKOfficeLogistics(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	testutil.AssertTestDatabase(t, pool)

	dbClient := &postgres.Client{Pool: pool}
	repo := returns.NewRepository(pool)
	svc := returns.NewService(repo, nil, nil, dbClient, nil, nil, 14, nil, nil, nil)

	userID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	returnID := uuid.New()
	returnShipmentID := uuid.New()
	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])
	t0 := time.Date(2026, 8, 30, 8, 0, 0, 0, time.UTC)

	cleanup := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM return_shipments WHERE id = $1", returnShipmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", returnID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}
	cleanup()
	t.Cleanup(cleanup)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Office Ret User', $2, 'hash', 'customer', 'active', false, now(), now())
	`, userID, fmt.Sprintf("office_user_%s@test.local", userID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'Office Seller', $2, 'desc', 'office@test.local', '123', 'active', now(), now())
	`, sellerID, fmt.Sprintf("office-seller-%s", sellerID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 100000, 'RUB', 'Office User', '123', 'o@b.c', 'Addr', $4, $4)
	`, orderID, userID, ordNumber, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 100000, 1000, 90000, $4, $4)
	`, fulfillmentID, orderID, sellerID, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'approved', 'size', 'too small', $5, $5)
	`, returnID, orderID, fulfillmentID, userID, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status, created_at, updated_at)
		VALUES ($1, $2, 'cdek', 'cdek_office', '1400999999', 'awaiting_handover', $3, $3)
	`, returnShipmentID, returnID, t0.Add(5*time.Minute))
	require.NoError(t, err)

	tl, err := svc.GetAdminTimeline(ctx, returnID)
	require.NoError(t, err)
	require.Len(t, tl.Events, 2)

	assert.Equal(t, "return.logistics_created", tl.Events[1].Type)
	assert.Equal(t, "Способ возврата: СДЭК — отделение", tl.Events[1].Description)
}

func TestReturnTimeline_RejectedFixture(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	testutil.AssertTestDatabase(t, pool)

	dbClient := &postgres.Client{Pool: pool}
	repo := returns.NewRepository(pool)
	svc := returns.NewService(repo, nil, nil, dbClient, nil, nil, 14, nil, nil, nil)

	userID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	returnID := uuid.New()
	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])

	t0 := time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)
	tRejected := t0.Add(15 * time.Minute)

	cleanup := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", returnID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}

	cleanup()
	t.Cleanup(cleanup)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Rej User', $2, 'hash', 'customer', 'active', false, now(), now())
	`, userID, fmt.Sprintf("rej_user_%s@test.local", userID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'Rej Seller', $2, 'desc', 'rej_seller@test.local', '123', 'active', now(), now())
	`, sellerID, fmt.Sprintf("rej-seller-%s", sellerID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 100000, 'RUB', 'Rej User', '123', 'rej@b.c', 'Addr', $4, $4)
	`, orderID, userID, ordNumber, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 100000, 1000, 90000, $4, $4)
	`, fulfillmentID, orderID, sellerID, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at, rejected_at)
		VALUES ($1, $2, $3, $4, 'rejected', 'defective', 'broken', $5, $5, $6)
	`, returnID, orderID, fulfillmentID, userID, t0, tRejected)
	require.NoError(t, err)

	tl, err := svc.GetAdminTimeline(ctx, returnID)
	require.NoError(t, err)
	require.Len(t, tl.Events, 2)

	assert.Equal(t, "return.requested", tl.Events[0].Type)
	assert.Equal(t, "return.rejected", tl.Events[1].Type)
	assert.Equal(t, "admin", tl.Events[1].ActorType)
	assert.Equal(t, "Администратор", tl.Events[1].ActorLabel)
	assert.True(t, tRejected.Equal(tl.Events[1].OccurredAt))
}

func TestReturnTimeline_EqualTimestampTieBreak(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	testutil.AssertTestDatabase(t, pool)

	dbClient := &postgres.Client{Pool: pool}
	repo := returns.NewRepository(pool)
	svc := returns.NewService(repo, nil, nil, dbClient, nil, nil, 14, nil, nil, nil)

	userID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	returnID := uuid.New()
	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])
	exactTime := time.Date(2026, 8, 30, 14, 0, 0, 0, time.UTC)

	cleanup := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", returnID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}

	cleanup()
	t.Cleanup(cleanup)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'TieBreak Ret User', $2, 'hash', 'customer', 'active', false, now(), now())
	`, userID, fmt.Sprintf("tbret_user_%s@test.local", userID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'TB Ret Seller', $2, 'desc', 'tbret_seller@test.local', '123', 'active', now(), now())
	`, sellerID, fmt.Sprintf("tbret-seller-%s", sellerID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 100000, 'RUB', 'TB Ret User', '123', 'tbret@b.c', 'Addr', $4, $4)
	`, orderID, userID, ordNumber, exactTime)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 100000, 1000, 90000, $4, $4)
	`, fulfillmentID, orderID, sellerID, exactTime)
	require.NoError(t, err)

	// approved_at is same as created_at
	_, err = pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at, approved_at)
		VALUES ($1, $2, $3, $4, 'approved', 'defective', 'broken', $5, $5, $5)
	`, returnID, orderID, fulfillmentID, userID, exactTime)
	require.NoError(t, err)

	tl, err := svc.GetAdminTimeline(ctx, returnID)
	require.NoError(t, err)
	require.Len(t, tl.Events, 2)

	assert.Equal(t, "return.requested", tl.Events[0].Type)
	assert.Equal(t, "return.approved", tl.Events[1].Type)
}

func TestReturnTimeline_RefundMatrix(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	testutil.AssertTestDatabase(t, pool)

	dbClient := &postgres.Client{Pool: pool}
	repo := returns.NewRepository(pool)
	svc := returns.NewService(repo, nil, nil, dbClient, nil, nil, 14, nil, nil, nil)

	userID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	returnID := uuid.New()
	refundID_A := uuid.New()
	refundID_B := uuid.New()
	refundID_C := uuid.New()
	refundID_D := uuid.New()
	refundID_E := uuid.New()

	ordNumber := fmt.Sprintf("ORD-%s", orderID.String()[:6])
	t0 := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	tProcessedA := t0.Add(30 * time.Minute)
	tCreatedE := t0.Add(1 * time.Hour)
	tProcessedE := t0.Add(3 * time.Hour)

	cleanup := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM refunds WHERE id = ANY($1)", []uuid.UUID{refundID_A, refundID_B, refundID_C, refundID_D, refundID_E})
		_, _ = pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", returnID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		var count int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE id = ANY($1)", []uuid.UUID{refundID_A, refundID_B, refundID_C, refundID_D, refundID_E}).Scan(&count)
		assert.Equal(t, 0, count, "leftover refund records found")
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Ref Matrix User', $2, 'hash', 'customer', 'active', false, now(), now())
	`, userID, fmt.Sprintf("ref_matrix_%s@test.local", userID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'Ref Matrix Seller', $2, 'desc', 'ref_matrix@test.local', '123', 'active', now(), now())
	`, sellerID, fmt.Sprintf("ref-mat-seller-%s", sellerID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 500000, 'RUB', 'Ref User', '123', 'r@b.c', 'Addr', $4, $4)
	`, orderID, userID, ordNumber, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 500000, 1000, 450000, $4, $4)
	`, fulfillmentID, orderID, sellerID, t0)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'approved', 'defective', 'broken', $5, $5)
	`, returnID, orderID, fulfillmentID, userID, t0)
	require.NoError(t, err)

	// Subtest A: refund status = succeeded with processed_at set
	_, err = pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'succeeded', 100000, 'RUB', $4, $5, $5)
	`, refundID_A, returnID, orderID, tProcessedA, t0)
	require.NoError(t, err)

	tlA, err := svc.GetAdminTimeline(ctx, returnID)
	require.NoError(t, err)
	var refundEventsA []returns.TimelineEvent
	for _, ev := range tlA.Events {
		if ev.Type == "return.refunded" {
			refundEventsA = append(refundEventsA, ev)
		}
	}
	require.Len(t, refundEventsA, 1, "Scenario A: must have exactly one return.refunded event")
	assert.True(t, tProcessedA.Equal(refundEventsA[0].OccurredAt), "Scenario A: occurredAt must equal processed_at")
	assert.Equal(t, "Возврат средств выполнен", refundEventsA[0].Title)
	assert.Contains(t, refundEventsA[0].Description, "1000 ₽")
	assert.NotContains(t, refundEventsA[0].Description, refundID_A.String())

	// Subtest B: refund pending with processed_at NULL -> must NOT emit return.refunded
	_, err = pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', 50000, 'RUB', NULL, $4, $4)
	`, refundID_B, returnID, orderID, t0.Add(40*time.Minute))
	require.NoError(t, err)

	// Subtest C: refund failed with processed_at NULL (or non-null) -> must NOT emit return.refunded
	_, err = pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'failed', 20000, 'RUB', NULL, $4, $4)
	`, refundID_C, returnID, orderID, t0.Add(45*time.Minute))
	require.NoError(t, err)

	// Subtest D: successful status but processed_at NULL -> must NOT emit return.refunded
	_, err = pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'succeeded', 30000, 'RUB', NULL, $4, $4)
	`, refundID_D, returnID, orderID, t0.Add(50*time.Minute))
	require.NoError(t, err)

	// Subtest E: created_at differs materially from processed_at (created_at = t0 + 1h, processed_at = t0 + 3h)
	_, err = pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'succeeded', 200000, 'RUB', $4, $5, $5)
	`, refundID_E, returnID, orderID, tProcessedE, tCreatedE)
	require.NoError(t, err)

	tlAll, err := svc.GetAdminTimeline(ctx, returnID)
	require.NoError(t, err)
	var refundEventsAll []returns.TimelineEvent
	for _, ev := range tlAll.Events {
		if ev.Type == "return.refunded" {
			refundEventsAll = append(refundEventsAll, ev)
		}
	}
	// Only refund A and refund E should be present
	require.Len(t, refundEventsAll, 2, "Only refunds with valid status AND non-null processed_at should produce return.refunded")
	assert.Equal(t, fmt.Sprintf("return-refunded-%s", refundID_A), refundEventsAll[0].ID)
	assert.True(t, tProcessedA.Equal(refundEventsAll[0].OccurredAt))

	assert.Equal(t, fmt.Sprintf("return-refunded-%s", refundID_E), refundEventsAll[1].ID)
	assert.True(t, tProcessedE.Equal(refundEventsAll[1].OccurredAt), "Scenario E: occurredAt must equal processed_at, not created_at")
	assert.False(t, tCreatedE.Equal(refundEventsAll[1].OccurredAt), "Scenario E: occurredAt must NOT equal created_at")
	assert.NotContains(t, refundEventsAll[1].Description, refundID_E.String())
}

func TestReturnTimeline_HandlerInternalErrorPrivacy(t *testing.T) {
	handler := returns.NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/returns/invalid-uuid/timeline", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetAdminReturnTimeline(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_id", resp["error"]["code"])
}
