package inventory

import "github.com/google/uuid"

type ReceiptRequest struct {
	ProductVariantID uuid.UUID `json:"productVariantId" validate:"required"`
	Quantity         int       `json:"quantity" validate:"required,gt=0"`
	Reason           *string   `json:"reason,omitempty"`
}

type AdjustmentRequest struct {
	ProductVariantID uuid.UUID `json:"productVariantId" validate:"required"`
	Quantity         int       `json:"quantity" validate:"required"` // Can be negative or positive
	Reason           string    `json:"reason" validate:"required"`
}

type WriteOffRequest struct {
	ProductVariantID uuid.UUID `json:"productVariantId" validate:"required"`
	Quantity         int       `json:"quantity" validate:"required,gt=0"` // Always positive, implies subtraction
	Reason           string    `json:"reason" validate:"required"`
}

type InventoryListResponse struct {
	Items      []Item `json:"items"`
	TotalCount int    `json:"totalCount"`
}

type StockMovementsListResponse struct {
	Items      []StockMovement `json:"items"`
	TotalCount int             `json:"totalCount"`
}
