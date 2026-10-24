package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BANK.REFUND.1: the T-Bank acquiring refund adapter (POST /v2/Cancel, POST /v2/GetState).
//
// What the T-Bank contract gives, and what this adapter therefore claims:
//   - Cancel is signed like every EACQ request (root scalars + Password, sorted by key, SHA-256; Receipt is not
//     signed) and always carries ExternalRequestId = the attempt's idempotency key, so a repeated request is
//     recognised by the bank instead of refunding twice.
//   - The Cancel answer (Success, ErrorCode, Status, PaymentId, OriginalAmount, NewAmount) is the answer to the
//     request, not a settled result: a successful answer is only RefundSubmitAccepted.
//   - GetState reports the ORIGINAL PAYMENT's aggregate status (the request carries TerminalKey, PaymentId, Token only).
//     Conclusion, on the basis of the checked Cancel/GetState schema: there is no proven way to unambiguously confirm
//     one particular partial refund attempt (two partial refunds of a payment look the same). This is a statement
//     about the schema checked here, not proof that no such mechanism exists: T-Bank updated the ExternalRequestId
//     description (12.08.2026) and Receipt (14.08.2026), and an older SDK lacking a field proves nothing. Until the bank
//     contract confirms how a refund attempt is identified, such operations stay on manual review. Its answers are
//     RefundStateUnattributed: a status that shows money returned sends the attempt to manual review, nothing else
//     changes. No answer of this adapter ever settles a refund automatically; no undocumented field is used.
//   - A refusal (Success=false) is final only for ErrorCodes listed in DefinitiveErrorCodes (confirmed by T-Bank);
//     every other refusal, timeout, transport error or unreadable answer is ErrRefundOutcomeUnknown.
//   - SBP payments (refund status via GetQrState), mock payments and partial refunds that need a fiscal Receipt
//     ZAMK cannot build are refused by Preflight, before anything is sent.
// Nothing here logs; errors carry no Token, Password or request body.

const defaultTBankRefundTimeout = 15 * time.Second

type TBankRefundConfig struct {
	TerminalKey string
	Password    string
	APIBaseURL  string
	Timeout     time.Duration
	// ReceiptRequired: the terminal has an online cash register, so a partial Cancel needs a Receipt. ZAMK keeps no
	// fiscal receipt data, so partial refunds are then refused before sending (no placeholder Receipt is ever sent).
	ReceiptRequired bool
	// DefinitiveErrorCodes are Cancel ErrorCodes T-Bank has confirmed as final refusals. Empty by default: with no
	// confirmed code every refusal is an unknown outcome and the refund stays reserved. A wrongly listed code would let a
	// refund be retried with a new key while the bank may have executed the first one (a double refund), so a code is
	// added only on written confirmation from T-Bank.
	DefinitiveErrorCodes []string
}

type TBankRefundProvider struct {
	terminalKey     string
	password        string
	apiBaseURL      string
	receiptRequired bool
	definitive      map[string]bool
	client          *http.Client
}

func NewTBankRefundProvider(c TBankRefundConfig) (*TBankRefundProvider, error) {
	key := strings.TrimSpace(c.TerminalKey)
	if key == "" || key == "STUB" || strings.TrimSpace(c.Password) == "" || strings.TrimSpace(c.APIBaseURL) == "" {
		return nil, ErrProviderNotConfigured
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTBankRefundTimeout
	}
	definitive := map[string]bool{}
	for _, code := range c.DefinitiveErrorCodes {
		if code = strings.TrimSpace(code); code != "" && code != "0" {
			definitive[code] = true
		}
	}
	return &TBankRefundProvider{
		terminalKey:     key,
		password:        c.Password,
		apiBaseURL:      strings.TrimRight(c.APIBaseURL, "/"),
		receiptRequired: c.ReceiptRequired,
		definitive:      definitive,
		client:          &http.Client{Timeout: timeout},
	}, nil
}

func (p *TBankRefundProvider) Name() string { return "tbank" }

func notSendable(reason string) error { return fmt.Errorf("%w: %s", ErrRefundNotSendable, reason) }

