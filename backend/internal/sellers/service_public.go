package sellers

import (
	"context"

	"github.com/google/uuid"
)

// ---------------------------------------------------------
// Public methods
// ---------------------------------------------------------

func (s *Service) GetPublicSeller(ctx context.Context, idOrSlug string) (*Seller, error) {
	var seller *Seller
	var err error

	parsedID, parseErr := uuid.Parse(idOrSlug)
	if parseErr == nil {
		seller, err = s.repo.GetSellerByID(ctx, parsedID)
	} else {
		seller, err = s.repo.GetSellerBySlug(ctx, idOrSlug)
	}

	if err != nil {
		return nil, err
	}

	if seller.Status != StatusActive {
		return nil, ErrSellerNotFound
	}

	// Remove private contacts if not explicitly required to be public
	seller.ContactEmail = ""
	seller.ContactPhone = nil

	return seller, nil
}
