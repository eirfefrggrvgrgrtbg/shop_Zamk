package returns_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

// ----------------------------------------------------------------------------
// 15. Critical Ambiguity Regression Test (Section 15)
// ----------------------------------------------------------------------------
func TestLegacyAttribution_15_AmbiguousQ2_CaseA_and_CaseB(t *testing.T) {
	ctx := context.Background()

	// ------------------------------------------------------------------------
	// CASE A:
	// return_item.quantity = 2, accepted = 1, damaged = 1
	// ZAMK allocation -> accepted
	// Seller allocation -> damaged
	// Expected Seller Compensation: 0
	// ------------------------------------------------------------------------
	t.Run("CaseA_ZamkAccepted_SellerDamaged_ZeroCompensation", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
		createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
		createSucceededPayment(t, fix, tOrd.orderID, 2000)
		empID := createEmployeeUser(t, fix)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)

		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: strPtr("mixed return"),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID
		retItemID := resp[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 1,
			DamagedQuantity:  1,
		}))
		ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// Split q2 into q1 and q1
		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		require.Len(t, allocs, 1)

		now := time.Now()
		// Slice 1: ZAMK allocation -> accepted
		alloc1, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocs[0].ID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkFulfillmentError),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionAccepted),
		})
		require.NoError(t, err)

		// Slice 2: Seller allocation -> damaged
		allocsAfter, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		require.Len(t, allocsAfter, 2)
		var otherAllocID = allocs[0].ID
		if allocsAfter[0].ID == alloc1.ID {
			otherAllocID = allocsAfter[1].ID
		} else {
			otherAllocID = allocsAfter[0].ID
		}

		_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      otherAllocID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartySeller),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
		})
		require.NoError(t, err)

		// Refund succeeds for both
		ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

		// Sale Reversal covers 2 units = -700000
		reversals := getSaleReversalEntries(t, fix, tOrd.orderItemID)
		require.Len(t, reversals, 1)
		assert.Equal(t, int64(-700000), reversals[0])

		// Expected Seller Compensation: 0
		assert.Equal(t, int64(0), getNetCompensation(t, fix, tOrd.orderItemID))
		assert.Len(t, getCompensationEntries(t, fix, tOrd.orderItemID), 0)
	})

	// ------------------------------------------------------------------------
	// CASE B (isolated fixture):
	// return_item.quantity = 2, accepted = 1, damaged = 1
	// ZAMK allocation -> damaged
	// Seller allocation -> accepted
	// Same aggregate physical counts.
	// Same aggregate responsibility counts.
	// Expected Seller Compensation: exactly 1-unit target (350000)
	// ------------------------------------------------------------------------
	t.Run("CaseB_ZamkDamaged_SellerAccepted_OneUnitCompensation", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
		createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
		createSucceededPayment(t, fix, tOrd.orderID, 2000)
		empID := createEmployeeUser(t, fix)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)

		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: strPtr("mixed return"),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID
		retItemID := resp[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 1,
			DamagedQuantity:  1,
		}))
		ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// Split q2 into q1 and q1
		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		require.Len(t, allocs, 1)

		now := time.Now()
		// Slice 1: ZAMK allocation -> damaged
		alloc1, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocs[0].ID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkFulfillmentError),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
		})
		require.NoError(t, err)

		// Slice 2: Seller allocation -> accepted
		allocsAfter, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		require.Len(t, allocsAfter, 2)
		var otherAllocID = allocs[0].ID
		if allocsAfter[0].ID == alloc1.ID {
			otherAllocID = allocsAfter[1].ID
		} else {
			otherAllocID = allocsAfter[0].ID
		}

		_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      otherAllocID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartySeller),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionAccepted),
		})
		require.NoError(t, err)

		// Refund succeeds for both
		ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

		// Sale Reversal covers 2 units = -700000
		reversals := getSaleReversalEntries(t, fix, tOrd.orderItemID)
		require.Len(t, reversals, 1)
		assert.Equal(t, int64(-700000), reversals[0])

		// Expected Seller Compensation: exactly 1 unit target = 350000
		comps := getCompensationEntries(t, fix, tOrd.orderItemID)
		require.Len(t, comps, 1)
		assert.Equal(t, int64(350000), comps[0])
		assert.Equal(t, int64(350000), getNetCompensation(t, fix, tOrd.orderItemID))
	})
}

