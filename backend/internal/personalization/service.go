package personalization

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

type Service struct {
	repo        *Repository
	productsSvc *products.Service
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) WithProductsService(productsSvc *products.Service) *Service {
	s.productsSvc = productsSvc
	return s
}

func (s *Service) GetCustomerCatalog(ctx context.Context, userID uuid.UUID, filter products.PublicProductFilter, limit, offset int) (products.PublicProductListResponse, error) {
	if s.productsSvc == nil {
		return products.PublicProductListResponse{}, fmt.Errorf("products service not configured")
	}

	isDefaultSort := filter.Sort == nil || *filter.Sort == "" || *filter.Sort == "default"
	if isDefaultSort {
		profile, err := s.GetCustomerPreferenceProfile(ctx, userID, 0)
		if err != nil {
			return products.PublicProductListResponse{}, fmt.Errorf("failed to get customer preference profile: %w", err)
		}

		if profile != nil {
			favCatIDs := make([]uuid.UUID, 0, len(profile.FavoriteCategories))
			for _, a := range profile.FavoriteCategories {
				favCatIDs = append(favCatIDs, a.CategoryID)
			}
			favBrandIDs := make([]uuid.UUID, 0, len(profile.FavoriteBrands))
			for _, a := range profile.FavoriteBrands {
				favBrandIDs = append(favBrandIDs, a.BrandID)
			}
			viewedCatIDs := make([]uuid.UUID, 0, len(profile.ViewedCategories))
			for _, a := range profile.ViewedCategories {
				viewedCatIDs = append(viewedCatIDs, a.CategoryID)
			}
			viewedBrandIDs := make([]uuid.UUID, 0, len(profile.ViewedBrands))
			for _, a := range profile.ViewedBrands {
				viewedBrandIDs = append(viewedBrandIDs, a.BrandID)
			}

			filter.Affinities = &products.CatalogAffinities{
				FavoriteCategoryIDs: favCatIDs,
				FavoriteBrandIDs:    favBrandIDs,
				ViewedCategoryIDs:   viewedCatIDs,
				ViewedBrandIDs:      viewedBrandIDs,
			}
		}
	}

	return s.productsSvc.ListPublicProducts(ctx, filter, limit, offset)
}

func (s *Service) RecordProductView(ctx context.Context, userID, productID uuid.UUID) error {
	return s.repo.RecordProductView(ctx, userID, productID)
}

func (s *Service) GetProductView(ctx context.Context, userID, productID uuid.UUID) (*CustomerProductView, error) {
	return s.repo.GetProductView(ctx, userID, productID)
}

func (s *Service) GetRecentlyViewedProducts(ctx context.Context, userID uuid.UUID, limit int) ([]products.PublicProduct, error) {
	prods, err := s.repo.GetRecentlyViewedProducts(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}

	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}

func (s *Service) GetSimilarProducts(ctx context.Context, productID uuid.UUID, limit int) ([]products.PublicProduct, error) {
	prods, err := s.repo.GetSimilarProducts(ctx, productID, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}
	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}

// GetCustomerPreferenceProfile returns the authenticated customer's preference profile derived on read
// from current favorites and product views.
func (s *Service) GetCustomerPreferenceProfile(ctx context.Context, userID uuid.UUID, limit int) (*CustomerPreferenceProfile, error) {
	return s.repo.GetCustomerPreferenceProfile(ctx, userID, limit)
}

// GetForYouProducts returns personalized product recommendations for the authenticated customer
// based on their preference profile (favorites and views). If the customer has no preferences or
// no matching storefront-accessible products, an empty slice is returned.
func (s *Service) GetForYouProducts(ctx context.Context, userID uuid.UUID, limit int) ([]products.PublicProduct, error) {
	if limit <= 0 {
		limit = DefaultForYouLimit
	} else if limit > MaxForYouLimit {
		limit = MaxForYouLimit
	}

	profile, err := s.GetCustomerPreferenceProfile(ctx, userID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer preference profile: %w", err)
	}

	if len(profile.Categories) == 0 && len(profile.Brands) == 0 {
		return []products.PublicProduct{}, nil
	}

	prods, err := s.repo.GetForYouProducts(ctx, userID, profile.Categories, profile.Brands, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}
	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}

// GetPopularProducts returns popular storefront products ranked by order_paid quantity in the last 30 days.
func (s *Service) GetPopularProducts(ctx context.Context, limit int) ([]products.PublicProduct, error) {
	if limit <= 0 {
		limit = DefaultPopularLimit
	} else if limit > MaxPopularLimit {
		limit = MaxPopularLimit
	}

	prods, err := s.repo.GetPopularProducts(ctx, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}
	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}

