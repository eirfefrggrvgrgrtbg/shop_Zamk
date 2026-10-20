package marketing_test

// MARKETING.M1.6 — seller promo statistics on zamk_test: outcomes are kept apart, only paid orders count as
// sales, repeated payment callbacks do not double count, and sellers only see their own codes.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
)

func TestSellerPromoStats(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()
	seller1 := createTestSeller(t, client)
	seller2 := createTestSeller(t, client)
	var userIDs, orderIDs, campaignIDs []uuid.UUID
	defer func() {
		for _, oid := range orderIDs {
			_, _ = client.Pool.Exec(ctx, `DELETE FROM return_items WHERE return_id IN (SELECT id FROM returns WHERE order_id = $1)`, oid)
			_, _ = client.Pool.Exec(ctx, `DELETE FROM returns WHERE order_id = $1`, oid)
		}
		cleanupExactFixtures(client, []uuid.UUID{seller1, seller2}, userIDs, orderIDs, campaignIDs)
	}()

	code := uniqueCode("ST")
	promo, err := svc.CreateSellerPromotion(ctx, seller1, marketing.CreateSellerPromoRequest{
		Code: code, DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, promo.CampaignID)
	emptyPromo, err := svc.CreateSellerPromotion(ctx, seller1, marketing.CreateSellerPromoRequest{
		Code: uniqueCode("EMP"), DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 500,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, emptyPromo.CampaignID)
	otherPromo, err := svc.CreateSellerPromotion(ctx, seller2, marketing.CreateSellerPromoRequest{
		Code: uniqueCode("OTH"), DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, otherPromo.CampaignID)

	// newOrder reserves the promo on a fresh 1000.00 ₽ order (10% off) for a fresh buyer.
	newOrder := func() uuid.UUID {
		userID := createTestStaffUser(t, client)
		userIDs = append(userIDs, userID)
		orderID := createTestOrder(t, client, userID)
		orderIDs = append(orderIDs, orderID)
		itemID, prodID, varID := createTestOrderItem(t, client, orderID, seller1, 100000, 1)
		items := []marketing.PromotedOrderItemInput{{
			OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: seller1,
			BaseUnitPriceCents: 100000, Quantity: 1,
		}}
		require.NoError(t, client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
			require.NoError(t, err)
			return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
		}))
		return orderID
	}
	consume := func(orderID uuid.UUID) {
		require.NoError(t, client.RunInTx(ctx, func(tx pgx.Tx) error { return svc.ConsumePromoForOrderTx(ctx, tx, orderID) }))
	}
	release := func(orderID uuid.UUID) {
		require.NoError(t, client.RunInTx(ctx, func(tx pgx.Tx) error { return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "test") }))
	}
	statsFor := func(sellerID, promoID uuid.UUID) marketing.PromoStats {
		items, err := svc.ListSellerPromoStats(ctx, sellerID)
		require.NoError(t, err)
		for _, s := range items {
			if s.PromoCodeID == promoID {
				return s
			}
		}
		t.Fatalf("promo %s missing in stats", promoID)
		return marketing.PromoStats{}
	}

	t.Run("empty state: a code without orders has all zeros", func(t *testing.T) {
		assert.Equal(t, marketing.PromoStats{PromoCodeID: emptyPromo.ID}, statsFor(seller1, emptyPromo.ID))
	})

	// O1 reserved only; O2 paid; O3 paid then cancelled; O4 released; O5 paid with a completed return.
	o1 := newOrder()
	o2 := newOrder()
	consume(o2)
	consume(o2) // duplicate payment callback must not double count
	o3 := newOrder()
	consume(o3)
	_, err = client.Pool.Exec(ctx, `UPDATE orders SET status = 'cancelled', cancelled_at = now() WHERE id = $1`, o3)
	require.NoError(t, err)
	o4 := newOrder()
	release(o4)
	o5 := newOrder()
	consume(o5)
	var fulfillmentID, orderItemID uuid.UUID
	require.NoError(t, client.Pool.QueryRow(ctx, `SELECT id FROM order_fulfillments WHERE order_id = $1`, o5).Scan(&fulfillmentID))
	require.NoError(t, client.Pool.QueryRow(ctx, `SELECT id FROM order_items WHERE order_id = $1`, o5).Scan(&orderItemID))
	var o5User uuid.UUID
	require.NoError(t, client.Pool.QueryRow(ctx, `SELECT user_id FROM orders WHERE id = $1`, o5).Scan(&o5User))
	returnID := uuid.New()
	_, err = client.Pool.Exec(ctx, `INSERT INTO returns (id, order_id, user_id, status, reason, fulfillment_id) VALUES ($1, $2, $3, 'refunded', 'size', $4)`, returnID, o5, o5User, fulfillmentID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity) VALUES ($1, $2, $3, 1)`, uuid.New(), returnID, orderItemID)
	require.NoError(t, err)

	t.Run("outcomes are separated and only paid orders are sales", func(t *testing.T) {
		s := statsFor(seller1, promo.ID)
		assert.Equal(t, 1, s.ReservedOrders, "reserved: unpaid hold")
		assert.Equal(t, 2, s.PaidOrders, "paid: O2 and O5; O3 was cancelled after payment")
		assert.Equal(t, 1, s.CancelledAfterPaymentOrders)
		assert.Equal(t, 1, s.ReleasedOrders)
		assert.Equal(t, 1, s.ReturnedOrders, "only O5 has a refunded return")
		assert.Equal(t, int64(2*90000), s.SalesCents, "2 paid orders x 900.00 ₽ customer-paid; reserved/released/cancelled excluded")
		assert.Equal(t, int64(2*10000), s.SellerDiscountCents)
	})

	t.Run("detail lists orders with outcomes and no buyer data", func(t *testing.T) {
		d, err := svc.GetSellerPromoStats(ctx, seller1, promo.ID)
		require.NoError(t, err)
		require.Len(t, d.RecentOrders, 5)
		byOrder := map[uuid.UUID]marketing.PromoStatsOrder{}
		for _, o := range d.RecentOrders {
			byOrder[o.OrderID] = o
		}
		assert.Equal(t, marketing.PromoOutcomeReserved, byOrder[o1].Outcome)
		assert.Equal(t, marketing.PromoOutcomePaid, byOrder[o2].Outcome)
		assert.Equal(t, marketing.PromoOutcomeCancelledAfterPayment, byOrder[o3].Outcome)
		assert.Equal(t, marketing.PromoOutcomeReleased, byOrder[o4].Outcome)
		assert.True(t, byOrder[o5].Returned)
		assert.Equal(t, int64(90000), byOrder[o2].SalesCents)
		for _, id := range []uuid.UUID{o1, o3, o4} {
			assert.Zero(t, byOrder[id].SalesCents, "non-paid orders carry no sales amount")
		}
		raw, err := json.Marshal(d)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "userId")
		assert.NotContains(t, string(raw), "customer")
	})

	t.Run("sellers only see their own codes", func(t *testing.T) {
		items, err := svc.ListSellerPromoStats(ctx, seller2)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, otherPromo.ID, items[0].PromoCodeID)
		assert.Zero(t, items[0].PaidOrders+items[0].ReservedOrders+items[0].ReleasedOrders)

		_, err = svc.GetSellerPromoStats(ctx, seller2, promo.ID)
		assert.ErrorIs(t, err, marketing.ErrPromoCodeNotFound, "foreign promo looks like a missing one")
		_, err = svc.GetSellerPromoStats(ctx, seller1, uuid.New())
		assert.ErrorIs(t, err, marketing.ErrPromoCodeNotFound)
	})

	t.Run("HTTP: list, detail, foreign 404, bad id 404", func(t *testing.T) {
		h, _ := setupMarketingHTTPHandler(svc)
		r := chi.NewRouter()
		r.Get("/stats", h.ListSellerPromoStats)
		r.Get("/{id}/stats", h.GetSellerPromoStats)
		call := func(sellerID uuid.UUID, path string) (int, []byte) {
			req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(newTestContextWithSeller(sellerID))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			return w.Code, w.Body.Bytes()
		}
		code200, body := call(seller1, "/"+promo.ID.String()+"/stats")
		require.Equal(t, http.StatusOK, code200)
		var d marketing.SellerPromoStatsDetail
		require.NoError(t, json.Unmarshal(body, &d))
		assert.Equal(t, 2, d.PaidOrders)

		c, _ := call(seller2, "/"+promo.ID.String()+"/stats")
		assert.Equal(t, http.StatusNotFound, c)
		c, _ = call(seller1, "/not-a-uuid/stats")
		assert.Equal(t, http.StatusNotFound, c)
		c, body = call(seller2, "/stats")
		assert.Equal(t, http.StatusOK, c)
		assert.NotContains(t, string(body), promo.ID.String())
	})
}
