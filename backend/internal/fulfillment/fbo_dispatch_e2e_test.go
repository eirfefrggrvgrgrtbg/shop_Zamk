package fulfillment_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// FBO.E2E.2 — packed order -> warehouse dispatch -> physical stock write-off, on an isolated disposable database
// (newFBOFixture). Dispatch is internal only: no carrier API exists on this path, notifications are in-app rows.

type unitRef = struct {
	UnitID   uuid.UUID
	UnitCode string
}

// payViaInventory moves an awaiting_payment order to paid. Statuses are set as the payment confirmation sets them;
// reservations go through the real inventory.ConvertReservationToSaleTx (converted + one 'sale' stock movement each).
func (f *fboFixture) payViaInventory(orderID, fulfillmentID uuid.UUID) {
	f.t.Helper()
	f.exec(`UPDATE orders SET status = 'paid', updated_at = now() WHERE id = $1`, orderID)
	f.exec(`UPDATE order_fulfillments SET status = 'paid', updated_at = now() WHERE id = $1`, fulfillmentID)
	f.exec(`INSERT INTO order_status_history (id, order_id, from_status, to_status, created_at)
		VALUES (gen_random_uuid(), $1, 'awaiting_payment', 'paid', now())`, orderID)

	invSvc := inventory.NewService(inventory.NewRepository(f.db), sellers.NewRepository(f.db), f.pgClient)
	rows, err := f.db.Query(f.ctx, `SELECT id FROM reservations WHERE order_id = $1 AND status = 'active' ORDER BY id`, orderID)
	require.NoError(f.t, err)
	var resIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		require.NoError(f.t, rows.Scan(&id))
		resIDs = append(resIDs, id)
	}
	rows.Close()
	require.NotEmpty(f.t, resIDs)
	require.NoError(f.t, f.pgClient.RunInTx(f.ctx, func(tx pgx.Tx) error {
		for _, id := range resIDs {
			if err := invSvc.ConvertReservationToSaleTx(f.ctx, tx, id); err != nil {
				return err
			}
		}
		return nil
	}))
}

// pickAll scans every given unit through the real picking service.
func (f *fboFixture) pickAll(fulfillmentID uuid.UUID, itemID uuid.UUID, units []unitRef) {
	f.t.Helper()
	for _, u := range units {
		res, err := f.svc.ScanPickingCode(f.ctx, f.adminID, fulfillmentID, u.UnitCode, &itemID)
		require.NoError(f.t, err)
		require.True(f.t, res.ScanResult.NewlyPicked)
	}
}

// dispatchHTTP calls the real admin dispatch handler, as the router does after the warehouse.dispatch check.
func (f *fboFixture) dispatchHTTP(fulfillmentID uuid.UUID, actor *uuid.UUID) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/fulfillments/"+fulfillmentID.String()+"/dispatch", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fulfillmentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if actor != nil {
		ctx = context.WithValue(ctx, "userID", *actor)
	}
	rec := httptest.NewRecorder()
	f.handler.DispatchFulfillment(rec, req.WithContext(ctx))
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	if e, ok := body["error"].(map[string]any); ok {
		if c, ok := e["code"].(string); ok {
			return c
		}
	}
	if c, ok := body["code"].(string); ok {
		return c
	}
	return rec.Body.String()
}

// uniqueBarcode: product_variants.barcode is globally unique, and the shared fixture numbers barcodes per call.
func uniqueBarcode() string {
	return fmt.Sprintf("29%011d", uint64(uuid.New().ID())%100000000000)
}

type stockSnap struct{ total, reserved int }

func (f *fboFixture) stock(variantID uuid.UUID) stockSnap {
	f.t.Helper()
	var s stockSnap
	require.NoError(f.t, f.db.QueryRow(f.ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&s.total, &s.reserved))
	return s
}

