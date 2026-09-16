package personalization

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RecordProductView(ctx context.Context, userID, productID uuid.UUID) error {
	return s.repo.RecordProductView(ctx, userID, productID)
}

func (s *Service) GetProductView(ctx context.Context, userID, productID uuid.UUID) (*CustomerProductView, error) {
	return s.repo.GetProductView(ctx, userID, productID)
}

func (s *Service) GetRecentlyViewedProducts(ctx context.Context, userID uuid.UUID, limit int) ([]products.PublicProduct, error) {
	prods, err := s.repo.GetRecentlyViewedProducts(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}

	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}

func (s *Service) GetSimilarProducts(ctx context.Context, productID uuid.UUID, limit int) ([]products.PublicProduct, error) {
	prods, err := s.repo.GetSimilarProducts(ctx, productID, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}
	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}

// GetCustomerPreferenceProfile returns the authenticated customer's preference profile derived on read
// from current favorites and product views.
func (s *Service) GetCustomerPreferenceProfile(ctx context.Context, userID uuid.UUID, limit int) (*CustomerPreferenceProfile, error) {
	return s.repo.GetCustomerPreferenceProfile(ctx, userID, limit)
}

// GetForYouProducts returns personalized product recommendations for the authenticated customer
// based on their preference profile (favorites and views). If the customer has no preferences or
// no matching storefront-accessible products, an empty slice is returned.
func (s *Service) GetForYouProducts(ctx context.Context, userID uuid.UUID, limit int) ([]products.PublicProduct, error) {
	if limit <= 0 {
		limit = DefaultForYouLimit
	} else if limit > MaxForYouLimit {
		limit = MaxForYouLimit
	}

	profile, err := s.GetCustomerPreferenceProfile(ctx, userID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer preference profile: %w", err)
	}

	if len(profile.FavoriteCategories) == 0 &&
		len(profile.FavoriteBrands) == 0 &&
		len(profile.ViewedCategories) == 0 &&
		len(profile.ViewedBrands) == 0 {
		return []products.PublicProduct{}, nil
	}

	favCatIDs := make([]uuid.UUID, 0, len(profile.FavoriteCategories))
	for _, a := range profile.FavoriteCategories {
		favCatIDs = append(favCatIDs, a.CategoryID)
	}

	favBrandIDs := make([]uuid.UUID, 0, len(profile.FavoriteBrands))
	for _, a := range profile.FavoriteBrands {
		favBrandIDs = append(favBrandIDs, a.BrandID)
	}

	viewedCatIDs := make([]uuid.UUID, 0, len(profile.ViewedCategories))
	for _, a := range profile.ViewedCategories {
		viewedCatIDs = append(viewedCatIDs, a.CategoryID)
	}

	viewedBrandIDs := make([]uuid.UUID, 0, len(profile.ViewedBrands))
	for _, a := range profile.ViewedBrands {
		viewedBrandIDs = append(viewedBrandIDs, a.BrandID)
	}

	prods, err := s.repo.GetForYouProducts(ctx, userID, favCatIDs, favBrandIDs, viewedCatIDs, viewedBrandIDs, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}
	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}
