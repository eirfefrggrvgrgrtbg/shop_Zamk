package supplies_test

import (
	"fmt"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func TestSupplyCreationSingleVariantQty13(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-13-SINGLE"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 13},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("unexpected create supply error: %v", err)
	}

	if supply.Status != "ready_to_ship" {
		t.Fatalf("expected status ready_to_ship, got %s", supply.Status)
	}

	if len(supply.Items) != 1 {
		t.Fatalf("expected 1 supply item, got %d", len(supply.Items))
	}
	if len(supply.Boxes) != 1 {
		t.Fatalf("expected 1 default box, got %d", len(supply.Boxes))
	}

	defaultBoxID := supply.Boxes[0].ID
	supplyItemID := supply.Items[0].ID

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to list units: %v", err)
	}

	if len(units) != 13 {
		t.Fatalf("expected exactly 13 units, got %d", len(units))
	}

	unitCodeRegex := regexp.MustCompile(`^ZMU-[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{16}$`)
	seenCodes := make(map[string]bool)

	for i, u := range units {
		expectedIndex := i + 1
		if u.UnitIndex != expectedIndex {
			t.Errorf("unit %d: expected unit_index %d, got %d", i, expectedIndex, u.UnitIndex)
		}
		if u.Status != "expected" {
			t.Errorf("unit %d: expected status 'expected', got %s", i, u.Status)
		}
		if u.ProductVariantID != tc.Variant1 {
			t.Errorf("unit %d: expected variant %s, got %s", i, tc.Variant1, u.ProductVariantID)
		}
		if u.OriginSupplyID != supply.ID {
			t.Errorf("unit %d: expected supply %s, got %s", i, supply.ID, u.OriginSupplyID)
		}
		if u.OriginSupplyItemID != supplyItemID {
			t.Errorf("unit %d: expected item %s, got %s", i, supplyItemID, u.OriginSupplyItemID)
		}
		if u.OriginBoxID == nil || *u.OriginBoxID != defaultBoxID {
			t.Errorf("unit %d: expected box %s, got %v", i, defaultBoxID, u.OriginBoxID)
		}
		if !unitCodeRegex.MatchString(u.UnitCode) {
			t.Errorf("unit %d: invalid unit code format: %s", i, u.UnitCode)
		}
		if seenCodes[u.UnitCode] {
			t.Errorf("unit %d: duplicate unit code: %s", i, u.UnitCode)
		}
		seenCodes[u.UnitCode] = true
	}
}