// packedSingleVariantOrder: 1 variant (stock 5), an order for 2 serialized units, paid via inventory, picked and packed
// through the real services unless stopAt says otherwise ("awaiting_payment", "partial", "picked").
func (f *fboFixture) packedSingleVariantOrder(stopAt string) (orderID, fulfillmentID, itemID uuid.UUID, v testVariantInfo, units []unitRef) {
	f.t.Helper()
	prodID, vs := f.createProductWithVariants([]testVariantInfo{{Size: "L", Color: "Молочный", Price: 450000, Barcode: uniqueBarcode()}}, 5)
	v = vs[0]
	for i := 0; i < 2; i++ {
		id, code := f.createSerializedUnit(v.VariantID)
		units = append(units, unitRef{id, code})
	}
	orderID, fulfillmentID, itemIDs := f.createOrderWithFulfillment("awaiting_payment", "awaiting_payment", prodID,
		[]orderItemSpec{{VariantInfo: v, Quantity: 2, SerializedUnits: units}})
	itemID = itemIDs[0]
	if stopAt == "awaiting_payment" {
		return
	}
	f.payViaInventory(orderID, fulfillmentID)
	if stopAt == "partial" {
		f.pickAll(fulfillmentID, itemID, units[:1])
		return
	}
	f.pickAll(fulfillmentID, itemID, units)
	if stopAt == "picked" {
		return
	}
	_, err := f.svc.PackFulfillment(f.ctx, f.adminID, fulfillmentID)
	require.NoError(f.t, err)
	return
}

// ============================================================================
// 1. MAIN SCENARIO
// ============================================================================

