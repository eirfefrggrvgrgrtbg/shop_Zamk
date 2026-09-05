package orders_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *syncBuffer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Reset()
}

type allocationLifecycleFixture struct {
	db               *pgxpool.Pool
	pgClient         *postgres.Client
	ordersRepo       *orders.Repository
	cartRepo         *cart.Repository
	invRepo          *inventory.Repository
	invSvc           *inventory.Service
	ordersSvc        *orders.Service
	paymentsRepo     *payments.Repository
	paymentsSvc      *payments.Service
	deliveryMethodID uuid.UUID
	sellerID         uuid.UUID
	buyerID          uuid.UUID
	buyer2ID         uuid.UUID
	catID            uuid.UUID
	prodID           uuid.UUID
	cfg              *config.Config
	logBuf           *syncBuffer
}

func setupAllocationLifecycleFixture(t *testing.T, ctx context.Context) *allocationLifecycleFixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	require.NotEmpty(t, dbURL, "test database URL must not be empty")

	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "failed to connect to test database")
	testutil.AssertTestDatabase(t, db)

	pgClient := &postgres.Client{Pool: db}
	ordersRepo := orders.NewRepository(db)
	cartRepo := cart.NewRepository(db)
	invRepo := inventory.NewRepository(db)
	invSvc := inventory.NewService(invRepo, nil, pgClient)

	cfg := &config.Config{
		App: config.AppConfig{PaymentStuckPendingMinutes: 30},
		Worker: config.WorkerConfig{
			MarketplaceCommissionBPS:       1500,
			OrderPaymentTimeoutMinutes:     30,
			OrderExpirationIntervalSeconds: 30,
		},
	}
	ordersSvc := orders.NewService(ordersRepo, cartRepo, invSvc, pgClient, cfg)

	tbankProvider := payments.NewTBankProvider("STUB", "STUB", "", "", "", true, "O", "mock")
	paymentsRepo := payments.NewRepository(db)
	notifRepo := notifications.NewRepository(pgClient)
	notifSvc := notifications.NewService(notifRepo, nil, nil)
	paymentsSvc := payments.NewService(paymentsRepo, ordersRepo, invSvc, tbankProvider, pgClient, notifSvc, cfg)

	suffix := uuid.New().String()[:8]
	sellerUserID := uuid.New()
	buyerID := uuid.New()
	buyer2ID := uuid.New()
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
		VALUES ($1, 'Buyer User', $2, 'hash', 'customer', 'active', now(), now()),
		       ($3, 'Buyer 2 User', $4, 'hash', 'customer', 'active', now(), now())
	`, buyerID, fmt.Sprintf("buyer1-%s@example.com", suffix), buyer2ID, fmt.Sprintf("buyer2-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Lifecycle Seller', $2, $3, 'active', now(), now())
	`, sellerID, fmt.Sprintf("lifecycle-seller-%s", suffix), fmt.Sprintf("lifecycle-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Lifecycle Cat', $2, now(), now())
	`, catID, fmt.Sprintf("lifecycle-cat-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Lifecycle Prod', $4, 1000, 'published', now(), now())
	`, prodID, sellerID, catID, fmt.Sprintf("lifecycle-prod-%s", suffix))
	require.NoError(t, err)

	dmID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Standard Delivery', 200, true, now(), now())
	`, dmID, fmt.Sprintf("dm-%s", suffix))
	require.NoError(t, err)

	logBuf := &syncBuffer{}
	testLogger := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ordersSvc.SetLogger(testLogger)
	paymentsSvc.SetLogger(testLogger)

	return &allocationLifecycleFixture{
		db:               db,
		pgClient:         pgClient,
		ordersRepo:       ordersRepo,
		cartRepo:         cartRepo,
		invRepo:          invRepo,
		invSvc:           invSvc,
		ordersSvc:        ordersSvc,
		paymentsRepo:     paymentsRepo,
		paymentsSvc:      paymentsSvc,
		deliveryMethodID: dmID,
		sellerID:         sellerID,
		buyerID:          buyerID,
		buyer2ID:         buyer2ID,
		catID:            catID,
		prodID:           prodID,
		cfg:              cfg,
		logBuf:           logBuf,
	}
}

func (f *allocationLifecycleFixture) parseLoggedEvents() []map[string]interface{} {
	logs := f.logBuf.String()
	var events []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			if _, ok := entry["event_name"]; ok {
				events = append(events, entry)
			}
		}
	}
	return events
}

func (f *allocationLifecycleFixture) findEventsByName(name string) []map[string]interface{} {
	all := f.parseLoggedEvents()
	var matched []map[string]interface{}
	for _, e := range all {
		if e["event_name"] == name {
			matched = append(matched, e)
		}
	}
	return matched
}

func (f *allocationLifecycleFixture) createVariantWithUnits(t *testing.T, ctx context.Context, skuPrefix string, count int) (uuid.UUID, []uuid.UUID) {
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
		VALUES ($1, $2, $3, $4, $5, 0, now(), now())
	`, uuid.New(), f.prodID, variantID, f.sellerID, count)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = f.db.Exec(ctx, `
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
		unitCode, err := supplies.GenerateUnitCode()
		require.NoError(t, err)
		_, err = f.db.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'warehouse', now(), now())
		`, unitID, unitCode, variantID, supplyID, supplyItemID, i)
		require.NoError(t, err)
		unitIDs = append(unitIDs, unitID)
	}

	return variantID, unitIDs
}

