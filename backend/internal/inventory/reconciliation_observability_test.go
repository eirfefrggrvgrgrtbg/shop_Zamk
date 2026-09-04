package inventory_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
)

type reconLogCapture struct {
	buf bytes.Buffer
}

func (c *reconLogCapture) entries() []map[string]interface{} {
	lines := bytes.Split(bytes.TrimSpace(c.buf.Bytes()), []byte("\n"))
	var result []map[string]interface{}
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(l, &m); err == nil {
			result = append(result, m)
		}
	}
	return result
}

func (c *reconLogCapture) findEvents(eventName string) []map[string]interface{} {
	var matches []map[string]interface{}
	for _, e := range c.entries() {
		if name, ok := e["event_name"].(string); ok && name == eventName {
			matches = append(matches, e)
		}
	}
	return matches
}

func (c *reconLogCapture) clear() {
	c.buf.Reset()
}

func TestReconciliationObservability_PostCommitContracts(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	cap := &reconLogCapture{}
	jsonHandler := slog.NewJSONHandler(&cap.buf, &slog.HandlerOptions{})
	logger := slog.New(jsonHandler)
	svc.SetLogger(logger)

	// Fixture setup:
	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 10, reserved_stock = 2 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	// Unit 1: Missing with stale allocation (order delivered)
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

	// Unit 2: Missing with live allocation (order assembling)
	unit2 := uuid.New()
	code2 := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 2, now(), now())", unit2, variantID, supplyID, supplyItemID, code2)
	require.NoError(t, err)

	allocID2 := uuid.New()
	orderItemID2 := createTestOrderItem(ctx, t, variantID, sellerID)
	var liveOrderNumber string
	err = testDB.QueryRow(ctx, "SELECT order_number FROM orders WHERE id = (SELECT order_id FROM order_items WHERE id = $1)", orderItemID2).Scan(&liveOrderNumber)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID2, orderItemID2, unit2)
	require.NoError(t, err)

	// Unit 3: Free warehouse candidate for replacement
	unit3 := uuid.New()
	code3 := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 3, now(), now())", unit3, variantID, supplyID, supplyItemID, code3)
	require.NoError(t, err)

	// Unit 4: Missing free unit (not scanned in session)
	unit4 := uuid.New()
	code4 := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 4, now(), now())", unit4, variantID, supplyID, supplyItemID, code4)
	require.NoError(t, err)

	// Start reconciliation session and move to completed
	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID))

	// =========================================================================
	// (A) COMMITTED STALE ALLOCATION RELEASE:
	// exactly 1 inventory.stale_allocation_released & 1 warehouse.reconciliation_discrepancy_resolved
	// =========================================================================
	cap.clear()
	reqStale := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit1,
		ActionID: inventory.ActionIDCloseStaleAllocation,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqStale)
	require.NoError(t, err)

	staleEvents := cap.findEvents("inventory.stale_allocation_released")
	require.Len(t, staleEvents, 1, "exactly 1 inventory.stale_allocation_released must be emitted")
	assert.Equal(t, "success", staleEvents[0]["result"])
	assert.Equal(t, code1, staleEvents[0]["zmu"])
	assert.Equal(t, allocID1.String(), staleEvents[0]["allocation_id"])

	resolvedEvents := cap.findEvents("warehouse.reconciliation_discrepancy_resolved")
	require.Len(t, resolvedEvents, 1, "exactly 1 warehouse.reconciliation_discrepancy_resolved must be emitted")
	assert.Equal(t, "success", resolvedEvents[0]["result"])
	assert.Equal(t, sessionID.String(), resolvedEvents[0]["reconciliation_session_id"])
	assert.Equal(t, unit1.String(), resolvedEvents[0]["discrepancy_id"])
	assert.Equal(t, inventory.ActionIDCloseStaleAllocation, resolvedEvents[0]["resolution_action"])

	// =========================================================================
	// (D) IDEMPOTENT REPEAT:
	// zero duplicate mutation SUCCESS events
	// =========================================================================
	cap.clear()
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqStale)
	require.NoError(t, err)

	assert.Empty(t, cap.findEvents("inventory.stale_allocation_released"), "idempotent retry must emit 0 stale_allocation_released")
	assert.Empty(t, cap.findEvents("warehouse.reconciliation_discrepancy_resolved"), "idempotent retry must emit 0 discrepancy_resolved")
	assert.Empty(t, cap.findEvents("inventory.unit_written_off"), "idempotent retry must emit 0 unit_written_off")
	assert.Empty(t, cap.findEvents("inventory.allocation_replaced"), "idempotent retry must emit 0 allocation_replaced")

	// =========================================================================
	// (B) COMMITTED MISSING WRITE-OFF:
	// exactly 1 inventory.unit_written_off & 1 warehouse.reconciliation_discrepancy_resolved
	// =========================================================================
	cap.clear()
	reqMissing := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit4,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqMissing)
	require.NoError(t, err)

	writeoffEvents := cap.findEvents("inventory.unit_written_off")
	require.Len(t, writeoffEvents, 1, "exactly 1 inventory.unit_written_off must be emitted")
	assert.Equal(t, "success", writeoffEvents[0]["result"])
	assert.Equal(t, code4, writeoffEvents[0]["zmu"])
	assert.Equal(t, unit4.String(), writeoffEvents[0]["inventory_unit_id"])
	assert.Equal(t, "reconciliation_missing", writeoffEvents[0]["reason"])

	resolvedMissingEvents := cap.findEvents("warehouse.reconciliation_discrepancy_resolved")
	require.Len(t, resolvedMissingEvents, 1, "exactly 1 warehouse.reconciliation_discrepancy_resolved must be emitted")
	assert.Equal(t, "success", resolvedMissingEvents[0]["result"])
	assert.Equal(t, sessionID.String(), resolvedMissingEvents[0]["reconciliation_session_id"])
	assert.Equal(t, unit4.String(), resolvedMissingEvents[0]["discrepancy_id"])

	// =========================================================================
	// (C) TRANSACTION ROLLBACK:
	// zero SUCCESS mutation events on failure
	// =========================================================================
	cap.clear()
	nonExistentUnitID := uuid.New()
	reqInvalid := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &nonExistentUnitID,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqInvalid)
	require.Error(t, err)

	assert.Empty(t, cap.findEvents("inventory.stale_allocation_released"), "failed tx must emit 0 stale_allocation_released")
	assert.Empty(t, cap.findEvents("inventory.unit_written_off"), "failed tx must emit 0 unit_written_off")
	assert.Empty(t, cap.findEvents("inventory.allocation_replaced"), "failed tx must emit 0 allocation_replaced")
	assert.Empty(t, cap.findEvents("warehouse.reconciliation_discrepancy_resolved"), "failed tx must emit 0 discrepancy_resolved")

	// =========================================================================
	// (E) LIVE ALLOCATION REPLACEMENT:
	// exactly 1 inventory.allocation_replaced with structured metadata:
	// missing_zmu, replacement_zmu, order_number, reconciliation_session_id
	// =========================================================================
	cap.clear()
	reqRepl := inventory.ResolveReconciliationCaseRequest{
		UnitID:            &unit2,
		ActionID:          inventory.ActionIDConfirmMissing,
		ReplacementUnitID: &unit3,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqRepl)
	require.NoError(t, err)

	replEvents := cap.findEvents("inventory.allocation_replaced")
	require.Len(t, replEvents, 1, "exactly 1 inventory.allocation_replaced must be emitted")
	assert.Equal(t, "success", replEvents[0]["result"])
	assert.Equal(t, code2, replEvents[0]["missing_zmu"])
	assert.Equal(t, code3, replEvents[0]["replacement_zmu"])
	assert.Equal(t, liveOrderNumber, replEvents[0]["order_number"])
	assert.Equal(t, sessionID.String(), replEvents[0]["reconciliation_session_id"])

	// Also verify that unit2 was written off and discrepancy resolved
	require.Len(t, cap.findEvents("inventory.unit_written_off"), 1)
	require.Len(t, cap.findEvents("warehouse.reconciliation_discrepancy_resolved"), 1)
}