func TestFBO_E2E2_PackedOrder_Dispatch_WritesOffStock(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)

	prodID, variants := f.createProductWithVariants([]testVariantInfo{
		{Size: "S", Color: "Черный", Price: 590000},
		{Size: "M", Color: "Графит", Price: 590000},
	}, 10)
	varS, varM := variants[0], variants[1]
	s1ID, s1Code := f.createSerializedUnit(varS.VariantID)
	m1ID, m1Code := f.createSerializedUnit(varM.VariantID)
	m2ID, m2Code := f.createSerializedUnit(varM.VariantID)
	spareID, _ := f.createSerializedUnit(varM.VariantID) // unrelated unit: must stay on the shelf

	orderID, fulfillmentID, itemIDs := f.createOrderWithFulfillment("awaiting_payment", "awaiting_payment", prodID, []orderItemSpec{
		{VariantInfo: varS, Quantity: 1, SerializedUnits: []unitRef{{s1ID, s1Code}}},
		{VariantInfo: varM, Quantity: 2, SerializedUnits: []unitRef{{m1ID, m1Code}, {m2ID, m2Code}}},
	})
	itemS, itemM := itemIDs[0], itemIDs[1]

	// Payment: reservations converted through the inventory service, one 'sale' movement per reservation.
	f.payViaInventory(orderID, fulfillmentID)
	require.Equal(t, 2, f.count(`SELECT count(*) FROM reservations WHERE order_id = $1 AND status = 'converted'`, orderID))
	require.Equal(t, 2, f.count(`SELECT count(*) FROM stock_movements WHERE reference_type = 'order' AND reference_id = $1 AND type = 'sale'`, orderID))

	// Picking and packing through the real services.
	f.pickAll(fulfillmentID, itemS, []unitRef{{s1ID, s1Code}})
	f.pickAll(fulfillmentID, itemM, []unitRef{{m1ID, m1Code}, {m2ID, m2Code}})
	_, err := f.svc.PackFulfillment(ctx, f.adminID, fulfillmentID)
	require.NoError(t, err)

	// 4. packing really finished
	require.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID))
	require.Equal(t, "packed", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
	require.Equal(t, 1, f.count(`SELECT count(*) FROM order_fulfillments WHERE id = $1 AND packed_at IS NOT NULL`, fulfillmentID))
	require.Equal(t, 0, f.count(`SELECT count(*) FROM order_item_allocations a JOIN order_items oi ON oi.id = a.order_item_id
		WHERE oi.order_fulfillment_id = $1 AND a.released_at IS NULL AND a.picked_at IS NULL`, fulfillmentID))

	// Before dispatch: stock held, nothing written off.
	assert.Equal(t, stockSnap{10, 1}, f.stock(varS.VariantID))
	assert.Equal(t, stockSnap{10, 2}, f.stock(varM.VariantID))
	movementsBefore := f.count(`SELECT count(*) FROM stock_movements WHERE product_id = $1`, prodID)

	// 1–2. dispatch queue shows exactly this fulfillment with the right totals
	queue, err := f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)
	var q *fulfillment.DispatchQueueItem
	for i := range queue {
		if queue[i].FulfillmentID == fulfillmentID {
			q = &queue[i]
		}
	}
	require.NotNil(t, q, "packed fulfillment must be in the dispatch queue")
	assert.Equal(t, orderID, q.OrderID)
	assert.Equal(t, "packed", q.Status)
	assert.Equal(t, "packed", q.OrderStatus)
	assert.Equal(t, 2, q.ItemsCount)
	assert.Equal(t, 3, q.TotalQuantity)
	assert.Equal(t, "Nord Atelier FBO", *q.SellerName)
	assert.Nil(t, q.ShipmentStatus)

	// 3. dispatch context: SKU, size, colour, quantity, picked units
	dc, err := f.svc.GetDispatchContext(ctx, fulfillmentID)
	require.NoError(t, err)
	require.Len(t, dc.Items, 2)
	byItem := map[uuid.UUID]fulfillment.DispatchContextItem{}
	for _, it := range dc.Items {
		byItem[it.OrderItemID] = it
	}
	assert.Equal(t, varS.SKU, *byItem[itemS].SKU)
	assert.Equal(t, "S", *byItem[itemS].VariantSize)
	assert.Equal(t, "Черный", *byItem[itemS].VariantColor)
	assert.Equal(t, 1, byItem[itemS].Quantity)
	assert.Equal(t, varM.SKU, *byItem[itemM].SKU)
	assert.Equal(t, "M", *byItem[itemM].VariantSize)
	assert.Equal(t, "Графит", *byItem[itemM].VariantColor)
	assert.Equal(t, 2, byItem[itemM].Quantity)
	require.Len(t, byItem[itemM].AllocatedUnits, 2)
	for _, u := range byItem[itemM].AllocatedUnits {
		assert.NotNil(t, u.PickedAt)
	}

	// 5. dispatch through the real HTTP handler
	rec := f.dispatchHTTP(fulfillmentID, &f.adminID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var res fulfillment.DispatchResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.Equal(t, "shipped", res.FulfillmentStatus)
	assert.Equal(t, "shipped", res.OrderStatus)
	assert.Equal(t, "shipped", res.ShipmentStatus)

	// 6–7. statuses + exactly one history step packed -> shipped
	assert.Equal(t, "shipped", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID))
	assert.Equal(t, "shipped", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND from_status = 'packed' AND to_status = 'shipped' AND actor_user_id = $2`, orderID, f.adminID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1 AND status = 'shipped' AND shipped_at IS NOT NULL AND carrier IS NULL AND tracking_number IS NULL`, fulfillmentID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events e JOIN shipments s ON s.id = e.shipment_id WHERE s.fulfillment_id = $1 AND e.to_status = 'shipped'`, fulfillmentID))

	// 8–9. total and reserved decrease by exactly the shipped quantity
	assert.Equal(t, stockSnap{9, 0}, f.stock(varS.VariantID))
	assert.Equal(t, stockSnap{8, 0}, f.stock(varM.VariantID))
	assert.Equal(t, 0, f.count(`SELECT count(*) FROM inventory_items WHERE product_id = $1 AND (total_stock < 0 OR reserved_stock < 0 OR reserved_stock > total_stock)`, prodID))

	// 10. physical units: the three picked ones shipped, the spare untouched; allocations keep their picked history
	for _, id := range []uuid.UUID{s1ID, m1ID, m2ID} {
		assert.Equal(t, "shipped", f.str(`SELECT status FROM inventory_units WHERE id = $1`, id))
	}
	assert.Equal(t, "warehouse", f.str(`SELECT status FROM inventory_units WHERE id = $1`, spareID))
	assert.Equal(t, 3, f.count(`SELECT count(*) FROM order_item_allocations a JOIN order_items oi ON oi.id = a.order_item_id
		WHERE oi.order_fulfillment_id = $1 AND a.released_at IS NULL AND a.picked_at IS NOT NULL`, fulfillmentID))
	assert.Equal(t, 2, f.count(`SELECT count(*) FROM reservations WHERE order_id = $1 AND status = 'converted'`, orderID))

	// 11. stock movements: the sale per SKU links order and variant; dispatch adds no duplicate movement
	assert.Equal(t, movementsBefore, f.count(`SELECT count(*) FROM stock_movements WHERE product_id = $1`, prodID))
	assert.Equal(t, 1, f.count(`SELECT quantity FROM stock_movements WHERE reference_id = $1 AND type = 'sale' AND product_variant_id = $2`, orderID, varS.VariantID))
	assert.Equal(t, 2, f.count(`SELECT quantity FROM stock_movements WHERE reference_id = $1 AND type = 'sale' AND product_variant_id = $2`, orderID, varM.VariantID))

	// 12. the buyer sees the truthful status ("Мои заказы" + per-parcel tracking); another customer does not
	ful, err := f.svc.CustomerGetOrderFulfillments(ctx, f.customerID, orderID)
	require.NoError(t, err)
	require.Len(t, ful, 1)
	assert.Equal(t, "shipped", ful[0].Status)
	assert.NotNil(t, ful[0].ShippedAt)
	myOrders, err := f.ordersRepo.ListCustomerOrders(ctx, f.customerID, 20, 0)
	require.NoError(t, err)
	var mine string
	for _, o := range myOrders {
		if o.ID == orderID {
			mine = o.Status
		}
	}
	assert.Equal(t, "shipped", mine)
	_, err = f.svc.CustomerGetOrderFulfillments(ctx, uuid.New(), orderID)
	assert.Error(t, err)

	// in-app notification only (no external sender on this path), exactly once
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = $2`, f.customerID, notifications.TypeCustomerShipmentShipped))

	// the dispatched fulfillment leaves the queue
	queue, err = f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)
	for _, it := range queue {
		assert.NotEqual(t, fulfillmentID, it.FulfillmentID)
	}
}

