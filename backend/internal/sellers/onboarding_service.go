package sellers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) InviteSeller(ctx context.Context, req *InviteSellerRequest) (*InviteSellerResponse, error) {
	existingUser, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, ErrDuplicateEmail
	} else if err != nil && !errors.Is(err, users.ErrNotFound) {
		return nil, err
	}

	hashedPassword, err := auth.HashPassword(req.TemporaryPassword)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &users.User{
		ID:                 uuid.New(),
		Name:               fmt.Sprintf("%s %s", req.FirstName, req.LastName),
		Email:              req.Email,
		PasswordHash:       hashedPassword,
		Role:               users.RoleSeller,
		Status:             users.StatusActive,
		MustChangePassword: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	brandName := "Draft Store"
	slug := "draft-" + uuid.New().String()[:8]
	email := req.Email
	seller := &Seller{
		ID:           uuid.New(),
		BrandName:    &brandName,
		Slug:         &slug,
		ContactEmail: &email,
		Status:       StatusPendingSetup,
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

	app := &SellerOnboardingApplication{
		ID:          uuid.New(),
		SellerID:    seller.ID,
		Status:      OnboardingStatusNotStarted,
		CurrentStep: 1,
		Payload: OnboardingPayload{
			Owner: OwnerPayload{
				FirstName:  req.FirstName,
				LastName:   req.LastName,
				MiddleName: req.MiddleName,
				Phone:      req.Phone,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
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

		if err := txSellerRepo.CreateOnboardingApplication(ctx, app); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &InviteSellerResponse{
		SellerID:                  seller.ID,
		OwnerUserID:               user.ID,
		TemporaryPasswordReturned: true,
	}, nil
}

func (s *Service) GetOnboardingApplication(ctx context.Context, sellerID uuid.UUID) (*SellerOnboardingApplication, error) {
	return s.repo.GetOnboardingApplicationBySellerID(ctx, sellerID)
}

func (s *Service) UpdateOnboardingStep(ctx context.Context, sellerID uuid.UUID, req *UpdateOnboardingStepRequest) (*SellerOnboardingApplication, error) {
	var updatedApp *SellerOnboardingApplication

	err := s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		
		app, err := txRepo.GetOnboardingApplicationBySellerIDForUpdate(ctx, sellerID)
		if err != nil {
			return err
		}

		if app.Status == OnboardingStatusApproved {
			return errors.New("cannot update approved onboarding application")
		}

		app.CurrentStep = req.CurrentStep
		app.Payload = req.Payload
		
		if app.Status == OnboardingStatusNotStarted {
			app.Status = OnboardingStatusDraft
		}

		app.UpdatedAt = time.Now()

		if err := txRepo.UpdateOnboardingApplication(ctx, app); err != nil {
			return err
		}

		updatedApp = app
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedApp, nil
}

func (s *Service) SubmitOnboarding(ctx context.Context, sellerID uuid.UUID, req *SubmitOnboardingRequest) (*SellerOnboardingApplication, error) {
	var submittedApp *SellerOnboardingApplication

	err := s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		
		app, err := txRepo.GetOnboardingApplicationBySellerIDForUpdate(ctx, sellerID)
		if err != nil {
			return err
		}

		if app.Status == OnboardingStatusApproved || app.Status == OnboardingStatusPendingReview {
			return errors.New("application is already submitted or approved")
		}

		existingSeller, err := txRepo.GetSellerBySlug(ctx, req.Payload.Store.Slug)
		if err == nil && existingSeller.ID != sellerID {
			return ErrStoreSlugTaken
		} else if err != nil && !errors.Is(err, ErrSellerNotFound) {
			return err
		}
		
		brandTaken, err := txRepo.IsBrandSlugTaken(ctx, req.Payload.Brand.Slug)
		if err != nil {
			return err
		}
		if brandTaken {
			return ErrBrandSlugTaken
		}

		app.Payload = req.Payload
		app.Status = OnboardingStatusPendingReview
		now := time.Now()
		app.SubmittedAt = &now
		app.UpdatedAt = now

		if err := txRepo.UpdateOnboardingApplication(ctx, app); err != nil {
			return err
		}

		submittedApp = app
		return nil
	})

	if err != nil {
		return nil, err
	}

	return submittedApp, nil
}

func (s *Service) RequestChangesOnboarding(ctx context.Context, adminID uuid.UUID, id uuid.UUID, req *RequestChangesOnboardingRequest) error {
	return s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		app, err := txRepo.GetOnboardingApplicationByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if app.Status != OnboardingStatusPendingReview {
			return errors.New("can only request changes from pending_review status")
		}

		app.Status = OnboardingStatusChangesRequested
		app.ReviewComment = &req.ReviewComment
		now := time.Now()
		app.ReviewedAt = &now
		app.ReviewedBy = &adminID
		app.UpdatedAt = now

		return txRepo.UpdateOnboardingApplication(ctx, app)
	})
}

