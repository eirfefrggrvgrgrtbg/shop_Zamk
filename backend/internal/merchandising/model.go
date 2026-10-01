package merchandising

import (
	"time"

	"github.com/google/uuid"
)

type Collection struct {
	ID            uuid.UUID  `json:"id"`
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	Description   *string    `json:"description"`
	CoverImageURL *string    `json:"coverImageUrl"`
	IsActive      bool       `json:"isActive"`
	StartsAt      *time.Time `json:"startsAt"`
	EndsAt        *time.Time `json:"endsAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type CollectionItem struct {
	CollectionID uuid.UUID `json:"collectionId"`
	ProductID    uuid.UUID `json:"productId"`
	SortOrder    int       `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
}
