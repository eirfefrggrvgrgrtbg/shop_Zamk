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

type Fulfillment struct {
	ID                uuid.UUID         `json:"id"`
	OrderID           uuid.UUID         `json:"orderId"`
	SellerID          uuid.UUID         `json:"sellerId"`
	SellerName        *string           `json:"sellerName,omitempty"`
	Status            string            `json:"status"`
	SubtotalCents     int64             `json:"subtotalCents"`
	CommissionBps     int               `json:"commissionBps"`
	SellerAmountCents int64             `json:"sellerAmountCents"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
	ShipmentStatus    *string           `json:"shipmentStatus,omitempty"`
	ShipmentID        *uuid.UUID        `json:"shipmentId,omitempty"`
	DeliveryAddress   *string           `json:"deliveryAddress,omitempty"`
	CustomerName      *string           `json:"customerName,omitempty"`
	CustomerPhone     *string           `json:"customerPhone,omitempty"`
	Items             []FulfillmentItem `json:"items"`
}

type FulfillmentItem struct {
	OrderItemID    uuid.UUID `json:"orderItemId"`
	ProductID      uuid.UUID `json:"productId"`
	ProductTitle   string    `json:"productTitle"`
	VariantID      *uuid.UUID `json:"variantId,omitempty"`
	SKU            *string   `json:"sku,omitempty"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unitPriceCents"`
	LineTotalCents int64     `json:"lineTotalCents"`
	ImageURL       *string   `json:"imageUrl,omitempty"`
}
