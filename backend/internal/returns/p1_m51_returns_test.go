package returns_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type m51Fixture struct {
	client      *postgres.Client
	svc         *returns.Service
	notifSvc    *notifications.Service
	ordersRepo  *orders.Repository
	returnsRepo *returns.Repository

	userID    uuid.UUID
	sellerAID uuid.UUID
	sellerBID uuid.UUID
	sellerCID uuid.UUID
	catID     uuid.UUID
	prodAID   uuid.UUID
	varAID    uuid.UUID
	prodBID   uuid.UUID
	varBID    uuid.UUID
	prodCID   uuid.UUID
	varCID    uuid.UUID
}

func setupM51Fixture(t *testing.T) *m51Fixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	ctx := context.Background()

	client, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, client.Pool)

	t.Cleanup(func() {
		client.Close()
	})

	ordersRepo := orders.NewRepository(client.Pool)
	returnsRepo := returns.NewRepository(client.Pool)
	notifRepo := notifications.NewRepository(client)
	notifSvc := notifications.NewService(notifRepo, nil, nil)
	invSvc := inventory.NewService(nil, nil, client)

	payRepo := payments.NewRepository(client.Pool)
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	paySvc := payments.NewService(payRepo, ordersRepo, nil, nil, client, nil, cfg)
	payoutRepo := payouts.NewRepository(client.Pool)
	payoutSvc := payouts.NewService(payoutRepo, client, returnsRepo, ordersRepo, cfg, notifSvc)

	windowDays := 14
	svc := returns.NewService(returnsRepo, ordersRepo, invSvc, client, payoutSvc, paySvc, windowDays, notifSvc)

	fix := &m51Fixture{
		client:      client,
		svc:         svc,
		notifSvc:    notifSvc,
		ordersRepo:  ordersRepo,
		returnsRepo: returnsRepo,
		userID:      uuid.New(),
		sellerAID:   uuid.New(),
		sellerBID:   uuid.New(),
		sellerCID:   uuid.New(),
		catID:       uuid.New(),
		prodAID:     uuid.New(),
		varAID:      uuid.New(),
		prodBID:     uuid.New(),
		varBID:      uuid.New(),
		prodCID:     uuid.New(),
		varCID:      uuid.New(),
	}

	// Insert base catalog
	_, err = client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1, 'Customer', '+79991112233', $2, 'hash')", fix.userID, "cust_"+uuid.New().String()+"@test.com")
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Seller A', $2, 'a@test.com', 'active')", fix.sellerAID, "slug_a_"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Seller B', $2, 'b@test.com', 'active')", fix.sellerBID, "slug_b_"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Seller C', $2, 'c@test.com', 'active')", fix.sellerCID, "slug_c_"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Category', $2)", fix.catID, "cat_"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, price_cents) VALUES ($1, $2, $3, 'Product A', $4, 1000)", fix.prodAID, fix.sellerAID, fix.catID, "prod_a_"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, $3, 1000)", fix.varAID, fix.prodAID, "SKU-A-"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, price_cents) VALUES ($1, $2, $3, 'Product B', $4, 2000)", fix.prodBID, fix.sellerBID, fix.catID, "prod_b_"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, $3, 2000)", fix.varBID, fix.prodBID, "SKU-B-"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, price_cents) VALUES ($1, $2, $3, 'Product C', $4, 3000)", fix.prodCID, fix.sellerCID, fix.catID, "prod_c_"+uuid.New().String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, $3, 3000)", fix.varCID, fix.prodCID, "SKU-C-"+uuid.New().String())
	require.NoError(t, err)

	return fix
}

type testOrder struct {
	orderID       uuid.UUID
	fulfillmentID uuid.UUID
	shipmentID    uuid.UUID
	orderItemID   uuid.UUID
}

