package payouts_test

// MARKETING.M0.1 — end-to-end promotion money integrity on zamk_test.
//
// Every scenario runs the real checkout (orders.Service.CreateOrder), payment confirmation
// (payments.Service mock integration, no external provider), seller ledger accrual
// (payouts.Service) and refunds (returns.Service) against PostgreSQL.
//
// Per-line invariants checked throughout (kopecks):
//   commission_base = base_gross - seller_discount          (ledger sale_gross)
//   customer_paid   = commission_base - zamk_subsidy
//   seller_earning  = commission_base - commission
//   cumulative customer refund for a line <= customer_paid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

const deliveryPriceCents = int64(200)

type mockModeProvider struct{}

func (mockModeProvider) CreatePayment(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
	return payments.ProviderCreatePaymentResult{ProviderPaymentID: "m01-" + uuid.NewString(), Status: "NEW"}, nil
}
func (mockModeProvider) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) error {
	return errors.New("webhooks are not used in this suite")
}
func (mockModeProvider) ParseWebhook(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
	return payments.ProviderWebhookEvent{}, errors.New("webhooks are not used in this suite")
}
func (mockModeProvider) GetMode(method string) string { return "mock" }
func (mockModeProvider) CheckOrder(ctx context.Context, orderID string) (payments.CheckOrderResult, error) {
	return payments.CheckOrderResult{OrderID: orderID}, nil
}

type finFixture struct {
	t            *testing.T
	ctx          context.Context
	db           *pgxpool.Pool
	pg           *postgres.Client
	marketingSvc *marketing.Service
	marketingRep *marketing.Repository
	ordersSvc    *orders.Service
	paymentsSvc  *payments.Service
	payoutsSvc   *payouts.Service
	returnsSvc   *returns.Service
	dmID         uuid.UUID
	adminID      uuid.UUID
	suffix       string
}

func newFinFixture(t *testing.T) *finFixture {
	t.Helper()
	ctx := context.Background()
	db, err := pgxpool.New(ctx, testutil.GetTestDatabaseURL())
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, db)
	require.NoError(t, testutil.EnsureMarketingMigrations(ctx, db))
	t.Cleanup(db.Close)

	pg := &postgres.Client{Pool: db}
	cfg := &config.Config{
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 900},
		App:    config.AppConfig{PaymentStuckPendingMinutes: 30},
	}
	ordersRepo := orders.NewRepository(db)
	invSvc := inventory.NewService(inventory.NewRepository(db), nil, pg)
	marketingRepo := marketing.NewRepository(db)
	marketingSvc := marketing.NewService(marketingRepo, db)
	ordersSvc := orders.NewService(ordersRepo, cart.NewRepository(db), invSvc, pg, cfg).WithMarketing(marketingSvc)
	paymentsSvc := payments.NewService(payments.NewRepository(db), ordersRepo, invSvc, mockModeProvider{}, pg,
		notifications.NewService(nil, nil, nil), nil, cfg).WithMarketing(marketingSvc)
	payoutsSvc := payouts.NewService(payouts.NewRepository(db), pg, nil, ordersRepo, cfg, nil)
	returnsSvc := returns.NewService(returns.NewRepository(db), ordersRepo, nil, pg, payoutsSvc, paymentsSvc, 30, nil, nil, nil, nil)

	f := &finFixture{
		t: t, ctx: ctx, db: db, pg: pg,
		marketingSvc: marketingSvc, marketingRep: marketingRepo,
		ordersSvc: ordersSvc, paymentsSvc: paymentsSvc, payoutsSvc: payoutsSvc, returnsSvc: returnsSvc,
		dmID: uuid.New(), adminID: uuid.New(), suffix: uuid.NewString()[:8],
	}
	f.exec(`INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, 'M01 Delivery', $3, true, now(), now())`, f.dmID, "m01-"+f.suffix, deliveryPriceCents)
	f.exec(`INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'M01 Admin', $2, 'hash', 'admin', 'active', now(), now())`, f.adminID, "m01-admin-"+f.suffix+"@test.local")
	return f
}

func (f *finFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err)
}

func (f *finFixture) seller() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	slug := "m01-" + id.String()[:8]
	f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', now(), now())`, id, "M01 "+slug, slug, slug+"@test.local")
	return id
}

