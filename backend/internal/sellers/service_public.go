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

	brandName := ""
	if seller.BrandName != nil {
		brandName = *seller.BrandName
	}
	slug := ""
	if seller.Slug != nil {
		slug = *seller.Slug
	}
	publicSeller := &PublicSeller{
		ID:          seller.ID,
		BrandName:   brandName,
		Slug:        slug,
		Description: seller.Description,
		LogoURL:     seller.LogoURL,
	}

	return publicSeller, nil
}
