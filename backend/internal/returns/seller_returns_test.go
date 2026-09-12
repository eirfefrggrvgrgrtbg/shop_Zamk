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

// ----------------------------------------------------------------------------
// SA.4.2A: Canonical Seller Return Finance Read Model Integration Tests
// ----------------------------------------------------------------------------

// Case A: Return without seller ledger adjustment -> financialAdjustment is nil (serializes to null in JSON)
func TestSellerReturn_FinancialAdjustment_CaseA_NoAdjustment(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case A test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// No seller ledger adjustment exists
	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Nil(t, items[0].FinancialAdjustment, "financialAdjustment must be nil when no adjustment entry exists")

	// Verify JSON serialization includes "financialAdjustment":null
	jsonBytes, err := json.Marshal(items[0])
	require.NoError(t, err)
	var rawMap map[string]interface{}
	err = json.Unmarshal(jsonBytes, &rawMap)
	require.NoError(t, err)
	val, exists := rawMap["financialAdjustment"]
	assert.True(t, exists, "financialAdjustment field must be present in JSON")
	assert.Nil(t, val, "financialAdjustment must serialize to null in JSON")
}

// Case B: Return with HOLD adjustment -> exact deduction amount, context = "hold", historically stable even after available_at has passed
func TestSellerReturn_FinancialAdjustment_CaseB_HoldContext_HistoricallyStable(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case B test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Create seller earning where available_at was 10 days ago (in the past relative to now)
	earningAvailableAt := time.Now().Add(-10 * 24 * time.Hour)
	earningCreatedAt := time.Now().Add(-24 * 24 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 10000, 'RUB', $5, $6)
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt, earningCreatedAt)
	require.NoError(t, err)

	// Adjustment occurred 15 days ago, which is BEFORE earningAvailableAt (10 days ago)
	// Even though now() is past available_at, the adjustment happened during HOLD!
	adjustedAt := time.Now().Add(-15 * 24 * time.Hour)
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5, $6::jsonb, $7)
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt, meta, adjustedAt)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NotNil(t, items[0].FinancialAdjustment)
	assert.Equal(t, int64(8500), items[0].FinancialAdjustment.DeductionCents)
	assert.Equal(t, "hold", items[0].FinancialAdjustment.Context, "Context must remain 'hold' historically even when now() > available_at")
	assert.Equal(t, adjustedAt.Unix(), items[0].FinancialAdjustment.AdjustedAt.Unix())
}

// Case C: Return with AVAILABLE adjustment -> exact deduction amount, context = "available"
func TestSellerReturn_FinancialAdjustment_CaseC_AvailableContext(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case C test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Earning became available 20 days ago
	earningAvailableAt := time.Now().Add(-20 * 24 * time.Hour)
	earningCreatedAt := time.Now().Add(-34 * 24 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 10000, 'RUB', $5, $6)
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt, earningCreatedAt)
	require.NoError(t, err)

	// Adjustment occurred 5 days ago (AFTER available_at)
	adjustedAt := time.Now().Add(-5 * 24 * time.Hour)
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -8500, 'RUB', $4, $5::jsonb, $6)
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, earningAvailableAt, meta, adjustedAt)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NotNil(t, items[0].FinancialAdjustment)
	assert.Equal(t, int64(8500), items[0].FinancialAdjustment.DeductionCents)
	assert.Equal(t, "available", items[0].FinancialAdjustment.Context)
}

// Case D: Return with POST_PAYOUT adjustment -> exact deduction amount, context = "post_payout", no debt claim
func TestSellerReturn_FinancialAdjustment_CaseD_PostPayoutContext_NoDebtClaim(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case D test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	adjustedAt := time.Now().Add(-2 * time.Hour)
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_post_payout"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -8500, 'RUB', $4::jsonb, $5)
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, meta, adjustedAt)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NotNil(t, items[0].FinancialAdjustment)
	assert.Equal(t, int64(8500), items[0].FinancialAdjustment.DeductionCents)
	assert.Equal(t, "post_payout", items[0].FinancialAdjustment.Context)

	// Ensure no debt claim exists in serialized JSON
	jsonBytes, err := json.Marshal(items[0])
	require.NoError(t, err)
	assert.NotContains(t, string(jsonBytes), "debt")
	assert.NotContains(t, string(jsonBytes), "deficit")
}

// Case E: Negative DB amount -> positive display magnitude in DTO
func TestSellerReturn_FinancialAdjustment_CaseE_NegativeDbToPositiveMagnitude(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case E test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Database stores -13500 cents
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -13500, 'RUB', $4::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NotNil(t, items[0].FinancialAdjustment)
	assert.Equal(t, int64(13500), items[0].FinancialAdjustment.DeductionCents, "DTO must expose positive magnitude (13500 cents for DB -13500)")
}

// Case F: Customer refund amount != Seller earning deduction -> DTO exposes seller deduction
func TestSellerReturn_FinancialAdjustment_CaseF_SellerDeductionNotCustomerRefund(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case F test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Customer paid 10000 cents (order_items.subtotal_price_cents = 10000)
	// But seller earning deduction is 8500 cents
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -8500, 'RUB', $4::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NotNil(t, items[0].FinancialAdjustment)
	assert.Equal(t, int64(8500), items[0].FinancialAdjustment.DeductionCents, "Must expose seller earning deduction (8500), not order subtotal (10000)")
	assert.NotEqual(t, items[0].SubtotalPriceCents, items[0].FinancialAdjustment.DeductionCents)
}

