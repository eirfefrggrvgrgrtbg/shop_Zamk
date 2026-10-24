package returns

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
)

// BANK.REFUND.0: handing a reserved refund to a bank provider and settling it from the provider's answer.
//
//	legacy (rows from before migration 000120): history, never handed to a provider
//	not_sent --SubmitRefund TX1--> sending --provider call--> accepted | unknown | rejected(failed)
//	sending / accepted / unknown --ReconcileRefund--> confirmed(succeeded) | rejected(failed) | unchanged | review
//
// Rules:
//   - The idempotency key, payment and amount are fixed before the provider call (TX1 commits first) and never
//     change (DB trigger refunds_bank_attempt_immutable).
//   - The submit answer is never a confirmation. A transport failure or timeout is 'unknown', never 'failed'.
//   - An attempt is sent at most once by this code: a repeated SubmitRefund returns the existing attempt, and an
//     unknown outcome is resolved only by asking the provider (ReconcileRefund), never by sending again.
//   - Success (status succeeded + the existing ProcessRefundSuccessTx effects: return refunded, seller deduction,
//     return_hold_release) and failure happen only on a provider result attributed to this very attempt (same key,
//     same provider payment id, same amount). Anything else that claims a terminal result is set aside for manual
//     review (review_reason) and changes nothing else.
//   - While an attempt is unresolved the refund keeps status 'pending', so its amount stays reserved and the open
//     return keeps holding the seller earning (PAYOUT.SAFETY.1).

// WithRefundProvider attaches the bank refund provider. The application wires none in BANK.REFUND.0.
func (s *Service) WithRefundProvider(p payments.RefundProvider) *Service {
	s.refundProvider = p
	return s
}

// WithRefundSimulator enables or disables the dev refund simulator (SimulateRefundSuccess / Failure).
func (s *Service) WithRefundSimulator(allowed bool) *Service {
	s.refundSimulatorDisabled = !allowed
	return s
}

type refundAttempt struct {
	ID                uuid.UUID
	ReturnID          *uuid.UUID
	OrderID           uuid.UUID
	Status            string
	AmountCents       int64
	Currency          string
	IdempotencyKey    *string
	SubmitOutcome     string
	ReviewReason      *string
	PaymentStatus     *string
	ProviderPaymentID *string
}

func (a *refundAttempt) key() string {
	if a.IdempotencyKey == nil {
		return ""
	}
	return *a.IdempotencyKey
}

func (a *refundAttempt) providerPaymentID() string {
	if a.ProviderPaymentID == nil {
		return ""
	}
	return *a.ProviderPaymentID
}

