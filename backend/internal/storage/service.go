package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	provider     Provider
	productsRepo *products.Repository
	catalogRepo  *catalog.Repository
	sellersRepo  *sellers.Repository
	dbPool       *postgres.Client
}

func NewService(provider Provider, productsRepo *products.Repository, catalogRepo *catalog.Repository, sellersRepo *sellers.Repository, dbPool *postgres.Client) *Service {
	return &Service{
		provider:     provider,
		productsRepo: productsRepo,
		catalogRepo:  catalogRepo,
		sellersRepo:  sellersRepo,
		dbPool:       dbPool,
	}
}

func validateImage(contentType, extension string, size, maxSizeMB int64) error {
	if size > maxSizeMB*1024*1024 {
		return ErrFileTooLarge
	}

	validMimes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	if !validMimes[contentType] {
		return ErrInvalidMimeType
	}

	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}
	ext := strings.ToLower(extension)
	if !validExts[ext] {
		return ErrInvalidExtension
	}

	return nil
}

func (s *Service) UploadSellerProductImage(ctx context.Context, userID, productID uuid.UUID, reader io.Reader, filename string, size int64, contentType string, maxSizeMB int64, opts UploadOptions) (*UploadImageResponse, error) {
	ext := filepath.Ext(filename)
	if err := validateImage(contentType, ext, size, maxSizeMB); err != nil {
		return nil, err
	}

	seller, _, err := s.sellersRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller profile: %w", err)
	}

	prod, err := s.productsRepo.GetProductByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	if prod.SellerID != seller.ID {
		return nil, ErrProductNotOwned
	}

	if len(prod.Images) >= 8 {
		return nil, fmt.Errorf("maximum 8 images allowed per product")
	}

	if !products.CanEditProduct(seller.Status, prod.Status) {
		return nil, products.ErrProductNotEditable
	}

	objectKey := fmt.Sprintf("products/%s/%s/%s%s", seller.ID.String(), productID.String(), uuid.New().String(), ext)

	// Read into memory for dimension extraction and upload
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}
	fileBytes := buf.Bytes()
	size = int64(len(fileBytes))

	// Decode dimensions
	var width, height *int
	cfg, _, err := image.DecodeConfig(bytes.NewReader(fileBytes))
	if err == nil {
		w := cfg.Width
		h := cfg.Height
		width = &w
		height = &h
	}

	stored, err := s.provider.UploadImage(ctx, bytes.NewReader(fileBytes), size, objectKey, contentType)

	if err != nil {
		return nil, err
	}

	// Best-effort cleanup on DB failure
	defer func() {
		if err != nil {
			go func() {
				_ = s.provider.DeleteObject(context.Background(), objectKey)
			}()
		}
	}()

	img := &products.ProductImage{
		ID:        uuid.New(),
		ProductID: productID,
		ImageURL:  stored.ObjectURL,
		ObjectKey: &stored.ObjectKey,
		AltText:   nil,
		SortOrder: opts.SortOrder,
		Width:     width,
		Height:    height,
		IsMain:    false,
		CreatedAt: time.Now().UTC(),
	}
	if opts.AltText != "" {
		img.AltText = &opts.AltText
	}

	if err = s.productsRepo.AddProductImage(ctx, img); err != nil {
		return nil, err
	}

	if opts.IsMain || prod.MainImageURL == nil {
		if err = s.productsRepo.SetMainImage(ctx, productID, stored.ObjectURL, stored.ObjectKey); err != nil {
			return nil, err
		}
	}

	return &UploadImageResponse{
		ID:        &img.ID,
		ImageURL:  stored.ObjectURL,
		ObjectKey: stored.ObjectKey,
		AltText:   opts.AltText,
		SortOrder: opts.SortOrder,
		IsMain:    opts.IsMain || prod.MainImageURL == nil,
	}, nil
}

