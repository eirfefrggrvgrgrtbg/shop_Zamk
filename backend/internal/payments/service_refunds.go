package payments

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ReserveRefund(ctx context.Context, paymentID uuid.UUID, requestedAmountCents int64, reason string, returnID *uuid.UUID) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		return s.ReserveRefundTx(ctx, tx, paymentID, requestedAmountCents, reason, returnID)
	})
}

func (s *Service) ReserveRefundTx(ctx context.Context, tx pgx.Tx, paymentID uuid.UUID, requestedAmountCents int64, reason string, returnID *uuid.UUID) error {
	if requestedAmountCents <= 0 {
		return ErrInvalidRefundAmount
	}

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
}

func (s *Service) GetSucceededPaymentIDForOrder(ctx context.Context, orderID uuid.UUID) (uuid.UUID, error) {
	rows, err := s.db.Pool.Query(ctx, "SELECT id FROM payments WHERE order_id = $1 AND status = 'succeeded' ORDER BY created_at ASC", orderID)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return uuid.Nil, err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return uuid.Nil, ErrPaymentNotFound
	}
	if len(ids) > 1 {
		return uuid.Nil, ErrAmbiguousFundingPayment
	}
	return ids[0], nil
}