func getRefundAttempt(ctx context.Context, q refundQuerier, refundID uuid.UUID, lock bool) (*refundAttempt, error) {
	query := `
		SELECT r.id, r.return_id, r.order_id, r.status, r.amount_cents, r.currency, r.idempotency_key, r.submit_outcome,
		       r.review_reason, p.status, p.provider_payment_id
		FROM refunds r
		LEFT JOIN payments p ON p.id = r.payment_id
		WHERE r.id = $1`
	if lock {
		query += ` FOR UPDATE OF r`
	}
	var a refundAttempt
	err := q.QueryRow(ctx, query, refundID).Scan(&a.ID, &a.ReturnID, &a.OrderID, &a.Status, &a.AmountCents, &a.Currency,
		&a.IdempotencyKey, &a.SubmitOutcome, &a.ReviewReason, &a.PaymentStatus, &a.ProviderPaymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SubmitRefund hands a reserved refund to the bank provider once. Calling it again for the same refund returns the
// existing attempt without a new provider call.
func (s *Service) SubmitRefund(ctx context.Context, refundID uuid.UUID) (*Refund, error) {
	if s.refundProvider == nil {
		return nil, ErrRefundProviderNotConfigured
	}

	var in payments.RefundSubmitInput
	send := false
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		a, err := getRefundAttempt(ctx, tx, refundID, true)
		if err != nil {
			return err
		}
		if a.ReviewReason != nil {
			return ErrRefundUnderReview
		}
		if a.SubmitOutcome == "legacy" {
			// A row from before BANK.REFUND.0 may have been refunded outside ZAMK: it is never sent automatically.
			return ErrRefundLegacyRecord
		}
		if a.Status != "pending" || a.SubmitOutcome != "not_sent" {
			return nil // settled, or already handed to the provider: the existing attempt is the answer
		}
		if a.PaymentStatus == nil || *a.PaymentStatus != "succeeded" || a.providerPaymentID() == "" {
			return ErrRefundPaymentNotLinked
		}
		key := a.key()
		if key == "" {
			key = uuid.New().String() // a reservation made before BANK.REFUND.0 gets its key here, before the call
		}
		if _, err := tx.Exec(ctx, `
			UPDATE refunds
			SET idempotency_key = $2, submit_outcome = 'sending', submit_attempts = submit_attempts + 1,
			    submitted_at = now(), provider = $3, updated_at = now()
			WHERE id = $1
		`, a.ID, key, s.refundProvider.Name()); err != nil {
			return err
		}
		in = payments.RefundSubmitInput{
			ProviderPaymentID: a.providerPaymentID(),
			AmountCents:       a.AmountCents,
			Currency:          a.Currency,
			IdempotencyKey:    key,
		}
		send = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !send {
		return s.repo.GetRefund(ctx, refundID)
	}

	res, submitErr := s.refundProvider.SubmitRefund(ctx, in)

	// The answer is recorded on a fresh context: a cancelled request must not leave a decided answer unrecorded.
	// If this transaction fails the attempt stays 'sending', which reconciliation treats like 'unknown'.
	recordCtx := context.WithoutCancel(ctx)
	var event string
	err = s.db.RunInTx(recordCtx, func(tx pgx.Tx) error {
		a, err := getRefundAttempt(recordCtx, tx, refundID, true)
		if err != nil {
			return err
		}
		if a.SubmitOutcome != "sending" {
			return nil // already resolved by reconciliation
		}
		switch {
		case submitErr == nil && res.Decision == payments.RefundSubmitAccepted:
			event = "refund.bank_submit_accepted"
			_, err = tx.Exec(recordCtx, `
				UPDATE refunds SET submit_outcome = 'accepted', provider_refund_id = NULLIF($2, ''),
				       provider_status = NULLIF($3, ''), updated_at = now()
				WHERE id = $1
			`, a.ID, res.ProviderRefundID, res.ProviderStatus)
			return err
		case submitErr == nil && res.Decision == payments.RefundSubmitRejected:
			event = "refund.bank_rejected"
			if _, err := tx.Exec(recordCtx, `
				UPDATE refunds SET provider_refund_id = NULLIF($2, ''), provider_status = NULLIF($3, '') WHERE id = $1
			`, a.ID, res.ProviderRefundID, res.ProviderStatus); err != nil {
				return err
			}
			return s.processRefundFailureTx(recordCtx, tx, a.ID, true, res.FailureCode)
		default:
			// Timeout, transport error or an answer without a decision: the provider may have executed it.
			event = "refund.bank_submit_unknown"
			_, err = tx.Exec(recordCtx, `UPDATE refunds SET submit_outcome = 'unknown', updated_at = now() WHERE id = $1`, a.ID)
			return err
		}
	})
	if err != nil {
		return nil, err
	}
	if event != "" {
		s.emitRefundBankEvent(recordCtx, event, refundID, slog.LevelInfo)
	}
	return s.repo.GetRefund(recordCtx, refundID)
}

// ReconcileRefund asks the provider about an unresolved attempt (sending / accepted / unknown) and applies an
// attributed terminal result. It never sends a refund. A provider error leaves the attempt unchanged.
func (s *Service) ReconcileRefund(ctx context.Context, refundID uuid.UUID) (*Refund, error) {
	if s.refundProvider == nil {
		return nil, ErrRefundProviderNotConfigured
	}
	a, err := getRefundAttempt(ctx, s.db.Pool, refundID, false)
	if err != nil {
		return nil, err
	}
	if a.ReviewReason != nil {
		return nil, ErrRefundUnderReview
	}
	if a.Status != "pending" || !bankAttemptUnresolved(a.SubmitOutcome) {
		return s.repo.GetRefund(ctx, refundID)
	}

	state, stateErr := s.refundProvider.GetRefundState(ctx, payments.RefundStateQuery{
		ProviderPaymentID: a.providerPaymentID(),
		IdempotencyKey:    a.key(),
		AmountCents:       a.AmountCents,
	})

	var event string
	level := slog.LevelInfo
	err = s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		cur, err := getRefundAttempt(ctx, tx, refundID, true)
		if err != nil {
			return err
		}
		if cur.Status != "pending" || !bankAttemptUnresolved(cur.SubmitOutcome) || cur.ReviewReason != nil || cur.key() != a.key() {
			return nil // settled meanwhile
		}
		if _, err := tx.Exec(ctx, `UPDATE refunds SET last_checked_at = now() WHERE id = $1`, cur.ID); err != nil {
			return err
		}
		if stateErr != nil {
			return nil // nothing learned; the attempt stays as it is
		}
		event, level, err = s.applyRefundStateTx(ctx, tx, cur, state)
		return err
	})
	if err != nil {
		return nil, err
	}
	if event != "" {
		s.emitRefundBankEvent(ctx, event, refundID, level)
	}
	if stateErr != nil {
		return nil, fmt.Errorf("refund state check failed: %w", stateErr)
	}
	return s.repo.GetRefund(ctx, refundID)
}

