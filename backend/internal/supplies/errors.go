package supplies

import "errors"

var (
	ErrSupplyNotFound             = errors.New("supply not found")
	ErrInvalidStatus              = errors.New("invalid status transition")
	ErrUnauthorized               = errors.New("unauthorized to access this supply")
	ErrInvalidQuantities          = errors.New("supply_items_required")
	ErrCarrierRequired            = errors.New("supply_carrier_required")
	ErrCarrierUnsupported         = errors.New("supply_carrier_unsupported")
	ErrTrackingNumberRequired     = errors.New("supply_tracking_number_required")
	ErrSessionNotFound            = errors.New("receiving session not found")
	ErrSessionAlreadyActive       = errors.New("receiving session already active")
	ErrItemNotFound               = errors.New("item not found in supply")
	ErrSupplyUnitIdentityMismatch = errors.New("supply_unit_identity_mismatch")
	ErrSupplyNotSerialized        = errors.New("supply_not_serialized")

	// Receiving domain errors
	ErrUnitNotFound                   = errors.New("unit_not_found")
	ErrUnitNotInSupply                = errors.New("unit_not_in_supply")
	ErrUnitAlreadyScanned             = errors.New("unit_already_scanned")
	ErrUnitAlreadyReceived            = errors.New("unit_already_received")
	ErrReceivingSessionFinalized      = errors.New("receiving_session_finalized")
	ErrSerializedUnitCodeRequired     = errors.New("serialized_unit_code_required")
	ErrInvalidReceivingCondition      = errors.New("invalid_receiving_condition")
	ErrScanNotFound                   = errors.New("scan_not_found")
	ErrScanAlreadyVoided              = errors.New("scan_already_voided")
	ErrScanNotInSession               = errors.New("scan_not_in_session")
	ErrSupplyNotReadyForReceiving     = errors.New("supply_not_ready_for_receiving")
	ErrSupplyAlreadyCompleted         = errors.New("supply_already_completed")
	ErrSupplyCancelled                = errors.New("supply_cancelled")
	ErrInvalidReceivingCode           = errors.New("invalid_receiving_code")
	ErrNoExpectedUnitsRemain          = errors.New("no_expected_units_remain")
)

var ErrSupplyNotArrived = errors.New("supply_not_arrived")
