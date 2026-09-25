package fulfillment_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type dispatchTruthFixture struct {
	t        *testing.T
	ctx      context.Context
	db       *pgxpool.Pool
	svc      *fulfillment.Service
	sellerID uuid.UUID
	adminID  uuid.UUID

	mu sync.Mutex

	sellerIDs        []uuid.UUID
	userIDs          []uuid.UUID
	categoryIDs      []uuid.UUID
	productIDs       []uuid.UUID
	variantIDs       []uuid.UUID
	inventoryItemIDs []uuid.UUID
	supplyIDs        []uuid.UUID
	supplyItemIDs    []uuid.UUID
	inventoryUnitIDs []uuid.UUID
	orderIDs         []uuid.UUID
	fulfillmentIDs   []uuid.UUID
	orderItemIDs     []uuid.UUID
	allocationIDs    []uuid.UUID
	shipmentIDs      []uuid.UUID
	reservationIDs   []uuid.UUID
}

func (f *dispatchTruthFixture) track(slice *[]uuid.UUID, id uuid.UUID) uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	*slice = append(*slice, id)
	return id
}

func newDispatchTruthFixture(t *testing.T, ctx context.Context) *dispatchTruthFixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	require.NotEmpty(t, dbURL, "test database URL must not be empty")

	postgresClient, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)

	// HARD GUARD: Verify connected database is strictly zamk_test BEFORE ANY MUTATION
	testutil.AssertTestDatabase(t, postgresClient.Pool)

	db := postgresClient.Pool
	repo := fulfillment.NewRepository(db)
	ordersRepo := orders.NewRepository(db)
	svc := fulfillment.NewService(repo, ordersRepo, postgresClient, nil, nil)

	f := &dispatchTruthFixture{
		t:   t,
		ctx: ctx,
		db:  db,
		svc: svc,
	}

	// Register scoped cleanup BEFORE the first database mutation
	t.Cleanup(func() {
		f.cleanup(context.Background())
		postgresClient.Close()
	})

	// Create baseline seller and admin
	f.sellerID = f.createSeller()
	f.adminID = f.createAdmin()

	return f
}

func (f *dispatchTruthFixture) cleanup(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()

	type cleanEntry struct {
		table string
		query string
		ids   []uuid.UUID
	}

	entries := []cleanEntry{
		{"shipment_events", `DELETE FROM shipment_events WHERE shipment_id = ANY($1)`, f.shipmentIDs},
		{"shipments", `DELETE FROM shipments WHERE id = ANY($1)`, f.shipmentIDs},
		{"order_item_allocations", `DELETE FROM order_item_allocations WHERE id = ANY($1)`, f.allocationIDs},
		{"order_status_history", `DELETE FROM order_status_history WHERE order_id = ANY($1)`, f.orderIDs},
		{"order_reservations", `DELETE FROM order_reservations WHERE order_id = ANY($1)`, f.orderIDs},
		{"reservations", `DELETE FROM reservations WHERE id = ANY($1)`, f.reservationIDs},
		{"order_items", `DELETE FROM order_items WHERE id = ANY($1)`, f.orderItemIDs},
		{"order_fulfillments", `DELETE FROM order_fulfillments WHERE id = ANY($1)`, f.fulfillmentIDs},
		{"orders", `DELETE FROM orders WHERE id = ANY($1)`, f.orderIDs},
		{"inventory_units", `DELETE FROM inventory_units WHERE id = ANY($1)`, f.inventoryUnitIDs},
		{"seller_supply_items", `DELETE FROM seller_supply_items WHERE id = ANY($1)`, f.supplyItemIDs},
		{"seller_supplies", `DELETE FROM seller_supplies WHERE id = ANY($1)`, f.supplyIDs},
		{"inventory_items", `DELETE FROM inventory_items WHERE id = ANY($1)`, f.inventoryItemIDs},
		{"product_variants", `DELETE FROM product_variants WHERE id = ANY($1)`, f.variantIDs},
		{"products", `DELETE FROM products WHERE id = ANY($1)`, f.productIDs},
		{"categories", `DELETE FROM categories WHERE id = ANY($1)`, f.categoryIDs},
		{"users", `DELETE FROM users WHERE id = ANY($1)`, f.userIDs},
		{"sellers", `DELETE FROM sellers WHERE id = ANY($1)`, f.sellerIDs},
	}

	for _, e := range entries {
		if len(e.ids) == 0 {
			continue
		}
		_, err := f.db.Exec(ctx, e.query, e.ids)
		if err != nil {
			f.t.Logf("cleanup error on %s: %v", e.table, err)
		}
	}
}

