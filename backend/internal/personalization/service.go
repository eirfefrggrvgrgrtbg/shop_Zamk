package personalization

import (
	"context"

	"github.com/google/uuid"
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
