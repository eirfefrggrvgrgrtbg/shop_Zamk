package returns_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

func strPtr(s string) *string { return &s }

// ── 1. Claim creation must NOT create ReturnShipment ──────────────────────────

func TestM533A_ClaimCreation_ZeroShipments(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: func() *string { s := "Arrived damaged."; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	var count int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_shipments WHERE return_id = $1", resp[0].Return.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "CreateReturn must NOT insert return_shipments")
}

// ── 2. Logistics eligibility enforcement ──────────────────────────────────────

func TestM533A_Eligibility_Requested(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, resp[0].Return.ID, returns.CreateReturnShipmentRequest{
		Method: "cdek_office", CDEKOfficeCode: func() *string { s := "MSK1"; return &s }(),
	})
	assert.ErrorIs(t, err, returns.ErrReturnNotApproved)
}

func TestM533A_Eligibility_Rejected(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "rejected", AdminComment: func() *string { s := "nope"; return &s }(),
	}))
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, returns.CreateReturnShipmentRequest{
		Method: "cdek_office", CDEKOfficeCode: func() *string { s := "MSK1"; return &s }(),
	})
	assert.ErrorIs(t, err, returns.ErrReturnNotApproved)
}

func TestM533A_Eligibility_OtherCustomer(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	otherID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1,'O','+70001112233',$2,'hash')", otherID, "o_"+uuid.New().String()+"@t.com")
	require.NoError(t, err)
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, otherID, retID, returns.CreateReturnShipmentRequest{
		Method: "cdek_office", CDEKOfficeCode: func() *string { s := "MSK1"; return &s }(),
	})
	assert.ErrorIs(t, err, returns.ErrUnauthorized)
}

func TestM533A_Eligibility_DuplicateBlocked(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	req := returns.CreateReturnShipmentRequest{Method: "cdek_office", CDEKOfficeCode: func() *string { s := "MSK1"; return &s }()}
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, req)
	require.NoError(t, err)
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, req)
	assert.ErrorIs(t, err, returns.ErrShipmentAlreadyExists)
}

// ── 3. Validation ──────────────────────────────────────────────────────────────

func TestM533A_CDEKOffice_CodeRequired(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, returns.CreateReturnShipmentRequest{Method: "cdek_office"})
	assert.ErrorIs(t, err, returns.ErrCDEKOfficeRequired)
}

func TestM533A_CDEKOffice_UnknownOfficeRejected(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

	// Fake provider returns MSK1, MSK2. Client submits UNKNOWN_OFFICE
	unknownOffice := "UNKNOWN_OFFICE_99"
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, returns.CreateReturnShipmentRequest{
		Method:         "cdek_office",
		CDEKOfficeCode: &unknownOffice,
	})
	assert.ErrorIs(t, err, returns.ErrInvalidCDEKOffice, "unknown office code must be rejected")
}

func TestM533A_CDEKCourier_InfoRequired(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, returns.CreateReturnShipmentRequest{Method: "cdek_courier"})
	assert.ErrorIs(t, err, returns.ErrCourierInfoRequired)
}

// ── 4. Warehouse gate full matrix ─────────────────────────────────────────────

func TestM533A_WarehouseGate_NoShipment(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	err = fix.svc.StartReceiving(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrReturnNotArrived)
}

func TestM533A_WarehouseGate_FullMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	nonArrivedStatuses := []string{"draft", "awaiting_handover", "handed_over", "in_transit", "cancelled"}

	for _, st := range nonArrivedStatuses {
		st := st
		t.Run("Status_"+st, func(t *testing.T) {
			tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
			evIDs := fix.createStagedEvidence(t, fix.userID, 2)
			resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
				Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
				Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
			})
			require.NoError(t, err)
			retID := resp[0].Return.ID
			require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

			var refundsBefore, unitsBefore int
			fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&refundsBefore)
			fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1", resp[0].Items[0].ID).Scan(&unitsBefore)

			_, err = fix.client.Pool.Exec(ctx,
				"INSERT INTO return_shipments (id, return_id, provider, method, status) VALUES ($1,$2,'cdek','cdek_office',$3)",
				uuid.New(), retID, st)
			require.NoError(t, err)

			// Gate check
			err = fix.svc.StartReceiving(ctx, retID)
			assert.ErrorIs(t, err, returns.ErrReturnNotArrived, "status "+st+" must block StartReceiving")

			// Check ZERO side-effects / mutations
			var refundsAfter, unitsAfter int
			fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&refundsAfter)
			fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1", resp[0].Items[0].ID).Scan(&unitsAfter)

			assert.Equal(t, refundsBefore, refundsAfter, "Failed gate must not mutate refunds")
			assert.Equal(t, unitsBefore, unitsAfter, "Failed gate must not mutate return item units")
		})
	}
}

