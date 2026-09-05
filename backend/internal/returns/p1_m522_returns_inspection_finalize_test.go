package returns_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

// 1. SERIALIZED INSPECTION & ZERO PHYSICAL SIDE EFFECTS
func TestM522_SerializedInspection(t *testing.T) {
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
	`, supplyID, fix.sellerAID, "SUP-INSP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-INSP-" + uuid.New().String()[:8]
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
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	evIDs1 := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs1}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	scanResp, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	unitID := scanResp.ReturnItemUnit.ID

	// A. Invalid disposition rejected
	err = fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "invalid_disp",
	})
	assert.ErrorIs(t, err, returns.ErrInvalidDisposition)

	// B. Invalid unit/return mismatch rejected
	err = fix.svc.InspectSerializedUnit(ctx, uuid.New(), unitID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "restock",
	})
	assert.ErrorIs(t, err, returns.ErrReturnNotFound)

	otherUnitID := uuid.New()
	err = fix.svc.InspectSerializedUnit(ctx, retID, otherUnitID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "restock",
	})
	assert.ErrorIs(t, err, returns.ErrUnitNotInReturn)

	// C. Valid disposition: restock
	cond := "Like new condition"
	err = fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
		InspectedCondition: &cond,
		Disposition:        "restock",
	})
	require.NoError(t, err)

	state, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	require.Len(t, state.Items[0].ScannedUnits, 1)
	assert.Equal(t, "restock", *state.Items[0].ScannedUnits[0].Disposition)
	assert.Equal(t, "Like new condition", *state.Items[0].ScannedUnits[0].InspectedCondition)
	assert.True(t, state.CanFinalize)

	// D. Update disposition to damaged
	condDamaged := "Scratch on back"
	err = fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
		InspectedCondition: &condDamaged,
		Disposition:        "damaged",
	})
	require.NoError(t, err)

	state, err = fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "damaged", *state.Items[0].ScannedUnits[0].Disposition)

	// E. Update disposition to reject
	err = fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "reject",
	})
	require.NoError(t, err)

	// F. Assert ZERO physical side effects occurred during inspection
	var totalStock, resStock int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStock, &resStock)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 0, resStock)

	var uStatus string
	_ = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&uStatus)
	assert.Equal(t, "shipped", uStatus)

	var movCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&movCount)
	assert.Equal(t, 0, movCount)
}

// 2. SERIALIZED REJECT FINALIZE & IDEMPOTENCY
func TestM522_SerializedRejectFinalize(t *testing.T) {
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
	`, supplyID, fix.sellerAID, "SUP-REJ-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-REJ-" + uuid.New().String()[:8]
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
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_fit",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)

	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanA.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "reject",
	}))

	// Finalize receiving
	err = fix.svc.FinalizeReceiving(ctx, retID)
	require.NoError(t, err)

	// Assertions:
	// 1. return.status = item_received
	ret, err := fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", ret.Status)

	// 2. inventory_unit.status remains shipped
	var uStatus string
	_ = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&uStatus)
	assert.Equal(t, "shipped", uStatus)

	// 3. inventory_items.total_stock unchanged (10), reserved_stock unchanged (0)
	var totalStock, resStock int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStock, &resStock)
	assert.Equal(t, 10, totalStock)
	assert.Equal(t, 0, resStock)

	// 4. no return stock movement created
	var movCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&movCount)
	assert.Equal(t, 0, movCount)

	// 5. reservation remains converted
	var resStatus string
	_ = fix.client.Pool.QueryRow(ctx, "SELECT status FROM reservations WHERE id = $1", resID).Scan(&resStatus)
	assert.Equal(t, "converted", resStatus)

	// 6. allocation picked_at unchanged, released_at remains NULL
	var pAt, rAt *time.Time
	_ = fix.client.Pool.QueryRow(ctx, "SELECT picked_at, released_at FROM order_item_allocations WHERE id = $1", allocID).Scan(&pAt, &rAt)
	assert.NotNil(t, pAt)
	assert.Nil(t, rAt)

	// Repeat FinalizeReceiving -> idempotent no-op, no second effects
	err = fix.svc.FinalizeReceiving(ctx, retID)
	require.NoError(t, err)

	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStock)
	assert.Equal(t, 10, totalStock)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&movCount)
	assert.Equal(t, 0, movCount)
}

