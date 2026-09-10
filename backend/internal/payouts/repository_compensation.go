package payouts

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetCompensableQuantityTx derives the authoritative compensable quantity C for an order item.
// It implements per-return intersection:
// For each return r for this order item:
//   reversedQty_r = authoritative Sale Reversal quantity from ledger
//   physicalCompensableQty_r = authoritative responsibility + physical loss quantity for same return
//   eligibleQty_r = min(reversedQty_r, physicalCompensableQty_r)
// Total C = SUM(eligibleQty_r) across all returns for this order item.
func (r *Repository) GetCompensableQuantityTx(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID) (int64, error) {
	query := `
		WITH reversed_per_return AS (
			SELECT
				(sle.metadata->>'return_id')::uuid AS return_id,
				COALESCE(SUM(
					CASE
						WHEN (sle.metadata->>'quantity') ~ '^[0-9]+$' THEN (sle.metadata->>'quantity')::bigint
						WHEN ri.quantity IS NOT NULL THEN ri.quantity::bigint
						ELSE 1
					END
				), 0) AS reversed_qty
			FROM seller_ledger_entries sle
			LEFT JOIN return_items ri
				ON ri.return_id::text = (sle.metadata->>'return_id')
			   AND ri.order_item_id = sle.order_item_id
			WHERE sle.order_item_id = $1
			  AND sle.type = 'adjustment'
			  AND (sle.metadata->>'reason') IN ('return_deduction', 'return_post_payout')
			  AND (sle.metadata->>'return_id') IS NOT NULL
			GROUP BY (sle.metadata->>'return_id')::uuid
		),
		return_items_for_order_item AS (
			SELECT
				ri.id AS return_item_id,
				ri.return_id
			FROM return_items ri
			WHERE ri.order_item_id = $1
		),
		returns_for_order_item AS (
			SELECT DISTINCT return_id
			FROM return_items_for_order_item
		),
		legacy_compensable_per_return AS (
			SELECT
				ri.return_id,
				COALESCE(SUM(rra.quantity), 0) AS qty
			FROM return_items_for_order_item ri
			JOIN return_responsibility_allocations rra
				ON rra.return_item_id = ri.return_item_id
			   AND rra.order_item_allocation_id IS NULL
			   AND rra.legacy_disposition = 'damaged'
			   AND rra.status = 'resolved'
			   AND (
			       (rra.responsible_party = 'zamk' AND rra.reason_code = 'zamk_warehouse_damage')
			       OR
			       (rra.responsible_party = 'zamk' AND rra.reason_code = 'zamk_fulfillment_error')
			       OR
			       (rra.responsible_party = 'carrier' AND rra.reason_code = 'carrier_damage')
			   )
			GROUP BY ri.return_id
		),
		serialized_compensable_per_return AS (
			SELECT
				ri.return_id,
				COUNT(DISTINCT rra.id) AS qty
			FROM return_items_for_order_item ri
			JOIN return_responsibility_allocations rra
				ON rra.return_item_id = ri.return_item_id
			   AND rra.order_item_allocation_id IS NOT NULL
			   AND rra.status = 'resolved'
			   AND (
			       (rra.responsible_party = 'zamk' AND rra.reason_code = 'zamk_warehouse_damage')
			       OR
			       (rra.responsible_party = 'zamk' AND rra.reason_code = 'zamk_fulfillment_error')
			       OR
			       (rra.responsible_party = 'carrier' AND rra.reason_code = 'carrier_damage')
			   )
			JOIN return_item_units riu
				ON riu.return_item_id = ri.return_item_id
			   AND riu.order_item_allocation_id = rra.order_item_allocation_id
			   AND riu.disposition = 'damaged'
			GROUP BY ri.return_id
		),
		physical_loss_per_return AS (
			SELECT
				ret.return_id,
				(COALESCE(lc.qty, 0) + COALESCE(sc.qty, 0)) AS physical_compensable_qty
			FROM returns_for_order_item ret
			LEFT JOIN legacy_compensable_per_return lc ON lc.return_id = ret.return_id
			LEFT JOIN serialized_compensable_per_return sc ON sc.return_id = ret.return_id
		)
		SELECT
			COALESCE(SUM(
				LEAST(COALESCE(r.reversed_qty, 0), p.physical_compensable_qty)
			), 0) AS total_compensable
		FROM physical_loss_per_return p
		LEFT JOIN reversed_per_return r ON r.return_id = p.return_id;
	`
	var totalCompensable int64
	err := tx.QueryRow(ctx, query, orderItemID).Scan(&totalCompensable)
	return totalCompensable, err
}

func (r *Repository) GetNetPriorCompensationTx(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM seller_ledger_entries
		WHERE order_item_id = $1
		  AND type = 'adjustment'
		  AND metadata->>'reason' IN ('return_compensation', 'return_compensation_correction')
	`
	var netPrior int64
	err := tx.QueryRow(ctx, query, orderItemID).Scan(&netPrior)
	return netPrior, err
}