func TestM533A_WarehouseGate_ArrivedAllows(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))
	fix.createArrivedReturnShipment(t, retID)
	assert.NoError(t, fix.svc.StartReceiving(ctx, retID))
}

// ── 5. State machine transitions ──────────────────────────────────────────────

func TestM533A_Shipment_StateProgressionAndValidation(t *testing.T) {
	// Canonical valid transitions
	assert.True(t, returns.IsValidShipmentTransition("draft", "awaiting_handover"))
	assert.True(t, returns.IsValidShipmentTransition("awaiting_handover", "handed_over"))
	assert.True(t, returns.IsValidShipmentTransition("handed_over", "in_transit"))
	assert.True(t, returns.IsValidShipmentTransition("in_transit", "arrived_at_zamk"))

	// Non-terminal cancellations
	assert.True(t, returns.IsValidShipmentTransition("draft", "cancelled"))
	assert.True(t, returns.IsValidShipmentTransition("awaiting_handover", "cancelled"))
	assert.True(t, returns.IsValidShipmentTransition("handed_over", "cancelled"))
	assert.True(t, returns.IsValidShipmentTransition("in_transit", "cancelled"))

	// Terminal states have no transitions
	assert.False(t, returns.IsValidShipmentTransition("arrived_at_zamk", "cancelled"))
	assert.False(t, returns.IsValidShipmentTransition("arrived_at_zamk", "draft"))
	assert.False(t, returns.IsValidShipmentTransition("cancelled", "in_transit"))

	// Regressions rejected
	assert.False(t, returns.IsValidShipmentTransition("in_transit", "draft"))
	assert.False(t, returns.IsValidShipmentTransition("in_transit", "awaiting_handover"))
	assert.False(t, returns.IsValidShipmentTransition("handed_over", "draft"))
}

func TestM533A_Shipment_UpdateStatusExecution(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

	// Create shipment (starts in awaiting_handover)
	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, returns.CreateReturnShipmentRequest{
		Method:         "cdek_office",
		CDEKOfficeCode: func() *string { s := "MSK1"; return &s }(),
	})
	require.NoError(t, err)

	// Step 1: awaiting_handover -> handed_over
	s, err := fix.svc.UpdateReturnShipmentStatus(ctx, retID, "handed_over")
	require.NoError(t, err)
	assert.Equal(t, "handed_over", s.Status)

	// Step 2: handed_over -> in_transit
	s, err = fix.svc.UpdateReturnShipmentStatus(ctx, retID, "in_transit")
	require.NoError(t, err)
	assert.Equal(t, "in_transit", s.Status)

	// Step 3: in_transit -> arrived_at_zamk
	s, err = fix.svc.UpdateReturnShipmentStatus(ctx, retID, "arrived_at_zamk")
	require.NoError(t, err)
	assert.Equal(t, "arrived_at_zamk", s.Status)

	// Illegal regression from arrived_at_zamk
	_, err = fix.svc.UpdateReturnShipmentStatus(ctx, retID, "draft")
	assert.ErrorIs(t, err, returns.ErrInvalidShipmentTransition)
}

// ── 6. Unconfigured provider behavior ─────────────────────────────────────────

func TestM533A_UnconfiguredProvider_ReturnsErrorAndNoPersist(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

	// Re-instantiate service with real unconfigured CDEK provider (empty credentials)
	invSvc := inventory.NewService(nil, nil, fix.client)
	payRepo := payments.NewRepository(fix.client.Pool)
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	paySvc := payments.NewService(payRepo, fix.ordersRepo, nil, nil, fix.client, nil, cfg)
	payoutRepo := payouts.NewRepository(fix.client.Pool)
	payoutSvc := payouts.NewService(payoutRepo, fix.client, fix.returnsRepo, fix.ordersRepo, cfg, fix.notifSvc)

	unconfiguredSvc := returns.NewService(fix.returnsRepo, fix.ordersRepo, invSvc, fix.client, payoutSvc, paySvc, 14, fix.notifSvc, nil, returns.NewCDEKProvider(config.CDEKConfig{}))

	_, err = unconfiguredSvc.CreateCustomerReturnShipment(ctx, fix.userID, retID, returns.CreateReturnShipmentRequest{
		Method:         "cdek_office",
		CDEKOfficeCode: func() *string { s := "MSK1"; return &s }(),
	})
	assert.ErrorIs(t, err, returns.ErrCDEKNotConfigured)

	// Verify ZERO shipment rows persisted
	var count int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_shipments WHERE return_id = $1", retID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Must persist ZERO return_shipment rows when unconfigured")
}

// ── 7. No side effects from shipment creation ──────────────────────────────────