func (f *finFixture) buyer() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'M01 Buyer', $2, 'hash', 'customer', 'active', now(), now())`, id, "m01-buyer-"+id.String()[:8]+"@test.local")
	return id
}

type product struct{ ID, VariantID, SellerID uuid.UUID }

func (f *finFixture) product(sellerID uuid.UUID, priceCents int64) product {
	f.t.Helper()
	p := product{ID: uuid.New(), VariantID: uuid.New(), SellerID: sellerID}
	catID := uuid.New()
	f.exec(`INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'M01 Cat', $2, now(), now())`, catID, "m01-cat-"+catID.String()[:8])
	f.exec(`INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'M01 Product', $4, $5, 'published', now(), now())`, p.ID, sellerID, catID, "m01-p-"+p.ID.String()[:8], priceCents)
	f.exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $3, $4, true, now(), now())`, p.VariantID, p.ID, "M01-"+p.VariantID.String()[:8], priceCents)
	f.exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1000, 0, now(), now())`, uuid.New(), p.ID, p.VariantID, sellerID)
	return p
}

func (f *finFixture) addToCart(buyerID uuid.UUID, p product, qty int) {
	f.t.Helper()
	repo := cart.NewRepository(f.db)
	c, err := repo.GetCartByUserID(f.ctx, buyerID)
	if errors.Is(err, cart.ErrCartNotFound) {
		c, err = repo.CreateCart(f.ctx, buyerID)
	}
	require.NoError(f.t, err)
	require.NoError(f.t, repo.AddItem(f.ctx, &cart.CartItem{ID: uuid.New(), CartID: c.ID, ProductID: p.ID, ProductVariantID: p.VariantID, Quantity: qty}))
}

func (f *finFixture) sellerPromo(sellerID uuid.UUID, req marketing.CreateSellerPromoRequest) string {
	f.t.Helper()
	req.Code = "M01S" + uuid.NewString()[:8]
	if req.PerCustomerUsageLimit == 0 {
		req.PerCustomerUsageLimit = 50
	}
	promo, err := f.marketingSvc.CreateSellerPromotion(f.ctx, sellerID, req)
	require.NoError(f.t, err)
	return promo.Code
}

// fundedPromo creates an approved ZAMK-funded or co-funded percent promotion. These campaigns have
// no HTTP route yet, so the campaign row is created through the marketing repository.
func (f *finFixture) fundedPromo(sellerID uuid.UUID, mode marketing.FundingMode, sellerBps, zamkBps int, budgetCents int64) (string, uuid.UUID) {
	f.t.Helper()
	camp := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		Purpose:                     marketing.CampaignPurposePromotion,
		SellerID:                    &sellerID,
		Title:                       "M01 funded",
		FundingMode:                 mode,
		Status:                      marketing.CampaignStatusActive,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           sellerBps,
		RequestedZamkShareBps:       zamkBps,
		RequestedZamkBudgetCapCents: budgetCents,
		ApprovedZamkShareBps:        zamkBps,
		ApprovedZamkBudgetCapCents:  budgetCents,
	}
	require.NoError(f.t, f.marketingRep.CreateCampaign(f.ctx, camp))
	promo, err := f.marketingSvc.CreatePromoCode(f.ctx, sellerID, marketing.CreatePromoCodeRequest{
		CampaignID:            camp.ID,
		Code:                  "M01F" + uuid.NewString()[:8],
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      sellerBps + zamkBps,
		PerCustomerUsageLimit: 50,
	})
	require.NoError(f.t, err)
	return promo.Code, camp.ID
}

func (f *finFixture) checkout(buyerID uuid.UUID, code string) *orders.Order {
	f.t.Helper()
	req := orders.CreateOrderRequest{
		CustomerName: "M01 Buyer", CustomerPhone: "+79990000000", CustomerEmail: "m01@test.local",
		DeliveryAddress: "Moscow", DeliveryMethodID: f.dmID,
	}
	if code != "" {
		req.PromoCode = &code
	}
	order, err := f.ordersSvc.CreateOrder(f.ctx, buyerID, req, nil)
	require.NoError(f.t, err)
	return order
}

func (f *finFixture) pay(buyerID, orderID uuid.UUID) uuid.UUID {
	f.t.Helper()
	res, err := f.paymentsSvc.CreatePayment(f.ctx, buyerID, orderID, "card")
	require.NoError(f.t, err)
	require.NoError(f.t, f.paymentsSvc.ProcessMockPaymentAction(f.ctx, res.PaymentID, "confirm"))
	return res.PaymentID
}

func (f *finFixture) deliver(fulfillmentID uuid.UUID, deliveredAt time.Time) {
	f.t.Helper()
	require.NoError(f.t, f.pg.RunInTx(f.ctx, func(tx pgx.Tx) error {
		if err := f.payoutsSvc.CreatePendingSalesForFulfillmentTx(f.ctx, tx, fulfillmentID); err != nil {
			return err
		}
		return f.payoutsSvc.MarkFulfillmentDeliveredTx(f.ctx, tx, fulfillmentID, deliveredAt)
	}))
}

// ledger sums seller_ledger_entries of an order item by type, with adjustments split by reason.
func (f *finFixture) ledger(orderItemID uuid.UUID) map[string]int64 {
	f.t.Helper()
	rows, err := f.db.Query(f.ctx, `
		SELECT CASE WHEN type = 'adjustment' THEN 'adjustment:' || COALESCE(metadata->>'reason', '') ELSE type END,
		       SUM(amount_cents), COUNT(*)
		FROM seller_ledger_entries WHERE order_item_id = $1 GROUP BY 1`, orderItemID)
	require.NoError(f.t, err)
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var sum, n int64
		require.NoError(f.t, rows.Scan(&k, &sum, &n))
		out[k] = sum
		out["count:"+k] = n
	}
	require.NoError(f.t, rows.Err())
	return out
}

func (f *finFixture) saleGrossMetadata(orderItemID uuid.UUID) map[string]any {
	f.t.Helper()
	var raw []byte
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT metadata FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'sale_gross'`, orderItemID).Scan(&raw))
	meta := map[string]any{}
	require.NoError(f.t, json.Unmarshal(raw, &meta))
	return meta
}

