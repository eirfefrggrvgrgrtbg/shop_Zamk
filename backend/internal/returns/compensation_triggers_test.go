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

func createSellerEarning(t *testing.T, fix *m51Fixture, orderID, orderItemID, sellerID uuid.UUID, earningCents int64) {
	t.Helper()
	ctx := context.Background()
	availAt := time.Now().Add(14 * 24 * time.Hour)
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', $5, 'RUB', $6, '{}'::jsonb, now())
	`, uuid.New(), sellerID, orderID, orderItemID, earningCents, availAt)
	require.NoError(t, err)
}

func getCompensationEntries(t *testing.T, fix *m51Fixture, orderItemID uuid.UUID) []int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := fix.client.Pool.Query(ctx, `
		SELECT amount_cents
		FROM seller_ledger_entries
		WHERE order_item_id = $1
		  AND type = 'adjustment'
		  AND metadata->>'reason' IN ('return_compensation', 'return_compensation_correction')
		ORDER BY created_at ASC, id ASC
	`, orderItemID)
	require.NoError(t, err)
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var a int64
		require.NoError(t, rows.Scan(&a))
		res = append(res, a)
	}
	return res
}

func getNetCompensation(t *testing.T, fix *m51Fixture, orderItemID uuid.UUID) int64 {
	entries := getCompensationEntries(t, fix, orderItemID)
	var sum int64
	for _, a := range entries {
		sum += a
	}
	return sum
}

func getSaleReversalEntries(t *testing.T, fix *m51Fixture, orderItemID uuid.UUID) []int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := fix.client.Pool.Query(ctx, `
		SELECT amount_cents
		FROM seller_ledger_entries
		WHERE order_item_id = $1
		  AND type = 'adjustment'
		  AND metadata->>'reason' IN ('return_deduction', 'return_post_payout')
		ORDER BY created_at ASC, id ASC
	`, orderItemID)
	require.NoError(t, err)
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var a int64
		require.NoError(t, rows.Scan(&a))
		res = append(res, a)
	}
	return res
}

func ensureInventoryItem(t *testing.T, fix *m51Fixture, pvID, prodID, sellerID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, seller_id, product_id, product_variant_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)
		ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = inventory_items.total_stock + EXCLUDED.total_stock
	`, uuid.New(), sellerID, prodID, pvID)
	require.NoError(t, err)
}

// ----------------------------------------------------------------------------
// 1. Production Flow Test A: Responsibility first, Refund second (Section 11)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_ProductionFlowA_ResponsibilityFirst_RefundSecond(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// 1. Historical order: Q=1, E=700000
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	empID := createEmployeeUser(t, fix)

	// 2. Create return
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	// 3. Arrive and receive at warehouse
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// 4. Resolve responsibility FIRST: zamk / zamk_warehouse_damage
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

	// Before refund: R=0, so compensation must be 0!
	assert.Len(t, getCompensationEntries(t, fix, tOrd.orderItemID), 0)

	// 5. Execute REAL production refund-success path
	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	// Expected automatically: Sale Reversal = -700000, Compensation = +700000
	reversals := getSaleReversalEntries(t, fix, tOrd.orderItemID)
	require.Len(t, reversals, 1)
	assert.Equal(t, int64(-700000), reversals[0])

	comps := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, comps, 1)
	assert.Equal(t, int64(700000), comps[0])
	assert.Equal(t, int64(700000), getNetCompensation(t, fix, tOrd.orderItemID))
}

// ----------------------------------------------------------------------------
// 2. Production Flow Test B: Refund first, Responsibility second (Section 12)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_ProductionFlowB_RefundFirst_ResponsibilitySecond(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// 1. Historical order: Q=1, E=700000
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	empID := createEmployeeUser(t, fix)

	// 2. Create return
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	// 3. Arrive and receive at warehouse
	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// 4. Responsibility remains PENDING. Execute refund-success path FIRST.
	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	// Sale Reversal exists, but compensation is 0 because responsibility is still pending!
	reversals := getSaleReversalEntries(t, fix, tOrd.orderItemID)
	require.Len(t, reversals, 1)
	assert.Equal(t, int64(-700000), reversals[0])
	assert.Len(t, getCompensationEntries(t, fix, tOrd.orderItemID), 0)

	// 5. Now resolve responsibility: SetReturnResponsibilityAllocation -> ZAMK
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

	// Automatically compensated without manual invocation!
	comps := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, comps, 1)
	assert.Equal(t, int64(700000), comps[0])
	assert.Equal(t, int64(700000), getNetCompensation(t, fix, tOrd.orderItemID))
}

