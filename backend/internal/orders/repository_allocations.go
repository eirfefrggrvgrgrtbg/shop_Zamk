package orders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInsufficientWarehouseUnits = errors.New("insufficient eligible warehouse units for allocation")

type OrderItemAllocation struct {
	ID              uuid.UUID
	OrderItemID     uuid.UUID
	InventoryUnitID uuid.UUID
	ReservationID   *uuid.UUID
	CreatedAt       time.Time
	ReleasedAt      *time.Time
	ReleaseReason   *string
}

// AllocateUnitsForOrderItem allocates exactly N physical ZMU units to an order item.
func (r *Repository) AllocateUnitsForOrderItem(ctx context.Context, tx pgx.Tx, orderItemID, variantID uuid.UUID, quantity int, reservationID *uuid.UUID) ([]uuid.UUID, error) {
	if quantity <= 0 {
		return nil, nil
	}

	// Lock exactly N eligible physical units for this variant
	// Eligibility: status = 'warehouse' and not currently active in order_item_allocations
	queryEligible := `
		SELECT u.id
		FROM inventory_units u
		WHERE u.product_variant_id = $1
		  AND u.status = 'warehouse'
		  AND NOT EXISTS (
			  SELECT 1 FROM order_item_allocations a
			  WHERE a.inventory_unit_id = u.id
			    AND a.released_at IS NULL
		  )
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(ctx, queryEligible, variantID, quantity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var unitIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		unitIDs = append(unitIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(unitIDs) < quantity {
		return nil, ErrInsufficientWarehouseUnits
	}

	// Insert allocations
	now := time.Now()
	for _, unitID := range unitIDs {
		allocID := uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, allocID, orderItemID, unitID, reservationID, now)
		if err != nil {
			return nil, err
		}
	}

	return unitIDs, nil
}

// ListActiveAllocationsForOrderItem returns active unit allocations for the given order item
func (r *Repository) ListActiveAllocationsForOrderItem(ctx context.Context, orderItemID uuid.UUID) ([]OrderItemAllocation, error) {
	query := `
		SELECT id, order_item_id, inventory_unit_id, reservation_id, created_at
		FROM order_item_allocations
		WHERE order_item_id = $1 AND released_at IS NULL
	`
	rows, err := r.db.Query(ctx, query, orderItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []OrderItemAllocation
	for rows.Next() {
		var a OrderItemAllocation
		if err := rows.Scan(&a.ID, &a.OrderItemID, &a.InventoryUnitID, &a.ReservationID, &a.CreatedAt); err != nil {
			return nil, err
		}
		allocs = append(allocs, a)
	}
	return allocs, rows.Err()
}

// ReleaseAllocationsForOrderItem releases all active allocations for a given order item
func (r *Repository) ReleaseAllocationsForOrderItem(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID, reason string) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE order_item_allocations
		SET released_at = $1, release_reason = $2
		WHERE order_item_id = $3 AND released_at IS NULL
	`, now, reason, orderItemID)
	return err
}

// ReleaseAllocationsForOrder releases all active allocations for a given order ID
func (r *Repository) ReleaseAllocationsForOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, reason string) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE order_item_allocations a
		SET released_at = $1, release_reason = $2
		FROM order_items oi
		WHERE a.order_item_id = oi.id
		  AND oi.order_id = $3
		  AND a.released_at IS NULL
	`, now, reason, orderID)
	return err
}