// GetNewProducts returns storefront products published within the last 14 days,
// ranked strictly by published_at DESC, then product_id ASC.
func (s *Service) GetNewProducts(ctx context.Context, limit int) ([]products.PublicProduct, error) {
	if limit <= 0 {
		limit = DefaultNewProductsLimit
	} else if limit > MaxNewProductsLimit {
		limit = MaxNewProductsLimit
	}

	prods, err := s.repo.GetNewProducts(ctx, limit)
	if err != nil {
		return nil, err
	}

	var pubItems []products.PublicProduct
	for _, p := range prods {
		pubItems = append(pubItems, products.MapToPublicProduct(p))
	}
	if pubItems == nil {
		pubItems = []products.PublicProduct{}
	}
	return pubItems, nil
}

// ComposeHomeBlocks executes deterministic cross-block deduplication (For You > Popular > New)
// and intra-block filling up to target items per block.
func ComposeHomeBlocks(forYouCandidates, popularCandidates, newCandidates []products.PublicProduct, target int) []HomeRecommendationBlock {
	if target <= 0 {
		target = TargetHomeRecommendationBlockSize
	}

	seenProductIDs := make(map[uuid.UUID]bool)
	var blocks []HomeRecommendationBlock

	// 1. For You block
	var forYouItems []products.PublicProduct
	for _, p := range forYouCandidates {
		if seenProductIDs[p.ID] {
			continue
		}
		seenProductIDs[p.ID] = true
		forYouItems = append(forYouItems, p)
		if len(forYouItems) == target {
			break
		}
	}
	if len(forYouItems) > 0 {
		blocks = append(blocks, HomeRecommendationBlock{
			Type:  RecommendationBlockTypeForYou,
			Title: RecommendationBlockTitleForYou,
			Items: forYouItems,
		})
	}

	// 2. Popular block (excluding items from For You and previous duplicates)
	var popularItems []products.PublicProduct
	for _, p := range popularCandidates {
		if seenProductIDs[p.ID] {
			continue
		}
		seenProductIDs[p.ID] = true
		popularItems = append(popularItems, p)
		if len(popularItems) == target {
			break
		}
	}
	if len(popularItems) > 0 {
		blocks = append(blocks, HomeRecommendationBlock{
			Type:  RecommendationBlockTypePopular,
			Title: RecommendationBlockTitlePopular,
			Items: popularItems,
		})
	}

	// 3. New block (excluding items from For You, Popular, and previous duplicates)
	var newItems []products.PublicProduct
	for _, p := range newCandidates {
		if seenProductIDs[p.ID] {
			continue
		}
		seenProductIDs[p.ID] = true
		newItems = append(newItems, p)
		if len(newItems) == target {
			break
		}
	}
	if len(newItems) > 0 {
		blocks = append(blocks, HomeRecommendationBlock{
			Type:  RecommendationBlockTypeNew,
			Title: RecommendationBlockTitleNew,
			Items: newItems,
		})
	}

	if blocks == nil {
		blocks = []HomeRecommendationBlock{}
	}

	return blocks
}

// GetHomeRecommendations retrieves composed discovery recommendation blocks for an authenticated customer.
func (s *Service) GetHomeRecommendations(ctx context.Context, userID uuid.UUID) (*HomeRecommendationsResponse, error) {
	return s.GetHomeRecommendationsWithTarget(ctx, userID, TargetHomeRecommendationBlockSize)
}

// GetHomeRecommendationsWithTarget retrieves composed discovery recommendation blocks with a specified target block size.
func (s *Service) GetHomeRecommendationsWithTarget(ctx context.Context, userID uuid.UUID, target int) (*HomeRecommendationsResponse, error) {
	if target <= 0 {
		target = TargetHomeRecommendationBlockSize
	}

	// 1. Fetch For You candidates (overfetch target * 3, capped at source max limit)
	forYouFetchLimit := target * 3
	if forYouFetchLimit > MaxForYouLimit {
		forYouFetchLimit = MaxForYouLimit
	}
	forYouCandidates, err := s.GetForYouProducts(ctx, userID, forYouFetchLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get for-you products for home recommendations: %w", err)
	}

	// 2. Fetch Popular candidates (overfetch target * 3, capped at source max limit)
	popularFetchLimit := target * 3
	if popularFetchLimit > MaxPopularLimit {
		popularFetchLimit = MaxPopularLimit
	}
	popularCandidates, err := s.GetPopularProducts(ctx, popularFetchLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular products for home recommendations: %w", err)
	}

	// 3. Fetch New candidates (overfetch target * 3, capped at source max limit)
	newFetchLimit := target * 3
	if newFetchLimit > MaxNewProductsLimit {
		newFetchLimit = MaxNewProductsLimit
	}
	newCandidates, err := s.GetNewProducts(ctx, newFetchLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get new products for home recommendations: %w", err)
	}

	blocks := ComposeHomeBlocks(forYouCandidates, popularCandidates, newCandidates, target)
	return &HomeRecommendationsResponse{Blocks: blocks}, nil
}
