package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ReserveRefund(ctx context.Context, paymentID uuid.UUID, requestedAmountCents int64, reason string, returnID *uuid.UUID) error {
	if requestedAmountCents <= 0 {
		return ErrInvalidRefundAmount
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Lock payment for update
		p, err := s.repo.GetPaymentByIDForUpdateTx(ctx, tx, paymentID)
		if err != nil {
			return err
		}

		if p.Status != "succeeded" {
			return ErrInvalidPaymentStatus
		}

		if returnID != nil {
			var returnOrderID uuid.UUID
			err := tx.QueryRow(ctx, "SELECT order_id FROM returns WHERE id = $1", *returnID).Scan(&returnOrderID)
			if err != nil {
				return err
			}
			if returnOrderID != p.OrderID {
				return ErrMismatchedOrderAndPayment
			}
		}

		// 2. Get current refunds
		summary, err := s.repo.GetRefundSummaryTx(ctx, tx, paymentID)
		if err != nil {
			return err
		}

		reserved := summary.SucceededAmountCents + summary.PendingAmountCents
		available := p.AmountCents - reserved

		// 3. Check over-refund
		if requestedAmountCents > available {
			return ErrRefundExceedsPaid
		}

		// 4. Create pending refund
		return s.repo.CreateRefundTx(ctx, tx, p.ID, p.OrderID, requestedAmountCents, reason, returnID)
	})
}

func (s *Service) GetSucceededPaymentIDForOrder(ctx context.Context, orderID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1 AND status = 'succeeded' ORDER BY created_at DESC LIMIT 1", orderID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrPaymentNotFound
		}
		return uuid.Nil, err
	}
	return id, nil
}
