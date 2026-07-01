package returns

import "errors"

var (
	ErrReturnNotFound       = errors.New("return not found")
	ErrRefundNotFound       = errors.New("refund not found")
	ErrOrderNotDelivered    = errors.New("can only return delivered orders")
	ErrReturnWindowExpired  = errors.New("return window has expired")
	ErrInvalidQuantity      = errors.New("invalid return quantity")
	ErrUnauthorized         = errors.New("unauthorized access to return")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrRefundExceedsPaid    = errors.New("refund exceeds total paid amount")
)
