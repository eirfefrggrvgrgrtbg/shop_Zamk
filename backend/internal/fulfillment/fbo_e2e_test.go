package fulfillment_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// FBO.E2E.1: End-to-end operational pipeline for paid orders transitioning through
// warehouse picking and packaging (paid -> assembling -> packed -> ready for dispatch),
// verifying inventory invariants (reserved_stock held, total_stock untouched until shipment)
// and strict concurrency safety on an isolated disposable database.

type fboFixture struct {
	t          *testing.T
	ctx        context.Context
	db         *pgxpool.Pool
	pgClient   *postgres.Client
	svc        *fulfillment.Service
	handler    *fulfillment.Handler
	ordersRepo *orders.Repository
	notifSvc   *notifications.Service
	sellerID   uuid.UUID
	adminID    uuid.UUID
	customerID uuid.UUID
	catID      uuid.UUID
	brandID    uuid.UUID
	dmID       uuid.UUID
}

func newFBOFixture(t *testing.T) *fboFixture {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.DisposableDatabaseURL(t)
	db, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(db.Close)
	testutil.MustBeDisposableDatabase(t, db)

	pgClient := &postgres.Client{Pool: db}
	repo := fulfillment.NewRepository(db)
	ordersRepo := orders.NewRepository(db)
	notifRepo := notifications.NewRepository(pgClient)
	notifSvc := notifications.NewService(notifRepo, nil, nil)
	svc := fulfillment.NewService(repo, ordersRepo, pgClient, nil, notifSvc, nil)
	handler := fulfillment.NewHandler(svc)

	sellerID := uuid.New()
	adminID := uuid.New()
	customerID := uuid.New()
	catID := uuid.New()
	brandID := uuid.New()
	dmID := uuid.New()

	f := &fboFixture{
		t:          t,
		ctx:        ctx,
		db:         db,
		pgClient:   pgClient,
		svc:        svc,
		handler:    handler,
		ordersRepo: ordersRepo,
		notifSvc:   notifSvc,
		sellerID:   sellerID,
		adminID:    adminID,
		customerID: customerID,
		catID:      catID,
		brandID:    brandID,
		dmID:       dmID,
	}

	f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Nord Atelier FBO', $2, $3, 'active', now(), now())`,
		sellerID, "fbo-"+sellerID.String()[:8], "seller-"+sellerID.String()[:8]+"@ex.com")

	f.exec(`INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Warehouse Operator', $2, 'hash', 'admin', 'active', now(), now())`,
		adminID, "operator-"+adminID.String()[:8]+"@zamk.internal")

	f.exec(`INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Test Customer', $2, 'hash', 'customer', 'active', now(), now())`,
		customerID, "customer-"+customerID.String()[:8]+"@buyer.test")

	f.exec(`INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Hoodies', $2, now(), now())`,
		catID, "cat-"+catID.String()[:8])

	f.exec(`INSERT INTO brands (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Nord Atelier Brand', $2, now(), now())`,
		brandID, "brand-"+brandID.String()[:8])

	f.exec(`INSERT INTO delivery_methods (id, code, name, price_cents, is_active)
		VALUES ($1, $2, 'Курьерская доставка ZAMK', 39000, true)`,
		dmID, "dm_"+dmID.String()[:8])

	return f
}

func (f *fboFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *fboFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&n), sql)
	return n
}

func (f *fboFixture) str(sql string, args ...any) string {
	f.t.Helper()
	var s string
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&s), sql)
	return s
}

type testVariantInfo struct {
	VariantID uuid.UUID
	SKU       string
	Barcode   string
	Size      string
	Color     string
	Price     int64
}

