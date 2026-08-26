package orders_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type reservationSyncFixture struct {
	db               *pgxpool.Pool
	pgClient         *postgres.Client
	ordersRepo       *orders.Repository
	cartRepo         *cart.Repository
	invRepo          *inventory.Repository
	invSvc           *inventory.Service
	ordersSvc        *orders.Service
	deliveryMethodID uuid.UUID
	sellerID         uuid.UUID
	buyerID          uuid.UUID
	catID            uuid.UUID
	prodID           uuid.UUID
}

func setupReservationSyncFixture(t *testing.T, ctx context.Context) *reservationSyncFixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	require.NotEmpty(t, dbURL, "test database URL must not be empty")

	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "failed to connect to test database")

	// Safety check: must strictly be zamk_test
	testutil.AssertTestDatabase(t, db)

	pgClient := &postgres.Client{Pool: db}
	ordersRepo := orders.NewRepository(db)
	cartRepo := cart.NewRepository(db)
	invRepo := inventory.NewRepository(db)
	invSvc := inventory.NewService(invRepo, nil, pgClient)

	cfg := &config.Config{
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
	}
	ordersSvc := orders.NewService(ordersRepo, cartRepo, invSvc, pgClient, cfg)

	suffix := uuid.New().String()[:8]
	sellerUserID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()

	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Seller User', $2, 'hash', 'seller', 'active', now(), now())
	`, sellerUserID, fmt.Sprintf("seller-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Buyer User', $2, 'hash', 'customer', 'active', now(), now())
	`, buyerID, fmt.Sprintf("buyer-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Sync Seller', $2, $3, 'active', now(), now())
	`, sellerID, fmt.Sprintf("sync-seller-%s", suffix), fmt.Sprintf("sync-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Sync Cat', $2, now(), now())
	`, catID, fmt.Sprintf("sync-cat-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Sync Prod', $4, 1000, 'published', now(), now())
	`, prodID, sellerID, catID, fmt.Sprintf("sync-prod-%s", suffix))
	require.NoError(t, err)

	dmID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Standard Delivery', 200, true, now(), now())
	`, dmID, fmt.Sprintf("dm-%s", suffix))
	require.NoError(t, err)

	return &reservationSyncFixture{
		db:               db,
		pgClient:         pgClient,
		ordersRepo:       ordersRepo,
		cartRepo:         cartRepo,
		invRepo:          invRepo,
		invSvc:           invSvc,
		ordersSvc:        ordersSvc,
		deliveryMethodID: dmID,
		sellerID:         sellerID,
		buyerID:          buyerID,
		catID:            catID,
		prodID:           prodID,
	}
}

func (f *reservationSyncFixture) createVariantWithInventory(t *testing.T, ctx context.Context, skuPrefix string, totalStock int, reservedStock int) uuid.UUID {
	t.Helper()
	variantID := uuid.New()
	suffix := uuid.New().String()[:8]

	_, err := f.db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())
	`, variantID, f.prodID, fmt.Sprintf("%s-%s", skuPrefix, suffix), fmt.Sprintf("SSKU-%s", suffix), fmt.Sprintf("BC-%s", suffix))
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
	`, uuid.New(), f.prodID, variantID, f.sellerID, totalStock, reservedStock)
	require.NoError(t, err)

	return variantID
}

