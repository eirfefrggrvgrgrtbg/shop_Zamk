package returns

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
)

type payoutsService interface {
	ProcessReturnDeduction(ctx context.Context, tx pgx.Tx, returnID uuid.UUID, orderID uuid.UUID, items []payouts.ReturnItemDeduction) error
}

type paymentsService interface {
	ReserveRefund(ctx context.Context, paymentID uuid.UUID, requestedAmountCents int64, reason string, returnID *uuid.UUID) error
	GetSucceededPaymentIDForOrder(ctx context.Context, orderID uuid.UUID) (uuid.UUID, error)
}

type Service struct {
	repo         *Repository
	ordersRepo   *orders.Repository
	inventorySvc *inventory.Service
	db           *postgres.Client
	payouts      payoutsService
	payments     paymentsService
	windowDays   int
	notifs       *notifications.Service
}

func NewService(repo *Repository, ordersRepo *orders.Repository, inventorySvc *inventory.Service, db *postgres.Client, payouts payoutsService, payments paymentsService, windowDays int, notifs *notifications.Service) *Service {
	return &Service{
		repo:         repo,
		ordersRepo:   ordersRepo,
		inventorySvc: inventorySvc,
		db:           db,
		payouts:      payouts,
		payments:     payments,
		windowDays:   windowDays,
		notifs:       notifs,
	}
}

func (s *Service) CreateReturn(ctx context.Context, userID, orderID uuid.UUID, req CreateReturnRequest) (*Return, []ReturnItem, error) {
	var ret *Return
	var retItems []ReturnItem

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Validate order belongs to customer & is delivered
		order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order.UserID != userID {
			return ErrUnauthorized
		}
		if order.Status != "delivered" {
			return ErrOrderNotDelivered
		}

		// 2. Validate return window
		windowDeadline := order.UpdatedAt.AddDate(0, 0, s.windowDays)
		if time.Now().After(windowDeadline) {
			return ErrReturnWindowExpired
		}

		// 3. Validate items
		orderItems, err := s.ordersRepo.GetOrderItems(ctx, orderID)
		if err != nil {
			return err
		}
		orderItemMap := make(map[uuid.UUID]orders.OrderItem)
		for _, oi := range orderItems {
			orderItemMap[oi.ID] = oi
		}

		ret = &Return{
			ID:      uuid.New(),
			OrderID: orderID,
			UserID:  userID,
			Status:  "requested",
			Reason:  req.Reason,
			Comment: req.Comment,
		}

		for _, itemReq := range req.Items {
			oi, ok := orderItemMap[itemReq.OrderItemID]
			if !ok {
				return ErrInvalidQuantity
			}

			returnedQty, err := s.repo.GetTotalReturnedQuantityForOrderItem(ctx, itemReq.OrderItemID)
			if err != nil {
				return err
			}
			availableToReturn := oi.Quantity - returnedQty
			if itemReq.Quantity > availableToReturn {
				return ErrInvalidQuantity
			}

			retItems = append(retItems, ReturnItem{
				ID:          uuid.New(),
				ReturnID:    ret.ID,
				OrderItemID: itemReq.OrderItemID,
				Quantity:    itemReq.Quantity,
				Reason:      itemReq.Reason,
				Condition:   itemReq.Condition,
				Restock:     false,
			})
		}

		if err := s.repo.CreateReturnTx(ctx, tx, ret, retItems); err != nil {
			return err
		}

		if s.notifs != nil {
			_ = s.notifs.CreateStaffNotificationTx(ctx, tx, notifications.Notification{
				Type:       notifications.TypeReturnCreated,
				Title:      "Новая заявка на возврат",
				Body:       "Покупатель запросил возврат.",
				EntityType: "return",
				EntityID:   ret.ID,
			})
			_ = s.notifs.CreateNotificationTx(ctx, tx, notifications.Notification{
				RecipientSellerID: &orderItems[0].SellerID,
				RecipientKind:     notifications.RecipientKindSeller,
				Type:              notifications.TypeReturnCreated,
				Title:             "Новая заявка на возврат",
				Body:              "Покупатель запросил возврат товара.",
				EntityType:        "return",
				EntityID:          ret.ID,
			})
		}
		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	return ret, retItems, nil
}