func (f *allocationLifecycleFixture) createOrderForUser(t *testing.T, ctx context.Context, userID, variantID uuid.UUID, quantity int) (*orders.Order, error) {
	t.Helper()
	userCart, err := f.cartRepo.GetCartByUserID(ctx, userID)
	if err != nil && errors.Is(err, cart.ErrCartNotFound) {
		userCart, err = f.cartRepo.CreateCart(ctx, userID)
		require.NoError(t, err)
	}
	item := &cart.CartItem{
		ID:               uuid.New(),
		CartID:           userCart.ID,
		ProductID:        f.prodID,
		ProductVariantID: variantID,
		Quantity:         quantity,
	}
	err = f.cartRepo.AddItem(ctx, item)
	if err != nil {
		return nil, err
	}

	return f.ordersSvc.CreateOrder(ctx, userID, orders.CreateOrderRequest{
		CustomerName:     "Buyer User",
		CustomerPhone:    "+79990000000",
		CustomerEmail:    "buyer@example.com",
		DeliveryAddress:  "Warehouse Road 1",
		DeliveryMethodID: f.deliveryMethodID,
	}, nil)
}

// CASE 1: Create order, payment never confirmed -> must not permanently hold serialized ZMU.
func TestAllocationLifecycle_Case1_PaymentNeverConfirmed(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE1", 1)
	require.Len(t, unitIDs, 1)

	order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	orderItemID := order.Items[0].ID

	// Initial check: 1 active allocation holds unit
	allocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	assert.Equal(t, unitIDs[0], allocs[0].InventoryUnitID)

	// Payment is never confirmed. Time advances past expiration window (e.g. 31 minutes)
	futureTime := time.Now().Add(31 * time.Minute)
	res, err := f.ordersSvc.ExpireAwaitingPaymentOrders(ctx, futureTime, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Expired, 1)

	// Invariant: Order is cancelled, 0 active allocations, 0 active reservations, reserved_stock restored to 0
	updatedOrder, err := f.ordersRepo.GetOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", updatedOrder.Status)

	activeAllocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Empty(t, activeAllocs)

	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 0, reservedStock)

	// ZMU unit is free for other orders
	var unitStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, unitIDs[0]).Scan(&unitStatus)
	require.NoError(t, err)
	assert.Equal(t, "warehouse", unitStatus)
}

// CASE 2: Payment fails/rejected -> reserved_stock restored, reservation released, allocation released, ZMU becomes eligible for another order.
func TestAllocationLifecycle_Case2_PaymentFailsOrRejected(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE2", 1)
	require.Len(t, unitIDs, 1)

	order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
	require.NoError(t, err)
	orderItemID := order.Items[0].ID

	// Create payment in mock mode
	paymentResp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
	require.NoError(t, err)

	// Payment is rejected
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, paymentResp.PaymentID, "reject")
	require.NoError(t, err)

	// Invariant: payment is failed
	var paymentStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, paymentResp.PaymentID).Scan(&paymentStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", paymentStatus)

	// Invariant: zero active reservation
	var activeResCount int
	err = f.db.QueryRow(ctx, `
		SELECT count(*) FROM reservations r
		JOIN order_reservations ord_r ON ord_r.reservation_id = r.id
		WHERE ord_r.order_id = $1 AND r.status = 'active'
	`, order.ID).Scan(&activeResCount)
	require.NoError(t, err)
	assert.Equal(t, 0, activeResCount)

	// Invariant: zero active physical allocation
	activeAllocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Empty(t, activeAllocs)

	// Invariant: reserved_stock restored to 0
	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 0, reservedStock)
}