type promoSnapshot struct {
	SellerDiscount, ZamkSubsidy, CustomerPaid, CommissionBase, Commission int64
	CommissionBps                                                         int
}

func (f *finFixture) snapshot(orderItemID uuid.UUID) promoSnapshot {
	f.t.Helper()
	var s promoSnapshot
	require.NoError(f.t, f.db.QueryRow(f.ctx, `
		SELECT total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents,
		       total_commission_base_cents, total_commission_charged_cents, commission_rate_bps
		FROM order_item_promotions WHERE order_item_id = $1`, orderItemID).
		Scan(&s.SellerDiscount, &s.ZamkSubsidy, &s.CustomerPaid, &s.CommissionBase, &s.Commission, &s.CommissionBps))
	return s
}

func (f *finFixture) fulfillment(id uuid.UUID) (subtotal int64, bps int, sellerAmount int64) {
	f.t.Helper()
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT subtotal_cents, commission_bps, seller_amount_cents FROM order_fulfillments WHERE id = $1`, id).Scan(&subtotal, &bps, &sellerAmount))
	return
}

// createReceivedReturn creates a return already accepted at the warehouse (legacy, non-serialized items).
func (f *finFixture) createReceivedReturn(order *orders.Order, buyerID uuid.UUID, qtyByItem map[uuid.UUID]int) uuid.UUID {
	f.t.Helper()
	returnID := uuid.New()
	var fulfillmentID uuid.UUID
	for _, it := range order.Items {
		if _, ok := qtyByItem[it.ID]; ok {
			fulfillmentID = *it.OrderFulfillmentID
		}
	}
	f.exec(`INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())`, returnID, order.ID, fulfillmentID, buyerID)
	for itemID, qty := range qtyByItem {
		f.exec(`INSERT INTO return_items (id, return_id, order_item_id, quantity, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at)
			VALUES ($1, $2, $3, $4, false, $4, 0, 0, now())`, uuid.New(), returnID, itemID, qty)
	}
	return returnID
}

func (f *finFixture) refund(returnID uuid.UUID) *returns.Refund {
	f.t.Helper()
	ref, err := f.returnsSvc.CreateRefund(f.ctx, f.adminID, returnID, returns.CreateRefundRequest{})
	require.NoError(f.t, err)
	require.NoError(f.t, f.returnsSvc.ProcessRefundSuccess(f.ctx, ref.ID, time.Now()))
	return ref
}

func (f *finFixture) setCommission(sellerID uuid.UUID, bps int) {
	f.t.Helper()
	require.NoError(f.t, f.payoutsSvc.SetCommissionRate(f.ctx, sellerID, payouts.AdminSellerCommissionRequest{RateBPS: bps, Reason: "m01"}, f.adminID))
	time.Sleep(5 * time.Millisecond) // created_at ordering of commission rules
}

// assertLine checks the ledger of one item against the expected frozen economics and the invariants.
func assertLine(t *testing.T, l map[string]int64, gross, commission, earning int64) {
	t.Helper()
	assert.Equal(t, gross, l["sale_gross"], "sale_gross (commission base)")
	assert.Equal(t, -commission, l["zamk_commission"], "zamk_commission")
	assert.Equal(t, earning, l["seller_earning"], "seller_earning")
	assert.Equal(t, l["sale_gross"], l["seller_earning"]-l["zamk_commission"], "invariant: gross = earning + commission")
	for _, k := range []string{"sale_gross", "zamk_commission", "seller_earning"} {
		assert.Equal(t, int64(1), l["count:"+k], "exactly one %s entry", k)
	}
}

func onlyItem(t *testing.T, o *orders.Order) orders.OrderItem {
	t.Helper()
	require.Len(t, o.Items, 1)
	return o.Items[0]
}

// A. Ordinary purchase without a discount — unchanged behaviour.
func TestM01_A_NoPromoRegression(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, "")
	assert.Equal(t, 1_000_000+deliveryPriceCents, order.TotalPriceCents)
	f.pay(buyer, order.ID)
	item := onlyItem(t, order)
	f.deliver(*item.OrderFulfillmentID, time.Now())

	assertLine(t, f.ledger(item.ID), 1_000_000, 90_000, 910_000)
	meta := f.saleGrossMetadata(item.ID)
	assert.Equal(t, "standard", meta["pricing_basis"])

	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(1_000_000), ref.AmountCents)
	assert.Equal(t, int64(-910_000), f.ledger(item.ID)["adjustment:return_deduction"])
}

// B. Seller-funded 30% — the discount reduces the accrual base and commission.
func TestM01_B_SellerFunded30(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000}))
	assert.Equal(t, 700_000+deliveryPriceCents, order.TotalPriceCents)
	item := onlyItem(t, order)

	subtotal, bps, sellerAmount := f.fulfillment(*item.OrderFulfillmentID)
	assert.Equal(t, int64(700_000), subtotal)
	assert.Equal(t, 900, bps)
	assert.Equal(t, int64(637_000), sellerAmount)

	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	l := f.ledger(item.ID)
	assertLine(t, l, 700_000, 63_000, 637_000)
	assert.Equal(t, sellerAmount, l["seller_earning"], "ledger agrees with checkout fulfillment snapshot")

	meta := f.saleGrossMetadata(item.ID)
	assert.Equal(t, "promotion", meta["pricing_basis"])
	assert.EqualValues(t, 1_000_000, meta["base_gross_cents"])
	assert.EqualValues(t, 300_000, meta["seller_discount_cents"])
	assert.EqualValues(t, 0, meta["zamk_subsidy_cents"])
	assert.EqualValues(t, 700_000, meta["customer_paid_cents"])
}

// C. ZAMK-only funding — the seller's accrual is identical to a full-price sale.
func TestM01_C_ZamkOnlyFunding(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	code, campID := f.fundedPromo(seller, marketing.FundingModeZamk, 0, 1000, 500_000)
	order := f.checkout(buyer, code)
	assert.Equal(t, 900_000+deliveryPriceCents, order.TotalPriceCents)
	item := onlyItem(t, order)

	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 1_000_000, 90_000, 910_000)

	snap := f.snapshot(item.ID)
	assert.Equal(t, int64(100_000), snap.ZamkSubsidy)
	assert.Equal(t, int64(900_000), snap.CustomerPaid)
	camp, err := f.marketingRep.GetCampaignByID(f.ctx, campID)
	require.NoError(t, err)
	assert.Equal(t, int64(100_000), camp.ZamkSpentCents, "ZAMK cost is recorded once as campaign spend")
	assert.Equal(t, int64(0), camp.ZamkReservedCents)

	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(900_000), ref.AmountCents, "customer gets back only what was paid")
}

// D. Co-funded 30% seller + 10% ZAMK — the canonical PO example.
func TestM01_D_CofundedCanonicalExample(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	code, _ := f.fundedPromo(seller, marketing.FundingModeCofunded, 3000, 1000, 500_000)
	order := f.checkout(buyer, code)
	assert.Equal(t, 600_000+deliveryPriceCents, order.TotalPriceCents)
	item := onlyItem(t, order)

	snap := f.snapshot(item.ID)
	assert.Equal(t, promoSnapshot{SellerDiscount: 300_000, ZamkSubsidy: 100_000, CustomerPaid: 600_000, CommissionBase: 700_000, Commission: 63_000, CommissionBps: 900}, snap)

	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	l := f.ledger(item.ID)
	assertLine(t, l, 700_000, 63_000, 637_000)
	assert.Equal(t, snap.CustomerPaid+snap.ZamkSubsidy, l["sale_gross"], "customer paid + ZAMK subsidy = accrual base")
	meta := f.saleGrossMetadata(item.ID)
	assert.EqualValues(t, 100_000, meta["zamk_subsidy_cents"])
	assert.EqualValues(t, 300_000, meta["seller_discount_cents"])

	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(600_000), ref.AmountCents)
	assert.Equal(t, int64(-637_000), f.ledger(item.ID)["adjustment:return_deduction"])
}

// E. Commission rule changes after checkout — the rate frozen at checkout is used.
func TestM01_E_CommissionFrozenAtCheckout(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.setCommission(seller, 1200)
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000}))
	item := onlyItem(t, order)
	_, bps, _ := f.fulfillment(*item.OrderFulfillmentID)
	assert.Equal(t, 1200, bps)
	assert.Equal(t, 1200, f.snapshot(item.ID).CommissionBps)

	f.setCommission(seller, 500)
	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 700_000, 84_000, 616_000)

	// A non-promo order of the same seller is also priced at the rate in force at its checkout.
	f.addToCart(buyer, f.product(seller, 100_000), 1)
	order2 := f.checkout(buyer, "")
	item2 := onlyItem(t, order2)
	f.setCommission(seller, 2000)
	f.pay(buyer, order2.ID)
	f.deliver(*item2.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item2.ID), 100_000, 5_000, 95_000)
}

// F. Promo disabled and product repriced after checkout — history is unchanged.
func TestM01_F_PromoChangesAfterCheckoutDoNotAffectHistory(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	p := f.product(seller, 1_000_000)
	f.addToCart(buyer, p, 1)
	code := f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000})
	order := f.checkout(buyer, code)
	item := onlyItem(t, order)
	f.pay(buyer, order.ID)

	var promoID uuid.UUID
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT id FROM promo_codes WHERE code = $1`, code).Scan(&promoID))
	inactive := false
	_, err := f.marketingSvc.UpdateSellerPromotion(f.ctx, seller, promoID, marketing.UpdateSellerPromoRequest{IsActive: &inactive})
	require.NoError(t, err)
	f.exec(`UPDATE products SET price_cents = 1 WHERE id = $1`, p.ID)
	f.exec(`UPDATE product_variants SET price_cents = 1 WHERE id = $1`, p.VariantID)

	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 700_000, 63_000, 637_000)
	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(700_000), ref.AmountCents)
}

