package payments

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// ORDER.E2E.2: what happens after a successful payment, on a disposable database. The mock confirmation and the
// signed T-Bank notification are the only two ways to confirm a payment; both must move the order to paid exactly
// once, keep the reservation as committed stock, and leave nothing to repeat on a replay.

type paidFixture struct {
	*payFixture
	p, v    uuid.UUID
	buyerID uuid.UUID
	orderID uuid.UUID
	number  string
	payID   uuid.UUID
}

// newPaidFixture starts a real terminal (so signed notifications can be checked) with T-Pay in mock mode and takes
// one order with a pending mock payment.
func newPaidFixture(t *testing.T, stock int) *paidFixture {
	t.Helper()
	f := &paidFixture{payFixture: newPayFixture(t, "TERM", "SECRET")}
	f.p, f.v = f.variant(500000, stock)
	f.buyerID = f.buyer()
	f.addToCart(f.buyerID, f.p, f.v, 1)
	f.orderID, f.number = f.mustOrder(f.buyerID, "k-"+uuid.NewString())
	f.payID = f.mustPay(f.buyerID, f.orderID)
	return f
}

// tableCounts is the row count of every public table: the replay must leave all of it untouched.
func (f *payFixture) tableCounts() map[string]int {
	f.t.Helper()
	rows, err := f.db.Query(f.ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename <> 'schema_migrations'`)
	require.NoError(f.t, err)
	var names []string
	for rows.Next() {
		var n string
		require.NoError(f.t, rows.Scan(&n))
		names = append(names, n)
	}
	rows.Close()
	out := map[string]int{}
	for _, n := range names {
		out[n] = f.count(`SELECT count(*) FROM "` + n + `"`)
	}
	return out
}

func (f *payFixture) str(sql string, args ...any) string {
	f.t.Helper()
	var s string
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&s), sql)
	return s
}

// signedWebhook builds a T-Bank notification for the payment signed with the terminal password.
func (f *paidFixture) signedWebhook(status string, extra map[string]any) []byte {
	f.t.Helper()
	providerID := f.str(`SELECT provider_payment_id FROM payments WHERE id=$1`, f.payID)
	pid, err := strconv.ParseInt(providerID, 10, 64)
	require.NoError(f.t, err)
	amount := int64(f.count(`SELECT amount_cents FROM payments WHERE id=$1`, f.payID))
	data := map[string]any{"TerminalKey": "TERM", "OrderId": f.orderID.String(), "Success": true, "Status": status, "PaymentId": pid, "Amount": amount, "ErrorCode": "0"}
	for k, v := range extra {
		data[k] = v
	}
	params := map[string]string{}
	for k, v := range data {
		switch x := v.(type) {
		case string:
			params[k] = x
		case int64:
			params[k] = strconv.FormatInt(x, 10)
		case bool:
			params[k] = strconv.FormatBool(x)
		}
	}
	data["Token"] = f.provider.generateToken(params)
	b, _ := json.Marshal(data)
	return b
}

func (f *paidFixture) mockAction(action string) error {
	return f.svc.ProcessMockPaymentAction(f.ctx, f.payID, action)
}

func (f *paidFixture) assertPaid() {
	t := f.t
	require.Equal(t, "paid", f.str(`SELECT status FROM orders WHERE id=$1`, f.orderID))
	require.Equal(t, "succeeded", f.str(`SELECT status FROM payments WHERE id=$1`, f.payID))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM payments WHERE order_id=$1 AND paid_at IS NOT NULL`, f.orderID))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id=$1 AND status='paid'`, f.orderID))
	require.Equal(t, 0, f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id=$1 AND status<>'paid'`, f.orderID))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id=$1 AND to_status='paid'`, f.orderID))
	// the reservation is converted, the unit stays committed (reserved_stock) until the physical shipment
	require.Equal(t, 0, f.activeReservations(f.v))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM reservations WHERE product_variant_id=$1 AND status='converted'`, f.v))
	require.Equal(t, 1, f.reserved(f.v))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM stock_movements WHERE product_variant_id=$1 AND type='sale'`, f.v))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM stock_movements WHERE product_variant_id=$1 AND type='sale' AND reference_id=$2`, f.v, f.orderID), "the sale movement references its order")
	require.Equal(t, 0, f.claims(f.buyerID))
}