// 3. LEGACY FINALIZE — FULL BUCKET MATRIX
func TestM522_LegacyFinalize_FullBucket(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Legacy order: requested = 5
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)

	invItemID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 100, 0)
	`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
	require.NoError(t, err)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "legacy return",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 5}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	itemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	// Inspect: accepted=2, damaged=1, rejected=1 (notReceived=1)
	err = fix.svc.InspectLegacyItem(ctx, retID, itemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 2,
		DamagedQuantity:  1,
		RejectedQuantity: 1,
	})
	require.NoError(t, err)

	// Finalize
	err = fix.svc.FinalizeReceiving(ctx, retID)
	require.NoError(t, err)

	// Assertions:
	// 1. Return -> item_received
	ret, err := fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", ret.Status)

	// 2. total_stock += 2 only (100 -> 102), reserved_stock unchanged (0)
	var totalStock, resStock int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStock, &resStock)
	assert.Equal(t, 102, totalStock)
	assert.Equal(t, 0, resStock)

	// 3. no fake inventory_units created
	var iuCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_units WHERE product_variant_id = $1", fix.varAID).Scan(&iuCount)
	assert.Equal(t, 0, iuCount, "Legacy finalize must not synthesize inventory_units")

	// 4. exactly 1 stock movement with quantity = 2
	var movQty int
	var movType, movReason, movRefType string
	var movRefID uuid.UUID
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT quantity, type, reason, reference_type, reference_id
		FROM stock_movements
		WHERE inventory_item_id = $1
	`, invItemID).Scan(&movQty, &movType, &movReason, &movRefType, &movRefID)
	require.NoError(t, err)
	assert.Equal(t, 2, movQty)
	assert.Equal(t, "return", movType)
	assert.Equal(t, "return_restock", movReason)
	assert.Equal(t, "return", movRefType)
	assert.Equal(t, retID, movRefID)
}

