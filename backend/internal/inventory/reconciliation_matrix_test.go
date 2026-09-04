package inventory_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
)

// 1. Traceability Events Contract Test
func TestReconciliationMatrix_TraceabilityEvents(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 10, reserved_stock = 2 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	// Unit 1: Missing with stale allocation
	unit1 := uuid.New()
	code1 := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit1, variantID, supplyID, supplyItemID, code1)
	require.NoError(t, err)

	allocID1 := uuid.New()
	orderItemID1 := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "UPDATE orders SET status = 'delivered' WHERE id = (SELECT order_id FROM order_items WHERE id = $1)", orderItemID1)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID1, orderItemID1, unit1)
	require.NoError(t, err)

	// Unit 2: Missing with live allocation
	unit2 := uuid.New()
	code2 := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 2, now(), now())", unit2, variantID, supplyID, supplyItemID, code2)
	require.NoError(t, err)

	allocID2 := uuid.New()
	orderItemID2 := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID2, orderItemID2, unit2)
	require.NoError(t, err)

	// Unit 3: Free warehouse candidate for replacement
	unit3 := uuid.New()
	code3 := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 3, now(), now())", unit3, variantID, supplyID, supplyItemID, code3)
	require.NoError(t, err)

	// Start session and move to completed
	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	// 1. Release stale allocation on Unit 1
	reqStale := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit1,
		ActionID: inventory.ActionIDCloseStaleAllocation,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqStale)
	require.NoError(t, err)

	// Verify traceability for Unit 1 contains "Старое назначение освобождено"
	trc1, err := repo.GetAdminInventoryUnitTraceability(ctx, code1)
	require.NoError(t, err)
	require.NotNil(t, trc1)
	foundStaleEvt := false
	for _, ev := range trc1.Timeline {
		if ev.EventName == "Старое назначение освобождено" {
			foundStaleEvt = true
			require.Equal(t, "reconciliation_stale_allocation_released", ev.Type)
			require.Equal(t, "commitment", ev.Category)
			require.Equal(t, "inventory_reconciliation_resolutions", ev.SourceEntity)
			break
		}
	}
	require.True(t, foundStaleEvt, "Timeline for unit 1 must include 'Старое назначение освобождено'")

	// 2. Confirm missing on Unit 1 (now free)
	reqMissing := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit1,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqMissing)
	require.NoError(t, err)

	trc1After, err := repo.GetAdminInventoryUnitTraceability(ctx, code1)
	require.NoError(t, err)
	foundMissingEvt := false
	for _, ev := range trc1After.Timeline {
		if ev.EventName == "Списана по результатам инвентаризации" {
			foundMissingEvt = true
			require.Equal(t, "reconciliation_missing_written_off", ev.Type)
			require.Equal(t, "physical", ev.Category)
			break
		}
	}
	require.True(t, foundMissingEvt, "Timeline for unit 1 must include 'Списана по результатам инвентаризации'")

	// 3. Confirm missing on Unit 2 with Unit 3 as replacement
	reqRepl := inventory.ResolveReconciliationCaseRequest{
		UnitID:            &unit2,
		ActionID:          inventory.ActionIDConfirmMissing,
		ReplacementUnitID: &unit3,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqRepl)
	require.NoError(t, err)

	// Verify Unit 3 has replacement event in timeline
	trc3, err := repo.GetAdminInventoryUnitTraceability(ctx, code3)
	require.NoError(t, err)
	require.NotNil(t, trc3)
	foundReplEvt := false
	for _, ev := range trc3.Timeline {
		if ev.EventName == "Назначена заказу после инвентаризации" {
			foundReplEvt = true
			require.Equal(t, "reconciliation_replacement_allocated", ev.Type)
			require.Equal(t, "commitment", ev.Category)
			break
		}
	}
	require.True(t, foundReplEvt, "Timeline for replacement unit 3 must include 'Назначена заказу после инвентаризации'")
}

