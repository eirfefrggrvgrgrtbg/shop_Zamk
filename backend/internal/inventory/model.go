package inventory

import (
	"time"

	"github.com/google/uuid"
)

const (
	MovementTypeReceipt             = "receipt"
	MovementTypeAdjustment          = "adjustment"
	MovementTypeWriteOff            = "write_off"
	MovementTypeReservationCreated  = "reservation_created"
	MovementTypeReservationReleased = "reservation_released"
	MovementTypeSale                = "sale"
	MovementTypeReturn              = "return"

	ReservationStatusActive    = "active"
	ReservationStatusReleased  = "released"
	ReservationStatusConverted = "converted"
	ReservationStatusExpired   = "expired"
)

type Item struct {
	ID               uuid.UUID `json:"id"`
	ProductID        uuid.UUID `json:"productId"`
	ProductVariantID uuid.UUID `json:"productVariantId"`
	SellerID         uuid.UUID `json:"sellerId"`
	TotalStock       int       `json:"totalStock"`
	ReservedStock    int       `json:"reservedStock"`
	AvailableStock   int       `json:"availableStock"` // Computed field
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (i *Item) ComputeAvailable() {
	i.AvailableStock = i.TotalStock - i.ReservedStock
}

type StockMovement struct {
	ID               uuid.UUID  `json:"id"`
	InventoryItemID  uuid.UUID  `json:"inventoryItemId"`
	ProductID        uuid.UUID  `json:"productId"`
	ProductVariantID uuid.UUID  `json:"productVariantId"`
	SellerID         uuid.UUID  `json:"sellerId"`
	Type             string     `json:"type"`
	Quantity         int        `json:"quantity"`
	Reason           *string    `json:"reason,omitempty"`
	ActorUserID      *uuid.UUID `json:"actorUserId,omitempty"`
	ReferenceType    *string    `json:"referenceType,omitempty"`
	ReferenceID      *uuid.UUID `json:"referenceId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type Reservation struct {
	ID               uuid.UUID  `json:"id"`
	InventoryItemID  uuid.UUID  `json:"inventoryItemId"`
	ProductID        uuid.UUID  `json:"productId"`
	ProductVariantID uuid.UUID  `json:"productVariantId"`
	UserID           *uuid.UUID `json:"userId,omitempty"`
	Quantity         int        `json:"quantity"`
	Status           string     `json:"status"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	OrderID          *uuid.UUID `json:"orderId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	ReleasedAt       *time.Time `json:"releasedAt,omitempty"`
}
