package payouts

import "errors"

var (
	ErrInsufficientBalance  = errors.New("insufficient balance for payout")
	ErrInvalidPayoutAmount  = errors.New("payout amount must be greater than zero")
	ErrPayoutNotFound       = errors.New("payout not found")
	ErrInvalidPayoutStatus  = errors.New("invalid status transition")
	ErrUnauthorized         = errors.New("unauthorized")
)
