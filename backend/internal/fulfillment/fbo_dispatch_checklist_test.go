package fulfillment_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
)

// FBO.DISPATCH.CHECKLIST.1 on a disposable database: the readiness the dispatch context reports is computed by the
// backend from the same data the dispatch locks, and the dispatch decides again by itself.

func (f *fboFixture) readiness(fulID uuid.UUID) *fulfillment.DispatchReadiness {
	f.t.Helper()
	r, err := fulfillment.NewRepository(f.db).GetDispatchReadiness(f.ctx, fulID)
	require.NoError(f.t, err)
	return r
}

func check(t *testing.T, r *fulfillment.DispatchReadiness, code string) fulfillment.ReadinessCheck {
	t.Helper()
	for _, c := range r.Checks {
		if c.Code == code {
			return c
		}
	}
	t.Fatalf("no check %s in %+v", code, r.Checks)
	return fulfillment.ReadinessCheck{}
}

func failing(r *fulfillment.DispatchReadiness) []string {
	var out []string
	for _, c := range r.Checks {
		if !c.OK && c.Severity == fulfillment.SeverityBlocking {
			out = append(out, c.Code)
		}
	}
	return out
}

func (f *fboFixture) contextHTTP(fulID uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/fulfillments/"+fulID.String()+"/dispatch-context", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fulID.String())
	rec := httptest.NewRecorder()
	f.handler.GetDispatchContext(rec, req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))
	return rec
}

// parity: the checklist says "ready" exactly when the dispatch would go through
func (f *fboFixture) parity(fulID uuid.UUID, wantReady bool, wantCode string) {
	f.t.Helper()
	r := f.readiness(fulID)
	require.Equal(f.t, wantReady, r.Ready, "readiness %+v", r.Checks)
	rec := f.dispatchHTTP(fulID, &f.adminID)
	if wantReady {
		require.Equal(f.t, http.StatusOK, rec.Code, rec.Body.String())
		return
	}
	wantStatus := http.StatusConflict
	if wantCode == "invariant_violation" {
		wantStatus = http.StatusInternalServerError // the existing contract for corrupt allocation data (not a business refusal)
	}
	require.Equal(f.t, wantStatus, rec.Code, rec.Body.String())
	assert.Equal(f.t, wantCode, errorCode(f.t, rec))
	assert.NotContains(f.t, rec.Body.String(), "SELECT")
}

