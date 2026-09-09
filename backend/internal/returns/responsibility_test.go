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

func createEmployeeUser(t *testing.T, fix *m51Fixture) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	empID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO users (id, name, phone, email, password_hash)
		VALUES ($1, 'ZAMK Staff', $2, $3, 'hash')
	`, empID, "+7999"+uuid.New().String()[:7], "emp_"+uuid.New().String()+"@test.com")
	require.NoError(t, err)
	return empID
}



// Case A. Canonical CreateReturn q=3: one pending allocation q=3.
func TestCaseA_CanonicalCreateReturn_OnePendingAllocationQ3(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Item is broken"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	assert.Equal(t, 3, allocs[0].Quantity)
	assert.Equal(t, returns.ReturnResponsibilityStatusPending, allocs[0].Status)
	assert.Nil(t, allocs[0].OrderItemAllocationID)

	hist, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocs[0].ID)
	require.NoError(t, err)
	require.Len(t, hist, 1)
	assert.Equal(t, 3, hist[0].Quantity)
	assert.Equal(t, returns.ReturnResponsibilityStatusPending, hist[0].Status)
	assert.Nil(t, hist[0].OrderItemAllocationID)
}

// Case B. Multi-item same Return:
// item1 = not_required/customer_change_of_mind
// item2 = seller/seller_product_defect
// Both coexist.
func TestCaseB_MultiItemSameReturn_Coexist(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	// Create order with 2 items in same fulfillment
	orderID := uuid.New()
	fID := uuid.New()
	shipmentID := uuid.New()
	oi1 := uuid.New()
	oi2 := uuid.New()

	orderNum := fmt.Sprintf("ORD-%s", uuid.New().String()[:12])
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 3000, 'RUB', 'Test Address', 'Courier', 0, 'Test User', 'test@example.com', '+79990001122', now(), now())
	`, orderID, fix.userID, orderNum)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES
			($1, $3, $4, $5, $6, $7, 'Product 1', 'slug-1', 1000, 1000, 1),
			($2, $3, $4, $5, $6, $7, 'Product 2', 'slug-2', 2000, 2000, 1)
	`, oi1, oi2, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '24 hour', now() - interval '1 hour')
	`, shipmentID, orderID, fID)
	require.NoError(t, err)

	evIDs1 := fix.createStagedEvidence(t, fix.userID, 2)
	evIDs2 := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Multi item return"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oi1, Quantity: 1, EvidenceIDs: evIDs1},
			{OrderItemID: oi2, Quantity: 1, EvidenceIDs: evIDs2},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allocs, 2)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// item1 = not_required/customer_change_of_mind
	alloc1, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:   allocs[0].ID,
		Quantity:       1,
		Status:         returns.ReturnResponsibilityStatusNotRequired,
		ReasonCode:     strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource: strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:      &now,
	})
	require.NoError(t, err)
	assert.Equal(t, returns.ReturnResponsibilityStatusNotRequired, alloc1.Status)

	// item2 = seller/seller_product_defect
	alloc2, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocs[1].ID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)
	assert.Equal(t, returns.ReturnResponsibilityStatusResolved, alloc2.Status)
	assert.Equal(t, returns.ReturnResponsiblePartySeller, *alloc2.ResponsibleParty)

	// Verify both coexist on the same return
	allAllocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allAllocs, 2)
	assert.Equal(t, returns.ReturnResponsibilityStatusNotRequired, allAllocs[0].Status)
	assert.Equal(t, returns.ReturnResponsibilityStatusResolved, allAllocs[1].Status)
}

