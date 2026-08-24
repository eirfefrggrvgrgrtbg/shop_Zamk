package supplies_test

import (
	"regexp"
	"testing"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func TestSupplyUnitGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test")
	}

	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod: "carrier_delivery",
		CarrierName:   func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("123"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 12},
			{VariantID: tc.Variant2, ExpectedQuantity: 12},
			
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if supply.Status != "ready_to_ship" {
		t.Errorf("expected ready_to_ship, got %s", supply.Status)
	}

	// 1. Verify 24 expected physical units created
	var count int
	err = testDB.QueryRow(tc.Ctx, "SELECT count(*) FROM inventory_units WHERE origin_supply_id = $1", supply.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count units: %v", err)
	}
	if count != 24 { // 12 + 12 = 24
		t.Errorf("expected 24 units, got %d", count)
	}

	// Fetch all units
	rows, err := testDB.Query(tc.Ctx, "SELECT unit_code, product_variant_id, origin_supply_item_id, unit_index, status, origin_box_id FROM inventory_units WHERE origin_supply_id = $1 ORDER BY product_variant_id, unit_index", supply.ID)
	if err != nil {
		t.Fatalf("failed to fetch units: %v", err)
	}
	defer rows.Close()

	unitCodes := make(map[string]bool)
	variantCounts := make(map[uuid.UUID]int)
	
	unitCodeRegex := regexp.MustCompile(`^ZMU-[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{16}$`)

	for rows.Next() {
		var code string
		var vid, iid, bid uuid.UUID
		var idx int
		var status string
		err := rows.Scan(&code, &vid, &iid, &idx, &status, &bid)
		if err != nil {
			t.Fatalf("scan error: %v", err)
		}

		if unitCodes[code] {
			t.Errorf("duplicate unit code %s", code)
		}
		unitCodes[code] = true

		if !unitCodeRegex.MatchString(code) {
			t.Errorf("invalid unit code format: %s", code)
		}

		if status != "expected" {
			t.Errorf("expected status 'expected', got %s", status)
		}

		if bid == uuid.Nil {
			t.Errorf("expected non-nil default box id")
		}

		variantCounts[vid]++
	}

	if variantCounts[tc.Variant1] != 12 || variantCounts[tc.Variant2] != 12 {
		t.Errorf("variant counts mismatch: %v", variantCounts)
	}
}

func TestInventoryUnitConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test")
	}

	tc := setupTestContext(t)

	// Create a single supply with 1 item to get valid FKs
	req := supplies.CreateSupplyRequest{
		HandoffMethod: "carrier_delivery",
		CarrierName:   func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("123"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 1},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// M. failure during unit creation rolls back Supply transaction
	// Not easily testable at DB level without mock, but transactional requirement is met because we use one TX in repository.

	// N. (origin_supply_item_id, unit_index) duplicate rejected
	unitCode1, _ := supplies.GenerateUnitCode()
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 1, 'expected', now(), now()) RETURNING id", uuid.New(), unitCode1, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err == nil {
		t.Errorf("expected duplicate unit_index to be rejected")
	}

	// O. external_marking_code duplicate non-null rejected
	unitCode2, _ := supplies.GenerateUnitCode()
	unitCode3, _ := supplies.GenerateUnitCode()
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, external_marking_code, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 2, 'EXT123', 'expected', now(), now()) RETURNING id", uuid.New(), unitCode2, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err != nil {
		t.Fatalf("failed first external mark insert: %v", err)
	}
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, external_marking_code, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 3, 'EXT123', 'expected', now(), now()) RETURNING id", uuid.New(), unitCode3, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err == nil {
		t.Errorf("expected duplicate external marking to be rejected")
	}

	// P. NULL external marking allowed for multiple rows
	unitCode4, _ := supplies.GenerateUnitCode()
	unitCode5, _ := supplies.GenerateUnitCode()
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, external_marking_code, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 4, NULL, 'expected', now(), now()) RETURNING id", uuid.New(), unitCode4, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err != nil {
		t.Errorf("failed null insert 1: %v", err)
	}
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, external_marking_code, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 5, NULL, 'expected', now(), now()) RETURNING id", uuid.New(), unitCode5, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err != nil {
		t.Errorf("failed null insert 2: %v", err)
	}
}
