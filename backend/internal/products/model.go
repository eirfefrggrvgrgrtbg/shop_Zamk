package products

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusDraft             = "draft"
	StatusPendingModeration = "pending_moderation"
	StatusApproved          = "approved"
	StatusPublished         = "published"
	StatusRejected          = "rejected"
	StatusHidden            = "hidden"
	StatusBlocked           = "blocked"
	StatusOutOfStock        = "out_of_stock"
)

type Product struct {
	ID                uuid.UUID  `json:"id"`
	SellerID          uuid.UUID  `json:"sellerId"`
	CategoryID        *uuid.UUID `json:"categoryId,omitempty"`
	BrandID           *uuid.UUID `json:"brandId,omitempty"`
	Title             string     `json:"title"`
	Slug              string     `json:"slug"`
	Description       *string    `json:"description,omitempty"`
	Status            string     `json:"status"`
	Gender            *string    `json:"gender,omitempty"`
	Color             *string    `json:"color,omitempty"`
	Material          *string    `json:"material,omitempty"`
	CareInstructions  *string    `json:"careInstructions,omitempty"`
	PriceCents        int64      `json:"priceCents"`
	OldPriceCents     *int64     `json:"oldPriceCents,omitempty"`
	Currency          string     `json:"currency"`
	MainImageURL      *string    `json:"mainImageUrl,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	SubmittedAt       *time.Time `json:"submittedAt,omitempty"`
	ApprovedAt        *time.Time `json:"approvedAt,omitempty"`
	PublishedAt       *time.Time `json:"publishedAt,omitempty"`
	RejectedAt        *time.Time `json:"rejectedAt,omitempty"`
	ModerationComment *string    `json:"moderationComment,omitempty"`
	InStock           *bool      `json:"inStock,omitempty"`

	// Associations
	Variants []ProductVariant `json:"variants,omitempty"`
	Images   []ProductImage   `json:"images,omitempty"`

	// Metrics
	Rating *RatingSummary `json:"rating,omitempty"`
}

type RatingSummary struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

type ProductVariant struct {
	ID         uuid.UUID `json:"id"`
	ProductID  uuid.UUID `json:"productId"`
	SKU        *string   `json:"sku,omitempty"`
	Size       *string   `json:"size,omitempty"`
	Color      *string   `json:"color,omitempty"`
	Barcode    *string   `json:"barcode,omitempty"`
	PriceCents *int64    `json:"priceCents,omitempty"`
	IsActive   bool      `json:"isActive"`
	InStock    *bool     `json:"inStock,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ProductImage struct {
	ID         uuid.UUID `json:"id"`
	ProductID  uuid.UUID `json:"productId"`
	ImageURL   string    `json:"imageUrl"`
	AltText    *string   `json:"altText,omitempty"`
	SortOrder  int       `json:"sortOrder"`
	CreatedAt  time.Time `json:"createdAt"`
}

type ProductModerationLog struct {
	ID          uuid.UUID  `json:"id"`
	ProductID   uuid.UUID  `json:"productId"`
	AdminUserID *uuid.UUID `json:"adminUserId,omitempty"`
	FromStatus  *string    `json:"fromStatus,omitempty"`
	ToStatus    string     `json:"toStatus"`
	Comment     *string    `json:"comment,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}
