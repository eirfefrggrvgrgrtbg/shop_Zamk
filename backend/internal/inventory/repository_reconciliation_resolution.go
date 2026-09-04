package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

	// Get terminal resolutions
	resolutions := make(map[uuid.UUID]ResolutionAuditDTO)
	resRows, err := r.db.Query(ctx, "SELECT r.inventory_unit_id, r.action_id, r.performed_by, r.performed_at, u.unit_code FROM inventory_reconciliation_resolutions r LEFT JOIN inventory_units u ON r.replacement_inventory_unit_id = u.id WHERE session_id = $1 AND action_id = 'confirm_missing'", sessionID)
	if err == nil {
		defer resRows.Close()
		for resRows.Next() {
			var uid uuid.UUID
			var dto ResolutionAuditDTO
			var repCode *string
			if err := resRows.Scan(&uid, &dto.ActionID, &dto.PerformedBy, &dto.PerformedAt, &repCode); err == nil {
				if repCode != nil { dto.ReplacementUnitCode = *repCode }
				resolutions[uid] = dto
			}
		}
	}

	var resCount int
	_ = r.db.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_resolutions WHERE session_id = $1", sessionID).Scan(&resCount)
	plan.ResolutionsCount = resCount

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

		if res, ok := resolutions[fact.UnitID]; ok {
			fact.Resolution = &res
		}
		resCase := ClassifyResolutionFact(fact)
		if resCase != nil {
			if res, ok := resolutions[resCase.UnitID]; ok {
				resCase.Resolution = &res
				resCase.AllowedActions = []ReconciliationResolutionAction{} // Hide actions if resolved
			} else if resCase.CaseType == CaseTypeMissingLiveAllocated {
				// Query replacement candidates
				candRows, err := r.db.Query(ctx, `
					SELECT id, unit_code
					FROM inventory_units
					WHERE product_variant_id = $1
					  AND status = 'warehouse'
					  AND id != $2
					  AND NOT EXISTS (
						  SELECT 1 FROM order_item_allocations a
						  WHERE a.inventory_unit_id = inventory_units.id
						    AND a.released_at IS NULL
					  )
					ORDER BY created_at ASC
				`, resCase.VariantID, resCase.UnitID)
				if err == nil {
					var candidates []ReplacementCandidateDTO
					for candRows.Next() {
						var cand ReplacementCandidateDTO
						if err := candRows.Scan(&cand.UnitID, &cand.UnitCode); err == nil {
							candidates = append(candidates, cand)
						}
					}
					candRows.Close()
					resCase.ReplacementCandidates = candidates
					if len(candidates) == 0 {
						for i := range resCase.AllowedActions {
							if resCase.AllowedActions[i].ID == ActionIDConfirmMissing {
								resCase.AllowedActions[i].Enabled = false
								resCase.AllowedActions[i].SafetyLevel = ActionSafetyBlocked
								resCase.AllowedActions[i].BlockedReason = "Нет доступных свободных единиц на складе для замены."
							}
						}
					}
				}
			}
			plan.Cases = append(plan.Cases, *resCase)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading resolution facts: %w", err)
	}

	resolvedCasesCount := 0
	for _, c := range plan.Cases {
		if c.Resolution != nil {
			resolvedCasesCount++
		}
	}
	plan.ResolvedCasesCount = resolvedCasesCount

	return plan, nil
}

