package selleranalytics

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetProductsPerformance(ctx context.Context, sellerID uuid.UUID, from, to time.Time, asOfOpt ...time.Time) ([]ProductPerformance, error) {
	asOf := time.Now().UTC()
	if len(asOfOpt) > 0 && !asOfOpt[0].IsZero() {
		asOf = asOfOpt[0]
	}
	// A Product that is now archived/blocked but had sales in the selected period must remain visible.
	// So we start from order_items joined to seller_ledger_entries.
	query := `
		WITH cohort_items AS (
			SELECT
				oi.id as order_item_id,
				oi.product_id,
				MAX(oi.title) as title,
				oi.quantity as delivered_units,
				SUM(sle.amount_cents) as gross_cents,
				sle.order_id
			FROM seller_ledger_entries sle
			JOIN order_items oi ON sle.order_item_id = oi.id
			WHERE sle.seller_id = $1
			  AND sle.type = 'sale_gross'
			  AND sle.created_at >= $2
			  AND sle.created_at < $3
			GROUP BY oi.id, oi.product_id, oi.quantity, sle.order_id
		),
		order_item_returns AS (
			SELECT
				ri.order_item_id,
				SUM(ri.quantity) as returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			WHERE r.status = 'completed'
			  AND r.completed_at IS NOT NULL
			  AND r.completed_at <= $4
			GROUP BY ri.order_item_id
		),
		sales AS (
			SELECT
				ci.product_id,
				MAX(ci.title) as title,
				SUM(ci.gross_cents) as gross,
				COUNT(DISTINCT ci.order_id) as orders_count,
				SUM(ci.delivered_units) as units_sold,
				SUM(COALESCE(oir.returned_units, 0)) as cohort_returned_units,
				BOOL_OR(COALESCE(oir.returned_units, 0) > ci.delivered_units) as has_item_anomaly
			FROM cohort_items ci
			LEFT JOIN order_item_returns oir ON ci.order_item_id = oir.order_item_id
			GROUP BY ci.product_id
		),
		calendar_returns AS (
			SELECT
				oi.product_id,
				SUM(ri.quantity) as returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			JOIN order_items oi ON ri.order_item_id = oi.id
			WHERE oi.seller_id = $1
			  AND r.status = 'completed'
			  AND r.completed_at IS NOT NULL
			  AND r.completed_at >= $2
			  AND r.completed_at < $3
			  AND r.completed_at <= $4
			GROUP BY oi.product_id
		),
		inventory AS (
			SELECT 
				product_id,
				SUM(total_stock - reserved_stock) as available_stock
			FROM inventory_items
			WHERE seller_id = $1
			GROUP BY product_id
		)
		SELECT 
			COALESCE(s.product_id, cr.product_id) as product_id,
			COALESCE(s.title, (SELECT title FROM products WHERE id = COALESCE(s.product_id, cr.product_id))) as title,
			COALESCE(s.gross, 0) as gross,
			COALESCE(s.orders_count, 0) as orders_count,
			COALESCE(s.units_sold, 0) as units_sold,
			COALESCE(s.cohort_returned_units, 0) as cohort_returned_units,
			COALESCE(cr.returned_units, 0) as returned_units,
			COALESCE(i.available_stock, 0) as available_stock,
			COALESCE(s.has_item_anomaly, false) as has_item_anomaly
		FROM sales s
		FULL OUTER JOIN calendar_returns cr ON s.product_id = cr.product_id
		LEFT JOIN inventory i ON COALESCE(s.product_id, cr.product_id) = i.product_id
	`
	rows, err := r.db.Query(ctx, query, sellerID, from, to, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProductPerformance
	for rows.Next() {
		var p ProductPerformance
		if err := rows.Scan(
			&p.ProductID,
			&p.Title,
			&p.GrossSalesCents,
			&p.OrdersCount,
			&p.UnitsSold,
			&p.CohortReturnedUnits,
			&p.ReturnedUnits,
			&p.AvailableStock,
			&p.HasItemAnomaly,
		); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (r *Repository) GetVariantsPerformance(ctx context.Context, sellerID, productID uuid.UUID, from, to time.Time, asOfOpt ...time.Time) ([]VariantPerformance, error) {
	asOf := time.Now().UTC()
	if len(asOfOpt) > 0 && !asOfOpt[0].IsZero() {
		asOf = asOfOpt[0]
	}
	query := `
		WITH cohort_items AS (
			SELECT
				oi.id as order_item_id,
				oi.product_variant_id,
				MAX(oi.sku) as sku,
				MAX(NULLIF(TRIM(CONCAT_WS(' ', oi.variant_size, oi.variant_color)), '')) as display_name,
				oi.quantity as delivered_units,
				SUM(sle.amount_cents) as gross_cents
			FROM seller_ledger_entries sle
			JOIN order_items oi ON sle.order_item_id = oi.id
			WHERE sle.seller_id = $1
			  AND oi.product_id = $2
			  AND sle.type = 'sale_gross'
			  AND sle.created_at >= $3
			  AND sle.created_at < $4
			GROUP BY oi.id, oi.product_variant_id, oi.quantity
		),
		order_item_returns AS (
			SELECT
				ri.order_item_id,
				SUM(ri.quantity) as returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			WHERE r.status = 'completed'
			  AND r.completed_at IS NOT NULL
			  AND r.completed_at <= $5
			GROUP BY ri.order_item_id
		),
		sales AS (
			SELECT
				ci.product_variant_id,
				MAX(ci.sku) as sku,
				MAX(ci.display_name) as display_name,
				SUM(ci.gross_cents) as gross,
				SUM(ci.delivered_units) as units_sold,
				SUM(COALESCE(oir.returned_units, 0)) as cohort_returned_units,
				BOOL_OR(COALESCE(oir.returned_units, 0) > ci.delivered_units) as has_item_anomaly
			FROM cohort_items ci
			LEFT JOIN order_item_returns oir ON ci.order_item_id = oir.order_item_id
			GROUP BY ci.product_variant_id
		),
		calendar_returns AS (
			SELECT
				oi.product_variant_id,
				SUM(ri.quantity) as returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			JOIN order_items oi ON ri.order_item_id = oi.id
			WHERE oi.seller_id = $1
			  AND oi.product_id = $2
			  AND r.status = 'completed'
			  AND r.completed_at IS NOT NULL
			  AND r.completed_at >= $3
			  AND r.completed_at < $4
			  AND r.completed_at <= $5
			GROUP BY oi.product_variant_id
		),
		inventory AS (
			SELECT 
				product_variant_id,
				SUM(total_stock - reserved_stock) as available_stock
			FROM inventory_items
			WHERE seller_id = $1 AND product_id = $2
			GROUP BY product_variant_id
		)
		SELECT 
			pv.id as variant_id,
			$2 as product_id,
			COALESCE(NULLIF(pv.seller_sku, ''), NULLIF(pv.sku, ''), NULLIF(s.sku, ''), '') as sku,
			COALESCE(
				NULLIF(TRIM(CONCAT_WS(' ', COALESCE(sv.value, pv.size), COALESCE(c.name_ru, pv.color))), ''),
				s.display_name,
				NULLIF(pv.seller_sku, ''),
				NULLIF(pv.sku, ''),
				NULLIF(s.sku, ''),
				pv.id::text
			) as display_name,
			COALESCE(s.units_sold, 0) as units_sold,
			COALESCE(s.gross, 0) as gross,
			COALESCE(s.cohort_returned_units, 0) as cohort_returned_units,
			COALESCE(cr.returned_units, 0) as returned_units,
			COALESCE(i.available_stock, 0) as available_stock,
			COALESCE(s.has_item_anomaly, false) as has_item_anomaly
		FROM product_variants pv
		JOIN products p ON pv.product_id = p.id AND p.seller_id = $1
		LEFT JOIN colors c ON pv.color_id = c.id
		LEFT JOIN size_values sv ON pv.size_value_id = sv.id
		LEFT JOIN sales s ON pv.id = s.product_variant_id
		LEFT JOIN calendar_returns cr ON pv.id = cr.product_variant_id
		LEFT JOIN inventory i ON pv.id = i.product_variant_id
		WHERE pv.product_id = $2
		ORDER BY pv.created_at ASC, pv.id ASC
	`
	rows, err := r.db.Query(ctx, query, sellerID, productID, from, to, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VariantPerformance
	for rows.Next() {
		var v VariantPerformance
		if err := rows.Scan(
			&v.VariantID,
			&v.ProductID,
			&v.SKU,
			&v.DisplayName,
			&v.UnitsSold,
			&v.GrossSalesCents,
			&v.CohortReturnedUnits,
			&v.ReturnedUnits,
			&v.AvailableStock,
			&v.HasItemAnomaly,
		); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}

func (r *Repository) GetProductOwnership(ctx context.Context, sellerID, productID uuid.UUID) (bool, string, error) {
	query := `SELECT title FROM products WHERE id = $1 AND seller_id = $2`
	var title string
	err := r.db.QueryRow(ctx, query, productID, sellerID).Scan(&title)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, title, nil
}

func (r *Repository) GetProductTimeseries(ctx context.Context, sellerID, productID uuid.UUID, from, to time.Time, timezone string) ([]TimeseriesRow, error) {
	query := `
		WITH dates AS (
			SELECT generate_series(
				date_trunc('day', $3::timestamptz AT TIME ZONE $5),
				date_trunc('day', ($4::timestamptz - interval '1 microsecond') AT TIME ZONE $5),
				'1 day'::interval
			)::date AS d
		),
		sales AS (
			SELECT
				date_trunc('day', sle.created_at AT TIME ZONE $5)::date AS d,
				SUM(sle.amount_cents) AS gross,
				COUNT(DISTINCT sle.order_id) AS orders_count,
				SUM(oi.quantity) AS units_sold
			FROM seller_ledger_entries sle
			JOIN order_items oi ON sle.order_item_id = oi.id
			WHERE sle.seller_id = $1
			  AND oi.product_id = $2
			  AND sle.type = 'sale_gross'
			  AND sle.created_at >= $3
			  AND sle.created_at < $4
			GROUP BY 1
		),
		returns_stats AS (
			SELECT
				date_trunc('day', r.completed_at AT TIME ZONE $5)::date AS d,
				SUM(ri.quantity) AS returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			JOIN order_items oi ON ri.order_item_id = oi.id
			WHERE oi.seller_id = $1
			  AND oi.product_id = $2
			  AND r.status = 'completed'
			  AND r.completed_at >= $3
			  AND r.completed_at < $4
			GROUP BY 1
		)
		SELECT
			dates.d,
			COALESCE(s.gross, 0),
			COALESCE(s.orders_count, 0),
			COALESCE(s.units_sold, 0),
			COALESCE(r.returned_units, 0)
		FROM dates
		LEFT JOIN sales s ON dates.d = s.d
		LEFT JOIN returns_stats r ON dates.d = r.d
		ORDER BY dates.d ASC
	`
	rows, err := r.db.Query(ctx, query, sellerID, productID, from, to, timezone)
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
			&row.ReturnedUnits,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}