func (s *Service) UploadAdminProductImage(ctx context.Context, productID uuid.UUID, reader io.Reader, filename string, size int64, contentType string, maxSizeMB int64, opts UploadOptions) (*UploadImageResponse, error) {
	ext := filepath.Ext(filename)
	if err := validateImage(contentType, ext, size, maxSizeMB); err != nil {
		return nil, err
	}

	prod, err := s.productsRepo.GetProductByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	objectKey := fmt.Sprintf("products/%s/%s/%s%s", prod.SellerID.String(), productID.String(), uuid.New().String(), ext)

	// Read into memory for dimension extraction and upload
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}
	fileBytes := buf.Bytes()
	size = int64(len(fileBytes))

	// Decode dimensions
	var width, height *int
	cfg, _, err := image.DecodeConfig(bytes.NewReader(fileBytes))
	if err == nil {
		w := cfg.Width
		h := cfg.Height
		width = &w
		height = &h
	}

	stored, err := s.provider.UploadImage(ctx, bytes.NewReader(fileBytes), size, objectKey, contentType)

	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			go func() {
				_ = s.provider.DeleteObject(context.Background(), objectKey)
			}()
		}
	}()

	img := &products.ProductImage{
		ID:        uuid.New(),
		ProductID: productID,
		ImageURL:  stored.ObjectURL,
		ObjectKey: &stored.ObjectKey,
		AltText:   nil,
		SortOrder: opts.SortOrder,
		Width:     width,
		Height:    height,
		IsMain:    false,
		CreatedAt: time.Now().UTC(),
	}
	if opts.AltText != "" {
		img.AltText = &opts.AltText
	}

	if err = s.productsRepo.AddProductImage(ctx, img); err != nil {
		return nil, err
	}

	if opts.IsMain || prod.MainImageURL == nil {
		if err = s.productsRepo.SetMainImage(ctx, productID, stored.ObjectURL, stored.ObjectKey); err != nil {
			return nil, err
		}
	}

	return &UploadImageResponse{
		ID:        &img.ID,
		ImageURL:  stored.ObjectURL,
		ObjectKey: stored.ObjectKey,
		AltText:   opts.AltText,
		SortOrder: opts.SortOrder,
		IsMain:    opts.IsMain || prod.MainImageURL == nil,
	}, nil
}

func (s *Service) UploadAdminBrandLogo(ctx context.Context, brandID uuid.UUID, reader io.Reader, filename string, size int64, contentType string, maxSizeMB int64) (*BrandLogoResponse, error) {
	ext := filepath.Ext(filename)
	if err := validateImage(contentType, ext, size, maxSizeMB); err != nil {
		return nil, err
	}

	_, err := s.catalogRepo.GetBrandByID(ctx, brandID)
	if err != nil {
		return nil, fmt.Errorf("failed to get brand: %w", err)
	}

	objectKey := fmt.Sprintf("brands/%s/%s%s", brandID.String(), uuid.New().String(), ext)

	// Read into memory for dimension extraction and upload
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}
	fileBytes := buf.Bytes()
	size = int64(len(fileBytes))

	// Decode dimensions
	stored, err := s.provider.UploadImage(ctx, bytes.NewReader(fileBytes), size, objectKey, contentType)

	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			go func() {
				_ = s.provider.DeleteObject(context.Background(), objectKey)
			}()
		}
	}()

	if err = s.catalogRepo.UpdateBrandLogo(ctx, brandID, stored.ObjectURL, stored.ObjectKey); err != nil {
		return nil, err
	}

	return &BrandLogoResponse{
		LogoURL: stored.ObjectURL,
	}, nil
}

