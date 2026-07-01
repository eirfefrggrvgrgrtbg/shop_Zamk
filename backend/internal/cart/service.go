package cart

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetCart(ctx context.Context, userID uuid.UUID) (*Cart, error) {
	cart, err := s.repo.GetCartByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrCartNotFound) {
			return s.repo.CreateCart(ctx, userID)
		}
		return nil, err
	}
	return cart, nil
}

func (s *Service) AddItem(ctx context.Context, userID uuid.UUID, req AddItemRequest) (*Cart, error) {
	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Validate product and variant
	info, err := s.repo.GetProductValidationInfo(ctx, req.ProductID, req.ProductVariantID)
	if err != nil {
		return nil, err
	}
	if info.Status != "published" {
		return nil, ErrProductNotPublished
	}
	if !info.VariantActive {
		return nil, ErrVariantNotFound
	}
	if info.Available < req.Quantity {
		return nil, ErrInsufficientStock
	}

	// Check if item already exists
	existingItem, err := s.repo.GetCartItem(ctx, cart.ID, req.ProductVariantID)
	if err != nil && !errors.Is(err, ErrCartItemNotFound) {
		return nil, err
	}

	if existingItem != nil {
		// Update quantity
		newQuantity := existingItem.Quantity + req.Quantity
		if info.Available < newQuantity {
			return nil, ErrInsufficientStock
		}
		if err := s.repo.UpdateItemQuantity(ctx, existingItem.ID, newQuantity); err != nil {
			return nil, err
		}
	} else {
		// Add new item
		item := &CartItem{
			ID:               uuid.New(),
			CartID:           cart.ID,
			ProductID:        req.ProductID,
			ProductVariantID: req.ProductVariantID,
			Quantity:         req.Quantity,
		}
		if err := s.repo.AddItem(ctx, item); err != nil {
			return nil, err
		}
	}

	return s.GetCart(ctx, userID)
}

func (s *Service) UpdateItemQuantity(ctx context.Context, userID, itemID uuid.UUID, req UpdateItemRequest) (*Cart, error) {
	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	var foundItem *CartItem
	for _, it := range cart.Items {
		if it.ID == itemID {
			foundItem = &it
			break
		}
	}
	if foundItem == nil {
		return nil, ErrCartItemNotFound
	}

	// Validate available stock
	info, err := s.repo.GetProductValidationInfo(ctx, foundItem.ProductID, foundItem.ProductVariantID)
	if err != nil {
		return nil, err
	}
	if info.Available < req.Quantity {
		return nil, ErrInsufficientStock
	}

	if err := s.repo.UpdateItemQuantity(ctx, itemID, req.Quantity); err != nil {
		return nil, err
	}

	return s.GetCart(ctx, userID)
}

func (s *Service) RemoveItem(ctx context.Context, userID, itemID uuid.UUID) (*Cart, error) {
	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.RemoveItem(ctx, cart.ID, itemID); err != nil {
		return nil, err
	}

	return s.GetCart(ctx, userID)
}

func (s *Service) ClearCart(ctx context.Context, userID uuid.UUID) error {
	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.ClearCart(ctx, cart.ID)
}
