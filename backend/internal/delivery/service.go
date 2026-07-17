package delivery

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

func (s *Service) GetActiveMethods(ctx context.Context) ([]DeliveryMethod, error) {
	return s.repo.GetActiveMethods(ctx)
}

func (s *Service) GetMethodByID(ctx context.Context, id uuid.UUID) (*DeliveryMethod, error) {
	return s.repo.GetMethodByID(ctx, id)
}
