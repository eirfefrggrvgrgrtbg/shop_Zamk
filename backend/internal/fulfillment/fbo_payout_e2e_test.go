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

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// PAYOUT.E2E.1 — delivery accrual -> 14-day hold -> available -> payout batch (scheduled) -> manual "paid" mark,
// with returns before / after the payout. Isolated disposable database. No bank call exists anywhere on this path:
// a batch is only scheduled; "paid" is the admin's manual confirmation of a transfer made outside the system.
// Time is moved only by shifting available_at of this test's own ledger rows (fixture), never the code under test.

func (f *fboFixture) payoutSvc() *payouts.Service {
	return payouts.NewService(payouts.NewRepository(f.db), f.pgClient, returns.NewRepository(f.db), f.ordersRepo, nil, nil)
}

// holdPassed moves the hold deadline of one order's ledger rows into the past (fixture "time passes").
func (f *fboFixture) holdPassed(orderID uuid.UUID) {
	f.t.Helper()
	testutil.MustBeDisposableDatabase(f.t, f.db)
	f.exec(`UPDATE seller_ledger_entries SET available_at = now() - interval '1 minute' WHERE order_id = $1 AND available_at IS NOT NULL`, orderID)
}

func (f *fboFixture) sum(sql string, args ...any) int64 {
	f.t.Helper()
	var n int64
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&n), sql)
	return n
}

func (f *fboFixture) orderEarning(orderID uuid.UUID) int64 {
	return f.sum(`SELECT COALESCE(SUM(amount_cents),0) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'seller_earning'`, orderID)
}

// conservation: every seller_earning/adjustment of the seller is either captured by a batch or still in the balance.
func (f *fboFixture) assertConservation(p *payouts.Service, sellerID uuid.UUID) {
	f.t.Helper()
	all := f.sum(`SELECT COALESCE(SUM(amount_cents),0) FROM seller_ledger_entries WHERE seller_id = $1 AND type IN ('seller_earning','adjustment')`, sellerID)
	batched := f.sum(`SELECT COALESCE(SUM(amount_cents),0) FROM seller_ledger_entries WHERE seller_id = $1 AND type IN ('seller_earning','adjustment') AND payout_batch_id IS NOT NULL`, sellerID)
	bal, err := p.GetSellerBalance(f.ctx, sellerID)
	require.NoError(f.t, err)
	// returnHoldCents = disputed share not yet held (taken out of "available") + holds inside batches
	heldInBatches := f.sum(`SELECT COALESCE(SUM(-amount_cents),0) FROM seller_ledger_entries s WHERE seller_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold'
		AND NOT EXISTS (SELECT 1 FROM seller_ledger_entries r WHERE r.metadata->>'reason' = 'return_hold_release' AND r.metadata->>'hold_entry_id' = s.id::text)`, sellerID)
	assert.Equal(f.t, all, batched+bal.AvailableCents+bal.FrozenCents+(bal.ReturnHoldCents-heldInBatches), "ledger money is either batched or in the balance, never both or lost")
}

// ============================================================================
// 1. MAIN SCENARIO
// ============================================================================

