package supplies_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func TestReceivingSessionModeDetection(t *testing.T) {
	tc := setupTestContext(t)

	// A. Serialized session reports mode=serialized
	supply := createShippedSupply(t, tc)
	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start serialized session: %v", err)
	}
	if session.ReceivingMode != "serialized" {
		t.Fatalf("expected ReceivingMode 'serialized', got '%s'", session.ReceivingMode)
	}

	// B. Legacy reports mode=legacy
	legacySupplyID := uuid.New()
	legacySupplyNumber := "SUP-LEGACY-002"
	legacyQRToken := "qr-legacy-mode-test"
	now := time.Now().UTC()

	testDB.Exec(tc.Ctx, `INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at) VALUES ($1, $2, $3, 'ready_to_ship', 'carrier_delivery', $4, $5, $5)`, legacySupplyID, legacySupplyNumber, tc.SellerID, legacyQRToken, now)
	itemID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, $4, $4)`, itemID, legacySupplyID, tc.Variant1, now)
	boxID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at) VALUES ($1, $2, 'BOX-01', 'box-qr-legacy-2', $3)`, boxID, legacySupplyID, now)
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_box_items (box_id, supply_item_id, quantity) VALUES ($1, $2, 10)`, boxID, itemID)

	_, err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, legacySupplyID)
	if err != nil {
		t.Fatalf("failed to mark legacy supply shipped: %v", err)
	}
	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, legacySupplyID)
	if err != nil {
		t.Fatalf("failed to mark legacy supply arrived: %v", err)
	}

	legacySession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, legacyQRToken)
	if err != nil {
		t.Fatalf("failed to start legacy session: %v", err)
	}
	if legacySession.ReceivingMode != "legacy" {
		t.Fatalf("expected ReceivingMode 'legacy', got '%s'", legacySession.ReceivingMode)
	}
}

func TestSerializedReceivingScanAndUndoLifecycle(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil || len(units) != 10 {
		t.Fatalf("failed to list units: %v, count=%d", err, len(units))
	}

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	// Capture initial stock
	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	// C. Scan OK increments session OK counter only
	unit1 := units[0]
	resp1, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit1.UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("failed valid OK scan: %v", err)
	}
	if resp1.SessionOk != 1 || resp1.SessionDamaged != 0 || resp1.SessionScanned != 1 {
		t.Fatalf("expected OK=1, Damaged=0, Scanned=1; got OK=%d, Damaged=%d, Scanned=%d", resp1.SessionOk, resp1.SessionDamaged, resp1.SessionScanned)
	}

	// D. Damaged scan increments damaged counter only
	unit2 := units[1]
	resp2, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit2.UnitCode,
		Condition: "damaged",
	})
	if err != nil {
		t.Fatalf("failed valid damaged scan: %v", err)
	}
	if resp2.SessionOk != 1 || resp2.SessionDamaged != 1 || resp2.SessionScanned != 2 {
		t.Fatalf("expected OK=1, Damaged=1, Scanned=2; got OK=%d, Damaged=%d, Scanned=%d", resp2.SessionOk, resp2.SessionDamaged, resp2.SessionScanned)
	}

	// M. Recent scans ordering newest-first
	scans, err := tc.Service.ListRecentSerializedScans(tc.Ctx, tc.AdminID, session.ID, 10)
	if err != nil {
		t.Fatalf("failed to list recent scans: %v", err)
	}
	if len(scans) != 2 {
		t.Fatalf("expected 2 recent scans, got %d", len(scans))
	}
	if scans[0].ScanID != resp2.ScanID || scans[1].ScanID != resp1.ScanID {
		t.Fatalf("expected newest-first ordering [resp2, resp1], got [%s, %s]", scans[0].ScanID, scans[1].ScanID)
	}

	// E. Undo OK decrements OK counter
	undoResp1, err := tc.Service.UndoSerializedScan(tc.Ctx, tc.AdminID, session.ID, resp1.ScanID)
	if err != nil {
		t.Fatalf("failed to undo OK scan: %v", err)
	}
	if undoResp1.SessionOk != 0 || undoResp1.SessionDamaged != 1 || undoResp1.SessionScanned != 1 {
		t.Fatalf("expected after undo: OK=0, Damaged=1, Scanned=1; got OK=%d, Damaged=%d, Scanned=%d", undoResp1.SessionOk, undoResp1.SessionDamaged, undoResp1.SessionScanned)
	}

	// G. Undo does not mutate stock
	var stockAfterUndo int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterUndo)
	if stockAfterUndo != initialStock {
		t.Fatalf("expected stock unchanged (%d), got %d", initialStock, stockAfterUndo)
	}

	// H. Undo does not change inventory_unit.status
	unit1db, _ := tc.Repo.GetInventoryUnitByCode(tc.Ctx, unit1.UnitCode)
	if unit1db.Status != "expected" {
		t.Fatalf("expected unit status 'expected', got '%s'", unit1db.Status)
	}

	// I. Rescan same ZMU after undo succeeds
	resp1Rescan, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit1.UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("failed rescan of unit1: %v", err)
	}
	if resp1Rescan.SessionOk != 1 || resp1Rescan.SessionDamaged != 1 || resp1Rescan.SessionScanned != 2 {
		t.Fatalf("expected after rescan: OK=1, Damaged=1, Scanned=2; got OK=%d, Damaged=%d, Scanned=%d", resp1Rescan.SessionOk, resp1Rescan.SessionDamaged, resp1Rescan.SessionScanned)
	}

	// J. History has one voided + one active scan
	var totalUnit1Scans, activeUnit1Scans, voidedUnit1Scans int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE inventory_unit_id = $1", unit1.ID).Scan(&totalUnit1Scans)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE inventory_unit_id = $1 AND voided_at IS NULL", unit1.ID).Scan(&activeUnit1Scans)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE inventory_unit_id = $1 AND voided_at IS NOT NULL", unit1.ID).Scan(&voidedUnit1Scans)
	if totalUnit1Scans != 2 || activeUnit1Scans != 1 || voidedUnit1Scans != 1 {
		t.Fatalf("expected total=2, active=1, voided=1 for unit1; got total=%d, active=%d, voided=%d", totalUnit1Scans, activeUnit1Scans, voidedUnit1Scans)
	}

	// Recent scans endpoint returns history including voided scans
	recentScans, err := tc.Service.ListRecentSerializedScans(tc.Ctx, tc.AdminID, session.ID, 10)
	if err != nil {
		t.Fatalf("failed to list recent scans: %v", err)
	}
	var unit1RecentVoided, unit1RecentActive int
	for _, sc := range recentScans {
		if sc.UnitCode == unit1.UnitCode {
			if sc.VoidedAt != nil {
				unit1RecentVoided++
			} else {
				unit1RecentActive++
			}
		}
	}
	if unit1RecentActive != 1 || unit1RecentVoided != 1 {
		t.Fatalf("expected 1 active and 1 voided in recent scans for unit1, got active=%d voided=%d", unit1RecentActive, unit1RecentVoided)
	}

	// F. Undo damaged decrements damaged counter
	undoResp2, err := tc.Service.UndoSerializedScan(tc.Ctx, tc.AdminID, session.ID, resp2.ScanID)
	if err != nil {
		t.Fatalf("failed to undo damaged scan: %v", err)
	}
	if undoResp2.SessionOk != 1 || undoResp2.SessionDamaged != 0 || undoResp2.SessionScanned != 1 {
		t.Fatalf("expected after undo damaged: OK=1, Damaged=0, Scanned=1; got OK=%d, Damaged=%d, Scanned=%d", undoResp2.SessionOk, undoResp2.SessionDamaged, undoResp2.SessionScanned)
	}

	// K. Second undo rejected
	_, err = tc.Service.UndoSerializedScan(tc.Ctx, tc.AdminID, session.ID, resp2.ScanID)
	if !errors.Is(err, supplies.ErrScanAlreadyVoided) {
		t.Fatalf("expected ErrScanAlreadyVoided on second undo, got %v", err)
	}

	// Unknown scan undo rejected
	_, err = tc.Service.UndoSerializedScan(tc.Ctx, tc.AdminID, session.ID, uuid.New())
	if !errors.Is(err, supplies.ErrScanNotFound) {
		t.Fatalf("expected ErrScanNotFound for random scanId, got %v", err)
	}

	// L. Undo finalized session rejected (tested canonically on a finalized legacy session)
	legacyFinalizedSupply := createLegacyShippedSupply(t, tc, 5)
	legacyFinalizedSession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *legacyFinalizedSupply.QRToken)
	if err != nil {
		t.Fatalf("failed to start legacy session: %v", err)
	}
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, legacyFinalizedSession.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("failed to finalize legacy session: %v", err)
	}

	_, err = tc.Service.UndoSerializedScan(tc.Ctx, tc.AdminID, legacyFinalizedSession.ID, resp1Rescan.ScanID)
	if !errors.Is(err, supplies.ErrReceivingSessionFinalized) {
		t.Fatalf("expected ErrReceivingSessionFinalized for completed session, got %v", err)
	}
}

func TestSerializedFinalize_WithDiscrepancies(t *testing.T) {
	tc := setupTestContext(t)

	// 1. Create serialized supply with 5 units
	carrier := "СДЭК"
	tracking := "121212123241"
	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    &carrier,
		TrackingNumber: &tracking,
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 5},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}
	_, err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("failed to mark shipped: %v", err)
	}
	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supply.ID)
	if err != nil {
		t.Fatalf("failed to mark arrived: %v", err)
	}

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start serialized session: %v", err)
	}

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil || len(units) != 5 {
		t.Fatalf("expected 5 units, got %d (err: %v)", len(units), err)
	}

	// Capture initial stock
	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	// Scan: 3 units OK, 1 unit Damaged, leave 1 unscanned
	for i := 0; i < 3; i++ {
		_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  units[i].UnitCode,
			Condition: "ok",
		})
		if err != nil {
			t.Fatalf("failed to scan unit %d as ok: %v", i, err)
		}
	}

	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[3].UnitCode,
		Condition: "damaged",
	})
	if err != nil {
		t.Fatalf("failed to scan unit 3 as damaged: %v", err)
	}
	// units[4] is left unscanned

	// Before finalize: all 5 units must still be expected, stock unchanged
	for _, u := range units {
		var status string
		var recSessionID *uuid.UUID
		err = testDB.QueryRow(tc.Ctx, "SELECT status, receiving_session_id FROM inventory_units WHERE id = $1", u.ID).Scan(&status, &recSessionID)
		if err != nil || status != "expected" || recSessionID != nil {
			t.Fatalf("before finalize: expected unit %s status 'expected' and nil session, got status='%s', session=%v", u.UnitCode, status, recSessionID)
		}
	}

	var stockBeforeFinalize int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockBeforeFinalize)
	if stockBeforeFinalize != initialStock {
		t.Fatalf("before finalize: expected stock unchanged (%d), got %d", initialStock, stockBeforeFinalize)
	}

	// Finalize receiving session
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("finalize receiving failed: %v", err)
	}

	// After finalize:
	// 3 OK units -> warehouse
	for i := 0; i < 3; i++ {
		var status string
		var recSessionID *uuid.UUID
		err = testDB.QueryRow(tc.Ctx, "SELECT status, receiving_session_id FROM inventory_units WHERE id = $1", units[i].ID).Scan(&status, &recSessionID)
		if err != nil || status != "warehouse" || recSessionID == nil || *recSessionID != session.ID {
			t.Fatalf("unit %d: expected status 'warehouse' and session %s, got status='%s', session=%v", i, session.ID, status, recSessionID)
		}
	}

	// 1 Damaged unit -> damaged
	var damagedStatus string
	var damagedSessionID *uuid.UUID
	err = testDB.QueryRow(tc.Ctx, "SELECT status, receiving_session_id FROM inventory_units WHERE id = $1", units[3].ID).Scan(&damagedStatus, &damagedSessionID)
	if err != nil || damagedStatus != "damaged" || damagedSessionID == nil || *damagedSessionID != session.ID {
		t.Fatalf("unit 3: expected status 'damaged' and session %s, got status='%s', session=%v", session.ID, damagedStatus, damagedSessionID)
	}

	// 1 Unscanned unit -> expected
	var unscannedStatus string
	var unscannedSessionID *uuid.UUID
	err = testDB.QueryRow(tc.Ctx, "SELECT status, receiving_session_id FROM inventory_units WHERE id = $1", units[4].ID).Scan(&unscannedStatus, &unscannedSessionID)
	if err != nil || unscannedStatus != "expected" || unscannedSessionID != nil {
		t.Fatalf("unit 4: expected status 'expected' and nil session, got status='%s', session=%v", unscannedStatus, unscannedSessionID)
	}

	// Stock onHand: +3 exactly
	var stockAfterFinalize int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterFinalize)
	if stockAfterFinalize != initialStock+3 {
		t.Fatalf("expected stock +3 (%d), got %d", initialStock+3, stockAfterFinalize)
	}

	// Supply item counters: accepted=3, damaged=1, missing=1
	var acceptedQty, damagedQty, missingQty int
	err = testDB.QueryRow(tc.Ctx, "SELECT accepted_quantity, damaged_quantity, missing_quantity FROM seller_supply_items WHERE supply_id = $1", supply.ID).Scan(&acceptedQty, &damagedQty, &missingQty)
	if err != nil || acceptedQty != 3 || damagedQty != 1 || missingQty != 1 {
		t.Fatalf("expected item counters accepted=3, damaged=1, missing=1; got accepted=%d, damaged=%d, missing=%d", acceptedQty, damagedQty, missingQty)
	}

	// Stock movements: exactly 1 receipt movement of +3 for this supply
	var movCount, movQty int
	err = testDB.QueryRow(tc.Ctx, "SELECT COUNT(*), COALESCE(SUM(quantity), 0) FROM stock_movements WHERE reference_type = 'supply' AND reference_id = $1", supply.ID).Scan(&movCount, &movQty)
	if err != nil || movCount != 1 || movQty != 3 {
		t.Fatalf("expected 1 stock movement with qty=3, got count=%d, qty=%d", movCount, movQty)
	}

	// Supply terminal status: completed_with_discrepancies
	finalSupply, err := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if err != nil || finalSupply.Status != "completed_with_discrepancies" {
		t.Fatalf("expected supply status 'completed_with_discrepancies', got '%s'", finalSupply.Status)
	}

	// Double finalize: must fail idempotently without duplicate stock/movement mutations
	err2 := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err2 == nil || err2.Error() != "session is not active" {
		t.Fatalf("second finalize should fail with 'session is not active', got: %v", err2)
	}

	var stockAfterSecondFinalize int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterSecondFinalize)
	if stockAfterSecondFinalize != initialStock+3 {
		t.Fatalf("after second finalize: expected stock unchanged (%d), got %d", initialStock+3, stockAfterSecondFinalize)
	}

	var movCountAfterSecond int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM stock_movements WHERE reference_type = 'supply' AND reference_id = $1", supply.ID).Scan(&movCountAfterSecond)
	if movCountAfterSecond != 1 {
		t.Fatalf("after second finalize: expected exactly 1 stock movement, got %d", movCountAfterSecond)
	}
}

func TestSerializedFinalize_AllOK_Completed(t *testing.T) {
	tc := setupTestContext(t)

	// Create supply with 5 units
	carrier := "СДЭК"
	tracking := "121212123241"
	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    &carrier,
		TrackingNumber: &tracking,
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 5},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}
	_, _ = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	_ = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supply.ID)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	// Capture initial stock
	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if len(units) != 5 {
		t.Fatalf("expected 5 units, got %d", len(units))
	}
	for _, u := range units {
		_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  u.UnitCode,
			Condition: "ok",
		})
		if err != nil {
			t.Fatalf("failed to scan unit %s as ok: %v", u.UnitCode, err)
		}
	}

	// Finalize session
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	// All 5 units -> warehouse with receiving_session_id
	for _, u := range units {
		var status string
		var recSessionID *uuid.UUID
		testDB.QueryRow(tc.Ctx, "SELECT status, receiving_session_id FROM inventory_units WHERE id = $1", u.ID).Scan(&status, &recSessionID)
		if status != "warehouse" || recSessionID == nil || *recSessionID != session.ID {
			t.Fatalf("expected unit %s status 'warehouse' and session %s, got status='%s', session=%v", u.UnitCode, session.ID, status, recSessionID)
		}
	}

	// Stock onHand: +5 exactly
	var stockAfter int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfter)
	if stockAfter != initialStock+5 {
		t.Fatalf("expected stock +5 (%d), got %d", initialStock+5, stockAfter)
	}

	// Accepted quantity: 5 exactly, damaged: 0, missing: 0
	var acceptedQty, damagedQty, missingQty int
	err = testDB.QueryRow(tc.Ctx, "SELECT accepted_quantity, damaged_quantity, missing_quantity FROM seller_supply_items WHERE supply_id = $1", supply.ID).Scan(&acceptedQty, &damagedQty, &missingQty)
	if err != nil || acceptedQty != 5 || damagedQty != 0 || missingQty != 0 {
		t.Fatalf("expected accepted=5, damaged=0, missing=0; got accepted=%d, damaged=%d, missing=%d", acceptedQty, damagedQty, missingQty)
	}

	// Stock movement: 1 receipt movement of +5
	var movCount, movQty int
	err = testDB.QueryRow(tc.Ctx, "SELECT COUNT(*), COALESCE(SUM(quantity), 0) FROM stock_movements WHERE reference_type = 'supply' AND reference_id = $1", supply.ID).Scan(&movCount, &movQty)
	if err != nil || movCount != 1 || movQty != 5 {
		t.Fatalf("expected 1 stock movement with qty=5, got count=%d, qty=%d", movCount, movQty)
	}

	// Supply status -> completed (no discrepancies)
	finalSupply, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if finalSupply.Status != "completed" {
		t.Fatalf("expected supply status 'completed', got '%s'", finalSupply.Status)
	}

	// Session status -> completed
	finalSession, _ := tc.Repo.GetSessionByID(tc.Ctx, session.ID)
	if finalSession.Status != "completed" {
		t.Fatalf("expected session status 'completed', got '%s'", finalSession.Status)
	}

	// Double finalize: must fail idempotently without duplicate stock/movement mutations
	err2 := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err2 == nil || err2.Error() != "session is not active" {
		t.Fatalf("second finalize should fail with 'session is not active', got: %v", err2)
	}

	var stockAfterSecondFinalize int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterSecondFinalize)
	if stockAfterSecondFinalize != initialStock+5 {
		t.Fatalf("after second finalize: expected stock unchanged (%d), got %d", initialStock+5, stockAfterSecondFinalize)
	}
}

func TestSerializedFinalize_RollbackAtomicity(t *testing.T) {
	tc := setupTestContext(t)

	// Create supply with 4 units
	carrier := "СДЭК"
	tracking := "121212123241"
	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    &carrier,
		TrackingNumber: &tracking,
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 4},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}
	_, _ = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	_ = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supply.ID)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	_, _ = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "ok",
	})
	_, _ = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[1].UnitCode,
		Condition: "damaged",
	})

	// Add temporary test constraint to force a DB failure at the very end of the transaction (when updating supply status)
	_, err = testDB.Exec(tc.Ctx, "ALTER TABLE seller_supplies ADD CONSTRAINT test_prevent_complete CHECK (status != 'completed_with_discrepancies')")
	if err != nil {
		t.Fatalf("failed to add test constraint: %v", err)
	}
	defer testDB.Exec(tc.Ctx, "ALTER TABLE seller_supplies DROP CONSTRAINT IF EXISTS test_prevent_complete")

	// Attempt finalize -> must fail on CHECK constraint and rollback entire transaction
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err == nil {
		t.Fatalf("expected finalize to fail on test constraint violation, got nil")
	}

	// Verify rollback integrity:
	// 1. All units must remain 'expected' with nil receiving_session_id
	for _, u := range units {
		var status string
		var recSessionID *uuid.UUID
		err = testDB.QueryRow(tc.Ctx, "SELECT status, receiving_session_id FROM inventory_units WHERE id = $1", u.ID).Scan(&status, &recSessionID)
		if err != nil || status != "expected" || recSessionID != nil {
			t.Fatalf("rollback: unit %s should remain 'expected' with nil session, got status='%s', session=%v", u.UnitCode, status, recSessionID)
		}
	}

	// 2. Stock must not be incremented
	var stock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stock)
	if stock != 0 {
		t.Fatalf("rollback: stock should be 0, got %d", stock)
	}

	// 3. No stock movements created
	var movCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM stock_movements WHERE reference_id = $1", supply.ID).Scan(&movCount)
	if movCount != 0 {
		t.Fatalf("rollback: stock movements should be 0, got %d", movCount)
	}

	// 4. Session status must still be 'active'
	currentSession, _ := tc.Repo.GetSessionByID(tc.Ctx, session.ID)
	if currentSession.Status != "active" {
		t.Fatalf("rollback: session should still be 'active', got '%s'", currentSession.Status)
	}

	// 5. Supply status must still be 'receiving'
	currentSupply, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if currentSupply.Status != "receiving" {
		t.Fatalf("rollback: supply should still be 'receiving', got '%s'", currentSupply.Status)
	}
}

func TestLegacyFinalizeAllowed(t *testing.T) {
	tc := setupTestContext(t)

	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	legacySupplyID := uuid.New()
	legacySupplyNumber := "SUP-LEGACY-003"
	legacyQRToken := "qr-legacy-finalize-test"
	now := time.Now().UTC()

	testDB.Exec(tc.Ctx, `INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at) VALUES ($1, $2, $3, 'ready_to_ship', 'carrier_delivery', $4, $5, $5)`, legacySupplyID, legacySupplyNumber, tc.SellerID, legacyQRToken, now)
	itemID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, $4, $4)`, itemID, legacySupplyID, tc.Variant1, now)
	boxID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at) VALUES ($1, $2, 'BOX-01', 'box-qr-legacy-3', $3)`, boxID, legacySupplyID, now)
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_box_items (box_id, supply_item_id, quantity) VALUES ($1, $2, 10)`, boxID, itemID)

	_, _ = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, legacySupplyID)
	_ = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, legacySupplyID)

	legacySession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, legacyQRToken)
	if err != nil {
		t.Fatalf("failed to start legacy session: %v", err)
	}

	err = tc.Service.RecordScan(tc.Ctx, tc.AdminID, legacySession.ID, supplies.RecordReceivingScanRequest{
		VariantID: tc.Variant1,
		Quantity:  8,
		IsDamage:  false,
	})
	if err != nil {
		t.Fatalf("failed to record legacy scan: %v", err)
	}

	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, legacySession.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("expected legacy finalize to succeed, got %v", err)
	}

	// Verify legacy supply completed and stock incremented by 8
	var legacyStockAfter int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&legacyStockAfter)
	if legacyStockAfter != initialStock+8 {
		t.Fatalf("expected legacy stock incremented to %d, got %d", initialStock+8, legacyStockAfter)
	}
}

