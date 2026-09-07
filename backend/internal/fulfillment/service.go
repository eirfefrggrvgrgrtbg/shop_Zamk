package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type payoutsService interface {
	CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error
	CreatePendingSalesForFulfillmentTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) error
}

type Service struct {
	repo       *Repository
	ordersRepo *orders.Repository
	db         *postgres.Client
	payouts    payoutsService
	notifSvc   *notifications.Service
	logger     *slog.Logger
}

func NewService(repo *Repository, ordersRepo *orders.Repository, db *postgres.Client, payouts payoutsService, notifSvc *notifications.Service) *Service {
	return &Service{
		repo:       repo,
		ordersRepo: ordersRepo,
		db:         db,
		payouts:    payouts,
		notifSvc:   notifSvc,
		logger:     slog.Default(),
	}
}

func (s *Service) SetLogger(l *slog.Logger) {
	if l != nil {
		s.logger = l
	}
}

func (s *Service) CreateShipment(ctx context.Context, adminID, orderID uuid.UUID, req CreateShipmentRequest) (*Shipment, error) {
	var shipment *Shipment

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// Verify order is paid
		order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order.Status != "paid" {
			return ErrOrderNotPaid
		}

		fulfillments, err := s.repo.GetOrderFulfillmentsTx(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if len(fulfillments) > 1 {
			return ErrMultipleFulfillments
		}
		if len(fulfillments) == 0 {
			return errors.New("Заказ не содержит сборок продавцов. Невозможно создать отгрузку.")
		}

		id := fulfillments[0].ID
		fulfillmentID := &id

		// Verify active shipment does not exist
		var existingShipmentExists bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM shipments
				WHERE order_id = $1 AND status NOT IN ('cancelled', 'failed')
			)
		`, orderID).Scan(&existingShipmentExists)
		if err != nil {
			return err
		}
		if existingShipmentExists {
			return ErrShipmentExists
		}

		shipment = &Shipment{
			ID:             uuid.New(),
			OrderID:        orderID,
			FulfillmentID:  fulfillmentID,
			Status:         "pending",
			Carrier:        req.Carrier,
			TrackingNumber: req.TrackingNumber,
			TrackingUrl:    req.TrackingUrl,
		}

		if err := s.repo.CreateShipmentTx(ctx, tx, shipment); err != nil {
			return err
		}

		event := &ShipmentEvent{
			ID:          uuid.New(),
			ShipmentID:  shipment.ID,
			FromStatus:  nil,
			ToStatus:    shipment.Status,
			ActorUserID: &adminID,
			Comment:     func(s string) *string { return &s }("shipment created"),
		}
		return s.repo.CreateShipmentEventTx(ctx, tx, event)
	})

	if err != nil {
		return nil, err
	}
	return shipment, nil
}

func (s *Service) CreateShipmentForFulfillment(ctx context.Context, adminID, fulfillmentID uuid.UUID, req CreateShipmentRequest) (*Shipment, error) {
	var shipment *Shipment

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Resolve parent order ID (plain lookup without locking)
		var orderID uuid.UUID
		err := tx.QueryRow(ctx, `SELECT order_id FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&orderID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFulfillmentNotFound
			}
			return fmt.Errorf("failed to lookup fulfillment order: %w", err)
		}

		// 2. Lock parent order FIRST (authoritative serialization point, prevents deadlocks with cancellation)
		order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order.Status == "cancelled" {
			return ErrOrderCancelled
		}
		if order.Status != "paid" && order.Status != "assembling" && order.Status != "packed" && order.Status != "shipped" && order.Status != "delivered" {
			return ErrOrderNotPaid
		}

		// 3. Lock fulfillment SECOND (consistent lock order: orders -> order_fulfillments)
		var fulfillmentStatus string
		err = tx.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1 FOR UPDATE`, fulfillmentID).Scan(&fulfillmentStatus)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFulfillmentNotFound
			}
			return fmt.Errorf("failed to lock fulfillment: %w", err)
		}
		if fulfillmentStatus == "cancelled" || fulfillmentStatus == "returned" || fulfillmentStatus == "refunded" || fulfillmentStatus == "delivered" {
			return ErrInvalidFulfillmentStatus
		}

		// 4. Verify active shipment doesn't already exist for this fulfillment or order
		var existingShipmentExists bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM shipments
				WHERE (fulfillment_id = $1 OR (order_id = $2 AND fulfillment_id IS NULL))
				  AND status NOT IN ('cancelled', 'failed')
			)
		`, fulfillmentID, orderID).Scan(&existingShipmentExists)
		if err != nil {
			return err
		}
		if existingShipmentExists {
			return ErrShipmentExists
		}

		fid := fulfillmentID
		shipment = &Shipment{
			ID:             uuid.New(),
			OrderID:        orderID,
			FulfillmentID:  &fid,
			Status:         "pending",
			Carrier:        req.Carrier,
			TrackingNumber: req.TrackingNumber,
			TrackingUrl:    req.TrackingUrl,
		}

		if err := s.repo.CreateShipmentTx(ctx, tx, shipment); err != nil {
			return err
		}

		event := &ShipmentEvent{
			ID:          uuid.New(),
			ShipmentID:  shipment.ID,
			FromStatus:  nil,
			ToStatus:    shipment.Status,
			ActorUserID: &adminID,
			Comment:     func(st string) *string { return &st }("shipment created"),
		}
		return s.repo.CreateShipmentEventTx(ctx, tx, event)
	})

	if err != nil {
		return nil, err
	}
	return shipment, nil
}

