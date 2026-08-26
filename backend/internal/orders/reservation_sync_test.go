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

// Test A, B, C, D, E, F, G, H, I:

func TestReservationSync_SerializedHappyPathAndSecondOrder(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	variantA := f.createVariantWithInventory(t, ctx, "SKU-SYNC-A", 10, 0)
	zmuUnits := f.createWarehouseUnits(t, ctx, variantA, 5)
	require.Len(t, zmuUnits, 5)

	// --- A. SERIALIZED HAPPY PATH ---
	// Buyer 1 adds 2 to cart and creates order
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
	require.Len(t, order1.Items, 1)
	order1ItemID := order1.Items[0].ID

	// Check aggregate reserved_stock incremented by 2
	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 2, reservedStock, "aggregate reserved_stock must be 2")

	// Check reservation quantity = 2
	resIDs, err := f.ordersRepo.GetOrderReservations(ctx, order1.ID)
	require.NoError(t, err)
	require.Len(t, resIDs, 1)
	res1ID := resIDs[0]

	// Check exactly 2 active allocations with reservation_id populated
	allocs1, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1ItemID)
	require.NoError(t, err)
	require.Len(t, allocs1, 2)
	assert.NotEqual(t, allocs1[0].InventoryUnitID, allocs1[1].InventoryUnitID)
	for _, a := range allocs1 {
		require.NotNil(t, a.ReservationID)
		assert.Equal(t, res1ID, *a.ReservationID)
	}

	// Verify all physical units remain status = 'warehouse'
	for _, uid := range zmuUnits {
		var status string
		err := f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uid).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", status)
	}

	// --- B. SECOND ORDER ---
	// Buyer 2 reserves another 2
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
	require.NotNil(t, order2)
	require.Len(t, order2.Items, 1)
	order2ItemID := order2.Items[0].ID

	// Check aggregate reserved_stock total = 4
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 4, reservedStock, "aggregate reserved_stock must be 4")

	// Check 4 distinct active ZMUs total
	allocs2, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order2ItemID)
	require.NoError(t, err)
	require.Len(t, allocs2, 2)

	allocatedUnits := map[uuid.UUID]bool{
		allocs1[0].InventoryUnitID: true,
		allocs1[1].InventoryUnitID: true,
		allocs2[0].InventoryUnitID: true,
		allocs2[1].InventoryUnitID: true,
	}
	assert.Len(t, allocatedUnits, 4, "all 4 allocated ZMUs must be distinct")

	// --- C. PHYSICAL SHORTAGE ---
	// 5 total ZMUs, 4 active allocated => only 1 remaining.
	// Buyer 3 tries to reserve 2 -> must fail completely (all-or-nothing rollback)
	buyer3ID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyer3ID, variantA, 2)

	order3, err := f.ordersSvc.CreateOrder(ctx, buyer3ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Three",
		CustomerPhone:    "+79990000003",
		CustomerEmail:    "buyer3@example.com",
		DeliveryAddress:  "Warehouse Lane 3",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.Error(t, err, "order creation must fail when physical ZMUs are insufficient")
	assert.Nil(t, order3)

	// Verify ZERO new reservations, reserved_stock remains 4, total active allocations for variantA remains 4
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 4, reservedStock, "reserved_stock must stay 4 after rolled back attempt")

	var totalActiveCount int
	err = f.db.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM order_item_allocations a
		JOIN inventory_units u ON a.inventory_unit_id = u.id
		WHERE u.product_variant_id = $1 AND a.released_at IS NULL
	`, variantA).Scan(&totalActiveCount)
	require.NoError(t, err)
	assert.Equal(t, 4, totalActiveCount, "active allocations count for variantA must stay 4")

	// --- E. CANCELLATION RELEASE ---
	// Cancel order 1 through canonical path
	err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order1.ID)
	require.NoError(t, err)

	// Check reserved_stock decremented to 2 (4 - 2 = 2)
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 2, reservedStock, "reserved_stock must decrement by 2 after cancellation")

	// Check order 1 active allocations = 0
	activeAllocs1After, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1ItemID)
	require.NoError(t, err)
	assert.Empty(t, activeAllocs1After)

	// Check order 1 historical allocations retained with released_at and release_reason
	allAllocs1, err := f.ordersRepo.ListAllAllocationsForOrderItem(ctx, order1ItemID)
	require.NoError(t, err)
	require.Len(t, allAllocs1, 2)
	for _, a := range allAllocs1 {
		assert.NotNil(t, a.ReleasedAt)
		require.NotNil(t, a.ReleaseReason)
	}

	// Units remain in warehouse status
	for _, uid := range zmuUnits {
		var status string
		err := f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uid).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", status)
	}

	// Now buyer 3 retries ordering 2 units -> should succeed because 2 units were released!
	order3Retry, err := f.ordersSvc.CreateOrder(ctx, buyer3ID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Three",
		CustomerPhone:    "+79990000003",
		CustomerEmail:    "buyer3@example.com",
		DeliveryAddress:  "Warehouse Lane 3",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	assert.NotNil(t, order3Retry)

	// --- G. IDEMPOTENT RELEASE ---
	// Cancelling order 1 again must not decrement reserved_stock again or error destructively
	err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order1.ID)
	require.Error(t, err, "cancelling already cancelled order should return error and not double decrement")
	assert.True(t, errors.Is(err, orders.ErrOrderNotCancellable))

	// Re-releasing reservation directly via inventory service
	err = f.invSvc.ReleaseReservation(ctx, res1ID)
	assert.True(t, errors.Is(err, inventory.ErrReservationNotActive), "idempotent release must return ErrReservationNotActive")

	// Verify reserved_stock is still valid (4 total from order 2 and order 3)
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 4, reservedStock)
}

func TestReservationSync_AggregateShortage(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// --- D. AGGREGATE SHORTAGE ---
	// 5 physical ZMUs exist, but aggregate total_stock = 1, reserved_stock = 0.
	variant := f.createVariantWithInventory(t, ctx, "SKU-AGG-SHORT", 1, 0)
	zmuUnits := f.createWarehouseUnits(t, ctx, variant, 5)
	require.Len(t, zmuUnits, 5)

	// Buyer tries to order 2 units (aggregate available is only 1)
	f.populateCart(t, ctx, f.buyerID, variant, 2)

	order, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Short",
		CustomerPhone:    "+79990000000",
		CustomerEmail:    "buyershort@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.Error(t, err, "aggregate shortage must fail order creation")
	assert.Nil(t, order)

	// Verify ZERO physical allocations were created
	var activeCount int
	err = f.db.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM order_item_allocations a
		JOIN inventory_units u ON a.inventory_unit_id = u.id
		WHERE u.product_variant_id = $1 AND a.released_at IS NULL
	`, variant).Scan(&activeCount)
	require.NoError(t, err)
	assert.Equal(t, 0, activeCount, "no physical units must be allocated")

	// Reserved stock unchanged
	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variant).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 0, reservedStock)
}