func TestDispatchChecklist(t *testing.T) {
	f := newFBOFixture(t)

	t.Run("A: a fully prepared parcel is ready; the dispatch goes through; afterwards it is no longer dispatchable (G)", func(t *testing.T) {
		_, ful, _, v, _ := f.packedSingleVariantOrder("")
		r := f.readiness(ful)
		assert.True(t, r.Ready)
		assert.Zero(t, r.BlockingCount)
		for _, c := range r.Checks {
			if c.Severity == fulfillment.SeverityBlocking {
				assert.True(t, c.OK, c.Code)
				assert.Empty(t, c.Detail, c.Code)
			}
		}
		before := f.stock(v.VariantID)
		f.parity(ful, true, "")
		assert.NotEqual(t, before, f.stock(v.VariantID), "the dispatch wrote the stock off")

		r = f.readiness(ful)
		assert.False(t, r.Ready)
		assert.Contains(t, failing(r), fulfillment.CheckNotShipped)
		assert.Contains(t, check(t, r, fulfillment.CheckNotShipped).Detail, "уже отгружена")
		stockAfter := f.stock(v.VariantID)
		rec := f.dispatchHTTP(ful, &f.adminID)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "dispatch_not_allowed", errorCode(t, rec))
		assert.Equal(t, stockAfter, f.stock(v.VariantID), "no second write-off")
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, ful))
	})

	t.Run("B: picking not finished — blocked with the picked count; paid-only and partly picked", func(t *testing.T) {
		_, ful, _, _, _ := f.packedSingleVariantOrder("partial") // 1 of 2 units picked
		r := f.readiness(ful)
		assert.Contains(t, failing(r), fulfillment.CheckPickingComplete)
		assert.Contains(t, failing(r), fulfillment.CheckPackingComplete)
		assert.Equal(t, "Собрано 1 из 2 ед.", check(t, r, fulfillment.CheckPickingComplete).Detail)
		f.parity(ful, false, "dispatch_not_allowed")

		prodID, vs := f.createProductWithVariants([]testVariantInfo{{Size: "M", Color: "Красный", Price: 100000, Barcode: uniqueBarcode()}}, 5)
		id, code := f.createSerializedUnit(vs[0].VariantID)
		oid, ful2, _ := f.createOrderWithFulfillment("awaiting_payment", "awaiting_payment", prodID, []orderItemSpec{{VariantInfo: vs[0], Quantity: 1, SerializedUnits: []unitRef{{id, code}}}})
		f.payViaInventory(oid, ful2)
		r = f.readiness(ful2)
		assert.Equal(t, "Собрано 0 из 1 ед.", check(t, r, fulfillment.CheckPickingComplete).Detail)
		assert.Contains(t, check(t, r, fulfillment.CheckPackingComplete).Detail, "Сборка не начата")
		f.parity(ful2, false, "dispatch_not_allowed")
	})

	t.Run("C: picked but not packed — packing is the only blocker", func(t *testing.T) {
		_, ful, _, _, _ := f.packedSingleVariantOrder("picked")
		r := f.readiness(ful)
		assert.Equal(t, []string{fulfillment.CheckPackingComplete}, failing(r))
		assert.True(t, check(t, r, fulfillment.CheckPickingComplete).OK)
		f.parity(ful, false, "dispatch_not_allowed")
	})

	t.Run("D: inconsistent composition and units are blocked with a reason", func(t *testing.T) {
		_, ful, item, _, units := f.packedSingleVariantOrder("")
		// one allocation of two released: 1 allocated for a quantity of 2
		f.exec(`UPDATE order_item_allocations SET released_at = now() WHERE order_item_id = $1 AND inventory_unit_id = $2`, item, units[0].UnitID)
		r := f.readiness(ful)
		assert.Contains(t, failing(r), fulfillment.CheckComposition)
		assert.Contains(t, check(t, r, fulfillment.CheckComposition).Detail, "Закреплено 1 ед. при заказанном количестве 2")
		f.parity(ful, false, "invariant_violation")

		_, ful2, _, _, units2 := f.packedSingleVariantOrder("")
		f.exec(`UPDATE inventory_units SET status = 'shipped' WHERE id = $1`, units2[0].UnitID)
		r = f.readiness(ful2)
		assert.Contains(t, failing(r), fulfillment.CheckUnitsAndStock)
		f.parity(ful2, false, "inventory_unit_state_conflict")

		_, ful3, _, v3, _ := f.packedSingleVariantOrder("")
		f.exec(`UPDATE inventory_items SET reserved_stock = 0 WHERE product_variant_id = $1`, v3.VariantID)
		r = f.readiness(ful3)
		assert.Contains(t, failing(r), fulfillment.CheckUnitsAndStock)
		assert.Contains(t, check(t, r, fulfillment.CheckUnitsAndStock).Detail, "недостаточно остатка или резерва")
		f.parity(ful3, false, "insufficient_reserved_stock")
	})

	t.Run("E: recipient, phone and address are required — blocked in the checklist AND in the dispatch itself", func(t *testing.T) {
		for _, c := range []struct{ col, check string }{{"customer_name", fulfillment.CheckRecipient}, {"customer_phone", fulfillment.CheckPhone}, {"delivery_address", fulfillment.CheckAddress}} {
			order, ful, _, v, _ := f.packedSingleVariantOrder("")
			f.exec(`UPDATE orders SET `+c.col+` = '   ' WHERE id = $1`, order) // blank counts as missing
			r := f.readiness(ful)
			assert.Equal(t, []string{c.check}, failing(r), c.col)
			assert.NotEmpty(t, check(t, r, c.check).Detail)
			before := f.stock(v.VariantID)
			f.parity(ful, false, "dispatch_delivery_data_missing")
			assert.Equal(t, before, f.stock(v.VariantID), "nothing was written off")
			assert.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, ful))
			assert.Equal(t, 0, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, ful))
			// fixed: it dispatches
			f.exec(`UPDATE orders SET `+c.col+` = 'значение' WHERE id = $1`, order)
			f.parity(ful, true, "")
		}
	})

	t.Run("F + L: no postal code, no comment, no delivery method — nothing blocks (old orders); only a warning for the method", func(t *testing.T) {
		order, ful, _, _, _ := f.packedSingleVariantOrder("")
		f.exec(`UPDATE orders SET delivery_postal_code = NULL, delivery_courier_comment = NULL, delivery_method_name = NULL WHERE id = $1`, order)
		r := f.readiness(ful)
		assert.True(t, r.Ready)
		assert.Equal(t, 1, r.WarningCount)
		m := check(t, r, fulfillment.CheckDeliveryMethod)
		assert.False(t, m.OK)
		assert.Equal(t, fulfillment.SeverityWarning, m.Severity)
		for _, code := range []string{fulfillment.CheckPostalCode, fulfillment.CheckCourierComment, fulfillment.CheckCarrierAtDispatch} {
			c := check(t, r, code)
			assert.Equal(t, fulfillment.SeverityInfo, c.Severity, code)
			assert.False(t, c.OK)
			assert.NotEmpty(t, c.Detail)
		}
		f.parity(ful, true, "")
		// postal code and comment present: info says so, still ready
		order2, ful2, _, _, _ := f.packedSingleVariantOrder("")
		f.exec(`UPDATE orders SET delivery_postal_code = '101000', delivery_courier_comment = 'Домофон' WHERE id = $1`, order2)
		r2 := f.readiness(ful2)
		assert.True(t, check(t, r2, fulfillment.CheckPostalCode).OK)
		assert.True(t, check(t, r2, fulfillment.CheckCourierComment).OK)
	})

	t.Run("I: the state changes between the context and the dispatch — the dispatch decides on fresh data", func(t *testing.T) {
		order, ful, _, v, _ := f.packedSingleVariantOrder("")
		loaded := f.readiness(ful)
		require.True(t, loaded.Ready) // what the screen showed
		f.exec(`UPDATE orders SET customer_phone = '' WHERE id = $1`, order)
		rec := f.dispatchHTTP(ful, &f.adminID)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "dispatch_delivery_data_missing", errorCode(t, rec))
		fresh := f.readiness(ful)
		assert.False(t, fresh.Ready)
		assert.Equal(t, []string{fulfillment.CheckPhone}, failing(fresh))
		assert.Equal(t, stockSnap{5, 0}, stockSnap{f.stock(v.VariantID).total, 0}, "nothing was shipped")
		// the order is cancelled meanwhile
		order2, ful2, _, _, _ := f.packedSingleVariantOrder("")
		require.True(t, f.readiness(ful2).Ready)
		f.exec(`UPDATE orders SET status = 'cancelled' WHERE id = $1`, order2)
		rec = f.dispatchHTTP(ful2, &f.adminID)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "dispatch_not_allowed", errorCode(t, rec))
		assert.Contains(t, failing(f.readiness(ful2)), fulfillment.CheckOrderState)
		assert.Contains(t, check(t, f.readiness(ful2), fulfillment.CheckOrderState).Detail, "отменён")
	})

	t.Run("J: parallel dispatches of one parcel — exactly one shipment, one write-off", func(t *testing.T) {
		_, ful, _, v, _ := f.packedSingleVariantOrder("")
		before := f.stock(v.VariantID)
		const n = 6
		codes := make([]int, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				codes[i] = f.dispatchHTTP(ful, &f.adminID).Code
			}(i)
		}
		close(start)
		wg.Wait()
		ok, conflict := 0, 0
		for _, c := range codes {
			switch c {
			case http.StatusOK:
				ok++
			case http.StatusConflict:
				conflict++
			default:
				t.Fatalf("unexpected %v", codes)
			}
		}
		assert.Equal(t, 1, ok, "%v", codes)
		assert.Equal(t, n-1, conflict)
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, ful))
		assert.Equal(t, before.total-2, f.stock(v.VariantID).total, "written off exactly once (2 units)")
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = (SELECT id FROM shipments WHERE fulfillment_id = $1) AND to_status = 'shipped'`, ful))
	})

	t.Run("the context carries the checklist, with no personal data in it", func(t *testing.T) {
		order, ful, _, _, _ := f.packedSingleVariantOrder("")
		f.exec(`UPDATE orders SET customer_name = 'Секретный Получатель', customer_phone = '+7 999 777-66-55', delivery_address = 'г. Тайноград, ул. Секретная, 7', delivery_postal_code = '999999', delivery_courier_comment = 'секретный комментарий' WHERE id = $1`, order)
		rec := f.contextHTTP(ful)
		require.Equal(t, http.StatusOK, rec.Code)
		var body struct {
			Readiness json.RawMessage `json:"readiness"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.NotEmpty(t, body.Readiness)
		for _, leak := range []string{"Секретный", "777-66-55", "Тайноград", "999999", "секретный"} {
			assert.False(t, strings.Contains(string(body.Readiness), leak), "readiness leaks %q", leak)
		}
		var r fulfillment.DispatchReadiness
		require.NoError(t, json.Unmarshal(body.Readiness, &r))
		assert.True(t, r.Ready)
		assert.Len(t, r.Checks, 14)
		assert.Equal(t, http.StatusNotFound, func() int { rec := f.contextHTTP(uuid.New()); return rec.Code }())
	})
}

