package cart

import "errors"

var (
	ErrCartNotFound         = errors.New("cart not found")
	ErrCartItemNotFound     = errors.New("cart item not found")
	ErrProductNotPublished  = errors.New("product is not published")
	ErrVariantNotFound      = errors.New("product variant not found")
	// ErrProductUnavailable: the product is not purchasable as a new or larger cart line (hidden from the public
	// catalog by the canonical storefront rules). Lines already in the cart are never removed because of it.
	ErrProductUnavailable = errors.New("product is currently unavailable")
	ErrInsufficientStock    = errors.New("insufficient stock")
	ErrInvalidQuantity      = errors.New("invalid quantity")
)
