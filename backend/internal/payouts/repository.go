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
		SELECT type, available_at <= now(), payout_batch_id IS NOT NULL, SUM(amount_cents)
		FROM seller_ledger_entries
		WHERE seller_id = $1
		GROUP BY type, available_at <= now(), payout_batch_id IS NOT NULL
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
				summary.AdjustmentsCents += total
			}
		case "payout":
			summary.PaidCents += total
		}
	}
	
	// Payouts are paid. The total Available is the sum of (sellerEarningAvailable + adjustments + payouts (which are negative)).
	// Because when a payout happens, we append a negative `payout` entry to ledger.
	summary.AvailableCents = sellerEarningAvailable + summary.AdjustmentsCents + summary.PaidCents
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
		UPDATE payout_batches SET status=$1, processed_at=$2, failure_reason=$3, updated_at=now() WHERE id=$4
	`
	_, err := tx.Exec(ctx, query, p.Status, p.ProcessedAt, p.FailureReason, p.ID)
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
		  AND (available_at <= now() OR type = 'adjustment')
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

func (r *Repository) UpdateAvailableAtByOrderIdTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, availableAt time.Time) error {
	query := `UPDATE seller_ledger_entries SET available_at = $1 WHERE order_id = $2 AND type = 'seller_earning'`
	_, err := tx.Exec(ctx, query, availableAt, orderID)
	return err
}

func (r *Repository) GetAdminPayoutSummary(ctx context.Context) (*AdminPayoutSummary, error) {
	// A simple aggregated view for admin
	query := `
		SELECT type, available_at <= now(), SUM(amount_cents)
		FROM seller_ledger_entries
		GROUP BY type, available_at <= now()
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
		var total int64
		if err := rows.Scan(&ltype, &isAvailable, &total); err != nil {
			return nil, err
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
			adjustments += total
		case "payout":
			summary.TotalPaidCents += total // Wait, payouts are negative.
		}
	}

	summary.TotalAvailableCents = sellerEarningAvailable + adjustments + summary.TotalPaidCents
	summary.TotalFrozenCents = sellerEarningFrozen
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