// G + R. Two sellers in one order: the promo applies only to its seller; fulfillments accrue independently.
// One order carries at most one promo code (uq_promo_usage_order), so "different promos" cannot share an order.
func TestM01_G_R_TwoSellersIndependentFulfillments(t *testing.T) {
	f := newFinFixture(t)
	s1, s2, buyer := f.seller(), f.seller(), f.buyer()
	f.addToCart(buyer, f.product(s1, 1_000_000), 1)
	f.addToCart(buyer, f.product(s2, 500_000), 1)
	order := f.checkout(buyer, f.sellerPromo(s1, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000}))
	require.Len(t, order.Items, 2)
	assert.Equal(t, 700_000+500_000+deliveryPriceCents, order.TotalPriceCents)
	var i1, i2 orders.OrderItem
	for _, it := range order.Items {
		if it.SellerID == s1 {
			i1 = it
		} else {
			i2 = it
		}
	}
	require.NotEqual(t, *i1.OrderFulfillmentID, *i2.OrderFulfillmentID)
	f.pay(buyer, order.ID)

	d1 := time.Now().Add(-48 * time.Hour)
	f.deliver(*i1.OrderFulfillmentID, d1)
	assertLine(t, f.ledger(i1.ID), 700_000, 63_000, 637_000)
	assert.Empty(t, f.ledger(i2.ID), "undelivered fulfillment of another seller has no accrual")

	d2 := time.Now()
	f.deliver(*i2.OrderFulfillmentID, d2)
	assertLine(t, f.ledger(i2.ID), 500_000, 45_000, 455_000)

	var a1, a2 time.Time
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT available_at FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, i1.ID).Scan(&a1))
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT available_at FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, i2.ID).Scan(&a2))
	assert.WithinDuration(t, d1.AddDate(0, 0, 14), a1, time.Second)
	assert.WithinDuration(t, d2.AddDate(0, 0, 14), a2, time.Second)
}

