package payouts

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	dbPool     *pgxpool.Pool
	returns    returnsRepo
	orders     ordersRepo
	cfg        *config.Config
	notifs     *notifications.Service
}

func NewService(repo *Repository, db *postgres.Client, returns returnsRepo, orders ordersRepo, cfg *config.Config, notifs *notifications.Service) *Service {
	return &Service{
		repo:    repo,
		db:      db,
		dbPool:  db.Pool, // Extract the pgxpool.Pool from the postgres.Client for backward compatibility
		returns: returns,
		orders:  orders,
		cfg:     cfg,
		notifs:  notifs,
	}
}

// --- Commissions ---
func (s *Service) GetActiveCommissionRateBPS(ctx context.Context, sellerID uuid.UUID) (int, error) {
	rule, err := s.repo.GetActiveCommissionRule(ctx, sellerID)
	if err != nil {
		return 0, err
	}
	if rule == nil {
		return 900, nil // 9.0% default
	}
	return rule.RateBPS, nil
}

func (s *Service) SetCommissionRate(ctx context.Context, sellerID uuid.UUID, req AdminSellerCommissionRequest, adminUserID uuid.UUID) error {
	rule := &SellerCommissionRule{
		ID:        uuid.New(),
		SellerID:  sellerID,
		RateBPS:   req.RateBPS,
		Reason:    req.Reason,
		CreatedBy: adminUserID,
		CreatedAt: time.Now(),
	}
	return s.repo.CreateCommissionRule(ctx, rule)
}

func (s *Service) ListCommissionHistory(ctx context.Context, sellerID uuid.UUID) ([]SellerCommissionRule, error) {
	return s.repo.ListCommissionRules(ctx, sellerID)
}

// --- Orders & Finances ---
func (s *Service) CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error {
	// Called when order is fulfilled. We snapshot the order items into ledger.
	order, err := s.orders.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}
	
	items, err := s.orders.GetOrderItems(ctx, orderID)
	if err != nil {
		return err
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, item := range items {
		// Use active rule for seller
		rateBPS, err := s.GetActiveCommissionRateBPS(ctx, item.SellerID)
		if err != nil {
			return err
		}

		grossCents := int64(item.PriceCents) * int64(item.Quantity)
		commCents := (grossCents * int64(rateBPS)) / 10000
		netCents := grossCents - commCents

		// 1. Gross Sale
		err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
			ID:          uuid.New(),
			SellerID:    item.SellerID,
			OrderID:     &order.ID,
			OrderItemID: &item.ID,
			Type:        "sale_gross",
			AmountCents: grossCents,
			Currency:    "RUB",
			CreatedAt:   time.Now(),
		})
		if err != nil { return err }

		// 2. Commission
		err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
			ID:          uuid.New(),
			SellerID:    item.SellerID,
			OrderID:     &order.ID,
			OrderItemID: &item.ID,
			Type:        "zamk_commission",
			AmountCents: -commCents,
			Currency:    "RUB",
			CreatedAt:   time.Now(),
		})
		if err != nil { return err }

		// 3. Seller Net Earning (Frozen initially - AvailableAt is null)
		err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
			ID:          uuid.New(),
			SellerID:    item.SellerID,
			OrderID:     &order.ID,
			OrderItemID: &item.ID,
			Type:        "seller_earning",
			AmountCents: netCents,
			Currency:    "RUB",
			CreatedAt:   time.Now(),
		})
		if err != nil { return err }
	}

	return tx.Commit(ctx)
}

func (s *Service) ProcessReturnDeduction(ctx context.Context, tx pgx.Tx, returnID uuid.UUID, orderID uuid.UUID, items []ReturnItemDeduction) error {
	orderItems, err := s.orders.GetOrderItems(ctx, orderID)
	if err != nil {
		return err
	}
	orderItemMap := make(map[uuid.UUID]orders.OrderItem)
	for _, oi := range orderItems {
		orderItemMap[oi.ID] = oi
	}

	for _, item := range items {
		log.Printf("ProcessReturnDeduction: processing item: %+v", item)
		oi, ok := orderItemMap[item.OrderItemID]
		if !ok {
			log.Printf("ProcessReturnDeduction: order item not found in orderItemMap for id %s", item.OrderItemID)
			continue
		}

		earningEntry, err := s.repo.GetSellerEarningEntryTx(ctx, tx, item.OrderItemID)
		if err != nil {
			log.Printf("ProcessReturnDeduction: error getting earning entry: %v", err)
			return err
		}
		if earningEntry == nil {
			log.Printf("ProcessReturnDeduction: earningEntry is nil for order item %s", item.OrderItemID)
			continue // Should not happen for fulfilled orders
		}
		log.Printf("ProcessReturnDeduction: found earningEntry: %+v", earningEntry)

		// Calculate proportional deduction
		// Wait, earningEntry.AmountCents is the net earning for the entire quantity in oi.
		// Net deduction per unit = earningEntry.AmountCents / int64(oi.Quantity)
		if oi.Quantity <= 0 {
			continue
		}
		deductionCents := (earningEntry.AmountCents / int64(oi.Quantity)) * int64(item.Quantity)

		if earningEntry.PayoutBatchID != nil {
			// POST-PAYOUT RETURN RECOVERY: DEFERRED logic.
			// The funds were already paid. We insert an adjustment to offset future payouts.
			err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
				ID:          uuid.New(),
				SellerID:    earningEntry.SellerID,
				OrderID:     &orderID,
				OrderItemID: &item.OrderItemID,
				Type:        "adjustment",
				AmountCents: -deductionCents,
				Currency:    "RUB",
				Metadata:    []byte(`{"reason":"return_post_payout","return_id":"` + returnID.String() + `"}`),
				CreatedAt:   time.Now(),
			})
			if err != nil {
				return err
			}
		} else {
			// Frozen or unfrozen but not yet paid.
			// Insert a negative seller_earning with the EXACT SAME available_at to offset it mathematically.
			err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
				ID:          uuid.New(),
				SellerID:    earningEntry.SellerID,
				OrderID:     &orderID,
				OrderItemID: &item.OrderItemID,
				Type:        "adjustment",
				AmountCents: -deductionCents,
				Currency:    "RUB",
				AvailableAt: earningEntry.AvailableAt,
				Metadata:    []byte(`{"reason":"return_deduction","return_id":"` + returnID.String() + `"}`),
				CreatedAt:   time.Now(),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Called by a background job, or delivery webhook
func (s *Service) MarkOrderDeliveredTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, deliveredAt time.Time) error {
	availableAt := deliveredAt.AddDate(0, 0, 14) // 14-day hold
	return s.repo.UpdateAvailableAtByOrderIdTx(ctx, tx, orderID, availableAt)
}