// createProductWithVariants creates a published product with multiple variants and initial stock.
func (f *fboFixture) createProductWithVariants(variants []testVariantInfo, totalStock int) (uuid.UUID, []testVariantInfo) {
	f.t.Helper()
	prodID := uuid.New()
	f.exec(`INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Премиальное оверсайз худи', $5, 'Описание худи', 'published', 590000, now(), now())`,
		prodID, f.sellerID, f.catID, f.brandID, "hoodie-"+prodID.String()[:8])

	result := make([]testVariantInfo, len(variants))
	for i, v := range variants {
		varID := uuid.New()
		if v.VariantID != uuid.Nil {
			varID = v.VariantID
		}
		sku := fmt.Sprintf("SKU-HOODIE-%s", varID.String()[:8])
		if v.SKU != "" {
			sku = v.SKU
		}
		barcode := fmt.Sprintf("4680000%06d", i+100)
		if v.Barcode != "" {
			barcode = v.Barcode
		}
		price := int64(590000)
		if v.Price > 0 {
			price = v.Price
		}
		sizeStr := "M"
		if v.Size != "" {
			sizeStr = v.Size
		}
		colorStr := "Графит"
		if v.Color != "" {
			colorStr = v.Color
		}

		f.exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, $5, true, now(), now())`,
			varID, prodID, sku, barcode, price)

		// Create inventory item
		f.exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, 0, now(), now())
			ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = $4, reserved_stock = 0`,
			prodID, varID, f.sellerID, totalStock)

		result[i] = testVariantInfo{
			VariantID: varID,
			SKU:       sku,
			Barcode:   barcode,
			Size:      sizeStr,
			Color:     colorStr,
			Price:     price,
		}
	}

	return prodID, result
}

