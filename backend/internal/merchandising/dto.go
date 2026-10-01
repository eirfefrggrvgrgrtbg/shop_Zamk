package merchandising

import (
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
)

type CreateCollectionRequest struct {
	Slug          string     `json:"slug" validate:"required,lowercase,alphanum_dash"`
	Title         string     `json:"title" validate:"required"`
	Description   *string    `json:"description"`
	CoverImageURL *string    `json:"coverImageUrl"`
	IsActive      bool       `json:"isActive"`
	StartsAt      *time.Time `json:"startsAt"`
	EndsAt        *time.Time `json:"endsAt"`
}

type UpdateCollectionRequest struct {
	Slug          string     `json:"slug" validate:"required,lowercase,alphanum_dash"`
	Title         string     `json:"title" validate:"required"`
	Description   *string    `json:"description"`
	CoverImageURL *string    `json:"coverImageUrl"`
	IsActive      bool       `json:"isActive"`
	StartsAt      *time.Time `json:"startsAt"`
	EndsAt        *time.Time `json:"endsAt"`
}

type ReplaceCollectionItemsRequest struct {
	ProductIDs []uuid.UUID `json:"productIds"`
}

type AdminCollectionDetailResponse struct {
	Collection
	Items []AdminCollectionItem `json:"items"`
}

type AdminCollectionItem struct {
	ProductID uuid.UUID `json:"productId"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	SortOrder int       `json:"sortOrder"`
}

type PublicCollectionDetailResponse struct {
	ID            uuid.UUID                `json:"id"`
	Slug          string                   `json:"slug"`
	Title         string                   `json:"title"`
	Description   *string                  `json:"description"`
	CoverImageURL *string                  `json:"coverImageUrl"`
	Products      []products.PublicProduct `json:"products"`
}
