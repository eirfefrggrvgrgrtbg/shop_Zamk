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
		return nil, fmt.Errorf("failed to lock parent order for dispatch: %w", err)
	}

	// 3. Lock fulfillment SECOND (consistent lock order: orders -> order_fulfillments)
	var fulfillmentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1 FOR UPDATE`, fulfillmentID).Scan(&fulfillmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to lock fulfillment for dispatch: %w", err)
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

func (r *Repository) GetDispatchContext(ctx context.Context, fulfillmentID uuid.UUID) (*DispatchContext, error) {
	queryHeader := `
		SELECT
			f.id, f.order_id, f.status, f.packed_at,
			s.status as shipment_status, s.id as shipment_id,
			o.order_number, o.delivery_address, o.customer_name, o.customer_phone, o.delivery_method_name
		FROM order_fulfillments f
		JOIN orders o ON o.id = f.order_id
		LEFT JOIN shipments s ON (s.fulfillment_id = f.id) OR (s.fulfillment_id IS NULL AND s.order_id = f.order_id AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = f.order_id) = 1)
		WHERE f.id = $1
	`

	var dc DispatchContext
	err := r.db.QueryRow(ctx, queryHeader, fulfillmentID).Scan(
		&dc.ID, &dc.OrderID, &dc.Status, &dc.PackedAt,
		&dc.ShipmentStatus, &dc.ShipmentID,
		&dc.OrderNumber, &dc.DeliveryAddress, &dc.CustomerName, &dc.CustomerPhone, &dc.DeliveryMethodName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to fetch dispatch context: %w", err)
	}
	dc.FulfillmentID = dc.ID
	dc.RecipientName = dc.CustomerName
	dc.RecipientPhone = dc.CustomerPhone

	// Query items without financial or commercial data
	queryItems := `
		SELECT oi.id, oi.title, oi.variant_size, oi.variant_color, oi.sku, pv.barcode, oi.quantity
		FROM order_items oi
		LEFT JOIN product_variants pv ON pv.id = oi.product_variant_id
		WHERE oi.order_fulfillment_id = $1
		ORDER BY oi.created_at ASC, oi.id ASC
	`
	rows, err := r.db.Query(ctx, queryItems, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query dispatch items: %w", err)
	}
	defer rows.Close()

	var items []DispatchContextItem
	for rows.Next() {
		var item DispatchContextItem
		if err := rows.Scan(
			&item.OrderItemID, &item.ProductTitle, &item.VariantSize, &item.VariantColor,
			&item.SKU, &item.Barcode, &item.Quantity,
		); err != nil {
			return nil, fmt.Errorf("failed to scan dispatch item: %w", err)
		}
		items = append(items, item)
	}
	rows.Close()

	if items == nil {
		items = make([]DispatchContextItem, 0)
	}

	queryAllocs := `
		SELECT a.inventory_unit_id, u.unit_code, a.picked_at
		FROM order_item_allocations a
		JOIN inventory_units u ON u.id = a.inventory_unit_id
		WHERE a.order_item_id = $1 AND a.released_at IS NULL
		ORDER BY a.created_at ASC, a.id ASC
	`
	for i := range items {
		arows, err := r.db.Query(ctx, queryAllocs, items[i].OrderItemID)
		if err != nil {
			return nil, fmt.Errorf("failed to query dispatch item allocations: %w", err)
		}
		var allocs []DispatchContextAllocatedUnit
		for arows.Next() {
			var a DispatchContextAllocatedUnit
			if err := arows.Scan(&a.InventoryUnitID, &a.UnitCode, &a.PickedAt); err != nil {
				arows.Close()
				return nil, fmt.Errorf("failed to scan dispatch unit: %w", err)
			}
			allocs = append(allocs, a)
		}
		arows.Close()

		if allocs == nil {
			allocs = make([]DispatchContextAllocatedUnit, 0)
		}
		items[i].AllocatedUnits = allocs
		if len(allocs) > 0 {
			items[i].AllocationMode = "serialized"
		} else {
			items[i].AllocationMode = "legacy"
		}
	}

	dc.Items = items
	return &dc, nil
}

func (r *Repository) GetDispatchQueue(ctx context.Context) ([]DispatchQueueItem, error) {
	query := `
		SELECT
			of.id AS fulfillment_id,
			o.id AS order_id,
			COALESCE(o.order_number, SUBSTRING(o.id::text, 1, 8)) AS order_number,
			of.status,
			o.status AS order_status,
			of.packed_at,
			of.created_at,
			o.delivery_method_name,
			COUNT(oi.id) AS items_count,
			COALESCE(SUM(oi.quantity), 0) AS total_quantity,
			s.id AS shipment_id,
			s.status AS shipment_status,
			s.carrier AS shipment_carrier
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		JOIN order_items oi ON oi.order_fulfillment_id = of.id
		LEFT JOIN shipments s ON (
			s.fulfillment_id = of.id
			OR (
				s.fulfillment_id IS NULL
				AND s.order_id = of.order_id
				AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = of.order_id) = 1
			)
		)
		WHERE of.status = 'packed'
		  AND o.status IN ('assembling', 'packed')
		  AND (s.status IS NULL OR s.status IN ('pending', 'assembling', 'packed'))
		GROUP BY of.id, o.id, o.order_number, of.status, o.status, of.packed_at, of.created_at, o.delivery_method_name, s.id, s.status, s.carrier
		HAVING COALESCE(SUM(oi.quantity), 0) > 0
		ORDER BY COALESCE(of.packed_at, of.created_at) ASC, of.created_at ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query dispatch queue: %w", err)
	}
	defer rows.Close()

	var items []DispatchQueueItem
	for rows.Next() {
		var item DispatchQueueItem
		if err := rows.Scan(
			&item.FulfillmentID,
			&item.OrderID,
			&item.OrderNumber,
			&item.Status,
			&item.OrderStatus,
			&item.PackedAt,
			&item.CreatedAt,
			&item.DeliveryMethodName,
			&item.ItemsCount,
			&item.TotalQuantity,
			&item.ShipmentID,
			&item.ShipmentStatus,
			&item.Carrier,
		); err != nil {
			return nil, fmt.Errorf("failed to scan dispatch queue item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []DispatchQueueItem{}
	}
	return items, nil
}
