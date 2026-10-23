package fulfillment_test

import (
	"context"
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

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// RETURN.E2E.1 — delivered order -> customer return -> reverse logistics -> warehouse receiving and inspection ->
// stock -> refund (pending until the provider confirms) -> seller ledger adjustment. Isolated disposable database.
// No external calls: no logistics provider (reverse shipments via the built-in simulator), the payments service has no
// provider (refunds are only reserved), and provider confirmation is the existing simulate-refund path.

type returnsKit struct {
	svc     *returns.Service
	handler *returns.Handler
}

func (f *fboFixture) returnsKit() returnsKit {
	f.t.Helper()
	returnsRepo := returns.NewRepository(f.db)
	invSvc := inventory.NewService(inventory.NewRepository(f.db), sellers.NewRepository(f.db), f.pgClient)
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	paySvc := payments.NewService(payments.NewRepository(f.db), f.ordersRepo, nil, nil, f.pgClient, nil, nil, cfg)
	payoutSvc := payouts.NewService(payouts.NewRepository(f.db), f.pgClient, returnsRepo, f.ordersRepo, cfg, nil)
	svc := returns.NewService(returnsRepo, f.ordersRepo, invSvc, f.pgClient, payoutSvc, paySvc, 14, f.notifSvc, nil, nil, nil) // no logistics provider: reverse shipments only via the built-in simulator
	return returnsKit{svc: svc, handler: returns.NewHandler(svc, "test")}
}

type deliveredOrder struct {
	orderID, fulfillmentID, shipmentID uuid.UUID
	items                              map[string]uuid.UUID // SKU size -> order item
	variants                           map[string]testVariantInfo
	units                              map[string][]unitRef
}

// deliveredTwoSkuOrder: S×1 + M×2 (serialized), paid via the inventory service with a succeeded payment row,
// picked, packed, dispatched and delivered through the real services (delivery accrues the seller ledger).
func (f *fboFixture) deliveredTwoSkuOrder() deliveredOrder {
	f.t.Helper()
	ctx := f.ctx
	prodID, vs := f.createProductWithVariants([]testVariantInfo{
		{Size: "S", Color: "Черный", Price: 590000, Barcode: uniqueBarcode()},
		{Size: "M", Color: "Графит", Price: 590000, Barcode: uniqueBarcode()},
	}, 10)
	d := deliveredOrder{items: map[string]uuid.UUID{}, variants: map[string]testVariantInfo{"S": vs[0], "M": vs[1]}, units: map[string][]unitRef{}}
	for size, n := range map[string]int{"S": 1, "M": 2} {
		for i := 0; i < n; i++ {
			id, code := f.createSerializedUnit(d.variants[size].VariantID)
			d.units[size] = append(d.units[size], unitRef{id, code})
		}
	}
	var itemIDs []uuid.UUID
	d.orderID, d.fulfillmentID, itemIDs = f.createOrderWithFulfillment("awaiting_payment", "awaiting_payment", prodID, []orderItemSpec{
		{VariantInfo: d.variants["S"], Quantity: 1, SerializedUnits: d.units["S"]},
		{VariantInfo: d.variants["M"], Quantity: 2, SerializedUnits: d.units["M"]},
	})
	d.items["S"], d.items["M"] = itemIDs[0], itemIDs[1]
	f.payViaInventory(d.orderID, d.fulfillmentID)
	f.exec(`INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'tbank', 'succeeded', 1770000, 'RUB', $2, now(), now())`, d.orderID, uuid.NewString())
	f.pickAll(d.fulfillmentID, d.items["S"], d.units["S"])
	f.pickAll(d.fulfillmentID, d.items["M"], d.units["M"])
	_, err := f.svc.PackFulfillment(ctx, f.adminID, d.fulfillmentID)
	require.NoError(f.t, err)
	res, err := f.svc.DispatchFulfillment(ctx, f.adminID, d.fulfillmentID)
	require.NoError(f.t, err)
	d.shipmentID = res.ShipmentID
	_, err = f.deliveryKit().svc.DeliverShipment(ctx, f.adminID, d.shipmentID, fulfillment.DeliverShipmentRequest{})
	require.NoError(f.t, err)
	return d
}

func (f *fboFixture) returnStatus(id uuid.UUID) string {
	return f.str(`SELECT status FROM returns WHERE id = $1`, id)
}

func (f *fboFixture) unitStatus(id uuid.UUID) string {
	return f.str(`SELECT status FROM inventory_units WHERE id = $1`, id)
}

func (f *fboFixture) adjustments(orderItemID uuid.UUID) int {
	return f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment'`, orderItemID)
}

func comment(s string) *string { return &s }

// createApproveAndArrive: customer return of qty units of one item, approved, reverse shipment simulated to ZAMK.
func (f *fboFixture) createApproveAndArrive(k returnsKit, d deliveredOrder, size string, qty int) uuid.UUID {
	f.t.Helper()
	ctx := f.ctx
	resp, err := k.svc.CreateReturn(ctx, f.customerID, d.orderID, returns.CreateReturnRequest{
		Reason: "size_fit", Comment: comment("Не подошёл размер"),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: d.items[size], Quantity: qty}},
	})
	require.NoError(f.t, err)
	require.Len(f.t, resp, 1)
	retID := resp[0].ID
	require.NoError(f.t, k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = k.svc.CreateSimulatedReturnShipment(ctx, retID)
	require.NoError(f.t, err)
	for i := 0; i < 3; i++ {
		_, err = k.svc.AdvanceSimulatedReturnShipment(ctx, retID)
		require.NoError(f.t, err)
	}
	require.Equal(f.t, "arrived_at_zamk", f.str(`SELECT status FROM return_shipments WHERE return_id = $1`, retID))
	return retID
}

