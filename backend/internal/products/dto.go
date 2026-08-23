package products

import (
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
	InitialStock *int    `json:"initialStock,omitempty" validate:"omitempty,min=0"`
}

type ProductImageRequest struct {
	ImageURL  string  `json:"imageUrl" validate:"required"`
	AltText   *string `json:"altText,omitempty"`
	SortOrder *int    `json:"sortOrder,omitempty"`
	ColorID   *uuid.UUID `json:"colorId,omitempty"`
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
	ID           uuid.UUID `json:"id"`
	ProductID    uuid.UUID `json:"productId"`
	Size         *string   `json:"size,omitempty"`
	Color        *string   `json:"color,omitempty"`
	OptionValues map[string]interface{} `json:"optionValues,omitempty"`
	SellerSKU          *string `json:"sellerSku,omitempty"`
	ColorID            *uuid.UUID `json:"colorId,omitempty"`
	SizeValueID        *uuid.UUID `json:"sizeValueId,omitempty"`
	ColorName          *string `json:"colorName,omitempty"`
	ColorHex           *string `json:"colorHex,omitempty"`
	ShadeName          *string `json:"shadeName,omitempty"`
	PriceCents   *int64    `json:"priceCents,omitempty"`
	IsActive     bool      `json:"isActive"`
	InStock      *bool     `json:"inStock,omitempty"`
}

type PublicProductImage struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"productId"`
	ImageURL  string    `json:"imageUrl"`
	AltText   *string   `json:"altText,omitempty"`
	SortOrder int       `json:"sortOrder"`
	ColorID   *uuid.UUID `json:"colorId,omitempty"`
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

type PublicProductFilter struct {
	Query         *string    `json:"q,omitempty"`
	CategoryID    *uuid.UUID `json:"categoryId,omitempty"`
	BrandID       *uuid.UUID `json:"brandId,omitempty"`
	SellerID      *uuid.UUID `json:"sellerId,omitempty"`
	Size          *string    `json:"size,omitempty"`
	MinPriceCents *int64     `json:"minPriceCents,omitempty"`
	MaxPriceCents *int64     `json:"maxPriceCents,omitempty"`
	InStock       *bool      `json:"inStock,omitempty"`
	Sort          *string    `json:"sort,omitempty"`
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
