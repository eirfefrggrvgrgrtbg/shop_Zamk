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
	PriceCents int64   `json:"priceCents" db:"-"`
	InStock    bool    `json:"inStock" db:"-"`
	// AvailableQuantity is the canonical free stock (total_stock - reserved_stock) at read time.
	AvailableQuantity int `json:"availableQuantity" db:"-"`
	// UnavailableReason is empty when the line can be purchased as it is; see the Reason* constants.
	UnavailableReason string `json:"unavailableReason,omitempty" db:"-"`
	Title      string  `json:"title" db:"-"`
	Size       *string `json:"size" db:"-"`
	Color      *string `json:"color" db:"-"`
	SellerSKU  *string `json:"sellerSku" db:"-"`
	ImageURL   *string `json:"imageUrl" db:"-"`
	BrandName  *string `json:"brandName,omitempty" db:"-"`
}

const (
	ReasonProductUnpublished = "product_unpublished"
	ReasonVariantInactive    = "variant_inactive"
	ReasonOutOfStock         = "out_of_stock"
	ReasonInsufficientStock  = "insufficient_stock"
)

// UnavailableReasonFor derives why a cart line cannot be purchased right now. Pure read-side derivation:
// it never reserves anything and uses the same free-stock figure as the add/update/checkout checks.
func UnavailableReasonFor(productStatus string, variantActive bool, available, quantity int) string {
	switch {
	case productStatus != "published":
		return ReasonProductUnpublished
	case !variantActive:
		return ReasonVariantInactive
	case available <= 0:
		return ReasonOutOfStock
	case available < quantity:
		return ReasonInsufficientStock
	}
	return ""
}
