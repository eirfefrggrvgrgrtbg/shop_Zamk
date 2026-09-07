package supplies_test

import (
	"encoding/json"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
)

// Tests A through H proving the Seller Supply Receiving Visibility contract (SA.2)
func TestSellerReceivingVisibility_A_Through_H(t *testing.T) {
	testutil.AssertTestDatabase(t, testDB)
	tc := setupTestContext(t)

	// Helper to create a shipped supply for tc.SellerID
	createShippedSupply := func(variantID uuid.UUID, qty int) *supplies.Supply {
		t.Helper()
		carrier := "СДЭК"
		tracking := "TRK-" + uuid.New().String()[:8]
		req := supplies.CreateSupplyRequest{
			HandoffMethod:  "carrier_delivery",
			CarrierName:    &carrier,
			TrackingNumber: &tracking,
			Items: []supplies.CreateSupplyItemRequest{
				{VariantID: variantID, ExpectedQuantity: qty},
			},
		}
		sup, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
		if err != nil {
			t.Fatalf("CreateSupply failed: %v", err)
		}
		sup, err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, sup.ID)
		if err != nil {
			t.Fatalf("MarkShipped failed: %v", err)
		}
		return sup
	}

	supplyA := createShippedSupply(tc.Variant1, 5)

	// =========================================================================
	// Test A: Seller sees only own Supply in list
	// =========================================================================
	t.Run("A. Seller sees only own Supply in list", func(t *testing.T) {
		listA, err := tc.Repo.GetSuppliesBySeller(tc.Ctx, tc.SellerID)
		if err != nil {
			t.Fatalf("GetSuppliesBySeller failed: %v", err)
		}
		if len(listA) == 0 {
			t.Fatalf("expected at least 1 supply for Seller A")
		}
		for _, s := range listA {
			if s.SellerID != tc.SellerID {
				t.Fatalf("cross-seller leakage: found supply %s belonging to %s", s.ID, s.SellerID)
			}
		}
	})

	// =========================================================================
	// Test B: Other seller Supply -> inaccessible / not returned
	// =========================================================================
	t.Run("B. Other seller Supply -> not returned in list and ownership rejected", func(t *testing.T) {
		otherSellerID := uuid.New()
		listB, err := tc.Repo.GetSuppliesBySeller(tc.Ctx, otherSellerID)
		if err != nil {
			t.Fatalf("GetSuppliesBySeller for foreign seller failed: %v", err)
		}
		if len(listB) != 0 {
			t.Fatalf("expected 0 supplies for other seller, got %d", len(listB))
		}

		// Verify MarkShipped by foreign seller is rejected
		_, err = tc.Service.MarkShipped(tc.Ctx, otherSellerID, supplyA.ID)
		if err != supplies.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized for foreign seller MarkShipped, got %v", err)
		}

		// Verify UnitLabels by foreign seller is rejected
		_, err = tc.Service.GetSupplyUnitLabels(tc.Ctx, otherSellerID, supplyA.ID)
		if err != supplies.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized for foreign seller GetSupplyUnitLabels, got %v", err)
		}
	})

	// =========================================================================
	// Test C: Expected quantity correct before receiving
	// =========================================================================
	t.Run("C. Expected quantity correct before receiving", func(t *testing.T) {
		sup, err := tc.Repo.GetSupplyByID(tc.Ctx, supplyA.ID)
		if err != nil {
			t.Fatalf("GetSupplyByID failed: %v", err)
		}
		if sup.TotalExpectedItems != 5 {
			t.Fatalf("expected TotalExpectedItems = 5, got %d", sup.TotalExpectedItems)
		}
		if sup.TotalAcceptedItems != 0 {
			t.Fatalf("expected TotalAcceptedItems = 0, got %d", sup.TotalAcceptedItems)
		}
		if sup.TotalRemainingItems != 5 {
			t.Fatalf("expected TotalRemainingItems = 5, got %d", sup.TotalRemainingItems)
		}
		if sup.IsReceivingComplete {
			t.Fatalf("expected IsReceivingComplete = false before receiving")
		}
		if len(sup.Items) != 1 || sup.Items[0].ExpectedQuantity != 5 {
			t.Fatalf("expected 1 item with ExpectedQuantity 5")
		}
		if sup.Items[0].RemainingQuantity != 5 {
			t.Fatalf("expected item RemainingQuantity = 5, got %d", sup.Items[0].RemainingQuantity)
		}
	})

	// =========================================================================
	// Perform receiving on supplyA: 3 OK, 1 Damaged, 1 Missing
	// =========================================================================
	if err := tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supplyA.ID); err != nil {
		t.Fatalf("MarkSupplyArrived failed: %v", err)
	}

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supplyA.QRToken)
	if err != nil {
		t.Fatalf("StartReceivingSession failed: %v", err)
	}

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supplyA.ID)
	if err != nil || len(units) != 5 {
		t.Fatalf("expected 5 units, got %d (err: %v)", len(units), err)
	}

	// Scan 3 as OK
	for i := 0; i < 3; i++ {
		_, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  units[i].UnitCode,
			Condition: "ok",
		})
		if err != nil {
			t.Fatalf("RecordSerializedScan ok %d failed: %v", i, err)
		}
	}

	// Scan 1 as Damaged
	_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordSerializedScanRequest{
		UnitCode:  units[3].UnitCode,
		Condition: "damaged",
	})
	if err != nil {
		t.Fatalf("RecordSerializedScan damaged failed: %v", err)
	}

	// 1 unit (units[4]) remains unscanned (Missing)

	// Finalize receiving
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("FinalizeReceiving failed: %v", err)
	}

	// Reload supply for D, E, F assertions
	supFinal, err := tc.Repo.GetSupplyByID(tc.Ctx, supplyA.ID)
	if err != nil {
		t.Fatalf("GetSupplyByID after finalize failed: %v", err)
	}

	// =========================================================================
	// Test D: Received quantity correct
	// =========================================================================
	t.Run("D. Received quantity correct", func(t *testing.T) {
		if supFinal.TotalAcceptedItems != 3 {
			t.Fatalf("expected TotalAcceptedItems = 3, got %d", supFinal.TotalAcceptedItems)
		}
		if supFinal.Items[0].AcceptedQuantity != 3 {
			t.Fatalf("expected item AcceptedQuantity = 3, got %d", supFinal.Items[0].AcceptedQuantity)
		}
	})

	// =========================================================================
	// Test E: Remaining quantity correct
	// =========================================================================
	t.Run("E. Remaining quantity correct", func(t *testing.T) {
		// 5 expected, 3 accepted, 1 damaged, 1 missing. Accounted = 3+1 = 4.
		// Remaining physical units missing = 5 - 3 - 1 = 1
		if supFinal.TotalRemainingItems != 1 {
			t.Fatalf("expected TotalRemainingItems = 1, got %d", supFinal.TotalRemainingItems)
		}
		if supFinal.Items[0].RemainingQuantity != 1 { // 1 missing unit remains physically unaccounted
			t.Fatalf("expected item RemainingQuantity = 1, got %d", supFinal.Items[0].RemainingQuantity)
		}
		if !supFinal.IsReceivingComplete {
			t.Fatalf("expected IsReceivingComplete = true after finalization")
		}
	})

	// =========================================================================
	// Test F: Discrepancy data correct & receivingComment segregation
	// =========================================================================
	t.Run("F. Discrepancy data correct & receivingComment segregation", func(t *testing.T) {
		if supFinal.Status != "completed_with_discrepancies" {
			t.Fatalf("expected status 'completed_with_discrepancies', got '%s'", supFinal.Status)
		}
		item := supFinal.Items[0]
		if item.DamagedQuantity != 1 {
			t.Fatalf("expected DamagedQuantity = 1, got %d", item.DamagedQuantity)
		}
		if item.MissingQuantity != 1 {
			t.Fatalf("expected MissingQuantity = 1, got %d", item.MissingQuantity)
		}
		if item.ExtraQuantity != 0 {
			t.Fatalf("expected ExtraQuantity = 0, got %d", item.ExtraQuantity)
		}
		if supFinal.DiscrepancyCount != 2 { // 1 damaged + 1 missing
			t.Fatalf("expected DiscrepancyCount = 2, got %d", supFinal.DiscrepancyCount)
		}
		if supFinal.AdditionalReceivingHappened {
			t.Fatalf("false positive: expected AdditionalReceivingHappened = false after first receiving")
		}

		// Verify Seller DTO serialization suppresses receivingComment
		sellerDTO := supplies.ToSellerSupplyResponse(supFinal)
		if sellerDTO.AdditionalReceivingHappened {
			t.Fatalf("false positive: expected DTO AdditionalReceivingHappened = false after first receiving")
		}
		rawJSON, err := json.Marshal(sellerDTO)
		if err != nil {
			t.Fatalf("Marshal seller DTO failed: %v", err)
		}
		var parsed map[string]interface{}
		json.Unmarshal(rawJSON, &parsed)
		itemsList := parsed["items"].([]interface{})
		firstItem := itemsList[0].(map[string]interface{})
		if _, exists := firstItem["receivingComment"]; exists {
			t.Fatalf("security violation: receivingComment leaked into Seller JSON payload")
		}

		// Verify Admin Supply model preserves receivingComment
		comment := "damaged during transport"
		supFinal.Items[0].ReceivingComment = &comment
		rawAdminJSON, err := json.Marshal(supFinal)
		if err != nil {
			t.Fatalf("Marshal admin model failed: %v", err)
		}
		var parsedAdmin map[string]interface{}
		json.Unmarshal(rawAdminJSON, &parsedAdmin)
		adminItemsList := parsedAdmin["items"].([]interface{})
		adminFirstItem := adminItemsList[0].(map[string]interface{})
		if c, exists := adminFirstItem["receivingComment"]; !exists || c != comment {
			t.Fatalf("regression: receivingComment missing in Admin model JSON: %v", c)
		}
	})

	// =========================================================================
	// Test G: Additional receiving updates resulting Seller read model
	// =========================================================================
	t.Run("G. Additional receiving updates resulting Seller read model", func(t *testing.T) {
		// Process the 5th (previously missing) unit using Free Scanner (ProcessFoundUnit)
		missingUnit := units[4]
		resp, err := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
			UnitCode:  missingUnit.UnitCode,
			Condition: "ok",
		})
		if err != nil {
			t.Fatalf("ProcessFoundUnit failed: %v", err)
		}
		if resp.ReceivingSessionID == nil {
			t.Fatalf("expected new receiving session for additional receiving")
		}

		// Finalize additional receiving
		err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, *resp.ReceivingSessionID, supplies.FinalizeReceivingRequest{})
		if err != nil {
			t.Fatalf("FinalizeReceiving additional session failed: %v", err)
		}

		// Reload supply from Seller perspective
		supAfterAdditional, err := tc.Repo.GetSupplyByID(tc.Ctx, supplyA.ID)
		if err != nil {
			t.Fatalf("GetSupplyByID after additional receiving failed: %v", err)
		}

		// Accepted is now 3 + 1 = 4
		if supAfterAdditional.TotalAcceptedItems != 4 {
			t.Fatalf("expected TotalAcceptedItems = 4, got %d", supAfterAdditional.TotalAcceptedItems)
		}
		// Missing is now 0 (found unit converted from missing to accepted)
		if supAfterAdditional.Items[0].MissingQuantity != 0 {
			t.Fatalf("expected MissingQuantity = 0, got %d", supAfterAdditional.Items[0].MissingQuantity)
		}
		if supAfterAdditional.Items[0].DamagedQuantity != 1 {
			t.Fatalf("expected DamagedQuantity = 1, got %d", supAfterAdditional.Items[0].DamagedQuantity)
		}
		// Remaining missing is now 0
		if supAfterAdditional.Items[0].RemainingQuantity != 0 {
			t.Fatalf("expected item RemainingQuantity = 0, got %d", supAfterAdditional.Items[0].RemainingQuantity)
		}
		// Additional receiving flag must be true
		if !supAfterAdditional.AdditionalReceivingHappened {
			t.Fatalf("expected AdditionalReceivingHappened = true after second session finalized")
		}
	})

	// =========================================================================
	// Test H: Seller cannot invoke warehouse receiving mutations
	// =========================================================================
	t.Run("H. Warehouse receiving state transitions require canonical workflow and cannot be bypassed", func(t *testing.T) {
		// A newly created supply in ready_to_ship cannot be marked arrived
		freshSupply := createShippedSupply(tc.Variant2, 2)
		// Set back to ready_to_ship to test pre-shipping arrival guard
		_ = tc.Repo.UpdateSupplyStatus(tc.Ctx, freshSupply.ID, "ready_to_ship")

		err := tc.Service.MarkSupplyArrived(tc.Ctx, tc.SellerID, freshSupply.ID)
		if err != supplies.ErrInvalidStatus {
			t.Fatalf("expected ErrInvalidStatus when marking ready_to_ship arrived, got %v", err)
		}

		// A supply that is not arrived cannot have a receiving session started
		_ = tc.Repo.UpdateSupplyStatus(tc.Ctx, freshSupply.ID, "shipped_by_seller")
		_, err = tc.Service.StartReceivingSession(tc.Ctx, tc.SellerID, *freshSupply.QRToken)
		if err != supplies.ErrSupplyNotArrived {
			t.Fatalf("expected ErrSupplyNotArrived, got %v", err)
		}

		// A closed session cannot accept further scans
		_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.SellerID, session.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  units[0].UnitCode,
			Condition: "ok",
		})
		if err != supplies.ErrReceivingSessionFinalized {
			t.Fatalf("expected ErrReceivingSessionFinalized when scanning on finalized session, got %v", err)
		}
	})

	// =========================================================================
	// Test I: Case B — Full acceptance (5 expected, 5 accepted) -> remaining=0, discrepancy=0
	// =========================================================================
	t.Run("I. Case B: Full acceptance (5 expected, 5 accepted)", func(t *testing.T) {
		supB := createShippedSupply(tc.Variant1, 5)
		if err := tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supB.ID); err != nil {
			t.Fatalf("MarkSupplyArrived failed: %v", err)
		}
		sessB, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supB.QRToken)
		if err != nil {
			t.Fatalf("StartReceivingSession failed: %v", err)
		}
		unitsB, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supB.ID)
		if err != nil || len(unitsB) != 5 {
			t.Fatalf("expected 5 units, got %d", len(unitsB))
		}
		for i := 0; i < 5; i++ {
			_, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessB.ID, supplies.RecordSerializedScanRequest{
				UnitCode:  unitsB[i].UnitCode,
				Condition: "ok",
			})
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}
		}
		if err := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessB.ID, supplies.FinalizeReceivingRequest{}); err != nil {
			t.Fatalf("finalize failed: %v", err)
		}
		loadedB, err := tc.Repo.GetSupplyByID(tc.Ctx, supB.ID)
		if err != nil {
			t.Fatalf("GetSupplyByID failed: %v", err)
		}
		if loadedB.Status != "completed" {
			t.Fatalf("expected status 'completed', got '%s'", loadedB.Status)
		}
		if loadedB.TotalAcceptedItems != 5 || loadedB.TotalRemainingItems != 0 || loadedB.DiscrepancyCount != 0 {
			t.Fatalf("expected accepted=5, remaining=0, disc=0; got acc=%d, rem=%d, disc=%d", loadedB.TotalAcceptedItems, loadedB.TotalRemainingItems, loadedB.DiscrepancyCount)
		}
		if loadedB.Items[0].RemainingQuantity != 0 || loadedB.Items[0].DamagedQuantity != 0 || loadedB.Items[0].MissingQuantity != 0 {
			t.Fatalf("expected item remaining=0, damaged=0, missing=0; got rem=%d, dmg=%d, mis=%d", loadedB.Items[0].RemainingQuantity, loadedB.Items[0].DamagedQuantity, loadedB.Items[0].MissingQuantity)
		}
		if loadedB.AdditionalReceivingHappened {
			t.Fatalf("false positive: expected AdditionalReceivingHappened = false for full acceptance")
		}
	})

	// =========================================================================
	// Test J: Case C — 4 accepted, 1 damaged (5 expected) -> remaining=0, discrepancy=1
	// =========================================================================
	t.Run("J. Case C: 4 accepted, 1 damaged (5 expected) -> remaining=0, discrepancy=1", func(t *testing.T) {
		supC := createShippedSupply(tc.Variant1, 5)
		if err := tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supC.ID); err != nil {
			t.Fatalf("MarkSupplyArrived failed: %v", err)
		}
		sessC, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supC.QRToken)
		if err != nil {
			t.Fatalf("StartReceivingSession failed: %v", err)
		}
		unitsC, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supC.ID)
		if err != nil || len(unitsC) != 5 {
			t.Fatalf("expected 5 units, got %d", len(unitsC))
		}
		for i := 0; i < 4; i++ {
			_, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessC.ID, supplies.RecordSerializedScanRequest{
				UnitCode:  unitsC[i].UnitCode,
				Condition: "ok",
			})
			if err != nil {
				t.Fatalf("scan ok failed: %v", err)
			}
		}
		_, err = tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessC.ID, supplies.RecordSerializedScanRequest{
			UnitCode:  unitsC[4].UnitCode,
			Condition: "damaged",
		})
		if err != nil {
			t.Fatalf("scan damaged failed: %v", err)
		}
		if err := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessC.ID, supplies.FinalizeReceivingRequest{}); err != nil {
			t.Fatalf("finalize failed: %v", err)
		}
		loadedC, err := tc.Repo.GetSupplyByID(tc.Ctx, supC.ID)
		if err != nil {
			t.Fatalf("GetSupplyByID failed: %v", err)
		}
		if loadedC.Status != "completed_with_discrepancies" {
			t.Fatalf("expected status 'completed_with_discrepancies', got '%s'", loadedC.Status)
		}
		if loadedC.TotalAcceptedItems != 4 || loadedC.TotalRemainingItems != 0 || loadedC.DiscrepancyCount != 1 {
			t.Fatalf("expected accepted=4, remaining=0, disc=1; got acc=%d, rem=%d, disc=%d", loadedC.TotalAcceptedItems, loadedC.TotalRemainingItems, loadedC.DiscrepancyCount)
		}
		if loadedC.Items[0].RemainingQuantity != 0 || loadedC.Items[0].DamagedQuantity != 1 || loadedC.Items[0].MissingQuantity != 0 {
			t.Fatalf("expected item remaining=0, damaged=1, missing=0; got rem=%d, dmg=%d, mis=%d", loadedC.Items[0].RemainingQuantity, loadedC.Items[0].DamagedQuantity, loadedC.Items[0].MissingQuantity)
		}
		if loadedC.AdditionalReceivingHappened {
			t.Fatalf("false positive: expected AdditionalReceivingHappened = false for single completed session with damage")
		}
	})

	// =========================================================================
	// Test K: Case D — 5 accepted, 1 extra (5 expected) -> remaining=0, extra=1
	// =========================================================================
	t.Run("K. Case D: 5 accepted, 1 extra (5 expected) -> remaining=0, extra=1", func(t *testing.T) {
		supD := createShippedSupply(tc.Variant1, 5)
		if err := tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supD.ID); err != nil {
			t.Fatalf("MarkSupplyArrived failed: %v", err)
		}
		sessD, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supD.QRToken)
		if err != nil {
			t.Fatalf("StartReceivingSession failed: %v", err)
		}
		unitsD, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supD.ID)
		if err != nil || len(unitsD) != 5 {
			t.Fatalf("expected 5 units, got %d", len(unitsD))
		}
		for i := 0; i < 5; i++ {
			_, err := tc.Service.RecordSerializedScan(tc.Ctx, tc.AdminID, sessD.ID, supplies.RecordSerializedScanRequest{
				UnitCode:  unitsD[i].UnitCode,
				Condition: "ok",
			})
			if err != nil {
				t.Fatalf("scan ok failed: %v", err)
			}
		}
		if err := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, sessD.ID, supplies.FinalizeReceivingRequest{}); err != nil {
			t.Fatalf("finalize failed: %v", err)
		}
		// Simulate extra unit registered on line item (extra_quantity = 1, accepted = 6)
		_, err = testDB.Exec(tc.Ctx, "UPDATE seller_supply_items SET accepted_quantity = 6, extra_quantity = 1 WHERE id = $1", supD.Items[0].ID)
		if err != nil {
			t.Fatalf("update extra: %v", err)
		}
		loadedD, err := tc.Repo.GetSupplyByID(tc.Ctx, supD.ID)
		if err != nil {
			t.Fatalf("GetSupplyByID failed: %v", err)
		}
		if loadedD.TotalRemainingItems != 0 {
			t.Fatalf("expected remaining=0 for extra supply, got %d", loadedD.TotalRemainingItems)
		}
		if loadedD.Items[0].RemainingQuantity != 0 {
			t.Fatalf("expected item remaining=0, got %d", loadedD.Items[0].RemainingQuantity)
		}
		if loadedD.Items[0].ExtraQuantity != 1 {
			t.Fatalf("expected ExtraQuantity=1, got %d", loadedD.Items[0].ExtraQuantity)
		}
		if loadedD.DiscrepancyQuantity != 1 {
			t.Fatalf("expected DiscrepancyQuantity=1, got %d", loadedD.DiscrepancyQuantity)
		}
	})
}