func (f *dispatchTruthFixture) createSeller() uuid.UUID {
	id := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Truth Seller', $2, $3, 'active', now(), now())
	`, id, fmt.Sprintf("seller-%s", id), fmt.Sprintf("seller-%s@example.com", id))
	require.NoError(f.t, err)
	return f.track(&f.sellerIDs, id)
}

func (f *dispatchTruthFixture) createAdmin() uuid.UUID {
	id := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Truth Admin', $2, 'hash', 'admin', 'active', now(), now())
	`, id, fmt.Sprintf("admin-%s@example.com", id))
	require.NoError(f.t, err)
	return f.track(&f.userIDs, id)
}

func (f *dispatchTruthFixture) createCustomer() uuid.UUID {
	id := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Truth Customer', $2, 'hash', 'customer', 'active', now(), now())
	`, id, fmt.Sprintf("cust-%s@example.com", id))
	require.NoError(f.t, err)
	return f.track(&f.userIDs, id)
}

func (f *dispatchTruthFixture) createOrderAndFulfillment(orderStatus, fulfillmentStatus string) (uuid.UUID, uuid.UUID) {
	customerID := f.createCustomer()
	orderID := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 1000, 'Customer Name', '+79990001122', 'cust@example.com', 'Warehouse Blvd 1', now(), now())
	`, orderID, customerID, orderStatus)
	require.NoError(f.t, err)
	f.track(&f.orderIDs, orderID)

	fulfillmentID := uuid.New()
	_, err = f.db.Exec(f.ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1000, 900, 900, now(), now())
	`, fulfillmentID, orderID, f.sellerID, fulfillmentStatus)
	require.NoError(f.t, err)
	f.track(&f.fulfillmentIDs, fulfillmentID)

	return orderID, fulfillmentID
}

func (f *dispatchTruthFixture) createProductAndVariant() (uuid.UUID, uuid.UUID) {
	catID := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Cat', $2, now(), now())
	`, catID, fmt.Sprintf("cat-%s", catID))
	require.NoError(f.t, err)
	f.track(&f.categoryIDs, catID)

	prodID := uuid.New()
	_, err = f.db.Exec(f.ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Truth Product', $4, 1000, 'published', now(), now())
	`, prodID, f.sellerID, catID, fmt.Sprintf("prod-%s", prodID))
	require.NoError(f.t, err)
	f.track(&f.productIDs, prodID)

	variantID := uuid.New()
	barcode := fmt.Sprintf("46%011d", rand.Int63n(90000000000)+10000000000)
	sku := fmt.Sprintf("SKU-%s", uuid.New().String()[:8])
	_, err = f.db.Exec(f.ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())
	`, variantID, prodID, sku, sku, barcode)
	require.NoError(f.t, err)
	f.track(&f.variantIDs, variantID)

	return prodID, variantID
}

func (f *dispatchTruthFixture) createOrderItem(orderID, fulfillmentID, prodID, variantID uuid.UUID, quantity, pickedQuantity int) uuid.UUID {
	itemID := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity, created_at)
		VALUES ($1, $2, $3, $4, $5, 'Item Title', 'slug', 100, $6, 100, $7, $8, now())
	`, itemID, orderID, prodID, variantID, f.sellerID, quantity, fulfillmentID, pickedQuantity)
	require.NoError(f.t, err)
	return f.track(&f.orderItemIDs, itemID)
}

func (f *dispatchTruthFixture) createInventoryItem(prodID, variantID uuid.UUID, totalStock, reservedStock int) uuid.UUID {
	invID := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = $5, reserved_stock = $6
	`, invID, prodID, variantID, f.sellerID, totalStock, reservedStock)
	require.NoError(f.t, err)
	return f.track(&f.inventoryItemIDs, invID)
}

func (f *dispatchTruthFixture) createSupplyAndItem(variantID uuid.UUID) (uuid.UUID, uuid.UUID) {
	supplyID := uuid.New()
	_, err := f.db.Exec(f.ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, f.sellerID, fmt.Sprintf("SUP-%s", supplyID.String()[:8]))
	require.NoError(f.t, err)
	f.track(&f.supplyIDs, supplyID)

	supplyItemID := uuid.New()
	_, err = f.db.Exec(f.ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 1, now(), now())
	`, supplyItemID, supplyID, variantID)
	require.NoError(f.t, err)
	f.track(&f.supplyItemIDs, supplyItemID)

	return supplyID, supplyItemID
}

func (f *dispatchTruthFixture) createAllocatedUnit(orderItemID, variantID uuid.UUID, unitStatus string, picked bool) (uuid.UUID, string, uuid.UUID) {
	supplyID, supplyItemID := f.createSupplyAndItem(variantID)

	unitID := uuid.New()
	unitCode, err := supplies.GenerateUnitCode()
	require.NoError(f.t, err)

	_, err = f.db.Exec(f.ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, $6, now(), now())
	`, unitID, unitCode, variantID, supplyID, supplyItemID, unitStatus)
	require.NoError(f.t, err)
	f.track(&f.inventoryUnitIDs, unitID)

	allocID := uuid.New()
	var pickedAt any
	if picked {
		pickedAt = time.Now().UTC()
	}
	_, err = f.db.Exec(f.ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, allocID, orderItemID, unitID, pickedAt)
	require.NoError(f.t, err)
	f.track(&f.allocationIDs, allocID)

	return unitID, unitCode, allocID
}

