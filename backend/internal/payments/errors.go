package payments

import "errors"

var (
	ErrPaymentNotFound          = errors.New("payment not found")
	ErrOrderNotFound            = errors.New("order not found or unauthorized")
	ErrOrderNotAwaitingPayment  = errors.New("order is not awaiting payment")
	ErrInvalidAmount            = errors.New("invalid payment amount")
	ErrInvalidCurrency          = errors.New("invalid payment currency")
	ErrInvalidSignature         = errors.New("invalid webhook signature")
	ErrPaymentAlreadyProcessed  = errors.New("payment already processed safely")
	ErrPaymentMethodUnavailable = errors.New("payment method is currently unavailable")
	ErrRefundExceedsPaid        = errors.New("refund amount exceeds paid amount")
	ErrInvalidPaymentStatus     = errors.New("invalid payment status for operation")
	ErrInvalidRefundAmount      = errors.New("invalid refund amount, must be greater than zero")
	ErrMismatchedOrderAndPayment = errors.New("return and payment orders do not match")
	ErrAmbiguousFundingPayment  = errors.New("ambiguous funding payment: multiple succeeded payments exist for order")
	ErrInsufficientStock        = errors.New("insufficient stock to initiate payment for order")
	ErrFirstPaymentInProgress   = errors.New("another order is currently in payment for this customer")
	ErrProviderRejected         = errors.New("payment provider rejected initialization")
	ErrPaymentOutcomeUncertain  = errors.New("payment outcome uncertain, awaiting provider resolution")
	ErrPaymentAmountMismatch             = errors.New("payment amount mismatch")
	ErrMultipleProviderPaymentsAmbiguous = errors.New("multiple ambiguous matching provider payments")
	ErrProviderPaymentAlreadyAssigned    = errors.New("provider payment id already assigned to another payment")
	ErrPaymentConflict                   = errors.New("payment conflict")
)
