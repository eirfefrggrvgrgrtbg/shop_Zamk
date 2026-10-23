package payouts

import "errors"

var (
	ErrInsufficientBalance  = errors.New("insufficient balance for payout")
	ErrInvalidPayoutAmount  = errors.New("payout amount must be greater than zero")
	ErrPayoutNotFound       = errors.New("payout not found")
	ErrInvalidPayoutStatus  = errors.New("invalid status transition")
	ErrUnauthorized         = errors.New("unauthorized")

	// Payout batch operations (PAYOUT.ADMIN.1): stable errors the admin API maps to readable answers.
	ErrPayoutBatchNotFound       = errors.New("payout batch not found")
	ErrPayoutBatchAlreadyPaid    = errors.New("already paid")
	ErrPayoutBatchNotProcessable = errors.New("payout batch cannot be confirmed in its current status")
	ErrPayoutBatchNotHoldable    = errors.New("payout batch cannot be held in its current status")
	ErrNothingToPay              = errors.New("no positive available balance")
)
