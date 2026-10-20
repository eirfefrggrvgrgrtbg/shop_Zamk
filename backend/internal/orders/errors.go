package orders

import "errors"

var (
	ErrOrderNotFound           = errors.New("order not found")
	ErrEmptyCart               = errors.New("cart is empty")
	ErrProductNotPublished     = errors.New("product is not published")
	ErrVariantNotFound         = errors.New("product variant not found")
	ErrInsufficientStock       = errors.New("insufficient stock")
	ErrOrderNotCancellable     = errors.New("order is not cancellable")
	ErrOrderAlreadyCancelled   = errors.New("order is already cancelled")
	ErrIdempotencyKeyConflict  = errors.New("idempotency key conflict")
	// ErrSelfPurchase: the buyer is an owner or manager of a store whose product is in the cart.
	ErrSelfPurchase = errors.New("cannot buy products of your own store")
)
