package returns_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

// 1. START RECEIVING IDEMPOTENCY & INVALID STATES MATRIX
func TestM521_StartReceiving_IdempotencyAndStateMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	retID := resp[0].Return.ID

	// A. Rejection from 'requested' state
	err = fix.svc.StartReceiving(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrInvalidStatusTransition, "Cannot start receiving from requested")

	// B. Transition to 'approved' -> StartReceiving succeeds
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	})
	require.NoError(t, err)

	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err)

	state1, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "receiving", state1.Return.Status)
	require.NotNil(t, state1.Return.ReceivingStartedAt)
	firstStartedAt := *state1.Return.ReceivingStartedAt

	// C. Call StartReceiving again -> Idempotent success, status remains receiving, receiving_started_at unchanged
	time.Sleep(5 * time.Millisecond)
	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err)

	state2, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "receiving", state2.Return.Status)
	require.NotNil(t, state2.Return.ReceivingStartedAt)
	assert.Equal(t, firstStartedAt.UnixNano(), state2.Return.ReceivingStartedAt.UnixNano(), "receiving_started_at must remain EXACTLY unchanged")

	// D. Test rejection from every other invalid state
	invalidStates := []string{"item_received", "refunded", "completed", "rejected", "cancelled"}
	for _, st := range invalidStates {
		t.Run("reject_from_"+st, func(t *testing.T) {
			testRetID := uuid.New()
			_, err := fix.client.Pool.Exec(ctx, `
				INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 'defective', now(), now())
			`, testRetID, tOrd.orderID, tOrd.fulfillmentID, fix.userID, st)
			require.NoError(t, err)

			_, err = fix.client.Pool.Exec(ctx, `
				INSERT INTO return_items (id, return_id, order_item_id, quantity, created_at)
				VALUES ($1, $2, $3, 1, now())
			`, uuid.New(), testRetID, tOrd.orderItemID)
			require.NoError(t, err)

			err = fix.svc.StartReceiving(ctx, testRetID)
			assert.ErrorIs(t, err, returns.ErrInvalidStatusTransition, "StartReceiving must reject state: "+st)
		})
	}
}