// createSerializedUnit creates a physical warehouse unit (ZMU) with status 'warehouse'.
func (f *fboFixture) createSerializedUnit(variantID uuid.UUID) (uuid.UUID, string) {
	f.t.Helper()
	unitID := uuid.New()
	unitCode, err := supplies.GenerateUnitCode()
	require.NoError(f.t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	f.exec(`INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`,
		supplyID, f.sellerID, "SUP-"+supplyID.String()[:8])
	f.exec(`INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 1, now(), now())`,
		supplyItemID, supplyID, variantID)

	f.exec(`INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, 'warehouse', now(), now())`,
		unitID, unitCode, variantID, supplyID, supplyItemID)

	return unitID, unitCode
}

type orderItemSpec struct {
	VariantInfo     testVariantInfo
	Quantity        int
	SerializedUnits []struct {
		UnitID   uuid.UUID
		UnitCode string
	}
}

// createOrderWithFulfillment creates an order, fulfillment, items, reservations, and unit allocations.
func (f *fboFixture) createOrderWithFulfillment(orderStatus, fulfillmentStatus string, prodID uuid.UUID, items []orderItemSpec) (uuid.UUID, uuid.UUID, []uuid.UUID) {
	f.t.Helper()
	orderID := uuid.New()
	fulfillmentID := uuid.New()

	var totalCents int64
	for _, it := range items {
		totalCents += it.VariantInfo.Price * int64(it.Quantity)
	}

	f.exec(`INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, delivery_method_id, delivery_method_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'Иван Иванов', '+79991234567', 'ivan@test.ru', 'г. Москва, ул. Складская, 1', $6, 'Курьерская доставка ZAMK', now(), now())`,
		orderID, f.customerID, "ZMK-"+orderID.String()[:8], orderStatus, totalCents, f.dmID)

	commBps := 1500
	sellerAmount := totalCents * int64(10000-commBps) / 10000
	f.exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())`,
		fulfillmentID, orderID, f.sellerID, fulfillmentStatus, totalCents, commBps, sellerAmount)

	orderItemIDs := make([]uuid.UUID, len(items))
	for i, it := range items {
		itemID := uuid.New()
		orderItemIDs[i] = itemID
		subtotal := it.VariantInfo.Price * int64(it.Quantity)

		f.exec(`INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, variant_size, variant_color, sku, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'Премиальное оверсайз худи', 'hoodie', $7, $8, $9, $10, $11, $12, now())`,
			itemID, orderID, fulfillmentID, prodID, it.VariantInfo.VariantID, f.sellerID, it.VariantInfo.Price, it.Quantity, subtotal, it.VariantInfo.Size, it.VariantInfo.Color, it.VariantInfo.SKU)

		// Create reservation
		resID := uuid.New()
		resStatus := "active"
		if orderStatus == "paid" || orderStatus == "assembling" || orderStatus == "packed" {
			resStatus = "converted"
		}
		var invItemID uuid.UUID
		err := f.db.QueryRow(f.ctx, `SELECT id FROM inventory_items WHERE product_variant_id = $1`, it.VariantInfo.VariantID).Scan(&invItemID)
		require.NoError(f.t, err)

		f.exec(`INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, now() + interval '30 minutes', $8, now())`,
			resID, invItemID, prodID, it.VariantInfo.VariantID, f.customerID, it.Quantity, resStatus, orderID)

		f.exec(`INSERT INTO order_reservations (id, order_id, reservation_id, created_at) VALUES (gen_random_uuid(), $1, $2, now())`, orderID, resID)

		// Update reserved_stock
		f.exec(`UPDATE inventory_items SET reserved_stock = reserved_stock + $1 WHERE id = $2`, it.Quantity, invItemID)

		// If serialized units provided, insert allocations
		for _, u := range it.SerializedUnits {
			allocID := uuid.New()
			f.exec(`INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, created_at)
				VALUES ($1, $2, $3, $4, now())`,
				allocID, itemID, u.UnitID, resID)
		}
	}

	return orderID, fulfillmentID, orderItemIDs
}

// ============================================================================
// 1. MAIN E2E SCENARIO
// ============================================================================

func TestFBO_E2E_FullPipeline_Paid_Picking_Packing_Success(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)

	// Step A: Create product with 2 variants: Size S and Size M, total stock = 10 each
	prodID, variants := f.createProductWithVariants([]testVariantInfo{
		{Size: "S", Color: "Черный", Price: 590000},
		{Size: "M", Color: "Графит", Price: 590000},
	}, 10)
	varS := variants[0]
	varM := variants[1]

	// Step B: Create physical warehouse units (ZMU barcodes)
	unitS1_ID, unitS1_Code := f.createSerializedUnit(varS.VariantID)
	unitM1_ID, unitM1_Code := f.createSerializedUnit(varM.VariantID)
	unitM2_ID, unitM2_Code := f.createSerializedUnit(varM.VariantID)

	// Free spare unit for replacement/substitution checks
	spareUnit_ID, spareUnit_Code := f.createSerializedUnit(varM.VariantID)
	_ = spareUnit_ID
	_ = spareUnit_Code

	// Step C: Customer places order for 1x Size S and 2x Size M
	itemsSpec := []orderItemSpec{
		{
			VariantInfo: varS,
			Quantity:    1,
			SerializedUnits: []struct {
				UnitID   uuid.UUID
				UnitCode string
			}{
				{UnitID: unitS1_ID, UnitCode: unitS1_Code},
			},
		},
		{
			VariantInfo: varM,
			Quantity:    2,
			SerializedUnits: []struct {
				UnitID   uuid.UUID
				UnitCode string
			}{
				{UnitID: unitM1_ID, UnitCode: unitM1_Code},
				{UnitID: unitM2_ID, UnitCode: unitM2_Code},
			},
		},
	}

	orderID, fulfillmentID, itemIDs := f.createOrderWithFulfillment("awaiting_payment", "awaiting_payment", prodID, itemsSpec)
	itemS_ID := itemIDs[0]
	itemM_ID := itemIDs[1]

	// Check pre-payment state
	require.Equal(t, "awaiting_payment", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
	require.Equal(t, "awaiting_payment", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID))
	require.Equal(t, 1, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID))
	require.Equal(t, 2, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID))
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID))
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID))

	// Step D: Payment succeeds (mock payment transition)
	f.exec(`UPDATE orders SET status = 'paid', updated_at = now() WHERE id = $1`, orderID)
	f.exec(`UPDATE order_fulfillments SET status = 'paid', updated_at = now() WHERE id = $1`, fulfillmentID)
	f.exec(`UPDATE reservations SET status = 'converted' WHERE order_id = $1`, orderID)
	f.exec(`INSERT INTO order_status_history (id, order_id, from_status, to_status, created_at)
		VALUES (gen_random_uuid(), $1, 'awaiting_payment', 'paid', now())`, orderID)

	// Invariant check after payment:
	// - reservations are converted
	// - reserved_stock is HELD (not decremented, not released)
	// - total_stock is UNTOUCHED
	require.Equal(t, 2, f.count(`SELECT count(*) FROM reservations WHERE order_id = $1 AND status = 'converted'`, orderID))
	require.Equal(t, 1, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID))
	require.Equal(t, 2, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID))
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID))
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID))

	// Step E: Warehouse Operator opens picking task
	pickingOrder, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, pickingOrder)
	assert.Equal(t, orderID, pickingOrder.OrderID)
	assert.Equal(t, "paid", pickingOrder.OrderStatus)
	assert.Equal(t, "paid", pickingOrder.FulfillmentStatus)
	require.Len(t, pickingOrder.Items, 2)

	// Validate picking order item metadata (SKU, title, size, color, quantity)
	var foundS, foundM *fulfillment.PickingItem
	for i := range pickingOrder.Items {
		it := &pickingOrder.Items[i]
		if it.OrderItemID == itemS_ID {
			foundS = it
		} else if it.OrderItemID == itemM_ID {
			foundM = it
		}
	}
	require.NotNil(t, foundS, "Size S item must be in picking task")
	require.NotNil(t, foundM, "Size M item must be in picking task")

	assert.Equal(t, "Премиальное оверсайз худи", foundS.Title)
	assert.Equal(t, varS.SKU, *foundS.SKU)
	assert.Equal(t, "S", *foundS.VariantSize)
	assert.Equal(t, "Черный", *foundS.VariantColor)
	assert.Equal(t, 1, foundS.Quantity)
	assert.Equal(t, 0, foundS.PickedQuantity)
	assert.Equal(t, 1, foundS.RemainingQuantity)
	assert.Equal(t, "serialized", foundS.AllocationMode)
	require.Len(t, foundS.AllocatedUnits, 1)
	assert.Equal(t, unitS1_Code, foundS.AllocatedUnits[0].UnitCode)

	assert.Equal(t, 2, foundM.Quantity)
	assert.Equal(t, 0, foundM.PickedQuantity)
	assert.Equal(t, 2, foundM.RemainingQuantity)
	assert.Equal(t, "serialized", foundM.AllocationMode)
	require.Len(t, foundM.AllocatedUnits, 2)

	// Check compatible units API
	compatUnits, err := f.svc.GetCompatibleUnits(ctx, fulfillmentID, itemM_ID)
	require.NoError(t, err)
	// Allocated units (2) + spare free unit (1) = 3 compatible units
	assert.GreaterOrEqual(t, len(compatUnits), 2)

	// Verify not yet in packing queue
	packQueue, err := f.svc.GetPackingQueue(ctx)
	require.NoError(t, err)
	for _, pq := range packQueue {
		assert.NotEqual(t, fulfillmentID, pq.FulfillmentID, "Fulfillment must not be in packing queue before picking is complete")
	}

	// Step F: Start Picking — Operator scans unit 1 (Size S)
	scanRes1, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitS1_Code, &itemS_ID)
	require.NoError(t, err)
	require.NotNil(t, scanRes1)
	assert.True(t, scanRes1.ScanResult.NewlyPicked)
	assert.Equal(t, "serialized", scanRes1.ScanResult.Type)
	assert.Equal(t, 1, scanRes1.Item.PickedQuantity)
	assert.Equal(t, 0, scanRes1.Item.RemainingQuantity)
	assert.Equal(t, 1, scanRes1.FulfillmentProgress.PickedQuantity)
	assert.Equal(t, 3, scanRes1.FulfillmentProgress.TotalQuantity)
	assert.False(t, scanRes1.FulfillmentProgress.IsComplete)

	// Verify status transition: paid -> assembling upon first picked item
	require.Equal(t, "assembling", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID))
	require.Equal(t, "assembling", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'assembling'`, orderID))

	// Invariant check: stock untouched during picking
	require.Equal(t, 1, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID))
	require.Equal(t, 2, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID))
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID))
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID))

	// Step G: Operator picks the remaining units of Size M
	// Scan unit 2 (first M unit)
	scanRes2, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitM1_Code, &itemM_ID)
	require.NoError(t, err)
	assert.True(t, scanRes2.ScanResult.NewlyPicked)
	assert.Equal(t, 1, scanRes2.Item.PickedQuantity)
	assert.Equal(t, 1, scanRes2.Item.RemainingQuantity)
	assert.False(t, scanRes2.FulfillmentProgress.IsComplete)

	// Scan unit 3 (second M unit)
	scanRes3, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitM2_Code, &itemM_ID)
	require.NoError(t, err)
	assert.True(t, scanRes3.ScanResult.NewlyPicked)
	assert.Equal(t, 2, scanRes3.Item.PickedQuantity)
	assert.Equal(t, 0, scanRes3.Item.RemainingQuantity)
	assert.True(t, scanRes3.FulfillmentProgress.IsComplete, "All items must now be complete")

	// Verify all allocations are stamped with picked_at
	require.Equal(t, 3, f.count(`SELECT count(*) FROM order_item_allocations a
		JOIN order_items oi ON oi.id = a.order_item_id
		WHERE oi.order_fulfillment_id = $1 AND a.picked_at IS NOT NULL`, fulfillmentID))

	// Step H: Packing Queue — fulfillment now appears as ready for packaging
	packQueueAfter, err := f.svc.GetPackingQueue(ctx)
	require.NoError(t, err)
	var foundInPackQueue bool
	for _, pq := range packQueueAfter {
		if pq.FulfillmentID == fulfillmentID {
			foundInPackQueue = true
			assert.Equal(t, "assembling", pq.Status)
			assert.Equal(t, "assembling", pq.OrderStatus)
			assert.Equal(t, 3, pq.TotalQuantity)
			assert.Equal(t, 3, pq.PickedQuantity)
			assert.NotNil(t, pq.PickingCompletedAt)
		}
	}
	assert.True(t, foundInPackQueue, "Fully picked fulfillment MUST appear in packing queue")

	// Step I: Operator executes Packing
	packResult, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, packResult)
	assert.Equal(t, fulfillmentID, packResult.FulfillmentID)
	assert.Equal(t, orderID, packResult.OrderID)
	assert.Equal(t, "packed", packResult.FulfillmentStatus)
	assert.Equal(t, "packed", packResult.OrderStatus)
	assert.WithinDuration(t, time.Now(), packResult.PackedAt, 5*time.Second)

	// Verify database persistence of packed state
	require.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID))
	require.Equal(t, "packed", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'packed'`, orderID))

	// Verify customer notification created for packaging
	require.Equal(t, 1, f.count(`SELECT count(*) FROM notifications
		WHERE recipient_user_id = $1 AND type = $2 AND entity_id = $3`,
		f.customerID, notifications.TypeCustomerFulfillmentPacked, fulfillmentID))

	// Step J: Readiness for Dispatch
	// Once packed, fulfillment is ready for dispatch: appears in DispatchQueue
	dispatchQueue, err := f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)
	var foundInDispatchQueue bool
	for _, dq := range dispatchQueue {
		if dq.FulfillmentID == fulfillmentID {
			foundInDispatchQueue = true
			assert.Equal(t, "packed", dq.Status)
			assert.Equal(t, "packed", dq.OrderStatus)
			assert.Equal(t, 3, dq.TotalQuantity)
			assert.NotNil(t, dq.PackedAt)
		}
	}
	assert.True(t, foundInDispatchQueue, "Packed fulfillment MUST appear in dispatch queue")

	// Dispatch context read model
	dispatchCtx, err := f.svc.GetDispatchContext(ctx, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, dispatchCtx)
	assert.Equal(t, "packed", dispatchCtx.Status)
	assert.Equal(t, "Иван Иванов", *dispatchCtx.CustomerName)
	assert.Equal(t, "+79991234567", *dispatchCtx.CustomerPhone)
	assert.Equal(t, "г. Москва, ул. Складская, 1", *dispatchCtx.DeliveryAddress)
	require.Len(t, dispatchCtx.Items, 2)

	// Step K: Final Warehouse Invariant Verification
	// Critical truth: physical total_stock and reserved_stock remain held inside the warehouse
	// until physical shipment/dispatch! They are NEVER decremented at picking or packing time!
	require.Equal(t, 1, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID), "reserved_stock must stay held")
	require.Equal(t, 2, f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID), "reserved_stock must stay held")
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varS.VariantID), "total_stock must not be decremented before dispatch")
	require.Equal(t, 10, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id = $1`, varM.VariantID), "total_stock must not be decremented before dispatch")
	require.Equal(t, 2, f.count(`SELECT count(*) FROM reservations WHERE order_id = $1 AND status = 'converted'`, orderID), "converted reservations must not be modified")
}