// CASE 3: Payment confirmed -> exactly one reservation, exactly quantity active allocations, no duplicate reservation on repeated webhook.
func TestAllocationLifecycle_Case3_PaymentConfirmedIdempotent(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE3", 2)
	require.Len(t, unitIDs, 2)

	order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 2)
	require.NoError(t, err)
	orderItemID := order.Items[0].ID

	paymentResp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
	require.NoError(t, err)

	// Payment confirmed
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, paymentResp.PaymentID, "confirm")
	require.NoError(t, err)

	// Invariant: Exactly 1 reservation per item, status = 'converted'
	var resStatus string
	var resCount int
	err = f.db.QueryRow(ctx, `
		SELECT count(*), max(r.status) FROM reservations r
		JOIN order_reservations ord_r ON ord_r.reservation_id = r.id
		WHERE ord_r.order_id = $1
	`, order.ID).Scan(&resCount, &resStatus)
	require.NoError(t, err)
	assert.Equal(t, 1, resCount)
	assert.Equal(t, "converted", resStatus)

	// Invariant: Exactly quantity (2) active allocations
	activeAllocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Len(t, activeAllocs, 2)

	// Order is paid
	updatedOrder, err := f.ordersRepo.GetOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, "paid", updatedOrder.Status)
}

// CASE 4: Repeated CONFIRMED webhook -> idempotent, reserved_stock not double-incremented, no duplicate active allocation.
func TestAllocationLifecycle_Case4_RepeatedConfirmedWebhook(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE4", 2)
	require.Len(t, unitIDs, 2)

	order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 2)
	require.NoError(t, err)
	orderItemID := order.Items[0].ID

	// Create payment in hosted form (card)
	paymentResp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "card")
	require.NoError(t, err)

	// Query the provider_payment_id assigned during CreatePayment
	var providerPaymentID string
	err = f.db.QueryRow(ctx, `SELECT provider_payment_id FROM payments WHERE id = $1`, paymentResp.PaymentID).Scan(&providerPaymentID)
	require.NoError(t, err)

	pidInt, err := strconv.ParseInt(providerPaymentID, 10, 64)
	require.NoError(t, err)

	// Webhook payload CONFIRMED
	payload := map[string]any{
		"TerminalKey": "STUB",
		"OrderId":     order.ID.String(),
		"PaymentId":   pidInt,
		"Amount":      2000,
		"Status":      "CONFIRMED",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	// 1st Webhook delivery -> success
	err = f.paymentsSvc.HandleWebhook(ctx, map[string]string{}, body)
	require.NoError(t, err)

	// 2nd Webhook delivery (gateway retry) -> idempotent (returns nil or ErrPaymentAlreadyProcessed)
	err = f.paymentsSvc.HandleWebhook(ctx, map[string]string{}, body)
	assert.True(t, err == nil || errors.Is(err, payments.ErrPaymentAlreadyProcessed))

	// Invariants after repeated webhook:
	// 1. Exactly 2 active allocations
	activeAllocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Len(t, activeAllocs, 2)

	// 2. Exactly 1 reservation, converted
	var resCount int
	err = f.db.QueryRow(ctx, `
		SELECT count(*) FROM reservations r
		JOIN order_reservations ord_r ON ord_r.reservation_id = r.id
		WHERE ord_r.order_id = $1
	`, order.ID).Scan(&resCount)
	require.NoError(t, err)
	assert.Equal(t, 1, resCount)

	// 3. Reserved stock remains 2, total stock remains 2
	var totalStock, reservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&totalStock, &reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 2, totalStock)
	assert.Equal(t, 2, reservedStock)
	_ = paymentResp
}