// Case G: Changing active seller commission does not alter return deduction calculation
func TestSellerReturn_FinancialAdjustment_CaseG_CommissionChangeDoesNotAlterRecordedDeduction(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case G test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Initial deduction recorded at 8500 cents
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -8500, 'RUB', $4::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	// Simulate platform-wide or seller commission rate update
	_, err = fix.client.Pool.Exec(ctx, `UPDATE sellers SET updated_at = now() WHERE id = $1`, fix.sellerAID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NotNil(t, items[0].FinancialAdjustment)
	assert.Equal(t, int64(8500), items[0].FinancialAdjustment.DeductionCents, "Adjustment must be read directly from immutable ledger entry")
}

// Case H: Deterministic authoritative adjustment selection (excludes batch_race_offset)
func TestSellerReturn_FinancialAdjustment_CaseH_DeterministicSelectionExcludesOffset(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case H test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Older authoritative deduction: 8500 cents, created 2 hours ago
	metaOld := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -8500, 'RUB', $4::jsonb, now() - interval '2 hour')
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, metaOld)
	require.NoError(t, err)

	// Newer non-authoritative offset entry: 5000 cents, reason 'batch_race_offset', created 1 hour ago
	metaOffset := fmt.Sprintf(`{"return_id": "%s", "reason": "batch_race_offset"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -5000, 'RUB', $4::jsonb, now() - interval '1 hour')
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, metaOffset)
	require.NoError(t, err)

	// Authoritative adjustment: 8500 cents (ignores batch_race_offset even though it is newer)
	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NotNil(t, items[0].FinancialAdjustment)
	assert.Equal(t, int64(8500), items[0].FinancialAdjustment.DeductionCents, "Must select authoritative return_deduction, ignoring batch_race_offset")
}

// Case I: Cross-seller isolation
func TestSellerReturn_FinancialAdjustment_CaseI_CrossSellerIsolation(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case I test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -8500, 'RUB', $4::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	// Seller B queries for this return ID -> empty result
	itemsB, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerBID, retID)
	require.NoError(t, err)
	assert.Empty(t, itemsB, "Seller B must NOT see Seller A's return items or financial adjustments")

	// Seller B queries list -> empty
	listB, err := fix.returnsRepo.GetSellerReturnItems(ctx, fix.sellerBID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, listB)
}

// Case J: Seeded/legacy refunded return without seller ledger -> financialAdjustment is nil
func TestSellerReturn_FinancialAdjustment_CaseJ_SeededRefundWithoutLedger(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case J test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Mark return as completed / refunded in DB without any seller_ledger_entries adjustment
	_, err = fix.client.Pool.Exec(ctx, `UPDATE returns SET status = 'completed', completed_at = now() WHERE id = $1`, retID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, "completed", items[0].Status)
	assert.Nil(t, items[0].FinancialAdjustment, "Seeded/legacy return without ledger adjustment must have financialAdjustment == nil (never fake 0 ₽)")
}

// ----------------------------------------------------------------------------
// SA.5.2A: Canonical Seller Return Finance Explanation Contract Integration Tests
// ----------------------------------------------------------------------------

func createCustomDeliveredOrder(t *testing.T, fix *m51Fixture, deliveredAt time.Time, priceCents int64, qty int) testOrder {
	t.Helper()
	ctx := context.Background()

	orderID := uuid.New()
	fID := uuid.New()
	shipmentID := uuid.New()
	oiID := uuid.New()

	orderNum := fmt.Sprintf("ORD-%s", uuid.New().String()[:12])
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', $4, 'RUB', 'Test Address', 'Courier', 0, 'Test User', 'test@example.com', '+79990001122', now(), now())
	`, orderID, fix.userID, orderNum, priceCents*int64(qty))
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Product Custom', 'slug-custom', $7, $8, $9)
	`, oiID, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID, priceCents, priceCents*int64(qty), qty)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', $4, $5)
	`, shipmentID, orderID, fID, deliveredAt.Add(-24*time.Hour), deliveredAt)
	require.NoError(t, err)

	return testOrder{
		orderID:       orderID,
		fulfillmentID: fID,
		shipmentID:    shipmentID,
		orderItemID:   oiID,
	}
}

// Case A: Full quantity (equivalent to ORD-100202)
// gross = 1299000, commission = 116910, earning = 1182090, deduction = 1182090
func TestSellerReturn_ExplanationFacts_CaseA_FullQuantity(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(1299000)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case A explanation test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Historical seller earning = 1182090, commission = -116910
	earningAvailableAt := time.Now().Add(14 * 24 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 1182090, 'RUB', $5, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, created_at)
		VALUES ($1, $2, $3, $4, 'zamk_commission', -116910, 'RUB', $5, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt)
	require.NoError(t, err)

	// Authoritative return deduction
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -1182090, 'RUB', $5, $6::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, int64(1182090), adj.DeductionCents)
	assert.Equal(t, int64(1299000), adj.GrossCents)
	assert.Equal(t, int64(116910), adj.CommissionCents)
	assert.Equal(t, int64(1182090), adj.SellerEarningCents)
	assert.Equal(t, "hold", adj.Context)

	// Economic invariants
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents, "Gross = Commission + SellerEarning")
	assert.Equal(t, adj.DeductionCents, adj.SellerEarningCents, "SellerEarning = Deduction")
}

// Case B: No financial adjustment -> financialAdjustment = null, no synthetic explanation
func TestSellerReturn_ExplanationFacts_CaseB_NoFinancialAdjustment(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case B test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Nil(t, items[0].FinancialAdjustment, "financialAdjustment must be nil when no adjustment entry exists")

	jsonBytes, err := json.Marshal(items[0])
	require.NoError(t, err)
	var rawMap map[string]interface{}
	err = json.Unmarshal(jsonBytes, &rawMap)
	require.NoError(t, err)
	val, exists := rawMap["financialAdjustment"]
	assert.True(t, exists)
	assert.Nil(t, val)
}

