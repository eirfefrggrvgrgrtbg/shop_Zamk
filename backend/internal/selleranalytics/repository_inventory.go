package selleranalytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) GetInventoryPerformance(ctx context.Context, sellerID uuid.UUID, from, to time.Time) ([]InventoryPerformance, error) {
	query := `
		WITH sales AS (
			SELECT 
				oi.product_variant_id,
				SUM(oi.quantity) as units_sold
			FROM seller_ledger_entries sle
			JOIN order_items oi ON sle.order_item_id = oi.id
			WHERE sle.seller_id = $1 AND sle.type = 'sale_gross' AND sle.created_at >= $2 AND sle.created_at < $3
			GROUP BY oi.product_variant_id
		),
		inbound AS (
			SELECT 
				si.variant_id,
				SUM(GREATEST(0, si.expected_quantity - si.accepted_quantity)) as inbound_qty
			FROM supplies s
			JOIN supply_items si ON s.id = si.supply_id
			WHERE s.seller_id = $1 AND s.status NOT IN ('completed', 'cancelled')
			GROUP BY si.variant_id
		),
		inventory AS (
			SELECT 
				product_id,
				product_variant_id,
				total_stock as on_hand,
				reserved_stock as reserved,
				(total_stock - reserved_stock) as available
			FROM inventory_items
			WHERE seller_id = $1
		)
		SELECT 
			i.product_id,
			i.product_variant_id,
			COALESCE((SELECT sku FROM product_variants WHERE id = i.product_variant_id), '') as sku,
			i.available,
			i.on_hand,
			i.reserved,
			COALESCE(ib.inbound_qty, 0) as inbound,
			COALESCE(s.units_sold, 0) as units_sold
		FROM inventory i
		LEFT JOIN inbound ib ON i.product_variant_id = ib.variant_id
		LEFT JOIN sales s ON i.product_variant_id = s.product_variant_id
	`
	rows, err := r.db.Query(ctx, query, sellerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []InventoryPerformance
	for rows.Next() {
		var i InventoryPerformance
		if err := rows.Scan(
			&i.ProductID,
			&i.VariantID,
			&i.SKU,
			&i.Available,
			&i.OnHand,
			&i.Reserved,
			&i.Inbound,
			&i.UnitsSold,
		); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, nil
}