// CASE 5: Two orders competing for limited physical units -> only legitimately reserved/paid order owns unit according to canonical contract.
func TestAllocationLifecycle_Case5_TwoOrdersCompetingForUnit(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	// Only 1 physical unit in warehouse
	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE5", 1)
	require.Len(t, unitIDs, 1)

	// User 1 creates Order 1 -> succeeds, holds the unit
	order1, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
	require.NoError(t, err)
	require.NotNil(t, order1)

	allocs1, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1.Items[0].ID)
	require.NoError(t, err)
	require.Len(t, allocs1, 1)
	assert.Equal(t, unitIDs[0], allocs1[0].InventoryUnitID)

	// User 2 attempts to create Order 2 for the same variant -> fails because unit is reserved/out of stock
	order2, err := f.createOrderForUser(t, ctx, f.buyer2ID, variantID, 1)
	assert.Error(t, err)
	assert.Nil(t, order2)

	// Verify only Order 1 owns the active allocation
	var allocOrderIDs []uuid.UUID
	rows, err := f.db.Query(ctx, `
		SELECT oi.order_id FROM order_item_allocations a
		JOIN order_items oi ON oi.id = a.order_item_id
		WHERE a.inventory_unit_id = $1 AND a.released_at IS NULL
	`, unitIDs[0])
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var oid uuid.UUID
		require.NoError(t, rows.Scan(&oid))
		allocOrderIDs = append(allocOrderIDs, oid)
	}
	assert.Equal(t, []uuid.UUID{order1.ID}, allocOrderIDs)
}

// CASE 6: Failed order followed by successful order -> successful order can allocate the unit released/not-held by failed order.
func TestAllocationLifecycle_Case6_FailedOrderFollowedBySuccessfulOrder(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	// Only 1 physical unit in warehouse
	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE6", 1)
	require.Len(t, unitIDs, 1)
	targetUnitID := unitIDs[0]

	// 1. Order 1 created by Buyer 1
	order1, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
	require.NoError(t, err)
	allocs1Pre, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1.Items[0].ID)
	require.NoError(t, err)
	require.Len(t, allocs1Pre, 1)
	assert.Equal(t, targetUnitID, allocs1Pre[0].InventoryUnitID)

	// 2. Order 1 payment fails (e.g. mock rejection or provider TLS failure)
	pResp1, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order1.ID, "tpay")
	require.NoError(t, err)
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp1.PaymentID, "reject")
	require.NoError(t, err)

	// 3. Order 1's allocation is released
	allocs1Post, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order1.Items[0].ID)
	require.NoError(t, err)
	assert.Empty(t, allocs1Post)

	// 4. Order 2 created by Buyer 2 -> succeeds and successfully allocates targetUnitID!
	order2, err := f.createOrderForUser(t, ctx, f.buyer2ID, variantID, 1)
	require.NoError(t, err)
	require.NotNil(t, order2)

	allocs2, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order2.Items[0].ID)
	require.NoError(t, err)
	require.Len(t, allocs2, 1)
	assert.Equal(t, targetUnitID, allocs2[0].InventoryUnitID)

	// 5. Order 2 payment is confirmed
	pResp2, err := f.paymentsSvc.CreatePayment(ctx, f.buyer2ID, order2.ID, "tpay")
	require.NoError(t, err)
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp2.PaymentID, "confirm")
	require.NoError(t, err)

	// Invariant: Order 2 is paid and legitimately owns targetUnitID
	updatedOrder2, err := f.ordersRepo.GetOrder(ctx, order2.ID)
	require.NoError(t, err)
	assert.Equal(t, "paid", updatedOrder2.Status)

	var activeOwner uuid.UUID
	err = f.db.QueryRow(ctx, `
		SELECT oi.order_id FROM order_item_allocations a
		JOIN order_items oi ON oi.id = a.order_item_id
		WHERE a.inventory_unit_id = $1 AND a.released_at IS NULL
	`, targetUnitID).Scan(&activeOwner)
	require.NoError(t, err)
	assert.Equal(t, order2.ID, activeOwner)
}

