package returns_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

func TestM532_Simulator_DomainLifecycle(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// 1. Create an approved return
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Simulator test return"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	// 2. Cannot simulate shipment before return approval
	_, err = fix.svc.CreateSimulatedReturnShipment(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrReturnNotApproved)

	// Approve the return
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

	// 3. Create simulated shipment (initial state: awaiting_handover)
	sh, err := fix.svc.CreateSimulatedReturnShipment(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, sh)
	assert.Equal(t, "cdek", sh.Provider)
	assert.Equal(t, "cdek_office", sh.Method)
	assert.Equal(t, "awaiting_handover", sh.Status)

	// Check DB row directly
	det, err := fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, det.ShipmentStatus)
	assert.Equal(t, "awaiting_handover", *det.ShipmentStatus)

	// 4. Duplicate active shipment creation must be rejected
	_, err = fix.svc.CreateSimulatedReturnShipment(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrShipmentAlreadyExists)

	// 5. Advance awaiting_handover -> handed_over
	sh, err = fix.svc.AdvanceSimulatedReturnShipment(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "handed_over", sh.Status)

	det, err = fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, det.ShipmentStatus)
	assert.Equal(t, "handed_over", *det.ShipmentStatus)

	// 6. Advance handed_over -> in_transit
	sh, err = fix.svc.AdvanceSimulatedReturnShipment(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "in_transit", sh.Status)

	det, err = fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, det.ShipmentStatus)
	assert.Equal(t, "in_transit", *det.ShipmentStatus)

	// 7. Advance in_transit -> arrived_at_zamk
	sh, err = fix.svc.AdvanceSimulatedReturnShipment(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "arrived_at_zamk", sh.Status)

	det, err = fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, det.ShipmentStatus)
	assert.Equal(t, "arrived_at_zamk", *det.ShipmentStatus)

	// 8. Further advancement after arrived_at_zamk must be rejected
	_, err = fix.svc.AdvanceSimulatedReturnShipment(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrInvalidShipmentTransition)

	// 9. Now receiving can naturally start
	err = fix.svc.StartReceiving(ctx, retID)
	require.NoError(t, err, "Warehouse receiving must succeed after arrived_at_zamk")
}

func TestM532_Simulator_RejectedReturnCannotSimulate(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Rejected return test"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status:       "rejected",
		AdminComment: strPtr("Invalid evidence"),
	}))

	_, err = fix.svc.CreateSimulatedReturnShipment(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrReturnNotApproved)
}

func TestM532_Simulator_ProductionGuard(t *testing.T) {
	fix := setupM51Fixture(t)

	// Initialize handler in production mode
	prodHandler := returns.NewHandler(fix.svc, "production")

	r := chi.NewRouter()
	r.Post("/admin/returns/{id}/simulate-shipment", prodHandler.SimulateCreateReturnShipment)
	r.Post("/admin/returns/{id}/simulate-shipment-step", prodHandler.SimulateAdvanceReturnShipment)

	retID := uuid.New().String()

	// 1. Test create simulator in prod
	req1 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID+"/simulate-shipment", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusForbidden, rec1.Code)
	assert.Contains(t, rec1.Body.String(), "dev_tool_disabled")

	// 2. Test advance simulator in prod
	req2 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID+"/simulate-shipment-step", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusForbidden, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "dev_tool_disabled")
}

func TestM532_Simulator_DevHTTP(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: strPtr("Dev HTTP return"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{Status: "approved"}))

	devHandler := returns.NewHandler(fix.svc, "development")
	r := chi.NewRouter()
	r.Post("/admin/returns/{id}/simulate-shipment", devHandler.SimulateCreateReturnShipment)
	r.Post("/admin/returns/{id}/simulate-shipment-step", devHandler.SimulateAdvanceReturnShipment)

	// Step 1: Create
	req1 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID.String()+"/simulate-shipment", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusCreated, rec1.Code)
	assert.Contains(t, rec1.Body.String(), "awaiting_handover")

	// Step 2: Step -> handed_over
	req2 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID.String()+"/simulate-shipment-step", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "handed_over")

	// Step 3: Step -> in_transit
	req3 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID.String()+"/simulate-shipment-step", nil)
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code)
	assert.Contains(t, rec3.Body.String(), "in_transit")

	// Step 4: Step -> arrived_at_zamk
	req4 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID.String()+"/simulate-shipment-step", nil)
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)
	assert.Equal(t, http.StatusOK, rec4.Code)
	assert.Contains(t, rec4.Body.String(), "arrived_at_zamk")

	// Negative 1: Nonexistent return ID
	nonExistentID := uuid.New().String()
	reqNonExistent := httptest.NewRequest(http.MethodPost, "/admin/returns/"+nonExistentID+"/simulate-shipment", nil)
	recNonExistent := httptest.NewRecorder()
	r.ServeHTTP(recNonExistent, reqNonExistent)
	assert.Equal(t, http.StatusNotFound, recNonExistent.Code)
	assert.Contains(t, recNonExistent.Body.String(), "not_found")

	// Negative 2: Invalid UUID
	reqInvalid := httptest.NewRequest(http.MethodPost, "/admin/returns/not-a-valid-uuid/simulate-shipment", nil)
	recInvalid := httptest.NewRecorder()
	r.ServeHTTP(recInvalid, reqInvalid)
	assert.Equal(t, http.StatusBadRequest, recInvalid.Code)
	assert.Contains(t, recInvalid.Body.String(), "invalid_id")
}
