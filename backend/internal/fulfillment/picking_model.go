package fulfillment

import (
	"time"

	"github.com/google/uuid"
)

type PickingOrder struct {
	OrderID           uuid.UUID      `json:"orderId"`
	OrderNumber       *string        `json:"orderNumber,omitempty"`
	OrderStatus       string         `json:"orderStatus"`
	FulfillmentID     uuid.UUID      `json:"fulfillmentId"`
	FulfillmentStatus string         `json:"fulfillmentStatus"`
	Items             []PickingItem  `json:"items"`
}

type PickingItem struct {
	OrderItemID       uuid.UUID              `json:"orderItemId"`
	Title             string                 `json:"title"`
	ProductVariantID  uuid.UUID              `json:"productVariantId"`
	Quantity          int                    `json:"quantity"`
	AllocationMode    string                 `json:"allocationMode"` // "serialized" | "legacy"
	PickedQuantity    int                    `json:"pickedQuantity"`
	RemainingQuantity int                    `json:"remainingQuantity"`
	AllocatedUnits    []PickingAllocatedUnit `json:"allocatedUnits"`
}

type PickingAllocatedUnit struct {
	InventoryUnitID uuid.UUID  `json:"inventoryUnitId"`
	UnitCode        string     `json:"unitCode"`
	PickedAt        *time.Time `json:"pickedAt,omitempty"`
}
