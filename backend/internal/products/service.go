package products

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/reviews"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

type Service struct {
	repo       *Repository
	sellerRepo *sellers.Repository
	dbPool     *postgres.Client
	reviews    *reviews.Service
}

func NewService(repo *Repository, sellerRepo *sellers.Repository, dbPool *postgres.Client, reviewsSvc *reviews.Service) *Service {
	return &Service{
		repo:       repo,
		sellerRepo: sellerRepo,
		dbPool:     dbPool,
		reviews:    reviewsSvc,
	}
}

// ---------------------------------------------------------
// Helper: Resolve Seller Ownership
// ---------------------------------------------------------

func (s *Service) getSellerForUser(ctx context.Context, userID uuid.UUID) (*sellers.Seller, error) {
	seller, _, err := s.sellerRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sellers.ErrSellerUserNotFound) {
			return nil, ErrSellerNotFound
		}
		return nil, err
	}
	return seller, nil
}

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	return slug
}

func CanEditProduct(sellerStatus sellers.SellerStatus, productStatus string) bool {
	if sellerStatus == sellers.StatusBlocked || sellerStatus == sellers.StatusArchived {
		return false
	}
	if sellerStatus == sellers.StatusPending || sellerStatus == sellers.StatusActive {
		return productStatus == StatusDraft || productStatus == StatusRejected
	}
	return false
}

func CanSubmitProduct(sellerStatus sellers.SellerStatus, productStatus string) bool {
	if sellerStatus == sellers.StatusActive {
		return productStatus == StatusDraft || productStatus == StatusRejected
	}
	return false
}

// ---------------------------------------------------------
// Seller Operations
// ---------------------------------------------------------

