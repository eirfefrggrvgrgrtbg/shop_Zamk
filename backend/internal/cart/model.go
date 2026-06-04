package cart

import (
	"time"

	"github.com/google/uuid"
)

type Cart struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"userId" db:"user_id"`
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time  `json:"updatedAt" db:"updated_at"`
	Items     []CartItem `json:"items" db:"-"`
}

type CartItem struct {
	ID               uuid.UUID `json:"id" db:"id"`
	CartID           uuid.UUID `json:"cartId" db:"cart_id"`
	ProductID        uuid.UUID `json:"productId" db:"product_id"`
	ProductVariantID uuid.UUID `json:"productVariantId" db:"product_variant_id"`
	Quantity         int       `json:"quantity" db:"quantity"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`

	// These fields are populated dynamically
	PriceCents int64  `json:"priceCents" db:"-"`
	InStock    bool   `json:"inStock" db:"-"`
	Title      string `json:"title" db:"-"`
}
