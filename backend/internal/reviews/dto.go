package reviews

import (
	"time"

	"github.com/google/uuid"
)

type CreateReviewRequest struct {
	Rating  int     `json:"rating"`
	Title   *string `json:"title,omitempty"`
	Comment *string `json:"comment,omitempty"`
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
	ProductTitle      *string    `json:"productTitle,omitempty"`
	Rating            int        `json:"rating"`
	Title             *string    `json:"title,omitempty"`
	Comment           *string    `json:"comment,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	PublishedAt       *time.Time `json:"publishedAt,omitempty"`
	ModerationComment *string    `json:"moderationComment,omitempty"`
}

type PublicReviewResponse struct {
	ID         uuid.UUID `json:"id"`
	Rating     int       `json:"rating"`
	Title      *string   `json:"title,omitempty"`
	Comment    *string   `json:"comment,omitempty"`
	AuthorName string    `json:"authorName"`
	CreatedAt  time.Time `json:"createdAt"`
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
	Items      []PublicReviewResponse `json:"items"`
	TotalCount int                    `json:"totalCount"`
}
