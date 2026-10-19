package fulfillment_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
)

type holdParts struct {
	fulfillmentID uuid.UUID
	orderItemID   uuid.UUID
	shipmentID    uuid.UUID
}

func (f *deliveryHoldFixture) newSeller(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Hold Test Seller 2', $2, $3, 'active', now(), now())
	`, id, uuid.New().String(), uuid.New().String()+"@seller.com")
	require.NoError(t, err)
	return id
}

// addShippedPart adds one more shipped fulfillment (one item, one shipment) to an existing order.
func (f *deliveryHoldFixture) addShippedPart(t *testing.T, ctx context.Context, orderID, sellerID uuid.UUID, priceCents int64) holdParts {
	t.Helper()
	p := holdParts{fulfillmentID: uuid.New(), orderItemID: uuid.New(), shipmentID: uuid.New()}

	_, err := f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'shipped', $4, 900, $5)
	`, p.fulfillmentID, orderID, sellerID, priceCents, priceCents-(priceCents*900/10000))
	require.NoError(t, err)

	catID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Test Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)
	prodID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Test Prod', $4, $5, 'published', now(), now())`, prodID, sellerID, catID, uuid.New().String(), priceCents)
	require.NoError(t, err)
	variantID := uuid.New()
	barcode := fmt.Sprintf("46%011d", rand.Int63n(90000000000)+10000000000)
	_, err = f.db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, true, now(), now())`, variantID, prodID, "SKU-"+uuid.New().String()[:8], uuid.New().String(), barcode, priceCents)
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		VALUES ($1, $2, $3, $4, $5, 'Test Prod', 'slug', $6, 1, $6, $7, 1)
	`, p.orderItemID, orderID, prodID, variantID, sellerID, priceCents, p.fulfillmentID)
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 'CDEK', $4, 'https://cdek.ru/trk', now() - interval '1 hour', now(), now())
	`, p.shipmentID, orderID, p.fulfillmentID, "TRK-"+uuid.New().String()[:8])
	require.NoError(t, err)
	return p
}

// insertEarning inserts a pending seller_earning entry (available_at NULL unless given) for an order item.
func (f *deliveryHoldFixture) insertEarning(t *testing.T, ctx context.Context, orderID, orderItemID, sellerID uuid.UUID, availableAt *time.Time, batchID *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, payout_batch_id, type, amount_cents, currency, available_at, created_at)
		VALUES ($1, $2, $3, $4, $5, 'seller_earning', 1000, 'RUB', $6, now())
	`, id, sellerID, orderID, orderItemID, batchID, availableAt)
	require.NoError(t, err)
	return id
}

func (f *deliveryHoldFixture) availableAt(t *testing.T, ctx context.Context, entryID uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	require.NoError(t, f.db.QueryRow(ctx, `SELECT available_at FROM seller_ledger_entries WHERE id = $1`, entryID).Scan(&at))
	return at
}

