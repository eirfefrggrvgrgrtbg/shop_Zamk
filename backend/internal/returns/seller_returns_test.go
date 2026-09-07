package returns_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

// Helper to create supply, inventory items, inventory_units and allocations
func createReturnTestAllocations(t *testing.T, fix *m51Fixture, orderID, orderItemID, variantID, productID, sellerID uuid.UUID, count int) ([]string, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	supplyID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', 'SUP-' || substr(md5(random()::text), 1, 8), 'pickup', now(), now())
	`, supplyID, sellerID)
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, supplyItemID, supplyID, variantID, count)
	require.NoError(t, err)

	var invItemID uuid.UUID
	err = fix.client.Pool.QueryRow(ctx, `
		INSERT INTO inventory_items (id, seller_id, product_id, product_variant_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, $5, 0)
		ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = inventory_items.total_stock + EXCLUDED.total_stock
		RETURNING id
	`, uuid.New(), sellerID, productID, variantID, count).Scan(&invItemID)
	require.NoError(t, err)

	var unitCodes []string
	var allocIDs []uuid.UUID
	for i := 0; i < count; i++ {
		invUnitID := uuid.New()
		zmu := fmt.Sprintf("ZMU-SA3-%s-%d", uuid.New().String()[:8], i)
		unitCodes = append(unitCodes, zmu)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'shipped')
		`, invUnitID, zmu, variantID, supplyID, supplyItemID, i+1)
		require.NoError(t, err)

		resID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
			VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
		`, resID, invItemID, productID, variantID, fix.userID, orderID)
		require.NoError(t, err)

		allocID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at)
			VALUES ($1, $2, $3, $4, now() - interval '2 hour')
		`, allocID, orderItemID, invUnitID, resID)
		require.NoError(t, err)
		allocIDs = append(allocIDs, allocID)
	}

	return unitCodes, allocIDs
}