// 3. EXACT ZMU VALIDATION TEST MATRIX
func TestM521_ScanValidationMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Create valid delivered order with 1 item
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	// Inventory Item & Supply
	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 50, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-VAL-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 50, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	unitIndexCounter := 0

	// Helper to create ZMU & Allocation
	createZMUWithAlloc := func(zmuCode string, unitStatus string, pickedAt *time.Time, releasedAt *time.Time, targetOrderID uuid.UUID, targetOrderItemID uuid.UUID) uuid.UUID {
		unitIndexCounter++
		invUnitID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, invUnitID, zmuCode, fix.varAID, supplyID, supplyItemID, unitIndexCounter, unitStatus)
		require.NoError(t, err)

		resID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
			VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
		`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, targetOrderID)
		require.NoError(t, err)

		allocID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, allocID, targetOrderItemID, invUnitID, resID, pickedAt, releasedAt)
		require.NoError(t, err)

		return allocID
	}

	pickedTime := time.Now().Add(-2 * time.Hour)

	// Valid exact outbound ZMU
	validZMU := "ZMU-VAL-OK-" + uuid.New().String()[:8]
	validAllocID := createZMUWithAlloc(validZMU, "shipped", &pickedTime, nil, tOrd.orderID, tOrd.orderItemID)

	// Other order ZMU
	otherOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	otherOrderZMU := "ZMU-OTHER-ORD-" + uuid.New().String()[:8]
	_ = createZMUWithAlloc(otherOrderZMU, "shipped", &pickedTime, nil, otherOrd.orderID, otherOrd.orderItemID)

	// Same order but different fulfillment ZMU
	fID2 := uuid.New()
	oiID2 := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID2, tOrd.orderID, fix.sellerBID)
	require.NoError(t, err)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 1000, 1000, 1)
	`, oiID2, tOrd.orderID, fID2, fix.sellerBID, fix.prodBID, fix.varBID)
	require.NoError(t, err)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '2 hours', now() - interval '1 hour', now(), now())
	`, uuid.New(), tOrd.orderID, fID2)
	require.NoError(t, err)

	otherFulfillmentZMU := "ZMU-OTHER-FUL-" + uuid.New().String()[:8]
	_ = createZMUWithAlloc(otherFulfillmentZMU, "shipped", &pickedTime, nil, tOrd.orderID, oiID2)

	// Same product/variant ZMU but not allocated to this order (in warehouse stock)
	unitIndexCounter++
	unallocatedZMU := "ZMU-UNALLOC-" + uuid.New().String()[:8]
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'warehouse')
	`, uuid.New(), unallocatedZMU, fix.varAID, supplyID, supplyItemID, unitIndexCounter)
	require.NoError(t, err)

	// Unit status = warehouse / damaged / written_off (even if attached to allocation)
	warehouseStatusZMU := "ZMU-STAT-WH-" + uuid.New().String()[:8]
	_ = createZMUWithAlloc(warehouseStatusZMU, "warehouse", &pickedTime, nil, tOrd.orderID, tOrd.orderItemID)

	damagedStatusZMU := "ZMU-STAT-DMG-" + uuid.New().String()[:8]
	_ = createZMUWithAlloc(damagedStatusZMU, "damaged", &pickedTime, nil, tOrd.orderID, tOrd.orderItemID)

	writtenOffStatusZMU := "ZMU-STAT-WO-" + uuid.New().String()[:8]
	_ = createZMUWithAlloc(writtenOffStatusZMU, "written_off", &pickedTime, nil, tOrd.orderID, tOrd.orderItemID)

	// Allocation picked_at IS NULL
	unpickedZMU := "ZMU-UNPICKED-" + uuid.New().String()[:8]
	_ = createZMUWithAlloc(unpickedZMU, "shipped", nil, nil, tOrd.orderID, tOrd.orderItemID)

	// Allocation released_at IS NOT NULL
	relTime := time.Now().Add(-1 * time.Hour)
	releasedZMU := "ZMU-RELEASED-" + uuid.New().String()[:8]
	_ = createZMUWithAlloc(releasedZMU, "shipped", &pickedTime, &relTime, tOrd.orderID, tOrd.orderItemID)

	// Create return in requested status
	evIDsScan := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsScan}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// Test 1: Scan while return is NOT in receiving -> ErrReturnNotInReceiving
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: validZMU})
	assert.ErrorIs(t, err, returns.ErrReturnNotInReceiving, "Return not in receiving must be rejected")

	// Transition to receiving
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"})
	require.NoError(t, err)
	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err)

	// Test 2: Unknown ZMU -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: "ZMU-DOES-NOT-EXIST"})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "Unknown ZMU must return canonical error")

	// Test 3: ZMU allocated to another order -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: otherOrderZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "ZMU from another order must be rejected")

	// Test 4: ZMU allocated to another fulfillment -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: otherFulfillmentZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "ZMU from another fulfillment must be rejected")

	// Test 5: Same product variant ZMU not allocated to this order -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unallocatedZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "Unallocated ZMU must be rejected")

	// Test 6: Unit status = warehouse -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: warehouseStatusZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "Warehouse unit status must be rejected")

	// Test 7: Unit status = damaged -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: damagedStatusZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "Damaged unit status must be rejected")

	// Test 8: Unit status = written_off -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: writtenOffStatusZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "Written off unit status must be rejected")

	// Test 9: Allocation picked_at IS NULL -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: unpickedZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "Unpicked allocation must be rejected")

	// Test 10: Allocation released_at IS NOT NULL -> ErrInvalidZMUForReturn
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: releasedZMU})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn, "Released allocation must be rejected")

	// Test 11: Valid exact outbound ZMU -> Succeeds
	scanRes, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: validZMU})
	require.NoError(t, err)
	assert.False(t, scanRes.AlreadyScanned)
	assert.Equal(t, validAllocID, scanRes.ReturnItemUnit.OrderItemAllocationID)
}

