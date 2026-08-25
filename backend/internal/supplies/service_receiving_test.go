package supplies_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func createShippedSupply(t *testing.T, tc *TestContext) *supplies.Supply {
	return createShippedSupplyWithUnits(t, tc, 10)
}

func createShippedSupplyWithUnits(t *testing.T, tc *TestContext, qty int) *supplies.Supply {
	carrier := "СДЭК"
	tracking := "121212123241"
	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    &carrier,
		TrackingNumber: &tracking,
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: qty},
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
	supply, err = tc.Repo.GetSupplyByID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to get supply: %v", err)
	}
	return supply
}

func createLegacyShippedSupply(t *testing.T, tc *TestContext, qty int) *supplies.Supply {
	supplyID := uuid.New()
	supplyNumber := "SUP-LEGACY-" + supplyID.String()[:8]
	qrToken := "qr-legacy-" + supplyID.String()[:8]
	now := time.Now().UTC()

	_, err := testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at)
		VALUES ($1, $2, $3, 'ready_to_ship', 'carrier_delivery', $4, $5, $5)
	`, supplyID, supplyNumber, tc.SellerID, qrToken, now)
	if err != nil {
		t.Fatalf("failed to insert legacy supply: %v", err)
	}

	itemID := uuid.New()
	_, err = testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, itemID, supplyID, tc.Variant1, qty, now)
	if err != nil {
		t.Fatalf("failed to insert legacy supply item: %v", err)
	}

	boxID := uuid.New()
	_, err = testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at)
		VALUES ($1, $2, 'BOX-01', $3, $4)
	`, boxID, supplyID, "box-"+qrToken, now)
	if err != nil {
		t.Fatalf("failed to insert legacy supply box: %v", err)
	}

	_, err = testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_box_items (box_id, supply_item_id, quantity)
		VALUES ($1, $2, $3)
	`, boxID, itemID, qty)
	if err != nil {
		t.Fatalf("failed to insert legacy supply box item: %v", err)
	}

	_, err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supplyID)
	if err != nil {
		t.Fatalf("failed to mark legacy shipped: %v", err)
	}
	err = tc.Service.MarkSupplyArrived(tc.Ctx, tc.AdminID, supplyID)
	if err != nil {
		t.Fatalf("failed to mark legacy arrived: %v", err)
	}
	supply, err := tc.Repo.GetSupplyByID(tc.Ctx, supplyID)
	if err != nil {
		t.Fatalf("failed to get legacy supply: %v", err)
	}
	return supply
}

func TestReceivingClean(t *testing.T) {
	tc := setupTestContext(t)
	supply := createLegacyShippedSupply(t, tc, 10)

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
	supply := createLegacyShippedSupply(t, tc, 10)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}
	
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
	supply := createLegacyShippedSupply(t, tc, 10)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}
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
	supply := createLegacyShippedSupply(t, tc, 10)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}
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
	supply := createLegacyShippedSupply(t, tc, 10)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}
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
	supply := createLegacyShippedSupply(t, tc, 20)

	// Capture stock before
	var stockBefore int
	testDB.QueryRow(tc.Ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&stockBefore)

	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, *supply.QRToken)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

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
