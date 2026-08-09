package products

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/reviews"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo       *Repository
	sellerRepo *sellers.Repository
	dbPool     *postgres.Client
	reviews    *reviews.Service
	notifs     *notifications.Service
	rdb        *redis.Client
}

func NewService(repo *Repository, sellerRepo *sellers.Repository, dbPool *postgres.Client, reviewsSvc *reviews.Service, notifs *notifications.Service) *Service {
	return &Service{
		repo:       repo,
		sellerRepo: sellerRepo,
		dbPool:     dbPool,
		reviews:    reviewsSvc,
		notifs:     notifs,
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
		return productStatus != StatusBlocked
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
	if err := req.ValidateSKUs(); err != nil {
		return Product{}, err
	}

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
	var skusToCheck []string
	for _, vr := range req.Variants {
		v := ProductVariant{
			ID:           uuid.New(),
			ProductID:    p.ID,
			SKU:          vr.SKU,
			Size:         vr.Size,
			Color:        vr.Color,
			OptionValues: vr.OptionValues,
			Barcode:      vr.Barcode,
			PriceCents:   vr.PriceCents,
			InitialStock: vr.InitialStock,
			IsActive:     true,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if v.SKU != nil && strings.TrimSpace(*v.SKU) != "" {
			trimmed := strings.ToLower(strings.TrimSpace(*v.SKU))
			skusToCheck = append(skusToCheck, trimmed)
		}
		variants = append(variants, v)
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
		
		// Lock the seller to prevent concurrent SKU creation races
		if err := txRepo.LockSellerForUpdate(ctx, seller.ID); err != nil {
			return err
		}
		
		// Check for cross-product duplicate SKUs for this seller
		if len(skusToCheck) > 0 {
			existing, err := txRepo.FindExistingSellerSKUs(ctx, seller.ID, skusToCheck, nil)
			if err != nil {
				return err
			}
			if len(existing) > 0 {
				return &DuplicateSKUError{SKU: existing[0]}
			}
		}

		if err := txRepo.CreateProduct(ctx, p); err != nil {
			return err
		}
		if len(variants) > 0 {
			if err := txRepo.MergeProductVariants(ctx, p.ID, variants); err != nil {
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
	if err := req.ValidateSKUs(); err != nil {
		return Product{}, err
	}

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
	var skusToCheck []string
	if req.Variants != nil {
		now := time.Now()
		for _, vr := range req.Variants {
			v := ProductVariant{
				ID:         uuid.New(),
				ProductID:  p.ID,
				SKU:        vr.SKU,
				Size:       vr.Size,
				Color:      vr.Color,
				OptionValues: vr.OptionValues,
				Barcode:    vr.Barcode,
				PriceCents:   vr.PriceCents,
				InitialStock: vr.InitialStock,
				IsActive:     true,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if v.SKU != nil && strings.TrimSpace(*v.SKU) != "" {
				skusToCheck = append(skusToCheck, strings.ToLower(strings.TrimSpace(*v.SKU)))
			}
			variants = append(variants, v)
		}
	}

	needsModerationReset := false
	if p.Status == StatusPublished || p.Status == StatusApproved {
		needsModerationReset = true
	}

	var modLog *ProductModerationLog
	if needsModerationReset {
		oldStatus := p.Status
		p.Status = StatusPendingModeration
		now := time.Now()
		p.SubmittedAt = &now

		modLog = &ProductModerationLog{
			ID:         uuid.New(),
			ProductID:  p.ID,
			FromStatus: &oldStatus,
			ToStatus:   StatusPendingModeration,
			Comment:    func(s string) *string { return &s }("Автоматический сброс модерации при редактировании"),
			CreatedAt:  now,
		}
	}

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		if err := txRepo.LockSellerForUpdate(ctx, seller.ID); err != nil {
			return err
		}

		if len(skusToCheck) > 0 {
			var excludeVariantIDs []uuid.UUID
			for _, ev := range p.Variants {
				excludeVariantIDs = append(excludeVariantIDs, ev.ID)
			}
			existing, err := txRepo.FindExistingSellerSKUs(ctx, seller.ID, skusToCheck, excludeVariantIDs)
			if err != nil {
				return err
			}
			if len(existing) > 0 {
				return &DuplicateSKUError{SKU: existing[0]}
			}
		}

		if needsModerationReset {
			if err := txRepo.UpdateProductStatus(ctx, p); err != nil {
				return err
			}
			if err := txRepo.AddModerationLog(ctx, modLog); err != nil {
				return err
			}
		}

		if err := txRepo.UpdateProduct(ctx, p); err != nil {
			return err
		}
		if req.Variants != nil {
			if err := txRepo.MergeProductVariants(ctx, p.ID, variants); err != nil {
				return err
			}
			p.Variants = variants
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
		if err := txRepo.AddModerationLog(ctx, log); err != nil {
			return err
		}
		if s.notifs != nil {
			_ = s.notifs.CreateStaffNotificationTx(ctx, tx, notifications.Notification{
				Type:       notifications.TypeProductModerationSubmitted,
				Title:      "Новый товар на модерацию",
				Body:       "Товар " + p.Title + " ожидает модерации.",
				EntityType: "product",
				EntityID:   p.ID,
			})
		}
		return nil
	})
}

// ---------------------------------------------------------
// Admin Moderation Operations
// ---------------------------------------------------------

func (s *Service) ListAdminProducts(ctx context.Context, filter AdminProductFilter, limit, offset int) (ProductListResponse, error) {
	items, totalCount, err := s.repo.ListAdminProducts(ctx, filter, limit, offset)
	if err != nil {
		return ProductListResponse{}, err
	}
	if items == nil {
		items = []Product{}
	}
	return ProductListResponse{Items: items, TotalCount: totalCount}, nil
}

func (s *Service) GetAdminProductDetail(ctx context.Context, productID uuid.UUID) (Product, error) {
	p, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		return Product{}, err
	}
	return *p, nil
}

func (s *Service) AdminUpdateProduct(ctx context.Context, adminID uuid.UUID, productID uuid.UUID, req UpdateProductRequest) (Product, error) {
	p, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		return Product{}, err
	}

	if req.Title != nil {
		p.Title = *req.Title
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

	p.UpdatedAt = time.Now()

	if err := s.repo.UpdateProduct(ctx, p); err != nil {
		return Product{}, err
	}

	// Create moderation log entry
	comment := "Администратор обновил данные товара"
	fromSt := p.Status
	_ = s.repo.AddModerationLog(ctx, &ProductModerationLog{
		ID:         uuid.New(),
		ProductID:  productID,
		FromStatus: &fromSt,
		ToStatus:   p.Status,
		Comment:    &comment,
		CreatedAt:  time.Now(),
	})

	return *p, nil
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

func (s *Service) GetAdminProductModerationHistory(ctx context.Context, productID uuid.UUID) ([]ProductModerationLog, error) {
	// Verify product exists
	_, err := s.repo.GetProductByID(ctx, productID)
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
		if err := txRepo.AddModerationLog(ctx, log); err != nil {
			return err
		}
		if s.notifs != nil {
			var notifType, title, body string
			if toStatus == StatusApproved {
				notifType = notifications.TypeProductApproved
				title = "Товар одобрен"
				body = "Ваш товар " + p.Title + " прошел модерацию и одобрен."
			} else if toStatus == StatusRejected {
				notifType = notifications.TypeProductRejected
				title = "Товар отклонен"
				body = "Ваш товар " + p.Title + " не прошел модерацию."
				if comment != nil {
					body += " Причина: " + *comment
				}
			}
			if notifType != "" {
				_ = s.notifs.CreateNotificationTx(ctx, tx, notifications.Notification{
					RecipientSellerID: &p.SellerID,
					RecipientKind:     notifications.RecipientKindSeller,
					Type:              notifType,
					Title:             title,
					Body:              body,
					EntityType:        "product",
					EntityID:          p.ID,
				})
			}
		}
		return nil
	})
}

func (s *Service) ApproveProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusApproved, comment, []string{StatusPendingModeration, StatusInReview}, func(p *Product, t time.Time) {
		p.ApprovedAt = &t
	})
}

func (s *Service) RejectProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment string) error {
	if comment == "" {
		return ErrRejectionReasonRequired
	}
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusRejected, &comment, []string{StatusPendingModeration, StatusInReview}, func(p *Product, t time.Time) {
		p.RejectedAt = &t
	})
}

