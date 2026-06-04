package inventory

import "errors"

var (
	ErrInventoryItemNotFound  = errors.New("inventory item not found")
	ErrInsufficientStock      = errors.New("insufficient stock available")
	ErrInvalidQuantity        = errors.New("quantity must be greater than zero")
	ErrNegativeStock          = errors.New("operation would result in negative stock")
	ErrStockBelowReserved     = errors.New("operation would drop total stock below reserved stock")
	ErrReservationNotFound    = errors.New("reservation not found")
	ErrReservationNotActive   = errors.New("reservation is not active")
	ErrInvalidMovementType    = errors.New("invalid stock movement type")
	ErrProductVariantNotFound = errors.New("product variant not found")
	ErrSellerMismatch         = errors.New("product variant does not belong to specified seller")
)