func (f *deliveryHoldFixture) earningFor(t *testing.T, ctx context.Context, orderItemID uuid.UUID) (uuid.UUID, int64, *time.Time) {
	t.Helper()
	var id uuid.UUID
	var amount int64
	var at *time.Time
	require.NoError(t, f.db.QueryRow(ctx, `SELECT id, amount_cents, available_at FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, orderItemID).Scan(&id, &amount, &at))
	return id, amount, at
}

func (f *deliveryHoldFixture) deliveredAt(t *testing.T, ctx context.Context, shipmentID uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	require.NoError(t, f.db.QueryRow(ctx, `SELECT delivered_at FROM shipments WHERE id = $1`, shipmentID).Scan(&at))
	return at
}

func (f *deliveryHoldFixture) assertTriple(t *testing.T, ctx context.Context, orderItemID uuid.UUID, price int64) {
	t.Helper()
	comm := price * 900 / 10000
	want := map[string]int64{"sale_gross": price, "zamk_commission": -comm, "seller_earning": price - comm}
	rows, err := f.db.Query(ctx, `SELECT type, amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type IN ('sale_gross','zamk_commission','seller_earning')`, orderItemID)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]int64{}
	n := 0
	for rows.Next() {
		var typ string
		var amt int64
		require.NoError(t, rows.Scan(&typ, &amt))
		got[typ] = amt
		n++
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, 3, n, "exactly one gross, commission and earning per item")
	assert.Equal(t, want, got)
}

// Two sellers in one order, delivered on different dates: each fulfillment gets its own deadline.
func TestDeliverShipment_TwoSellersIndependentHold(t *testing.T) {
	ctx := context.Background()
	f := setupDeliveryHoldFixture(t, ctx)
	defer f.client.Close()

	orderID, _, itemA, shipA := f.createShippedOrderWithItem(t, ctx, 3000, 1)
	sellerB := f.newSeller(t, ctx)
	partB := f.addShippedPart(t, ctx, orderID, sellerB, 5000)

	_, err := f.svc.DeliverShipment(ctx, f.adminID, shipA, fulfillment.DeliverShipmentRequest{})
	require.NoError(t, err)

	earnA, amountA, atA := f.earningFor(t, ctx, itemA)
	require.NotNil(t, atA)
	assert.True(t, atA.Equal(f.deliveredAt(t, ctx, shipA).AddDate(0, 0, 14)))
	assert.Equal(t, int64(2730), amountA)
	var cntB int
	require.NoError(t, f.db.QueryRow(ctx, `SELECT COUNT(*) FROM seller_ledger_entries WHERE order_item_id = $1`, partB.orderItemID).Scan(&cntB))
	assert.Equal(t, 0, cntB, "delivering seller A must not create or touch seller B entries")

	time.Sleep(20 * time.Millisecond)
	_, err = f.svc.DeliverShipment(ctx, f.adminID, partB.shipmentID, fulfillment.DeliverShipmentRequest{})
	require.NoError(t, err)

	_, amountB, atB := f.earningFor(t, ctx, partB.orderItemID)
	require.NotNil(t, atB)
	assert.True(t, atB.Equal(f.deliveredAt(t, ctx, partB.shipmentID).AddDate(0, 0, 14)))
	assert.Equal(t, int64(4550), amountB)
	assert.True(t, atB.After(*atA), "seller B deadline must follow its own later delivery")

	assert.True(t, f.availableAt(t, ctx, earnA).Equal(*atA), "seller A deadline must not move when B is delivered")
	f.assertTriple(t, ctx, itemA, 3000)
	f.assertTriple(t, ctx, partB.orderItemID, 5000)
}

// One seller: a multi-item fulfillment plus another order of the same seller (a seller has at most one
// fulfillment per order). Delivering one fulfillment must not touch entries of the other order,
// entries linked to a payout batch, or entries that already have a deadline.
func TestDeliverShipment_SameSellerFulfillmentsAndPayoutProtection(t *testing.T) {
	ctx := context.Background()
	f := setupDeliveryHoldFixture(t, ctx)
	defer f.client.Close()

	orderID, fulfillment1, item1, ship1 := f.createShippedOrderWithItem(t, ctx, 3000, 1)

	// Second item of the same seller inside the same fulfillment.
	item1b := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity)
		SELECT $1, order_id, product_id, product_variant_id, seller_id, title, product_slug, 1000, 1, 1000, order_fulfillment_id, 1
		FROM order_items WHERE id = $2`, item1b, item1)
	require.NoError(t, err)

	// Another order of the same seller with a pending earning (not delivered yet).
	order2, _, item2, _ := f.createShippedOrderWithItem(t, ctx, 2000, 1)
	sentinel := f.insertEarning(t, ctx, order2, item2, f.sellerID, nil, nil)

	// Earning of the delivered fulfillment already tied to a payout batch (must never be rewritten).
	batchID := uuid.New()
	_, err = f.db.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for) VALUES ($1, $2, 1000, 'scheduled', now())`, batchID, f.sellerID)
	require.NoError(t, err)
	batched := f.insertEarning(t, ctx, orderID, item1, f.sellerID, nil, &batchID)

	// Earning of the delivered fulfillment that already has a deadline (must keep it).
	fixed := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Microsecond)
	withDeadline := f.insertEarning(t, ctx, orderID, item1, f.sellerID, &fixed, nil)

	_, err = f.svc.DeliverShipment(ctx, f.adminID, ship1, fulfillment.DeliverShipmentRequest{})
	require.NoError(t, err)

	assert.Nil(t, f.availableAt(t, ctx, sentinel), "another order of the same seller must keep its state")
	assert.Nil(t, f.availableAt(t, ctx, batched), "payout-linked entry must not be rewritten")
	got := f.availableAt(t, ctx, withDeadline)
	require.NotNil(t, got)
	assert.True(t, got.Equal(fixed), "existing deadline must not move")

	// Both items of the delivered fulfillment get the same hold from the delivery moment.
	want := f.deliveredAt(t, ctx, ship1).AddDate(0, 0, 14)
	for _, item := range []uuid.UUID{item1, item1b} {
		var at *time.Time
		require.NoError(t, f.db.QueryRow(ctx, `
			SELECT available_at FROM seller_ledger_entries
			WHERE order_item_id = $1 AND type = 'seller_earning' AND id NOT IN ($2, $3)`, item, batched, withDeadline).Scan(&at))
		require.NotNil(t, at)
		assert.True(t, at.Equal(want))
	}
	_ = fulfillment1
}

// Re-running the hold step for the same fulfillment is a no-op and keeps the first deadline.
func TestMarkFulfillmentDelivered_RepeatKeepsDeadline(t *testing.T) {
	ctx := context.Background()
	f := setupDeliveryHoldFixture(t, ctx)
	defer f.client.Close()

	orderID, fulfillmentID, item, ship := f.createShippedOrderWithItem(t, ctx, 4000, 1)
	_, err := f.svc.DeliverShipment(ctx, f.adminID, ship, fulfillment.DeliverShipmentRequest{})
	require.NoError(t, err)

	id, _, first := f.earningFor(t, ctx, item)
	require.NotNil(t, first)

	require.NoError(t, f.client.RunInTx(ctx, func(tx pgx.Tx) error {
		return f.payoutSvc.MarkFulfillmentDeliveredTx(ctx, tx, fulfillmentID, time.Now().Add(72*time.Hour))
	}))
	again := f.availableAt(t, ctx, id)
	require.NotNil(t, again)
	assert.True(t, again.Equal(*first), "repeated processing must not move the deadline")

	var cnt int
	require.NoError(t, f.db.QueryRow(ctx, `SELECT COUNT(*) FROM seller_ledger_entries WHERE order_id = $1`, orderID).Scan(&cnt))
	assert.Equal(t, 3, cnt)
	f.assertTriple(t, ctx, item, 4000)
}
