package fulfillment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// DELIVERY.E2E.1 — shipped order -> delivered, on an isolated disposable database (newFBOFixture).
// Delivery is confirmed only by an authorised admin (POST /admin/shipments/{id}/deliver, shipments.update_status);
// there is no carrier integration or webhook. The real payouts service is wired in: on delivery it writes frozen
// ledger rows (DB only, no money moves), which is checked to happen exactly once and only on delivery.

type deliveryKit struct {
	svc     *fulfillment.Service
	handler *fulfillment.Handler
}

func (f *fboFixture) deliveryKit() deliveryKit {
	f.t.Helper()
	payoutSvc := payouts.NewService(payouts.NewRepository(f.db), f.pgClient, returns.NewRepository(f.db), f.ordersRepo, nil, nil)
	svc := fulfillment.NewService(fulfillment.NewRepository(f.db), f.ordersRepo, f.pgClient, payoutSvc, f.notifSvc, nil)
	return deliveryKit{svc: svc, handler: fulfillment.NewHandler(svc)}
}

func (k deliveryKit) deliverHTTP(t *testing.T, shipmentID uuid.UUID, actor *uuid.UUID, comment string) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if comment != "" {
		body, _ = json.Marshal(map[string]string{"comment": comment})
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", shipmentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if actor != nil {
		ctx = context.WithValue(ctx, "userID", *actor)
	}
	rec := httptest.NewRecorder()
	k.handler.DeliverShipment(rec, req.WithContext(ctx))
	return rec
}

// shippedSingleVariantOrder: packedSingleVariantOrder + dispatch through the real handler.
func (f *fboFixture) shippedSingleVariantOrder() (orderID, fulfillmentID, shipmentID uuid.UUID, v testVariantInfo, units []unitRef) {
	f.t.Helper()
	orderID, fulfillmentID, _, v, units = f.packedSingleVariantOrder("")
	rec := f.dispatchHTTP(fulfillmentID, &f.adminID)
	require.Equal(f.t, http.StatusOK, rec.Code, rec.Body.String())
	var res fulfillment.DispatchResult
	require.NoError(f.t, json.Unmarshal(rec.Body.Bytes(), &res))
	return orderID, fulfillmentID, res.ShipmentID, v, units
}

func (f *fboFixture) ledgerRows(fulfillmentID uuid.UUID) int {
	return f.count(`SELECT count(*) FROM seller_ledger_entries sle JOIN order_items oi ON oi.id = sle.order_item_id WHERE oi.order_fulfillment_id = $1`, fulfillmentID)
}

// ============================================================================
// 1. MAIN SCENARIO
// ============================================================================

