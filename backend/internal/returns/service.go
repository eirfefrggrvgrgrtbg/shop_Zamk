package returns

import (
	"context"
	"sort"
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

func (s *Service) CreateReturn(ctx context.Context, userID, orderID uuid.UUID, req CreateReturnRequest) ([]ReturnResponse, error) {
	var responses []ReturnResponse

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Validate order belongs to customer
		order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order.UserID != userID {
			return ErrUnauthorized
		}

		// 2. Lock requested order items in deterministic order to prevent deadlock
		var requestedItemIDs []uuid.UUID
		reqItemMap := make(map[uuid.UUID]CreateReturnItemRequest)
		for _, itemReq := range req.Items {
			if itemReq.Quantity <= 0 {
				return ErrInvalidQuantity
			}
			requestedItemIDs = append(requestedItemIDs, itemReq.OrderItemID)
			reqItemMap[itemReq.OrderItemID] = itemReq
		}

		if len(requestedItemIDs) == 0 {
			return ErrInvalidQuantity
		}

		// Lock order items
		query := `SELECT id, order_id, order_fulfillment_id, quantity, seller_id FROM order_items WHERE id = ANY($1) AND order_id = $2 ORDER BY id FOR UPDATE`
		rows, err := tx.Query(ctx, query, requestedItemIDs, orderID)
		if err != nil {
			return err
		}
		defer rows.Close()

		type lockedItem struct {
			ID            uuid.UUID
			FulfillmentID *uuid.UUID
			Quantity      int
			SellerID      uuid.UUID
		}

		var lockedItems []lockedItem
		for rows.Next() {
			var li lockedItem
			var oid uuid.UUID
			if err := rows.Scan(&li.ID, &oid, &li.FulfillmentID, &li.Quantity, &li.SellerID); err != nil {
				return err
			}
			lockedItems = append(lockedItems, li)
		}
		rows.Close()

		if len(lockedItems) != len(requestedItemIDs) {
			return ErrInvalidQuantity // some items missing or not in order
		}

		// Group by fulfillment deterministically
		fulfillmentGroups := make(map[uuid.UUID][]lockedItem)
		var fulfillmentIDs []uuid.UUID
		for _, li := range lockedItems {
			if li.FulfillmentID == nil {
				return ErrOrderNotDelivered // FBO items must have fulfillment
			}
			if _, exists := fulfillmentGroups[*li.FulfillmentID]; !exists {
				fulfillmentIDs = append(fulfillmentIDs, *li.FulfillmentID)
			}
			fulfillmentGroups[*li.FulfillmentID] = append(fulfillmentGroups[*li.FulfillmentID], li)
		}

		sort.Slice(fulfillmentIDs, func(i, j int) bool {
			return fulfillmentIDs[i].String() < fulfillmentIDs[j].String()
		})

		for _, fulfillmentID := range fulfillmentIDs {
			itemsGroup := fulfillmentGroups[fulfillmentID]
			// Resolve canonical seller and shipment from order_fulfillments
			var sellerID uuid.UUID
			var shipmentStatus *string
			var deliveredAt *time.Time
			err := tx.QueryRow(ctx, `
				SELECT of.seller_id, s.status, s.delivered_at
				FROM order_fulfillments of
				LEFT JOIN shipments s ON s.fulfillment_id = of.id
				WHERE of.id = $1 AND of.order_id = $2
			`, fulfillmentID, orderID).Scan(&sellerID, &shipmentStatus, &deliveredAt)
			if err != nil {
				return err
			}
			if shipmentStatus == nil || *shipmentStatus != "delivered" || deliveredAt == nil {
				return ErrOrderNotDelivered
			}

			windowDeadline := deliveredAt.AddDate(0, 0, s.windowDays)
			if time.Now().After(windowDeadline) {
				return ErrReturnWindowExpired
			}

			ret := &Return{
				ID:            uuid.New(),
				OrderID:       orderID,
				FulfillmentID: fulfillmentID,
				UserID:        userID,
				Status:        "requested",
				Reason:        req.Reason,
				Comment:       req.Comment,
			}

			var retItems []ReturnItem

			for _, li := range itemsGroup {
				itemReq := reqItemMap[li.ID]

				// Calculate active return qty (locking guarantees safe sum)
				var returnedQty int
				err := tx.QueryRow(ctx, `
					SELECT COALESCE(SUM(ri.quantity), 0)
					FROM return_items ri
					JOIN returns r ON r.id = ri.return_id
					WHERE ri.order_item_id = $1 AND r.status NOT IN ('rejected', 'cancelled')
				`, li.ID).Scan(&returnedQty)
				if err != nil {
					return err
				}

				if itemReq.Quantity > (li.Quantity - returnedQty) {
					return ErrInvalidQuantity
				}

				retItems = append(retItems, ReturnItem{
					ID:               uuid.New(),
					ReturnID:         ret.ID,
					OrderItemID:      li.ID,
					Quantity:         itemReq.Quantity,
					Reason:           itemReq.Reason,
					Condition:        itemReq.Condition,
					Restock:          false,
					AcceptedQuantity: 0,
					DamagedQuantity:  0,
					RejectedQuantity: 0,
				})
			}

			if err := s.repo.CreateReturnTx(ctx, tx, ret, retItems); err != nil {
				return err
			}

			// Notify seller safely from canonical order_fulfillments.seller_id
			if s.notifs != nil {
				_ = s.notifs.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientSellerID: &sellerID,
					RecipientKind:     notifications.RecipientKindSeller,
					Type:              notifications.TypeReturnCreated,
					Title:             "Новая заявка на возврат",
					Body:              "Покупатель запросил возврат товара.",
					EntityType:        "return",
					EntityID:          ret.ID,
				})
			}

			responses = append(responses, ReturnResponse{
				Return: *ret,
				Items:  retItems,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return responses, nil
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

			var sellerID uuid.UUID
			var orderUserID uuid.UUID
			err := tx.QueryRow(ctx, `
				SELECT of.seller_id, o.user_id
				FROM order_fulfillments of
				JOIN orders o ON o.id = of.order_id
				WHERE of.id = $1
			`, ret.FulfillmentID).Scan(&sellerID, &orderUserID)
			if err == nil {
				_ = s.notifs.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientSellerID: &sellerID,
					RecipientKind:     notifications.RecipientKindSeller,
					Type:              notifType,
					Title:             title,
					Body:              body,
					EntityType:        "return",
					EntityID:          ret.ID,
				})
				_ = s.notifs.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientUserID: &orderUserID,
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

		// Process payouts deduction for return items (financial deduction only; old generic physical restock removed before M5.2)
		var deductionItems []payouts.ReturnItemDeduction
		for _, item := range items {
			deductionItems = append(deductionItems, payouts.ReturnItemDeduction{
				OrderItemID: item.OrderItemID,
				Quantity:    item.Quantity,
			})
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

func (s *Service) GetAdminReturnReceivingState(ctx context.Context, returnID uuid.UUID) (*AdminReturnReceivingState, error) {
	return s.repo.GetReturnReceivingState(ctx, returnID)
}

func (s *Service) StartReceiving(ctx context.Context, returnID uuid.UUID) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		if ret.Status == "receiving" {
			return nil // idempotent
		}

		if ret.Status != "approved" {
			return ErrInvalidStatusTransition
		}

		now := time.Now()
		ret.Status = "receiving"
		ret.ReceivingStartedAt = &now

		return s.repo.UpdateReturnTx(ctx, tx, ret)
	})
}

