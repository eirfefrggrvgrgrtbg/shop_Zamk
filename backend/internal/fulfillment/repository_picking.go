package fulfillment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetPickingOrder(ctx context.Context, fulfillmentID uuid.UUID) (*PickingOrder, error) {
	var po PickingOrder
	var orderNumber *string

	// 1. Fetch order & fulfillment basic details
	queryHeader := `
		SELECT o.id, o.status, of.id, of.status
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.id = $1
	`
	err := r.db.QueryRow(ctx, queryHeader, fulfillmentID).Scan(
		&po.OrderID, &po.OrderStatus, &po.FulfillmentID, &po.FulfillmentStatus,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to fetch picking order header: %w", err)
	}
	po.OrderNumber = orderNumber // Not stored in legacy orders table without snapshots; leave nil or add later

	// 2. Validate business rules for eligibility
	if po.OrderStatus == "awaiting_payment" {
		return nil, ErrPickingNotAllowed
	}
	if po.FulfillmentStatus != "paid" && po.FulfillmentStatus != "assembling" {
		// Could be packed, shipped, etc. We just strictly allow paid and assembling.
		return nil, ErrPickingNotAllowed
	}

	// 3. Fetch items and their exact allocation counts
	queryItems := `
		SELECT id, product_variant_id, title, quantity, picked_quantity
		FROM order_items
		WHERE order_fulfillment_id = $1
		ORDER BY created_at ASC, id ASC
	`
	rows, err := r.db.Query(ctx, queryItems, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order items: %w", err)
	}
	defer rows.Close()

	var orderItems []struct {
		id uuid.UUID
		variantID uuid.UUID
		title string
		quantity int
		legacyPicked int
	}
	for rows.Next() {
		var item struct {
			id uuid.UUID
			variantID uuid.UUID
			title string
			quantity int
			legacyPicked int
		}
		if err := rows.Scan(&item.id, &item.variantID, &item.title, &item.quantity, &item.legacyPicked); err != nil {
			return nil, err
		}
		orderItems = append(orderItems, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, item := range orderItems {
		queryAllocs := `
			SELECT a.inventory_unit_id, u.unit_code, a.picked_at
			FROM order_item_allocations a
			JOIN inventory_units u ON u.id = a.inventory_unit_id
			WHERE a.order_item_id = $1 AND a.released_at IS NULL
		`
		arows, err := r.db.Query(ctx, queryAllocs, item.id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch allocations: %w", err)
		}

		var allocs []PickingAllocatedUnit
		var serializedPicked int
		for arows.Next() {
			var a PickingAllocatedUnit
			if err := arows.Scan(&a.InventoryUnitID, &a.UnitCode, &a.PickedAt); err != nil {
				arows.Close()
				return nil, err
			}
			allocs = append(allocs, a)
			if a.PickedAt != nil {
				serializedPicked++
			}
		}
		arows.Close()

		aCount := len(allocs)
		qCount := item.quantity

		pi := PickingItem{
			OrderItemID:      item.id,
			Title:            item.title,
			ProductVariantID: item.variantID,
			Quantity:         qCount,
			AllocatedUnits:   allocs, // will be empty for legacy
		}

		// Classification
		if aCount == qCount && qCount > 0 {
			pi.AllocationMode = "serialized"
			pi.PickedQuantity = serializedPicked
			pi.RemainingQuantity = qCount - serializedPicked
		} else if aCount == 0 {
			pi.AllocationMode = "legacy"
			pi.PickedQuantity = item.legacyPicked
			pi.RemainingQuantity = qCount - item.legacyPicked
		} else {
			// INVALID (e.g. partial allocation)
			return nil, ErrInvariantViolation
		}

		po.Items = append(po.Items, pi)
	}

	return &po, nil
}