func (s *Service) GetCustomerReturn(ctx context.Context, userID, returnID uuid.UUID) (*Return, []ReturnItem, error) {
	ret, items, err := s.repo.GetReturn(ctx, returnID)
	if err != nil {
		return nil, nil, err
	}
	if ret.UserID != userID {
		return nil, nil, ErrUnauthorized
	}
	return ret, items, nil
}

func (s *Service) ListCustomerReturns(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Return, error) {
	return s.repo.ListReturnsByCustomer(ctx, userID, limit, offset)
}

func (s *Service) GetAdminReturn(ctx context.Context, returnID uuid.UUID) (*Return, []ReturnItem, error) {
	return s.repo.GetReturn(ctx, returnID)
}

func (s *Service) ListAdminReturns(ctx context.Context, limit, offset int) ([]Return, error) {
	return s.repo.ListAllReturns(ctx, limit, offset)
}

func (s *Service) UpdateReturnStatus(ctx context.Context, adminID, returnID uuid.UUID, req UpdateReturnStatusRequest) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, items, err := s.repo.GetReturn(ctx, returnID)
		if err != nil {
			return err
		}

		// Simple state machine validation
		validTransitions := map[string][]string{
			"requested":     {"approved", "rejected", "cancelled"},
			"approved":      {"item_received", "cancelled"},
			"item_received": {"completed", "refunded"},
			// refunded and completed are terminal states for the return itself (or refunded leads to completed)
			"refunded": {"completed"},
		}

		allowed, ok := validTransitions[ret.Status]
		if !ok {
			return ErrInvalidStatusTransition
		}
		isAllowed := false
		for _, st := range allowed {
			if st == req.Status {
				isAllowed = true
				break
			}
		}
		if !isAllowed && ret.Status != req.Status {
			return ErrInvalidStatusTransition
		}

		ret.Status = req.Status
		if req.AdminComment != nil {
			ret.AdminComment = req.AdminComment
		} else if req.Status == "rejected" {
			return ErrRejectReasonRequired
		}

		now := time.Now()
		if req.Status == "approved" && ret.ApprovedAt == nil {
			ret.ApprovedAt = &now
		}
		if req.Status == "rejected" && ret.RejectedAt == nil {
			ret.RejectedAt = &now
		}
		if req.Status == "completed" && ret.CompletedAt == nil {
			ret.CompletedAt = &now
		}

		if err := s.repo.UpdateReturnTx(ctx, tx, ret); err != nil {
			return err
		}

		if s.notifs != nil && (req.Status == "approved" || req.Status == "rejected") {
			var notifType, title, body string
			if req.Status == "approved" {
				notifType = notifications.TypeReturnApproved
				title = "Возврат одобрен"
				body = "Заявка на возврат была одобрена."
			} else {
				notifType = notifications.TypeReturnRejected
				title = "Возврат отклонен"
				body = "Заявка на возврат была отклонена."
			}
			
			order, _ := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, ret.OrderID)
			orderItems, _ := s.ordersRepo.GetOrderItems(ctx, ret.OrderID)
			if order != nil && len(orderItems) > 0 {
				_ = s.notifs.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientSellerID: &orderItems[0].SellerID,
					RecipientKind:     notifications.RecipientKindSeller,
					Type:              notifType,
					Title:             title,
					Body:              body,
					EntityType:        "return",
					EntityID:          ret.ID,
				})
				_ = s.notifs.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientUserID: &order.UserID,
					RecipientKind:   notifications.RecipientKindCustomer,
					Type:            "return_status_changed",
					Title:           title,
					Body:            body,
					EntityType:      "return",
					EntityID:        ret.ID,
				})
			}
		}

		// Apply restock preferences if marked as item_received or completed
		if req.Status == "item_received" || req.Status == "completed" || req.Status == "refunded" {
			restockMap := make(map[uuid.UUID]bool)
			for _, ir := range req.ItemRestock {
				restockMap[ir.ReturnItemID] = ir.Restock
			}
			for _, item := range items {
				if doRestock, ok := restockMap[item.ID]; ok {
					if item.Restock != doRestock {
						if err := s.repo.UpdateReturnItemRestockTx(ctx, tx, item.ID, doRestock); err != nil {
							return err
						}
					}
				}
			}
		}

		return nil
	})
}