// Case A: Stale allocation release
func TestReconciliationMatrix_CaseA_StaleAllocationRelease(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 1 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unit := uuid.New()
	code := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit, variantID, supplyID, supplyItemID, code)
	require.NoError(t, err)

	allocID := uuid.New()
	orderItemID := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "UPDATE orders SET status = 'delivered' WHERE id = (SELECT order_id FROM order_items WHERE id = $1)", orderItemID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID, orderItemID, unit)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	req := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDCloseStaleAllocation,
	}
	plan, err := svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.NoError(t, err)
	require.NotNil(t, plan)

	// Check allocation is released
	var releasedAt *time.Time
	var reason *string
	err = testDB.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocID).Scan(&releasedAt, &reason)
	require.NoError(t, err)
	require.NotNil(t, releasedAt)
	require.Equal(t, "inventory_reconciliation", *reason)

	// Check reserved stock decremented
	var resStock int
	err = testDB.QueryRow(ctx, "SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&resStock)
	require.NoError(t, err)
	require.Equal(t, 0, resStock)

	// Check unit status is still warehouse
	var uStatus string
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unit).Scan(&uStatus)
	require.NoError(t, err)
	require.Equal(t, "warehouse", uStatus)

	// Verify resolution audit row contains before_context and after_context
	var beforeCtx, afterCtx []byte
	err = testDB.QueryRow(ctx, "SELECT before_context, after_context FROM inventory_reconciliation_resolutions WHERE session_id = $1 AND inventory_unit_id = $2", sessionID, unit).Scan(&beforeCtx, &afterCtx)
	require.NoError(t, err)
	require.NotEmpty(t, beforeCtx)
	require.NotEmpty(t, afterCtx)
}

// Case B: Idempotency & duplicate action rejection
func TestReconciliationMatrix_CaseB_Idempotency(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 0 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unit := uuid.New()
	code := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit, variantID, supplyID, supplyItemID, code)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	req := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.NoError(t, err)

	// Second confirm_missing on same unit in same session MUST be idempotent (succeed without double mutations)
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.NoError(t, err, "Idempotent duplicate call must succeed")

	// Verify total stock was only decremented ONCE (from 5 to 4, NOT to 3)
	var totalStock int
	err = testDB.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalStock)
	require.NoError(t, err)
	require.Equal(t, 4, totalStock, "Stock must only be decremented once")

	// Verify exactly ONE resolution row exists
	var resCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_resolutions WHERE session_id = $1 AND inventory_unit_id = $2", sessionID, unit).Scan(&resCount)
	require.NoError(t, err)
	require.Equal(t, 1, resCount, "Exactly one resolution record must exist")
}

// Case C: Concurrent state change (409 conflict)
func TestReconciliationMatrix_CaseC_StateConflict409(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 0 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unit := uuid.New()
	code := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit, variantID, supplyID, supplyItemID, code)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	// Unit status mutated concurrently to 'shipped'
	_, err = testDB.Exec(ctx, "UPDATE inventory_units SET status = 'shipped' WHERE id = $1", unit)
	require.NoError(t, err)

	req := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.Error(t, err)
	require.ErrorIs(t, err, inventory.ErrReconciliationConflict)
}

// Case D: Missing free write-off & stock_movements check
func TestReconciliationMatrix_CaseD_MissingFreeWriteOff(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 3, reserved_stock = 0 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unit := uuid.New()
	code := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit, variantID, supplyID, supplyItemID, code)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	req := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.NoError(t, err)

	// Check unit status = written_off
	var uStatus string
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unit).Scan(&uStatus)
	require.NoError(t, err)
	require.Equal(t, "written_off", uStatus)

	// Check total stock decremented
	var totalStock int
	err = testDB.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalStock)
	require.NoError(t, err)
	require.Equal(t, 2, totalStock)

	// Check stock_movements record exists with type 'write_off'
	var mvtCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE product_id = (SELECT product_id FROM product_variants WHERE id = $1) AND type = 'write_off'", variantID).Scan(&mvtCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, mvtCount, 1)
}

