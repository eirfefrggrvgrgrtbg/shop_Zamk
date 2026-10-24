package payments

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RefundSummary struct {
	SucceededAmountCents int64
	PendingAmountCents   int64
}

func (r *Repository) GetRefundSummaryTx(ctx context.Context, tx pgx.Tx, paymentID uuid.UUID) (RefundSummary, error) {
	query := `
		SELECT 
			COALESCE(SUM(amount_cents) FILTER (WHERE status IN ('succeeded', 'completed')), 0) as succeeded_amount,
			-- pending covers every unresolved bank attempt (sending / accepted / unknown keep status pending)
			COALESCE(SUM(amount_cents) FILTER (WHERE status IN ('pending', 'processing')), 0) as pending_amount
		FROM refunds
		WHERE payment_id = $1
	`
	var summary RefundSummary
	err := tx.QueryRow(ctx, query, paymentID).Scan(&summary.SucceededAmountCents, &summary.PendingAmountCents)
	return summary, err
}

func (r *Repository) CreateRefundTx(ctx context.Context, tx pgx.Tx, paymentID, orderID uuid.UUID, amountCents int64, reason string, returnID *uuid.UUID) error {
	query := `
		INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at, return_id, idempotency_key)
		VALUES ($1, $2, $3, $4, 'pending', $5, 'RUB', now(), now(), $6, $7)
	`
	// BANK.REFUND.0: the idempotency key of the bank attempt is fixed with the reservation, before any provider call.
	_, err := tx.Exec(ctx, query, uuid.New(), paymentID, orderID, amountCents, reason, returnID, uuid.New().String())
	return err
}
