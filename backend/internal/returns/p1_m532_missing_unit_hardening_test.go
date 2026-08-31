package returns_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

func createSucceededPaymentForM532(t *testing.T, fix *m51Fixture, orderID uuid.UUID, amountCents int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	payID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, 'tbank', $3, 'succeeded', $4, 'RUB', $5, now(), now())
	`, payID, orderID, "PAY-"+uuid.New().String()[:8], amountCents, "IDEM-"+uuid.New().String())
	require.NoError(t, err)
	return payID
}

func ensureInventoryItemExists(t *testing.T, fix *m51Fixture) {
	t.Helper()
	ctx := context.Background()
	var count int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_items WHERE product_variant_id = $1", fix.varAID).Scan(&count)
	if count == 0 {
		invItemID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
			VALUES ($1, $2, $3, $4, 20, 0)
		`, invItemID, fix.prodAID, fix.varAID, fix.sellerAID)
		require.NoError(t, err)
	}
}

func createAllocatedUnitsForOrder(t *testing.T, fix *m51Fixture, tOrd testOrder, count int) ([]string, []uuid.UUID) {
	ctx := context.Background()
	ensureInventoryItemExists(t, fix)

	var invItemID uuid.UUID
	err := fix.client.Pool.QueryRow(ctx, "SELECT id FROM inventory_items WHERE product_variant_id = $1 LIMIT 1", fix.varAID).Scan(&invItemID)
	require.NoError(t, err)

	supplyID := uuid.New()
	supplyItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, fix.sellerAID, "SUP-M532-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 20, now(), now())
	`, supplyItemID, supplyID, fix.varAID)
	require.NoError(t, err)

	var zmuCodes []string
	var unitIDs []uuid.UUID

	for i := 1; i <= count; i++ {
		zmuCode := fmt.Sprintf("ZMU-M532-%s-%d", uuid.New().String()[:6], i)
		invUnitID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'shipped')
		`, invUnitID, zmuCode, fix.varAID, supplyID, supplyItemID, i)
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

		zmuCodes = append(zmuCodes, zmuCode)
		unitIDs = append(unitIDs, invUnitID)
	}

	return zmuCodes, unitIDs
}

