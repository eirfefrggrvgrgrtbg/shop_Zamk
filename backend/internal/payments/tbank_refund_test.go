package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BANK.REFUND.1: the T-Bank refund adapter against a local httptest server answering like T-Bank EACQ v2.
// No test here reaches a real bank URL.

const (
	stubTerminal = "TESTTERMINAL"
	stubPassword = "test-password-not-real"
)

type stubTBank struct {
	mu       sync.Mutex
	server   *httptest.Server
	requests map[string][]map[string]any
	cancel   func(req map[string]any) (int, string)
	getState func(req map[string]any) (int, string)
	delay    time.Duration
}

func newStubTBank(t *testing.T) *stubTBank {
	t.Helper()
	s := &stubTBank{requests: map[string][]map[string]any{}}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		method := strings.TrimPrefix(r.URL.Path, "/v2/")
		s.mu.Lock()
		s.requests[method] = append(s.requests[method], req)
		handler := map[string]func(map[string]any) (int, string){"Cancel": s.cancel, "GetState": s.getState}[method]
		delay := s.delay
		s.mu.Unlock()
		if delay > 0 {
			time.Sleep(delay)
		}
		if handler == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		code, resp := handler(req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *stubTBank) calls(method string) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.requests[method]...)
}

func (s *stubTBank) provider(t *testing.T, mutate ...func(*TBankRefundConfig)) *TBankRefundProvider {
	t.Helper()
	c := TBankRefundConfig{TerminalKey: stubTerminal, Password: stubPassword, APIBaseURL: s.server.URL + "/v2", Timeout: 2 * time.Second}
	for _, m := range mutate {
		m(&c)
	}
	p, err := NewTBankRefundProvider(c)
	require.NoError(t, err)
	return p
}

// expectedToken recomputes the EACQ signature independently: root scalars + Password, by key, SHA-256.
func expectedToken(params map[string]string) string {
	params["Password"] = stubPassword
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
	return hex.EncodeToString(sum[:])
}

func okCancel(paymentID string, status string, orig, newAmount int64) func(map[string]any) (int, string) {
	return func(map[string]any) (int, string) {
		b, _ := json.Marshal(map[string]any{"Success": true, "ErrorCode": "0", "TerminalKey": stubTerminal, "OrderId": "o-1",
			"Status": status, "PaymentId": paymentID, "OriginalAmount": orig, "NewAmount": newAmount})
		return http.StatusOK, string(b)
	}
}

func stateAnswer(paymentID, status string, amount int64) func(map[string]any) (int, string) {
	return func(map[string]any) (int, string) {
		b, _ := json.Marshal(map[string]any{"Success": true, "ErrorCode": "0", "TerminalKey": stubTerminal, "OrderId": "o-1",
			"Status": status, "PaymentId": paymentID, "Amount": amount})
		return http.StatusOK, string(b)
	}
}

func refundInput(key string, amount, paid int64) RefundSubmitInput {
	return RefundSubmitInput{ProviderPaymentID: "700000001", AmountCents: amount, Currency: "RUB", IdempotencyKey: key,
		PaymentAmountCents: paid, PaymentMethod: "card", IntegrationMode: "hosted_form"}
}

func assertUnknown(t *testing.T, res RefundSubmitResult, err error) {
	t.Helper()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRefundOutcomeUnknown)
	assert.Empty(t, res.Decision, "an unknown outcome is neither accepted nor rejected")
	assert.NotContains(t, err.Error(), stubPassword)
}