// 4. ANOTHER RETURN OWNS ALLOCATION
func TestM521_AnotherReturnOwnsAllocation(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)

	// Inventory & Supply
	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-2RET-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-2RET-" + uuid.New().String()[:8]
	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, zmuCode, fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 2, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, tOrd.orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, now() - interval '2 hours', NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID)
	require.NoError(t, err)

	// Return A (qty 1)
	respA, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "reason A",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	retIDA := respA[0].Return.ID

	// Return B (qty 1)
	respB, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "reason B",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	retIDB := respB[0].Return.ID

	// Move both to receiving
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDA, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retIDA))

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDB, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retIDB))

	// Bind allocation to Return A
	scanA, err := fix.svc.ScanReturnUnit(ctx, retIDA, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	assert.False(t, scanA.AlreadyScanned)

	// Attempt scan through Return B -> rejected with ErrAllocationAlreadyBound
	_, err = fix.svc.ScanReturnUnit(ctx, retIDB, returns.ScanReturnUnitRequest{Code: zmuCode})
	assert.ErrorIs(t, err, returns.ErrAllocationAlreadyBound, "Should return ErrAllocationAlreadyBound when allocation is owned by another return")

	// Prove exactly one return_item_units row exists and is bound to Return A
	var rowCount int
	var boundReturnItemID uuid.UUID
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*), (ARRAY_AGG(return_item_id))[1] FROM return_item_units WHERE order_item_allocation_id = $1 GROUP BY order_item_allocation_id", allocID).Scan(&rowCount, &boundReturnItemID)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)
	assert.Equal(t, respA[0].Items[0].ID, boundReturnItemID, "Allocation must remain bound to Return A")
}

// 5. DUPLICATE SAME-RETURN SCAN
func TestM521_DuplicateSameReturnScan(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-DUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-DUP-" + uuid.New().String()[:8]
	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, zmuCode, fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, tOrd.orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, now() - interval '2 hours', NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID)
	require.NoError(t, err)

	evIDsDup := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsDup}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	// First scan
	scan1, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	assert.False(t, scan1.AlreadyScanned)
	require.NotNil(t, scan1.ReturnItemUnit.ScannedAt)
	firstScannedAt := *scan1.ReturnItemUnit.ScannedAt

	time.Sleep(5 * time.Millisecond)

	// Second duplicate scan
	scan2, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	assert.True(t, scan2.AlreadyScanned)
	require.NotNil(t, scan2.ReturnItemUnit.ScannedAt)
	assert.Equal(t, firstScannedAt.UnixNano(), scan2.ReturnItemUnit.ScannedAt.UnixNano(), "scanned_at must not be overwritten")

	// Assert exactly 1 row exists
	var count int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1", resp[0].Items[0].ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Progress in read model must be exactly 1 scanned, 0 remaining
	state, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, 1, state.TotalRequested)
	assert.Equal(t, 1, state.TotalScanned)
	assert.Equal(t, 0, state.TotalRemaining)
}

