package fulfillment

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListSellerFulfillments(ctx context.Context, userID uuid.UUID, limit, offset int, status *string) ([]Fulfillment, error) {
	sellerID, err := s.ordersRepo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListSellerFulfillments(ctx, sellerID, limit, offset, status)
}

func (s *Service) GetSellerFulfillment(ctx context.Context, userID, fulfillmentID uuid.UUID) (*Fulfillment, error) {
	sellerID, err := s.ordersRepo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetSellerFulfillment(ctx, sellerID, fulfillmentID)
}

func (s *Service) MarkSellerFulfillmentAssembling(ctx context.Context, userID, fulfillmentID uuid.UUID) error {
	sellerID, err := s.ordersRepo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		f, err := s.repo.GetSellerFulfillmentTx(ctx, tx, sellerID, fulfillmentID)
		if err != nil {
			return err
		}

		if f.Status == "awaiting_payment" {
			return errors.New("Сборка ещё не оплачена.")
		}
		if f.Status == "shipped" || f.Status == "delivered" {
			return errors.New("Сборка уже отправлена или завершена.")
		}
		if f.Status != "paid" {
			return errors.New("Сборку нельзя перевести в этот статус.")
		}

		if err := s.repo.UpdateFulfillmentStatusTx(ctx, tx, fulfillmentID, "assembling"); err != nil {
			return err
		}

		return s.recalculateParentOrderStatusTx(ctx, tx, f.OrderID, userID)
	})
}

func (s *Service) MarkSellerFulfillmentPacked(ctx context.Context, userID, fulfillmentID uuid.UUID) error {
	sellerID, err := s.ordersRepo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		f, err := s.repo.GetSellerFulfillmentTx(ctx, tx, sellerID, fulfillmentID)
		if err != nil {
			return err
		}

		if f.Status == "awaiting_payment" {
			return errors.New("Сборка ещё не оплачена.")
		}
		if f.Status == "shipped" || f.Status == "delivered" {
			return errors.New("Сборка уже отправлена или завершена.")
		}
		if f.Status != "assembling" {
			return errors.New("Сборку нельзя перевести в этот статус.")
		}

		if err := s.repo.UpdateFulfillmentStatusTx(ctx, tx, fulfillmentID, "packed"); err != nil {
			return err
		}

		return s.recalculateParentOrderStatusTx(ctx, tx, f.OrderID, userID)
	})
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
