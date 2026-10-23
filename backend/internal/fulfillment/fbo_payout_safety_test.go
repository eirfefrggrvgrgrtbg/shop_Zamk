package fulfillment_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// PAYOUT.SAFETY.1 — Product Owner decision B: the earning of units under an open return is not paid out while the
// return is open; undisputed earning stays payable. Isolated disposable database per test.

func (f *fboFixture) earningOf(itemID uuid.UUID) int64 {
	return f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, itemID)
}

func (f *fboFixture) holds(itemID uuid.UUID) (n int, cents int64) {
	n = f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold'`, itemID)
	cents = f.sum(`SELECT COALESCE(SUM(amount_cents),0) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold'`, itemID)
	return
}

func (f *fboFixture) releases(itemID uuid.UUID) int {
	return f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold_release'`, itemID)
}

func (f *fboFixture) unreleasedHolds(sellerID uuid.UUID) int {
	return f.count(`SELECT count(*) FROM seller_ledger_entries s WHERE seller_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold'
		AND NOT EXISTS (SELECT 1 FROM seller_ledger_entries r WHERE r.metadata->>'reason' = 'return_hold_release' AND r.metadata->>'hold_entry_id' = s.id::text)`, sellerID)
}

// openReturn files and approves a customer return of qty units of one item (open, not refunded).
func (f *fboFixture) openReturn(k returnsKit, d deliveredOrder, size string, qty int) uuid.UUID {
	f.t.Helper()
	resp, err := k.svc.CreateReturn(f.ctx, f.customerID, d.orderID, returns.CreateReturnRequest{
		Reason: "size_fit", Comment: comment("возврат"), Items: []returns.CreateReturnItemRequest{{OrderItemID: d.items[size], Quantity: qty}},
	})
	require.NoError(f.t, err)
	require.NoError(f.t, k.svc.UpdateReturnStatus(f.ctx, f.adminID, resp[0].ID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	return resp[0].ID
}

func (f *fboFixture) refund(k returnsKit, retID uuid.UUID, d deliveredOrder, size string) {
	f.t.Helper()
	require.NoError(f.t, k.svc.StartReceiving(f.ctx, retID)) // parcel must be at ZAMK
	res, err := k.svc.ScanReturnUnit(f.ctx, retID, returns.ScanReturnUnitRequest{Code: d.units[size][0].UnitCode})
	require.NoError(f.t, err)
	require.NoError(f.t, k.svc.InspectSerializedUnit(f.ctx, retID, res.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))
	require.NoError(f.t, k.svc.FinalizeReceiving(f.ctx, retID))
	_, err = k.svc.CreateRefund(f.ctx, f.adminID, retID, returns.CreateRefundRequest{})
	require.NoError(f.t, err)
	_, err = k.svc.SimulateRefundSuccess(f.ctx, retID)
	require.NoError(f.t, err)
}

func (f *fboFixture) arrive(k returnsKit, retID uuid.UUID) {
	f.t.Helper()
	_, err := k.svc.CreateSimulatedReturnShipment(f.ctx, retID)
	require.NoError(f.t, err)
	for i := 0; i < 3; i++ {
		_, err = k.svc.AdvanceSimulatedReturnShipment(f.ctx, retID)
		require.NoError(f.t, err)
	}
}

func TestPayoutSafety_OpenReturnHold(t *testing.T) {
	ctx := context.Background()

	t.Run("one open return of one unit: only that unit's share is held; other units, items, orders and sellers are paid", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d1 := f.deliveredTwoSkuOrder()
		d2 := f.deliveredTwoSkuOrder() // another order of the same seller
		seller1 := f.sellerID
		seller2 := uuid.New()
		f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Other Seller', $2, $3, 'active', now(), now())`,
			seller2, "o-"+seller2.String()[:8], "o-"+seller2.String()[:8]+"@ex.com")
		f.sellerID = seller2
		d3 := f.deliveredTwoSkuOrder()
		f.sellerID = seller1
		earnM := f.earningOf(d1.items["M"])
		e1, e2, e3 := f.orderEarning(d1.orderID), f.orderEarning(d2.orderID), f.orderEarning(d3.orderID)

		f.openReturn(k, d1, "M", 1)
		for _, o := range []uuid.UUID{d1.orderID, d2.orderID, d3.orderID} {
			f.holdPassed(o)
		}
		bal, err := p.GetSellerBalance(ctx, seller1)
		require.NoError(t, err)
		assert.Equal(t, e1+e2-earnM/2, bal.AvailableCents, "the disputed share is not shown as available")
		assert.Equal(t, earnM/2, bal.ReturnHoldCents)
		sum, err := p.GetAdminPayoutSummary(ctx)
		require.NoError(t, err)
		assert.Equal(t, e1+e2+e3-earnM/2, sum.TotalAvailableCents)
		assert.Equal(t, earnM/2, sum.TotalReturnHoldCents)

		b, err := p.CreatePayoutBatchForSeller(ctx, seller1)
		require.NoError(t, err)
		assert.Equal(t, e1+e2-earnM/2, b.AmountCents, "other unit of M, item S and order 2 are paid; one unit of M is held")
		n, cents := f.holds(d1.items["M"])
		assert.Equal(t, 1, n)
		assert.Equal(t, -earnM/2, cents)
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND metadata->>'reason' = 'return_hold'`, b.ID))
		assert.Equal(t, b.AmountCents, f.sum(`SELECT SUM(amount_cents) FROM seller_ledger_entries WHERE payout_batch_id = $1`, b.ID), "batch = its ledger rows")
		hs, _ := f.holds(d1.items["S"])
		assert.Equal(t, 0, hs)
		bal, err = p.GetSellerBalance(ctx, seller1)
		require.NoError(t, err)
		assert.Equal(t, int64(0), bal.AvailableCents)
		assert.Equal(t, earnM/2, bal.ReturnHoldCents, "held in the batch, not payable, not paid")
		f.assertConservation(p, seller1)
		bal2, err := p.GetSellerBalance(ctx, seller2)
		require.NoError(t, err)
		assert.Equal(t, e3, bal2.AvailableCents, "the other seller is untouched")
		assert.Equal(t, int64(0), bal2.ReturnHoldCents)

		// a repeat captures nothing and never holds twice
		_, err = p.CreatePayoutBatchForSeller(ctx, seller1)
		assert.Error(t, err)
		n, _ = f.holds(d1.items["M"])
		assert.Equal(t, 1, n)
		require.NoError(t, p.ProcessPayoutBatch(ctx, b.ID), "the batch holds the dispute, so it can be confirmed")
	})

	t.Run("rejected / cancelled return: the held share is released once and paid in the next batch", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		earnM := f.earningOf(d.items["M"])
		earning := f.orderEarning(d.orderID)
		retID := f.openReturn(k, d, "M", 1)
		f.holdPassed(d.orderID)
		b1, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		require.NoError(t, p.ProcessPayoutBatch(ctx, b1.ID))

		require.NoError(t, k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "cancelled"}))
		assert.Error(t, k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "cancelled"}), "a closed return cannot change again")
		assert.Equal(t, 1, f.releases(d.items["M"]))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earnM/2, bal.AvailableCents)
		assert.Equal(t, int64(0), bal.ReturnHoldCents)
		b2, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earnM/2, b2.AmountCents)
		assert.Equal(t, earning, b1.AmountCents+b2.AmountCents, "the seller gets the whole earning, once")
		f.assertConservation(p, f.sellerID)

		// a rejected return behaves the same
		d2 := f.deliveredTwoSkuOrder()
		ret2, err := k.svc.CreateReturn(ctx, f.customerID, d2.orderID, returns.CreateReturnRequest{Reason: "size_fit", Comment: comment("x"), Items: []returns.CreateReturnItemRequest{{OrderItemID: d2.items["S"], Quantity: 1}}})
		require.NoError(t, err)
		f.holdPassed(d2.orderID)
		b3, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, f.orderEarning(d2.orderID)-f.earningOf(d2.items["S"]), b3.AmountCents, "the whole single unit of S is disputed")
		require.NoError(t, k.svc.UpdateReturnStatus(ctx, f.adminID, ret2[0].ID, returns.UpdateReturnStatusRequest{Status: "rejected", AdminComment: comment("не подлежит возврату")}))
		bal, err = p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, f.earningOf(d2.items["S"]), bal.AvailableCents)
		assert.Equal(t, 0, f.unreleasedHolds(f.sellerID))
	})

	t.Run("refund confirmed after the hold: deduction once, hold released, no debt; repeat confirmation adds nothing", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		earnM := f.earningOf(d.items["M"])
		earning := f.orderEarning(d.orderID)
		retID := f.openReturn(k, d, "M", 1)
		f.arrive(k, retID)
		f.holdPassed(d.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earning-earnM/2, b.AmountCents)
		f.refund(k, retID, d, "M")
		refundID := uuid.MustParse(f.str(`SELECT id::text FROM refunds WHERE return_id = $1`, retID))
		require.NoError(t, k.svc.ProcessRefundSuccess(ctx, refundID, time.Now()))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND metadata->>'reason' IN ('return_deduction','return_post_payout')`, d.items["M"]))
		assert.Equal(t, 1, f.releases(d.items["M"]))
		require.NoError(t, p.ProcessPayoutBatch(ctx, b.ID))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), bal.AvailableCents, "deduction −½M and release +½M cancel: no debt")
		assert.Equal(t, earning-earnM/2, bal.PaidCents, "the seller was paid exactly the undisputed earning")
		f.assertConservation(p, f.sellerID)
	})

	t.Run("return opened after the batch was formed: confirming the payout is refused until the return is resolved", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		earnM := f.earningOf(d.items["M"])
		earning := f.orderEarning(d.orderID)
		f.holdPassed(d.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earning, b.AmountCents)
		retID := f.openReturn(k, d, "M", 1)
		err = p.ProcessPayoutBatch(ctx, b.ID)
		assert.ErrorIs(t, err, payouts.ErrPayoutBatchHasOpenReturns)
		assert.Equal(t, "scheduled", f.str(`SELECT status FROM payout_batches WHERE id = $1`, b.ID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b.ID))

		// resolved by refund: the payout can be confirmed, the refunded share is offset in that same payout
		f.arrive(k, retID)
		f.refund(k, retID, d, "M")
		require.NoError(t, p.ProcessPayoutBatch(ctx, b.ID))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earning-earnM/2, bal.PaidCents, "the refunded share was taken out of this payout")
		assert.Equal(t, int64(0), bal.AvailableCents)
		assert.Equal(t, int64(0), bal.FrozenCents)
	})

	t.Run("return opened after the batch was paid: existing post-payout recovery, nothing held", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		earnM := f.earningOf(d.items["M"])
		f.holdPassed(d.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		require.NoError(t, p.ProcessPayoutBatch(ctx, b.ID))
		retID := f.openReturn(k, d, "M", 1)
		n, _ := f.holds(d.items["M"])
		assert.Equal(t, 0, n)
		f.arrive(k, retID)
		f.refund(k, retID, d, "M")
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, -earnM/2, bal.AvailableCents, "debt recovered from future payouts, as before")
		assert.Equal(t, 0, f.releases(d.items["M"]))
	})

	t.Run("failure while writing the hold: nothing captured, then a clean batch", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		f.openReturn(k, d, "M", 1)
		f.holdPassed(d.orderID)
		testutil.MustBeDisposableDatabase(t, f.db)
		f.exec(`CREATE FUNCTION safety_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'safety injected failure'; END $$`)
		f.exec(`CREATE TRIGGER safety_fail BEFORE INSERT ON seller_ledger_entries FOR EACH ROW WHEN (NEW.metadata->>'reason' = 'return_hold') EXECUTE FUNCTION safety_fail()`)
		_, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.Error(t, err)
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM payout_batches WHERE seller_id = $1`, f.sellerID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1 AND payout_batch_id IS NOT NULL`, f.sellerID))
		f.exec(`DROP TRIGGER safety_fail ON seller_ledger_entries`)
		f.exec(`DROP FUNCTION safety_fail()`)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, f.orderEarning(d.orderID)-f.earningOf(d.items["M"])/2, b.AmountCents)
	})
}