func TestSupplyCreation49UnitsFourVariants(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-49-FOUR"),
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

	if supply.Status != "ready_to_ship" {
		t.Fatalf("expected ready_to_ship, got %s", supply.Status)
	}

	// 1. Verify 49 total inventory units in DB
	var totalCount int
	err = testDB.QueryRow(tc.Ctx, "SELECT count(*) FROM inventory_units WHERE origin_supply_id = $1", supply.ID).Scan(&totalCount)
	if err != nil {
		t.Fatalf("failed to count units in DB: %v", err)
	}
	if totalCount != 49 {
		t.Fatalf("expected 49 total units in DB, got %d", totalCount)
	}

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to list units via repo: %v", err)
	}
	if len(units) != 49 {
		t.Fatalf("expected 49 units from repo, got %d", len(units))
	}

	unitCodeRegex := regexp.MustCompile(`^ZMU-[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{16}$`)
	seenCodes := make(map[string]bool)
	perItemUnits := make(map[uuid.UUID][]supplies.InventoryUnit)

	for _, u := range units {
		if !unitCodeRegex.MatchString(u.UnitCode) {
			t.Errorf("invalid unit code format: %s", u.UnitCode)
		}
		if seenCodes[u.UnitCode] {
			t.Errorf("duplicate unit code across 49 units: %s", u.UnitCode)
		}
		seenCodes[u.UnitCode] = true

		if u.Status != "expected" {
			t.Errorf("expected status 'expected', got %s", u.Status)
		}
		if u.OriginBoxID == nil || *u.OriginBoxID != supply.Boxes[0].ID {
			t.Errorf("expected default box %s, got %v", supply.Boxes[0].ID, u.OriginBoxID)
		}

		perItemUnits[u.OriginSupplyItemID] = append(perItemUnits[u.OriginSupplyItemID], u)
	}

	if len(supply.Items) != 4 {
		t.Fatalf("expected 4 supply items, got %d", len(supply.Items))
	}

	expectedCounts := make(map[uuid.UUID]int)
	quantitiesList := make([]int, 0, len(supply.Items))
	for _, item := range supply.Items {
		expectedCounts[item.ID] = item.ExpectedQuantity
		quantitiesList = append(quantitiesList, item.ExpectedQuantity)
	}

	// Verify quantities list contains 12, 12, 12, 13
	count12 := 0
	count13 := 0
	for _, q := range quantitiesList {
		if q == 12 {
			count12++
		} else if q == 13 {
			count13++
		}
	}
	if count12 != 3 || count13 != 1 {
		t.Fatalf("expected item quantities to be [12, 12, 12, 13], got: %v", quantitiesList)
	}

	for itemID, expQty := range expectedCounts {
		itemUnits := perItemUnits[itemID]
		if len(itemUnits) != expQty {
			t.Errorf("item %s: expected %d units, got %d", itemID, expQty, len(itemUnits))
		}
		for idx, u := range itemUnits {
			expectedIndex := idx + 1
			if u.UnitIndex != expectedIndex {
				t.Errorf("item %s unit %d: expected unit_index %d, got %d", itemID, idx, expectedIndex, u.UnitIndex)
			}
		}
	}
}

func TestSupplyAtomicRollbackOnUnitCreationFailure(t *testing.T) {
	tc := setupTestContext(t)

	// Pre-insert a unit with a collision code in DB
	collisionCode := "ZMU-COLLISIONTEST1"
	preSupplyID := uuid.New()
	preItemID := uuid.New()
	preBoxID := uuid.New()

	// Insert dummy supply/item/box for pre-existing unit
	_, err := testDB.Exec(tc.Ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, 'SUP-PREV', $2, 'ready_to_ship', 'carrier_delivery', now(), now())", preSupplyID, tc.SellerID)
	if err != nil {
		t.Fatalf("failed setup pre supply: %v", err)
	}
	_, err = testDB.Exec(tc.Ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())", preItemID, preSupplyID, tc.Variant1)
	if err != nil {
		t.Fatalf("failed setup pre item: %v", err)
	}
	_, err = testDB.Exec(tc.Ctx, "INSERT INTO seller_supply_boxes (id, supply_id, box_number, created_at) VALUES ($1, $2, 'BOX-PREV', now())", preBoxID, preSupplyID)
	if err != nil {
		t.Fatalf("failed setup pre box: %v", err)
	}
	_, err = testDB.Exec(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 1, 'expected', now(), now())", uuid.New(), collisionCode, tc.Variant1, preSupplyID, preItemID, preBoxID)
	if err != nil {
		t.Fatalf("failed setup pre unit: %v", err)
	}

	// Set generator to always return collisionCode so it repeatedly violates unique constraint and exhausts retries
	tc.Service.SetUnitCodeGeneratorForTest(func() (string, error) {
		return collisionCode, nil
	})

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-FAIL"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 5},
		},
	}

	failedSupply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err == nil {
		t.Fatalf("expected CreateSupply to fail due to unit_code uniqueness exhaustion, got success: %v", failedSupply)
	}

	// Verify rollback with SELECT: no rows must exist for any new supply created by this attempt
	var supplyCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM seller_supplies WHERE tracking_number = 'TRK-FAIL'").Scan(&supplyCount)
	if supplyCount != 0 {
		t.Errorf("atomic rollback failed: seller_supplies count = %d, expected 0", supplyCount)
	}

	var itemsCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM seller_supply_items WHERE supply_id NOT IN ($1)", preSupplyID).Scan(&itemsCount)
	if itemsCount != 0 {
		t.Errorf("atomic rollback failed: seller_supply_items count = %d, expected 0", itemsCount)
	}

	var boxesCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM seller_supply_boxes WHERE supply_id NOT IN ($1)", preSupplyID).Scan(&boxesCount)
	if boxesCount != 0 {
		t.Errorf("atomic rollback failed: seller_supply_boxes count = %d, expected 0", boxesCount)
	}

	var boxItemsCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM seller_supply_box_items WHERE box_id NOT IN ($1)", preBoxID).Scan(&boxItemsCount)
	if boxItemsCount != 0 {
		t.Errorf("atomic rollback failed: seller_supply_box_items count = %d, expected 0", boxItemsCount)
	}

	var unitsCount int
	testDB.QueryRow(tc.Ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id NOT IN ($1)", preSupplyID).Scan(&unitsCount)
	if unitsCount != 0 {
		t.Errorf("atomic rollback failed: inventory_units count = %d, expected 0", unitsCount)
	}
}

