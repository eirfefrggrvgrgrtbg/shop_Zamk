package supplies

import "errors"

var (
	ErrSupplyNotFound       = errors.New("supply not found")
	ErrInvalidStatus        = errors.New("invalid status transition")
	ErrUnauthorized         = errors.New("unauthorized to access this supply")
	ErrInvalidQuantities    = errors.New("invalid box quantities, must match expected quantities")
	ErrSessionNotFound      = errors.New("receiving session not found")
	ErrSessionAlreadyActive = errors.New("receiving session already active")
	ErrItemNotFound         = errors.New("item not found in supply")
)
