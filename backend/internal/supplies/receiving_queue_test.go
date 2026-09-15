package supplies_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func TestSupplyReceivingQueue_FilteringAndExclusions(t *testing.T) {
	tc := setupTestContext(t)
	now := time.Now().UTC()

	// 1. Arrived at ZAMK supply -> MUST BE INCLUDED
	arrivedSupplyID := uuid.New()
	arrivedNumber := "SUP-Q-ARRIVED-001"
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, arrived_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'arrived_at_zamk', 'self_delivery', $4, $5, $5, $5)
	`, arrivedSupplyID, arrivedNumber, tc.SellerID, "qr-arr-001", now)
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 5, $4, $4)
	`, uuid.New(), arrivedSupplyID, tc.Variant1, now)
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at)
		VALUES ($1, $2, 'BOX-01', 'qr-box-arr-1', $3)
	`, uuid.New(), arrivedSupplyID, now)

	// 2. Receiving in progress with active session -> MUST BE INCLUDED
	receivingSupplyID := uuid.New()
	receivingNumber := "SUP-Q-REC-002"
	receivingQR := "qr-rec-002"
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, arrived_at, receiving_started_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'arrived_at_zamk', 'carrier_delivery', $4, $5, $5, $5, $5)
	`, receivingSupplyID, receivingNumber, tc.SellerID, receivingQR, now)
	recItemID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, $4, $4)
	`, recItemID, receivingSupplyID, tc.Variant1, now)
	recBoxID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at)
		VALUES ($1, $2, 'BOX-01', 'qr-box-rec-1', $3)
	`, recBoxID, receivingSupplyID, now)

	// Start receiving session for receivingSupplyID
	session, err := tc.Service.StartReceivingSession(tc.Ctx, tc.AdminID, receivingQR)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Scan 2 items in the active session
	err = tc.Service.RecordScan(tc.Ctx, tc.AdminID, session.ID, supplies.RecordReceivingScanRequest{
		VariantID: tc.Variant1,
		Quantity:  2,
		IsDamage:  false,
	})
	require.NoError(t, err)

	// 3. Shipped by seller (in transit, not yet arrived) -> MUST BE EXCLUDED
	shippedSupplyID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, shipped_at, created_at, updated_at)
		VALUES ($1, 'SUP-Q-SHIPPED-003', $2, 'shipped_by_seller', 'carrier_delivery', 'qr-ship-003', $3, $3, $3)
	`, shippedSupplyID, tc.SellerID, now)

	// 4. Draft -> MUST BE EXCLUDED
	draftSupplyID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at)
		VALUES ($1, 'SUP-Q-DRAFT-004', $2, 'draft', 'self_delivery', 'qr-draft-004', $3, $3)
	`, draftSupplyID, tc.SellerID, now)

	// 5. Ready to ship -> MUST BE EXCLUDED
	readySupplyID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at)
		VALUES ($1, 'SUP-Q-READY-005', $2, 'ready_to_ship', 'self_delivery', 'qr-ready-005', $3, $3)
	`, readySupplyID, tc.SellerID, now)

	// 6. Completed -> MUST BE EXCLUDED
	completedSupplyID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, completed_at, created_at, updated_at)
		VALUES ($1, 'SUP-Q-COMPLETED-006', $2, 'completed', 'self_delivery', 'qr-comp-006', $3, $3, $3)
	`, completedSupplyID, tc.SellerID, now)

	// 7. Completed with discrepancies -> MUST BE EXCLUDED from incoming queue
	compDiscrepSupplyID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, completed_at, created_at, updated_at)
		VALUES ($1, 'SUP-Q-COMPDISC-007', $2, 'completed_with_discrepancies', 'self_delivery', 'qr-cdisc-007', $3, $3, $3)
	`, compDiscrepSupplyID, tc.SellerID, now)

	// 8. Cancelled -> MUST BE EXCLUDED
	cancelledSupplyID := uuid.New()
	testDB.Exec(tc.Ctx, `
		INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, qr_token, created_at, updated_at)
		VALUES ($1, 'SUP-Q-CANCELLED-008', $2, 'cancelled', 'self_delivery', 'qr-canc-008', $3, $3)
	`, cancelledSupplyID, tc.SellerID, now)

	// Query queue
	queue, err := tc.Service.GetReceivingQueue(tc.Ctx)
	require.NoError(t, err)

	queueMap := make(map[uuid.UUID]supplies.SupplyReceivingQueueItem)
	for _, item := range queue {
		queueMap[item.SupplyID] = item
	}

	// Assertions for inclusion
	assert.Contains(t, queueMap, arrivedSupplyID, "Arrived at ZAMK supply must be in receiving queue")
	assert.Contains(t, queueMap, receivingSupplyID, "In-progress receiving supply must be in receiving queue")

	// Assertions for exclusion
	assert.NotContains(t, queueMap, shippedSupplyID, "Shipped (not arrived) supply must be excluded from queue")
	assert.NotContains(t, queueMap, draftSupplyID, "Draft supply must be excluded from queue")
	assert.NotContains(t, queueMap, readySupplyID, "Ready to ship supply must be excluded from queue")
	assert.NotContains(t, queueMap, completedSupplyID, "Completed supply must be excluded from queue")
	assert.NotContains(t, queueMap, compDiscrepSupplyID, "Completed with discrepancies supply must be excluded from incoming queue")
	assert.NotContains(t, queueMap, cancelledSupplyID, "Cancelled supply must be excluded from queue")

	// Detailed verification for arrivedSupply
	arrItem := queueMap[arrivedSupplyID]
	assert.Equal(t, arrivedNumber, arrItem.SupplyNumber)
	assert.Equal(t, "arrived_at_zamk", arrItem.Status)
	assert.Equal(t, 5, arrItem.ExpectedUnitsCount)
	assert.Equal(t, 0, arrItem.AcceptedUnitsCount)
	assert.Equal(t, 5, arrItem.RemainingUnitsCount)
	assert.Equal(t, 1, arrItem.CargoPlacesCount)
	assert.NotNil(t, arrItem.ArrivedAt)
	assert.Nil(t, arrItem.ActiveReceivingSessionID)

	// Detailed verification for receivingSupply
	recItem := queueMap[receivingSupplyID]
	assert.Equal(t, receivingNumber, recItem.SupplyNumber)
	assert.Equal(t, "receiving", recItem.Status)
	assert.Equal(t, 10, recItem.ExpectedUnitsCount)
	assert.Equal(t, 2, recItem.AcceptedUnitsCount)
	assert.Equal(t, 8, recItem.RemainingUnitsCount)
	assert.Equal(t, 1, recItem.CargoPlacesCount)
	assert.NotNil(t, recItem.ReceivingStartedAt)
	require.NotNil(t, recItem.ActiveReceivingSessionID)
	assert.Equal(t, session.ID, *recItem.ActiveReceivingSessionID)
}

func TestSupplyReceivingQueue_DTOLeastPrivilege_NoCommercialOrPIIFields(t *testing.T) {
	now := time.Now().UTC()
	sessID := uuid.New()
	item := supplies.SupplyReceivingQueueItem{
		SupplyID:                 uuid.New(),
		SupplyNumber:             "SUP-100",
		Status:                   "receiving",
		SellerID:                 uuid.New(),
		SellerName:               "Best Brand",
		ExpectedUnitsCount:       10,
		AcceptedUnitsCount:       3,
		RemainingUnitsCount:      7,
		CargoPlacesCount:         2,
		ArrivedAt:                &now,
		ReceivingStartedAt:       &now,
		ActiveReceivingSessionID: &sessID,
	}

	bytes, err := json.Marshal(item)
	require.NoError(t, err)

	var jsonMap map[string]interface{}
	err = json.Unmarshal(bytes, &jsonMap)
	require.NoError(t, err)

	// Forbidden commercial, financial, and PII fields
	forbiddenFields := []string{
		"subtotalCents",
		"subtotal_cents",
		"commissionBps",
		"commission_bps",
		"sellerAmountCents",
		"seller_amount_cents",
		"unitPriceCents",
		"unit_price_cents",
		"priceCents",
		"price_cents",
		"payoutCents",
		"payout_cents",
		"customerPhone",
		"customer_phone",
		"deliveryAddress",
		"delivery_address",
		"recipientName",
		"recipient_name",
		"customerEmail",
		"customer_email",
	}

	for _, field := range forbiddenFields {
		_, exists := jsonMap[field]
		assert.False(t, exists, "DTO must not expose forbidden field %q", field)
	}

	// Required operational fields
	assert.Contains(t, jsonMap, "supplyId")
	assert.Contains(t, jsonMap, "supplyNumber")
	assert.Contains(t, jsonMap, "status")
	assert.Contains(t, jsonMap, "sellerId")
	assert.Contains(t, jsonMap, "sellerName")
	assert.Contains(t, jsonMap, "expectedUnitsCount")
	assert.Contains(t, jsonMap, "acceptedUnitsCount")
	assert.Contains(t, jsonMap, "remainingUnitsCount")
	assert.Contains(t, jsonMap, "cargoPlacesCount")
	assert.Contains(t, jsonMap, "arrivedAt")
	assert.Contains(t, jsonMap, "receivingStartedAt")
	assert.Contains(t, jsonMap, "activeReceivingSessionId")
}
