package fulfillment_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type pickingFixture struct {
	db       *pgxpool.Pool
	svc      *fulfillment.Service
	sellerID uuid.UUID
	adminID  uuid.UUID
}

func setupPickingFixture(t *testing.T, ctx context.Context) *pickingFixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	require.NotEmpty(t, dbURL, "test database URL must not be empty")

	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, db)

	postgresClient, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)

	repo := fulfillment.NewRepository(db)
	ordersRepo := orders.NewRepository(db)
	svc := fulfillment.NewService(repo, ordersRepo, postgresClient, nil, nil) // Dependencies mocked/nil as this is a read model test

	sellerID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Picking Seller', $2, $3, 'active', now(), now())
	`, sellerID, uuid.New().String(), uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	adminID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Admin', $2, 'hash', 'admin', 'active', now(), now())
	`, adminID, uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	return &pickingFixture{
		db:       db,
		svc:      svc,
		sellerID: sellerID,
		adminID:  adminID,
	}
}

func (f *pickingFixture) createOrderAndFulfillment(t *testing.T, ctx context.Context, orderStatus, fulfillmentStatus string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	orderID := uuid.New()
	userID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Buyer', $2, 'hash', 'customer', 'active', now(), now())
	`, userID, uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, $3, 1000, 'N', 'P', 'E', 'A')
	`, orderID, userID, orderStatus)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, $4, 1000, 900, 900)
	`, fulfillmentID, orderID, f.sellerID, fulfillmentStatus)
	require.NoError(t, err)

	return orderID, fulfillmentID
}

func (f *pickingFixture) createOrderItem(t *testing.T, ctx context.Context, orderID, fulfillmentID uuid.UUID, quantity int, pickedQuantity int) uuid.UUID {
	t.Helper()
	itemID := uuid.New()
	prodID := uuid.New()
	variantID := uuid.New()
	catID := uuid.New()

	_, err := f.db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, 1000, 'published', now(), now())`, prodID, f.sellerID, catID, uuid.New().String())
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())`, variantID, prodID, uuid.New().String(), uuid.New().String(), uuid.New().String())
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Item Title', 'slug', 100, $6, 100, $7, $8)
	`, itemID, orderID, prodID, variantID, f.sellerID, quantity, fulfillmentID, pickedQuantity)
	require.NoError(t, err)

	return itemID
}

func (f *pickingFixture) createAllocation(t *testing.T, ctx context.Context, orderItemID uuid.UUID, picked bool) {
	t.Helper()
	var variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, orderItemID).Scan(&variantID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())`, supplyID, f.sellerID, uuid.New().String()[:8])
	require.NoError(t, err)
	_, err = f.db.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status) VALUES ($1, $2, $3, $4, $5, 1, 'warehouse')`, unitID, uuid.New().String()[:12], variantID, supplyID, supplyItemID)
	require.NoError(t, err)

	allocID := uuid.New()
	var pickedAt interface{}
	if picked {
		pickedAt = time.Now()
	}
	_, err = f.db.Exec(ctx, `INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at) VALUES ($1, $2, $3, $4)`, allocID, orderItemID, unitID, pickedAt)
	require.NoError(t, err)
}

func TestPickingRead_AwaitingPayment_Rejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	_, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "awaiting_payment", "awaiting_payment")
	
	_, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.ErrorIs(t, err, fulfillment.ErrPickingNotAllowed)
}

func TestPickingRead_Paid_Allowed(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	_, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	
	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, "paid", po.FulfillmentStatus)
}

func TestPickingRead_Assembling_Allowed(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	_, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "assembling", "assembling")
	
	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	assert.Equal(t, "assembling", po.FulfillmentStatus)
}

func TestPickingRead_Classification_Serialized(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	
	// 2 active allocations => serialized
	f.createAllocation(t, ctx, itemID, false)
	f.createAllocation(t, ctx, itemID, true) // one picked

	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)

	item := po.Items[0]
	assert.Equal(t, "serialized", item.AllocationMode)
	assert.Equal(t, 2, item.Quantity)
	assert.Equal(t, 1, item.PickedQuantity, "count of picked_at != NULL")
	assert.Equal(t, 1, item.RemainingQuantity)
	assert.Len(t, item.AllocatedUnits, 2)
}

func TestPickingRead_Classification_Legacy(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	// 0 active allocations, picked_quantity = 1 => legacy
	_ = f.createOrderItem(t, ctx, orderID, fulfillmentID, 3, 1)

	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)

	item := po.Items[0]
	assert.Equal(t, "legacy", item.AllocationMode)
	assert.Equal(t, 3, item.Quantity)
	assert.Equal(t, 1, item.PickedQuantity, "uses order_items.picked_quantity")
	assert.Equal(t, 2, item.RemainingQuantity)
	assert.Len(t, item.AllocatedUnits, 0)
}

func TestPickingRead_Classification_InvalidPartial(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 3, 0)
	
	// quantity = 3, but 1 active allocation => INVALID
	f.createAllocation(t, ctx, itemID, false)

	_, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.ErrorIs(t, err, fulfillment.ErrInvariantViolation)
}

func TestPickingRead_Classification_InvalidOverallocation(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	
	// quantity = 2, but 3 active allocations => INVALID
	f.createAllocation(t, ctx, itemID, false)
	f.createAllocation(t, ctx, itemID, false)
	f.createAllocation(t, ctx, itemID, false)

	_, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.ErrorIs(t, err, fulfillment.ErrInvariantViolation)
}
