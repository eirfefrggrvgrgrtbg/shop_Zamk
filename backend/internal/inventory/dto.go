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

type AdminInventoryItem struct {
	ID               uuid.UUID `json:"id"`
	ProductID        uuid.UUID `json:"productId"`
	ProductVariantID uuid.UUID `json:"productVariantId"`
	ProductTitle     string    `json:"productTitle"`
	VariantLabel     string    `json:"variant"`
	SellerID         uuid.UUID `json:"sellerId"`
	SellerName       string    `json:"sellerName"`
	Source           string    `json:"source"` // 'auction_direct_sale' or 'seller'
	TotalStock       int       `json:"totalStock"`
	ReservedStock    int       `json:"reservedStock"`
	AvailableStock   int       `json:"availableStock"`
	CreatedAt        string    `json:"createdAt"`
	UpdatedAt        string    `json:"updatedAt"`
}

type AdminInventoryListResponse struct {
	Items      []AdminInventoryItem `json:"items"`
	TotalCount int                  `json:"totalCount"`
}

type InventoryListResponse struct {
	Items      []Item `json:"items"`
	TotalCount int    `json:"totalCount"`
}

type StockMovementsListResponse struct {
	Items      []StockMovement `json:"items"`
	TotalCount int             `json:"totalCount"`
}
