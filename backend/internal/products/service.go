package products

import (
	"encoding/json"
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

	if req.MaterialComposition != nil {
		if err := s.validateMaterialComposition(ctx, req.MaterialComposition); err != nil {
			return Product{}, err
		}
	}
	if req.CategoryID != nil {
		if err := s.validateSizeChart(ctx, *req.CategoryID, req.SizeChartRows); err != nil {
			return Product{}, err
		}
		if err := s.validateAttributes(ctx, *req.CategoryID, req.Attributes, req.Variants); err != nil {
			return Product{}, err
		}
	}

	primaryBrandID, err := s.repo.GetPrimaryBrandForSeller(ctx, seller.ID)
	if err != nil {
		return Product{}, err
	}

	now := time.Now()
	p := &Product{
		ID:               uuid.New(),
		SellerID:         seller.ID,
		CategoryID:       req.CategoryID,
		BrandID:          primaryBrandID,
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
		Currency:         func() string { if req.Currency == "" { return "RUB" }; return req.Currency }(),
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
			SellerSKU:    vr.SellerSKU,
			ColorID:      vr.ColorID,
			SizeValueID:  vr.SizeValueID,
			ShadeName:    vr.ShadeName,
			Barcode:      vr.Barcode,
			PriceCents:   vr.PriceCents,
			InitialStock: vr.InitialStock,
			IsActive:     true,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if v.Barcode == nil || *v.Barcode == "" {
				generated := "ZMK-" + uuid.New().String()[:12]
				v.Barcode = &generated
			}
			if v.Barcode == nil || *v.Barcode == "" {
				generated := "ZMK-" + uuid.New().String()[:12]
				v.Barcode = &generated
			}
			if v.SellerSKU != nil && strings.TrimSpace(*v.SellerSKU) != "" {
			trimmed := strings.ToLower(strings.TrimSpace(*v.SellerSKU))
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

	if req.MaterialComposition != nil {
		if err := s.validateMaterialComposition(ctx, req.MaterialComposition); err != nil {
			return Product{}, err
		}
	}
	if p.CategoryID != nil {
		if err := s.validateSizeChart(ctx, *p.CategoryID, req.SizeChartRows); err != nil {
			return Product{}, err
		}
		if err := s.validateAttributes(ctx, *p.CategoryID, req.Attributes, req.Variants); err != nil {
			return Product{}, err
		}
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
	primaryBrandID, err := s.repo.GetPrimaryBrandForSeller(ctx, seller.ID)
	if err == nil {
		p.BrandID = primaryBrandID
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
	
	getVariantID := func(id *uuid.UUID) uuid.UUID {
		if id != nil && *id != uuid.Nil {
			return *id
		}
		return uuid.New()
	}

	if req.Variants != nil {
		now := time.Now()
		for _, vr := range req.Variants {
			v := ProductVariant{
				ID:         getVariantID(vr.ID),
				ProductID:  p.ID,
				SKU:        vr.SKU,
				Size:       vr.Size,
				Color:      vr.Color,
				OptionValues: vr.OptionValues,
			SellerSKU:    vr.SellerSKU,
			ColorID:      vr.ColorID,
			SizeValueID:  vr.SizeValueID,
			ShadeName:    vr.ShadeName,
				Barcode:    vr.Barcode,
				PriceCents:   vr.PriceCents,
				InitialStock: vr.InitialStock,
				IsActive:     true,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			
			if v.Barcode == nil || *v.Barcode == "" {
				generated := "ZMK-" + uuid.New().String()[:12]
				v.Barcode = &generated
				
			}
			if v.SellerSKU != nil && strings.TrimSpace(*v.SellerSKU) != "" {
				skusToCheck = append(skusToCheck, strings.ToLower(strings.TrimSpace(*v.SellerSKU)))
			}
			variants = append(variants, v)
		}
	}

	var modLog *ProductModerationLog
	var revision *ProductRevision

	if p.Status == StatusPublished || p.Status == StatusApproved {
		now := time.Now()
		revID := uuid.New()
				// Construct full target state for snapshot
		if req.Variants != nil {
			p.Variants = variants
		}
		if req.Attributes != nil {
			var pAttrs []ProductAttributeValue
			for _, a := range req.Attributes {
				pAttrs = append(pAttrs, ProductAttributeValue{
					AttributeDefinitionID: a.AttributeDefinitionID,
					EnumValueID:           a.EnumValueID,
					TextValue:             a.TextValue,
					NumberValue:           a.NumberValue,
					BoolValue:             a.BoolValue,
				})
			}
			p.Attributes = pAttrs
		}
		if req.MaterialComposition != nil {
			var comps []ProductMaterialComposition
			for _, c := range req.MaterialComposition {
				comps = append(comps, ProductMaterialComposition{
					MaterialID: c.MaterialID,
					Percentage: c.Percentage,
				})
			}
			p.MaterialComposition = comps
		}
		if req.SizeChartRows != nil && p.CategoryID != nil {
			chart := ProductSizeChart{
				ProductID: p.ID,
				CategoryID: *p.CategoryID,
			}
			for _, r := range req.SizeChartRows {
				chart.Rows = append(chart.Rows, ProductSizeChartRow{
					SizeValueID:  r.SizeValueID,
					Measurements: r.Measurements,
				})
			}
			p.SizeChart = &chart
		}
		
		// For variants attributes
		if req.Variants != nil {
			for i, v := range p.Variants {
				for _, reqV := range req.Variants {
					if (reqV.SellerSKU != nil && v.SellerSKU != nil && *reqV.SellerSKU == *v.SellerSKU) || (reqV.SKU != nil && v.SKU != nil && *reqV.SKU == *v.SKU) {
						if reqV.Attributes != nil {
							var vAttrs []VariantAttributeValue
							for _, a := range reqV.Attributes {
								vAttrs = append(vAttrs, VariantAttributeValue{
									AttributeDefinitionID: a.AttributeDefinitionID,
									EnumValueID:           a.EnumValueID,
									TextValue:             a.TextValue,
									NumberValue:           a.NumberValue,
									BoolValue:             a.BoolValue,
								})
							}
							p.Variants[i].Attributes = vAttrs
						}
						break
					}
				}
			}
		}


		var snap map[string]interface{}
		b, _ := json.Marshal(p)
		json.Unmarshal(b, &snap)

		revision = &ProductRevision{
			ID: revID,
			ProductID: p.ID,
			Status: "pending",
			ContentSnapshot: snap,
			CreatedAt: now,
			UpdatedAt: now,
		}

		p.LiveRevisionID = &revID


		if req.ContinueSelling == nil || !*req.ContinueSelling {
			oldStatus := p.Status
			p.Status = StatusPendingModeration
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
	}

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		if err := txRepo.LockSellerForUpdate(ctx, seller.ID); err != nil {
			return err
		}

		if len(skusToCheck) > 0 {
			var excludeVariantIDs []uuid.UUID
			existingVariants, err := txRepo.GetProductVariants(ctx, p.ID)
			if err != nil {
				return err
			}
			for _, ev := range existingVariants {
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

		if modLog != nil {
			if err := txRepo.UpdateProductStatus(ctx, p); err != nil {
				return err
			}
			if err := txRepo.AddModerationLog(ctx, modLog); err != nil {
				return err
			}
		}

		if revision != nil {
			revQuery := `INSERT INTO product_revisions (id, product_id, status, content_snapshot, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
			_, err := tx.Exec(ctx, revQuery, revision.ID, revision.ProductID, revision.Status, revision.ContentSnapshot, revision.CreatedAt, revision.UpdatedAt)
			if err != nil {
				return fmt.Errorf("failed to insert revision: %w", err)
			}
			
			if modLog != nil {
				// ContinueSelling=false -> hide it
				updQuery := `UPDATE products SET live_revision_id = $1, status = $2, submitted_at = $3, updated_at = now() WHERE id = $4`
				if _, err := tx.Exec(ctx, updQuery, revision.ID, StatusPendingModeration, p.SubmittedAt, p.ID); err != nil {
					return fmt.Errorf("failed to link revision and update status: %w", err)
				}
			} else {
				// ContinueSelling=true -> keep published
				updQuery := `UPDATE products SET live_revision_id = $1, updated_at = now() WHERE id = $2`
				if _, err := tx.Exec(ctx, updQuery, revision.ID, p.ID); err != nil {
					return fmt.Errorf("failed to link revision to product: %w", err)
				}
			}
		} else {
			if err := txRepo.UpdateProduct(ctx, p); err != nil {
				return err
			}
			
			// Update product attributes
			if req.Attributes != nil {
				var pAttrs []ProductAttributeValue
				for _, a := range req.Attributes {
					pAttrs = append(pAttrs, ProductAttributeValue{
						AttributeDefinitionID: a.AttributeDefinitionID,
						EnumValueID:           a.EnumValueID,
						TextValue:             a.TextValue,
						NumberValue:           a.NumberValue,
						BoolValue:             a.BoolValue,
					})
				}
				if err := txRepo.InsertProductAttributeValues(ctx, p.ID, pAttrs); err != nil {
					return err
				}
			}
			
			// Update composition
			if req.MaterialComposition != nil {
				var comps []ProductMaterialComposition
				for _, c := range req.MaterialComposition {
					comps = append(comps, ProductMaterialComposition{
						MaterialID: c.MaterialID,
						Percentage: c.Percentage,
					})
				}
				if err := txRepo.InsertMaterialComposition(ctx, p.ID, comps); err != nil {
					return err
				}
			}
			
			// Update size chart
			if req.SizeChartRows != nil && p.CategoryID != nil {
				var rows []ProductSizeChartRow
				for _, r := range req.SizeChartRows {
					rows = append(rows, ProductSizeChartRow{
						SizeValueID:  r.SizeValueID,
						Measurements: r.Measurements,
					})
				}
				if err := txRepo.InsertSizeChart(ctx, p.ID, *p.CategoryID, rows); err != nil {
					return err
				}
			}
		}
		
		if req.Variants != nil && revision == nil {
			if err := txRepo.MergeProductVariants(ctx, p.ID, variants); err != nil {
				return err
			}
			
			// Update variant attributes
			for _, v := range variants {
				for _, reqV := range req.Variants {
					if (reqV.SellerSKU != nil && v.SellerSKU != nil && *reqV.SellerSKU == *v.SellerSKU) || (reqV.SKU != nil && v.SKU != nil && *reqV.SKU == *v.SKU) {
						if reqV.Attributes != nil {
							var vAttrs []VariantAttributeValue
							for _, a := range reqV.Attributes {
								vAttrs = append(vAttrs, VariantAttributeValue{
									AttributeDefinitionID: a.AttributeDefinitionID,
									EnumValueID:           a.EnumValueID,
									TextValue:             a.TextValue,
									NumberValue:           a.NumberValue,
									BoolValue:             a.BoolValue,
								})
							}
							if err := txRepo.InsertVariantAttributeValues(ctx, v.ID, vAttrs); err != nil {
								return err
							}
						}
						break
					}
				}
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


func (s *Service) UpdateVariantPricesForSeller(ctx context.Context, currentUserID uuid.UUID, productID uuid.UUID, req UpdateVariantPricesRequest) (Product, error) {
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

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		for variantID, priceCents := range req.Prices {
			found := false
			for _, v := range p.Variants {
				if v.ID == variantID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("variant %s not found in product %s", variantID, p.ID)
			}
			if err := txRepo.UpdateVariantPrice(ctx, variantID, priceCents); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return Product{}, err
	}
	
	updated, err := s.repo.GetProductByIDForSeller(ctx, productID, seller.ID)
	if err != nil {
		return Product{}, err
	}
	return *updated, nil
}

func (s *Service) validateMaterialComposition(ctx context.Context, comp []ProductMaterialCompositionRequest) error {
	if len(comp) == 0 {
		return nil
	}
	var total float64
	seen := make(map[uuid.UUID]bool)
	for _, c := range comp {
		if seen[c.MaterialID] {
			return fmt.Errorf("duplicate material entry")
		}
		seen[c.MaterialID] = true
		total += c.Percentage

		var isActive bool
		err := s.dbPool.Pool.QueryRow(ctx, "SELECT is_active FROM materials WHERE id = $1", c.MaterialID).Scan(&isActive)
		if err != nil {
			return fmt.Errorf("unknown material %s", c.MaterialID)
		}
		if !isActive {
			return fmt.Errorf("inactive material %s", c.MaterialID)
		}
	}
	if total != 100.0 {
		return fmt.Errorf("material composition must sum to 100, got %f", total)
	}
	return nil
}


func (s *Service) validateSizeChart(ctx context.Context, categoryID uuid.UUID, rows []ProductSizeChartRowRequest) error {
	var required bool
	err := s.dbPool.Pool.QueryRow(ctx, "SELECT size_chart_required FROM categories WHERE id = $1", categoryID).Scan(&required)
	if err != nil {
		return err
	}
	if required && len(rows) == 0 {
		return errors.New("category requires a size chart")
	}
	if len(rows) == 0 {
		return nil
	}
	
	// Fetch schema
	fieldRows, err := s.dbPool.Pool.Query(ctx, "SELECT code, is_required FROM category_size_chart_fields WHERE category_id = $1", categoryID)
	if err != nil {
		return err
	}
	defer fieldRows.Close()
	
	schema := make(map[string]bool)
	for fieldRows.Next() {
		var code string
		var req bool
		if err := fieldRows.Scan(&code, &req); err != nil {
			return err
		}
		schema[code] = req
	}
	fieldRows.Close() // explicitly close
	
	seenSizes := make(map[uuid.UUID]bool)
	for _, r := range rows {
		if seenSizes[r.SizeValueID] {
			return fmt.Errorf("duplicate size row for size_value_id: %s", r.SizeValueID)
		}
		seenSizes[r.SizeValueID] = true
		
		var active bool
		err := s.dbPool.Pool.QueryRow(ctx, "SELECT is_active FROM size_values WHERE id = $1", r.SizeValueID).Scan(&active)
		if err != nil || !active {
			return fmt.Errorf("size_value_id %s is inactive or does not exist", r.SizeValueID)
		}
		
		// Check required fields
		for code, req := range schema {
			if req {
				if _, ok := r.Measurements[code]; !ok {
					return fmt.Errorf("missing required measurement %s in size chart row", code)
				}
			}
		}
		
		// Check that all provided fields are in schema and numeric > 0
		for k, v := range r.Measurements {
			if _, ok := schema[k]; !ok {
				return fmt.Errorf("measurement %s is not valid for this category", k)
			}
			
			// Validate numeric > 0
			valFloat, ok := v.(float64)
			if !ok {
				return fmt.Errorf("measurement %s must be a number", k)
			}
			if valFloat <= 0 {
				return fmt.Errorf("measurement %s must be greater than 0", k)
			}
		}
	}

	return nil
}


func (s *Service) validateAttributes(ctx context.Context, categoryID uuid.UUID, productAttrs []ProductAttributeValueRequest, variants []ProductVariantRequest) error {
	// Fetch schema
	rows, err := s.dbPool.Pool.Query(ctx, `
		SELECT ad.id, ad.value_type, ad.scope, cad.required, cad.dictionary_id, cad.min_values, cad.max_values
		FROM category_attribute_definitions cad
		JOIN attribute_definitions ad ON cad.attribute_definition_id = ad.id
		WHERE cad.category_id = $1 AND ad.is_active = true
	`, categoryID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type schemaDef struct {
		ID           uuid.UUID
		ValueType    string
		Scope        string
		Required     bool
		DictionaryID *uuid.UUID
		MinValues    *int
		MaxValues    *int
	}

	schemaMap := make(map[uuid.UUID]schemaDef)
	for rows.Next() {
		var d schemaDef
		if err := rows.Scan(&d.ID, &d.ValueType, &d.Scope, &d.Required, &d.DictionaryID, &d.MinValues, &d.MaxValues); err != nil {
			return err
		}
		schemaMap[d.ID] = d
	}
	rows.Close() // explicitly close connection before making more queries!

	// Validate Product Attributes
	pAttrMap := make(map[uuid.UUID]int)
	for _, a := range productAttrs {
		def, ok := schemaMap[a.AttributeDefinitionID]
		if !ok {
			return fmt.Errorf("attribute %s is not valid for this category or inactive", a.AttributeDefinitionID)
		}
		if def.Scope != "PRODUCT" {
			return fmt.Errorf("attribute %s has PRODUCT scope but definition is %s", a.AttributeDefinitionID, def.Scope)
		}
		
		if def.ValueType == "ENUM" || def.ValueType == "MULTI_ENUM" {
			if a.EnumValueID == nil {
				return fmt.Errorf("attribute %s requires an enum value", a.AttributeDefinitionID)
			}
			var active bool
			err := s.dbPool.Pool.QueryRow(ctx, "SELECT is_active FROM attribute_dictionary_values WHERE id = $1 AND dictionary_id = $2", *a.EnumValueID, def.DictionaryID).Scan(&active)
			if err != nil || !active {
				return fmt.Errorf("enum value %s is invalid or inactive for attribute %s", *a.EnumValueID, a.AttributeDefinitionID)
			}
		}
		pAttrMap[a.AttributeDefinitionID]++
	}

	for id, def := range schemaMap {
		if def.Scope == "PRODUCT" {
			count := pAttrMap[id]
			if def.Required && count == 0 {
				return fmt.Errorf("required product attribute missing: %s", id)
			}
			if def.MinValues != nil && count < *def.MinValues {
				return fmt.Errorf("product attribute %s requires at least %d values", id, *def.MinValues)
			}
			if def.MaxValues != nil && count > *def.MaxValues {
				return fmt.Errorf("product attribute %s requires at most %d values", id, *def.MaxValues)
			}
		}
	}

	// Validate Variant Attributes
	for i, v := range variants {
		vAttrMap := make(map[uuid.UUID]int)
		for _, a := range v.Attributes {
			def, ok := schemaMap[a.AttributeDefinitionID]
			if !ok {
				return fmt.Errorf("attribute %s is not valid for this category or inactive", a.AttributeDefinitionID)
			}
			if def.Scope != "VARIANT" {
				return fmt.Errorf("attribute %s has VARIANT scope but definition is %s", a.AttributeDefinitionID, def.Scope)
			}
			if def.ValueType == "ENUM" || def.ValueType == "MULTI_ENUM" {
				if a.EnumValueID == nil {
					return fmt.Errorf("attribute %s requires an enum value", a.AttributeDefinitionID)
				}
				var active bool
				err := s.dbPool.Pool.QueryRow(ctx, "SELECT is_active FROM attribute_dictionary_values WHERE id = $1 AND dictionary_id = $2", *a.EnumValueID, def.DictionaryID).Scan(&active)
				if err != nil || !active {
					return fmt.Errorf("enum value %s is invalid or inactive for attribute %s", *a.EnumValueID, a.AttributeDefinitionID)
				}
			}
			vAttrMap[a.AttributeDefinitionID]++
		}
		
		for id, def := range schemaMap {
			if def.Scope == "VARIANT" {
				count := vAttrMap[id]
				if def.Required && count == 0 {
					return fmt.Errorf("required variant attribute missing: %s on variant %d", id, i)
				}
				if def.MinValues != nil && count < *def.MinValues {
					return fmt.Errorf("variant attribute %s requires at least %d values", id, *def.MinValues)
				}
				if def.MaxValues != nil && count > *def.MaxValues {
					return fmt.Errorf("variant attribute %s requires at most %d values", id, *def.MaxValues)
				}
			}
		}
	}
	
	return nil
}

// --- Reference Data Read Contracts ---

func (s *Service) GetCategoryAttributeSchema(ctx context.Context, categoryID uuid.UUID) (interface{}, error) {
	// Simple pass-through or structured query
	rows, err := s.dbPool.Pool.Query(ctx, `
		SELECT ad.code, ad.name_ru, ad.value_type, ad.scope, cad.required, cad.min_values, cad.max_values, cad.dictionary_id
		FROM category_attribute_definitions cad
		JOIN attribute_definitions ad ON cad.attribute_definition_id = ad.id
		WHERE cad.category_id = $1 AND ad.is_active = true
	`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []map[string]interface{}
	for rows.Next() {
		var code, nameRu, valType, scope string
		var required bool
		var minV, maxV *int
		var dictID *uuid.UUID
		if err := rows.Scan(&code, &nameRu, &valType, &scope, &required, &minV, &maxV, &dictID); err != nil {
			return nil, err
		}
		res = append(res, map[string]interface{}{
			"code": code, "nameRu": nameRu, "valueType": valType, "scope": scope,
			"required": required, "minValues": minV, "maxValues": maxV, "dictionaryId": dictID,
		})
	}
	return res, nil
}

func (s *Service) ListColors(ctx context.Context) ([]Color, error) {
	rows, err := s.dbPool.Pool.Query(ctx, "SELECT id, code, name_ru, hex, sort_order FROM colors WHERE is_active = true ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Color
	for rows.Next() {
		var c Color
		if err := rows.Scan(&c.ID, &c.Code, &c.NameRU, &c.Hex, &c.SortOrder); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, nil
}

func (s *Service) ListMaterials(ctx context.Context) ([]Material, error) {
	rows, err := s.dbPool.Pool.Query(ctx, "SELECT id, code, name_ru, sort_order FROM materials WHERE is_active = true ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Material
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.Code, &m.NameRU, &m.SortOrder); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	return res, nil
}

func (s *Service) ListSizeSystems(ctx context.Context) ([]SizeSystem, error) {
	rows, err := s.dbPool.Pool.Query(ctx, "SELECT id, code, name FROM size_systems WHERE is_active = true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SizeSystem
	for rows.Next() {
		var sys SizeSystem
		if err := rows.Scan(&sys.ID, &sys.Code, &sys.Name); err != nil {
			return nil, err
		}
		res = append(res, sys)
	}
	return res, nil
}

func (s *Service) ListSizeValues(ctx context.Context, systemID uuid.UUID) ([]SizeValue, error) {
	rows, err := s.dbPool.Pool.Query(ctx, "SELECT id, value, sort_order FROM size_values WHERE size_system_id = $1 AND is_active = true ORDER BY sort_order", systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SizeValue
	for rows.Next() {
		var sv SizeValue
		if err := rows.Scan(&sv.ID, &sv.Value, &sv.SortOrder); err != nil {
			return nil, err
		}
		res = append(res, sv)
	}
	return res, nil
}