func (f *reservationSyncFixture) createWarehouseUnits(t *testing.T, ctx context.Context, variantID uuid.UUID, count int) []uuid.UUID {
	t.Helper()
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	suffix := uuid.New().String()[:8]

	_, err := f.db.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, f.sellerID, fmt.Sprintf("SUP-%s", suffix))
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, supplyItemID, supplyID, variantID, count)
	require.NoError(t, err)

	var unitIDs []uuid.UUID
	for i := 1; i <= count; i++ {
		unitID := uuid.New()
		unitCode := fmt.Sprintf("ZMU-%s-%03d", suffix, i)
		_, err := f.db.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'warehouse', now(), now())
		`, unitID, unitCode, variantID, supplyID, supplyItemID, i)
		require.NoError(t, err)
		unitIDs = append(unitIDs, unitID)
	}

	return unitIDs
}

func (f *reservationSyncFixture) populateCart(t *testing.T, ctx context.Context, userID, variantID uuid.UUID, quantity int) {
	t.Helper()
	userCart, err := f.cartRepo.GetCartByUserID(ctx, userID)
	if err != nil && errors.Is(err, cart.ErrCartNotFound) {
		userCart, err = f.cartRepo.CreateCart(ctx, userID)
		require.NoError(t, err)
	}
	require.NoError(t, err)

	existingItem, err := f.cartRepo.GetCartItem(ctx, userCart.ID, variantID)
	if err == nil && existingItem != nil {
		err = f.cartRepo.UpdateItemQuantity(ctx, existingItem.ID, quantity)
		require.NoError(t, err)
		return
	}

	item := &cart.CartItem{
		ID:               uuid.New(),
		CartID:           userCart.ID,
		ProductID:        f.prodID,
		ProductVariantID: variantID,
		Quantity:         quantity,
	}
	err = f.cartRepo.AddItem(ctx, item)
	require.NoError(t, err)
}

func (f *reservationSyncFixture) createNewBuyer(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()
	buyerID := uuid.New()
	suffix := uuid.New().String()[:8]
	_, err := f.db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Buyer User', $2, 'hash', 'customer', 'active', now(), now())
	`, buyerID, fmt.Sprintf("buyer-%s@example.com", suffix))
	require.NoError(t, err)
	return buyerID
}

// A. PURE SERIALIZED VARIANT TEST
func TestRegression_PureSerializedVariant(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// 5 physical units in warehouse, aggregate total_stock = 5, reserved = 0
	variantA := f.createVariantWithInventory(t, ctx, "SKU-PURE-SER", 5, 0)
	zmuUnits := f.createWarehouseUnits(t, ctx, variantA, 5)
	require.Len(t, zmuUnits, 5)

	// Order 1: reserve 2 units
	f.populateCart(t, ctx, f.buyerID, variantA, 2)
	order1, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer One",
		CustomerPhone:    "+79990000001",
		CustomerEmail:    "buyer1@example.com",
		DeliveryAddress:  "Warehouse Lane 1",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order1)
	order1ItemID := order1.Items[0].ID

	// Check total_stock unchanged = 5, reserved_stock = 2
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 5, totalStock, "total_stock must remain 5 (physical onHand)")
	assert.Equal(t, 2, reservedStock, "reserved_stock must be 2")

	// Check 2 active allocations with reservation_id
	allocs1, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1ItemID)
	require.NoError(t, err)
	require.Len(t, allocs1, 2)
	assert.Equal(t, order1.Items[0].Quantity, len(allocs1), "allocation count must exactly match order item quantity")

	resIDs, err := f.ordersRepo.GetOrderReservations(ctx, order1.ID)
	require.NoError(t, err)
	require.Len(t, resIDs, 1)
	res1ID := resIDs[0]
	for _, a := range allocs1 {
		require.NotNil(t, a.ReservationID)
		assert.Equal(t, res1ID, *a.ReservationID)
	}

	// Order 2: reserve another 2 units
	buyer2ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer2ID, variantA, 2)
	order2, err := f.ordersSvc.CreateOrder(ctx, buyer2ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Two",
		CustomerPhone:    "+79990000002",
		CustomerEmail:    "buyer2@example.com",
		DeliveryAddress:  "Warehouse Lane 2",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	order2ItemID := order2.Items[0].ID

	// Check total_stock = 5, reserved_stock = 4
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 5, totalStock)
	assert.Equal(t, 4, reservedStock)

	// 4 distinct active ZMUs allocated
	allocs2, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order2ItemID)
	require.NoError(t, err)
	require.Len(t, allocs2, 2)
	assert.Equal(t, order2.Items[0].Quantity, len(allocs2))

	allocatedMap := map[uuid.UUID]bool{
		allocs1[0].InventoryUnitID: true,
		allocs1[1].InventoryUnitID: true,
		allocs2[0].InventoryUnitID: true,
		allocs2[1].InventoryUnitID: true,
	}
	assert.Len(t, allocatedMap, 4)

	// Order 3: only 1 unit remaining; attempt ordering 2 units -> must fail
	buyer3ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer3ID, variantA, 2)
	order3, err := f.ordersSvc.CreateOrder(ctx, buyer3ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Three",
		CustomerPhone:    "+79990000003",
		CustomerEmail:    "buyer3@example.com",
		DeliveryAddress:  "Warehouse Lane 3",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.Error(t, err)
	assert.Nil(t, order3)

	// Invariants hold: total_stock = 5, reserved_stock remains 4, active allocations count remains 4
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 5, totalStock)
	assert.Equal(t, 4, reservedStock)

	// Cancellation of Order 1: single canonical release decrements reserved_stock to 2
	err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order1.ID)
	require.NoError(t, err)

	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 5, totalStock, "total_stock remains 5 upon cancellation")
	assert.Equal(t, 2, reservedStock, "reserved_stock decrements to 2")

	// Released allocations recorded with history preserved
	allAllocs1, err := f.ordersRepo.ListAllAllocationsForOrderItem(ctx, order1ItemID)
	require.NoError(t, err)
	require.Len(t, allAllocs1, 2)
	for _, a := range allAllocs1 {
		assert.NotNil(t, a.ReleasedAt)
		assert.NotNil(t, a.ReleaseReason)
	}

	// Units remain status = 'warehouse'
	for _, uid := range zmuUnits {
		var status string
		err := f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uid).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", status)
	}
}

