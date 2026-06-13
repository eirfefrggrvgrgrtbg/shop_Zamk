package orders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Service struct {
	repo         *Repository
	cartRepo     *cart.Repository
	inventorySvc *inventory.Service
	db           *postgres.Client
}

func NewService(repo *Repository, cartRepo *cart.Repository, inventorySvc *inventory.Service, db *postgres.Client) *Service {
	return &Service{
		repo:         repo,
		cartRepo:     cartRepo,
		inventorySvc: inventorySvc,
		db:           db,
	}
}

func (s *Service) CreateOrder(ctx context.Context, userID uuid.UUID, req CreateOrderRequest) (*Order, error) {
	// 1. Load cart
	userCart, err := s.cartRepo.GetCartByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, cart.ErrCartNotFound) {
			return nil, ErrEmptyCart
		}
		return nil, err
	}
	if len(userCart.Items) == 0 {
		return nil, ErrEmptyCart
	}

	var createdOrder *Order

	// 2. Start transaction
	err = s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		totalPriceCents := int64(0)
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

			// Reserve stock
			// 1 hour TTL for the reservation just in case checkout abandons? 
			// Wait, order is awaiting_payment, so the reservation should live until payment or cancellation.
			// Let's set it to 24 hours.
			res, err := s.inventorySvc.CreateReservationTx(ctx, tx, userID, item.ProductVariantID, item.Quantity, 24*time.Hour)
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
			ID:              orderID,
			UserID:          userID,
			Status:          "awaiting_payment",
			TotalPriceCents: totalPriceCents,
			Currency:        "RUB",
			CustomerName:    req.CustomerName,
			CustomerPhone:   req.CustomerPhone,
			CustomerEmail:   req.CustomerEmail,
			DeliveryAddress: req.DeliveryAddress,
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
			commissionBps := 900 // Default base commission
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
				return err
			}
			sellerFulfillments[sellerID] = f
		}

		for i := range orderItems {
			f := sellerFulfillments[orderItems[i].SellerID]
			orderItems[i].OrderFulfillmentID = &f.ID
			if err := s.repo.CreateOrderItemTx(ctx, tx, &orderItems[i]); err != nil {
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
		return nil, err
	}

	return createdOrder, nil
}

func (s *Service) CancelCustomerOrder(ctx context.Context, userID, orderID uuid.UUID) error {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return ErrOrderNotFound // act as if it doesn't exist
	}
	if order.Status != "awaiting_payment" {
		return ErrOrderNotCancellable
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.SetOrderCancelledTx(ctx, tx, orderID); err != nil {
			return err
		}

		// Release reservations
		resIDs, err := s.repo.GetOrderReservations(ctx, orderID)
		if err != nil {
			return err
		}
		for _, rid := range resIDs {
			if err := s.inventorySvc.ReleaseReservationTx(ctx, tx, rid); err != nil {
				// We ignore ErrReservationNotActive since it might have expired
				if !errors.Is(err, inventory.ErrReservationNotActive) {
					return err
				}
			}
		}

		history := &OrderStatusHistory{
			ID:          uuid.New(),
			OrderID:     orderID,
			FromStatus:  &order.Status,
			ToStatus:    "cancelled",
			ActorUserID: &userID,
		}
		if err := s.repo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
			return err
		}

		// Cascade cancellation to fulfillments
		if err := s.repo.MarkOrderFulfillmentsStatusTx(ctx, tx, orderID, order.Status, "cancelled"); err != nil {
			return err
		}
		
		return nil
	})
}

func (s *Service) UpdateOrderStatus(ctx context.Context, adminID, orderID uuid.UUID, req UpdateOrderStatusRequest) error {
	if req.Status == "paid" {
		return ErrManualPaidNotAllowed
	}

	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}

	if order.Status == "cancelled" {
		return errors.New("cannot change status of cancelled order")
	}
	if order.Status == "delivered" && req.Status != "returned" && req.Status != "refunded" {
		return errors.New("cannot change status of delivered order except to return/refund")
	}
	if order.Status == "awaiting_payment" && (req.Status == "shipped" || req.Status == "delivered" || req.Status == "packed") {
		return errors.New("unpaid order cannot skip to packed/shipped/delivered")
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.UpdateOrderStatusTx(ctx, tx, orderID, req.Status); err != nil {
			return err
		}

		history := &OrderStatusHistory{
			ID:          uuid.New(),
			OrderID:     orderID,
			FromStatus:  &order.Status,
			ToStatus:    req.Status,
			ActorUserID: &adminID,
			Comment:     req.Comment,
		}
		if err := s.repo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
			return err
		}

		if req.Status == "cancelled" {
			if err := s.repo.MarkOrderFulfillmentsStatusTx(ctx, tx, orderID, order.Status, "cancelled"); err != nil {
				return err
			}
		}

		return nil
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

func (s *Service) ListCustomerOrders(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Order, error) {
	return s.repo.ListCustomerOrders(ctx, userID, limit, offset)
}

func (s *Service) GetAdminOrder(ctx context.Context, orderID uuid.UUID) (*Order, error) {
	return s.repo.GetOrder(ctx, orderID)
}

func (s *Service) ListAdminOrders(ctx context.Context, limit, offset int) ([]Order, error) {
	return s.repo.ListAdminOrders(ctx, limit, offset)
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

type ExpireAwaitingPaymentOrdersResult struct {
	Checked              int `json:"checked"`
	Expired              int `json:"expired"`
	ReleasedReservations int `json:"releasedReservations"`
}

func (s *Service) ExpireAwaitingPaymentOrders(ctx context.Context, now time.Time, limit int) (*ExpireAwaitingPaymentOrdersResult, error) {
	result := &ExpireAwaitingPaymentOrdersResult{}

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

			// Release reservations
			resIDs, err := s.repo.GetOrderReservations(ctx, orderID)
			if err != nil {
				return err
			}
			for _, rid := range resIDs {
				if err := s.inventorySvc.ReleaseReservationTx(ctx, tx, rid); err != nil {
					if !errors.Is(err, inventory.ErrReservationNotActive) {
						return err
					}
				} else {
					result.ReleasedReservations++
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
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