// 4. LEGACY ENDPOINT MUST REJECT SERIALIZED ITEM
func TestM522_LegacyEndpoint_RejectsSerializedItem(t *testing.T) {
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
	`, supplyID, fix.sellerAID, "SUP-SERREJ-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-SERREJ-" + uuid.New().String()[:8]
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

	evIDsLeg := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsLeg}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	itemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	// Call InspectLegacyItem on serialized return item -> ErrItemNotLegacy
	err = fix.svc.InspectLegacyItem(ctx, retID, itemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 1,
		DamagedQuantity:  0,
		RejectedQuantity: 0,
	})
	assert.ErrorIs(t, err, returns.ErrItemNotLegacy)

	// Verify no legacy quantities were set in DB
	var accQty, dmgQty, rejQty int
	err = fix.client.Pool.QueryRow(ctx, "SELECT accepted_quantity, damaged_quantity, rejected_quantity FROM return_items WHERE id = $1", itemID).Scan(&accQty, &dmgQty, &rejQty)
	require.NoError(t, err)
	assert.Equal(t, 0, accQty)
	assert.Equal(t, 0, dmgQty)
	assert.Equal(t, 0, rejQty)
}

// 5. INSPECTION AFTER FINALIZE REJECTION
func TestM522_InspectionAfterFinalize(t *testing.T) {
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
	`, supplyID, fix.sellerAID, "SUP-POSTFIN-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-POSTFIN-" + uuid.New().String()[:8]
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
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	evIDsFin := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsFin}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	unitID := scanA.ReturnItemUnit.ID

	cond := "Original Inspection"
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
		InspectedCondition: &cond,
		Disposition:        "restock",
	}))

	// Finalize successfully
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Attempt inspection after finalize -> ErrReturnNotInReceiving
	newCond := "Changed Inspection"
	err = fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
		InspectedCondition: &newCond,
		Disposition:        "damaged",
	})
	assert.ErrorIs(t, err, returns.ErrReturnNotInReceiving)

	// Verify disposition and condition remain unchanged
	var dbDisp string
	var dbCond *string
	err = fix.client.Pool.QueryRow(ctx, "SELECT disposition, inspected_condition FROM return_item_units WHERE id = $1", unitID).Scan(&dbDisp, &dbCond)
	require.NoError(t, err)
	assert.Equal(t, "restock", dbDisp)
	assert.Equal(t, "Original Inspection", *dbCond)

	// Verify allocation was cleanly released upon restock finalize (Case D)
	var allocReleasedAt *time.Time
	var releaseReason *string
	err = fix.client.Pool.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocID).Scan(&allocReleasedAt, &releaseReason)
	require.NoError(t, err)
	assert.NotNil(t, allocReleasedAt, "allocation must be released upon restock finalize")
	assert.Equal(t, "returned_restock", *releaseReason)

	// B. Legacy post-finalize inspection test
	tOrdLeg := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)

	respLeg, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdLeg.orderID, returns.CreateReturnRequest{
		Reason:  "legacy return",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdLeg.orderItemID, Quantity: 3}},
	})
	require.NoError(t, err)
	legRetID := respLeg[0].Return.ID
	legItemID := respLeg[0].Items[0].ID


	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, legRetID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, legRetID)

	require.NoError(t, fix.svc.StartReceiving(ctx, legRetID))

	// Initial legacy inspection
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, legRetID, legItemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 1,
		DamagedQuantity:  1,
		RejectedQuantity: 0,
	}))

	// Snapshot stock and movements before legacy finalize (total_stock = 11, movements = 1 from serialized Part A)
	var stockBeforeLegFinalize, movementsBeforeLegFinalize int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&stockBeforeLegFinalize)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&movementsBeforeLegFinalize)

	// Finalize legacy receiving
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, legRetID))

	// Assert return.status = item_received
	retLeg, err := fix.svc.GetAdminReturn(ctx, legRetID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", retLeg.Status)

	// Snapshot stock and movements after finalize (total_stock = 11 + 1 = 12, movements = 2)
	var stockAfterFinalize, movementsAfterFinalize int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&stockAfterFinalize)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&movementsAfterFinalize)
	assert.Equal(t, stockBeforeLegFinalize+1, stockAfterFinalize)
	assert.Equal(t, movementsBeforeLegFinalize+1, movementsAfterFinalize)

	// Attempt InspectLegacyItem after finalize -> ErrReturnNotInReceiving
	err = fix.svc.InspectLegacyItem(ctx, legRetID, legItemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 2,
		DamagedQuantity:  0,
		RejectedQuantity: 1,
	})
	assert.ErrorIs(t, err, returns.ErrReturnNotInReceiving)

	// Prove return_items quantities remain unchanged
	var accQty, dmgQty, rejQty int
	err = fix.client.Pool.QueryRow(ctx, "SELECT accepted_quantity, damaged_quantity, rejected_quantity FROM return_items WHERE id = $1", legItemID).Scan(&accQty, &dmgQty, &rejQty)
	require.NoError(t, err)
	assert.Equal(t, 1, accQty)
	assert.Equal(t, 1, dmgQty)
	assert.Equal(t, 0, rejQty)

	// Prove total_stock is unchanged by rejected post-finalize inspection
	var finalLegStock, finalLegMovements int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&finalLegStock)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&finalLegMovements)
	assert.Equal(t, stockAfterFinalize, finalLegStock, "total_stock must be unchanged by rejected post-finalize inspection")
	assert.Equal(t, movementsAfterFinalize, finalLegMovements, "no additional stock movement created")
}

