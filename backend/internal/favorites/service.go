package favorites

import (
	"context"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
)

// ProductLister is the slice of the products service used for the personal brand feed.
type ProductLister interface {
	ListPublicProducts(ctx context.Context, filter products.PublicProductFilter, limit, offset int) (products.PublicProductListResponse, error)
}

type Service struct {
	repo     *Repository
	products ProductLister
}

func (s *Service) WithProducts(p ProductLister) *Service {
	s.products = p
	return s
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) AddFavorite(ctx context.Context, userID, productID uuid.UUID) error {
	return s.repo.AddFavorite(ctx, userID, productID)
}

func (s *Service) RemoveFavorite(ctx context.Context, userID, productID uuid.UUID) error {
	return s.repo.RemoveFavorite(ctx, userID, productID)
}

func (s *Service) ListFavorites(ctx context.Context, userID uuid.UUID) ([]products.Product, error) {
	return s.repo.ListFavorites(ctx, userID)
}