func (s *Service) PublishProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	p, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		return err
	}

	if p.Status != StatusApproved && p.Status != StatusHidden {
		return ErrInvalidStatusTransition
	}

	elig := ValidatePublishEligibility(p)
	if !elig.IsEligible {
		return &ErrNotPublishable{Reasons: elig.EligibilityReasons}
	}

	now := time.Now()
	fromStatus := p.Status
	p.Status = StatusPublished
	p.PublishedAt = &now
	p.UpdatedAt = now
	p.ModerationComment = comment

	if err := s.repo.UpdateProductStatus(ctx, p); err != nil {
		return err
	}

	_ = s.repo.AddModerationLog(ctx, &ProductModerationLog{
		ID:          uuid.New(),
		ProductID:   p.ID,
		AdminUserID: &adminUserID,
		FromStatus:  &fromStatus,
		ToStatus:    StatusPublished,
		Comment:     comment,
		CreatedAt:   now,
	})

	return nil
}

func (s *Service) HideProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusHidden, comment, []string{StatusPublished, StatusApproved}, nil)
}

func (s *Service) BlockProduct(ctx context.Context, adminUserID, productID uuid.UUID, comment *string) error {
	// Block can happen from any state except already blocked/deleted
	return s.applyModerationTransition(ctx, adminUserID, productID, StatusBlocked, comment, []string{StatusDraft, StatusPendingModeration, StatusInReview, StatusApproved, StatusPublished, StatusRejected, StatusHidden, StatusOutOfStock}, nil)
}

