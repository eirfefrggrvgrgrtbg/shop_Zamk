package supplies

import "errors"

var (
	ErrSupplyNotFound         = errors.New("supply not found")
	ErrInvalidStatus          = errors.New("invalid status transition")
	ErrUnauthorized           = errors.New("unauthorized to access this supply")
	ErrInvalidQuantities      = errors.New("supply_items_required")
	ErrCarrierRequired        = errors.New("supply_carrier_required")
	ErrCarrierUnsupported     = errors.New("supply_carrier_unsupported")
	ErrTrackingNumberRequired = errors.New("supply_tracking_number_required")
	ErrSessionNotFound        = errors.New("receiving session not found")
	ErrSessionAlreadyActive   = errors.New("receiving session already active")
	ErrItemNotFound           = errors.New("item not found in supply")
)

var ErrSupplyNotArrived = errors.New("supply is not arrived yet")