// CASE 7: Same-order retry after failure -> atomically reacquires stock before payment init, succeeds on confirmation.
func TestAllocationLifecycle_Case7_SameOrderRetry_ReacquiresStockAndSucceeds(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	// 1 unit in warehouse
	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE7", 1)
	require.Len(t, unitIDs, 1)

	// 1. Buyer creates order
	order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	orderItemID := order.Items[0].ID

	// Active hold exists initially
	allocsPre, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	require.Len(t, allocsPre, 1)

	// 2. Initial payment fails/rejected -> hold released
	paymentResp1, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
	require.NoError(t, err)
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, paymentResp1.PaymentID, "reject")
	require.NoError(t, err)

	// Invariant: hold is released
	allocsReleased, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Empty(t, allocsReleased)

	var reservedStockAfterFail int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&reservedStockAfterFail)
	require.NoError(t, err)
	assert.Equal(t, 0, reservedStockAfterFail)

	// 3. Retry payment on THE SAME ORDER
	paymentResp2, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
	require.NoError(t, err)
	require.NotNil(t, paymentResp2)

	// Invariant: BEFORE payment confirms, stock and allocation are atomically reacquired!
	allocsReacquired, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	require.Len(t, allocsReacquired, 1)
	assert.Equal(t, unitIDs[0], allocsReacquired[0].InventoryUnitID)

	var reservedStockReacquired int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&reservedStockReacquired)
	require.NoError(t, err)
	assert.Equal(t, 1, reservedStockReacquired)

	// 4. Confirm payment
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, paymentResp2.PaymentID, "confirm")
	require.NoError(t, err)

	// Final invariants:
	// - order.status = paid
	updatedOrder, err := f.ordersRepo.GetOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, "paid", updatedOrder.Status)

	// - exactly 1 converted reservation
	var convertedResCount int
	err = f.db.QueryRow(ctx, `
		SELECT count(*) FROM reservations r
		JOIN order_reservations ord_r ON ord_r.reservation_id = r.id
		WHERE ord_r.order_id = $1 AND r.status = 'converted'
	`, order.ID).Scan(&convertedResCount)
	require.NoError(t, err)
	assert.Equal(t, 1, convertedResCount)

	// - exactly quantity active allocations
	activeAllocsFinal, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
	require.NoError(t, err)
	assert.Len(t, activeAllocsFinal, 1)
	assert.Equal(t, unitIDs[0], activeAllocsFinal[0].InventoryUnitID)

	// - reserved_stock = quantity
	var finalReservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&finalReservedStock)
	require.NoError(t, err)
	assert.Equal(t, 1, finalReservedStock)
}

// CASE 8: Retry when stock was acquired by another order -> fails with ErrInsufficientStock before charging.
func TestAllocationLifecycle_Case8_SameOrderRetry_StockLost_RefusesPayment(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	// Only 1 unit in warehouse
	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE8", 1)
	require.Len(t, unitIDs, 1)

	// 1. Order A created by Buyer 1
	orderA, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
	require.NoError(t, err)

	// Payment fails -> hold releases
	pRespA1, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, orderA.ID, "tpay")
	require.NoError(t, err)
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pRespA1.PaymentID, "reject")
	require.NoError(t, err)

	// 2. Order B created by Buyer 2 legitimately acquires the only unit and pays
	orderB, err := f.createOrderForUser(t, ctx, f.buyer2ID, variantID, 1)
	require.NoError(t, err)
	pRespB, err := f.paymentsSvc.CreatePayment(ctx, f.buyer2ID, orderB.ID, "tpay")
	require.NoError(t, err)
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pRespB.PaymentID, "confirm")
	require.NoError(t, err)

	// Order B legitimately owns the unit
	orderBUpdated, err := f.ordersRepo.GetOrder(ctx, orderB.ID)
	require.NoError(t, err)
	assert.Equal(t, "paid", orderBUpdated.Status)

	// 3. Customer retries payment for Order A
	pRespA2, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, orderA.ID, "tpay")

	// Invariant: Retry fails BEFORE charging customer!
	assert.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrInsufficientStock))
	assert.Nil(t, pRespA2)

	// Invariant: Order A remains unpaid
	orderAUpdated, err := f.ordersRepo.GetOrder(ctx, orderA.ID)
	require.NoError(t, err)
	assert.Equal(t, "awaiting_payment", orderAUpdated.Status)

	// Invariant: Order B keeps the ZMU
	var activeOwner uuid.UUID
	err = f.db.QueryRow(ctx, `
		SELECT oi.order_id FROM order_item_allocations a
		JOIN order_items oi ON oi.id = a.order_item_id
		WHERE a.inventory_unit_id = $1 AND a.released_at IS NULL
	`, unitIDs[0]).Scan(&activeOwner)
	require.NoError(t, err)
	assert.Equal(t, orderB.ID, activeOwner)
}