// ---------------------------------------------------------
// Public Operations
// ---------------------------------------------------------

func mapToPublicProduct(p Product) PublicProduct {
	var sellerSlug, sellerName string
	if p.SellerSlug != nil {
		sellerSlug = *p.SellerSlug
	}
	if p.SellerName != nil {
		sellerName = *p.SellerName
	}

	pub := PublicProduct{
		ID:               p.ID,
		SellerID:         p.SellerID,
		SellerSlug:       sellerSlug,
		SellerName:       sellerName,
		CategoryID:       p.CategoryID,
		BrandID:          p.BrandID,
		Title:            p.Title,
		Slug:             p.Slug,
		Description:      p.Description,
		Status:           p.Status,
		Gender:           p.Gender,
		Color:            p.Color,
		Material:         p.Material,
		CareInstructions: p.CareInstructions,
		PriceCents:       p.PriceCents,
		OldPriceCents:    p.OldPriceCents,
		Currency:         p.Currency,
		MainImageURL:     p.MainImageURL,
		AverageRating:    p.AverageRating,
		ReviewsCount:     p.ReviewsCount,
		InStock:          p.InStock,
		CreatedAt:        p.CreatedAt,
		Rating:           p.Rating,
	}

	for _, v := range p.Variants {
		pub.Variants = append(pub.Variants, PublicProductVariant{
			ID:         v.ID,
			ProductID:  v.ProductID,
			Size:       v.Size,
			Color:      v.Color,
			PriceCents: v.PriceCents,
			IsActive:   v.IsActive,
			InStock:    v.InStock,
		})
	}

	for _, i := range p.Images {
		pub.Images = append(pub.Images, PublicProductImage{
			ID:        i.ID,
			ProductID: i.ProductID,
			ImageURL:  i.ImageURL,
			AltText:   i.AltText,
			SortOrder: i.SortOrder,
		})
	}

	return pub
}

func (s *Service) ListPublicProducts(ctx context.Context, filter PublicProductFilter, limit, offset int) (PublicProductListResponse, error) {
	items, totalCount, err := s.repo.ListPublishedProducts(ctx, filter, limit, offset)
	if err != nil {
		return PublicProductListResponse{}, err
	}

	var pubItems []PublicProduct
	for i := range items {
		if s.reviews != nil {
			summary, err := s.reviews.GetRatingSummary(ctx, items[i].ID)
			if err == nil && summary != nil {
				items[i].Rating = &RatingSummary{
					Average: summary.Average,
					Count:   summary.Count,
				}
			}
		}
		pubItems = append(pubItems, mapToPublicProduct(items[i]))
	}

	if pubItems == nil {
		pubItems = []PublicProduct{}
	}

	return PublicProductListResponse{Items: pubItems, TotalCount: totalCount}, nil
}

func (s *Service) GetPublicProduct(ctx context.Context, idOrSlug string) (PublicProduct, error) {
	p, err := s.repo.GetPublishedProductBySlugOrID(ctx, idOrSlug)
	if err != nil {
		return PublicProduct{}, err
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

	return mapToPublicProduct(*p), nil
}
