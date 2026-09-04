package inventory

import (
	"context"

	"github.com/google/uuid"
)

func (s *Service) StartReconciliationSession(ctx context.Context, sessionID, variantID, startedBy uuid.UUID) error {
	item, err := s.repo.GetAdminInventoryItemRichByVariantID(ctx, variantID)
	if err != nil {
		return err
	}
	if item.AccountingMode == "legacy" {
		return ErrLegacyReconciliationNotAllowed
	}

	return s.repo.StartReconciliationSession(ctx, sessionID, variantID, startedBy)
}

func (s *Service) GetActiveReconciliationSession(ctx context.Context, variantID uuid.UUID) (*ReconciliationSessionDTO, error) {
	return s.repo.GetActiveReconciliationSession(ctx, variantID)
}

func (s *Service) GetReconciliationSessionByID(ctx context.Context, sessionID uuid.UUID) (*ReconciliationSessionDTO, error) {
	return s.repo.GetReconciliationSessionByID(ctx, sessionID)
}

func (s *Service) ProcessReconciliationScan(ctx context.Context, sessionID uuid.UUID, rawCode string, scannedBy uuid.UUID) (*ScanReconciliationResponse, error) {
	return s.repo.ProcessReconciliationScan(ctx, sessionID, rawCode, scannedBy)
}

func (s *Service) MoveReconciliationToReview(ctx context.Context, sessionID, by uuid.UUID) error {
	return s.repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", by)
}

func (s *Service) CancelReconciliationSession(ctx context.Context, sessionID, cancelledBy uuid.UUID) error {
	return s.repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "cancelled", cancelledBy)
}

func (s *Service) CompleteReconciliationSession(ctx context.Context, sessionID, completedBy uuid.UUID) error {
	return s.repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", completedBy)
}

func (s *Service) GetReconciliationReview(ctx context.Context, sessionID uuid.UUID) (*ReconciliationReviewDTO, error) {
	return s.repo.GetReconciliationReview(ctx, sessionID)
}

func (s *Service) ListReconciliationSessionsByVariant(ctx context.Context, variantID uuid.UUID, limit int) ([]ReconciliationSessionDTO, error) {
	return s.repo.ListReconciliationSessionsByVariant(ctx, variantID, limit)
}