func (f *dispatchTruthFixture) createUnallocatedUnit(variantID uuid.UUID, unitStatus string) (uuid.UUID, string) {
	supplyID, supplyItemID := f.createSupplyAndItem(variantID)

	unitID := uuid.New()
	unitCode, err := supplies.GenerateUnitCode()
	require.NoError(f.t, err)

	_, err = f.db.Exec(f.ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, $6, now(), now())
	`, unitID, unitCode, variantID, supplyID, supplyItemID, unitStatus)
	require.NoError(f.t, err)
	f.track(&f.inventoryUnitIDs, unitID)

	return unitID, unitCode
}

// ------------------------------------------------------------------------------------------------
// SECTION 3: SERIALIZED SUCCESS CASE
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_Serialized_Success(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
	prodID, variantID := f.createProductAndVariant()
	itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0)

	totalBefore := 10
	reservedBefore := 5
	f.createInventoryItem(prodID, variantID, totalBefore, reservedBefore)

	unit1, _, alloc1 := f.createAllocatedUnit(itemID, variantID, "warehouse", true)
	unit2, _, alloc2 := f.createAllocatedUnit(itemID, variantID, "warehouse", true)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)
	f.track(&f.shipmentIDs, res.ShipmentID)

	// 1. Fulfillment status -> shipped
	assert.Equal(t, "shipped", res.FulfillmentStatus)
	assert.Equal(t, "shipped", res.OrderStatus)
	assert.Equal(t, "shipped", res.ShipmentStatus)
	assert.NotEqual(t, uuid.Nil, res.ShipmentID)
	assert.WithinDuration(t, time.Now(), res.ShippedAt, 5*time.Second)

	var fStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", fStatus)

	// 2. Canonical shipment and event
	var sStatus string
	var shippedAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT status, shipped_at FROM shipments WHERE id = $1`, res.ShipmentID).Scan(&sStatus, &shippedAt)
	require.NoError(t, err)
	assert.Equal(t, "shipped", sStatus)
	assert.NotNil(t, shippedAt)

	var eventCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'shipped'`, res.ShipmentID).Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount)

	// 3. Exact allocated ZMU: warehouse -> shipped
	for _, uID := range []uuid.UUID{unit1, unit2} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", uStatus, "allocated ZMU must be mutated to shipped")
	}

	// 4. inventory_items.total_stock: before - Q, reserved_stock: before - Q
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, totalBefore-2, totalStock, "total_stock must decrement by Q")
	assert.Equal(t, reservedBefore-2, reservedStock, "reserved_stock must decrement by Q")

	// 5. Available (total - reserved) remains mathematically consistent
	assert.Equal(t, totalBefore-reservedBefore, totalStock-reservedStock, "available stock must remain constant")

	// 6. Allocations remain present as historical evidence (released_at is NULL, picked_at preserved)
	for _, aID := range []uuid.UUID{alloc1, alloc2} {
		var pickedAt *time.Time
		var releasedAt *time.Time
		err = f.db.QueryRow(ctx, `SELECT picked_at, released_at FROM order_item_allocations WHERE id = $1`, aID).Scan(&pickedAt, &releasedAt)
		require.NoError(t, err)
		assert.NotNil(t, pickedAt, "picked_at must be preserved as historical evidence")
		assert.Nil(t, releasedAt, "released_at must remain nil")
	}

	// 7. Parent order status recalculated
	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", oStatus)
}

// ------------------------------------------------------------------------------------------------
// SECTION 4: EXACT UNIT SAFETY (UNRELATED ZMU UNTOUCHED)
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_ExactUnitSafety_UnrelatedUnitsUntouched(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	// Target Order & Fulfillment
	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
	prodID, variantID := f.createProductAndVariant()
	itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0)
	f.createInventoryItem(prodID, variantID, 20, 10)

	// Target units: 2 allocated and picked
	targetUnit1, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)
	targetUnit2, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)

	// Unrelated unallocated warehouse units for the SAME variant
	unrelatedFree1, _ := f.createUnallocatedUnit(variantID, "warehouse")
	unrelatedFree2, _ := f.createUnallocatedUnit(variantID, "warehouse")

	// Another order with an allocated unit for the SAME variant
	otherOrderID, otherFulfillmentID := f.createOrderAndFulfillment("assembling", "assembling")
	otherItemID := f.createOrderItem(otherOrderID, otherFulfillmentID, prodID, variantID, 1, 0)
	otherUnit, _, _ := f.createAllocatedUnit(otherItemID, variantID, "warehouse", true)

	// Dispatch target fulfillment only
	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	f.track(&f.shipmentIDs, res.ShipmentID)

	// Assert ONLY target units are shipped
	for _, uID := range []uuid.UUID{targetUnit1, targetUnit2} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", uStatus, "target unit must be shipped")
	}

	// Assert unrelated free units remain 'warehouse'
	for _, uID := range []uuid.UUID{unrelatedFree1, unrelatedFree2} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", uStatus, "unrelated warehouse unit must remain unchanged")
	}

	// Assert other order's unit remains 'warehouse'
	var otherUnitStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, otherUnit).Scan(&otherUnitStatus)
	require.NoError(t, err)
	assert.Equal(t, "warehouse", otherUnitStatus, "other order unit must remain unchanged")
}

// ------------------------------------------------------------------------------------------------
// SECTION 5: NOT FULLY PICKED MUST FAIL
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_NotFullyPicked_Rejections(t *testing.T) {
	ctx := context.Background()

	t.Run("serialized allocation unpicked -> rejected", func(t *testing.T) {
		f := newDispatchTruthFixture(t, ctx)
		orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
		prodID, variantID := f.createProductAndVariant()
		itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0)
		f.createInventoryItem(prodID, variantID, 10, 5)

		// 1 picked, 1 unpicked
		unit1, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)
		unit2, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", false)

		_, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
		assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFullyPicked)

		// Assert NO mutations occurred
		var fStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
		require.NoError(t, err)
		assert.Equal(t, "packed", fStatus)

		for _, uID := range []uuid.UUID{unit1, unit2} {
			var uStatus string
			err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
			require.NoError(t, err)
			assert.Equal(t, "warehouse", uStatus)
		}

		var totalStock, reservedStock int
		err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
		require.NoError(t, err)
		assert.Equal(t, 10, totalStock)
		assert.Equal(t, 5, reservedStock)
	})

	t.Run("legacy item picked_quantity < quantity -> rejected", func(t *testing.T) {
		f := newDispatchTruthFixture(t, ctx)
		orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
		prodID, variantID := f.createProductAndVariant()
		_ = f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 3, 2) // Q=3, picked=2
		f.createInventoryItem(prodID, variantID, 10, 5)

		_, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
		assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFullyPicked)

		var totalStock, reservedStock int
		err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
		require.NoError(t, err)
		assert.Equal(t, 10, totalStock)
		assert.Equal(t, 5, reservedStock)
	})

	t.Run("partial allocation count != quantity -> rejected invariant violation", func(t *testing.T) {
		f := newDispatchTruthFixture(t, ctx)
		orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
		prodID, variantID := f.createProductAndVariant()
		itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0) // Q=2
		f.createInventoryItem(prodID, variantID, 10, 5)

		// Only 1 allocation for quantity 2
		_, _, _ = f.createAllocatedUnit(itemID, variantID, "warehouse", true)

		_, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
		assert.ErrorIs(t, err, fulfillment.ErrInvariantViolation)
	})
}

// ------------------------------------------------------------------------------------------------
// SECTION 6: DOUBLE DISPATCH — NO DOUBLE DECREMENT
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_DoubleDispatch_NoDoubleDecrement(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
	prodID, variantID := f.createProductAndVariant()
	itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0)
	f.createInventoryItem(prodID, variantID, 10, 5)

	u1, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)
	u2, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)

	// First dispatch -> success
	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	f.track(&f.shipmentIDs, res.ShipmentID)

	var totalAfterFirst, reservedAfterFirst int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalAfterFirst, &reservedAfterFirst)
	require.NoError(t, err)
	assert.Equal(t, 8, totalAfterFirst)
	assert.Equal(t, 3, reservedAfterFirst)

	// Second dispatch -> rejected by status precondition
	_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)

	// Assert NO double decrement after second call
	var totalAfterSecond, reservedAfterSecond int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalAfterSecond, &reservedAfterSecond)
	require.NoError(t, err)
	assert.Equal(t, totalAfterFirst, totalAfterSecond, "stock must NOT double decrement")
	assert.Equal(t, reservedAfterFirst, reservedAfterSecond, "reserved must NOT double decrement")

	// Assert unit statuses unchanged
	for _, uID := range []uuid.UUID{u1, u2} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", uStatus)
	}

	// Exactly 1 shipment and 1 shipment event
	var sCount, seCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulfillmentID).Scan(&sCount)
	require.NoError(t, err)
	assert.Equal(t, 1, sCount)

	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'shipped'`, res.ShipmentID).Scan(&seCount)
	require.NoError(t, err)
	assert.Equal(t, 1, seCount)
}

