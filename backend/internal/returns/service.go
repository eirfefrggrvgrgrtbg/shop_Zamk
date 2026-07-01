package returns

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type payoutsService interface {
	ProcessRefundDeduction(ctx context.Context, refundID uuid.UUID, returnID uuid.UUID, orderID uuid.UUID, amountCents int64) error
}

type Service struct {
	repo         *Repository
	ordersRepo   *orders.Repository
	inventorySvc *inventory.Service
	db           *postgres.Client
	payouts      payoutsService
	windowDays   int
}

func NewService(repo *Repository, ordersRepo *orders.Repository, inventorySvc *inventory.Service, db *postgres.Client, payouts payoutsService, windowDays int) *Service {
	return &Service{
		repo:         repo,
		ordersRepo:   ordersRepo,
		inventorySvc: inventorySvc,
		db:           db,
		payouts:      payouts,
		windowDays:   windowDays,
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

		return s.repo.CreateReturnTx(ctx, tx, ret, retItems)
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
	var ref *Refund

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, items, err := s.repo.GetReturn(ctx, returnID)
		if err != nil {
			return err
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

		// Fetch the order to ensure we don't exceed total paid
		order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, ret.OrderID)
		if err != nil {
			return err
		}

		totalRefunded, err := s.repo.GetTotalRefundedAmountForOrder(ctx, ret.OrderID)
		if err != nil {
			return err
		}

		// We assume order.TotalPriceCents is the max we can refund.
		if amountCentsToRefund+totalRefunded > order.TotalPriceCents {
			return ErrRefundExceedsPaid
		}

		providerName := "tbank-stub"
		providerRefundID := uuid.New().String()
		now := time.Now()

		ref = &Refund{
			ID:               uuid.New(),
			ReturnID:         &ret.ID,
			PaymentID:        nil, // We would lookup original payment ID here, simplified for Phase 9A
			OrderID:          ret.OrderID,
			Status:           "succeeded", // stubbed immediate success
			AmountCents:      amountCentsToRefund,
			Currency:         order.Currency,
			Provider:         &providerName,
			ProviderRefundID: &providerRefundID,
			Reason:           req.Reason,
			ProcessedAt:      &now,
		}

		if err := s.repo.CreateRefundTx(ctx, tx, ref); err != nil {
			return err
		}

		// Update return status to refunded
		ret.Status = "refunded"
		if err := s.repo.UpdateReturnTx(ctx, tx, ret); err != nil {
			return err
		}

		// Process inventory restock for items marked restock=true
		for _, item := range items {
			if item.Restock {
				oi := orderItemMap[item.OrderItemID]
				if err := s.inventorySvc.ProcessRestockTx(ctx, tx, oi.ProductVariantID, item.Quantity, &ret.ID); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err == nil && s.payouts != nil {
		_ = s.payouts.ProcessRefundDeduction(ctx, ref.ID, *ref.ReturnID, ref.OrderID, ref.AmountCents)
	}

	if err != nil {
		return nil, err
	}
	return ref, nil
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