func TestReservationSync_TTLExpiryRelease(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// --- F. TTL EXPIRY ---
	variant := f.createVariantWithInventory(t, ctx, "SKU-TTL", 5, 0)
	f.createWarehouseUnits(t, ctx, variant, 3)

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

	// Trigger expiration worker logic for orders created before now + 1 hour with sufficient limit
	res, err := f.ordersSvc.ExpireAwaitingPaymentOrders(ctx, time.Now().Add(1*time.Hour), 1000)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Expired, 1)

	// Check order is cancelled
	updatedOrder, err := f.ordersRepo.GetOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", updatedOrder.Status)

	// Check allocations released
	activeAfter, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Empty(t, activeAfter, "active allocations must be released after TTL expiry")

	// Check reserved_stock decremented back to 0
	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variant).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 0, reservedStock, "reserved_stock must be 0 after expiry release")
}

func TestReservationSync_PaymentSuccessKeepsAllocationActive(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// --- H. PAYMENT SUCCESS ---
	variant := f.createVariantWithInventory(t, ctx, "SKU-PAID", 5, 0)
	f.createWarehouseUnits(t, ctx, variant, 2)

	f.populateCart(t, ctx, f.buyerID, variant, 2)

	order, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Paid",
		CustomerPhone:    "+79990000000",
		CustomerEmail:    "buyerpaid@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order)

	orderItemID := order.Items[0].ID

	resIDs, err := f.ordersRepo.GetOrderReservations(ctx, order.ID)
	require.NoError(t, err)
	require.Len(t, resIDs, 1)

	// Simulate payment success converting reservation to sale
	err = f.pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
		return f.invSvc.ConvertReservationToSaleTx(ctx, tx, resIDs[0])
	})
	require.NoError(t, err)

	// Check allocations REMAIN ACTIVE (released_at is NULL) for future picking
	activeAllocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Len(t, activeAllocs, 2, "physical allocations must remain active after payment success")
}

func TestReservationSync_LegacyStockRegression(t *testing.T) {
	ctx := context.Background()
	f := setupReservationSyncFixture(t, ctx)
	defer f.db.Close()

	// --- I. LEGACY STOCK REGRESSION ---
	// Create aggregate inventory with NO physical ZMU representation (0 inventory_units).
	legacyVariant := f.createVariantWithInventory(t, ctx, "SKU-LEGACY", 10, 0)

	// Verify no ZMU exists
	hasSerialized, err := f.ordersRepo.HasSerializedUnits(ctx, legacyVariant)
	require.NoError(t, err)
	assert.False(t, hasSerialized, "legacy variant has no serialized units")

	// Buyer orders 3 units of legacy item
	f.populateCart(t, ctx, f.buyerID, legacyVariant, 3)

	order, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Legacy",
		CustomerPhone:    "+79990000000",
		CustomerEmail:    "buyerlegacy@example.com",
		DeliveryAddress:  "Warehouse Lane",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
	require.NoError(t, err, "legacy order creation must succeed without error")
	require.NotNil(t, order)
	require.Len(t, order.Items, 1)

	// Aggregate reserved_stock is incremented to 3
	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, legacyVariant).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 3, reservedStock, "aggregate reserved_stock must increment to 3")

	// Order item allocations table has 0 rows for this item (no fake ZMUs created)
	allocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order.Items[0].ID)
	require.NoError(t, err)
	assert.Empty(t, allocs, "legacy order items must have 0 physical allocation rows")

	// Cancellation releases aggregate reserved_stock
	err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order.ID)
	require.NoError(t, err)

	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, legacyVariant).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 0, reservedStock, "aggregate reserved_stock must decrement back to 0")
}