// 1. Restocked physical outcome visibility
func TestSellerReturn_PhysicalOutcome_Restocked(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Did not fit"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	retID := resp[0].Return.ID

	// Approve return
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"})
	require.NoError(t, err)

	// Create arrived shipment with tracking number and method
	trk := "TRK-CDEK-RESTOCK-1"
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', $3, 'arrived_at_zamk')
	`, uuid.New(), retID, trk)
	require.NoError(t, err)

	// Warehouse receiving: start -> scan -> inspect restock -> finalize
	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err)

	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "as_new"
	err = fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "restock",
		InspectedCondition: &cond,
	})
	require.NoError(t, err)

	err = fix.svc.FinalizeReceiving(ctx, retID)
	require.NoError(t, err)

	// Query Seller visibility
	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "restocked", item.PhysicalOutcome)
	assert.Equal(t, 1, item.RestockedQuantity)
	assert.Equal(t, 0, item.DamagedQuantity)
	assert.Equal(t, 0, item.RejectedQuantity)
	assert.Equal(t, 0, item.NotReceivedQuantity)
	assert.True(t, item.Restock)
	assert.True(t, item.ArrivedAtZamk)
	assert.True(t, item.InspectionCompleted)
	assert.Equal(t, "completed", item.ProcessingStatus)
	assert.NotNil(t, item.ReceivingStartedAt)
	assert.Nil(t, item.CompletedAt) // In item_received state, receiving completed but return not yet completed
	require.NotNil(t, item.TrackingNumber)
	assert.Equal(t, trk, *item.TrackingNumber)
	require.NotNil(t, item.ShipmentMethod)
	assert.Equal(t, "cdek_courier", *item.ShipmentMethod)

	// Transition to completed -> CompletedAt is set
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "completed"})
	require.NoError(t, err)

	itemsCompleted, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	assert.NotNil(t, itemsCompleted[0].CompletedAt)
	assert.Equal(t, "restocked", itemsCompleted[0].PhysicalOutcome)

	// Units breakdown
	require.Len(t, item.Units, 1)
	assert.Equal(t, unitCodes[0], item.Units[0].UnitCode)
	require.NotNil(t, item.Units[0].Disposition)
	assert.Equal(t, "restock", *item.Units[0].Disposition)
	assert.NotNil(t, item.Units[0].ScannedAt)
}

// 2. Damaged physical outcome visibility
func TestSellerReturn_PhysicalOutcome_Damaged(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Damaged zipper"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"})
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'TRK-CDEK-DMG-1', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err)

	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "torn_fabric"
	err = fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond,
	})
	require.NoError(t, err)

	err = fix.svc.FinalizeReceiving(ctx, retID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "damaged", item.PhysicalOutcome)
	assert.Equal(t, 0, item.RestockedQuantity)
	assert.Equal(t, 1, item.DamagedQuantity)
	assert.Equal(t, 0, item.RejectedQuantity)
	assert.Equal(t, 0, item.NotReceivedQuantity)
	assert.False(t, item.Restock)
	assert.True(t, item.ArrivedAtZamk)
	assert.True(t, item.InspectionCompleted)

	require.Len(t, item.Units, 1)
	assert.Equal(t, unitCodes[0], item.Units[0].UnitCode)
	require.NotNil(t, item.Units[0].Disposition)
	assert.Equal(t, "damaged", *item.Units[0].Disposition)
}

// 3. In-inspection state visibility
func TestSellerReturn_PhysicalOutcome_InInspection(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Testing in inspection"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"})
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'TRK-CDEK-INSP-1', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "in_inspection", item.PhysicalOutcome)
	assert.Equal(t, "receiving", item.ProcessingStatus)
	assert.True(t, item.ArrivedAtZamk)
	assert.False(t, item.InspectionCompleted)
	assert.NotNil(t, item.ReceivingStartedAt)
	assert.Nil(t, item.CompletedAt)
}

// 4. Multi-quantity mixed outcome (quantity=2: 1 restock, 1 damaged)
func TestSellerReturn_PhysicalOutcome_MultiQuantityMixed(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Multi-quantity test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 2},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"})
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'TRK-CDEK-MIX-2', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err)

	// Scan unit 1 -> restock
	scan1, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)
	cond1 := "good"
	err = fix.svc.InspectSerializedUnit(ctx, retID, scan1.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "restock",
		InspectedCondition: &cond1,
	})
	require.NoError(t, err)

	// Scan unit 2 -> damaged
	scan2, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[1]})
	require.NoError(t, err)
	cond2 := "broken_zipper"
	err = fix.svc.InspectSerializedUnit(ctx, retID, scan2.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond2,
	})
	require.NoError(t, err)

	err = fix.svc.FinalizeReceiving(ctx, retID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, 2, item.Quantity)
	assert.Equal(t, "partial_restock", item.PhysicalOutcome)
	assert.Equal(t, 1, item.RestockedQuantity)
	assert.Equal(t, 1, item.DamagedQuantity)
	assert.Equal(t, 0, item.RejectedQuantity)
	assert.Equal(t, 0, item.NotReceivedQuantity)
	assert.True(t, item.Restock) // at least one unit restocked

	require.Len(t, item.Units, 2)
	assert.Equal(t, unitCodes[0], item.Units[0].UnitCode)
	assert.Equal(t, "restock", *item.Units[0].Disposition)
	assert.Equal(t, unitCodes[1], item.Units[1].UnitCode)
	assert.Equal(t, "damaged", *item.Units[1].Disposition)
}

// 5. Cross-seller isolation: Seller B cannot access Seller A's returns
func TestSellerReturn_CrossSellerIsolation(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Cross seller test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Seller A sees the item
	itemsA, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	assert.Len(t, itemsA, 1)

	// Seller B tries to see the return by ID -> returns empty
	itemsB, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerBID, retID)
	require.NoError(t, err)
	assert.Empty(t, itemsB, "Seller B must NOT see Seller A's return items")

	// Seller B list -> empty
	listB, err := fix.returnsRepo.GetSellerReturnItems(ctx, fix.sellerBID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, listB, "Seller B list must NOT contain Seller A's return items")
}

// 6. PII and internal admin comment redaction in Seller DTO serialization
func TestSellerReturn_PIIAndAdminCommentRedacted(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Redaction test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Insert secret internal support comment in database
	secretComment := "SECRET_INTERNAL_SUPPORT_NOTE_12345"
	_, err = fix.client.Pool.Exec(ctx, `UPDATE returns SET admin_comment = $1 WHERE id = $2`, secretComment, retID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Serialize item to JSON as returned to Seller API clients
	rawJSON, err := json.Marshal(items[0])
	require.NoError(t, err)
	jsonStr := string(rawJSON)

	// Verify secret comment is completely absent
	assert.NotContains(t, jsonStr, secretComment)
	assert.NotContains(t, jsonStr, "adminComment")
	assert.NotContains(t, jsonStr, "admin_comment")

	// Verify customer PII is absent
	assert.NotContains(t, jsonStr, "customer_phone")
	assert.NotContains(t, jsonStr, "customer_email")
	assert.NotContains(t, jsonStr, "customer_name")
	assert.NotContains(t, jsonStr, "+79990001122")
	assert.NotContains(t, jsonStr, "test@example.com")
}

// 7. needs_info MUST NOT become awaiting_arrival
func TestSellerReturn_PhysicalOutcome_NeedsInfoIsNotAwaitingArrival(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Needs info test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Transition return to needs_info (support requesting information from buyer)
	_, err = fix.client.Pool.Exec(ctx, `UPDATE returns SET status = 'needs_info' WHERE id = $1`, retID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "needs_info", item.Status)
	assert.Equal(t, "needs_info", item.ProcessingStatus)
	assert.Equal(t, "needs_info", item.PhysicalOutcome, "needs_info must NOT become awaiting_arrival")
	assert.NotEqual(t, "awaiting_arrival", item.PhysicalOutcome)
	assert.False(t, item.ArrivedAtZamk)
	assert.False(t, item.InspectionCompleted)
}

// 8. Logistics lifecycle mapping: awaiting_handover, in_transit, arrived_at_zamk, receiving/in_inspection
func TestSellerReturn_PhysicalOutcome_LogisticsLifecycle(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Logistics test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Approve return
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"})
	require.NoError(t, err)

	// A. Shipment in awaiting_handover
	shipmentID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'TRK-LOG-1', 'awaiting_handover')
	`, shipmentID, retID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	assert.Equal(t, "awaiting_handover", items[0].PhysicalOutcome)
	assert.Equal(t, "awaiting_handover", items[0].ProcessingStatus)
	assert.False(t, items[0].ArrivedAtZamk)

	// B. Shipment in in_transit
	_, err = fix.client.Pool.Exec(ctx, `UPDATE return_shipments SET status = 'in_transit' WHERE id = $1`, shipmentID)
	require.NoError(t, err)

	items, err = fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	assert.Equal(t, "in_transit", items[0].PhysicalOutcome)
	assert.Equal(t, "in_transit", items[0].ProcessingStatus)
	assert.False(t, items[0].ArrivedAtZamk)

	// C. Shipment in arrived_at_zamk
	_, err = fix.client.Pool.Exec(ctx, `UPDATE return_shipments SET status = 'arrived_at_zamk' WHERE id = $1`, shipmentID)
	require.NoError(t, err)

	items, err = fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	assert.Equal(t, "arrived_at_zamk", items[0].PhysicalOutcome)
	assert.Equal(t, "arrived_at_zamk", items[0].ProcessingStatus)
	assert.True(t, items[0].ArrivedAtZamk)

	// D. Warehouse started receiving -> in_inspection
	_, err = fix.client.Pool.Exec(ctx, `UPDATE returns SET status = 'receiving', receiving_started_at = now() WHERE id = $1`, retID)
	require.NoError(t, err)

	items, err = fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	assert.Equal(t, "in_inspection", items[0].PhysicalOutcome)
	assert.Equal(t, "receiving", items[0].ProcessingStatus)
	assert.True(t, items[0].ArrivedAtZamk)
	assert.False(t, items[0].InspectionCompleted)
}

