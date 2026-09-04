package inventory

import (
	"context"
	"log/slog"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) StartReconciliationSession(ctx context.Context, sessionID, variantID, startedBy uuid.UUID) error {
	item, err := s.repo.GetAdminInventoryItemRichByVariantID(ctx, variantID)
	if err != nil {
		return err
	}
	if item.AccountingMode == "legacy" {
		return ErrLegacyReconciliationNotAllowed
	}

	if err := s.repo.StartReconciliationSession(ctx, sessionID, variantID, startedBy); err != nil {
		return err
	}

	observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
		EventName: "warehouse.reconciliation_started",
		Domain:    "warehouse",
		Action:    "start_reconciliation",
		Result:    "success",
		ActorID:   startedBy.String(),
		ActorRole: "admin",
		Level:     slog.LevelInfo,
		Attributes: []slog.Attr{
			slog.String("reconciliation_session_id", sessionID.String()),
			slog.String("warehouse_id", "main"),
			slog.String("product_variant_id", variantID.String()),
			slog.String("actor_id", startedBy.String()),
		},
	})

	return nil
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
	if err := s.repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", completedBy); err != nil {
		return err
	}

	foundCount := 0
	resolvedCount := 0
	if plan, errPlan := s.repo.GetReconciliationResolutionPlan(ctx, sessionID); errPlan == nil && plan != nil {
		foundCount = len(plan.Cases)
		resolvedCount = plan.ResolvedCasesCount
	}

	observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
		EventName: "warehouse.reconciliation_completed",
		Domain:    "warehouse",
		Action:    "complete_reconciliation",
		Result:    "success",
		ActorID:   completedBy.String(),
		ActorRole: "admin",
		Level:     slog.LevelInfo,
		Attributes: []slog.Attr{
			slog.String("reconciliation_session_id", sessionID.String()),
			slog.Int("discrepancies_found_count", foundCount),
			slog.Int("discrepancies_resolved_count", resolvedCount),
			slog.String("actor_id", completedBy.String()),
		},
	})

	return nil
}

func (s *Service) GetReconciliationReview(ctx context.Context, sessionID uuid.UUID) (*ReconciliationReviewDTO, error) {
	return s.repo.GetReconciliationReview(ctx, sessionID)
}

func (s *Service) ListReconciliationSessionsByVariant(ctx context.Context, variantID uuid.UUID, limit int) ([]ReconciliationSessionDTO, error) {
	return s.repo.ListReconciliationSessionsByVariant(ctx, variantID, limit)
}

func (s *Service) GetReconciliationResolutionPlan(ctx context.Context, sessionID uuid.UUID) (*ReconciliationResolutionPlanDTO, error) {
	return s.repo.GetReconciliationResolutionPlan(ctx, sessionID)
}

func (s *Service) ResolveReconciliationCase(ctx context.Context, sessionID uuid.UUID, adminID uuid.UUID, req ResolveReconciliationCaseRequest) (*ReconciliationResolutionPlanDTO, error) {
	var mutRec *ReconciliationMutationRecord
	err := s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		var errTx error
		mutRec, errTx = txRepo.resolveReconciliationCaseTx(ctx, tx, sessionID, adminID, req)
		return errTx
	})
	if err != nil {
		observability.RecordReconciliationResolution(ctx, req.ActionID, "error")
		return nil, err
	}

	// Durable audit consistency: emit business events and metrics ONLY AFTER successful commit
	if mutRec != nil && mutRec.Mutated {
		// 1. Reconciliation discrepancy resolved
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName: "warehouse.reconciliation_discrepancy_resolved",
			Domain:    "warehouse",
			Action:    "reconciliation_discrepancy_resolved",
			Result:    "success",
			ActorID:   adminID.String(),
			ActorRole: "admin",
			Level:     slog.LevelInfo,
			Attributes: []slog.Attr{
				slog.String("reconciliation_session_id", sessionID.String()),
				slog.String("discrepancy_id", mutRec.InventoryUnitID.String()),
				slog.String("resolution_action", mutRec.ActionID),
				slog.String("result", "success"),
				slog.String("actor_id", adminID.String()),
			},
		})
		actionMetric := mutRec.ActionID
		if mutRec.IsLiveReplacement {
			actionMetric = "replace_live_allocation"
		}
		observability.RecordReconciliationResolution(ctx, actionMetric, "success")

		// 2. Specific mutation events
		switch mutRec.ActionID {
		case ActionIDCloseStaleAllocation:
			allocIDStr := ""
			if mutRec.AllocationID != nil {
				allocIDStr = mutRec.AllocationID.String()
			}
			observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
				EventName: "inventory.stale_allocation_released",
				Domain:    "inventory",
				Action:    "release_stale_allocation",
				Result:    "success",
				ActorID:   adminID.String(),
				ActorRole: "admin",
				Level:     slog.LevelInfo,
				Attributes: []slog.Attr{
					slog.String("allocation_id", allocIDStr),
					slog.String("zmu", mutRec.UnitCode),
					slog.String("order_number", mutRec.OrderNumber),
				},
			})

		case ActionIDConfirmMissing:
			// Missing unit written off
			observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
				EventName: "inventory.unit_written_off",
				Domain:    "inventory",
				Action:    "write_off_unit",
				Result:    "success",
				ActorID:   adminID.String(),
				ActorRole: "admin",
				Level:     slog.LevelInfo,
				Attributes: []slog.Attr{
					slog.String("inventory_unit_id", mutRec.InventoryUnitID.String()),
					slog.String("zmu", mutRec.UnitCode),
					slog.String("reason", "reconciliation_missing"),
				},
			})
			observability.RecordInventoryWriteoff(ctx, "reconciliation_missing")

			// If live allocation was replaced
			if mutRec.IsLiveReplacement {
				observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
					EventName: "inventory.allocation_replaced",
					Domain:    "inventory",
					Action:    "replace_allocation",
					Result:    "success",
					ActorID:   adminID.String(),
					ActorRole: "admin",
					Level:     slog.LevelInfo,
					Attributes: []slog.Attr{
						slog.String("missing_zmu", mutRec.UnitCode),
						slog.String("replacement_zmu", mutRec.ReplacementCode),
						slog.String("order_number", mutRec.OrderNumber),
						slog.String("reconciliation_session_id", sessionID.String()),
					},
				})
			}
		}
	}

	return s.repo.GetReconciliationResolutionPlan(ctx, sessionID)
}