// ------------------------------------------------------------------------------------------------
// SECTION 7: CONCURRENT DOUBLE DISPATCH (EXACTLY ONE WINS)
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_ConcurrentDoubleDispatch_ExactlyOneWins(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
	prodID, variantID := f.createProductAndVariant()
	itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0)
	f.createInventoryItem(prodID, variantID, 10, 5)

	u1, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)
	u2, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)

	concurrency := 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	failures := 0
	var winningShipmentID uuid.UUID

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successes++
				winningShipmentID = res.ShipmentID
			} else {
				failures++
				assert.True(t, errors.Is(err, fulfillment.ErrDispatchNotAllowed) || errors.Is(err, fulfillment.ErrShipmentContradictoryState))
			}
		}()
	}
	wg.Wait()

	if winningShipmentID != uuid.Nil {
		f.track(&f.shipmentIDs, winningShipmentID)
	}

	assert.Equal(t, 1, successes, "exactly one concurrent dispatch must succeed")
	assert.Equal(t, concurrency-1, failures, "all other concurrent attempts must be rejected")

	// Stock decremented exactly once
	var totalStock, reservedStock int
	err := f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock, "total_stock must be decremented exactly once")
	assert.Equal(t, 3, reservedStock, "reserved_stock must be decremented exactly once")

	// Units shipped
	for _, uID := range []uuid.UUID{u1, u2} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", uStatus)
	}

	// Fulfillment shipped
	var fStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", fStatus)
}