// ============================================================================
// 2. NEGATIVE SCENARIOS
// ============================================================================

func TestFBO_E2E_NegativeScenarios(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)

	prodID, variants := f.createProductWithVariants([]testVariantInfo{
		{Size: "M", Color: "Черный", Price: 300000},
		{Size: "L", Color: "Черный", Price: 300000},
	}, 50)
	varM := variants[0]
	varL := variants[1]

	createSingleUnitOrder := func(orderStatus, fulfillmentStatus string) (uuid.UUID, uuid.UUID, uuid.UUID, string) {
		uID, uCode := f.createSerializedUnit(varM.VariantID)
		spec := []orderItemSpec{
			{
				VariantInfo: varM,
				Quantity:    1,
				SerializedUnits: []struct {
					UnitID   uuid.UUID
					UnitCode string
				}{
					{UnitID: uID, UnitCode: uCode},
				},
			},
		}
		oID, fID, itemIDs := f.createOrderWithFulfillment(orderStatus, fulfillmentStatus, prodID, spec)
		return oID, fID, itemIDs[0], uCode
	}

	t.Run("Negative A: Picking unpaid order is rejected", func(t *testing.T) {
		_, fulfillmentID, itemID, unitCode := createSingleUnitOrder("awaiting_payment", "awaiting_payment")
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode, &itemID)
		assert.ErrorIs(t, err, fulfillment.ErrPickingNotAllowed, "Unpaid fulfillment picking must be rejected")
	})

	t.Run("Negative B: Picking cancelled order is rejected", func(t *testing.T) {
		_, fulfillmentID, itemID, unitCode := createSingleUnitOrder("cancelled", "cancelled")
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode, &itemID)
		assert.ErrorIs(t, err, fulfillment.ErrPickingNotAllowed, "Cancelled fulfillment picking must be rejected")
	})

	t.Run("Negative C: Premature packing of unpicked fulfillment is rejected", func(t *testing.T) {
		_, fulfillmentID, _, _ := createSingleUnitOrder("assembling", "assembling")
		res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFullyPicked, "Packing unpicked items must be rejected")
	})

	t.Run("Negative D: Packing order not in assembling state is rejected", func(t *testing.T) {
		_, fulfillmentID, _, _ := createSingleUnitOrder("paid", "paid")
		res, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, fulfillment.ErrPackingNotAllowed, "Packing non-assembling fulfillment must be rejected")
	})

	t.Run("Negative E: Scanning unit allocated to another order is rejected", func(t *testing.T) {
		_, _, _, unitCode1 := createSingleUnitOrder("paid", "paid")
		_, ful2_ID, _, _ := createSingleUnitOrder("paid", "paid")

		// Worker on Order 2 tries to scan unit belonging to Order 1
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, ful2_ID, unitCode1, nil)
		assert.ErrorIs(t, err, fulfillment.ErrUnitAllocatedToOtherOrder, "Scanning unit of another order must be rejected")
	})

	t.Run("Negative F: Scanning unit not in warehouse status is rejected", func(t *testing.T) {
		damagedUnit_ID, damagedUnit_Code := f.createSerializedUnit(varM.VariantID)
		f.exec(`UPDATE inventory_units SET status = 'damaged' WHERE id = $1`, damagedUnit_ID)

		itemsSpecDamaged := []orderItemSpec{
			{
				VariantInfo: varM,
				Quantity:    1,
				SerializedUnits: []struct {
					UnitID   uuid.UUID
					UnitCode string
				}{
					{UnitID: damagedUnit_ID, UnitCode: damagedUnit_Code},
				},
			},
		}
		_, fulID, itemIDs := f.createOrderWithFulfillment("paid", "paid", prodID, itemsSpecDamaged)

		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, damagedUnit_Code, &itemIDs[0])
		assert.ErrorIs(t, err, fulfillment.ErrUnitNotInWarehouse, "Scanning non-warehouse unit must be rejected")
	})

	t.Run("Negative G: Barcode scan on serialized item is rejected", func(t *testing.T) {
		_, fulID, itemID, _ := createSingleUnitOrder("paid", "paid")

		// Operator scans variant EAN barcode instead of unit ZMU
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, varM.Barcode, &itemID)
		assert.ErrorIs(t, err, fulfillment.ErrCannotPickSerializedWithBarcode, "Serialized item cannot be picked with EAN barcode")
	})

	t.Run("Negative H: Repeated picking scan is idempotent (already picked)", func(t *testing.T) {
		_, fulID, itemID, unitCode := createSingleUnitOrder("paid", "paid")

		// First scan
		res1, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, unitCode, &itemID)
		require.NoError(t, err)
		assert.True(t, res1.ScanResult.NewlyPicked)

		// Second identical scan
		res2, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, unitCode, &itemID)
		require.NoError(t, err)
		assert.False(t, res2.ScanResult.NewlyPicked)
		assert.True(t, res2.ScanResult.AlreadyPicked, "Repeated scan must report already picked without error or double pick")
	})

	t.Run("Negative I: Duplicate pack attempt is rejected", func(t *testing.T) {
		_, fulID, itemID, unitCode := createSingleUnitOrder("paid", "paid")

		// Pick unit
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, unitCode, &itemID)
		require.NoError(t, err)

		// First pack
		res1, err := f.svc.PackFulfillment(ctx, f.adminID, fulID)
		require.NoError(t, err)
		assert.Equal(t, "packed", res1.FulfillmentStatus)

		// Second pack attempt
		res2, err := f.svc.PackFulfillment(ctx, f.adminID, fulID)
		assert.Nil(t, res2)
		assert.ErrorIs(t, err, fulfillment.ErrPackingNotAllowed, "Duplicate packing call must be rejected")
	})

	t.Run("Negative J: Malformed scanner code is rejected cleanly", func(t *testing.T) {
		_, fulID, itemID, _ := createSingleUnitOrder("paid", "paid")
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, "INVALID_MALFORMED_CODE_$$$!", &itemID)
		assert.ErrorIs(t, err, fulfillment.ErrMalformedScannerCode, "Malformed scanner code must be rejected")
	})

	t.Run("Negative K: Scanning unit of wrong variant fails with ErrUnitVariantMismatch", func(t *testing.T) {
		_, fulID, itemID, _ := createSingleUnitOrder("paid", "paid")
		_, wrongUnitCode := f.createSerializedUnit(varL.VariantID)
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, wrongUnitCode, &itemID)
		assert.ErrorIs(t, err, fulfillment.ErrUnitVariantMismatch, "Scanning unit of different variant must be rejected")
	})
}

