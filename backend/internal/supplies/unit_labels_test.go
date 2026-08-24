package supplies_test

import (
	"errors"
	"regexp"
	"testing"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func TestSupplyUnitLabels49UnitsFourVariants(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-49-LABELS"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 12},
			{VariantID: tc.Variant2, ExpectedQuantity: 12},
			{VariantID: tc.Variant3, ExpectedQuantity: 12},
			{VariantID: tc.Variant4, ExpectedQuantity: 13},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("unexpected create supply error: %v", err)
	}

	res, err := tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("unexpected GetSupplyUnitLabels error: %v", err)
	}

	if !res.Serialized {
		t.Errorf("expected Serialized to be true")
	}
	if res.TotalUnits != 49 {
		t.Fatalf("expected TotalUnits 49, got %d", res.TotalUnits)
	}
	if len(res.Units) != 49 {
		t.Fatalf("expected len(Units) 49, got %d", len(res.Units))
	}
	if res.Box == nil || res.Box.BoxNumber == "" {
		t.Fatalf("expected box information in response")
	}

	unitCodeRegex := regexp.MustCompile(`^ZMU-[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{16}$`)
	seenCodes := make(map[string]bool)
	variantCounts := make(map[uuid.UUID]int)

	for i, u := range res.Units {
		if !unitCodeRegex.MatchString(u.UnitCode) {
			t.Errorf("unit %d: invalid unit_code format: %s", i, u.UnitCode)
		}
		if seenCodes[u.UnitCode] {
			t.Errorf("unit %d: duplicate unit_code: %s", i, u.UnitCode)
		}
		seenCodes[u.UnitCode] = true

		if u.ProductTitle == "" {
			t.Errorf("unit %d: empty product title", i)
		}
		if u.SellerSKU == nil || *u.SellerSKU == "" {
			t.Errorf("unit %d: missing seller SKU", i)
		}
		if u.VariantBarcode == nil || *u.VariantBarcode == "" {
			t.Errorf("unit %d: missing variant barcode (ZMK)", i)
		}
		if u.BoxNumber == nil || *u.BoxNumber != res.Box.BoxNumber {
			t.Errorf("unit %d: unexpected box number: %v", i, u.BoxNumber)
		}

		variantCounts[u.ProductVariantID]++
	}

	if variantCounts[tc.Variant1] != 12 {
		t.Errorf("variant 1: expected 12 units, got %d", variantCounts[tc.Variant1])
	}
	if variantCounts[tc.Variant2] != 12 {
		t.Errorf("variant 2: expected 12 units, got %d", variantCounts[tc.Variant2])
	}
	if variantCounts[tc.Variant3] != 12 {
		t.Errorf("variant 3: expected 12 units, got %d", variantCounts[tc.Variant3])
	}
	if variantCounts[tc.Variant4] != 13 {
		t.Errorf("variant 4: expected 13 units, got %d", variantCounts[tc.Variant4])
	}
}

func TestSupplyUnitLabelsSellerOwnershipAndForbidden(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-AUTH"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 2},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}

	// 1. Legitimate seller can read
	res, err := tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("expected seller to access own supply unit labels: %v", err)
	}
	if len(res.Units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(res.Units))
	}

	// 2. Other seller is forbidden
	otherSellerID := uuid.New()
	_, err = tc.Service.GetSupplyUnitLabels(tc.Ctx, otherSellerID, supply.ID)
	if !errors.Is(err, supplies.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for other seller, got: %v", err)
	}

	// 3. Non-existent supply returns not found
	_, err = tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, uuid.New())
	if !errors.Is(err, supplies.ErrSupplyNotFound) {
		t.Fatalf("expected ErrSupplyNotFound, got: %v", err)
	}
}

func TestSupplyUnitLabelsDeterministicOrderAndStability(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-ORDER-STABLE"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 5},
			{VariantID: tc.Variant2, ExpectedQuantity: 4},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}

	res1, err := tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("read 1 failed: %v", err)
	}

	res2, err := tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("read 2 failed: %v", err)
	}

	if len(res1.Units) != len(res2.Units) || len(res1.Units) != 9 {
		t.Fatalf("expected 9 units in both reads, got %d and %d", len(res1.Units), len(res2.Units))
	}

	for i := 0; i < len(res1.Units); i++ {
		u1 := res1.Units[i]
		u2 := res2.Units[i]

		if u1.InventoryUnitID != u2.InventoryUnitID {
			t.Errorf("index %d ID mismatch: %s vs %s", i, u1.InventoryUnitID, u2.InventoryUnitID)
		}
		if u1.UnitCode != u2.UnitCode {
			t.Errorf("index %d UnitCode mismatch: %s vs %s", i, u1.UnitCode, u2.UnitCode)
		}
		if u1.UnitIndex != u2.UnitIndex {
			t.Errorf("index %d UnitIndex mismatch: %d vs %d", i, u1.UnitIndex, u2.UnitIndex)
		}
		if u1.SupplyItemID != u2.SupplyItemID {
			t.Errorf("index %d SupplyItemID mismatch: %s vs %s", i, u1.SupplyItemID, u2.SupplyItemID)
		}
	}

	// Verify unit_index is ascending within each item
	itemIndexMap := make(map[uuid.UUID][]int)
	for _, u := range res1.Units {
		itemIndexMap[u.SupplyItemID] = append(itemIndexMap[u.SupplyItemID], u.UnitIndex)
	}
	for itemID, indices := range itemIndexMap {
		for idx, val := range indices {
			expectedVal := idx + 1
			if val != expectedVal {
				t.Errorf("item %s index position %d: expected %d, got %d", itemID, idx, expectedVal, val)
			}
		}
	}
}