func TestPayoutSafety_Concurrency(t *testing.T) {
	ctx := context.Background()
	race := func(a, b func()) {
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; a() }()
		go func() { defer wg.Done(); <-start; b() }()
		close(start)
		wg.Wait()
	}

	t.Run("A: return created while the batch is formed: the dispute is held or the payout confirmation is refused", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			f := newFBOFixture(t)
			p := f.payoutSvc()
			k := f.returnsKit()
			d := f.deliveredTwoSkuOrder()
			earnM := f.earningOf(d.items["M"])
			f.holdPassed(d.orderID)
			var errRet, errBatch error
			var batch *payouts.PayoutBatch
			race(func() {
				_, errRet = k.svc.CreateReturn(ctx, f.customerID, d.orderID, returns.CreateReturnRequest{Reason: "size_fit", Comment: comment("x"), Items: []returns.CreateReturnItemRequest{{OrderItemID: d.items["M"], Quantity: 1}}})
			}, func() { batch, errBatch = p.CreatePayoutBatchForSeller(ctx, f.sellerID) })
			require.NoError(t, errRet)
			require.NoError(t, errBatch)
			n, cents := f.holds(d.items["M"])
			if n == 1 {
				assert.Equal(t, -earnM/2, cents)
				require.NoError(t, p.ProcessPayoutBatch(ctx, batch.ID))
			} else {
				assert.ErrorIs(t, p.ProcessPayoutBatch(ctx, batch.ID), payouts.ErrPayoutBatchHasOpenReturns, "batch formed first: confirmation must wait")
			}
		}
	})

	t.Run("B: return created while the payout is confirmed: never paid as undisputed after the return is recorded", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			f := newFBOFixture(t)
			p := f.payoutSvc()
			k := f.returnsKit()
			d := f.deliveredTwoSkuOrder()
			f.holdPassed(d.orderID)
			b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
			require.NoError(t, err)
			var errRet, errPaid error
			race(func() {
				_, errRet = k.svc.CreateReturn(ctx, f.customerID, d.orderID, returns.CreateReturnRequest{Reason: "size_fit", Comment: comment("x"), Items: []returns.CreateReturnItemRequest{{OrderItemID: d.items["M"], Quantity: 1}}})
			}, func() { errPaid = p.ProcessPayoutBatch(ctx, b.ID) })
			require.NoError(t, errRet)
			status := f.str(`SELECT status FROM payout_batches WHERE id = $1`, b.ID)
			if errPaid == nil {
				assert.Equal(t, "paid", status) // the confirmation serialized before the return existed
				assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b.ID))
			} else {
				assert.ErrorIs(t, errPaid, payouts.ErrPayoutBatchHasOpenReturns)
				assert.Equal(t, "scheduled", status)
				assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b.ID))
			}
		}
	})

	t.Run("C: refund confirmed while the batch is formed: deduction once, no hold left, exact total", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			f := newFBOFixture(t)
			p := f.payoutSvc()
			k := f.returnsKit()
			d := f.deliveredTwoSkuOrder()
			earnM := f.earningOf(d.items["M"])
			earning := f.orderEarning(d.orderID)
			retID := f.openReturn(k, d, "M", 1)
			f.arrive(k, retID)
			require.NoError(t, k.svc.StartReceiving(ctx, retID))
			res, err := k.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: d.units["M"][0].UnitCode})
			require.NoError(t, err)
			require.NoError(t, k.svc.InspectSerializedUnit(ctx, retID, res.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))
			require.NoError(t, k.svc.FinalizeReceiving(ctx, retID))
			_, err = k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
			require.NoError(t, err)
			f.holdPassed(d.orderID)
			var errRefund, errBatch error
			race(func() { _, errRefund = k.svc.SimulateRefundSuccess(ctx, retID) }, func() { _, errBatch = p.CreatePayoutBatchForSeller(ctx, f.sellerID) })
			require.NoError(t, errRefund)
			require.NoError(t, errBatch)
			assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND metadata->>'reason' IN ('return_deduction','return_post_payout')`, d.items["M"]))
			assert.Equal(t, 0, f.unreleasedHolds(f.sellerID))
			bal, err := p.GetSellerBalance(ctx, f.sellerID)
			require.NoError(t, err)
			batched := f.sum(`SELECT COALESCE(SUM(amount_cents),0) FROM payout_batches WHERE seller_id = $1`, f.sellerID)
			assert.Equal(t, earning-earnM/2, batched+bal.AvailableCents, "captured + remaining = earning − one deduction")
			f.assertConservation(p, f.sellerID)
		}
	})

	t.Run("D: return cancelled while the batch is formed: no hold left behind, exact total", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			f := newFBOFixture(t)
			p := f.payoutSvc()
			k := f.returnsKit()
			d := f.deliveredTwoSkuOrder()
			earning := f.orderEarning(d.orderID)
			retID := f.openReturn(k, d, "M", 1)
			f.holdPassed(d.orderID)
			var errCancel, errBatch error
			race(func() {
				errCancel = k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "cancelled"})
			}, func() { _, errBatch = p.CreatePayoutBatchForSeller(ctx, f.sellerID) })
			require.NoError(t, errCancel)
			require.NoError(t, errBatch)
			assert.Equal(t, 0, f.unreleasedHolds(f.sellerID), "a hold written by the batch is always released by the cancel")
			bal, err := p.GetSellerBalance(ctx, f.sellerID)
			require.NoError(t, err)
			batched := f.sum(`SELECT COALESCE(SUM(amount_cents),0) FROM payout_batches WHERE seller_id = $1`, f.sellerID)
			assert.Equal(t, earning, batched+bal.AvailableCents)
		}
	})

	t.Run("E: two sellers form batches at once, each with its own open return", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		seller1 := f.sellerID
		d1 := f.deliveredTwoSkuOrder()
		seller2 := uuid.New()
		f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Other Seller', $2, $3, 'active', now(), now())`,
			seller2, "o-"+seller2.String()[:8], "o-"+seller2.String()[:8]+"@ex.com")
		f.sellerID = seller2
		d2 := f.deliveredTwoSkuOrder()
		f.sellerID = seller1
		f.openReturn(k, d1, "M", 1)
		f.openReturn(k, d2, "S", 1)
		f.holdPassed(d1.orderID)
		f.holdPassed(d2.orderID)
		var b1, b2 *payouts.PayoutBatch
		var e1, e2 error
		race(func() { b1, e1 = p.CreatePayoutBatchForSeller(ctx, seller1) }, func() { b2, e2 = p.CreatePayoutBatchForSeller(ctx, seller2) })
		require.NoError(t, e1)
		require.NoError(t, e2)
		assert.Equal(t, f.orderEarning(d1.orderID)-f.earningOf(d1.items["M"])/2, b1.AmountCents)
		assert.Equal(t, f.orderEarning(d2.orderID)-f.earningOf(d2.items["S"]), b2.AmountCents)
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND seller_id <> $2`, b1.ID, seller1))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND seller_id <> $2`, b2.ID, seller2))
		assert.False(t, errors.Is(e1, payouts.ErrPayoutBatchHasOpenReturns))
	})
}

