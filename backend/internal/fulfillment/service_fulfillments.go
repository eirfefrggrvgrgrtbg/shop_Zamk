package fulfillment

import (
	"context"

	"github.com/google/uuid"
)

func (s *Service) ListSellerFulfillments(ctx context.Context, sellerID uuid.UUID, limit, offset int, status *string) ([]Fulfillment, error) {
	return s.repo.ListSellerFulfillments(ctx, sellerID, limit, offset, status)
}

func (s *Service) GetSellerFulfillment(ctx context.Context, sellerID, fulfillmentID uuid.UUID) (*Fulfillment, error) {
	return s.repo.GetSellerFulfillment(ctx, sellerID, fulfillmentID)
}

func (s *Service) ListAdminFulfillments(ctx context.Context, limit, offset int, status *string) ([]Fulfillment, error) {
	return s.repo.ListAdminFulfillments(ctx, limit, offset, status)
}

func (s *Service) GetAdminFulfillment(ctx context.Context, id uuid.UUID) (*Fulfillment, error) {
	return s.repo.GetAdminFulfillment(ctx, id)
}

func (s *Service) GetOrderFulfillments(ctx context.Context, orderID uuid.UUID) ([]Fulfillment, error) {
	return s.repo.GetOrderFulfillments(ctx, orderID)
}

func (s *Service) CustomerGetOrderFulfillments(ctx context.Context, customerID, orderID uuid.UUID) ([]Fulfillment, error) {
	// First ensure order belongs to customer
	order, err := s.ordersRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != customerID {
		return nil, ErrUnauthorized
	}
	_ = order

	return s.repo.GetOrderFulfillments(ctx, orderID)
}
