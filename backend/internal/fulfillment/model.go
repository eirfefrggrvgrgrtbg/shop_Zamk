package fulfillment

import (
	"time"

	"github.com/google/uuid"
)

type Shipment struct {
	ID             uuid.UUID  `json:"id"`
	OrderID        uuid.UUID  `json:"orderId"`
	Status         string     `json:"status"`
	Carrier        *string    `json:"carrier"`
	TrackingNumber *string    `json:"trackingNumber"`
	TrackingUrl    *string    `json:"trackingUrl"`
	ShippedAt      *time.Time `json:"shippedAt"`
	DeliveredAt    *time.Time `json:"deliveredAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type ShipmentEvent struct {
	ID          uuid.UUID  `json:"id"`
	ShipmentID  uuid.UUID  `json:"shipmentId"`
	FromStatus  *string    `json:"fromStatus"`
	ToStatus    string     `json:"toStatus"`
	ActorUserID *uuid.UUID `json:"actorUserId"`
	Comment     *string    `json:"comment"`
	CreatedAt   time.Time  `json:"createdAt"`
}