// H + I + J. Fixed discount spread over several lines with cent remainders, then sequential partial returns.
func TestM01_H_I_J_RoundingAndSequentialPartialReturns(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	pa, pb := f.product(seller, 3_333), f.product(seller, 7_777)
	f.addToCart(buyer, pa, 3)
	f.addToCart(buyer, pb, 1)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypeFixed, DiscountValueFixedCents: 1_001}))
	require.Len(t, order.Items, 2)
	var ia, ib orders.OrderItem
	for _, it := range order.Items {
		if it.ProductID == pa.ID {
			ia = it
		} else {
			ib = it
		}
	}
	sa, sb := f.snapshot(ia.ID), f.snapshot(ib.ID)
	assert.Equal(t, int64(1_001), sa.SellerDiscount+sb.SellerDiscount, "fixed discount allocated exactly")
	assert.Equal(t, 9_999+7_777-1_001+deliveryPriceCents, order.TotalPriceCents)

	f.pay(buyer, order.ID)
	f.deliver(*ia.OrderFulfillmentID, time.Now())
	la, lb := f.ledger(ia.ID), f.ledger(ib.ID)
	assertLine(t, la, sa.CommissionBase, sa.Commission, sa.CommissionBase-sa.Commission)
	assertLine(t, lb, sb.CommissionBase, sb.Commission, sb.CommissionBase-sb.Commission)
	assert.Equal(t, int64(9_999+7_777-1_001), la["sale_gross"]+lb["sale_gross"])

	paidA := sa.CustomerPaid
	require.NotZero(t, paidA%3, "line total must not divide evenly to exercise remainders")

	// I: one unit of line A.
	r1 := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{ia.ID: 1}))
	assert.Equal(t, paidA/3, r1.AmountCents)
	// J: the remaining two units — the cumulative total equals exactly what was paid.
	r2 := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{ia.ID: 2}))
	assert.Equal(t, paidA, r1.AmountCents+r2.AmountCents)
	// A further return of an already fully refunded line yields nothing.
	_, err := f.returnsSvc.CreateRefund(f.ctx, f.adminID, f.createReceivedReturn(order, buyer, map[uuid.UUID]int{ia.ID: 1}), returns.CreateRefundRequest{})
	require.ErrorIs(t, err, returns.ErrRefundNoEligibleItems)

	la = f.ledger(ia.ID)
	assert.Equal(t, -la["seller_earning"], la["adjustment:return_deduction"], "seller earning fully reversed, never more")
	assert.Equal(t, sb.CommissionBase-sb.Commission, f.ledger(ib.ID)["seller_earning"], "other line untouched")
}

