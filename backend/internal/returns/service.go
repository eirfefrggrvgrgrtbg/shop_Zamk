package returns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
)

type payoutsService interface {
	ProcessReturnDeduction(ctx context.Context, tx pgx.Tx, returnID uuid.UUID, orderID uuid.UUID, items []payouts.ReturnItemDeduction) error
}

type paymentsService interface {
	ReserveRefund(ctx context.Context, paymentID uuid.UUID, requestedAmountCents int64, reason string, returnID *uuid.UUID) error
	ReserveRefundTx(ctx context.Context, tx pgx.Tx, paymentID uuid.UUID, requestedAmountCents int64, reason string, returnID *uuid.UUID) error
	GetSucceededPaymentIDForOrder(ctx context.Context, orderID uuid.UUID) (uuid.UUID, error)
}

type Service struct {
	storageProvider   storage.Provider
	repo              *Repository
	ordersRepo        *orders.Repository
	inventorySvc      *inventory.Service
	db                *postgres.Client
	payouts           payoutsService
	payments          paymentsService
	windowDays        int
	notifs            *notifications.Service
	logisticsProvider ReturnLogisticsProvider
}

func NewService(repo *Repository, ordersRepo *orders.Repository, inventorySvc *inventory.Service, db *postgres.Client, payouts payoutsService, payments paymentsService, windowDays int, notifs *notifications.Service, storageProvider storage.Provider, logisticsProvider ReturnLogisticsProvider) *Service {
	return &Service{
		storageProvider:   storageProvider,
		repo:              repo,
		ordersRepo:        ordersRepo,
		inventorySvc:      inventorySvc,
		db:                db,
		payouts:           payouts,
		payments:          payments,
		windowDays:        windowDays,
		notifs:            notifs,
		logisticsProvider: logisticsProvider,
	}
}

func (s *Service) SetStorageProvider(p storage.Provider) {
	s.storageProvider = p
}

func (s *Service) UploadReturnEvidence(ctx context.Context, customerID uuid.UUID, file io.Reader, filename string, size int64, contentType string) (*UploadEvidenceResponse, error) {
	if s.storageProvider == nil {
		return nil, errors.New("storage provider not configured")
	}

	if size > 10*1024*1024 {
		return nil, errors.New("file too large")
	}

	validMimes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	if !validMimes[contentType] {
		return nil, ErrEvidenceInvalidFormat
	}

	ext := strings.ToLower(filepath.Ext(filename))
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}
	if !validExts[ext] {
		return nil, ErrEvidenceInvalidFormat
	}

	evidenceID := uuid.New()
	objectKey := "returns/" + customerID.String() + "/" + evidenceID.String() + ext

	stored, err := s.storageProvider.UploadImage(ctx, file, size, objectKey, contentType)
	if err != nil {
		return nil, err
	}

	evidence := &ReturnItemEvidence{
		ID:          evidenceID,
		CustomerID:  customerID,
		StorageKey:  stored.ObjectKey,
		ContentType: contentType,
		SortOrder:   0,
	}

	if err := s.repo.CreateEvidence(ctx, evidence); err != nil {
		return nil, err
	}

	return &UploadEvidenceResponse{
		ID:  evidence.ID,
		URL: s.storageProvider.BuildPublicURL(stored.ObjectKey),
	}, nil
}

