package payments

import (
	"context"
	"github.com/google/uuid"
)

func (s *Service) ListAdminPayments(ctx context.Context, q, status, provider, method, mode, refundState, probCode, dateFrom, dateTo string, amountFrom, amountTo int64, hasProblem bool, sort, direction string, limit, offset int) ([]AdminPaymentDTO, int, error) {
	stuckMins := s.cfg.App.PaymentStuckPendingMinutes
	if stuckMins == 0 {
		stuckMins = 30
	}
	return s.repo.ListAdminPayments(ctx, q, status, provider, method, mode, refundState, probCode, dateFrom, dateTo, amountFrom, amountTo, hasProblem, sort, direction, limit, offset, stuckMins)
}

func (s *Service) GetAdminPaymentDetail(ctx context.Context, id uuid.UUID) (*AdminPaymentDetailDTO, error) {
	stuckMins := s.cfg.App.PaymentStuckPendingMinutes
	if stuckMins == 0 {
		stuckMins = 30
	}
	return s.repo.GetAdminPaymentDetail(ctx, id, stuckMins)
}