func (s *Service) UploadSellerProfileImage(ctx context.Context, userID uuid.UUID, reader io.Reader, filename string, size int64, contentType string, maxSizeMB int64) (*SellerLogoResponse, error) {
	ext := filepath.Ext(filename)
	if err := validateImage(contentType, ext, size, maxSizeMB); err != nil {
		return nil, err
	}

	seller, _, err := s.sellersRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller: %w", err)
	}

	objectKey := fmt.Sprintf("sellers/%s/%s%s", seller.ID.String(), uuid.New().String(), ext)

	// Read into memory for dimension extraction and upload
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}
	fileBytes := buf.Bytes()
	size = int64(len(fileBytes))

	// Decode dimensions
	stored, err := s.provider.UploadImage(ctx, bytes.NewReader(fileBytes), size, objectKey, contentType)

	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			go func() {
				_ = s.provider.DeleteObject(context.Background(), objectKey)
			}()
		}
	}()

	if err = s.sellersRepo.UpdateSellerLogo(ctx, seller.ID, stored.ObjectURL, stored.ObjectKey); err != nil {
		return nil, err
	}

	return &SellerLogoResponse{
		LogoURL: stored.ObjectURL,
	}, nil
}

func (s *Service) DeleteSellerProductImage(ctx context.Context, userID, productID, imageID uuid.UUID) error {
	seller, _, err := s.sellersRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get seller profile: %w", err)
	}

	prod, err := s.productsRepo.GetProductByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	if prod.SellerID != seller.ID {
		return ErrProductNotOwned
	}

	if !products.CanEditProduct(seller.Status, prod.Status) {
		return products.ErrProductNotEditable
	}

	img, err := s.productsRepo.GetProductImageByID(ctx, imageID)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}
	if img.ProductID != productID {
		return fmt.Errorf("image does not belong to product")
	}

	// Delete from DB first
	if err := s.productsRepo.DeleteProductImage(ctx, imageID); err != nil {
		return err
	}

	// Then from storage
	if img.ObjectKey != nil {
		_ = s.provider.DeleteObject(context.Background(), *img.ObjectKey)
	}

	// If it was the main image, reset it
	if prod.MainImageURL != nil && *prod.MainImageURL == img.ImageURL {
		// Just clear it for simplicity, UI/User can set another one or we can pick the first remaining
		remaining, _ := s.productsRepo.GetProductImages(ctx, productID)
		if len(remaining) > 0 {
			objKey := ""
			if remaining[0].ObjectKey != nil {
				objKey = *remaining[0].ObjectKey
			}
			_ = s.productsRepo.SetMainImage(ctx, productID, remaining[0].ImageURL, objKey)
		} else {
			_ = s.productsRepo.SetMainImage(ctx, productID, "", "")
		}
	}
	return nil
}

func (s *Service) ReorderSellerProductImages(ctx context.Context, userID, productID uuid.UUID, imageIDs []uuid.UUID) error {
	seller, _, err := s.sellersRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get seller profile: %w", err)
	}

	prod, err := s.productsRepo.GetProductByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	if prod.SellerID != seller.ID {
		return ErrProductNotOwned
	}

	if !products.CanEditProduct(seller.Status, prod.Status) {
		return products.ErrProductNotEditable
	}

	existingImages, err := s.productsRepo.GetProductImages(ctx, productID)
	if err != nil {
		return fmt.Errorf("failed to get product images: %w", err)
	}

	existingMap := make(map[uuid.UUID]bool)
	for _, img := range existingImages {
		existingMap[img.ID] = true
	}

	seen := make(map[uuid.UUID]bool)
	for _, id := range imageIDs {
		if !existingMap[id] {
			return fmt.Errorf("image %s does not belong to product", id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate image ID %s", id)
		}
		seen[id] = true
	}

	if len(imageIDs) != len(existingImages) {
		return fmt.Errorf("missing images in reorder request")
	}

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		repoTx := s.productsRepo.WithTx(tx)
		if err := repoTx.ReorderProductImages(ctx, productID, imageIDs); err != nil {
			return err
		}

		if len(imageIDs) > 0 {
			firstImg, err := repoTx.GetProductImageByID(ctx, imageIDs[0])
			if err == nil {
				objKey := ""
				if firstImg.ObjectKey != nil {
					objKey = *firstImg.ObjectKey
				}
				_ = repoTx.SetMainImage(ctx, productID, firstImg.ImageURL, objKey)
			}
		} else {
			_ = repoTx.SetMainImage(ctx, productID, "", "")
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to reorder images: %w", err)
	}

	return nil
}