// Case C: Customer refund differs from Seller deduction -> explanation uses Seller sale economics
func TestSellerReturn_ExplanationFacts_CaseC_CustomerRefundDiffersFromSellerDeduction(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(1000000) // 10 000 RUB
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case C refund diff"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Customer gets 1000000 full refund, but seller was deducted 850000 (earning minus commission)
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -850000, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, int64(850000), adj.DeductionCents)
	assert.Equal(t, int64(1000000), adj.GrossCents)
	assert.Equal(t, int64(150000), adj.CommissionCents)
	assert.Equal(t, int64(850000), adj.SellerEarningCents)
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)
}

// Case D: Current commission rule changed after sale -> explanation remains historical
func TestSellerReturn_ExplanationFacts_CaseD_CommissionRuleChangedAfterSale(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(100000) // 1 000 RUB
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case D rule change"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Historical sale at 9% commission -> earning 91000, deduction 91000
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -91000, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	// Simulate subsequent seller commission rate or profile update
	_, err = fix.client.Pool.Exec(ctx, `UPDATE sellers SET updated_at = now() WHERE id = $1`, fix.sellerAID)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, int64(91000), adj.DeductionCents)
	assert.Equal(t, int64(100000), adj.GrossCents)
	assert.Equal(t, int64(9000), adj.CommissionCents)
	assert.Equal(t, int64(91000), adj.SellerEarningCents)
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)
}

// Case E: Hold context -> facts correct
func TestSellerReturn_ExplanationFacts_CaseE_HoldContext(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(200000)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case E hold"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	earningAvailableAt := time.Now().Add(10 * 24 * time.Hour)
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -180000, 'RUB', $5, $6::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, "hold", adj.Context)
	assert.Equal(t, int64(180000), adj.DeductionCents)
	assert.Equal(t, int64(200000), adj.GrossCents)
	assert.Equal(t, int64(20000), adj.CommissionCents)
	assert.Equal(t, int64(180000), adj.SellerEarningCents)
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)
}

// Case F: Available context -> facts correct
func TestSellerReturn_ExplanationFacts_CaseF_AvailableContext(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(200000)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case F available"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	earningAvailableAt := time.Now().Add(-10 * 24 * time.Hour) // in past
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -180000, 'RUB', $5, $6::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, earningAvailableAt, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, "available", adj.Context)
	assert.Equal(t, int64(180000), adj.DeductionCents)
	assert.Equal(t, int64(200000), adj.GrossCents)
	assert.Equal(t, int64(20000), adj.CommissionCents)
	assert.Equal(t, int64(180000), adj.SellerEarningCents)
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)
}

// Case G: Post payout context -> facts correct
func TestSellerReturn_ExplanationFacts_CaseG_PostPayoutContext(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(200000)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case G post payout"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_post_payout"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -180000, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, "post_payout", adj.Context)
	assert.Equal(t, int64(180000), adj.DeductionCents)
	assert.Equal(t, int64(200000), adj.GrossCents)
	assert.Equal(t, int64(20000), adj.CommissionCents)
	assert.Equal(t, int64(180000), adj.SellerEarningCents)
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)

	jsonBytes, err := json.Marshal(items[0])
	require.NoError(t, err)
	assert.NotContains(t, string(jsonBytes), "debt")
	assert.NotContains(t, string(jsonBytes), "deficit")
}

// Case H: Quantity > 1 clean division
// Original qty = 2, unit price = 500000 (total gross 1000000), original earning = 800000
// Returned qty = 1 -> deduction = 400000, gross = 500000, commission = 100000
func TestSellerReturn_ExplanationFacts_CaseH_QuantityGreaterThan1_CleanDivision(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(500000)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 2)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case H clean division"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Earning was 800000 for 2 items (400000 per unit). Return 1 item -> deduction is 400000
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -400000, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, int64(400000), adj.DeductionCents)
	assert.Equal(t, int64(500000), adj.GrossCents, "Gross must be price * return_quantity (500000 * 1)")
	assert.Equal(t, int64(100000), adj.CommissionCents, "Commission must explain remaining economics (500000 - 400000)")
	assert.Equal(t, int64(400000), adj.SellerEarningCents)
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)
	assert.Equal(t, adj.DeductionCents, adj.SellerEarningCents)
}

// Case I: Quantity > 1 non-even division
// Original qty = 3, price = 333333, original earning = 700000
// Returned qty = 1 -> deduction = (700000 / 3) * 1 = 233333
// gross = 333333 * 1 = 333333, earning = 233333, commission = 100000
// All explanation values are exact, positive, and internally consistent with zero kopeck discrepancy.
func TestSellerReturn_ExplanationFacts_CaseI_QuantityGreaterThan1_NonEvenDivision(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(333333)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 3)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 3)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case I non-even"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Exact deduction computed by ProcessReturnDeduction algorithm: (700000 / 3) * 1 = 233333
	deductionCents := int64(233333)
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', $5, 'RUB', $6::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, -deductionCents, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	adj := items[0].FinancialAdjustment
	require.NotNil(t, adj)
	assert.Equal(t, int64(233333), adj.DeductionCents)
	assert.Equal(t, int64(333333), adj.GrossCents)
	assert.Equal(t, int64(233333), adj.SellerEarningCents)
	assert.Equal(t, int64(100000), adj.CommissionCents)

	// Zero kopeck discrepancy invariant
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)
	assert.Equal(t, adj.DeductionCents, adj.SellerEarningCents)
}