// Case E: Mixed accounting invariant proof
func TestReconciliationMatrix_CaseE_MixedAccountingProof(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	// In mixed mode:
	// Total Stock = 5
	// Physical Warehouse Units = 3
	// Legacy On-Hand = Total Stock - Physical Warehouse Units = 5 - 3 = 2
	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 0 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unit1 := uuid.New()
	unit2 := uuid.New()
	unit3 := uuid.New()
	for i, u := range []uuid.UUID{unit1, unit2, unit3} {
		code := mustGenerateUnitCode()
		_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', $6, now(), now())", u, variantID, supplyID, supplyItemID, code, i+1)
		require.NoError(t, err)
	}

	// Verify BEFORE values
	var totalBefore int
	err = testDB.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalBefore)
	require.NoError(t, err)
	var physBefore int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_units WHERE product_variant_id = $1 AND status = 'warehouse'", variantID).Scan(&physBefore)
	require.NoError(t, err)
	legacyBefore := totalBefore - physBefore

	require.Equal(t, 5, totalBefore)
	require.Equal(t, 3, physBefore)
	require.Equal(t, 2, legacyBefore)

	// Perform resolution: write off unit 1
	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	req := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit1,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.NoError(t, err)

	// Verify AFTER values
	var totalAfter int
	err = testDB.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalAfter)
	require.NoError(t, err)
	var physAfter int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_units WHERE product_variant_id = $1 AND status = 'warehouse'", variantID).Scan(&physAfter)
	require.NoError(t, err)
	legacyAfter := totalAfter - physAfter

	require.Equal(t, 4, totalAfter, "Total stock must decrement from 5 to 4 (-1)")
	require.Equal(t, 2, physAfter, "Physical warehouse units must decrement from 3 to 2 (-1)")
	require.Equal(t, 2, legacyAfter, "Legacy on-hand must remain EXACTLY 2 (4 - 2 = 2)")
}

