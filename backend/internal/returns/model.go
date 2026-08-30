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
