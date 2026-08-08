package supplies_test

import (
	"sync"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func createShippedSupply(t *testing.T, tc *TestContext) *supplies.Supply {
	req := supplies.CreateSupplyRequest{
		HandoffMethod: "pvz",
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 10},
		},
		Boxes: []supplies.CreateSupplyBoxRequest{
			{BoxNumber: "1", Items: []supplies.CreateSupplyBoxItemRequest{{VariantID: tc.Variant1, Quantity: 10}}},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}
	err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("failed to mark shipped: %v", err)
	}
	return supply
}

func TestReceivingClean(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	// Admin starts session
	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}
	
	// Admin records scan (10 ok)
	err = tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:  10,
		IsDamage:  false,
	})
	if err != nil {
		t.Fatalf("failed to record scan: %v", err)
	}

	// Admin finalizes session
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("failed to finalize: %v", err)
	}

	// Verify supply status is completed
	finalSupply, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if finalSupply.Status != "completed" {
		t.Fatalf("expected completed, got %s", finalSupply.Status)
	}
}

func TestReceivingWithDiscrepancy(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	session, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	
	// Admin records scan (8 ok, 1 damaged)
	tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:  8,
		IsDamage:  false,
	})
	tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:  1,
		IsDamage:  true,
	})

	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})

	finalSupply, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if finalSupply.Status != "completed_with_discrepancies" {
		t.Fatalf("expected completed_with_discrepancies, got %s", finalSupply.Status)
	}
}

func TestReceivingInventoryIncrement(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	session, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:  10,
		IsDamage:  false,
	})
	tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})

	// Check stock
	var stock int
	testDB.QueryRow(tc.Ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stock)
	if stock != 10 {
		t.Fatalf("expected 10 stock, got %d", stock)
	}
}

func TestReceivingDoubleFinalize(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	session, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:  10,
		IsDamage:  false,
	})
	
	err1 := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err1 != nil {
		t.Fatalf("first finalize should succeed")
	}

	err2 := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err2 == nil || err2.Error() != "session is not active" {
		t.Fatalf("second finalize should fail with session not active, got: %v", err2)
	}

	// Check stock is only incremented once
	var stock int
	testDB.QueryRow(tc.Ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stock)
	if stock != 10 {
		t.Fatalf("expected 10 stock (single increment), got %d", stock)
	}
}

func TestReceivingConcurrentFinalize(t *testing.T) {
	tc := setupTestContext(t)
	supply := createShippedSupply(t, tc)

	session, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:     10,
		IsDamage:     false,
	})
	
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	
	// Send 2 concurrent finalizations
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	successCount := 0
	failCount := 0
	for err := range errs {
		if err == nil {
			successCount++
		} else {
			failCount++
		}
	}

	if successCount != 1 {
		t.Fatalf("expected exactly 1 success, got %d", successCount)
	}
	if failCount != 1 {
		t.Fatalf("expected exactly 1 failure, got %d", failCount)
	}

	// Check stock is exactly 10
	var stock int
	testDB.QueryRow(tc.Ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stock)
	if stock != 10 {
		t.Fatalf("expected 10 stock (single increment), got %d", stock)
	}
}

func TestReceivingDamagePersistsCorrectly(t *testing.T) {
	tc := setupTestContext(t)

	// Create supply with expected = 20
	req := supplies.CreateSupplyRequest{
		HandoffMethod: "pvz",
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 20},
		},
		Boxes: []supplies.CreateSupplyBoxRequest{
			{BoxNumber: "1", Items: []supplies.CreateSupplyBoxItemRequest{{VariantID: tc.Variant1, Quantity: 20}}},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}
	err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("failed to mark shipped: %v", err)
	}

	// Capture stock before
	var stockBefore int
	testDB.QueryRow(tc.Ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockBefore)

	// Admin starts session
	session, _ := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)

	// Accepted = 17
	tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:  17,
		IsDamage:  false,
	})
	// Damaged = 2
	tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: *session.Items[0].VariantID,
		Quantity:  2,
		IsDamage:  true,
	})

	// Missing = 1 implicitly (20 - 17 - 2 = 1)
	err = tc.Service.FinalizeReceiving(tc.Ctx, tc.AdminID, session.ID, supplies.FinalizeReceivingRequest{})
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	// Verify stock = Before + 17
	var stockAfter int
	testDB.QueryRow(tc.Ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockAfter)

	if stockAfter != stockBefore+17 {
		t.Fatalf("expected stock %d, got %d", stockBefore+17, stockAfter)
	}

	// Verify supply items persisted expected=20, accepted=17, damaged=2, missing=1
	finalSupply, _ := tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if len(finalSupply.Items) == 0 {
		t.Fatalf("supply items empty")
	}
	item := finalSupply.Items[0]

	if item.ExpectedQuantity != 20 {
		t.Errorf("ExpectedQuantity: got %d, want 20", item.ExpectedQuantity)
	}
	if item.AcceptedQuantity != 17 {
		t.Errorf("AcceptedQuantity: got %d, want 17", item.AcceptedQuantity)
	}
	if item.DamagedQuantity != 2 {
		t.Errorf("DamagedQuantity: got %d, want 2", item.DamagedQuantity)
	}
	if item.MissingQuantity != 1 {
		t.Errorf("MissingQuantity: got %d, want 1", item.MissingQuantity)
	}
}