func TestPayout_E2E1_DeliveryToBatch(t *testing.T) {
	ctx := context.Background()
	f := newFBOFixture(t)
	p := f.payoutSvc()
	d := f.deliveredTwoSkuOrder()

	// 1. accrual on delivery: per item sale_gross, zamk_commission, seller_earning; commission excluded from earning
	for _, size := range []string{"S", "M"} {
		item := d.items[size]
		gross := f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'sale_gross'`, item)
		comm := f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'zamk_commission'`, item)
		earn := f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, item)
		assert.Greater(t, gross, int64(0))
		assert.NotZero(t, comm)
		abs := comm
		if abs < 0 {
			abs = -abs
		}
		assert.Equal(t, gross-abs, earn, "seller earning = gross − ZAMK commission (%s)", size)
	}
	assert.Equal(t, int64(1770000), f.sum(`SELECT SUM(amount_cents) FROM seller_ledger_entries WHERE order_id = $1 AND type = 'sale_gross'`, d.orderID))
	earning := f.orderEarning(d.orderID)

	// 2–3. seller sees it as frozen; not payable before available_at
	bal, err := p.GetSellerBalance(ctx, f.sellerID)
	require.NoError(t, err)
	assert.Equal(t, earning, bal.FrozenCents)
	assert.Equal(t, int64(0), bal.AvailableCents)
	assert.Equal(t, int64(0), bal.PaidCents)
	_, err = p.CreatePayoutBatchForSeller(ctx, f.sellerID)
	assert.Error(t, err, "nothing payable during the hold")
	assert.Equal(t, 0, f.count(`SELECT count(*) FROM payout_batches WHERE seller_id = $1`, f.sellerID))
	n, err := p.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "the worker releases nothing before available_at")

	// 4–5. hold passes; the worker marks the entries, a repeat changes nothing; no new ledger rows
	rowsBefore := f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1`, f.sellerID)
	f.holdPassed(d.orderID)
	n, err = p.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "the two seller_earning entries")
	n, err = p.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, rowsBefore, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1`, f.sellerID))

	// 6. available balance
	bal, err = p.GetSellerBalance(ctx, f.sellerID)
	require.NoError(t, err)
	assert.Equal(t, earning, bal.AvailableCents)
	assert.Equal(t, int64(0), bal.FrozenCents)

	sum0, err := p.GetAdminPayoutSummary(ctx) // this disposable database holds only this test's data
	require.NoError(t, err)
	assert.Equal(t, earning, sum0.TotalAvailableCents)
	assert.Equal(t, int64(0), sum0.TotalPendingCents)

	// 7–10. batch for this seller: only seller_earning rows, amount = earning (commission excluded), scheduled, not paid
	batch, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
	require.NoError(t, err)
	assert.Equal(t, "scheduled", batch.Status)
	assert.Equal(t, earning, batch.AmountCents)
	assert.Equal(t, 2, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'seller_earning'`, batch.ID))
	assert.Equal(t, 2, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1`, batch.ID))
	assert.Equal(t, earning, f.sum(`SELECT SUM(amount_cents) FROM seller_ledger_entries WHERE payout_batch_id = $1`, batch.ID))
	assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1 AND type = 'payout'`, f.sellerID), "a scheduled batch is not a payout")
	bal, err = p.GetSellerBalance(ctx, f.sellerID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.AvailableCents)
	assert.Equal(t, int64(0), bal.PaidCents, "scheduled money is not reported as paid")
	f.assertConservation(p, f.sellerID)
	sum1, err := p.GetAdminPayoutSummary(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), sum1.TotalAvailableCents, "admin: batched money is no longer available")
	assert.Equal(t, earning, sum1.TotalPendingCents, "admin: it is pending in a scheduled batch")
	assert.Equal(t, int64(0), sum1.TotalPaidCents, "admin: scheduled is not paid")

	// 11. repeat (e.g. lost response): nothing left to capture
	_, err = p.CreatePayoutBatchForSeller(ctx, f.sellerID)
	assert.Error(t, err)
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM payout_batches WHERE seller_id = $1`, f.sellerID))

	// 12. seller list shows the scheduled batch; the admin's manual "paid" mark books exactly one payout row
	list, total, err := p.ListSellerPayoutBatches(ctx, f.sellerID, 20, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, "scheduled", list[0].Status)
	require.NoError(t, p.ProcessPayoutBatch(ctx, batch.ID))
	assert.Equal(t, "paid", f.str(`SELECT status FROM payout_batches WHERE id = $1`, batch.ID))
	assert.Equal(t, -earning, f.sum(`SELECT SUM(amount_cents) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batch.ID))
	assert.Error(t, p.ProcessPayoutBatch(ctx, batch.ID), "already paid")
	assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batch.ID))
	bal, err = p.GetSellerBalance(ctx, f.sellerID)
	require.NoError(t, err)
	assert.Equal(t, earning, bal.PaidCents)
	assert.Equal(t, int64(0), bal.AvailableCents)
	sum2, err := p.GetAdminPayoutSummary(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), sum2.TotalAvailableCents)
	assert.Equal(t, int64(0), sum2.TotalPendingCents)
	assert.Equal(t, earning, sum2.TotalPaidCents)
}

// ============================================================================
// 2. RETURNS AND PAYOUTS
// ============================================================================

func TestPayout_E2E1_ReturnsAndPayouts(t *testing.T) {
	ctx := context.Background()

	t.Run("refund confirmed during the hold: net payout, no double reduction", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		earning := f.orderEarning(d.orderID)
		earnM := f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, d.items["M"])
		retID := f.createApproveAndArrive(k, d, "M", 1)
		f.receive(k, retID, d.units["M"][:1], "restock")
		_, err := k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		_, err = k.svc.SimulateRefundSuccess(ctx, retID)
		require.NoError(t, err)
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earning-earnM/2, bal.FrozenCents, "the frozen offset reduces the frozen balance")

		f.holdPassed(d.orderID)
		batch, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earning-earnM/2, batch.AmountCents, "earning minus the returned unit, once")
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'adjustment'`, batch.ID))
		f.assertConservation(p, f.sellerID)
	})

	t.Run("open return when the hold ends: the disputed share is held in the batch, released by the refund", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		earning := f.orderEarning(d.orderID)
		earnM := f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, d.items["M"])
		retID := f.createApproveAndArrive(k, d, "M", 1) // return approved, parcel at ZAMK, not yet refunded

		f.holdPassed(d.orderID)
		batch, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, earning-earnM/2, batch.AmountCents, "PAYOUT.SAFETY.1: the returned unit's share is held")
		require.NoError(t, p.ProcessPayoutBatch(ctx, batch.ID))

		f.receive(k, retID, d.units["M"][:1], "restock")
		_, err = k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		_, err = k.svc.SimulateRefundSuccess(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_post_payout'`, d.items["M"]))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_hold_release'`, d.items["M"]))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), bal.AvailableCents, "no seller debt: the refunded share was never paid")
		assert.Equal(t, int64(0), bal.ReturnHoldCents)
		f.assertConservation(p, f.sellerID)
	})

	t.Run("refund after a paid batch: history kept, one adjustment, recovered from the next payout, sellers isolated", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d1 := f.deliveredTwoSkuOrder()
		earning1 := f.orderEarning(d1.orderID)
		earnM := f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, d1.items["M"])
		f.holdPassed(d1.orderID)
		b1, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		require.NoError(t, p.ProcessPayoutBatch(ctx, b1.ID))

		// a second, independent seller with its own available earning
		seller1 := f.sellerID
		seller2 := uuid.New()
		f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Other Seller', $2, $3, 'active', now(), now())`,
			seller2, "o-"+seller2.String()[:8], "o-"+seller2.String()[:8]+"@ex.com")
		f.sellerID = seller2
		d2 := f.deliveredTwoSkuOrder()
		f.sellerID = seller1
		f.holdPassed(d2.orderID)
		earning2 := f.orderEarning(d2.orderID)

		retID := f.createApproveAndArrive(k, d1, "M", 1)
		f.receive(k, retID, d1.units["M"][:1], "restock")
		_, err = k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		_, err = k.svc.SimulateRefundSuccess(ctx, retID)
		require.NoError(t, err)
		require.NoError(t, k.svc.ProcessRefundSuccess(ctx, uuid.MustParse(f.str(`SELECT id::text FROM refunds WHERE return_id = $1`, retID)), time.Now()))

		// history kept: original earning still linked to the paid batch; one negative adjustment for half of M only
		assert.Equal(t, earning1, f.sum(`SELECT SUM(amount_cents) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type IN ('seller_earning','adjustment')`, b1.ID))
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment'`, d1.items["M"]))
		assert.Equal(t, -earnM/2, f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment'`, d1.items["M"]))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment'`, d1.items["S"]))

		// next order of seller 1 becomes available: the next batch nets the debt; seller 2 untouched
		d3 := f.deliveredTwoSkuOrder()
		f.holdPassed(d3.orderID)
		earning3 := f.orderEarning(d3.orderID)
		b2, err := p.CreatePayoutBatchForSeller(ctx, seller1)
		require.NoError(t, err)
		assert.Equal(t, earning3-earnM/2, b2.AmountCents)
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND seller_id = $2`, b2.ID, seller2), "no money of seller 2")
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND order_id = $2 AND type <> 'adjustment'`, b2.ID, d1.orderID), "already paid earning is not captured again")
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND order_id = $2 AND type = 'adjustment'`, b2.ID, d1.orderID), "the debt is recovered once")
		bal2, err := p.GetSellerBalance(ctx, seller2)
		require.NoError(t, err)
		assert.Equal(t, earning2, bal2.AvailableCents)
		f.assertConservation(p, seller1)
		f.assertConservation(p, seller2)
	})
}

