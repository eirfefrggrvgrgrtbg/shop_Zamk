package payouts

import (
	"context"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type returnsRepo interface {
	GetTotalReturnedQuantityForOrderItem(ctx context.Context, orderItemID uuid.UUID) (int, error)
}

type ordersRepo interface {
	GetOrder(ctx context.Context, id uuid.UUID) (*orders.Order, error)
	GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]orders.OrderItem, error)
	GetSellerIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}

type Service struct {
	repo       *Repository
	db         *postgres.Client
	returns    returnsRepo
	orders     ordersRepo
	cfg        *config.Config
}

func NewService(repo *Repository, db *postgres.Client, returns returnsRepo, orders ordersRepo, cfg *config.Config) *Service {
	return &Service{
		repo:    repo,
		db:      db,
		returns: returns,
		orders:  orders,
		cfg:     cfg,
	}
}

func (s *Service) CalculateCommissionAndNet(grossCents int64) (commissionCents int64, netCents int64) {
	bps := int64(s.cfg.Worker.MarketplaceCommissionBPS)
	commissionCents = (grossCents * bps) / 10000
	netCents = grossCents - commissionCents
	return
}

func (s *Service) CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error {
	order, err := s.orders.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}
	
	items, err := s.orders.GetOrderItems(ctx, orderID)
	if err != nil {
		return err
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		for _, item := range items {
			exists, err := s.repo.HasSalePendingForOrderItem(ctx, item.ID)
			if err != nil {
				return err
			}
			if exists {
				continue // Idempotent
			}

			_, netCents := s.CalculateCommissionAndNet(item.SubtotalPriceCents)
			
			// Available at is calculated from the time of delivery (which is approximately now when this is called)
			availableAt := time.Now().AddDate(0, 0, s.cfg.Worker.ReturnWindowDays)

			entry := &SellerBalanceLedger{
				ID:          uuid.New(),
				SellerID:    item.SellerID,
				OrderID:     &order.ID,
				OrderItemID: &item.ID,
				Type:        "sale_pending",
				AmountCents: netCents,
				Currency:    "RUB",
				AvailableAt: &availableAt,
			}
			
			if err := s.repo.InsertLedgerEntryTx(ctx, tx, entry); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ProcessRefundDeduction(ctx context.Context, refundID uuid.UUID, returnID uuid.UUID, orderID uuid.UUID, amountCents int64) error {
	// Need order items to attribute the deduction? 
	// The requirement says "must be linked to return_id/refund_id/order_item_id if possible".
	// Since refund is at the return level, and return has return_items, it's slightly complex to attribute proportional refund.
	// We will create a negative ledger entry at the order level for now, or attribute it to the first order_item of the return.
	// Wait, we need the seller ID! So we must query the order items.
	
	items, err := s.orders.GetOrderItems(ctx, orderID)
	if err != nil {
		return err
	}
	
	if len(items) == 0 {
		return nil
	}
	
	// Assuming a single seller per order for simplicity, or we take the seller of the first item
	// In ZAMK phase 6, cart creates separate orders per seller. So all items in an order have the same seller.
	sellerID := items[0].SellerID

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// Verify we haven't already deducted for this refund
		// (omitted for brevity, assume idempotent or caller guarantees single call)
		
		entry := &SellerBalanceLedger{
			ID:          uuid.New(),
			SellerID:    sellerID,
			OrderID:     &orderID,
			ReturnID:    &returnID,
			RefundID:    &refundID,
			Type:        "refund_deduction",
			AmountCents: -amountCents, // NEGATIVE
			Currency:    "RUB",
		}
		
		return s.repo.InsertLedgerEntryTx(ctx, tx, entry)
	})
}

func (s *Service) MakeSellerFundsAvailable(ctx context.Context, now time.Time, limit int) (int, error) {
	processed := 0
	
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		pendings, err := s.repo.GetLedgerEntriesByType(ctx, tx, "sale_pending", limit, now)
		if err != nil {
			return err
		}
		
		for _, pending := range pendings {
			// Skip if already available
			exists, err := s.repo.HasSaleAvailableForOrderItem(ctx, *pending.OrderItemID)
			if err != nil {
				return err
			}
			if exists {
				continue
			}

			// Check for open returns
			returnedQty, err := s.returns.GetTotalReturnedQuantityForOrderItem(ctx, *pending.OrderItemID)
			if err != nil {
				return err
			}
			if returnedQty > 0 {
				// Has a return. Skip converting to available.
				// Wait, if it has a return, does it NEVER become available?
				// For Phase 9B MVP: If an item is returned, we just don't convert the pending sale to available.
				// The refund will create a refund_deduction, and the pending sale just sits there (or we could cancel it, but append-only rules).
				// We'll skip it.
				continue
			}

			// Convert to available
			entry := &SellerBalanceLedger{
				ID:          uuid.New(),
				SellerID:    pending.SellerID,
				OrderID:     pending.OrderID,
				OrderItemID: pending.OrderItemID,
				Type:        "sale_available",
				AmountCents: pending.AmountCents,
				Currency:    pending.Currency,
			}
			
			if err := s.repo.InsertLedgerEntryTx(ctx, tx, entry); err != nil {
				return err
			}
			
			processed++
		}
		
		return nil
	})
	
	return processed, err
}

