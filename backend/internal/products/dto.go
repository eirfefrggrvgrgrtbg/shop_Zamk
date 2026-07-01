package products

import "github.com/google/uuid"

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
}

type ProductVariantRequest struct {
	SKU        *string `json:"sku,omitempty"`
	Size       *string `json:"size,omitempty"`
	Color      *string `json:"color,omitempty"`
	Barcode    *string `json:"barcode,omitempty"`
	PriceCents *int64  `json:"priceCents,omitempty" validate:"omitempty,min=0"`
}

type ProductImageRequest struct {
	ImageURL  string  `json:"imageUrl" validate:"required"`
	AltText   *string `json:"altText,omitempty"`
	SortOrder *int    `json:"sortOrder,omitempty"`
}

type SubmitProductModerationRequest struct {
	Comment *string `json:"comment,omitempty"`
}

type AdminProductModerationRequest struct {
	Comment *string `json:"comment,omitempty"`
}

type RejectProductRequest struct {
	Comment string `json:"comment" validate:"required"`
}

type ProductListResponse struct {
	Items      []Product `json:"items"`
	TotalCount int       `json:"totalCount"`
}

type ModerationHistoryItem struct {
	ID         uuid.UUID `json:"id"`
	ProductID  uuid.UUID `json:"productId"`
	FromStatus *string   `json:"fromStatus,omitempty"`
	ToStatus   string    `json:"toStatus"`
	Comment    *string   `json:"comment,omitempty"`
	CreatedAt  string    `json:"createdAt"` // Formatting time to ISO8601
}

type ModerationHistoryResponse struct {
	Items []ModerationHistoryItem `json:"items"`
}

type PublicProductFilter struct {
	Query         *string    `json:"q,omitempty"`
	CategoryID    *uuid.UUID `json:"categoryId,omitempty"`
	BrandID       *uuid.UUID `json:"brandId,omitempty"`
	SellerID      *uuid.UUID `json:"sellerId,omitempty"`
	MinPriceCents *int64     `json:"minPriceCents,omitempty"`
	MaxPriceCents *int64     `json:"maxPriceCents,omitempty"`
	InStock       *bool      `json:"inStock,omitempty"`
	Sort          *string    `json:"sort,omitempty"`
}
