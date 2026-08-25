package supplies

import (
	"time"

	"github.com/google/uuid"
)

type Supply struct {
	ID                  uuid.UUID    `json:"id"`
	SupplyNumber        string       `json:"supplyNumber"`
	SellerID            uuid.UUID    `json:"sellerId"`
	SellerName          string       `json:"sellerName,omitempty"`
	Status              string       `json:"status"`
	HandoffMethod       string       `json:"handoffMethod"`
	CarrierName         *string      `json:"carrierName,omitempty"`
	TrackingNumber      *string      `json:"trackingNumber,omitempty"`
	ExpectedArrivalDate *time.Time   `json:"expectedArrivalDate,omitempty"`
	QRToken             *string      `json:"qrToken,omitempty"`
	CreatedAt           time.Time    `json:"createdAt"`
	ShippedAt           *time.Time   `json:"shippedAt,omitempty"`
	ArrivedAt           *time.Time   `json:"arrivedAt,omitempty"`
	ReceivingStartedAt  *time.Time   `json:"receivingStartedAt,omitempty"`
	CompletedAt         *time.Time   `json:"completedAt,omitempty"`
	UpdatedAt           time.Time    `json:"updatedAt"`

	TotalExpectedItems int `json:"totalExpectedItems"`
	TotalAcceptedItems int `json:"totalAcceptedItems"`
	TotalExpectedBoxes int `json:"totalExpectedBoxes"`
	SKUCount           int `json:"skuCount"`

	Items []SupplyItem `json:"items,omitempty"`
	Boxes []SupplyBox  `json:"boxes,omitempty"`
	InventoryUnits []InventoryUnit `json:"inventoryUnits,omitempty"`
}

type SupplyItem struct {
	ID               uuid.UUID `json:"id"`
	SupplyID         uuid.UUID `json:"supplyId"`
	VariantID        uuid.UUID `json:"variantId"`
	ExpectedQuantity int       `json:"expectedQuantity"`
	AcceptedQuantity int       `json:"acceptedQuantity"`
	DamagedQuantity  int       `json:"damagedQuantity"`
	MissingQuantity  int       `json:"missingQuantity"`
	ExtraQuantity    int       `json:"extraQuantity"`
	ReceivingComment *string   `json:"receivingComment,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`

	// Joins
	SKU          string  `json:"sku,omitempty"`
	SellerSKU    *string `json:"sellerSku,omitempty"`
	ProductTitle string  `json:"productTitle,omitempty"`
	ColorName    *string `json:"colorName,omitempty"`
	SizeName     *string `json:"sizeName,omitempty"`
	Barcode      *string `json:"barcode,omitempty"`
}

type SupplyBox struct {
	ID        uuid.UUID       `json:"id"`
	SupplyID  uuid.UUID       `json:"supplyId"`
	BoxNumber string          `json:"boxNumber"`
	QRToken   *string         `json:"qrToken,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	Items     []SupplyBoxItem `json:"items,omitempty"`
}

type SupplyBoxItem struct {
	BoxID        uuid.UUID `json:"boxId"`
	SupplyItemID uuid.UUID `json:"supplyItemId"`
	Quantity     int       `json:"quantity"`
}

type ReceivingSession struct {
	ID               uuid.UUID        `json:"id"`
	SupplyID         uuid.UUID        `json:"supplyId"`
	Status           string           `json:"status"` // active, completed, cancelled
	Version          int              `json:"version"`
	StartedAt        time.Time        `json:"startedAt"`
	StartedByStaffID *uuid.UUID       `json:"startedByStaffId,omitempty"`
	CompletedAt      *time.Time       `json:"completedAt,omitempty"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	Items            []ReceivingItem  `json:"items,omitempty"`
	ReceivingMode    string           `json:"receivingMode"` // "serialized" | "legacy"
}

type ReceivingItem struct {
	ID                 uuid.UUID `json:"id"`
	SessionID          uuid.UUID `json:"sessionId"`
	SupplyItemID       *uuid.UUID `json:"supplyItemId,omitempty"`
	VariantID          *uuid.UUID `json:"variantId,omitempty"`
	SKU                string    `json:"sku"`
	Barcode            *string   `json:"barcode,omitempty"`
	ProductTitle       string    `json:"productTitle"`
	ExpectedQuantity   int       `json:"expectedQuantity"`
	ScannedQuantity    int       `json:"scannedQuantity"`
	DamagedQuantity    int       `json:"damagedQuantity"`
	UnexpectedQuantity int       `json:"unexpectedQuantity"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type ReceivingScan struct {
	ID                   uuid.UUID  `json:"id"`
	SessionID            uuid.UUID  `json:"sessionId"`
	SupplyReceivingItemID uuid.UUID  `json:"supplyReceivingItemId"`
	StaffID              *uuid.UUID `json:"staffId,omitempty"`
	Quantity             int        `json:"quantity"`
	IsDamage             bool       `json:"isDamage"`
	CreatedAt            time.Time  `json:"createdAt"`
}

type InventoryUnit struct {
	ID                  uuid.UUID  `json:"id"`
	UnitCode            string     `json:"unitCode"`
	ProductVariantID    uuid.UUID  `json:"productVariantId"`
	OriginSupplyID      uuid.UUID  `json:"originSupplyId"`
	OriginSupplyItemID  uuid.UUID  `json:"originSupplyItemId"`
	OriginBoxID         *uuid.UUID `json:"originBoxId,omitempty"`
	UnitIndex           int        `json:"unitIndex"`
	ExternalMarkingCode *string    `json:"externalMarkingCode,omitempty"`
	Status              string     `json:"status"`
	ReceivingSessionID  *uuid.UUID `json:"receivingSessionId,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type EnrichedInventoryUnit struct {
	ID                 uuid.UUID
	UnitCode           string
	ProductVariantID   uuid.UUID
	OriginSupplyID     uuid.UUID
	OriginSupplyItemID uuid.UUID
	OriginBoxID        *uuid.UUID
	UnitIndex          int
	Status             string
	ProductTitle       string
	ColorName          *string
	SizeName           *string
	SellerSKU          *string
	VariantBarcode     *string
}