func (s *Service) CreateProductForSeller(ctx context.Context, currentUserID uuid.UUID, req CreateProductRequest) (Product, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return Product{}, err
	}

	if seller.Status == sellers.StatusBlocked || seller.Status == sellers.StatusArchived {
		return Product{}, ErrSellerBlocked
	}

	slug := req.Title
	if req.Slug != nil && *req.Slug != "" {
		slug = *req.Slug
	}
	slug = generateSlug(slug)

	now := time.Now()
	p := &Product{
		ID:               uuid.New(),
		SellerID:         seller.ID,
		CategoryID:       req.CategoryID,
		BrandID:          req.BrandID,
		Title:            req.Title,
		Slug:             slug,
		Description:      req.Description,
		Status:           StatusDraft,
		Gender:           req.Gender,
		Color:            req.Color,
		Material:         req.Material,
		CareInstructions: req.CareInstructions,
		PriceCents:       req.PriceCents,
		OldPriceCents:    req.OldPriceCents,
		Currency:         req.Currency,
		MainImageURL:     req.MainImageURL,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	var variants []ProductVariant
	for _, vr := range req.Variants {
		variants = append(variants, ProductVariant{
			ID:         uuid.New(),
			ProductID:  p.ID,
			SKU:        vr.SKU,
			Size:       vr.Size,
			Color:      vr.Color,
			Barcode:    vr.Barcode,
			PriceCents: vr.PriceCents,
			IsActive:   true,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	var images []ProductImage
	for i, ir := range req.Images {
		sortOrder := i
		if ir.SortOrder != nil {
			sortOrder = *ir.SortOrder
		}
		images = append(images, ProductImage{
			ID:        uuid.New(),
			ProductID: p.ID,
			ImageURL:  ir.ImageURL,
			AltText:   ir.AltText,
			SortOrder: sortOrder,
			CreatedAt: now,
		})
	}

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.CreateProduct(ctx, p); err != nil {
			return err
		}
		if len(variants) > 0 {
			if err := txRepo.ReplaceProductVariants(ctx, p.ID, variants); err != nil {
				return err
			}
		}
		if len(images) > 0 {
			if err := txRepo.ReplaceProductImages(ctx, p.ID, images); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return Product{}, err
	}

	p.Variants = variants
	p.Images = images
	return *p, nil
}

func (s *Service) UpdateProductForSeller(ctx context.Context, currentUserID uuid.UUID, productID uuid.UUID, req UpdateProductRequest) (Product, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return Product{}, err
	}

	if seller.Status == sellers.StatusBlocked || seller.Status == sellers.StatusArchived {
		return Product{}, ErrSellerBlocked
	}

	p, err := s.repo.GetProductByIDForSeller(ctx, productID, seller.ID)
	if err != nil {
		return Product{}, err
	}

	if !CanEditProduct(seller.Status, p.Status) {
		return Product{}, fmt.Errorf("%w: cannot edit product in status %s with seller status %s", ErrProductNotEditable, p.Status, seller.Status)
	}

	if req.Title != nil {
		p.Title = *req.Title
	}
	if req.Slug != nil {
		p.Slug = generateSlug(*req.Slug)
	}
	if req.Description != nil {
		p.Description = req.Description
	}
	if req.CategoryID != nil {
		p.CategoryID = req.CategoryID
	}
	if req.BrandID != nil {
		p.BrandID = req.BrandID
	}
	if req.Gender != nil {
		p.Gender = req.Gender
	}
	if req.Color != nil {
		p.Color = req.Color
	}
	if req.Material != nil {
		p.Material = req.Material
	}
	if req.CareInstructions != nil {
		p.CareInstructions = req.CareInstructions
	}
	if req.PriceCents != nil {
		p.PriceCents = *req.PriceCents
	}
	if req.OldPriceCents != nil {
		p.OldPriceCents = req.OldPriceCents
	}
	if req.MainImageURL != nil {
		p.MainImageURL = req.MainImageURL
	}

	var variants []ProductVariant
	if req.Variants != nil {
		now := time.Now()
		for _, vr := range req.Variants {
			variants = append(variants, ProductVariant{
				ID:         uuid.New(),
				ProductID:  p.ID,
				SKU:        vr.SKU,
				Size:       vr.Size,
				Color:      vr.Color,
				Barcode:    vr.Barcode,
				PriceCents: vr.PriceCents,
				IsActive:   true,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	}

	var images []ProductImage
	if req.Images != nil {
		now := time.Now()
		for i, ir := range req.Images {
			sortOrder := i
			if ir.SortOrder != nil {
				sortOrder = *ir.SortOrder
			}
			images = append(images, ProductImage{
				ID:        uuid.New(),
				ProductID: p.ID,
				ImageURL:  ir.ImageURL,
				AltText:   ir.AltText,
				SortOrder: sortOrder,
				CreatedAt: now,
			})
		}
	}

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.UpdateProduct(ctx, p); err != nil {
			return err
		}
		if req.Variants != nil {
			if err := txRepo.ReplaceProductVariants(ctx, p.ID, variants); err != nil {
				return err
			}
			p.Variants = variants
		}
		if req.Images != nil {
			if err := txRepo.ReplaceProductImages(ctx, p.ID, images); err != nil {
				return err
			}
			p.Images = images
		}
		return nil
	})

	if err != nil {
		return Product{}, err
	}
	return *p, nil
}

func (s *Service) ListSellerProducts(ctx context.Context, currentUserID uuid.UUID, limit, offset int) (ProductListResponse, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return ProductListResponse{}, err
	}

	items, err := s.repo.ListProductsBySeller(ctx, seller.ID, limit, offset)
	if err != nil {
		return ProductListResponse{}, err
	}
	if items == nil {
		items = []Product{}
	}
	return ProductListResponse{Items: items, TotalCount: len(items)}, nil
}

func (s *Service) GetSellerProduct(ctx context.Context, currentUserID, productID uuid.UUID) (Product, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return Product{}, err
	}

	p, err := s.repo.GetProductByIDForSeller(ctx, productID, seller.ID)
	if err != nil {
		return Product{}, err
	}
	return *p, nil
}

func (s *Service) DeleteSellerDraftProduct(ctx context.Context, currentUserID, productID uuid.UUID) error {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return err
	}

	if seller.Status == sellers.StatusBlocked || seller.Status == sellers.StatusArchived {
		return ErrSellerBlocked
	}

	return s.repo.DeleteDraftProduct(ctx, productID, seller.ID)
}

func (s *Service) SubmitProductToModeration(ctx context.Context, currentUserID, productID uuid.UUID, req SubmitProductModerationRequest) error {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return err
	}

	if seller.Status == sellers.StatusBlocked || seller.Status == sellers.StatusArchived {
		return ErrSellerBlocked
	}
	if seller.Status == sellers.StatusPending {
		return ErrSellerNotActive
	}

	p, err := s.repo.GetProductByIDForSeller(ctx, productID, seller.ID)
	if err != nil {
		return err
	}

	if !CanSubmitProduct(seller.Status, p.Status) {
		return fmt.Errorf("%w: can only submit draft or rejected products", ErrInvalidStatusTransition)
	}

	fromStatus := p.Status
	now := time.Now()
	p.Status = StatusPendingModeration
	p.SubmittedAt = &now

	log := &ProductModerationLog{
		ID:         uuid.New(),
		ProductID:  p.ID,
		FromStatus: &fromStatus,
		ToStatus:   StatusPendingModeration,
		Comment:    req.Comment,
		CreatedAt:  now,
	}

	return s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.UpdateProductStatus(ctx, p); err != nil {
			return err
		}
		return txRepo.AddModerationLog(ctx, log)
	})
}

