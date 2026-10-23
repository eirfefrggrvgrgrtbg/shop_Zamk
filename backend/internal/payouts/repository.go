package payouts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// SellerCommissionRules
func (r *Repository) GetActiveCommissionRule(ctx context.Context, sellerID uuid.UUID) (*SellerCommissionRule, error) {
	query := `SELECT id, seller_id, rate_bps, reason, created_by, created_at FROM seller_commission_rules WHERE seller_id = $1 ORDER BY created_at DESC LIMIT 1`
	var rule SellerCommissionRule
	err := r.db.QueryRow(ctx, query, sellerID).Scan(&rule.ID, &rule.SellerID, &rule.RateBPS, &rule.Reason, &rule.CreatedBy, &rule.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil // No rule found
	}
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *Repository) ListCommissionRules(ctx context.Context, sellerID uuid.UUID) ([]SellerCommissionRule, error) {
	query := `SELECT id, seller_id, rate_bps, reason, created_by, created_at FROM seller_commission_rules WHERE seller_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []SellerCommissionRule
	for rows.Next() {
		var rule SellerCommissionRule
		if err := rows.Scan(&rule.ID, &rule.SellerID, &rule.RateBPS, &rule.Reason, &rule.CreatedBy, &rule.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, rule)
	}
	if list == nil {
		list = []SellerCommissionRule{}
	}
	return list, nil
}

func (r *Repository) CreateCommissionRule(ctx context.Context, rule *SellerCommissionRule) error {
	query := `INSERT INTO seller_commission_rules (id, seller_id, rate_bps, reason, created_by, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, query, rule.ID, rule.SellerID, rule.RateBPS, rule.Reason, rule.CreatedBy, rule.CreatedAt)
	return err
}

// Ledger Entries
func (r *Repository) CreateLedgerEntryTx(ctx context.Context, tx pgx.Tx, entry *SellerLedgerEntry) error {
	query := `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, payout_batch_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := tx.Exec(ctx, query, entry.ID, entry.SellerID, entry.OrderID, entry.OrderItemID, entry.PayoutBatchID, entry.Type, entry.AmountCents, entry.Currency, entry.AvailableAt, entry.Metadata, entry.CreatedAt)
	return err
}

func (r *Repository) GetSellerEarningEntryTx(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID) (*SellerLedgerEntry, error) {
	query := `
		SELECT id, seller_id, order_id, order_item_id, payout_batch_id, type, amount_cents, currency, available_at, metadata, created_at
		FROM seller_ledger_entries
		WHERE order_item_id = $1 AND type = 'seller_earning'
		FOR UPDATE
		LIMIT 1
	`
	var e SellerLedgerEntry
	err := tx.QueryRow(ctx, query, orderItemID).Scan(&e.ID, &e.SellerID, &e.OrderID, &e.OrderItemID, &e.PayoutBatchID, &e.Type, &e.AmountCents, &e.Currency, &e.AvailableAt, &e.Metadata, &e.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &e, err
}

func (r *Repository) HasReturnAdjustmentTx(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID, returnID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM seller_ledger_entries
			WHERE order_item_id = $1
			  AND type = 'adjustment'
			  AND (metadata->>'return_id') = $2
			  AND (metadata->>'reason') IN ('return_deduction', 'return_post_payout')
		)
	`
	var exists bool
	err := tx.QueryRow(ctx, query, orderItemID, returnID.String()).Scan(&exists)
	return exists, err
}