// Case C. Same Return:
// item1 = zamk/zamk_warehouse_damage
// item2 = carrier/carrier_damage
func TestCaseC_SameReturn_ZamkAndCarrier(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	orderID := uuid.New()
	fID := uuid.New()
	shipmentID := uuid.New()
	oi1 := uuid.New()
	oi2 := uuid.New()

	orderNum := fmt.Sprintf("ORD-%s", uuid.New().String()[:12])
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 3000, 'RUB', 'Test Address', 'Courier', 0, 'Test User', 'test@example.com', '+79990001122', now(), now())
	`, orderID, fix.userID, orderNum)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES
			($1, $3, $4, $5, $6, $7, 'Product 1', 'slug-1', 1000, 1000, 1),
			($2, $3, $4, $5, $6, $7, 'Product 2', 'slug-2', 2000, 2000, 1)
	`, oi1, oi2, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '24 hour', now() - interval '1 hour')
	`, shipmentID, orderID, fID)
	require.NoError(t, err)

	evIDs1 := fix.createStagedEvidence(t, fix.userID, 2)
	evIDs2 := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Damaged items"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oi1, Quantity: 1, EvidenceIDs: evIDs1},
			{OrderItemID: oi2, Quantity: 1, EvidenceIDs: evIDs2},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allocs, 2)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// item1 = zamk/zamk_warehouse_damage
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocs[0].ID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)

	// item2 = carrier/carrier_damage
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocs[1].ID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartyCarrier),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonCarrierDamage),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)

	allAllocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allAllocs, 2)
	assert.Equal(t, returns.ReturnResponsiblePartyZamk, *allAllocs[0].ResponsibleParty)
	assert.Equal(t, returns.ReturnResponsiblePartyCarrier, *allAllocs[1].ResponsibleParty)
}

// Case D. Legacy q=3 split:
// q1 not_required
// q1 seller
// q1 zamk
// SUM current quantity == 3.
func TestCaseD_LegacyQ3Split(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Defects on multiple units"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	sourceAllocID := allocs[0].ID

	now := time.Now().UTC().Truncate(time.Microsecond)

	// 1. Split q1 not_required from source (source becomes q2)
	allocNotReq, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:   sourceAllocID,
		Quantity:       1,
		Status:         returns.ReturnResponsibilityStatusNotRequired,
		ReasonCode:     strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource: strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:      &now,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, allocNotReq.Quantity)
	assert.Equal(t, returns.ReturnResponsibilityStatusNotRequired, allocNotReq.Status)

	// 2. Split q1 seller from remaining source (source becomes q1)
	allocSeller, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     sourceAllocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, allocSeller.Quantity)
	assert.Equal(t, returns.ReturnResponsiblePartySeller, *allocSeller.ResponsibleParty)

	// 3. Resolve remaining q1 on source to zamk (full update)
	allocZamk, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     sourceAllocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, allocZamk.Quantity)
	assert.Equal(t, returns.ReturnResponsiblePartyZamk, *allocZamk.ResponsibleParty)

	// Verify exact coverage sum: 1 + 1 + 1 == 3
	allAllocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allAllocs, 3)

	sum := 0
	for _, a := range allAllocs {
		sum += a.Quantity
	}
	assert.Equal(t, 3, sum)
}

// Case E. Correction:
// existing q2 seller
// split into:
// q1 seller
// q1 zamk
// history preserves earlier q2 seller snapshot.
func TestCaseE_CorrectionPreservesHistory(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Defects on 2 units"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	allocID := allocs[0].ID

	t1 := time.Now().UTC().Truncate(time.Microsecond)

	// Resolve whole q2 to seller
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocID,
		Quantity:         2,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &t1,
	})
	require.NoError(t, err)

	// History before split must have: [q2 pending, q2 seller]
	histBefore, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocID)
	require.NoError(t, err)
	require.Len(t, histBefore, 2)
	assert.Equal(t, 2, histBefore[1].Quantity)
	assert.Equal(t, returns.ReturnResponsiblePartySeller, *histBefore[1].ResponsibleParty)

	// Now correct by splitting 1 to ZAMK
	t2 := time.Now().UTC().Truncate(time.Microsecond)
	allocZamk, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &t2,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, allocZamk.Quantity)
	assert.Equal(t, returns.ReturnResponsiblePartyZamk, *allocZamk.ResponsibleParty)

	// History of original alloc must preserve the earlier q2 seller snapshot and have the new q1 seller snapshot
	histAfter, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocID)
	require.NoError(t, err)
	require.Len(t, histAfter, 3)
	assert.Equal(t, 2, histAfter[0].Quantity)
	assert.Equal(t, returns.ReturnResponsibilityStatusPending, histAfter[0].Status)

	assert.Equal(t, 2, histAfter[1].Quantity)
	assert.Equal(t, returns.ReturnResponsiblePartySeller, *histAfter[1].ResponsibleParty)

	assert.Equal(t, 1, histAfter[2].Quantity)
	assert.Equal(t, returns.ReturnResponsiblePartySeller, *histAfter[2].ResponsibleParty)

	returnItemID := allocs[0].ReturnItemID
	for i, h := range histAfter {
		assert.Equal(t, returnItemID, h.ReturnItemID, "history event %d must have correct return_item_id", i)
		assert.Equal(t, allocID, h.AllocationID, "history event %d must have correct allocation_id", i)
	}

	// New zamk allocation history has its snapshot
	histZamk, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocZamk.ID)
	require.NoError(t, err)
	require.Len(t, histZamk, 1)
	assert.Equal(t, 1, histZamk[0].Quantity)
	assert.Equal(t, returns.ReturnResponsiblePartyZamk, *histZamk[0].ResponsibleParty)
	assert.Equal(t, returnItemID, histZamk[0].ReturnItemID, "zamk history event must have correct return_item_id")
	assert.Equal(t, allocZamk.ID, histZamk[0].AllocationID, "zamk history event must have correct allocation_id")

	// Section 4/5: Prove allocation deletion cannot erase history (ON DELETE RESTRICT)
	_, delErr := fix.client.Pool.Exec(ctx, "DELETE FROM return_responsibility_allocations WHERE id = $1", allocID)
	require.Error(t, delErr, "deleting allocation with history must be rejected by foreign key constraint")
	assert.Contains(t, delErr.Error(), "violates foreign key constraint")

	// Verify all history snapshots remain intact
	histStillExists, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocID)
	require.NoError(t, err)
	assert.Len(t, histStillExists, 3, "all history snapshots must remain intact")
}

// Case F. Serialized three-unit case:
// three original order_item_allocations.
// Known unit A -> seller
// known unit B -> zamk
// third unresolved/missing quantity remains represented.
func TestCaseF_SerializedThreeUnits(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	_, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 3)
	require.Len(t, allocIDs, 3)

	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Serialized 3 units"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	sourceAllocID := allocs[0].ID

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Unit A -> seller (split 1 with orderItemAllocationID = allocIDs[0])
	allocA, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          sourceAllocID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, allocA.Quantity)
	require.NotNil(t, allocA.OrderItemAllocationID)
	assert.Equal(t, allocIDs[0], *allocA.OrderItemAllocationID)
	assert.Equal(t, returns.ReturnResponsiblePartySeller, *allocA.ResponsibleParty)

	// Unit B -> zamk (split 1 with orderItemAllocationID = allocIDs[1] from remaining source)
	allocB, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          sourceAllocID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[1],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, allocB.Quantity)
	require.NotNil(t, allocB.OrderItemAllocationID)
	assert.Equal(t, allocIDs[1], *allocB.OrderItemAllocationID)
	assert.Equal(t, returns.ReturnResponsiblePartyZamk, *allocB.ResponsibleParty)

	// Third unresolved/missing quantity remains represented as unbound pending
	allAllocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allAllocs, 3)

	var unboundPendingCount int
	for _, a := range allAllocs {
		if a.OrderItemAllocationID == nil && a.Status == returns.ReturnResponsibilityStatusPending && a.Quantity == 1 {
			unboundPendingCount++
		}
	}
	assert.Equal(t, 1, unboundPendingCount, "Third unit remains represented as unbound pending")
}

// Case G. duplicate order_item_allocation_id -> REJECT
func TestCaseG_DuplicateOrderItemAllocationRejected(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	_, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 2)
	require.Len(t, allocIDs, 2)

	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test duplicate binding"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	sourceAllocID := allocs[0].ID

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Bind first unit to allocIDs[0]
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          sourceAllocID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)

	// Attempt to bind the remaining allocation to the SAME allocIDs[0] -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          sourceAllocID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartyZamk),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonZamkWarehouseDamage),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	assert.ErrorIs(t, err, returns.ErrAllocationAlreadyBound)

	// DB-level test: direct duplicate INSERT must trigger PostgreSQL UNIQUE violation
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_responsibility_allocations (id, return_item_id, order_item_allocation_id, quantity, status)
		VALUES (gen_random_uuid(), $1, $2, 1, 'pending')
	`, allocs[0].ReturnItemID, allocIDs[0])
	assert.Error(t, err, "Database unique constraint must reject duplicate order_item_allocation_id")
}

