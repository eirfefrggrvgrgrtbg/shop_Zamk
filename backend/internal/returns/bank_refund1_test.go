package returns_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

// BANK.REFUND.1: the real T-Bank refund adapter end to end (returns service + DB), against a local httptest server
// answering like T-Bank EACQ v2. No request leaves the machine.

const (
	r1Terminal = "TESTTERMINAL"
	r1Password = "test-password-not-real"
)

type r1Bank struct {
	mu       sync.Mutex
	server   *httptest.Server
	counts   map[string]int
	cancel   string
	getState string
	delay    time.Duration
}

func newR1Bank(t *testing.T) *r1Bank {
	t.Helper()
	b := &r1Bank{counts: map[string]int{}}
	b.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		method := strings.TrimPrefix(r.URL.Path, "/v2/")
		b.mu.Lock()
		b.counts[method]++
		resp := map[string]string{"Cancel": b.cancel, "GetState": b.getState}[method]
		delay := b.delay
		b.mu.Unlock()
		if delay > 0 {
			time.Sleep(delay)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(b.server.Close)
	return b
}

func (b *r1Bank) set(cancel, getState string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cancel, b.getState = cancel, getState
}

func (b *r1Bank) count(method string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.counts[method]
}

func (b *r1Bank) provider(t *testing.T, receiptRequired bool, timeout time.Duration) *payments.TBankRefundProvider {
	t.Helper()
	p, err := payments.NewTBankRefundProvider(payments.TBankRefundConfig{
		TerminalKey: r1Terminal, Password: r1Password, APIBaseURL: b.server.URL + "/v2",
		Timeout: timeout, ReceiptRequired: receiptRequired, DefinitiveErrorCodes: []string{"FINAL-1"},
	})
	require.NoError(t, err)
	return p
}

func cancelJSON(paymentID, status string, orig, newAmount int64) string {
	return fmt.Sprintf(`{"Success":true,"ErrorCode":"0","TerminalKey":%q,"Status":%q,"PaymentId":%q,"OriginalAmount":%d,"NewAmount":%d}`,
		r1Terminal, status, paymentID, orig, newAmount)
}

func getStateJSON(paymentID, status string, amount int64) string {
	return fmt.Sprintf(`{"Success":true,"ErrorCode":"0","TerminalKey":%q,"Status":%q,"PaymentId":%q,"Amount":%d}`,
		r1Terminal, status, paymentID, amount)
}

// setProviderPaymentID gives the test payment a numeric T-Bank PaymentId, as real payments have.
func setProviderPaymentID(t *testing.T, fix *m51Fixture, o *bankRefundOrder) {
	t.Helper()
	o.providerPaymentID = fmt.Sprintf("%d", 700000000+time.Now().UnixNano()%99999999)
	_, err := fix.client.Pool.Exec(context.Background(), `UPDATE payments SET provider_payment_id = $2 WHERE id = $1`, o.paymentID, o.providerPaymentID)
	require.NoError(t, err)
}

func assertNoFinancialSuccess(t *testing.T, fix *m51Fixture, o bankRefundOrder, refundID, retID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "pending", readRefundRow(t, fix, refundID).Status)
	assert.Equal(t, "item_received", returnStatus(t, fix, retID))
	n, _ := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 0, n, "no seller deduction without a confirmed refund")
	var paymentStatus string
	require.NoError(t, fix.client.Pool.QueryRow(context.Background(), `SELECT status FROM payments WHERE id = $1`, o.paymentID).Scan(&paymentStatus))
	assert.Equal(t, "succeeded", paymentStatus, "the original payment is never rewritten")
}