// H: two parcels of one order have their own readiness, and each is judged on its own data.
func TestDispatchChecklist_TwoParcels(t *testing.T) {
	f := newFBOFixture(t)
	ctx := context.Background()
	order, ful1, item1, _, units1 := f.packedSingleVariantOrder("awaiting_payment")
	seller2 := uuid.New()
	f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1,'Second Seller',$2,$3,'active',now(),now())`, seller2, "s2-"+seller2.String()[:8], "s2-"+seller2.String()[:8]+"@ex.com")
	first := f.sellerID
	f.sellerID = seller2
	prodID, vs := f.createProductWithVariants([]testVariantInfo{{Size: "M", Color: "Олива", Price: 300000, Barcode: uniqueBarcode()}}, 5)
	v2 := vs[0]
	u2ID, u2Code := f.createSerializedUnit(v2.VariantID)
	f.sellerID = first
	ful2, item2, res2 := uuid.New(), uuid.New(), uuid.New()
	f.exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1,$2,$3,'awaiting_payment',300000,1500,255000,now() + interval '1 second',now())`, ful2, order, seller2)
	f.exec(`INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, variant_size, variant_color, sku, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,'Футболка','tee',300000,1,300000,'M','Олива',$7,now())`, item2, order, ful2, prodID, v2.VariantID, seller2, v2.SKU)
	f.exec(`INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at)
		SELECT $1, id, $2, $3, $4, 1, 'active', now() + interval '30 minutes', $5, now() FROM inventory_items WHERE product_variant_id = $3`, res2, prodID, v2.VariantID, f.customerID, order)
	f.exec(`INSERT INTO order_reservations (id, order_id, reservation_id, created_at) VALUES (gen_random_uuid(), $1, $2, now())`, order, res2)
	f.exec(`UPDATE inventory_items SET reserved_stock = reserved_stock + 1 WHERE product_variant_id = $1`, v2.VariantID)
	f.exec(`INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, created_at) VALUES (gen_random_uuid(), $1, $2, $3, now())`, item2, u2ID, res2)
	f.payViaInventory(order, ful1)
	f.exec(`UPDATE order_fulfillments SET status = 'paid', updated_at = now() WHERE id = $1`, ful2)
	f.pickAll(ful1, item1, units1)
	_, err := f.svc.PackFulfillment(ctx, f.adminID, ful1)
	require.NoError(t, err)

	t.Run("parcel 1 is ready, parcel 2 (not picked) is not", func(t *testing.T) {
		r1, r2 := f.readiness(ful1), f.readiness(ful2)
		assert.True(t, r1.Ready)
		assert.False(t, r2.Ready)
		assert.Contains(t, failing(r2), fulfillment.CheckPickingComplete)
		assert.Contains(t, failing(r2), fulfillment.CheckPackingComplete)
		// the recipient data of both parcels is one snapshot of the order
		for _, id := range []uuid.UUID{ful1, ful2} {
			var d struct {
				OrderID                    string `json:"orderId"`
				ParcelNumber, ParcelsTotal int
			}
			rec := f.contextHTTP(id)
			require.Equal(t, http.StatusOK, rec.Code)
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
			assert.Equal(t, order.String(), d.OrderID)
			assert.Equal(t, 2, d.ParcelsTotal)
		}
	})

	t.Run("dispatching parcel 1 does not make parcel 2 ready or change its checklist", func(t *testing.T) {
		before := f.readiness(ful2)
		f.parity(ful1, true, "")
		assert.JSONEq(t, mustJSON(before), mustJSON(f.readiness(ful2)), "parcel 2 keeps its own state")
		assert.False(t, f.readiness(ful1).Ready)
		rec := f.dispatchHTTP(ful2, &f.adminID)
		assert.Equal(t, http.StatusConflict, rec.Code, "parcel 2 is still refused")
		assert.Equal(t, "paid", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, ful2))
	})

	t.Run("then parcel 2 is picked and packed: its own checklist turns ready and it dispatches", func(t *testing.T) {
		f.pickAll(ful2, item2, []unitRef{{u2ID, u2Code}})
		assert.Equal(t, []string{fulfillment.CheckPackingComplete}, failing(f.readiness(ful2)))
		_, err := f.svc.PackFulfillment(ctx, f.adminID, ful2)
		require.NoError(t, err)
		f.parity(ful2, true, "")
	})
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