// Cases K-P: Candidate Rejection Rules
func TestReconciliationMatrix_CasesKP_CandidateRejectionRules(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 10, reserved_stock = 1 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	// Missing unit with active order
	unitLive := uuid.New()
	codeLive := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unitLive, variantID, supplyID, supplyItemID, codeLive)
	require.NoError(t, err)

	allocID := uuid.New()
	orderItemID := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID, orderItemID, unitLive)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	// Rule K: Candidate belongs to another variant
	otherVariantID := uuid.New()
	otherProdID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO products (id, title, slug, price_cents, status, seller_id) VALUES ($1, 'Other Prod', $2, 1000, 'published', $3)", otherProdID, "other-"+uuid.New().String()[:6], sellerID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, barcode) VALUES ($1, $2, $3, $4)", otherVariantID, otherProdID, "OTHER-SKU-"+uuid.New().String()[:6], "OTHER-BAR-"+uuid.New().String()[:6])
	require.NoError(t, err)
	candidateWrongVar := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 101, now(), now())", candidateWrongVar, otherVariantID, supplyID, supplyItemID, mustGenerateUnitCode())
	require.NoError(t, err)

	reqRuleK := inventory.ResolveReconciliationCaseRequest{
		UnitID:            &unitLive,
		ActionID:          inventory.ActionIDConfirmMissing,
		ReplacementUnitID: &candidateWrongVar,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqRuleK)
	require.Error(t, err, "Rule K: must reject replacement candidate from different variant")

	// Rule L: Candidate status is damaged / expected / written_off / shipped
	candidateDamaged := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'damaged', 2, now(), now())", candidateDamaged, variantID, supplyID, supplyItemID, mustGenerateUnitCode())
	require.NoError(t, err)

	reqRuleL := inventory.ResolveReconciliationCaseRequest{
		UnitID:            &unitLive,
		ActionID:          inventory.ActionIDConfirmMissing,
		ReplacementUnitID: &candidateDamaged,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqRuleL)
	require.Error(t, err, "Rule L: must reject replacement candidate with status damaged")

	// Rule M: Candidate is already allocated to another order
	candidateAllocated := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 3, now(), now())", candidateAllocated, variantID, supplyID, supplyItemID, mustGenerateUnitCode())
	require.NoError(t, err)
	allocIDOther := uuid.New()
	orderItemIDOther := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocIDOther, orderItemIDOther, candidateAllocated)
	require.NoError(t, err)

	reqRuleM := inventory.ResolveReconciliationCaseRequest{
		UnitID:            &unitLive,
		ActionID:          inventory.ActionIDConfirmMissing,
		ReplacementUnitID: &candidateAllocated,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqRuleM)
	require.Error(t, err, "Rule M: must reject replacement candidate that is already allocated")

	// Rule N: Candidate is the missing unit itself
	reqRuleN := inventory.ResolveReconciliationCaseRequest{
		UnitID:            &unitLive,
		ActionID:          inventory.ActionIDConfirmMissing,
		ReplacementUnitID: &unitLive,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqRuleN)
	require.Error(t, err, "Rule N: cannot use missing unit as its own replacement")

	// Rule P: Concurrency test - two concurrent attempts picking the same candidate
	candidateFree := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 4, now(), now())", candidateFree, variantID, supplyID, supplyItemID, mustGenerateUnitCode())
	require.NoError(t, err)

	// Second live missing unit
	unitLive2 := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 5, now(), now())", unitLive2, variantID, supplyID, supplyItemID, mustGenerateUnitCode())
	require.NoError(t, err)
	allocID2 := uuid.New()
	orderItemID2 := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID2, orderItemID2, unitLive2)
	require.NoError(t, err)

	var wg sync.WaitGroup
	results := make([]error, 2)
	reqs := []inventory.ResolveReconciliationCaseRequest{
		{
			UnitID:            &unitLive,
			ActionID:          inventory.ActionIDConfirmMissing,
			ReplacementUnitID: &candidateFree,
		},
		{
			UnitID:            &unitLive2,
			ActionID:          inventory.ActionIDConfirmMissing,
			ReplacementUnitID: &candidateFree,
		},
	}

	wg.Add(2)
	for i := 0; i < 2; i++ {
		idx := i
		go func() {
			defer wg.Done()
			_, results[idx] = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqs[idx])
		}()
	}
	wg.Wait()

	// Exactly one must succeed, one must fail with conflict (409)
	successCount := 0
	conflictCount := 0
	for _, resErr := range results {
		if resErr == nil {
			successCount++
		} else {
			conflictCount++
		}
	}
	require.Equal(t, 1, successCount, "Exactly one concurrent resolution must succeed")
	require.Equal(t, 1, conflictCount, "Second concurrent resolution must fail")
}

// Cases Q/W: Picked Protection
func TestReconciliationMatrix_CasesQW_PickedProtection(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unit := uuid.New()
	code := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit, variantID, supplyID, supplyItemID, code)
	require.NoError(t, err)

	allocID := uuid.New()
	orderItemID := createTestOrderItem(ctx, t, variantID, sellerID)
	now := time.Now()
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at) VALUES ($1, $2, $3, $4, now())", allocID, orderItemID, unit, now)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	// Attempting close_stale_allocation on a picked allocation must be rejected
	reqClose := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDCloseStaleAllocation,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqClose)
	require.Error(t, err)
	require.ErrorIs(t, err, inventory.ErrReconciliationConflict)
}

// Case AC: Partial Failure Rollback
func TestReconciliationMatrix_CaseAC_PartialFailureRollback(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 1 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unit := uuid.New()
	code := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit, variantID, supplyID, supplyItemID, code)
	require.NoError(t, err)

	allocID := uuid.New()
	orderItemID := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID, orderItemID, unit)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	// Missing live allocated requires replacement; passing nil replacement fails validation
	reqInvalid := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqInvalid)
	require.Error(t, err)

	// Verify complete rollback: zero resolutions recorded, unit still warehouse, allocation untouched
	var resCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_resolutions WHERE session_id = $1", sessionID).Scan(&resCount)
	require.NoError(t, err)
	require.Equal(t, 0, resCount, "Transaction must rollback cleanly: 0 resolutions recorded")

	var uStatus string
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unit).Scan(&uStatus)
	require.NoError(t, err)
	require.Equal(t, "warehouse", uStatus, "Unit status must remain warehouse")

	var allocReleased *time.Time
	err = testDB.QueryRow(ctx, "SELECT released_at FROM order_item_allocations WHERE id = $1", allocID).Scan(&allocReleased)
	require.NoError(t, err)
	require.Nil(t, allocReleased, "Allocation must not be released")
}