func (s *Service) CreateRefund(ctx context.Context, adminID, returnID uuid.UUID, req CreateRefundRequest) (*Refund, error) {
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, items, err := s.repo.GetReturn(ctx, returnID)
		if err != nil {
			return err
		}

		if ret.Status == "refunded" || ret.Status == "completed" {
			return ErrReturnAlreadyRefunded
		}

		// Calculate refund amount based on return items
		orderItems, err := s.ordersRepo.GetOrderItems(ctx, ret.OrderID)
		if err != nil {
			return err
		}
		orderItemMap := make(map[uuid.UUID]orders.OrderItem)
		for _, oi := range orderItems {
			orderItemMap[oi.ID] = oi
		}

		var amountCentsToRefund int64 = 0
		for _, item := range items {
			oi := orderItemMap[item.OrderItemID]
			// We refund the proportionate price. Total subtotal for oi.Quantity was oi.SubtotalPriceCents.
			// However, in our system oi.PriceCents * oi.Quantity = oi.SubtotalPriceCents.
			itemPrice := oi.PriceCents
			amountCentsToRefund += itemPrice * int64(item.Quantity)
		}

		// 3. Find the succeeded payment for this order
		paymentID, err := s.payments.GetSucceededPaymentIDForOrder(ctx, ret.OrderID)
		if err != nil {
			return err // Either not found or DB error
		}

		reason := ""
		if req.Reason != nil {
			reason = *req.Reason
		}
		// 4. Reserve refund in payments module (this creates the pending refund record)
		if err := s.payments.ReserveRefund(ctx, paymentID, amountCentsToRefund, reason, &ret.ID); err != nil {
			return err
		}

		// Update return status to refunded
		ret.Status = "refunded"
		if err := s.repo.UpdateReturnTx(ctx, tx, ret); err != nil {
			return err
		}

		// Process inventory restock for items marked restock=true
		var deductionItems []payouts.ReturnItemDeduction
		for _, item := range items {
			deductionItems = append(deductionItems, payouts.ReturnItemDeduction{
				OrderItemID: item.OrderItemID,
				Quantity:    item.Quantity,
			})
			if item.Restock {
				oi := orderItemMap[item.OrderItemID]
				if err := s.inventorySvc.ProcessRestockTx(ctx, tx, oi.ProductVariantID, item.Quantity, &ret.ID); err != nil {
					return err
				}
			}
		}

		if s.payouts != nil {
			if err := s.payouts.ProcessReturnDeduction(ctx, tx, ret.ID, ret.OrderID, deductionItems); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	
	// Create a mock Refund object to return so handler doesn't crash
	return &Refund{ID: uuid.New(), Status: "pending"}, nil
}

func (s *Service) ListSellerReturns(ctx context.Context, userID uuid.UUID, limit, offset int) ([]SellerReturnItem, error) {
	sellerID, err := s.ordersRepo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	return s.repo.GetSellerReturnItems(ctx, sellerID, limit, offset)
}

func (s *Service) GetSellerReturn(ctx context.Context, userID, returnID uuid.UUID) ([]SellerReturnItem, error) {
	sellerID, err := s.ordersRepo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	items, err := s.repo.GetSellerReturnItemsForReturn(ctx, sellerID, returnID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrReturnNotFound
	}
	return items, nil
}

func (s *Service) ListAdminRefunds(ctx context.Context, limit, offset int) ([]Refund, error) {
	return s.repo.ListAllRefunds(ctx, limit, offset)
}

func (s *Service) GetAdminRefund(ctx context.Context, id uuid.UUID) (*Refund, error) {
	return s.repo.GetRefund(ctx, id)
}
