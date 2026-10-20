package payouts_test

// MARKETING.M0.2 — commission rate compatibility of historical orders (zamk_test).
//
// order_fulfillments.commission_frozen = TRUE marks fulfillments whose rate was frozen at checkout.
// FALSE rows (created before M0.1, or by an old checkout during a rolling deploy) only hold the column
// default / config rate in commission_bps, so accrual keeps the pre-M0.1 algorithm for them: the
// active seller rule at delivery time. A legacy promo snapshot keeps its discount economics, but its
// rate and calculated commission (computed with the marketplace default rate) are not used.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/selleranalytics"
)

// makeLegacy turns a freshly checked-out fulfillment of the seller into the shape produced by the
// pre-M0.1 checkout: marketplace rate in commission_bps, no frozen marker, and (for promo orders)
// a snapshot priced with that same marketplace rate.
func (f *finFixture) makeLegacy(orderID, sellerID uuid.UUID, bps int) {
	f.t.Helper()
	f.exec(`UPDATE order_fulfillments SET commission_frozen = false, commission_bps = $3::int WHERE order_id = $1 AND seller_id = $2`, orderID, sellerID, bps)
	f.exec(`UPDATE order_item_promotions SET commission_rate_bps = $3::int,
			total_commission_charged_cents = (total_commission_base_cents * $3::bigint) / 10000
		WHERE order_id = $1 AND seller_id = $2`, orderID, sellerID, bps)
}

func (f *finFixture) frozenFlag(fulfillmentID uuid.UUID) bool {
	f.t.Helper()
	var v bool
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT commission_frozen FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&v))
	return v
}

func (f *finFixture) deliverErr(fulfillmentID uuid.UUID) error {
	return f.pg.RunInTx(f.ctx, func(tx pgx.Tx) error {
		return f.payoutsSvc.CreatePendingSalesForFulfillmentTx(f.ctx, tx, fulfillmentID)
	})
}

// A. Legacy order, individual rate 5 %: accrual uses 5 %, not the 9 % stored in the fulfillment.
func TestM02_A_LegacyOrderIndividualRate(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.setCommission(seller, 500)
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, "")
	item := onlyItem(t, order)
	f.makeLegacy(order.ID, seller, 900)
	require.False(t, f.frozenFlag(*item.OrderFulfillmentID))

	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 1_000_000, 50_000, 950_000)
	assert.Equal(t, "active_rule_at_delivery", f.saleGrossMetadata(item.ID)["commission_source"])
}

// B. New order: the rate frozen at checkout survives a later rule change.
func TestM02_B_NewOrderFrozenRate(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.setCommission(seller, 500)
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, "")
	item := onlyItem(t, order)
	require.True(t, f.frozenFlag(*item.OrderFulfillmentID), "checkout marks the fulfillment frozen")
	_, bps, _ := f.fulfillment(*item.OrderFulfillmentID)
	assert.Equal(t, 500, bps)

	f.setCommission(seller, 900)
	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 1_000_000, 50_000, 950_000)
	assert.Equal(t, "frozen_at_checkout", f.saleGrossMetadata(item.ID)["commission_source"])
}

// C. Legacy promo order: snapshot priced at 9 %, active rule 5 % — 5 % of the discounted base.
func TestM02_C_LegacyPromoOrder(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.setCommission(seller, 500)
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000}))
	item := onlyItem(t, order)
	f.makeLegacy(order.ID, seller, 900)
	snap := f.snapshot(item.ID)
	require.Equal(t, promoSnapshot{SellerDiscount: 300_000, CustomerPaid: 700_000, CommissionBase: 700_000, Commission: 63_000, CommissionBps: 900}, snap)

	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 700_000, 35_000, 665_000)
	meta := f.saleGrossMetadata(item.ID)
	assert.Equal(t, "active_rule_at_delivery", meta["commission_source"])
	assert.EqualValues(t, 500, meta["commission_bps"])

	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(700_000), ref.AmountCents)
	assert.Equal(t, int64(-665_000), f.ledger(item.ID)["adjustment:return_deduction"])
}

// D. Legacy order of a seller without an individual rule: the pre-M0.1 default (9 %), not the
// config rate that old checkout stored in the fulfillment.
func TestM02_D_LegacyOrderWithoutRule(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, "")
	item := onlyItem(t, order)
	f.makeLegacy(order.ID, seller, 1200)

	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 1_000_000, 90_000, 910_000)
}

// E. Redelivery through both accrual paths never adds or rewrites ledger entries.
func TestM02_E_ExistingLedgerNotRecalculated(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.setCommission(seller, 500)
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, "")
	item := onlyItem(t, order)
	f.makeLegacy(order.ID, seller, 900)
	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 1_000_000, 50_000, 950_000)

	// The rule changes and the same order is accrued again through every path.
	f.setCommission(seller, 1500)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	require.NoError(t, f.payoutsSvc.CreatePendingSalesForOrder(f.ctx, order.ID))
	assertLine(t, f.ledger(item.ID), 1_000_000, 50_000, 950_000)

	// Entries written by the pre-M0.1 code are left untouched as well, whatever the flag says.
	seller2, buyer2 := f.seller(), f.buyer()
	f.setCommission(seller2, 500)
	f.addToCart(buyer2, f.product(seller2, 1_000_000), 1)
	order2 := f.checkout(buyer2, "")
	item2 := onlyItem(t, order2)
	f.exec(`UPDATE order_fulfillments SET commission_frozen = false, commission_bps = 900 WHERE order_id = $1`, order2.ID)
	for typ, amount := range map[string]int64{"sale_gross": 1_000_000, "zamk_commission": -90_000, "seller_earning": 910_000} {
		f.exec(`INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'RUB', now())`, uuid.New(), seller2, order2.ID, item2.ID, typ, amount)
	}
	f.deliver(*item2.OrderFulfillmentID, time.Now())
	require.NoError(t, f.payoutsSvc.CreatePendingSalesForOrder(f.ctx, order2.ID))
	assertLine(t, f.ledger(item2.ID), 1_000_000, 90_000, 910_000)
}

