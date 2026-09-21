package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type StageProductImageResponse struct {
	StagedMediaID uuid.UUID `json:"stagedMediaId"`
	ClientMediaID uuid.UUID `json:"clientMediaId"`
	ImageURL      string    `json:"imageUrl"`
	Status        string    `json:"status"`
}

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
	if err != nil {
		return nil, fmt.Errorf("failed to decode image dimensions: %w", err)
	}

	w := cfg.Width
	h := cfg.Height

	if w >= h {
		return nil, ErrProductMediaPortraitRequired
	}

	const minWidth = 800
	const minHeight = 1000
	if w < minWidth || h < minHeight {
		return nil, ErrProductMediaTooSmall
	}

	width = &w
	height = &h

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

type validatedImageInfo struct {
	fileBytes     []byte
	size          int64
	width         int
	height        int
	contentSHA256 string
	canonicalExt  string
	mimeType      string
}

func validateAndExtractImage(reader io.Reader, filename string, size int64, contentType string, maxSizeMB int64) (*validatedImageInfo, error) {
	if ext := filepath.Ext(filename); ext != "" {
		if err := validateImage(contentType, ext, size, maxSizeMB); err != nil {
			return nil, err
		}
	} else if size > maxSizeMB*1024*1024 {
		return nil, ErrFileTooLarge
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}
	fileBytes := buf.Bytes()
	actualSize := int64(len(fileBytes))
	if actualSize > maxSizeMB*1024*1024 {
		return nil, ErrFileTooLarge
	}

	// Canonical image format must be derived strictly from uploaded bytes
	cfg, format, err := image.DecodeConfig(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image dimensions: %w", err)
	}

	var canonicalMIME, canonicalExt string
	switch format {
	case "jpeg":
		canonicalMIME = "image/jpeg"
		canonicalExt = ".jpg"
	case "png":
		canonicalMIME = "image/png"
		canonicalExt = ".png"
	case "webp":
		canonicalMIME = "image/webp"
		canonicalExt = ".webp"
	default:
		return nil, ErrInvalidMimeType
	}

	w := cfg.Width
	h := cfg.Height

	if w >= h {
		return nil, ErrProductMediaPortraitRequired
	}

	const minWidth = 800
	const minHeight = 1000
	if w < minWidth || h < minHeight {
		return nil, ErrProductMediaTooSmall
	}

	sha := sha256.Sum256(fileBytes)
	contentSHA256 := hex.EncodeToString(sha[:])

	return &validatedImageInfo{
		fileBytes:     fileBytes,
		size:          actualSize,
		width:         w,
		height:        h,
		contentSHA256: contentSHA256,
		canonicalExt:  canonicalExt,
		mimeType:      canonicalMIME,
	}, nil
}