func TestOrderE2E2(t *testing.T) {
	t.Run("mock confirmation: order and fulfillment paid, reservation converted, stock consistent, ready for picking", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.Equal(t, "awaiting_payment", f.str(`SELECT status FROM orders WHERE id=$1`, f.orderID))
		require.Equal(t, "pending", f.str(`SELECT status FROM payments WHERE id=$1`, f.payID))
		require.Equal(t, 1, f.reserved(f.v))
		require.Equal(t, 1, f.activeReservations(f.v))

		beforeCounts := f.tableCounts()
		require.NoError(t, f.mockAction("confirm"))
		f.assertPaid()
		// the rows a confirmation may add: no ledger, balance, payout or refund record is created at payment time
		allowed := map[string]bool{"payment_events": true, "order_status_history": true, "stock_movements": true, "notifications": true, "analytics_events": true, "behavioral_events": true}
		for name, n := range f.tableCounts() {
			if n != beforeCounts[name] && !allowed[name] {
				t.Logf("confirmation changed %s: %d -> %d", name, beforeCounts[name], n)
				require.NotRegexp(t, `ledger|payout|balance|refund|commission|transaction`, name, "finance table written at payment time")
			}
		}
		require.Equal(t, 5, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id=$1`, f.v), "physical stock is untouched until shipment")
		require.Equal(t, 1, f.count(`SELECT count(*) FROM notifications WHERE type='fulfillment_paid'`), "the seller gets one notification")

		// the existing hand-off: a paid fulfillment of a paid order is in the warehouse picking queue
		var fid uuid.UUID
		require.NoError(t, f.db.QueryRow(f.ctx, `SELECT id FROM order_fulfillments WHERE order_id=$1`, f.orderID).Scan(&fid))
		ok, err := fulfillment.IsFulfillmentActionablePicking(f.ctx, f.db, fid)
		require.NoError(t, err)
		require.True(t, ok, "paid order must be actionable for picking")

		// an already paid order cannot be paid again
		res := f.pay(f.buyerID, f.orderID, "tpay")
		require.Equal(t, http.StatusBadRequest, res.Status, "%+v", res)
		require.Equal(t, 1, f.paymentsOf(f.orderID))
		f.assertPaid()
		require.Equal(t, int32(0), f.net.calls.Load(), "no outgoing request")
	})

	t.Run("signed T-Bank notification confirms the same way", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", nil)))
		f.assertPaid()
	})

	t.Run("replays change nothing: mock again, same notification, a differently-keyed notification", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", nil)))
		f.assertPaid()
		before := f.tableCounts()
		eventsBefore := f.count(`SELECT count(*) FROM payment_events WHERE payment_id=$1`, f.payID)

		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", nil)))
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", map[string]any{"CardId": int64(77)})))
		err := f.mockAction("confirm")
		require.True(t, err == nil || errors.Is(err, ErrPaymentAlreadyProcessed), "%v", err)
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("AUTHORIZED", nil)))

		after := f.tableCounts()
		for name, n := range before {
			if name == "payment_events" {
				continue // the replayed notifications are recorded as events, nothing else changes
			}
			require.Equal(t, n, after[name], "table %s changed on a replay", name)
		}
		require.GreaterOrEqual(t, f.count(`SELECT count(*) FROM payment_events WHERE payment_id=$1`, f.payID), eventsBefore)
		f.assertPaid()
	})

	t.Run("concurrent confirmations of one payment transition once", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		var wg sync.WaitGroup
		errs := make(chan error, 12)
		bodies := make([][]byte, 12) // built up front: the pool is small and every worker holds a connection
		for i := range bodies {
			bodies[i] = f.signedWebhook("CONFIRMED", map[string]any{"CardId": int64(i)})
		}
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if i%2 == 0 {
					errs <- f.mockAction("confirm")
				} else {
					errs <- f.svc.HandleWebhook(f.ctx, nil, bodies[i])
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				require.True(t, errors.Is(err, ErrPaymentAlreadyProcessed), "unexpected error: %v", err)
			}
		}
		f.assertPaid()
		require.Equal(t, 1, f.count(`SELECT count(*) FROM notifications WHERE type='fulfillment_paid'`))
	})

	t.Run("failure while confirming rolls everything back and the retry succeeds", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		// the reserved unit disappears: conversion cannot happen, the confirmation must fail as a whole
		f.exec(`UPDATE reservations SET status='released' WHERE product_variant_id=$1`, f.v)
		before := f.tableCounts()
		err := f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", nil))
		require.Error(t, err)
		require.Equal(t, "awaiting_payment", f.str(`SELECT status FROM orders WHERE id=$1`, f.orderID))
		require.Equal(t, "pending", f.str(`SELECT status FROM payments WHERE id=$1`, f.payID))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id=$1 AND status='awaiting_payment'`, f.orderID))
		require.Equal(t, before, f.tableCounts(), "nothing is left half-confirmed")

		f.exec(`UPDATE reservations SET status='active' WHERE product_variant_id=$1`, f.v)
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", nil)))
		f.assertPaid()
	})

	t.Run("forged or altered notifications never confirm a payment", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		before := f.tableCounts()
		body := f.signedWebhook("CONFIRMED", nil)
		var m map[string]any
		require.NoError(t, json.Unmarshal(body, &m))

		bad := map[string]any{}
		for k, v := range m {
			bad[k] = v
		}
		bad["Token"] = "deadbeef"
		b1, _ := json.Marshal(bad)
		require.ErrorIs(t, f.svc.HandleWebhook(f.ctx, nil, b1), ErrInvalidSignature)

		missing := map[string]any{}
		for k, v := range m {
			if k != "Token" {
				missing[k] = v
			}
		}
		b2, _ := json.Marshal(missing)
		require.ErrorIs(t, f.svc.HandleWebhook(f.ctx, nil, b2), ErrInvalidSignature)

		changed := map[string]any{}
		for k, v := range m {
			changed[k] = v
		}
		changed["Amount"] = int64(1) // signed for another amount
		b3, _ := json.Marshal(changed)
		require.ErrorIs(t, f.svc.HandleWebhook(f.ctx, nil, b3), ErrInvalidSignature)

		// correctly signed, but for the wrong amount: refused, nothing confirmed
		wrong := f.signedWebhook("CONFIRMED", map[string]any{"Amount": int64(100)})
		require.Error(t, f.svc.HandleWebhook(f.ctx, nil, wrong))

		require.Equal(t, "awaiting_payment", f.str(`SELECT status FROM orders WHERE id=$1`, f.orderID))
		require.Equal(t, "pending", f.str(`SELECT status FROM payments WHERE id=$1`, f.payID))
		require.Equal(t, before["order_status_history"], f.tableCounts()["order_status_history"])
	})

	t.Run("mock endpoints are closed in production", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		h := NewHandler(f.svc, "production")
		r := httptest.NewRequest(http.MethodPost, "/api/dev/payments/mock/"+f.payID.String()+"/confirm", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", f.payID.String())
		rctx.URLParams.Add("action", "confirm")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.ProcessDevMockAction(w, r)
		require.Equal(t, http.StatusForbidden, w.Code)
		require.Equal(t, "pending", f.str(`SELECT status FROM payments WHERE id=$1`, f.payID))
	})

	t.Run("a paid order keeps its reservation against a competing order for the last unit", func(t *testing.T) {
		f := newPaidFixture(t, 1) // the only unit is reserved by the first order
		other := f.buyer()
		f.addToCart(other, f.p, f.v, 1)
		res := f.order(other, "k-"+uuid.NewString())
		require.GreaterOrEqual(t, res.Status, 400, "%+v", res)
		require.Equal(t, 0, f.ordersOf(other))
		require.NoError(t, f.mockAction("confirm"))
		f.assertPaid()
		res = f.order(other, "k-"+uuid.NewString())
		require.GreaterOrEqual(t, res.Status, 400, "paid stock stays committed: %+v", res)
		require.Equal(t, 0, f.ordersOf(other))
		require.Equal(t, 1, f.reserved(f.v))
		require.Equal(t, 1, f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id=$1`, f.v))
		f.assertPaid()
	})

	t.Run("confirmation arriving after the payment was rejected does not move the order", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		require.Equal(t, "failed", f.str(`SELECT status FROM payments WHERE id=$1`, f.payID))
		require.Equal(t, 0, f.reserved(f.v))
		before := f.tableCounts()
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", nil)))
		after := f.tableCounts()
		for _, name := range []string{"orders", "order_status_history", "stock_movements", "reservations", "notifications", "order_fulfillments"} {
			require.Equal(t, before[name], after[name], name)
		}
		require.NotEqual(t, "paid", f.str(`SELECT status FROM orders WHERE id=$1`, f.orderID))
		require.Equal(t, 0, f.reserved(f.v))
		require.GreaterOrEqual(t, f.count(`SELECT total_stock - reserved_stock FROM inventory_items WHERE product_variant_id=$1`, f.v), 0)
	})
}
