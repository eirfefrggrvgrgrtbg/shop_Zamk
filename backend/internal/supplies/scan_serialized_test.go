package supplies_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

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

	// A. Valid ZMU scan creates one scan event
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

	// B. Unit remains status=expected
	unit1db, _ := tc.Repo.GetInventoryUnitByCode(tc.Ctx, unit1.UnitCode)
	if unit1db.Status != "expected" {
		t.Fatalf("expected status=expected, got %s", unit1db.Status)
	}

	// C. Stock unchanged after scan
	var stockAfterScan1 int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfterScan1)
	if stockAfterScan1 != initialStock {
		t.Fatalf("expected stock unchanged (%d), got %d", initialStock, stockAfterScan1)
	}

	// D. Same ZMU second scan: unit_already_scanned
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unit1.UnitCode,
		Condition: "damaged",
	})
	if !errors.Is(err, supplies.ErrUnitAlreadyScanned) {
		t.Fatalf("expected ErrUnitAlreadyScanned, got %v", err)
	}

	// E. Active event count remains 1
	var count int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE inventory_unit_id = $1 AND voided_at IS NULL", unit1.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 active event, got %d", count)
	}

	// F. Two concurrent scans same ZMU: one PASS, one duplicate
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

	// K. Damaged condition stored correctly
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

	// G. Wrong Supply ZMU rejected
	supply2 := createShippedSupply(t, tc)
	units2, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply2.ID)
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units2[0].UnitCode,
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrUnitNotInSupply) {
		t.Fatalf("expected ErrUnitNotInSupply, got %v", err)
	}

	// H. Unknown ZMU rejected
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  "ZMU-UNKNOWN12345",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrUnitNotFound) {
		t.Fatalf("expected ErrUnitNotFound, got %v", err)
	}

	// I. ZMK passed to serialized endpoint rejected
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  "ZMK-123",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSerializedUnitCodeRequired) {
		t.Fatalf("expected ErrSerializedUnitCodeRequired, got %v", err)
	}

	// J. Seller SKU passed to serialized endpoint rejected
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  "SKU-123",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSerializedUnitCodeRequired) {
		t.Fatalf("expected ErrSerializedUnitCodeRequired, got %v", err)
	}

	// L. Finalized session rejects scan
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
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
	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, qrToken)
	if err != nil {
		t.Fatalf("failed to start receiving session: %v", err)
	}

	// Capture initial stock
	var stockBefore int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockBefore)

	// Attempt scan on corrupt supply
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  unitCodes[0],
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSupplyUnitIdentityMismatch) {
		t.Fatalf("expected ErrSupplyUnitIdentityMismatch, got %v", err)
	}

	// Verify no scan events created
	var scanCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE session_id = $1", session.ID).Scan(&scanCount)
	if scanCount != 0 {
		t.Fatalf("expected 0 scan events, got %d", scanCount)
	}

	// Verify stock unchanged
	var stockAfter int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfter)
	if stockAfter != stockBefore {
		t.Fatalf("expected stock unchanged (%d), got %d", stockBefore, stockAfter)
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
