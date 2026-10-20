package payouts_test

// MARKETING.M0.3 — self-purchase guard, 100% promotions and co-funded max_discount_cents on zamk_test.
// Uses the M0.1 fixture: real checkout, mock payment confirmation (no external provider), accrual, refunds.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
)

func (f *finFixture) joinStore(userID, sellerID uuid.UUID, role string) {
	f.t.Helper()
	f.exec(`INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, $4)`, uuid.New(), sellerID, userID, role)
}

func (f *finFixture) checkoutErr(buyerID uuid.UUID, code string) (*orders.Order, error) {
	req := orders.CreateOrderRequest{
		CustomerName: "M03 Buyer", CustomerPhone: "+79990000000", CustomerEmail: "m03@test.local",
		DeliveryAddress: "Moscow", DeliveryMethodID: f.dmID,
	}
	if code != "" {
		req.PromoCode = &code
	}
	return f.ordersSvc.CreateOrder(f.ctx, buyerID, req, nil)
}

func (f *finFixture) reservedStock(p product) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, p.VariantID).Scan(&n))
	return n
}

func (f *finFixture) ordersOf(buyerID uuid.UUID) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT count(*) FROM orders WHERE user_id = $1`, buyerID).Scan(&n))
	return n
}

func (f *finFixture) promoUsages(code string) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT count(*) FROM promo_code_usages u JOIN promo_codes p ON p.id = u.promo_code_id WHERE p.code = $1`, code).Scan(&n))
	return n
}

// cofundedPromo creates an approved co-funded promotion (no HTTP route exists for it; the M0.1 fixture
// creates the campaign through the repository) and optionally sets the promo's max_discount_cents.
func (f *finFixture) cofundedPromo(sellerID uuid.UUID, sellerBps, zamkBps int, budgetCents int64, capCents *int64) (string, uuid.UUID) {
	f.t.Helper()
	code, campID := f.fundedPromo(sellerID, marketing.FundingModeCofunded, sellerBps, zamkBps, budgetCents)
	if capCents != nil {
		f.exec(`UPDATE promo_codes SET max_discount_cents = $2 WHERE code = $1`, code, *capCents)
	}
	return code, campID
}