func TestDelivery_E2E1_ShippedToDelivered(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)
	k := f.deliveryKit()

	orderID, fulID, shipID, v, units := f.shippedSingleVariantOrder()
	stockAfterDispatch := f.stock(v.VariantID)
	require.Equal(t, stockSnap{3, 0}, stockAfterDispatch)

	// 1–2. one shipment, linked to the right order and fulfillment
	var sOrder, sFul uuid.UUID
	var shippedAt time.Time
	require.NoError(t, f.db.QueryRow(ctx, `SELECT order_id, fulfillment_id, shipped_at FROM shipments WHERE id = $1 AND status = 'shipped'`, shipID).Scan(&sOrder, &sFul, &shippedAt))
	assert.Equal(t, orderID, sOrder)
	assert.Equal(t, fulID, sFul)
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE order_id = $1`, orderID))

	// 3–4. buyer sees "shipped"; no carrier / tracking because none exist yet
	ful, err := k.svc.CustomerGetOrderFulfillments(ctx, f.customerID, orderID)
	require.NoError(t, err)
	require.Len(t, ful, 1)
	assert.Equal(t, "shipped", ful[0].Status)
	assert.Nil(t, ful[0].Carrier)
	assert.Nil(t, ful[0].TrackingNumber)
	assert.Nil(t, ful[0].DeliveredAt)

	// staff records carrier / tracking without changing the status (internal data, no carrier call)
	carrier, track := "Курьер ZAMK", "ZMK-TRK-0001"
	require.NoError(t, k.svc.UpdateShipmentStatus(ctx, f.adminID, shipID, fulfillment.UpdateShipmentStatusRequest{Status: "shipped", Carrier: &carrier, TrackingNumber: &track}))
	ful, err = k.svc.CustomerGetOrderFulfillments(ctx, f.customerID, orderID)
	require.NoError(t, err)
	require.NotNil(t, ful[0].Carrier)
	assert.Equal(t, carrier, *ful[0].Carrier)
	assert.Equal(t, track, *ful[0].TrackingNumber)
	assert.Equal(t, "shipped", ful[0].Status)

	// nothing financial before delivery
	assert.Equal(t, 0, f.ledgerRows(fulID))

	// 5. delivery confirmed through the authorised admin handler
	before := time.Now().UTC().Add(-time.Second)
	rec := k.deliverHTTP(t, shipID, &f.adminID, "Вручено покупателю лично")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var res fulfillment.DeliveryResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.Equal(t, "delivered", res.ShipmentStatus)
	assert.Equal(t, "delivered", res.FulfillmentStatus)
	assert.Equal(t, "delivered", res.OrderStatus)

	// 6–9. shipment / fulfillment / order and the delivery date
	var status string
	var deliveredAt time.Time
	var shippedAfter time.Time
	var carrierAfter *string
	require.NoError(t, f.db.QueryRow(ctx, `SELECT status, delivered_at, shipped_at, carrier FROM shipments WHERE id = $1`, shipID).Scan(&status, &deliveredAt, &shippedAfter, &carrierAfter))
	assert.Equal(t, "delivered", status)
	assert.True(t, !deliveredAt.Before(before) && deliveredAt.Before(time.Now().UTC().Add(time.Second)), "delivered_at = time of confirmation")
	assert.True(t, shippedAfter.Equal(shippedAt), "shipped_at is preserved")
	require.NotNil(t, carrierAfter)
	assert.Equal(t, carrier, *carrierAfter)
	assert.Equal(t, "delivered", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
	assert.Equal(t, "delivered", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))

	// 10. exactly one history step and one shipment event
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND from_status = 'shipped' AND to_status = 'delivered' AND actor_user_id = $2`, orderID, f.adminID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND from_status = 'shipped' AND to_status = 'delivered' AND comment = 'Вручено покупателю лично'`, shipID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = $2 AND entity_id = $3`, f.customerID, notifications.TypeCustomerShipmentDelivered, shipID))

	// 11. buyer sees the completed delivery
	ful, err = k.svc.CustomerGetOrderFulfillments(ctx, f.customerID, orderID)
	require.NoError(t, err)
	assert.Equal(t, "delivered", ful[0].Status)
	require.NotNil(t, ful[0].DeliveredAt)
	myOrders, err := f.ordersRepo.ListCustomerOrders(ctx, f.customerID, 50, 0)
	require.NoError(t, err)
	found := ""
	for _, o := range myOrders {
		if o.ID == orderID {
			found = o.Status
		}
	}
	assert.Equal(t, "delivered", found)
	sh, err := k.svc.GetCustomerShipment(ctx, f.customerID, orderID)
	require.NoError(t, err)
	assert.Equal(t, "delivered", sh.Status)

	// 12. no second stock write-off; units stay shipped
	assert.Equal(t, stockAfterDispatch, f.stock(v.VariantID))
	for _, u := range units {
		assert.Equal(t, "shipped", f.str(`SELECT status FROM inventory_units WHERE id = $1`, u.UnitID))
	}

	// finance (existing behaviour): accrual happens on delivery, frozen for 14 days from delivered_at, once
	assert.Equal(t, 3, f.ledgerRows(fulID), "sale_gross + zamk_commission + seller_earning for the one order item")
	var availableAt time.Time
	require.NoError(t, f.db.QueryRow(ctx, `SELECT sle.available_at FROM seller_ledger_entries sle JOIN order_items oi ON oi.id = sle.order_item_id
		WHERE oi.order_fulfillment_id = $1 AND sle.type = 'seller_earning' AND sle.payout_batch_id IS NULL`, fulID).Scan(&availableAt))
	assert.WithinDuration(t, deliveredAt.AddDate(0, 0, 14), availableAt, time.Second)
}

