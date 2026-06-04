package payments

import "errors"

var (
	ErrPaymentNotFound         = errors.New("payment not found")
	ErrOrderNotFound           = errors.New("order not found or unauthorized")
	ErrOrderNotAwaitingPayment = errors.New("order is not awaiting payment")
	ErrInvalidAmount           = errors.New("invalid payment amount")
	ErrInvalidCurrency         = errors.New("invalid payment currency")
	ErrInvalidSignature        = errors.New("invalid webhook signature")
	ErrPaymentAlreadyProcessed = errors.New("payment already processed safely")
)
