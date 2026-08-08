package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

func GetUserID(ctx context.Context) uuid.UUID {
	val := ctx.Value("userID")
	if val == nil {
		return uuid.Nil
	}
	id, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

// GetSellerID extracts a previously injected seller ID.
// Requires RequireActiveSeller middleware to have run.
func GetSellerID(ctx context.Context) (uuid.UUID, error) {
	val := ctx.Value("sellerID")
	if val == nil {
		return uuid.Nil, errors.New("no seller ID in context")
	}
	id, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid seller ID type in context")
	}
	return id, nil
}
