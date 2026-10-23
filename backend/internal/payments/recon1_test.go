package payments

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// PAY.RECON.1 on a disposable database: a signed bank CONFIRMED for a payment that is already failed locally is
// recorded and shown to the admin, and nothing else changes.

const bankConflict = "BANK_CONFIRMED_PAYMENT_NOT_SUCCEEDED"

func (f *paidFixture) problems() []string {
	f.t.Helper()
	d, err := f.svc.GetAdminPaymentDetail(f.ctx, f.payID)
	require.NoError(f.t, err)
	var codes []string
	for _, p := range d.Problems {
		codes = append(codes, p.Code)
	}
	return codes
}

func (f *paidFixture) listedWithConflict() int {
	f.t.Helper()
	items, _, err := f.svc.ListAdminPayments(f.ctx, "", "", "", "", "", "", bankConflict, "", "", 0, 0, false, "", "", 50, 0)
	require.NoError(f.t, err)
	return len(items)
}

// snapshot is everything that must stay as it is while the conflict is only being detected.
type snapshot struct {
	Counts       map[string]int
	OrderStatus  string
	PayStatus    string
	PaidAt       string
	FailedAt     string
	Reserved     int
	Active       int
	HistoryCount int
}

func (f *paidFixture) snap() snapshot {
	c := f.tableCounts()
	delete(c, "payment_events") // the recorded event is the one thing that may be added
	return snapshot{
		Counts:      c,
		OrderStatus: f.str(`SELECT status FROM orders WHERE id=$1`, f.orderID),
		PayStatus:   f.str(`SELECT status FROM payments WHERE id=$1`, f.payID),
		PaidAt:      f.str(`SELECT coalesce(paid_at::text,'') FROM payments WHERE id=$1`, f.payID),
		FailedAt:    f.str(`SELECT coalesce(failed_at::text,'') FROM payments WHERE id=$1`, f.payID),
		Reserved:    f.reserved(f.v),
		Active:      f.activeReservations(f.v),
	}
}

func (f *paidFixture) confirmedEvents() int {
	return f.count(`SELECT count(*) FROM payment_events WHERE payment_id=$1 AND event_type='CONFIRMED'`, f.payID)
}

// checkProvider answers the provider order check with fixed items and counts the calls.
type checkProvider struct {
	*TBankProvider
	items []ProviderPaymentItem
	calls int
}

func (c *checkProvider) CheckOrder(ctx context.Context, orderID string) (CheckOrderResult, error) {
	c.calls++
	return CheckOrderResult{OrderID: orderID, Payments: c.items}, nil
}

