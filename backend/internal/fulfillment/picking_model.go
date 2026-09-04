package fulfillment

import (
	"time"

	"github.com/google/uuid"
)

type PickingOrder struct {
	OrderID           uuid.UUID     `json:"orderId"`
	OrderNumber       *string       `json:"orderNumber,omitempty"`
	OrderStatus       string        `json:"orderStatus"`
	FulfillmentID     uuid.UUID     `json:"fulfillmentId"`
	FulfillmentStatus string        `json:"fulfillmentStatus"`
	Items             []PickingItem `json:"items"`
}

type PickingItem struct {
	OrderItemID          uuid.UUID              `json:"orderItemId"`
	Title                string                 `json:"title"`
	ProductVariantID     uuid.UUID              `json:"productVariantId"`
	VariantSize          *string                `json:"variantSize,omitempty"`
	VariantColor         *string                `json:"variantColor,omitempty"`
	ImageURL             *string                `json:"imageUrl,omitempty"`
	SKU                  *string                `json:"sku,omitempty"`
	Barcode              *string                `json:"barcode,omitempty"`
	Quantity             int                    `json:"quantity"`
	AllocationMode       string                 `json:"allocationMode"` // "serialized" | "legacy"
	PickedQuantity       int                    `json:"pickedQuantity"`
	RemainingQuantity    int                    `json:"remainingQuantity"`
	AllocatedUnits       []PickingAllocatedUnit `json:"allocatedUnits"`
	CompatibleUnitsCount int                    `json:"compatibleUnitsCount"`
}

type PickingAllocatedUnit struct {
	InventoryUnitID uuid.UUID  `json:"inventoryUnitId"`
	UnitCode        string     `json:"unitCode"`
	PickedAt        *time.Time `json:"pickedAt,omitempty"`
}

type PickingScanResult struct {
	FulfillmentID       uuid.UUID         `json:"fulfillmentId"`
	OrderID             uuid.UUID         `json:"orderId"`
	OrderNumber         string            `json:"orderNumber,omitempty"`
	ScanResult          PickingScanDetail `json:"scanResult"`
	Item                PickingItemState  `json:"item"`
	FulfillmentProgress PickingProgress   `json:"fulfillmentProgress"`
}

type PickingScanDetail struct {
	Type            string    `json:"type"` // "serialized" | "legacy"
	OrderItemID     uuid.UUID `json:"orderItemId"`
	Code            string    `json:"code"`
	NewlyPicked     bool      `json:"newlyPicked"`
	AlreadyPicked   bool      `json:"alreadyPicked,omitempty"`
	Substituted     bool      `json:"substituted,omitempty"`
	AlreadyComplete bool      `json:"alreadyComplete,omitempty"`
}

type PickingItemState struct {
	Quantity          int    `json:"quantity"`
	PickedQuantity    int    `json:"pickedQuantity"`
	RemainingQuantity int    `json:"remainingQuantity"`
	AllocationMode    string `json:"allocationMode"`
}

type PickingProgress struct {
	TotalQuantity     int  `json:"totalQuantity"`
	PickedQuantity    int  `json:"pickedQuantity"`
	RemainingQuantity int  `json:"remainingQuantity"`
	IsComplete        bool `json:"isComplete"`
}

type CompatibleUnit struct {
	InventoryUnitID  uuid.UUID  `json:"inventoryUnitId"`
	UnitCode         string     `json:"unitCode"`
	ProductVariantID uuid.UUID  `json:"productVariantId"`
	Availability     string     `json:"availability"` // "allocated_to_current_item" | "free"
	PickedAt         *time.Time `json:"pickedAt,omitempty"`
}
