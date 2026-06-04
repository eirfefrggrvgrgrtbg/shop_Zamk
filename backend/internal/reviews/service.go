package reviews

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

type Service struct {
	repo       *Repository
	ordersRepo *orders.Repository
	sellerRepo *sellers.Repository
	db         *postgres.Client
}

func NewService(repo *Repository, ordersRepo *orders.Repository, sellerRepo *sellers.Repository, db *postgres.Client) *Service {
	return &Service{
		repo:       repo,
		ordersRepo: ordersRepo,
		sellerRepo: sellerRepo,
		db:         db,
	}
}

func (s *Service) CreateReview(ctx context.Context, userID, orderID, orderItemID uuid.UUID, req CreateReviewRequest) (*ProductReview, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, ErrInvalidRating
	}

	var review *ProductReview
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// Verify order belongs to user and is delivered
		order, err := s.ordersRepo.GetOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if order.UserID != userID {
			return ErrItemNotPurchased
		}
		if order.Status != "delivered" {
			return ErrOrderNotDelivered
		}

		// Verify order item
		items, err := s.ordersRepo.GetOrderItems(ctx, orderID)
		if err != nil {
			return err
		}
		var targetItem *orders.OrderItem
		for _, it := range items {
			if it.ID == orderItemID {
				targetItem = &it
				break
			}
		}
		if targetItem == nil {
			return ErrItemNotPurchased
		}

		var variantID *uuid.UUID
		if targetItem.ProductVariantID != uuid.Nil {
			vid := targetItem.ProductVariantID
			variantID = &vid
		}

		now := time.Now()
		review = &ProductReview{
			ID:               uuid.New(),
			ProductID:        targetItem.ProductID,
			ProductVariantID: variantID,
			OrderID:          orderID,
			OrderItemID:      orderItemID,
			UserID:           userID,
			SellerID:         targetItem.SellerID,
			Rating:           req.Rating,
			Title:            req.Title,
			Comment:          req.Comment,
			Status:           "pending_moderation",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := s.repo.CreateReview(ctx, tx, review); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return review, nil
}

func (s *Service) ModerateReview(ctx context.Context, adminID, reviewID uuid.UUID, action string, comment *string) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		review, err := s.repo.GetReviewByID(ctx, tx, reviewID)
		if err != nil {
			return err
		}

		fromStatus := review.Status
		toStatus := action // action matches status (published, rejected, hidden, blocked)

		var publishedAt *time.Time
		var rejectedAt *time.Time
		now := time.Now()

		if toStatus == "published" {
			publishedAt = &now
		} else if toStatus == "rejected" {
			rejectedAt = &now
		}

		if err := s.repo.UpdateReviewStatus(ctx, tx, reviewID, toStatus, publishedAt, rejectedAt, comment); err != nil {
			return err
		}

		log := &ProductReviewModerationLog{
			ID:          uuid.New(),
			ReviewID:    reviewID,
			AdminUserID: &adminID,
			FromStatus:  &fromStatus,
			ToStatus:    toStatus,
			Comment:     comment,
			CreatedAt:   now,
		}
		return s.repo.LogModeration(ctx, tx, log)
	})
}

func (s *Service) GetCustomerReviews(ctx context.Context, userID uuid.UUID) ([]ProductReview, error) {
	return s.repo.ListReviews(ctx, map[string]interface{}{"user_id": userID})
}

func (s *Service) GetCustomerReviewByID(ctx context.Context, userID, reviewID uuid.UUID) (*ProductReview, error) {
	rev, err := s.repo.GetReviewByID(ctx, nil, reviewID)
	if err != nil {
		return nil, err
	}
	if rev.UserID != userID {
		return nil, ErrReviewNotFound
	}
	return rev, nil
}

func (s *Service) GetAdminReviews(ctx context.Context, status string) ([]ProductReview, error) {
	filters := make(map[string]interface{})
	if status != "" {
		filters["status"] = status
	}
	return s.repo.ListReviews(ctx, filters)
}

func (s *Service) GetAdminReviewByID(ctx context.Context, reviewID uuid.UUID) (*ProductReview, error) {
	return s.repo.GetReviewByID(ctx, nil, reviewID)
}

func (s *Service) GetSellerReviews(ctx context.Context, userID uuid.UUID) ([]ProductReview, error) {
	seller, _, err := s.sellerRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListReviews(ctx, map[string]interface{}{"seller_id": seller.ID})
}

func (s *Service) GetSellerReviewByID(ctx context.Context, userID, reviewID uuid.UUID) (*ProductReview, error) {
	seller, _, err := s.sellerRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rev, err := s.repo.GetReviewByID(ctx, nil, reviewID)
	if err != nil {
		return nil, err
	}
	if rev.SellerID != seller.ID {
		return nil, ErrReviewNotFound
	}
	return rev, nil
}

func (s *Service) GetPublicProductReviews(ctx context.Context, productID uuid.UUID) ([]ProductReview, error) {
	return s.repo.ListReviews(ctx, map[string]interface{}{"product_id": productID, "status": "published"})
}

func (s *Service) GetRatingSummary(ctx context.Context, productID uuid.UUID) (*RatingSummaryResponse, error) {
	return s.repo.GetRatingSummary(ctx, productID)
}