func TestPayRecon1(t *testing.T) {
	t.Run("normal success and normal failure show no problem", func(t *testing.T) {
		ok := newPaidFixture(t, 5)
		require.NoError(t, ok.svc.HandleWebhook(ok.ctx, nil, ok.signedWebhook("CONFIRMED", nil)))
		require.Empty(t, ok.problems())
		require.Equal(t, 0, ok.listedWithConflict())

		bad := newPaidFixture(t, 5)
		require.NoError(t, bad.mockAction("reject"))
		require.Empty(t, bad.problems())
		require.NoError(t, bad.svc.HandleWebhook(bad.ctx, nil, bad.signedWebhook("REJECTED", map[string]any{"Success": false, "ErrorCode": "1051"})))
		require.Empty(t, bad.problems(), "a bank rejection of a failed payment is no conflict")
		require.Equal(t, 0, bad.listedWithConflict())
	})

	t.Run("late valid CONFIRMED after failed: recorded, visible, nothing else changes", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		before := f.snap()
		require.Equal(t, 0, f.confirmedEvents())

		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", nil)))

		require.Equal(t, 1, f.confirmedEvents())
		require.Equal(t, []string{bankConflict}, f.problems())
		require.Equal(t, 1, f.listedWithConflict())
		require.Equal(t, before, f.snap(), "payment, order, stock, reservations and every other table are untouched")
		require.Equal(t, "failed", f.str(`SELECT status FROM payments WHERE id=$1`, f.payID))
		require.NotEqual(t, "paid", f.str(`SELECT status FROM orders WHERE id=$1`, f.orderID))
		for _, tbl := range []string{"refunds", "seller_ledger_entries", "payout_batches"} {
			require.Equal(t, 0, f.count(`SELECT count(*) FROM `+tbl))
		}
	})

	t.Run("replays and concurrent identical notifications leave one problem and one event", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		body := f.signedWebhook("CONFIRMED", nil)
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, body))
		before := f.snap()

		var wg sync.WaitGroup
		errs := make(chan error, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- f.svc.HandleWebhook(f.ctx, nil, body) }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.True(t, err == nil || errors.Is(err, ErrPaymentAlreadyProcessed), "%v", err)
		}
		require.Equal(t, 1, f.confirmedEvents(), "the same notification is stored once")
		require.Equal(t, []string{bankConflict}, f.problems(), "one problem, however many notifications")
		require.Equal(t, 1, f.listedWithConflict())

		// a differently keyed notification is one more event, still one problem
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", map[string]any{"CardId": int64(5)})))
		require.Equal(t, []string{bankConflict}, f.problems())
		require.Equal(t, before, f.snap())
	})

	t.Run("wrong signature and wrong amount record nothing", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		before := f.snap()
		var m map[string]any
		require.NoError(t, json.Unmarshal(f.signedWebhook("CONFIRMED", nil), &m))
		m["Token"] = "deadbeef"
		bad, _ := json.Marshal(m)
		require.ErrorIs(t, f.svc.HandleWebhook(f.ctx, nil, bad), ErrInvalidSignature)
		require.Error(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", map[string]any{"Amount": int64(100)})))
		require.Equal(t, 0, f.confirmedEvents())
		require.Empty(t, f.problems())
		require.Equal(t, before, f.snap())
	})

	t.Run("a notification that does not report a successful charge is no conflict", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, f.signedWebhook("CONFIRMED", map[string]any{"Success": false, "ErrorCode": "9999"})))
		require.Empty(t, f.problems())
	})

	t.Run("database error while recording the event: nothing is stored, the retry records the conflict", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		before := f.snap()
		f.exec(`CREATE FUNCTION recon1_fail() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$ LANGUAGE plpgsql`)
		f.exec(`CREATE TRIGGER recon1_fail BEFORE INSERT ON payment_events FOR EACH ROW EXECUTE FUNCTION recon1_fail()`)
		body := f.signedWebhook("CONFIRMED", nil)
		require.Error(t, f.svc.HandleWebhook(f.ctx, nil, body))
		require.Equal(t, 0, f.confirmedEvents())
		require.Empty(t, f.problems())
		require.Equal(t, before, f.snap())

		f.exec(`DROP TRIGGER recon1_fail ON payment_events`)
		require.NoError(t, f.svc.HandleWebhook(f.ctx, nil, body))
		require.Equal(t, []string{bankConflict}, f.problems())
	})

	t.Run("reconciliation finds the conflict once, asks the provider once, and does not touch anything", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		f.exec(`UPDATE payments SET integration_mode='hosted_form' WHERE id=$1`, f.payID) // a real (non-mock) payment
		providerID := f.str(`SELECT provider_payment_id FROM payments WHERE id=$1`, f.payID)
		amount := int64(f.count(`SELECT amount_cents FROM payments WHERE id=$1`, f.payID))
		before := f.snap()

		// the provider does not confirm it: nothing recorded
		cp := &checkProvider{TBankProvider: f.provider, items: []ProviderPaymentItem{{ProviderPaymentID: providerID, AmountCents: amount, ProviderStatus: "REJECTED", Success: false}}}
		svc := *f.svc
		svc.provider = cp
		n, err := svc.DetectLateConfirmations(f.ctx, 10)
		require.NoError(t, err)
		require.Equal(t, 0, n)
		require.Empty(t, f.problems())

		// confirmed for another amount: not trusted
		f.exec(`UPDATE payments SET reconciliation_attempted_at=NULL WHERE id=$1`, f.payID)
		cp.items = []ProviderPaymentItem{{ProviderPaymentID: providerID, AmountCents: amount + 1, ProviderStatus: "CONFIRMED", Success: true}}
		n, err = svc.DetectLateConfirmations(f.ctx, 10)
		require.NoError(t, err)
		require.Equal(t, 0, n)
		require.Empty(t, f.problems())

		// confirmed for this payment and amount: recorded
		f.exec(`UPDATE payments SET reconciliation_attempted_at=NULL WHERE id=$1`, f.payID)
		cp.items = []ProviderPaymentItem{{ProviderPaymentID: providerID, AmountCents: amount, ProviderStatus: "CONFIRMED", Success: true}}
		n, err = svc.DetectLateConfirmations(f.ctx, 10)
		require.NoError(t, err)
		require.Equal(t, 1, n)
		require.Equal(t, []string{bankConflict}, f.problems())
		require.Equal(t, 1, f.listedWithConflict())

		// a second run asks nobody and adds nothing; a fresh Service sees the same problem (state is in the database)
		calls := cp.calls
		n, err = svc.DetectLateConfirmations(f.ctx, 10)
		require.NoError(t, err)
		require.Equal(t, 0, n)
		require.Equal(t, calls, cp.calls)
		require.Equal(t, 1, f.confirmedEvents())
		require.Equal(t, []string{bankConflict}, f.problems())
		require.Equal(t, before, f.snap())
		require.Equal(t, int32(0), f.net.calls.Load(), "no real request to the bank")
	})

	t.Run("mock payments are never asked at the provider", func(t *testing.T) {
		f := newPaidFixture(t, 5)
		require.NoError(t, f.mockAction("reject"))
		cp := &checkProvider{TBankProvider: f.provider}
		svc := *f.svc
		svc.provider = cp
		_, err := svc.DetectLateConfirmations(f.ctx, 10)
		require.NoError(t, err)
		require.Equal(t, 0, cp.calls)
	})
}