func TestM533A_Shipment_NoRefundSideEffects(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason: "damaged", Comment: func() *string { s := "x"; return &s }(),
		Items: []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

	var before int
	fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&before)

	_, err = fix.svc.CreateCustomerReturnShipment(ctx, fix.userID, retID, returns.CreateReturnShipmentRequest{
		Method: "cdek_office", CDEKOfficeCode: func() *string { s := "MSK1"; return &s }(),
	})
	require.NoError(t, err)

	var after int
	fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&after)
	assert.Equal(t, before, after, "shipment creation must not create refunds")
}

func TestM533A_CustomerReturns_ReadModel_EnrichedWithProductAndOrder(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: func() *string { s := "Item damaged on arrival"; return &s }(),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	retID := resp[0].Return.ID

	// Test GetCustomerReturn
	custRet, err := fix.svc.GetCustomerReturn(ctx, fix.userID, retID)
	require.NoError(t, err)
	require.NotNil(t, custRet)
	assert.Equal(t, retID, custRet.ID)
	require.NotNil(t, custRet.OrderNumber)
	assert.NotEmpty(t, *custRet.OrderNumber)
	require.Len(t, custRet.Items, 1)
	assert.NotEmpty(t, custRet.Items[0].ProductTitle)
	assert.Equal(t, 1, custRet.Items[0].Quantity)

	// Test ListCustomerReturns
	list, total, err := fix.svc.ListCustomerReturns(ctx, fix.userID, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)
	require.NotEmpty(t, list)
	var found *returns.ReturnResponse
	for i := range list {
		if list[i].ID == retID {
			found = &list[i]
			break
		}
	}
	require.NotNil(t, found, "created return must be present in customer return list")
	require.NotNil(t, found.OrderNumber)
	assert.NotEmpty(t, *found.OrderNumber)
	require.Len(t, found.Items, 1)
	assert.NotEmpty(t, found.Items[0].ProductTitle)
}