// Case J: Multi-return / repeated partial-return behavior
// Original qty = 3, price = 333333, original earning = 700000
// 3 separate returns of qty = 1 each:
// Deduction per return = (700000 / 3) * 1 = 233333
// Total deductions = 233333 * 3 = 699999 <= 700000 (No over-deduction! 1 kopeck remainder stays with seller)
func TestSellerReturn_ExplanationFacts_CaseJ_MultiReturn_RemainderSemantics(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(333333)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 3)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 3)

	originalEarning := int64(700000)
	expectedDeductions := []int64{233333, 233333, 233334}
	var totalGross, totalCommission, totalDeductions int64

	for i := 1; i <= 3; i++ {
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "size_mismatch",
			Comment: func() *string { s := fmt.Sprintf("Return %d", i); return &s }(),
			Items: []returns.CreateReturnItemRequest{
				{OrderItemID: tOrd.orderItemID, Quantity: 1},
			},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		deductionCents := expectedDeductions[i-1]
		totalDeductions += deductionCents

		meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction", "quantity": 1}`, retID)
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'adjustment', $5, 'RUB', $6::jsonb, now())
		`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, -deductionCents, meta)
		require.NoError(t, err)

		items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
		require.NoError(t, err)
		require.Len(t, items, 1)

		adj := items[0].FinancialAdjustment
		require.NotNil(t, adj)
		assert.Equal(t, deductionCents, adj.DeductionCents)
		assert.Equal(t, priceCents, adj.GrossCents)
		assert.Equal(t, deductionCents, adj.SellerEarningCents)
		assert.Equal(t, priceCents-deductionCents, adj.CommissionCents)

		// Per-return invariants
		assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents)
		assert.Equal(t, adj.DeductionCents, adj.SellerEarningCents)

		totalGross += adj.GrossCents
		totalCommission += adj.CommissionCents
	}

	// Cumulative exactness: across all 3 returns, total deductions must equal original earning exactly
	assert.Equal(t, int64(700000), totalDeductions, "Cumulative deduction across all partial returns must equal original earning exactly")
	assert.Equal(t, originalEarning, totalDeductions)
	assert.Equal(t, priceCents*3, totalGross, "Cumulative gross must equal original order item gross")
	assert.Equal(t, (priceCents*3)-originalEarning, totalCommission, "Cumulative commission must equal original commission")
}

// Case K: Seller isolation unchanged
func TestSellerReturn_ExplanationFacts_CaseK_SellerIsolation(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	priceCents := int64(1299000)
	tOrd := createCustomDeliveredOrder(t, fix, time.Now().Add(-1*time.Hour), priceCents, 1)
	createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Case K isolation"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -1182090, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	// Seller A gets return items with financial facts
	itemsA, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, itemsA, 1)
	require.NotNil(t, itemsA[0].FinancialAdjustment)

	// Seller B queries -> empty result
	itemsB, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerBID, retID)
	require.NoError(t, err)
	assert.Empty(t, itemsB)

	// Seller B queries list -> empty
	listB, err := fix.returnsRepo.GetSellerReturnItems(ctx, fix.sellerBID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, listB)
}

// ----------------------------------------------------------------------------
// SA.5.3B: Canonical Seller Return Finance Read-Model Truth Integration Tests
// ----------------------------------------------------------------------------

// Scenario A: Ordinary return (restocked)
// - physical = restocked
// - Sale Reversal formed
// - compensation == nil
func TestSellerReturnFinanceTruth_ScenarioA_OrdinaryRestocked(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Ordinary restocked test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-SCEN-A', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "as_new"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "restock",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Form Sale Reversal
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "restocked", item.PhysicalOutcome)
	assert.NotNil(t, item.FinancialAdjustment, "Sale Reversal must be formed")
	assert.Nil(t, item.Compensation, "Compensation must be nil for restocked ordinary return")

	// Verify JSON serialization omits or sets null for compensation
	rawBytes, err := json.Marshal(item)
	require.NoError(t, err)
	var rawMap map[string]interface{}
	require.NoError(t, json.Unmarshal(rawBytes, &rawMap))
	val, exists := rawMap["compensation"]
	if exists {
		assert.Nil(t, val, "compensation field in JSON must be null when absent")
	}
}

// Scenario B: Damaged return, pending responsibility
// - physical = damaged
// - Sale Reversal formed
// - responsibility pending
// - compensation.status == 'pending'
// - responsibleParty == nil, reasonCode == nil
func TestSellerReturnFinanceTruth_ScenarioB_DamagedPendingResponsibility(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: func() *string { s := "Pending responsibility test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-SCEN-B', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "damaged_zipper"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Form Sale Reversal
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "damaged", item.PhysicalOutcome)
	assert.NotNil(t, item.FinancialAdjustment, "Sale Reversal must be formed")
	require.NotNil(t, item.Compensation, "Compensation must be non-nil for damaged return awaiting resolution")
	assert.Equal(t, "pending", item.Compensation.Status)
	assert.Nil(t, item.Compensation.ResponsibleParty)
	assert.Nil(t, item.Compensation.ReasonCode)
}

// Scenario C: Damaged return, ZAMK warehouse damage
// - physical = damaged
// - Sale Reversal formed
// - responsibility = zamk, zamk_warehouse_damage
// - compensation.status == 'credited'
// - responsibleParty == 'zamk', reasonCode == 'zamk_warehouse_damage'
func TestSellerReturnFinanceTruth_ScenarioC_DamagedZamkWarehouseDamage(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: func() *string { s := "ZAMK damage test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-SCEN-C', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "warehouse_stain"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Resolve responsibility to ZAMK warehouse damage
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now().UTC()
	party := returns.ReturnResponsiblePartyZamk
	reason := returns.ReturnResponsibilityReasonZamkWarehouseDamage
	source := returns.ReturnResponsibilityDecisionSourceEmployee
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      &party,
		ReasonCode:            &reason,
		DecisionSource:        &source,
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)

	// Form Sale Reversal
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "damaged", item.PhysicalOutcome)
	assert.NotNil(t, item.FinancialAdjustment)
	require.NotNil(t, item.Compensation)
	assert.Equal(t, "credited", item.Compensation.Status)
	require.NotNil(t, item.Compensation.ResponsibleParty)
	assert.Equal(t, "zamk", *item.Compensation.ResponsibleParty)
	require.NotNil(t, item.Compensation.ReasonCode)
	assert.Equal(t, "zamk_warehouse_damage", *item.Compensation.ReasonCode)
}

// Scenario D: Damaged return, carrier damage
// - physical = damaged
// - Sale Reversal formed
// - responsibility = carrier, carrier_damage
// - compensation.status == 'credited'
// - responsibleParty == 'carrier', reasonCode == 'carrier_damage'
func TestSellerReturnFinanceTruth_ScenarioD_DamagedCarrierDamage(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: func() *string { s := "Carrier damage test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-SCEN-D', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "crushed_box"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Resolve responsibility to carrier damage
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now().UTC()
	party := returns.ReturnResponsiblePartyCarrier
	reason := returns.ReturnResponsibilityReasonCarrierDamage
	source := returns.ReturnResponsibilityDecisionSourceEmployee
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      &party,
		ReasonCode:            &reason,
		DecisionSource:        &source,
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)

	// Form Sale Reversal
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "damaged", item.PhysicalOutcome)
	assert.NotNil(t, item.FinancialAdjustment)
	require.NotNil(t, item.Compensation)
	assert.Equal(t, "credited", item.Compensation.Status)
	require.NotNil(t, item.Compensation.ResponsibleParty)
	assert.Equal(t, "carrier", *item.Compensation.ResponsibleParty)
	require.NotNil(t, item.Compensation.ReasonCode)
	assert.Equal(t, "carrier_damage", *item.Compensation.ReasonCode)
}

// Scenario E: Damaged return, seller product defect
// - physical = damaged
// - Sale Reversal formed
// - responsibility = seller, seller_product_defect
// - compensation == nil
func TestSellerReturnFinanceTruth_ScenarioE_DamagedSellerDefect(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: func() *string { s := "Seller defect test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-SCEN-E', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "factory_defect"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Resolve responsibility to seller defect
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now().UTC()
	party := returns.ReturnResponsiblePartySeller
	reason := returns.ReturnResponsibilityReasonSellerProductDefect
	source := returns.ReturnResponsibilityDecisionSourceEmployee
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      &party,
		ReasonCode:            &reason,
		DecisionSource:        &source,
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)

	// Form Sale Reversal
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "damaged", item.PhysicalOutcome)
	assert.NotNil(t, item.FinancialAdjustment)
	assert.Nil(t, item.Compensation, "Seller defect must NOT receive ZAMK compensation")
}

// Scenario F: Restocked fulfillment error (wrong item)
// - physical = restocked
// - Sale Reversal formed
// - responsibility = zamk, zamk_fulfillment_error
// - compensation == nil (not damaged!)
func TestSellerReturnFinanceTruth_ScenarioF_RestockedFulfillmentError(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "wrong_item",
		Comment: func() *string { s := "Fulfillment error restocked test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-SCEN-F', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "good_condition"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "restock",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Resolve responsibility to ZAMK fulfillment error
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now().UTC()
	party := returns.ReturnResponsiblePartyZamk
	reason := returns.ReturnResponsibilityReasonZamkFulfillmentError
	source := returns.ReturnResponsibilityDecisionSourceEmployee
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      &party,
		ReasonCode:            &reason,
		DecisionSource:        &source,
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)

	// Form Sale Reversal
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "restocked", item.PhysicalOutcome)
	assert.NotNil(t, item.FinancialAdjustment)
	assert.Nil(t, item.Compensation, "Restocked return must not receive compensation even if ZAMK caused the return")
}

// Scenario G: Responsibility correction (reversal of compensation)
// - damaged return begins with zamk responsibility -> status == 'credited'
// - responsibility corrected to seller -> status becomes nil / compensation == nil
func TestSellerReturnFinanceTruth_ScenarioG_ResponsibilityCorrection(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: func() *string { s := "Correction test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-SCEN-G', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "damaged_condition"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "damaged",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Form Sale Reversal
	meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction"}`, retID)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
	require.NoError(t, err)

	// Step 1: Initial resolution = ZAMK warehouse damage
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now().UTC()
	partyZamk := returns.ReturnResponsiblePartyZamk
	reasonZamk := returns.ReturnResponsibilityReasonZamkWarehouseDamage
	source := returns.ReturnResponsibilityDecisionSourceEmployee
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      &partyZamk,
		ReasonCode:            &reasonZamk,
		DecisionSource:        &source,
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)

	// Verify status is credited
	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.NotNil(t, items[0].Compensation)
	assert.Equal(t, "credited", items[0].Compensation.Status)

	// Step 2: Correct resolution to seller product defect
	partySeller := returns.ReturnResponsiblePartySeller
	reasonSeller := returns.ReturnResponsibilityReasonSellerProductDefect
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      &partySeller,
		ReasonCode:            &reasonSeller,
		DecisionSource:        &source,
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)

	// Verify status becomes nil
	itemsCorrected, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	assert.Nil(t, itemsCorrected[0].Compensation, "Compensation must become nil after correction to seller responsibility")
}