// ----------------------------------------------------------------------------
// 16. Bucket Capacity Tests (Section 16)
// ----------------------------------------------------------------------------
func TestLegacyAttribution_16_BucketCapacityTests(t *testing.T) {
	ctx := context.Background()

	// A: quantity=2, damaged=1 -> allocation damaged q1, second allocation damaged q1 => REJECT
	t.Run("A_ExceedDamagedCapacity_Rejected", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		empID := createEmployeeUser(t, fix)

		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: strPtr("test return"),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID
		retItemID := resp[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
			DamagedQuantity: 1,
		}))
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		require.Len(t, allocs, 1)

		now := time.Now()
		// First allocation damaged q1 -> PASS
		alloc1, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocs[0].ID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
		})
		require.NoError(t, err)

		// Second allocation damaged q1 -> MUST BE REJECTED (capacity = 1, requested total = 2)
		allocsAfter, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		var otherAllocID = allocs[0].ID
		if allocsAfter[0].ID == alloc1.ID {
			otherAllocID = allocsAfter[1].ID
		} else {
			otherAllocID = allocsAfter[0].ID
		}

		_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      otherAllocID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, returns.ErrResponsibilityBucketCapacityExceeded)
	})

	// B: accepted=1, damaged=1 -> q1 accepted, q1 damaged => PASS
	t.Run("B_Accepted1_Damaged1_Passes", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		empID := createEmployeeUser(t, fix)

		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: strPtr("test return"),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID
		retItemID := resp[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 1,
			DamagedQuantity:  1,
		}))
		ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)

		now := time.Now()
		alloc1, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocs[0].ID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusNotRequired,
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionAccepted),
		})
		require.NoError(t, err)

		allocsAfter, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		var otherAllocID = allocs[0].ID
		if allocsAfter[0].ID == alloc1.ID {
			otherAllocID = allocsAfter[1].ID
		} else {
			otherAllocID = allocsAfter[0].ID
		}

		_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      otherAllocID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
		})
		require.NoError(t, err)
	})

	// C: unreceived=1 -> q1 unreceived => PASS
	t.Run("C_Unreceived1_Passes", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		empID := createEmployeeUser(t, fix)

		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: strPtr("test return"),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID
		retItemID := resp[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 1,
			DamagedQuantity:  0,
			RejectedQuantity: 0,
		}))
		ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// unreceived capacity = 2 - 1 - 0 - 0 = 1
		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)

		now := time.Now()
		_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocs[0].ID,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyCarrier),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonCarrierDamage),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionUnreceived),
		})
		require.NoError(t, err)
	})

	// D: attempt unreceived q2 when capacity=1 => REJECT
	t.Run("D_AttemptUnreceivedExceedingCapacity_Rejected", func(t *testing.T) {
		fix := setupM51Fixture(t)
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		empID := createEmployeeUser(t, fix)

		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: strPtr("test return"),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID
		retItemID := resp[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
			AcceptedQuantity: 1,
		}))
		ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		// unreceived capacity = 2 - 1 = 1. Attempting q=2 unreceived:
		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)

		now := time.Now()
		_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocs[0].ID,
			Quantity:          2,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyCarrier),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonCarrierDamage),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionUnreceived),
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, returns.ErrResponsibilityBucketCapacityExceeded)
	})

	// E: serialized allocation + legacy_disposition => REJECT
	t.Run("E_SerializedAllocationWithLegacyDisposition_Rejected", func(t *testing.T) {
		fix := setupM51Fixture(t)
		empID := createEmployeeUser(t, fix)

		// Create serialized order item & allocation
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
		_, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)
		allocID := allocIDs[0]

		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "size_fit",
			Comment: strPtr("test size return"),
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID

		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		require.Len(t, allocs, 1)

		now := time.Now()
		// Bind serialized unit first
		boundAlloc, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:          allocs[0].ID,
			Quantity:              1,
			OrderItemAllocationID: &allocID,
			Status:                returns.ReturnResponsibilityStatusPending,
		})
		require.NoError(t, err)
		require.NotNil(t, boundAlloc.OrderItemAllocationID)

		// Now attempt to supply LegacyDisposition on serialized allocation -> REJECT
		_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:          boundAlloc.ID,
			Quantity:              1,
			OrderItemAllocationID: &allocID,
			Status:                returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:      strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:            strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
			DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:               &empID,
			DecidedAt:             &now,
			LegacyDisposition:     strPtr(returns.LegacyDispositionDamaged),
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, returns.ErrSerializedLegacyDispositionForbidden)
	})
}