func (fix *m51Fixture) createDeliveredOrder(t *testing.T, deliveredAt time.Time, qty int) testOrder {
	t.Helper()
	ctx := context.Background()

	orderID := uuid.New()
	fID := uuid.New()
	shipmentID := uuid.New()
	oiID := uuid.New()

	orderNum := fmt.Sprintf("ORD-%s", uuid.New().String()[:12])
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', $4, 'RUB', 'Test Address', 'Courier', 0, 'Test User', 'test@example.com', '+79990001122', now(), now())
	`, orderID, fix.userID, orderNum, int64(qty*1000))
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Product A', 'slug-a', 1000, $7, $8)
	`, oiID, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID, int64(qty*1000), qty)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', $4, $5)
	`, shipmentID, orderID, fID, deliveredAt.Add(-24*time.Hour), deliveredAt)
	require.NoError(t, err)

	return testOrder{
		orderID:       orderID,
		fulfillmentID: fID,
		shipmentID:    shipmentID,
		orderItemID:   oiID,
	}
}

// ----------------------------------------------------------------------------
// 1. Schema Invariant Tests
// ----------------------------------------------------------------------------

func TestM51_SchemaInvariants(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	t.Cleanup(func() {
		fix.client.Pool.Exec(ctx, "DELETE FROM return_item_units")
	})

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)

	// A. Composite FK mismatch: Return(order_id = OtherOrder, fulfillment_id = tOrd.fulfillmentID) must FAIL
	otherOrderID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, 'ORD-OTHER-' || substr(md5(random()::text), 1, 8), 'delivered', 1000, 'RUB', 'Addr', 'Method', 0, 'Name', 'Email', 'Phone')
	`, otherOrderID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason)
		VALUES ($1, $2, $3, $4, 'requested', 'defective')
	`, uuid.New(), otherOrderID, tOrd.fulfillmentID, fix.userID)
	assert.Error(t, err, "Mismatch between order_id and fulfillment_id composite FK must fail")

	// B. Status 'receiving' is accepted
	validRetID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason)
		VALUES ($1, $2, $3, $4, 'receiving', 'defective')
	`, validRetID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	assert.NoError(t, err, "Status 'receiving' must be accepted by CHECK constraint")

	// C. Invalid arbitrary status is rejected
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason)
		VALUES ($1, $2, $3, $4, 'arbitrary_invalid_status', 'defective')
	`, uuid.New(), tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	assert.Error(t, err, "Arbitrary status must be rejected by valid_return_status CHECK constraint")

	// D. Negative inspection quantities rejected
	retItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
		VALUES ($1, $2, $3, 2, -1, 0, 0)
	`, retItemID, validRetID, tOrd.orderItemID)
	assert.Error(t, err, "Negative accepted_quantity must be rejected by valid_inspection_qtys CHECK constraint")

	// E. Inspection sum > quantity rejected
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
		VALUES ($1, $2, $3, 2, 2, 1, 0)
	`, retItemID, validRetID, tOrd.orderItemID)
	assert.Error(t, err, "Inspection sum exceeding quantity must be rejected by check_inspection_sum CHECK constraint")

	// Valid return item insert
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
		VALUES ($1, $2, $3, 2, 1, 1, 0)
	`, retItemID, validRetID, tOrd.orderItemID)
	require.NoError(t, err)

	// F. Duplicate allocation in return_item_units rejected
	allocID1 := uuid.New()
	invUnitID := uuid.New()
	supplyID := uuid.New()
	supplyItemID := uuid.New()

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, "ZMU-"+uuid.New().String()[:8], fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at)
		VALUES ($1, $2, $3, now())
	`, allocID1, tOrd.orderItemID, invUnitID)
	require.NoError(t, err)

	// First insert into return_item_units succeeds
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id)
		VALUES ($1, $2, $3)
	`, uuid.New(), retItemID, allocID1)
	require.NoError(t, err)

	// Duplicate insertion of SAME allocation must FAIL
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id)
		VALUES ($1, $2, $3)
	`, uuid.New(), retItemID, allocID1)
	assert.Error(t, err, "Duplicate order_item_allocation_id in return_item_units must be rejected by UNIQUE constraint")

	// G. Two DIFFERENT allocations of the SAME physical inventory unit across different orders are structurally ALLOWED
	// Release the first allocation to simulate completion of prior outbound cycle
	_, err = fix.client.Pool.Exec(ctx, "UPDATE order_item_allocations SET released_at = now() WHERE id = $1", allocID1)
	require.NoError(t, err)

	tOrd2 := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	allocID2 := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at)
		VALUES ($1, $2, $3, now())
	`, allocID2, tOrd2.orderItemID, invUnitID)
	require.NoError(t, err)

	retID2 := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason)
		VALUES ($1, $2, $3, $4, 'requested', 'defective')
	`, retID2, tOrd2.orderID, tOrd2.fulfillmentID, fix.userID)
	require.NoError(t, err)

	retItemID2 := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity)
		VALUES ($1, $2, $3, 1)
	`, retItemID2, retID2, tOrd2.orderItemID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id)
		VALUES ($1, $2, $3)
	`, uuid.New(), retItemID2, allocID2)
	assert.NoError(t, err, "Different allocation of the same ZMU in another outbound cycle must be structurally allowed")
}

// ----------------------------------------------------------------------------
// 2. Return Window & Eligibility Tests
// ----------------------------------------------------------------------------

func TestM51_ReturnWindowEligibility(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// A. Delivered shipment inside window (delivered 2 days ago, window = 14 days) -> Succeeds
	tOrdInside := fix.createDeliveredOrder(t, time.Now().Add(-2*24*time.Hour), 1)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdInside.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrdInside.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, tOrdInside.fulfillmentID, resp[0].Return.FulfillmentID)

	// B. Delivered shipment outside window (delivered 20 days ago) -> Fails with ErrReturnWindowExpired
	tOrdExpired := fix.createDeliveredOrder(t, time.Now().Add(-20*24*time.Hour), 1)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrdExpired.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrdExpired.orderItemID, Quantity: 1},
		},
	})
	assert.ErrorIs(t, err, returns.ErrReturnWindowExpired)

	// C. Mutating orders.updated_at to NOW() does NOT extend window when shipment.delivered_at is expired
	_, err = fix.client.Pool.Exec(ctx, "UPDATE orders SET updated_at = now() WHERE id = $1", tOrdExpired.orderID)
	require.NoError(t, err)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrdExpired.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrdExpired.orderItemID, Quantity: 1},
		},
	})
	assert.ErrorIs(t, err, returns.ErrReturnWindowExpired, "Window must use shipment.delivered_at, NEVER orders.updated_at")

	// D. Shipment status not delivered (status = 'shipped') -> Fails with ErrOrderNotDelivered
	tOrdShipped := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	_, err = fix.client.Pool.Exec(ctx, "UPDATE shipments SET status = 'shipped' WHERE id = $1", tOrdShipped.shipmentID)
	require.NoError(t, err)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrdShipped.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrdShipped.orderItemID, Quantity: 1},
		},
	})
	assert.ErrorIs(t, err, returns.ErrOrderNotDelivered)

	// E. Shipment delivered_at NULL -> Fails with ErrOrderNotDelivered
	tOrdNullDelivered := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	_, err = fix.client.Pool.Exec(ctx, "UPDATE shipments SET delivered_at = NULL WHERE id = $1", tOrdNullDelivered.shipmentID)
	require.NoError(t, err)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrdNullDelivered.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrdNullDelivered.orderItemID, Quantity: 1},
		},
	})
	assert.ErrorIs(t, err, returns.ErrOrderNotDelivered)

	// F. Fulfillment A and B with different delivered_at: Item A must use A timestamp, Item B must use B timestamp
	orderID_AB := uuid.New()
	fID_A := uuid.New()
	fID_B := uuid.New()
	oiID_A := uuid.New()
	oiID_B := uuid.New()

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 2000, 'RUB', 'Addr', 'Method', 0, 'Name', 'Email', 'Phone')
	`, orderID_AB, fix.userID, fmt.Sprintf("ORD-WIN-%s", uuid.New().String()[:8]))
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID_A, orderID_AB, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID_B, orderID_AB, fix.sellerBID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item A', 'slug-a', 1000, 1000, 1)
	`, oiID_A, orderID_AB, fID_A, fix.sellerAID, fix.prodAID, fix.varAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item B', 'slug-b', 1000, 1000, 1)
	`, oiID_B, orderID_AB, fID_B, fix.sellerBID, fix.prodBID, fix.varBID)
	require.NoError(t, err)

	// Fulfillment A delivered 2 days ago (inside window)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '3 days', now() - interval '2 days')
	`, uuid.New(), orderID_AB, fID_A)
	require.NoError(t, err)

	// Fulfillment B delivered 20 days ago (outside window)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '21 days', now() - interval '20 days')
	`, uuid.New(), orderID_AB, fID_B)
	require.NoError(t, err)

	// Item A alone succeeds using A's delivered_at
	respA, err := fix.svc.CreateReturn(ctx, fix.userID, orderID_AB, returns.CreateReturnRequest{
		Reason: "item_a_return",
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: oiID_A, Quantity: 1}},
	})
	require.NoError(t, err)
	assert.Len(t, respA, 1)
	assert.Equal(t, fID_A, respA[0].Return.FulfillmentID)

	// Item B alone fails with ErrReturnWindowExpired using B's delivered_at
	_, err = fix.svc.CreateReturn(ctx, fix.userID, orderID_AB, returns.CreateReturnRequest{
		Reason: "item_b_return",
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: oiID_B, Quantity: 1}},
	})
	assert.ErrorIs(t, err, returns.ErrReturnWindowExpired)
}

