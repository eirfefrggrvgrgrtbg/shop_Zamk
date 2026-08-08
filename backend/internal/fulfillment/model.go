package fulfillment

import (
	"time"

	"github.com/google/uuid"
)

type Shipment struct {
	ID             uuid.UUID  `json:"id"`
	OrderID        uuid.UUID  `json:"orderId"`
	FulfillmentID  *uuid.UUID `json:"fulfillmentId,omitempty"`
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
	OrderNumber        *string           `json:"orderNumber,omitempty"`
	ReceivingCode      *string           `json:"receivingCode,omitempty"`
	ReceivingQRToken   *string           `json:"receivingQrToken,omitempty"`
	PackedAt           *time.Time        `json:"packedAt,omitempty"`
	AcceptedAt         *time.Time        `json:"acceptedAt,omitempty"`
	AcceptedByStaffID  *uuid.UUID        `json:"acceptedByStaffId,omitempty"`
	ReceivingResult    interface{}       `json:"receivingResult,omitempty"`
	DiscrepancyReason  *string           `json:"discrepancyReason,omitempty"`
	DiscrepancyComment *string           `json:"discrepancyComment,omitempty"`
	DiscrepancyAt      *time.Time        `json:"discrepancyAt,omitempty"`
	ShipmentStatus     *string           `json:"shipmentStatus,omitempty"`
	ShipmentID         *uuid.UUID        `json:"shipmentId,omitempty"`
	DeliveryAddress    *string           `json:"deliveryAddress,omitempty"`
	CustomerName       *string           `json:"customerName,omitempty"`
	CustomerPhone      *string           `json:"customerPhone,omitempty"`
	Items              []FulfillmentItem `json:"items"`
}

type FulfillmentItem struct {
	OrderItemID    uuid.UUID `json:"orderItemId"`
	ProductID      uuid.UUID `json:"productId"`
	ProductTitle   string    `json:"productTitle"`
	VariantID      *uuid.UUID `json:"variantId,omitempty"`
	VariantSize    *string   `json:"variantSize,omitempty"`
	VariantColor   *string   `json:"variantColor,omitempty"`
	SKU            *string   `json:"sku,omitempty"`
	Barcode        *string   `json:"barcode,omitempty"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unitPriceCents"`
	LineTotalCents int64     `json:"lineTotalCents"`
	ImageURL       *string   `json:"imageUrl,omitempty"`
}

type ReceivingSession struct {
	ID               uuid.UUID       `json:"id"`
	FulfillmentID    uuid.UUID       `json:"fulfillmentId"`
	Status           string          `json:"status"` // active, accepted, discrepancy, cancelled
	Version          int             `json:"version"`
	StartedAt        time.Time       `json:"startedAt"`
	StartedByStaffID *uuid.UUID      `json:"startedByStaffId,omitempty"`
	CompletedAt      *time.Time      `json:"completedAt,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	Items            []ReceivingItem `json:"items"`
	CanConfirm       bool            `json:"canConfirm"`
}

type ReceivingItem struct {
	ID                 uuid.UUID  `json:"id"`
	SessionID          uuid.UUID  `json:"sessionId"`
	FulfillmentItemID  *uuid.UUID `json:"fulfillmentItemId,omitempty"`
	VariantID          *uuid.UUID `json:"variantId,omitempty"`
	SKU                string     `json:"sku"`
	Barcode            *string    `json:"barcode,omitempty"`
	ProductTitle       string     `json:"productTitle"`
	ExpectedQuantity   int        `json:"expectedQuantity"`
	ScannedQuantity    int        `json:"scannedQuantity"`
	DamagedQuantity    int        `json:"damagedQuantity"`
	UnexpectedQuantity int        `json:"unexpectedQuantity"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

