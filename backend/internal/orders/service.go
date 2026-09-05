package orders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Service struct {
	repo         *Repository
	cartRepo     *cart.Repository
	inventorySvc *inventory.Service
	db           *postgres.Client
	cfg          *config.Config
	logger       *slog.Logger
}

func NewService(repo *Repository, cartRepo *cart.Repository, inventorySvc *inventory.Service, db *postgres.Client, cfg *config.Config) *Service {
	return &Service{
		repo:         repo,
		cartRepo:     cartRepo,
		inventorySvc: inventorySvc,
		db:           db,
		cfg:          cfg,
		logger:       slog.Default(),
	}
}

func (s *Service) SetLogger(l *slog.Logger) *Service {
	if l == nil {
		l = slog.Default()
	}
	s.logger = l
	return s
}

func (s *Service) CreateOrder(ctx context.Context, userID uuid.UUID, req CreateOrderRequest, idempotencyKey *uuid.UUID) (*Order, error) {
	// 1. Load cart
	userCart, err := s.cartRepo.GetCartByUserID(ctx, userID)
	if err != nil && !errors.Is(err, cart.ErrCartNotFound) {
		return nil, err
	}

	var cartItems []cart.CartItem
	if userCart != nil {
		cartItems = userCart.Items
	}

	if idempotencyKey != nil {
		existing, err := s.repo.GetOrderByIdempotencyKey(ctx, userID, *idempotencyKey)
		if err == nil {
			// Found existing order
			itemsForHash := cartItems
			if len(itemsForHash) == 0 {
				for _, oi := range existing.Items {
					itemsForHash = append(itemsForHash, cart.CartItem{
						ProductID:        oi.ProductID,
						ProductVariantID: oi.ProductVariantID,
						Quantity:         oi.Quantity,
					})
				}
			}
			requestHash := calculateRequestHash(userID, req, itemsForHash)
			if existing.CheckoutRequestHash != nil && *existing.CheckoutRequestHash == requestHash {
				return existing, nil // Idempotent success, return existing order
			}
			return nil, ErrIdempotencyKeyConflict // Same key, different payload
		} else if !errors.Is(err, ErrOrderNotFound) {
			return nil, err
		}
	}

	if len(cartItems) == 0 {
		return nil, ErrEmptyCart
	}

	// Calculate Request Hash for new order
	var requestHash string
	if idempotencyKey != nil {
		requestHash = calculateRequestHash(userID, req, cartItems)
	}

	dm, err := s.repo.GetDeliveryMethod(ctx, req.DeliveryMethodID)
	if err != nil {
		return nil, errors.New("invalid delivery method")
	}
	if !dm.IsActive {
		return nil, errors.New("delivery method is not active")
	}

	var createdOrder *Order

	// 2. Start transaction
	err = s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		totalPriceCents := dm.PriceCents
		var orderItems []OrderItem
		var reservations []OrderReservation

		orderID := uuid.New()

		// Prepare order items and reservations
		for _, item := range userCart.Items {
			// Validate product and variant
			info, err := s.cartRepo.GetProductValidationInfo(ctx, item.ProductID, item.ProductVariantID)
			if err != nil {
				return err
			}
			if info.Status != "published" {
				return ErrProductNotPublished
			}
			if !info.VariantActive {
				return ErrVariantNotFound
			}

			// Reserve stock with TTL bounded to payment timeout
			timeoutMinutes := 30
			if s.cfg != nil && s.cfg.Worker.OrderPaymentTimeoutMinutes > 0 {
				timeoutMinutes = s.cfg.Worker.OrderPaymentTimeoutMinutes
			}
			resTTL := time.Duration(timeoutMinutes) * time.Minute
			res, err := s.inventorySvc.CreateReservationTx(ctx, tx, userID, item.ProductVariantID, item.Quantity, resTTL)
			if err != nil {
				return err
			}

			reservations = append(reservations, OrderReservation{
				ID:            uuid.New(),
				OrderID:       orderID,
				ReservationID: res.ID,
			})

			// Fetch snapshot details
			snap, err := s.getSnapshot(ctx, tx, item.ProductID, item.ProductVariantID)
			if err != nil {
				return err
			}

			if snap.SellerID == uuid.Nil {
				return errors.New("cannot create order: cart item seller_id cannot be resolved")
			}

			subtotal := snap.PriceCents * int64(item.Quantity)
			totalPriceCents += subtotal

			orderItems = append(orderItems, OrderItem{
				ID:                 uuid.New(),
				OrderID:            orderID,
				ProductID:          item.ProductID,
				ProductVariantID:   item.ProductVariantID,
				SellerID:           snap.SellerID,
				Title:              snap.Title,
				ProductSlug:        snap.ProductSlug,
				VariantSize:        snap.VariantSize,
				VariantColor:       snap.VariantColor,
				Sku:                snap.Sku,
				ImageURL:           snap.ImageURL,
				PriceCents:         snap.PriceCents,
				Quantity:           item.Quantity,
				SubtotalPriceCents: subtotal,
			})
		}

		// Create order
		order := &Order{
			ID:                       orderID,
			UserID:                   userID,
			Status:                   "awaiting_payment",
			TotalPriceCents:          totalPriceCents,
			Currency:                 "RUB",
			CustomerName:             req.CustomerName,
			CustomerPhone:            req.CustomerPhone,
			CustomerEmail:            req.CustomerEmail,
			DeliveryAddress:          req.DeliveryAddress,
			DeliveryMethodID:         &dm.ID,
			DeliveryMethodCode:       &dm.Code,
			DeliveryMethodName:       &dm.Name,
			DeliveryPriceCents:       &dm.PriceCents,
			DeliveryEstimatedDaysMin: dm.EstimatedDaysMin,
			DeliveryEstimatedDaysMax: dm.EstimatedDaysMax,
			CheckoutIdempotencyKey:   idempotencyKey,
		}

		if idempotencyKey != nil {
			order.CheckoutRequestHash = &requestHash
		}

		if err := s.repo.CreateOrderTx(ctx, tx, order); err != nil {
			return err
		}

		// Group items by seller and create fulfillments
		sellerTotals := make(map[uuid.UUID]int64)
		for _, item := range orderItems {
			sellerTotals[item.SellerID] += item.SubtotalPriceCents
		}

		sellerFulfillments := make(map[uuid.UUID]*OrderFulfillment)
		for sellerID, subtotal := range sellerTotals {
			commissionBps := s.cfg.Worker.MarketplaceCommissionBPS // From MARKETPLACE_COMMISSION_BPS env var (default 900, .env sets 1500)
			commissionAmount := (subtotal * int64(commissionBps)) / 10000
			sellerAmount := subtotal - commissionAmount

			f := &OrderFulfillment{
				ID:                uuid.New(),
				OrderID:           orderID,
				SellerID:          sellerID,
				Status:            "awaiting_payment",
				SubtotalCents:     subtotal,
				CommissionBps:     commissionBps,
				SellerAmountCents: sellerAmount,
			}
			if err := s.repo.CreateOrderFulfillmentTx(ctx, tx, f); err != nil {
				return errors.New("cannot create order: fulfillment creation failed: " + err.Error())
			}
			sellerFulfillments[sellerID] = f
		}

		for i := range orderItems {
			f := sellerFulfillments[orderItems[i].SellerID]
			if f == nil || f.ID == uuid.Nil {
				return errors.New("cannot create order: missing fulfillment for order item")
			}
			orderItems[i].OrderFulfillmentID = &f.ID
			if err := s.repo.CreateOrderItemTx(ctx, tx, &orderItems[i]); err != nil {
				return err
			}

			// Synchronize physical ZMU allocations if eligible units are available
			resID := reservations[i].ReservationID
			if _, _, err := s.repo.TryAllocateUnitsForOrderItem(ctx, tx, orderItems[i].ID, orderItems[i].Quantity, &resID); err != nil {
				return err
			}
		}

		for i := range reservations {
			if err := s.repo.CreateOrderReservationTx(ctx, tx, &reservations[i]); err != nil {
				return err
			}
		}

		history := &OrderStatusHistory{
			ID:          uuid.New(),
			OrderID:     orderID,
			FromStatus:  nil,
			ToStatus:    "awaiting_payment",
			ActorUserID: &userID,
			Comment:     nil,
		}
		if err := s.repo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
			return err
		}

		// Clear cart
		if err := s.cartRepo.ClearCartTx(ctx, tx, userCart.ID); err != nil {
			return err
		}

		order.Items = orderItems
		createdOrder = order
		return nil
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "orders_customer_id_idempotency_key_idx" {
			// Idempotency key conflict due to concurrency
			if idempotencyKey != nil {
				existing, getErr := s.repo.GetOrderByIdempotencyKey(ctx, userID, *idempotencyKey)
				if getErr == nil {
					if existing.CheckoutRequestHash != nil && *existing.CheckoutRequestHash == requestHash {
						return existing, nil
					}
					return nil, ErrIdempotencyKeyConflict
				}
			}
			return nil, ErrIdempotencyKeyConflict
		}
		return nil, err
	}

	return createdOrder, nil
}