func (s *Service) UpdateShipmentStatus(ctx context.Context, adminID, shipmentID uuid.UUID, req UpdateShipmentStatusRequest) error {
	validStatuses := map[string]bool{
		"pending": true, "assembling": true, "packed": true, "shipped": true, "delivered": true, "failed": true, "cancelled": true,
	}
	if !validStatuses[req.Status] {
		return ErrInvalidStatus
	}

	var wasDelivered bool
	var reqOrderID uuid.UUID

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		lockedCtx, err := s.repo.LockShipmentForUpdateTx(ctx, tx, shipmentID)
		if err != nil {
			return err
		}

		shipment := lockedCtx.Shipment
		orderStatus := lockedCtx.OrderStatus
		fulfillmentStatus := lockedCtx.FulfillmentStatus
		reqOrderID = shipment.OrderID

		if shipment.Status == req.Status && req.Carrier == nil && req.TrackingNumber == nil && req.TrackingUrl == nil {
			return nil // no changes
		}

		oldStatus := shipment.Status
		if oldStatus == "delivered" || fulfillmentStatus == "delivered" || orderStatus == "delivered" {
			return ErrShipmentDeliveredImmutable
		}
		if oldStatus == "cancelled" || fulfillmentStatus == "cancelled" || orderStatus == "cancelled" {
			return ErrShipmentCancelledImmutable
		}
		if oldStatus == "failed" && req.Status != "failed" {
			return ErrShipmentFailedImmutable
		}
		if req.Status != oldStatus && (req.Status == "shipped" || req.Status == "delivered") {
			return ErrDispatchNotAllowed
		}
		if req.Status == "delivered" && oldStatus != "delivered" {
			wasDelivered = true
		}

		shipment.Status = req.Status
		if req.Carrier != nil {
			shipment.Carrier = req.Carrier
		}
		if req.TrackingNumber != nil {
			shipment.TrackingNumber = req.TrackingNumber
		}
		if req.TrackingUrl != nil {
			shipment.TrackingUrl = req.TrackingUrl
		}

		now := time.Now().UTC()
		if req.Status == "shipped" && shipment.ShippedAt == nil {
			shipment.ShippedAt = &now
		}
		if req.Status == "delivered" && shipment.DeliveredAt == nil {
			shipment.DeliveredAt = &now
		}

		if err := s.repo.UpdateShipmentTx(ctx, tx, shipment); err != nil {
			return err
		}

		if oldStatus != req.Status {
			event := &ShipmentEvent{
				ID:          uuid.New(),
				ShipmentID:  shipment.ID,
				FromStatus:  &oldStatus,
				ToStatus:    req.Status,
				ActorUserID: &adminID,
				Comment:     req.Comment,
			}
			if err := s.repo.CreateShipmentEventTx(ctx, tx, event); err != nil {
				return err
			}

			// Sync order status or fulfillment status
			orderStatusMap := map[string]string{
				"assembling": "assembling",
				"packed":     "packed",
				"shipped":    "shipped",
				"delivered":  "delivered",
			}

			if newStatus, ok := orderStatusMap[req.Status]; ok {
				if shipment.FulfillmentID != nil {
					if req.Status == "shipped" || req.Status == "delivered" {
						f, err := s.repo.GetAdminFulfillmentTx(ctx, tx, *shipment.FulfillmentID)
						if err == nil && f != nil {
							order, errOrder := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, f.OrderID)
							if errOrder == nil && s.notifSvc != nil {
								bodyC := "Ваша сборка отправлена."
								if req.Status == "delivered" {
									bodyC = "Ваша сборка доставлена."
								}
								notifC := notifications.Notification{
									RecipientUserID: &order.UserID,
									RecipientKind:   notifications.RecipientKindCustomer,
									Type:            "shipment_" + req.Status,
									Title:           "Статус отправления обновлен",
									Body:            bodyC,
									EntityType:      "shipment",
									EntityID:        shipment.ID,
								}
								_ = s.notifSvc.CreateNotificationTx(ctx, tx, notifC)

								bodyS := "Сборка отправлена."
								if req.Status == "delivered" {
									bodyS = "Сборка доставлена."
								}
								notifS := notifications.Notification{
									RecipientSellerID: &f.SellerID,
									RecipientKind:     notifications.RecipientKindSeller,
									Type:              "shipment_" + req.Status,
									Title:             "Статус отправления обновлен",
									Body:              bodyS,
									EntityType:        "shipment",
									EntityID:          shipment.ID,
								}
								_ = s.notifSvc.CreateNotificationTx(ctx, tx, notifS)
							}
						}
					}

					if err := s.repo.UpdateFulfillmentStatusTx(ctx, tx, *shipment.FulfillmentID, newStatus); err != nil {
						return err
					}
					if err := s.recalculateParentOrderStatusTx(ctx, tx, shipment.OrderID, adminID); err != nil {
						return err
					}
				} else {
					// Lock order and sync
					order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, shipment.OrderID)
					if err != nil {
						return err
					}
					if order.Status != newStatus {
						if err := s.ordersRepo.UpdateOrderStatusTx(ctx, tx, order.ID, newStatus); err != nil {
							return err
						}
						history := &orders.OrderStatusHistory{
							ID:          uuid.New(),
							OrderID:     order.ID,
							FromStatus:  &order.Status,
							ToStatus:    newStatus,
							ActorUserID: &adminID,
							Comment:     func(st string) *string { return &st }("synced from shipment status"),
						}
						if err := s.ordersRepo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
							return err
						}
					}
				}
			}
		}

		return nil
	})

	if err == nil && wasDelivered && s.payouts != nil {
		// Log error but don't fail shipment update
		_ = s.payouts.CreatePendingSalesForOrder(ctx, reqOrderID)
	}

	return err
}

func (s *Service) recalculateParentOrderStatusTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, adminID uuid.UUID) error {
	order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if order.Status == "awaiting_payment" {
		return nil
	}

	fulfillments, err := s.repo.GetOrderFulfillmentsTx(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if len(fulfillments) == 0 {
		return nil
	}

	allDelivered := true
	allShippedOrDelivered := true
	allPackedOrLater := true
	anyAssemblingOrPacked := false
	allCancelled := true
	anyStarted := false

	for _, f := range fulfillments {
		if f.Status != "delivered" {
			allDelivered = false
		}
		if f.Status != "shipped" && f.Status != "delivered" {
			allShippedOrDelivered = false
		}
		if f.Status != "packed" && f.Status != "shipped" && f.Status != "delivered" {
			allPackedOrLater = false
		}
		if f.Status == "assembling" || f.Status == "packed" {
			anyAssemblingOrPacked = true
		}
		if f.Status != "cancelled" {
			allCancelled = false
		}
		if f.Status != "paid" && f.Status != "awaiting_payment" && f.Status != "cancelled" {
			anyStarted = true
		}
	}

	var newStatus string
	if allCancelled {
		newStatus = "cancelled"
	} else if allDelivered {
		newStatus = "delivered"
	} else if allShippedOrDelivered {
		newStatus = "shipped"
	} else if allPackedOrLater {
		newStatus = "packed"
	} else if anyAssemblingOrPacked || anyStarted {
		newStatus = "assembling"
	} else {
		newStatus = "paid"
	}

	if order.Status != newStatus {
		if err := s.ordersRepo.UpdateOrderStatusTx(ctx, tx, order.ID, newStatus); err != nil {
			return err
		}
		history := &orders.OrderStatusHistory{
			ID:          uuid.New(),
			OrderID:     order.ID,
			FromStatus:  &order.Status,
			ToStatus:    newStatus,
			ActorUserID: &adminID,
			Comment:     func(st string) *string { return &st }("recalculated from fulfillment statuses"),
		}
		if err := s.ordersRepo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GetAdminShipment(ctx context.Context, shipmentID uuid.UUID) (*Shipment, error) {
	return s.repo.GetShipment(ctx, shipmentID)
}

func (s *Service) ListAdminShipments(ctx context.Context, limit, offset int) ([]Shipment, error) {
	return s.repo.ListShipments(ctx, limit, offset)
}

func (s *Service) GetCustomerShipment(ctx context.Context, userID, orderID uuid.UUID) (*Shipment, error) {
	order, err := s.ordersRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, ErrUnauthorized
	}
	return s.repo.GetShipmentByOrderID(ctx, orderID)
}

func (s *Service) GetSellerShipment(ctx context.Context, userID, orderID uuid.UUID) (*Shipment, error) {
	sellerID, err := s.ordersRepo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	// Verify seller has items in this order
	_, err = s.ordersRepo.GetSellerOrder(ctx, sellerID, orderID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	// Return limited details (filtering done in handler/dto mapping)
	return s.repo.GetShipmentByOrderID(ctx, orderID)
}
