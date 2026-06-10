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
		Status:       StatusPending, // New sellers start as pending; admin must activate them
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

func (s *Service) UpdateSellerProfile(ctx context.Context, currentUserID uuid.UUID, req *UpdateSellerProfileRequest) (*SellerMeResponse, error) {
	seller, _, err := s.repo.GetSellerByUserID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}

	if req.Slug != nil && *req.Slug != "" {
		existing, slugErr := s.repo.GetSellerBySlug(ctx, *req.Slug)
		if slugErr == nil && existing != nil && existing.ID != seller.ID {
			return nil, ErrDuplicateSlug
		}
	}

	if err := s.repo.UpdateSellerProfile(ctx, seller.ID, req); err != nil {
		return nil, err
	}

	return s.GetSellerMe(ctx, currentUserID)
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

// ---- Phase E: Seller Management Service Methods ----

const (
	baseCommissionBps    = 900  // 9%
	penaltyCommissionBps = 1800 // 18%
)

// GetSellerDetail returns full seller detail for admin view.
func (s *Service) GetSellerDetail(ctx context.Context, sellerID uuid.UUID) (*SellerDetailResponse, error) {
	d, err := s.repo.GetSellerDetailByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}

	resp := &SellerDetailResponse{
		ID:        d.ID,
		BrandName: d.BrandName,
		Status:    string(d.Status),
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	slug := d.Slug
	if slug != "" {
		resp.Slug = &slug
	}
	resp.Description = d.Description
	resp.LogoURL = d.LogoURL
	email := d.ContactEmail
	if email != "" {
		resp.ContactEmail = &email
	}
	resp.ContactPhone = d.ContactPhone

	resp.Owner.ID = d.OwnerID
	resp.Owner.Name = d.OwnerName
	resp.Owner.Email = d.OwnerEmail
	resp.Owner.Status = d.OwnerStatus

	resp.Counts.WarningsActive = d.WarningsActive
	resp.Counts.ViolationsActive = d.ViolationsActive
	resp.Counts.ActivePenaltyViolations = d.ActivePenaltyViolations

	resp.CommissionPolicy.BaseCommissionBps = baseCommissionBps
	resp.CommissionPolicy.PenaltyCommissionBps = penaltyCommissionBps
	resp.CommissionPolicy.PenaltyRule = "2 active penalty violations triggers 18% for 1 month"
	resp.CommissionPolicy.CurrentAppliedCommissionBps = baseCommissionBps
	resp.CommissionPolicy.AutomaticPenaltyEnabled = false

	return resp, nil
}

// UpdateSellerStatusWithHistory changes seller status and writes status history.
func (s *Service) UpdateSellerStatusWithHistory(ctx context.Context, sellerID uuid.UUID, newStatus string, reason *string, actorUserID uuid.UUID) error {
	switch SellerStatus(newStatus) {
	case StatusPending, StatusActive, StatusBlocked, StatusArchived:
		// valid
	default:
		return errors.New("invalid status")
	}

	if (newStatus == string(StatusBlocked) || newStatus == string(StatusArchived)) && (reason == nil || *reason == "") {
		return ErrReasonRequired
	}

	seller, err := s.repo.GetSellerByID(ctx, sellerID)
	if err != nil {
		return err
	}

	oldStatus := string(seller.Status)
	if err := s.repo.UpdateSellerStatus(ctx, sellerID, SellerStatus(newStatus)); err != nil {
		return err
	}

	actor := actorUserID
	return s.repo.WriteStatusHistory(ctx, sellerID, &oldStatus, newStatus, reason, &actor)
}