// 9. Damaged outcome does NOT execute a write-off
func TestSellerReturn_DamagedDoesNotExecuteWriteOff(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Damaged write-off test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'TRK-DMG-1', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scan, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "torn_fabric"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scan.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Verify inventory unit status is 'damaged'
	var iuStatus string
	err = fix.client.Pool.QueryRow(ctx, `SELECT status FROM inventory_units WHERE unit_code = $1`, unitCodes[0]).Scan(&iuStatus)
	require.NoError(t, err)
	assert.Equal(t, "damaged", iuStatus)

	// Verify NO stock_movements of type 'write_off' exist for this return
	var writeOffCount int
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM stock_movements
		WHERE reference_id = $1 AND (type = 'write_off' OR reason LIKE '%write_off%')
	`, retID).Scan(&writeOffCount)
	require.NoError(t, err)
	assert.Equal(t, 0, writeOffCount, "disposition=damaged must NOT execute canonical inventory write-off")

	// Verify Seller DTO shows damaged outcome, NOT write-off
	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, "damaged", items[0].PhysicalOutcome)
	assert.Equal(t, 1, items[0].DamagedQuantity)
	assert.Equal(t, 0, items[0].RestockedQuantity)
	assert.False(t, items[0].Restock)

	rawJSON, err := json.Marshal(items[0])
	require.NoError(t, err)
	jsonStr := string(rawJSON)
	assert.NotContains(t, jsonStr, "write_off")
	assert.NotContains(t, jsonStr, "written_off")
}

// 10. Database safety confirmation: tests run exclusively against zamk_test
func TestSellerReturn_DatabaseSafety(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	var dbName string
	err := fix.client.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	assert.Equal(t, "zamk_test", dbName, "Destructive/integration tests MUST run exclusively against zamk_test")
}