// ============================================================================
// 2. NEGATIVE SCENARIOS
// ============================================================================

func TestDelivery_E2E1_NegativeScenarios(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)
	k := f.deliveryKit()

	t.Run("repeat after a lost response / late repeated event: 409, nothing duplicated", func(t *testing.T) {
		orderID, fulID, shipID, v, _ := f.shippedSingleVariantOrder()
		require.Equal(t, http.StatusOK, k.deliverHTTP(t, shipID, &f.adminID, "").Code)
		deliveredAt := f.str(`SELECT delivered_at::text FROM shipments WHERE id = $1`, shipID)
		stock := f.stock(v.VariantID)

		for i := 0; i < 2; i++ {
			rec := k.deliverHTTP(t, shipID, &f.adminID, "повтор")
			require.Equal(t, http.StatusConflict, rec.Code)
			assert.Equal(t, "shipment_already_delivered", errorCode(t, rec))
		}
		assert.Equal(t, deliveredAt, f.str(`SELECT delivered_at::text FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'delivered'`, shipID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'delivered'`, orderID))
		assert.Equal(t, 3, f.ledgerRows(fulID))
		assert.Equal(t, stock, f.stock(v.VariantID))
		// the generic status endpoint cannot touch a delivered shipment either
		err := k.svc.UpdateShipmentStatus(ctx, f.adminID, shipID, fulfillment.UpdateShipmentStatusRequest{Status: "failed"})
		assert.ErrorIs(t, err, fulfillment.ErrShipmentDeliveredImmutable)
	})

	t.Run("concurrent confirmations: exactly one wins", func(t *testing.T) {
		orderID, fulID, shipID, _, _ := f.shippedSingleVariantOrder()
		const n = 8
		var wg sync.WaitGroup
		errs := make([]error, n)
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, errs[i] = k.svc.DeliverShipment(ctx, f.adminID, shipID, fulfillment.DeliverShipmentRequest{})
			}(i)
		}
		close(start)
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else {
				assert.ErrorIs(t, err, fulfillment.ErrShipmentAlreadyDelivered)
			}
		}
		assert.Equal(t, 1, ok)
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'delivered'`, shipID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'delivered'`, orderID))
		assert.Equal(t, 3, f.ledgerRows(fulID))
	})

	t.Run("not shipped yet (packed, pre-dispatch shipment): 409, nothing changes", func(t *testing.T) {
		orderID, fulID, _, v, _ := f.packedSingleVariantOrder("")
		sh, err := k.svc.CreateShipmentForFulfillment(ctx, f.adminID, fulID, fulfillment.CreateShipmentRequest{})
		require.NoError(t, err)
		require.NotEqual(t, "shipped", sh.Status)
		stock := f.stock(v.VariantID)
		rec := k.deliverHTTP(t, sh.ID, &f.adminID, "")
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "delivery_not_allowed", errorCode(t, rec))
		assert.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		assert.Equal(t, "packed", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM shipments WHERE id = $1 AND delivered_at IS NOT NULL`, sh.ID))
		assert.Equal(t, 0, f.ledgerRows(fulID))
		assert.Equal(t, stock, f.stock(v.VariantID))
	})

	t.Run("unknown shipment: 404; another customer cannot read the shipment", func(t *testing.T) {
		rec := k.deliverHTTP(t, uuid.New(), &f.adminID, "")
		require.Equal(t, http.StatusNotFound, rec.Code)
		orderID, _, _, _, _ := f.shippedSingleVariantOrder()
		_, err := k.svc.GetCustomerShipment(ctx, uuid.New(), orderID)
		assert.Error(t, err)
		_, err = k.svc.CustomerGetOrderFulfillments(ctx, uuid.New(), orderID)
		assert.Error(t, err)
	})

	t.Run("wrong sequence: generic endpoint cannot deliver; a failed shipment cannot be delivered", func(t *testing.T) {
		orderID, fulID, shipID, _, _ := f.shippedSingleVariantOrder()
		err := k.svc.UpdateShipmentStatus(ctx, f.adminID, shipID, fulfillment.UpdateShipmentStatusRequest{Status: "delivered"})
		assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)
		assert.Equal(t, "shipped", f.str(`SELECT status FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, 0, f.ledgerRows(fulID))

		// a shipment already 'failed' in the data (e.g. legacy row) still cannot be delivered
		f.exec(`UPDATE shipments SET status = 'failed' WHERE id = $1`, shipID)
		rec := k.deliverHTTP(t, shipID, &f.adminID, "")
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "delivery_not_allowed", errorCode(t, rec))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 0, f.ledgerRows(fulID))
	})

	t.Run("after dispatch the generic endpoint cannot move a shipment to failed / cancelled / earlier statuses", func(t *testing.T) {
		orderID, fulID, shipID, v, units := f.shippedSingleVariantOrder()
		stock := f.stock(v.VariantID)
		for _, to := range []string{"failed", "cancelled", "packed", "assembling", "pending"} {
			err := k.svc.UpdateShipmentStatus(ctx, f.adminID, shipID, fulfillment.UpdateShipmentStatusRequest{Status: to})
			assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed, to)
		}
		assert.Equal(t, "shipped", f.str(`SELECT status FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status <> 'shipped'`, shipID))
		assert.Equal(t, stock, f.stock(v.VariantID))
		for _, u := range units {
			assert.Equal(t, "shipped", f.str(`SELECT status FROM inventory_units WHERE id = $1`, u.UnitID))
		}
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM reservations WHERE order_id = $1 AND status = 'converted'`, orderID), "the one reservation stays converted")
		assert.Equal(t, 0, f.ledgerRows(fulID))

		// the HTTP contract: existing 400 for a refused status change
		body, _ := json.Marshal(map[string]string{"status": "failed"})
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/shipments/"+shipID.String()+"/status", bytes.NewReader(body))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", shipID.String())
		rec := httptest.NewRecorder()
		k.handler.UpdateShipmentStatus(rec, req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), "userID", f.adminID)))
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

		// tracking metadata may still be recorded, without any status transition
		carrier, track := "Курьер ZAMK", "ZMK-TRK-0002"
		require.NoError(t, k.svc.UpdateShipmentStatus(ctx, f.adminID, shipID, fulfillment.UpdateShipmentStatusRequest{Status: "shipped", Carrier: &carrier, TrackingNumber: &track}))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, track, f.str(`SELECT tracking_number FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = $1`, shipID), "metadata update writes no status event")
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status <> 'paid' AND to_status <> 'assembling' AND to_status <> 'packed' AND to_status <> 'shipped'`, orderID))

		// the dedicated /deliver still works, once
		require.Equal(t, http.StatusOK, k.deliverHTTP(t, shipID, &f.adminID, "").Code)
		require.Equal(t, http.StatusConflict, k.deliverHTTP(t, shipID, &f.adminID, "").Code)
		assert.Equal(t, "delivered", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 3, f.ledgerRows(fulID), "repeat delivery does not accrue twice")
		assert.Equal(t, stock, f.stock(v.VariantID))

		// after delivery the generic endpoint changes nothing
		for _, to := range []string{"packed", "failed", "cancelled", "shipped"} {
			err := k.svc.UpdateShipmentStatus(ctx, f.adminID, shipID, fulfillment.UpdateShipmentStatusRequest{Status: to})
			assert.ErrorIs(t, err, fulfillment.ErrShipmentDeliveredImmutable, to)
		}
		err := k.svc.UpdateShipmentStatus(ctx, f.adminID, shipID, fulfillment.UpdateShipmentStatusRequest{Status: "delivered", TrackingNumber: &track})
		assert.ErrorIs(t, err, fulfillment.ErrShipmentDeliveredImmutable)
		assert.Equal(t, "delivered", f.str(`SELECT status FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, "delivered", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		assert.Equal(t, "delivered", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 3, f.ledgerRows(fulID))
		assert.Equal(t, stock, f.stock(v.VariantID))
	})

	t.Run("shipped shipment cannot be moved back to a pre-dispatch status (no second write-off)", func(t *testing.T) {
		// legacy (barcode) item: dispatch has no physical unit state to stop a second write-off
		prodID, vs := f.createProductWithVariants([]testVariantInfo{{Size: "XS", Color: "Песочный", Barcode: uniqueBarcode()}}, 5)
		v := vs[0]
		orderID, fulID, itemIDs := f.createOrderWithFulfillment("awaiting_payment", "awaiting_payment", prodID, []orderItemSpec{{VariantInfo: v, Quantity: 1}})
		f.payViaInventory(orderID, fulID)
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulID, v.Barcode, &itemIDs[0])
		require.NoError(t, err)
		_, err = f.svc.PackFulfillment(ctx, f.adminID, fulID)
		require.NoError(t, err)
		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var dres fulfillment.DispatchResult
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dres))
		require.Equal(t, stockSnap{4, 0}, f.stock(v.VariantID))

		for _, back := range []string{"packed", "assembling", "pending"} {
			err := k.svc.UpdateShipmentStatus(ctx, f.adminID, dres.ShipmentID, fulfillment.UpdateShipmentStatusRequest{Status: back})
			assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed, back)
		}
		assert.Equal(t, "shipped", f.str(`SELECT status FROM shipments WHERE id = $1`, dres.ShipmentID))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))

		_, err = f.svc.DispatchFulfillment(ctx, f.adminID, fulID)
		assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)
		assert.Equal(t, stockSnap{4, 0}, f.stock(v.VariantID), "stock is written off exactly once")
	})

	t.Run("no authenticated operator: 401, nothing changes", func(t *testing.T) {
		_, fulID, shipID, _, _ := f.shippedSingleVariantOrder()
		rec := k.deliverHTTP(t, shipID, nil, "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "shipped", f.str(`SELECT status FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, 0, f.ledgerRows(fulID))
	})

	t.Run("order with two fulfillments: delivered only when both are", func(t *testing.T) {
		// first parcel (seller 1): order created, not yet paid
		orderID, ful1, item1, _, units1 := f.packedSingleVariantOrder("awaiting_payment")
		// second parcel of the same order from a second seller (one fulfillment per seller and order)
		seller2 := uuid.New()
		f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Second Seller', $2, $3, 'active', now(), now())`, seller2, "s2-"+seller2.String()[:8], "s2-"+seller2.String()[:8]+"@ex.com")
		firstSeller := f.sellerID
		f.sellerID = seller2
		prodID, vs := f.createProductWithVariants([]testVariantInfo{{Size: "M", Color: "Олива", Price: 300000, Barcode: uniqueBarcode()}}, 5)
		v2 := vs[0]
		u2ID, u2Code := f.createSerializedUnit(v2.VariantID)
		f.sellerID = firstSeller
		ful2, item2, res2 := uuid.New(), uuid.New(), uuid.New()
		f.exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'awaiting_payment', 300000, 1500, 255000, now(), now())`, ful2, orderID, seller2)
		f.exec(`INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, variant_size, variant_color, sku, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'Футболка', 'tee', 300000, 1, 300000, 'M', 'Олива', $7, now())`, item2, orderID, ful2, prodID, v2.VariantID, seller2, v2.SKU)
		f.exec(`INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at)
			SELECT $1, id, $2, $3, $4, 1, 'active', now() + interval '30 minutes', $5, now() FROM inventory_items WHERE product_variant_id = $3`, res2, prodID, v2.VariantID, f.customerID, orderID)
		f.exec(`INSERT INTO order_reservations (id, order_id, reservation_id, created_at) VALUES (gen_random_uuid(), $1, $2, now())`, orderID, res2)
		f.exec(`UPDATE inventory_items SET reserved_stock = reserved_stock + 1 WHERE product_variant_id = $1`, v2.VariantID)
		f.exec(`INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, created_at) VALUES (gen_random_uuid(), $1, $2, $3, now())`, item2, u2ID, res2)

		// payment covers both parcels; both are picked, packed and dispatched by the real services
		f.payViaInventory(orderID, ful1)
		f.exec(`UPDATE order_fulfillments SET status = 'paid', updated_at = now() WHERE id = $1`, ful2)
		f.pickAll(ful1, item1, units1)
		f.pickAll(ful2, item2, []unitRef{{u2ID, u2Code}})
		for _, id := range []uuid.UUID{ful1, ful2} {
			_, err := f.svc.PackFulfillment(ctx, f.adminID, id)
			require.NoError(t, err)
		}
		shipments := map[uuid.UUID]uuid.UUID{}
		for _, id := range []uuid.UUID{ful1, ful2} {
			rec := f.dispatchHTTP(id, &f.adminID)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var d fulfillment.DispatchResult
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
			shipments[id] = d.ShipmentID
		}
		require.Equal(t, "shipped", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))

		res, err := k.svc.DeliverShipment(ctx, f.adminID, shipments[ful1], fulfillment.DeliverShipmentRequest{})
		require.NoError(t, err)
		assert.Equal(t, "shipped", res.OrderStatus, "one parcel still on the way")
		assert.Equal(t, "delivered", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, ful1))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, ful2))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 3, f.ledgerRows(ful1))
		assert.Equal(t, 0, f.ledgerRows(ful2), "the second parcel is not accrued before its own delivery")
		cust, err := k.svc.CustomerGetOrderFulfillments(ctx, f.customerID, orderID)
		require.NoError(t, err)
		statuses := map[uuid.UUID]string{}
		for _, c := range cust {
			statuses[c.ID] = c.Status
		}
		assert.Equal(t, "delivered", statuses[ful1])
		assert.Equal(t, "shipped", statuses[ful2])

		res, err = k.svc.DeliverShipment(ctx, f.adminID, shipments[ful2], fulfillment.DeliverShipmentRequest{})
		require.NoError(t, err)
		assert.Equal(t, "delivered", res.OrderStatus)
		assert.Equal(t, "delivered", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'delivered'`, orderID))
		assert.Equal(t, 3, f.ledgerRows(ful2))
	})

	t.Run("transaction failure: full rollback, then a clean delivery", func(t *testing.T) {
		orderID, fulID, shipID, _, _ := f.shippedSingleVariantOrder()
		testutil.MustBeDisposableDatabase(t, f.db)
		f.exec(`CREATE FUNCTION delivery_e2e1_fail() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'delivery_e2e1 injected failure'; END $$`)
		f.exec(`CREATE TRIGGER delivery_e2e1_fail BEFORE INSERT ON seller_ledger_entries FOR EACH ROW EXECUTE FUNCTION delivery_e2e1_fail()`)

		_, err := k.svc.DeliverShipment(ctx, f.adminID, shipID, fulfillment.DeliverShipmentRequest{})
		require.Error(t, err)
		assert.False(t, errors.Is(err, fulfillment.ErrShipmentAlreadyDelivered))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM shipments WHERE id = $1`, shipID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM shipments WHERE id = $1 AND delivered_at IS NOT NULL`, shipID))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		assert.Equal(t, "shipped", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'delivered'`, shipID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM notifications WHERE entity_id = $1 AND type = $2`, shipID, notifications.TypeCustomerShipmentDelivered))
		assert.Equal(t, 0, f.ledgerRows(fulID))

		f.exec(`DROP TRIGGER delivery_e2e1_fail ON seller_ledger_entries`)
		f.exec(`DROP FUNCTION delivery_e2e1_fail()`)

		rec := k.deliverHTTP(t, shipID, &f.adminID, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "delivered", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'delivered'`, shipID))
		assert.Equal(t, 3, f.ledgerRows(fulID))
	})
}