// resolveReconciliationCaseTx executes the mutation within an already-open transaction.
// The caller (Service) is responsible for beginning and committing the transaction.
func (r *Repository) resolveReconciliationCaseTx(ctx context.Context, tx pgx.Tx, sessionID, adminID uuid.UUID, req ResolveReconciliationCaseRequest) error {
	// 1. Lock session and verify state
	var sessStatus string
	var sessVariantID uuid.UUID
	err := tx.QueryRow(ctx, "SELECT status, product_variant_id FROM inventory_reconciliation_sessions WHERE id = $1 FOR UPDATE", sessionID).Scan(&sessStatus, &sessVariantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReconciliationNotFound
		}
		return err
	}
	if sessStatus != "completed" && sessStatus != "review" {
		return fmt.Errorf("%w: session is in status %s (must be completed or review)", ErrInvalidReconciliationState, sessStatus)
	}

	// 2. Resolve target unit ID if needed
	targetUnitID := req.UnitID
	if targetUnitID == nil {
		if strings.TrimSpace(req.UnitCode) == "" {
			return errors.New("unitId or unitCode required")
		}
		var resolvedID uuid.UUID
		err = tx.QueryRow(ctx, "SELECT id FROM inventory_units WHERE LOWER(TRIM(unit_code)) = LOWER(TRIM($1))", req.UnitCode).Scan(&resolvedID)
		if err != nil {
			return ErrInventoryUnitNotFound
		}
		targetUnitID = &resolvedID
	}

	// 3. Lock the inventory unit
	var unitStatus, unitCode string
	var variantID uuid.UUID
	err = tx.QueryRow(ctx,
		"SELECT status, unit_code, product_variant_id FROM inventory_units WHERE id = $1 FOR UPDATE",
		*targetUnitID,
	).Scan(&unitStatus, &unitCode, &variantID)
	if err != nil {
		return fmt.Errorf("failed to lock unit: %w", err)
	}

	if variantID != sessVariantID {
		return fmt.Errorf("%w: unit does not belong to session variant", ErrReconciliationConflict)
	}

	// 4. Shipped found cannot be mutated
	if unitStatus == "shipped" {
		return fmt.Errorf("%w: shipped unit cannot be mutated through reconciliation", ErrReconciliationConflict)
	}

	// 5. Check if unit was scanned as expected_found
	if req.ActionID == ActionIDConfirmMissing {
		var scanCount int
		_ = tx.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_scans WHERE session_id = $1 AND inventory_unit_id = $2 AND classification = 'expected_found'", sessionID, *targetUnitID).Scan(&scanCount)
		if scanCount > 0 {
			return fmt.Errorf("%w: unit was found during scan, cannot confirm missing", ErrReconciliationConflict)
		}
	}

	// 6. Check if unit is already resolved with confirm_missing in this session
	var existingTerminalAction string
	err = tx.QueryRow(ctx, `
		SELECT action_id
		FROM inventory_reconciliation_resolutions
		WHERE session_id = $1 AND inventory_unit_id = $2 AND action_id = 'confirm_missing'
	`, sessionID, *targetUnitID).Scan(&existingTerminalAction)
	if err == nil {
		if req.ActionID == ActionIDConfirmMissing {
			// Idempotent duplicate call - already resolved, return success
			return nil
		}
		return fmt.Errorf("%w: unit is already resolved with %s", ErrReconciliationConflict, existingTerminalAction)
	}

	if unitStatus == "written_off" {
		return fmt.Errorf("%w: unit is already written off", ErrReconciliationConflict)
	}

	// 7. Find active allocation if any
	var allocID uuid.UUID
	var allocPickedAt *time.Time
	var ordStatus *string
	var fulStatus *string
	err = tx.QueryRow(ctx, `
		SELECT a.id, a.picked_at, o.status, f.status
		FROM order_item_allocations a
		JOIN order_items oi ON a.order_item_id = oi.id
		JOIN orders o ON oi.order_id = o.id
		LEFT JOIN order_fulfillments f ON oi.order_fulfillment_id = f.id
		WHERE a.inventory_unit_id = $1 AND a.released_at IS NULL
	`, *targetUnitID).Scan(&allocID, &allocPickedAt, &ordStatus, &fulStatus)
	hasAlloc := err == nil

	allocIsStale := hasAlloc && (isTerminalOrder(ordStatus) || isTerminalFulfillment(fulStatus))
	allocIsLive := hasAlloc && !allocIsStale
	allocIsPicked := allocIsLive && allocPickedAt != nil

	if allocIsPicked {
		return fmt.Errorf("%w: unit is already picked for active order, mutation forbidden", ErrReconciliationConflict)
	}

	switch req.ActionID {
	case ActionIDCloseStaleAllocation:
		if !hasAlloc {
			// Check if already closed in this session (idempotency)
			var count int
			_ = tx.QueryRow(ctx, `
				SELECT count(*) FROM inventory_reconciliation_resolutions
				WHERE session_id = $1 AND inventory_unit_id = $2 AND action_id = $3
			`, sessionID, *targetUnitID, ActionIDCloseStaleAllocation).Scan(&count)
			if count > 0 {
				return nil
			}
			return fmt.Errorf("%w: unit has no active allocation to close", ErrReconciliationConflict)
		}
		if !allocIsStale {
			ordStatStr := "unknown"
			if ordStatus != nil {
				ordStatStr = *ordStatus
			}
			return fmt.Errorf("%w: allocation belongs to active order (%s), cannot close as stale", ErrReconciliationConflict, ordStatStr)
		}

		// Release the allocation
		_, err = tx.Exec(ctx,
			"UPDATE order_item_allocations SET released_at = now(), release_reason = 'inventory_reconciliation' WHERE id = $1",
			allocID,
		)
		if err != nil {
			return fmt.Errorf("failed to release allocation: %w", err)
		}
		// Adjust reserved_stock safely
		_, err = tx.Exec(ctx,
			"UPDATE inventory_items SET reserved_stock = GREATEST(reserved_stock - 1, 0) WHERE product_variant_id = $1",
			variantID,
		)
		if err != nil {
			return fmt.Errorf("failed to adjust reserved stock: %w", err)
		}
		// Write traceability record
		ordStatStr := ""
		if ordStatus != nil {
			ordStatStr = *ordStatus
		}
		beforeCtx := map[string]interface{}{"allocation_id": allocID.String(), "unit_status": unitStatus, "order_status": ordStatStr}
		afterCtx := map[string]interface{}{"allocation_released": true, "unit_status": unitStatus}
		beforeJSON, _ := json.Marshal(beforeCtx)
		afterJSON, _ := json.Marshal(afterCtx)
		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_reconciliation_resolutions
			  (session_id, inventory_unit_id, case_type, action_id, performed_by, related_allocation_id, before_context, after_context)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, sessionID, *targetUnitID, CaseTypeStaleAllocation, ActionIDCloseStaleAllocation, adminID, allocID, beforeJSON, afterJSON)
		if err != nil {
			return fmt.Errorf("failed to write resolution record: %w", err)
		}

	case ActionIDConfirmMissing:
		if hasAlloc && allocIsStale {
			return fmt.Errorf("%w: unit has stale allocation, must close allocation before confirming missing", ErrReconciliationConflict)
		}
		caseType := CaseTypeMissingFree
		var repID *uuid.UUID
		var repCode string
		// If unit has a live allocation, a replacement is mandatory
		if allocIsLive {
			caseType = CaseTypeMissingLiveAllocated
			repID = req.ReplacementUnitID
			if repID == nil && strings.TrimSpace(req.ReplacementUnitCode) != "" {
				var resolvedRepID uuid.UUID
				err = tx.QueryRow(ctx, "SELECT id FROM inventory_units WHERE LOWER(TRIM(unit_code)) = LOWER(TRIM($1))", req.ReplacementUnitCode).Scan(&resolvedRepID)
				if err != nil {
					return fmt.Errorf("%w: replacement unit not found", ErrReconciliationConflict)
				}
				repID = &resolvedRepID
			}
			if repID == nil {
				return fmt.Errorf("%w: replacement unit required when unit has a live allocation", ErrReconciliationConflict)
			}
			// Lock replacement and verify all safety conditions
			var repStatus string
			var repVariantID uuid.UUID
			err = tx.QueryRow(ctx,
				"SELECT status, unit_code, product_variant_id FROM inventory_units WHERE id = $1 FOR UPDATE",
				*repID,
			).Scan(&repStatus, &repCode, &repVariantID)
			if err != nil {
				return fmt.Errorf("%w: replacement unit not found", ErrReconciliationConflict)
			}
			if repVariantID != variantID {
				return fmt.Errorf("%w: replacement unit wrong variant", ErrReconciliationConflict)
			}
			if repStatus == "damaged" {
				return fmt.Errorf("%w: replacement unit is damaged", ErrReconciliationConflict)
			}
			if repStatus == "expected" {
				return fmt.Errorf("%w: replacement unit is expected (not received)", ErrReconciliationConflict)
			}
			if repStatus == "shipped" {
				return fmt.Errorf("%w: replacement unit is shipped", ErrReconciliationConflict)
			}
			if repStatus == "written_off" {
				return fmt.Errorf("%w: replacement unit is written off", ErrReconciliationConflict)
			}
			if repStatus != "warehouse" {
				return fmt.Errorf("%w: replacement unit is not free in warehouse (status=%s)", ErrReconciliationConflict, repStatus)
			}
			var repAllocCount int
			_ = tx.QueryRow(ctx,
				"SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id = $1 AND released_at IS NULL",
				*repID,
			).Scan(&repAllocCount)
			if repAllocCount > 0 {
				return fmt.Errorf("%w: replacement unit is already allocated to another order", ErrReconciliationConflict)
			}
			// Rebind allocation to replacement
			_, err = tx.Exec(ctx,
				"UPDATE order_item_allocations SET inventory_unit_id = $1 WHERE id = $2",
				*repID, allocID,
			)
			if err != nil {
				return fmt.Errorf("failed to rebind allocation: %w", err)
			}
		}

		// Write off the missing unit (do NOT delete)
		_, err = tx.Exec(ctx, "UPDATE inventory_units SET status = 'written_off' WHERE id = $1", *targetUnitID)
		if err != nil {
			return fmt.Errorf("failed to write off unit: %w", err)
		}

		// Decrement aggregate total_stock by 1 (legacy on_hand remains unchanged: legOnHand = aggTotal - phys.Warehouse; phys.Warehouse already decreased by status change)
		var itemID, prodID, sellerID uuid.UUID
		err = tx.QueryRow(ctx, `
			UPDATE inventory_items
			SET total_stock = GREATEST(total_stock - 1, 0)
			WHERE product_variant_id = $1
			RETURNING id, product_id, seller_id
		`, variantID).Scan(&itemID, &prodID, &sellerID)
		if err != nil {
			return fmt.Errorf("failed to decrement aggregate stock: %w", err)
		}

		// Write canonical stock movement
		_, err = tx.Exec(ctx, `
			INSERT INTO stock_movements
			  (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id, reference_type, reference_id)
			VALUES ($1, $2, $3, $4, $5, 'write_off', 1, 'reconciliation_write_off', $6, 'reconciliation_session', $7)
		`, uuid.New(), itemID, prodID, variantID, sellerID, adminID, sessionID)
		if err != nil {
			return fmt.Errorf("failed to write stock movement: %w", err)
		}

		var relAlloc *uuid.UUID
		if hasAlloc {
			relAlloc = &allocID
		}

		beforeCtx := map[string]interface{}{"unit_status": unitStatus, "had_allocation": hasAlloc}
		afterCtx := map[string]interface{}{"unit_status": "written_off"}
		if hasAlloc && repID != nil {
			afterCtx["replacement_unit_id"] = repID.String()
			afterCtx["replacement_unit_code"] = repCode
		}
		beforeJSON, _ := json.Marshal(beforeCtx)
		afterJSON, _ := json.Marshal(afterCtx)

		// Terminal resolution record – covered by the partial unique index (action_id = 'confirm_missing')
		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_reconciliation_resolutions
			  (session_id, inventory_unit_id, case_type, action_id, performed_by, related_allocation_id, replacement_inventory_unit_id, before_context, after_context)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, sessionID, *targetUnitID, caseType, ActionIDConfirmMissing, adminID, relAlloc, repID, beforeJSON, afterJSON)
		if err != nil {
			return fmt.Errorf("failed to write resolution record: %w", err)
		}

	default:
		return fmt.Errorf("unknown or disallowed action: %s", req.ActionID)
	}

	return nil
}
