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
			COALESCE(SUM(CASE WHEN type = 'sale_gross' THEN amount_cents ELSE 0 END), 0) AS gross_sales_cents,
			COALESCE(SUM(CASE WHEN type = 'zamk_commission' THEN ABS(amount_cents) ELSE 0 END), 0) AS commission_cents,
			COALESCE(SUM(CASE WHEN type = 'seller_earning' THEN amount_cents ELSE 0 END), 0) AS seller_earning_cents,
			COALESCE(SUM(CASE WHEN type = 'adjustment' AND metadata->>'reason' IN ('return_deduction', 'return_post_payout') THEN amount_cents ELSE 0 END), 0) AS return_deductions_cents,
			COALESCE(SUM(CASE WHEN type = 'adjustment' AND (metadata->>'reason' IS NULL OR metadata->>'reason' NOT IN ('return_deduction', 'return_post_payout')) THEN amount_cents ELSE 0 END), 0) AS other_adjustments_cents
		FROM seller_ledger_entries
		WHERE seller_id = $1 AND created_at >= $2 AND created_at < $3
	`
	var summary LedgerSummary
	err := r.db.QueryRow(ctx, query, sellerID, from, to).Scan(
		&summary.GrossSalesCents,
		&summary.CommissionCents,
		&summary.SellerEarningCents,
		&summary.ReturnDeductionsCents,
		&summary.OtherAdjustmentsCents,
	)
	return summary, err
}

func (r *Repository) GetSellerPaidOrders(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (int, error) {
	query := `
		WITH canonical_payments AS (
			SELECT
				order_id,
				MIN(paid_at) AS paid_at
			FROM payments
			WHERE status = 'succeeded' AND paid_at IS NOT NULL
			GROUP BY order_id
		)
		SELECT COUNT(DISTINCT f.order_id)
		FROM order_fulfillments f
		JOIN canonical_payments cp ON cp.order_id = f.order_id
		JOIN orders o ON o.id = f.order_id
		WHERE f.seller_id = $1
		  AND (
			  f.status != 'cancelled'
			  OR (o.cancelled_at IS NOT NULL AND o.cancelled_at >= cp.paid_at)
			  OR (
				  o.cancelled_at IS NULL
				  AND EXISTS (
					  SELECT 1 FROM order_status_history osh
					  WHERE osh.order_id = f.order_id
					    AND osh.to_status = 'cancelled'
					    AND osh.created_at >= cp.paid_at
				  )
			  )
		  )
		  AND cp.paid_at >= $2
		  AND cp.paid_at < $3
	`
	var count int
	err := r.db.QueryRow(ctx, query, sellerID, from, to).Scan(&count)
	return count, err
}

func (r *Repository) GetSellerDeliveredOrders(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (int, error) {
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

// GetSellerOrders delegates to GetSellerPaidOrders under the confirmed Option A contract.
func (r *Repository) GetSellerOrders(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (int, error) {
	return r.GetSellerPaidOrders(ctx, sellerID, from, to)
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
				date_trunc('day', ($3::timestamptz - interval '1 microsecond') AT TIME ZONE $4),
				'1 day'::interval
			)::date AS d
		),
		ledger_stats AS (
			SELECT 
				date_trunc('day', created_at AT TIME ZONE $4)::date as d,
				SUM(CASE WHEN type = 'sale_gross' THEN amount_cents ELSE 0 END) as gross,
				SUM(CASE WHEN type = 'zamk_commission' THEN ABS(amount_cents) ELSE 0 END) as commission,
				SUM(CASE WHEN type = 'seller_earning' THEN amount_cents ELSE 0 END) as earning,
				SUM(CASE WHEN type = 'adjustment' AND (metadata->>'reason' = 'return_deduction' OR metadata->>'reason' = 'return_post_payout') THEN amount_cents ELSE 0 END) as return_deductions
			FROM seller_ledger_entries
			WHERE seller_id = $1 AND created_at >= $2 AND created_at < $3
			GROUP BY 1
		),
		canonical_payments AS (
			SELECT
				order_id,
				MIN(paid_at) AS paid_at
			FROM payments
			WHERE status = 'succeeded' AND paid_at IS NOT NULL
			GROUP BY order_id
		),
		paid_orders_stats AS (
			SELECT
				date_trunc('day', cp.paid_at AT TIME ZONE $4)::date as d,
				COUNT(DISTINCT f.order_id) as orders_count
			FROM order_fulfillments f
			JOIN canonical_payments cp ON cp.order_id = f.order_id
			JOIN orders o ON o.id = f.order_id
			WHERE f.seller_id = $1
			  AND (
				  f.status != 'cancelled'
				  OR (o.cancelled_at IS NOT NULL AND o.cancelled_at >= cp.paid_at)
				  OR (
					  o.cancelled_at IS NULL
					  AND EXISTS (
						  SELECT 1 FROM order_status_history osh
						  WHERE osh.order_id = f.order_id
						    AND osh.to_status = 'cancelled'
						    AND osh.created_at >= cp.paid_at
					  )
				  )
			  )
			  AND cp.paid_at >= $2 AND cp.paid_at < $3
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
			COALESCE(po.orders_count, 0),
			COALESCE(u.units_sold, 0),
			COALESCE(l.commission, 0),
			COALESCE(l.earning, 0),
			COALESCE(l.return_deductions, 0),
			COALESCE(r.returned_units, 0)
		FROM dates
		LEFT JOIN ledger_stats l ON dates.d = l.d
		LEFT JOIN paid_orders_stats po ON dates.d = po.d
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