// ============================================================================
// PAYOUT.SAFETY.1.1 — covered vs uncovered disputes at payout confirmation
// ============================================================================

type fiveUnitOrder struct {
	orderID, itemID uuid.UUID
	units           []unitRef
}

// deliveredFiveUnitOrder: 5 serialized units at 2 500 ₽ with a 20 % commission frozen at checkout (order data),
// so the seller earning is exactly 10 000 ₽ and each unit is 2 000 ₽ of it. Paid, picked, packed, dispatched and
// delivered through the real services; a succeeded payment row allows refunds.
func (f *fboFixture) deliveredFiveUnitOrder() fiveUnitOrder {
	f.t.Helper()
	prodID, vs := f.createProductWithVariants([]testVariantInfo{{Size: "L", Color: "Песочный", Price: 250000, Barcode: uniqueBarcode()}}, 10)
	v := vs[0]
	var units []unitRef
	for i := 0; i < 5; i++ {
		id, code := f.createSerializedUnit(v.VariantID)
		units = append(units, unitRef{id, code})
	}
	orderID, fulID, items := f.createOrderWithFulfillment("awaiting_payment", "awaiting_payment", prodID, []orderItemSpec{{VariantInfo: v, Quantity: 5, SerializedUnits: units}})
	f.exec(`UPDATE order_fulfillments SET commission_bps = 2000, commission_frozen = true WHERE id = $1`, fulID)
	f.payViaInventory(orderID, fulID)
	f.exec(`INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'tbank', 'succeeded', 1250000, 'RUB', $2, now(), now())`, orderID, uuid.NewString())
	f.pickAll(fulID, items[0], units)
	_, err := f.svc.PackFulfillment(f.ctx, f.adminID, fulID)
	require.NoError(f.t, err)
	res, err := f.svc.DispatchFulfillment(f.ctx, f.adminID, fulID)
	require.NoError(f.t, err)
	_, err = f.deliveryKit().svc.DeliverShipment(f.ctx, f.adminID, res.ShipmentID, fulfillment.DeliverShipmentRequest{})
	require.NoError(f.t, err)
	require.Equal(f.t, int64(1000000), f.earningOf(items[0]), "earning 10 000 ₽")
	return fiveUnitOrder{orderID: orderID, itemID: items[0], units: units}
}