// TestM532_MissingUnitHardeningMatrix validates all combinations of missing/unreceived physical units.
func TestM532_MissingUnitHardeningMatrix(t *testing.T) {
	ctx := context.Background()

	t.Run("SERIALIZED_CaseA_Q3_AllScanned_AllDispositionsSet", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		createSucceededPaymentForM532(t, fix, tOrd.orderID, 3000)
		zmus, _ := createAllocatedUnitsForOrder(t, fix, tOrd, 3)

		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "damaged",
			Comment: func() *string { s := "3 items return"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		// Scan 3 units
		s1, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmus[0]})
		require.NoError(t, err)
		s2, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmus[1]})
		require.NoError(t, err)
		s3, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmus[2]})
		require.NoError(t, err)

		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, s1.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, s2.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "damaged"}))
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, s3.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "reject"}))

		st, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
		require.NoError(t, err)
		assert.True(t, st.CanFinalize)
		assert.Equal(t, 0, st.Items[0].NotReceivedQuantity)

		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.True(t, quote.CanRefund)
		assert.Equal(t, 2, quote.Items[0].RefundableQuantity) // 1 restock + 1 damaged = 2 refundable
	})

	t.Run("SERIALIZED_CaseB_Q3_PartialScanned_2Scanned_1Unscanned", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		createSucceededPaymentForM532(t, fix, tOrd.orderID, 3000)
		zmus, unitIDs := createAllocatedUnitsForOrder(t, fix, tOrd, 3)

		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "damaged",
			Comment: func() *string { s := "Partial return test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		// Scan only 2 units (1 restock, 1 damaged)
		s1, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmus[0]})
		require.NoError(t, err)
		s2, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmus[1]})
		require.NoError(t, err)

		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, s1.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))
		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, s2.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "damaged"}))

		st, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
		require.NoError(t, err)
		assert.True(t, st.CanFinalize, "CanFinalize must be TRUE when all scanned units have disposition, even with unscanned items")
		assert.Equal(t, 1, st.Items[0].NotReceivedQuantity, "1 unscanned unit must be derived as notReceived")
		assert.Equal(t, 2, st.Items[0].ScannedQuantity)

		// Finalize receiving
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// Verify unscanned 3rd unit remains 'shipped' in inventory_units with NO side effects
		var unscannedStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unitIDs[2]).Scan(&unscannedStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", unscannedStatus, "Unreceived unit must remain in shipped status")

		// Verify refund quote reflects only physical refundable units (1 restock + 1 damaged = 2)
		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.True(t, quote.CanRefund)
		assert.Equal(t, 2, quote.Items[0].RefundableQuantity, "Refund quote must only include 2 physical units, unreceived is not refundable")
	})

	t.Run("SERIALIZED_CaseC_Q3_PartialScanned_MissingDisposition_BlocksFinalize", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
		createSucceededPaymentForM532(t, fix, tOrd.orderID, 3000)
		zmus, _ := createAllocatedUnitsForOrder(t, fix, tOrd, 3)

		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: func() *string { s := "Missing disposition test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		// Scan 2 units, but only set disposition on 1
		s1, err := fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmus[0]})
		require.NoError(t, err)
		_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: zmus[1]})
		require.NoError(t, err)

		require.NoError(t, fix.svc.InspectSerializedUnit(ctx, retID, s1.ReturnItemUnit.ID, returns.UpdateSerializedUnitInspectionRequest{Disposition: "restock"}))

		st, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
		require.NoError(t, err)
		assert.False(t, st.CanFinalize, "CanFinalize must be FALSE when a scanned unit lacks disposition")

		// Attempt finalize -> must error
		err = fix.svc.FinalizeReceiving(ctx, retID)
		require.Error(t, err)
		assert.ErrorIs(t, err, returns.ErrFinalizeMissingDisposition)
	})

	t.Run("SERIALIZED_CaseD_Q1_ZeroScanned_AllowsFinalize_NotRefundable", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPaymentForM532(t, fix, tOrd.orderID, 1000)
		_, unitIDs := createAllocatedUnitsForOrder(t, fix, tOrd, 1)

		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "damaged",
			Comment: func() *string { s := "Empty parcel test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		// 0 units scanned
		st, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
		require.NoError(t, err)
		assert.True(t, st.CanFinalize, "Zero-received after receiving start must allow finalize")
		assert.Equal(t, 1, st.Items[0].NotReceivedQuantity)
		assert.Equal(t, 0, st.Items[0].ScannedQuantity)

		// Finalize zero-received return
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// Check Return transitioned to item_received
		retState, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, "item_received", retState.Return.Status)

		// Unit remains 'shipped'
		var iuStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unitIDs[0]).Scan(&iuStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", iuStatus)

		// Refund quote must report canRefund=false (0 refundable units)
		quote, err := fix.svc.CalculateRefundQuote(ctx, retID)
		require.NoError(t, err)
		assert.False(t, quote.CanRefund)
		assert.Equal(t, 0, quote.Items[0].RefundableQuantity)
		assert.Equal(t, int64(0), quote.TotalRefundCents)
		assert.NotEmpty(t, quote.BlockingReason)
	})

	t.Run("SERIALIZED_CaseE_ForeignZMUScan_Rejected_NotReceivedUnchanged", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		createSucceededPaymentForM532(t, fix, tOrd.orderID, 1000)
		_, _ = createAllocatedUnitsForOrder(t, fix, tOrd, 1)

		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "damaged",
			Comment: func() *string { s := "Foreign scan test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))

		// Scan random non-existent ZMU
		_, err = fix.svc.ScanReturnUnit(ctx, retID, returns.ScanReturnUnitRequest{Code: "ZMU-FOREIGN-99999"})
		require.Error(t, err)
		assert.ErrorIs(t, err, returns.ErrInvalidZMUForReturn)

		st, err := fix.svc.GetAdminReturnReceivingState(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, 1, st.Items[0].NotReceivedQuantity)
		assert.Equal(t, 0, st.Items[0].ScannedQuantity)
	})

	t.Run("LEGACY_CaseF_Q5_PartialInspection_AllowsFinalize", func(t *testing.T) {
		fix := setupM51Fixture(t)
		ensureInventoryItemExists(t, fix)
		tOrdLeg := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
		createSucceededPaymentForM532(t, fix, tOrdLeg.orderID, 5000)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		respLeg, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdLeg.orderID, returns.CreateReturnRequest{
			Reason:  "wrong_item",
			Comment: func() *string { s := "Legacy partial test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdLeg.orderItemID, Quantity: 5, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		legRetID := respLeg[0].Return.ID
		legItemID := respLeg[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, legRetID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, legRetID)
		require.NoError(t, fix.svc.StartReceiving(ctx, legRetID))

		// accepted=2, damaged=1, rejected=1 -> sum=4, notReceived=1
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, legRetID, legItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 2,
			DamagedQuantity:  1,
			RejectedQuantity: 1,
		}))

		st, err := fix.svc.GetAdminReturnReceivingState(ctx, legRetID)
		require.NoError(t, err)
		assert.True(t, st.CanFinalize)
		assert.Equal(t, 1, st.Items[0].NotReceivedQuantity)

		require.NoError(t, fix.svc.FinalizeReceiving(ctx, legRetID))

		quote, err := fix.svc.CalculateRefundQuote(ctx, legRetID)
		require.NoError(t, err)
		assert.True(t, quote.CanRefund)
		assert.Equal(t, 3, quote.Items[0].RefundableQuantity) // accepted 2 + damaged 1 = 3 refundable
	})

	t.Run("LEGACY_CaseG_Q5_FullInspection_SumEqualsRequested", func(t *testing.T) {
		fix := setupM51Fixture(t)
		ensureInventoryItemExists(t, fix)
		tOrdLeg := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
		createSucceededPaymentForM532(t, fix, tOrdLeg.orderID, 5000)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		respLeg, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdLeg.orderID, returns.CreateReturnRequest{
			Reason:  "wrong_item",
			Comment: func() *string { s := "Legacy full test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdLeg.orderItemID, Quantity: 5, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		legRetID := respLeg[0].Return.ID
		legItemID := respLeg[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, legRetID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, legRetID)
		require.NoError(t, fix.svc.StartReceiving(ctx, legRetID))

		// accepted=3, damaged=2, rejected=0 -> sum=5, notReceived=0
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, legRetID, legItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 3,
			DamagedQuantity:  2,
			RejectedQuantity: 0,
		}))

		st, err := fix.svc.GetAdminReturnReceivingState(ctx, legRetID)
		require.NoError(t, err)
		assert.True(t, st.CanFinalize)
		assert.Equal(t, 0, st.Items[0].NotReceivedQuantity)
	})

	t.Run("LEGACY_CaseH_Q5_OverQuantity_FailsInspection", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrdLeg := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		respLeg, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdLeg.orderID, returns.CreateReturnRequest{
			Reason:  "wrong_item",
			Comment: func() *string { s := "Legacy over quantity test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdLeg.orderItemID, Quantity: 5, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		legRetID := respLeg[0].Return.ID
		legItemID := respLeg[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, legRetID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, legRetID)
		require.NoError(t, fix.svc.StartReceiving(ctx, legRetID))

		// accepted=3, damaged=3, rejected=0 -> sum=6 > 5 -> must fail
		err = fix.svc.InspectLegacyItem(ctx, legRetID, legItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 3,
			DamagedQuantity:  3,
			RejectedQuantity: 0,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, returns.ErrInvalidInspectionQuantity)
	})

	t.Run("LEGACY_CaseI_NegativeQuantity_FailsInspection", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrdLeg := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		respLeg, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdLeg.orderID, returns.CreateReturnRequest{
			Reason:  "wrong_item",
			Comment: func() *string { s := "Legacy negative quantity test"; return &s }(),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdLeg.orderItemID, Quantity: 5, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		legRetID := respLeg[0].Return.ID
		legItemID := respLeg[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, legRetID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, legRetID)
		require.NoError(t, fix.svc.StartReceiving(ctx, legRetID))

		err = fix.svc.InspectLegacyItem(ctx, legRetID, legItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: -1,
			DamagedQuantity:  2,
			RejectedQuantity: 0,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, returns.ErrInvalidInspectionQuantity)
	})
}
