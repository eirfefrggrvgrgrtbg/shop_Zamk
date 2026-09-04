package inventory

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)


func (r *Repository) GetReconciliationResolutionPlan(ctx context.Context, sessionID uuid.UUID) (*ReconciliationResolutionPlanDTO, error) {
	// 1. Verify session exists
	var sessID uuid.UUID
	var sessStatus string
	var sessVariantID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT id, status, product_variant_id
		FROM inventory_reconciliation_sessions
		WHERE id = $1
	`, sessionID).Scan(&sessID, &sessStatus, &sessVariantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReconciliationNotFound
		}
		return nil, fmt.Errorf("failed to load reconciliation session: %w", err)
	}

	plan := &ReconciliationResolutionPlanDTO{
		SessionID: sessionID,
		Cases:     []ReconciliationResolutionCase{},
	}

	// 2. Query facts for expected units and unexpected scans
	query := `
		SELECT
			COALESCE(e.inventory_unit_id, s.inventory_unit_id) as unit_id,
			iu.unit_code,
			iu.product_variant_id,
			p.title as product_title,
			COALESCE(pv.size, '') as variant_size,
			COALESCE(pv.color, '') as variant_color,
			COALESCE(pv.sku, '') as variant_sku,
			COALESCE(pv.barcode, '') as variant_barcode,
			e.expected_status,
			iu.status as current_status,
			s.classification,
			s.scanned_at,
			alloc.allocation_id,
			alloc.picked_at,
			alloc.released_at,
			alloc.release_reason,
			alloc.order_id,
			alloc.order_number,
			alloc.order_status,
			alloc.fulfillment_id,
			alloc.fulfillment_status,
			alloc.shipment_id,
			alloc.shipment_status,
			alloc.return_id,
			alloc.return_status,
			ss.id as origin_supply_id,
			ss.supply_number as origin_supply_number,
			ss.status as origin_supply_status
		FROM inventory_reconciliation_expected_units e
		FULL OUTER JOIN inventory_reconciliation_scans s
			ON e.session_id = s.session_id AND e.inventory_unit_id = s.inventory_unit_id
		JOIN inventory_units iu ON iu.id = COALESCE(e.inventory_unit_id, s.inventory_unit_id)
		JOIN product_variants pv ON iu.product_variant_id = pv.id
		JOIN products p ON pv.product_id = p.id
		LEFT JOIN seller_supplies ss ON iu.origin_supply_id = ss.id
		LEFT JOIN LATERAL (
			SELECT oia.id as allocation_id, oia.picked_at, oia.released_at, oia.release_reason,
			       oi.order_id, oi.order_fulfillment_id as fulfillment_id,
			       o.order_number, o.status as order_status,
			       f.status as fulfillment_status,
			       sh.id as shipment_id, sh.status as shipment_status,
			       ret.id as return_id, ret.status as return_status
			FROM order_item_allocations oia
			JOIN order_items oi ON oia.order_item_id = oi.id
			JOIN orders o ON oi.order_id = o.id
			LEFT JOIN order_fulfillments f ON oi.order_fulfillment_id = f.id
			LEFT JOIN shipments sh ON f.id = sh.fulfillment_id
			LEFT JOIN return_items ri ON oi.id = ri.order_item_id
			LEFT JOIN returns ret ON ri.return_id = ret.id
			WHERE oia.inventory_unit_id = iu.id
			ORDER BY (oia.released_at IS NULL) DESC, oia.created_at DESC
			LIMIT 1
		) alloc ON true
		WHERE (e.session_id = $1 OR s.session_id = $1)
		  AND (s.classification IS NULL OR s.classification NOT IN ('wrong_variant', 'unknown_code', 'duplicate'))
		ORDER BY iu.unit_code ASC
	`

	rows, err := r.db.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query resolution facts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fact RawResolutionFact
		err := rows.Scan(
			&fact.UnitID,
			&fact.UnitCode,
			&fact.VariantID,
			&fact.ProductTitle,
			&fact.VariantSize,
			&fact.VariantColor,
			&fact.VariantSKU,
			&fact.VariantBarcode,
			&fact.SnapshotStatus,
			&fact.CurrentStatus,
			&fact.Classification,
			&fact.ScannedAt,
			&fact.AllocationID,
			&fact.AllocationPickedAt,
			&fact.AllocationReleasedAt,
			&fact.AllocationReleaseReason,
			&fact.OrderID,
			&fact.OrderNumber,
			&fact.OrderStatus,
			&fact.FulfillmentID,
			&fact.FulfillmentStatus,
			&fact.ShipmentID,
			&fact.ShipmentStatus,
			&fact.ReturnID,
			&fact.ReturnStatus,
			&fact.OriginSupplyID,
			&fact.OriginSupplyNumber,
			&fact.OriginSupplyStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan resolution fact: %w", err)
		}

		resCase := ClassifyResolutionFact(fact)
		if resCase != nil {
			plan.Cases = append(plan.Cases, *resCase)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading resolution facts: %w", err)
	}

	return plan, nil
}