func (s *Service) DeleteStagedEvidence(ctx context.Context, customerID, evidenceID uuid.UUID) error {
	ev, err := s.repo.GetEvidenceByID(ctx, evidenceID)
	if err != nil {
		return err
	}
	if ev.CustomerID != customerID {
		return ErrEvidenceNotFound
	}
	if ev.ReturnItemID != nil {
		return ErrEvidenceAlreadyBound
	}

	if s.storageProvider != nil && ev.StorageKey != "" {
		if err := s.storageProvider.DeleteObject(ctx, ev.StorageKey); err != nil {
			return fmt.Errorf("failed to delete storage object: %w", err)
		}
	}

	if err := s.repo.DeleteEvidence(ctx, evidenceID); err != nil {
		return err
	}

	return nil
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

		if req.Comment == nil || strings.TrimSpace(*req.Comment) == "" {
			return ErrCommentRequired
		}
		trimmedComment := strings.TrimSpace(*req.Comment)

		// Lock order items
		query := `SELECT id, order_id, order_fulfillment_id, quantity, seller_id, title, image_url, variant_size, variant_color, sku, price_cents FROM order_items WHERE id = ANY($1) AND order_id = $2 ORDER BY id FOR UPDATE`
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
			Title         string
			ImageURL      *string
			VariantSize   *string
			VariantColor  *string
			SKU           *string
			PriceCents    int64
		}

		var lockedItems []lockedItem
		for rows.Next() {
			var li lockedItem
			var oid uuid.UUID
			if err := rows.Scan(&li.ID, &oid, &li.FulfillmentID, &li.Quantity, &li.SellerID, &li.Title, &li.ImageURL, &li.VariantSize, &li.VariantColor, &li.SKU, &li.PriceCents); err != nil {
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
				Comment:       &trimmedComment,
			}

			var retItems []ReturnItem

			for _, li := range itemsGroup {
				itemReq := reqItemMap[li.ID]

				// Taxonomy validation
				reason := req.Reason
				if itemReq.Reason != nil && *itemReq.Reason != "" {
					reason = *itemReq.Reason
				}

				isCanonicalRequired := map[string]bool{
					"defective":        true,
					"damaged":          true,
					"wrong_item":       true,
					"not_as_described": true,
					"incomplete":       true,
				}
				isCanonicalOptional := map[string]bool{
					"size_fit":     true,
					"changed_mind": true,
					"other":        true,
				}

				evCount := len(itemReq.EvidenceIDs)

				if isCanonicalRequired[reason] {
					if evCount < 2 {
						return ErrEvidenceRequired
					}
					if evCount > 6 {
						return ErrEvidenceTooMany
					}
				} else if isCanonicalOptional[reason] {
					if evCount > 6 {
						return ErrEvidenceTooMany
					}
				}

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

				itemReason := itemReq.Reason
				if itemReason == nil || *itemReason == "" {
					itemReason = &req.Reason
				}
				itemCond := itemReq.Condition
				if itemCond == nil || *itemCond == "" {
					defaultCond := "new"
					itemCond = &defaultCond
				}

				retItemID := uuid.New()
				retItems = append(retItems, ReturnItem{
					ID:               retItemID,
					ReturnID:         ret.ID,
					OrderItemID:      li.ID,
					Quantity:         itemReq.Quantity,
					Reason:           itemReason,
					Condition:        itemCond,
					Restock:          false,
					AcceptedQuantity: 0,
					DamagedQuantity:  0,
					RejectedQuantity: 0,
				})
			}

			if err := s.repo.CreateReturnTx(ctx, tx, ret, retItems); err != nil {
				return err
			}

			// Bind evidences
			for i, li := range itemsGroup {
				itemReq := reqItemMap[li.ID]
				if len(itemReq.EvidenceIDs) > 0 {
					if err := s.repo.BindEvidenceTx(ctx, tx, userID, itemReq.EvidenceIDs, retItems[i].ID); err != nil {
						return err
					}
				}
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

			var customerItems []CustomerReturnItemDetail
			for i, li := range itemsGroup {
				customerItems = append(customerItems, CustomerReturnItemDetail{
					ID:                 retItems[i].ID,
					ReturnID:           ret.ID,
					OrderItemID:        li.ID,
					ProductTitle:       li.Title,
					ProductImageURL:    li.ImageURL,
					VariantSize:        li.VariantSize,
					VariantColor:       li.VariantColor,
					SKU:                li.SKU,
					Quantity:           retItems[i].Quantity,
					PriceCents:         li.PriceCents,
					SubtotalPriceCents: li.PriceCents * int64(retItems[i].Quantity),
					Reason:             retItems[i].Reason,
					Condition:          retItems[i].Condition,
				})
			}

			responses = append(responses, ReturnResponse{
				Return:      *ret,
				OrderNumber: order.OrderNumber,
				Items:       customerItems,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return responses, nil
}

func (s *Service) buildEvidenceURL(key string) string {
	if s.storageProvider != nil {
		return s.storageProvider.BuildPublicURL(key)
	}
	return "/media/" + key
}

func (s *Service) GetCustomerReturn(ctx context.Context, userID, returnID uuid.UUID) (*ReturnResponse, error) {
	return s.repo.GetCustomerReturn(ctx, userID, returnID, s.buildEvidenceURL)
}

func (s *Service) ListCustomerReturns(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ReturnResponse, int, error) {
	return s.repo.ListReturnsByCustomer(ctx, userID, limit, offset, s.buildEvidenceURL)
}

func (s *Service) GetAdminReturn(ctx context.Context, returnID uuid.UUID) (*AdminReturnResponse, error) {
	return s.repo.GetAdminReturn(ctx, returnID, s.buildEvidenceURL)
}

func (s *Service) ListAdminReturns(ctx context.Context, limit, offset int) ([]AdminReturnResponse, int, error) {
	return s.repo.ListAdminReturns(ctx, limit, offset, s.buildEvidenceURL)
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
		if req.Status == "rejected" {
			if req.AdminComment == nil || strings.TrimSpace(*req.AdminComment) == "" {
				return ErrRejectReasonRequired
			}
			trimmed := strings.TrimSpace(*req.AdminComment)
			ret.AdminComment = &trimmed
		} else if req.AdminComment != nil {
			trimmed := strings.TrimSpace(*req.AdminComment)
			ret.AdminComment = &trimmed
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

		shipment, err := s.repo.GetReturnShipmentByReturnIDTx(ctx, tx, returnID)
		if err != nil {
			return err
		}
		if shipment == nil || shipment.Status != "arrived_at_zamk" {
			return ErrReturnNotArrived
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

func (s *Service) InspectSerializedUnit(ctx context.Context, returnID uuid.UUID, unitID uuid.UUID, req UpdateSerializedUnitInspectionRequest) error {
	if req.Disposition != "restock" && req.Disposition != "damaged" && req.Disposition != "reject" {
		return ErrInvalidDisposition
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		if ret.Status != "receiving" {
			return ErrReturnNotInReceiving
		}

		_, boundReturnID, err := s.repo.GetReturnItemUnitWithReturnIDTx(ctx, tx, unitID)
		if err != nil {
			return err
		}

		if boundReturnID != returnID {
			return ErrUnitNotInReturn
		}

		return s.repo.UpdateSerializedUnitInspectionTx(ctx, tx, unitID, req.InspectedCondition, req.Disposition)
	})
}

func (s *Service) InspectLegacyItem(ctx context.Context, returnID uuid.UUID, itemID uuid.UUID, req UpdateLegacyItemInspectionRequest) error {
	if req.AcceptedQuantity < 0 || req.DamagedQuantity < 0 || req.RejectedQuantity < 0 {
		return ErrInvalidInspectionQuantity
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		if ret.Status != "receiving" {
			return ErrReturnNotInReceiving
		}

		item, err := s.repo.GetReturnItemByIDTx(ctx, tx, itemID)
		if err != nil {
			return err
		}

		if item.ReturnID != returnID {
			return ErrUnitNotInReturn
		}

		allocs, err := s.repo.GetAllocationsForOrderItemTx(ctx, tx, item.OrderItemID)
		if err != nil {
			return err
		}
		if len(allocs) > 0 {
			return ErrItemNotLegacy
		}

		if req.AcceptedQuantity+req.DamagedQuantity+req.RejectedQuantity > item.Quantity {
			return ErrInvalidInspectionQuantity
		}

		return s.repo.UpdateLegacyItemInspectionTx(ctx, tx, itemID, req.AcceptedQuantity, req.DamagedQuantity, req.RejectedQuantity)
	})
}

func (s *Service) FinalizeReceiving(ctx context.Context, returnID uuid.UUID) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		if ret.Status == "item_received" {
			return nil // idempotent no-op
		}

		if ret.Status != "receiving" {
			return ErrReturnNotInReceiving
		}

		items, err := s.repo.GetReturnItemsForUpdateTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		restockCounts := make(map[uuid.UUID]int)

		for _, item := range items {
			allocs, err := s.repo.GetAllocationsForOrderItemTx(ctx, tx, item.OrderItemID)
			if err != nil {
				return err
			}

			if len(allocs) > 0 {
				// Serialized item
				scannedUnits, err := s.repo.GetScannedUnitsForReturnItemTx(ctx, tx, item.ID)
				if err != nil {
					return err
				}

				for _, u := range scannedUnits {
					if u.Disposition == nil || (*u.Disposition != "restock" && *u.Disposition != "damaged" && *u.Disposition != "reject") {
						return ErrFinalizeMissingDisposition
					}

					var iuID uuid.UUID
					var iuStatus string
					var pvID uuid.UUID
					var pickedAt *time.Time
					var releasedAt *time.Time
					var orderID uuid.UUID
					var fulfillmentID *uuid.UUID

					queryUnit := `
						SELECT iu.id, iu.status, iu.product_variant_id, oia.picked_at, oia.released_at, oi.order_id, oi.order_fulfillment_id
						FROM inventory_units iu
						JOIN order_item_allocations oia ON oia.inventory_unit_id = iu.id
						JOIN order_items oi ON oi.id = oia.order_item_id
						WHERE oia.id = $1
						FOR UPDATE
					`
					err = tx.QueryRow(ctx, queryUnit, u.OrderItemAllocationID).Scan(&iuID, &iuStatus, &pvID, &pickedAt, &releasedAt, &orderID, &fulfillmentID)
					if err != nil {
						return ErrInvalidUnitState
					}

					if iuStatus != "shipped" {
						return ErrInvalidUnitState
					}
					if pickedAt == nil || releasedAt != nil {
						return ErrInvalidUnitState
					}
					if orderID != ret.OrderID || fulfillmentID == nil || *fulfillmentID != ret.FulfillmentID {
						return ErrInvalidUnitState
					}

					switch *u.Disposition {
					case "restock":
						_, err = tx.Exec(ctx, "UPDATE inventory_units SET status = 'warehouse', updated_at = now() WHERE id = $1", iuID)
						if err != nil {
							return err
						}
						restockCounts[pvID]++
					case "damaged":
						_, err = tx.Exec(ctx, "UPDATE inventory_units SET status = 'damaged', updated_at = now() WHERE id = $1", iuID)
						if err != nil {
							return err
						}
					case "reject":
						// Remains shipped
					}
				}
			} else {
				// Legacy item
				if item.AcceptedQuantity < 0 || item.DamagedQuantity < 0 || item.RejectedQuantity < 0 {
					return ErrInvalidInspectionQuantity
				}
				if item.AcceptedQuantity+item.DamagedQuantity+item.RejectedQuantity > item.Quantity {
					return ErrInvalidInspectionQuantity
				}

				if item.AcceptedQuantity > 0 {
					var pvID uuid.UUID
					err = tx.QueryRow(ctx, "SELECT product_variant_id FROM order_items WHERE id = $1", item.OrderItemID).Scan(&pvID)
					if err != nil {
						return err
					}
					restockCounts[pvID] += item.AcceptedQuantity
				}
			}
		}

		// Sort variant IDs deterministically to prevent deadlocks
		var variantIDs []uuid.UUID
		for vid := range restockCounts {
			variantIDs = append(variantIDs, vid)
		}
		sort.Slice(variantIDs, func(i, j int) bool {
			return variantIDs[i].String() < variantIDs[j].String()
		})

		for _, vid := range variantIDs {
			qty := restockCounts[vid]
			if qty <= 0 {
				continue
			}

			var invItemID uuid.UUID
			var prodID uuid.UUID
			var sellerID uuid.UUID
			var totalStock int

			queryInv := `
				SELECT id, product_id, seller_id, total_stock
				FROM inventory_items
				WHERE product_variant_id = $1
				FOR UPDATE
			`
			err = tx.QueryRow(ctx, queryInv, vid).Scan(&invItemID, &prodID, &sellerID, &totalStock)
			if err != nil {
				return err
			}

			_, err = tx.Exec(ctx, "UPDATE inventory_items SET total_stock = total_stock + $1, updated_at = now() WHERE id = $2", qty, invItemID)
			if err != nil {
				return err
			}

			movID := uuid.New()
			insertMov := `
				INSERT INTO stock_movements (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, reference_type, reference_id, created_at)
				VALUES ($1, $2, $3, $4, $5, 'return', $6, 'return_restock', 'return', $7, now())
			`
			_, err = tx.Exec(ctx, insertMov, movID, invItemID, prodID, vid, sellerID, qty, ret.ID)
			if err != nil {
				return err
			}
		}

		ret.Status = "item_received"
		return s.repo.UpdateReturnTx(ctx, tx, ret)
	})
}

// GetCustomerReturnShipment returns the active shipment for a return owned by the customer.
func (s *Service) GetCustomerReturnShipment(ctx context.Context, customerID, returnID uuid.UUID) (*ReturnShipment, error) {
	ret, _, err := s.repo.GetReturn(ctx, returnID)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return nil, ErrReturnNotFound
	}
	if ret.UserID != customerID {
		return nil, ErrUnauthorized
	}
	return s.repo.GetReturnShipmentByReturnID(ctx, returnID)
}