func TestSerializedReceivingFoundation(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to list units: %v", err)
	}
	if len(units) != 10 {
		t.Fatalf("expected 10 units, got %d", len(units))
	}

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	// Capture initial stock
	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	// Valid ZMU scan creates one scan event
	unit1 := units[0]
	resp, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit1.UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("failed valid scan: %v", err)
	}
	if resp.SessionOk != 1 || resp.SessionScanned != 1 {
		t.Fatalf("expected ok=1 scanned=1, got ok=%d scanned=%d", resp.SessionOk, resp.SessionScanned)
	}
	if resp.ScanID == uuid.Nil {
		t.Fatalf("expected valid scan id")
	}

	// Response field enrichment verification (Requirement 5 & 8)
	if resp.ProductTitle != "Test Product" {
		t.Fatalf("expected ProductTitle 'Test Product', got '%s'", resp.ProductTitle)
	}
	if resp.SellerSKU == nil || *resp.SellerSKU != "SKU-TEST-1" {
		t.Fatalf("expected SellerSKU 'SKU-TEST-1', got %v", resp.SellerSKU)
	}
	if resp.VariantBarcode == nil || *resp.VariantBarcode != "ZMK-TEST-1" {
		t.Fatalf("expected VariantBarcode 'ZMK-TEST-1', got %v", resp.VariantBarcode)
	}
	if resp.ColorName == nil || *resp.ColorName != "Черный" {
		t.Fatalf("expected ColorName 'Черный', got %v", resp.ColorName)
	}
	if resp.SizeName == nil || *resp.SizeName != "M" {
		t.Fatalf("expected SizeName 'M', got %v", resp.SizeName)
	}
	if resp.UnitCode != unit1.UnitCode {
		t.Fatalf("expected UnitCode '%s', got '%s'", unit1.UnitCode, resp.UnitCode)
	}
	if resp.Condition != "ok" {
		t.Fatalf("expected Condition 'ok', got '%s'", resp.Condition)
	}
	if resp.ProductVariantID != tc.Variant1 {
		t.Fatalf("expected ProductVariantID %s, got %s", tc.Variant1, resp.ProductVariantID)
	}

	// Unit remains status=expected
	unit1db, _ := tc.Repo.GetInventoryUnitByCode(tc.Ctx, unit1.UnitCode)
	if unit1db.Status != "expected" {
		t.Fatalf("expected status=expected, got %s", unit1db.Status)
	}

	// Stock unchanged after scan
	var stockAfterScan1 int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterScan1)
	if stockAfterScan1 != initialStock {
		t.Fatalf("expected stock unchanged (%d), got %d", initialStock, stockAfterScan1)
	}

	// Same ZMU second scan: unit_already_scanned
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit1.UnitCode,
		Condition: "damaged",
	})
	if !errors.Is(err, supplies.ErrUnitAlreadyScanned) {
		t.Fatalf("expected ErrUnitAlreadyScanned, got %v", err)
	}

	// Active event count remains 1
	var count int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE inventory_unit_id = $1 AND voided_at IS NULL", unit1.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 active event, got %d", count)
	}

	// Two concurrent scans same ZMU: one PASS, one duplicate
	unit2 := units[1]
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
				UnitCode:  unit2.UnitCode,
				Condition: "ok",
			})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)

	successCount, failCount := 0, 0
	for e := range errs {
		if e == nil {
			successCount++
		} else if errors.Is(e, supplies.ErrUnitAlreadyScanned) {
			failCount++
		}
	}
	if successCount != 1 || failCount != 1 {
		t.Fatalf("expected 1 success and 1 duplicate, got %d, %d", successCount, failCount)
	}

	var countUnit2 int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE inventory_unit_id = $1 AND voided_at IS NULL", unit2.ID).Scan(&countUnit2)
	if countUnit2 != 1 {
		t.Fatalf("expected 1 active event for unit2, got %d", countUnit2)
	}

	// Damaged condition stored correctly
	unit3 := units[2]
	respDmg, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit3.UnitCode,
		Condition: "damaged",
	})
	if err != nil {
		t.Fatalf("failed damaged scan: %v", err)
	}
	if respDmg.SessionDamaged != 1 || respDmg.SessionScanned != 3 {
		t.Fatalf("expected damaged=1 scanned=3, got %d, %d", respDmg.SessionDamaged, respDmg.SessionScanned)
	}
	if respDmg.Condition != "damaged" {
		t.Fatalf("expected condition damaged, got %s", respDmg.Condition)
	}

	// Stock still unchanged
	var stockAfterDamage int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterDamage)
	if stockAfterDamage != initialStock {
		t.Fatalf("expected stock unchanged after damage scan (%d), got %d", initialStock, stockAfterDamage)
	}

	// Invalid receiving condition rejected
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[3].UnitCode,
		Condition: "broken",
	})
	if !errors.Is(err, supplies.ErrInvalidReceivingCondition) {
		t.Fatalf("expected ErrInvalidReceivingCondition, got %v", err)
	}

	// Wrong Supply ZMU rejected
	supply2 := createShippedSupply(t, tc)
	units2, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply2.ID)
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units2[0].UnitCode,
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrUnitNotInSupply) {
		t.Fatalf("expected ErrUnitNotInSupply, got %v", err)
	}

	// Unknown ZMU rejected
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  "ZMU-UNKNOWN12345",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrUnitNotFound) {
		t.Fatalf("expected ErrUnitNotFound, got %v", err)
	}

	// ZMK passed to serialized endpoint rejected
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  "ZMK-123",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSerializedUnitCodeRequired) {
		t.Fatalf("expected ErrSerializedUnitCodeRequired, got %v", err)
	}

	// Seller SKU passed to serialized endpoint rejected
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  "SKU-123",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSerializedUnitCodeRequired) {
		t.Fatalf("expected ErrSerializedUnitCodeRequired, got %v", err)
	}

	// Finalized session rejects scan
	testDB.Exec(tc.Ctx, "UPDATE supply_receiving_sessions SET status = 'completed' WHERE id = $1", session.ID)
	unit4 := units[3]
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit4.UnitCode,
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrReceivingSessionFinalized) {
		t.Fatalf("expected ErrReceivingSessionFinalized, got %v", err)
	}
}

