package payouts

import (
	"context"
	"fmt"
	"log"
	"sort"
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

// CreatePendingSalesForFulfillmentTx accrues seller ledger entries for one delivered fulfillment
// inside the delivery transaction, using the economics frozen at checkout.
func (s *Service) CreatePendingSalesForFulfillmentTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) error {
	return s.accrueOrderItemsTx(ctx, tx, "oi.order_fulfillment_id", fulfillmentID)
}

// CreatePendingSalesForOrder accrues seller ledger entries for every item of an order.
// Items that were already accrued are skipped.
func (s *Service) CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error {
	if _, err := s.orders.GetOrder(ctx, orderID); err != nil {
		return err
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.accrueOrderItemsTx(ctx, tx, "oi.order_id", orderID); err != nil {
		return err
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

	// Deterministic ordering by OrderItemID to prevent deadlocks under concurrent execution
	sortedItems := make([]ReturnItemDeduction, len(items))
	copy(sortedItems, items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].OrderItemID.String() < sortedItems[j].OrderItemID.String()
	})

	for _, item := range sortedItems {
		log.Printf("ProcessReturnDeduction: processing item: %+v", item)
		oi, ok := orderItemMap[item.OrderItemID]
		if !ok {
			log.Printf("ProcessReturnDeduction: order item not found in orderItemMap for id %s", item.OrderItemID)
			continue
		}

		if oi.Quantity <= 0 || item.Quantity <= 0 {
			continue
		}

		// Idempotency: skip if an authoritative return adjustment for this return_id already exists
		alreadyApplied, err := s.repo.HasReturnAdjustmentTx(ctx, tx, item.OrderItemID, returnID)
		if err != nil {
			return err
		}
		if alreadyApplied {
			log.Printf("ProcessReturnDeduction: adjustment for return %s on item %s already exists, skipping", returnID, item.OrderItemID)
			continue
		}

		// Acquire row-level lock on seller_earning entry to serialize concurrent return processing
		earningEntry, err := s.repo.GetSellerEarningEntryTx(ctx, tx, item.OrderItemID)
		if err != nil {
			log.Printf("ProcessReturnDeduction: error getting earning entry: %v", err)
			return err
		}
		if earningEntry == nil {
			log.Printf("ProcessReturnDeduction: earningEntry is nil for order item %s", item.OrderItemID)
			continue // Should not happen for fulfilled orders
		}
		if earningEntry.AmountCents <= 0 {
			continue
		}
		log.Printf("ProcessReturnDeduction: found earningEntry: %+v", earningEntry)

		// Authoritative prior deductions and returned quantity for this order_item_id
		priorDeductions, priorQty, err := s.repo.GetPriorReturnDeductionsTx(ctx, tx, item.OrderItemID, returnID)
		if err != nil {
			return err
		}

		// Cumulative target allocation:
		// Let E = earningEntry.AmountCents, Q = int64(oi.Quantity)
		// previousReturnedQty = priorQty
		// newReturnedQty = priorQty + item.Quantity
		// previousTarget = floor(E * previousReturnedQty / Q)
		// newTarget = floor(E * newReturnedQty / Q)
		// currentDeduction = newTarget - priorDeductions
		totalEarning := earningEntry.AmountCents
		totalOrderQty := int64(oi.Quantity)

		newReturnedQty := priorQty + int64(item.Quantity)
		if newReturnedQty > totalOrderQty {
			newReturnedQty = totalOrderQty
		}

		newTarget := (totalEarning * newReturnedQty) / totalOrderQty
		deductionCents := newTarget - priorDeductions

		// Safe boundary checks
		if deductionCents < 0 {
			deductionCents = 0
		}
		remainingEarning := totalEarning - priorDeductions
		if deductionCents > remainingEarning {
			deductionCents = remainingEarning
		}
		if deductionCents <= 0 {
			continue
		}

		reason := "return_deduction"
		if earningEntry.PayoutBatchID != nil {
			reason = "return_post_payout"
		}

		metaJSON := fmt.Sprintf(`{"reason":"%s","return_id":"%s","quantity":%d}`,
			reason,
			returnID.String(),
			item.Quantity,
		)

		if earningEntry.PayoutBatchID != nil {
			// POST-PAYOUT RETURN RECOVERY: DEFERRED logic.
			// The funds were already paid. We insert an adjustment to offset future payouts.
			// No hold applies to this post-payout adjustment (available_at = NULL means immediately available).
			err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
				ID:          uuid.New(),
				SellerID:    earningEntry.SellerID,
				OrderID:     &orderID,
				OrderItemID: &item.OrderItemID,
				Type:        "adjustment",
				AmountCents: -deductionCents,
				Currency:    "RUB",
				AvailableAt: nil,
				Metadata:    []byte(metaJSON),
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
				Metadata:    []byte(metaJSON),
				CreatedAt:   time.Now(),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// MarkFulfillmentDeliveredTx starts the 14-day hold for the earnings of a single delivered fulfillment.
// Other fulfillments of the same order (including other sellers) keep their own deadlines.
func (s *Service) MarkFulfillmentDeliveredTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, deliveredAt time.Time) error {
	availableAt := deliveredAt.AddDate(0, 0, 14) // 14-day hold
	return s.repo.UpdateAvailableAtByFulfillmentTx(ctx, tx, fulfillmentID, availableAt)
}

// Deprecated: affects all fulfillments of the order; delivery uses MarkFulfillmentDeliveredTx.
func (s *Service) MarkOrderDeliveredTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, deliveredAt time.Time) error {
	availableAt := deliveredAt.AddDate(0, 0, 14) // 14-day hold
	return s.repo.UpdateAvailableAtByOrderIdTx(ctx, tx, orderID, availableAt)
}


// --- Seller View ---
func (s *Service) GetSellerBalance(ctx context.Context, sellerID uuid.UUID) (*BalanceResponse, error) {
	bal, err := s.repo.GetSellerBalanceSummary(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	// Money under an open return is not payable now: it is shown as returnHoldCents, not as available.
	disputed, err := disputedAvailableCents(ctx, s.dbPool, &sellerID)
	if err != nil {
		return nil, err
	}
	held, err := heldInBatchesCents(ctx, s.dbPool, &sellerID)
	if err != nil {
		return nil, err
	}
	bal.AvailableCents -= disputed
	bal.ReturnHoldCents = disputed + held
	return bal, nil
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

	// PAYOUT.SAFETY.1 lock order: open returns of the seller first, then the ledger rows.
	if err := lockOpenReturnsForSellerTx(ctx, tx, sellerID); err != nil {
		return nil, err
	}
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
		return nil, ErrNothingToPay
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

	// Earnings disputed by an open return are held inside this batch: it pays the undisputed part only.
	held, err := s.applyReturnHoldsTx(ctx, tx, batch.ID, entries)
	if err != nil {
		return nil, err
	}
	if held > 0 {
		batch.AmountCents = total - held
		if batch.AmountCents <= 0 {
			return nil, ErrNothingToPay
		}
		if err := s.repo.UpdatePayoutBatchTx(ctx, tx, batch); err != nil {
			return nil, err
		}
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

	batch, err := s.repo.GetPayoutBatchForUpdateTx(ctx, tx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return ErrPayoutBatchNotFound
	}
	if batch.Status == "paid" {
		return ErrPayoutBatchAlreadyPaid
	}
	if batch.Status == "held" || batch.Status == "failed" {
		return fmt.Errorf("%w: %s", ErrPayoutBatchNotProcessable, batch.Status)
	}
	// PAYOUT.SAFETY.1: fail closed if a return opened after this batch disputes money in it.
	disputed, err := batchHasUnheldOpenReturnsTx(ctx, tx, batch.ID)
	if err != nil {
		return err
	}
	if disputed {
		return ErrPayoutBatchHasOpenReturns
	}
	now := time.Now()
	batch.Status = "paid"
	batch.ProcessedAt = &now

	// Batch Race Safety: 
	// If a return happened after this batch was created but before execution,
	// the seller might have a negative available balance. We must reduce the batch amount.
	balance, err := s.repo.GetSellerBalanceSummaryTx(ctx, tx, batch.SellerID)
	if err != nil {
		return err
	}
	originalAmount := batch.AmountCents
	currentUnbatchedAvailable := balance.AvailableCents
	if currentUnbatchedAvailable < 0 {
		reduction := -currentUnbatchedAvailable
		if reduction > batch.AmountCents {
			reduction = batch.AmountCents
		}
		batch.AmountCents -= reduction
		
		// If the batch was completely zeroed out, we still mark it as paid (for 0) 
		// but we must offset the debt so the seller doesn't get double-charged.
		// Wait, if we reduce the batch amount, we are effectively paying the debt.
		// We need to create a positive adjustment to clear the negative balance we just consumed!
		if reduction > 0 {
			// Immediately available like every other offsetting adjustment (available_at NULL): an app-clock
			// timestamp ahead of the DB clock would keep this offset out of the next batch while the debt it offsets
			// is already in it, recovering the debt twice (PAYOUT.SAFETY.1.1).
			err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
				ID:          uuid.New(),
				SellerID:    batch.SellerID,
				Type:        "adjustment",
				AmountCents: reduction,
				Currency:    "RUB",
				AvailableAt: nil,
				Metadata:    []byte(`{"reason":"batch_race_offset","batch_id":"` + batch.ID.String() + `"}`),
				CreatedAt:   time.Now(),
			})
			if err != nil {
				return err
			}
		}
	}

	if batch.AmountCents <= 0 {
		batch.Status = "failed"
		reason := "insufficient_balance"
		batch.FailureReason = &reason
		batch.AmountCents = originalAmount // prevent check constraint violation
		
		if err := s.repo.UpdatePayoutBatchTx(ctx, tx, batch); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

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

	batch, err := s.repo.GetPayoutBatchForUpdateTx(ctx, tx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return ErrPayoutBatchNotFound
	}
	if batch.Status == "paid" {
		return ErrPayoutBatchAlreadyPaid
	}
	// Only a batch that is still to be paid can be held; a failed batch has released its entries already.
	if batch.Status != "scheduled" && batch.Status != "processing" && batch.Status != "held" {
		return fmt.Errorf("%w: %s", ErrPayoutBatchNotHoldable, batch.Status)
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

	batch, err := s.repo.GetPayoutBatchForUpdateTx(ctx, tx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return ErrPayoutBatchNotFound
	}
	if batch.Status == "paid" {
		return ErrPayoutBatchAlreadyPaid
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
	sum, err := s.repo.GetAdminPayoutSummary(ctx)
	if err != nil {
		return nil, err
	}
	disputed, err := disputedAvailableCents(ctx, s.dbPool, nil)
	if err != nil {
		return nil, err
	}
	held, err := heldInBatchesCents(ctx, s.dbPool, nil)
	if err != nil {
		return nil, err
	}
	sum.TotalAvailableCents -= disputed
	sum.TotalReturnHoldCents = disputed + held
	return sum, nil
}

func (s *Service) ListAdminSellerBalances(ctx context.Context, limit, offset int) ([]AdminSellerBalance, int, error) {
	return []AdminSellerBalance{}, 0, nil
}

// ListAdminPayoutBatches lists real payout batches (newest first) with their seller and ledger composition.
func (s *Service) ListAdminPayoutBatches(ctx context.Context, filter PayoutFilter, limit, offset int) ([]AdminPayoutListItem, int, error) {
	return s.repo.ListAdminPayoutBatches(ctx, filter, limit, offset)
}

// GetAdminPayoutBatchDetail returns one batch with its ledger rows, totals and the actions its state allows.
// The confirm-paid check mirrors ProcessPayoutBatch (status + disputes not covered by a hold), read-only.
func (s *Service) GetAdminPayoutBatchDetail(ctx context.Context, batchID uuid.UUID) (*AdminPayoutDetail, error) {
	d, err := s.repo.GetAdminPayoutBatchDetail(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrPayoutBatchNotFound
	}
	open := d.Payout.Status == "scheduled" || d.Payout.Status == "processing"
	if open {
		uncovered, err := uncoveredDisputeCents(ctx, s.dbPool, batchID)
		if err != nil {
			return nil, err
		}
		d.UncoveredDisputeCents = uncovered
	}
	d.CanConfirmPaid = open && d.UncoveredDisputeCents == 0
	d.CanHold = open
	switch {
	case d.Payout.Status == "paid":
		d.ConfirmBlockedReason = "already_paid"
	case d.Payout.Status == "held":
		d.ConfirmBlockedReason = "held"
	case d.Payout.Status == "failed":
		d.ConfirmBlockedReason = "failed"
	case d.UncoveredDisputeCents > 0:
		d.ConfirmBlockedReason = "open_return_not_covered"
	}
	return d, nil
}