// applyRefundStateTx applies one provider answer to a locked, unresolved attempt. It returns the business event to
// emit after commit ("" when nothing changed).
func (s *Service) applyRefundStateTx(ctx context.Context, tx pgx.Tx, a *refundAttempt, st payments.RefundStateResult) (string, slog.Level, error) {
	if st.Outcome == payments.RefundStateNotFound {
		return "", slog.LevelInfo, nil
	}
	if reason := attributionMismatch(a, st); reason != "" {
		_, err := tx.Exec(ctx, `
			UPDATE refunds SET review_reason = $2, provider_status = COALESCE(NULLIF($3, ''), provider_status), updated_at = now()
			WHERE id = $1
		`, a.ID, reason, st.ProviderStatus)
		return "refund.bank_review_required", slog.LevelWarn, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refunds SET provider_status = COALESCE(NULLIF($2, ''), provider_status),
		       provider_refund_id = COALESCE(provider_refund_id, NULLIF($3, ''))
		WHERE id = $1
	`, a.ID, st.ProviderStatus, st.ProviderRefundID); err != nil {
		return "", slog.LevelInfo, err
	}

	switch st.Outcome {
	case payments.RefundStateInProgress:
		if a.SubmitOutcome == "accepted" {
			return "", slog.LevelInfo, nil
		}
		// The provider holds this very request: the unknown submit was received.
		_, err := tx.Exec(ctx, `UPDATE refunds SET submit_outcome = 'accepted', updated_at = now() WHERE id = $1`, a.ID)
		return "refund.bank_submit_accepted", slog.LevelInfo, err
	case payments.RefundStateSucceeded:
		processedAt := time.Now().Truncate(time.Microsecond)
		return "refund.bank_confirmed", slog.LevelInfo, s.processRefundSuccessTx(ctx, tx, a.ID, processedAt, true)
	case payments.RefundStateRejected:
		return "refund.bank_rejected", slog.LevelInfo, s.processRefundFailureTx(ctx, tx, a.ID, true, st.FailureCode)
	default:
		return "", slog.LevelInfo, nil // an outcome this code does not know changes nothing
	}
}

// attributionMismatch explains why a provider answer cannot be proven to belong to this attempt ("" when it can).
func attributionMismatch(a *refundAttempt, st payments.RefundStateResult) string {
	switch {
	case st.IdempotencyKey == "" || st.IdempotencyKey != a.key():
		return "provider answer is not attributed to this refund attempt"
	case st.ProviderPaymentID != a.providerPaymentID():
		return "provider answer names another payment"
	case st.AmountCents != a.AmountCents:
		return fmt.Sprintf("provider amount %d differs from refund amount %d", st.AmountCents, a.AmountCents)
	}
	return ""
}

func (s *Service) emitRefundBankEvent(ctx context.Context, event string, refundID uuid.UUID, level slog.Level) {
	result := "success"
	if level >= slog.LevelWarn {
		result = "review_required"
	}
	observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
		EventName: event,
		Domain:    "refund",
		Action:    "bank_refund",
		Result:    result,
		Level:     level,
		ActorRole: "system",
		Attributes: []slog.Attr{
			slog.String("refund_id", refundID.String()),
		},
	})
}