// Case H. bound order_item_allocation_id with quantity != 1 -> REJECT
func TestCaseH_BoundAllocationWithQuantityNotOneRejected(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	_, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 3)

	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test qty != 1 with binding"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Service rejection
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              2, // != 1
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:             &now,
	})
	assert.ErrorIs(t, err, returns.ErrInvalidQuantity)

	// DB check constraint test
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_responsibility_allocations (id, return_item_id, order_item_allocation_id, quantity, status)
		VALUES (gen_random_uuid(), $1, $2, 2, 'pending')
	`, allocs[0].ReturnItemID, allocIDs[1])
	assert.Error(t, err, "DB constraint chk_rra_serialized_qty must reject order_item_allocation_id with quantity != 1")
}

// Case I. order_item_allocation from wrong order_item -> REJECT
func TestCaseI_WrongOrderItemAllocationRejected(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd1 := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	tOrd2 := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)

	_, allocIDs2 := createReturnTestAllocations(t, fix, tOrd2.orderID, tOrd2.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 1)

	evIDs1 := fix.createStagedEvidence(t, fix.userID, 2)
	resp1, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd1.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Return 1"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd1.orderItemID, Quantity: 1, EvidenceIDs: evIDs1},
		},
	})
	require.NoError(t, err)

	allocs1, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp1[0].Return.ID)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Try to bind allocation from order 2 to return 1 -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs1[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs2[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	assert.ErrorIs(t, err, returns.ErrResponsibilityInvariants)
}

// Case J. service must never produce: SUM(current allocation.quantity) > return_item.quantity
func TestCaseJ_QuantityExceedingRejected(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test over-allocation"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Requesting quantity = 5 when source only has 2 -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:   allocs[0].ID,
		Quantity:       5,
		Status:         returns.ReturnResponsibilityStatusNotRequired,
		ReasonCode:     strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource: strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:      &now,
	})
	assert.ErrorIs(t, err, returns.ErrInvalidQuantity)
}

// Case K. split must preserve exact coverage: SUM == return_item.quantity
func TestCaseK_SplitPreservesExactCoverage(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test coverage invariant"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 5, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	sourceAllocID := allocs[0].ID

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Split 2
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:   sourceAllocID,
		Quantity:       2,
		Status:         returns.ReturnResponsibilityStatusNotRequired,
		ReasonCode:     strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource: strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:      &now,
	})
	require.NoError(t, err)

	// Split 1
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     sourceAllocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)

	allAllocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	total := 0
	for _, a := range allAllocs {
		total += a.Quantity
	}
	assert.Equal(t, 5, total, "Sum of all allocations must strictly equal return_item.quantity")
}

// Case L. same exact decision retry: ZERO new history rows, updated_at unchanged
func TestCaseL_SemanticIdempotency(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test idempotency"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	allocID := allocs[0].ID

	decidedAt := time.Now().UTC().Truncate(time.Microsecond)

	req := returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		InternalNote:     strPtr("First decision"),
		ActorID:          &empID,
		DecidedAt:        &decidedAt,
	}

	res1, err := fix.svc.SetReturnResponsibilityAllocation(ctx, req)
	require.NoError(t, err)

	hist1, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocID)
	require.NoError(t, err)
	require.Len(t, hist1, 2) // initial pending + resolved

	// Retry exact same request
	res2, err := fix.svc.SetReturnResponsibilityAllocation(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, res1.UpdatedAt.UnixNano(), res2.UpdatedAt.UnixNano(), "updated_at must remain unchanged")

	hist2, err := fix.svc.GetReturnResponsibilityAllocationHistory(ctx, allocID)
	require.NoError(t, err)
	assert.Len(t, hist2, len(hist1), "Retry of exact same request must produce ZERO new history rows")
}

// Case M. concurrent split attempts on same return_item:
// no lost quantity, no over-allocation, no duplicate physical allocation
func TestCaseM_ConcurrentSplitAttempts(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 10)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Concurrent split test"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 10, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	sourceAllocID := allocs[0].ID

	concurrency := 8
	type splitResult struct {
		alloc *returns.ReturnResponsibilityAllocation
		err   error
	}
	results := make(chan splitResult, concurrency)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			now := time.Now().UTC().Truncate(time.Microsecond)
			note := fmt.Sprintf("Concurrent split %d", idx)
			res, err := fix.svc.SetReturnResponsibilityAllocation(context.Background(), returns.SetReturnResponsibilityAllocationRequest{
				AllocationID:   sourceAllocID,
				Quantity:       1,
				Status:         returns.ReturnResponsibilityStatusNotRequired,
				ReasonCode:     strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
				DecisionSource: strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
				InternalNote:   &note,
				DecidedAt:      &now,
			})
			results <- splitResult{alloc: res, err: err}
		}(i)
	}
	wg.Wait()
	close(results)

	var successfulSplits []*returns.ReturnResponsibilityAllocation
	for r := range results {
		require.NoError(t, r.err, "all 8 concurrent goroutines must return nil error")
		require.NotNil(t, r.alloc, "successful split must return allocation")
		assert.Equal(t, 1, r.alloc.Quantity)
		assert.Equal(t, returns.ReturnResponsibilityStatusNotRequired, r.alloc.Status)
		successfulSplits = append(successfulSplits, r.alloc)
	}
	require.Len(t, successfulSplits, concurrency, "exactly 8 successful split operations occurred")

	allAllocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	require.Len(t, allAllocs, 9, "must have 8 split siblings + 1 remaining source allocation")

	seenIDs := make(map[uuid.UUID]bool)
	var q1DecidedCount int
	var q2PendingCount int
	totalQuantity := 0

	for _, a := range allAllocs {
		assert.False(t, seenIDs[a.ID], "no duplicate allocation IDs")
		seenIDs[a.ID] = true
		totalQuantity += a.Quantity

		if a.ID == sourceAllocID {
			assert.Equal(t, 2, a.Quantity, "source allocation must have remaining quantity = 2")
			assert.Equal(t, returns.ReturnResponsibilityStatusPending, a.Status, "source allocation remains pending")
			q2PendingCount++
		} else {
			assert.Equal(t, 1, a.Quantity, "sibling allocation must have quantity = 1")
			assert.Equal(t, returns.ReturnResponsibilityStatusNotRequired, a.Status, "sibling allocation must be decided")
			q1DecidedCount++
		}
	}

	assert.Equal(t, 8, q1DecidedCount, "exactly 8 decided q=1 sibling allocations")
	assert.Equal(t, 1, q2PendingCount, "exactly 1 remaining q=2 source allocation")
	assert.Equal(t, 10, totalQuantity, "SUM(quantity) must equal 10 with zero lost or created quantity")
}

// Case N. employee/system provenance rules
func TestCaseN_ProvenanceRules(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test provenance"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	allocID := allocs[0].ID

	now := time.Now().UTC().Truncate(time.Microsecond)

	// 1. Employee without actorID -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          nil, // missing
		DecidedAt:        &now,
	})
	assert.ErrorIs(t, err, returns.ErrResponsibilityActorRequired)

	// 2. System with actorID -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:   allocID,
		Quantity:       1,
		Status:         returns.ReturnResponsibilityStatusNotRequired,
		ReasonCode:     strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource: strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		ActorID:        &empID, // forbidden
		DecidedAt:      &now,
	})
	assert.ErrorIs(t, err, returns.ErrResponsibilityActorForbidden)
}

// Case O. reason/party compatibility
func TestCaseO_ReasonPartyCompatibility(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test compatibility"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	allocID := allocs[0].ID

	now := time.Now().UTC().Truncate(time.Microsecond)

	// seller_product_defect with responsible_party = zamk -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartyZamk), // mismatch
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	assert.ErrorIs(t, err, returns.ErrResponsibilityReasonPartyMismatch)

	// customer_change_of_mind with status = resolved -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     allocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved, // mismatch
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartyCustomer),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:        &now,
	})
	assert.ErrorIs(t, err, returns.ErrResponsibilityReasonPartyMismatch)
}

// Case P. responsibility operations: seller_ledger_entries count and sum unchanged
func TestCaseP_LedgerInertness(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	var countBefore int
	var sumBefore int64
	err := fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*), COALESCE(SUM(amount_cents), 0) FROM seller_ledger_entries").Scan(&countBefore, &sumBefore)
	require.NoError(t, err)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test ledger inertness"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 3, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)
	sourceAllocID := allocs[0].ID

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Perform splits and updates
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:   sourceAllocID,
		Quantity:       1,
		Status:         returns.ReturnResponsibilityStatusNotRequired,
		ReasonCode:     strPtr(returns.ReturnResponsibilityReasonCustomerChangeOfMind),
		DecisionSource: strPtr(returns.ReturnResponsibilityDecisionSourceSystem),
		DecidedAt:      &now,
	})
	require.NoError(t, err)

	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:     sourceAllocID,
		Quantity:         1,
		Status:           returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty: strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:       strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:   strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:          &empID,
		DecidedAt:        &now,
	})
	require.NoError(t, err)

	var countAfter int
	var sumAfter int64
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*), COALESCE(SUM(amount_cents), 0) FROM seller_ledger_entries").Scan(&countAfter, &sumAfter)
	require.NoError(t, err)

	assert.Equal(t, countBefore, countAfter, "seller_ledger_entries count must remain strictly unchanged")
	assert.Equal(t, sumBefore, sumAfter, "seller_ledger_entries sum must remain strictly unchanged")
}

// Case Q. Physical binding immutability
func TestCaseQ_PhysicalBindingImmutability(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	empID := createEmployeeUser(t, fix)

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	_, allocIDs := createReturnTestAllocations(t, fix, tOrd.orderID, tOrd.orderItemID, fix.varAID, fix.prodAID, fix.sellerAID, 2)
	require.Len(t, allocIDs, 2)

	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Test physical binding immutability"),
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: tOrd.orderItemID, Quantity: 2, EvidenceIDs: evIDs},
		},
	})
	require.NoError(t, err)

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, resp[0].Return.ID)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Bind unit A to new allocation via split
	allocA, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocs[0].ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0],
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)
	assert.Equal(t, allocIDs[0], *allocA.OrderItemAllocationID)

	// 1. Bound A + request NULL -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocA.ID,
		Quantity:              1,
		OrderItemAllocationID: nil, // attempted clear
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	assert.ErrorIs(t, err, returns.ErrPhysicalBindingImmutable)

	// 2. Bound A + request B -> REJECT
	_, err = fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocA.ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[1], // attempted change to B
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	assert.ErrorIs(t, err, returns.ErrPhysicalBindingImmutable)

	// 3. Bound A + request A -> ALLOWED (e.g. note or internal detail change)
	newNote := "Updated note"
	updatedA, err := fix.svc.SetReturnResponsibilityAllocation(ctx, returns.SetReturnResponsibilityAllocationRequest{
		AllocationID:          allocA.ID,
		Quantity:              1,
		OrderItemAllocationID: &allocIDs[0], // same unit A
		Status:                returns.ReturnResponsibilityStatusResolved,
		ResponsibleParty:      strPtr(returns.ReturnResponsiblePartySeller),
		ReasonCode:            strPtr(returns.ReturnResponsibilityReasonSellerProductDefect),
		DecisionSource:        strPtr(returns.ReturnResponsibilityDecisionSourceEmployee),
		InternalNote:          &newNote,
		ActorID:               &empID,
		DecidedAt:             &now,
	})
	require.NoError(t, err)
	assert.Equal(t, allocIDs[0], *updatedA.OrderItemAllocationID)
	assert.Equal(t, "Updated note", *updatedA.InternalNote)
}