// B. PURE LEGACY VARIANT TEST
func TestRegression_PureLegacyVariant(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// Pure aggregate: 0 ZMUs in warehouse, total_stock = 10, reserved = 0
	legacyVariant := f.createVariantWithInventory(t, ctx, "SKU-PURE-LEG", 10, 0)

	// Order 1: 3 units
	f.populateCart(t, ctx, f.buyerID, legacyVariant, 3)
	order1, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Legacy 1",
		CustomerPhone:    "+79990000001",
		CustomerEmail:    "buyerleg1@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order1)

	// Aggregate total_stock = 10, reserved_stock = 3, allocations count = 0 (no fake ZMUs)
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, legacyVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 3, reservedStock)

	allocs1, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1.Items[0].ID)
	require.NoError(t, err)
	assert.Empty(t, allocs1, "pure legacy orders must have 0 physical allocations")

	// Order 2: 5 units -> reserved_stock = 8
	buyer2ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer2ID, legacyVariant, 5)
	order2, err := f.ordersSvc.CreateOrder(ctx, buyer2ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Legacy 2",
		CustomerPhone:    "+79990000002",
		CustomerEmail:    "buyerleg2@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order2)

	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, legacyVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 8, reservedStock)

	allocs2, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order2.Items[0].ID)
	require.NoError(t, err)
	assert.Empty(t, allocs2, "pure legacy orders must have 0 physical allocations")

	// Order 3: 5 units (available is 10 - 8 = 2 < 5) -> must fail
	buyer3ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer3ID, legacyVariant, 5)
	order3, err := f.ordersSvc.CreateOrder(ctx, buyer3ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Legacy 3",
		CustomerPhone:    "+79990000003",
		CustomerEmail:    "buyerleg3@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.Error(t, err)
	assert.Nil(t, order3)

	// Cancellation of Order 1 releases aggregate reserved_stock back to 5
	err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order1.ID)
	require.NoError(t, err)

	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, legacyVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 5, reservedStock)
}