type cancelOrderParams struct {
	ActorID          *uuid.UUID
	ActorRole        string // "customer", "admin"
	StableReasonCode string // strictly "customer_cancelled", "admin_cancelled"
	AuditReason      *string
	AuditComment     *string
}

func (s *Service) executeOrderCancellation(ctx context.Context, orderID uuid.UUID, params cancelOrderParams) error {
	var (
		resReleasedCount   int
		allocReleasedCount int
		cancelledOrder     *Order
	)

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		order, err := s.repo.GetOrderForUpdateTx(ctx, tx, orderID)
		if err != nil {
			return err
		}

		if params.ActorRole == "customer" {
			if params.ActorID == nil || order.UserID != *params.ActorID {
				return ErrOrderNotFound
			}
			if order.Status == "cancelled" {
				return ErrOrderAlreadyCancelled
			}
			if order.Status != "awaiting_payment" {
				return ErrOrderNotCancellable
			}
		} else if params.ActorRole == "admin" {
			if order.Status == "cancelled" {
				return ErrOrderAlreadyCancelled
			}
			if order.Status == "shipped" || order.Status == "delivered" || order.Status == "returned" || order.Status == "refunded" {
				return fmt.Errorf("%w: cannot cancel order in '%s' status", ErrOrderNotCancellable, order.Status)
			}
			if order.Status != "awaiting_payment" && order.Status != "paid" && order.Status != "assembling" && order.Status != "packed" {
				return fmt.Errorf("%w: cannot cancel order in '%s' status", ErrOrderNotCancellable, order.Status)
			}

			// Reject cancellation if an active shipment already exists (FBO shipment consistency rule A)
			hasActiveShipment, err := s.repo.HasActiveShipmentTx(ctx, tx, orderID)
			if err != nil {
				return err
			}
			if hasActiveShipment {
				return fmt.Errorf("%w: order has an active shipment and cannot be cancelled", ErrOrderNotCancellable)
			}
		} else {
			if order.Status == "cancelled" {
				return ErrOrderAlreadyCancelled
			}
		}

		// 1. Mark order cancelled in DB
		if err := s.repo.SetOrderCancelledTx(ctx, tx, orderID); err != nil {
			return err
		}

		// 2. Release physical allocations
		allocCount, err := s.repo.ReleaseAllocationsForOrderCountTx(ctx, tx, orderID, params.StableReasonCode)
		if err != nil {
			return err
		}
		allocReleasedCount = allocCount

		// 3. Release reservations
		resIDs, err := s.repo.GetActiveOrderReservations(ctx, orderID)
		if err != nil {
			return err
		}
		if s.inventorySvc != nil {
			for _, rid := range resIDs {
				if err := s.inventorySvc.ReleaseReservationTx(ctx, tx, rid); err != nil {
					if !errors.Is(err, inventory.ErrReservationNotActive) {
						return err
					}
				} else {
					resReleasedCount++
				}
			}
		}

		// 4. Record durable order status history
		var histComment *string
		if params.AuditComment != nil && *params.AuditComment != "" {
			if params.AuditReason != nil && *params.AuditReason != "" && *params.AuditReason != params.StableReasonCode {
				c := fmt.Sprintf("[%s] %s", *params.AuditReason, *params.AuditComment)
				histComment = &c
			} else {
				histComment = params.AuditComment
			}
		} else if params.AuditReason != nil && *params.AuditReason != "" && *params.AuditReason != params.StableReasonCode {
			histComment = params.AuditReason
		}

		history := &OrderStatusHistory{
			ID:          uuid.New(),
			OrderID:     orderID,
			FromStatus:  &order.Status,
			ToStatus:    "cancelled",
			ActorUserID: params.ActorID,
			Comment:     histComment,
		}
		if err := s.repo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
			return err
		}

		// 5. Cascade cancellation to active fulfillments
		if _, err := s.repo.CancelActiveOrderFulfillmentsTx(ctx, tx, orderID); err != nil {
			return err
		}

		cancelledOrder = order
		return nil
	})
	if err != nil {
		return err
	}

	actorIDStr := ""
	if params.ActorID != nil {
		actorIDStr = params.ActorID.String()
	}

	// Post-commit: emit inventory.order_hold_released if real holds released
	if resReleasedCount+allocReleasedCount > 0 {
		attrs := []slog.Attr{
			slog.String("order_id", orderID.String()),
		}
		if cancelledOrder != nil && cancelledOrder.OrderNumber != nil && *cancelledOrder.OrderNumber != "" {
			attrs = append(attrs, slog.String("order_number", *cancelledOrder.OrderNumber))
		}
		attrs = append(attrs,
			slog.Int("reservations_released_count", resReleasedCount),
			slog.Int("allocations_released_count", allocReleasedCount),
			slog.String("reason", params.StableReasonCode),
		)

		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName:  "inventory.order_hold_released",
			Domain:     "inventory",
			Action:     "release_order_hold",
			Result:     "success",
			ActorID:    actorIDStr,
			ActorRole:  params.ActorRole,
			Attributes: attrs,
		})
	}

	// Post-commit: emit canonical order.cancelled business event with stable reason_code
	cancelAttrs := []slog.Attr{
		slog.String("order_id", orderID.String()),
		slog.String("actor_role", params.ActorRole),
		slog.String("reason_code", params.StableReasonCode),
	}
	if cancelledOrder != nil && cancelledOrder.OrderNumber != nil && *cancelledOrder.OrderNumber != "" {
		cancelAttrs = append(cancelAttrs, slog.String("order_number", *cancelledOrder.OrderNumber))
	}
	observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
		EventName:  "order.cancelled",
		Domain:     "orders",
		Action:     "cancel_order",
		Result:     "success",
		ActorID:    actorIDStr,
		ActorRole:  params.ActorRole,
		Attributes: cancelAttrs,
	})

	return nil
}