// receive scans the given units and records one disposition each, then finalizes.
func (f *fboFixture) receive(k returnsKit, retID uuid.UUID, units []unitRef, disposition string) {
	f.t.Helper()
	require.NoError(f.t, k.svc.StartReceiving(f.ctx, retID))
	for _, u := range units {
		res, err := k.svc.ScanReturnUnit(f.ctx, retID, returns.ScanReturnUnitRequest{Code: u.UnitCode})
		require.NoError(f.t, err)
		require.NoError(f.t, k.svc.InspectSerializedUnit(f.ctx, retID, res.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: disposition, InspectedCondition: comment("осмотр")}))
	}
	require.NoError(f.t, k.svc.FinalizeReceiving(f.ctx, retID))
}

// ============================================================================
// 1. MAIN SCENARIO
// ============================================================================

func TestReturn_E2E1_DeliveredOrder_ReturnOneUnit_RestockRefundLedger(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)
	k := f.returnsKit()
	d := f.deliveredTwoSkuOrder()
	varM := d.variants["M"]

	// 2. delivered with a delivery date
	require.Equal(t, 1, f.count(`SELECT count(*) FROM shipments WHERE id = $1 AND status = 'delivered' AND delivered_at IS NOT NULL`, d.shipmentID))
	require.Equal(t, stockSnap{8, 0}, f.stock(varM.VariantID))
	earningM := int64(f.count(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, d.items["M"]))
	require.Greater(t, earningM, int64(0))
	movementsBefore := f.count(`SELECT count(*) FROM stock_movements WHERE product_variant_id = $1`, varM.VariantID)

	// 4–5. customer returns ONE unit of M (one position of two, partial quantity)
	resp, err := k.svc.CreateReturn(ctx, f.customerID, d.orderID, returns.CreateReturnRequest{
		Reason: "size_fit", Comment: comment("Не подошёл размер"),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: d.items["M"], Quantity: 1}},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	retID := resp[0].ID
	assert.Equal(t, "requested", resp[0].Status)
	assert.Equal(t, d.orderID, resp[0].OrderID)
	assert.Equal(t, d.fulfillmentID, resp[0].FulfillmentID)
	require.Len(t, resp[0].Items, 1)
	assert.Equal(t, 1, resp[0].Items[0].Quantity)
	assert.Equal(t, varM.SKU, *resp[0].Items[0].SKU)

	// 6. visible to admin
	adminList, _, err := k.svc.ListAdminReturns(ctx, 50, 0, false)
	require.NoError(t, err)
	seen := false
	for _, r := range adminList {
		seen = seen || r.ID == retID
	}
	assert.True(t, seen)

	// nothing comes back to the shelf before physical receiving
	assert.Equal(t, stockSnap{8, 0}, f.stock(varM.VariantID))
	for _, u := range d.units["M"] {
		assert.Equal(t, "shipped", f.unitStatus(u.UnitID))
	}

	// 7. review + reverse logistics (simulated, no carrier call)
	require.NoError(t, k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	assert.ErrorIs(t, k.svc.StartReceiving(ctx, retID), returns.ErrReturnNotArrived, "receiving waits for the parcel")
	_, err = k.svc.CreateSimulatedReturnShipment(ctx, retID)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = k.svc.AdvanceSimulatedReturnShipment(ctx, retID)
		require.NoError(t, err)
	}
	require.Equal(t, "arrived_at_zamk", f.str(`SELECT status FROM return_shipments WHERE return_id = $1`, retID))
	assert.Equal(t, stockSnap{8, 0}, f.stock(varM.VariantID), "arrival alone does not restock")

	// 8. warehouse receiving through the real service
	require.NoError(t, k.svc.StartReceiving(ctx, retID))
	assert.Equal(t, "receiving", f.returnStatus(retID))
	_, err = k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: d.units["S"][0].UnitCode})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "unit of another SKU / item is refused")
	_, err = k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: "ZMU-DOESNOTEXIST01"})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "unknown unit is refused")
	scan, err := k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: d.units["M"][0].UnitCode})
	require.NoError(t, err)
	assert.False(t, scan.AlreadyScanned)
	again, err := k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: d.units["M"][0].UnitCode})
	require.NoError(t, err)
	assert.True(t, again.AlreadyScanned)
	_, err = k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: d.units["M"][1].UnitCode})
	assert.ErrorIs(t, err, returns.ErrQuantityExceeded, "only one unit was returned")
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM return_item_units riu JOIN return_items ri ON ri.id = riu.return_item_id WHERE ri.return_id = $1`, retID))

	// 9. inspection is required before finalizing
	assert.ErrorIs(t, k.svc.FinalizeReceiving(ctx, retID), returns.ErrFinalizeMissingDisposition)
	require.NoError(t, k.svc.InspectSerializedUnit(ctx, retID, scan.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock", InspectedCondition: comment("без следов носки")}))

	// 10. finalize: exactly this unit goes back to the shelf, +1 available
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/returns/"+retID.String()+"/receiving/finalize", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", retID.String())
	k.handler.FinalizeReceiving(rec, req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, "item_received", f.returnStatus(retID))
	assert.Equal(t, stockSnap{9, 0}, f.stock(varM.VariantID))
	assert.Equal(t, "warehouse", f.unitStatus(d.units["M"][0].UnitID))
	assert.Equal(t, "shipped", f.unitStatus(d.units["M"][1].UnitID))
	assert.Equal(t, "shipped", f.unitStatus(d.units["S"][0].UnitID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1 AND released_at IS NOT NULL AND release_reason = 'returned_restock'`, d.units["M"][0].UnitID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM stock_movements WHERE type = 'return' AND reference_id = $1 AND product_variant_id = $2 AND quantity = 1`, retID, varM.VariantID))
	assert.Equal(t, movementsBefore+1, f.count(`SELECT count(*) FROM stock_movements WHERE product_variant_id = $1`, varM.VariantID))
	// original sale, shipment and delivery history are kept; the order is not rolled back
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM stock_movements WHERE type = 'sale' AND reference_id = $1 AND product_variant_id = $2`, d.orderID, varM.VariantID))
	assert.Equal(t, "delivered", f.str(`SELECT status FROM orders WHERE id = $1`, d.orderID))
	assert.Equal(t, "delivered", f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, d.fulfillmentID))
	assert.Equal(t, "delivered", f.str(`SELECT status FROM shipments WHERE id = $1`, d.shipmentID))

	// repeated finalize (lost response): no-op, no second restock
	rec2 := httptest.NewRecorder()
	k.handler.FinalizeReceiving(rec2, req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))
	require.Equal(t, http.StatusNoContent, rec2.Code)
	assert.Equal(t, stockSnap{9, 0}, f.stock(varM.VariantID))
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM stock_movements WHERE type = 'return' AND reference_id = $1`, retID))

	// 11. money: refund is only RESERVED (pending) — not refunded, seller not debited yet
	assert.Equal(t, 0, f.adjustments(d.items["M"]))
	refund, err := k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{Reason: comment("возврат по заявке")})
	require.NoError(t, err)
	assert.Equal(t, "pending", refund.Status)
	assert.Equal(t, int64(590000), refund.AmountCents, "one unit of M at the order price")
	refundAgain, err := k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.Equal(t, refund.ID, refundAgain.ID, "repeat creates no second refund")
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM refunds WHERE return_id = $1`, retID))
	assert.Equal(t, 0, f.count(`SELECT count(*) FROM refunds WHERE return_id = $1 AND processed_at IS NOT NULL`, retID))
	assert.Equal(t, "item_received", f.returnStatus(retID))
	assert.Equal(t, 0, f.adjustments(d.items["M"]))
	cust, err := k.svc.GetCustomerReturn(ctx, f.customerID, retID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", cust.Status, "the customer is not told the money is back")

	// provider confirmation (existing simulate path == canonical ProcessRefundSuccessTx)
	_, err = k.svc.SimulateRefundSuccess(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM refunds WHERE return_id = $1 AND status = 'succeeded' AND processed_at IS NOT NULL`, retID))
	assert.Equal(t, "refunded", f.returnStatus(retID))
	// seller: half of the M earning (1 of 2 units) offset, frozen with the same available_at; S untouched
	assert.Equal(t, 1, f.adjustments(d.items["M"]))
	var adj int64
	var sameHold bool
	require.NoError(t, f.db.QueryRow(ctx, `SELECT a.amount_cents, a.available_at IS NOT DISTINCT FROM e.available_at
		FROM seller_ledger_entries a JOIN seller_ledger_entries e ON e.order_item_id = a.order_item_id AND e.type = 'seller_earning'
		WHERE a.order_item_id = $1 AND a.type = 'adjustment'`, d.items["M"]).Scan(&adj, &sameHold))
	assert.Equal(t, -earningM/2, adj)
	assert.True(t, sameHold, "the offset stays frozen exactly like the earning")
	assert.Equal(t, 0, f.adjustments(d.items["S"]))

	// repeated provider confirmation: no second refund / adjustment
	require.NoError(t, k.svc.ProcessRefundSuccess(ctx, refund.ID, refund.CreatedAt))
	_, err = k.svc.SimulateRefundSuccess(ctx, retID)
	assert.Error(t, err)
	assert.Equal(t, 1, f.adjustments(d.items["M"]))
	assert.Equal(t, stockSnap{9, 0}, f.stock(varM.VariantID))

	// 12. the customer sees the truthful final status
	cust, err = k.svc.GetCustomerReturn(ctx, f.customerID, retID)
	require.NoError(t, err)
	assert.Equal(t, "refunded", cust.Status)
	list, _, err := k.svc.ListCustomerReturns(ctx, f.customerID, 20, 0)
	require.NoError(t, err)
	require.NotEmpty(t, list)

	// the second unit of M can still be returned, but not more than bought
	_, err = k.svc.CreateReturn(ctx, f.customerID, d.orderID, returns.CreateReturnRequest{
		Reason: "size_fit", Comment: comment("ещё раз"), Items: []returns.CreateReturnItemRequest{{OrderItemID: d.items["M"], Quantity: 2}},
	})
	assert.Error(t, err, "2 more of M would exceed the 2 bought")
}

// ============================================================================
// 2. NEGATIVE AND CONCURRENT SCENARIOS
// ============================================================================

func TestReturn_E2E1_NegativeScenarios(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)
	k := f.returnsKit()
	req := func(itemID uuid.UUID, qty int) returns.CreateReturnRequest {
		return returns.CreateReturnRequest{Reason: "size_fit", Comment: comment("возврат"), Items: []returns.CreateReturnItemRequest{{OrderItemID: itemID, Quantity: qty}}}
	}

	t.Run("not delivered yet: refused", func(t *testing.T) {
		orderID, _, _, _, _ := f.shippedSingleVariantOrder()
		itemID := uuid.MustParse(f.str(`SELECT id::text FROM order_items WHERE order_id = $1`, orderID))
		_, err := k.svc.CreateReturn(ctx, f.customerID, orderID, req(itemID, 1))
		assert.ErrorIs(t, err, returns.ErrOrderNotDelivered)
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM returns WHERE order_id = $1`, orderID))
	})

	t.Run("window expired, foreign order, over quantity, duplicate", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		_, err := k.svc.CreateReturn(ctx, uuid.New(), d.orderID, req(d.items["M"], 1))
		assert.ErrorIs(t, err, returns.ErrUnauthorized)
		_, err = k.svc.CreateReturn(ctx, f.customerID, d.orderID, req(d.items["M"], 3))
		assert.Error(t, err, "more than bought")
		_, err = k.svc.CreateReturn(ctx, f.customerID, d.orderID, req(d.items["M"], 2))
		require.NoError(t, err)
		_, err = k.svc.CreateReturn(ctx, f.customerID, d.orderID, req(d.items["M"], 1))
		assert.Error(t, err, "both units already in an active return")
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM returns WHERE order_id = $1`, d.orderID))

		testutil.MustBeDisposableDatabase(t, f.db)
		f.exec(`UPDATE shipments SET delivered_at = now() - interval '15 days' WHERE id = $1`, d.shipmentID) // time passes (fixture)
		_, err = k.svc.CreateReturn(ctx, f.customerID, d.orderID, req(d.items["S"], 1))
		assert.ErrorIs(t, err, returns.ErrReturnWindowExpired)
	})

	t.Run("generic status endpoint cannot mark goods received without warehouse receiving", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		resp, err := k.svc.CreateReturn(ctx, f.customerID, d.orderID, req(d.items["M"], 1))
		require.NoError(t, err)
		retID := resp[0].ID
		require.NoError(t, k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		err = k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "item_received"})
		assert.ErrorIs(t, err, returns.ErrInvalidStatusTransition)
		assert.Equal(t, "approved", f.returnStatus(retID))
		assert.Equal(t, stockSnap{8, 0}, f.stock(d.variants["M"].VariantID))
	})

	t.Run("approve vs reject at the same time: exactly one decision wins", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		resp, err := k.svc.CreateReturn(ctx, f.customerID, d.orderID, req(d.items["M"], 1))
		require.NoError(t, err)
		retID := resp[0].ID
		var wg sync.WaitGroup
		var errApprove, errReject error
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			errApprove = k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "approved"})
		}()
		go func() {
			defer wg.Done()
			<-start
			errReject = k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "rejected", AdminComment: comment("не наш товар")})
		}()
		close(start)
		wg.Wait()
		final := f.returnStatus(retID)
		ok := 0
		for _, e := range []error{errApprove, errReject} {
			if e == nil {
				ok++
			} else {
				assert.ErrorIs(t, e, returns.ErrInvalidStatusTransition)
			}
		}
		assert.Equal(t, 1, ok, "approve=%v reject=%v final=%s", errApprove, errReject, final)
		if errApprove == nil {
			assert.Equal(t, "approved", final)
		} else {
			assert.Equal(t, "rejected", final)
		}
	})

	t.Run("two workers scan / finalize the same unit at once: one receipt, one restock", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		retID := f.createApproveAndArrive(k, d, "M", 1)
		require.NoError(t, k.svc.StartReceiving(ctx, retID))
		code := d.units["M"][0].UnitCode
		var wg sync.WaitGroup
		res := make([]*returns.ScanReturnUnitResponse, 2)
		errs := make([]error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				res[i], errs[i] = k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: code})
			}(i)
		}
		wg.Wait()
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM return_item_units riu JOIN return_items ri ON ri.id = riu.return_item_id WHERE ri.return_id = $1`, retID))
		var unitRowID uuid.UUID
		for i := range res {
			if errs[i] == nil {
				unitRowID = res[i].ReturnItemUnit.ID
			}
		}
		require.NotEqual(t, uuid.Nil, unitRowID)
		require.NoError(t, k.svc.InspectSerializedUnit(ctx, retID, unitRowID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))

		const n = 6
		ferrs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) { defer wg.Done(); ferrs[i] = k.svc.FinalizeReceiving(ctx, retID) }(i)
		}
		wg.Wait()
		for _, e := range ferrs {
			assert.NoError(t, e)
		}
		assert.Equal(t, stockSnap{9, 0}, f.stock(d.variants["M"].VariantID), "restocked exactly once")
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM stock_movements WHERE type = 'return' AND reference_id = $1`, retID))
		assert.Equal(t, "warehouse", f.unitStatus(d.units["M"][0].UnitID))
	})

	t.Run("damaged unit: not back on sale, no sellable stock", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		retID := f.createApproveAndArrive(k, d, "M", 1)
		f.receive(k, retID, d.units["M"][:1], "damaged")
		assert.Equal(t, "item_received", f.returnStatus(retID))
		assert.Equal(t, "damaged", f.unitStatus(d.units["M"][0].UnitID))
		assert.Equal(t, stockSnap{8, 0}, f.stock(d.variants["M"].VariantID), "damaged goods add no sellable stock")
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM stock_movements WHERE type = 'return' AND reference_id = $1`, retID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1 AND release_reason = 'returned_damaged'`, d.units["M"][0].UnitID))
	})

	t.Run("rejected unit and missing unit: no stock, no refund", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		retID := f.createApproveAndArrive(k, d, "M", 1)
		f.receive(k, retID, d.units["M"][:1], "reject")
		assert.Equal(t, "shipped", f.unitStatus(d.units["M"][0].UnitID), "rejected goods are not taken in")
		assert.Equal(t, stockSnap{8, 0}, f.stock(d.variants["M"].VariantID))
		_, err := k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
		assert.ErrorIs(t, err, returns.ErrRefundNoEligibleItems)

		d2 := f.deliveredTwoSkuOrder()
		ret2 := f.createApproveAndArrive(k, d2, "S", 1)
		f.receive(k, ret2, nil, "") // parcel arrived, the unit is not in it
		assert.Equal(t, "item_received", f.returnStatus(ret2))
		assert.Equal(t, "shipped", f.unitStatus(d2.units["S"][0].UnitID))
		assert.Equal(t, stockSnap{9, 0}, f.stock(d2.variants["S"].VariantID))
		_, err = k.svc.CreateRefund(ctx, f.adminID, ret2, returns.CreateRefundRequest{})
		assert.ErrorIs(t, err, returns.ErrRefundNoEligibleItems)
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM refunds WHERE return_id = ANY($1)`, []uuid.UUID{retID, ret2}))
	})

	t.Run("failed refund: return stays received, no seller debit", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		retID := f.createApproveAndArrive(k, d, "M", 1)
		f.receive(k, retID, d.units["M"][:1], "restock")
		_, err := k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		_, err = k.svc.SimulateRefundFailure(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, "item_received", f.returnStatus(retID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM refunds WHERE return_id = $1 AND status = 'failed' AND processed_at IS NULL`, retID))
		assert.Equal(t, 0, f.adjustments(d.items["M"]))
	})

	t.Run("payout batch formed while the return is open: disputed share held, refund deducts once and releases the hold", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		retID := f.createApproveAndArrive(k, d, "M", 1)
		f.receive(k, retID, d.units["M"][:1], "restock")
		_, err := k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		// 14 days pass (fixture), the payout batch is formed by the real service (DB only, no bank call)
		testutil.MustBeDisposableDatabase(t, f.db)
		f.exec(`UPDATE seller_ledger_entries SET available_at = now() - interval '1 day' WHERE order_id = $1`, d.orderID)
		payoutSvc := payouts.NewService(payouts.NewRepository(f.db), f.pgClient, returns.NewRepository(f.db), f.ordersRepo, &config.Config{}, nil)
		_, err = payoutSvc.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		require.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning' AND payout_batch_id IS NULL`, d.items["M"]))

		_, err = k.svc.SimulateRefundSuccess(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND available_at IS NULL AND metadata->>'reason' = 'return_post_payout'`, d.items["M"]))
		require.NoError(t, k.svc.ProcessRefundSuccess(ctx, uuid.MustParse(f.str(`SELECT id::text FROM refunds WHERE return_id = $1`, retID)), time.Now()))
		// PAYOUT.SAFETY.1: the batch held the disputed share; the refund adds the deduction and releases the hold
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' IN ('return_deduction','return_post_payout')`, d.items["M"]))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold'`, d.items["M"]))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold_release'`, d.items["M"]))
	})

	t.Run("transaction failure while finalizing: full rollback, then a clean receipt", func(t *testing.T) {
		d := f.deliveredTwoSkuOrder()
		retID := f.createApproveAndArrive(k, d, "M", 1)
		require.NoError(t, k.svc.StartReceiving(ctx, retID))
		s, err := k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: d.units["M"][0].UnitCode})
		require.NoError(t, err)
		require.NoError(t, k.svc.InspectSerializedUnit(ctx, retID, s.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))
		testutil.MustBeDisposableDatabase(t, f.db)
		f.exec(`CREATE FUNCTION return_e2e1_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'return_e2e1 injected failure'; END $$`)
		f.exec(`CREATE TRIGGER return_e2e1_fail BEFORE INSERT ON stock_movements FOR EACH ROW WHEN (NEW.type = 'return') EXECUTE FUNCTION return_e2e1_fail()`)
		err = k.svc.FinalizeReceiving(ctx, retID)
		require.Error(t, err)
		assert.False(t, errors.Is(err, returns.ErrReturnNotInReceiving))
		assert.Equal(t, "receiving", f.returnStatus(retID))
		assert.Equal(t, "shipped", f.unitStatus(d.units["M"][0].UnitID))
		assert.Equal(t, stockSnap{8, 0}, f.stock(d.variants["M"].VariantID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1 AND released_at IS NOT NULL`, d.units["M"][0].UnitID))
		f.exec(`DROP TRIGGER return_e2e1_fail ON stock_movements`)
		f.exec(`DROP FUNCTION return_e2e1_fail()`)
		require.NoError(t, k.svc.FinalizeReceiving(ctx, retID))
		assert.Equal(t, stockSnap{9, 0}, f.stock(d.variants["M"].VariantID))
		assert.Equal(t, "warehouse", f.unitStatus(d.units["M"][0].UnitID))
	})
}