// ============================================================================
// 3. CONCURRENCY AND FAILURES
// ============================================================================

func TestPayout_E2E1_Concurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("two workers release funds at once: each entry marked once, no new rows", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		d := f.deliveredTwoSkuOrder()
		f.holdPassed(d.orderID)
		rows := f.count(`SELECT count(*) FROM seller_ledger_entries`)
		var wg sync.WaitGroup
		res := make([]int, 4)
		for i := range res {
			wg.Add(1)
			go func(i int) { defer wg.Done(); res[i], _ = p.MakeSellerFundsAvailable(ctx, time.Now(), 100) }(i)
		}
		wg.Wait()
		assert.Equal(t, 2, res[0]+res[1]+res[2]+res[3])
		assert.Equal(t, rows, f.count(`SELECT count(*) FROM seller_ledger_entries`))
	})

	t.Run("two batch requests at once: one batch, each entry captured once", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		d := f.deliveredTwoSkuOrder()
		f.holdPassed(d.orderID)
		earning := f.orderEarning(d.orderID)
		var wg sync.WaitGroup
		errs := make([]error, 4)
		start := make(chan struct{})
		for i := range errs {
			wg.Add(1)
			go func(i int) { defer wg.Done(); <-start; _, errs[i] = p.CreatePayoutBatchForSeller(ctx, f.sellerID) }(i)
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
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM payout_batches WHERE seller_id = $1`, f.sellerID))
		assert.Equal(t, earning, f.sum(`SELECT amount_cents FROM payout_batches WHERE seller_id = $1`, f.sellerID))
		f.assertConservation(p, f.sellerID)
	})

	t.Run("two 'paid' marks of the same batch at once: one payout row", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		d := f.deliveredTwoSkuOrder()
		f.holdPassed(d.orderID)
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
		assert.Equal(t, 1, ok, "%v", errs)
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b.ID))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, b.AmountCents, bal.PaidCents)
	})

	t.Run("'paid' and 'hold' at the same time: one consistent outcome", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		d := f.deliveredTwoSkuOrder()
		f.holdPassed(d.orderID)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		var wg sync.WaitGroup
		var errPaid, errHold error
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; errPaid = p.ProcessPayoutBatch(ctx, b.ID) }()
		go func() { defer wg.Done(); <-start; errHold = p.HoldPayoutBatch(ctx, b.ID) }()
		close(start)
		wg.Wait()
		status := f.str(`SELECT status FROM payout_batches WHERE id = $1`, b.ID)
		payoutRows := f.count(`SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, b.ID)
		switch status {
		case "paid":
			assert.Equal(t, 1, payoutRows)
			assert.Error(t, errHold, "hold must not succeed on a batch that ended paid")
		case "held":
			assert.Equal(t, 0, payoutRows, "a held batch has no payout row")
			assert.Error(t, errPaid, "paid must not succeed on a batch that ended held")
		default:
			t.Fatalf("unexpected status %s (paid=%v hold=%v)", status, errPaid, errHold)
		}
	})

	t.Run("refund confirmation racing with batch creation: the deduction is counted exactly once", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		k := f.returnsKit()
		d := f.deliveredTwoSkuOrder()
		earning := f.orderEarning(d.orderID)
		earnM := f.sum(`SELECT amount_cents FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'seller_earning'`, d.items["M"])
		retID := f.createApproveAndArrive(k, d, "M", 1)
		f.receive(k, retID, d.units["M"][:1], "restock")
		_, err := k.svc.CreateRefund(ctx, f.adminID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		f.holdPassed(d.orderID)
		var wg sync.WaitGroup
		var errRefund, errBatch error
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _, errRefund = k.svc.SimulateRefundSuccess(ctx, retID) }()
		go func() { defer wg.Done(); <-start; _, errBatch = p.CreatePayoutBatchForSeller(ctx, f.sellerID) }()
		close(start)
		wg.Wait()
		require.NoError(t, errRefund)
		require.NoError(t, errBatch)
		assert.Equal(t, 1, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' IN ('return_deduction','return_post_payout')`, d.items["M"]))
		bal, err := p.GetSellerBalance(ctx, f.sellerID)
		require.NoError(t, err)
		batched := f.sum(`SELECT COALESCE(SUM(amount_cents),0) FROM payout_batches WHERE seller_id = $1`, f.sellerID)
		assert.Equal(t, earning-earnM/2, batched+bal.AvailableCents, "captured + remaining = earning − one deduction")
		f.assertConservation(p, f.sellerID)
	})

	t.Run("failure while creating a batch: nothing captured, then a clean batch", func(t *testing.T) {
		f := newFBOFixture(t)
		p := f.payoutSvc()
		d := f.deliveredTwoSkuOrder()
		f.holdPassed(d.orderID)
		testutil.MustBeDisposableDatabase(t, f.db)
		f.exec(`CREATE FUNCTION payout_e2e1_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'payout_e2e1 injected failure'; END $$`)
		f.exec(`CREATE TRIGGER payout_e2e1_fail BEFORE UPDATE OF payout_batch_id ON seller_ledger_entries FOR EACH ROW EXECUTE FUNCTION payout_e2e1_fail()`)
		_, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.Error(t, err)
		assert.False(t, errors.Is(err, context.Canceled))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM payout_batches WHERE seller_id = $1`, f.sellerID))
		assert.Equal(t, 0, f.count(`SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1 AND payout_batch_id IS NOT NULL`, f.sellerID))
		f.exec(`DROP TRIGGER payout_e2e1_fail ON seller_ledger_entries`)
		f.exec(`DROP FUNCTION payout_e2e1_fail()`)
		b, err := p.CreatePayoutBatchForSeller(ctx, f.sellerID)
		require.NoError(t, err)
		assert.Equal(t, f.orderEarning(d.orderID), b.AmountCents)
	})
}
