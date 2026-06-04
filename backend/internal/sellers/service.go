package sellers

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo     *Repository
	userRepo *users.Repository
	dbClient *postgres.Client
}

func NewService(repo *Repository, userRepo *users.Repository, dbClient *postgres.Client) *Service {
	return &Service{
		repo:     repo,
		userRepo: userRepo,
		dbClient: dbClient,
	}
}

func (s *Service) CreateSellerByAdmin(ctx context.Context, req *CreateSellerRequest) (*CreateSellerResponse, error) {
	// Check for duplicate email
	existingUser, err := s.userRepo.GetUserByEmail(ctx, req.OwnerEmail)
	if err == nil && existingUser != nil {
		return nil, ErrDuplicateEmail
	} else if err != nil && !errors.Is(err, users.ErrNotFound) {
		return nil, err
	}

	slug := generateSlug(req.BrandName)
	if req.Slug != nil && *req.Slug != "" {
		slug = *req.Slug
	}

	existingSeller, err := s.repo.GetSellerBySlug(ctx, slug)
	if err == nil && existingSeller != nil {
		return nil, ErrDuplicateSlug
	} else if err != nil && !errors.Is(err, ErrSellerNotFound) {
		return nil, err
	}

	hashedPassword, err := auth.HashPassword(req.TemporaryPassword)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &users.User{
		ID:                 uuid.New(),
		Name:               req.OwnerName,
		Email:              req.OwnerEmail,
		PasswordHash:       hashedPassword,
		Role:               users.RoleSeller,
		Status:             users.StatusActive,
		MustChangePassword: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	seller := &Seller{
		ID:           uuid.New(),
		BrandName:    req.BrandName,
		Slug:         slug,
		Description:  req.Description,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	sellerUser := &SellerUser{
		ID:        uuid.New(),
		SellerID:  seller.ID,
		UserID:    user.ID,
		Role:      RoleOwner,
		CreatedAt: now,
	}

	err = s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
		txUserRepo := s.userRepo.WithTx(tx)
		if err := txUserRepo.CreateUser(ctx, user); err != nil {
			return err
		}

		txSellerRepo := s.repo.WithTx(tx)
		if err := txSellerRepo.CreateSeller(ctx, seller); err != nil {
			return err
		}

		if err := txSellerRepo.CreateSellerUser(ctx, sellerUser); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Do not return password hash
	return &CreateSellerResponse{
		Seller: *seller,
		OwnerUser: *user,
		TemporaryPasswordReturned: false,
	}, nil
}

func (s *Service) ListSellers(ctx context.Context, limit, offset int) (*ListSellersResponse, error) {
	items, err := s.repo.ListSellers(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	// For MVP, just returning items. Pagination total count needs a count query which we skip for MVP.
	return &ListSellersResponse{
		Items:      items,
		TotalCount: len(items), 
	}, nil
}

func (s *Service) UpdateSellerStatus(ctx context.Context, id uuid.UUID, req *UpdateSellerStatusRequest) error {
	switch req.Status {
	case StatusPending, StatusActive, StatusBlocked, StatusArchived:
		// Valid
	default:
		return errors.New("invalid status")
	}

	return s.repo.UpdateSellerStatus(ctx, id, req.Status)
}

func (s *Service) GetSellerMe(ctx context.Context, currentUserID uuid.UUID) (*SellerMeResponse, error) {
	seller, sellerUser, err := s.repo.GetSellerByUserID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUserByID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}

	return &SellerMeResponse{
		Seller:     *seller,
		SellerUser: *sellerUser,
		User:       *user,
	}, nil
}

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
