package fulfillment

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
)

func (s *Service) ResolveReceivingCode(ctx context.Context, code string) (*Fulfillment, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("code is required")
	}

	f, err := s.repo.GetFulfillmentByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) StartReceivingSession(ctx context.Context, staffID *uuid.UUID, fulfillmentID uuid.UUID) (*ReceivingSession, error) {
	var sess *ReceivingSession
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		var err error
		sess, err = s.repo.StartReceivingSessionTx(ctx, tx, fulfillmentID, staffID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *Service) ScanReceivingItem(ctx context.Context, fulfillmentID uuid.UUID, req ScanItemRequest) (*ReceivingSession, error) {
	if strings.TrimSpace(req.Barcode) == "" {
		return nil, ErrInvalidBarcode
	}
	var sess *ReceivingSession
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		var err error
		sess, err = s.repo.ScanReceivingItemTx(ctx, tx, fulfillmentID, req.Barcode, req.ExpectedVersion, req.IdempotencyKey)
		return err
	})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *Service) ConfirmReceiving(ctx context.Context, staffID, fulfillmentID uuid.UUID, req ConfirmReceivingRequest) (*Shipment, error) {
	var createdShipment *Shipment

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		sessionID, err := uuid.Parse(req.SessionID)
		if err != nil {
			return errors.New("invalid session id")
		}

		sh, err := s.repo.ConfirmReceivingSessionTx(ctx, tx, fulfillmentID, sessionID, req.ExpectedVersion, staffID, req.Comment)
		if err != nil {
			return err
		}
		createdShipment = sh

		// Notify staff and recalculate parent order status
		if s.notifSvc != nil {
			notif := notifications.Notification{
				RecipientKind: notifications.RecipientKindStaff,
				Type:          notifications.TypeStaffFulfillmentPacked,
				Title:         "Сборка принята на хабе",
				Body:          "Сборка успешно принята и создано отправление.",
				EntityType:    "fulfillment",
				EntityID:      fulfillmentID,
			}
			s.notifSvc.CreateStaffNotificationTx(ctx, tx, notif)
		}

		// Get fulfillment order_id
		f, err := s.repo.GetAdminFulfillmentTx(ctx, tx, fulfillmentID)
		if err == nil {
			_ = s.recalculateParentOrderStatusTx(ctx, tx, f.OrderID, staffID)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return createdShipment, nil
}

func (s *Service) RecordDiscrepancy(ctx context.Context, staffID, fulfillmentID uuid.UUID, req RecordDiscrepancyRequest) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		var sessionID uuid.UUID
		if req.SessionID != "" {
			var err error
			sessionID, err = uuid.Parse(req.SessionID)
			if err != nil {
				return errors.New("invalid session id")
			}
		}

		if err := s.repo.RecordReceivingDiscrepancySessionTx(ctx, tx, fulfillmentID, sessionID, req.Reason, req.Comment, staffID); err != nil {
			return err
		}

		f, err := s.repo.GetAdminFulfillmentTx(ctx, tx, fulfillmentID)
		if err != nil {
			return err
		}

		if s.notifSvc != nil {
			notif := notifications.Notification{
				RecipientSellerID: &f.SellerID,
				RecipientKind:     notifications.RecipientKindSeller,
				Type:              "fulfillment_discrepancy",
				Title:             "Обнаружено расхождение при приёмке",
				Body:              "На хабе зафиксировано расхождение по вашей сборке. Причина: " + req.Reason,
				EntityType:        "fulfillment",
				EntityID:          fulfillmentID,
			}
			if err := s.notifSvc.CreateNotificationTx(ctx, tx, notif); err != nil {
				return err
			}
		}
		return s.recalculateParentOrderStatusTx(ctx, tx, f.OrderID, staffID)
	})
}
