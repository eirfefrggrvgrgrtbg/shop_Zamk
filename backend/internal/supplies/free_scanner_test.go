package supplies_test

import (
	"errors"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func TestResolvePhysicalUnit_ExpectedZMU_AdditionalReceiving(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 5)

	// First finalize 4/5 units
	sessionA, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)

	for i := 0; i < 4; i++ {
		tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessionA.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  units[i].UnitCode,
			Condition: "ok",
		})
	}
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessionA.ID, supplies.FinalizeReceivingRequest{})

	// 5th unit is missing (expected). Supply is completed_with_discrepancies.
	missingUnit := units[4]

	resolved, err := tc.Service.ResolvePhysicalUnit(tc.Ctx, missingUnit.UnitCode)
	if err != nil {
		t.Fatalf("expected to resolve missing unit, got err: %v", err)
	}

	if resolved.RecommendedAction != "additional_receiving" {
		t.Fatalf("expected additional_receiving, got %v", resolved.RecommendedAction)
	}
	if resolved.InventoryUnitID != missingUnit.ID {
		t.Fatalf("id mismatch")
	}
	if resolved.Origin.SupplyID != supply.ID {
		t.Fatalf("origin supply mismatch")
	}
}

func TestResolvePhysicalUnit_WarehouseZMU(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 1)

	sessionA, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessionA.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "ok",
	})
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessionA.ID, supplies.FinalizeReceivingRequest{})

	resolved, err := tc.Service.ResolvePhysicalUnit(tc.Ctx, units[0].UnitCode)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if resolved.RecommendedAction != "already_in_warehouse" {
		t.Fatalf("expected already_in_warehouse, got %v", resolved.RecommendedAction)
	}
}

func TestResolvePhysicalUnit_DamagedZMU(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 1)

	sessionA, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessionA.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "damaged",
	})
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessionA.ID, supplies.FinalizeReceivingRequest{})

	resolved, err := tc.Service.ResolvePhysicalUnit(tc.Ctx, units[0].UnitCode)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if resolved.RecommendedAction != "already_damaged" {
		t.Fatalf("expected already_damaged, got %v", resolved.RecommendedAction)
	}
}

func TestResolvePhysicalUnit_NotFound(t *testing.T) {
	tc := setupTestContext(t)
	_, err := tc.Service.ResolvePhysicalUnit(tc.Ctx, "ZMU-BOGUS-123")
	if !errors.Is(err, supplies.ErrUnitNotFound) {
		t.Fatalf("expected ErrUnitNotFound, got %v", err)
	}
}

func TestResolvePhysicalUnit_ActiveSession(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 1)

	sessionA, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)

	resolved, err := tc.Service.ResolvePhysicalUnit(tc.Ctx, units[0].UnitCode)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if resolved.RecommendedAction != "continue_receiving" {
		t.Fatalf("expected continue_receiving, got %v", resolved.RecommendedAction)
	}
	if resolved.ReceivingState.ActiveReceivingSessionID == nil || *resolved.ReceivingState.ActiveReceivingSessionID != sessionA.ID {
		t.Fatalf("expected active session %v, got %v", sessionA.ID, resolved.ReceivingState.ActiveReceivingSessionID)
	}
}

