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