// 6. QUANTITY RACE — TWO DIFFERENT ZMUs CONSUMING ONE SLOT
func TestM521_QuantityRace_TwoDifferentZMUs(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Outbound order item quantity = 2, with 2 outbound ZMUs
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)

	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-RACE-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuA := "ZMU-RACE-A-" + uuid.New().String()[:8]
	zmuB := "ZMU-RACE-B-" + uuid.New().String()[:8]
	invUnitA, invUnitB := uuid.New(), uuid.New()

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped'), ($6, $7, $8, $9, $10, 2, 'shipped')
	`, invUnitA, zmuA, fix.varAID, supplyID, supplyItemID,
		invUnitB, zmuB, fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 2, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, tOrd.orderID)
	require.NoError(t, err)

	allocA, allocB := uuid.New(), uuid.New()
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL), ($6, $7, $8, $9, $10, NULL)
	`, allocA, tOrd.orderItemID, invUnitA, resID, pickedTime,
		allocB, tOrd.orderItemID, invUnitB, resID, pickedTime)
	require.NoError(t, err)

	// Return item requested quantity = 1 ONLY!
	evIDsRace := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsRace}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	// Concurrent race between ZMU-A and ZMU-B
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	quantityExceededCount := 0

	codes := []string{zmuA, zmuB}
	wg.Add(2)
	for _, c := range codes {
		code := c
		go func() {
			defer wg.Done()
			_, scanErr := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: code})
			mu.Lock()
			defer mu.Unlock()
			if scanErr == nil {
				successCount++
			} else if scanErr == returns.ErrQuantityExceeded {
				quantityExceededCount++
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, successCount, "Exactly one scan must succeed")
	assert.Equal(t, 1, quantityExceededCount, "The second scan must fail with ErrQuantityExceeded")

	// DB check: exactly one row in return_item_units
	var dbRowCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1", resp[0].Items[0].ID).Scan(&dbRowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, dbRowCount)
}

// 7. SAME ZMU CONCURRENCY
func TestM521_SameZMUConcurrency(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-SAMECONC-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-SAMECONC-" + uuid.New().String()[:8]
	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, zmuCode, fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, tOrd.orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, now() - interval '2 hours', NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID)
	require.NoError(t, err)

	evIDsSame := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsSame}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	var wg sync.WaitGroup
	var mu sync.Mutex
	newlyScannedCount := 0
	alreadyScannedCount := 0

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
			if err == nil {
				mu.Lock()
				if !scanResp.AlreadyScanned {
					newlyScannedCount++
				} else {
					alreadyScannedCount++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, newlyScannedCount, "Exactly one concurrent call must return alreadyScanned = false")
	assert.Equal(t, 4, alreadyScannedCount, "All other concurrent calls must return alreadyScanned = true")

	// DB: exactly 1 row
	var dbCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1", resp[0].Items[0].ID).Scan(&dbCount)
	require.NoError(t, err)
	assert.Equal(t, 1, dbCount)
}

// 8. LEGACY ITEM SCAN REJECTION & READ MODEL
func TestM521_LegacyItem(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Pure legacy order: 0 order_item_allocations
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "legacy return",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	// Attempt scanning arbitrary ZMU
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: "ZMU-ANYTHING"})
	assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn)

	// DB: 0 return_item_units
	var count int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1", resp[0].Items[0].ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Read model: item is exposed as legacy, 0 outbound allocations
	state, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	require.Len(t, state.Items, 1)
	assert.Equal(t, "legacy", state.Items[0].AllocationMode)
	assert.Empty(t, state.Items[0].OutboundAllocations)
	assert.Empty(t, state.Items[0].ScannedUnits)
	assert.Equal(t, 1, state.LegacyRequested)
	assert.Equal(t, 0, state.SerializedRequested)
	assert.Equal(t, 0, state.SerializedScanned)
	assert.Equal(t, 1, state.TotalRequested)
	assert.Equal(t, 0, state.TotalScanned)
	assert.Equal(t, 1, state.TotalRemaining)
}