// ----------------------------------------------------------------------------
// 3. Production Flow Test C: Correction after compensation (Section 13)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_ProductionFlowC_CorrectionAfterCompensation(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// 1. Historical order: Q=1, E=700000
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	empID := createEmployeeUser(t, fix)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Resolve to ZAMK
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	now1 := time.Now()
	alloc, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
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

	// Refund
	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	assert.Equal(t, int64(700000), getNetCompensation(t, fix, tOrd.orderItemID))

	// Step 1: Real responsibility mutation to seller_product_defect / seller
	now2 := time.Now()
	alloc, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:      alloc.ID,
		Quantity:          1,
		Status:            returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:  strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:        strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:           &empID,
		DecidedAt:         &now2,
		LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
	})
	require.NoError(t, err)

	comps := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, comps, 2)
	assert.Equal(t, int64(700000), comps[0])
	assert.Equal(t, int64(-700000), comps[1]) // negative correction
	assert.Equal(t, int64(0), getNetCompensation(t, fix, tOrd.orderItemID))

	// Step 2: Change back to ZAMK
	now3 := time.Now()
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:      alloc.ID,
		Quantity:          1,
		Status:            returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:  strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:        strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:           &empID,
		DecidedAt:         &now3,
		LegacyDisposition: strPtr(returns.LegacyDispositionDamaged),
	})
	require.NoError(t, err)

	compsAfter := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, compsAfter, 3)
	assert.Equal(t, int64(700000), compsAfter[0])
	assert.Equal(t, int64(-700000), compsAfter[1])
	assert.Equal(t, int64(700000), compsAfter[2])
	assert.Equal(t, int64(700000), getNetCompensation(t, fix, tOrd.orderItemID))
}

// ----------------------------------------------------------------------------
// 4. Restocked ZAMK Error Test (Section 14)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_RestockedZamkError_NoCompensation(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	empID := createEmployeeUser(t, fix)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	// Item is restocked / accepted (saleable), 0 damaged
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 1,
		DamagedQuantity:  0,
	}))
	ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Responsibility: zamk / zamk_fulfillment_error
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	now := time.Now()
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocs[0].ID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonZamkFulfillmentError),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)

	// Refund succeeds
	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	// Sale Reversal exists, but NO compensation (restocked != loss)
	reversals := getSaleReversalEntries(t, fix, tOrd.orderItemID)
	require.Len(t, reversals, 1)
	assert.Len(t, getCompensationEntries(t, fix, tOrd.orderItemID), 0)
}

