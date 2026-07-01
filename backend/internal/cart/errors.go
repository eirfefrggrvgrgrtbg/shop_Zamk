package cart

import "errors"

var (
	ErrCartNotFound         = errors.New("cart not found")
	ErrCartItemNotFound     = errors.New("cart item not found")
	ErrProductNotPublished  = errors.New("product is not published")
	ErrVariantNotFound      = errors.New("product variant not found")
	ErrInsufficientStock    = errors.New("insufficient stock")
	ErrInvalidQuantity      = errors.New("invalid quantity")
)