// 9. MIXED RETURN (SERIALIZED + LEGACY)
func TestM521_MixedReturn(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Create order with 1 fulfillment, 2 items (Item 1 = Serialized, Item 2 = Legacy)
	orderID := uuid.New()
	fID := uuid.New()
	oiSerial := uuid.New()
	oiLegacy := uuid.New()

	orderNum := fmt.Sprintf("ORD-MIX-%s", uuid.New().String()[:8])
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 2000, 'RUB', 'Addr', 'Method', 0, 'Name', 'Email', 'Phone')
	`, orderID, fix.userID, orderNum)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Serial Item', 'slug-a', 1000, 1000, 1),
		       ($7, $2, $3, $4, $8, $9, 'Legacy Item', 'slug-b', 1000, 1000, 1)
	`, oiSerial, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID,
		oiLegacy, fix.prodBID, fix.varBID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '2 hours', now() - interval '1 hour', now(), now())
	`, uuid.New(), orderID, fID)
	require.NoError(t, err)

	// Create physical ZMU allocation for oiSerial ONLY
	invItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-MIX-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuSerial := "ZMU-MIX-SER-" + uuid.New().String()[:8]
	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, zmuSerial, fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, now() - interval '2 hours', NULL)
	`, allocID, oiSerial, invUnitID, resID)
	require.NoError(t, err)

	// Create single Return with BOTH items
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason:  "mixed return",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oiSerial, Quantity: 1},
			{OrderItemID: oiLegacy, Quantity: 1},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	// Initial Read Model
	state1, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, 2, state1.TotalRequested)
	assert.Equal(t, 0, state1.TotalScanned)
	assert.Equal(t, 2, state1.TotalRemaining)
	assert.Equal(t, 1, state1.SerializedRequested)
	assert.Equal(t, 0, state1.SerializedScanned)
	assert.Equal(t, 1, state1.LegacyRequested)

	// Scan valid serialized ZMU
	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuSerial})
	require.NoError(t, err)
	assert.False(t, scanResp.AlreadyScanned)

	// Updated Read Model
	state2, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, 2, state2.TotalRequested)
	assert.Equal(t, 1, state2.TotalScanned)
	assert.Equal(t, 1, state2.TotalRemaining)
	assert.Equal(t, 1, state2.SerializedRequested)
	assert.Equal(t, 1, state2.SerializedScanned)
	assert.Equal(t, 1, state2.LegacyRequested)

	// Verify legacy item remains 0 scanned
	for _, item := range state2.Items {
		if item.AllocationMode == "legacy" {
			assert.Equal(t, 1, item.RequestedQuantity)
			assert.Equal(t, 0, item.ScannedQuantity)
			assert.Equal(t, 1, item.RemainingQuantity)
			assert.Empty(t, item.ScannedUnits)
		} else if item.AllocationMode == "serialized" {
			assert.Equal(t, 1, item.RequestedQuantity)
			assert.Equal(t, 1, item.ScannedQuantity)
			assert.Equal(t, 0, item.RemainingQuantity)
			assert.Len(t, item.ScannedUnits, 1)
		}
	}
}

// 10. READ MODEL COMPLETENESS
func TestM521_ReadModelCompleteness(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-READ-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-READ-" + uuid.New().String()[:8]
	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, zmuCode, fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, tOrd.orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	pickedTime := time.Now().Add(-2 * time.Hour).Truncate(time.Microsecond)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "read model check",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)

	state, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)

	// Check Return fields
	assert.Equal(t, retID, state.Return.ID)
	assert.Equal(t, tOrd.orderID, state.Return.OrderID)
	assert.Equal(t, tOrd.fulfillmentID, state.Return.FulfillmentID)
	assert.Equal(t, "receiving", state.Return.Status)
	assert.NotNil(t, state.Return.ReceivingStartedAt)

	// Check OrderNumber
	assert.NotNil(t, state.OrderNumber)
	assert.NotEmpty(t, *state.OrderNumber)

	// Check Item details
	require.Len(t, state.Items, 1)
	item := state.Items[0]
	assert.Equal(t, "serialized", item.AllocationMode)
	assert.Equal(t, 1, item.RequestedQuantity)
	assert.Equal(t, 1, item.ScannedQuantity)
	assert.Equal(t, 0, item.RemainingQuantity)

	// Check OutboundAllocations detail
	require.Len(t, item.OutboundAllocations, 1)
	outAlloc := item.OutboundAllocations[0]
	assert.Equal(t, allocID, outAlloc.AllocationID)
	assert.Equal(t, zmuCode, outAlloc.UnitCode)
	assert.Equal(t, "shipped", outAlloc.UnitStatus)
	require.NotNil(t, outAlloc.PickedAt)
	assert.Nil(t, outAlloc.ReleasedAt)

	// Check ScannedUnits detail
	require.Len(t, item.ScannedUnits, 1)
	scannedUnit := item.ScannedUnits[0]
	assert.Equal(t, allocID, scannedUnit.OrderItemAllocationID)
	assert.Equal(t, zmuCode, scannedUnit.UnitCode)
	assert.NotNil(t, scannedUnit.ScannedAt)
}

