package payments

import (
	"context"
	"errors"
)

// BANK.REFUND.0: the contract a bank refund adapter fulfils (FakeRefundProvider; TBankRefundProvider since
// BANK.REFUND.1, wired only behind an explicit, default-off switch).
//
// The answer to SubmitRefund is never a confirmation of the money movement: the refund is final only when
// GetRefundState attributes a terminal result to this very attempt (its idempotency key, payment and amount).

// ErrRefundOutcomeUnknown: the submit call ended without a usable answer (timeout, transport error, unreadable
// response). The provider may or may not have executed the refund.
var ErrRefundOutcomeUnknown = errors.New("refund submission outcome unknown")

// ErrRefundNotSendable: Preflight refused the attempt; nothing was sent and the refund stays reserved, unsent.
var ErrRefundNotSendable = errors.New("refund cannot be sent to the provider")

type RefundSubmitInput struct {
	// ProviderPaymentID is the bank id of the original payment (payments.provider_payment_id).
	ProviderPaymentID string
	AmountCents       int64
	Currency          string
	// IdempotencyKey is never empty: an empty key would disable the provider's duplicate protection.
	IdempotencyKey string
	// The original payment: its full amount (a smaller AmountCents is a partial refund), method and integration.
	PaymentAmountCents int64
	PaymentMethod      string
	IntegrationMode    string
}

type RefundSubmitDecision string

const (
	// RefundSubmitAccepted: the provider took the request; the result is still to be confirmed.
	RefundSubmitAccepted RefundSubmitDecision = "accepted"
	// RefundSubmitRejected: the provider definitively refused this attempt. An adapter returns it only for answers
	// its documentation states to be final; any other failure is ErrRefundOutcomeUnknown.
	RefundSubmitRejected RefundSubmitDecision = "rejected"
)

type RefundSubmitResult struct {
	Decision         RefundSubmitDecision
	ProviderRefundID string
	ProviderStatus   string
	FailureCode      string
}

type RefundStateQuery struct {
	ProviderPaymentID string
	IdempotencyKey    string
	AmountCents       int64
}

type RefundStateOutcome string

const (
	// RefundStateNotFound: the provider reports no operation for this key (yet).
	RefundStateNotFound   RefundStateOutcome = "not_found"
	RefundStateInProgress RefundStateOutcome = "in_progress"
	RefundStateSucceeded  RefundStateOutcome = "succeeded"
	RefundStateRejected   RefundStateOutcome = "rejected"
	// RefundStateUnattributed: the provider only reports the original payment's aggregate state, which cannot be
	// tied to one attempt. ReviewRequired tells whether that state shows money returned, so a person must reconcile.
	RefundStateUnattributed RefundStateOutcome = "unattributed"
)

// RefundStateResult is what the provider says about one attempt. The attribution fields (ProviderPaymentID,
// IdempotencyKey, AmountCents) are the provider's own values; the caller compares them with its record and acts
// only on an exact match. An adapter that cannot attribute a result to one attempt must report it as such (empty
// IdempotencyKey), never guess.
type RefundStateResult struct {
	Outcome           RefundStateOutcome
	ProviderPaymentID string
	IdempotencyKey    string
	AmountCents       int64
	ProviderRefundID  string
	ProviderStatus    string
	FailureCode       string
	// ReviewRequired (RefundStateUnattributed only) and ReviewNote: why a person must reconcile, without secrets.
	ReviewRequired bool
	ReviewNote     string
}

type RefundProvider interface {
	// Name is stored in refunds.provider.
	Name() string
	// Preflight checks, before anything is recorded as sent, that the attempt can be sent at all (an
	// ErrRefundNotSendable error). It makes no network call.
	Preflight(in RefundSubmitInput) error
	// SubmitRefund returns ErrRefundOutcomeUnknown (possibly wrapped) when the outcome is not known.
	SubmitRefund(ctx context.Context, in RefundSubmitInput) (RefundSubmitResult, error)
	GetRefundState(ctx context.Context, q RefundStateQuery) (RefundStateResult, error)
}