// Scenario H: Multiple partial returns on same order_item (Anti-misattribution proof)
// - order_item qty = 3
// - Return A: qty = 1, restocked -> compensation == nil
// - Return B: qty = 1, damaged, zamk responsibility -> status == 'credited'
// - Return C: qty = 1, damaged, zamk responsibility -> status == 'credited'
// - PROVE: neither Return B nor Return C exposes a compensation amount cents
// - PROVE: neither Return B nor Return C exposes a net cents
// - PROVE: both show status == 'credited'
func TestSellerReturnFinanceTruth_ScenarioH_MultiPartialReturn_AntiMisattribution(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	unitCodes, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 3)

	processReturn := func(unitIdx int, disp string, party *string, reason *string) uuid.UUID {
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "size_mismatch",
			Comment: func() *string { s := fmt.Sprintf("Return for unit %d", unitIdx); return &s }(),
			Items: []returns.CreateReturnItemRequest{
				{OrderItemID: tOrd.orderItemID, Quantity: 1},
			},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
			VALUES ($1, $2, 'cdek', 'cdek_courier', $3, 'arrived_at_zamk')
		`, uuid.New(), retID, fmt.Sprintf("TRK-MULTI-%d", unitIdx))
		require.NoError(t, err)

		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[unitIdx]})
		require.NoError(t, err)

		cond := "inspected"
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
			Disposition:        disp,
			InspectedCondition: &cond,
		}))
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// Form Sale Reversal
		meta := fmt.Sprintf(`{"return_id": "%s", "reason": "return_deduction", "quantity": 1}`, retID)
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'adjustment', -8500, 'RUB', $5::jsonb, now())
		`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, meta)
		require.NoError(t, err)

		if party != nil && reason != nil {
			allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
			require.NoError(t, err)
			require.Len(t, allocs, 1)

			now := time.Now().UTC()
			source := returns.ReturnResponsibilityDecisionSourceEmployee
			_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
				AllocationID:          allocs[0].ID,
				Quantity:              1,
				OrderItemAllocationID: &allocIDs[unitIdx],
				Status:                returns.ReturnResponsibilityStatusResolved,
				ResponsibleParty:      party,
				ReasonCode:            reason,
				DecisionSource:        &source,
				ActorID:               &empID,
				DecidedAt:             &now,
			})
			require.NoError(t, err)
		}

		return retID
	}

	zamkParty := returns.ReturnResponsiblePartyZamk
	zamkReason := returns.ReturnResponsibilityReasonZamkWarehouseDamage

	// Return A: Restocked
	retA := processReturn(0, "restock", nil, nil)

	// Return B: Damaged, ZAMK responsibility
	retB := processReturn(1, "damaged", &zamkParty, &zamkReason)

	// Return C: Damaged, ZAMK responsibility
	retC := processReturn(2, "damaged", &zamkParty, &zamkReason)

	// Verify Return A
	itemsA, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retA)
	require.NoError(t, err)
	require.Len(t, itemsA, 1)
	assert.Equal(t, "restocked", itemsA[0].PhysicalOutcome)
	assert.Nil(t, itemsA[0].Compensation, "Return A (restocked) must NOT have compensation")

	// Verify Return B
	itemsB, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retB)
	require.NoError(t, err)
	require.Len(t, itemsB, 1)
	assert.Equal(t, "damaged", itemsB[0].PhysicalOutcome)
	require.NotNil(t, itemsB[0].Compensation)
	assert.Equal(t, "credited", itemsB[0].Compensation.Status)
	assert.Equal(t, "zamk", *itemsB[0].Compensation.ResponsibleParty)
	assert.Equal(t, "zamk_warehouse_damage", *itemsB[0].Compensation.ReasonCode)

	// Verify Return C
	itemsC, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retC)
	require.NoError(t, err)
	require.Len(t, itemsC, 1)
	assert.Equal(t, "damaged", itemsC[0].PhysicalOutcome)
	require.NotNil(t, itemsC[0].Compensation)
	assert.Equal(t, "credited", itemsC[0].Compensation.Status)
	assert.Equal(t, "zamk", *itemsC[0].Compensation.ResponsibleParty)
	assert.Equal(t, "zamk_warehouse_damage", *itemsC[0].Compensation.ReasonCode)

	// PROVE: Anti-misattribution & No invented money cents in JSON for Return B and Return C
	for _, itm := range []returns.SellerReturnItem{itemsB[0], itemsC[0]} {
		jsonBytes, err := json.Marshal(itm)
		require.NoError(t, err)
		var rawMap map[string]interface{}
		require.NoError(t, json.Unmarshal(jsonBytes, &rawMap))

		// Check compensation structure
		compMap, ok := rawMap["compensation"].(map[string]interface{})
		require.True(t, ok, "compensation must be a JSON object")
		assert.Equal(t, "credited", compMap["status"])

		// PROVE: NO compensation amount cents anywhere in compensation object
		_, hasCompCents := compMap["amountCents"]
		assert.False(t, hasCompCents, "compensation object must NOT expose amountCents")
		_, hasCompCentsSnake := compMap["amount_cents"]
		assert.False(t, hasCompCentsSnake, "compensation object must NOT expose amount_cents")
		_, hasCompCentsNamed := compMap["compensationCents"]
		assert.False(t, hasCompCentsNamed, "compensation object must NOT expose compensationCents")

		// PROVE: NO net cents in item root
		_, hasNetCents := rawMap["netCents"]
		assert.False(t, hasNetCents, "item must NOT expose netCents")
		_, hasNetCentsSnake := rawMap["net_cents"]
		assert.False(t, hasNetCentsSnake, "item must NOT expose net_cents")
	}
}

