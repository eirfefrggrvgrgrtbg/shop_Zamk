package fulfillment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type payoutsService interface {
	CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error
}

type Service struct {
	repo       *Repository
	ordersRepo *orders.Repository
	db         *postgres.Client
	payouts    payoutsService
}

func NewService(repo *Repository, ordersRepo *orders.Repository, db *postgres.Client, payouts payoutsService) *Service {
	return &Service{
		repo:       repo,
		ordersRepo: ordersRepo,
		db:         db,
		payouts:    payouts,
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

		// Verify shipment does not exist
		existing, err := s.repo.GetShipmentByOrderID(ctx, orderID)
		if err == nil && existing != nil {
			return ErrShipmentExists
		}
		if err != nil && err != ErrShipmentNotFound {
			return err
		}

		shipment = &Shipment{
			ID:             uuid.New(),
			OrderID:        orderID,
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
		shipment, err := s.repo.GetShipment(ctx, shipmentID)
		if err != nil {
			return err
		}
		
		reqOrderID = shipment.OrderID

		if shipment.Status == req.Status && req.Carrier == nil && req.TrackingNumber == nil && req.TrackingUrl == nil {
			return nil // no changes
		}

		oldStatus := shipment.Status
		if oldStatus == "delivered" {
			return errors.New("cannot change status of delivered shipment")
		}
		if oldStatus == "cancelled" {
			return errors.New("cannot change status of cancelled shipment")
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

		now := time.Now()
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

			// Sync order status
			orderStatusMap := map[string]string{
				"assembling": "assembling",
				"packed":     "packed",
				"shipped":    "shipped",
				"delivered":  "delivered",
			}

			if newOrderStatus, ok := orderStatusMap[req.Status]; ok {
				// Lock order and sync
				order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, shipment.OrderID)
				if err != nil {
					return err
				}
				if order.Status != newOrderStatus {
					if err := s.ordersRepo.UpdateOrderStatusTx(ctx, tx, order.ID, newOrderStatus); err != nil {
						return err
					}
					history := &orders.OrderStatusHistory{
						ID:          uuid.New(),
						OrderID:     order.ID,
						FromStatus:  &order.Status,
						ToStatus:    newOrderStatus,
						ActorUserID: &adminID,
						Comment:     func(s string) *string { return &s }("synced from shipment status"),
					}
					if err := s.ordersRepo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
						return err
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