func TestSupplyUnitCodeCollisionRetrySuccess(t *testing.T) {
	tc := setupTestContext(t)

	// Pre-insert a unit with a known code in DB
	existingCode := "ZMU-COLLIDEONCE12"
	preSupplyID := uuid.New()
	preItemID := uuid.New()
	preBoxID := uuid.New()

	testDB.Exec(tc.Ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, 'SUP-COLL', $2, 'ready_to_ship', 'carrier_delivery', now(), now())", preSupplyID, tc.SellerID)
	testDB.Exec(tc.Ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 1, now(), now())", preItemID, preSupplyID, tc.Variant1)
	testDB.Exec(tc.Ctx, "INSERT INTO seller_supply_boxes (id, supply_id, box_number, created_at) VALUES ($1, $2, 'BOX-COLL', now())", preBoxID, preSupplyID)
	testDB.Exec(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 1, 'expected', now(), now())", uuid.New(), existingCode, tc.Variant1, preSupplyID, preItemID, preBoxID)

	// Sequence: 1st unit initial generates existingCode (will collide), then retry generates recoveryCode1. 2nd unit generates uniqueCode2.
	var seq int64
	recoveryCode1 := "ZMU-RECOVERED0001"
	uniqueCode2 := "ZMU-UNIQUE0000002"

	tc.Service.SetUnitCodeGeneratorForTest(func() (string, error) {
		n := atomic.AddInt64(&seq, 1)
		switch n {
		case 1:
			return existingCode, nil // Unit 1 initial -> collides
		case 2:
			return recoveryCode1, nil // Unit 1 retry -> succeeds
		case 3:
			return uniqueCode2, nil // Unit 2 initial -> succeeds
		default:
			return fmt.Sprintf("ZMU-DEF%013d", n), nil
		}
	})

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-RETRY"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 2},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("expected CreateSupply to succeed with retry, got: %v", err)
	}

	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to list units: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}

	codesMap := map[string]bool{units[0].UnitCode: true, units[1].UnitCode: true}
	if !codesMap[recoveryCode1] {
		t.Errorf("expected unit codes to contain %s, got: %v", recoveryCode1, units)
	}
	if !codesMap[uniqueCode2] {
		t.Errorf("expected unit codes to contain %s, got: %v", uniqueCode2, units)
	}
	if codesMap[existingCode] {
		t.Errorf("colliding code %s should not be present in created units", existingCode)
	}
}

