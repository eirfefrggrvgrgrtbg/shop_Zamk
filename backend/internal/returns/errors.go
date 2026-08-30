package returns

import "errors"

var (
	ErrReturnNotFound             = errors.New("return not found")
	ErrRefundNotFound             = errors.New("refund not found")
	ErrOrderNotDelivered          = errors.New("can only return delivered orders")
	ErrReturnWindowExpired        = errors.New("return window has expired")
	ErrInvalidQuantity            = errors.New("invalid return quantity")
	ErrUnauthorized               = errors.New("unauthorized access to return")
	ErrInvalidStatusTransition    = errors.New("invalid status transition")
	ErrRefundExceedsPaid          = errors.New("refund exceeds total paid amount")
	ErrRejectReasonRequired       = errors.New("admin comment/reason is required for rejection")
	ErrReturnAlreadyRefunded      = errors.New("return is already refunded or completed")
	ErrReturnNotInReceiving       = errors.New("return is not in receiving state")
	ErrInvalidZMUForReturn        = errors.New("zmu code is not valid for this return")
	ErrAllocationAlreadyBound     = errors.New("zmu allocation is already bound to a different return")
	ErrQuantityExceeded           = errors.New("scanned quantity exceeds requested return quantity")
	ErrInvalidDisposition         = errors.New("invalid unit disposition")
	ErrUnitNotInReturn            = errors.New("unit does not belong to this return")
	ErrItemNotLegacy              = errors.New("legacy inspection only allowed for legacy items")
	ErrItemNotSerialized          = errors.New("serialized inspection only allowed for serialized items")
	ErrFinalizeMissingDisposition = errors.New("all scanned serialized units must have a disposition before finalize")
	ErrInvalidInspectionQuantity  = errors.New("invalid inspection quantities")
	ErrInvalidUnitState           = errors.New("inventory unit is not in eligible state for return")

	// M5.3.1A Evidence errors
	ErrEvidenceRequired      = errors.New("2 to 6 photos required for this return reason")
	ErrEvidenceTooMany       = errors.New("maximum 6 photos allowed")
	ErrCommentRequired       = errors.New("comment is required")
	ErrEvidenceNotFound      = errors.New("evidence not found or not owned by customer")
	ErrEvidenceAlreadyBound  = errors.New("evidence already used in another return")
	ErrEvidenceDuplicate     = errors.New("duplicate evidence photo")
	ErrEvidenceInvalidFormat = errors.New("invalid evidence media format")
)