func TestCorruptSerializationRejectsScan(t *testing.T) {
	tc := setupTestContext(t)

	// Setup fixture: Supply with expected quantity 5, but only 4 inventory_units
	supplyID := uuid.New()
	supplyNumber := "SUP-CORRUPT-001"
	qrToken := "qr-corrupt-123"
	now := time.Now().UTC()

	testDB.Exec(tc.Ctx, `INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at) VALUES ($1, $2, $3, 'ready_to_ship', 'carrier_delivery', $4, $5, $5)`, supplyID, supplyNumber, tc.SellerID, qrToken, now)
	itemID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 5, $4, $4)`, itemID, supplyID, tc.Variant1, now)
	boxID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at) VALUES ($1, $2, 'BOX-01', 'box-qr-corrupt', $3)`, boxID, supplyID, now)
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_box_items (box_id, supply_item_id, quantity) VALUES ($1, $2, 5)`, boxID, itemID)

	// Insert only 4 units for 5 expected items
	unitCodes := []string{"ZMU-CORRUPT000001", "ZMU-CORRUPT000002", "ZMU-CORRUPT000003", "ZMU-CORRUPT000004"}
	for i, code := range unitCodes {
		testDB.Exec(tc.Ctx, `INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, 'expected', $8, $8)`,
			uuid.New(), code, tc.Variant1, supplyID, itemID, boxID, i+1, now)
	}

	// Canonical business flow
	_, err := tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supplyID)
	if err != nil {
		t.Fatalf("failed to mark shipped: %v", err)
	}
	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supplyID)
	if err != nil {
		t.Fatalf("failed to mark arrived: %v", err)
	}

	// Starting receiving session on corrupt supply should return ErrSupplyUnitIdentityMismatch
	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, qrToken)
	if !errors.Is(err, supplies.ErrSupplyUnitIdentityMismatch) {
		t.Fatalf("expected ErrSupplyUnitIdentityMismatch on StartReceivingSession, got %v", err)
	}

	// Verify no auto-repaired units (still exactly 4)
	var unitsCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1", supplyID).Scan(&unitsCount)
	if unitsCount != 4 {
		t.Fatalf("expected units count to remain 4, got %d", unitsCount)
	}
}

func TestLegacySupplyRejectsSerializedScan(t *testing.T) {
	tc := setupTestContext(t)

	// True isolated legacy fixture: Supply without any inventory_units in setup
	supplyID := uuid.New()
	supplyNumber := "SUP-LEGACY-001"
	qrToken := "qr-legacy-123"
	now := time.Now().UTC()

	testDB.Exec(tc.Ctx, `INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at) VALUES ($1, $2, $3, 'ready_to_ship', 'carrier_delivery', $4, $5, $5)`, supplyID, supplyNumber, tc.SellerID, qrToken, now)
	itemID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, $4, $4)`, itemID, supplyID, tc.Variant1, now)
	boxID := uuid.New()
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at) VALUES ($1, $2, 'BOX-01', 'box-qr-legacy', $3)`, boxID, supplyID, now)
	testDB.Exec(tc.Ctx, `INSERT INTO seller_supply_box_items (box_id, supply_item_id, quantity) VALUES ($1, $2, 10)`, boxID, itemID)

	// Canonical business lifecycle methods
	_, err := tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supplyID)
	if err != nil {
		t.Fatalf("failed to mark shipped: %v", err)
	}
	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supplyID)
	if err != nil {
		t.Fatalf("failed to mark arrived: %v", err)
	}
	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, qrToken)
	if err != nil {
		t.Fatalf("failed to start receiving session: %v", err)
	}

	// Attempt serialized scan on legacy supply -> must reject with ErrSupplyNotSerialized
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  "ZMU-LEGACYTEST1234",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSupplyNotSerialized) {
		t.Fatalf("expected ErrSupplyNotSerialized, got %v", err)
	}

	// Legacy aggregate scan should still work on legacy supply
	err = tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: tc.Variant1,
		Quantity:  5,
		IsDamage:  false,
	})
	if err != nil {
		t.Fatalf("expected legacy aggregate scan to succeed, got %v", err)
	}
}

func TestStartReceivingSessionLookupIdentifiersAndErrors(t *testing.T) {
	tc := setupTestContext(t)

	// A, B, C, D: Test lookup by Supply Number, Supply QR, Box Number, Box QR
	supply := createShippedSupply(t, tc)
	if len(supply.Boxes) == 0 {
		t.Fatalf("expected supply to have at least one box")
	}
	box := supply.Boxes[0]

	// 1. Start by Supply number
	sessionByNum, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, supply.SupplyNumber)
	if err != nil {
		t.Fatalf("failed to start session by supply number: %v", err)
	}
	if sessionByNum.ReceivingMode != "serialized" {
		t.Fatalf("expected ReceivingMode 'serialized', got %s", sessionByNum.ReceivingMode)
	}

	// 2. Re-lookup / resume active session by Supply QR token
	sessionByQR, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to resume session by supply QR: %v", err)
	}
	if sessionByQR.ID != sessionByNum.ID {
		t.Fatalf("expected resumed session ID %s, got %s", sessionByNum.ID, sessionByQR.ID)
	}

	// 3. Re-lookup / resume active session by Box Number
	sessionByBoxNum, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, box.BoxNumber)
	if err != nil {
		t.Fatalf("failed to resume session by box number: %v", err)
	}
	if sessionByBoxNum.ID != sessionByNum.ID {
		t.Fatalf("expected resumed session ID %s, got %s", sessionByNum.ID, sessionByBoxNum.ID)
	}

	// 4. Re-lookup / resume active session by Box QR token
	sessionByBoxQR, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *box.QRToken)
	if err != nil {
		t.Fatalf("failed to resume session by box QR: %v", err)
	}
	if sessionByBoxQR.ID != sessionByNum.ID {
		t.Fatalf("expected resumed session ID %s, got %s", sessionByNum.ID, sessionByBoxQR.ID)
	}

	// E. Unknown identifier -> ErrSupplyNotFound
	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, "NONEXISTENT-CODE-999")
	if !errors.Is(err, supplies.ErrSupplyNotFound) {
		t.Fatalf("expected ErrSupplyNotFound for unknown code, got %v", err)
	}

	// Empty identifier -> ErrInvalidReceivingCode
	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, "   ")
	if !errors.Is(err, supplies.ErrInvalidReceivingCode) {
		t.Fatalf("expected ErrInvalidReceivingCode for blank code, got %v", err)
	}

	// F. Status: shipped_by_seller (not arrived yet) -> ErrSupplyNotArrived
	carrier := "СДЭК"
	tracking := "999888777"
	unArrivedReq := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    &carrier,
		TrackingNumber: &tracking,
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 5},
		},
	}
	unArrivedSupply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, unArrivedReq)
	if err != nil {
		t.Fatalf("failed to create unarrived supply: %v", err)
	}
	_, err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, unArrivedSupply.ID)
	if err != nil {
		t.Fatalf("failed to mark unarrived supply shipped: %v", err)
	}

	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, unArrivedSupply.SupplyNumber)
	if !errors.Is(err, supplies.ErrSupplyNotArrived) {
		t.Fatalf("expected ErrSupplyNotArrived for shipped-only supply, got %v", err)
	}

	// G. Status: ready_to_ship (not even shipped) -> ErrSupplyNotReadyForReceiving
	notShippedSupply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, unArrivedReq)
	if err != nil {
		t.Fatalf("failed to create not shipped supply: %v", err)
	}
	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, notShippedSupply.SupplyNumber)
	if !errors.Is(err, supplies.ErrSupplyNotReadyForReceiving) {
		t.Fatalf("expected ErrSupplyNotReadyForReceiving for ready_to_ship supply, got %v", err)
	}

	// H. Corrupt serialized supply -> ErrSupplyUnitIdentityMismatch
	corruptSupply := createShippedSupply(t, tc)
	// Delete one inventory unit from DB to corrupt expected vs actual count
	testDB.Exec(tc.Ctx, "DELETE FROM inventory_units WHERE id = (SELECT id FROM inventory_units WHERE origin_supply_id = $1 LIMIT 1)", corruptSupply.ID)

	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, corruptSupply.SupplyNumber)
	if !errors.Is(err, supplies.ErrSupplyUnitIdentityMismatch) {
		t.Fatalf("expected ErrSupplyUnitIdentityMismatch for corrupted unit count, got %v", err)
	}
}

func TestSupplyArrivalLifecycle(t *testing.T) {
	tc := setupTestContext(t)

	carrier := "СДЭК"
	tracking := "TRACK-ARRIVAL-123"
	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    &carrier,
		TrackingNumber: &tracking,
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 4},
		},
	}

	// 1. Create supply -> status is ready_to_ship
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("CreateSupply failed: %v", err)
	}

	// F. ready_to_ship supply cannot be marked arrived
	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supply.ID)
	if !errors.Is(err, supplies.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus when marking ready_to_ship arrived, got %v", err)
	}

	// 2. Mark shipped -> status is shipped_by_seller
	_, err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("MarkShipped failed: %v", err)
	}

	// A. Lookup shipped_by_seller supply -> resolves dossier, no receiving session created
	dossier, err := tc.Repo.GetSupplyByQRToken(tc.Ctx, supply.SupplyNumber)
	if err != nil {
		t.Fatalf("lookup supply by number failed: %v", err)
	}
	if dossier.Status != "shipped_by_seller" {
		t.Fatalf("expected status 'shipped_by_seller', got '%s'", dossier.Status)
	}
	if dossier.SellerName == "" {
		t.Fatalf("expected seller name to be populated")
	}

	// Verify full supply dossier loaded by ID
	fullSupply, err := tc.Repo.GetSupplyByID(tc.Ctx, dossier.ID)
	if err != nil {
		t.Fatalf("GetSupplyByID failed: %v", err)
	}
	if len(fullSupply.Items) != 1 || fullSupply.Items[0].ExpectedQuantity != 4 {
		t.Fatalf("expected 1 item with quantity 4, got %+v", fullSupply.Items)
	}
	if len(fullSupply.Boxes) != 1 {
		t.Fatalf("expected 1 box, got %d", len(fullSupply.Boxes))
	}

	// Verify no receiving session exists yet
	_, err = tc.Repo.GetActiveSession(tc.Ctx, supply.ID)
	if !errors.Is(err, supplies.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound before receiving starts, got %v", err)
	}

	// B. MarkSupplyArrived -> status arrived_at_zamk and arrived_at is set
	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supply.ID)
	if err != nil {
		t.Fatalf("MarkSupplyArrived failed: %v", err)
	}
	arrivedSupply, err := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("GetSupplyByID after arrival failed: %v", err)
	}
	if arrivedSupply.Status != "arrived_at_zamk" {
		t.Fatalf("expected status 'arrived_at_zamk', got '%s'", arrivedSupply.Status)
	}
	if arrivedSupply.ArrivedAt == nil {
		t.Fatalf("expected arrived_at timestamp to be set")
	}

	// C, D. Start session after arrival -> PASS, receivingMode = serialized
	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, supply.SupplyNumber)
	if err != nil {
		t.Fatalf("StartReceivingSession after arrival failed: %v", err)
	}
	if session.ReceivingMode != "serialized" {
		t.Fatalf("expected receivingMode 'serialized', got '%s'", session.ReceivingMode)
	}

	// E. Existing active session -> continue/resume same session
	resumedSession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, supply.SupplyNumber)
	if err != nil {
		t.Fatalf("resuming active session failed: %v", err)
	}
	if resumedSession.ID != session.ID {
		t.Fatalf("expected resumed session ID %s, got %s", session.ID, resumedSession.ID)
	}
}

func TestMultiVariantSerializedScanLifecycle(t *testing.T) {
	tc := setupTestContext(t)

	carrier := "СДЭК"
	tracking := "TRACK-36-UNITS"
	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    &carrier,
		TrackingNumber: &tracking,
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 12},
			{VariantID: tc.Variant2, ExpectedQuantity: 12},
			{VariantID: tc.Variant3, ExpectedQuantity: 12},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("CreateSupply 36 units failed: %v", err)
	}

	_, err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("MarkShipped failed: %v", err)
	}

	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supply.ID)
	if err != nil {
		t.Fatalf("MarkSupplyArrived failed: %v", err)
	}

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, supply.SupplyNumber)
	if err != nil {
		t.Fatalf("StartReceivingSession failed: %v", err)
	}
	if session.ReceivingMode != "serialized" {
		t.Fatalf("expected serialized receivingMode, got %s", session.ReceivingMode)
	}

	// 1. Initial list of recent scans should be empty (not error 500)
	recentScans, err := tc.Service.ListRecentSerializedScans(tc.Ctx, tc.AdminID, session.ID, 10)
	if err != nil {
		t.Fatalf("ListRecentSerializedScans on empty session failed: %v", err)
	}
	if len(recentScans) != 0 {
		t.Fatalf("expected 0 initial recent scans, got %d", len(recentScans))
	}

	// Load all units of supply
	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("ListUnitsBySupplyID failed: %v", err)
	}
	if len(units) != 36 {
		t.Fatalf("expected 36 units, got %d", len(units))
	}

	// Group units by variant
	unitsByVariant := make(map[uuid.UUID][]supplies.InventoryUnit)
	for _, u := range units {
		unitsByVariant[u.ProductVariantID] = append(unitsByVariant[u.ProductVariantID], u)
	}

	if len(unitsByVariant[tc.Variant1]) != 12 || len(unitsByVariant[tc.Variant2]) != 12 || len(unitsByVariant[tc.Variant3]) != 12 {
		t.Fatalf("expected 12 units per variant, got v1=%d, v2=%d, v3=%d",
			len(unitsByVariant[tc.Variant1]), len(unitsByVariant[tc.Variant2]), len(unitsByVariant[tc.Variant3]))
	}

	// 2. Scan 1 unit from Variant 1 as OK
	v1Unit := unitsByVariant[tc.Variant1][0]
	resp1, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  v1Unit.UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("RecordSerializedScan variant 1 failed: %v", err)
	}
	if resp1.SessionOk != 1 || resp1.SessionDamaged != 0 || resp1.SessionScanned != 1 || resp1.SessionRemaining != 35 {
		t.Fatalf("unexpected totals after scan 1: %+v", resp1)
	}
	if resp1.ProductTitle == "" || resp1.ProductVariantID != tc.Variant1 {
		t.Fatalf("unexpected enrichment on resp1: %+v", resp1)
	}

	// 3. Scan 1 unit from Variant 2 as OK
	v2Unit := unitsByVariant[tc.Variant2][0]
	resp2, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  v2Unit.UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("RecordSerializedScan variant 2 failed: %v", err)
	}
	if resp2.SessionOk != 2 || resp2.SessionDamaged != 0 || resp2.SessionScanned != 2 || resp2.SessionRemaining != 34 {
		t.Fatalf("unexpected totals after scan 2: %+v", resp2)
	}

	// 4. Scan 1 unit from Variant 3 as DAMAGED
	v3Unit := unitsByVariant[tc.Variant3][0]
	resp3, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  v3Unit.UnitCode,
		Condition: "damaged",
	})
	if err != nil {
		t.Fatalf("RecordSerializedScan variant 3 damaged failed: %v", err)
	}
	if resp3.SessionOk != 2 || resp3.SessionDamaged != 1 || resp3.SessionScanned != 3 || resp3.SessionRemaining != 33 {
		t.Fatalf("unexpected totals after scan 3: %+v", resp3)
	}

	// 5. Duplicate scan on already scanned unit -> ErrUnitAlreadyScanned
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  v1Unit.UnitCode,
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrUnitAlreadyScanned) {
		t.Fatalf("expected ErrUnitAlreadyScanned, got %v", err)
	}

	// 6. Verify recent scans list contains 3 entries
	recent, err := tc.Service.ListRecentSerializedScans(tc.Ctx, tc.AdminID, session.ID, 10)
	if err != nil {
		t.Fatalf("ListRecentSerializedScans failed: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent scans, got %d", len(recent))
	}
}

func TestAdditionalSerializedReceiving_FullResolutionToCompleted_5Units(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 5)

	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	// Step 1: Start first session
	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start first session: %v", err)
	}

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil || len(units) != 5 {
		t.Fatalf("expected 5 units, got %d, err=%v", len(units), err)
	}

	// Scan 4 OK, 1 unscanned
	for i := 0; i < 4; i++ {
		_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  units[i].UnitCode,
			Condition: "ok",
		})
		if err != nil {
			t.Fatalf("scan %d failed: %v", i, err)
		}
	}

	// Finalize first session
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}

	// Assertions after first finalize:
	// warehouse = 4, expected = 1, damaged = 0
	var whCount, expCount, dmgCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'warehouse'", supply.ID).Scan(&whCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'expected'", supply.ID).Scan(&expCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'damaged'", supply.ID).Scan(&dmgCount)
	if whCount != 4 || expCount != 1 || dmgCount != 0 {
		t.Fatalf("expected warehouse=4, expected=1, damaged=0; got wh=%d, exp=%d, dmg=%d", whCount, expCount, dmgCount)
	}

	// stock = +4
	var stockAfterFirst int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterFirst)
	if stockAfterFirst != initialStock+4 {
		t.Fatalf("expected stock %d, got %d", initialStock+4, stockAfterFirst)
	}

	// accepted_quantity = 4
	var acc1, dmg1, mis1 int
	testDB.QueryRow(tc.Ctx, "SELECT accepted_quantity, damaged_quantity, missing_quantity FROM seller_supply_items WHERE supply_id = $1", supply.ID).Scan(&acc1, &dmg1, &mis1)
	if acc1 != 4 || dmg1 != 0 || mis1 != 1 {
		t.Fatalf("expected item acc=4, dmg=0, mis=1; got acc=%d, dmg=%d, mis=%d", acc1, dmg1, mis1)
	}

	// Supply = completed_with_discrepancies
	s1, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if s1.Status != "completed_with_discrepancies" {
		t.Fatalf("expected supply status completed_with_discrepancies, got %s", s1.Status)
	}

	// Capture old session & stock movement state before additional receiving
	oldSessionID := session.ID
	var oldSessionStatus string
	testDB.QueryRow(tc.Ctx, "SELECT status FROM supply_receiving_sessions WHERE id = $1", oldSessionID).Scan(&oldSessionStatus)
	var oldScansCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE session_id = $1", oldSessionID).Scan(&oldScansCount)
	var oldItemScanned int
	testDB.QueryRow(tc.Ctx, "SELECT scanned_quantity FROM supply_receiving_items WHERE session_id = $1", oldSessionID).Scan(&oldItemScanned)

	var firstMovID uuid.UUID
	var firstMovQty int
	err = testDB.QueryRow(tc.Ctx, "SELECT id, quantity FROM stock_movements WHERE reference_type = 'supply' AND reference_id = $1", supply.ID).Scan(&firstMovID, &firstMovQty)
	if err != nil || firstMovQty != 4 {
		t.Fatalf("expected 1 stock movement with qty=4, got err=%v, qty=%d", err, firstMovQty)
	}

	// Step 2: Start additional session
	addSession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start additional session: %v", err)
	}
	if addSession.ID == oldSessionID {
		t.Fatalf("expected new session ID, got same as old %v", addSession.ID)
	}
	if len(addSession.Items) != 1 || addSession.Items[0].ExpectedQuantity != 1 {
		t.Fatalf("expected 1 item with expected_quantity=1, got len=%d, qty=%v", len(addSession.Items), addSession.Items[0].ExpectedQuantity)
	}

	// Scan remaining ZMU OK
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, addSession.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[4].UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("failed to scan remaining unit: %v", err)
	}

	// Finalize additional session
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, addSession.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("failed to finalize additional session: %v", err)
	}

	// Assertions after additional finalize:
	// warehouse = 5, expected = 0, damaged = 0
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'warehouse'", supply.ID).Scan(&whCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'expected'", supply.ID).Scan(&expCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'damaged'", supply.ID).Scan(&dmgCount)
	if whCount != 5 || expCount != 0 || dmgCount != 0 {
		t.Fatalf("expected warehouse=5, expected=0, damaged=0; got wh=%d, exp=%d, dmg=%d", whCount, expCount, dmgCount)
	}

	// stock total = +5
	var stockAfterSecond int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterSecond)
	if stockAfterSecond != initialStock+5 {
		t.Fatalf("expected total stock %d, got %d", initialStock+5, stockAfterSecond)
	}

	// accepted_quantity = 5
	var acc2, dmg2, mis2, ext2 int
	testDB.QueryRow(tc.Ctx, "SELECT accepted_quantity, damaged_quantity, missing_quantity, extra_quantity FROM seller_supply_items WHERE supply_id = $1", supply.ID).Scan(&acc2, &dmg2, &mis2, &ext2)
	if acc2 != 5 || dmg2 != 0 || mis2 != 0 || ext2 != 0 {
		t.Fatalf("expected item acc=5, dmg=0, mis=0, ext=0; got acc=%d, dmg=%d, mis=%d, ext=%d", acc2, dmg2, mis2, ext2)
	}

	// Supply = completed
	s2, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if s2.Status != "completed" {
		t.Fatalf("expected supply status completed, got %s", s2.Status)
	}

	// Stock movements: exactly 2 distinct rows
	var movCount, totalMovQty int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*), COALESCE(SUM(quantity), 0) FROM stock_movements WHERE reference_type = 'supply' AND reference_id = $1", supply.ID).Scan(&movCount, &totalMovQty)
	if movCount != 2 || totalMovQty != 5 {
		t.Fatalf("expected 2 stock movements with sum=5, got count=%d, sum=%d", movCount, totalMovQty)
	}

	// First movement unchanged
	var checkFirstQty int
	err = testDB.QueryRow(tc.Ctx, "SELECT quantity FROM stock_movements WHERE id = $1", firstMovID).Scan(&checkFirstQty)
	if err != nil || checkFirstQty != 4 {
		t.Fatalf("first stock movement modified: err=%v, qty=%d", err, checkFirstQty)
	}

	// Old session immutability:
	var checkOldStatus string
	testDB.QueryRow(tc.Ctx, "SELECT status FROM supply_receiving_sessions WHERE id = $1", oldSessionID).Scan(&checkOldStatus)
	if checkOldStatus != oldSessionStatus {
		t.Fatalf("old session status changed from %s to %s", oldSessionStatus, checkOldStatus)
	}
	var checkOldScansCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE session_id = $1", oldSessionID).Scan(&checkOldScansCount)
	if checkOldScansCount != oldScansCount {
		t.Fatalf("old scans count changed from %d to %d", oldScansCount, checkOldScansCount)
	}
	var checkOldItemScanned int
	testDB.QueryRow(tc.Ctx, "SELECT scanned_quantity FROM supply_receiving_items WHERE session_id = $1", oldSessionID).Scan(&checkOldItemScanned)
	if checkOldItemScanned != oldItemScanned {
		t.Fatalf("old item scanned quantity changed from %d to %d", oldItemScanned, checkOldItemScanned)
	}

	// Double finalize on additional session: must fail safely and idempotently
	errDouble := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, addSession.ID, supplies.FinalizeReceivingRequest{})
	if errDouble == nil || (errDouble.Error() != "session is not active" && !errors.Is(errDouble, supplies.ErrReceivingSessionFinalized)) {
		t.Fatalf("expected error on double finalize, got: %v", errDouble)
	}

	// Stock and item state unchanged after double finalize
	var stockAfterDouble, accAfterDouble, movCountAfterDouble int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterDouble)
	testDB.QueryRow(tc.Ctx, "SELECT accepted_quantity FROM seller_supply_items WHERE supply_id = $1", supply.ID).Scan(&accAfterDouble)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM stock_movements WHERE reference_type = 'supply' AND reference_id = $1", supply.ID).Scan(&movCountAfterDouble)
	if stockAfterDouble != initialStock+5 || accAfterDouble != 5 || movCountAfterDouble != 2 {
		t.Fatalf("state mutated after double finalize: stock=%d, acc=%d, movs=%d", stockAfterDouble, accAfterDouble, movCountAfterDouble)
	}
}

func TestAdditionalSerializedReceiving_DamageScenario_5Units(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 5)

	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start first session: %v", err)
	}

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil || len(units) != 5 {
		t.Fatalf("expected 5 units, got %d", len(units))
	}

	// First session: 3 OK, 1 damaged, 1 unscanned
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{UnitCode: units[0].UnitCode, Condition: "ok"})
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{UnitCode: units[1].UnitCode, Condition: "ok"})
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{UnitCode: units[2].UnitCode, Condition: "ok"})
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{UnitCode: units[3].UnitCode, Condition: "damaged"})

	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}

	// Assert first finalize state:
	var whCount, dmgCount, expCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'warehouse'", supply.ID).Scan(&whCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'damaged'", supply.ID).Scan(&dmgCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'expected'", supply.ID).Scan(&expCount)
	if whCount != 3 || dmgCount != 1 || expCount != 1 {
		t.Fatalf("expected wh=3, dmg=1, exp=1; got wh=%d, dmg=%d, exp=%d", whCount, dmgCount, expCount)
	}

	var acc1, dmgQty1, mis1 int
	testDB.QueryRow(tc.Ctx, "SELECT accepted_quantity, damaged_quantity, missing_quantity FROM seller_supply_items WHERE supply_id = $1", supply.ID).Scan(&acc1, &dmgQty1, &mis1)
	if acc1 != 3 || dmgQty1 != 1 || mis1 != 1 {
		t.Fatalf("expected acc=3, dmg=1, mis=1; got acc=%d, dmg=%d, mis=%d", acc1, dmgQty1, mis1)
	}

	s1, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if s1.Status != "completed_with_discrepancies" {
		t.Fatalf("expected completed_with_discrepancies, got %s", s1.Status)
	}

	// Step 2: Start additional session
	addSession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start additional session: %v", err)
	}
	if len(addSession.Items) != 1 || addSession.Items[0].ExpectedQuantity != 1 {
		t.Fatalf("expected 1 remaining expected unit, got %d", addSession.Items[0].ExpectedQuantity)
	}

	// Scan remaining expected unit -> OK
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, addSession.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[4].UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("failed to scan unit 4: %v", err)
	}

	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, addSession.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("additional finalize failed: %v", err)
	}

	// Final Assertions:
	// warehouse = 4, damaged = 1, expected = 0
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'warehouse'", supply.ID).Scan(&whCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'damaged'", supply.ID).Scan(&dmgCount)
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1 AND status = 'expected'", supply.ID).Scan(&expCount)
	if whCount != 4 || dmgCount != 1 || expCount != 0 {
		t.Fatalf("expected wh=4, dmg=1, exp=0; got wh=%d, dmg=%d, exp=%d", whCount, dmgCount, expCount)
	}

	// stock total = +4
	var finalStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&finalStock)
	if finalStock != initialStock+4 {
		t.Fatalf("expected stock %d, got %d", initialStock+4, finalStock)
	}

	// accepted_quantity = 4, damaged_quantity = 1, missing_quantity = 0
	var finalAcc, finalDmg, finalMis int
	testDB.QueryRow(tc.Ctx, "SELECT accepted_quantity, damaged_quantity, missing_quantity FROM seller_supply_items WHERE supply_id = $1", supply.ID).Scan(&finalAcc, &finalDmg, &finalMis)
	if finalAcc != 4 || finalDmg != 1 || finalMis != 0 {
		t.Fatalf("expected acc=4, dmg=1, mis=0; got acc=%d, dmg=%d, mis=%d", finalAcc, finalDmg, finalMis)
	}

	// Supply status remains completed_with_discrepancies (due to damaged=1)
	s2, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if s2.Status != "completed_with_discrepancies" {
		t.Fatalf("expected supply status completed_with_discrepancies, got %s", s2.Status)
	}

	// Two distinct movements: +3 and +1
	var movCount, movSum int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*), COALESCE(SUM(quantity), 0) FROM stock_movements WHERE reference_type = 'supply' AND reference_id = $1", supply.ID).Scan(&movCount, &movSum)
	if movCount != 2 || movSum != 4 {
		t.Fatalf("expected 2 stock movements summing to 4, got count=%d, sum=%d", movCount, movSum)
	}
}

func TestAdditionalSerializedReceiving_ActiveSessionResume(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 5)

	// First session: 4 OK, 1 unscanned -> finalize with discrepancies
	s1, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	for i := 0; i < 4; i++ {
		tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, s1.ID, supplies.RecordSerializedScanRequest{UnitCode: units[i].UnitCode, Condition: "ok"})
	}
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, s1.ID, supplies.FinalizeReceivingRequest{})

	// Start additional session first time
	sess1, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("first start additional session failed: %v", err)
	}
	if sess1.Status != "active" {
		t.Fatalf("expected session status active, got %s", sess1.Status)
	}

	// Start additional session second time
	sess2, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("second start additional session failed: %v", err)
	}

	// Same active session ID returned/resumed
	if sess1.ID != sess2.ID {
		t.Fatalf("expected same session resumed (%v), got new session (%v)", sess1.ID, sess2.ID)
	}

	// Assert exactly ONE active session exists in DB
	var activeCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_sessions WHERE supply_id = $1 AND status = 'active'", supply.ID).Scan(&activeCount)
	if activeCount != 1 {
		t.Fatalf("expected 1 active session in DB, got %d", activeCount)
	}
}

func TestAdditionalSerializedReceiving_InvalidCases(t *testing.T) {
	tc := setupTestContext(t)

	// Case 1: completed Supply with no expected units
	supplyCompleted := createShippedSupplyWithUnits(t, tc, 5)
	sessCompleted, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyCompleted.QRToken)
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	unitsCompleted, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supplyCompleted.ID)
	for _, u := range unitsCompleted {
		tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessCompleted.ID, supplies.RecordSerializedScanRequest{UnitCode: u.UnitCode, Condition: "ok"})
	}
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessCompleted.ID, supplies.FinalizeReceivingRequest{})

	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyCompleted.QRToken)
	if !errors.Is(err, supplies.ErrSupplyAlreadyCompleted) {
		t.Fatalf("expected ErrSupplyAlreadyCompleted on completed supply, got %v", err)
	}

	// Case 2: cancelled Supply
	supplyCancelled := createShippedSupplyWithUnits(t, tc, 5)
	tc.Repo.UpdateSupplyStatus(tc.Ctx, supplyCancelled.ID, "cancelled")
	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyCancelled.QRToken)
	if !errors.Is(err, supplies.ErrSupplyCancelled) {
		t.Fatalf("expected ErrSupplyCancelled on cancelled supply, got %v", err)
	}

	// Case 3: completed_with_discrepancies but NO expected units remain (e.g. 4 OK, 1 Damaged)
	supplyNoExp := createShippedSupplyWithUnits(t, tc, 5)
	sessNoExp, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyNoExp.QRToken)
	unitsNoExp, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supplyNoExp.ID)
	for i := 0; i < 4; i++ {
		tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessNoExp.ID, supplies.RecordSerializedScanRequest{UnitCode: unitsNoExp[i].UnitCode, Condition: "ok"})
	}
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessNoExp.ID, supplies.RecordSerializedScanRequest{UnitCode: unitsNoExp[4].UnitCode, Condition: "damaged"})
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessNoExp.ID, supplies.FinalizeReceivingRequest{})

	_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyNoExp.QRToken)
	if !errors.Is(err, supplies.ErrNoExpectedUnitsRemain) {
		t.Fatalf("expected ErrNoExpectedUnitsRemain when 0 expected units remain, got %v", err)
	}

	// Case 4, 5, 6: Rescan rejections during additional receiving
	supplyActive := createShippedSupplyWithUnits(t, tc, 5)
	sessActive1, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyActive.QRToken)
	unitsActive, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supplyActive.ID)
	// Scan unit 0 OK, unit 1 Damaged, leave 2, 3, 4 expected
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessActive1.ID, supplies.RecordSerializedScanRequest{UnitCode: unitsActive[0].UnitCode, Condition: "ok"})
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessActive1.ID, supplies.RecordSerializedScanRequest{UnitCode: unitsActive[1].UnitCode, Condition: "damaged"})
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessActive1.ID, supplies.FinalizeReceivingRequest{})

	// Start additional session
	addSession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyActive.QRToken)
	if err != nil {
		t.Fatalf("failed to start additional session: %v", err)
	}

	// Rescan warehouse ZMU (unit 0) -> ErrUnitAlreadyReceived
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, addSession.ID, supplies.RecordSerializedScanRequest{UnitCode: unitsActive[0].UnitCode, Condition: "ok"})
	if !errors.Is(err, supplies.ErrUnitAlreadyReceived) {
		t.Fatalf("expected ErrUnitAlreadyReceived for warehouse unit, got %v", err)
	}

	// Rescan damaged ZMU (unit 1) -> ErrUnitAlreadyReceived
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, addSession.ID, supplies.RecordSerializedScanRequest{UnitCode: unitsActive[1].UnitCode, Condition: "ok"})
	if !errors.Is(err, supplies.ErrUnitAlreadyReceived) {
		t.Fatalf("expected ErrUnitAlreadyReceived for damaged unit, got %v", err)
	}

	// Rescan unit from a different supply -> ErrUnitNotInSupply
	otherSupply := createShippedSupplyWithUnits(t, tc, 5)
	otherUnits, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, otherSupply.ID)
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, addSession.ID, supplies.RecordSerializedScanRequest{UnitCode: otherUnits[0].UnitCode, Condition: "ok"})
	if !errors.Is(err, supplies.ErrUnitNotInSupply) {
		t.Fatalf("expected ErrUnitNotInSupply for other supply unit, got %v", err)
	}
}
