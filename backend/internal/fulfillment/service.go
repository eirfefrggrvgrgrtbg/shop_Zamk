package fulfillment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
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
	notifSvc   *notifications.Service
}

func NewService(repo *Repository, ordersRepo *orders.Repository, db *postgres.Client, payouts payoutsService, notifSvc *notifications.Service) *Service {
	return &Service{
		repo:       repo,
		ordersRepo: ordersRepo,
		db:         db,
		payouts:    payouts,
		notifSvc:   notifSvc,
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

		fulfillments, err := s.repo.GetOrderFulfillments(ctx, orderID)
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
		// Load fulfillment
		fulfillment, err := s.repo.GetAdminFulfillment(ctx, fulfillmentID)
		if err != nil {
			return err
		}

		// Verify order is paid
		order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, fulfillment.OrderID)
		if err != nil {
			return err
		}
		if order.Status != "paid" && order.Status != "assembling" && order.Status != "packed" && order.Status != "shipped" && order.Status != "delivered" {
			return errors.New("order is not paid or in a state that can be fulfilled") // Using a generic error string
		}

		// Verify fulfillment status
		if fulfillment.Status == "cancelled" || fulfillment.Status == "returned" || fulfillment.Status == "refunded" || fulfillment.Status == "delivered" {
			return errors.New("cannot create shipment for this fulfillment status")
		}

		// Verify shipment doesn't already exist for this fulfillment
		if fulfillment.ShipmentID != nil {
			return ErrShipmentExists
		}

		fid := fulfillmentID
		shipment = &Shipment{
			ID:             uuid.New(),
			OrderID:        fulfillment.OrderID,
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
							if errOrder == nil {
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
								s.notifSvc.CreateNotificationTx(ctx, tx, notifC)
							}

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
							s.notifSvc.CreateNotificationTx(ctx, tx, notifS)
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

func (s *Service) UpdateAdminOrderFulfillmentStatus(ctx context.Context, adminID, orderID uuid.UUID, status string) error {
	if !IsValidFulfillmentStatus(status) {
		return ErrInvalidStatus
	}

	// This is a simplified approach, updating all fulfillments for the order.
	if err := s.repo.UpdateOrderFulfillmentsStatus(ctx, orderID, status); err != nil {
		return err
	}

	return nil
}

func IsValidFulfillmentStatus(status string) bool {
	validStatuses := map[string]bool{
		"awaiting_payment": true,
		"paid":             true,
		"assembling":       true,
		"packed":           true,
		"shipped":          true,
		"delivered":        true,
		"cancelled":        true,
		"returned":         true,
		"refunded":         true,
	}
	return validStatuses[status]
}