func (s *Service) CancelCustomerOrder(ctx context.Context, userID, orderID uuid.UUID) error {
	return s.executeOrderCancellation(ctx, orderID, cancelOrderParams{
		ActorID:          &userID,
		ActorRole:        "customer",
		StableReasonCode: "customer_cancelled",
	})
}

func (s *Service) CancelAdminOrder(ctx context.Context, adminID, orderID uuid.UUID, req CancelAdminOrderRequest) error {
	return s.executeOrderCancellation(ctx, orderID, cancelOrderParams{
		ActorID:          &adminID,
		ActorRole:        "admin",
		StableReasonCode: "admin_cancelled",
		AuditReason:      req.Reason,
		AuditComment:     req.Comment,
	})
}

func (s *Service) GetCustomerOrder(ctx context.Context, userID, orderID uuid.UUID) (*Order, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

func calculateRequestHash(userID uuid.UUID, req CreateOrderRequest, cartItems []cart.CartItem) string {
	// Sort items for deterministic hashing
	type hashItem struct {
		VariantID uuid.UUID
		Quantity  int
	}
	items := make([]hashItem, len(cartItems))
	for i, item := range cartItems {
		items[i] = hashItem{VariantID: item.ProductVariantID, Quantity: item.Quantity}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].VariantID.String() < items[j].VariantID.String()
	})

	hashData := struct {
		UserID           string
		DeliveryMethodID string
		DeliveryAddress  string
		Items            []hashItem
	}{
		UserID:           userID.String(),
		DeliveryMethodID: req.DeliveryMethodID.String(),
		DeliveryAddress:  req.DeliveryAddress, // spaces normalization can be added, assuming exact match for now
		Items:            items,
	}

	b, _ := json.Marshal(hashData)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func (s *Service) ListCustomerOrders(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Order, error) {
	return s.repo.ListCustomerOrders(ctx, userID, limit, offset)
}