// C & E. MIXED STOCK VARIANT TEST (3 serialized ZMU + 5 legacy units, total = 8)
// Proves allocation coverage invariant: count is ALWAYS 0 or quantity, NEVER partial!
func TestRegression_MixedStockVariant(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// total_stock = 8, warehouse ZMU = 3, reserved_stock = 0
	mixedVariant := f.createVariantWithInventory(t, ctx, "SKU-MIXED", 8, 0)
	zmuUnits := f.createWarehouseUnits(t, ctx, mixedVariant, 3)
	require.Len(t, zmuUnits, 3)

	// --- 1. Order 1: quantity 2 (<= 3 ZMUs) ---
	// Fully backed by serialized ZMU -> allocates EXACTLY 2 physical units (count == quantity)
	f.populateCart(t, ctx, f.buyerID, mixedVariant, 2)
	order1, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Mixed 1",
		CustomerPhone:    "+79990000001",
		CustomerEmail:    "buyermixed1@example.com",
		DeliveryAddress:  "Warehouse Lane 1",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order1)
	order1ItemID := order1.Items[0].ID

	// Check total_stock = 8, reserved_stock = 2
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, mixedVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 2, reservedStock)

	// Invariant: Exactly 2 active allocations (count == quantity == 2)
	allocs1, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1ItemID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(allocs1))
	assert.NotEqual(t, allocs1[0].InventoryUnitID, allocs1[1].InventoryUnitID)

	// --- 2. Order 2: quantity 4 (> 3 ZMUs total, and > 1 remaining ZMU) ---
	// Available aggregate stock = 8 - 2 = 6 >= 4.
	// Invariant: Cannot be fully backed by remaining 1 ZMU -> fallback to legacy aggregate.
	// Active allocations count MUST BE EXACTLY 0 (NEVER partial 1 of 4)!
	buyer2ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer2ID, mixedVariant, 4)
	order2, err := f.ordersSvc.CreateOrder(ctx, buyer2ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Mixed 2",
		CustomerPhone:    "+79990000002",
		CustomerEmail:    "buyermixed2@example.com",
		DeliveryAddress:  "Warehouse Lane 2",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err, "ordering quantity > warehouse_zmu_count backed by legacy stock must succeed")
	require.NotNil(t, order2)
	order2ItemID := order2.Items[0].ID

	// Aggregate total_stock = 8, reserved_stock becomes 2 + 4 = 6
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, mixedVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 6, reservedStock)

	// Invariant: Order 2 has EXACTLY 0 allocations (NEVER 1 partial allocation!)
	allocs2, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order2ItemID)
	require.NoError(t, err)
	assert.Equal(t, 0, len(allocs2), "legacy order item must have EXACTLY 0 allocations (no partial allocation)")

	// --- 3. Order 3: quantity 1 (<= 1 remaining ZMU, <= 2 remaining aggregate) ---
	// Invariant: Backed by the 1 remaining ZMU -> allocates EXACTLY 1 physical ZMU (count == quantity == 1)
	buyer3ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer3ID, mixedVariant, 1)
	order3, err := f.ordersSvc.CreateOrder(ctx, buyer3ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Mixed 3",
		CustomerPhone:    "+79990000003",
		CustomerEmail:    "buyermixed3@example.com",
		DeliveryAddress:  "Warehouse Lane 3",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order3)
	order3ItemID := order3.Items[0].ID

	// Aggregate total_stock = 8, reserved_stock = 7
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, mixedVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 7, reservedStock)

	// Invariant: Order 3 has EXACTLY 1 active allocation (count == quantity == 1)
	allocs3, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order3ItemID)
	require.NoError(t, err)
	assert.Equal(t, 1, len(allocs3))

	// Verify all 3 allocated ZMUs across Order 1 and Order 3 are distinct
	allocatedUnits := map[uuid.UUID]bool{
		allocs1[0].InventoryUnitID: true,
		allocs1[1].InventoryUnitID: true,
		allocs3[0].InventoryUnitID: true,
	}
	assert.Len(t, allocatedUnits, 3, "all 3 warehouse ZMUs must be uniquely allocated")

	// --- 4. Order 4: quantity 2 (> 1 remaining aggregate available: 8 - 7 = 1) -> must fail ---
	buyer4ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer4ID, mixedVariant, 2)
	order4, err := f.ordersSvc.CreateOrder(ctx, buyer4ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Mixed 4",
		CustomerPhone:    "+79990000004",
		CustomerEmail:    "buyermixed4@example.com",
		DeliveryAddress:  "Warehouse Lane 4",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.Error(t, err, "ordering beyond aggregate available must fail")
	assert.Nil(t, order4)

	// --- 5. Cancellation of Order 2 (legacy): releases 4 aggregate units ---
	err = f.ordersSvc.CancelCustomerOrder(ctx, buyer2ID, order2.ID)
	require.NoError(t, err)

	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, mixedVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 3, reservedStock, "reserved_stock must be 7 - 4 = 3")

	// --- 6. Cancellation of Order 1 (serialized): releases 2 aggregate units and 2 ZMUs ---
	err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order1.ID)
	require.NoError(t, err)

	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, mixedVariant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 8, totalStock)
	assert.Equal(t, 1, reservedStock, "reserved_stock must be 3 - 2 = 1")

	// The 2 ZMUs from Order 1 are now released and available again
	activeAllocs1After, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1ItemID)
	require.NoError(t, err)
	assert.Empty(t, activeAllocs1After)

	// Re-order 2 units with buyer 4 -> succeeds and re-allocates those 2 released ZMUs!
	order4Retry, err := f.ordersSvc.CreateOrder(ctx, buyer4ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Mixed 4 Retry",
		CustomerPhone:    "+79990000004",
		CustomerEmail:    "buyermixed4@example.com",
		DeliveryAddress:  "Warehouse Lane 4",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order4Retry)

	allocs4, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order4Retry.Items[0].ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(allocs4), "re-order must allocate the 2 released physical ZMUs (count == quantity)")
}

