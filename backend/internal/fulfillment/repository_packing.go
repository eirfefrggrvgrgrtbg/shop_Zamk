package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PackResult struct {
	FulfillmentID     uuid.UUID `json:"fulfillmentId"`
	OrderID           uuid.UUID `json:"orderId"`
	FulfillmentStatus string    `json:"fulfillmentStatus"`
	OrderStatus       string    `json:"orderStatus"`
	PackedAt          time.Time `json:"packedAt"`
}

func (r *Repository) PackFulfillmentTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) (*PackResult, error) {
	// 1. Resolve parent order ID (plain lookup without locking)
	var orderID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT order_id FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to lookup fulfillment order: %w", err)
	}

	// 2. Lock parent order FIRST (authoritative serialization point, prevents deadlocks with cancellation)
	var orderStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&orderStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to lock parent order for packing: %w", err)
	}

	// 3. Lock fulfillment SECOND (consistent lock order: orders -> order_fulfillments)
	var fulfillmentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1 FOR UPDATE`, fulfillmentID).Scan(&fulfillmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to lock fulfillment for packing: %w", err)
	}

	// 2. Validate fulfillment status: MUST be "assembling"
	if fulfillmentStatus != "assembling" {
		return nil, ErrPackingNotAllowed
	}

	// 3. Validate parent order status: MUST be "assembling"
	if orderStatus != "assembling" {
		return nil, ErrPackingNotAllowed
	}

	// 4. Query order items for this fulfillment
	queryItems := `
		SELECT id, quantity, picked_quantity
		FROM order_items
		WHERE order_fulfillment_id = $1
		ORDER BY created_at ASC, id ASC
	`
	rows, err := tx.Query(ctx, queryItems, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order items: %w", err)
	}
	defer rows.Close()

	type itemInfo struct {
		id           uuid.UUID
		quantity     int
		legacyPicked int
	}
	var items []itemInfo
	for rows.Next() {
		var it itemInfo
		if err := rows.Scan(&it.id, &it.quantity, &it.legacyPicked); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, ErrInvariantViolation
	}

	// 5. Verify every item satisfies packing rules
	for _, it := range items {
		if it.quantity <= 0 {
			return nil, ErrInvariantViolation
		}

		queryAllocs := `
			SELECT id, picked_at
			FROM order_item_allocations
			WHERE order_item_id = $1 AND released_at IS NULL
		`
		arows, err := tx.Query(ctx, queryAllocs, it.id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch allocations: %w", err)
		}

		var allocCount int
		var pickedAllocCount int
		for arows.Next() {
			var aID uuid.UUID
			var pickedAt *time.Time
			if err := arows.Scan(&aID, &pickedAt); err != nil {
				arows.Close()
				return nil, err
			}
			allocCount++
			if pickedAt != nil {
				pickedAllocCount++
			}
		}
		arows.Close()

		if allocCount == it.quantity {
			// Serialized item: all active allocations must be picked
			if pickedAllocCount != it.quantity {
				return nil, ErrFulfillmentNotFullyPicked
			}
		} else if allocCount == 0 {
			// Legacy item: picked_quantity must equal quantity
			if it.legacyPicked != it.quantity {
				return nil, ErrFulfillmentNotFullyPicked
			}
		} else {
			// Invalid invariant: 0 < allocCount < quantity OR allocCount > quantity
			return nil, ErrInvariantViolation
		}
	}

	// 6. Mutate fulfillment status to 'packed' and set packed_at
	now := time.Now()
	var packedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE order_fulfillments
		SET status = 'packed', packed_at = $1, updated_at = $1
		WHERE id = $2
		RETURNING packed_at
	`, now, fulfillmentID).Scan(&packedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update fulfillment to packed: %w", err)
	}

	return &PackResult{
		FulfillmentID:     fulfillmentID,
		OrderID:           orderID,
		FulfillmentStatus: "packed",
		PackedAt:          packedAt,
	}, nil
}

func (r *Repository) GetPackingQueue(ctx context.Context) ([]PackingQueueItem, error) {
	query := `
		SELECT
			of.id AS fulfillment_id,
			o.id AS order_id,
			COALESCE(o.order_number, SUBSTRING(o.id::text, 1, 8)) AS order_number,
			of.status,
			o.status AS order_status,
			of.created_at,
			(
				SELECT MAX(a.picked_at)
				FROM order_items oi_t
				JOIN order_item_allocations a ON a.order_item_id = oi_t.id AND a.released_at IS NULL
				WHERE oi_t.order_fulfillment_id = of.id
			) AS picking_completed_at,
			COUNT(oi.id) AS items_count,
			COALESCE(SUM(oi.quantity), 0) AS total_quantity,
			COALESCE(SUM(
				CASE
					WHEN (SELECT COUNT(*) FROM order_item_allocations a2 WHERE a2.order_item_id = oi.id AND a2.released_at IS NULL) = oi.quantity
					THEN (SELECT COUNT(*) FROM order_item_allocations a3 WHERE a3.order_item_id = oi.id AND a3.released_at IS NULL AND a3.picked_at IS NOT NULL)
					ELSE oi.picked_quantity
				END
			), 0) AS picked_quantity
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		JOIN order_items oi ON oi.order_fulfillment_id = of.id
		WHERE of.status = 'assembling'
		  AND o.status = 'assembling'
		  AND NOT EXISTS (
			  SELECT 1
			  FROM order_items oi2
			  WHERE oi2.order_fulfillment_id = of.id
				AND oi2.quantity > 0
				AND (
					-- Serialized: active allocations == quantity, but at least one allocation is unpicked
					(
						(SELECT COUNT(*) FROM order_item_allocations a_sub WHERE a_sub.order_item_id = oi2.id AND a_sub.released_at IS NULL) = oi2.quantity
						AND EXISTS (
							SELECT 1
							FROM order_item_allocations a_unpicked
							WHERE a_unpicked.order_item_id = oi2.id
							  AND a_unpicked.released_at IS NULL
							  AND a_unpicked.picked_at IS NULL
						)
					)
					OR
					-- Legacy: no active allocations, and picked_quantity < quantity
					(
						NOT EXISTS (
							SELECT 1
							FROM order_item_allocations a_none
							WHERE a_none.order_item_id = oi2.id
							  AND a_none.released_at IS NULL
						)
						AND oi2.picked_quantity < oi2.quantity
					)
					OR
					-- Invariant failure: active allocations count > 0 but != quantity
					(
						(SELECT COUNT(*) FROM order_item_allocations a_inv WHERE a_inv.order_item_id = oi2.id AND a_inv.released_at IS NULL) > 0
						AND (SELECT COUNT(*) FROM order_item_allocations a_inv WHERE a_inv.order_item_id = oi2.id AND a_inv.released_at IS NULL) != oi2.quantity
					)
				)
		  )
		GROUP BY of.id, o.id, o.order_number, of.status, o.status, of.created_at
		HAVING COALESCE(SUM(oi.quantity), 0) > 0
		ORDER BY of.created_at ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query packing queue: %w", err)
	}
	defer rows.Close()

	var items []PackingQueueItem
	for rows.Next() {
		var item PackingQueueItem
		if err := rows.Scan(
			&item.FulfillmentID,
			&item.OrderID,
			&item.OrderNumber,
			&item.Status,
			&item.OrderStatus,
			&item.CreatedAt,
			&item.PickingCompletedAt,
			&item.ItemsCount,
			&item.TotalQuantity,
			&item.PickedQuantity,
		); err != nil {
			return nil, fmt.Errorf("failed to scan packing queue item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []PackingQueueItem{}
	}
	return items, nil
}
