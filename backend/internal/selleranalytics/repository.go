package selleranalytics

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetLedgerSummary(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (LedgerSummary, error) {
	query := `
		SELECT 
			type,
			amount_cents,
			metadata
		FROM seller_ledger_entries
		WHERE seller_id = $1 AND created_at >= $2 AND created_at < $3
	`
	rows, err := r.db.Query(ctx, query, sellerID, from, to)
	if err != nil {
		return LedgerSummary{}, err
	}
	defer rows.Close()

	var summary LedgerSummary
	for rows.Next() {
		var lType string
		var amount int64
		var meta []byte
		if err := rows.Scan(&lType, &amount, &meta); err != nil {
			return summary, err
		}

		switch lType {
		case "sale_gross":
			summary.GrossSalesCents += amount
		case "zamk_commission":
			if amount < 0 {
				summary.CommissionCents += -amount // ABS as per requirements
			} else {
				summary.CommissionCents += amount
			}
		case "seller_earning":
			summary.SellerEarningCents += amount
		case "adjustment":
			isReturn := false
			if len(meta) > 0 {
				// quick and dirty metadata check
				metaStr := string(meta)
				if containsString(metaStr, `"reason":"return_deduction"`) || containsString(metaStr, `"reason":"return_post_payout"`) || containsString(metaStr, `"reason": "return_deduction"`) || containsString(metaStr, `"reason": "return_post_payout"`) {
					isReturn = true
				}
			}
			if isReturn {
				summary.ReturnDeductionsCents += amount
			} else {
				summary.OtherAdjustmentsCents += amount
			}
		}
	}
	return summary, nil
}

func containsString(s, substr string) bool {
	// Simple sub string check for metadata (in reality json parsing in SQL is better, but this works given exact writes)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (r *Repository) GetSellerOrders(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (int, error) {
	query := `
		SELECT COUNT(DISTINCT order_id)
		FROM seller_ledger_entries
		WHERE seller_id = $1 
		AND type = 'sale_gross'
		AND created_at >= $2 
		AND created_at < $3
	`
	var count int
	err := r.db.QueryRow(ctx, query, sellerID, from, to).Scan(&count)
	return count, err
}

func (r *Repository) GetUnitsSold(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (int, error) {
	query := `
		SELECT COALESCE(SUM(oi.quantity), 0)
		FROM seller_ledger_entries sle
		JOIN order_items oi ON sle.order_item_id = oi.id
		WHERE sle.seller_id = $1 
		AND sle.type = 'sale_gross'
		AND sle.created_at >= $2 
		AND sle.created_at < $3
	`
	var count int
	err := r.db.QueryRow(ctx, query, sellerID, from, to).Scan(&count)
	return count, err
}

func (r *Repository) GetReturnedUnits(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (int, error) {
	query := `
		SELECT COALESCE(SUM(ri.quantity), 0)
		FROM returns r
		JOIN return_items ri ON r.id = ri.return_id
		JOIN order_items oi ON ri.order_item_id = oi.id
		WHERE oi.seller_id = $1 
		AND r.status = 'completed'
		AND r.completed_at >= $2 
		AND r.completed_at < $3
	`
	var count int
	err := r.db.QueryRow(ctx, query, sellerID, from, to).Scan(&count)
	return count, err
}

func (r *Repository) GetOverviewTimeseries(ctx context.Context, sellerID uuid.UUID, from, to time.Time, timezone string) ([]TimeseriesRow, error) {
	// Uses Europe/Moscow for DATE_TRUNC
	query := `
		WITH dates AS (
			SELECT generate_series(
				date_trunc('day', $2::timestamptz AT TIME ZONE $4),
				date_trunc('day', $3::timestamptz AT TIME ZONE $4) - interval '1 day',
				'1 day'::interval
			)::date AS d
		),
		ledger_stats AS (
			SELECT 
				date_trunc('day', created_at AT TIME ZONE $4)::date as d,
				SUM(CASE WHEN type = 'sale_gross' THEN amount_cents ELSE 0 END) as gross,
				COUNT(DISTINCT CASE WHEN type = 'sale_gross' THEN order_id END) as orders_count,
				SUM(CASE WHEN type = 'zamk_commission' THEN ABS(amount_cents) ELSE 0 END) as commission,
				SUM(CASE WHEN type = 'seller_earning' THEN amount_cents ELSE 0 END) as earning,
				SUM(CASE WHEN type = 'adjustment' AND (metadata->>'reason' = 'return_deduction' OR metadata->>'reason' = 'return_post_payout') THEN amount_cents ELSE 0 END) as return_deductions
			FROM seller_ledger_entries
			WHERE seller_id = $1 AND created_at >= $2 AND created_at < $3
			GROUP BY 1
		),
		units_stats AS (
			SELECT 
				date_trunc('day', sle.created_at AT TIME ZONE $4)::date as d,
				SUM(oi.quantity) as units_sold
			FROM seller_ledger_entries sle
			JOIN order_items oi ON sle.order_item_id = oi.id
			WHERE sle.seller_id = $1 AND sle.type = 'sale_gross' AND sle.created_at >= $2 AND sle.created_at < $3
			GROUP BY 1
		),
		returns_stats AS (
			SELECT 
				date_trunc('day', r.completed_at AT TIME ZONE $4)::date as d,
				SUM(ri.quantity) as returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			JOIN order_items oi ON ri.order_item_id = oi.id
			WHERE oi.seller_id = $1 AND r.status = 'completed' AND r.completed_at >= $2 AND r.completed_at < $3
			GROUP BY 1
		)
		SELECT 
			dates.d,
			COALESCE(l.gross, 0),
			COALESCE(l.orders_count, 0),
			COALESCE(u.units_sold, 0),
			COALESCE(l.commission, 0),
			COALESCE(l.earning, 0),
			COALESCE(l.return_deductions, 0),
			COALESCE(r.returned_units, 0)
		FROM dates
		LEFT JOIN ledger_stats l ON dates.d = l.d
		LEFT JOIN units_stats u ON dates.d = u.d
		LEFT JOIN returns_stats r ON dates.d = r.d
		ORDER BY dates.d ASC
	`
	rows, err := r.db.Query(ctx, query, sellerID, from, to, timezone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TimeseriesRow
	for rows.Next() {
		var row TimeseriesRow
		if err := rows.Scan(
			&row.Date,
			&row.GrossSalesCents,
			&row.OrdersCount,
			&row.UnitsSold,
			&row.CommissionCents,
			&row.SellerEarningCents,
			&row.ReturnDeductionsCents,
			&row.ReturnedUnits,
		); err != nil {
			return nil, err
		}
		row.NetCommercialEarningCents = row.SellerEarningCents + row.ReturnDeductionsCents
		result = append(result, row)
	}
	return result, nil
}

func (r *Repository) CheckHasHistoricalSales(ctx context.Context, sellerID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM seller_ledger_entries WHERE seller_id = $1 AND type = 'sale_gross')`
	err := r.db.QueryRow(ctx, query, sellerID).Scan(&exists)
	return exists, err
}

// ResolveSellerID resolves a user UUID (from JWT sub) to the seller UUID.
func (r *Repository) ResolveSellerID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var sellerID uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT seller_id FROM seller_users WHERE user_id = $1 LIMIT 1`,
		userID,
	).Scan(&sellerID)
	return sellerID, err
}
