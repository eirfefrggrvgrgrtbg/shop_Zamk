package supplies_test

import (
	"errors"
	"sync"
	"testing"

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

	unit1 := units[0]
	resp, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: unit1.UnitCode,
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

	unit1db, _ := tc.Repo.GetInventoryUnitByCode(tc.Ctx, unit1.UnitCode)
	if unit1db.Status != "expected" {
		t.Fatalf("expected status=expected, got %s", unit1db.Status)
	}

	var stock int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stock)
	if stock != 0 {
		t.Fatalf("expected stock=0, got %d", stock)
	}

	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: unit1.UnitCode,
		Condition: "damaged",
	})
	if !errors.Is(err, supplies.ErrUnitAlreadyScanned) {
		t.Fatalf("expected ErrUnitAlreadyScanned, got %v", err)
	}

	var count int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM supply_receiving_scans WHERE inventory_unit_id = $1", unit1.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 event, got %d", count)
	}

	unit2 := units[1]
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
				UnitCode: unit2.UnitCode,
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

	unit3 := units[2]
	respDmg, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: unit3.UnitCode,
		Condition: "damaged",
	})
	if err != nil {
		t.Fatalf("failed damaged scan: %v", err)
	}
	if respDmg.SessionDamaged != 1 || respDmg.SessionScanned != 3 {
		t.Fatalf("expected damaged=1 scanned=3, got %d, %d", respDmg.SessionDamaged, respDmg.SessionScanned)
	}

	supply2 := createShippedSupply(t, tc)
	units2, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply2.ID)
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: units2[0].UnitCode,
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrUnitNotInSupply) {
		t.Fatalf("expected ErrUnitNotInSupply, got %v", err)
	}

	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: "ZMU-UNKNOWN12345",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrUnitNotFound) {
		t.Fatalf("expected ErrUnitNotFound, got %v", err)
	}

	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: "ZMK-123",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSerializedUnitCodeRequired) {
		t.Fatalf("expected ErrSerializedUnitCodeRequired, got %v", err)
	}

	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: "SKU-123",
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrSerializedUnitCodeRequired) {
		t.Fatalf("expected ErrSerializedUnitCodeRequired, got %v", err)
	}

	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	unit4 := units[3]
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: unit4.UnitCode,
		Condition: "ok",
	})
	if !errors.Is(err, supplies.ErrReceivingSessionFinalized) {
		t.Fatalf("expected ErrReceivingSessionFinalized, got %v", err)
	}
}

func TestLegacySupplyRejectsSerializedScan(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	testDB.Exec(tc.Ctx, "DELETE FROM inventory_units WHERE origin_supply_id = $1", supply.ID)

	session, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)

	_, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode: "ZMU-DOESNTMATTER",
		Condition: "ok",
	})
	if err == nil || err.Error() != "legacy supply, use aggregate scan endpoint" {
		t.Fatalf("expected legacy supply error, got %v", err)
	}
}
