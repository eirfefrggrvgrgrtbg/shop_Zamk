package fulfillment

type CreateShipmentRequest struct {
	Carrier        *string `json:"carrier"`
	TrackingNumber *string `json:"trackingNumber"`
	TrackingUrl    *string `json:"trackingUrl"`
}

type UpdateShipmentStatusRequest struct {
	Status         string  `json:"status"`
	Carrier        *string `json:"carrier"`
	TrackingNumber *string `json:"trackingNumber"`
	TrackingUrl    *string `json:"trackingUrl"`
	Comment        *string `json:"comment"`
}

type CustomerFulfillmentResponse struct {
	ID              string            `json:"id"`
	OrderID         string            `json:"orderId"`
	SellerID        string            `json:"sellerId"`
	SellerName      *string           `json:"sellerName,omitempty"`
	Status          string            `json:"status"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
	ShipmentID      *string           `json:"shipmentId,omitempty"`
	ShipmentStatus  *string           `json:"shipmentStatus,omitempty"`
	Items           []FulfillmentItem `json:"items"`
}

type ResolveReceivingCodeRequest struct {
	Code string `json:"code"`
}

type ScanItemRequest struct {
	Barcode         string `json:"barcode"`
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type ScannedItemState struct {
	OrderItemID  string `json:"orderItemId"`
	ProductID    string `json:"productId"`
	ProductTitle string `json:"productTitle"`
	VariantID    string `json:"variantId"`
	SKU          string `json:"sku"`
	ExpectedQty  int    `json:"expectedQty"`
	ScannedQty   int    `json:"scannedQty"`
	Status       string `json:"status"`
}

type ConfirmReceivingRequest struct {
	SessionID       string             `json:"sessionId,omitempty"`
	ExpectedVersion int                `json:"expectedVersion,omitempty"`
	IdempotencyKey  string             `json:"idempotencyKey,omitempty"`
	Comment         *string            `json:"comment,omitempty"`
	Items           []ScannedItemState `json:"items,omitempty"`
	Carrier         *string            `json:"carrier,omitempty"`
}

type RecordDiscrepancyRequest struct {
	SessionID string             `json:"sessionId,omitempty"`
	Reason    string             `json:"reason"`
	Comment   string             `json:"comment"`
	Items     []ScannedItemState `json:"items,omitempty"`
}

type DeliverShipmentRequest struct {
	Comment *string `json:"comment,omitempty"`
}