// ----------------------------------------------------------------------------
// 17. Pre-Finalization Test (Section 17)
// ----------------------------------------------------------------------------
func TestLegacyAttribution_17_PreFinalizationAndMigration(t *testing.T) {
	ctx := context.Background()
	fix := setupM51Fixture(t)

	// Canonical CreateReturn: legacy q2, physical receiving not finalized
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("test return"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	// Initial: pending q2, legacy_disposition = NULL
	assert.Equal(t, 2, allocs[0].Quantity)
	assert.Equal(t, returns.ReturnResponsibilityStatusPending, allocs[0].Status)
	assert.Nil(t, allocs[0].LegacyDisposition)
	assert.Nil(t, allocs[0].OrderItemAllocationID)

	// In database, legacy_disposition IS NULL
	var dbDisp *string
	err = fix.client.Pool.QueryRow(ctx, "SELECT legacy_disposition FROM return_responsibility_allocations WHERE id = $1", allocs[0].ID).Scan(&dbDisp)
	require.NoError(t, err)
	assert.Nil(t, dbDisp, "Existing/new pre-finalization allocation must have legacy_disposition IS NULL")
}

// ----------------------------------------------------------------------------
// 18. History Test (Section 18)
// ----------------------------------------------------------------------------
func TestLegacyAttribution_18_HistoryRecordsCorrection(t *testing.T) {
	ctx := context.Background()
	fix := setupM51Fixture(t)
	empID := createEmployeeUser(t, fix)

	// Setup return with q=3: accepted=2, damaged=1
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("test return"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 2,
		DamagedQuantity:  1,
	}))
	ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now1 := time.Now().Truncate(time.Millisecond)
	// Split 1: q1 damaged / ZAMK
	allocDamaged, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:      allocs[0].ID,
		Quantity:          1,
		Status:            returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:           &empID,
		DecidedAt:         &now1,
		LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
	})
	require.NoError(t, err)

	// Now correct allocDamaged: damaged -> accepted
	// In this fixture: accepted capacity = 2, used = 0 (other 2 units still pending NULL).
	// So changing allocDamaged from damaged to accepted makes accepted used = 1 <= 2. Valid!
	now2 := time.Now().Add(1 * time.Second).Truncate(time.Millisecond)
	correctedAlloc, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:      allocDamaged.ID,
		Quantity:          1,
		Status:            returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkFulfillmentError),
		DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:           &empID,
		DecidedAt:         &now2,
		LegacyDisposition: strPtr(returns.LegacyDispositionAccepted),
	})
	require.NoError(t, err)
	require.NotNil(t, correctedAlloc.LegacyDisposition)
	assert.Equal(t, returns.LegacyDispositionAccepted, *correctedAlloc.LegacyDisposition)

	// Verify history for allocDamaged
	hist, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocDamaged.ID)
	require.NoError(t, err)
	// Expect 2 history rows for allocDamaged (split created it with damaged, then updated to accepted)
	require.Len(t, hist, 2)

	assert.Equal(t, returns.LegacyDispositionDamaged, *hist[0].LegacyDisposition)
	assert.Equal(t, returns.ReturnResponsibilityReasonZamkWarehouseDamage, *hist[0].ReasonCode)

	assert.Equal(t, returns.LegacyDispositionAccepted, *hist[1].LegacyDisposition)
	assert.Equal(t, returns.ReturnResponsibilityReasonZamkFulfillmentError, *hist[1].ReasonCode)
}

// ----------------------------------------------------------------------------
// 19. Concurrency Test (Section 19)
// ----------------------------------------------------------------------------
func TestLegacyAttribution_19_ConcurrentAttribution(t *testing.T) {
	ctx := context.Background()
	fix := setupM51Fixture(t)
	empID := createEmployeeUser(t, fix)

	// quantity=2, damaged=1, unreceived=1
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("test return"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Split into two distinct unbound allocations of quantity 1 each
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now()
	// Split off 1 unit (remains pending, legacy_disposition = nil)
	splitAlloc, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID: allocs[0].ID,
		Quantity:     1,
		Status:       returns.ReturnResponsibilityStatusPending,
	})
	require.NoError(t, err)

	allocsAfter, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocsAfter, 2)

	allocID1 := splitAlloc.ID
	var allocID2 = allocs[0].ID
	if allocsAfter[0].ID == allocID1 {
		allocID2 = allocsAfter[1].ID
	} else {
		allocID2 = allocsAfter[0].ID
	}

	// Two concurrent goroutines attempt to bind both allocations to legacy_disposition = damaged
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, errs[0] = fix.svc.SetReturnResponsibilityAllocation(context.Background(), returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocID1,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
		})
	}()

	go func() {
		defer wg.Done()
		_, errs[1] = fix.svc.SetReturnResponsibilityAllocation(context.Background(), returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      allocID2,
			Quantity:          1,
			Status:            returns.ReturnResponsibilityStatusResolved,
			ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
			ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
			DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
			ActorID:           &empID,
			DecidedAt:         &now,
			LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
		})
	}()

	wg.Wait()

	// Exactly ONE must succeed and the other must fail with ErrResponsibilityBucketCapacityExceeded
	successCount := 0
	failureCount := 0
	for _, e := range errs {
		if e == nil {
			successCount++
		} else {
			failureCount++
			assert.ErrorIs(t, e, returns.ErrResponsibilityBucketCapacityExceeded)
		}
	}

	assert.Equal(t, 1, successCount, "Exactly ONE concurrent allocation to damaged must succeed")
	assert.Equal(t, 1, failureCount, "The other concurrent allocation must fail due to bucket capacity")

	// Final damaged-attributed responsibility quantity in DB = 1
	var totalDamagedInDB int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM return_responsibility_allocations WHERE return_item_id = $1 AND legacy_disposition = 'damaged'", retItemID).Scan(&totalDamagedInDB)
	require.NoError(t, err)
	assert.Equal(t, 1, totalDamagedInDB, "Final damaged-attributed quantity must equal damaged capacity (1)")
}