// B, C, D. PAYMENT SUCCESS & PHYSICAL ON-HAND INVARIANT TEST
func TestRegression_PaidSerializedOrder(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// 1 ZMU, total_stock = 2, reserved = 0 (1 serialized + 1 legacy)
	variant := f.createVariantWithInventory(t, ctx, "SKU-PAID-INV", 2, 0)
	zmuUnits := f.createWarehouseUnits(t, ctx, variant, 1)
	require.Len(t, zmuUnits, 1)
	zmuID := zmuUnits[0]

	// Order A reserves 1 unit (the only ZMU)
	f.populateCart(t, ctx, f.buyerID, variant, 1)
	orderA, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer A",
		CustomerPhone:    "+79990000001",
		CustomerEmail:    "buyera@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	orderAItemID := orderA.Items[0].ID

	resIDs, err := f.ordersRepo.GetOrderReservations(ctx, orderA.ID)
	require.NoError(t, err)
	require.Len(t, resIDs, 1)

	// Before payment: total_stock = 2, reserved_stock = 1
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 2, totalStock)
	assert.Equal(t, 1, reservedStock)

	// Simulate payment success converting reservation to sale
	err = f.pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
		return f.invSvc.ConvertReservationToSaleTx(ctx, tx, resIDs[0])
	})
	require.NoError(t, err)

	// 1. Check reservation status is converted
	var resStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, resIDs[0]).Scan(&resStatus)
	require.NoError(t, err)
	assert.Equal(t, "converted", resStatus)

	// 2. Physical onHand (total_stock) MUST REMAIN 2 (unit physically remains at ZAMK!)
	// Committed stock (reserved_stock) MUST REMAIN 1 (paid unit remains committed/unavailable!)
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 2, totalStock, "payment must NOT decrement physical total_stock before shipment")
	assert.Equal(t, 1, reservedStock, "payment must retain reserved_stock commitment to prevent oversell")

	// 3. Physical unit status remains 'warehouse'
	var unitStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, zmuID).Scan(&unitStatus)
	require.NoError(t, err)
	assert.Equal(t, "warehouse", unitStatus, "physical unit must stay in warehouse status after payment")

	// 4. Physical allocation remains ACTIVE (released_at IS NULL)
	allocsA, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderAItemID)
	require.NoError(t, err)
	require.Len(t, allocsA, 1)
	assert.Equal(t, zmuID, allocsA[0].InventoryUnitID)
	assert.Nil(t, allocsA[0].ReleasedAt)

	// 5. Payment does not create availability:
	// Available = total_stock (2) - reserved_stock (1) = 1.
	// Order B attempts to order 2 units (> 1 available) -> MUST FAIL (no oversell)!
	buyerBID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerBID, variant, 2)
	orderBExceed, err := f.ordersSvc.CreateOrder(ctx, buyerBID, orders.CreateOrderRequest{
		CustomerName:     "Buyer B Exceed",
		CustomerPhone:    "+79990000002",
		CustomerEmail:    "buyerbe@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.Error(t, err, "attempting to order exceeding uncommitted capacity must fail")
	assert.Nil(t, orderBExceed)

	// 6. Order B orders 1 unit (the remaining legacy capacity):
	// Succeeds as legacy aggregate reservation.
	f.populateCart(t, ctx, buyerBID, variant, 1)
	orderB, err := f.ordersSvc.CreateOrder(ctx, buyerBID, orders.CreateOrderRequest{
		CustomerName:     "Buyer B",
		CustomerPhone:    "+79990000002",
		CustomerEmail:    "buyerb@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, orderB)

	// Order B did NOT allocate Order A's paid ZMU (allocations count = 0)
	allocsB, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderB.Items[0].ID)
	require.NoError(t, err)
	assert.Empty(t, allocsB, "Order B must NOT allocate Order A's paid ZMU")

	// 7. Order C attempts to order when 0 available -> must fail
	buyerCID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerCID, variant, 1)
	orderC, err := f.ordersSvc.CreateOrder(ctx, buyerCID, orders.CreateOrderRequest{
		CustomerName:     "Buyer C",
		CustomerPhone:    "+79990000003",
		CustomerEmail:    "buyerc@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.Error(t, err, "cannot oversell paid stock")
	assert.Nil(t, orderC)
}

// E & F. TTL EXPIRY AND CANONICAL RELEASE TEST
func TestRegression_TTLExpiryAndCanonicalRelease(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	variant := f.createVariantWithInventory(t, ctx, "SKU-TTL-CANON", 5, 0)
	zmuUnits := f.createWarehouseUnits(t, ctx, variant, 3)
	require.Len(t, zmuUnits, 3)

	f.populateCart(t, ctx, f.buyerID, variant, 2)
	order, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer TTL",
		CustomerPhone:    "+79990000000",
		CustomerEmail:    "buyerttl@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order)
	orderItemID := order.Items[0].ID

	// Verify allocated
	allocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	require.Len(t, allocs, 2)

	// Trigger expiration worker logic
	res, err := f.ordersSvc.ExpireAwaitingPaymentOrders(ctx, time.Now().Add(1*time.Hour), 1000)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Expired, 1)

	// Order cancelled
	updatedOrder, err := f.ordersRepo.GetOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", updatedOrder.Status)

	// Active allocations released
	activeAfter, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Empty(t, activeAfter)

	// Total stock = 5, reserved stock decremented back to 0
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variant).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 5, totalStock)
	assert.Equal(t, 0, reservedStock)

	// Units remain status = 'warehouse'
	for _, uid := range zmuUnits {
		var status string
		err := f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uid).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", status)
	}
}
