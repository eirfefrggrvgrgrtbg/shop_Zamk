package selleranalytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) GetProductsPerformance(ctx context.Context, sellerID uuid.UUID, from, to time.Time) ([]ProductPerformance, error) {
	// A Product that is now archived/blocked but had sales in the selected period must remain visible.
	// So we start from order_items joined to seller_ledger_entries.
	query := `
		WITH sales AS (
			SELECT 
				oi.product_id,
				MAX(oi.title) as title,
				SUM(sle.amount_cents) as gross,
				COUNT(DISTINCT sle.order_id) as orders_count,
				SUM(oi.quantity) as units_sold
			FROM seller_ledger_entries sle
			JOIN order_items oi ON sle.order_item_id = oi.id
			WHERE sle.seller_id = $1 AND sle.type = 'sale_gross' AND sle.created_at >= $2 AND sle.created_at < $3
			GROUP BY oi.product_id
		),
		returns_stats AS (
			SELECT 
				oi.product_id,
				SUM(ri.quantity) as returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			JOIN order_items oi ON ri.order_item_id = oi.id
			WHERE oi.seller_id = $1 AND r.status = 'completed' AND r.completed_at >= $2 AND r.completed_at < $3
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
			COALESCE(s.product_id, r.product_id) as product_id,
			COALESCE(s.title, (SELECT title FROM products WHERE id = COALESCE(s.product_id, r.product_id))) as title,
			COALESCE(s.gross, 0) as gross,
			COALESCE(s.orders_count, 0) as orders_count,
			COALESCE(s.units_sold, 0) as units_sold,
			COALESCE(r.returned_units, 0) as returned_units,
			COALESCE(i.available_stock, 0) as available_stock
		FROM sales s
		FULL OUTER JOIN returns_stats r ON s.product_id = r.product_id
		LEFT JOIN inventory i ON COALESCE(s.product_id, r.product_id) = i.product_id
	`
	rows, err := r.db.Query(ctx, query, sellerID, from, to)
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
			&p.ReturnedUnits,
			&p.AvailableStock,
		); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (r *Repository) GetVariantsPerformance(ctx context.Context, sellerID, productID uuid.UUID, from, to time.Time) ([]VariantPerformance, error) {
	query := `
		WITH sales AS (
			SELECT 
				oi.product_variant_id,
				MAX(oi.sku) as sku,
				MAX(NULLIF(TRIM(CONCAT_WS(' ', oi.variant_size, oi.variant_color)), '')) as display_name,
				SUM(sle.amount_cents) as gross,
				SUM(oi.quantity) as units_sold
			FROM seller_ledger_entries sle
			JOIN order_items oi ON sle.order_item_id = oi.id
			WHERE sle.seller_id = $1 AND oi.product_id = $2 AND sle.type = 'sale_gross' AND sle.created_at >= $3 AND sle.created_at < $4
			GROUP BY oi.product_variant_id
		),
		returns_stats AS (
			SELECT 
				oi.product_variant_id,
				SUM(ri.quantity) as returned_units
			FROM returns r
			JOIN return_items ri ON r.id = ri.return_id
			JOIN order_items oi ON ri.order_item_id = oi.id
			WHERE oi.seller_id = $1 AND oi.product_id = $2 AND r.status = 'completed' AND r.completed_at >= $3 AND r.completed_at < $4
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
			COALESCE(pv.sku, '') as sku,
			COALESCE(
				s.display_name, 
				NULLIF(TRIM(CONCAT_WS(' ', pv.size, pv.color)), ''),
				pv.sku,
				pv.id::text
			) as display_name,
			COALESCE(s.units_sold, 0) as units_sold,
			COALESCE(s.gross, 0) as gross,
			COALESCE(r.returned_units, 0) as returned_units,
			COALESCE(i.available_stock, 0) as available_stock
		FROM product_variants pv
		LEFT JOIN sales s ON pv.id = s.product_variant_id
		LEFT JOIN returns_stats r ON pv.id = r.product_variant_id
		LEFT JOIN inventory i ON pv.id = i.product_variant_id
		WHERE pv.product_id = $2
	`
	rows, err := r.db.Query(ctx, query, sellerID, productID, from, to)
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
			&v.ReturnedUnits,
			&v.AvailableStock,
		); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}
