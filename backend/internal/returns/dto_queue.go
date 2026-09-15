package returns

import (
	"time"

	"github.com/google/uuid"
)

// AdminReturnReceivingQueueItem represents a return awaiting or undergoing physical warehouse receiving.
// Contains only operational logistics fields; no customer PII, dispute conversations, or financial data.
type AdminReturnReceivingQueueItem struct {
	ReturnID            uuid.UUID  `json:"returnId"`
	OrderID             uuid.UUID  `json:"orderId"`
	OrderNumber         string     `json:"orderNumber"`
	ReturnStatus        string     `json:"returnStatus"`   // "approved" | "receiving"
	ShipmentStatus      string     `json:"shipmentStatus"` // "arrived_at_zamk"
	TrackingNumber      *string    `json:"trackingNumber,omitempty"`
	ExpectedUnitsCount  int        `json:"expectedUnitsCount"`
	ReceivedUnitsCount  int        `json:"receivedUnitsCount"`
	RemainingUnitsCount int        `json:"remainingUnitsCount"`
	ArrivedAt           *time.Time `json:"arrivedAt,omitempty"`
	ReceivingStartedAt  *time.Time `json:"receivingStartedAt,omitempty"`
	SellerName          *string    `json:"sellerName,omitempty"`
	ProductSummary      *string    `json:"productSummary,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
}