func (r *Repository) GetPriorReturnDeductionsTx(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID, excludeReturnID uuid.UUID) (int64, int64, error) {
	query := `
		SELECT
			COALESCE(SUM(ABS(sle.amount_cents)), 0) AS total_prior_deduction,
			COALESCE(SUM(
				CASE
					WHEN (sle.metadata->>'quantity') ~ '^[0-9]+$' THEN (sle.metadata->>'quantity')::bigint
					WHEN ri.quantity IS NOT NULL THEN ri.quantity::bigint
					ELSE 1
				END
			), 0) AS total_prior_qty
		FROM seller_ledger_entries sle
		LEFT JOIN return_items ri
			ON ri.return_id::text = (sle.metadata->>'return_id')
		   AND ri.order_item_id = sle.order_item_id
		WHERE sle.order_item_id = $1
		  AND sle.type = 'adjustment'
		  AND (sle.metadata->>'reason') IN ('return_deduction', 'return_post_payout')
		  AND (sle.metadata->>'return_id') IS NOT NULL
		  AND (sle.metadata->>'return_id') != $2
	`
	var totalDeduction, totalQty int64
	err := tx.QueryRow(ctx, query, orderItemID, excludeReturnID.String()).Scan(&totalDeduction, &totalQty)
	return totalDeduction, totalQty, err
}

func (r *Repository) ListSellerLedger(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerLedgerEntry, int, error) {
	query := `
		SELECT id, seller_id, order_id, order_item_id, payout_batch_id, type, amount_cents, currency, available_at, metadata, created_at
		FROM seller_ledger_entries
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, sellerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []SellerLedgerEntry
	for rows.Next() {
		var e SellerLedgerEntry
		if err := rows.Scan(&e.ID, &e.SellerID, &e.OrderID, &e.OrderItemID, &e.PayoutBatchID, &e.Type, &e.AmountCents, &e.Currency, &e.AvailableAt, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, e)
	}
	if list == nil {
		list = []SellerLedgerEntry{}
	}

	var count int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE seller_id = $1`, sellerID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return list, count, nil
}