func (p *TBankRefundProvider) Preflight(in RefundSubmitInput) error {
	switch {
	case strings.TrimSpace(in.IdempotencyKey) == "":
		return notSendable("empty ExternalRequestId would disable duplicate protection")
	case strings.TrimSpace(in.ProviderPaymentID) == "":
		return notSendable("no bank PaymentId")
	case in.Currency != "" && in.Currency != "RUB":
		return notSendable("only RUB payments are refunded")
	case in.IntegrationMode == "mock":
		return notSendable("the payment was made with the local mock: there is nothing to refund at the bank")
	case in.PaymentMethod == "sbp":
		return notSendable("SBP refunds are tracked with GetQrState, which is not implemented")
	case in.PaymentAmountCents <= 0:
		return notSendable("the original payment amount is unknown")
	case in.AmountCents <= 0 || in.AmountCents > in.PaymentAmountCents:
		return notSendable("the refund amount is outside the original payment")
	case p.receiptRequired && in.AmountCents < in.PaymentAmountCents:
		return notSendable("a partial refund needs a fiscal Receipt, which ZAMK does not hold")
	}
	return nil
}

type tbankCancelRequest struct {
	TerminalKey       string `json:"TerminalKey"`
	PaymentId         string `json:"PaymentId"`
	Amount            int64  `json:"Amount"`
	ExternalRequestId string `json:"ExternalRequestId"`
	Token             string `json:"Token"`
}

// tbankID accepts T-Bank ids sent either as JSON numbers or strings.
type tbankID string

func (id *tbankID) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		*id = ""
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*id = tbankID(v)
		return nil
	}
	if _, err := strconv.ParseInt(s, 10, 64); err != nil {
		return fmt.Errorf("unexpected PaymentId %q", s)
	}
	*id = tbankID(s)
	return nil
}

type tbankCancelResponse struct {
	Success        bool    `json:"Success"`
	ErrorCode      string  `json:"ErrorCode"`
	Message        string  `json:"Message"`
	Details        string  `json:"Details"`
	Status         string  `json:"Status"`
	PaymentId      tbankID `json:"PaymentId"`
	OriginalAmount *int64  `json:"OriginalAmount"`
	NewAmount      *int64  `json:"NewAmount"`
}

// cancelAcceptedStatuses are the Cancel answer statuses of a refund or reversal in progress or done.
var cancelAcceptedStatuses = map[string]bool{
	"REFUNDING": true, "REFUNDED": true, "PARTIAL_REFUNDED": true,
	"REVERSING": true, "REVERSED": true, "PARTIAL_REVERSED": true,
}

// refundObservedStatuses are aggregate payment statuses that show money returned to the customer.
var refundObservedStatuses = map[string]bool{
	"REFUNDED": true, "PARTIAL_REFUNDED": true, "REVERSED": true, "PARTIAL_REVERSED": true,
}