// ----------------------------------------------------------------------------
// 20. B.2B Production Triggers Proof (Section 20)
// ----------------------------------------------------------------------------
func TestLegacyAttribution_20_TriggerB_PositiveCompensation(t *testing.T) {
	ctx := context.Background()
	fix := setupM51Fixture(t)
	empID := createEmployeeUser(t, fix)

	// Order: Q=1, E=700000
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("test return"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Refund already succeeded
	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	// Legacy damaged slice currently unattributed / null => compensation = 0
	assert.Equal(t, int64(0), getNetCompensation(t, fix, tOrd.orderItemID))
	assert.Len(t, getCompensationEntries(t, fix, tOrd.orderItemID), 0)

	// Then update allocation through real service to: legacy_disposition = damaged, resolved ZAMK
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now()
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:      allocs[0].ID,
		Quantity:          1,
		Status:            returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:           &empID,
		DecidedAt:         &now,
		LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
	})
	require.NoError(t, err)

	// Expected existing production Trigger B automatically creates compensation!
	// No direct ReconcileReturnCompensationTx call in test.
	comps := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, comps, 1)
	assert.Equal(t, int64(700000), comps[0])
	assert.Equal(t, int64(700000), getNetCompensation(t, fix, tOrd.orderItemID))
}

// ----------------------------------------------------------------------------
// 21. Negative Correction Through Attribution (Section 21)
// ----------------------------------------------------------------------------
func TestLegacyAttribution_21_NegativeCorrectionThroughAttribution(t *testing.T) {
	ctx := context.Background()
	fix := setupM51Fixture(t)
	empID := createEmployeeUser(t, fix)

	// Setup Order Q=2, E=700000 (350000 per unit)
	// physical: accepted = 2, damaged = 1 => wait, quantity=3!
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 750000) // 250000 per unit
	createSucceededPayment(t, fix, tOrd.orderID, 3000)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("test return"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 2,
		DamagedQuantity:  1,
	}))
	ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Refund all 3 units
	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	now := time.Now()
	// Split slice 1: q1 damaged / ZAMK
	slice1, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:      allocs[0].ID,
		Quantity:          1,
		Status:            returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:           &empID,
		DecidedAt:         &now,
		LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
	})
	require.NoError(t, err)

	// Verify positive compensation exists for slice 1
	comps := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, comps, 1)
	assert.Equal(t, int64(250000), comps[0])
	assert.Equal(t, int64(250000), getNetCompensation(t, fix, tOrd.orderItemID))

	// Now correct same slice 1 through real service to: legacy_disposition = accepted, customer_change_of_mind
	// Capacities allow: accepted capacity = 2, currently 0 used, after correction 1 used <= 2.
	now2 := time.Now().Add(1 * time.Second)
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:      slice1.ID,
		Quantity:          1,
		Status:            returns.ReturnResponsibilityStatusNotRequired,
		ReasonCode:        strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:         &now2,
		LegacyDisposition: strPtr(returns.LegacyDispositionAccepted),
	})
	require.NoError(t, err)

	// Expected existing B.2B trigger automatically reconciles negative return_compensation_correction!
	// Historical positive row remains! No direct financial invocation!
	compsAfter := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, compsAfter, 2)
	assert.Equal(t, int64(250000), compsAfter[0], "Historical positive compensation row remains immutable")
	assert.Equal(t, int64(-250000), compsAfter[1], "Negative return_compensation_correction automatically appended")
	assert.Equal(t, int64(0), getNetCompensation(t, fix, tOrd.orderItemID))
}