// ----------------------------------------------------------------------------
// 3. Concurrency and Quantity Limit Tests
// ----------------------------------------------------------------------------

func TestM51_ConcurrencyAndQuantityLimits(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// A. order_item.quantity = 1, two simultaneous CreateReturn requests qty 1
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	var wg sync.WaitGroup
	var successCount int
	var failCount int
	var mu sync.Mutex

	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
				Reason: "size_mismatch",
				Items: []returns.CreateReturnItemRequest{
					{OrderItemID: tOrd.orderItemID, Quantity: 1},
				},
			})
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, successCount, "Exactly one concurrent request must succeed")
	assert.Equal(t, 1, failCount, "Exactly one concurrent request must fail")

	var totalActiveReturnQty int
	err := fix.client.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(ri.quantity), 0)
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		WHERE ri.order_item_id = $1 AND r.status NOT IN ('rejected', 'cancelled')
	`, tOrd.orderItemID).Scan(&totalActiveReturnQty)
	require.NoError(t, err)
	assert.Equal(t, 1, totalActiveReturnQty, "Active returned quantity must be exactly 1")

	// B. Sequential partial returns for quantity = 5
	tOrd5 := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)

	// Step 1: Return 2 of 5 -> Succeeds (remaining 3)
	resp1, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd5.orderID, returns.CreateReturnRequest{
		Reason: "reason 1",
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd5.orderItemID, Quantity: 2}},
	})
	require.NoError(t, err)
	assert.Len(t, resp1, 1)

	// Step 2: Return 2 of 5 -> Succeeds (remaining 1)
	resp2, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd5.orderID, returns.CreateReturnRequest{
		Reason: "reason 2",
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd5.orderItemID, Quantity: 2}},
	})
	require.NoError(t, err)
	assert.Len(t, resp2, 1)

	// Step 3: Return 2 of 5 -> Fails because 2 + 2 + 2 > 5
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd5.orderID, returns.CreateReturnRequest{
		Reason: "reason 3",
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd5.orderItemID, Quantity: 2}},
	})
	assert.ErrorIs(t, err, returns.ErrInvalidQuantity)

	// Step 4: Return remaining 1 -> Succeeds
	resp4, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd5.orderID, returns.CreateReturnRequest{
		Reason: "reason 4",
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd5.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	assert.Len(t, resp4, 1)

	// Step 5: Return 1 more -> Fails (exhausted)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd5.orderID, returns.CreateReturnRequest{
		Reason: "reason 5",
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd5.orderItemID, Quantity: 1}},
	})
	assert.ErrorIs(t, err, returns.ErrInvalidQuantity)
}

// ----------------------------------------------------------------------------
// 4. Multi-Fulfillment Atomicity, Seller Isolation & Determinism
// ----------------------------------------------------------------------------

func TestM51_MultiFulfillmentAtomicityAndSellerIsolation(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Create multi-fulfillment order: Item A in Fulfillment A (Seller A), Item B in Fulfillment B (Seller B)
	orderID := uuid.New()
	fIDA := uuid.New()
	fIDB := uuid.New()
	shipIDA := uuid.New()
	shipIDB := uuid.New()
	oiIDA := uuid.New()
	oiIDB := uuid.New()

	orderNum := fmt.Sprintf("ORD-MF-%s", uuid.New().String()[:8])
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 3000, 'RUB', 'Address', 'Courier', 0, 'Customer', 'c@test.com', '+79991234567')
	`, orderID, fix.userID, orderNum)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fIDA, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fIDB, orderID, fix.sellerBID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES 
		($1, $2, $3, $4, $5, $6, 'Prod A', 'slug-a', 1000, 1000, 1)
	`, oiIDA, orderID, fIDA, fix.sellerAID, fix.prodAID, fix.varAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES 
		($1, $2, $3, $4, $5, $6, 'Prod B', 'slug-b', 2000, 2000, 1)
	`, oiIDB, orderID, fIDB, fix.sellerBID, fix.prodBID, fix.varBID)
	require.NoError(t, err)

	delivTimeA := time.Now().Add(-2 * 24 * time.Hour)
	delivTimeB := time.Now().Add(-1 * 24 * time.Hour)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', $4, $5)
	`, shipIDA, orderID, fIDA, delivTimeA.Add(-24*time.Hour), delivTimeA)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', $4, $5)
	`, shipIDB, orderID, fIDB, delivTimeB.Add(-24*time.Hour), delivTimeB)
	require.NoError(t, err)

	// Clean any previous test notifications for these sellers
	_, _ = fix.client.Pool.Exec(ctx, "DELETE FROM notifications WHERE recipient_seller_id IN ($1, $2)", fix.sellerAID, fix.sellerBID)

	// A. One submission with items from Fulfillment A and B -> exactly TWO Returns created
	responses, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason: "multi_return",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oiIDA, Quantity: 1},
			{OrderItemID: oiIDB, Quantity: 1},
		},
	})
	require.NoError(t, err)
	assert.Len(t, responses, 2, "Must create exactly 2 Return records for 2 fulfillments")

	returnMap := make(map[uuid.UUID]returns.ReturnResponse)
	for _, rResp := range responses {
		returnMap[rResp.Return.FulfillmentID] = rResp
	}

	retA, hasA := returnMap[fIDA]
	require.True(t, hasA, "Must contain return for Fulfillment A")
	assert.Equal(t, fIDA, retA.Return.FulfillmentID)
	assert.Len(t, retA.Items, 1)
	assert.Equal(t, oiIDA, retA.Items[0].OrderItemID)

	retB, hasB := returnMap[fIDB]
	require.True(t, hasB, "Must contain return for Fulfillment B")
	assert.Equal(t, fIDB, retB.Return.FulfillmentID)
	assert.Len(t, retB.Items, 1)
	assert.Equal(t, oiIDB, retB.Items[0].OrderItemID)

	// B. Verify Seller Notification Isolation (Seller A notified for Return A, Seller B for Return B)
	var notifSellerAEntity uuid.UUID
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT entity_id FROM notifications WHERE recipient_seller_id = $1 AND entity_type = 'return' ORDER BY created_at DESC LIMIT 1
	`, fix.sellerAID).Scan(&notifSellerAEntity)
	require.NoError(t, err)
	assert.Equal(t, retA.Return.ID, notifSellerAEntity, "Seller A must receive notification strictly for Return A")

	var notifSellerBEntity uuid.UUID
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT entity_id FROM notifications WHERE recipient_seller_id = $1 AND entity_type = 'return' ORDER BY created_at DESC LIMIT 1
	`, fix.sellerBID).Scan(&notifSellerBEntity)
	require.NoError(t, err)
	assert.Equal(t, retB.Return.ID, notifSellerBEntity, "Seller B must receive notification strictly for Return B")

	// C. Multi-fulfillment Atomicity: If one fulfillment is ineligible, entire transaction rolls back (0 returns)
	orderID2 := uuid.New()
	fIDA2 := uuid.New()
	fIDB2 := uuid.New()
	oiIDA2 := uuid.New()
	oiIDB2 := uuid.New()

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 3000, 'RUB', 'Address', 'Courier', 0, 'Customer', 'c@test.com', '+79991234567')
	`, orderID2, fix.userID, fmt.Sprintf("ORD-ATOMIC-%s", uuid.New().String()[:8]))
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fIDA2, orderID2, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'shipped')
	`, fIDB2, orderID2, fix.sellerBID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES 
		($1, $2, $3, $4, $5, $6, 'Prod A2', 'slug-a2', 1000, 1000, 1)
	`, oiIDA2, orderID2, fIDA2, fix.sellerAID, fix.prodAID, fix.varAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES 
		($1, $2, $3, $4, $5, $6, 'Prod B2', 'slug-b2', 2000, 2000, 1)
	`, oiIDB2, orderID2, fIDB2, fix.sellerBID, fix.prodBID, fix.varBID)
	require.NoError(t, err)

	// Fulfillment A is delivered, but Fulfillment B is only 'shipped' (ineligible)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now(), now())
	`, uuid.New(), orderID2, fIDA2)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'shipped', now(), NULL)
	`, uuid.New(), orderID2, fIDB2)
	require.NoError(t, err)

	_, err = fix.svc.CreateReturn(ctx, fix.userID, orderID2, returns.CreateReturnRequest{
		Reason: "atomic_test",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oiIDA2, Quantity: 1},
			{OrderItemID: oiIDB2, Quantity: 1},
		},
	})
	assert.ErrorIs(t, err, returns.ErrOrderNotDelivered, "Should fail because fulfillment B is not delivered")

	var returnCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM returns WHERE order_id = $1", orderID2).Scan(&returnCount)
	require.NoError(t, err)
	assert.Equal(t, 0, returnCount, "Zero returns must be persisted when any fulfillment group fails")
}

