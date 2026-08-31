package returns

import (
	"time"

	"github.com/google/uuid"
)

type CreateReturnRequest struct {
	Reason  string                    `json:"reason" validate:"required"`
	Comment *string                   `json:"comment"`
	Items   []CreateReturnItemRequest `json:"items" validate:"required,min=1"`
}

type CreateReturnItemRequest struct {
	OrderItemID uuid.UUID   `json:"orderItemId" validate:"required"`
	Quantity    int         `json:"quantity" validate:"required,min=1"`
	Reason      *string     `json:"reason"`
	Condition   *string     `json:"condition"`
	EvidenceIDs []uuid.UUID `json:"evidenceIds"`
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

type CustomerReturnEvidence struct {
	ID          uuid.UUID `json:"id"`
	URL         string    `json:"url"`
	ContentType string    `json:"contentType"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CustomerReturnItemDetail struct {
	ID                 uuid.UUID                `json:"id"`
	ReturnID           uuid.UUID                `json:"returnId"`
	OrderItemID        uuid.UUID                `json:"orderItemId"`
	ProductTitle       string                   `json:"productTitle"`
	ProductImageURL    *string                  `json:"productImageUrl"`
	VariantSize        *string                  `json:"variantSize"`
	VariantColor       *string                  `json:"variantColor"`
	SKU                *string                  `json:"sku"`
	Quantity           int                      `json:"quantity"`
	PriceCents         int64                    `json:"priceCents"`
	SubtotalPriceCents int64                    `json:"subtotalPriceCents"`
	Reason             *string                  `json:"reason"`
	Condition          *string                  `json:"condition"`
	Evidence           []CustomerReturnEvidence `json:"evidence"`
}

type CreateReturnResponse struct {
	ReturnResponse
	Returns []ReturnResponse `json:"returns"`
}

type ReturnResponse struct {
	Return
	OrderNumber *string                    `json:"orderNumber,omitempty"`
	Items       []CustomerReturnItemDetail `json:"items"`
	Shipment    *ReturnShipmentResponse    `json:"shipment,omitempty"`
}

type ReturnListResponse struct {
	Items      []ReturnResponse `json:"items"`
	TotalCount int              `json:"totalCount"`
}

type CustomerReturnResponse = ReturnResponse
type CustomerReturnListResponse = ReturnListResponse

type RefundListResponse struct {
	Items      []Refund `json:"items"`
	TotalCount int      `json:"totalCount"`
}

type AdminReturnEvidence struct {
	ID          uuid.UUID `json:"id"`
	URL         string    `json:"url"`
	ContentType string    `json:"contentType"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AdminReturnItemDetail struct {
	ID                 uuid.UUID             `json:"id"`
	ReturnID           uuid.UUID             `json:"returnId"`
	OrderItemID        uuid.UUID             `json:"orderItemId"`
	ProductTitle       string                `json:"productTitle"`
	ProductImageURL    *string               `json:"productImageUrl"`
	VariantSize        *string               `json:"variantSize"`
	VariantColor       *string               `json:"variantColor"`
	SKU                *string               `json:"sku"`
	Quantity           int                   `json:"quantity"`
	PriceCents         int64                 `json:"priceCents"`
	SubtotalPriceCents int64                 `json:"subtotalPriceCents"`
	Reason             *string               `json:"reason"`
	Condition          *string               `json:"condition"`
	Restock            bool                  `json:"restock"`
	Evidence           []AdminReturnEvidence `json:"evidence"`
}

type AdminReturnResponse struct {
	ID            uuid.UUID               `json:"id"`
	OrderID       uuid.UUID               `json:"orderId"`
	OrderNumber   *string                 `json:"orderNumber"`
	FulfillmentID uuid.UUID               `json:"fulfillmentId"`
	UserID        uuid.UUID               `json:"userId"`
	CustomerName  *string                 `json:"customerName"`
	CustomerEmail *string                 `json:"customerEmail"`
	CustomerPhone *string                 `json:"customerPhone"`
	SellerID      *uuid.UUID              `json:"sellerId"`
	SellerName    *string                 `json:"sellerName"`
	Status        string                  `json:"status"`
	Reason        string                  `json:"reason"`
	Comment       *string                 `json:"comment"`
	AdminComment  *string                 `json:"adminComment"`
	CreatedAt     time.Time               `json:"createdAt"`
	UpdatedAt     time.Time               `json:"updatedAt"`
	ApprovedAt    *time.Time              `json:"approvedAt"`
	RejectedAt    *time.Time              `json:"rejectedAt"`
	CompletedAt   *time.Time              `json:"completedAt"`
	DeliveredAt   *time.Time              `json:"deliveredAt"`
	EvidenceCount int                     `json:"evidenceCount"`
	Items          []AdminReturnItemDetail `json:"items"`
	Shipment       *ReturnShipmentResponse `json:"shipment,omitempty"`
	ShipmentStatus *string                 `json:"shipmentStatus"`
	ShipmentMethod *string                 `json:"shipmentMethod,omitempty"`
}

type AdminReturnListResponse struct {
	Items      []AdminReturnResponse `json:"items"`
	TotalCount int                   `json:"totalCount"`
}

type SellerReturnItem struct {
	ReturnItemID             uuid.UUID `json:"returnItemId"`
	ReturnID                 uuid.UUID `json:"returnId"`
	OrderID                  uuid.UUID `json:"orderId"`
	OrderNumber              *string   `json:"orderNumber"`
	OrderItemID              uuid.UUID `json:"orderItemId"`
	Status                   string    `json:"status"` // return status
	Quantity                 int       `json:"quantity"`
	Reason                   *string   `json:"reason"`
	Condition                *string   `json:"condition"`
	ProductTitle             string    `json:"productTitle"`
	VariantSize              *string   `json:"variantSize"`
	VariantColor             *string   `json:"variantColor"`
	SKU                      *string   `json:"sku"`
	ImageURL                 *string   `json:"imageUrl"`
	PriceCents               int64     `json:"priceCents"`
	SubtotalPriceCents       int64     `json:"subtotalPriceCents"`
	Restock                  bool      `json:"restock"`
	AdminComment             *string   `json:"adminComment"`
	FinancialAdjustmentCents *int64    `json:"financialAdjustmentCents"`
	FinancialImpactType      *string   `json:"financialImpactType"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
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
	ProductTitle        *string                    `json:"productTitle,omitempty"`
	ProductImageURL     *string                    `json:"productImageUrl,omitempty"`
	VariantSize         *string                    `json:"variantSize,omitempty"`
	VariantColor        *string                    `json:"variantColor,omitempty"`
	SKU                 *string                    `json:"sku,omitempty"`
	PriceCents          *int64                     `json:"priceCents,omitempty"`
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

type UploadEvidenceResponse struct {
	ID  uuid.UUID `json:"id"`
	URL string    `json:"url"`
}

type CreateReturnShipmentRequest struct {
	Method         string            `json:"method" validate:"required,oneof=cdek_courier cdek_office"`
	CDEKOfficeCode *string           `json:"cdekOfficeCode" validate:"required_if=Method cdek_office"`
	CustomerName   *string           `json:"customerName" validate:"required_if=Method cdek_courier"`
	CustomerPhone  *string           `json:"customerPhone" validate:"required_if=Method cdek_courier"`
	PickupAddress  *PickupAddressDTO `json:"pickupAddress" validate:"required_if=Method cdek_courier"`
}

type PickupAddressDTO struct {
	City   string  `json:"city" validate:"required"`
	Street string  `json:"street" validate:"required"`
	House  string  `json:"house" validate:"required"`
	Flat   *string `json:"flat,omitempty"`
}

type CDEKOfficeDTO struct {
	Code         string  `json:"code"`
	Address      string  `json:"address"`
	Name         string  `json:"name"`
	WorkingHours *string `json:"workingHours,omitempty"`
}

type ReturnShipmentResponse struct {
	ID                     uuid.UUID         `json:"id"`
	Provider               string            `json:"provider"`
	Method                 string            `json:"method"`
	TrackingNumber         *string           `json:"trackingNumber,omitempty"`
	ProviderShipmentID     *string           `json:"providerShipmentId,omitempty"`
	Status                 string            `json:"status"`
	SelectedCDEKOfficeCode *string           `json:"selectedCdekOfficeCode,omitempty"`
	CustomerName           *string           `json:"customerName,omitempty"`
	CustomerPhone          *string           `json:"customerPhone,omitempty"`
	PickupAddress          *PickupAddressDTO `json:"pickupAddress,omitempty"`
	CDEKOfficeAddress      *string           `json:"cdekOfficeAddress,omitempty"`
}


type ReturnMessageAttachmentResponse struct {
	ID               uuid.UUID `json:"id"`
	URL              string    `json:"url"`
	ContentType      string    `json:"contentType"`
	SizeBytes        int64     `json:"sizeBytes"`
	OriginalFilename *string   `json:"originalFilename"`
	SortOrder        int       `json:"sortOrder"`
}

type UploadReturnMessageAttachmentResponse struct {
	ID  uuid.UUID `json:"id"`
	URL string    `json:"url"`
}

type ReturnRefundQuoteItem struct {
	OrderItemID        uuid.UUID `json:"orderItemId"`
	ProductTitle       string    `json:"productTitle"`
	Mode               string    `json:"mode"` // "serialized" | "legacy"
	RequestedQuantity  int       `json:"requestedQuantity"`
	RefundableQuantity int       `json:"refundableQuantity"`
	UnitPriceCents     int64     `json:"unitPriceCents"`
	RefundCents        int64     `json:"refundCents"`
}

type ReturnRefundQuote struct {
	ReturnID                 uuid.UUID               `json:"returnId"`
	OrderNumber              *string                 `json:"orderNumber"`
	Currency                 string                  `json:"currency"`
	Items                    []ReturnRefundQuoteItem `json:"items"`
	ProductsRefundCents      int64                   `json:"productsRefundCents"`
	DeliveryRefundCents      int64                   `json:"deliveryRefundCents"`
	TotalRefundCents         int64                   `json:"totalRefundCents"`
	AlreadyRefundedCents     int64                   `json:"alreadyRefundedCents"`
	RemainingRefundableCents int64                   `json:"remainingRefundableCents"`
	CanRefund                bool                    `json:"canRefund"`
	BlockingReason           *string                 `json:"blockingReason"`
	LatestRefundStatus       *string                 `json:"latestRefundStatus"`
	LatestRefundProcessedAt  *time.Time              `json:"latestRefundProcessedAt,omitempty"`
}

type AdminSendReturnMessageRequest struct {
	Message       string      `json:"message"`
	NeedsResponse bool        `json:"needsResponse"`
	AttachmentIDs []uuid.UUID `json:"attachmentIds"`
}

type CustomerSendReturnMessageRequest struct {
	Message       string      `json:"message"`
	AttachmentIDs []uuid.UUID `json:"attachmentIds"`
}

type ReturnMessageResponse struct {
	ID          uuid.UUID                         `json:"id"`
	ReturnID    uuid.UUID                         `json:"returnId"`
	SenderRole  string                            `json:"senderRole"`
	MessageType string                            `json:"messageType"`
	Body        string                            `json:"body"`
	CreatedAt   time.Time                         `json:"createdAt"`
	Attachments []ReturnMessageAttachmentResponse `json:"attachments"`
}

type ReturnConversationResponse struct {
	Messages []ReturnMessageResponse `json:"messages"`
}

type TimelineEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	OccurredAt  time.Time              `json:"occurredAt"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	ActorType   string                 `json:"actorType"`
	ActorLabel  string                 `json:"actorLabel"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type TimelineResponse struct {
	EntityType          string          `json:"entityType"`
	EntityID            uuid.UUID       `json:"entityId"`
	CanonicalIdentifier string          `json:"canonicalIdentifier"`
	Events              []TimelineEvent `json:"events"`
}