func (s *Service) StageSellerProductImage(
	ctx context.Context,
	userID, productID, clientMediaID uuid.UUID,
	reader io.Reader,
	filename string,
	size int64,
	contentType string,
	maxSizeMB int64,
) (*StageProductImageResponse, error) {
	// 1. Authenticate & resolve seller
	seller, _, err := s.sellersRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller profile: %w", err)
	}

	// 2. Validate product ownership and existence using owner-scoped lookup
	// Both nonexistent and foreign products return ErrProductNotFound, preventing information leak.
	prod, err := s.productsRepo.GetProductByIDForSeller(ctx, productID, seller.ID)
	if err != nil {
		return nil, err
	}

	// 3. Validate product editability
	if !products.CanEditProduct(seller.Status, prod.Status) {
		return nil, products.ErrProductNotEditable
	}

	// 4. Validate image bytes and extract metadata strictly from actual bytes
	info, err := validateAndExtractImage(reader, filename, size, contentType, maxSizeMB)
	if err != nil {
		return nil, err
	}

	// 5. Candidate stagedMediaID, immutable object key + public URL
	candidateStagedMediaID := uuid.New()
	objectKey := fmt.Sprintf("products/%s/%s/staged/%s%s", seller.ID.String(), productID.String(), candidateStagedMediaID.String(), info.canonicalExt)
	imageURL := s.provider.BuildPublicURL(objectKey)

	// 6. Atomically claim staged media slot under short Product row lock (COMMIT happens before S3 upload)
	stagingItem := &products.ProductMediaStaging{
		ID:            candidateStagedMediaID,
		SellerID:      seller.ID,
		ProductID:     productID,
		ClientMediaID: clientMediaID,
		Status:        products.ProductMediaStagingUploading,
		ObjectKey:     objectKey,
		ImageURL:      imageURL,
		ContentSHA256: info.contentSHA256,
		ByteSize:      info.size,
		Width:         info.width,
		Height:        info.height,
	}

	var staged *products.ProductMediaStaging
	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.productsRepo.WithTx(tx)
		var claimErr error
		staged, claimErr = txRepo.ClaimStagedMediaSlotForSellerProduct(ctx, stagingItem, seller.Status)
		return claimErr
	})
	if err != nil {
		return nil, err
	}

	// 7. Resolve lifecycle state (S3 upload occurs OUTSIDE DB transaction)
	switch staged.Status {
	case products.ProductMediaStagingReady:
		// Case B: Same content, already ready -> do not upload again
		return &StageProductImageResponse{
			StagedMediaID: staged.ID,
			ClientMediaID: staged.ClientMediaID,
			ImageURL:      staged.ImageURL,
			Status:        string(staged.Status),
		}, nil

	case products.ProductMediaStagingConsumed:
		// Case E: Already consumed by canonical media -> do not upload again
		return &StageProductImageResponse{
			StagedMediaID: staged.ID,
			ClientMediaID: staged.ClientMediaID,
			ImageURL:      staged.ImageURL,
			Status:        string(staged.Status),
		}, nil

	case products.ProductMediaStagingUploading:
		// Case A (new) or Case C (retry)
		// 8. Upload to object storage using canonical byte-derived MIME type (outside DB transaction)
		_, err = s.provider.UploadImage(ctx, bytes.NewReader(info.fileBytes), staged.ByteSize, staged.ObjectKey, info.mimeType)
		if err != nil {
			return nil, fmt.Errorf("failed to upload staged image: %w", err)
		}

		// 9. Mark ready using ownership-scoped mutation
		if err = s.productsRepo.MarkStagedMediaReadyForSellerProduct(ctx, staged.ID, seller.ID, productID); err != nil {
			if errors.Is(err, products.ErrStagedMediaNotFound) {
				// Definite missing row: durably enqueue cleanup so newly uploaded S3 object is never untracked
				if enqueueErr := s.productsRepo.EnqueueMediaCleanup(ctx, staged.ObjectKey); enqueueErr != nil {
					return nil, fmt.Errorf("staged media row missing and failed to enqueue cleanup (%v): %w", enqueueErr, err)
				}
				return nil, fmt.Errorf("staged media row missing after upload (cleanup enqueued): %w", err)
			}
			return nil, fmt.Errorf("failed to mark staged media ready: %w", err)
		}

		// 10. Return success
		return &StageProductImageResponse{
			StagedMediaID: staged.ID,
			ClientMediaID: staged.ClientMediaID,
			ImageURL:      staged.ImageURL,
			Status:        string(products.ProductMediaStagingReady),
		}, nil

	default:
		return nil, fmt.Errorf("unknown staged media status: %s", staged.Status)
	}
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

func (s *Service) SetMainProductImage(ctx context.Context, userID, productID, imageID uuid.UUID) error {
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

	imgRow, err := s.productsRepo.GetProductImageByID(ctx, imageID)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}
	if imgRow.ProductID != productID {
		return fmt.Errorf("image does not belong to product")
	}

	if imgRow.CropWidth == nil || imgRow.CropHeight == nil {
		return fmt.Errorf("image must be cropped to 4:5 before it can be made main")
	}

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		repoTx := s.productsRepo.WithTx(tx)
		if err := repoTx.ClearOtherMainImages(ctx, productID, imageID); err != nil {
			return err
		}

		query := `UPDATE product_images SET is_main = true WHERE id = $1`
		if _, err := tx.Exec(ctx, query, imageID); err != nil {
			return err
		}

		imgRow, err := repoTx.GetProductImageByID(ctx, imageID)
		if err != nil {
			return err
		}
		if imgRow.RenditionURL == nil || imgRow.RenditionObjectKey == nil {
			return fmt.Errorf("selected image is not ready (missing rendition)")
		}
		

		if err := repoTx.SetMainImage(ctx, productID, *imgRow.RenditionURL, *imgRow.RenditionObjectKey); err != nil {
			return err
		}

		return nil
	})

	return err
}
