package personalization

import (
	"time"

	"github.com/google/uuid"
)

// CustomerProductView represents an aggregated view record of a product by a customer.
// product_view is a client-observed soft signal that does not alter product, stock, or order state.
type CustomerProductView struct {
	UserID       uuid.UUID `json:"userId"`
	ProductID    uuid.UUID `json:"productId"`
	LastViewedAt time.Time `json:"lastViewedAt"`
	ViewCount    int64     `json:"viewCount"`
}

const (
	AffinityProvenanceFavorite = "favorite"
	AffinityProvenanceViewed   = "viewed"

	DefaultProfileAffinityLimit = 5
	MaxProfileAffinityLimit     = 20
)

// CategoryAffinity captures an authenticated customer's category interest derived from
// a specific signal source (favorites or views).
type CategoryAffinity struct {
	CategoryID           uuid.UUID  `json:"categoryId"`
	DistinctProductCount int64      `json:"distinctProductCount"`
	LatestInteractionAt  *time.Time `json:"latestInteractionAt,omitempty"`
	Provenance           string     `json:"provenance"`
}

// BrandAffinity captures an authenticated customer's brand interest derived from
// a specific signal source (favorites or views).
type BrandAffinity struct {
	BrandID              uuid.UUID  `json:"brandId"`
	DistinctProductCount int64      `json:"distinctProductCount"`
	LatestInteractionAt  *time.Time `json:"latestInteractionAt,omitempty"`
	Provenance           string     `json:"provenance"`
}

// CustomerPreferenceProfile holds separately ranked category and brand affinities
// for an authenticated customer without collapsing strong and soft signals into arbitrary scores.
type CustomerPreferenceProfile struct {
	UserID             uuid.UUID          `json:"userId"`
	FavoriteCategories []CategoryAffinity `json:"favoriteCategories"`
	FavoriteBrands     []BrandAffinity    `json:"favoriteBrands"`
	ViewedCategories   []CategoryAffinity `json:"viewedCategories"`
	ViewedBrands       []BrandAffinity    `json:"viewedBrands"`
}
