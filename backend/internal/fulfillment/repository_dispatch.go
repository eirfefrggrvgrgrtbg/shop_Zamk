package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DispatchResult struct {
	FulfillmentID     uuid.UUID `json:"fulfillmentId"`
	OrderID           uuid.UUID `json:"orderId"`
	ShipmentID        uuid.UUID `json:"shipmentId"`
	FulfillmentStatus string    `json:"fulfillmentStatus"`
	OrderStatus       string    `json:"orderStatus"`
	ShipmentStatus    string    `json:"shipmentStatus"`
	ShippedAt         time.Time `json:"shippedAt"`
}

func (r *Repository) DispatchFulfillmentTx(ctx context.Context, tx pgx.Tx, adminID, fulfillmentID uuid.UUID) (*DispatchResult, error) {
	// 1. Lock fulfillment and parent order
	var orderID uuid.UUID
	var orderStatus string
	var fulfillmentStatus string

	queryHeader := `
		SELECT o.id, o.status, of.status
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.id = $1
		FOR UPDATE OF of, o
	`
	err := tx.QueryRow(ctx, queryHeader, fulfillmentID).Scan(&orderID, &orderStatus, &fulfillmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to lock fulfillment and order for dispatch: %w", err)
	}

	// 2. Validate fulfillment status: MUST be "packed"
	if fulfillmentStatus != "packed" {
		return nil, ErrDispatchNotAllowed
	}

	// 3. Validate parent order status: MUST be coherent active state ("assembling", "packed")
	// Parent order MUST NOT be "shipped" while this fulfillment is still packed.
	if orderStatus != "assembling" && orderStatus != "packed" {
		return nil, ErrDispatchNotAllowed
	}

	// 4. Lock existing shipment row if present and validate against pre-dispatch allowlist
	var existingShipmentID *uuid.UUID
	var existingShipmentStatus *string

	validPreDispatchShipmentStates := map[string]bool{
		"pending":    true,
		"assembling": true,
		"packed":     true,
	}

	queryShipment := `
		SELECT id, status, shipped_at
		FROM shipments
		WHERE fulfillment_id = $1
		FOR UPDATE
	`
	var sID uuid.UUID
	var sStatus string
	var sShippedAt *time.Time
	err = tx.QueryRow(ctx, queryShipment, fulfillmentID).Scan(&sID, &sStatus, &sShippedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing shipment: %w", err)
	}
	if err == nil {
		existingShipmentID = &sID
		existingShipmentStatus = &sStatus

		if !validPreDispatchShipmentStates[sStatus] {
			return nil, ErrShipmentContradictoryState
		}
	}

	// 5. Query order items for this fulfillment
	queryItems := `
		SELECT id, product_variant_id, quantity, picked_quantity
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
		id               uuid.UUID
		productVariantID uuid.UUID
		quantity         int
		legacyPicked     int
	}
	var items []itemInfo
	for rows.Next() {
		var it itemInfo
		if err := rows.Scan(&it.id, &it.productVariantID, &it.quantity, &it.legacyPicked); err != nil {
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

	// 6. Lock and load active order item allocations in deterministic order
	type allocInfo struct {
		allocID     uuid.UUID
		orderItemID uuid.UUID
		unitID      uuid.UUID
		pickedAt    *time.Time
		uStatus     string
		uVarID      uuid.UUID
	}

	queryAllocs := `
		SELECT a.id, a.order_item_id, a.inventory_unit_id, a.picked_at, u.status, u.product_variant_id
		FROM order_item_allocations a
		JOIN inventory_units u ON u.id = a.inventory_unit_id
		JOIN order_items oi ON oi.id = a.order_item_id
		WHERE oi.order_fulfillment_id = $1 AND a.released_at IS NULL
		ORDER BY a.id ASC
		FOR UPDATE OF a
	`
	arows, err := tx.Query(ctx, queryAllocs, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to lock allocations: %w", err)
	}
	defer arows.Close()

	allocsByItem := make(map[uuid.UUID][]allocInfo)
	for arows.Next() {
		var a allocInfo
		if err := arows.Scan(&a.allocID, &a.orderItemID, &a.unitID, &a.pickedAt, &a.uStatus, &a.uVarID); err != nil {
			return nil, err
		}
		allocsByItem[a.orderItemID] = append(allocsByItem[a.orderItemID], a)
	}
	if err := arows.Err(); err != nil {
		return nil, err
	}

	// 7. Verify every item satisfies dispatch rules and aggregate quantities
	variantQuantities := make(map[uuid.UUID]int)
	var serializedUnitIDs []uuid.UUID
	expectedUnitVariants := make(map[uuid.UUID]uuid.UUID)

	for _, it := range items {
		if it.quantity <= 0 {
			return nil, ErrInvariantViolation
		}

		itemAllocs := allocsByItem[it.id]
		allocCount := len(itemAllocs)

		if allocCount == it.quantity {
			// Serialized item
			for _, a := range itemAllocs {
				if a.pickedAt == nil {
					return nil, ErrFulfillmentNotFullyPicked
				}
				if a.uStatus != "warehouse" {
					return nil, ErrInventoryUnitStateConflict
				}
				if a.uVarID != it.productVariantID {
					return nil, ErrInvariantViolation
				}
				serializedUnitIDs = append(serializedUnitIDs, a.unitID)
				expectedUnitVariants[a.unitID] = it.productVariantID
			}
			variantQuantities[it.productVariantID] += it.quantity
		} else if allocCount == 0 {
			// Legacy item
			if it.legacyPicked != it.quantity {
				return nil, ErrFulfillmentNotFullyPicked
			}
			variantQuantities[it.productVariantID] += it.quantity
		} else {
			// Invalid invariant: 0 < allocCount < quantity OR allocCount > quantity
			return nil, ErrInvariantViolation
		}
	}

	// 7. Lock and decrement inventory_items deterministically
	var variantIDs []uuid.UUID
	for vID := range variantQuantities {
		variantIDs = append(variantIDs, vID)
	}
	sort.Slice(variantIDs, func(i, j int) bool {
		return variantIDs[i].String() < variantIDs[j].String()
	})

	for _, vID := range variantIDs {
		shippedQty := variantQuantities[vID]

		var itemID uuid.UUID
		var totalStock int
		var reservedStock int

		queryLockItem := `
			SELECT id, total_stock, reserved_stock
			FROM inventory_items
			WHERE product_variant_id = $1
			FOR UPDATE
		`
		err := tx.QueryRow(ctx, queryLockItem, vID).Scan(&itemID, &totalStock, &reservedStock)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInventoryItemNotFound
			}
			return nil, fmt.Errorf("failed to lock inventory item: %w", err)
		}

		if totalStock < shippedQty {
			return nil, ErrInsufficientTotalStock
		}
		if reservedStock < shippedQty {
			return nil, ErrInsufficientReservedStock
		}

		queryUpdateStock := `
			UPDATE inventory_items
			SET total_stock = total_stock - $1,
			    reserved_stock = reserved_stock - $1,
			    updated_at = now()
			WHERE id = $2
		`
		_, err = tx.Exec(ctx, queryUpdateStock, shippedQty, itemID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrement inventory stock: %w", err)
		}
	}

	// 8. Lock and update inventory_units deterministically
	sort.Slice(serializedUnitIDs, func(i, j int) bool {
		return serializedUnitIDs[i].String() < serializedUnitIDs[j].String()
	})

	for _, uID := range serializedUnitIDs {
		var uStatus string
		var uVarID uuid.UUID

		queryLockUnit := `
			SELECT status, product_variant_id
			FROM inventory_units
			WHERE id = $1
			FOR UPDATE
		`
		err := tx.QueryRow(ctx, queryLockUnit, uID).Scan(&uStatus, &uVarID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInventoryUnitStateConflict
			}
			return nil, fmt.Errorf("failed to lock inventory unit: %w", err)
		}

		if uStatus != "warehouse" {
			return nil, ErrInventoryUnitStateConflict
		}
		if uVarID != expectedUnitVariants[uID] {
			return nil, ErrInvariantViolation
		}

		queryUpdateUnit := `
			UPDATE inventory_units
			SET status = 'shipped', updated_at = now()
			WHERE id = $1
		`
		_, err = tx.Exec(ctx, queryUpdateUnit, uID)
		if err != nil {
			return nil, fmt.Errorf("failed to update inventory unit status: %w", err)
		}
	}

	// 9. Mutate or create shipment record and record shipment event
	now := time.Now()
	var targetShipmentID uuid.UUID
	comment := "warehouse dispatch"

	if existingShipmentID != nil {
		targetShipmentID = *existingShipmentID
		queryUpdateShipment := `
			UPDATE shipments
			SET status = 'shipped', shipped_at = COALESCE(shipped_at, $1), updated_at = $1
			WHERE id = $2
		`
		_, err = tx.Exec(ctx, queryUpdateShipment, now, targetShipmentID)
		if err != nil {
			return nil, fmt.Errorf("failed to update shipment: %w", err)
		}

		event := &ShipmentEvent{
			ID:          uuid.New(),
			ShipmentID:  targetShipmentID,
			FromStatus:  existingShipmentStatus,
			ToStatus:    "shipped",
			ActorUserID: &adminID,
			Comment:     &comment,
		}
		if err := r.CreateShipmentEventTx(ctx, tx, event); err != nil {
			return nil, fmt.Errorf("failed to create shipment event: %w", err)
		}
	} else {
		targetShipmentID = uuid.New()
		queryCreateShipment := `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'shipped', NULL, NULL, NULL, $4, $4, $4)
		`
		_, err = tx.Exec(ctx, queryCreateShipment, targetShipmentID, orderID, fulfillmentID, now)
		if err != nil {
			return nil, fmt.Errorf("failed to create shipment: %w", err)
		}

		event := &ShipmentEvent{
			ID:          uuid.New(),
			ShipmentID:  targetShipmentID,
			FromStatus:  nil,
			ToStatus:    "shipped",
			ActorUserID: &adminID,
			Comment:     &comment,
		}
		if err := r.CreateShipmentEventTx(ctx, tx, event); err != nil {
			return nil, fmt.Errorf("failed to create shipment event: %w", err)
		}
	}

	// 10. Mutate fulfillment status to 'shipped'
	queryUpdateFulfillment := `
		UPDATE order_fulfillments
		SET status = 'shipped', updated_at = $1
		WHERE id = $2
	`
	_, err = tx.Exec(ctx, queryUpdateFulfillment, now, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to update fulfillment to shipped: %w", err)
	}

	return &DispatchResult{
		FulfillmentID:     fulfillmentID,
		OrderID:           orderID,
		ShipmentID:        targetShipmentID,
		FulfillmentStatus: "shipped",
		ShipmentStatus:    "shipped",
		ShippedAt:         now,
	}, nil
}
