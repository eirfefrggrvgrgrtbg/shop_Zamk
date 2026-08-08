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
			COALESCE(SUM(amount_cents) FILTER (WHERE status = 'succeeded'), 0) as succeeded_amount,
			COALESCE(SUM(amount_cents) FILTER (WHERE status = 'pending'), 0) as pending_amount
		FROM refunds
		WHERE payment_id = $1
	`
	var summary RefundSummary
	err := tx.QueryRow(ctx, query, paymentID).Scan(&summary.SucceededAmountCents, &summary.PendingAmountCents)
	return summary, err
}

func (r *Repository) CreateRefundTx(ctx context.Context, tx pgx.Tx, paymentID, orderID uuid.UUID, amountCents int64, reason string, returnID *uuid.UUID) error {
	query := `
		INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at, return_id)
		VALUES ($1, $2, $3, $4, 'pending', $5, 'RUB', now(), now(), $6)
	`
	_, err := tx.Exec(ctx, query, uuid.New(), paymentID, orderID, amountCents, reason, returnID)
	return err
}