func TestM51_MultiFulfillmentDeterminism(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Create 3 fulfillments in one order
	orderID := uuid.New()
	fID1 := uuid.New()
	fID2 := uuid.New()
	fID3 := uuid.New()
	oiID1 := uuid.New()
	oiID2 := uuid.New()
	oiID3 := uuid.New()

	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 6000, 'RUB', 'Addr', 'Method', 0, 'Name', 'Email', 'Phone')
	`, orderID, fix.userID, fmt.Sprintf("ORD-DET-%s", uuid.New().String()[:8]))
	require.NoError(t, err)

	fIDs := []uuid.UUID{fID1, fID2, fID3}
	sellers := []uuid.UUID{fix.sellerAID, fix.sellerBID, fix.sellerCID}
	prods := []uuid.UUID{fix.prodAID, fix.prodBID, fix.prodCID}
	vars := []uuid.UUID{fix.varAID, fix.varBID, fix.varCID}
	oiIDs := []uuid.UUID{oiID1, oiID2, oiID3}

	for i := 0; i < 3; i++ {
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status)
			VALUES ($1, $2, $3, 'delivered')
		`, fIDs[i], orderID, sellers[i])
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
			VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 1000, 1000, 1)
		`, oiIDs[i], orderID, fIDs[i], sellers[i], prods[i], vars[i])
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
			VALUES ($1, $2, $3, 'delivered', now() - interval '2 days', now() - interval '1 day')
		`, uuid.New(), orderID, fIDs[i])
		require.NoError(t, err)
	}

	// Submit items in arbitrary / non-sorted order (3, 1, 2)
	responses, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason: "det_test",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oiID3, Quantity: 1},
			{OrderItemID: oiID1, Quantity: 1},
			{OrderItemID: oiID2, Quantity: 1},
		},
	})
	require.NoError(t, err)
	require.Len(t, responses, 3)

	// Sorted expected UUIDs
	expectedSortedFIDs := []uuid.UUID{fID1, fID2, fID3}
	sort.Slice(expectedSortedFIDs, func(i, j int) bool {
		return expectedSortedFIDs[i].String() < expectedSortedFIDs[j].String()
	})

	for i := 0; i < 3; i++ {
		assert.Equal(t, expectedSortedFIDs[i], responses[i].Return.FulfillmentID, "Response array must be deterministically sorted by fulfillment_id ascending")
	}
}