func TestSupplyConstraintViolations(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-CONSTR"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 1},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// 1. (origin_supply_item_id, unit_index) duplicate rejected
	unitCode1, _ := supplies.GenerateUnitCode()
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 1, 'expected', now(), now()) RETURNING id", uuid.New(), unitCode1, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err == nil {
		t.Errorf("expected duplicate (origin_supply_item_id, unit_index) to be rejected")
	}

	// 2. external_marking_code duplicate non-null rejected
	unitCode2, _ := supplies.GenerateUnitCode()
	unitCode3, _ := supplies.GenerateUnitCode()
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, external_marking_code, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 2, 'EXT-TEST-1', 'expected', now(), now()) RETURNING id", uuid.New(), unitCode2, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err != nil {
		t.Fatalf("failed first external mark insert: %v", err)
	}
	err = testDB.QueryRow(tc.Ctx, "INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, external_marking_code, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 3, 'EXT-TEST-1', 'expected', now(), now()) RETURNING id", uuid.New(), unitCode3, tc.Variant1, supply.ID, supply.Items[0].ID, supply.Boxes[0].ID).Scan(new(uuid.UUID))
	if err == nil {
		t.Errorf("expected duplicate external marking to be rejected")
	}

	// 3. NULL external marking allowed for multiple rows
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

func TestSupplyCreationDoesNotMutateAggregateStock(t *testing.T) {
	tc := setupTestContext(t)

	// Set initial stock to 7 for variant 1
	_, err := testDB.Exec(tc.Ctx, "UPDATE inventory_items SET total_stock = 7, reserved_stock = 2 WHERE product_variant_id = $1", tc.Variant1)
	if err != nil {
		t.Fatalf("failed to seed inventory stock: %v", err)
	}

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-NOSTOCKCHANGE"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 25},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}

	// Verify inventory stock after create remains untouched
	var totalStock, reservedStock int
	err = testDB.QueryRow(tc.Ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1", tc.Variant1).Scan(&totalStock, &reservedStock)
	if err != nil {
		t.Fatalf("failed to query inventory stock: %v", err)
	}

	if totalStock != 7 {
		t.Fatalf("expected total_stock 7, got %d (aggregate inventory mutated by Supply creation!)", totalStock)
	}
	if reservedStock != 2 {
		t.Fatalf("expected reserved_stock 2, got %d", reservedStock)
	}

	// Verify units were still created
	units, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("failed to list units: %v", err)
	}
	if len(units) != 25 {
		t.Fatalf("expected 25 units, got %d", len(units))
	}
}

func TestSupplyStableIdentityRead(t *testing.T) {
	tc := setupTestContext(t)

	req := supplies.CreateSupplyRequest{
		HandoffMethod:  "carrier_delivery",
		CarrierName:    func(s string) *string { return &s }("СДЭК"),
		TrackingNumber: func(s string) *string { return &s }("TRK-READ-STABLE"),
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 3},
		},
	}

	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}

	// Read 1
	units1, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("read 1 failed: %v", err)
	}
	if len(units1) != 3 {
		t.Fatalf("read 1 expected 3 units, got %d", len(units1))
	}

	// Read 2
	units2, err := tc.Repo.ListUnitsBySupplyID(tc.Ctx, supply.ID)
	if err != nil {
		t.Fatalf("read 2 failed: %v", err)
	}
	if len(units2) != 3 {
		t.Fatalf("read 2 expected 3 units, got %d", len(units2))
	}

	// Verify exact match
	for i := 0; i < 3; i++ {
		if units1[i].ID != units2[i].ID {
			t.Errorf("unit %d ID mismatch: %s vs %s", i, units1[i].ID, units2[i].ID)
		}
		if units1[i].UnitCode != units2[i].UnitCode {
			t.Errorf("unit %d UnitCode mismatch: %s vs %s", i, units1[i].UnitCode, units2[i].UnitCode)
		}
		if units1[i].UnitIndex != units2[i].UnitIndex {
			t.Errorf("unit %d UnitIndex mismatch: %d vs %d", i, units1[i].UnitIndex, units2[i].UnitIndex)
		}
		if units1[i].Status != units2[i].Status {
			t.Errorf("unit %d Status mismatch: %s vs %s", i, units1[i].Status, units2[i].Status)
		}
	}
}
