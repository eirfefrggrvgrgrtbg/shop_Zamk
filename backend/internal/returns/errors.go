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
	ErrReturnNotReceived          = errors.New("return is not in a received state for refund")
	ErrReturnRejected             = errors.New("return is rejected or cancelled")
	ErrAmbiguousFundingPayment    = errors.New("ambiguous funding payment: multiple succeeded payments exist for order")
	ErrPaymentNotFound            = errors.New("succeeded payment not found for order")
	ErrRefundNoEligibleItems      = errors.New("no accepted items eligible for refund")
	ErrRefundAllocationInvariant  = errors.New("refund_allocation_invariant: order item allocation state is inconsistent with order quantity")
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
	ErrReturnNotArrived           = errors.New("return_not_arrived")
	ErrInvalidUnitState           = errors.New("inventory unit is not in eligible state for return")

	// M5.3.1A Evidence errors
	ErrEvidenceRequired      = errors.New("2 to 6 photos required for this return reason")
	ErrEvidenceTooMany       = errors.New("maximum 6 photos allowed")
	ErrCommentRequired       = errors.New("comment is required")
	ErrEvidenceNotFound      = errors.New("evidence not found or not owned by customer")
	ErrEvidenceAlreadyBound  = errors.New("evidence already used in another return")
	ErrEvidenceDuplicate     = errors.New("duplicate evidence photo")
	ErrEvidenceInvalidFormat = errors.New("invalid evidence media format")

	// M5.3.3A Logistics errors
	ErrReturnNotApproved         = errors.New("return_not_approved")
	ErrShipmentAlreadyExists     = errors.New("shipment_already_exists")
	ErrInvalidShipmentTransition = errors.New("invalid_shipment_status_transition")
	ErrCDEKOfficeRequired        = errors.New("cdek_office_code_required")
	ErrInvalidCDEKOffice         = errors.New("cdek_office_invalid")
	ErrCourierInfoRequired       = errors.New("courier_info_required")

	// M5.3.4A Communication errors
	ErrReturnMessageRequired           = errors.New("message_required")
	ErrReturnNotRequestableInfo        = errors.New("return_not_requestable_info")
	ErrReturnNotAwaitingInfo           = errors.New("return_not_awaiting_info")
	ErrReturnTerminalState             = errors.New("return_terminal_state")
	ErrReturnMessageAttachmentInvalid  = errors.New("return_message_attachment_invalid")
	ErrReturnMessageAttachmentTooLarge = errors.New("return_message_attachment_too_large")
	ErrReturnMessageTooManyAttachments = errors.New("return_message_too_many_attachments")
	ErrReturnMessageAttachmentNotOwned = errors.New("return_message_attachment_not_owned")

	// Dev simulator errors
	ErrShipmentNotFound       = errors.New("shipment_not_found")
	ErrDevToolDisabled        = errors.New("dev_tool_disabled")
	ErrNoPendingRefund        = errors.New("no_pending_refund")
	ErrMultiplePendingRefunds = errors.New("multiple_pending_refunds")
)
