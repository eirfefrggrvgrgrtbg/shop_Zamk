package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func isPlaceholderBarcode(b string) bool {
	trimmed := strings.TrimSpace(b)
	if trimmed == "" {
		return true
	}
	for _, r := range trimmed {
		if r != '0' {
			return false
		}
	}
	return true
}

func (r *Repository) GetPickingOrderTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) (*PickingOrder, error) {
	var po PickingOrder

	// 1. Fetch order & fulfillment basic details
	queryHeader := `
		SELECT o.id, o.status, of.id, of.status, o.order_number
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.id = $1
	`
	err := tx.QueryRow(ctx, queryHeader, fulfillmentID).Scan(
		&po.OrderID, &po.OrderStatus, &po.FulfillmentID, &po.FulfillmentStatus, &po.OrderNumber,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to fetch picking order header: %w", err)
	}

	// 2. Validate business rules for eligibility
	if (po.OrderStatus != "paid" && po.OrderStatus != "assembling") ||
		(po.FulfillmentStatus != "paid" && po.FulfillmentStatus != "assembling") {
		return nil, ErrPickingNotAllowed
	}

	// 3. Fetch items and their exact allocation counts
	queryItems := `
		SELECT oi.id, oi.product_variant_id, oi.title, oi.quantity, oi.picked_quantity,
		       oi.variant_size, oi.variant_color, oi.image_url, oi.sku, pv.barcode
		FROM order_items oi
		LEFT JOIN product_variants pv ON pv.id = oi.product_variant_id
		WHERE oi.order_fulfillment_id = $1
		ORDER BY oi.created_at ASC, oi.id ASC
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
		quantity     int
		legacyPicked int
		variantSize  *string
		variantColor *string
		imageURL     *string
		sku          *string
		barcode      *string
	}
	for rows.Next() {
		var item struct {
			id           uuid.UUID
			variantID    uuid.UUID
			title        string
			quantity     int
			legacyPicked int
			variantSize  *string
			variantColor *string
			imageURL     *string
			sku          *string
			barcode      *string
		}
		if err := rows.Scan(
			&item.id, &item.variantID, &item.title, &item.quantity, &item.legacyPicked,
			&item.variantSize, &item.variantColor, &item.imageURL, &item.sku, &item.barcode,
		); err != nil {
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
		arows, err := tx.Query(ctx, queryAllocs, item.id)
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

		barcode := item.barcode
		if barcode != nil && isPlaceholderBarcode(*barcode) {
			barcode = nil
		}

		pi := PickingItem{
			OrderItemID:      item.id,
			Title:            item.title,
			ProductVariantID: item.variantID,
			VariantSize:      item.variantSize,
			VariantColor:     item.variantColor,
			ImageURL:         item.imageURL,
			SKU:              item.sku,
			Barcode:          barcode,
			Quantity:         qCount,
			AllocatedUnits:   allocs, // will be empty for legacy
		}

		// Classification
		if aCount == qCount && qCount > 0 {
			pi.AllocationMode = "serialized"
			pi.PickedQuantity = serializedPicked
			pi.RemainingQuantity = qCount - serializedPicked

			// Calculate compatible units count (active unpicked + free warehouse units)
			if pi.RemainingQuantity > 0 {
				queryCompatCount := `
					SELECT COUNT(*)
					FROM inventory_units u
					LEFT JOIN order_item_allocations a ON a.inventory_unit_id = u.id
					    AND a.order_item_id = $1
					    AND a.released_at IS NULL
					WHERE u.product_variant_id = $2
					  AND u.status = 'warehouse'
					  AND (
					      (a.id IS NOT NULL AND a.picked_at IS NULL)
					      OR (
					          a.id IS NULL
					          AND NOT EXISTS (
					              SELECT 1 FROM order_item_allocations other_a
					              JOIN order_items other_oi ON other_oi.id = other_a.order_item_id
					              JOIN orders other_o ON other_o.id = other_oi.order_id
					              LEFT JOIN order_fulfillments other_f ON other_f.id = other_oi.order_fulfillment_id
					              WHERE other_a.inventory_unit_id = u.id
					                AND other_a.released_at IS NULL
					                AND other_o.status NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					                AND COALESCE(other_f.status, '') NOT IN ('delivered', 'cancelled')
					          )
					      )
					  )
				`
				var cCount int
				if err := tx.QueryRow(ctx, queryCompatCount, item.id, item.variantID).Scan(&cCount); err == nil {
					pi.CompatibleUnitsCount = cCount
				}
			} else {
				pi.CompatibleUnitsCount = 0
			}
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

func (r *Repository) ScanPickingCodeTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, code string, targetOrderItemID *uuid.UUID) (*PickingScanResult, error) {
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

	if (po.OrderStatus != "paid" && po.OrderStatus != "assembling") ||
		(po.FulfillmentStatus != "paid" && po.FulfillmentStatus != "assembling") {
		return nil, ErrPickingNotAllowed
	}

	type targetItemInfo struct {
		id                uuid.UUID
		productVariantID  uuid.UUID
		quantity          int
		pickedQuantity    int
		activeAllocations int
		serializedPicked  int
	}
	var targetItem *targetItemInfo
	if targetOrderItemID != nil {
		var t targetItemInfo
		errTarget := tx.QueryRow(ctx, `
			SELECT oi.id, oi.product_variant_id, oi.quantity, oi.picked_quantity
			FROM order_items oi
			WHERE oi.id = $1 AND oi.order_fulfillment_id = $2
			FOR UPDATE
		`, *targetOrderItemID, fulfillmentID).Scan(&t.id, &t.productVariantID, &t.quantity, &t.pickedQuantity)
		if errTarget != nil {
			if errors.Is(errTarget, pgx.ErrNoRows) {
				return nil, ErrItemNotInFulfillment
			}
			return nil, fmt.Errorf("failed to load target order item: %w", errTarget)
		}

		errCount := tx.QueryRow(ctx, `
			SELECT COUNT(*),
			       COUNT(*) FILTER (WHERE picked_at IS NOT NULL)
			FROM order_item_allocations a
			WHERE a.order_item_id = $1 AND a.released_at IS NULL
		`, *targetOrderItemID).Scan(&t.activeAllocations, &t.serializedPicked)
		if errCount != nil {
			return nil, fmt.Errorf("failed to count target item allocations: %w", errCount)
		}
		targetItem = &t
	}

	orderNum := ""
	if po.OrderNumber != nil {
		orderNum = *po.OrderNumber
	}
	res := &PickingScanResult{
		FulfillmentID: po.FulfillmentID,
		OrderID:       po.OrderID,
		OrderNumber:   orderNum,
		ScanResult: PickingScanDetail{
			Code: code,
		},
	}

	// 2. Try ZMU
	var unitID uuid.UUID
	var unitVariantID uuid.UUID
	var unitStatus string
	err = tx.QueryRow(ctx, `SELECT id, product_variant_id, status FROM inventory_units WHERE unit_code = $1 FOR UPDATE`, code).Scan(&unitID, &unitVariantID, &unitStatus)
	if err == nil {
		// It IS a ZMU
		if unitStatus != "warehouse" {
			return nil, ErrUnitNotInWarehouse
		}

		// Check active allocation
		var allocID uuid.UUID
		var allocOrderItemID uuid.UUID
		var allocFulfillmentID *uuid.UUID
		var pickedAt *time.Time
		var allocOrderStatus string
		var allocFulfillmentStatus *string
		err = tx.QueryRow(ctx, `
			SELECT a.id, a.order_item_id, a.picked_at, oi.order_fulfillment_id, o.status, f.status
			FROM order_item_allocations a
			JOIN order_items oi ON oi.id = a.order_item_id
			JOIN orders o ON o.id = oi.order_id
			LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
			WHERE a.inventory_unit_id = $1 AND a.released_at IS NULL
			FOR UPDATE OF a
		`, unitID).Scan(&allocID, &allocOrderItemID, &pickedAt, &allocFulfillmentID, &allocOrderStatus, &allocFulfillmentStatus)

		isStaleAllocation := false
		if err == nil {
			fStatus := ""
			if allocFulfillmentStatus != nil {
				fStatus = *allocFulfillmentStatus
			}
			if allocOrderStatus == "delivered" || allocOrderStatus == "cancelled" || allocOrderStatus == "returned" || allocOrderStatus == "refunded" ||
				fStatus == "delivered" || fStatus == "cancelled" {
				isStaleAllocation = true
			}
		}

		if err == nil && !isStaleAllocation {
			// Unit is already allocated to a live order
			if allocFulfillmentID == nil || *allocFulfillmentID != fulfillmentID {
				return nil, ErrUnitAllocatedToOtherOrder
			}
			if targetItem != nil && allocOrderItemID != targetItem.id {
				return nil, ErrUnitAllocatedToOtherOrderItem
			}

			res.ScanResult.Type = "serialized"
			res.ScanResult.OrderItemID = allocOrderItemID

			if pickedAt != nil {
				res.ScanResult.AlreadyPicked = true
				res.ScanResult.NewlyPicked = false
				res.ScanResult.Substituted = false
			} else {
				_, err = tx.Exec(ctx, `UPDATE order_item_allocations SET picked_at = now() WHERE id = $1`, allocID)
				if err != nil {
					return nil, fmt.Errorf("failed to pick unit: %w", err)
				}
				res.ScanResult.NewlyPicked = true
			}
		} else if errors.Is(err, pgx.ErrNoRows) || isStaleAllocation {
			if isStaleAllocation {
				_, err = tx.Exec(ctx, `UPDATE order_item_allocations SET released_at = now(), release_reason = 'stale_allocation_superseded' WHERE id = $1`, allocID)
				if err != nil {
					return nil, fmt.Errorf("failed to release stale allocation: %w", err)
				}
			}

			// Unit is FREE!
			// 1. Check if unit variant is in this fulfillment
			var variantInFulfillment bool
			_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM order_items WHERE order_fulfillment_id = $1 AND product_variant_id = $2)`, fulfillmentID, unitVariantID).Scan(&variantInFulfillment)
			if !variantInFulfillment {
				return nil, ErrUnitVariantMismatch
			}

			// 2. Check if fulfillment has serialized allocations for this variant
			var hasSerializedAllocations bool
			_ = tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM order_items oi
					JOIN order_item_allocations a ON a.order_item_id = oi.id AND a.released_at IS NULL
					WHERE oi.order_fulfillment_id = $1 AND oi.product_variant_id = $2
				)
			`, fulfillmentID, unitVariantID).Scan(&hasSerializedAllocations)
			if !hasSerializedAllocations {
				if targetItem != nil && targetItem.activeAllocations == 0 {
					return nil, ErrItemNotSerialized
				}
				return nil, ErrUnitNotAllocatedToFulfillment
			}

			// 3. Free ZMU substitution requires an explicit target orderItemId.
			if targetItem == nil {
				return nil, ErrOrderItemRequiredForSubstitution
			}

			// Target item must be serialized
			if targetItem.activeAllocations == 0 {
				return nil, ErrItemNotSerialized
			}
			if targetItem.activeAllocations != targetItem.quantity {
				return nil, ErrInvariantViolation
			}
			if targetItem.serializedPicked >= targetItem.quantity {
				return nil, ErrNoUnpickedAllocationForVariant
			}
			if targetItem.productVariantID != unitVariantID {
				return nil, ErrUnitVariantMismatch
			}

			// Find unpicked allocation specifically on targetItem
			var oldAllocID uuid.UUID
			var reservationID *uuid.UUID
			errSub := tx.QueryRow(ctx, `
				SELECT a.id, a.reservation_id
				FROM order_item_allocations a
				WHERE a.order_item_id = $1 AND a.released_at IS NULL AND a.picked_at IS NULL
				ORDER BY a.created_at ASC
				LIMIT 1
				FOR UPDATE OF a
			`, targetItem.id).Scan(&oldAllocID, &reservationID)
			if errSub != nil {
				if errors.Is(errSub, pgx.ErrNoRows) {
					return nil, ErrNoUnpickedAllocationForVariant
				}
				return nil, fmt.Errorf("failed to find unpicked allocation for target item: %w", errSub)
			}

			now := time.Now()
			_, err = tx.Exec(ctx, `
				UPDATE order_item_allocations
				SET released_at = $1, release_reason = 'picking_substitution'
				WHERE id = $2
			`, now, oldAllocID)
			if err != nil {
				return nil, fmt.Errorf("failed to release old allocation for substitution: %w", err)
			}

			newAllocID := uuid.New()
			_, err = tx.Exec(ctx, `
				INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, created_at, picked_at)
				VALUES ($1, $2, $3, $4, $5, $5)
			`, newAllocID, targetItem.id, unitID, reservationID, now)
			if err != nil {
				return nil, fmt.Errorf("failed to insert replacement allocation: %w", err)
			}

			res.ScanResult.Type = "serialized"
			res.ScanResult.OrderItemID = targetItem.id
			res.ScanResult.NewlyPicked = true
			res.ScanResult.Substituted = true
		} else {
			return nil, fmt.Errorf("failed to check unit allocation: %w", err)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		// 3. Not a ZMU -> Legacy barcode / SKU
		if targetItem != nil {
			var matchesBarcode bool
			_ = tx.QueryRow(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM product_variants pv
					WHERE pv.id = $1 AND (((pv.barcode = $2 AND pv.barcode !~ '^0+$') OR pv.sku = $2 OR pv.seller_sku = $2))
				)
			`, targetItem.productVariantID, code).Scan(&matchesBarcode)
			if !matchesBarcode {
				return nil, ErrCodeNotFound
			}
			if targetItem.activeAllocations > 0 {
				return nil, ErrCannotPickSerializedWithBarcode
			}

			res.ScanResult.OrderItemID = targetItem.id
			res.ScanResult.Type = "legacy"

			if targetItem.pickedQuantity >= targetItem.quantity {
				res.ScanResult.AlreadyComplete = true
			} else {
				cmdTag, err := tx.Exec(ctx, `
					UPDATE order_items
					SET picked_quantity = picked_quantity + 1
					WHERE id = $1 AND picked_quantity < quantity
				`, targetItem.id)
				if err != nil {
					return nil, fmt.Errorf("failed to increment legacy pick: %w", err)
				}
				if cmdTag.RowsAffected() == 1 {
					res.ScanResult.NewlyPicked = true
				} else {
					res.ScanResult.AlreadyComplete = true
				}
			}
		} else {
			queryLegacy := `
				SELECT oi.id, oi.quantity, oi.picked_quantity,
				       (SELECT count(*) FROM order_item_allocations a WHERE a.order_item_id = oi.id AND a.released_at IS NULL) as alloc_count
				FROM order_items oi
				JOIN product_variants pv ON pv.id = oi.product_variant_id
				WHERE oi.order_fulfillment_id = $1
				  AND ((pv.barcode = $2 AND pv.barcode !~ '^0+$') OR pv.sku = $2 OR pv.seller_sku = $2)
				FOR UPDATE OF oi
			`
			rows, err := tx.Query(ctx, queryLegacy, fulfillmentID, code)
			if err != nil {
				return nil, fmt.Errorf("failed to query legacy barcode: %w", err)
			}

			type match struct {
				id       uuid.UUID
				quantity int
				picked   int
				allocs   int
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
					res.ScanResult.AlreadyComplete = true
				}
			}
		}
	} else {
		return nil, fmt.Errorf("failed to query inventory_units: %w", err)
	}

	// 4. Calculate progress
	po2, err := r.GetPickingOrderTx(ctx, tx, fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrInvariantViolation) {
			return nil, ErrInvariantViolation
		}
		if errors.Is(err, ErrPickingNotAllowed) {
			return nil, ErrPickingNotAllowed
		}
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

