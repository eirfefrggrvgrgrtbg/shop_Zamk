package supplies

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (s *Service) StartReceivingSession(ctx context.Context, staffID uuid.UUID, qrToken string) (*ReceivingSession, error) {
	supply, err := s.repo.GetSupplyByQRToken(ctx, qrToken)
	if err != nil {
		return nil, err
	}

	if supply.Status != "shipped_by_seller" && supply.Status != "arrived_at_zamk" && supply.Status != "receiving" {
		return nil, ErrInvalidStatus
	}

	// Check for active session
	active, err := s.repo.GetActiveSession(ctx, supply.ID)
	if err == nil {
		return active, nil // Return existing
	}
	if err != ErrSessionNotFound {
		return nil, err
	}

	// Start new session
	now := time.Now().UTC()
	session := &ReceivingSession{
		ID:               uuid.New(),
		SupplyID:         supply.ID,
		Status:           "active",
		Version:          1,
		StartedAt:        now,
		StartedByStaffID: &staffID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// We need to fetch items to populate the session items
	fullSupply, err := s.repo.GetSupplyByID(ctx, supply.ID)
	if err != nil {
		return nil, err
	}

	for _, item := range fullSupply.Items {
		session.Items = append(session.Items, ReceivingItem{
			ID:               uuid.New(),
			SessionID:        session.ID,
			SupplyItemID:     &item.ID,
			VariantID:        &item.VariantID,
			SKU:              item.SKU,
			Barcode:          item.Barcode,
			ProductTitle:     item.ProductTitle,
			ExpectedQuantity: item.ExpectedQuantity,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}

	err = s.repo.StartReceivingSession(ctx, session, fullSupply.Items)
	if err != nil {
		return nil, err
	}

	// Update supply status
	err = s.repo.UpdateSupplyStatus(ctx, supply.ID, "receiving")
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Service) RecordScan(ctx context.Context, staffID uuid.UUID, sessionID uuid.UUID, req RecordReceivingScanRequest) error {
	session, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.Status != "active" {
		return errors.New("session is not active")
	}

	var matchedItem *ReceivingItem
	for i := range session.Items {
		if session.Items[i].VariantID != nil && *session.Items[i].VariantID == req.VariantID {
			matchedItem = &session.Items[i]
			break
		}
	}

	if matchedItem == nil {
		return errors.New("variant not found in this supply")
	}

	err = s.repo.AddReceivingScan(ctx, sessionID, matchedItem.ID, &staffID, req.Quantity, req.IsDamage)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) FinalizeReceiving(ctx context.Context, staffID uuid.UUID, sessionID uuid.UUID, req FinalizeReceivingRequest) error {
	pool, ok := s.db.(*pgxpool.Pool)
	if !ok {
		return errors.New("expected *pgxpool.Pool")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repoTx := s.repo.WithTx(tx)

	// Lock the session to prevent concurrent finalizations
	err = repoTx.LockSessionForUpdate(ctx, sessionID)
	if err != nil {
		return err
	}

	session, err := repoTx.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.Status != "active" {
		return errors.New("session is not active")
	}

	// Lock the supply to prevent concurrent updates to supply
	err = repoTx.LockSupplyForUpdate(ctx, session.SupplyID)
	if err != nil {
		return err
	}

	supply, err := repoTx.GetSupplyByID(ctx, session.SupplyID)
	if err != nil {
		return err
	}

	hasDiscrepancies := false

	// Update items and inventory
	for _, item := range session.Items {
		if item.SupplyItemID == nil {
			continue // unexpected item not supported fully yet
		}

		accepted := item.ScannedQuantity
		damaged := item.DamagedQuantity
		missing := item.ExpectedQuantity - accepted - damaged
		extra := 0
		if missing < 0 {
			extra = -missing
			missing = 0
		}

		if missing > 0 || damaged > 0 || extra > 0 {
			hasDiscrepancies = true
		}

		// Finalize item in supply
		err = repoTx.FinalizeSupplyItem(ctx, *item.SupplyItemID, accepted, damaged, missing, extra)
		if err != nil {
			return err
		}

		// Update inventory (will insert stock movement internally)
		if accepted > 0 && item.VariantID != nil {
			err = repoTx.UpdateInventoryStock(ctx, *item.VariantID, accepted, supply.ID)
			if err != nil {
				return fmt.Errorf("failed to update inventory: %w", err)
			}
		}
	}

	err = repoTx.CompleteReceivingSession(ctx, sessionID)
	if err != nil {
		return err
	}

	newStatus := "completed"
	if hasDiscrepancies {
		newStatus = "completed_with_discrepancies"
	}

	err = repoTx.UpdateSupplyStatus(ctx, supply.ID, newStatus)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}