// CreateCustomerReturnShipment creates a shipment for an approved return.
func (s *Service) CreateCustomerReturnShipment(ctx context.Context, customerID, returnID uuid.UUID, req CreateReturnShipmentRequest) (*ReturnShipment, error) {
	var result *ReturnShipment
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, _, err := s.repo.GetReturn(ctx, returnID)
		if err != nil {
			return err
		}
		if ret == nil {
			return ErrReturnNotFound
		}
		if ret.UserID != customerID {
			return ErrUnauthorized
		}
		if ret.Status != "approved" {
			return ErrReturnNotApproved
		}
		existing, err := s.repo.GetReturnShipmentByReturnIDTx(ctx, tx, returnID)
		if err != nil {
			return err
		}
		if existing != nil {
			return ErrShipmentAlreadyExists
		}

		if s.logisticsProvider == nil {
			return ErrCDEKNotConfigured
		}

		var selectedOffice *Office
		if req.Method == "cdek_office" {
			if req.CDEKOfficeCode == nil || strings.TrimSpace(*req.CDEKOfficeCode) == "" {
				return ErrCDEKOfficeRequired
			}
			offices, err := s.logisticsProvider.ListOffices(ctx)
			if err != nil {
				return err
			}
			for _, o := range offices {
				if o.Code == *req.CDEKOfficeCode {
					match := o
					selectedOffice = &match
					break
				}
			}
			if selectedOffice == nil {
				return ErrInvalidCDEKOffice
			}
		} else if req.Method == "cdek_courier" {
			if req.CustomerName == nil || strings.TrimSpace(*req.CustomerName) == "" ||
				req.CustomerPhone == nil || strings.TrimSpace(*req.CustomerPhone) == "" ||
				req.PickupAddress == nil || strings.TrimSpace(req.PickupAddress.City) == "" ||
				strings.TrimSpace(req.PickupAddress.Street) == "" || strings.TrimSpace(req.PickupAddress.House) == "" {
				return ErrCourierInfoRequired
			}
		}

		provReq := ProviderShipmentRequest{
			ReturnID: returnID,
			Method:   req.Method,
		}
		if req.CDEKOfficeCode != nil {
			provReq.CDEKOfficeCode = *req.CDEKOfficeCode
		}
		if req.CustomerName != nil {
			provReq.CustomerName = *req.CustomerName
		}
		if req.CustomerPhone != nil {
			provReq.CustomerPhone = *req.CustomerPhone
		}
		if req.PickupAddress != nil {
			provReq.PickupAddress = req.PickupAddress
		}

		provResult, err := s.logisticsProvider.CreateShipment(ctx, provReq)
		if err != nil {
			return err
		}

		shipment := &ReturnShipment{
			ID:       uuid.New(),
			ReturnID: returnID,
			Provider: "cdek",
			Method:   req.Method,
			Status:   provResult.Status,
		}
		if provResult.ProviderShipmentID != "" {
			shipment.ProviderShipmentID = &provResult.ProviderShipmentID
		}
		if provResult.TrackingNumber != "" {
			shipment.TrackingNumber = &provResult.TrackingNumber
		}
		if req.CDEKOfficeCode != nil {
			shipment.SelectedCDEKOfficeCode = req.CDEKOfficeCode
		}
		if selectedOffice != nil {
			shipment.CDEKOfficeAddress = &selectedOffice.Address
		}
		if req.CustomerName != nil {
			shipment.CustomerName = req.CustomerName
		}
		if req.CustomerPhone != nil {
			shipment.CustomerPhone = req.CustomerPhone
		}
		if req.PickupAddress != nil {
			addrBytes, _ := json.Marshal(req.PickupAddress)
			shipment.PickupAddress = addrBytes
		}

		if err := s.repo.CreateReturnShipmentTx(ctx, tx, shipment); err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

// UpdateReturnShipmentStatus transitions a shipment to a new status obeying the canonical state machine.
func (s *Service) UpdateReturnShipmentStatus(ctx context.Context, returnID uuid.UUID, newStatus string) (*ReturnShipment, error) {
	var result *ReturnShipment
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		shipment, err := s.repo.GetReturnShipmentByReturnIDTx(ctx, tx, returnID)
		if err != nil {
			return err
		}
		if shipment == nil {
			return ErrReturnNotFound
		}
		if !IsValidShipmentTransition(shipment.Status, newStatus) {
			return ErrInvalidShipmentTransition
		}
		shipment.Status = newStatus
		if err := s.repo.UpdateReturnShipmentTx(ctx, tx, shipment); err != nil {
			return err
		}
		result = shipment
		return nil
	})
	return result, err
}

// ListCDEKOffices proxies office lookup to the logistics provider.
func (s *Service) ListCDEKOffices(ctx context.Context) ([]Office, error) {
	if s.logisticsProvider == nil {
		return nil, ErrCDEKNotConfigured
	}
	return s.logisticsProvider.ListOffices(ctx)
}
