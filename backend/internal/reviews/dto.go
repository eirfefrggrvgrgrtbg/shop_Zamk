package reviews

import (
	"time"

	"github.com/google/uuid"
)

type CreateReviewRequest struct {
	OrderItemID *uuid.UUID `json:"orderItemId,omitempty"`
	Rating      int        `json:"rating"`
	Title       *string    `json:"title,omitempty"`
	Comment     *string    `json:"comment,omitempty"`
	Text        *string    `json:"text,omitempty"`
}

type AdminRejectReviewRequest struct {
	Comment string `json:"comment"`
}

type AdminModerationRequest struct {
	Comment *string `json:"comment,omitempty"`
}

type ReviewResponse struct {
	ID                uuid.UUID  `json:"id"`
	ProductID         uuid.UUID  `json:"productId"`
	ProductVariantID  *uuid.UUID `json:"variantId,omitempty"`
	ProductTitle      *string    `json:"productTitle,omitempty"`
	OrderItemID       *uuid.UUID `json:"orderItemId,omitempty"`
	Rating            int        `json:"rating"`
	Title             *string    `json:"title,omitempty"`
	Comment           *string    `json:"comment,omitempty"`
	Text              *string    `json:"text,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	PublishedAt       *time.Time `json:"publishedAt,omitempty"`
	ModerationComment *string    `json:"moderationComment,omitempty"`
}

type PublicReviewResponse struct {
	ID                  uuid.UUID `json:"id"`
	Rating              int       `json:"rating"`
	Title               *string   `json:"title,omitempty"`
	Comment             *string   `json:"comment,omitempty"`
	Text                *string   `json:"text,omitempty"`
	ReviewerDisplayName string    `json:"reviewerDisplayName"`
	AuthorName          string    `json:"authorName"` // legacy compatibility
	ProductTitle        string    `json:"productTitle,omitempty"`
	VariantSize         *string   `json:"variantSize,omitempty"`
	VariantColor        *string   `json:"variantColor,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

type PublicReviewRow struct {
	ID                uuid.UUID
	ProductID         uuid.UUID
	ProductVariantID  *uuid.UUID
	OrderID           uuid.UUID
	OrderItemID       uuid.UUID
	UserID            uuid.UUID
	SellerID          uuid.UUID
	Rating            int
	Title             *string
	Comment           *string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	PublishedAt       *time.Time
	RejectedAt        *time.Time
	ModerationComment *string
	ReviewerFirstName string
	ReviewerLastName  string
	OrderItemTitle    string
	OrderItemSize     *string
	OrderItemColor    *string
}

type RatingSummaryResponse struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

type ReviewListResponse struct {
	Items      []ReviewResponse `json:"items"`
	TotalCount int              `json:"totalCount"`
}

type PublicReviewListResponse struct {
	Items         []PublicReviewResponse `json:"items"`
	AverageRating float64                `json:"averageRating"`
	ReviewCount   int                    `json:"reviewCount"`
	TotalCount    int                    `json:"totalCount,omitempty"`
}