// DBRowScanner abstracts QueryRow for transactions, pools, and connections.
type DBRowScanner interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CountActionablePicking returns the number of eligible fulfillments that still have real unfinished picking work.
// Authoritative picking semantics:
// 1. Order status IN ('paid', 'assembling') AND Fulfillment status IN ('paid', 'assembling').
// 2. Contains at least one item with remainingQuantity > 0:
//   - Serialized item (allocations == quantity): has at least one active allocation with picked_at IS NULL.
//   - Legacy item (allocations == 0): picked_quantity < quantity.
func CountActionablePicking(ctx context.Context, db DBRowScanner) (int, error) {
	query := `
		SELECT COUNT(DISTINCT of.id)
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.status IN ('paid', 'assembling')
		  AND o.status IN ('paid', 'assembling')
		  AND EXISTS (
		      SELECT 1
		      FROM order_items oi
		      WHERE oi.order_fulfillment_id = of.id
		        AND oi.quantity > 0
		        AND (
		            -- Serialized: full allocation where at least one unit is not yet picked
		            (
		                (SELECT COUNT(*) FROM order_item_allocations a WHERE a.order_item_id = oi.id AND a.released_at IS NULL) = oi.quantity
		                AND EXISTS (
		                    SELECT 1
		                    FROM order_item_allocations a
		                    WHERE a.order_item_id = oi.id
		                      AND a.released_at IS NULL
		                      AND a.picked_at IS NULL
		                )
		            )
		            OR
		            -- Legacy: zero active allocations and picked_quantity < quantity
		            (
		                NOT EXISTS (
		                    SELECT 1
		                    FROM order_item_allocations a
		                    WHERE a.order_item_id = oi.id
		                      AND a.released_at IS NULL
		                )
		                AND oi.picked_quantity < oi.quantity
		            )
		        )
		  )
	`
	var count int
	if err := db.QueryRow(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// IsFulfillmentActionablePicking returns true if the specific fulfillment requires picking work.
func IsFulfillmentActionablePicking(ctx context.Context, db DBRowScanner, fulfillmentID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM order_fulfillments of
			JOIN orders o ON o.id = of.order_id
			WHERE of.id = $1
			  AND of.status IN ('paid', 'assembling')
			  AND o.status IN ('paid', 'assembling')
			  AND EXISTS (
			      SELECT 1
			      FROM order_items oi
			      WHERE oi.order_fulfillment_id = of.id
			        AND oi.quantity > 0
			        AND (
			            -- Serialized: full allocation where at least one unit is not yet picked
			            (
			                (SELECT COUNT(*) FROM order_item_allocations a WHERE a.order_item_id = oi.id AND a.released_at IS NULL) = oi.quantity
			                AND EXISTS (
			                    SELECT 1
			                    FROM order_item_allocations a
			                    WHERE a.order_item_id = oi.id
			                      AND a.released_at IS NULL
			                      AND a.picked_at IS NULL
			                )
			            )
			            OR
			            -- Legacy: zero active allocations and picked_quantity < quantity
			            (
			                NOT EXISTS (
			                    SELECT 1
			                    FROM order_item_allocations a
			                    WHERE a.order_item_id = oi.id
			                      AND a.released_at IS NULL
			                )
			                AND oi.picked_quantity < oi.quantity
			            )
			        )
			  )
		)
	`
	var actionable bool
	if err := db.QueryRow(ctx, query, fulfillmentID).Scan(&actionable); err != nil {
		return false, err
	}
	return actionable, nil
}

func (r *Repository) CountActionablePicking(ctx context.Context) (int, error) {
	return CountActionablePicking(ctx, r.db)
}

func (r *Repository) IsFulfillmentActionablePicking(ctx context.Context, fulfillmentID uuid.UUID) (bool, error) {
	return IsFulfillmentActionablePicking(ctx, r.db, fulfillmentID)
}

func (r *Repository) GetCompatibleUnits(ctx context.Context, fulfillmentID uuid.UUID, orderItemID uuid.UUID) ([]CompatibleUnit, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	units, err := r.GetCompatibleUnitsTx(ctx, tx, fulfillmentID, orderItemID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return units, nil
}

func (r *Repository) GetCompatibleUnitsTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, orderItemID uuid.UUID) ([]CompatibleUnit, error) {
	// 1. Validate fulfillment exists and picking is eligible
	var orderStatus, fulfillmentStatus string
	err := tx.QueryRow(ctx, `
		SELECT o.status, of.status
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.id = $1
	`, fulfillmentID).Scan(&orderStatus, &fulfillmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to check fulfillment: %w", err)
	}

	if (orderStatus != "paid" && orderStatus != "assembling") ||
		(fulfillmentStatus != "paid" && fulfillmentStatus != "assembling") {
		return nil, ErrPickingNotAllowed
	}

	// 2. Validate order item belongs to fulfillment
	var variantID uuid.UUID
	var quantity int
	err = tx.QueryRow(ctx, `
		SELECT product_variant_id, quantity
		FROM order_items
		WHERE id = $1 AND order_fulfillment_id = $2
	`, orderItemID, fulfillmentID).Scan(&variantID, &quantity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotInFulfillment
		}
		return nil, fmt.Errorf("failed to find order item: %w", err)
	}

	// 3. Canonical serialized state derived from active allocations
	var activeAllocations, serializedPicked int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE picked_at IS NOT NULL)
		FROM order_item_allocations
		WHERE order_item_id = $1 AND released_at IS NULL
	`, orderItemID).Scan(&activeAllocations, &serializedPicked)
	if err != nil {
		return nil, fmt.Errorf("failed to count allocations: %w", err)
	}

	// Rules:
	// - A == 0 -> legacy -> ErrItemNotSerialized
	// - 0 < A < Q or A > Q -> invariant violation
	// - A == Q -> serialized
	// - serializedPicked == Q -> ErrItemAlreadyComplete
	// - serializedPicked < Q -> actionable
	if activeAllocations == 0 {
		return nil, ErrItemNotSerialized
	}
	if activeAllocations != quantity {
		return nil, ErrInvariantViolation
	}
	if serializedPicked >= quantity {
		return nil, ErrItemAlreadyComplete
	}

	// 5. Query scannable candidates
	query := `
		SELECT u.id, u.unit_code, u.product_variant_id,
		       CASE
		           WHEN a.id IS NOT NULL THEN 'allocated_to_current_item'
		           ELSE 'free'
		       END as availability
		FROM inventory_units u
		LEFT JOIN order_item_allocations a ON a.inventory_unit_id = u.id
		    AND a.order_item_id = $1
		    AND a.released_at IS NULL
		WHERE u.product_variant_id = $2
		  AND u.status = 'warehouse'
		  AND (
		      (a.id IS NOT NULL AND a.picked_at IS NULL)
		      OR (
		          a.id IS NULL
		          AND NOT EXISTS (
		              SELECT 1 FROM order_item_allocations other_a
		              JOIN order_items other_oi ON other_oi.id = other_a.order_item_id
		              JOIN orders other_o ON other_o.id = other_oi.order_id
		              LEFT JOIN order_fulfillments other_f ON other_f.id = other_oi.order_fulfillment_id
		              WHERE other_a.inventory_unit_id = u.id
		                AND other_a.released_at IS NULL
		                AND other_o.status NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
		                AND COALESCE(other_f.status, '') NOT IN ('delivered', 'cancelled')
		          )
		      )
		  )
		ORDER BY
		    CASE WHEN a.id IS NOT NULL THEN 0 ELSE 1 END,
		    u.created_at ASC,
		    u.unit_code ASC
	`
	rows, err := tx.Query(ctx, query, orderItemID, variantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query compatible units: %w", err)
	}
	defer rows.Close()

	var units []CompatibleUnit
	for rows.Next() {
		var u CompatibleUnit
		if err := rows.Scan(&u.InventoryUnitID, &u.UnitCode, &u.ProductVariantID, &u.Availability); err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	if units == nil {
		units = []CompatibleUnit{}
	}
	return units, rows.Err()
}