func TestSupplyUnitLabelsNoSideEffects(t *testing.T) {
	tc := setupTestContext(t)

	// Set initial stock to 15
	_, err := testDB.Exec(tc.Ctx, "UPDATE inventory_items SET total_stock = 15, reserved_stock = 3 WHERE product_variant_id = $1", tc.Variant1)
	if err != nil {
		t.Fatalf("failed to seed inventory stock: %v", err)
	}

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-READ-NO-MUTATE"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 3},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}

	var countBefore int
	err = testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1", supply.ID).Scan(&countBefore)
	if err != nil || countBefore != 3 {
		t.Fatalf("expected 3 units before GET labels, got %d, err: %v", countBefore, err)
	}

	// Perform 5 consecutive reads
	for i := 1; i <= 5; i++ {
		res, err := tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, supply.ID)
		if err != nil {
			t.Fatalf("read %d failed: %v", i, err)
		}
		if len(res.Units) != 3 {
			t.Fatalf("read %d expected 3 units, got %d", i, len(res.Units))
		}
	}

	// Verify inventory_units row count remains 3
	var countAfter int
	err = testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1", supply.ID).Scan(&countAfter)
	if err != nil || countAfter != 3 {
		t.Fatalf("expected 3 units after 5 reads, got %d, err: %v", countAfter, err)
	}

	// Verify aggregate stock is unchanged
	var totalStock, reservedStock int
	err = testDB.QueryRow(tc.Ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&totalStock, &reservedStock)
	if err != nil {
		t.Fatalf("failed to query inventory stock: %v", err)
	}
	if totalStock != 15 || reservedStock != 3 {
		t.Fatalf("stock mutated by GET labels! total_stock=%d, reserved_stock=%d", totalStock, reservedStock)
	}

	// Verify supply status is unchanged
	var supplyStatus string
	err = testDB.QueryRow(tc.Ctx, "SELECT status FROM seller_supplies WHERE id = $1", supply.ID).Scan(&supplyStatus)
	if err != nil || supplyStatus != "ready_to_ship" {
		t.Fatalf("supply status mutated! status=%s, err=%v", supplyStatus, err)
	}
}

func TestSupplyUnitLabelsLegacySupply(t *testing.T) {
	tc := setupTestContext(t)

	// Create a supply and item directly in DB without creating inventory_units (simulating pre-M64 legacy supply)
	legacySupplyID := uuid.New()
	legacySupplyNumber := "SUP-LEGACY-001"
	_, err := testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, seller_id, supply_number, status, handoff_method, created_at, updated_at)
		VALUES ($1, $2, $3, 'completed', 'carrier_delivery', now(), now())
	`, legacySupplyID, tc.SellerID, legacySupplyNumber)
	if err != nil {
		t.Fatalf("failed to insert legacy supply: %v", err)
	}

	legacyItemID := uuid.New()
	_, err = testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 5, 5, now(), now())
	`, legacyItemID, legacySupplyID, tc.Variant1)
	if err != nil {
		t.Fatalf("failed to insert legacy supply item: %v", err)
	}

	// GET labels on legacy supply
	res, err := tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, legacySupplyID)
	if err != nil {
		t.Fatalf("unexpected error for legacy supply: %v", err)
	}

	if res.Serialized {
		t.Errorf("expected Serialized to be false for legacy supply")
	}
	if res.TotalUnits != 0 {
		t.Errorf("expected TotalUnits to be 0, got %d", res.TotalUnits)
	}
	if len(res.Units) != 0 {
		t.Errorf("expected 0 units for legacy supply, got %d", len(res.Units))
	}

	// Verify no units were created in DB
	var count int
	err = testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1", legacySupplyID).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("legacy supply read must NOT insert inventory_units, found %d rows", count)
	}
}

func TestSupplyUnitLabelsInvariantMismatch(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-MISMATCH"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 3},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}

	// Manually delete 1 inventory unit to construct corrupted state: expected=3, actual=2
	_, err = testDB.Exec(tc.Ctx, "DELETE FROM inventory_units WHERE id = (SELECT id FROM inventory_units WHERE origin_supply_id = $1 LIMIT 1)", supply.ID)
	if err != nil {
		t.Fatalf("failed to delete 1 unit: %v", err)
	}

	// Calling GetSupplyUnitLabels should return ErrSupplyUnitIdentityMismatch
	_, err = tc.Service.GetSupplyUnitLabels(tc.Ctx, tc.SellerID, supply.ID)
	if !errors.Is(err, supplies.ErrSupplyUnitIdentityMismatch) {
		t.Fatalf("expected ErrSupplyUnitIdentityMismatch, got: %v", err)
	}

	// Verify no auto-creation / repair happened
	var count int
	err = testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1", supply.ID).Scan(&count)
	if err != nil || count != 2 {
		t.Fatalf("expected unit count to remain 2 (no repair), got %d", count)
	}
}