// 6. INSPECTION VS FINALIZE RACE
func TestM522_InspectionVsFinalizeRace(t *testing.T) {
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
	`, supplyID, fix.sellerAID, "SUP-RACEFIN-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-RACEFIN-" + uuid.New().String()[:8]
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
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	evIDsRace := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsRace}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	unitID := scanA.ReturnItemUnit.ID

	// Initial disposition = restock
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "restock",
	}))

	// Concurrently race:
	// A. Change disposition to damaged
	// B. Finalize receiving
	var wg sync.WaitGroup
	var inspectErr, finalizeErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		inspectErr = fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
			Disposition: "damaged",
		})
	}()
	go func() {
		defer wg.Done()
		finalizeErr = fix.svc.FinalizeReceiving(ctx, retID)
	}()
	wg.Wait()

	require.NoError(t, finalizeErr, "Finalize must always succeed")

	// Verify that state is strictly either OUTCOME 1 or OUTCOME 2:
	var finalUnitStatus, finalDisposition, finalRetStatus string
	var finalTotalStock, finalMovCount int

	_ = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", invUnitID).Scan(&finalUnitStatus)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT disposition FROM return_item_units WHERE id = $1", unitID).Scan(&finalDisposition)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", retID).Scan(&finalRetStatus)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&finalTotalStock)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&finalMovCount)

	assert.Equal(t, "item_received", finalRetStatus)

	if finalDisposition == "restock" {
		// OUTCOME 1: Finalize executed before inspection
		assert.Equal(t, "warehouse", finalUnitStatus)
		assert.Equal(t, 11, finalTotalStock)
		assert.Equal(t, 1, finalMovCount)
		assert.ErrorIs(t, inspectErr, returns.ErrReturnNotInReceiving)
	} else if finalDisposition == "damaged" {
		// OUTCOME 2: Inspection executed before finalize
		assert.Equal(t, "damaged", finalUnitStatus)
		assert.Equal(t, 10, finalTotalStock)
		assert.Equal(t, 0, finalMovCount)
		assert.NoError(t, inspectErr)
	} else {
		t.Fatalf("Unexpected disposition: %s", finalDisposition)
	}
}

// 7. CONCURRENT FINALIZE CALLS
func TestM522_ConcurrentFinalize(t *testing.T) {
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
	`, supplyID, fix.sellerAID, "SUP-CONC-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-CONC-" + uuid.New().String()[:8]
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
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	evIDsConc := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsConc}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanA.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "restock",
	}))

	// Run 5 concurrent Finalize calls and collect all errors
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[idx] = fix.svc.FinalizeReceiving(ctx, retID)
		}()
	}
	wg.Wait()

	// Assert all 5 calls return nil (idempotent success)
	for i, callErr := range errs {
		assert.NoError(t, callErr, fmt.Sprintf("Call %d should return nil", i))
	}

	// Physical mutation happened EXACTLY ONCE
	var totalStock int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE id = $1", invItemID).Scan(&totalStock)
	assert.Equal(t, 11, totalStock, "total_stock must increment by exactly 1")

	var movCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", invItemID).Scan(&movCount)
	assert.Equal(t, 1, movCount, "stock_movements must contain exactly 1 row")
}