// K. Full return of a discounted order refunds the paid amount and succeeds.
// L. Return after the seller payout creates a post-payout adjustment.
func TestM01_K_L_FullReturnAfterPayout(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000}))
	item := onlyItem(t, order)
	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now().AddDate(0, 0, -20))

	_, err := f.payoutsSvc.MakeSellerFundsAvailable(f.ctx, time.Now(), 1000)
	require.NoError(t, err)
	batch, err := f.payoutsSvc.CreatePayoutBatchForSeller(f.ctx, seller)
	require.NoError(t, err)
	assert.Equal(t, int64(637_000), batch.AmountCents, "payout equals discounted seller earning")
	require.NoError(t, f.payoutsSvc.ProcessPayoutBatch(f.ctx, batch.ID))

	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(700_000), ref.AmountCents, "full return refunds the discounted price, not the base price")
	l := f.ledger(item.ID)
	assert.Equal(t, int64(-637_000), l["adjustment:return_post_payout"])
	assert.Zero(t, l["adjustment:return_deduction"])
}

// M. Repeated accrual (both delivery paths) never duplicates ledger entries.
// S. Repeated payment confirmation consumes the promo once and spends ZAMK budget once.
func TestM01_M_S_IdempotentDeliveryAndPayment(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	code, campID := f.fundedPromo(seller, marketing.FundingModeCofunded, 3000, 1000, 500_000)
	order := f.checkout(buyer, code)
	item := onlyItem(t, order)
	paymentID := f.pay(buyer, order.ID)

	require.ErrorIs(t, f.paymentsSvc.ProcessMockPaymentAction(f.ctx, paymentID, "confirm"), payments.ErrPaymentAlreadyProcessed)
	require.NoError(t, f.pg.RunInTx(f.ctx, func(tx pgx.Tx) error { return f.marketingSvc.ConsumePromoForOrderTx(f.ctx, tx, order.ID) }))
	var consumed int
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT COUNT(*) FROM promo_code_usages WHERE order_id = $1 AND status = 'consumed'`, order.ID).Scan(&consumed))
	assert.Equal(t, 1, consumed)
	camp, err := f.marketingRep.GetCampaignByID(f.ctx, campID)
	require.NoError(t, err)
	assert.Equal(t, int64(100_000), camp.ZamkSpentCents)

	f.deliver(*item.OrderFulfillmentID, time.Now())
	f.deliver(*item.OrderFulfillmentID, time.Now())
	require.NoError(t, f.payoutsSvc.CreatePendingSalesForOrder(f.ctx, order.ID))

	// Concurrent duplicate accrual attempts serialize on the order item lock.
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = f.pg.RunInTx(f.ctx, func(tx pgx.Tx) error {
				return f.payoutsSvc.CreatePendingSalesForFulfillmentTx(f.ctx, tx, *item.OrderFulfillmentID)
			})
		}(i)
	}
	wg.Wait()
	for _, e := range errs {
		require.NoError(t, e)
	}
	assertLine(t, f.ledger(item.ID), 700_000, 63_000, 637_000)
}

// N. Repeated and concurrent refund processing never refunds twice.
func TestM01_N_RefundIdempotencyAndConcurrency(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 100_001), 3)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypeFixed, DiscountValueFixedCents: 1}))
	item := onlyItem(t, order)
	paid := f.snapshot(item.ID).CustomerPaid
	require.Equal(t, int64(300_002), paid)
	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())

	// Same return: repeated creation returns the same pending refund; repeated success is a no-op.
	ret := f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1})
	ref1, err := f.returnsSvc.CreateRefund(f.ctx, f.adminID, ret, returns.CreateRefundRequest{})
	require.NoError(t, err)
	ref1b, err := f.returnsSvc.CreateRefund(f.ctx, f.adminID, ret, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.Equal(t, ref1.ID, ref1b.ID)
	require.NoError(t, f.returnsSvc.ProcessRefundSuccess(f.ctx, ref1.ID, time.Now()))
	require.NoError(t, f.returnsSvc.ProcessRefundSuccess(f.ctx, ref1.ID, time.Now()))
	assert.Equal(t, int64(1), f.ledger(item.ID)["count:adjustment:return_deduction"])

	// Two different returns refunded concurrently for the remaining two units.
	retA := f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1})
	retB := f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1})
	var wg sync.WaitGroup
	refs := make([]*returns.Refund, 2)
	errs := make([]error, 2)
	for i, r := range []uuid.UUID{retA, retB} {
		wg.Add(1)
		go func(i int, r uuid.UUID) {
			defer wg.Done()
			refs[i], errs[i] = f.returnsSvc.CreateRefund(f.ctx, f.adminID, r, returns.CreateRefundRequest{})
		}(i, r)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, paid, ref1.AmountCents+refs[0].AmountCents+refs[1].AmountCents, "three units refund exactly the paid line total")

	var totalRefunded int64
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM refunds WHERE order_id = $1 AND status IN ('pending', 'succeeded')`, order.ID).Scan(&totalRefunded))
	assert.LessOrEqual(t, totalRefunded, order.TotalPriceCents)
	assert.Equal(t, paid, totalRefunded)
}