// A + B + J: Cancel JSON, signature, kopecks, ExternalRequestId; a partial answer is only "accepted".
func TestTBankRefund_A_B_J_CancelRequestAndSignature(t *testing.T) {
	bank := newStubTBank(t)
	bank.cancel = okCancel("700000001", "PARTIAL_REFUNDED", 5000, 3500)
	p := bank.provider(t)

	res, err := p.SubmitRefund(context.Background(), refundInput("key-abc", 1500, 5000))
	require.NoError(t, err)
	assert.Equal(t, RefundSubmitAccepted, res.Decision, "a Cancel answer is never a confirmed refund")
	assert.Equal(t, "PARTIAL_REFUNDED", res.ProviderStatus)

	calls := bank.calls("Cancel")
	require.Len(t, calls, 1)
	req := calls[0]
	keys := make([]string, 0, len(req))
	for k := range req {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	assert.Equal(t, []string{"Amount", "ExternalRequestId", "PaymentId", "TerminalKey", "Token"}, keys, "no Receipt, no extra fields")
	assert.Equal(t, stubTerminal, req["TerminalKey"])
	assert.Equal(t, "700000001", req["PaymentId"])
	assert.Equal(t, float64(1500), req["Amount"], "the amount travels in kopecks as a JSON number")
	assert.Equal(t, "key-abc", req["ExternalRequestId"])
	assert.Equal(t, expectedToken(map[string]string{
		"TerminalKey": stubTerminal, "PaymentId": "700000001", "Amount": "1500", "ExternalRequestId": "key-abc",
	}), req["Token"])
}

// B: an empty ExternalRequestId is never sent.
func TestTBankRefund_B_EmptyKeyRefusedBeforeSending(t *testing.T) {
	bank := newStubTBank(t)
	bank.cancel = okCancel("700000001", "REFUNDED", 5000, 0)
	_, err := bank.provider(t).SubmitRefund(context.Background(), refundInput("", 5000, 5000))
	assert.ErrorIs(t, err, ErrRefundNotSendable)
	assert.Empty(t, bank.calls("Cancel"))
}

// C: the same attempt always carries the same ExternalRequestId (the bank's duplicate protection).
func TestTBankRefund_C_RepeatCarriesSameKey(t *testing.T) {
	bank := newStubTBank(t)
	bank.cancel = okCancel("700000001", "REFUNDED", 5000, 0)
	p := bank.provider(t)
	for i := 0; i < 2; i++ {
		_, err := p.SubmitRefund(context.Background(), refundInput("key-same", 5000, 5000))
		require.NoError(t, err)
	}
	calls := bank.calls("Cancel")
	require.Len(t, calls, 2)
	assert.Equal(t, calls[0]["ExternalRequestId"], calls[1]["ExternalRequestId"])
	assert.Equal(t, calls[0]["Token"], calls[1]["Token"])
}

// D: a timeout before the answer is an unknown outcome, never a refusal.
func TestTBankRefund_D_TimeoutIsUnknown(t *testing.T) {
	bank := newStubTBank(t)
	bank.cancel = okCancel("700000001", "REFUNDED", 5000, 0)
	bank.delay = 300 * time.Millisecond
	p := bank.provider(t, func(c *TBankRefundConfig) { c.Timeout = 50 * time.Millisecond })
	res, err := p.SubmitRefund(context.Background(), refundInput("key-t", 5000, 5000))
	assertUnknown(t, res, err)
}

// E: accepted without completion; GetState without a refund status says nothing.
func TestTBankRefund_E_AcceptedNotConfirmed(t *testing.T) {
	bank := newStubTBank(t)
	bank.cancel = okCancel("700000001", "REFUNDING", 5000, 5000)
	bank.getState = stateAnswer("700000001", "CONFIRMED", 5000)
	p := bank.provider(t)
	res, err := p.SubmitRefund(context.Background(), refundInput("key-e", 5000, 5000))
	// REFUNDING with unchanged amounts: Original-New (0) differs from the requested 5000 -> not even accepted.
	assertUnknown(t, res, err)

	bank.cancel = func(map[string]any) (int, string) {
		return http.StatusOK, `{"Success":true,"ErrorCode":"0","Status":"REFUNDING","PaymentId":700000001}`
	}
	res, err = p.SubmitRefund(context.Background(), refundInput("key-e", 5000, 5000))
	require.NoError(t, err)
	assert.Equal(t, RefundSubmitAccepted, res.Decision)

	st, err := p.GetRefundState(context.Background(), RefundStateQuery{ProviderPaymentID: "700000001", IdempotencyKey: "key-e", AmountCents: 5000})
	require.NoError(t, err)
	assert.Equal(t, RefundStateUnattributed, st.Outcome)
	assert.False(t, st.ReviewRequired)
	assert.Empty(t, st.IdempotencyKey, "GetState never names the attempt")
	req := bank.calls("GetState")[0]
	assert.Equal(t, expectedToken(map[string]string{"TerminalKey": stubTerminal, "PaymentId": "700000001"}), req["Token"])
}

// F: a refusal is final only for a confirmed ErrorCode; any other refusal is unknown.
func TestTBankRefund_F_DefinitiveRefusal(t *testing.T) {
	bank := newStubTBank(t)
	bank.cancel = func(map[string]any) (int, string) {
		return http.StatusOK, `{"Success":false,"ErrorCode":"FINAL-1","Message":"declined","PaymentId":"700000001"}`
	}
	res, err := bank.provider(t, func(c *TBankRefundConfig) { c.DefinitiveErrorCodes = []string{"FINAL-1"} }).
		SubmitRefund(context.Background(), refundInput("key-f", 5000, 5000))
	require.NoError(t, err)
	assert.Equal(t, RefundSubmitRejected, res.Decision)
	assert.Equal(t, "FINAL-1", res.FailureCode)

	res, err = bank.provider(t).SubmitRefund(context.Background(), refundInput("key-f2", 5000, 5000))
	assertUnknown(t, res, err)
}

// G + H + I: unknown status, HTML / HTTP errors, a foreign PaymentId: all unknown.
func TestTBankRefund_G_H_I_UnusableAnswersAreUnknown(t *testing.T) {
	cases := map[string]func(map[string]any) (int, string){
		"unknown_status": okCancel("700000001", "SOMETHING_NEW", 5000, 0),
		"html_200":       func(map[string]any) (int, string) { return http.StatusOK, "<html><body>maintenance</body></html>" },
		"http_502":       func(map[string]any) (int, string) { return http.StatusBadGateway, `{"Success":true}` },
		"empty_body":     func(map[string]any) (int, string) { return http.StatusOK, "" },
		"other_payment":  okCancel("700000999", "REFUNDED", 5000, 0),
	}
	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			bank := newStubTBank(t)
			bank.cancel = answer
			res, err := bank.provider(t).SubmitRefund(context.Background(), refundInput("key-"+name, 5000, 5000))
			assertUnknown(t, res, err)
			assert.NotContains(t, err.Error(), "maintenance", "the response body is not echoed")
		})
	}

	bank := newStubTBank(t)
	bank.getState = stateAnswer("700000999", "CONFIRMED", 5000)
	st, err := bank.provider(t).GetRefundState(context.Background(), RefundStateQuery{ProviderPaymentID: "700000001", IdempotencyKey: "k", AmountCents: 5000})
	require.NoError(t, err)
	assert.Equal(t, RefundStateUnattributed, st.Outcome)
	assert.True(t, st.ReviewRequired, "an answer about another payment is a contradiction for review")
}