func (s *Service) ScanReturnUnit(ctx context.Context, returnID uuid.UUID, req ScanReturnUnitRequest) (*ScanReturnUnitResponse, error) {
	var resp ScanReturnUnitResponse
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		if ret.Status != "receiving" {
			return ErrReturnNotInReceiving
		}

		allocRes, err := s.repo.GetAllocationByZMUCode(ctx, req.Code)
		if err != nil {
			return ErrInvalidZMUForReturn
		}

		if allocRes.UnitStatus != "shipped" {
			return ErrInvalidZMUForReturn
		}

		if allocRes.PickedAt == nil {
			return ErrInvalidZMUForReturn
		}

		if allocRes.ReleasedAt != nil {
			return ErrInvalidZMUForReturn
		}

		if allocRes.OrderID != ret.OrderID || allocRes.FulfillmentID != ret.FulfillmentID {
			return ErrInvalidZMUForReturn
		}

		existingUnit, err := s.repo.GetReturnItemUnitByAllocationID(ctx, allocRes.OrderItemAllocationID)
		if err != nil {
			return err
		}

		if existingUnit != nil {
			// Check if bound to THIS return
			var itemForUnit ReturnItem
			err := tx.QueryRow(ctx, "SELECT return_id FROM return_items WHERE id = $1", existingUnit.ReturnItemID).Scan(&itemForUnit.ReturnID)
			if err != nil {
				return err
			}
			if itemForUnit.ReturnID == ret.ID {
				resp.AlreadyScanned = true
				resp.ReturnItemUnit = *existingUnit
				return nil
			}
			return ErrAllocationAlreadyBound
		}

		items, err := s.repo.GetReturnItemsForUpdateTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		hasItemInReturn := false
		var targetItem *ReturnItem
		for _, it := range items {
			if it.OrderItemID == allocRes.OrderItemID {
				hasItemInReturn = true
				count, err := s.repo.GetScannedUnitCountForReturnItemTx(ctx, tx, it.ID)
				if err != nil {
					return err
				}
				if count < it.Quantity {
					itCopy := it
					targetItem = &itCopy
					break
				}
			}
		}

		if targetItem == nil {
			if !hasItemInReturn {
				return ErrInvalidZMUForReturn
			}
			return ErrQuantityExceeded
		}

		now := time.Now()
		newUnit := ReturnItemUnit{
			ID:                    uuid.New(),
			ReturnItemID:          targetItem.ID,
			OrderItemAllocationID: allocRes.OrderItemAllocationID,
			ScannedAt:             &now,
		}

		if err := s.repo.CreateReturnItemUnitTx(ctx, tx, &newUnit); err != nil {
			return err
		}

		resp.AlreadyScanned = false
		resp.ReturnItemUnit = newUnit
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &resp, nil
}
