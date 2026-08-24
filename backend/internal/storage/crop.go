package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

type CropImageRequest struct {
	CropX      float64 `json:"cropX"`
	CropY      float64 `json:"cropY"`
	CropWidth  float64 `json:"cropWidth"`
	CropHeight float64 `json:"cropHeight"`
}

var ErrInvalidCropParameters = fmt.Errorf("invalid crop parameters")

func (req CropImageRequest) Validate() error {
	epsilon := 0.01
	if req.CropX < 0 || req.CropY < 0 || req.CropWidth <= 0 || req.CropHeight <= 0 {
		return ErrInvalidCropParameters
	}
	if req.CropX+req.CropWidth > 1+epsilon || req.CropY+req.CropHeight > 1+epsilon {
		return ErrInvalidCropParameters
	}

	aspect := req.CropWidth / req.CropHeight
	expectedAspect := 4.0 / 5.0
	if aspect < expectedAspect-epsilon || aspect > expectedAspect+epsilon {
		return fmt.Errorf("crop aspect ratio must be 4:5")
	}
	return nil
}

func (s *Service) CropSellerProductImage(ctx context.Context, userID, productID, imageID uuid.UUID, req CropImageRequest) (*UploadImageResponse, error) {
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

	imgRow, err := s.productsRepo.GetProductImageByID(ctx, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}
	if imgRow.ProductID != productID {
		return nil, fmt.Errorf("image does not belong to product")
	}

	if imgRow.ObjectKey == nil {
		return nil, fmt.Errorf("image has no object key to download")
	}

	// 1. Download original from Minio
	originalData, err := s.provider.DownloadObject(ctx, *imgRow.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to download original image: %w", err)
	}

	origImg, _, err := image.Decode(bytes.NewReader(originalData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := origImg.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// 2. Calculate crop rect
	x0 := int(req.CropX * float64(origW))
	y0 := int(req.CropY * float64(origH))
	x1 := x0 + int(req.CropWidth*float64(origW))
	y1 := y0 + int(req.CropHeight*float64(origH))

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > origW {
		x1 = origW
	}
	if y1 > origH {
		y1 = origH
	}

	cropRect := image.Rect(x0, y0, x1, y1)
	if cropRect.Empty() {
		return nil, fmt.Errorf("invalid crop rectangle")
	}

	// Create sub-image interface or new RGBA
	subImg, ok := origImg.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	var cropped image.Image
	if ok {
		cropped = subImg.SubImage(cropRect)
	} else {
		// Fallback copy
		rgba := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
		draw.Draw(rgba, rgba.Bounds(), origImg, cropRect.Min, draw.Src)
		cropped = rgba
	}

	// 3. Scale to canonical 1200x1500 (4:5)
	targetW, targetH := 1200, 1500
	scaled := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), cropped, cropped.Bounds(), draw.Src, nil)

	// 4. Encode to JPEG
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, scaled, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, fmt.Errorf("failed to encode jpeg: %w", err)
	}

	// 5. Upload rendition
	ext := ".jpg"
	renditionKey := fmt.Sprintf("products/%s/%s/%s_rendition%s", seller.ID.String(), productID.String(), imageID.String(), ext)

	_, err = s.provider.UploadImage(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()), renditionKey, "image/jpeg")
	if err != nil {
		return nil, fmt.Errorf("failed to upload rendition: %w", err)
	}

	// 6. Update DB
	renditionURL := s.provider.BuildPublicURL(renditionKey)
	err = s.productsRepo.UpdateProductImageCrop(ctx, imageID, req.CropX, req.CropY, req.CropWidth, req.CropHeight, renditionURL, renditionKey)
	if err != nil {
		return nil, err
	}

	return &UploadImageResponse{
		ID:        &imageID,
		ImageURL:  imgRow.ImageURL, // original
		ObjectKey: *imgRow.ObjectKey,
		RenditionURL: renditionURL,
		IsMain:    imgRow.IsMain,
	}, nil
}