// ------------------------------------------------------------------------------------------------
// SECTION 8: LEGACY PATH
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_LegacyPath_Success(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
	prodID, variantID := f.createProductAndVariant()
	qty := 3
	_ = f.createOrderItem(orderID, fulfillmentID, prodID, variantID, qty, qty) // allocation count = 0, picked_quantity = qty

	totalBefore := 15
	reservedBefore := 8
	f.createInventoryItem(prodID, variantID, totalBefore, reservedBefore)

	// Also have an unrelated warehouse ZMU for another variant to prove legacy dispatch doesn't touch any ZMUs
	_, otherVarID := f.createProductAndVariant()
	unrelatedUnit, _ := f.createUnallocatedUnit(otherVarID, "warehouse")

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)
	f.track(&f.shipmentIDs, res.ShipmentID)

	assert.Equal(t, "shipped", res.FulfillmentStatus)
	assert.Equal(t, "shipped", res.OrderStatus)

	// total_stock and reserved_stock decremented by legacy quantity
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, totalBefore-qty, totalStock)
	assert.Equal(t, reservedBefore-qty, reservedStock)

	// NO inventory_unit rows created for this variant
	var unitCountForVariant int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM inventory_units WHERE product_variant_id = $1`, variantID).Scan(&unitCountForVariant)
	require.NoError(t, err)
	assert.Equal(t, 0, unitCountForVariant, "legacy dispatch must create NO fake inventory_units")

	// Unrelated ZMU untouched
	var unrelatedStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, unrelatedUnit).Scan(&unrelatedStatus)
	require.NoError(t, err)
	assert.Equal(t, "warehouse", unrelatedStatus)
}

// ------------------------------------------------------------------------------------------------
// SECTION 9: MIXED ITEM CLASSIFICATION (SERIALIZED + LEGACY IN SAME FULFILLMENT)
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_MixedItemClassification_Supported(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")

	// Item 1: Serialized, Q=2
	prodID1, variantID1 := f.createProductAndVariant()
	item1ID := f.createOrderItem(orderID, fulfillmentID, prodID1, variantID1, 2, 0)
	f.createInventoryItem(prodID1, variantID1, 10, 5)
	u1, _, _ := f.createAllocatedUnit(item1ID, variantID1, "warehouse", true)
	u2, _, _ := f.createAllocatedUnit(item1ID, variantID1, "warehouse", true)

	// Item 2: Legacy, Q=3 (0 allocations, picked_quantity=3)
	prodID2, variantID2 := f.createProductAndVariant()
	_ = f.createOrderItem(orderID, fulfillmentID, prodID2, variantID2, 3, 3)
	f.createInventoryItem(prodID2, variantID2, 20, 8)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)
	f.track(&f.shipmentIDs, res.ShipmentID)

	assert.Equal(t, "shipped", res.FulfillmentStatus)
	assert.Equal(t, "shipped", res.OrderStatus)

	// Variant 1 (serialized) inventory & units
	var total1, res1 int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID1).Scan(&total1, &res1)
	require.NoError(t, err)
	assert.Equal(t, 8, total1)
	assert.Equal(t, 3, res1)

	for _, uID := range []uuid.UUID{u1, u2} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", uStatus)
	}

	// Variant 2 (legacy) inventory
	var total2, res2 int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID2).Scan(&total2, &res2)
	require.NoError(t, err)
	assert.Equal(t, 17, total2)
	assert.Equal(t, 5, res2)

	var legacyUnitCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM inventory_units WHERE product_variant_id = $1`, variantID2).Scan(&legacyUnitCount)
	require.NoError(t, err)
	assert.Equal(t, 0, legacyUnitCount, "no units created for legacy item")
}