func (f *finFixture) campaignSpend(campID uuid.UUID) (reserved, spent int64) {
	f.t.Helper()
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT zamk_reserved_cents, zamk_spent_cents FROM marketing_campaigns WHERE id = $1`, campID).Scan(&reserved, &spent))
	return
}

// assertSnapshotInvariants checks customer_paid + zamk_subsidy = commission_base and
// commission_base = base - seller_discount, and the accrued ledger: commission_base = earning + commission.
func assertSnapshotInvariants(t *testing.T, f *finFixture, itemID uuid.UUID) promoSnapshot {
	t.Helper()
	snap := f.snapshot(itemID)
	assert.Equal(t, snap.CommissionBase, snap.CustomerPaid+snap.ZamkSubsidy, "customer_paid + zamk_subsidy = commission_base")
	l := f.ledger(itemID)
	if len(l) > 0 {
		assert.Equal(t, l["sale_gross"], l["seller_earning"]-l["zamk_commission"], "commission_base = seller_earning + zamk_commission")
		assert.Equal(t, snap.CommissionBase, l["sale_gross"])
		assert.Equal(t, snap.Commission, -l["zamk_commission"])
	}
	return snap
}

// --- 1. Self-purchase -----------------------------------------------------------------------------

func TestM03_A_C_E_SelfPurchaseGuard(t *testing.T) {
	f := newFinFixture(t)
	own, other := f.seller(), f.seller()
	ownProduct, otherProduct := f.product(own, 100_000), f.product(other, 100_000)

	for _, role := range []string{"owner", "manager"} {
		t.Run("A: "+role+" cannot buy own product", func(t *testing.T) {
			member := f.buyer()
			f.joinStore(member, own, role)
			f.addToCart(member, ownProduct, 1)
			before := f.reservedStock(ownProduct)
			_, err := f.checkoutErr(member, "")
			require.ErrorIs(t, err, orders.ErrSelfPurchase)
			assert.Equal(t, before, f.reservedStock(ownProduct), "no stock reservation")
			assert.Zero(t, f.ordersOf(member), "no order")
		})
	}

	t.Run("A: a promo code changes nothing (no promo reservation either)", func(t *testing.T) {
		member := f.buyer()
		f.joinStore(member, own, "owner")
		f.addToCart(member, ownProduct, 1)
		code := f.sellerPromo(own, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 3000})
		_, err := f.checkoutErr(member, code)
		require.ErrorIs(t, err, orders.ErrSelfPurchase)
		assert.Zero(t, f.promoUsages(code))
	})

	t.Run("B: a store member buys from another store", func(t *testing.T) {
		member := f.buyer()
		f.joinStore(member, own, "owner")
		f.addToCart(member, otherProduct, 1)
		order, err := f.checkoutErr(member, "")
		require.NoError(t, err)
		assert.Equal(t, 100_000+deliveryPriceCents, order.TotalPriceCents)
	})

	t.Run("C: a mixed cart is rejected as a whole", func(t *testing.T) {
		member := f.buyer()
		f.joinStore(member, own, "manager")
		f.addToCart(member, otherProduct, 1)
		f.addToCart(member, ownProduct, 2)
		beforeOwn, beforeOther := f.reservedStock(ownProduct), f.reservedStock(otherProduct)
		_, err := f.checkoutErr(member, "")
		require.ErrorIs(t, err, orders.ErrSelfPurchase)
		assert.Equal(t, beforeOwn, f.reservedStock(ownProduct))
		assert.Equal(t, beforeOther, f.reservedStock(otherProduct), "the other store's item is not reserved either")
		assert.Zero(t, f.ordersOf(member))
	})

	t.Run("E: a buyer without any store link checks out normally", func(t *testing.T) {
		buyer := f.buyer()
		f.addToCart(buyer, ownProduct, 1)
		f.addToCart(buyer, otherProduct, 1)
		_, err := f.checkoutErr(buyer, "")
		require.NoError(t, err)
	})
}

func TestM03_D_SelfPurchaseViaCheckoutAPI(t *testing.T) {
	f := newFinFixture(t)
	own := f.seller()
	p := f.product(own, 100_000)
	member := f.buyer()
	f.joinStore(member, own, "owner")
	f.addToCart(member, p, 1)

	h := orders.NewHandler(f.ordersSvc)
	body, err := json.Marshal(orders.CreateOrderRequest{
		CustomerName: "M03", CustomerPhone: "+79990000000", CustomerEmail: "m03@test.local",
		DeliveryAddress: "Moscow", DeliveryMethodID: f.dmID,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "userID", member))
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var out struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "self_purchase_not_allowed", out.Error.Code)
	assert.NotEmpty(t, out.Error.Message)
	assert.NotContains(t, w.Body.String(), own.String(), "no store id in the response")
	assert.NotContains(t, w.Body.String(), "owner", "no membership details in the response")
	assert.Zero(t, f.ordersOf(member))
}

// --- 2. 100% promotions -----------------------------------------------------------------------------

func TestM03_F_G_FullDiscountCreationRules(t *testing.T) {
	f := newFinFixture(t)
	seller := f.seller()

	t.Run("F: seller promo at 100% is rejected", func(t *testing.T) {
		_, err := f.marketingSvc.CreateSellerPromotion(f.ctx, seller, marketing.CreateSellerPromoRequest{
			Code: "M03F" + uuid.NewString()[:8], DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 10000, PerCustomerUsageLimit: 1})
		require.ErrorIs(t, err, marketing.ErrFullDiscountNotAllowed)
	})

	t.Run("F: co-funded promo code adding up to 100% is rejected", func(t *testing.T) {
		_, campID := f.fundedPromo(seller, marketing.FundingModeCofunded, 3000, 1000, 500_000)
		_, err := f.marketingSvc.CreatePromoCode(f.ctx, seller, marketing.CreatePromoCodeRequest{
			CampaignID: campID, Code: "M03C" + uuid.NewString()[:8], DiscountType: marketing.DiscountTypePercent,
			DiscountValueBps: 10000, PerCustomerUsageLimit: 1})
		require.ErrorIs(t, err, marketing.ErrFullDiscountNotAllowed)
	})

	t.Run("F: seller campaign at 100% is rejected", func(t *testing.T) {
		_, err := f.marketingSvc.CreateSellerCampaign(f.ctx, seller, marketing.CreateSellerCampaignRequest{
			Title: "M03 full", DiscountType: marketing.DiscountTypePercent, SellerDiscountBps: 10000})
		require.ErrorIs(t, err, marketing.ErrFullDiscountNotAllowed)
	})

	t.Run("G: discounts below 100% keep working", func(t *testing.T) {
		buyer := f.buyer()
		f.addToCart(buyer, f.product(seller, 1_000_000), 1)
		order := f.checkout(buyer, f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 9999}))
		snap := assertSnapshotInvariants(t, f, onlyItem(t, order).ID)
		assert.Equal(t, int64(100), snap.CustomerPaid, "99.99% of 10 000 ₽ leaves 1 ₽")
	})
}

func TestM03_H_LegacyFullDiscountCannotBeUsed(t *testing.T) {
	f := newFinFixture(t)
	seller := f.seller()
	p := f.product(seller, 1_000_000)

	// A 100% seller promo created before M0.3 (the row is written the way the old code wrote it).
	code := f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 5000})
	f.exec(`UPDATE promo_codes SET discount_value_bps = 10000 WHERE code = $1`, code)
	f.exec(`UPDATE marketing_campaigns SET seller_discount_bps = 10000 WHERE id = (SELECT campaign_id FROM promo_codes WHERE code = $1)`, code)

	buyer := f.buyer()
	f.addToCart(buyer, p, 1)
	before := f.reservedStock(p)
	_, err := f.checkoutErr(buyer, code)
	require.ErrorIs(t, err, marketing.ErrPromoFullDiscount)
	assert.Equal(t, before, f.reservedStock(p))
	assert.Zero(t, f.ordersOf(buyer))
	assert.Zero(t, f.promoUsages(code))

	_, err = f.ordersSvc.ValidateCartPromo(f.ctx, buyer, code)
	require.ErrorIs(t, err, marketing.ErrPromoFullDiscount, "the cart preview refuses it too")

	// The legacy row can be deactivated but not reactivated.
	var promoID uuid.UUID
	require.NoError(t, f.db.QueryRow(f.ctx, `SELECT id FROM promo_codes WHERE code = $1`, code).Scan(&promoID))
	off, on := false, true
	_, err = f.marketingSvc.UpdateSellerPromotion(f.ctx, seller, promoID, marketing.UpdateSellerPromoRequest{IsActive: &off})
	require.NoError(t, err)
	_, err = f.marketingSvc.UpdateSellerPromotion(f.ctx, seller, promoID, marketing.UpdateSellerPromoRequest{IsActive: &on})
	require.ErrorIs(t, err, marketing.ErrFullDiscountNotAllowed)

	t.Run("a fixed discount covering the whole item is refused at checkout", func(t *testing.T) {
		b := f.buyer()
		f.addToCart(b, f.product(seller, 50_000), 1)
		fixed := f.sellerPromo(seller, marketing.CreateSellerPromoRequest{DiscountType: marketing.DiscountTypeFixed, DiscountValueFixedCents: 50_000})
		_, err := f.checkoutErr(b, fixed)
		require.ErrorIs(t, err, marketing.ErrPromoFullDiscount)
	})

	t.Run("co-funding cannot be used to make an item free", func(t *testing.T) {
		code, _ := f.cofundedPromo(seller, 3000, 1000, 5_000_000, nil)
		f.exec(`UPDATE promo_codes SET discount_value_bps = 10000 WHERE code = $1`, code)
		f.exec(`UPDATE marketing_campaigns SET seller_discount_bps = 7500, requested_zamk_share_bps = 2500, approved_zamk_share_bps = 2500
			WHERE id = (SELECT campaign_id FROM promo_codes WHERE code = $1)`, code)
		b := f.buyer()
		f.addToCart(b, f.product(seller, 1_000_000), 1)
		_, err := f.checkoutErr(b, code)
		require.ErrorIs(t, err, marketing.ErrPromoFullDiscount)
	})
}

// --- 3. Co-funded max_discount_cents ------------------------------------------------------------------

func TestM03_I_J_K_CofundedCap(t *testing.T) {
	cases := []struct {
		name                          string
		capCents                      *int64
		seller, zamk, paid, base, com int64
	}{
		{"I: no cap", nil, 300_000, 100_000, 600_000, 700_000, 63_000},
		{"J: cap 2 000 ₽ below the 4 000 ₽ discount", ptr(200_000), 150_000, 50_000, 800_000, 850_000, 76_500},
		{"K: cap 5 000 ₽ above the discount", ptr(500_000), 300_000, 100_000, 600_000, 700_000, 63_000},
		{"cap equal to the discount", ptr(400_000), 300_000, 100_000, 600_000, 700_000, 63_000},
		{"cap 1 kopeck below the discount", ptr(399_999), 299_999, 100_000, 600_001, 700_001, 63_000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFinFixture(t)
			seller, buyer := f.seller(), f.buyer()
			f.addToCart(buyer, f.product(seller, 1_000_000), 1)
			code, campID := f.cofundedPromo(seller, 3000, 1000, 5_000_000, c.capCents)
			order := f.checkout(buyer, code)
			item := onlyItem(t, order)
			assert.Equal(t, c.paid+deliveryPriceCents, order.TotalPriceCents)
			snap := assertSnapshotInvariants(t, f, item.ID)
			assert.Equal(t, c.seller, snap.SellerDiscount)
			assert.Equal(t, c.zamk, snap.ZamkSubsidy)
			assert.Equal(t, c.paid, snap.CustomerPaid)
			assert.Equal(t, c.base, snap.CommissionBase)
			assert.Equal(t, c.com, snap.Commission)
			reserved, _ := f.campaignSpend(campID)
			assert.Equal(t, c.zamk, reserved, "ZAMK budget reserves only its capped share")

			f.pay(buyer, order.ID)
			_, spent := f.campaignSpend(campID)
			assert.Equal(t, c.zamk, spent)
			f.deliver(*item.OrderFulfillmentID, time.Now())
			assertLine(t, f.ledger(item.ID), c.base, c.com, c.base-c.com)
			assertSnapshotInvariants(t, f, item.ID)
		})
	}
}

func ptr(v int64) *int64 { return &v }

func TestM03_L_M_CofundedCapRoundingAcrossLinesAndSellers(t *testing.T) {
	f := newFinFixture(t)
	sellerA, sellerB, buyer := f.seller(), f.seller(), f.buyer()
	a1, a2, b1 := f.product(sellerA, 33_333), f.product(sellerA, 77_777), f.product(sellerB, 50_000)
	f.addToCart(buyer, a1, 3)
	f.addToCart(buyer, a2, 2)
	f.addToCart(buyer, b1, 1)
	capCents := int64(30_001)
	code, campID := f.cofundedPromo(sellerA, 3000, 1000, 5_000_000, &capCents)
	order := f.checkout(buyer, code)
	require.Len(t, order.Items, 3)

	var S, Z int64
	for _, it := range order.Items {
		if it.SellerID == sellerB {
			var n int
			require.NoError(t, f.db.QueryRow(f.ctx, `SELECT count(*) FROM order_item_promotions WHERE order_item_id = $1`, it.ID).Scan(&n))
			assert.Zero(t, n, "another seller's item is not promoted")
			continue
		}
		snap := assertSnapshotInvariants(t, f, it.ID)
		S += snap.SellerDiscount
		Z += snap.ZamkSubsidy
		assert.Zero(t, snap.ZamkSubsidy%int64(it.Quantity), "ZAMK subsidy stays a per-unit amount")
	}
	// uncapped shares of seller A's lines (30 % seller + 10 % ZAMK)
	var uncappedS, uncappedZ int64
	for _, l := range []struct {
		base int64
		q    int
	}{{33_333, 3}, {77_777, 2}} {
		r, err := marketing.CalculateLineSplitDiscount(l.base, l.q, 3000, 1000, 900)
		require.NoError(t, err)
		uncappedS += r.TotalSellerDiscountCents
		uncappedZ += r.TotalZamkSubsidyCents
	}
	require.Greater(t, uncappedS+uncappedZ, capCents)
	assert.Equal(t, capCents, S+Z, "the capped total is exact to the kopeck")
	assert.LessOrEqual(t, Z, uncappedZ)
	assert.LessOrEqual(t, S, uncappedS)
	assert.GreaterOrEqual(t, S, capCents*uncappedS/(uncappedS+uncappedZ), "seller pays at least its proportional share")
	assert.LessOrEqual(t, S, capCents*uncappedS/(uncappedS+uncappedZ)+5, "and only rounding (< one unit step) above it")

	reserved, _ := f.campaignSpend(campID)
	assert.Equal(t, Z, reserved)

	// deterministic: the same cart yields the same split
	buyer2 := f.buyer()
	f.addToCart(buyer2, a1, 3)
	f.addToCart(buyer2, a2, 2)
	order2 := f.checkout(buyer2, code)
	var S2, Z2 int64
	for _, it := range order2.Items {
		snap := f.snapshot(it.ID)
		S2 += snap.SellerDiscount
		Z2 += snap.ZamkSubsidy
	}
	assert.Equal(t, S, S2)
	assert.Equal(t, Z, Z2)
}

func TestM03_N_O_CappedCofundedPaymentIdempotencyAndRefund(t *testing.T) {
	f := newFinFixture(t)
	seller, buyer := f.seller(), f.buyer()
	f.addToCart(buyer, f.product(seller, 1_000_000), 1)
	capCents := int64(200_000)
	code, campID := f.cofundedPromo(seller, 3000, 1000, 5_000_000, &capCents)
	order := f.checkout(buyer, code)
	item := onlyItem(t, order)

	paymentID := f.pay(buyer, order.ID)
	require.ErrorIs(t, f.paymentsSvc.ProcessMockPaymentAction(f.ctx, paymentID, "confirm"), payments.ErrPaymentAlreadyProcessed)
	reserved, spent := f.campaignSpend(campID)
	assert.Equal(t, int64(50_000), spent, "N: repeated confirmation spends the capped ZAMK share once")
	assert.Zero(t, reserved)

	f.deliver(*item.OrderFulfillmentID, time.Now())
	ref := f.refund(f.createReceivedReturn(order, buyer, map[uuid.UUID]int{item.ID: 1}))
	assert.Equal(t, int64(800_000), ref.AmountCents, "O: the customer gets back what was paid")
	l := f.ledger(item.ID)
	assert.Equal(t, -l["seller_earning"], l["adjustment:return_deduction"], "seller earning reversed exactly once")
	_, spent = f.campaignSpend(campID)
	assert.Equal(t, int64(50_000), spent, "subsidy budget is not restored after a refund (unchanged rule)")
}

func TestM03_ConcurrentCappedCofundedBudget(t *testing.T) {
	f := newFinFixture(t)
	seller := f.seller()
	p := f.product(seller, 1_000_000)
	capCents := int64(200_000) // each order: ZAMK 500 ₽
	code, campID := f.cofundedPromo(seller, 3000, 1000, 150_000, &capCents)

	const n = 6
	buyers := make([]uuid.UUID, n)
	for i := range buyers {
		buyers[i] = f.buyer()
		f.addToCart(buyers[i], p, 1)
	}
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range buyers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = f.checkoutErr(buyers[i], code)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else {
			assert.ErrorIs(t, err, marketing.ErrPromoBudgetExhausted)
		}
	}
	assert.Equal(t, 3, ok, "exactly the budget's worth of capped subsidies is reserved")
	reserved, spent := f.campaignSpend(campID)
	assert.Equal(t, int64(150_000), reserved+spent)
}