// CASE 9: Two concurrent retry-payment requests for same released awaiting_payment order -> idempotent, no double reservation or allocation.
func TestAllocationLifecycle_Case9_SameOrderRetry_ConcurrentRequests_Idempotent(t *testing.T) {
	ctx := context.Background()
	f := setupAllocationLifecycleFixture(t, ctx)
	defer f.db.Close()

	variantID, unitIDs := f.createVariantWithUnits(t, ctx, "SKU-CASE9", 1)
	require.Len(t, unitIDs, 1)

	order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
	require.NoError(t, err)

	// Initial payment fails -> hold released
	pResp1, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
	require.NoError(t, err)
	err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp1.PaymentID, "reject")
	require.NoError(t, err)

	// Now two concurrent retry-payment requests are fired simultaneously
	type resHolder struct {
		resp *payments.CreatePaymentResponse
		err  error
	}
	ch := make(chan resHolder, 2)

	go func() {
		resp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		ch <- resHolder{resp: resp, err: err}
	}()
	go func() {
		resp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		ch <- resHolder{resp: resp, err: err}
	}()

	r1 := <-ch
	r2 := <-ch

	// At least one must succeed
	successCount := 0
	if r1.err == nil && r1.resp != nil {
		successCount++
	}
	if r2.err == nil && r2.resp != nil {
		successCount++
	}
	assert.GreaterOrEqual(t, successCount, 1)

	// Invariants:
	// - reserved_stock MUST be exactly 1 (NEVER double-incremented to 2)
	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 1, reservedStock)

	// - active allocations MUST be exactly 1 (NEVER duplicated)
	allocs, err := f.ordersRepo.ListActiveAllocationsForOrderItem(ctx, order.Items[0].ID)
	require.NoError(t, err)
	assert.Len(t, allocs, 1)

	// - active reservations for order MUST be exactly 1
	activeRes, err := f.ordersRepo.GetActiveOrderReservations(ctx, order.ID)
	require.NoError(t, err)
	assert.Len(t, activeRes, 1)
}

// =========================================================================
// OBSERVABILITY TEST MATRIX (Scenarios A - H)
// =========================================================================