// ------------------------------------------------------------------------------------------------
// SECTION 10: ATOMIC ROLLBACK (NO PARTIAL MUTATIONS)
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_AtomicRollback_NoPartialMutations(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")

	// Generate two variants such that variantA string < variantB string to guarantee processing order
	var prodIDA, variantIDA, prodIDB, variantIDB uuid.UUID
	for {
		p1, v1 := f.createProductAndVariant()
		p2, v2 := f.createProductAndVariant()
		if v1.String() < v2.String() {
			prodIDA, variantIDA = p1, v1
			prodIDB, variantIDB = p2, v2
			break
		}
	}

	// Item A (valid, processed first)
	itemA := f.createOrderItem(orderID, fulfillmentID, prodIDA, variantIDA, 2, 0)
	f.createInventoryItem(prodIDA, variantIDA, 10, 5)
	unitA1, _, _ := f.createAllocatedUnit(itemA, variantIDA, "warehouse", true)
	unitA2, _, _ := f.createAllocatedUnit(itemA, variantIDA, "warehouse", true)

	// Item B (processed second, INSUFFICIENT reserved stock: reserved=1, required=2)
	itemB := f.createOrderItem(orderID, fulfillmentID, prodIDB, variantIDB, 2, 0)
	f.createInventoryItem(prodIDB, variantIDB, 10, 1) // reserved=1 < 2!
	unitB1, _, _ := f.createAllocatedUnit(itemB, variantIDB, "warehouse", true)
	unitB2, _, _ := f.createAllocatedUnit(itemB, variantIDB, "warehouse", true)

	// Attempt dispatch: will fail when processing variant B
	_, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	assert.ErrorIs(t, err, fulfillment.ErrInsufficientReservedStock)

	// Assert ATOMIC ROLLBACK: Variant A's stock was NOT decremented
	var totalA, resA int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantIDA).Scan(&totalA, &resA)
	require.NoError(t, err)
	assert.Equal(t, 10, totalA, "Variant A stock must NOT be decremented")
	assert.Equal(t, 5, resA, "Variant A reserved must NOT be decremented")

	// Variant A's units must still be 'warehouse'
	for _, uID := range []uuid.UUID{unitA1, unitA2, unitB1, unitB2} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", uStatus, "units must remain in warehouse")
	}

	// Fulfillment status remains 'packed'
	var fStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "packed", fStatus)

	// No shipment row created
	var sCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulfillmentID).Scan(&sCount)
	require.NoError(t, err)
	assert.Equal(t, 0, sCount)
}

// ------------------------------------------------------------------------------------------------
// SECTION 11: MULTI-ITEM FULFILLMENT ATOMICITY
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_MultiItemFulfillment_Atomicity(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")

	prodID1, variantID1 := f.createProductAndVariant()
	item1ID := f.createOrderItem(orderID, fulfillmentID, prodID1, variantID1, 2, 0)
	f.createInventoryItem(prodID1, variantID1, 10, 5)
	u1, _, _ := f.createAllocatedUnit(item1ID, variantID1, "warehouse", true)
	u2, _, _ := f.createAllocatedUnit(item1ID, variantID1, "warehouse", true)

	prodID2, variantID2 := f.createProductAndVariant()
	item2ID := f.createOrderItem(orderID, fulfillmentID, prodID2, variantID2, 1, 0)
	f.createInventoryItem(prodID2, variantID2, 15, 6)
	u3, _, _ := f.createAllocatedUnit(item2ID, variantID2, "warehouse", true)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	require.NotNil(t, res)
	f.track(&f.shipmentIDs, res.ShipmentID)

	assert.Equal(t, "shipped", res.FulfillmentStatus)

	// Item 1 decremented by 2
	var total1, res1 int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID1).Scan(&total1, &res1)
	require.NoError(t, err)
	assert.Equal(t, 8, total1)
	assert.Equal(t, 3, res1)

	// Item 2 decremented by 1
	var total2, res2 int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID2).Scan(&total2, &res2)
	require.NoError(t, err)
	assert.Equal(t, 14, total2)
	assert.Equal(t, 5, res2)

	// All units shipped
	for _, uID := range []uuid.UUID{u1, u2, u3} {
		var uStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", uStatus)
	}
}