// ============================================================================
// 3. CONCURRENCY & RACE HARDENING
// ============================================================================

func TestFBO_E2E_ConcurrentPickingAndPackingRace(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)

	prodID, variants := f.createProductWithVariants([]testVariantInfo{
		{Size: "XL", Color: "Хаки", Price: 450000},
	}, 10)
	varXL := variants[0]

	t.Run("Concurrency Race 1: Two workers simultaneously scan the same unpicked unit", func(t *testing.T) {
		unitID, unitCode := f.createSerializedUnit(varXL.VariantID)
		spec := []orderItemSpec{
			{
				VariantInfo: varXL,
				Quantity:    1,
				SerializedUnits: []struct {
					UnitID   uuid.UUID
					UnitCode string
				}{
					{UnitID: unitID, UnitCode: unitCode},
				},
			},
		}
		_, fulID, itemIDs := f.createOrderWithFulfillment("paid", "paid", prodID, spec)

		worker1 := uuid.New()
		worker2 := uuid.New()
		f.exec(`INSERT INTO users (id, name, email, password_hash, role, status) VALUES ($1, 'Worker 1', $2, 'h', 'admin', 'active')`, worker1, "w1-"+worker1.String()[:8]+"@zamk.internal")
		f.exec(`INSERT INTO users (id, name, email, password_hash, role, status) VALUES ($1, 'Worker 2', $2, 'h', 'admin', 'active')`, worker2, "w2-"+worker2.String()[:8]+"@zamk.internal")

		startGate := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		var res1, res2 *fulfillment.PickingScanResult
		var err1, err2 error

		go func() {
			defer wg.Done()
			<-startGate
			res1, err1 = f.svc.ScanPickingCode(ctx, worker1, fulID, unitCode, &itemIDs[0])
		}()

		go func() {
			defer wg.Done()
			<-startGate
			res2, err2 = f.svc.ScanPickingCode(ctx, worker2, fulID, unitCode, &itemIDs[0])
		}()

		close(startGate)
		wg.Wait()

		require.NoError(t, err1)
		require.NoError(t, err2)

		// Exactly one worker must have newly picked the unit, and the other must get already picked
		newlyPickedCount := 0
		if res1 != nil && res1.ScanResult.NewlyPicked {
			newlyPickedCount++
		}
		if res2 != nil && res2.ScanResult.NewlyPicked {
			newlyPickedCount++
		}
		assert.Equal(t, 1, newlyPickedCount, "Exactly one worker must get NewlyPicked=true")

		alreadyPickedCount := 0
		if res1 != nil && res1.ScanResult.AlreadyPicked {
			alreadyPickedCount++
		}
		if res2 != nil && res2.ScanResult.AlreadyPicked {
			alreadyPickedCount++
		}
		assert.Equal(t, 1, alreadyPickedCount, "Exactly one worker must get AlreadyPicked=true")

		// Allocation picked_at must be set exactly once
		require.Equal(t, 1, f.count(`SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1 AND picked_at IS NOT NULL`, unitID))
	})

	t.Run("Concurrency Race 2: Two workers simultaneously call PackFulfillment", func(t *testing.T) {
		unitID, unitCode := f.createSerializedUnit(varXL.VariantID)
		spec := []orderItemSpec{
			{
				VariantInfo: varXL,
				Quantity:    1,
				SerializedUnits: []struct {
					UnitID   uuid.UUID
					UnitCode string
				}{
					{UnitID: unitID, UnitCode: unitCode},
				},
			},
		}
		orderID, fulID, itemIDs := f.createOrderWithFulfillment("paid", "paid", prodID, spec)

		// Pre-pick the unit
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, unitCode, &itemIDs[0])
		require.NoError(t, err)

		workerA := uuid.New()
		workerB := uuid.New()
		f.exec(`INSERT INTO users (id, name, email, password_hash, role, status) VALUES ($1, 'Worker A', $2, 'h', 'admin', 'active')`, workerA, "wa-"+workerA.String()[:8]+"@zamk.internal")
		f.exec(`INSERT INTO users (id, name, email, password_hash, role, status) VALUES ($1, 'Worker B', $2, 'h', 'admin', 'active')`, workerB, "wb-"+workerB.String()[:8]+"@zamk.internal")

		startGate := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		var packResA, packResB *fulfillment.PackResult
		var packErrA, packErrB error

		go func() {
			defer wg.Done()
			<-startGate
			packResA, packErrA = f.svc.PackFulfillment(ctx, workerA, fulID)
		}()

		go func() {
			defer wg.Done()
			<-startGate
			packResB, packErrB = f.svc.PackFulfillment(ctx, workerB, fulID)
		}()

		close(startGate)
		wg.Wait()

		successes := 0
		if packErrA == nil && packResA != nil && packResA.FulfillmentStatus == "packed" {
			successes++
		}
		if packErrB == nil && packResB != nil && packResB.FulfillmentStatus == "packed" {
			successes++
		}
		assert.Equal(t, 1, successes, "Exactly one pack call must succeed")

		rejections := 0
		if errors.Is(packErrA, fulfillment.ErrPackingNotAllowed) {
			rejections++
		}
		if errors.Is(packErrB, fulfillment.ErrPackingNotAllowed) {
			rejections++
		}
		assert.Equal(t, 1, rejections, "The concurrent pack call must be safely rejected with ErrPackingNotAllowed")

		// Persistent database state must be packed exactly once
		require.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		require.Equal(t, "packed", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
	})
}

