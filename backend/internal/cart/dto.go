package cart

import "github.com/google/uuid"

type AddItemRequest struct {
	ProductID        uuid.UUID `json:"productId" validate:"required"`
	ProductVariantID uuid.UUID `json:"productVariantId" validate:"required"`
	Quantity         int       `json:"quantity" validate:"required,gt=0"`
}

type UpdateItemRequest struct {
	Quantity int `json:"quantity" validate:"required,gt=0"`
}

type CartResponse struct {
	ID    uuid.UUID  `json:"id"`
	Items []CartItem `json:"items"`
}