// O. A failure inside the delivery transaction rolls back accrual; inconsistent snapshots fail closed.
func TestM01_O_RollbackAndFailClosed(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000}))
	item := onlyItem(t, order)
	f.pay(buyer, order.ID)

	boom := errors.New("simulated failure after accrual")
	err := f.pg.RunInTx(f.ctx, func(tx pgx.Tx) error {
		if err := f.payoutsSvc.CreatePendingSalesForFulfillmentTx(f.ctx, tx, *item.OrderFulfillmentID); err != nil {
			return err
		}
		return boom
	})
	require.ErrorIs(t, err, boom)
	assert.Empty(t, f.ledger(item.ID), "rolled back accrual leaves no ledger entries")

	f.exec(`UPDATE order_fulfillments SET commission_bps = 1500 WHERE id = $1`, *item.OrderFulfillmentID)
	err = f.pg.RunInTx(f.ctx, func(tx pgx.Tx) error {
		return f.payoutsSvc.CreatePendingSalesForFulfillmentTx(f.ctx, tx, *item.OrderFulfillmentID)
	})
	require.ErrorIs(t, err, payouts.ErrCommissionSnapshotMismatch)
	assert.Empty(t, f.ledger(item.ID))
}

// P. 100% seller discount — no accrual at the full price.
func TestM01_P_SellerDiscount100(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 10000}))
	assert.Equal(t, deliveryPriceCents, order.TotalPriceCents)
	item := onlyItem(t, order)
	f.pay(buyer, order.ID)
	f.deliver(*item.OrderFulfillmentID, time.Now())
	assertLine(t, f.ledger(item.ID), 0, 0, 0)

	_, err := f.returnsSvc.CreateRefund(f.ctx, f.adminID, f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}), returns.CreateRefundRequest{})
	require.ErrorIs(t, err, returns.ErrRefundNoEligibleItems, "nothing was paid for the item, nothing is refunded")
}

