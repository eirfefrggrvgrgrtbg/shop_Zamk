package fulfillment

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrPickingNotAllowed  = errors.New("picking not allowed for this order status")
	ErrInvariantViolation = errors.New("invariant violation in allocation data")
)

// GetPickingOrder reads the persistent state of a picking fulfillment.
func (s *Service) GetPickingOrder(ctx context.Context, fulfillmentID uuid.UUID) (*PickingOrder, error) {
	return s.repo.GetPickingOrder(ctx, fulfillmentID)
}