func insertTestReturnShipment(t *testing.T, fix *m51Fixture, returnID uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	shID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, status, created_at, updated_at)
		VALUES ($1, $2, 'cdek', 'cdek_office', $3, $4, $4)
	`, shID, returnID, status, createdAt)
	require.NoError(t, err)
	return shID
}

func TestM532_AdminReturnLogisticsVisibility(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	now := time.Now()

	// ── Case A: No shipment (shipmentStatus == nil) ──
	tOrdA := fix.createDeliveredOrder(t, now.Add(-1*time.Hour), 1)
	evIDsA := fix.createStagedEvidence(t, fix.userID, 2)
	respA, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdA.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Case A no shipment"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdA.orderItemID, Quantity: 1, EvidenceIDs: evIDsA}},
	})
	require.NoError(t, err)
	retIDA := respA[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDA, returns.UpdateReturnStatusRequest{Status: "approved"}))

	// ── Case B: awaiting_handover ──
	tOrdB := fix.createDeliveredOrder(t, now.Add(-1*time.Hour), 1)
	evIDsB := fix.createStagedEvidence(t, fix.userID, 2)
	respB, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdB.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("Case B awaiting_handover"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdB.orderItemID, Quantity: 1, EvidenceIDs: evIDsB}},
	})
	require.NoError(t, err)
	retIDB := respB[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDB, returns.UpdateReturnStatusRequest{Status: "approved"}))
	insertTestReturnShipment(t, fix, retIDB, "awaiting_handover", now.Add(-30*time.Minute))

	// ── Case C: arrived_at_zamk ──
	tOrdC := fix.createDeliveredOrder(t, now.Add(-2*time.Hour), 1)
	evIDsC := fix.createStagedEvidence(t, fix.userID, 2)
	respC, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdC.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Case C arrived_at_zamk"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdC.orderItemID, Quantity: 1, EvidenceIDs: evIDsC}},
	})
	require.NoError(t, err)
	retIDC := respC[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDC, returns.UpdateReturnStatusRequest{Status: "approved"}))
	insertTestReturnShipment(t, fix, retIDC, "arrived_at_zamk", now.Add(-20*time.Minute))

	// ── Case D: cancelled only ──
	tOrdD := fix.createDeliveredOrder(t, now.Add(-1*time.Hour), 1)
	evIDsD := fix.createStagedEvidence(t, fix.userID, 2)
	respD, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdD.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Case D cancelled only"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdD.orderItemID, Quantity: 1, EvidenceIDs: evIDsD}},
	})
	require.NoError(t, err)
	retIDD := respD[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDD, returns.UpdateReturnStatusRequest{Status: "approved"}))
	insertTestReturnShipment(t, fix, retIDD, "cancelled", now.Add(-40*time.Minute))

	// ── Case E: older cancelled + newer in_transit (in_transit must win) ──
	tOrdE := fix.createDeliveredOrder(t, now.Add(-1*time.Hour), 1)
	evIDsE := fix.createStagedEvidence(t, fix.userID, 2)
	respE, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdE.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Case E older cancelled + newer in_transit"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdE.orderItemID, Quantity: 1, EvidenceIDs: evIDsE}},
	})
	require.NoError(t, err)
	retIDE := respE[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDE, returns.UpdateReturnStatusRequest{Status: "approved"}))
	// Insert older cancelled shipment
	insertTestReturnShipment(t, fix, retIDE, "cancelled", now.Add(-50*time.Minute))
	// Insert newer in_transit shipment
	insertTestReturnShipment(t, fix, retIDE, "in_transit", now.Add(-10*time.Minute))

	// ── Case F: older arrived (cancelled) + newer arrived_at_zamk ──
	tOrdF := fix.createDeliveredOrder(t, now.Add(-1*time.Hour), 1)
	evIDsF := fix.createStagedEvidence(t, fix.userID, 2)
	respF, err := fix.svc.CreateReturn(ctx, fix.userID, tOrdF.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Case F re-issued shipment"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrdF.orderItemID, Quantity: 1, EvidenceIDs: evIDsF}},
	})
	require.NoError(t, err)
	retIDF := respF[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDF, returns.UpdateReturnStatusRequest{Status: "approved"}))
	insertTestReturnShipment(t, fix, retIDF, "cancelled", now.Add(-60*time.Minute))
	insertTestReturnShipment(t, fix, retIDF, "arrived_at_zamk", now.Add(-5*time.Minute))

	// ── Verify GetAdminReturn for all cases ──
	detA, err := fix.svc.GetAdminReturn(ctx, retIDA)
	require.NoError(t, err)
	assert.Nil(t, detA.ShipmentStatus, "Case A: must be nil")

	detB, err := fix.svc.GetAdminReturn(ctx, retIDB)
	require.NoError(t, err)
	require.NotNil(t, detB.ShipmentStatus)
	assert.Equal(t, "awaiting_handover", *detB.ShipmentStatus, "Case B: awaiting_handover")

	detC, err := fix.svc.GetAdminReturn(ctx, retIDC)
	require.NoError(t, err)
	require.NotNil(t, detC.ShipmentStatus)
	assert.Equal(t, "arrived_at_zamk", *detC.ShipmentStatus, "Case C: arrived_at_zamk")

	detD, err := fix.svc.GetAdminReturn(ctx, retIDD)
	require.NoError(t, err)
	require.NotNil(t, detD.ShipmentStatus)
	assert.Equal(t, "cancelled", *detD.ShipmentStatus, "Case D: cancelled")

	detE, err := fix.svc.GetAdminReturn(ctx, retIDE)
	require.NoError(t, err)
	require.NotNil(t, detE.ShipmentStatus)
	assert.Equal(t, "in_transit", *detE.ShipmentStatus, "Case E: in_transit active wins over older cancelled")

	detF, err := fix.svc.GetAdminReturn(ctx, retIDF)
	require.NoError(t, err)
	require.NotNil(t, detF.ShipmentStatus)
	assert.Equal(t, "arrived_at_zamk", *detF.ShipmentStatus, "Case F: arrived_at_zamk active wins over older cancelled")

	// ── Verify ListAdminReturns for all cases ──
	list, _, err := fix.svc.ListAdminReturns(ctx, 20, 0, false)
	require.NoError(t, err)

	statusMap := make(map[uuid.UUID]*string)
	for i := range list {
		statusMap[list[i].ID] = list[i].ShipmentStatus
	}

	assert.Nil(t, statusMap[retIDA], "List Case A: must be nil")
	require.NotNil(t, statusMap[retIDB], "List Case B: must not be nil")
	assert.Equal(t, "awaiting_handover", *statusMap[retIDB], "List Case B: awaiting_handover")
	require.NotNil(t, statusMap[retIDC], "List Case C: must not be nil")
	assert.Equal(t, "arrived_at_zamk", *statusMap[retIDC], "List Case C: arrived_at_zamk")
	require.NotNil(t, statusMap[retIDD], "List Case D: must not be nil")
	assert.Equal(t, "cancelled", *statusMap[retIDD], "List Case D: cancelled")
	require.NotNil(t, statusMap[retIDE], "List Case E: must not be nil")
	assert.Equal(t, "in_transit", *statusMap[retIDE], "List Case E: in_transit")
	require.NotNil(t, statusMap[retIDF], "List Case F: must not be nil")
	assert.Equal(t, "arrived_at_zamk", *statusMap[retIDF], "List Case F: arrived_at_zamk")
}