// VerifySeller verifies a pending seller.
func (s *Service) VerifySeller(ctx context.Context, sellerID uuid.UUID, actorUserID uuid.UUID) (*VerifySellerResponse, error) {
	seller, err := s.repo.GetSellerByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}

	if seller.Status != StatusPending {
		return nil, ErrSellerNotPending
	}

	var missing []string
	if seller.BrandName == "" {
		missing = append(missing, "brandName")
	}
	if seller.Slug == "" {
		missing = append(missing, "slug")
	}
	if seller.Description == nil || len(*seller.Description) < 10 {
		missing = append(missing, "description")
	}
	if seller.ContactEmail == "" && (seller.ContactPhone == nil || *seller.ContactPhone == "") {
		missing = append(missing, "contactEmail or contactPhone")
	}

	if len(missing) > 0 {
		return nil, &VerifyMissingFieldsError{Fields: missing}
	}

	oldStatus := string(seller.Status)
	if err := s.repo.UpdateSellerStatus(ctx, sellerID, StatusActive); err != nil {
		return nil, err
	}

	actor := actorUserID
	_ = s.repo.WriteStatusHistory(ctx, sellerID, &oldStatus, string(StatusActive), nil, &actor)

	return &VerifySellerResponse{
		SellerID: sellerID,
		Status:   string(StatusActive),
	}, nil
}

// GetStatusHistory returns seller status timeline.
func (s *Service) GetStatusHistory(ctx context.Context, sellerID uuid.UUID) ([]SellerStatusHistoryItem, error) {
	return s.repo.GetStatusHistory(ctx, sellerID)
}

// CreateWarning creates a seller warning.
func (s *Service) CreateWarning(ctx context.Context, sellerID uuid.UUID, req CreateWarningRequest, actorUserID uuid.UUID) (*WarningResponse, error) {
	actor := actorUserID
	return s.repo.CreateWarning(ctx, CreateWarningInput{
		SellerID:    sellerID,
		Type:        req.Type,
		Title:       req.Title,
		Message:     req.Message,
		Severity:    req.Severity,
		ActorUserID: &actor,
	})
}

// ListWarnings lists warnings for a seller.
func (s *Service) ListWarnings(ctx context.Context, sellerID uuid.UUID) ([]WarningResponse, error) {
	return s.repo.ListWarnings(ctx, sellerID)
}

// ResolveWarning resolves a warning.
func (s *Service) ResolveWarning(ctx context.Context, sellerID uuid.UUID, warningID uuid.UUID, req ResolveWarningRequest, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateWarningStatus(ctx, warningID, "resolved", &actor, req.ResolutionNote)
}

// CancelWarning cancels a warning.
func (s *Service) CancelWarning(ctx context.Context, sellerID uuid.UUID, warningID uuid.UUID, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateWarningStatus(ctx, warningID, "cancelled", &actor, nil)
}

// CreateViolation creates a seller violation.
func (s *Service) CreateViolation(ctx context.Context, sellerID uuid.UUID, req CreateViolationRequest, actorUserID uuid.UUID) (*ViolationResponse, error) {
	actor := actorUserID
	return s.repo.CreateViolation(ctx, CreateViolationInput{
		SellerID:         sellerID,
		Type:             req.Type,
		Title:            req.Title,
		Description:      req.Description,
		Severity:         req.Severity,
		CountsForPenalty: req.CountsForPenalty,
		ActorUserID:      &actor,
	})
}

// ListViolations lists violations for a seller.
func (s *Service) ListViolations(ctx context.Context, sellerID uuid.UUID) ([]ViolationResponse, error) {
	return s.repo.ListViolations(ctx, sellerID)
}

// ResolveViolation resolves a violation.
func (s *Service) ResolveViolation(ctx context.Context, sellerID uuid.UUID, violationID uuid.UUID, req ResolveViolationRequest, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateViolationStatus(ctx, violationID, "resolved", &actor, req.ResolutionNote)
}

// CancelViolation cancels a violation.
func (s *Service) CancelViolation(ctx context.Context, sellerID uuid.UUID, violationID uuid.UUID, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateViolationStatus(ctx, violationID, "cancelled", &actor, nil)
}