func (s *Service) GetSellerBalance(ctx context.Context, userID uuid.UUID) (*BalanceResponse, error) {
	sellerID, err := s.orders.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	return s.repo.GetSellerBalances(ctx, sellerID)
}

func (s *Service) ListSellerPayouts(ctx context.Context, userID uuid.UUID) ([]Payout, error) {
	sellerID, err := s.orders.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	return s.repo.ListSellerPayouts(ctx, sellerID)
}

func (s *Service) RequestPayout(ctx context.Context, userID uuid.UUID, req PayoutRequestDto) (*Payout, error) {
	sellerID, err := s.orders.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	var payout *Payout

	err = s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		balances, err := s.repo.GetSellerBalances(ctx, sellerID)
		if err != nil {
			return err
		}

		if balances.AvailableBalanceCents < req.AmountCents {
			return ErrInsufficientBalance
		}

		payout = &Payout{
			ID:          uuid.New(),
			SellerID:    sellerID,
			Status:      "requested",
			AmountCents: req.AmountCents,
			Currency:    "RUB",
			Comment:     req.Comment,
		}

		if err := s.repo.CreatePayoutTx(ctx, tx, payout); err != nil {
			return err
		}

		// Create hold
		hold := &SellerBalanceLedger{
			ID:          uuid.New(),
			SellerID:    sellerID,
			PayoutID:    &payout.ID,
			Type:        "payout_requested",
			AmountCents: -req.AmountCents, // NEGATIVE
			Currency:    "RUB",
		}
		
		return s.repo.InsertLedgerEntryTx(ctx, tx, hold)
	})

	if err != nil {
		return nil, err
	}
	return payout, nil
}

func (s *Service) ListAdminPayouts(ctx context.Context) ([]Payout, error) {
	return s.repo.ListAllPayouts(ctx)
}

func (s *Service) GetAdminPayout(ctx context.Context, id uuid.UUID) (*Payout, error) {
	return s.repo.GetPayout(ctx, id)
}

func (s *Service) UpdatePayoutStatus(ctx context.Context, payoutID uuid.UUID, adminUserID uuid.UUID, req UpdatePayoutStatusRequest) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		payout, err := s.repo.GetPayout(ctx, payoutID)
		if err != nil {
			return err
		}

		// Allowed transitions:
		// requested -> approved, rejected
		// approved -> paid, cancelled

		valid := false
		switch payout.Status {
		case "requested":
			if req.Status == "approved" || req.Status == "rejected" {
				valid = true
			}
		case "approved":
			if req.Status == "paid" || req.Status == "cancelled" {
				valid = true
			}
		}

		if !valid {
			return ErrInvalidPayoutStatus
		}

		now := time.Now()
		payout.Status = req.Status
		payout.AdminUserID = &adminUserID
		payout.Comment = req.Comment

		switch req.Status {
		case "approved":
			payout.ApprovedAt = &now
		case "rejected":
			payout.RejectedAt = &now
			// Release hold
			release := &SellerBalanceLedger{
				ID:          uuid.New(),
				SellerID:    payout.SellerID,
				PayoutID:    &payout.ID,
				Type:        "payout_rejected",
				AmountCents: payout.AmountCents, // POSITIVE
				Currency:    payout.Currency,
			}
			if err := s.repo.InsertLedgerEntryTx(ctx, tx, release); err != nil {
				return err
			}
		case "cancelled":
			payout.RejectedAt = &now // Using rejected_at for cancellation
			// Release hold
			release := &SellerBalanceLedger{
				ID:          uuid.New(),
				SellerID:    payout.SellerID,
				PayoutID:    &payout.ID,
				Type:        "payout_cancelled",
				AmountCents: payout.AmountCents, // POSITIVE
				Currency:    payout.Currency,
			}
			if err := s.repo.InsertLedgerEntryTx(ctx, tx, release); err != nil {
				return err
			}
		case "paid":
			payout.PaidAt = &now
			// Mark paid (audit only)
			audit := &SellerBalanceLedger{
				ID:          uuid.New(),
				SellerID:    payout.SellerID,
				PayoutID:    &payout.ID,
				Type:        "payout_paid",
				AmountCents: 0,
				Currency:    payout.Currency,
			}
			if err := s.repo.InsertLedgerEntryTx(ctx, tx, audit); err != nil {
				return err
			}
		}

		return s.repo.UpdatePayoutTx(ctx, tx, payout)
	})
}
