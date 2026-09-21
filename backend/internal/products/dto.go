package products

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)
type CreateProductRequest struct {
	Title            string                  `json:"title" validate:"required"`
	Slug             *string                 `json:"slug,omitempty"`
	Description      *string                 `json:"description,omitempty"`
	CategoryID       *uuid.UUID              `json:"categoryId,omitempty"`
	BrandID          *uuid.UUID              `json:"brandId,omitempty"`
	Gender           *string                 `json:"gender,omitempty"`
	Color            *string                 `json:"color,omitempty"`
	Material         *string                 `json:"material,omitempty"`
	CareInstructions *string                 `json:"careInstructions,omitempty"`
	PriceCents       int64                   `json:"priceCents" validate:"min=0"`
	OldPriceCents    *int64                  `json:"oldPriceCents,omitempty" validate:"omitempty,min=0"`
	Currency         string                  `json:"currency" validate:"required,eq=RUB"`
	MainImageURL     *string                 `json:"mainImageUrl,omitempty"`
	Variants         []ProductVariantRequest `json:"variants,omitempty"`
	Images           []ProductImageRequest   `json:"images,omitempty"`
	ContinueSelling  *bool                   `json:"continueSelling,omitempty"`

	MaterialComposition []ProductMaterialCompositionRequest `json:"materialComposition,omitempty"`
	SizeChartRows       []ProductSizeChartRowRequest        `json:"sizeChartRows,omitempty"`
	Attributes          []ProductAttributeValueRequest      `json:"attributes,omitempty"`
}

func (req *CreateProductRequest) ValidateSKUs() error {
	seen := make(map[string]bool)
	for i := range req.Variants {
		v := &req.Variants[i]
		if v.SellerSKU != nil {
			// ZAMK Rule: SKU unique per Seller.
			// Normalization: trim surrounding whitespace and case-insensitive comparison.
			trimmed := strings.ToLower(strings.TrimSpace(*v.SellerSKU))
			if trimmed != "" {
				if seen[trimmed] {
					return &DuplicateSKUError{SKU: strings.TrimSpace(*v.SellerSKU)}
				}
				seen[trimmed] = true
			}
		}
	}
	return nil
}

type UpdateProductRequest struct {
	Title            *string                 `json:"title,omitempty"`
	Slug             *string                 `json:"slug,omitempty"`
	Description      *string                 `json:"description,omitempty"`
	CategoryID       *uuid.UUID              `json:"categoryId,omitempty"`
	BrandID          *uuid.UUID              `json:"brandId,omitempty"`
	Gender           *string                 `json:"gender,omitempty"`
	Color            *string                 `json:"color,omitempty"`
	Material         *string                 `json:"material,omitempty"`
	CareInstructions *string                 `json:"careInstructions,omitempty"`
	PriceCents       *int64                  `json:"priceCents,omitempty" validate:"omitempty,min=0"`
	OldPriceCents    *int64                  `json:"oldPriceCents,omitempty" validate:"omitempty,min=0"`
	MainImageURL     *string                 `json:"mainImageUrl,omitempty"`
	Variants         []ProductVariantRequest `json:"variants,omitempty"`
	Images           []ProductImageRequest   `json:"images,omitempty"`
	ContinueSelling  *bool                   `json:"continueSelling,omitempty"`

	MaterialComposition []ProductMaterialCompositionRequest `json:"materialComposition,omitempty"`
	SizeChartRows       []ProductSizeChartRowRequest        `json:"sizeChartRows,omitempty"`
	Attributes          []ProductAttributeValueRequest      `json:"attributes,omitempty"`
}

