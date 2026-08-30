package returns

import (
	"time"

	"github.com/google/uuid"
)

type CreateReturnRequest struct {
	Reason  string                   `json:"reason" validate:"required"`
	Comment *string                  `json:"comment"`
	Items   []CreateReturnItemRequest `json:"items" validate:"required,min=1"`
}

type CreateReturnItemRequest struct {
	OrderItemID uuid.UUID `json:"orderItemId" validate:"required"`
	Quantity    int       `json:"quantity" validate:"required,min=1"`
	Reason      *string   `json:"reason"`
	Condition   *string   `json:"condition"`
}

type UpdateReturnStatusRequest struct {
	Status       string  `json:"status" validate:"required,oneof=approved rejected item_received completed cancelled"`
	AdminComment *string `json:"adminComment"`
	// For item_received/completed status, optionally specify restock decision per item
	ItemRestock []UpdateReturnItemRestockRequest `json:"itemRestock"`
}

type UpdateReturnItemRestockRequest struct {
	ReturnItemID uuid.UUID `json:"returnItemId" validate:"required"`
	Restock      bool      `json:"restock"`
}

type CreateRefundRequest struct {
	Reason *string `json:"reason"`
}

type CreateReturnResponse struct {
	ReturnResponse
	Returns []ReturnResponse `json:"returns"`
}

type ReturnResponse struct {
	Return
	Items []ReturnItem `json:"items"`
}

type ReturnListResponse struct {
	Items      []ReturnResponse `json:"items"`
	TotalCount int              `json:"totalCount"`
}

type RefundListResponse struct {
	Items      []Refund `json:"items"`
	TotalCount int      `json:"totalCount"`
}

type SellerReturnItem struct {
	ReturnItemID       uuid.UUID `json:"returnItemId"`
	ReturnID           uuid.UUID `json:"returnId"`
	OrderID            uuid.UUID `json:"orderId"`
	OrderNumber        *string   `json:"orderNumber"`
	OrderItemID        uuid.UUID `json:"orderItemId"`
	Status             string    `json:"status"` // return status
	Quantity           int       `json:"quantity"`
	Reason             *string   `json:"reason"`
	Condition          *string   `json:"condition"`
	ProductTitle       string     `json:"productTitle"`
	VariantSize        *string    `json:"variantSize"`
	VariantColor       *string    `json:"variantColor"`
	SKU                *string    `json:"sku"`
	ImageURL           *string    `json:"imageUrl"`
	PriceCents         int64      `json:"priceCents"`
	SubtotalPriceCents int64      `json:"subtotalPriceCents"`
	Restock            bool       `json:"restock"`
	AdminComment       *string    `json:"adminComment"`
	FinancialAdjustmentCents *int64 `json:"financialAdjustmentCents"`
	FinancialImpactType      *string `json:"financialImpactType"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type SellerReturnListResponse struct {
	Items      []SellerReturnItem `json:"items"`
	TotalCount int                `json:"totalCount"`
}

type StartReceivingRequest struct{}

type ScanReturnUnitRequest struct {
	Code string `json:"code" validate:"required"`
}

type ScanReturnUnitResponse struct {
	AlreadyScanned bool           `json:"alreadyScanned"`
	ReturnItemUnit ReturnItemUnit `json:"returnItemUnit"`
}

type UpdateSerializedUnitInspectionRequest struct {
	InspectedCondition *string `json:"inspectedCondition"`
	Disposition        string  `json:"disposition" validate:"required"`
}

type UpdateLegacyItemInspectionRequest struct {
	AcceptedQuantity int `json:"acceptedQuantity"`
	DamagedQuantity  int `json:"damagedQuantity"`
	RejectedQuantity int `json:"rejectedQuantity"`
}

type AdminReturnReceivingState struct {
	Return              Return                     `json:"return"`
	OrderNumber         *string                    `json:"orderNumber"`
	Items               []AdminReturnReceivingItem `json:"items"`
	TotalRequested      int                        `json:"totalRequested"`
	TotalScanned        int                        `json:"totalScanned"`
	TotalRemaining      int                        `json:"totalRemaining"`
	SerializedRequested int                        `json:"serializedRequested"`
	SerializedScanned   int                        `json:"serializedScanned"`
	LegacyRequested     int                        `json:"legacyRequested"`
	CanFinalize         bool                       `json:"canFinalize"`
}

type OutboundAllocationDetail struct {
	AllocationID uuid.UUID  `json:"allocationId"`
	UnitCode     string     `json:"unitCode"`
	PickedAt     *time.Time `json:"pickedAt"`
	ReleasedAt   *time.Time `json:"releasedAt"`
	UnitStatus   string     `json:"unitStatus"`
}

type ScannedUnitDetail struct {
	ID                    uuid.UUID  `json:"id"`
	ReturnItemID          uuid.UUID  `json:"returnItemId"`
	OrderItemAllocationID uuid.UUID  `json:"orderItemAllocationId"`
	UnitCode              string     `json:"unitCode"`
	ScannedAt             *time.Time `json:"scannedAt"`
	InspectedCondition    *string    `json:"inspectedCondition"`
	Disposition           *string    `json:"disposition"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type AdminReturnReceivingItem struct {
	ReturnItem          ReturnItem                 `json:"returnItem"`
	AllocationMode      string                     `json:"allocationMode"` // "serialized" | "legacy"
	OutboundAllocations []OutboundAllocationDetail `json:"outboundAllocations"`
	ScannedUnits        []ScannedUnitDetail        `json:"scannedUnits"`
	RequestedQuantity   int                        `json:"requestedQuantity"`
	ScannedQuantity     int                        `json:"scannedQuantity"`
	RemainingQuantity   int                        `json:"remainingQuantity"`
	NotReceivedQuantity int                        `json:"notReceivedQuantity"`
	AcceptedQuantity    int                        `json:"acceptedQuantity"`
	DamagedQuantity     int                        `json:"damagedQuantity"`
	RejectedQuantity    int                        `json:"rejectedQuantity"`
	CanFinalize         bool                       `json:"canFinalize"`
}
