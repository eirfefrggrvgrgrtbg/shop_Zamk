package sellers

import (
	"context"

	"github.com/google/uuid"
)

// ---------------------------------------------------------
// Public methods
// ---------------------------------------------------------

func (s *Service) GetPublicSeller(ctx context.Context, idOrSlug string) (*PublicSeller, error) {
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

	publicSeller := &PublicSeller{
		ID:          seller.ID,
		BrandName:   seller.BrandName,
		Slug:        seller.Slug,
		Description: seller.Description,
		LogoURL:     seller.LogoURL,
	}

	return publicSeller, nil
}