func (f *fboFixture) returnUnits(k returnsKit, o fiveUnitOrder, qty int) uuid.UUID {
	f.t.Helper()
	resp, err := k.svc.CreateReturn(f.ctx, f.customerID, o.orderID, returns.CreateReturnRequest{
		Reason: "size_fit", Comment: comment("возврат"), Items: []returns.CreateReturnItemRequest{{OrderItemID: o.itemID, Quantity: qty}},
	})
	require.NoError(f.t, err)
	require.NoError(f.t, k.svc.UpdateReturnStatus(f.ctx, f.adminID, resp[0].ID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	return resp[0].ID
}

func (f *fboFixture) refundUnits(k returnsKit, retID uuid.UUID, units []unitRef) {
	f.t.Helper()
	f.arrive(k, retID)
	require.NoError(f.t, k.svc.StartReceiving(f.ctx, retID))
	for _, u := range units {
		res, err := k.svc.ScanReturnUnit(f.ctx, retID, returns.ScanReturnUnitRequest{Code: u.UnitCode})
		require.NoError(f.t, err)
		require.NoError(f.t, k.svc.InspectSerializedUnit(f.ctx, retID, res.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))
	}
	require.NoError(f.t, k.svc.FinalizeReceiving(f.ctx, retID))
	_, err := k.svc.CreateRefund(f.ctx, f.adminID, retID, returns.CreateRefundRequest{})
	require.NoError(f.t, err)
	_, err = k.svc.SimulateRefundSuccess(f.ctx, retID)
	require.NoError(f.t, err)
}

func (f *fboFixture) payoutRows(batchID uuid.UUID) int {
	return f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batchID)
}

func TestPayoutSafety_CoveredDispute(t *testing.T) {
	ctx := context.Background()

	t.Run("A: 10 000 ₽ earning, open return for 4 000 ₽: the undisputed 6 000 ₽ is confirmed while the return stays open", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		o := f.deliveredFiveUnitOrder()
		retID := f.returnUnits(k, o, 2) // 2 of 5 units = 4 000 ₽
		f.holdPassed(o.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(600000), b.AmountCents)
		_, held := f.holds(o.itemID)
		assert.Equal(t, int64(-400000), held)

		require.NoError(t, p.ProcessPayoutBatch(ctx, b.ID), "covered dispute: the safe part is payable")
		assert.Equal(t, "paid", f.str(`SELECT status FROM payout_batches WHERE id = $1`, b.ID))
		assert.Equal(t, int64(-600000), f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b.ID))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(600000), bal.PaidCents)
		assert.Equal(t, int64(400000), bal.ReturnHoldCents, "4 000 ₽ still held")
		assert.Equal(t, int64(0), bal.AvailableCents)
		assert.Equal(t, "approved", f.returnStatus(retID), "the return is still open")

		// D after payout — refund of the 2 units: deduction 4 000 ₽ once, hold released, no debt
		f.refundUnits(k, retID, o.units[:2])
		require.NoError(t, k.svc.ProcessRefundSuccess(ctx, uuid.MustParse(f.str(`SELECT id::text FROM refunds WHERE return_id = $1`, retID)), time.Now()))
		assert.Equal(t, int64(-400000), f.sum(`SELECT SUM(amount_cents) FROM seller_ledger_entries WHERE order_item_id = $1 AND metadata->>'reason' IN ('return_deduction','return_post_payout')`, o.itemID))
		assert.Equal(t, 1, f.releases(o.itemID))
		bal, err = p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), bal.AvailableCents)
		assert.Equal(t, int64(0), bal.ReturnHoldCents)
		assert.Equal(t, int64(600000), bal.PaidCents, "seller got exactly 10 000 − 4 000")
		f.assertConservation(p, f.sellerID)
	})

	t.Run("B: return opened after the batch: confirmation refused, nothing paid", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		o := f.deliveredFiveUnitOrder()
		f.holdPassed(o.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(1000000), b.AmountCents)
		f.returnUnits(k, o, 2)
		assert.ErrorIs(t, p.ProcessPayoutBatch(ctx, b.ID), payouts.ErrPayoutBatchHasOpenReturns)
		assert.Equal(t, "scheduled", f.str(`SELECT status FROM payout_batches WHERE id = $1`, b.ID))
		assert.Equal(t, 0, f.payoutRows(b.ID))
	})

	t.Run("C: disputed quantity grows after the batch: refused until the extra return is resolved", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		o := f.deliveredFiveUnitOrder()
		f.returnUnits(k, o, 1) // 2 000 ₽ held in the batch
		f.holdPassed(o.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(800000), b.AmountCents)
		ret2 := f.returnUnits(k, o, 1) // one more unit disputed: 2 000 ₽ not covered by the batch
		assert.ErrorIs(t, p.ProcessPayoutBatch(ctx, b.ID), payouts.ErrPayoutBatchHasOpenReturns)
		assert.Equal(t, 0, f.payoutRows(b.ID))
		require.NoError(t, k.svc.UpdateReturnStatus(ctx, f.adminID, ret2, returns.UpdateReturnStatusRequest{Status: "cancelled"}))
		require.NoError(t, p.ProcessPayoutBatch(ctx, b.ID))
		assert.Equal(t, int64(-800000), f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b.ID))
	})

	t.Run("D: return closed before confirmation — rejection pays the held 4 000 ₽ once; refund deducts once", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		o := f.deliveredFiveUnitOrder()
		retID := f.returnUnits(k, o, 2)
		f.holdPassed(o.orderID)
		b1, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		require.NoError(t, k.svc.UpdateReturnStatus(ctx, f.adminID, retID, returns.UpdateReturnStatusRequest{Status: "cancelled"}))
		require.NoError(t, p.ProcessPayoutBatch(ctx, b1.ID))
		assert.Equal(t, int64(-600000), f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b1.ID))
		b2, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(400000), b2.AmountCents, "the released 4 000 ₽")
		require.NoError(t, p.ProcessPayoutBatch(ctx, b2.ID))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(1000000), bal.PaidCents, "10 000 ₽ in total, no double payout")
		_, err = p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		assert.Error(t, err)

		f2 := newFBOFixture(t)
		p2 := f2.payoutSvc()
		k2 := f2.returnsKit()
		o2 := f2.deliveredFiveUnitOrder()
		ret2 := f2.returnUnits(k2, o2, 2)
		f2.holdPassed(o2.orderID)
		b3, err := p2.CreatePayoutBatchForSeller(ctx, f2.sellerID)
		require.NoError(t, err)
		f2.refundUnits(k2, ret2, o2.units[:2])
		require.NoError(t, p2.ProcessPayoutBatch(ctx, b3.ID))
		bal, err = p2.GetSellerBalance(ctx, f2.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(600000), bal.PaidCents)
		assert.Equal(t, int64(0), bal.AvailableCents, "deduction −4 000 and release +4 000 cancel")
		assert.Equal(t, 0, f2.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1 AND metadata->>'reason' = 'batch_race_offset'`, f2.sellerID))
		f2.assertConservation(p2, f2.sellerID)
	})

	t.Run("concurrent confirmations of a covered batch: one payout row", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		o := f.deliveredFiveUnitOrder()
		f.returnUnits(k, o, 2)
		f.holdPassed(o.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		var wg sync.WaitGroup
		errs := make([]error, 4)
		start := make(chan struct{})
		for i := range errs {
			wg.Add(1)
			go func(i int) { defer wg.Done(); <-start; errs[i] = p.ProcessPayoutBatch(ctx, b.ID) }(i)
		}
		close(start)
		wg.Wait()
		ok := 0
		for _, e := range errs {
			if e == nil {
				ok++
			}
		}
		assert.Equal(t, 1, ok)
		assert.Equal(t, 1, f.payoutRows(b.ID))
	})

	t.Run("new return racing with the confirmation of a covered batch: the uncovered part is never paid", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			f := newFBOFixture(t)
			p := f.payoutSvc()
			k := f.returnsKit()
			o := f.deliveredFiveUnitOrder()
			f.returnUnits(k, o, 1)
			f.holdPassed(o.orderID)
			b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
			require.NoError(t, err)
			var wg sync.WaitGroup
			var errRet, errPaid error
			start := make(chan struct{})
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				_, errRet = k.svc.CreateReturn(ctx, f.customerID, o.orderID, returns.CreateReturnRequest{Reason: "size_fit", Comment: comment("x"), Items: []returns.CreateReturnItemRequest{{OrderItemID: o.itemID, Quantity: 1}}})
			}()
			go func() { defer wg.Done(); <-start; errPaid = p.ProcessPayoutBatch(ctx, b.ID) }()
			close(start)
			wg.Wait()
			require.NoError(t, errRet)
			if errPaid == nil {
				assert.Equal(t, 1, f.payoutRows(b.ID), "confirmation serialized before the new return existed")
			} else {
				assert.ErrorIs(t, errPaid, payouts.ErrPayoutBatchHasOpenReturns)
				assert.Equal(t, 0, f.payoutRows(b.ID))
			}
		}
	})

	t.Run("failure during confirmation: batch, holds and ledger unchanged, then a clean confirmation", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		o := f.deliveredFiveUnitOrder()
		f.returnUnits(k, o, 2)
		f.holdPassed(o.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		rows := f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1`, f.sellerID)
		testutil.MustBeDisposableDatabase(t, f.db)
		f.exec(`CREATE FUNCTION safety11_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'safety11 injected failure'; END $$`)
		f.exec(`CREATE TRIGGER safety11_fail BEFORE INSERT ON seller_ledger_entries FOR EACH ROW WHEN (NEW.type = 'payout') EXECUTE FUNCTION safety11_fail()`)
		require.Error(t, p.ProcessPayoutBatch(ctx, b.ID))
		assert.Equal(t, "scheduled", f.str(`SELECT status FROM payout_batches WHERE id = $1`, b.ID))
		assert.Equal(t, rows, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1`, f.sellerID))
		f.exec(`DROP TRIGGER safety11_fail ON seller_ledger_entries`)
		f.exec(`DROP FUNCTION safety11_fail()`)
		require.NoError(t, p.ProcessPayoutBatch(ctx, b.ID))
		assert.Equal(t, 1, f.payoutRows(b.ID))
	})

	t.Run("batch_race_offset is immediately available: the next batch does not recover the debt twice", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		o := f.deliveredFiveUnitOrder()
		f.holdPassed(o.orderID)
		b1, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		retID := f.returnUnits(k, o, 2) // opened after the batch: refunded before confirmation -> debt offset in b1
		f.refundUnits(k, retID, o.units[:2])
		require.NoError(t, p.ProcessPayoutBatch(ctx, b1.ID))
		assert.Equal(t, int64(-600000), f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b1.ID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1 AND metadata->>'reason' = 'batch_race_offset' AND available_at IS NULL`, f.sellerID))

		o2 := f.deliveredFiveUnitOrder()
		f.holdPassed(o2.orderID)
		b2, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID) // the offset must be in this batch together with the debt
		require.NoError(t, err)
		assert.Equal(t, int64(1000000), b2.AmountCents, "the next order is paid in full; the debt was already offset")
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND metadata->>'reason' = 'batch_race_offset'`, b2.ID), "offset and debt travel together")
		f.assertConservation(p, f.sellerID)
	})
}
