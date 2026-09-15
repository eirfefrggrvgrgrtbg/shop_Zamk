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
