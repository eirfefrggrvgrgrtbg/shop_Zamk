package personalization

import (
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
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
	AffinityProvenanceFavorite  = "favorite"
	AffinityProvenanceViewed    = "viewed"
	AffinityProvenanceAggregate = "aggregate"

	WeightProductView = 1
	WeightFavorite    = 3
	WeightOrderPaid   = 10

	LookbackProductViewDays = 30
	LookbackOrderPaidDays   = 180

	DefaultProfileAffinityLimit = 5
	MaxProfileAffinityLimit     = 20

	DefaultForYouLimit = 12
	MaxForYouLimit     = 24

	DefaultPopularLimit = 12
	MaxPopularLimit     = 50
	LookbackPopularDays = 30

	DefaultNewProductsLimit = 12
	MaxNewProductsLimit     = 50
	LookbackNewProductsDays = 14

	DefaultSimilarProductsLimit = 8
	MaxSimilarProductsLimit     = 50

	RecommendationBlockTypeForYou = "for_you"
	RecommendationBlockTypePopular = "popular"
	RecommendationBlockTypeNew     = "new"

	RecommendationBlockTitleForYou = "Для вас"
	RecommendationBlockTitlePopular = "Популярное"
	RecommendationBlockTitleNew     = "Новинки"

	TargetHomeRecommendationBlockSize = 12
)

// HomeRecommendationBlock represents a single discovery recommendation carousel block.
type HomeRecommendationBlock struct {
	Type  string                   `json:"type"`
	Title string                   `json:"title"`
	Items []products.PublicProduct `json:"items"`
}

// HomeRecommendationsResponse represents the composed discovery recommendation response for Home.
type HomeRecommendationsResponse struct {
	Blocks []HomeRecommendationBlock `json:"blocks"`
}

// CategoryAffinity captures an authenticated customer's category interest derived from
// behavioral signals or specific provenances.
type CategoryAffinity struct {
	CategoryID           uuid.UUID  `json:"categoryId"`
	DistinctProductCount int64      `json:"distinctProductCount"`
	Score                int64      `json:"score"`
	LatestInteractionAt  *time.Time `json:"latestInteractionAt,omitempty"`
	Provenance           string     `json:"provenance"`
}

// BrandAffinity captures an authenticated customer's brand interest derived from
// behavioral signals or specific provenances.
type BrandAffinity struct {
	BrandID              uuid.UUID  `json:"brandId"`
	DistinctProductCount int64      `json:"distinctProductCount"`
	Score                int64      `json:"score"`
	LatestInteractionAt  *time.Time `json:"latestInteractionAt,omitempty"`
	Provenance           string     `json:"provenance"`
}

// CustomerPreferenceProfile holds aggregated category and brand affinities
// for an authenticated customer derived deterministically from canonical behavioral signals.
type CustomerPreferenceProfile struct {
	UserID             uuid.UUID          `json:"userId"`
	Categories         []CategoryAffinity `json:"categories"`
	Brands             []BrandAffinity    `json:"brands"`
	FavoriteCategories []CategoryAffinity `json:"favoriteCategories"`
	FavoriteBrands     []BrandAffinity    `json:"favoriteBrands"`
	ViewedCategories   []CategoryAffinity `json:"viewedCategories"`
	ViewedBrands       []BrandAffinity    `json:"viewedBrands"`
}
