package reviews

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

type Service struct {
	repo       *Repository
	ordersRepo *orders.Repository
	sellerRepo *sellers.Repository
	db         *postgres.Client
	notifSvc   *notifications.Service
}

func NewService(repo *Repository, ordersRepo *orders.Repository, sellerRepo *sellers.Repository, db *postgres.Client, notifSvc *notifications.Service) *Service {
	return &Service{
		repo:       repo,
		ordersRepo: ordersRepo,
		sellerRepo: sellerRepo,
		db:         db,
		notifSvc:   notifSvc,
	}
}

func (s *Service) CreateReview(ctx context.Context, userID uuid.UUID, orderItemID uuid.UUID, pathOrderID *uuid.UUID, req CreateReviewRequest) (*ProductReview, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, ErrInvalidRating
	}

	var reviewText *string
	if req.Text != nil {
		reviewText = req.Text
	} else if req.Comment != nil {
		reviewText = req.Comment
	}

	if reviewText != nil && len(*reviewText) > 1000 {
		return nil, ErrReviewTextTooLong
	}

	var review *ProductReview
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Get order item and verify existence & quantity
		targetItem, err := s.ordersRepo.GetOrderItemByID(ctx, tx, orderItemID)
		if err != nil {
			return ErrItemNotPurchased
		}
		if targetItem.Quantity <= 0 {
			return ErrItemNotPurchased
		}

		// 2. Verify order belongs to user and is delivered
		order, err := s.ordersRepo.GetOrder(ctx, targetItem.OrderID)
		if err != nil {
			return err
		}
		if order.UserID != userID {
			return ErrItemNotPurchased
		}
		if pathOrderID != nil && order.ID != *pathOrderID {
			return ErrItemNotPurchased
		}
		if order.Status != "delivered" {
			return ErrOrderNotDelivered
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
			ProductTitle:     &targetItem.Title,
			OrderID:          order.ID,
			OrderItemID:      orderItemID,
			UserID:           userID,
			SellerID:         targetItem.SellerID,
			Rating:           req.Rating,
			Title:            req.Title,
			Comment:          reviewText,
			Status:           "pending_moderation",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := s.repo.CreateReview(ctx, tx, review); err != nil {
			return err
		}

		if s.notifSvc != nil {
			_ = s.notifSvc.CreateStaffNotificationTx(ctx, tx, notifications.Notification{
				Type:       "review.created",
				EntityType: "review",
				EntityID:   review.ID,
				Title:      "New Product Review",
				Body:       "A new review requires moderation.",
			})
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
		if err := s.repo.LogModeration(ctx, tx, log); err != nil {
			return err
		}

		if err := s.repo.RecalculateProductRating(ctx, tx, review.ProductID); err != nil {
			return err
		}
		if err := s.repo.RecalculateSellerRating(ctx, tx, review.SellerID); err != nil {
			return err
		}

		if s.notifSvc != nil {
			if toStatus == "published" {
				_ = s.notifSvc.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientKind:   notifications.RecipientKindCustomer,
					RecipientUserID: &review.UserID,
					Type:            "review.published",
					EntityType:      "review",
					EntityID:        review.ID,
					Title:           "Your review is published",
					Body:            "Your review has been approved and published.",
				})
				_ = s.notifSvc.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientKind:     notifications.RecipientKindSeller,
					RecipientSellerID: &review.SellerID,
					Type:              "review.published_seller",
					EntityType:        "review",
					EntityID:          review.ID,
					Title:             "New Product Review",
					Body:              "A customer left a new review on your product.",
				})
			} else if toStatus == "rejected" {
				_ = s.notifSvc.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientKind:   notifications.RecipientKindCustomer,
					RecipientUserID: &review.UserID,
					Type:            "review.rejected",
					EntityType:      "review",
					EntityID:        review.ID,
					Title:           "Your review was rejected",
					Body:            "Your review didn't meet our guidelines.",
				})
			}
		}

		return nil
	})
}

func (s *Service) GetCustomerReviews(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ProductReview, error) {
	return s.repo.ListReviews(ctx, map[string]interface{}{"user_id": userID}, limit, offset)
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

func (s *Service) GetAdminReviews(ctx context.Context, status string, limit, offset int) ([]ProductReview, error) {
	filters := make(map[string]interface{})
	if status != "" {
		filters["status"] = status
	}
	return s.repo.ListReviews(ctx, filters, limit, offset)
}

func (s *Service) GetAdminReviewByID(ctx context.Context, reviewID uuid.UUID) (*ProductReview, error) {
	return s.repo.GetReviewByID(ctx, nil, reviewID)
}

func (s *Service) GetSellerReviews(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ProductReview, error) {
	seller, _, err := s.sellerRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListReviews(ctx, map[string]interface{}{"seller_id": seller.ID, "status": "published"}, limit, offset)
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

func (s *Service) ResolvePublishedProductID(ctx context.Context, idOrSlug string) (uuid.UUID, error) {
	return s.repo.ResolvePublishedProductID(ctx, idOrSlug)
}

func (s *Service) GetPublicProductReviews(ctx context.Context, productID uuid.UUID, limit, offset int) ([]PublicReviewRow, error) {
	return s.repo.ListPublicReviews(ctx, productID, limit, offset)
}

func (s *Service) GetRatingSummary(ctx context.Context, productID uuid.UUID) (*RatingSummaryResponse, error) {
	return s.repo.GetRatingSummary(ctx, productID)
}