func TestProcessFoundUnit_FoundLastUnit_AdditionalReceivingAndFinalize(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 5)

	// First finalize 4/5 units
	sessionA, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session A: %v", err)
	}
	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to list units: %v", err)
	}

	for i := 0; i < 4; i++ {
		_, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessionA.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  units[i].UnitCode,
			Condition: "ok",
		})
		if err != nil {
			t.Fatalf("failed to scan unit %d: %v", i, err)
		}
	}
	if err := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessionA.ID, supplies.FinalizeReceivingRequest{}); err != nil {
		t.Fatalf("failed to finalize session A: %v", err)
	}

	// Verify supply is completed_with_discrepancies
	supplyAfterA, err := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to get supply: %v", err)
	}
	if supplyAfterA.Status != "completed_with_discrepancies" {
		t.Fatalf("expected completed_with_discrepancies, got %s", supplyAfterA.Status)
	}

	// Verify stock is +4
	var stockA int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", units[0].ProductVariantID).Scan(&stockA)
	if stockA != 4 {
		t.Fatalf("expected stock 4, got %d", stockA)
	}

	// Last unit (5th) is missing
	missingUnit := units[4]

	// Process found unit with Free Scanner (ONE SCAN)
	resp, err := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
		UnitCode:  missingUnit.UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("ProcessFoundUnit failed: %v", err)
	}

	if resp.UnitCode != missingUnit.UnitCode {
		t.Fatalf("expected unitCode %s, got %s", missingUnit.UnitCode, resp.UnitCode)
	}
	if resp.ReceivingSessionID == nil {
		t.Fatalf("expected receiving session to be created/resumed")
	}
	if resp.SessionExpected != 1 {
		t.Fatalf("expected sessionExpected 1, got %d", resp.SessionExpected)
	}
	if resp.SessionScanned != 1 {
		t.Fatalf("expected sessionScanned 1, got %d", resp.SessionScanned)
	}
	if resp.SessionRemaining != 0 {
		t.Fatalf("expected sessionRemaining 0, got %d", resp.SessionRemaining)
	}
	if resp.RecommendedNextAction != "can_finalize" {
		t.Fatalf("expected can_finalize, got %s", resp.RecommendedNextAction)
	}

	// BEFORE FINALIZE: unit status must still be 'expected'
	unitBeforeFinalize, err := tc.Repo.GetInventoryUnitByCode(tc.Ctx, missingUnit.UnitCode)
	if err != nil {
		t.Fatalf("failed to get unit: %v", err)
	}
	if unitBeforeFinalize.Status != "expected" {
		t.Fatalf("expected unit status 'expected' before finalize, got %s", unitBeforeFinalize.Status)
	}

	// Stock before finalize must be UNCHANGED (still 4)
	var stockBeforeFinalize int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", missingUnit.ProductVariantID).Scan(&stockBeforeFinalize)
	if stockBeforeFinalize != 4 {
		t.Fatalf("expected stock to remain 4 before finalize, got %d", stockBeforeFinalize)
	}

	// Now finalize the additional receiving session
	if err := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, *resp.ReceivingSessionID, supplies.FinalizeReceivingRequest{}); err != nil {
		t.Fatalf("failed to finalize additional session: %v", err)
	}

	// AFTER FINALIZE: unit status must be 'warehouse'
	unitAfterFinalize, err := tc.Repo.GetInventoryUnitByCode(tc.Ctx, missingUnit.UnitCode)
	if err != nil {
		t.Fatalf("failed to get unit: %v", err)
	}
	if unitAfterFinalize.Status != "warehouse" {
		t.Fatalf("expected unit status 'warehouse', got %s", unitAfterFinalize.Status)
	}

	// Stock must now be +5
	var stockAfterFinalize int
	testDB.QueryRow(tc.Ctx, "SELECT COALESCE(total_stock, 0) FROM inventory_items WHERE product_variant_id = $1", missingUnit.ProductVariantID).Scan(&stockAfterFinalize)
	if stockAfterFinalize != 5 {
		t.Fatalf("expected stock 5, got %d", stockAfterFinalize)
	}

	// Supply must now be 'completed'
	supplyFinal, err := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to get final supply: %v", err)
	}
	if supplyFinal.Status != "completed" {
		t.Fatalf("expected supply status 'completed', got %s", supplyFinal.Status)
	}
}

func TestProcessFoundUnit_ExistingActiveSessionReused(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 2)

	// Start an active session beforehand
	activeSession, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start active session: %v", err)
	}

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to list units: %v", err)
	}

	// Process first unit via Free Scanner
	resp1, err := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("ProcessFoundUnit 1 failed: %v", err)
	}

	if resp1.ReceivingSessionID == nil || *resp1.ReceivingSessionID != activeSession.ID {
		t.Fatalf("expected active session %v to be reused, got %v", activeSession.ID, resp1.ReceivingSessionID)
	}
	if resp1.SessionScanned != 1 || resp1.SessionRemaining != 1 {
		t.Fatalf("expected 1 scanned, 1 remaining, got scanned=%d remaining=%d", resp1.SessionScanned, resp1.SessionRemaining)
	}

	// Duplicate scan of same unit in same session
	_, errDup := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "ok",
	})
	if !errors.Is(errDup, supplies.ErrUnitAlreadyScanned) {
		t.Fatalf("expected ErrUnitAlreadyScanned on duplicate scan, got %v", errDup)
	}

	// Process second unit with damaged condition
	resp2, err := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
		UnitCode:  units[1].UnitCode,
		Condition: "damaged",
	})
	if err != nil {
		t.Fatalf("ProcessFoundUnit 2 failed: %v", err)
	}
	if resp2.SessionScanned != 2 || resp2.SessionDamaged != 1 || resp2.SessionRemaining != 0 {
		t.Fatalf("expected 2 scanned, 1 damaged, 0 remaining, got scanned=%d damaged=%d remaining=%d",
			resp2.SessionScanned, resp2.SessionDamaged, resp2.SessionRemaining)
	}
	if resp2.RecommendedNextAction != "can_finalize" {
		t.Fatalf("expected can_finalize, got %s", resp2.RecommendedNextAction)
	}
}

func TestProcessFoundUnit_AlreadyInWarehouse(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupplyWithUnits(t, tc, 1)

	sessionA, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	units, _ := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessionA.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "ok",
	})
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessionA.ID, supplies.FinalizeReceivingRequest{})

	resp, err := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
		UnitCode:  units[0].UnitCode,
		Condition: "ok",
	})
	if err != nil {
		t.Fatalf("ProcessFoundUnit failed: %v", err)
	}
	if resp.UnitStatus != "warehouse" || resp.RecommendedNextAction != "already_in_warehouse" {
		t.Fatalf("expected warehouse / already_in_warehouse, got status=%s action=%s", resp.UnitStatus, resp.RecommendedNextAction)
	}
	if resp.ReceivingSessionID != nil {
		t.Fatalf("expected no receiving session ID for already processed unit")
	}
}