func (r *Repository) GetSellerBalanceSummary(ctx context.Context, sellerID uuid.UUID) (*BalanceResponse, error) {
	query := `
		SELECT type, (available_at <= now() OR (type = 'adjustment' AND available_at IS NULL)), payout_batch_id IS NOT NULL, SUM(amount_cents)
		FROM seller_ledger_entries
		WHERE seller_id = $1
		GROUP BY type, (available_at <= now() OR (type = 'adjustment' AND available_at IS NULL)), payout_batch_id IS NOT NULL
	`
	rows, err := r.db.Query(ctx, query, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &BalanceResponse{Currency: "RUB"}
	var sellerEarningAvailable int64
	var sellerEarningFrozen int64

	for rows.Next() {
		var ltype string
		var isAvailable *bool
		var hasBatch bool
		var total int64
		if err := rows.Scan(&ltype, &isAvailable, &hasBatch, &total); err != nil {
			return nil, err
		}
		
		switch ltype {
		case "sale_gross":
			summary.GrossSalesCents += total
		case "zamk_commission":
			summary.CommissionCents += total
		case "seller_earning":
			if !hasBatch {
				if isAvailable != nil && *isAvailable {
					sellerEarningAvailable += total
				} else {
					sellerEarningFrozen += total
				}
			}
		case "adjustment":
			if !hasBatch {
				if isAvailable != nil && *isAvailable {
					summary.AdjustmentsCents += total
				} else {
					// Frozen adjustment (e.g. for a return of a still-frozen earning)
					// We subtract it from the frozen amount.
					sellerEarningFrozen += total
				}
			}
		case "payout":
			if total < 0 {
				summary.PaidCents += -total
			} else {
				summary.PaidCents += total
			}
		}
	}
	
	// Available balance is the sum of unbatched available earnings and unbatched adjustments.
	// Batched earnings/adjustments are already excluded via !hasBatch (payout_batch_id IS NULL).
	// PaidCents tracks historical disbursed payouts and is not deducted again.
	summary.AvailableCents = sellerEarningAvailable + summary.AdjustmentsCents
	summary.FrozenCents = sellerEarningFrozen

	return summary, nil
}

func (r *Repository) GetSellerBalanceSummaryTx(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID) (*BalanceResponse, error) {
	query := `
		SELECT type, (available_at <= now() OR (type = 'adjustment' AND available_at IS NULL)), payout_batch_id IS NOT NULL, SUM(amount_cents)
		FROM seller_ledger_entries
		WHERE seller_id = $1
		GROUP BY type, (available_at <= now() OR (type = 'adjustment' AND available_at IS NULL)), payout_batch_id IS NOT NULL
	`
	rows, err := tx.Query(ctx, query, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &BalanceResponse{Currency: "RUB"}
	var sellerEarningAvailable int64
	var sellerEarningFrozen int64

	for rows.Next() {
		var ltype string
		var isAvailable *bool
		var hasBatch bool
		var total int64
		if err := rows.Scan(&ltype, &isAvailable, &hasBatch, &total); err != nil {
			return nil, err
		}
		
		switch ltype {
		case "sale_gross":
			summary.GrossSalesCents += total
		case "zamk_commission":
			summary.CommissionCents += total
		case "seller_earning":
			if !hasBatch {
				if isAvailable != nil && *isAvailable {
					sellerEarningAvailable += total
				} else {
					sellerEarningFrozen += total
				}
			}
		case "adjustment":
			if !hasBatch {
				if isAvailable != nil && *isAvailable {
					summary.AdjustmentsCents += total
				} else {
					sellerEarningFrozen += total
				}
			}
		case "payout":
			if total < 0 {
				summary.PaidCents += -total
			} else {
				summary.PaidCents += total
			}
		}
	}

	// Available balance is the sum of unbatched available earnings and unbatched adjustments.
	// Batched earnings/adjustments are already excluded via !hasBatch (payout_batch_id IS NULL).
	// PaidCents tracks historical disbursed payouts and is not deducted again.
	summary.AvailableCents = sellerEarningAvailable + summary.AdjustmentsCents
	summary.FrozenCents = sellerEarningFrozen

	return summary, nil
}

// Payout Batches
func (r *Repository) ListPayoutBatches(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]PayoutBatch, int, error) {
	query := `
		SELECT id, seller_id, amount_cents, status, scheduled_for, processed_at, failure_reason, created_at, updated_at
		FROM payout_batches WHERE seller_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, sellerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []PayoutBatch
	for rows.Next() {
		var p PayoutBatch
		if err := rows.Scan(&p.ID, &p.SellerID, &p.AmountCents, &p.Status, &p.ScheduledFor, &p.ProcessedAt, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, p)
	}
	if list == nil {
		list = []PayoutBatch{}
	}
	var count int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM payout_batches WHERE seller_id = $1`, sellerID).Scan(&count)
	return list, count, err
}