func TestOrderAllocationLifecycle_Observability(t *testing.T) {
	ctx := context.Background()

	// SCENARIO A: Order Expiration
	t.Run("ScenarioA_OrderExpiration_EmitsHoldReleasedOnce", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		// Drain any pre-existing expired awaiting_payment orders in shared test DB
		_, _ = f.ordersSvc.ExpireAwaitingPaymentOrders(ctx, time.Now().Add(24*time.Hour), 1000)
		f.logBuf.Reset()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-A", 1)
		order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)

		// Clear logs from order creation
		f.logBuf.Reset()

		// Advance time past timeout
		futureTime := time.Now().Add(35 * time.Minute)
		res, err := f.ordersSvc.ExpireAwaitingPaymentOrders(ctx, futureTime, 10)
		require.NoError(t, err)
		assert.Equal(t, 1, res.Expired)

		// Assert exactly one inventory.order_hold_released event for our order
		events := f.findEventsByName("inventory.order_hold_released")
		var orderEvents []map[string]interface{}
		for _, e := range events {
			if e["order_id"] == order.ID.String() {
				orderEvents = append(orderEvents, e)
			}
		}
		require.Len(t, orderEvents, 1, "must emit exactly one release event upon expiration for order")
		ev := orderEvents[0]
		assert.Equal(t, "inventory", ev["domain"])
		assert.Equal(t, "release_order_hold", ev["action"])
		assert.Equal(t, "success", ev["result"])
		assert.Equal(t, "order_expired", ev["reason"])
		assert.Equal(t, "system", ev["actor_role"])
		assert.Equal(t, order.ID.String(), ev["order_id"])
		assert.NotEmpty(t, ev["order_number"])
		assert.Equal(t, float64(1), ev["reservations_released_count"])
		assert.Equal(t, float64(1), ev["allocations_released_count"])

		// Repeated expiration: must emit ZERO duplicate release events
		f.logBuf.Reset()
		res2, err := f.ordersSvc.ExpireAwaitingPaymentOrders(ctx, futureTime, 10)
		require.NoError(t, err)
		assert.Equal(t, 0, res2.Expired)
		events2 := f.findEventsByName("inventory.order_hold_released")
		var orderEvents2 []map[string]interface{}
		for _, e := range events2 {
			if e["order_id"] == order.ID.String() {
				orderEvents2 = append(orderEvents2, e)
			}
		}
		assert.Empty(t, orderEvents2, "repeated expiration must emit 0 duplicate release events")
	})

	// SCENARIO B: Customer Cancel
	t.Run("ScenarioB_CustomerCancel_EmitsHoldReleasedOnce", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-B", 1)
		order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)

		f.logBuf.Reset()

		// Customer cancels order
		err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order.ID)
		require.NoError(t, err)

		events := f.findEventsByName("inventory.order_hold_released")
		require.Len(t, events, 1, "must emit exactly one release event upon customer cancellation")
		ev := events[0]
		assert.Equal(t, "inventory", ev["domain"])
		assert.Equal(t, "release_order_hold", ev["action"])
		assert.Equal(t, "success", ev["result"])
		assert.Equal(t, "customer_cancelled", ev["reason"])
		assert.Equal(t, "customer", ev["actor_role"])
		assert.Equal(t, f.buyerID.String(), ev["actor_id"])
		assert.Equal(t, order.ID.String(), ev["order_id"])
		assert.NotEmpty(t, ev["order_number"])
		assert.Equal(t, float64(1), ev["reservations_released_count"])
		assert.Equal(t, float64(1), ev["allocations_released_count"])

		// Repeated cancel: returns error, emits 0 events
		f.logBuf.Reset()
		err = f.ordersSvc.CancelCustomerOrder(ctx, f.buyerID, order.ID)
		assert.Error(t, err)
		events2 := f.findEventsByName("inventory.order_hold_released")
		assert.Empty(t, events2, "repeated cancellation must emit 0 duplicate release events")
	})

	// SCENARIO C: Payment Failure / Rejection
	t.Run("ScenarioC_PaymentFailureOrRejection_EmitsHoldReleasedOnce", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-C", 1)
		order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)

		pResp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		require.NoError(t, err)

		f.logBuf.Reset()

		// Reject payment
		err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp.PaymentID, "reject")
		require.NoError(t, err)

		events := f.findEventsByName("inventory.order_hold_released")
		require.Len(t, events, 1, "must emit exactly one release event upon payment failure")
		ev := events[0]
		assert.Equal(t, "inventory", ev["domain"])
		assert.Equal(t, "payment_failed", ev["reason"])
		assert.Equal(t, "system", ev["actor_role"])
		assert.Equal(t, order.ID.String(), ev["order_id"])
		assert.Equal(t, float64(1), ev["reservations_released_count"])
		assert.Equal(t, float64(1), ev["allocations_released_count"])

		// Repeated reject: returns error, emits 0 events
		f.logBuf.Reset()
		err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp.PaymentID, "reject")
		assert.Error(t, err)
		events2 := f.findEventsByName("inventory.order_hold_released")
		assert.Empty(t, events2, "duplicate reject must emit 0 duplicate release events")
	})

	// SCENARIO D: Retry Success
	t.Run("ScenarioD_PaymentRetrySuccess_EmitsReacquiredOnce", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-D", 1)
		order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)

		// First payment rejected -> hold released
		pResp1, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		require.NoError(t, err)
		err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp1.PaymentID, "reject")
		require.NoError(t, err)

		f.logBuf.Reset()

		// Retry payment for SAME order
		pResp2, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		require.NoError(t, err)
		require.NotNil(t, pResp2)

		events := f.findEventsByName("payment.retry_inventory_reacquired")
		require.Len(t, events, 1, "must emit exactly one retry_inventory_reacquired on real reacquisition")
		ev := events[0]
		assert.Equal(t, "payment", ev["domain"])
		assert.Equal(t, "retry_reacquire_inventory", ev["action"])
		assert.Equal(t, "success", ev["result"])
		assert.Equal(t, "customer", ev["actor_role"])
		assert.Equal(t, f.buyerID.String(), ev["actor_id"])
		assert.Equal(t, order.ID.String(), ev["order_id"])
		assert.NotEmpty(t, ev["reservation_id"])
		assert.Equal(t, float64(1), ev["allocations_created_count"])

		// Immediate second CreatePayment while hold is active: must NOT emit retry_inventory_reacquired
		f.logBuf.Reset()
		pResp3, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		require.NoError(t, err)
		require.NotNil(t, pResp3)

		events2 := f.findEventsByName("payment.retry_inventory_reacquired")
		assert.Empty(t, events2, "when hold is already active and only refreshed, must not emit reacquired event")
	})

	// SCENARIO E: Retry Insufficient Stock
	t.Run("ScenarioE_PaymentRetryInsufficientStock_EmitsWarnRejected", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-E", 1)

		// Order A created and payment rejected -> hold released
		orderA, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)
		pRespA, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, orderA.ID, "tpay")
		require.NoError(t, err)
		err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pRespA.PaymentID, "reject")
		require.NoError(t, err)

		// Order B claims the remaining unit
		orderB, err := f.createOrderForUser(t, ctx, f.buyer2ID, variantID, 1)
		require.NoError(t, err)
		require.NotNil(t, orderB)

		f.logBuf.Reset()

		// Retry payment for Order A -> stock is lost!
		pRespA2, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, orderA.ID, "tpay")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, payments.ErrInsufficientStock))
		assert.Nil(t, pRespA2)

		// Assert exactly one WARN payment.retry_rejected event
		rejectedEvents := f.findEventsByName("payment.retry_rejected")
		require.Len(t, rejectedEvents, 1, "must emit exactly one payment.retry_rejected event")
		ev := rejectedEvents[0]
		assert.Equal(t, "payment", ev["domain"])
		assert.Equal(t, "retry_reacquire_inventory", ev["action"])
		assert.Equal(t, "rejected", ev["result"])
		assert.Equal(t, "insufficient_stock", ev["reason_code"])
		assert.Equal(t, "WARN", ev["level"])
		assert.Equal(t, orderA.ID.String(), ev["order_id"])

		// Assert ZERO reacquired events
		reacquiredEvents := f.findEventsByName("payment.retry_inventory_reacquired")
		assert.Empty(t, reacquiredEvents, "must not emit reacquired event on stock loss")
	})

	// SCENARIO F: Duplicate Confirmed Webhook
	t.Run("ScenarioF_DuplicateConfirmedWebhook_EmitsConfirmedOnce", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-F", 1)
		order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)

		pResp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		require.NoError(t, err)

		f.logBuf.Reset()

		// First confirmation
		err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp.PaymentID, "confirm")
		require.NoError(t, err)

		events := f.findEventsByName("payment.confirmed")
		require.Len(t, events, 1, "first confirmation must emit exactly one payment.confirmed event")
		ev := events[0]
		assert.Equal(t, "payment", ev["domain"])
		assert.Equal(t, "confirm_payment", ev["action"])
		assert.Equal(t, "success", ev["result"])
		assert.Equal(t, pResp.PaymentID.String(), ev["payment_id"])
		assert.Equal(t, order.ID.String(), ev["order_id"])

		// Repeated confirmation: returns error, emits 0 events
		f.logBuf.Reset()
		err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp.PaymentID, "confirm")
		assert.Error(t, err)
		events2 := f.findEventsByName("payment.confirmed")
		assert.Empty(t, events2, "duplicate confirmation must emit 0 duplicate payment.confirmed events")
	})

	// SCENARIO G: Rollback Safety (Zero Fake Mutation Success Events)
	t.Run("ScenarioG_Rollback_EmitsZeroSuccessEvents", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-G", 1)
		order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)

		f.logBuf.Reset()

		// Induce a transaction error during cancel/release
		err = f.pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
			// Do some DB mutations that should be rolled back
			_ = f.ordersRepo.SetOrderCancelledTx(ctx, tx, order.ID)
			return errors.New("simulated transaction failure")
		})
		assert.Error(t, err)

		// Assert zero success business events emitted
		events := f.parseLoggedEvents()
		for _, ev := range events {
			assert.NotEqual(t, "success", ev["result"], "must not emit SUCCESS event when transaction failed")
		}
	})

	// SCENARIO H: Privacy (No Customer PII / Secrets / Tokens in Logs)
	t.Run("ScenarioH_Privacy_NoSensitiveDataInLogs", func(t *testing.T) {
		f := setupAllocationLifecycleFixture(t, ctx)
		defer f.db.Close()

		variantID, _ := f.createVariantWithUnits(t, ctx, "OBS-SCEN-H", 1)
		order, err := f.createOrderForUser(t, ctx, f.buyerID, variantID, 1)
		require.NoError(t, err)

		pResp, err := f.paymentsSvc.CreatePayment(ctx, f.buyerID, order.ID, "tpay")
		require.NoError(t, err)

		err = f.paymentsSvc.ProcessMockPaymentAction(ctx, pResp.PaymentID, "reject")
		require.NoError(t, err)

		allLogs := f.logBuf.String()
		require.NotEmpty(t, allLogs)

		// Disallowed PII and secrets
		disallowed := []string{
			"buyer@example.com",
			"+79990000000",
			"Warehouse Road 1",
			"password",
			"Bearer ",
			"token",
			"secret",
		}

		for _, secret := range disallowed {
			assert.False(t, strings.Contains(allLogs, secret), "logs must not contain sensitive string %q", secret)
		}
	})
}
