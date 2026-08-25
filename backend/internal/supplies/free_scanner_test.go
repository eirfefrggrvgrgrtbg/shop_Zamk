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