func (s *Service) GetAdminOrder(ctx context.Context, orderID uuid.UUID) (*AdminOrderDetail, error) {
	return s.repo.GetAdminOrderDetail(ctx, orderID)
}

func (s *Service) ListAdminOrders(ctx context.Context, q, status, paymentStatus, fulfillmentStatus, sourceType, sellerId string, limit, offset int) ([]AdminOrder, int, error) {
	return s.repo.ListAdminOrders(ctx, q, status, paymentStatus, fulfillmentStatus, sourceType, sellerId, limit, offset)
}

func (s *Service) ListSellerOrders(ctx context.Context, userID uuid.UUID, limit, offset int) ([]SellerOrder, error) {
	sellerID, err := s.repo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListSellerOrders(ctx, sellerID, limit, offset)
}

func (s *Service) GetSellerOrder(ctx context.Context, userID, orderID uuid.UUID) (*SellerOrder, error) {
	sellerID, err := s.repo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetSellerOrder(ctx, sellerID, orderID)
}

func (s *Service) GetSellerOrderSummary(ctx context.Context, userID uuid.UUID) (*SellerOrderSummary, error) {
	sellerID, err := s.repo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetSellerOrderSummary(ctx, sellerID)
}

type ExpireAwaitingPaymentOrdersResult struct {
	Checked              int `json:"checked"`
	Expired              int `json:"expired"`
	ReleasedReservations int `json:"releasedReservations"`
}