func (req *UpdateProductRequest) ValidateSKUs() error {
	seen := make(map[string]bool)
	for i := range req.Variants {
		v := &req.Variants[i]
		if v.SellerSKU != nil {
			// ZAMK Rule: SKU unique per Seller.
			// Normalization: trim surrounding whitespace and case-insensitive comparison.
			trimmed := strings.ToLower(strings.TrimSpace(*v.SellerSKU))
			if trimmed != "" {
				if seen[trimmed] {
					return &DuplicateSKUError{SKU: strings.TrimSpace(*v.SellerSKU)}
				}
				seen[trimmed] = true
			}
		}
	}
	return nil
}

type ProductVariantRequest struct {
	ID           *uuid.UUID `json:"id,omitempty"`
	SKU          *string `json:"sku,omitempty"`
	Size         *string `json:"size,omitempty"`
	Color        *string `json:"color,omitempty"`
	OptionValues map[string]interface{} `json:"optionValues,omitempty"`
	SellerSKU          *string `json:"sellerSku,omitempty"`
	ColorID            *uuid.UUID `json:"colorId,omitempty"`
	SizeValueID        *uuid.UUID `json:"sizeValueId,omitempty"`
	ShadeName          *string `json:"shadeName,omitempty"`
	Barcode      *string `json:"barcode,omitempty"`
	Attributes   []VariantAttributeValueRequest `json:"attributes,omitempty"`
	PriceCents   *int64  `json:"priceCents,omitempty" validate:"omitempty,min=0"`
}

type ProductImageRequest struct {
	ID        *uuid.UUID `json:"id,omitempty"`
	ImageURL  string     `json:"imageUrl" validate:"required"`
	AltText   *string    `json:"altText,omitempty"`
	SortOrder *int       `json:"sortOrder,omitempty"`
	ColorID   *uuid.UUID `json:"colorId,omitempty"`
	IsMain    *bool      `json:"isMain,omitempty"`
}

type SubmitProductModerationRequest struct {
	Comment *string `json:"comment,omitempty"`
}

type AdminProductModerationRequest struct {
	Comment           *string `json:"comment,omitempty"`
	ExpectedUpdatedAt *string `json:"expectedUpdatedAt,omitempty"`
}

type RejectProductRequest struct {
	Comment string `json:"comment" validate:"required"`
}

type ProductListResponse struct {
	Items      []Product `json:"items"`
	TotalCount int       `json:"totalCount"`
}

type PublicProduct struct {
	ID               uuid.UUID              `json:"id"`
	SellerID         uuid.UUID              `json:"sellerId"`
	SellerSlug       string                 `json:"sellerSlug"`
	SellerName       string                 `json:"sellerName"`
	CategoryID       *uuid.UUID             `json:"categoryId,omitempty"`
	BrandID          *uuid.UUID             `json:"brandId,omitempty"`
	Title            string                 `json:"title"`
	Slug             string                 `json:"slug"`
	Description      *string                `json:"description,omitempty"`
	Status           string                 `json:"status"`
	Gender           *string                `json:"gender,omitempty"`
	Color            *string                `json:"color,omitempty"`
	Material         *string                `json:"material,omitempty"`
	CareInstructions *string                `json:"careInstructions,omitempty"`
	PriceCents       int64                  `json:"priceCents"`
	OldPriceCents    *int64                 `json:"oldPriceCents,omitempty"`
	Currency         string                 `json:"currency"`
	MainImageURL     *string                `json:"mainImageUrl,omitempty"`
	AverageRating    float64                `json:"averageRating"`
	ReviewsCount     int                    `json:"reviewsCount"`
	InStock          *bool                  `json:"inStock,omitempty"`
	CreatedAt        time.Time              `json:"createdAt"`

	MaterialComposition []ProductMaterialComposition `json:"materialComposition,omitempty"`
	SizeChart   *ProductSizeChart `json:"sizeChart,omitempty"`

	Variants []PublicProductVariant `json:"variants,omitempty"`
	Images   []PublicProductImage   `json:"images,omitempty"`
	ContinueSelling  *bool                   `json:"continueSelling,omitempty"`
	Rating   *RatingSummary         `json:"rating,omitempty"`
}

