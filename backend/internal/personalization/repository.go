package personalization

import (
	"context"
	"errors"
	"fmt"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
