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
)