// ---------------------------------------------------------
// Admin Moderation Operations
// ---------------------------------------------------------

func (s *Service) ListAdminProducts(ctx context.Context, limit, offset int) (ProductListResponse, error) {
	items, err := s.repo.ListAllProducts(ctx, limit, offset)
	if err != nil {
		return ProductListResponse{}, err
	}
	if items == nil {
		items = []Product{}
	}
	return ProductListResponse{Items: items, TotalCount: len(items)}, nil
}

func (s *Service) ListProductsForModeration(ctx context.Context, limit, offset int) (ProductListResponse, error) {
	items, err := s.repo.ListProductsForModeration(ctx, limit, offset)
	if err != nil {
		return ProductListResponse{}, err
	}
	if items == nil {
		items = []Product{}
	}
	return ProductListResponse{Items: items, TotalCount: len(items)}, nil
}

func (s *Service) GetProductModerationHistory(ctx context.Context, sellerID, productID uuid.UUID) ([]ProductModerationLog, error) {
	// First verify the seller owns the product
	_, err := s.repo.GetProductByIDForSeller(ctx, productID, sellerID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListProductModerationLogs(ctx, productID)
}

func (s *Service) applyModerationTransition(ctx context.Context, adminUserID, productID uuid.UUID, toStatus string, comment *string, allowedFromStatuses []string, timeFieldSetter func(*Product, time.Time)) error {
	p, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		return err
	}

	validFrom := false
	for _, s := range allowedFromStatuses {
		if p.Status == s {
			validFrom = true
			break
		}
	}
	if !validFrom {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatusTransition, p.Status, toStatus)
	}

	fromStatus := p.Status
	now := time.Now()
	p.Status = toStatus
	p.ModerationComment = comment
	if timeFieldSetter != nil {
		timeFieldSetter(p, now)
	}

	log := &ProductModerationLog{
		ID:          uuid.New(),
		ProductID:   p.ID,
		AdminUserID: &adminUserID,
		FromStatus:  &fromStatus,
		ToStatus:    toStatus,
		Comment:     comment,
		CreatedAt:   now,
	}

	return s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.UpdateProductStatus(ctx, p); err != nil {
			return err
		}
		return txRepo.AddModerationLog(ctx, log)
	})
}

func (s *Service) ApproveProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusApproved, comment, []string{StatusPendingModeration}, func(p *Product, t time.Time) {
		p.ApprovedAt = &t
	})
}

func (s *Service) RejectProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment string) error {
	if comment == "" {
		return ErrRejectionReasonRequired
	}
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusRejected, &comment, []string{StatusPendingModeration}, func(p *Product, t time.Time) {
		p.RejectedAt = &t
	})
}

func (s *Service) PublishProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusPublished, comment, []string{StatusApproved, StatusHidden}, func(p *Product, t time.Time) {
		p.PublishedAt = &t
	})
}

func (s *Service) HideProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusHidden, comment, []string{StatusPublished, StatusApproved}, nil)
}

func (s *Service) BlockProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	// Block can happen from any state except already blocked/deleted
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusBlocked, comment, []string{StatusDraft, StatusPendingModeration, StatusApproved, StatusPublished, StatusRejected, StatusHidden, StatusOutOfStock}, nil)
}

// ---------------------------------------------------------
// Public Operations
// ---------------------------------------------------------

func (s *Service) ListPublicProducts(ctx context.Context, filter PublicProductFilter, limit, offset int) (ProductListResponse, error) {
	items, totalCount, err := s.repo.ListPublishedProducts(ctx, filter, limit, offset)
	if err != nil {
		return ProductListResponse{}, err
	}
	if items == nil {
		items = []Product{}
	}
	
	if s.reviews != nil {
		for i := range items {
			summary, err := s.reviews.GetRatingSummary(ctx, items[i].ID)
			if err == nil && summary != nil {
				items[i].Rating = &RatingSummary{
					Average: summary.Average,
					Count:   summary.Count,
				}
			}
		}
	}
	
	return ProductListResponse{Items: items, TotalCount: totalCount}, nil
}

func (s *Service) GetPublicProduct(ctx context.Context, idOrSlug string) (Product, error) {
	p, err := s.repo.GetPublishedProductBySlugOrID(ctx, idOrSlug)
	if err != nil {
		return Product{}, err
	}
	
	if s.reviews != nil {
		summary, err := s.reviews.GetRatingSummary(ctx, p.ID)
		if err == nil && summary != nil {
			p.Rating = &RatingSummary{
				Average: summary.Average,
				Count:   summary.Count,
			}
		}
	}
	
	return *p, nil
}