// ----------------------------------------------------------------------------
// 5. No Side Effects & No Physical Mutation Tests
// ----------------------------------------------------------------------------

func TestM51_NoSideEffects(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	// Setup full physical inventory, supply, unit, reservation and allocation structure
	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-NSE-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, "ZMU-NSE-"+uuid.New().String()[:8], fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, tOrd.orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	// Snapshot DB state before CreateReturn
	var totalStockBefore, reservedStockBefore int
	err = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStockBefore, &reservedStockBefore)
	require.NoError(t, err)

	var unitStatusBefore string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&unitStatusBefore)
	require.NoError(t, err)

	var resStatusBefore string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM reservations WHERE id = $1", resID).Scan(&resStatusBefore)
	require.NoError(t, err)

	var pickedAtBefore *time.Time
	var releasedAtBefore *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT picked_at, released_at FROM order_item_allocations WHERE id = $1", allocID).Scan(&pickedAtBefore, &releasedAtBefore)
	require.NoError(t, err)

	var refundCountBefore int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE order_id = $1", tOrd.orderID).Scan(&refundCountBefore)
	require.NoError(t, err)

	var stockMovementsBefore int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&stockMovementsBefore)
	require.NoError(t, err)

	var returnUnitsBefore int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM return_item_units WHERE order_item_allocation_id = $1", allocID).Scan(&returnUnitsBefore)
	require.NoError(t, err)

	// Execute CreateReturn
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "no_side_effects",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)

	// Assertions after CreateReturn
	var totalStockAfter, reservedStockAfter int
	err = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStockAfter, &reservedStockAfter)
	require.NoError(t, err)
	assert.Equal(t, totalStockBefore, totalStockAfter, "total_stock must not be mutated by CreateReturn")
	assert.Equal(t, reservedStockBefore, reservedStockAfter, "reserved_stock must not be mutated by CreateReturn")

	var unitStatusAfter string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&unitStatusAfter)
	require.NoError(t, err)
	assert.Equal(t, "shipped", unitStatusAfter, "ZMU unit status must remain 'shipped'")

	var resStatusAfter string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM reservations WHERE id = $1", resID).Scan(&resStatusAfter)
	require.NoError(t, err)
	assert.Equal(t, "converted", resStatusAfter, "Reservation status must remain 'converted'")

	var pickedAtAfter *time.Time
	var releasedAtAfter *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT picked_at, released_at FROM order_item_allocations WHERE id = $1", allocID).Scan(&pickedAtAfter, &releasedAtAfter)
	require.NoError(t, err)
	assert.Equal(t, pickedAtBefore.Unix(), pickedAtAfter.Unix(), "picked_at must remain unchanged")
	assert.Nil(t, releasedAtAfter, "released_at must remain NULL")

	var refundCountAfter int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE order_id = $1", tOrd.orderID).Scan(&refundCountAfter)
	require.NoError(t, err)
	assert.Equal(t, refundCountBefore, refundCountAfter, "refunds count must not change on CreateReturn")

	var stockMovementsAfter int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&stockMovementsAfter)
	require.NoError(t, err)
	assert.Equal(t, stockMovementsBefore, stockMovementsAfter, "stock_movements count must not change on CreateReturn")

	var returnUnitsAfter int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM return_item_units WHERE order_item_allocation_id = $1", allocID).Scan(&returnUnitsAfter)
	require.NoError(t, err)
	assert.Equal(t, 0, returnUnitsAfter, "return_item_units count must remain 0 during CreateReturn (no ZMU bound)")
}

