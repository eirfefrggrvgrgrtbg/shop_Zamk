package personalization

import (
	"context"

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