func (s *Service) RejectOnboarding(ctx context.Context, adminID uuid.UUID, id uuid.UUID, req *RejectOnboardingRequest) error {
	return s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		app, err := txRepo.GetOnboardingApplicationByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		
		if app.Status == OnboardingStatusApproved {
			return errors.New("cannot reject approved application")
		}

		app.Status = OnboardingStatusRejected
		app.ReviewComment = &req.ReviewComment
		now := time.Now()
		app.ReviewedAt = &now
		app.ReviewedBy = &adminID
		app.UpdatedAt = now

		return txRepo.UpdateOnboardingApplication(ctx, app)
	})
}

func (s *Service) ApproveOnboarding(ctx context.Context, adminID uuid.UUID, id uuid.UUID, req *ApproveOnboardingRequest) error {
	return s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		app, err := txRepo.GetOnboardingApplicationByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if app.Status == OnboardingStatusApproved {
			return nil // idempotent
		}
		if app.Status != OnboardingStatusPendingReview {
			return errors.New("can only approve pending_review application")
		}

		// Check Store Slug
		existingSeller, err := txRepo.GetSellerBySlug(ctx, app.Payload.Store.Slug)
		if err == nil && existingSeller.ID != app.SellerID {
			return ErrStoreSlugTaken
		} else if err != nil && !errors.Is(err, ErrSellerNotFound) {
			return err
		}

		// Check Brand Slug
		brandTaken, err := txRepo.IsBrandSlugTaken(ctx, app.Payload.Brand.Slug)
		if err != nil {
			return err
		}
		if brandTaken {
			return ErrBrandSlugTaken
		}

		// Update Seller (apply store data)
		seller, err := txRepo.GetSellerByID(ctx, app.SellerID)
		if err != nil {
			return err
		}
		brandName := app.Payload.Store.Name
		slug := app.Payload.Store.Slug
		seller.BrandName = &brandName
		seller.Slug = &slug
		seller.Description = &app.Payload.Store.Description
		seller.Status = StatusActive
		seller.UpdatedAt = time.Now()
		
		if err := txRepo.UpdateSeller(ctx, seller); err != nil {
			return err
		}

		// Create Brand
		brandID := uuid.New()
		err = txRepo.CreateBrand(ctx, brandID, app.Payload.Brand.Name, app.Payload.Brand.Slug, app.Payload.Brand.Description)
		if err != nil {
			return err
		}

		// Link Seller <-> Brand
		err = txRepo.CreateSellerBrand(ctx, &SellerBrand{
			ID:               uuid.New(),
			SellerID:         seller.ID,
			BrandID:          brandID,
			IsPrimary:        true,
			RelationshipType: "owner",
			Status:           "active",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		})
		if err != nil {
			return err
		}

		app.Status = OnboardingStatusApproved
		if req.ReviewComment != "" {
			app.ReviewComment = &req.ReviewComment
		}
		now := time.Now()
		app.ReviewedAt = &now
		app.ReviewedBy = &adminID
		app.UpdatedAt = now

		return txRepo.UpdateOnboardingApplication(ctx, app)
	})
}

func (s *Service) ListOnboardingApplications(ctx context.Context, status string) ([]SellerOnboardingApplication, error) {
	return s.repo.ListOnboardingApplications(ctx, status)
}

func (s *Service) GetAdminOnboardingApplication(ctx context.Context, id uuid.UUID) (*SellerOnboardingApplication, error) {
	return s.repo.GetOnboardingApplicationByID(ctx, id)
}