// ============================================================================
// 2. NEGATIVE SCENARIOS
// ============================================================================

func TestFBO_E2E2_Dispatch_NegativeScenarios(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)

	t.Run("repeat after a lost response: 409, no second write-off", func(t *testing.T) {
		orderID, fulID, _, v, _ := f.packedSingleVariantOrder("")
		first := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusOK, first.Code, first.Body.String())
		after := f.stock(v.VariantID)
		require.Equal(t, stockSnap{3, 0}, after)

		retry := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusConflict, retry.Code)
		assert.Equal(t, "dispatch_not_allowed", errorCode(t, retry))
		assert.Equal(t, after, f.stock(v.VariantID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipment_events e JOIN shipments s ON s.id = e.shipment_id WHERE s.fulfillment_id = $1`, fulID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'shipped'`, orderID))
	})

	t.Run("concurrent dispatch: exactly one wins, stock written off once", func(t *testing.T) {
		orderID, fulID, _, v, _ := f.packedSingleVariantOrder("")
		const n = 8
		var wg sync.WaitGroup
		errs := make([]error, n)
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, errs[i] = f.svc.DispatchFulfillment(ctx, f.adminID, fulID)
			}(i)
		}
		close(start)
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else {
				assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)
			}
		}
		assert.Equal(t, 1, ok)
		assert.Equal(t, stockSnap{3, 0}, f.stock(v.VariantID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'shipped'`, orderID))
	})

	t.Run("unpaid order: 409, nothing changes", func(t *testing.T) {
		orderID, fulID, _, v, units := f.packedSingleVariantOrder("awaiting_payment")
		before := f.stock(v.VariantID)
		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "dispatch_not_allowed", errorCode(t, rec))
		assert.Equal(t, before, f.stock(v.VariantID))
		assert.Equal(t, "awaiting_payment", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, "warehouse", f.str(`SELECT status FROM inventory_units WHERE id = $1`, units[0].UnitID))
	})

	t.Run("partially picked order: 409, nothing changes", func(t *testing.T) {
		_, fulID, _, v, units := f.packedSingleVariantOrder("partial")
		before := f.stock(v.VariantID)
		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, before, f.stock(v.VariantID))
		assert.Equal(t, "warehouse", f.str(`SELECT status FROM inventory_units WHERE id = $1`, units[0].UnitID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulID))
	})

	t.Run("fully picked but not packed: 409, nothing changes", func(t *testing.T) {
		_, fulID, _, v, _ := f.packedSingleVariantOrder("picked")
		require.Equal(t, "assembling", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		before := f.stock(v.VariantID)
		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "dispatch_not_allowed", errorCode(t, rec))
		assert.Equal(t, before, f.stock(v.VariantID))
	})

	t.Run("packed status with an unpicked unit (forged state): 409 not fully picked", func(t *testing.T) {
		orderID, fulID, _, v, _ := f.packedSingleVariantOrder("partial")
		f.exec(`UPDATE order_fulfillments SET status = 'packed', packed_at = now() WHERE id = $1`, fulID)
		f.exec(`UPDATE orders SET status = 'packed' WHERE id = $1`, orderID)
		before := f.stock(v.VariantID)
		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "fulfillment_not_fully_picked", errorCode(t, rec))
		assert.Equal(t, before, f.stock(v.VariantID))
	})

	t.Run("allocated unit no longer on the shelf: 409, nothing changes", func(t *testing.T) {
		_, fulID, _, v, units := f.packedSingleVariantOrder("")
		f.exec(`UPDATE inventory_units SET status = 'damaged' WHERE id = $1`, units[1].UnitID)
		before := f.stock(v.VariantID)
		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "inventory_unit_state_conflict", errorCode(t, rec))
		assert.Equal(t, before, f.stock(v.VariantID))
		assert.Equal(t, "warehouse", f.str(`SELECT status FROM inventory_units WHERE id = $1`, units[0].UnitID))
	})

	t.Run("allocated unit of another SKU: rejected as invariant violation, nothing changes", func(t *testing.T) {
		_, fulID, _, v, units := f.packedSingleVariantOrder("")
		_, other := f.createProductWithVariants([]testVariantInfo{{Size: "XL", Color: "Синий", Barcode: uniqueBarcode()}}, 5)
		f.exec(`UPDATE inventory_units SET product_variant_id = $1 WHERE id = $2`, other[0].VariantID, units[1].UnitID)
		before := f.stock(v.VariantID)
		otherBefore := f.stock(other[0].VariantID)
		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "invariant_violation", errorCode(t, rec))
		assert.Equal(t, before, f.stock(v.VariantID))
		assert.Equal(t, otherBefore, f.stock(other[0].VariantID))
	})

	t.Run("no authenticated operator: 401, nothing changes", func(t *testing.T) {
		_, fulID, _, v, _ := f.packedSingleVariantOrder("")
		before := f.stock(v.VariantID)
		rec := f.dispatchHTTP(fulID, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, before, f.stock(v.VariantID))
		assert.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
	})

	t.Run("database error in the middle of the transaction: full rollback, then a clean dispatch", func(t *testing.T) {
		orderID, fulID, _, v, units := f.packedSingleVariantOrder("")
		before := f.stock(v.VariantID)
		testutil.MustBeDisposableDatabase(t, f.db)
		// Fails the final fulfillment update, i.e. after stock, units and shipment were already written in the tx.
		f.exec(`CREATE FUNCTION fbo_e2e2_fail_dispatch() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'fbo_e2e2 injected failure'; END $$`)
		f.exec(`CREATE TRIGGER fbo_e2e2_fail BEFORE UPDATE ON order_fulfillments FOR EACH ROW
			WHEN (NEW.status = 'shipped' AND NEW.id = '` + fulID.String() + `') EXECUTE FUNCTION fbo_e2e2_fail_dispatch()`)

		_, err := f.svc.DispatchFulfillment(ctx, f.adminID, fulID)
		require.Error(t, err)
		assert.False(t, errors.Is(err, fulfillment.ErrDispatchNotAllowed))
		assert.Equal(t, before, f.stock(v.VariantID))
		for _, u := range units {
			assert.Equal(t, "warehouse", f.str(`SELECT status FROM inventory_units WHERE id = $1`, u.UnitID))
		}
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulID))
		assert.Equal(t, "packed", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fulID))
		assert.Equal(t, "packed", f.str(`SELECT status FROM orders WHERE id = $1`, orderID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM notifications WHERE entity_type = 'shipment' AND recipient_user_id = $1 AND created_at > now() - interval '1 minute' AND entity_id IN (SELECT id FROM shipments WHERE fulfillment_id = $2)`, f.customerID, fulID))

		f.exec(`DROP TRIGGER fbo_e2e2_fail ON order_fulfillments`)
		f.exec(`DROP FUNCTION fbo_e2e2_fail_dispatch()`)

		rec := f.dispatchHTTP(fulID, &f.adminID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, stockSnap{before.total - 2, before.reserved - 2}, f.stock(v.VariantID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE fulfillment_id = $1`, fulID))
	})
}
