package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PAY.RECON.1: a payment that is already failed/cancelled locally but that the bank reports as charged.
//
// Detection only. The conflict is stored as a payment_events row (event_type CONFIRMED, the same row a late webhook
// leaves) and is shown to the admin as BANK_CONFIRMED_PAYMENT_NOT_SUCCEEDED, computed from that row on every read,
// so it survives a restart and a replay cannot duplicate it. Nothing else changes: the payment stays failed, the
// order is not paid, no refund, ledger entry, payout, reservation or stock is touched. The resolution (refund or
// manual settlement) is a Product Owner decision.

const lateConfirmationRecheckAfter = 24 * time.Hour
const lateConfirmationWindow = 30 * 24 * time.Hour

type lateConfirmationCandidate struct {
	ID                uuid.UUID
	OrderID           uuid.UUID
	ProviderPaymentID string
	AmountCents       int64
}

// claimLateConfirmationCandidates returns real (non-mock) payments that ended failed/cancelled, have a provider id and no
// bank confirmation on record yet, and marks them as checked so each is asked at most once per recheck period.
func (r *Repository) claimLateConfirmationCandidates(ctx context.Context, limit int) ([]lateConfirmationCandidate, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE payments p SET reconciliation_attempted_at = now()
		WHERE p.id IN (
			SELECT id FROM payments
			WHERE status IN ('failed', 'cancelled')
			  AND integration_mode <> 'mock'
			  AND provider_payment_id IS NOT NULL AND provider_payment_id <> ''
			  AND created_at > now() - make_interval(secs => $2)
			  AND (reconciliation_attempted_at IS NULL OR reconciliation_attempted_at < now() - make_interval(secs => $3))
			  AND NOT EXISTS (
			      SELECT 1 FROM payment_events e
			      WHERE e.payment_id = payments.id AND e.event_type = 'CONFIRMED' AND e.signature_valid = true)
			ORDER BY reconciliation_attempted_at NULLS FIRST, created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING p.id, p.order_id, p.provider_payment_id, p.amount_cents
	`, limit, lateConfirmationWindow.Seconds(), lateConfirmationRecheckAfter.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lateConfirmationCandidate
	for rows.Next() {
		var c lateConfirmationCandidate
		if err := rows.Scan(&c.ID, &c.OrderID, &c.ProviderPaymentID, &c.AmountCents); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DetectLateConfirmations asks the provider about failed/cancelled payments and records a bank-confirmed conflict.
// It never changes the payment, the order, stock or any financial record. Returns the number of conflicts recorded.
func (s *Service) DetectLateConfirmations(ctx context.Context, batchLimit int) (int, error) {
	candidates, err := s.repo.claimLateConfirmationCandidates(ctx, batchLimit)
	if err != nil {
		return 0, err
	}
	found := 0
	for _, c := range candidates {
		ok, err := s.detectLateConfirmation(ctx, c)
		if err != nil {
			s.logger.WarnContext(ctx, "late confirmation check failed", "payment_id", c.ID.String(), "order_id", c.OrderID.String(), "error", err)
			continue
		}
		if ok {
			found++
		}
	}
	return found, nil
}

func (s *Service) detectLateConfirmation(ctx context.Context, c lateConfirmationCandidate) (bool, error) {
	res, err := s.provider.CheckOrder(ctx, c.OrderID.String())
	if err != nil {
		return false, err
	}
	var item *ProviderPaymentItem
	for i := range res.Payments {
		if res.Payments[i].ProviderPaymentID == c.ProviderPaymentID {
			item = &res.Payments[i]
			break
		}
	}
	// Only the provider's own record of this very payment counts: confirmed, successful and for the amount we asked.
	if item == nil || !item.Success || ClassifyProviderStatus(item.ProviderStatus) != ProviderStatusCategorySuccess || item.AmountCents != c.AmountCents {
		return false, nil
	}

	raw, _ := json.Marshal(item)
	recorded := false
	err = s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		now := time.Now()
		pid := c.ProviderPaymentID
		ev := PaymentEvent{
			ID:                uuid.New(),
			PaymentID:         &c.ID,
			Provider:          "tbank",
			ProviderPaymentID: &pid,
			EventType:         "CONFIRMED",
			EventKey:          fmt.Sprintf("reconcile:%s:CONFIRMED", c.ProviderPaymentID),
			RawPayload:        raw,
			SignatureValid:    true, // authenticated by the terminal credentials of the provider API call
			ProcessedAt:       &now,
		}
		if err := s.repo.CreatePaymentEventTx(ctx, tx, &ev); err != nil {
			if err == ErrPaymentAlreadyProcessed {
				return nil
			}
			return err
		}
		recorded = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if recorded {
		s.emitBankConfirmedConflict(ctx, c.ID, c.OrderID, "reconciliation")
	}
	return recorded, nil
}

func (s *Service) emitBankConfirmedConflict(ctx context.Context, paymentID, orderID uuid.UUID, source string) {
	observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
		EventName: "payment.bank_confirmed_conflict_detected",
		Domain:    "payment",
		Action:    "detect_late_confirmation",
		Result:    "success",
		ActorRole: "system",
		Attributes: []slog.Attr{
			slog.String("payment_id", paymentID.String()),
			slog.String("order_id", orderID.String()),
			slog.String("source", source),
		},
	})
}
