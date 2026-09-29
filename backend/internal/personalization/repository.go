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
// from canonical behavioral_events that meet storefront eligibility criteria (published, active seller, sufficient free stock).
func (r *Repository) GetRecentlyViewedProducts(ctx context.Context, userID uuid.UUID, limit int) ([]products.Product, error) {
	query := fmt.Sprintf(`
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
		FROM (
			SELECT product_id, MAX(occurred_at) AS last_viewed_at
			FROM behavioral_events
			WHERE user_id = $1
			  AND event_type = 'product_view'
			  AND product_id IS NOT NULL
			  AND occurred_at >= now() - INTERVAL '30 days'
			GROUP BY product_id
		) v
		INNER JOIN products p ON v.product_id = p.id
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE p.status = 'published'
		  AND s.status = 'active'
		  AND %s >= %d
		ORDER BY v.last_viewed_at DESC, p.id ASC
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
// deterministically from canonical behavioral events (product_view, favorite_added/favorite_removed, order_paid)
// with fixed transparent weights (view=1 < favorite=3 < order_paid=10).
func (r *Repository) GetCustomerPreferenceProfile(ctx context.Context, userID uuid.UUID, limit int) (*CustomerPreferenceProfile, error) {
	if limit <= 0 {
		limit = DefaultProfileAffinityLimit
	} else if limit > MaxProfileAffinityLimit {
		limit = MaxProfileAffinityLimit
	}

	profile := &CustomerPreferenceProfile{
		UserID:             userID,
		Categories:         make([]CategoryAffinity, 0),
		Brands:             make([]BrandAffinity, 0),
		FavoriteCategories: make([]CategoryAffinity, 0),
		FavoriteBrands:     make([]BrandAffinity, 0),
		ViewedCategories:   make([]CategoryAffinity, 0),
		ViewedBrands:       make([]BrandAffinity, 0),
	}

	signalsCTE := fmt.Sprintf(`
		WITH ranked_fav_events AS (
			SELECT product_id, event_type, occurred_at,
				   ROW_NUMBER() OVER (PARTITION BY product_id ORDER BY occurred_at DESC, id DESC) AS rn
			FROM behavioral_events
			WHERE user_id = $1
			  AND event_type IN ('favorite_added', 'favorite_removed')
			  AND product_id IS NOT NULL
		),
		active_favs AS (
			SELECT rfe.product_id, rfe.occurred_at
			FROM ranked_fav_events rfe
			WHERE rfe.rn = 1 AND rfe.event_type = 'favorite_added'

			UNION ALL

			SELECT cf.product_id, cf.created_at AS occurred_at
			FROM customer_favorites cf
			WHERE cf.user_id = $1
			  AND NOT EXISTS (
				  SELECT 1 FROM ranked_fav_events rfe
				  WHERE rfe.product_id = cf.product_id
			  )
		),
		signals AS (
			-- 1. product_view: weak signal (weight 1), lookback 30 days
			SELECT e.product_id, p.category_id, p.brand_id, e.occurred_at, %d AS weight, 'viewed' AS signal_type
			FROM behavioral_events e
			INNER JOIN products p ON e.product_id = p.id
			WHERE e.user_id = $1
			  AND e.event_type = 'product_view'
			  AND e.product_id IS NOT NULL
			  AND e.occurred_at >= now() - INTERVAL '%d days'

			UNION ALL

			-- 2. active favorite: medium signal (weight 3), current state
			SELECT af.product_id, p.category_id, p.brand_id, af.occurred_at, %d AS weight, 'favorite' AS signal_type
			FROM active_favs af
			INNER JOIN products p ON af.product_id = p.id

			UNION ALL

			-- 3. order_paid: strong signal (weight 10), lookback 180 days
			SELECT e.product_id, p.category_id, p.brand_id, e.occurred_at, %d AS weight, 'paid' AS signal_type
			FROM behavioral_events e
			INNER JOIN products p ON e.product_id = p.id
			WHERE e.user_id = $1
			  AND e.event_type = 'order_paid'
			  AND e.product_id IS NOT NULL
			  AND e.occurred_at >= now() - INTERVAL '%d days'
		)
	`, WeightProductView, LookbackProductViewDays, WeightFavorite, WeightOrderPaid, LookbackOrderPaidDays)

	// 1. Unified Categories Profile (ranked by score DESC, then latest_interaction_at DESC, then category_id ASC)
	catQuery := signalsCTE + `
		SELECT
			s.category_id,
			COUNT(DISTINCT s.product_id) AS distinct_product_count,
			SUM(s.weight) AS score,
			MAX(s.occurred_at) AS latest_interaction_at
		FROM signals s
		INNER JOIN categories c ON s.category_id = c.id
		WHERE s.category_id IS NOT NULL
		GROUP BY s.category_id
		ORDER BY
			SUM(s.weight) DESC,
			MAX(s.occurred_at) DESC,
			s.category_id ASC
		LIMIT $2
	`
	catRows, err := r.db.Query(ctx, catQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query customer category affinities: %w", err)
	}
	for catRows.Next() {
		var catID uuid.UUID
		var count int64
		var score int64
		var latest time.Time
		if err := catRows.Scan(&catID, &count, &score, &latest); err != nil {
			catRows.Close()
			return nil, fmt.Errorf("failed to scan category affinity row: %w", err)
		}
		t := latest
		profile.Categories = append(profile.Categories, CategoryAffinity{
			CategoryID:           catID,
			DistinctProductCount: count,
			Score:                score,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceAggregate,
		})
	}
	catRows.Close()
	if catRows.Err() != nil {
		return nil, catRows.Err()
	}

	// 2. Unified Brands Profile (ranked by score DESC, then latest_interaction_at DESC, then brand_id ASC)
	brandQuery := signalsCTE + `
		SELECT
			s.brand_id,
			COUNT(DISTINCT s.product_id) AS distinct_product_count,
			SUM(s.weight) AS score,
			MAX(s.occurred_at) AS latest_interaction_at
		FROM signals s
		INNER JOIN brands b ON s.brand_id = b.id
		WHERE s.brand_id IS NOT NULL
		GROUP BY s.brand_id
		ORDER BY
			SUM(s.weight) DESC,
			MAX(s.occurred_at) DESC,
			s.brand_id ASC
		LIMIT $2
	`
	brandRows, err := r.db.Query(ctx, brandQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query customer brand affinities: %w", err)
	}
	for brandRows.Next() {
		var brandID uuid.UUID
		var count int64
		var score int64
		var latest time.Time
		if err := brandRows.Scan(&brandID, &count, &score, &latest); err != nil {
			brandRows.Close()
			return nil, fmt.Errorf("failed to scan brand affinity row: %w", err)
		}
		t := latest
		profile.Brands = append(profile.Brands, BrandAffinity{
			BrandID:              brandID,
			DistinctProductCount: count,
			Score:                score,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceAggregate,
		})
	}
	brandRows.Close()
	if brandRows.Err() != nil {
		return nil, brandRows.Err()
	}

	// 3. Favorite Categories (for backward-compatible downstream consumers)
	favCatQuery := signalsCTE + `
		SELECT
			s.category_id,
			COUNT(DISTINCT s.product_id) AS distinct_product_count,
			SUM(s.weight) AS score,
			MAX(s.occurred_at) AS latest_interaction_at
		FROM signals s
		INNER JOIN categories c ON s.category_id = c.id
		WHERE s.signal_type = 'favorite' AND s.category_id IS NOT NULL
		GROUP BY s.category_id
		ORDER BY
			COUNT(DISTINCT s.product_id) DESC,
			MAX(s.occurred_at) DESC,
			s.category_id ASC
		LIMIT $2
	`
	favCatRows, err := r.db.Query(ctx, favCatQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorite categories: %w", err)
	}
	for favCatRows.Next() {
		var catID uuid.UUID
		var count int64
		var score int64
		var latest time.Time
		if err := favCatRows.Scan(&catID, &count, &score, &latest); err != nil {
			favCatRows.Close()
			return nil, fmt.Errorf("failed to scan favorite category row: %w", err)
		}
		t := latest
		profile.FavoriteCategories = append(profile.FavoriteCategories, CategoryAffinity{
			CategoryID:           catID,
			DistinctProductCount: count,
			Score:                score,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceFavorite,
		})
	}
	favCatRows.Close()
	if favCatRows.Err() != nil {
		return nil, favCatRows.Err()
	}

	// 4. Favorite Brands (for backward-compatible downstream consumers)
	favBrandQuery := signalsCTE + `
		SELECT
			s.brand_id,
			COUNT(DISTINCT s.product_id) AS distinct_product_count,
			SUM(s.weight) AS score,
			MAX(s.occurred_at) AS latest_interaction_at
		FROM signals s
		INNER JOIN brands b ON s.brand_id = b.id
		WHERE s.signal_type = 'favorite' AND s.brand_id IS NOT NULL
		GROUP BY s.brand_id
		ORDER BY
			COUNT(DISTINCT s.product_id) DESC,
			MAX(s.occurred_at) DESC,
			s.brand_id ASC
		LIMIT $2
	`
	favBrandRows, err := r.db.Query(ctx, favBrandQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorite brands: %w", err)
	}
	for favBrandRows.Next() {
		var brandID uuid.UUID
		var count int64
		var score int64
		var latest time.Time
		if err := favBrandRows.Scan(&brandID, &count, &score, &latest); err != nil {
			favBrandRows.Close()
			return nil, fmt.Errorf("failed to scan favorite brand row: %w", err)
		}
		t := latest
		profile.FavoriteBrands = append(profile.FavoriteBrands, BrandAffinity{
			BrandID:              brandID,
			DistinctProductCount: count,
			Score:                score,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceFavorite,
		})
	}
	favBrandRows.Close()
	if favBrandRows.Err() != nil {
		return nil, favBrandRows.Err()
	}

	// 5. Viewed Categories (from product_view in behavioral_events, 30 days lookback)
	viewCatQuery := signalsCTE + `
		SELECT
			s.category_id,
			COUNT(DISTINCT s.product_id) AS distinct_product_count,
			SUM(s.weight) AS score,
			MAX(s.occurred_at) AS latest_interaction_at
		FROM signals s
		INNER JOIN categories c ON s.category_id = c.id
		WHERE s.signal_type = 'viewed' AND s.category_id IS NOT NULL
		GROUP BY s.category_id
		ORDER BY
			COUNT(DISTINCT s.product_id) DESC,
			MAX(s.occurred_at) DESC,
			s.category_id ASC
		LIMIT $2
	`
	viewCatRows, err := r.db.Query(ctx, viewCatQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query viewed categories: %w", err)
	}
	for viewCatRows.Next() {
		var catID uuid.UUID
		var count int64
		var score int64
		var latest time.Time
		if err := viewCatRows.Scan(&catID, &count, &score, &latest); err != nil {
			viewCatRows.Close()
			return nil, fmt.Errorf("failed to scan viewed category row: %w", err)
		}
		t := latest
		profile.ViewedCategories = append(profile.ViewedCategories, CategoryAffinity{
			CategoryID:           catID,
			DistinctProductCount: count,
			Score:                score,
			LatestInteractionAt:  &t,
			Provenance:           AffinityProvenanceViewed,
		})
	}
	viewCatRows.Close()
	if viewCatRows.Err() != nil {
		return nil, viewCatRows.Err()
	}

	// 6. Viewed Brands (from product_view in behavioral_events, 30 days lookback)
	viewBrandQuery := signalsCTE + `
		SELECT
			s.brand_id,
			COUNT(DISTINCT s.product_id) AS distinct_product_count,
			SUM(s.weight) AS score,
			MAX(s.occurred_at) AS latest_interaction_at
		FROM signals s
		INNER JOIN brands b ON s.brand_id = b.id
		WHERE s.signal_type = 'viewed' AND s.brand_id IS NOT NULL
		GROUP BY s.brand_id
		ORDER BY
			COUNT(DISTINCT s.product_id) DESC,
			MAX(s.occurred_at) DESC,
			s.brand_id ASC
		LIMIT $2
	`
	viewBrandRows, err := r.db.Query(ctx, viewBrandQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query viewed brands: %w", err)
	}
	for viewBrandRows.Next() {
		var brandID uuid.UUID
		var count int64
		var score int64
		var latest time.Time
		if err := viewBrandRows.Scan(&brandID, &count, &score, &latest); err != nil {
			viewBrandRows.Close()
			return nil, fmt.Errorf("failed to scan viewed brand row: %w", err)
		}
		t := latest
		profile.ViewedBrands = append(profile.ViewedBrands, BrandAffinity{
			BrandID:              brandID,
			DistinctProductCount: count,
			Score:                score,
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

// GetForYouProducts retrieves personalized storefront product recommendations for the authenticated customer
// ranked by deterministic affinity score (category affinity score + brand affinity score) derived from
// the customer preference profile (product_view = 1, favorite = 3, order_paid = 10).
// Storefront eligibility and exclusion of currently favorited products are preserved.
func (r *Repository) GetForYouProducts(
	ctx context.Context,
	userID uuid.UUID,
	categories []CategoryAffinity,
	brands []BrandAffinity,
	limit int,
) ([]products.Product, error) {
	if limit <= 0 {
		limit = DefaultForYouLimit
	} else if limit > MaxForYouLimit {
		limit = MaxForYouLimit
	}

	if len(categories) == 0 && len(brands) == 0 {
		return []products.Product{}, nil
	}

	catIDs := make([]uuid.UUID, len(categories))
	catScores := make([]int64, len(categories))
	for i, c := range categories {
		catIDs[i] = c.CategoryID
		catScores[i] = c.Score
	}

	brandIDs := make([]uuid.UUID, len(brands))
	brandScores := make([]int64, len(brands))
	for i, b := range brands {
		brandIDs[i] = b.BrandID
		brandScores[i] = b.Score
	}

	query := fmt.Sprintf(`
		WITH profile_cats AS (
			SELECT cat_id, score FROM unnest($2::uuid[], $3::bigint[]) AS t(cat_id, score)
		),
		profile_brands AS (
			SELECT brand_id, score FROM unnest($4::uuid[], $5::bigint[]) AS t(brand_id, score)
		)
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		LEFT JOIN profile_cats pc ON p.category_id = pc.cat_id
		LEFT JOIN profile_brands pb ON p.brand_id = pb.brand_id
		WHERE p.status = 'published'
		  AND s.status = 'active'
		  AND %s >= %d
		  AND p.id NOT IN (
		      SELECT product_id FROM customer_favorites cf WHERE user_id = $1
		      AND NOT EXISTS (
		          SELECT 1 FROM (
		              SELECT product_id, event_type,
		                     ROW_NUMBER() OVER (PARTITION BY product_id ORDER BY occurred_at DESC, id DESC) AS rn
		              FROM behavioral_events
		              WHERE user_id = $1 AND event_type IN ('favorite_added', 'favorite_removed') AND product_id IS NOT NULL
		          ) rfe WHERE rfe.product_id = cf.product_id AND rfe.rn = 1 AND rfe.event_type = 'favorite_removed'
		      )
		      UNION
		      SELECT product_id FROM (
		          SELECT product_id, event_type,
		                 ROW_NUMBER() OVER (PARTITION BY product_id ORDER BY occurred_at DESC, id DESC) AS rn
		          FROM behavioral_events
		          WHERE user_id = $1 AND event_type IN ('favorite_added', 'favorite_removed') AND product_id IS NOT NULL
		      ) rfe WHERE rfe.rn = 1 AND rfe.event_type = 'favorite_added'
		  )
		  AND (pc.score IS NOT NULL OR pb.score IS NOT NULL)
		ORDER BY
		  (COALESCE(pc.score, 0) + COALESCE(pb.score, 0)) DESC,
		  p.average_rating DESC NULLS LAST,
		  p.reviews_count DESC,
		  p.published_at DESC NULLS LAST,
		  p.id ASC
		LIMIT $6
	`, products.CanonicalProductFreeStockSQL("p.id"), products.MinStorefrontFreeSellableUnits)

	rows, err := r.db.Query(ctx, query, userID, catIDs, catScores, brandIDs, brandScores, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get for-you products: %w", err)
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

// GetPopularProducts retrieves storefront products ranked by order_paid quantity in the last 30 days.
func (r *Repository) GetPopularProducts(ctx context.Context, limit int) ([]products.Product, error) {
	if limit <= 0 {
		limit = DefaultPopularLimit
	} else if limit > MaxPopularLimit {
		limit = MaxPopularLimit
	}

	query := fmt.Sprintf(`
		WITH popular_sales AS (
			SELECT
				e.product_id,
				SUM(e.quantity) AS paid_quantity
			FROM behavioral_events e
			WHERE e.event_type = 'order_paid'
			  AND e.product_id IS NOT NULL
			  AND e.quantity IS NOT NULL
			  AND e.quantity > 0
			  AND e.occurred_at >= now() - INTERVAL '%d days'
			GROUP BY e.product_id
		)
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
		FROM popular_sales ps
		INNER JOIN products p ON ps.product_id = p.id
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE p.status = 'published'
		  AND s.status = 'active'
		  AND %s >= %d
		ORDER BY
		  ps.paid_quantity DESC,
		  p.average_rating DESC NULLS LAST,
		  p.reviews_count DESC,
		  p.published_at DESC NULLS LAST,
		  p.id ASC
		LIMIT $1
	`, LookbackPopularDays, products.CanonicalProductFreeStockSQL("p.id"), products.MinStorefrontFreeSellableUnits)

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular products: %w", err)
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
