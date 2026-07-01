package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

type Service struct {
	repo *Repository
	userRepo *users.Repository
}

func NewService(repo *Repository, userRepo *users.Repository) *Service {
	return &Service{repo: repo, userRepo: userRepo}
}

// CreateNotificationTx creates a single notification, skipping if deduplication rule applies.
func (s *Service) CreateNotificationTx(ctx context.Context, tx pgx.Tx, n Notification) error {
	// Deduplication: Prevent duplicate notification of the same type for the same entity + recipientKind
	exists, err := s.repo.CheckExistsTx(ctx, tx, n.RecipientKind, n.Type, n.EntityType, n.EntityID, n.RecipientUserID)
	if err != nil {
		return err
	}
	if exists {
		return nil // skip
	}

	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	if n.Metadata == nil {
		n.Metadata = map[string]interface{}{}
	}

	return s.repo.CreateNotificationTx(ctx, tx, &n)
}

func (s *Service) CreateManyNotificationsTx(ctx context.Context, tx pgx.Tx, ns []Notification) error {
	for _, n := range ns {
		if err := s.CreateNotificationTx(ctx, tx, n); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CreateStaffNotificationTx(ctx context.Context, tx pgx.Tx, n Notification) error {
	staffIDs, err := s.userRepo.GetStaffUserIDs(ctx)
	if err != nil {
		return err
	}
	n.RecipientKind = RecipientKindStaff
	for _, id := range staffIDs {
		nCopy := n
		nCopy.RecipientUserID = &id
		if err := s.CreateNotificationTx(ctx, tx, nCopy); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListForCustomer(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Notification, int, error) {
	return s.repo.ListNotifications(ctx, &userID, nil, RecipientKindCustomer, limit, offset)
}

func (s *Service) ListForSeller(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Notification, int, error) {
	sellerID, err := s.repo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.ListNotifications(ctx, nil, &sellerID, RecipientKindSeller, limit, offset)
}

func (s *Service) ListForStaff(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Notification, int, error) {
	return s.repo.ListNotifications(ctx, &userID, nil, RecipientKindStaff, limit, offset)
}

func (s *Service) MarkReadCustomer(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.MarkRead(ctx, id, &userID, nil, RecipientKindCustomer)
}

func (s *Service) MarkReadSeller(ctx context.Context, id, userID uuid.UUID) error {
	sellerID, err := s.repo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.MarkRead(ctx, id, nil, &sellerID, RecipientKindSeller)
}

func (s *Service) MarkReadStaff(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return s.repo.MarkRead(ctx, id, &userID, nil, RecipientKindStaff)
}

func (s *Service) MarkAllReadCustomer(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, &userID, nil, RecipientKindCustomer)
}

func (s *Service) MarkAllReadSeller(ctx context.Context, userID uuid.UUID) error {
	sellerID, err := s.repo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.MarkAllRead(ctx, nil, &sellerID, RecipientKindSeller)
}

func (s *Service) MarkAllReadStaff(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, &userID, nil, RecipientKindStaff)
}

func (s *Service) CountUnreadCustomer(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.CountUnread(ctx, &userID, nil, RecipientKindCustomer)
}

func (s *Service) CountUnreadSeller(ctx context.Context, userID uuid.UUID) (int, error) {
	sellerID, err := s.repo.GetSellerIDByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return s.repo.CountUnread(ctx, nil, &sellerID, RecipientKindSeller)
}

func (s *Service) CountUnreadStaff(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.CountUnread(ctx, &userID, nil, RecipientKindStaff)
}
