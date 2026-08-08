package payouts

import (
	"context"
	"errors"
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

func (s *Service) ProcessRefundDeduction(ctx context.Context, refundID uuid.UUID, returnID uuid.UUID, orderID uuid.UUID, amountCents int64) error {
	// For simplicity, just insert a negative adjustment to offset earnings.
	// Since order id is enough, we can attribute it back to the seller by querying the seller of this order.
	items, err := s.orders.GetOrderItems(ctx, orderID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	// just assign to first item's seller
	sellerID := items[0].SellerID

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    sellerID,
		OrderID:     &orderID,
		Type:        "adjustment",
		AmountCents: -amountCents,
		Currency:    "RUB",
		Metadata:    []byte(`{"reason":"refund","refund_id":"` + refundID.String() + `"}`),
		CreatedAt:   time.Now(),
	})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
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