// ------------------------------------------------------------------------------------------------
// SECTION 12: PARENT ORDER STATUS BEHAVIOR
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_ParentOrderStatus_Transitions(t *testing.T) {
	ctx := context.Background()

	t.Run("single fulfillment: packed -> dispatch -> parent shipped", func(t *testing.T) {
		f := newDispatchTruthFixture(t, ctx)
		orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
		prodID, variantID := f.createProductAndVariant()
		item1 := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 1, 1)
		f.createInventoryItem(prodID, variantID, 10, 5)
		_ = item1

		res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
		require.NoError(t, err)
		f.track(&f.shipmentIDs, res.ShipmentID)

		assert.Equal(t, "shipped", res.FulfillmentStatus)
		assert.Equal(t, "shipped", res.OrderStatus)

		var oStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", oStatus)
	})

	t.Run("multiple fulfillments: one shipped + sibling assembling -> parent assembling", func(t *testing.T) {
		f := newDispatchTruthFixture(t, ctx)
		orderID, f1ID := f.createOrderAndFulfillment("assembling", "packed")

		seller2ID := f.createSeller()
		f2ID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'assembling', 1000, 900, 900, now(), now())
		`, f2ID, orderID, seller2ID)
		require.NoError(t, err)
		f.track(&f.fulfillmentIDs, f2ID)

		prodID, variantID := f.createProductAndVariant()
		_ = f.createOrderItem(orderID, f1ID, prodID, variantID, 1, 1)
		f.createInventoryItem(prodID, variantID, 10, 5)

		res, err := f.svc.DispatchFulfillment(ctx, f.adminID, f1ID)
		require.NoError(t, err)
		f.track(&f.shipmentIDs, res.ShipmentID)

		assert.Equal(t, "shipped", res.FulfillmentStatus)
		assert.Equal(t, "assembling", res.OrderStatus, "parent remains assembling while sibling is assembling")
	})

	t.Run("multiple fulfillments: one shipped + sibling packed -> parent packed", func(t *testing.T) {
		f := newDispatchTruthFixture(t, ctx)
		orderID, f1ID := f.createOrderAndFulfillment("packed", "packed")

		seller2ID := f.createSeller()
		f2ID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'packed', 1000, 900, 900, now(), now())
		`, f2ID, orderID, seller2ID)
		require.NoError(t, err)
		f.track(&f.fulfillmentIDs, f2ID)

		prodID1, variantID1 := f.createProductAndVariant()
		_ = f.createOrderItem(orderID, f1ID, prodID1, variantID1, 1, 1)
		f.createInventoryItem(prodID1, variantID1, 10, 5)

		prodID2, variantID2 := f.createProductAndVariant()
		_ = f.createOrderItem(orderID, f2ID, prodID2, variantID2, 1, 1)
		f.createInventoryItem(prodID2, variantID2, 10, 5)

		res1, err := f.svc.DispatchFulfillment(ctx, f.adminID, f1ID)
		require.NoError(t, err)
		f.track(&f.shipmentIDs, res1.ShipmentID)
		assert.Equal(t, "shipped", res1.FulfillmentStatus)
		assert.Equal(t, "packed", res1.OrderStatus, "parent is packed while sibling is packed")

		res2, err := f.svc.DispatchFulfillment(ctx, f.adminID, f2ID)
		require.NoError(t, err)
		f.track(&f.shipmentIDs, res2.ShipmentID)
		assert.Equal(t, "shipped", res2.FulfillmentStatus)
		assert.Equal(t, "shipped", res2.OrderStatus, "all fulfillments shipped -> parent shipped")
	})
}