func (r *Repository) GetPayoutBatch(ctx context.Context, id uuid.UUID) (*PayoutBatch, error) {
	query := `
		SELECT id, seller_id, amount_cents, status, scheduled_for, processed_at, failure_reason, created_at, updated_at
		FROM payout_batches WHERE id = $1
	`
	var p PayoutBatch
	err := r.db.QueryRow(ctx, query, id).Scan(&p.ID, &p.SellerID, &p.AmountCents, &p.Status, &p.ScheduledFor, &p.ProcessedAt, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// GetPayoutBatchForUpdateTx reads a batch inside the caller's transaction and locks its row, so concurrent status
// changes of the same batch (paid / held / failed) serialize on the current status.
func (r *Repository) GetPayoutBatchForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*PayoutBatch, error) {
	query := `
		SELECT id, seller_id, amount_cents, status, scheduled_for, processed_at, failure_reason, created_at, updated_at
		FROM payout_batches WHERE id = $1 FOR UPDATE
	`
	var p PayoutBatch
	err := tx.QueryRow(ctx, query, id).Scan(&p.ID, &p.SellerID, &p.AmountCents, &p.Status, &p.ScheduledFor, &p.ProcessedAt, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (r *Repository) CreatePayoutBatchTx(ctx context.Context, tx pgx.Tx, p *PayoutBatch) error {
	query := `
		INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, processed_at, failure_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := tx.Exec(ctx, query, p.ID, p.SellerID, p.AmountCents, p.Status, p.ScheduledFor, p.ProcessedAt, p.FailureReason, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *Repository) UpdatePayoutBatchTx(ctx context.Context, tx pgx.Tx, p *PayoutBatch) error {
	query := `
		UPDATE payout_batches SET amount_cents=$1, status=$2, processed_at=$3, failure_reason=$4, updated_at=now() WHERE id=$5
	`
	_, err := tx.Exec(ctx, query, p.AmountCents, p.Status, p.ProcessedAt, p.FailureReason, p.ID)
	return err
}

// Atomic payout processing
func (r *Repository) LockAvailableLedgerEntriesTx(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID) ([]SellerLedgerEntry, error) {
	// To prevent double payouts, we find entries that are "available" (available_at <= now())
	// and are not yet linked to a payout batch.
	// But wait, adjustments don't have available_at? Oh, we need to consider how adjustments and payouts work.
	// Let's just lock all available unpayouted entries. Payout batch id is null for those that haven't been picked up.
	query := `
		SELECT id, seller_id, order_id, order_item_id, payout_batch_id, type, amount_cents, currency, available_at, metadata, created_at
		FROM seller_ledger_entries
		WHERE seller_id = $1 
		  AND (available_at <= now() OR (type = 'adjustment' AND available_at IS NULL))
		  AND payout_batch_id IS NULL
		  AND type IN ('seller_earning', 'adjustment')
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SellerLedgerEntry
	for rows.Next() {
		var e SellerLedgerEntry
		if err := rows.Scan(&e.ID, &e.SellerID, &e.OrderID, &e.OrderItemID, &e.PayoutBatchID, &e.Type, &e.AmountCents, &e.Currency, &e.AvailableAt, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

func (r *Repository) LinkLedgerEntriesToPayoutTx(ctx context.Context, tx pgx.Tx, payoutBatchID uuid.UUID, entryIDs []uuid.UUID) error {
	if len(entryIDs) == 0 {
		return nil
	}
	query := `UPDATE seller_ledger_entries SET payout_batch_id = $1 WHERE id = ANY($2)`
	_, err := tx.Exec(ctx, query, payoutBatchID, entryIDs)
	return err
}

// UpdateAvailableAtByFulfillmentTx sets the hold deadline only on the seller_earning entries of order items
// that belong to the given fulfillment. Entries that already have a deadline or are linked to a payout batch
// are left untouched, so a repeated call never moves an existing deadline.
func (r *Repository) UpdateAvailableAtByFulfillmentTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, availableAt time.Time) error {
	query := `
		UPDATE seller_ledger_entries sle
		SET available_at = $1
		FROM order_items oi
		WHERE oi.id = sle.order_item_id
		  AND oi.order_fulfillment_id = $2
		  AND sle.type = 'seller_earning'
		  AND sle.available_at IS NULL
		  AND sle.payout_batch_id IS NULL
	`
	_, err := tx.Exec(ctx, query, availableAt, fulfillmentID)
	return err
}

// Deprecated: updates every seller_earning of the order regardless of fulfillment; use UpdateAvailableAtByFulfillmentTx.
func (r *Repository) UpdateAvailableAtByOrderIdTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, availableAt time.Time) error {
	query := `UPDATE seller_ledger_entries SET available_at = $1 WHERE order_id = $2 AND type = 'seller_earning'`
	_, err := tx.Exec(ctx, query, availableAt, orderID)
	return err
}

func (r *Repository) GetAdminPayoutSummary(ctx context.Context) (*AdminPayoutSummary, error) {
	// A simple aggregated view for admin
	query := `
		SELECT type, (available_at <= now() OR (type = 'adjustment' AND available_at IS NULL)), payout_batch_id IS NOT NULL, SUM(amount_cents)
		FROM seller_ledger_entries
		GROUP BY type, (available_at <= now() OR (type = 'adjustment' AND available_at IS NULL)), payout_batch_id IS NOT NULL
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &AdminPayoutSummary{Currency: "RUB"}
	var sellerEarningFrozen, sellerEarningAvailable int64
	var adjustments int64

	for rows.Next() {
		var ltype string
		var isAvailable *bool
		var hasBatch bool
		var total int64
		if err := rows.Scan(&ltype, &isAvailable, &hasBatch, &total); err != nil {
			return nil, err
		}

		// Like the seller balance: earnings and adjustments captured by a payout batch are no longer
		// available or frozen; they are pending (scheduled batch) or paid (payout entry).
		if hasBatch && (ltype == "seller_earning" || ltype == "adjustment") {
			continue
		}
		switch ltype {
		case "zamk_commission":
			summary.TotalCommissionCents += total
		case "seller_earning":
			if isAvailable != nil && *isAvailable {
				sellerEarningAvailable += total
			} else {
				sellerEarningFrozen += total
			}
		case "adjustment":
			if isAvailable != nil && *isAvailable {
				adjustments += total
			} else {
				sellerEarningFrozen += total
			}
		case "payout":
			summary.TotalPaidCents += total // Wait, payouts are negative.
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	summary.TotalAvailableCents = sellerEarningAvailable + adjustments
	summary.TotalFrozenCents = sellerEarningFrozen
	if err := r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM payout_batches WHERE status IN ('scheduled', 'processing')`).Scan(&summary.TotalPendingCents); err != nil {
		return nil, err
	}
	// TotalCommissionCents is negative, make it positive for display
	summary.TotalCommissionCents = -summary.TotalCommissionCents
	// TotalPaidCents is negative, make it positive for display
	summary.TotalPaidCents = -summary.TotalPaidCents

	return summary, nil
}

// UnfreezeAvailableEntries marks ledger entries as available when their available_at
// timestamp has elapsed. Returns how many rows were updated.
func (r *Repository) UnfreezeAvailableEntries(ctx context.Context, now time.Time, limit int) (int, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE seller_ledger_entries
		SET metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{unfrozen}', 'true')
		WHERE id IN (
			SELECT id FROM seller_ledger_entries
			WHERE available_at IS NOT NULL
			  AND available_at <= $1
			  AND (metadata->>'unfrozen') IS DISTINCT FROM 'true'
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
	`, now, limit)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// AdminPayoutStatuses are the payout_batches statuses an admin can filter by.
var AdminPayoutStatuses = map[string]bool{"scheduled": true, "processing": true, "paid": true, "failed": true, "held": true}

// ListAdminPayoutBatches: one page of real batches, newest first (created_at, id), with the seller name and the
// batch's ledger composition aggregated in the same query (no per-batch round trips); plus the filtered total.
func (r *Repository) ListAdminPayoutBatches(ctx context.Context, f PayoutFilter, limit, offset int) ([]AdminPayoutListItem, int, error) {
	var sellerID *uuid.UUID
	if f.SellerID != "" {
		id, err := uuid.Parse(f.SellerID)
		if err != nil {
			return nil, 0, err
		}
		sellerID = &id
	}
	where := `
		WHERE ($1 = '' OR pb.status = $1)
		  AND ($2::uuid IS NULL OR pb.seller_id = $2)
		  AND ($3 = '' OR pb.id::text ILIKE $3 || '%' OR s.brand_name ILIKE '%' || $3 || '%')`
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM payout_batches pb JOIN sellers s ON s.id = pb.seller_id`+where, f.Status, sellerID, f.Q).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT pb.id, pb.seller_id, pb.amount_cents, pb.status, pb.scheduled_for, pb.processed_at, pb.failure_reason,
		       pb.created_at, pb.updated_at, s.brand_name,
		       COALESCE(agg.earnings, 0), COALESCE(agg.entries, 0), COALESCE(agg.held, 0)
		FROM payout_batches pb
		JOIN sellers s ON s.id = pb.seller_id
		LEFT JOIN LATERAL (
			SELECT count(*) FILTER (WHERE sle.type = 'seller_earning') AS earnings,
			       count(*) FILTER (WHERE sle.type IN ('seller_earning', 'adjustment')) AS entries,
			       -SUM(sle.amount_cents) FILTER (WHERE sle.type = 'adjustment' AND sle.metadata->>'reason' = 'return_hold') AS held
			FROM seller_ledger_entries sle WHERE sle.payout_batch_id = pb.id
		) agg ON true`+where+`
		ORDER BY pb.created_at DESC, pb.id DESC
		LIMIT $4 OFFSET $5`, f.Status, sellerID, f.Q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []AdminPayoutListItem{}
	for rows.Next() {
		var it AdminPayoutListItem
		if err := rows.Scan(&it.ID, &it.SellerID, &it.AmountCents, &it.Status, &it.ScheduledFor, &it.ProcessedAt, &it.FailureReason,
			&it.CreatedAt, &it.UpdatedAt, &it.SellerName, &it.EarningsCount, &it.EntriesCount, &it.ReturnHoldCents); err != nil {
			return nil, 0, err
		}
		it.Currency = "RUB"
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// GetAdminPayoutBatchDetail loads a batch, its seller and every ledger row linked to it (nil if not found).
func (r *Repository) GetAdminPayoutBatchDetail(ctx context.Context, id uuid.UUID) (*AdminPayoutDetail, error) {
	d := &AdminPayoutDetail{Entries: []AdminPayoutEntry{}}
	p := &d.Payout
	err := r.db.QueryRow(ctx, `
		SELECT pb.id, pb.seller_id, pb.amount_cents, pb.status, pb.scheduled_for, pb.processed_at, pb.failure_reason,
		       pb.created_at, pb.updated_at, s.brand_name
		FROM payout_batches pb JOIN sellers s ON s.id = pb.seller_id WHERE pb.id = $1`, id).Scan(
		&p.ID, &p.SellerID, &p.AmountCents, &p.Status, &p.ScheduledFor, &p.ProcessedAt, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt, &p.SellerName)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Currency = "RUB"
	rows, err := r.db.Query(ctx, `
		SELECT sle.id, sle.type, sle.metadata->>'reason', sle.amount_cents, sle.order_id, o.order_number, sle.order_item_id, oi.title,
		       sle.available_at, sle.created_at
		FROM seller_ledger_entries sle
		LEFT JOIN orders o ON o.id = sle.order_id
		LEFT JOIN order_items oi ON oi.id = sle.order_item_id
		WHERE sle.payout_batch_id = $1
		ORDER BY sle.created_at, sle.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e AdminPayoutEntry
		if err := rows.Scan(&e.ID, &e.Type, &e.Reason, &e.AmountCents, &e.OrderID, &e.OrderNumber, &e.OrderItemID, &e.ProductTitle, &e.AvailableAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		switch {
		case e.Type == "seller_earning":
			d.Totals.EarningsCents += e.AmountCents
			p.EarningsCount++
		case e.Type == "payout":
			d.Totals.PayoutCents += e.AmountCents
		case e.Reason != nil && *e.Reason == "return_hold":
			d.Totals.ReturnHoldCents += -e.AmountCents
		default:
			d.Totals.AdjustmentsCents += e.AmountCents
		}
		if e.Type == "seller_earning" || e.Type == "adjustment" {
			p.EntriesCount++
		}
		d.Entries = append(d.Entries, e)
	}
	p.ReturnHoldCents = d.Totals.ReturnHoldCents
	return d, rows.Err()
}