// 11. FULL NO-SIDE-EFFECT PROOF
func TestM521_FullNoSideEffectProof(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-NOSE-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-NOSE-" + uuid.New().String()[:8]
	invUnitID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, 1, 'shipped')
	`, invUnitID, zmuCode, fix.varAID, supplyID, supplyItemID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id)
		VALUES ($1, $2, $3, $4, $5, 1, 'converted', now() + interval '1 hour', $6)
	`, resID, invItemID, fix.prodAID, fix.varAID, fix.userID, tOrd.orderID)
	require.NoError(t, err)

	allocID := uuid.New()
	pickedTime := time.Now().Add(-2 * time.Hour).Truncate(time.Microsecond)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "no side effects",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

	// Take full baseline snapshot before StartReceiving & Scan
	var baseTotalStock, baseReservedStock int
	err = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&baseTotalStock, &baseReservedStock)
	require.NoError(t, err)

	var baseUnitStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&baseUnitStatus)
	require.NoError(t, err)

	var baseResStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM reservations WHERE id = $1", resID).Scan(&baseResStatus)
	require.NoError(t, err)

	var basePickedAt *time.Time
	var baseReleasedAt *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT picked_at, released_at FROM order_item_allocations WHERE id = $1", allocID).Scan(&basePickedAt, &baseReleasedAt)
	require.NoError(t, err)

	var baseRefundCount, baseMovementCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE order_id = $1", tOrd.orderID).Scan(&baseRefundCount)
	require.NoError(t, err)
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&baseMovementCount)
	require.NoError(t, err)

	// Execute StartReceiving + Scan
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	scanRes, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	assert.False(t, scanRes.AlreadyScanned)

	// Take snapshot after
	var afterTotalStock, afterReservedStock int
	err = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&afterTotalStock, &afterReservedStock)
	require.NoError(t, err)
	assert.Equal(t, baseTotalStock, afterTotalStock, "total_stock must not change")
	assert.Equal(t, baseReservedStock, afterReservedStock, "reserved_stock must not change")

	var afterUnitStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&afterUnitStatus)
	require.NoError(t, err)
	assert.Equal(t, baseUnitStatus, afterUnitStatus, "inventory_unit status must not change")

	var afterResStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM reservations WHERE id = $1", resID).Scan(&afterResStatus)
	require.NoError(t, err)
	assert.Equal(t, baseResStatus, afterResStatus, "reservation status must not change")

	var afterPickedAt, afterReleasedAt *time.Time
	err = fix.client.Pool.QueryRow(ctx, "SELECT picked_at, released_at FROM order_item_allocations WHERE id = $1", allocID).Scan(&afterPickedAt, &afterReleasedAt)
	require.NoError(t, err)
	assert.Equal(t, basePickedAt.UnixNano(), afterPickedAt.UnixNano(), "picked_at must not change")
	assert.Equal(t, baseReleasedAt, afterReleasedAt, "released_at must not change")

	var afterRefundCount, afterMovementCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE order_id = $1", tOrd.orderID).Scan(&afterRefundCount)
	require.NoError(t, err)
	assert.Equal(t, baseRefundCount, afterRefundCount, "refunds count must not change")

	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&afterMovementCount)
	require.NoError(t, err)
	assert.Equal(t, baseMovementCount, afterMovementCount, "stock movements count must not change")
}