// Exact regression test matching real dev incident with ORD-100193:
// Completed reconciliation, missing expected warehouse ZMU, unreleased allocation with picked_at set,
// terminal delivered order + fulfillment + return restock.
func TestReconciliationMatrix_RealDevRegression_ORD100193_StaleAllocationPickedAndDelivered(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	// 1. Initial stock: total = 29, reserved = 2
	_, err := testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 29, reserved_stock = 2 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	// 2. Create historical supply and inventory unit
	supplyID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unit := uuid.New()
	code := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit, variantID, supplyID, supplyItemID, code)
	require.NoError(t, err)

	// 3. Create historical order ORD-100193 in 'delivered' status
	orderItemID := createTestOrderItem(ctx, t, variantID, sellerID)
	var orderID, fulfillmentID uuid.UUID
	err = testDB.QueryRow(ctx, "SELECT order_id, order_fulfillment_id FROM order_items WHERE id = $1", orderItemID).Scan(&orderID, &fulfillmentID)
	require.NoError(t, err)

	ordNum := "ORD-100193-" + uuid.New().String()[:6]
	_, err = testDB.Exec(ctx, "UPDATE orders SET order_number = $1, status = 'delivered' WHERE id = $2", ordNum, orderID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "UPDATE order_fulfillments SET status = 'delivered' WHERE id = $1", fulfillmentID)
	require.NoError(t, err)

	// 4. Create historical shipment in 'delivered' status
	shipmentID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO shipments (id, order_id, fulfillment_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'delivered', now(), now())", shipmentID, orderID, fulfillmentID)
	require.NoError(t, err)

	// 5. Create unreleased allocation with picked_at set (exactly like real dev!)
	allocID := uuid.New()
	pickedAt := time.Now().Add(-5 * 24 * time.Hour)
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, allocID, orderItemID, unit, pickedAt, pickedAt.Add(-2*time.Hour))
	require.NoError(t, err)

	// 6. Create historical return with 'refunded' status and restock disposition
	var userID uuid.UUID
	err = testDB.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", orderID).Scan(&userID)
	require.NoError(t, err)

	returnID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO returns (id, order_id, user_id, fulfillment_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, $4, 'refunded', 'size_mismatch', now(), now())", returnID, orderID, userID, fulfillmentID)
	require.NoError(t, err)
	returnItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at) VALUES ($1, $2, $3, 1, now())", returnItemID, returnID, orderItemID)
	require.NoError(t, err)
	returnItemUnitID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, disposition, created_at, updated_at) VALUES ($1, $2, $3, 'restock', now(), now())", returnItemUnitID, returnItemID, allocID)
	require.NoError(t, err)

	// 7. Create completed reconciliation session where unit is missing (absence)
	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	// 8. Verify resolution plan
	plan, err := repo.GetReconciliationResolutionPlan(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, plan.Cases, 1)
	c := plan.Cases[0]
	require.Equal(t, unit, c.UnitID)
	require.Equal(t, inventory.CaseTypeStaleAllocation, c.CaseType)
	require.Equal(t, "Не найдена — старое назначение", c.Title)
	require.Contains(t, c.CurrentAllocationCtx, "ORD-100193")
	require.Contains(t, c.CurrentAllocationCtx, "Доставлен")

	var staleAction *inventory.ReconciliationResolutionAction
	for i := range c.AllowedActions {
		if c.AllowedActions[i].ID == inventory.ActionIDCloseStaleAllocation {
			staleAction = &c.AllowedActions[i]
			break
		}
	}
	require.NotNil(t, staleAction)
	require.True(t, staleAction.Enabled, "close_stale_allocation must be ENABLED in resolution plan")

	// 9. Guard check: Attempting confirm_missing directly before closing stale allocation must return 409
	reqPrematureMissing := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqPrematureMissing)
	require.Error(t, err)
	require.ErrorIs(t, err, inventory.ErrReconciliationConflict, "Attempting confirm_missing while stale allocation exists must return 409")

	// 10. Resolve stale allocation: MUST SUCCEED (previously failed with 409 due to picked_at != nil)
	reqClose := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDCloseStaleAllocation,
	}
	updatedPlan, err := svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqClose)
	require.NoError(t, err, "Closing stale allocation on delivered order must succeed even when picked_at was populated")
	require.NotNil(t, updatedPlan)

	// 11. Verify state after close_stale_allocation
	// - Allocation is released
	var releasedAt *time.Time
	var relReason *string
	err = testDB.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocID).Scan(&releasedAt, &relReason)
	require.NoError(t, err)
	require.NotNil(t, releasedAt, "Allocation must be released")
	require.Equal(t, "inventory_reconciliation", *relReason)

	// - Unit remains warehouse
	var uStatus string
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unit).Scan(&uStatus)
	require.NoError(t, err)
	require.Equal(t, "warehouse", uStatus, "Unit must remain warehouse after stale allocation is released")

	// - total_stock remains 29 (unchanged), reserved_stock decrements 2 -> 1
	var totStock, resStock int
	err = testDB.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totStock, &resStock)
	require.NoError(t, err)
	require.Equal(t, 29, totStock, "total_stock must not change when stale allocation is released")
	require.Equal(t, 1, resStock, "reserved_stock must decrement by 1 (2 -> 1)")

	// - Traceability event recorded
	trc, err := repo.GetAdminInventoryUnitTraceability(ctx, code)
	require.NoError(t, err)
	foundStaleEvt := false
	for _, ev := range trc.Timeline {
		if ev.EventName == "Старое назначение освобождено" {
			foundStaleEvt = true
			require.Equal(t, "reconciliation_stale_allocation_released", ev.Type)
			break
		}
	}
	require.True(t, foundStaleEvt, "Traceability timeline must contain 'Старое назначение освобождено'")

	// 12. Re-fetch resolution plan: Case reclassifies to missing_free with confirm_missing enabled!
	plan2, err := repo.GetReconciliationResolutionPlan(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, plan2.Cases, 1)
	c2 := plan2.Cases[0]
	require.Equal(t, inventory.CaseTypeMissingFree, c2.CaseType)
	require.Equal(t, "Единица не найдена", c2.Title)

	var missingAction *inventory.ReconciliationResolutionAction
	for i := range c2.AllowedActions {
		if c2.AllowedActions[i].ID == inventory.ActionIDConfirmMissing {
			missingAction = &c2.AllowedActions[i]
			break
		}
	}
	require.NotNil(t, missingAction)
	require.True(t, missingAction.Enabled, "confirm_missing must now be ENABLED")

	// 13. Confirm missing write-off
	reqMissing := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqMissing)
	require.NoError(t, err)

	// - Unit status becomes written_off
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unit).Scan(&uStatus)
	require.NoError(t, err)
	require.Equal(t, "written_off", uStatus)

	// - total_stock decrements 29 -> 28
	err = testDB.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totStock)
	require.NoError(t, err)
	require.Equal(t, 28, totStock, "total_stock must decrement by 1 (29 -> 28) after confirm_missing")

	// 14. Genuinely changed state returns 409
	// Attempting to resolve already written off unit from another session returns 409
	otherSessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, otherSessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, otherSessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, otherSessionID, "review", "completed", adminID))
	_, err = svc.ResolveReconciliationCase(ctx, otherSessionID, adminID, reqMissing)
	require.Error(t, err)
	require.ErrorIs(t, err, inventory.ErrReconciliationConflict)
}
