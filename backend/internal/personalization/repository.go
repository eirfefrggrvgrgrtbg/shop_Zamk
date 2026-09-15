package personalization

import (
	"context"
	"errors"
	"fmt"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

var (
	ErrProductNotAccessible = errors.New("product not found or not accessible")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// IsProductAccessible verifies whether a product exists, is published, belongs to an active seller,
// and satisfies the canonical minimum storefront free sellable stock rule (CAT.1A).
func (r *Repository) IsProductAccessible(ctx context.Context, productID uuid.UUID) (bool, error) {
	queryCheck := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1 FROM products p
			INNER JOIN sellers s ON p.seller_id = s.id
			WHERE p.id = $1 AND p.status = 'published' AND s.status = 'active' AND %s >= %d
		)
	`, products.CanonicalProductFreeStockSQL("p.id"), products.MinStorefrontFreeSellableUnits)

	var exists bool
	err := r.db.QueryRow(ctx, queryCheck, productID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check product accessibility: %w", err)
	}
	return exists, nil
}

// RecordProductView records or updates an authenticated customer's product view.
// Concurrency-safe atomic upsert ensuring no duplicate (user_id, product_id) rows.
func (r *Repository) RecordProductView(ctx context.Context, userID, productID uuid.UUID) error {
	accessible, err := r.IsProductAccessible(ctx, productID)
	if err != nil {
		return err
	}
	if !accessible {
		return ErrProductNotAccessible
	}

	queryUpsert := `
		INSERT INTO customer_product_views (user_id, product_id, last_viewed_at, view_count)
		VALUES ($1, $2, now(), 1)
		ON CONFLICT (user_id, product_id)
		DO UPDATE SET
			view_count = customer_product_views.view_count + 1,
			last_viewed_at = now()
	`
	_, err = r.db.Exec(ctx, queryUpsert, userID, productID)
	if err != nil {
		return fmt.Errorf("failed to record product view: %w", err)
	}
	return nil
}

// GetProductView retrieves an existing view record for tests and verification.
func (r *Repository) GetProductView(ctx context.Context, userID, productID uuid.UUID) (*CustomerProductView, error) {
	query := `
		SELECT user_id, product_id, last_viewed_at, view_count
		FROM customer_product_views
		WHERE user_id = $1 AND product_id = $2
	`
	var view CustomerProductView
	err := r.db.QueryRow(ctx, query, userID, productID).Scan(
		&view.UserID,
		&view.ProductID,
		&view.LastViewedAt,
		&view.ViewCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get product view: %w", err)
	}
	return &view, nil
}

// GetRecentlyViewedProducts retrieves a list of recently viewed products for the authenticated user
// that meet the storefront eligibility criteria (published, active seller, sufficient free stock).
func (r *Repository) GetRecentlyViewedProducts(ctx context.Context, userID uuid.UUID, limit int) ([]products.Product, error) {
	query := fmt.Sprintf(`
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
		FROM customer_product_views cv
		INNER JOIN products p ON cv.product_id = p.id
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE cv.user_id = $1
		  AND p.status = 'published'
		  AND s.status = 'active'
		  AND %s >= %d
		ORDER BY cv.last_viewed_at DESC, cv.product_id DESC
		LIMIT $2
	`, products.CanonicalProductFreeStockSQL("p.id"), products.MinStorefrontFreeSellableUnits)

	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recently viewed products: %w", err)
	}
	defer rows.Close()

	var results []products.Product
	for rows.Next() {
		var p products.Product
		if err := rows.Scan(
			&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
			&p.Status, &p.Source, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.AverageRating, &p.ReviewsCount,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
			&p.SellerSlug, &p.SellerName,
		); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if results == nil {
		results = []products.Product{}
	}

	return results, nil
}

// GetStorefrontProduct retrieves an accessible storefront product by ID.
func (r *Repository) GetStorefrontProduct(ctx context.Context, productID uuid.UUID) (*products.Product, error) {
	query := fmt.Sprintf(`
		SELECT p.id, p.category_id, p.brand_id, p.price_cents
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE p.id = $1 AND p.status = 'published' AND s.status = 'active' AND %s >= %d
	`, products.CanonicalProductFreeStockSQL("p.id"), products.MinStorefrontFreeSellableUnits)

	var p products.Product
	err := r.db.QueryRow(ctx, query, productID).Scan(
		&p.ID,
		&p.CategoryID,
		&p.BrandID,
		&p.PriceCents,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotAccessible
		}
		return nil, fmt.Errorf("failed to get storefront product: %w", err)
	}
	return &p, nil
}

// GetSimilarProducts retrieves similar products within the same direct category,
// ranked by deterministic relevance tiers (Tier 1: same brand, Tier 2: other/any brand)
// and closeness (absolute price distance, rating, reviews count, published_at, id).
func (r *Repository) GetSimilarProducts(ctx context.Context, productID uuid.UUID, limit int) ([]products.Product, error) {
	source, err := r.GetStorefrontProduct(ctx, productID)
	if err != nil {
		return nil, err
	}

	if source.CategoryID == nil {
		return []products.Product{}, nil
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE p.id != $1
		  AND p.category_id = $2
		  AND p.status = 'published'
		  AND s.status = 'active'
		  AND %s >= %d
		ORDER BY
		  CASE
		    WHEN $3::uuid IS NOT NULL AND p.brand_id = $3::uuid THEN 1
		    ELSE 2
		  END ASC,
		  ABS(p.price_cents - $4::bigint) ASC,
		  p.average_rating DESC NULLS LAST,
		  p.reviews_count DESC,
		  p.published_at DESC NULLS LAST,
		  p.id ASC
		LIMIT $5
	`, products.CanonicalProductFreeStockSQL("p.id"), products.MinStorefrontFreeSellableUnits)

	rows, err := r.db.Query(ctx, query, source.ID, *source.CategoryID, source.BrandID, source.PriceCents, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get similar products: %w", err)
	}
	defer rows.Close()

	var results []products.Product
	for rows.Next() {
		var p products.Product
		if err := rows.Scan(
			&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
			&p.Status, &p.Source, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.AverageRating, &p.ReviewsCount,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
			&p.SellerSlug, &p.SellerName,
		); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if results == nil {
		results = []products.Product{}
	}

	return results, nil
}

// GetCustomerPreferenceProfile derives an authenticated customer's preference profile on read
// from current favorites and product views without collapsing strong and soft signals into arbitrary scores.
func (r *Repository) GetCustomerPreferenceProfile(ctx context.Context, userID uuid.UUID, limit int) (*CustomerPreferenceProfile, error) {
	if limit <= 0 {
		limit = DefaultProfileAffinityLimit
	} else if limit > MaxProfileAffinityLimit {
		limit = MaxProfileAffinityLimit
	}

	profile := &CustomerPreferenceProfile{
		UserID:             userID,
		FavoriteCategories: make([]CategoryAffinity, 0),
		FavoriteBrands:     make([]BrandAffinity, 0),
		ViewedCategories:   make([]CategoryAffinity, 0),
		ViewedBrands:       make([]BrandAffinity, 0),
	}

	// 1. Favorite Categories (Strong explicit preference)
	// Ranked by distinct favorited products DESC, then trustworthy MAX(created_at) DESC, then category_id ASC.
	favCatQuery := `
		SELECT
			p.category_id,
			COUNT(DISTINCT cf.product_id) AS distinct_product_count,
			MAX(cf.created_at) AS latest_interaction_at
		FROM customer_favorites cf
		INNER JOIN products p ON cf.product_id = p.id
		INNER JOIN categories c ON p.category_id = c.id
		WHERE cf.user_id = $1
		  AND p.category_id IS NOT NULL
		GROUP BY p.category_id
		ORDER BY
			COUNT(DISTINCT cf.product_id) DESC,
			MAX(cf.created_at) DESC,
			p.category_id ASC
		LIMIT $2
	`
	favCatRows, err := r.db.Query(ctx, favCatQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorite categories: %w", err)
	}
	for favCatRows.Next() {
		var catID uuid.UUID
		var count int64
		var latest time.Time
		if err := favCatRows.Scan(&catID, &count, &latest); err != nil {
			favCatRows.Close()
			return nil, fmt.Errorf("failed to scan favorite category row: %w", err)
		}
		t := latest
		profile.FavoriteCategories = append(profile.FavoriteCategories, CategoryAffinity{
			CategoryID:           catID,
			DistinctProductCount: count,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceFavorite,
		})
	}
	favCatRows.Close()
	if favCatRows.Err() != nil {
		return nil, favCatRows.Err()
	}

	// 2. Favorite Brands (Strong explicit preference)
	// Ranked by distinct favorited products DESC, then trustworthy MAX(created_at) DESC, then brand_id ASC.
	favBrandQuery := `
		SELECT
			p.brand_id,
			COUNT(DISTINCT cf.product_id) AS distinct_product_count,
			MAX(cf.created_at) AS latest_interaction_at
		FROM customer_favorites cf
		INNER JOIN products p ON cf.product_id = p.id
		INNER JOIN brands b ON p.brand_id = b.id
		WHERE cf.user_id = $1
		  AND p.brand_id IS NOT NULL
		GROUP BY p.brand_id
		ORDER BY
			COUNT(DISTINCT cf.product_id) DESC,
			MAX(cf.created_at) DESC,
			p.brand_id ASC
		LIMIT $2
	`
	favBrandRows, err := r.db.Query(ctx, favBrandQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorite brands: %w", err)
	}
	for favBrandRows.Next() {
		var brandID uuid.UUID
		var count int64
		var latest time.Time
		if err := favBrandRows.Scan(&brandID, &count, &latest); err != nil {
			favBrandRows.Close()
			return nil, fmt.Errorf("failed to scan favorite brand row: %w", err)
		}
		t := latest
		profile.FavoriteBrands = append(profile.FavoriteBrands, BrandAffinity{
			BrandID:              brandID,
			DistinctProductCount: count,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceFavorite,
		})
	}
	favBrandRows.Close()
	if favBrandRows.Err() != nil {
		return nil, favBrandRows.Err()
	}

	// 3. Viewed Categories (Soft behavioral signal)
	// Distinct product count only (view_count is not used as affinity weight).
	// Ties broken by MAX(last_viewed_at) DESC, then category_id ASC.
	viewCatQuery := `
		SELECT
			p.category_id,
			COUNT(DISTINCT cv.product_id) AS distinct_product_count,
			MAX(cv.last_viewed_at) AS latest_interaction_at
		FROM customer_product_views cv
		INNER JOIN products p ON cv.product_id = p.id
		INNER JOIN categories c ON p.category_id = c.id
		WHERE cv.user_id = $1
		  AND p.category_id IS NOT NULL
		GROUP BY p.category_id
		ORDER BY
			COUNT(DISTINCT cv.product_id) DESC,
			MAX(cv.last_viewed_at) DESC,
			p.category_id ASC
		LIMIT $2
	`
	viewCatRows, err := r.db.Query(ctx, viewCatQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query viewed categories: %w", err)
	}
	for viewCatRows.Next() {
		var catID uuid.UUID
		var count int64
		var latest time.Time
		if err := viewCatRows.Scan(&catID, &count, &latest); err != nil {
			viewCatRows.Close()
			return nil, fmt.Errorf("failed to scan viewed category row: %w", err)
		}
		t := latest
		profile.ViewedCategories = append(profile.ViewedCategories, CategoryAffinity{
			CategoryID:           catID,
			DistinctProductCount: count,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceViewed,
		})
	}
	viewCatRows.Close()
	if viewCatRows.Err() != nil {
		return nil, viewCatRows.Err()
	}

	// 4. Viewed Brands (Soft behavioral signal)
	// Distinct product count only (view_count is not used as affinity weight).
	// Ties broken by MAX(last_viewed_at) DESC, then brand_id ASC.
	viewBrandQuery := `
		SELECT
			p.brand_id,
			COUNT(DISTINCT cv.product_id) AS distinct_product_count,
			MAX(cv.last_viewed_at) AS latest_interaction_at
		FROM customer_product_views cv
		INNER JOIN products p ON cv.product_id = p.id
		INNER JOIN brands b ON p.brand_id = b.id
		WHERE cv.user_id = $1
		  AND p.brand_id IS NOT NULL
		GROUP BY p.brand_id
		ORDER BY
			COUNT(DISTINCT cv.product_id) DESC,
			MAX(cv.last_viewed_at) DESC,
			p.brand_id ASC
		LIMIT $2
	`
	viewBrandRows, err := r.db.Query(ctx, viewBrandQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query viewed brands: %w", err)
	}
	for viewBrandRows.Next() {
		var brandID uuid.UUID
		var count int64
		var latest time.Time
		if err := viewBrandRows.Scan(&brandID, &count, &latest); err != nil {
			viewBrandRows.Close()
			return nil, fmt.Errorf("failed to scan viewed brand row: %w", err)
		}
		t := latest
		profile.ViewedBrands = append(profile.ViewedBrands, BrandAffinity{
			BrandID:              brandID,
			DistinctProductCount: count,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceViewed,
		})
	}
	viewBrandRows.Close()
	if viewBrandRows.Err() != nil {
		return nil, viewBrandRows.Err()
	}

	return profile, nil
}
