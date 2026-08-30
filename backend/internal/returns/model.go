package returns

import (
	"time"

	"github.com/google/uuid"
)

type Return struct {
	ID                 uuid.UUID  `json:"id"`
	OrderID            uuid.UUID  `json:"orderId"`
	FulfillmentID      uuid.UUID  `json:"fulfillmentId"`
	UserID             uuid.UUID  `json:"userId"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason"`
	Comment            *string    `json:"comment"`
	AdminComment       *string    `json:"adminComment"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	ApprovedAt         *time.Time `json:"approvedAt"`
	RejectedAt         *time.Time `json:"rejectedAt"`
	CompletedAt        *time.Time `json:"completedAt"`
	ReceivingStartedAt *time.Time `json:"receivingStartedAt"`
}

type ReturnItem struct {
	ID               uuid.UUID `json:"id"`
	ReturnID         uuid.UUID `json:"returnId"`
	OrderItemID      uuid.UUID `json:"orderItemId"`
	Quantity         int       `json:"quantity"`
	Reason           *string   `json:"reason"`
	Condition        *string   `json:"condition"`
	Restock          bool      `json:"restock"`
	AcceptedQuantity int       `json:"acceptedQuantity"`
	DamagedQuantity  int       `json:"damagedQuantity"`
	RejectedQuantity int       `json:"rejectedQuantity"`
	CreatedAt        time.Time `json:"createdAt"`
}

type ReturnItemUnit struct {
	ID                    uuid.UUID  `json:"id"`
	ReturnItemID          uuid.UUID  `json:"returnItemId"`
	OrderItemAllocationID uuid.UUID  `json:"orderItemAllocationId"`
	ScannedAt             *time.Time `json:"scannedAt"`
	InspectedCondition    *string    `json:"inspectedCondition"`
	Disposition           *string    `json:"disposition"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type Refund struct {
	ID               uuid.UUID  `json:"id"`
	ReturnID         *uuid.UUID `json:"returnId"`
	PaymentID        *uuid.UUID `json:"paymentId"`
	OrderID          uuid.UUID  `json:"orderId"`
	Status           string     `json:"status"`
	AmountCents      int64      `json:"amountCents"`
	Currency         string     `json:"currency"`
	Provider         *string    `json:"provider"`
	ProviderRefundID *string    `json:"providerRefundId"`
	Reason           *string    `json:"reason"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	ProcessedAt      *time.Time `json:"processedAt"`
	FailedAt         *time.Time `json:"failedAt"`
}

type ReturnItemEvidence struct {
	ID           uuid.UUID  `json:"id"`
	CustomerID   uuid.UUID  `json:"customerId"`
	ReturnItemID *uuid.UUID `json:"returnItemId"`
	StorageKey   string     `json:"storageKey"`
	ContentType  string     `json:"contentType"`
	SortOrder    int        `json:"sortOrder"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type ReturnShipment struct {
	ID                     uuid.UUID `json:"id"`
	ReturnID               uuid.UUID `json:"returnId"`
	Provider               string    `json:"provider"`
	Method                 string    `json:"method"`
	TrackingNumber         *string   `json:"trackingNumber"`
	ProviderShipmentID     *string   `json:"providerShipmentId"`
	Status                 string    `json:"status"`
	SelectedCDEKOfficeCode *string   `json:"selectedCdekOfficeCode"`
	CustomerName           *string   `json:"customerName"`
	CustomerPhone          *string   `json:"customerPhone"`
	PickupAddress          []byte    `json:"pickupAddress"`
	CDEKOfficeAddress      *string   `json:"cdekOfficeAddress"`
	DestinationAddress     []byte    `json:"destinationAddress"`
	Snapshots              []byte    `json:"snapshots"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

var AllowedShipmentTransitions = map[string]map[string]bool{
	"draft": {
		"awaiting_handover": true,
		"cancelled":         true,
	},
	"awaiting_handover": {
		"handed_over": true,
		"cancelled":   true,
	},
	"handed_over": {
		"in_transit": true,
		"cancelled":  true,
	},
	"in_transit": {
		"arrived_at_zamk": true,
		"cancelled":       true,
	},
	"arrived_at_zamk": {},
	"cancelled":       {},
}

func IsValidShipmentTransition(from, to string) bool {
	if from == to {
		return true
	}
	nextMap, exists := AllowedShipmentTransitions[from]
	if !exists {
		return false
	}
	return nextMap[to]
}