// ------------------------------------------------------------------------------------------------
// SECTION 13: FINANCE NON-INTERACTION (SHIPMENT DOES NOT CREATE DELIVERY FINANCE)
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_FinanceNonInteraction(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
	prodID, variantID := f.createProductAndVariant()
	itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0)
	f.createInventoryItem(prodID, variantID, 10, 5)
	_, _, _ = f.createAllocatedUnit(itemID, variantID, "warehouse", true)
	_, _, _ = f.createAllocatedUnit(itemID, variantID, "warehouse", true)

	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)
	f.track(&f.shipmentIDs, res.ShipmentID)

	// 1. Assert ZERO seller ledger entries created
	var ledgerCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE order_id = $1`, orderID).Scan(&ledgerCount)
	require.NoError(t, err)
	assert.Equal(t, 0, ledgerCount, "dispatch must NOT create seller_ledger_entries")

	// 2. Assert ZERO payout batches
	var payoutCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM payout_batches WHERE seller_id = $1`, f.sellerID).Scan(&payoutCount)
	require.NoError(t, err)
	assert.Equal(t, 0, payoutCount, "dispatch must NOT create payout_batches")

	// 3. Parent order status is 'shipped', definitely NOT 'delivered'
	var oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", oStatus)
	assert.NotEqual(t, "delivered", oStatus)

	// 4. Shipment status is 'shipped', delivered_at is NULL
	var sStatus string
	var deliveredAt *time.Time
	err = f.db.QueryRow(ctx, `SELECT status, delivered_at FROM shipments WHERE id = $1`, res.ShipmentID).Scan(&sStatus, &deliveredAt)
	require.NoError(t, err)
	assert.Equal(t, "shipped", sStatus)
	assert.Nil(t, deliveredAt, "delivered_at must remain nil upon dispatch")
}

// ------------------------------------------------------------------------------------------------
// SECTION 14: CANCELLATION RACE
// ------------------------------------------------------------------------------------------------
func TestDispatchPhysicalTruth_CancellationRace(t *testing.T) {
	ctx := context.Background()
	f := newDispatchTruthFixture(t, ctx)

	orderID, fulfillmentID := f.createOrderAndFulfillment("packed", "packed")
	prodID, variantID := f.createProductAndVariant()
	itemID := f.createOrderItem(orderID, fulfillmentID, prodID, variantID, 2, 0)
	f.createInventoryItem(prodID, variantID, 10, 5)

	u1, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)
	u2, _, _ := f.createAllocatedUnit(itemID, variantID, "warehouse", true)

	var wg sync.WaitGroup
	wg.Add(2)

	var dispatchErr error
	var cancelErr error
	var dispatchRes *fulfillment.DispatchResult

	// Goroutine 1: Dispatch
	go func() {
		defer wg.Done()
		dispatchRes, dispatchErr = f.svc.DispatchFulfillment(ctx, f.adminID, fulfillmentID)
	}()

	// Goroutine 2: Cancel order in a separate transaction
	go func() {
		defer wg.Done()
		tx, err := f.db.Begin(ctx)
		if err != nil {
			cancelErr = err
			return
		}
		var oStatus string
		err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&oStatus)
		if err != nil {
			_ = tx.Rollback(ctx)
			cancelErr = err
			return
		}
		if oStatus == "shipped" || oStatus == "delivered" {
			_ = tx.Rollback(ctx)
			cancelErr = errors.New("cannot cancel shipped order")
			return
		}
		_, err = tx.Exec(ctx, `UPDATE orders SET status = 'cancelled', updated_at = now() WHERE id = $1`, orderID)
		if err != nil {
			_ = tx.Rollback(ctx)
			cancelErr = err
			return
		}
		_, err = tx.Exec(ctx, `UPDATE order_fulfillments SET status = 'cancelled', updated_at = now() WHERE id = $1`, fulfillmentID)
		if err != nil {
			_ = tx.Rollback(ctx)
			cancelErr = err
			return
		}
		cancelErr = tx.Commit(ctx)
	}()

	wg.Wait()

	if dispatchRes != nil {
		f.track(&f.shipmentIDs, dispatchRes.ShipmentID)
	}

	// Invariant: Exactly one must succeed and determine the consistent state
	var finalFulfillmentStatus, finalOrderStatus string
	err := f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&finalFulfillmentStatus)
	require.NoError(t, err)
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&finalOrderStatus)
	require.NoError(t, err)

	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)

	if dispatchErr == nil {
		// Dispatch won
		assert.Equal(t, "shipped", finalFulfillmentStatus)
		assert.Equal(t, "shipped", finalOrderStatus)
		assert.Equal(t, 8, totalStock)
		assert.Equal(t, 3, reservedStock)

		for _, uID := range []uuid.UUID{u1, u2} {
			var uStatus string
			err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
			require.NoError(t, err)
			assert.Equal(t, "shipped", uStatus)
		}
	} else {
		// Cancel won
		assert.ErrorIs(t, dispatchErr, fulfillment.ErrDispatchNotAllowed)
		assert.NoError(t, cancelErr)
		assert.Equal(t, "cancelled", finalFulfillmentStatus)
		assert.Equal(t, "cancelled", finalOrderStatus)
		assert.Equal(t, 10, totalStock, "no stock decrement on cancel")
		assert.Equal(t, 5, reservedStock, "no reserved decrement on cancel")

		for _, uID := range []uuid.UUID{u1, u2} {
			var uStatus string
			err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uID).Scan(&uStatus)
			require.NoError(t, err)
			assert.Equal(t, "warehouse", uStatus)
		}
	}
}