// 8. CAN FINALIZE READ MODEL MATRIX
func TestM522_CanFinalize_ReadModel(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

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
	`, supplyID, fix.sellerAID, "SUP-CANFIN-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuA := "ZMU-CANFIN-A-" + uuid.New().String()[:8]
	zmuB := "ZMU-CANFIN-B-" + uuid.New().String()[:8]
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

	evIDsCanFin := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDsCanFin}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	// A. Scanned serialized unit without disposition -> canFinalize = false
	scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuA})
	require.NoError(t, err)

	stateA, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.False(t, stateA.CanFinalize, "CanFinalize must be false when a scanned unit lacks disposition")

	// B. After setting disposition on the scanned unit:
	// Requested = 2, Scanned = 1 with disposition -> canFinalize = true, notReceivedQuantity = 1
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanA.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "restock",
	}))

	stateB, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.True(t, stateB.CanFinalize, "CanFinalize must be true when all scanned units have a disposition")
	assert.Equal(t, 1, stateB.Items[0].NotReceivedQuantity)

	// C. Legacy return with valid quantities -> canFinalize = true, notReceived = 1
	tOrdLeg := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
	respLeg, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdLeg.orderID, returns.CreateReturnRequest{
		Reason:  "legacy return",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdLeg.orderItemID, Quantity: 5}},
	})
	require.NoError(t, err)
	legRetID := respLeg[0].Return.ID
	legItemID := respLeg[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, legRetID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, legRetID)

	require.NoError(t, fix.svc.StartReceiving(ctx, legRetID))

	require.NoError(t, fix.svc.InspectLegacyItem(ctx, legRetID, legItemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 2,
		DamagedQuantity:  1,
		RejectedQuantity: 1,
	}))

	stateLeg, err := fix.svc.GetAdminReturnReceivingState(ctx, legRetID)
	require.NoError(t, err)
	assert.True(t, stateLeg.CanFinalize)
	assert.Equal(t, 1, stateLeg.Items[0].NotReceivedQuantity)

	// D. After Finalize (item_received) -> canFinalize = false
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	stateFin, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", stateFin.Return.Status)
	assert.False(t, stateFin.CanFinalize, "CanFinalize must be false once return is finalized")
}

// 9. NO FINANCIAL SIDE EFFECTS PROOF
func TestM522_NoFinancialSideEffects(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	// Create payment
	payID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, 'tbank', 'succeeded', 1000, 'RUB', $3, now(), now())
	`, payID, tOrd.orderID, "PAY-IDEM-"+uuid.New().String())
	require.NoError(t, err)

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
	`, supplyID, fix.sellerAID, "SUP-NOFIN-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	zmuCode := "ZMU-NOFIN-" + uuid.New().String()[:8]
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
	pickedTime := time.Now().Add(-2 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
	require.NoError(t, err)

	evIDsNoFin := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: func() *string { s := "Valid test comment"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDsNoFin}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))

	scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
	require.NoError(t, err)
	require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, scanA.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{
		Disposition: "restock",
	}))

	// Finalize
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Assert NO refunds created
	var refundCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE order_id = $1", tOrd.orderID).Scan(&refundCount)
	assert.Equal(t, 0, refundCount, "Finalize must not create refund records")

	// Assert payments table untouched
	var paymentStatus string
	_ = fix.client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", tOrd.orderID).Scan(&paymentStatus)
	assert.Equal(t, "succeeded", paymentStatus)

	// Assert seller balance / payouts untouched
	var payoutCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM seller_payouts WHERE seller_id = $1", fix.sellerAID).Scan(&payoutCount)
	assert.Equal(t, 0, payoutCount)
}

func TestReturnAllocationCleanup_ObservabilityAndLifecycle(t *testing.T) {
	ctx := context.Background()

	t.Run("Restock_And_UnrelatedAllocation_And_Idempotency_And_Privacy", func(t *testing.T) {
		fix := setupM51Fixture(t)
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		fix.svc.SetLogger(logger)

		// 1. Setup primary delivered order with serialized item
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
		`, supplyID, fix.sellerAID, "SUP-RESTOCK-"+uuid.New().String()[:8])
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 10, now(), now())
		`, supplyItemID, supplyID, fix.varAID)
		require.NoError(t, err)

		zmuCode := "ZMU-RESTOCK-" + uuid.New().String()[:8]
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
		pickedTime := time.Now().Add(-2 * time.Hour)
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at, release_reason)
			VALUES ($1, $2, $3, $4, $5, NULL, NULL)
		`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
		require.NoError(t, err)

		// 2. Setup UNRELATED order and allocation to prove it remains untouched (Requirement E)
		tOrdUnrelated := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		invUnitUnrelatedID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
			VALUES ($1, $2, $3, $4, $5, 2, 'shipped')
		`, invUnitUnrelatedID, "ZMU-UNRELATED-"+uuid.New().String()[:8], fix.varAID, supplyID, supplyItemID)
		require.NoError(t, err)

		allocUnrelatedID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at, release_reason)
			VALUES ($1, $2, $3, $4, $5, NULL, NULL)
		`, allocUnrelatedID, tOrdUnrelated.orderItemID, invUnitUnrelatedID, resID, pickedTime)
		require.NoError(t, err)

		// 3. Create return with private customer comment (Requirement F)
		secretComment := "PRIVATE_SECRET_CUSTOMER_COMMENT_9999"
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: &secretComment,
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
		require.NoError(t, err)
		unitID := scanA.ReturnItemUnit.ID

		cond := "Good condition"
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
			InspectedCondition: &cond,
			Disposition:        "restock",
		}))

		// Reset log buffer right before FinalizeReceiving
		logBuf.Reset()

		// A. Finalize Receiving -> RESTOCK
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// Verify allocation is released with returned_restock
		var allocReleasedAt *time.Time
		var releaseReason *string
		err = fix.client.Pool.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocID).Scan(&allocReleasedAt, &releaseReason)
		require.NoError(t, err)
		assert.NotNil(t, allocReleasedAt, "allocation must be released upon restock finalize")
		assert.Equal(t, "returned_restock", *releaseReason)

		// E. Verify UNRELATED allocation is UNTOUCHED
		var unrelReleasedAt *time.Time
		var unrelReason *string
		err = fix.client.Pool.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocUnrelatedID).Scan(&unrelReleasedAt, &unrelReason)
		require.NoError(t, err)
		assert.Nil(t, unrelReleasedAt, "unrelated allocation must remain active/unreleased")
		assert.Nil(t, unrelReason, "unrelated allocation release_reason must remain NULL")

		// Verify Business Event
		logs := logBuf.String()
		require.NotEmpty(t, logs)

		var matchedEvent map[string]interface{}
		for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
			var entry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &entry); err == nil {
				if entry["event_name"] == "inventory.return_allocations_released" {
					matchedEvent = entry
					break
				}
			}
		}
		require.NotNil(t, matchedEvent, "expected inventory.return_allocations_released event in logs: %s", logs)
		assert.Equal(t, "inventory", matchedEvent["domain"])
		assert.Equal(t, "return_allocations_released", matchedEvent["action"])
		assert.Equal(t, "success", matchedEvent["result"])
		assert.Equal(t, retID.String(), matchedEvent["return_id"])
		assert.Equal(t, tOrd.orderID.String(), matchedEvent["order_id"])
		assert.NotEmpty(t, matchedEvent["order_number"])
		assert.Equal(t, float64(1), matchedEvent["released_count"])
		assert.Equal(t, float64(1), matchedEvent["restock_released_count"])
		assert.Equal(t, float64(0), matchedEvent["damaged_released_count"])

		// F. Privacy check
		assert.NotContains(t, logs, secretComment, "customer comment must not leak into logs")

		// D. IDEMPOTENT REPEAT
		logBuf.Reset()
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// Check zero events on repeated finalize
		for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var entry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &entry); err == nil {
				assert.NotEqual(t, "inventory.return_allocations_released", entry["event_name"], "repeated finalize must emit zero release events")
			}
		}
	})

	t.Run("Damaged", func(t *testing.T) {
		fix := setupM51Fixture(t)
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		fix.svc.SetLogger(logger)

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
		`, supplyID, fix.sellerAID, "SUP-DAMAGED-"+uuid.New().String()[:8])
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 10, now(), now())
		`, supplyItemID, supplyID, fix.varAID)
		require.NoError(t, err)

		zmuCode := "ZMU-DAMAGED-" + uuid.New().String()[:8]
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
		pickedTime := time.Now().Add(-2 * time.Hour)
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at, release_reason)
			VALUES ($1, $2, $3, $4, $5, NULL, NULL)
		`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
		require.NoError(t, err)

		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: func() *string { s := "Damaged item"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
		require.NoError(t, err)
		unitID := scanA.ReturnItemUnit.ID

		cond := "Broken screen"
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
			InspectedCondition: &cond,
			Disposition:        "damaged",
		}))

		logBuf.Reset()

		// B. Finalize Receiving -> DAMAGED
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		var allocReleasedAt *time.Time
		var releaseReason *string
		err = fix.client.Pool.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocID).Scan(&allocReleasedAt, &releaseReason)
		require.NoError(t, err)
		assert.NotNil(t, allocReleasedAt, "allocation must be released upon damaged finalize")
		assert.Equal(t, "returned_damaged", *releaseReason)

		// Verify event
		logs := logBuf.String()
		require.NotEmpty(t, logs)

		var matchedEvent map[string]interface{}
		for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
			var entry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &entry); err == nil {
				if entry["event_name"] == "inventory.return_allocations_released" {
					matchedEvent = entry
					break
				}
			}
		}
		require.NotNil(t, matchedEvent, "expected inventory.return_allocations_released event in logs: %s", logs)
		assert.Equal(t, float64(1), matchedEvent["released_count"])
		assert.Equal(t, float64(0), matchedEvent["restock_released_count"])
		assert.Equal(t, float64(1), matchedEvent["damaged_released_count"])
	})

	t.Run("Rollback", func(t *testing.T) {
		fix := setupM51Fixture(t)
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		fix.svc.SetLogger(logger)

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
		`, supplyID, fix.sellerAID, "SUP-ROLLBACK-"+uuid.New().String()[:8])
		require.NoError(t, err)

		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 10, now(), now())
		`, supplyItemID, supplyID, fix.varAID)
		require.NoError(t, err)

		zmuCode := "ZMU-ROLLBACK-" + uuid.New().String()[:8]
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
		pickedTime := time.Now().Add(-2 * time.Hour)
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, reservation_id, picked_at, released_at, release_reason)
			VALUES ($1, $2, $3, $4, $5, NULL, NULL)
		`, allocID, tOrd.orderItemID, invUnitID, resID, pickedTime)
		require.NoError(t, err)

		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: func() *string { s := "Rollback test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		scanA, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmuCode})
		require.NoError(t, err)
		unitID := scanA.ReturnItemUnit.ID

		cond := "Good condition"
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, unitID, returns.UpdateSerializedUnitInspectionRequest{
			InspectedCondition: &cond,
			Disposition:        "restock",
		}))

		// Delete inventory_items row to cause tx.QueryRow in FinalizeReceiving to fail on restock stock increment
		_, err = fix.client.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE id = $1", invItemID)
		require.NoError(t, err)

		logBuf.Reset()

		// C. Attempt FinalizeReceiving -> fails in transaction
		err = fix.svc.FinalizeReceiving(ctx, retID)
		require.Error(t, err, "expected FinalizeReceiving to fail due to missing inventory item")

		// Verify allocation released_at rolled back to NULL
		var allocReleasedAt *time.Time
		var releaseReason *string
		err = fix.client.Pool.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocID).Scan(&allocReleasedAt, &releaseReason)
		require.NoError(t, err)
		assert.Nil(t, allocReleasedAt, "allocation released_at must rollback to NULL on transaction failure")
		assert.Nil(t, releaseReason, "allocation release_reason must rollback to NULL on transaction failure")

		// Verify zero events emitted
		for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var entry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &entry); err == nil {
				assert.NotEqual(t, "inventory.return_allocations_released", entry["event_name"], "failed transaction must emit zero release events")
			}
		}
	})
}
