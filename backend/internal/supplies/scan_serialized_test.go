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

func TestSerializedFinalizeBlockedAndLegacyFinalizeAllowed(t *testing.T) {
	tc := setupTestContext(t)

	// N. Serialized old finalize is BLOCKED with zero stock/status mutation
	serializedSupply := createShippedSupply(t, tc)
	serializedSession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *serializedSupply.QRToken)
	if err != nil {
		t.Fatalf("failed to start serialized session: %v", err)
	}

	// Capture initial stock
	var initialStock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&initialStock)

	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, serializedSupply.ID)
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, serializedSession.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("failed to record serialized scan: %v", err)
	}

	// Attempt finalize on serialized supply
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, serializedSession.ID, supplies.FinalizeReceivingRequest{})
	if !errors.Is(err, supplies.ErrSerializedFinalizeNotSupported) {
		t.Fatalf("expected ErrSerializedFinalizeNotSupported, got %v", err)
	}

	// Verify stock delta == 0
	var stockAfterFinalize int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterFinalize)
	if stockAfterFinalize != initialStock {
		t.Fatalf("expected stock unchanged (%d), got %d", initialStock, stockAfterFinalize)
	}

	// Verify supply status remains 'receiving'
	currentSupply, _ := tc.Repo.GetSupplyByID(tc.Ctx, serializedSupply.ID)
	if currentSupply.Status != "receiving" {
		t.Fatalf("expected supply status 'receiving', got '%s'", currentSupply.Status)
	}

	// Verify session status remains 'active'
	currentSession, _ := tc.Repo.GetSessionByID(tc.Ctx, serializedSession.ID)
	if currentSession.Status != "active" {
		t.Fatalf("expected session status 'active', got '%s'", currentSession.Status)
	}

	// O. Legacy finalize still works
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