// Q. Orders created by the previous version: no promo snapshot, rows inserted without explicit
// commission, and items already accrued by the old code.
func TestM01_Q_LegacyOrders(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	p := f.product(seller, 50_000)

	insertLegacyOrder := func() (uuid.UUID, uuid.UUID, uuid.UUID) {
		orderID, fulID, itemID := uuid.New(), uuid.New(), uuid.New()
		f.exec(`INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
			VALUES ($1, $2, 'paid', 100000, 'n', 'p', 'e', 'a')`, orderID, buyer)
		f.exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'paid')`, fulID, orderID, seller)
		f.exec(`INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id)
			VALUES ($1, $2, $3, $4, $5, 'legacy', 'legacy', 50000, 2, 100000, $6)`, itemID, orderID, p.ID, p.VariantID, seller, fulID)
		return orderID, fulID, itemID
	}

	_, fulID, itemID := insertLegacyOrder()
	f.deliver(fulID, time.Now())
	assertLine(t, f.ledger(itemID), 100_000, 9_000, 91_000)

	// Already accrued by the old code: the new code adds nothing and rewrites nothing.
	orderID2, fulID2, itemID2 := insertLegacyOrder()
	for typ, amount := range map[string]int64{"sale_gross": 100_000, "zamk_commission": -9_000, "seller_earning": 91_000} {
		f.exec(`INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'RUB', now())`, uuid.New(), seller, orderID2, itemID2, typ, amount)
	}
	f.deliver(fulID2, time.Now())
	assertLine(t, f.ledger(itemID2), 100_000, 9_000, 91_000)
	var nullMeta bool
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT metadata IS NULL FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'sale_gross'`, itemID2).Scan(&nullMeta))
	assert.True(t, nullMeta, "historical entry left untouched")
}

func TestM01_InvariantSanity(t *testing.T) {
	// Guard against the suite silently running nowhere: the fixture must reach zamk_test.
	f := newFinFixture(t)
	var db string
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT current_database()`).Scan(&db))
	require.Equal(t, "zamk_test", db, fmt.Sprintf("unexpected database %q", db))
}