// K: the aggregate PARTIAL_REFUNDED / REFUNDED never identifies an attempt.
func TestTBankRefund_K_AggregateStateIsUnattributed(t *testing.T) {
	for _, status := range []string{"PARTIAL_REFUNDED", "REFUNDED", "REVERSED", "PARTIAL_REVERSED"} {
		bank := newStubTBank(t)
		bank.getState = stateAnswer("700000001", status, 5000)
		st, err := bank.provider(t).GetRefundState(context.Background(), RefundStateQuery{ProviderPaymentID: "700000001", IdempotencyKey: "k1", AmountCents: 1000})
		require.NoError(t, err)
		assert.Equal(t, RefundStateUnattributed, st.Outcome, status)
		assert.True(t, st.ReviewRequired, status)
		assert.Empty(t, st.IdempotencyKey, status)
		assert.NotContains(t, st.ReviewNote, stubPassword)
	}
}

// L + preflight: a partial refund needing a Receipt, a mock payment, an SBP payment, a bad amount: never sent.
func TestTBankRefund_L_PreflightStopsBeforeSending(t *testing.T) {
	bank := newStubTBank(t)
	bank.cancel = okCancel("700000001", "REFUNDED", 5000, 0)
	withReceipt := bank.provider(t, func(c *TBankRefundConfig) { c.ReceiptRequired = true })

	_, err := withReceipt.SubmitRefund(context.Background(), refundInput("key-l", 1000, 5000))
	assert.ErrorIs(t, err, ErrRefundNotSendable, "no placeholder Receipt is sent")

	mock := refundInput("key-m", 5000, 5000)
	mock.IntegrationMode = "mock"
	sbp := refundInput("key-s", 5000, 5000)
	sbp.PaymentMethod = "sbp"
	for name, in := range map[string]RefundSubmitInput{
		"mock": mock, "sbp": sbp, "over_paid": refundInput("key-o", 6000, 5000), "zero": refundInput("key-z", 0, 5000),
		"unknown_paid": refundInput("key-u", 1000, 0),
	} {
		_, err := bank.provider(t).SubmitRefund(context.Background(), in)
		assert.ErrorIs(t, err, ErrRefundNotSendable, name)
	}
	assert.Empty(t, bank.calls("Cancel"))

	// A full refund needs no Receipt.
	_, err = withReceipt.SubmitRefund(context.Background(), refundInput("key-full", 5000, 5000))
	require.NoError(t, err)
	assert.Len(t, bank.calls("Cancel"), 1)
}

// O: the bank API is unreachable.
func TestTBankRefund_O_BankUnavailable(t *testing.T) {
	bank := newStubTBank(t)
	p := bank.provider(t)
	bank.server.Close()
	res, err := p.SubmitRefund(context.Background(), refundInput("key-o", 5000, 5000))
	assertUnknown(t, res, err)
	_, err = p.GetRefundState(context.Background(), RefundStateQuery{ProviderPaymentID: "700000001", IdempotencyKey: "key-o", AmountCents: 5000})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrRefundNotSendable))
}

// A real refund adapter needs a real terminal; the local STUB or empty credentials build none.
func TestTBankRefund_RequiresRealTerminal(t *testing.T) {
	for _, c := range []TBankRefundConfig{
		{TerminalKey: "STUB", Password: "STUB", APIBaseURL: "http://127.0.0.1:1"},
		{TerminalKey: "", Password: "x", APIBaseURL: "http://127.0.0.1:1"},
		{TerminalKey: "T", Password: "", APIBaseURL: "http://127.0.0.1:1"},
		{TerminalKey: "T", Password: "x", APIBaseURL: ""},
	} {
		_, err := NewTBankRefundProvider(c)
		assert.ErrorIs(t, err, ErrProviderNotConfigured)
	}
}

// Refund notifications are their own category: never success, never failure of the payment.
func TestTBankRefund_RefundStatusesClassified(t *testing.T) {
	for _, s := range []string{"REFUNDING", "REFUNDED", "PARTIAL_REFUNDED", "REVERSING", "REVERSED", "PARTIAL_REVERSED"} {
		assert.Equal(t, ProviderStatusCategoryRefund, ClassifyProviderStatus(s), s)
	}
	assert.Equal(t, ProviderStatusCategorySuccess, ClassifyProviderStatus("CONFIRMED"))
}
