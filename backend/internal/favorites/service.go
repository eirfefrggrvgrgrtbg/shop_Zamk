package favorites

import (
	"context"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
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