// F. Refund of a co-funded sale: customer refund, seller balance and the Seller Analytics ledger
// summary. The summary semantics are documented here as they are and are NOT changed by M0.2:
// gross = commission base (after the seller discount, ZAMK subsidy not included), commission is not
// reversed by a return, and return_deductions carries the reversal of the net seller earning.
func TestM02_F_CofundedRefundAndAnalytics(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	code, _ := f.fundedPromo(seller, marketing.FundingModeCofunded, 3000, 1000, 500_000)
	order := f.checkout(buyer, code)
	item := onlyItem(t, order)
	require.True(t, f.frozenFlag(*item.OrderFulfillmentID))
	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 700_000, 63_000, 637_000)

	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	repo := selleranalytics.NewRepository(f.db)
	before, err := repo.GetLedgerSummary(f.ctx, seller, from, to)
	require.NoError(t, err)
	assert.Equal(t, selleranalytics.LedgerSummary{GrossSalesCents: 700_000, CommissionCents: 63_000, SellerEarningCents: 637_000}, before)

	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(600_000), ref.AmountCents, "customer gets back what was paid")

	after, err := repo.GetLedgerSummary(f.ctx, seller, from, to)
	require.NoError(t, err)
	assert.Equal(t, int64(700_000), after.GrossSalesCents, "gross is not reduced by the return")
	assert.Equal(t, int64(63_000), after.CommissionCents, "commission is not reversed separately")
	assert.Equal(t, int64(637_000), after.SellerEarningCents)
	assert.Equal(t, int64(-637_000), after.ReturnDeductionsCents, "the net earning is reversed once")
	assert.Equal(t, after.SellerEarningCents+after.ReturnDeductionsCents, int64(0), "nothing is left for the seller")
	assert.Equal(t, int64(0), after.OtherAdjustmentsCents)
}

// G. One order, two sellers: one frozen at checkout, one legacy.
func TestM02_G_MixedFrozenStatesInOneOrder(t *testing.T) {
	f := newFinFixture(t)
	sellerA, sellerB, buyer := f.seller(), f.seller(), f.buyer()
	f.setCommission(sellerA, 500)
	f.setCommission(sellerB, 700)
	f.addToCart(buyer, f.product(sellerA, 1_000_000), 1)
	f.addToCart(buyer, f.product(sellerB, 1_000_000), 1)
	order := f.checkout(buyer, "")
	require.Len(t, order.Items, 2)
	f.makeLegacy(order.ID, sellerB, 900)

	f.setCommission(sellerA, 1500)
	f.setCommission(sellerB, 800)
	f.pay(buyer, order.ID)
	for _, it := range order.Items {
		f.deliver(*it.OrderFulfillmentID, time.Now())
		if it.SellerID == sellerA {
			assertLine(t, f.ledger(it.ID), 1_000_000, 50_000, 950_000) // frozen 5 %
		} else {
			assertLine(t, f.ledger(it.ID), 1_000_000, 80_000, 920_000) // rule at delivery 8 %
		}
	}
}

// H. An inconsistent promo snapshot fails closed with a typed error and writes nothing.
func TestM02_H_InconsistentSnapshotFailsClosed(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	code := f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000})

	// Frozen fulfillment whose snapshot rate differs from the frozen rate.
	order := f.checkout(buyer, code)
	item := onlyItem(t, order)
	f.pay(buyer, order.ID)
	f.exec(`UPDATE order_item_promotions SET commission_rate_bps = 700, total_commission_charged_cents = total_commission_base_cents * 700 / 10000 WHERE order_id = $1`, order.ID)
	require.ErrorIs(t, f.deliverErr(*item.OrderFulfillmentID), payouts.ErrCommissionSnapshotMismatch)
	assert.Empty(t, f.ledger(item.ID))

	// Legacy fulfillment whose snapshot no longer matches the order item price.
	buyer2 := f.buyer()
	f.addToCart(buyer2, f.product(seller, 1_000_000), 1)
	order2 := f.checkout(buyer2, code)
	item2 := onlyItem(t, order2)
	f.makeLegacy(order2.ID, seller, 900)
	f.pay(buyer2, order2.ID)
	f.exec(`UPDATE order_items SET price_cents = price_cents + 1 WHERE id = $1`, item2.ID)
	require.ErrorIs(t, f.deliverErr(*item2.OrderFulfillmentID), payouts.ErrPromotionSnapshotInvalid)
	assert.Empty(t, f.ledger(item2.ID))
}

// I. A fulfillment inserted the way the previous application version does during a rolling deploy
// (column list without commission_frozen) is legacy: the column default is FALSE.
func TestM02_I_OldCheckoutInsertDuringRollingDeploy(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.setCommission(seller, 500)
	p := f.product(seller, 50_000)

	orderID, fulID, itemID := uuid.New(), uuid.New(), uuid.New()
	f.exec(`INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'paid', 100000, 'n', 'p', 'e', 'a')`, orderID, buyer)
	f.exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'paid', 100000, 900, 91000)`, fulID, orderID, seller)
	f.exec(`INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id)
		VALUES ($1, $2, $3, $4, $5, 'old', 'old', 50000, 2, 100000, $6)`, itemID, orderID, p.ID, p.VariantID, seller, fulID)
	require.False(t, f.frozenFlag(fulID))

	f.deliver(fulID, time.Now())
	assertLine(t, f.ledger(itemID), 100_000, 5_000, 95_000)
}