// ============================================================================
// 4. LEGACY / NON-SERIALIZED AGGREGATE PICKING & PACKING
// ============================================================================

func TestFBO_E2E_LegacyItem_PickingAndPacking(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)

	prodID, variants := f.createProductWithVariants([]testVariantInfo{
		{Size: "OneSize", Color: "Бежевый", Price: 150000},
	}, 20)
	varItem := variants[0]

	// Order with 3 legacy units (no initial serialized inventory_units)
	spec := []orderItemSpec{
		{
			VariantInfo:     varItem,
			Quantity:        3,
			SerializedUnits: nil, // Legacy: 0 serialized unit allocations
		},
	}
	orderID, fulID, itemIDs := f.createOrderWithFulfillment("paid", "paid", prodID, spec)
	legacyItemID := itemIDs[0]

	// Operator scans barcode 3 times
	for i := 1; i <= 3; i++ {
		res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, varItem.Barcode, &legacyItemID)
		require.NoError(t, err)
		assert.True(t, res.ScanResult.NewlyPicked)
		assert.Equal(t, "legacy", res.ScanResult.Type)
		assert.Equal(t, i, res.Item.PickedQuantity)
		assert.Equal(t, 3-i, res.Item.RemainingQuantity)
	}

	// 4th scan attempt: already complete, no over-picking
	res4, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, varItem.Barcode, &legacyItemID)
	require.NoError(t, err)
	assert.False(t, res4.ScanResult.NewlyPicked)
	assert.True(t, res4.ScanResult.AlreadyComplete, "Over-picking must be blocked with AlreadyComplete")
	assert.Equal(t, 3, res4.Item.PickedQuantity)

	// Packing legacy items
	packRes, err := f.svc.PackFulfillment(ctx, f.adminID, fulID)
	require.NoError(t, err)
	assert.Equal(t, "packed", packRes.FulfillmentStatus)
	assert.Equal(t, "packed", packRes.OrderStatus)

	// Verification
	require.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
	require.Equal(t, "packed", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
}

// NOTE on ORDER.LIFECYCLE.1 Open Race Condition:
// As documented in ORDER.LIFECYCLE.1, tests H and I verify deterministic resolution of
// either payment first or expiration first. True simultaneous goroutine collision between
// payment confirmation and background worker cancellation remains an open concurrency risk
// to be comprehensively tested and hardened in a future milestone.
