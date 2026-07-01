package orders

import "errors"

var (
	ErrOrderNotFound           = errors.New("order not found")
	ErrEmptyCart               = errors.New("cart is empty")
	ErrProductNotPublished     = errors.New("product is not published")
	ErrVariantNotFound         = errors.New("product variant not found")
	ErrInsufficientStock       = errors.New("insufficient stock")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrManualPaidNotAllowed    = errors.New("manual transition to paid is not allowed in this phase")
	ErrOrderNotCancellable     = errors.New("order is not cancellable")
)
