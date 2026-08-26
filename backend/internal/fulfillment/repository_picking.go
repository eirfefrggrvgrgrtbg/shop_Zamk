package fulfillmen

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetPickingOrderTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) (*PickingOrder, error) {
	var po PickingOrder
	var orderNumber *string

	// 1. Fetch order & fulfillment basic details
	queryHeader := `
		SELECT o.id, o.status, of.id, of.status
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.id = $1
	`
	err := tx.QueryRow(ctx, queryHeader, fulfillmentID).Scan(
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
	rows, err := tx.Query(ctx, queryItems, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order items: %w", err)
	}
	defer rows.Close()

	var orderItems []struct {
		id           uuid.UUID
		variantID    uuid.UUID
		title        string
		quantity     in
		legacyPicked in
	}
	for rows.Next() {
		var item struct {
			id           uuid.UUID
			variantID    uuid.UUID
			title        string
			quantity     in
			legacyPicked in
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
			SELECT a.inventory_unit_id, u.unit_code, a.picked_a
			FROM order_item_allocations a
			JOIN inventory_units u ON u.id = a.inventory_unit_id
			WHERE a.order_item_id = $1 AND a.released_at IS NULL
		`
		arows, err := tx.Query(ctx, queryAllocs, item.id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch allocations: %w", err)
		}

		var allocs []PickingAllocatedUni
		var serializedPicked in
		for arows.Next() {
			var a PickingAllocatedUni
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

func (r *Repository) ScanPickingCodeTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, code string) (*PickingScanResult, error) {
	// 1. Lock fulfillment and check eligibility
	var po PickingOrder
	err := tx.QueryRow(ctx, `
		SELECT o.id, o.status, of.id, of.status
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.id = $1
		FOR UPDATE OF of
	`, fulfillmentID).Scan(&po.OrderID, &po.OrderStatus, &po.FulfillmentID, &po.FulfillmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to lock fulfillment: %w", err)
	}

	if po.FulfillmentStatus == "awaiting_payment" || po.OrderStatus == "awaiting_payment" {
		return nil, ErrPickingNotAllowed
	}
	if po.FulfillmentStatus != "paid" && po.FulfillmentStatus != "assembling" {
		return nil, ErrPickingNotAllowed
	}

	res := &PickingScanResult{
		FulfillmentID: po.FulfillmentID,
		OrderID:       po.OrderID,
		ScanResult: PickingScanDetail{
			Code: code,
		},
	}

	// 2. Try ZMU
	var unitID uuid.UUID
	var unitStatus string
	err = tx.QueryRow(ctx, `SELECT id, status FROM inventory_units WHERE unit_code = $1`, code).Scan(&unitID, &unitStatus)
	if err == nil {
		// It IS a ZMU
		if unitStatus != "warehouse" {
			return nil, ErrUnitNotInWarehouse
		}

		// Find its active allocation
		var allocID uuid.UUID
		var allocOrderItemID uuid.UUID
		var allocFulfillmentID uuid.UUID
		var pickedAt *time.Time
		err = tx.QueryRow(ctx, `
			SELECT a.id, a.order_item_id, a.picked_at, oi.order_fulfillment_id
			FROM order_item_allocations a
			JOIN order_items oi ON oi.id = a.order_item_id
			WHERE a.inventory_unit_id = $1 AND a.released_at IS NULL
			FOR UPDATE OF a
		`, unitID).Scan(&allocID, &allocOrderItemID, &pickedAt, &allocFulfillmentID)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrUnitNotAllocatedToFulfillmen
			}
			return nil, fmt.Errorf("failed to check unit allocation: %w", err)
		}

		if allocFulfillmentID != fulfillmentID {
			return nil, ErrUnitAllocatedToOtherOrder
		}

		res.ScanResult.Type = "serialized"
		res.ScanResult.OrderItemID = allocOrderItemID

		if pickedAt != nil {
			res.ScanResult.AlreadyPicked = true
		} else {
			_, err = tx.Exec(ctx, `UPDATE order_item_allocations SET picked_at = now() WHERE id = $1`, allocID)
			if err != nil {
				return nil, fmt.Errorf("failed to pick unit: %w", err)
			}
			res.ScanResult.NewlyPicked = true
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		// 3. Not a ZMU -> Legacy barcode
		// Find order items in this fulfillment matching the barcode
		queryLegacy := `
			SELECT oi.id, oi.quantity, oi.picked_quantity,
			       (SELECT count(*) FROM order_item_allocations a WHERE a.order_item_id = oi.id AND a.released_at IS NULL) as alloc_coun
			FROM order_items oi
			JOIN product_variants pv ON pv.id = oi.product_variant_id
			WHERE oi.order_fulfillment_id = $1
			  AND (pv.barcode = $2 OR pv.sku = $2 OR pv.seller_sku = $2)
			FOR UPDATE OF oi
		`
		rows, err := tx.Query(ctx, queryLegacy, fulfillmentID, code)
		if err != nil {
			return nil, fmt.Errorf("failed to query legacy barcode: %w", err)
		}

		type match struct {
			id       uuid.UUID
			quantity in
			picked   in
			allocs   in
		}
		var matches []match
		for rows.Next() {
			var m match
			if err := rows.Scan(&m.id, &m.quantity, &m.picked, &m.allocs); err != nil {
				rows.Close()
				return nil, err
			}
			matches = append(matches, m)
		}
		rows.Close()

		if len(matches) == 0 {
			return nil, ErrCodeNotFound
		}
		if len(matches) > 1 {
			return nil, ErrAmbiguousPickingCode
		}

		m := matches[0]
		res.ScanResult.OrderItemID = m.id

		// If it's a serialized item, we can't pick it with a legacy barcode
		if m.allocs > 0 {
			return nil, ErrCannotPickSerializedWithBarcode
		}

		res.ScanResult.Type = "legacy"

		if m.picked >= m.quantity {
			res.ScanResult.AlreadyComplete = true
		} else {
			cmdTag, err := tx.Exec(ctx, `
				UPDATE order_items
				SET picked_quantity = picked_quantity + 1
				WHERE id = $1 AND picked_quantity < quantity
			`, m.id)
			if err != nil {
				return nil, fmt.Errorf("failed to increment legacy pick: %w", err)
			}
			if cmdTag.RowsAffected() == 1 {
				res.ScanResult.NewlyPicked = true
			} else {
				// E.g. concurrent update finished i
				res.ScanResult.AlreadyComplete = true
			}
		}
	} else {
		return nil, fmt.Errorf("failed to query inventory_units: %w", err)
	}

	// 4. Calculate progress
	po2, err := r.GetPickingOrderTx(ctx, tx, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get picking order progress: %w", err)
	}

	totalQ := 0
	pickedQ := 0
	isComplete := true

	for _, it := range po2.Items {
		totalQ += it.Quantity
		pickedQ += it.PickedQuantity
		if it.PickedQuantity < it.Quantity {
			isComplete = false
		}

		if it.OrderItemID == res.ScanResult.OrderItemID {
			res.Item = PickingItemState{
				Quantity:          it.Quantity,
				PickedQuantity:    it.PickedQuantity,
				RemainingQuantity: it.RemainingQuantity,
				AllocationMode:    it.AllocationMode,
			}
		}
	}

	res.FulfillmentProgress = PickingProgress{
		TotalQuantity:     totalQ,
		PickedQuantity:    pickedQ,
		RemainingQuantity: totalQ - pickedQ,
		IsComplete:        isComplete,
	}

	return res, nil
}

func (r *Repository) GetPickingOrder(ctx context.Context, fulfillmentID uuid.UUID) (*PickingOrder, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	res, err := r.GetPickingOrderTx(ctx, tx, fulfillmentID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}
