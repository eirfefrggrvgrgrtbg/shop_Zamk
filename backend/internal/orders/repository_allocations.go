package orders

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInsufficientWarehouseUnits = errors.New("insufficient eligible warehouse units for allocation")
	ErrOrderItemNotFound          = errors.New("order item not found")
	ErrInvalidAllocationQuantity  = errors.New("allocation quantity must be greater than zero")
)

type OrderItemAllocation struct {
	ID              uuid.UUID  `json:"id"`
	OrderItemID     uuid.UUID  `json:"order_item_id"`
	InventoryUnitID uuid.UUID  `json:"inventory_unit_id"`
	ReservationID   *uuid.UUID `json:"reservation_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ReleasedAt      *time.Time `json:"released_at,omitempty"`
	ReleaseReason   *string    `json:"release_reason,omitempty"`
}

// AllocateUnitsForOrderItem looks up the authoritative product_variant_id from order_items
// and allocates exactly quantity physical ZMU warehouse units in an atomic transaction.
func (r *Repository) AllocateUnitsForOrderItem(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID, quantity int, reservationID *uuid.UUID) ([]uuid.UUID, error) {
	if quantity <= 0 {
		return nil, ErrInvalidAllocationQuantity
	}

	// 1. Authoritative lookup and row lock on order_item
	var variantID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT product_variant_id
		FROM order_items
		WHERE id = $1
		FOR UPDATE
	`, orderItemID).Scan(&variantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderItemNotFound
		}
		return nil, fmt.Errorf("failed to fetch order item: %w", err)
	}

	// 2. Select and lock exactly N eligible units for this variant
	// Eligibility:
	// - product_variant_id matches order_item.product_variant_id
	// - status = 'warehouse'
	// - no active allocation in order_item_allocations (released_at IS NULL)
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
		ORDER BY u.created_at ASC, u.id ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(ctx, queryEligible, variantID, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to query eligible warehouse units: %w", err)
	}
	defer rows.Close()

	var unitIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan unit id: %w", err)
		}
		unitIDs = append(unitIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating units: %w", err)
	}

	// 3. Exact quantity all-or-nothing check
	if len(unitIDs) < quantity {
		return nil, ErrInsufficientWarehouseUnits
	}

	// 4. Insert allocation records
	now := time.Now()
	for _, unitID := range unitIDs {
		allocID := uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, allocID, orderItemID, unitID, reservationID, now)
		if err != nil {
			return nil, fmt.Errorf("failed to insert order item allocation: %w", err)
		}
	}

	return unitIDs, nil
}

// ListActiveAllocationsForOrderItem returns active unit allocations for the given order item
func (r *Repository) ListActiveAllocationsForOrderItem(ctx context.Context, orderItemID uuid.UUID) ([]OrderItemAllocation, error) {
	query := `
		SELECT id, order_item_id, inventory_unit_id, reservation_id, created_at, released_at, release_reason
		FROM order_item_allocations
		WHERE order_item_id = $1 AND released_at IS NULL
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, orderItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []OrderItemAllocation
	for rows.Next() {
		var a OrderItemAllocation
		if err := rows.Scan(&a.ID, &a.OrderItemID, &a.InventoryUnitID, &a.ReservationID, &a.CreatedAt, &a.ReleasedAt, &a.ReleaseReason); err != nil {
			return nil, err
		}
		allocs = append(allocs, a)
	}
	return allocs, rows.Err()
}

// ListAllAllocationsForOrderItem returns all unit allocations (active and historical) for the given order item
func (r *Repository) ListAllAllocationsForOrderItem(ctx context.Context, orderItemID uuid.UUID) ([]OrderItemAllocation, error) {
	query := `
		SELECT id, order_item_id, inventory_unit_id, reservation_id, created_at, released_at, release_reason
		FROM order_item_allocations
		WHERE order_item_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, orderItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []OrderItemAllocation
	for rows.Next() {
		var a OrderItemAllocation
		if err := rows.Scan(&a.ID, &a.OrderItemID, &a.InventoryUnitID, &a.ReservationID, &a.CreatedAt, &a.ReleasedAt, &a.ReleaseReason); err != nil {
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

// ReleaseAllocationsForReservation releases all active allocations for a given reservation ID
func (r *Repository) ReleaseAllocationsForReservation(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID, reason string) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE order_item_allocations
		SET released_at = $1, release_reason = $2
		WHERE reservation_id = $3 AND released_at IS NULL
	`, now, reason, reservationID)
	return err
}
