package payments

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// FakeRefundProvider is an in-memory RefundProvider for tests and local development. It never opens a network
// connection and is not wired into the application.
//
// Each operation is keyed by its idempotency key, like a provider with request-level duplicate protection: a
// repeated submit with the same key, payment and amount returns the existing operation instead of a new one.
type FakeRefundProvider struct {
	mu          sync.Mutex
	ops         map[string]*fakeRefundOp
	order       []string
	next        []FakeSubmitBehavior
	submitCalls map[string]int
	overrides   map[string]RefundStateResult
	stateErr    error
	seq         int
}

// FakeSubmitBehavior scripts the answer of the next SubmitRefund call that creates an operation.
type FakeSubmitBehavior int

const (
	// FakeSubmitAccept: the operation is recorded as in progress and the call answers accepted (the default).
	FakeSubmitAccept FakeSubmitBehavior = iota
	// FakeSubmitReject: the operation is recorded as rejected and the call answers rejected.
	FakeSubmitReject
	// FakeSubmitTimeout: the request never reaches the provider; nothing is recorded and the outcome is unknown.
	FakeSubmitTimeout
	// FakeSubmitLostResponse: the provider records the operation in progress, but the answer is lost.
	FakeSubmitLostResponse
)

type fakeRefundOp struct {
	key         string
	paymentID   string
	amountCents int64
	refundID    string
	outcome     RefundStateOutcome
	failureCode string
}

func NewFakeRefundProvider() *FakeRefundProvider {
	return &FakeRefundProvider{
		ops:         map[string]*fakeRefundOp{},
		submitCalls: map[string]int{},
		overrides:   map[string]RefundStateResult{},
	}
}

func (f *FakeRefundProvider) Name() string { return "fake" }

// QueueSubmit sets the behaviour of the next submits that create an operation, in order.
func (f *FakeRefundProvider) QueueSubmit(b ...FakeSubmitBehavior) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next = append(f.next, b...)
}

func (f *FakeRefundProvider) SubmitRefund(ctx context.Context, in RefundSubmitInput) (RefundSubmitResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if in.IdempotencyKey == "" {
		return RefundSubmitResult{}, errors.New("fake refund provider: empty idempotency key")
	}
	f.submitCalls[in.IdempotencyKey]++

	if op, ok := f.ops[in.IdempotencyKey]; ok {
		if op.paymentID != in.ProviderPaymentID || op.amountCents != in.AmountCents {
			return RefundSubmitResult{}, fmt.Errorf("%w: idempotency key reused with different parameters", ErrRefundOutcomeUnknown)
		}
		if op.outcome == RefundStateRejected {
			return RefundSubmitResult{Decision: RefundSubmitRejected, ProviderRefundID: op.refundID, ProviderStatus: "FAKE_REJECTED", FailureCode: op.failureCode}, nil
		}
		return RefundSubmitResult{Decision: RefundSubmitAccepted, ProviderRefundID: op.refundID, ProviderStatus: fakeStatus(op.outcome)}, nil
	}

	behavior := FakeSubmitAccept
	if len(f.next) > 0 {
		behavior = f.next[0]
		f.next = f.next[1:]
	}
	if behavior == FakeSubmitTimeout {
		return RefundSubmitResult{}, fmt.Errorf("%w: %w", ErrRefundOutcomeUnknown, context.DeadlineExceeded)
	}

	f.seq++
	op := &fakeRefundOp{
		key:         in.IdempotencyKey,
		paymentID:   in.ProviderPaymentID,
		amountCents: in.AmountCents,
		refundID:    fmt.Sprintf("fake-refund-%d", f.seq),
		outcome:     RefundStateInProgress,
	}
	if behavior == FakeSubmitReject {
		op.outcome = RefundStateRejected
		op.failureCode = "FAKE_DECLINED"
	}
	f.ops[op.key] = op
	f.order = append(f.order, op.key)

	switch behavior {
	case FakeSubmitReject:
		return RefundSubmitResult{Decision: RefundSubmitRejected, ProviderRefundID: op.refundID, ProviderStatus: "FAKE_REJECTED", FailureCode: op.failureCode}, nil
	case FakeSubmitLostResponse:
		return RefundSubmitResult{}, fmt.Errorf("%w: response lost", ErrRefundOutcomeUnknown)
	default:
		return RefundSubmitResult{Decision: RefundSubmitAccepted, ProviderRefundID: op.refundID, ProviderStatus: fakeStatus(op.outcome)}, nil
	}
}

func (f *FakeRefundProvider) GetRefundState(ctx context.Context, q RefundStateQuery) (RefundStateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stateErr != nil {
		return RefundStateResult{}, f.stateErr
	}
	if r, ok := f.overrides[q.IdempotencyKey]; ok {
		return r, nil
	}
	op, ok := f.ops[q.IdempotencyKey]
	if !ok {
		return RefundStateResult{Outcome: RefundStateNotFound}, nil
	}
	return RefundStateResult{
		Outcome:           op.outcome,
		ProviderPaymentID: op.paymentID,
		IdempotencyKey:    op.key,
		AmountCents:       op.amountCents,
		ProviderRefundID:  op.refundID,
		ProviderStatus:    fakeStatus(op.outcome),
		FailureCode:       op.failureCode,
	}, nil
}

// Confirm makes the provider report the operation of this key as refunded.
func (f *FakeRefundProvider) Confirm(key string) error {
	return f.settle(key, RefundStateSucceeded, "")
}

// Fail makes the provider report the operation of this key as definitively rejected.
func (f *FakeRefundProvider) Fail(key, failureCode string) error {
	return f.settle(key, RefundStateRejected, failureCode)
}

func (f *FakeRefundProvider) settle(key string, outcome RefundStateOutcome, failureCode string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	op, ok := f.ops[key]
	if !ok {
		return fmt.Errorf("fake refund provider: no operation for key %q", key)
	}
	op.outcome = outcome
	op.failureCode = failureCode
	return nil
}

// OverrideState makes GetRefundState answer r for this key, e.g. a contradictory confirmation.
func (f *FakeRefundProvider) OverrideState(key string, r RefundStateResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.overrides[key] = r
}

// FailStateQueries makes every GetRefundState fail with err (nil restores normal answers).
func (f *FakeRefundProvider) FailStateQueries(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stateErr = err
}

// SubmitCalls counts the submits received for a key, including repeats.
func (f *FakeRefundProvider) SubmitCalls(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.submitCalls[key]
}

// Operations counts the distinct operations the provider recorded for a payment.
func (f *FakeRefundProvider) Operations(providerPaymentID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, op := range f.ops {
		if op.paymentID == providerPaymentID {
			n++
		}
	}
	return n
}

// RefundedCents sums the operations of a payment the provider reports as refunded.
func (f *FakeRefundProvider) RefundedCents(providerPaymentID string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var total int64
	for _, op := range f.ops {
		if op.paymentID == providerPaymentID && op.outcome == RefundStateSucceeded {
			total += op.amountCents
		}
	}
	return total
}

func fakeStatus(o RefundStateOutcome) string {
	switch o {
	case RefundStateSucceeded:
		return "FAKE_REFUNDED"
	case RefundStateRejected:
		return "FAKE_REJECTED"
	default:
		return "FAKE_IN_PROGRESS"
	}
}