// ----------------------------------------------------------------------------
// 5. Multi-Item Return Test (Section 15)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_MultiItemReturn_DeterministicOrder_NoLeakage(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Create order with 2 items
	orderID := uuid.New()
	fID := uuid.New()
	shipmentID := uuid.New()
	oi1 := uuid.New()
	oi2 := uuid.New()

	orderNum := fmt.Sprintf("ORD-%s", uuid.New().String()[:12])
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 2000, 'RUB', 'Test Address', 'Courier', 0, 'Test User', 'test@example.com', '+79990001122', now(), now())
	`, orderID, fix.userID, orderNum)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item A', 'slug-a', 1000, 1000, 1),
		       ($7, $2, $3, $4, $5, $6, 'Item B', 'slug-b', 1000, 1000, 1)
	`, oi1, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID, oi2)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '1 day', now())
	`, shipmentID, orderID, fID)
	require.NoError(t, err)

	createSellerEarning(t, fix, orderID, oi1, fix.sellerAID, 700000)
	createSellerEarning(t, fix, orderID, oi2, fix.sellerAID, 500000)
	createSucceededPayment(t, fix, orderID, 2000)
	empID := createEmployeeUser(t, fix)
	evIDs := fix.createStagedEvidence(t, fix.userID, 4)

	// Create return with both items
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oi1, Quantity: 1, EvidenceIDs: evIDs[:2]},
			{OrderItemID: oi2, Quantity: 1, EvidenceIDs: evIDs[2:]},
		},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	// Map return items
	var retItemA, retItemB uuid.UUID
	for _, it := range resp[0].Items {
		if it.OrderItemID == oi1 {
			retItemA = it.ID
		} else if it.OrderItemID == oi2 {
			retItemB = it.ID
		}
	}

	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	// Item A: damaged
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemA, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	// Item B: restocked (accepted)
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemB, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 1,
	}))
	ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Responsibility: Item A -> ZAMK, Item B -> customer_change_of_mind (or not_required)
	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	now := time.Now()
	for _, a := range allocs {
		if a.ReturnItemID == retItemA {
			_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
				AllocationID:      a.ID,
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
		} else {
			_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
				AllocationID:      a.ID,
				Quantity:          1,
				Status:            returns.ReturnResponsibilityStatusNotRequired,
				ReasonCode:        strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
				DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
				DecidedAt:         &now,
				LegacyDisposition: strPtr(returns.LegacyDispositionAccepted),
			})
			require.NoError(t, err)
		}
	}

	// Refund succeeds for both
	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	// Item A: Sale Reversal = -700000, Compensation = +700000
	require.Len(t, getSaleReversalEntries(t, fix, oi1), 1)
	assert.Equal(t, int64(700000), getNetCompensation(t, fix, oi1))

	// Item B: Sale Reversal = -500000, Compensation = 0
	require.Len(t, getSaleReversalEntries(t, fix, oi2), 1)
	assert.Equal(t, int64(0), getNetCompensation(t, fix, oi2))
}

// ----------------------------------------------------------------------------
// 6. Partial / Rounding Production Test (Section 16)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_PartialRoundingProduction(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Q=3, E=700000
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 3000)
	empID := createEmployeeUser(t, fix)

	processReturn := func(isZamkLoss bool) {
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason: "defective",
		Comment: strPtr("item is defective"),
			Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
		})
		require.NoError(t, err)
		retID := resp[0].Return.ID
		retItemID := resp[0].Items[0].ID

		require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
			Status: "approved",
		}))

		fix.createArrivedReturnShipment(t, retID)
		require.NoError(t, fix.svc.StartReceiving(ctx, retID))
		if isZamkLoss {
			require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
				DamagedQuantity: 1,
			}))
		} else {
			require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
				AcceptedQuantity: 1,
			}))
		}
		ensureInventoryItem(t, fix, fix.varAID, fix.prodAID, fix.sellerAID)
		require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

		now := time.Now()
		allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
		require.NoError(t, err)
		if isZamkLoss {
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
		} else {
			_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
				AllocationID:      allocs[0].ID,
				Quantity:          1,
				Status:            returns.ReturnResponsibilityStatusNotRequired,
				ReasonCode:        strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
				DecisionSource:    strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
				DecidedAt:         &now,
				LegacyDisposition: strPtr(returns.LegacyDispositionAccepted),
			})
			require.NoError(t, err)
		}

		ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
		require.NoError(t, err)
		require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))
	}

	// 1. q1 ordinary
	processReturn(false)
	assert.Equal(t, int64(0), getNetCompensation(t, fix, tOrd.orderItemID))

	// 2. q1 zamk loss
	processReturn(true)
	assert.Equal(t, int64(233333), getNetCompensation(t, fix, tOrd.orderItemID))

	// 3. q1 zamk loss
	processReturn(true)
	assert.Equal(t, int64(466666), getNetCompensation(t, fix, tOrd.orderItemID))

	// Reversals sum: 233333 + 233333 + 233334 = 700000
	revs := getSaleReversalEntries(t, fix, tOrd.orderItemID)
	var revSum int64
	for _, r := range revs {
		revSum += r
	}
	assert.Equal(t, int64(-700000), revSum)
}

// ----------------------------------------------------------------------------
// 7. Refund Retry Idempotency (Section 5)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_RefundRetryIdempotency(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	empID := createEmployeeUser(t, fix)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
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

	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	// Call ProcessRefundSuccess first time
	nowTime := time.Now()
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, nowTime))

	comps1 := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, comps1, 1)
	assert.Equal(t, int64(700000), comps1[0])

	// Call ProcessRefundSuccess second time (retry callback)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, nowTime))

	comps2 := getCompensationEntries(t, fix, tOrd.orderItemID)
	require.Len(t, comps2, 1, "Retry must not create duplicate compensation rows")
	assert.Equal(t, int64(700000), comps2[0])

	revs := getSaleReversalEntries(t, fix, tOrd.orderItemID)
	require.Len(t, revs, 1, "Retry must not duplicate Sale Reversal")
}

// ----------------------------------------------------------------------------
// 8. Concurrent Refund vs Responsibility Test (Section 17)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_ConcurrentRefundVsResponsibility(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	empID := createEmployeeUser(t, fix)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	targetAllocID := allocs[0].ID

	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	// Run concurrently:
	// A: ProcessRefundSuccess
	// B: SetReturnResponsibilityAllocation
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		errs[0] = fix.svc.ProcessRefundSuccess(context.Background(), ref.ID, time.Now())
	}()

	go func() {
		defer wg.Done()
		now := time.Now()
		_, errs[1] = fix.svc.SetReturnResponsibilityAllocation(context.Background(), returns.SetReturnResponsibilityAllocationRequest{
			AllocationID:      targetAllocID,
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

	// Assert no errors / no deadlocks
	require.NoError(t, errs[0], "Refund processing error")
	require.NoError(t, errs[1], "Responsibility allocation error")

	// Verify exact convergence:
	// Sale reversal: exactly once (-700000)
	revs := getSaleReversalEntries(t, fix, tOrd.orderItemID)
	require.Len(t, revs, 1)
	assert.Equal(t, int64(-700000), revs[0])

	// Net compensation: exact target (+700000)
	assert.Equal(t, int64(700000), getNetCompensation(t, fix, tOrd.orderItemID))
}

// ----------------------------------------------------------------------------
// 9. Failure Atomicity Test (Section 10)
// ----------------------------------------------------------------------------
func TestCompensation_Trigger_FailureAtomicity_Rollback(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	empID := createEmployeeUser(t, fix)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "defective",
		Comment: strPtr("item is defective"),
		Items:  []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)

	ref, err := fix.svc.CreateRefund(ctx, empID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	// Inject a real PostgreSQL trigger on seller_ledger_entries that fails compensation insertion
	_, err = fix.client.Pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION fail_compensation_test() RETURNS TRIGGER AS $$
		BEGIN
			IF NEW.metadata->>'reason' = 'return_compensation' THEN
				RAISE EXCEPTION 'forced compensation failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS trg_fail_compensation_test ON seller_ledger_entries;
		CREATE TRIGGER trg_fail_compensation_test
		BEFORE INSERT ON seller_ledger_entries
		FOR EACH ROW EXECUTE FUNCTION fail_compensation_test();
	`)
	require.NoError(t, err)

	defer func() {
		_, _ = fix.client.Pool.Exec(context.Background(), `
			DROP TRIGGER IF EXISTS trg_fail_compensation_test ON seller_ledger_entries;
			DROP FUNCTION IF EXISTS fail_compensation_test();
		`)
	}()

	// 1. First resolve responsibility to ZAMK (before refund, compensation delta is 0 so this succeeds)
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

	// 2. Now call ProcessRefundSuccess:
	// ProcessReturnDeduction will create the deduction, then ReconcileReturnCompensationTx will try to insert compensation,
	// which will trigger 'forced compensation failure'!
	err = fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now())
	require.Error(t, err, "Expected ProcessRefundSuccess to fail due to compensation error")
	assert.Contains(t, err.Error(), "forced compensation failure")

	// 3. PROVE FAILURE ATOMICITY:
	// Whole transaction must have rolled back:
	// - Refund status must STILL be 'pending' (NOT 'succeeded')
	var refStatus string
	err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM refunds WHERE id = $1", ref.ID).Scan(&refStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending", refStatus, "Refund must not be marked succeeded when compensation fails")

	// - Return status must STILL be 'item_received' (NOT 'refunded')
	retRow, _, err := fix.returnsRepo.GetReturn(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", retRow.Status, "Return must remain item_received when refund transaction rolls back")

	// - Zero Sale Reversal deduction rows committed
	assert.Len(t, getSaleReversalEntries(t, fix, tOrd.orderItemID), 0, "Sale Reversal must roll back on compensation error")

	// - Zero Compensation rows committed
	assert.Len(t, getCompensationEntries(t, fix, tOrd.orderItemID), 0, "No half-state compensation rows")
}
