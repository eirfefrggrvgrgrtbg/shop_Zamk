package favorites

import (
	"context"
	"errors"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrFavoriteNotFound = errors.New("favorite not found")
	ErrProductNotFound  = errors.New("product not found or not published")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) IsProductPublic(ctx context.Context, productID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM products p
			INNER JOIN sellers s ON p.seller_id = s.id
			WHERE p.id = $1 AND p.status = 'published' AND s.status = 'active'
		)
	`, productID).Scan(&exists)
	return exists, err
}

func (r *Repository) AddFavorite(ctx context.Context, userID, productID uuid.UUID) error {
	public, err := r.IsProductPublic(ctx, productID)
	if err != nil {
		return err
	}
	if !public {
		return ErrProductNotFound
	}

	query := `
		INSERT INTO customer_favorites (id, user_id, product_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, product_id) DO NOTHING
	`
	_, err = r.db.Exec(ctx, query, uuid.New(), userID, productID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return ErrProductNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) RemoveFavorite(ctx context.Context, userID, productID uuid.UUID) error {
	query := `DELETE FROM customer_favorites WHERE user_id = $1 AND product_id = $2`
	cmd, err := r.db.Exec(ctx, query, userID, productID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrFavoriteNotFound
	}
	return nil
}

func (r *Repository) ListFavorites(ctx context.Context, userID uuid.UUID) ([]products.Product, error) {
	query := `
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		INNER JOIN customer_favorites cf ON p.id = cf.product_id
		WHERE cf.user_id = $1 AND p.status = 'published' AND s.status = 'active'
		ORDER BY cf.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []products.Product
	for rows.Next() {
		var p products.Product
		err := rows.Scan(
			&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
			&p.Status, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
