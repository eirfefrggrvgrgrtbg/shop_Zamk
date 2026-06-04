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
