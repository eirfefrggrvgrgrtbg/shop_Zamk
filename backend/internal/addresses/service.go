package addresses

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

func (s *Service) ListAddresses(ctx context.Context, userID uuid.UUID) ([]Address, error) {
	return s.repo.ListAddresses(ctx, userID)
}

func (s *Service) CreateAddress(ctx context.Context, userID uuid.UUID, req CreateAddressRequest) (*Address, error) {
	addr := &Address{
		ID:            uuid.New(),
		UserID:        userID,
		Label:         req.Label,
		RecipientName: req.RecipientName,
		Phone:         req.Phone,
		City:          req.City,
		Street:        req.Street,
		House:         req.House,
		Apartment:     req.Apartment,
		PostalCode:    req.PostalCode,
		Comment:       req.Comment,
		IsDefault:     req.IsDefault,
	}
	if err := s.repo.CreateAddress(ctx, addr); err != nil {
		return nil, err
	}
	return s.repo.GetAddressByID(ctx, addr.ID, userID)
}

func (s *Service) UpdateAddress(ctx context.Context, id, userID uuid.UUID, req UpdateAddressRequest) (*Address, error) {
	addr, err := s.repo.GetAddressByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if req.Label != nil {
		addr.Label = req.Label
	}
	if req.RecipientName != nil {
		addr.RecipientName = *req.RecipientName
	}
	if req.Phone != nil {
		addr.Phone = *req.Phone
	}
	if req.City != nil {
		addr.City = *req.City
	}
	if req.Street != nil {
		addr.Street = *req.Street
	}
	if req.House != nil {
		addr.House = *req.House
	}
	if req.Apartment != nil {
		addr.Apartment = req.Apartment
	}
	if req.PostalCode != nil {
		addr.PostalCode = req.PostalCode
	}
	if req.Comment != nil {
		addr.Comment = req.Comment
	}
	if req.IsDefault != nil {
		addr.IsDefault = *req.IsDefault
	}

	if err := s.repo.UpdateAddress(ctx, addr); err != nil {
		return nil, err
	}
	return s.repo.GetAddressByID(ctx, id, userID)
}

func (s *Service) DeleteAddress(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.DeleteAddress(ctx, id, userID)
}

func (s *Service) SetDefault(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.SetDefault(ctx, id, userID)
}
