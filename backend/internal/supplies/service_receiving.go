package supplies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (s *Service) MarkSupplyArrived(ctx context.Context, adminID uuid.UUID, supplyID uuid.UUID) error {
	supply, err := s.repo.GetSupplyByID(ctx, supplyID)
	if err != nil {
		return err
	}
	if supply.Status != "shipped_by_seller" {
		return ErrInvalidStatus
	}
	return s.repo.UpdateSupplyStatus(ctx, supplyID, "arrived_at_zamk")
}

func (s *Service) StartReceivingSession(ctx context.Context, staffID uuid.UUID, qrToken string) (*ReceivingSession, error) {
	qrToken = strings.TrimSpace(qrToken)
	if qrToken == "" {
		return nil, ErrInvalidReceivingCode
	}

	supply, err := s.repo.GetSupplyByQRToken(ctx, qrToken)
	if err != nil {
		return nil, err
	}

	mode, err := s.repo.GetSupplyReceivingMode(ctx, supply.ID)
	if err != nil {
		return nil, err
	}

	isAdditional := false
	if supply.Status == "completed_with_discrepancies" {
		if mode != "serialized" {
			return nil, ErrInvalidStatus
		}
		isAdditional = true
	} else {
		if supply.Status == "draft" || supply.Status == "ready_to_ship" {
			return nil, ErrSupplyNotReadyForReceiving
		}
		if supply.Status == "shipped_by_seller" {
			return nil, ErrSupplyNotArrived
		}
		if supply.Status == "completed" {
			return nil, ErrSupplyAlreadyCompleted
		}
		if supply.Status == "cancelled" {
			return nil, ErrSupplyCancelled
		}
		if supply.Status != "arrived_at_zamk" && supply.Status != "receiving" {
			return nil, ErrInvalidStatus
		}
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
		ReceivingMode:    mode,
	}

	// We need to fetch items to populate the session items
	fullSupply, err := s.repo.GetSupplyByID(ctx, supply.ID)
	if err != nil {
		return nil, err
	}

	for _, item := range fullSupply.Items {
		expectedQty := item.ExpectedQuantity
		if isAdditional {
			remaining, err := s.repo.CountRemainingExpectedUnitsForItem(ctx, item.ID)
			if err != nil {
				return nil, err
			}
			if remaining == 0 {
				continue
			}
			expectedQty = remaining
		}

		session.Items = append(session.Items, ReceivingItem{
			ID:               uuid.New(),
			SessionID:        session.ID,
			SupplyItemID:     &item.ID,
			VariantID:        &item.VariantID,
			SKU:              item.SKU,
			Barcode:          item.Barcode,
			ProductTitle:     item.ProductTitle,
			ExpectedQuantity: expectedQty,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}

	if isAdditional && len(session.Items) == 0 {
		return nil, ErrNoExpectedUnitsRemain
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

	mode, err := repoTx.GetSupplyReceivingMode(ctx, session.SupplyID)
	if err != nil {
		return err
	}

	supply, err := repoTx.GetSupplyByID(ctx, session.SupplyID)
	if err != nil {
		return err
	}

	if mode == "serialized" {
		// Lock all physical units of this supply
		if err := repoTx.LockUnitsForSupply(ctx, session.SupplyID); err != nil {
			return err
		}

		// Finalize unit states: OK -> warehouse, Damaged -> damaged, set receiving_session_id
		if err := repoTx.FinalizeSerializedUnits(ctx, sessionID); err != nil {
			return err
		}
	}

	// Update items and inventory
	for _, item := range session.Items {
		if item.SupplyItemID == nil {
			continue // unexpected item not supported fully yet
		}

		accepted := item.ScannedQuantity
		damaged := item.DamagedQuantity

		// Finalize item in supply (accumulates quantities)
		err = repoTx.FinalizeSupplyItem(ctx, *item.SupplyItemID, accepted, damaged)
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

	hasDiscrepancies, err := repoTx.CheckSupplyDiscrepancies(ctx, supply.ID)
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

func (s *Service) ListRecentSerializedScans(ctx context.Context, staffID uuid.UUID, sessionID uuid.UUID, limit int) ([]SerializedRecentScanDTO, error) {
	_, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRecentSerializedScans(ctx, sessionID, limit)
}

func (s *Service) UndoSerializedScan(ctx context.Context, staffID uuid.UUID, sessionID uuid.UUID, scanID uuid.UUID) (*UndoSerializedScanResponse, error) {
	pool, ok := s.db.(*pgxpool.Pool)
	if !ok {
		return nil, errors.New("expected *pgxpool.Pool")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	repoTx := s.repo.WithTx(tx)

	err = repoTx.LockSessionForUpdate(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	session, err := repoTx.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != "active" {
		return nil, ErrReceivingSessionFinalized
	}

	scan, err := repoTx.LockScanForUpdate(ctx, scanID)
	if err != nil {
		return nil, err
	}

	if scan.SessionID != sessionID {
		return nil, ErrScanNotInSession
	}

	if scan.InventoryUnitID == nil {
		return nil, ErrScanNotFound
	}

	if scan.VoidedAt != nil {
		return nil, ErrScanAlreadyVoided
	}

	err = repoTx.VoidSerializedScan(ctx, scanID, staffID, scan.SupplyReceivingItemID, scan.IsDamage)
	if err != nil {
		return nil, err
	}

	exp, scn, okCount, dmgCount, err := repoTx.GetReceivingSessionTotals(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return &UndoSerializedScanResponse{
		ScanID:           scanID,
		VoidedAt:         time.Now().UTC(),
		SessionExpected:  exp,
		SessionScanned:   scn,
		SessionOk:        okCount,
		SessionDamaged:   dmgCount,
		SessionRemaining: exp - scn,
	}, nil
}

func (s *Service) RecordSerializedScan(ctx context.Context, staffID uuid.UUID, sessionID uuid.UUID, req RecordSerializedScanRequest) (*SerializedScanResponse, error) {
	req.UnitCode = strings.TrimSpace(req.UnitCode)
	if req.UnitCode == "" || strings.HasPrefix(req.UnitCode, "ZMK-") || strings.HasPrefix(req.UnitCode, "SKU-") || strings.HasPrefix(req.UnitCode, "ZMK") {
		return nil, ErrSerializedUnitCodeRequired
	}

	if req.Condition != "ok" && req.Condition != "damaged" {
		return nil, ErrInvalidReceivingCondition
	}

	// We lock session using a tx
	pool, ok := s.db.(*pgxpool.Pool)
	if !ok {
		return nil, errors.New("expected *pgxpool.Pool")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	repoTx := s.repo.WithTx(tx)

	err = repoTx.LockSessionForUpdate(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	session, err := repoTx.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != "active" {
		return nil, ErrReceivingSessionFinalized
	}

	// Verify Serialization Health
	supply, err := repoTx.GetSupplyByID(ctx, session.SupplyID)
	if err != nil {
		return nil, err
	}

	var expected int
	for _, item := range supply.Items {
		expected += item.ExpectedQuantity
	}

	units, err := repoTx.ListUnitsBySupplyID(ctx, session.SupplyID)
	if err != nil {
		return nil, err
	}
	actual := len(units)

	if actual == 0 {
		return nil, ErrSupplyNotSerialized
	}
	if actual != expected {
		return nil, ErrSupplyUnitIdentityMismatch
	}

	// Find the enriched unit
	unit, err := repoTx.GetEnrichedInventoryUnitByCode(ctx, req.UnitCode)
	if err != nil {
		return nil, err
	}

	if unit.OriginSupplyID != session.SupplyID {
		return nil, ErrUnitNotInSupply
	}
	if unit.Status != "expected" {
		return nil, ErrUnitAlreadyReceived
	}

	// Find session item
	var matchedItem *ReceivingItem
	for i := range session.Items {
		if session.Items[i].SupplyItemID != nil && *session.Items[i].SupplyItemID == unit.OriginSupplyItemID {
			matchedItem = &session.Items[i]
			break
		}
	}
	if matchedItem == nil {
		return nil, ErrItemNotFound
	}

	isDamage := req.Condition == "damaged"
	scanID, err := repoTx.AddSerializedReceivingScan(ctx, sessionID, matchedItem.ID, unit.ID, &staffID, isDamage, req.Condition)
	if err != nil {
		return nil, err
	}

	exp, scn, okCount, dmgCount, err := repoTx.GetReceivingSessionTotals(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return &SerializedScanResponse{
		ScanID:           scanID,
		UnitCode:         req.UnitCode,
		Condition:        req.Condition,
		ProductVariantID: unit.ProductVariantID,
		ProductTitle:     unit.ProductTitle,
		ColorName:        unit.ColorName,
		SizeName:         unit.SizeName,
		SellerSKU:        unit.SellerSKU,
		VariantBarcode:   unit.VariantBarcode,
		SessionExpected:  exp,
		SessionScanned:   scn,
		SessionOk:        okCount,
		SessionDamaged:   dmgCount,
		SessionRemaining: exp - scn,
	}, nil
}
