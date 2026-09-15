package returns_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

func TestReturnReceivingQueue_LifecycleAndExclusions(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Helper to create return
	createTestReturn := func(qty int) (uuid.UUID, testOrder) {
		tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), qty)
		evIDs := fix.createStagedEvidence(t, fix.userID, 2)
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  "defective",
			Comment: func() *string { s := "Queue test return"; return &s }(),
			Items: []returns.CreateReturnItemRequest{
				{OrderItemID: tOrd.orderItemID, Quantity: qty, EvidenceIDs: evIDs},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp, 1)
		return resp[0].Return.ID, tOrd
	}

	// 1. Requested return -> MUST NOT appear in queue
	retIDRequested, _ := createTestReturn(1)

	// 2. Approved but in_transit return -> MUST NOT appear in queue
	retIDInTransit, _ := createTestReturn(2)
	err := fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDInTransit, returns.UpdateReturnStatusRequest{
		Status: "approved",
	})
	require.NoError(t, err)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, status, tracking_number, updated_at)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'in_transit', 'TRK-TRANSIT', NOW())
	`, uuid.New(), retIDInTransit)
	require.NoError(t, err)

	// 3. Approved and arrived_at_zamk return -> MUST appear in queue ("approved")
	retIDArrived, ordArrived := createTestReturn(3)
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDArrived, returns.UpdateReturnStatusRequest{
		Status: "approved",
	})
	require.NoError(t, err)
	arrivedTime := time.Now().Add(-10 * time.Minute).Truncate(time.Microsecond)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_shipments (id, return_id, provider, method, status, tracking_number, updated_at)
		VALUES ($1, $2, 'cdek', 'cdek_office', 'arrived_at_zamk', 'TRK-ARRIVED-1', $3)
	`, uuid.New(), retIDArrived, arrivedTime)
	require.NoError(t, err)

	// 4. In progress receiving return -> MUST appear in queue ("receiving")
	retIDReceiving, ordReceiving := createTestReturn(2)
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDReceiving, returns.UpdateReturnStatusRequest{
		Status: "approved",
	})
	require.NoError(t, err)
	fix.createArrivedReturnShipment(t, retIDReceiving)
	err = fix.svc.StartReceiving(ctx, retIDReceiving)
	require.NoError(t, err)

	// Inspect 1 unit of legacy item to verify received & remaining counts
	retState, err := fix.svc.GetAdminReturnReceivingState(ctx, retIDReceiving)
	require.NoError(t, err)
	require.NotEmpty(t, retState.Items)
	err = fix.svc.InspectLegacyItem(ctx, retIDReceiving, retState.Items[0].ReturnItem.ID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 1,
		DamagedQuantity:  0,
		RejectedQuantity: 0,
	})
	require.NoError(t, err)

	// 5. Finalized return (item_received) -> MUST NOT appear in queue
	retIDCompleted, _ := createTestReturn(1)
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDCompleted, returns.UpdateReturnStatusRequest{
		Status: "approved",
	})
	require.NoError(t, err)
	fix.createArrivedReturnShipment(t, retIDCompleted)
	err = fix.svc.StartReceiving(ctx, retIDCompleted)
	require.NoError(t, err)
	compState, err := fix.svc.GetAdminReturnReceivingState(ctx, retIDCompleted)
	require.NoError(t, err)
	err = fix.svc.InspectLegacyItem(ctx, retIDCompleted, compState.Items[0].ReturnItem.ID, returns.UpdateLegacyItemInspectionRequest{
		AcceptedQuantity: 0,
		DamagedQuantity:  1,
		RejectedQuantity: 0,
	})
	require.NoError(t, err)
	err = fix.svc.FinalizeReceiving(ctx, retIDCompleted)
	require.NoError(t, err)

	// 6. Rejected and Cancelled returns -> MUST NOT appear in queue
	retIDRejected, _ := createTestReturn(1)
	rejectComment := "Not eligible"
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDRejected, returns.UpdateReturnStatusRequest{
		Status:       "rejected",
		AdminComment: &rejectComment,
	})
	require.NoError(t, err)

	retIDCancelled, _ := createTestReturn(1)
	err = fix.svc.UpdateReturnStatus(ctx, fix.userID, retIDCancelled, returns.UpdateReturnStatusRequest{
		Status: "cancelled",
	})
	require.NoError(t, err)

	// Query Queue
	queue, err := fix.svc.GetReturnReceivingQueue(ctx)
	require.NoError(t, err)

	queueMap := make(map[uuid.UUID]returns.AdminReturnReceivingQueueItem)
	for _, it := range queue {
		queueMap[it.ReturnID] = it
	}

	// Verify exclusions
	assert.NotContains(t, queueMap, retIDRequested, "Requested return must be excluded")
	assert.NotContains(t, queueMap, retIDInTransit, "In-transit return not yet arrived must be excluded")
	assert.NotContains(t, queueMap, retIDCompleted, "Finalized item_received return must be excluded")
	assert.NotContains(t, queueMap, retIDRejected, "Rejected return must be excluded")
	assert.NotContains(t, queueMap, retIDCancelled, "Cancelled return must be excluded")

	// Verify arrived item in queue
	require.Contains(t, queueMap, retIDArrived, "Arrived return must appear in queue")
	arrivedItem := queueMap[retIDArrived]
	assert.Equal(t, retIDArrived, arrivedItem.ReturnID)
	assert.Equal(t, ordArrived.orderID, arrivedItem.OrderID)
	assert.Equal(t, "approved", arrivedItem.ReturnStatus)
	assert.Equal(t, "arrived_at_zamk", arrivedItem.ShipmentStatus)
	assert.Equal(t, 3, arrivedItem.ExpectedUnitsCount)
	assert.Equal(t, 0, arrivedItem.ReceivedUnitsCount)
	assert.Equal(t, 3, arrivedItem.RemainingUnitsCount)
	assert.NotNil(t, arrivedItem.ArrivedAt)

	// Verify receiving item in queue with partial inspection
	require.Contains(t, queueMap, retIDReceiving, "Receiving return must appear in queue")
	recItem := queueMap[retIDReceiving]
	assert.Equal(t, retIDReceiving, recItem.ReturnID)
	assert.Equal(t, ordReceiving.orderID, recItem.OrderID)
	assert.Equal(t, "receiving", recItem.ReturnStatus)
	assert.Equal(t, 2, recItem.ExpectedUnitsCount)
	assert.Equal(t, 1, recItem.ReceivedUnitsCount)
	assert.Equal(t, 1, recItem.RemainingUnitsCount)
	assert.NotNil(t, recItem.ReceivingStartedAt)
}
