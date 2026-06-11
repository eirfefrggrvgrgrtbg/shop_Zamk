package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

type Service struct {
	provider     Provider
	productsRepo *products.Repository
	catalogRepo  *catalog.Repository
	sellersRepo  *sellers.Repository
}

func NewService(provider Provider, productsRepo *products.Repository, catalogRepo *catalog.Repository, sellersRepo *sellers.Repository) *Service {
	return &Service{
		provider:     provider,
		productsRepo: productsRepo,
		catalogRepo:  catalogRepo,
		sellersRepo:  sellersRepo,
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

	if !products.CanEditProduct(seller.Status, prod.Status) {
		return nil, products.ErrProductNotEditable
	}

	objectKey := fmt.Sprintf("products/%s/%s/%s%s", seller.ID.String(), productID.String(), uuid.New().String(), ext)

	stored, err := s.provider.UploadImage(ctx, reader, size, objectKey, contentType)
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

	stored, err := s.provider.UploadImage(ctx, reader, size, objectKey, contentType)
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

	stored, err := s.provider.UploadImage(ctx, reader, size, objectKey, contentType)
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

	stored, err := s.provider.UploadImage(ctx, reader, size, objectKey, contentType)
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