type PublicProductVariant struct {
	ID           uuid.UUID              `json:"id"`
	ProductID    uuid.UUID              `json:"productId"`
	SKU          *string                `json:"sku,omitempty"`
	Size         *string                `json:"size,omitempty"`
	Color        *string                `json:"color,omitempty"`
	OptionValues map[string]interface{} `json:"optionValues,omitempty"`
	SellerSKU    *string                `json:"sellerSku,omitempty"`
	ColorID      *uuid.UUID             `json:"colorId,omitempty"`
	SizeValueID  *uuid.UUID             `json:"sizeValueId,omitempty"`
	ColorName    *string                `json:"colorName,omitempty"`
	ColorHex     *string                `json:"colorHex,omitempty"`
	ShadeName    *string                `json:"shadeName,omitempty"`
	Barcode      *string                `json:"barcode,omitempty"`
	PriceCents   *int64                 `json:"priceCents,omitempty"`
	IsActive     bool                   `json:"isActive"`
	InStock      *bool                  `json:"inStock,omitempty"`
}

type PublicProductImage struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"productId"`
	ImageURL  string    `json:"imageUrl"`
	AltText   *string   `json:"altText,omitempty"`
	SortOrder int       `json:"sortOrder"`
	ColorID   *uuid.UUID `json:"colorId,omitempty"`

	Width     *int       `json:"width,omitempty"`
	Height    *int       `json:"height,omitempty"`
	CropX     *float64   `json:"cropX,omitempty"`
	CropY     *float64   `json:"cropY,omitempty"`
	CropWidth *float64   `json:"cropWidth,omitempty"`
	CropHeight *float64  `json:"cropHeight,omitempty"`
	IsMain    bool       `json:"isMain"`
}

type PublicProductListResponse struct {
	Items      []PublicProduct `json:"items"`
	TotalCount int             `json:"totalCount"`
}

type ModerationHistoryItem struct {
	ID          uuid.UUID  `json:"id"`
	ProductID   uuid.UUID  `json:"productId"`
	AdminUserID *uuid.UUID `json:"adminUserId,omitempty"`
	AdminName   *string    `json:"adminName,omitempty"`
	FromStatus  *string    `json:"fromStatus,omitempty"`
	ToStatus    string     `json:"toStatus"`
	Comment     *string    `json:"comment,omitempty"`
	CreatedAt   string     `json:"createdAt"` // Formatting time to ISO8601
}

type ModerationHistoryResponse struct {
	Items []ModerationHistoryItem `json:"items"`
}

type CatalogAffinities struct {
	FavoriteCategoryIDs []uuid.UUID
	FavoriteBrandIDs    []uuid.UUID
	ViewedCategoryIDs   []uuid.UUID
	ViewedBrandIDs      []uuid.UUID
}

func (a *CatalogAffinities) HasAny() bool {
	if a == nil {
		return false
	}
	return len(a.FavoriteCategoryIDs) > 0 || len(a.FavoriteBrandIDs) > 0 ||
		len(a.ViewedCategoryIDs) > 0 || len(a.ViewedBrandIDs) > 0
}

type PublicProductFilter struct {
	Query         *string            `json:"q,omitempty"`
	CategoryID    *uuid.UUID         `json:"categoryId,omitempty"`
	BrandID       *uuid.UUID         `json:"brandId,omitempty"`
	SellerID      *uuid.UUID         `json:"sellerId,omitempty"`
	Size          *string            `json:"size,omitempty"`
	MinPriceCents *int64             `json:"minPriceCents,omitempty"`
	MaxPriceCents *int64             `json:"maxPriceCents,omitempty"`
	InStock       *bool              `json:"inStock,omitempty"`
	Sort          *string            `json:"sort,omitempty"`
	Affinities    *CatalogAffinities `json:"-"`
}

type ModerationConfig struct {
	WarningHours  int `json:"warningHours"`  // SLA Warning threshold in hours (default: 24)
	CriticalHours int `json:"criticalHours"` // SLA Critical threshold in hours (default: 48)
}

type AdminModerationListResponse struct {
	Items      []Product        `json:"items"`
	TotalCount int              `json:"totalCount"`
	Config     ModerationConfig `json:"config"`
}

type AdminProductFilter struct {
	Query           *string      `json:"q,omitempty"`
	Status          *string      `json:"status,omitempty"`
	SellerID        *uuid.UUID   `json:"sellerId,omitempty"`
	CategoryID      *uuid.UUID   `json:"categoryId,omitempty"`
	CategoryIDs     []uuid.UUID  `json:"categoryIds,omitempty"`
	BrandID         *uuid.UUID   `json:"brandId,omitempty"`
	BrandIDs        []uuid.UUID  `json:"brandIds,omitempty"`
	Source          *string      `json:"source,omitempty"`
	HasProblems     *bool        `json:"hasProblems,omitempty"`
	SubmittedPeriod *string      `json:"submittedPeriod,omitempty"`
	SubmittedFrom   *time.Time   `json:"submittedFrom,omitempty"`
	SubmittedTo     *time.Time   `json:"submittedTo,omitempty"`
	NoMainImage     *bool        `json:"noMainImage,omitempty"`
	NoDescription   *bool        `json:"noDescription,omitempty"`
	NoBrand         *bool        `json:"noBrand,omitempty"`
	NoVariants      *bool        `json:"noVariants,omitempty"`
	NoPrice         *bool        `json:"noPrice,omitempty"`
	DuplicateSKU    *bool        `json:"duplicateSku,omitempty"`
	NoStock         *bool        `json:"noStock,omitempty"`
	Resubmitted     *bool        `json:"resubmitted,omitempty"`
	Sort            *string      `json:"sort,omitempty"`
	SortOrder       *string      `json:"sortOrder,omitempty"`
}

type StartReviewRequest struct {
	ExpectedUpdatedAt *time.Time `json:"expectedUpdatedAt,omitempty"`
}

type ProductPreviewLinkResponse struct {
	PageURL        string `json:"pageUrl"`
	CatalogCardURL string `json:"catalogCardUrl"`
	ExpiresAt      string `json:"expiresAt"`
}

type ProductPublishErrorResponse struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Reasons []string `json:"reasons"`
}


type ProductMaterialCompositionRequest struct {
	MaterialID uuid.UUID `json:"materialId"`
	Percentage float64   `json:"percentage"`
}

type ProductSizeChartRowRequest struct {
	SizeValueID  uuid.UUID              `json:"sizeValueId"`
	Measurements map[string]interface{} `json:"measurements"`
}

type UpdateVariantPricesRequest struct {
	Prices map[uuid.UUID]int64 `json:"prices" validate:"required,dive,min=0"`
}

type ProductAttributeValueRequest struct {
	AttributeDefinitionID uuid.UUID  `json:"attributeDefinitionId"`
	EnumValueID           *uuid.UUID `json:"enumValueId,omitempty"`
	TextValue             *string    `json:"textValue,omitempty"`
	NumberValue           *float64   `json:"numberValue,omitempty"`
	BoolValue             *bool      `json:"boolValue,omitempty"`
}

type VariantAttributeValueRequest struct {
	AttributeDefinitionID uuid.UUID  `json:"attributeDefinitionId"`
	EnumValueID           *uuid.UUID `json:"enumValueId,omitempty"`
	TextValue             *string    `json:"textValue,omitempty"`
	NumberValue           *float64   `json:"numberValue,omitempty"`
	BoolValue             *bool      `json:"boolValue,omitempty"`
}

type UpdateProductPricesRequest struct {
	Variants []VariantPriceUpdateRequest `json:"variants" validate:"required,min=1"`
}

type VariantPriceUpdateRequest struct {
	ID            uuid.UUID `json:"id" validate:"required"`
	PriceCents    int64     `json:"priceCents" validate:"required,gt=0"`
	OldPriceCents *int64    `json:"oldPriceCents,omitempty" validate:"omitempty,min=0"`
}

type NormalizedProductCreateRequest struct {
	Title               string                              `json:"title"`
	Slug                *string                             `json:"slug,omitempty"`
	Description         *string                             `json:"description,omitempty"`
	CategoryID          *string                             `json:"categoryId,omitempty"`
	BrandID             *string                             `json:"brandId,omitempty"`
	Gender              *string                             `json:"gender,omitempty"`
	Color               *string                             `json:"color,omitempty"`
	Material            *string                             `json:"material,omitempty"`
	CareInstructions    *string                             `json:"careInstructions,omitempty"`
	PriceCents          int64                               `json:"priceCents"`
	OldPriceCents       *int64                              `json:"oldPriceCents,omitempty"`
	Currency            string                              `json:"currency"`
	MainImageURL        *string                             `json:"mainImageUrl,omitempty"`
	Variants            []NormalizedVariantCreateRequest    `json:"variants,omitempty"`
	Images              []NormalizedImageCreateRequest      `json:"images,omitempty"`
	Attributes          []ProductAttributeValueRequest      `json:"attributes,omitempty"`
	MaterialComposition []ProductMaterialCompositionRequest `json:"materialComposition,omitempty"`
	SizeChartRows       []ProductSizeChartRowRequest        `json:"sizeChartRows,omitempty"`
}

type NormalizedVariantCreateRequest struct {
	SellerSKU    *string                        `json:"sellerSku,omitempty"`
	SKU          *string                        `json:"sku,omitempty"`
	Size         *string                        `json:"size,omitempty"`
	Color        *string                        `json:"color,omitempty"`
	ColorID      *string                        `json:"colorId,omitempty"`
	SizeValueID  *string                        `json:"sizeValueId,omitempty"`
	ShadeName    *string                        `json:"shadeName,omitempty"`
	OptionValues string                         `json:"optionValues,omitempty"`
	PriceCents   *int64                         `json:"priceCents,omitempty"`
	Attributes   []VariantAttributeValueRequest `json:"attributes,omitempty"`
}

type NormalizedImageCreateRequest struct {
	ImageURL  string  `json:"imageUrl"`
	AltText   *string `json:"altText,omitempty"`
	SortOrder int     `json:"sortOrder"`
	ColorID   *string `json:"colorId,omitempty"`
}

func (req *CreateProductRequest) NormalizeForHash() NormalizedProductCreateRequest {
	normVariants := make([]NormalizedVariantCreateRequest, len(req.Variants))
	for i, v := range req.Variants {
		var colorID, sizeValueID, optValues string
		if v.ColorID != nil {
			colorID = v.ColorID.String()
		}
		if v.SizeValueID != nil {
			sizeValueID = v.SizeValueID.String()
		}
		if v.OptionValues != nil {
			bOpt, _ := json.Marshal(v.OptionValues)
			optValues = string(bOpt)
		}

		var attrs []VariantAttributeValueRequest
		if v.Attributes != nil {
			attrs = make([]VariantAttributeValueRequest, len(v.Attributes))
			copy(attrs, v.Attributes)
			sort.Slice(attrs, func(x, y int) bool {
				bX, _ := json.Marshal(attrs[x])
				bY, _ := json.Marshal(attrs[y])
				return string(bX) < string(bY)
			})
		}

		var colorIDPtr, sizeValueIDPtr *string
		if colorID != "" {
			colorIDPtr = &colorID
		}
		if sizeValueID != "" {
			sizeValueIDPtr = &sizeValueID
		}

		normVariants[i] = NormalizedVariantCreateRequest{
			SellerSKU:    v.SellerSKU,
			SKU:          v.SKU,
			Size:         v.Size,
			Color:        v.Color,
			ColorID:      colorIDPtr,
			SizeValueID:  sizeValueIDPtr,
			ShadeName:    v.ShadeName,
			OptionValues: optValues,
			PriceCents:   v.PriceCents,
			Attributes:   attrs,
		}
	}

	sort.Slice(normVariants, func(i, j int) bool {
		bI, _ := json.Marshal(normVariants[i])
		bJ, _ := json.Marshal(normVariants[j])
		return string(bI) < string(bJ)
	})

	normImages := make([]NormalizedImageCreateRequest, len(req.Images))
	for i, img := range req.Images {
		sortOrder := i
		if img.SortOrder != nil {
			sortOrder = *img.SortOrder
		}
		var colorIDPtr *string
		if img.ColorID != nil {
			cStr := img.ColorID.String()
			colorIDPtr = &cStr
		}
		normImages[i] = NormalizedImageCreateRequest{
			ImageURL:  img.ImageURL,
			AltText:   img.AltText,
			SortOrder: sortOrder,
			ColorID:   colorIDPtr,
		}
	}

	var catIDPtr, brandIDPtr *string
	if req.CategoryID != nil {
		cStr := req.CategoryID.String()
		catIDPtr = &cStr
	}
	if req.BrandID != nil {
		bStr := req.BrandID.String()
		brandIDPtr = &bStr
	}

	var slugPtr *string
	slugBase := req.Title
	if req.Slug != nil && *req.Slug != "" {
		slugBase = *req.Slug
	}
	sGen := generateSlug(slugBase)
	slugPtr = &sGen

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	var normAttrs []ProductAttributeValueRequest
	if req.Attributes != nil {
		normAttrs = make([]ProductAttributeValueRequest, len(req.Attributes))
		copy(normAttrs, req.Attributes)
		sort.Slice(normAttrs, func(i, j int) bool {
			bI, _ := json.Marshal(normAttrs[i])
			bJ, _ := json.Marshal(normAttrs[j])
			return string(bI) < string(bJ)
		})
	}

	var normMats []ProductMaterialCompositionRequest
	if req.MaterialComposition != nil {
		normMats = make([]ProductMaterialCompositionRequest, len(req.MaterialComposition))
		copy(normMats, req.MaterialComposition)
		sort.Slice(normMats, func(i, j int) bool {
			bI, _ := json.Marshal(normMats[i])
			bJ, _ := json.Marshal(normMats[j])
			return string(bI) < string(bJ)
		})
	}

	var normSizes []ProductSizeChartRowRequest
	if req.SizeChartRows != nil {
		normSizes = make([]ProductSizeChartRowRequest, len(req.SizeChartRows))
		copy(normSizes, req.SizeChartRows)
		sort.Slice(normSizes, func(i, j int) bool {
			bI, _ := json.Marshal(normSizes[i])
			bJ, _ := json.Marshal(normSizes[j])
			return string(bI) < string(bJ)
		})
	}

	return NormalizedProductCreateRequest{
		Title:               req.Title,
		Slug:                slugPtr,
		Description:         req.Description,
		CategoryID:          catIDPtr,
		BrandID:             brandIDPtr,
		Gender:              req.Gender,
		Color:               req.Color,
		Material:            req.Material,
		CareInstructions:    req.CareInstructions,
		PriceCents:          req.PriceCents,
		OldPriceCents:       req.OldPriceCents,
		Currency:            currency,
		MainImageURL:        req.MainImageURL,
		Variants:            normVariants,
		Images:              normImages,
		Attributes:          normAttrs,
		MaterialComposition: normMats,
		SizeChartRows:       normSizes,
	}
}

func (req *CreateProductRequest) NormalizedHash() string {
	norm := req.NormalizeForHash()
	b, _ := json.Marshal(norm)
	hash := sha256.Sum256(b)
	return fmt.Sprintf("%x", hash)
}