// --- Seller View ---
func (s *Service) GetSellerBalance(ctx context.Context, sellerID uuid.UUID) (*BalanceResponse, error) {
	return s.repo.GetSellerBalanceSummary(ctx, sellerID)
}

func (s *Service) ListSellerLedger(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerLedgerEntry, int, error) {
	return s.repo.ListSellerLedger(ctx, sellerID, limit, offset)
}

func (s *Service) ListSellerPayoutBatches(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]PayoutBatch, int, error) {
	return s.repo.ListPayoutBatches(ctx, sellerID, limit, offset)
}

// MakeSellerFundsAvailable is called by the background worker to release frozen
// ledger entries whose available_at timestamp has passed. Returns the number of
// entries updated.
func (s *Service) MakeSellerFundsAvailable(ctx context.Context, now time.Time, limit int) (int, error) {
	return s.repo.UnfreezeAvailableEntries(ctx, now, limit)
}

// --- Payout Processing ---
func (s *Service) CreatePayoutBatchForSeller(ctx context.Context, sellerID uuid.UUID) (*PayoutBatch, error) {
	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	entries, err := s.repo.LockAvailableLedgerEntriesTx(ctx, tx, sellerID)
	if err != nil {
		return nil, err
	}

	var total int64
	var ids []uuid.UUID
	for _, e := range entries {
		total += e.AmountCents
		ids = append(ids, e.ID)
	}

	if total <= 0 {
		return nil, errors.New("no positive available balance")
	}

	batch := &PayoutBatch{
		ID:           uuid.New(),
		SellerID:     sellerID,
		AmountCents:  total,
		Status:       "scheduled",
		ScheduledFor: time.Now().AddDate(0, 0, 7), // next week
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreatePayoutBatchTx(ctx, tx, batch); err != nil {
		return nil, err
	}

	if err := s.repo.LinkLedgerEntriesToPayoutTx(ctx, tx, batch.ID, ids); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return batch, nil
}

func (s *Service) ProcessPayoutBatch(ctx context.Context, batchID uuid.UUID) error {
	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch, err := s.repo.GetPayoutBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return errors.New("not found")
	}
	if batch.Status == "paid" {
		return errors.New("already paid")
	}
	if batch.Status == "held" || batch.Status == "failed" {
		return errors.New("cannot process batch with status: " + batch.Status)
	}
	now := time.Now()
	batch.Status = "paid"
	batch.ProcessedAt = &now

	if err := s.repo.UpdatePayoutBatchTx(ctx, tx, batch); err != nil {
		return err
	}

	// Add payout deduction entry
	err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
		ID:            uuid.New(),
		SellerID:      batch.SellerID,
		PayoutBatchID: &batch.ID,
		Type:          "payout",
		AmountCents:   -batch.AmountCents,
		Currency:      "RUB",
		CreatedAt:     time.Now(),
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) HoldPayoutBatch(ctx context.Context, batchID uuid.UUID) error {
	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch, err := s.repo.GetPayoutBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return errors.New("not found")
	}
	if batch.Status == "paid" {
		return errors.New("already paid")
	}

	batch.Status = "held"
	if err := s.repo.UpdatePayoutBatchTx(ctx, tx, batch); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) FailPayoutBatch(ctx context.Context, batchID uuid.UUID) error {
	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch, err := s.repo.GetPayoutBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return errors.New("not found")
	}
	if batch.Status == "paid" {
		return errors.New("already paid")
	}

	batch.Status = "failed"
	if err := s.repo.UpdatePayoutBatchTx(ctx, tx, batch); err != nil {
		return err
	}

	// Unlink/re-release eligible ledger funds
	_, err = tx.Exec(ctx, "UPDATE seller_ledger_entries SET payout_batch_id = NULL WHERE payout_batch_id = $1", batch.ID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}


// --- Admin Endpoints ---
func (s *Service) GetAdminPayoutSummary(ctx context.Context) (*AdminPayoutSummary, error) {
	return s.repo.GetAdminPayoutSummary(ctx)
}

func (s *Service) ListAdminSellerBalances(ctx context.Context, limit, offset int) ([]AdminSellerBalance, int, error) {
	return []AdminSellerBalance{}, 0, nil
}

func (s *Service) ListAdminPayoutsFiltered(ctx context.Context, filter PayoutFilter, limit, offset int) ([]PayoutBatch, int, error) {
	return []PayoutBatch{}, 0, nil
}
