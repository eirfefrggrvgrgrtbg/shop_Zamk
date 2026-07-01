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