// E + K: accepted by T-Bank, then GetState shows the aggregate refund status: review, never success.
func TestBankRefund1_AggregateStateGoesToReview(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	bank := newR1Bank(t)
	fix.svc.WithRefundProvider(bank.provider(t, false, 2*time.Second))
	o := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o)
	ret1 := newReceivedReturn(t, fix, o, 1)
	ret2 := newReceivedReturn(t, fix, o, 1)

	ref1, err := fix.svc.CreateRefund(ctx, fix.userID, ret1, returns.CreateRefundRequest{})
	require.NoError(t, err)
	ref2, err := fix.svc.CreateRefund(ctx, fix.userID, ret2, returns.CreateRefundRequest{})
	require.NoError(t, err)

	bank.set(cancelJSON(o.providerPaymentID, "PARTIAL_REFUNDED", 3000, 2000), "")
	_, err = fix.svc.SubmitRefund(ctx, ref1.ID)
	require.NoError(t, err)
	bank.set(cancelJSON(o.providerPaymentID, "PARTIAL_REFUNDED", 2000, 1000), "")
	_, err = fix.svc.SubmitRefund(ctx, ref2.ID)
	require.NoError(t, err)
	assert.Equal(t, "accepted", readRefundRow(t, fix, ref1.ID).SubmitOutcome)
	assert.Equal(t, "accepted", readRefundRow(t, fix, ref2.ID).SubmitOutcome)
	assert.Equal(t, 2, bank.count("Cancel"))

	// Before any refund shows on the payment: nothing learned, no review.
	bank.set("", getStateJSON(o.providerPaymentID, "CONFIRMED", 3000))
	_, err = fix.svc.ReconcileRefund(ctx, ref1.ID)
	require.NoError(t, err)
	assert.Nil(t, readRefundRow(t, fix, ref1.ID).ReviewReason)

	// The payment shows PARTIAL_REFUNDED: it cannot tell which attempt -> both to review, neither succeeds.
	bank.set("", getStateJSON(o.providerPaymentID, "PARTIAL_REFUNDED", 3000))

	// Identity of the two attempts before the batch: ids, keys and amounts must come out unchanged and unswapped.
	type refundIdentity struct {
		Key    string
		Amount int64
	}
	identity := func(id uuid.UUID) refundIdentity {
		var r refundIdentity
		require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT idempotency_key, amount_cents FROM refunds WHERE id = $1`, id).Scan(&r.Key, &r.Amount))
		return r
	}
	id1, id2 := identity(ref1.ID), identity(ref2.ID)
	require.NotEqual(t, id1.Key, id2.Key)
	require.Equal(t, int64(1000), id1.Amount)
	require.Equal(t, int64(1000), id2.Amount)

	// Other tests of this database may have left unresolved refunds: remember them, they are not ours to settle.
	type otherRefund struct {
		Status, Outcome, Key string
		Amount               int64
	}
	others := map[uuid.UUID]otherRefund{}
	rows, err := fix.client.Pool.Query(ctx, `SELECT id, status, submit_outcome, COALESCE(idempotency_key, ''), amount_cents FROM refunds WHERE id NOT IN ($1, $2)`, ref1.ID, ref2.ID)
	require.NoError(t, err)
	for rows.Next() {
		var id uuid.UUID
		var o otherRefund
		require.NoError(t, rows.Scan(&id, &o.Status, &o.Outcome, &o.Key, &o.Amount))
		others[id] = o
	}
	require.NoError(t, rows.Err())
	rows.Close()
	var succeededBefore, adjustmentsBefore int
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT count(*) FROM refunds WHERE status IN ('succeeded', 'completed')`).Scan(&succeededBefore))
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE type = 'adjustment'`).Scan(&adjustmentsBefore))

	// A large limit: both of our attempts are certainly inside the batch, wherever the other rows sort.
	checked, err := fix.svc.ReconcileUnresolvedRefunds(ctx, 10000)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, checked, 2, "at least our two attempts were checked (others may be in the batch too)")

	// Our two attempts, by id: processed, to review, not swapped, nothing settled.
	for _, r := range []struct {
		ref, ret uuid.UUID
		want     refundIdentity
	}{{ref1.ID, ret1, id1}, {ref2.ID, ret2, id2}} {
		row := readRefundRow(t, fix, r.ref)
		assert.Equal(t, "accepted", row.SubmitOutcome)
		require.NotNil(t, row.ReviewReason, "refund %s was processed by the batch", r.ref)
		assert.Contains(t, *row.ReviewReason, "aggregate")
		assert.Equal(t, r.want, identity(r.ref), "key and amount of %s are unchanged", r.ref)
		assert.Equal(t, 1, row.Attempts)
		var checkedAt *time.Time
		require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT last_checked_at FROM refunds WHERE id = $1`, r.ref).Scan(&checkedAt))
		assert.NotNil(t, checkedAt, "refund %s was asked at the provider", r.ref)
		assertNoFinancialSuccess(t, fix, o, r.ref, r.ret)
	}
	assert.Equal(t, 2, bank.count("Cancel"), "reconciliation never sends a Cancel")

	// Foreign refunds: identity and amount untouched, no success; at most they were set aside for review.
	for id, before := range others {
		var now otherRefund
		require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT status, submit_outcome, COALESCE(idempotency_key, ''), amount_cents FROM refunds WHERE id = $1`, id).Scan(&now.Status, &now.Outcome, &now.Key, &now.Amount))
		assert.Equal(t, before, now, "foreign refund %s changed unexpectedly", id)
	}
	var succeededAfter, adjustmentsAfter int
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT count(*) FROM refunds WHERE status IN ('succeeded', 'completed')`).Scan(&succeededAfter))
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE type = 'adjustment'`).Scan(&adjustmentsAfter))
	assert.Equal(t, succeededBefore, succeededAfter, "the batch settled no refund")
	assert.Equal(t, adjustmentsBefore, adjustmentsAfter, "no financial adjustment came from the batch")

	// Even a full REFUNDED does not settle a reviewed attempt.
	bank.set("", getStateJSON(o.providerPaymentID, "REFUNDED", 3000))
	_, err = fix.svc.ReconcileRefund(ctx, ref1.ID)
	assert.ErrorIs(t, err, returns.ErrRefundUnderReview)
	assertNoFinancialSuccess(t, fix, o, ref1.ID, ret1)
}

// D + C: a timeout leaves the attempt unknown; repeating never sends a second Cancel.
func TestBankRefund1_TimeoutThenRepeat(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	bank := newR1Bank(t)
	fix.svc.WithRefundProvider(bank.provider(t, true, 50*time.Millisecond))
	o := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o)
	retID := newReceivedReturn(t, fix, o, 3) // a full refund: no Receipt needed

	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	bank.set(cancelJSON(o.providerPaymentID, "REFUNDED", 3000, 0), "")
	bank.delay = 300 * time.Millisecond
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)
	assertNoFinancialSuccess(t, fix, o, ref.ID, retID)

	bank.delay = 0
	for i := 0; i < 3; i++ {
		_, err = fix.svc.SubmitRefund(ctx, ref.ID)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, bank.count("Cancel"), "an unknown attempt is never resent")
	assert.Equal(t, 1, readRefundRow(t, fix, ref.ID).Attempts)
}

// F: a refusal with a confirmed final ErrorCode fails the attempt; an unconfirmed code stays unknown.
func TestBankRefund1_DefinitiveAndUnconfirmedRefusal(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	bank := newR1Bank(t)
	fix.svc.WithRefundProvider(bank.provider(t, false, 2*time.Second))

	o := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o)
	retID := newReceivedReturn(t, fix, o, 3)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	bank.set(`{"Success":false,"ErrorCode":"FINAL-1","Message":"declined"}`, "")
	got, err := fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", got.Status)
	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "rejected", row.SubmitOutcome)
	assert.Equal(t, "FINAL-1", *row.FailureCode)
	n, _ := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 0, n)

	o2 := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o2)
	ret2 := newReceivedReturn(t, fix, o2, 3)
	ref2, err := fix.svc.CreateRefund(ctx, fix.userID, ret2, returns.CreateRefundRequest{})
	require.NoError(t, err)
	bank.set(`{"Success":false,"ErrorCode":"9999","Message":"internal"}`, "")
	_, err = fix.svc.SubmitRefund(ctx, ref2.ID)
	require.NoError(t, err)
	assert.Equal(t, "unknown", readRefundRow(t, fix, ref2.ID).SubmitOutcome, "an unconfirmed refusal code is not a failure")
	assertNoFinancialSuccess(t, fix, o2, ref2.ID, ret2)
}

// L + mock + legacy: nothing is sent when it cannot be sent safely; the refund stays reserved and unsent.
func TestBankRefund1_PreflightKeepsRefundUnsent(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	bank := newR1Bank(t)
	bank.set(cancelJSON("1", "REFUNDED", 3000, 0), "")
	fix.svc.WithRefundProvider(bank.provider(t, true, 2*time.Second))

	o := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o)
	partial := newReceivedReturn(t, fix, o, 1)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, partial, returns.CreateRefundRequest{})
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	assert.ErrorIs(t, err, payments.ErrRefundNotSendable, "a partial refund without fiscal Receipt data is not sent")
	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "not_sent", row.SubmitOutcome)
	assert.Equal(t, 0, row.Attempts)

	om := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &om)
	_, err = fix.client.Pool.Exec(ctx, `UPDATE payments SET payment_method = 'tpay', integration_mode = 'mock' WHERE id = $1`, om.paymentID)
	require.NoError(t, err)
	retM := newReceivedReturn(t, fix, om, 3)
	refM, err := fix.svc.CreateRefund(ctx, fix.userID, retM, returns.CreateRefundRequest{})
	require.NoError(t, err)
	_, err = fix.svc.SubmitRefund(ctx, refM.ID)
	assert.ErrorIs(t, err, payments.ErrRefundNotSendable, "a mock payment is never refunded at the bank")
	assert.Equal(t, "not_sent", readRefundRow(t, fix, refM.ID).SubmitOutcome)

	assert.Equal(t, 0, bank.count("Cancel"))
}

// O: the bank is unreachable: the attempt is unknown, a later check errors and changes nothing.
func TestBankRefund1_BankUnavailable(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	bank := newR1Bank(t)
	fix.svc.WithRefundProvider(bank.provider(t, false, 2*time.Second))
	o := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o)
	retID := newReceivedReturn(t, fix, o, 3)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	bank.server.Close()

	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "unknown", readRefundRow(t, fix, ref.ID).SubmitOutcome)
	_, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.Error(t, err)
	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "unknown", row.SubmitOutcome)
	assert.Nil(t, row.ReviewReason)
	assertNoFinancialSuccess(t, fix, o, ref.ID, retID)
}

// signedWebhook builds a T-Bank notification signed like VerifyWebhook checks it.
func signedWebhook(t *testing.T, fields map[string]any) []byte {
	t.Helper()
	params := map[string]string{}
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			params[k] = val
		case int64:
			params[k] = fmt.Sprintf("%d", val)
		case bool:
			params[k] = fmt.Sprintf("%t", val)
		}
	}
	params["Password"] = r1Password
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(params[k])
	}
	sum := sha256.Sum256([]byte(sb.String()))
	fields["Token"] = hex.EncodeToString(sum[:])
	b, err := json.Marshal(fields)
	require.NoError(t, err)
	return b
}

// M + N: a refund notification is recorded once for reconciliation; it changes neither the payment nor the refund,
// and a GetState that contradicts it changes nothing either.
func TestBankRefund1_RefundWebhookIsRecordedOnly(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	bank := newR1Bank(t)
	fix.svc.WithRefundProvider(bank.provider(t, false, 2*time.Second))
	o := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o)
	retID := newReceivedReturn(t, fix, o, 1)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	bank.set(cancelJSON(o.providerPaymentID, "PARTIAL_REFUNDED", 3000, 2000), "")
	_, err = fix.svc.SubmitRefund(ctx, ref.ID)
	require.NoError(t, err)

	paySvc := payments.NewService(payments.NewRepository(fix.client.Pool), fix.ordersRepo, nil,
		payments.NewTBankProvider(r1Terminal, r1Password, bank.server.URL+"/v2", "", "", false, "O", "hosted_form"),
		fix.client, nil, nil, &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}})
	var pid int64
	_, err = fmt.Sscanf(o.providerPaymentID, "%d", &pid)
	require.NoError(t, err)
	body := signedWebhook(t, map[string]any{
		"TerminalKey": r1Terminal, "OrderId": o.ord.orderID.String(), "Success": true, "Status": "PARTIAL_REFUNDED",
		"PaymentId": pid, "ErrorCode": "0", "Amount": int64(2000),
	})
	for i := 0; i < 2; i++ {
		require.NoError(t, paySvc.HandleWebhook(ctx, map[string]string{}, body), "repeat %d", i)
	}
	var events int
	require.NoError(t, fix.client.Pool.QueryRow(ctx, `
		SELECT count(*) FROM payment_events WHERE payment_id = $1 AND event_type = 'PARTIAL_REFUNDED'
	`, o.paymentID).Scan(&events))
	assert.Equal(t, 1, events, "the notification is kept once")
	assert.Equal(t, "accepted", readRefundRow(t, fix, ref.ID).SubmitOutcome, "a notification settles no refund")
	assertNoFinancialSuccess(t, fix, o, ref.ID, retID)

	// A forged notification is still refused.
	forged := strings.Replace(string(body), "PARTIAL_REFUNDED", "REFUNDED", 1)
	assert.ErrorIs(t, paySvc.HandleWebhook(ctx, map[string]string{}, []byte(forged)), payments.ErrInvalidSignature)

	// GetState contradicts the notification (still CONFIRMED): nothing changes.
	bank.set("", getStateJSON(o.providerPaymentID, "CONFIRMED", 3000))
	_, err = fix.svc.ReconcileRefund(ctx, ref.ID)
	require.NoError(t, err)
	row := readRefundRow(t, fix, ref.ID)
	assert.Equal(t, "accepted", row.SubmitOutcome)
	assert.Nil(t, row.ReviewReason)
	assertNoFinancialSuccess(t, fix, o, ref.ID, retID)
}

// With refunds enabled the simulator stays off, and enabling needs the explicit switch.
func TestBankRefund1_EnablementIsExplicit(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{Env: "development"}}
	cfg.TBank.TerminalKey = "STUB"
	cfg.TBank.Password = "STUB"
	assert.True(t, cfg.RefundSimulatorAllowed())
	cfg.TBank.RefundsEnabled = true
	assert.False(t, cfg.RefundSimulatorAllowed(), "with real refunds switched on the simulator is off")
}