// ----------------------------------------------------------------------------
// 6. Old Restock & Refund Safety Regression Test
// ----------------------------------------------------------------------------

func TestM51_OldRestockSafety(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	// Create inventory item & stock record
	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	// Create payment in status succeeded so CreateRefund can reserve refund
	paymentID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key)
		VALUES ($1, $2, 'tbank', 'P-RESTOCK-1', 'succeeded', 1000, 'RUB', $3)
	`, paymentID, tOrd.orderID, "IDEM-"+uuid.New().String())
	require.NoError(t, err)

	// Create ZMU inventory unit in status 'shipped'
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-RESTOCK-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, "ZMU-RESTOCK-"+uuid.New().String()[:8], fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	// 1. Create return
	responses, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "restock_safety_test",
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	ret := responses[0].Return
	retItem := responses[0].Items[0]

	// 2. Approve return
	adminID := uuid.New()
	err = fix.svc.UpdateReturnStatus(ctx, adminID, ret.ID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	})
	require.NoError(t, err)

	// 3. Update return status to item_received with Restock = true
	err = fix.svc.UpdateReturnStatus(ctx, adminID, ret.ID, returns.UpdateReturnStatusRequest{
		Status: "item_received",
		ItemRestock: []returns.UpdateReturnItemRestockRequest{
			{ReturnItemID: retItem.ID, Restock: true},
		},
	})
	require.NoError(t, err)

	// Snapshot total stock before CreateRefund
	var totalStockBefore int
	err = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStockBefore)
	require.NoError(t, err)
	assert.Equal(t, 10, totalStockBefore)

	// 4. Execute CreateRefund via normal service path
	refundReason := "customer_return_refund"
	refund, err := fix.svc.CreateRefund(ctx, adminID, ret.ID, returns.CreateRefundRequest{
		Reason: &refundReason,
	})
	require.NoError(t, err)
	assert.NotNil(t, refund)

	// 5. Assert total_stock remains EXACTLY unchanged (10, not 11)
	var totalStockAfter int
	err = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStockAfter)
	require.NoError(t, err)
	assert.Equal(t, totalStockBefore, totalStockAfter, "total_stock must NOT be incremented by CreateRefund / return status update")

	// 6. Assert no return stock movement was created
	var returnMovementsCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE inventory_item_id = $1 AND reason ILIKE '%return%'", invItemID).Scan(&returnMovementsCount)
	require.NoError(t, err)
	assert.Equal(t, 0, returnMovementsCount, "No return stock movement must be created")

	// 7. Assert serialized inventory unit remains 'shipped'
	var unitStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&unitStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", unitStatus, "ZMU unit status must remain 'shipped'")

	// 8. Assert financial refund reservation succeeded
	var refundCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT count(*) FROM refunds WHERE return_id = $1 AND status = 'pending'", ret.ID).Scan(&refundCount)
	require.NoError(t, err)
	assert.Equal(t, 1, refundCount, "Financial refund reservation must exist in pending status")
}

// ----------------------------------------------------------------------------
// 7. API Contract Test (Single and Multi Fulfillment HTTP Response Structure)
// ----------------------------------------------------------------------------

func TestM51_APIContract(t *testing.T) {
	fix := setupM51Fixture(t)

	h := returns.NewHandler(fix.svc)

	// A. Single fulfillment HTTP response contract
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	reqBodySingle := fmt.Sprintf(`{
		"reason": "contract_test",
		"items": [{"orderItemId": "%s", "quantity": 1}]
	}`, tOrd.orderItemID)

	reqSingle, err := http.NewRequest("POST", "/customer/orders/"+tOrd.orderID.String()+"/returns", strings.NewReader(reqBodySingle))
	require.NoError(t, err)
	reqSingle = reqSingle.WithContext(context.WithValue(reqSingle.Context(), "userID", fix.userID))
	
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tOrd.orderID.String())
	reqSingle = reqSingle.WithContext(context.WithValue(reqSingle.Context(), chi.RouteCtxKey, rctx))

	rrSingle := httptest.NewRecorder()
	h.CreateCustomerReturn(rrSingle, reqSingle)

	assert.Equal(t, http.StatusCreated, rrSingle.Code)

	var jsonMapSingle map[string]interface{}
	err = json.Unmarshal(rrSingle.Body.Bytes(), &jsonMapSingle)
	require.NoError(t, err)

	// Assert root-level fields exist for single return backward compatibility
	assert.NotEmpty(t, jsonMapSingle["id"], "Root id must exist for backward compatibility")
	assert.NotEmpty(t, jsonMapSingle["status"], "Root status must exist for backward compatibility")
	assert.NotEmpty(t, jsonMapSingle["fulfillmentId"], "Root fulfillmentId must exist")
	assert.NotEmpty(t, jsonMapSingle["items"], "Root items array must exist")

	// Assert returns array exists and contains 1 element
	returnsArray, ok := jsonMapSingle["returns"].([]interface{})
	require.True(t, ok, "returns array must be present in response")
	assert.Len(t, returnsArray, 1)

	var singleResp returns.CreateReturnResponse
	err = json.Unmarshal(rrSingle.Body.Bytes(), &singleResp)
	require.NoError(t, err)
	assert.Equal(t, tOrd.fulfillmentID, singleResp.FulfillmentID)
	assert.Len(t, singleResp.Returns, 1)
	assert.Equal(t, tOrd.fulfillmentID, singleResp.Returns[0].FulfillmentID)
}