// ----------------------------------------------------------------------------
// SA.5.3B.1: Mandatory Integration Tests
// ----------------------------------------------------------------------------

// 1. TestSellerReturnFinanceTruth_RealReversalFlow_QuantityKey
// Creates return, inspects, calls SimulateRefundSuccess, proves the resulting ledger adjustment has "quantity",
// and DTO reads GrossCents and reversed_quantity correctly without manual metadata.
func TestSellerReturnFinanceTruth_RealReversalFlow_QuantityKey(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)

	// Create seller earning (amount 850 cents for unit price 1000)
	earningID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 850, 'RUB', now() + interval '14 days', '{}', now())
	`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_mismatch",
		Comment: func() *string { s := "Real reversal flow test"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
		VALUES ($1, $2, 'cdek', 'cdek_courier', 'TRK-REAL-REV-1', 'arrived_at_zamk')
	`, uuid.New(), retID)
	require.NoError(t, err)

	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
	require.NoError(t, err)

	cond := "as_new"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition:        "restock",
		InspectedCondition: &cond,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Create refund quote & pending refund
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NotNil(t, ref)

	// Execute SimulateRefundSuccess -> canonical ProcessReturnDeduction writes adjustment
	succRef, err := fix.svc.SimulateRefundSuccess(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, succRef)
	assert.Equal(t, "succeeded", succRef.Status)

	// PROVE: Resulting ledger adjustment has metadata key "quantity" and NOT "reversed_quantity"
	var metaBytes []byte
	var adjAmount int64
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT metadata, amount_cents
		FROM seller_ledger_entries
		WHERE order_item_id = $1 AND type = 'adjustment'
	`, tOrd.orderItemID).Scan(&metaBytes, &adjAmount)
	require.NoError(t, err)
	assert.Equal(t, int64(-850), adjAmount)

	var metaMap map[string]interface{}
	err = json.Unmarshal(metaBytes, &metaMap)
	require.NoError(t, err)
	assert.Equal(t, float64(1), metaMap["quantity"], "Production metadata key MUST be 'quantity'")
	_, hasReversedQty := metaMap["reversed_quantity"]
	assert.False(t, hasReversedQty, "Production metadata MUST NOT contain 'reversed_quantity'")

	// PROVE: DTO reads GrossCents correctly without manual metadata manipulation
	items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	require.NotNil(t, item.FinancialAdjustment)
	adj := item.FinancialAdjustment
	assert.Equal(t, int64(850), adj.DeductionCents)
	assert.Equal(t, "hold", adj.Context)
	assert.Equal(t, int64(1000), adj.GrossCents, "GrossCents must equal PriceCents * reversed_quantity")
	assert.Equal(t, int64(850), adj.SellerEarningCents)
	assert.Equal(t, int64(150), adj.CommissionCents)

	// Strict invariants:
	assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents, "Invariant: gross == commission + earning")
	assert.Equal(t, adj.SellerEarningCents, adj.DeductionCents, "Invariant: sellerEarning == deductionCents")
}

// 2. TestSellerReturnFinanceTruth_ContextTruth_HoldAvailablePostPayout
// Real tests for Hold (available_at in future), Available (available_at in past),
// and Post-Payout (payout_batch_id != nil -> return_post_payout).
func TestSellerReturnFinanceTruth_ContextTruth_HoldAvailablePostPayout(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	runContextTest := func(testName string, availableAt *time.Time, payoutBatchID *uuid.UUID, expectedContext string) {
		t.Run(testName, func(t *testing.T) {
			tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
			unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
			createSucceededPayment(t, fix, tOrd.orderID, 1000)

			_, err := fix.client.Pool.Exec(ctx, `
				INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, payout_batch_id, metadata, created_at)
				VALUES ($1, $2, $3, $4, 'seller_earning', 850, 'RUB', $5, $6, '{}', now())
			`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID, availableAt, payoutBatchID)
			require.NoError(t, err)

			resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
				Reason:  "size_mismatch",
				Comment: func() *string { s := "Context truth test: " + testName; return &s }(),
				Items: []returns.CreateReturnItemRequest{
					{OrderItemID: tOrd.orderItemID, Quantity: 1},
				},
			})
			require.NoError(t, err)
			retID := resp[0].Return.ID

			require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
				VALUES ($1, $2, 'cdek', 'cdek_courier', $3, 'arrived_at_zamk')
			`, uuid.New(), retID, "TRK-CTX-"+uuid.New().String()[:8])
			require.NoError(t, err)

			require.NoError(t, fix.svc.StartReceiving(ctx, retID))
			scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[0]})
			require.NoError(t, err)

			cond := "as_new"
			require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
				Disposition:        "restock",
				InspectedCondition: &cond,
			}))
			require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

			_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
			require.NoError(t, err)

			succRef, err := fix.svc.SimulateRefundSuccess(ctx, retID)
			require.NoError(t, err)
			require.NotNil(t, succRef)

			items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
			require.NoError(t, err)
			require.Len(t, items, 1)

			item := items[0]
			require.NotNil(t, item.FinancialAdjustment)
			assert.Equal(t, expectedContext, item.FinancialAdjustment.Context, "Context MUST match canonical enum value")
			assert.Contains(t, []string{"hold", "available", "post_payout"}, item.FinancialAdjustment.Context)

			if expectedContext == "post_payout" {
				rawJSON, err := json.Marshal(item)
				require.NoError(t, err)
				assert.NotContains(t, string(rawJSON), "debt")
				assert.NotContains(t, string(rawJSON), "deficit")
			}
		})
	}

	// 1. Hold: available_at in future
	future := time.Now().Add(14 * 24 * time.Hour)
	runContextTest("Hold_FutureAvailableAt", &future, nil, "hold")

	// 2. Available: available_at in past
	past := time.Now().Add(-5 * 24 * time.Hour)
	runContextTest("Available_PastAvailableAt", &past, nil, "available")

	// 3. Post-Payout: payout_batch_id != nil
	batchID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'paid', now(), now(), now(), now())
	`, batchID, fix.sellerAID, 850)
	require.NoError(t, err)
	runContextTest("PostPayout_BatchIDPresent", &past, &batchID, "post_payout")
}

// 3. TestSellerReturnFinanceTruth_CumulativeRounding_Q3_E700000
// Historical Q=3, E=700000, 3 partial returns processed via SimulateRefundSuccess,
// asserting reversals 233333, 233333, 233334, and verifying sellerEarningCents == deductionCents
// (especially 3rd return = 233334) and grossCents == commissionCents + sellerEarningCents.
func TestSellerReturnFinanceTruth_CumulativeRounding_Q3_E700000(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Historical order: Q=3, unit price = 300000, total price = 900000
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	unitCodes, _ := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 3)

	_, err := fix.client.Pool.Exec(ctx, `UPDATE order_items SET price_cents = 300000, subtotal_price_cents = 900000 WHERE id = $1`, tOrd.orderItemID)
	require.NoError(t, err)
	_, err = fix.client.Pool.Exec(ctx, `UPDATE orders SET total_price_cents = 900000 WHERE id = $1`, tOrd.orderID)
	require.NoError(t, err)

	createSucceededPayment(t, fix, tOrd.orderID, 900000)

	// Historical total seller earning E = 700000
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 700000, 'RUB', now() + interval '14 days', '{}', now())
	`, uuid.New(), fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	expectedDeductions := []int64{233333, 233333, 233334}
	var totalDeductions int64
	var totalSellerEarnings int64
	var totalCommissions int64
	var totalGross int64

	for i := 0; i < 3; i++ {
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "size_mismatch",
			Comment: func() *string { s := fmt.Sprintf("Partial return %d for unit %d", i+1, i); return &s }(),
			Items: []returns.CreateReturnItemRequest{
				{OrderItemID: tOrd.orderItemID, Quantity: 1},
			},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO return_shipments (id, return_id, provider, method, tracking_number, status)
			VALUES ($1, $2, 'cdek', 'cdek_courier', $3, 'arrived_at_zamk')
		`, uuid.New(), retID, fmt.Sprintf("TRK-CUMUL-%d", i))
		require.NoError(t, err)

		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unitCodes[i]})
		require.NoError(t, err)

		cond := "as_new"
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanResp.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
			Disposition:        "restock",
			InspectedCondition: &cond,
		}))
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		_, err = fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)

		succRef, err := fix.svc.SimulateRefundSuccess(ctx, retID)
		require.NoError(t, err)
		require.NotNil(t, succRef)

		items, err := fix.returnsRepo.GetSellerReturnItemsForReturn(ctx, fix.sellerAID, retID)
		require.NoError(t, err)
		require.Len(t, items, 1)

		item := items[0]
		require.NotNil(t, item.FinancialAdjustment, "Return %d must have financial adjustment", i+1)

		expectedDeduction := expectedDeductions[i]
		adj := item.FinancialAdjustment

		// 1. Verify exact deduction amount matches cumulative rounding (233333, 233333, 233334)
		assert.Equal(t, expectedDeduction, adj.DeductionCents, "Return %d deduction amount must match cumulative sequence", i+1)

		// 2. Verify sellerEarningCents == deductionCents (especially 3rd return = 233334)
		assert.Equal(t, expectedDeduction, adj.SellerEarningCents, "Return %d sellerEarningCents must equal exact deduction", i+1)
		assert.Equal(t, adj.DeductionCents, adj.SellerEarningCents, "Return %d invariant: sellerEarningCents == deductionCents", i+1)

		// 3. Verify grossCents == commissionCents + sellerEarningCents
		assert.Equal(t, int64(300000), adj.GrossCents, "Return %d grossCents must equal unit price * 1", i+1)
		assert.Equal(t, adj.GrossCents, adj.CommissionCents+adj.SellerEarningCents, "Return %d invariant: grossCents == commissionCents + sellerEarningCents", i+1)

		totalDeductions += adj.DeductionCents
		totalSellerEarnings += adj.SellerEarningCents
		totalCommissions += adj.CommissionCents
		totalGross += adj.GrossCents
	}

	// PROVE: Totals across all 3 partial returns exactly sum to original amounts
	assert.Equal(t, int64(700000), totalDeductions, "Total deductions across 3 returns must exactly equal 700000")
	assert.Equal(t, int64(700000), totalSellerEarnings, "Total seller earnings across 3 returns must exactly equal 700000")
	assert.Equal(t, int64(200000), totalCommissions, "Total commission adjustments across 3 returns must exactly equal 200000")
	assert.Equal(t, int64(900000), totalGross, "Total gross across 3 returns must exactly equal 900000")
}