func (s *Service) ExpireAwaitingPaymentOrders(ctx context.Context, now time.Time, limit int) (*ExpireAwaitingPaymentOrdersResult, error) {
	result := &ExpireAwaitingPaymentOrdersResult{}

	type expiredOrderEventData struct {
		orderID       uuid.UUID
		orderNumber   string
		resReleased   int
		allocReleased int
	}
	var expiredEvents []expiredOrderEventData

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Find orders to expire with FOR UPDATE SKIP LOCKED
		orderIDs, err := s.repo.GetExpiredAwaitingPaymentOrdersTx(ctx, tx, now, limit)
		if err != nil {
			return err
		}

		result.Checked = len(orderIDs)

		for _, orderID := range orderIDs {
			// Lock order row explicitly again just in case, though GetExpiredAwaitingPaymentOrdersTx did lock it
			order, err := s.repo.GetOrderForUpdateTx(ctx, tx, orderID)
			if err != nil {
				return err
			}

			// Verify it's still awaiting_payment
			if order.Status != "awaiting_payment" {
				continue
			}

			// Cancel order
			if err := s.repo.SetOrderCancelledTx(ctx, tx, orderID); err != nil {
				return err
			}

			history := &OrderStatusHistory{
				ID:          uuid.New(),
				OrderID:     orderID,
				FromStatus:  &order.Status,
				ToStatus:    "cancelled",
				ActorUserID: nil,
				Comment:     func(s string) *string { return &s }("expired awaiting payment"),
			}
			if err := s.repo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
				return err
			}

			result.Expired++

			// Release physical allocations
			allocCount, err := s.repo.ReleaseAllocationsForOrderCountTx(ctx, tx, orderID, "order_expired")
			if err != nil {
				return err
			}

			// Release reservations
			resIDs, err := s.repo.GetActiveOrderReservations(ctx, orderID)
			if err != nil {
				return err
			}
			orderResReleased := 0
			for _, rid := range resIDs {
				if err := s.inventorySvc.ReleaseReservationTx(ctx, tx, rid); err != nil {
					if !errors.Is(err, inventory.ErrReservationNotActive) {
						return err
					}
				} else {
					orderResReleased++
					result.ReleasedReservations++
				}
			}

			orderNum := ""
			if order.OrderNumber != nil {
				orderNum = *order.OrderNumber
			}
			expiredEvents = append(expiredEvents, expiredOrderEventData{
				orderID:       orderID,
				orderNumber:   orderNum,
				resReleased:   orderResReleased,
				allocReleased: allocCount,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Post-commit: emit events for expired orders with real released holds
	for _, item := range expiredEvents {
		if item.resReleased+item.allocReleased > 0 {
			attrs := []slog.Attr{
				slog.String("order_id", item.orderID.String()),
			}
			if item.orderNumber != "" {
				attrs = append(attrs, slog.String("order_number", item.orderNumber))
			}
			attrs = append(attrs,
				slog.Int("reservations_released_count", item.resReleased),
				slog.Int("allocations_released_count", item.allocReleased),
				slog.String("reason", "order_expired"),
			)

			observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
				EventName:  "inventory.order_hold_released",
				Domain:     "inventory",
				Action:     "release_order_hold",
				Result:     "success",
				ActorRole:  "system",
				Attributes: attrs,
			})
		}
	}

	return result, nil
}

// Internal snapshot struct
type snapshotData struct {
	SellerID     uuid.UUID
	Title        string
	ProductSlug  string
	VariantSize  *string
	VariantColor *string
	Sku          *string
	ImageURL     *string
	PriceCents   int64
}

func (s *Service) getSnapshot(ctx context.Context, tx pgx.Tx, productID, variantID uuid.UUID) (*snapshotData, error) {
	query := `
		SELECT p.seller_id, p.title, p.slug, pv.size, pv.color, pv.sku, COALESCE(pv.price_cents, p.price_cents)
		FROM products p
		JOIN product_variants pv ON p.id = pv.product_id
		WHERE p.id = $1 AND pv.id = $2
	`
	var snap snapshotData
	err := tx.QueryRow(ctx, query, productID, variantID).Scan(
		&snap.SellerID, &snap.Title, &snap.ProductSlug, &snap.VariantSize, &snap.VariantColor, &snap.Sku, &snap.PriceCents,
	)
	if err != nil {
		return nil, err
	}

	imgQuery := `SELECT image_url FROM product_images WHERE product_id = $1 ORDER BY sort_order ASC LIMIT 1`
	var url string
	err = tx.QueryRow(ctx, imgQuery, productID).Scan(&url)
	if err == nil {
		snap.ImageURL = &url
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	return &snap, nil
}
