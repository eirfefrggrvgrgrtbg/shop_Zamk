package orders

import (
	"errors"

	"github.com/google/uuid"
)

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
	// ErrInvalidDeliveryMethod: the delivery method does not exist or is not active.
	ErrInvalidDeliveryMethod = errors.New("invalid delivery method")
	// ErrFirstPaymentPending: the customer has no paid orders yet and another order's payment is still in progress.
	ErrFirstPaymentPending = errors.New("another order is awaiting payment")
)

// FirstPaymentPendingError names the order whose payment must finish before a new order is created.
type FirstPaymentPendingError struct {
	OrderID     uuid.UUID
	OrderNumber string
}

func (e *FirstPaymentPendingError) Error() string { return ErrFirstPaymentPending.Error() }

func (e *FirstPaymentPendingError) Is(target error) bool { return target == ErrFirstPaymentPending }