func (p *TBankRefundProvider) SubmitRefund(ctx context.Context, in RefundSubmitInput) (RefundSubmitResult, error) {
	if err := p.Preflight(in); err != nil {
		return RefundSubmitResult{}, err
	}
	amount := strconv.FormatInt(in.AmountCents, 10)
	req := tbankCancelRequest{
		TerminalKey:       p.terminalKey,
		PaymentId:         in.ProviderPaymentID,
		Amount:            in.AmountCents,
		ExternalRequestId: in.IdempotencyKey,
	}
	req.Token = tbankToken(p.password, map[string]string{
		"TerminalKey":       p.terminalKey,
		"PaymentId":         in.ProviderPaymentID,
		"Amount":            amount,
		"ExternalRequestId": in.IdempotencyKey,
	})

	var resp tbankCancelResponse
	if err := p.post(ctx, "Cancel", req, &resp); err != nil {
		return RefundSubmitResult{}, fmt.Errorf("%w: %w", ErrRefundOutcomeUnknown, err)
	}
	status := strings.ToUpper(strings.TrimSpace(resp.Status))
	if !resp.Success {
		code := strings.TrimSpace(resp.ErrorCode)
		if p.definitive[code] {
			return RefundSubmitResult{Decision: RefundSubmitRejected, ProviderStatus: status, FailureCode: code}, nil
		}
		return RefundSubmitResult{}, fmt.Errorf("%w: T-Bank refused Cancel with ErrorCode %q, not confirmed as final", ErrRefundOutcomeUnknown, code)
	}
	if string(resp.PaymentId) != in.ProviderPaymentID {
		return RefundSubmitResult{}, fmt.Errorf("%w: Cancel answer names another PaymentId", ErrRefundOutcomeUnknown)
	}
	if !cancelAcceptedStatuses[status] {
		return RefundSubmitResult{}, fmt.Errorf("%w: unrecognised Cancel status %q", ErrRefundOutcomeUnknown, status)
	}
	if resp.OriginalAmount != nil && resp.NewAmount != nil && *resp.OriginalAmount-*resp.NewAmount != in.AmountCents {
		return RefundSubmitResult{}, fmt.Errorf("%w: Cancel amounts (%d -> %d) do not match the refund of %d",
			ErrRefundOutcomeUnknown, *resp.OriginalAmount, *resp.NewAmount, in.AmountCents)
	}
	return RefundSubmitResult{Decision: RefundSubmitAccepted, ProviderStatus: status}, nil
}

type tbankGetStateRequest struct {
	TerminalKey string `json:"TerminalKey"`
	PaymentId   string `json:"PaymentId"`
	Token       string `json:"Token"`
}

type tbankGetStateResponse struct {
	Success   bool    `json:"Success"`
	ErrorCode string  `json:"ErrorCode"`
	Status    string  `json:"Status"`
	PaymentId tbankID `json:"PaymentId"`
	Amount    *int64  `json:"Amount"`
}

// GetRefundState reads the original payment with GetState. The answer is never attributed to an attempt.
func (p *TBankRefundProvider) GetRefundState(ctx context.Context, q RefundStateQuery) (RefundStateResult, error) {
	if strings.TrimSpace(q.ProviderPaymentID) == "" {
		return RefundStateResult{}, errors.New("tbank GetState: no PaymentId")
	}
	req := tbankGetStateRequest{TerminalKey: p.terminalKey, PaymentId: q.ProviderPaymentID}
	req.Token = tbankToken(p.password, map[string]string{"TerminalKey": p.terminalKey, "PaymentId": q.ProviderPaymentID})

	var resp tbankGetStateResponse
	if err := p.post(ctx, "GetState", req, &resp); err != nil {
		return RefundStateResult{}, err
	}
	if !resp.Success {
		return RefundStateResult{}, fmt.Errorf("tbank GetState refused with ErrorCode %q", strings.TrimSpace(resp.ErrorCode))
	}
	status := strings.ToUpper(strings.TrimSpace(resp.Status))
	res := RefundStateResult{Outcome: RefundStateUnattributed, ProviderPaymentID: string(resp.PaymentId), ProviderStatus: status}
	if resp.Amount != nil {
		res.AmountCents = *resp.Amount
	}
	switch {
	case string(resp.PaymentId) != q.ProviderPaymentID:
		res.ReviewRequired = true
		res.ReviewNote = "GetState answered for another PaymentId"
	case refundObservedStatuses[status]:
		res.ReviewRequired = true
		res.ReviewNote = fmt.Sprintf("payment status %s is aggregate: T-Bank GetState does not identify the refund attempt", status)
	}
	return res, nil
}

const tbankMaxResponseBytes = 1 << 20

// post sends one signed EACQ request. Errors describe the failure without the request body or any secret.
func (p *TBankRefundProvider) post(ctx context.Context, method string, req, out any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBaseURL+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("tbank %s: transport error: %w", method, unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tbank %s: unexpected HTTP status %d", method, resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, tbankMaxResponseBytes))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("tbank %s: unreadable response (content type %q)", method, resp.Header.Get("Content-Type"))
	}
	return nil
}

// unwrapURLError drops the *url.Error wrapper (method and URL) and keeps the cause, e.g. a deadline.
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}
