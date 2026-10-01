package merchandising

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrCollectionNotFound = errors.New("collection not found")
	ErrDuplicateSlug      = errors.New("slug already exists")
	ErrProductNotFound    = errors.New("product not found")
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) CreateCollection(ctx context.Context, c *Collection) error {
	query := `
		INSERT INTO collections (id, slug, title, description, cover_image_url, is_active, starts_at, ends_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		c.ID, c.Slug, c.Title, c.Description, c.CoverImageURL,
		c.IsActive, c.StartsAt, c.EndsAt, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("failed to create collection: %w", err)
	}
	return nil
}

func (r *Repository) UpdateCollection(ctx context.Context, c *Collection) error {
	query := `
		UPDATE collections
		SET slug = $1, title = $2, description = $3, cover_image_url = $4, is_active = $5, starts_at = $6, ends_at = $7, updated_at = $8
		WHERE id = $9
	`
	cmd, err := r.db.Exec(ctx, query,
		c.Slug, c.Title, c.Description, c.CoverImageURL,
		c.IsActive, c.StartsAt, c.EndsAt, c.UpdatedAt, c.ID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("failed to update collection: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrCollectionNotFound
	}
	return nil
}

func (r *Repository) GetCollectionByID(ctx context.Context, id uuid.UUID) (*Collection, error) {
	query := `
		SELECT id, slug, title, description, cover_image_url, is_active, starts_at, ends_at, created_at, updated_at
		FROM collections WHERE id = $1
	`
	var c Collection
	err := r.db.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.Slug, &c.Title, &c.Description, &c.CoverImageURL,
		&c.IsActive, &c.StartsAt, &c.EndsAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCollectionNotFound
		}
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}
	return &c, nil
}

func (r *Repository) GetCollectionBySlug(ctx context.Context, slug string) (*Collection, error) {
	query := `
		SELECT id, slug, title, description, cover_image_url, is_active, starts_at, ends_at, created_at, updated_at
		FROM collections WHERE slug = $1
	`
	var c Collection
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&c.ID, &c.Slug, &c.Title, &c.Description, &c.CoverImageURL,
		&c.IsActive, &c.StartsAt, &c.EndsAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCollectionNotFound
		}
		return nil, fmt.Errorf("failed to get collection by slug: %w", err)
	}
	return &c, nil
}

func (r *Repository) ListCollectionsAdmin(ctx context.Context) ([]Collection, error) {
	query := `
		SELECT id, slug, title, description, cover_image_url, is_active, starts_at, ends_at, created_at, updated_at
		FROM collections ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	defer rows.Close()

	var res []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(
			&c.ID, &c.Slug, &c.Title, &c.Description, &c.CoverImageURL,
			&c.IsActive, &c.StartsAt, &c.EndsAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, nil
}

func (r *Repository) ListActiveCollectionsPublic(ctx context.Context, asOf time.Time) ([]Collection, error) {
	query := `
		SELECT id, slug, title, description, cover_image_url, is_active, starts_at, ends_at, created_at, updated_at
		FROM collections
		WHERE is_active = true
		  AND (starts_at IS NULL OR starts_at <= $1)
		  AND (ends_at IS NULL OR ends_at > $1)
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, asOf)
	if err != nil {
		return nil, fmt.Errorf("failed to list active collections: %w", err)
	}
	defer rows.Close()

	var res []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(
			&c.ID, &c.Slug, &c.Title, &c.Description, &c.CoverImageURL,
			&c.IsActive, &c.StartsAt, &c.EndsAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, nil
}

func (r *Repository) GetAdminCollectionItems(ctx context.Context, collectionID uuid.UUID) ([]AdminCollectionItem, error) {
	query := `
		SELECT ci.product_id, p.title, p.status, ci.sort_order
		FROM collection_items ci
		JOIN products p ON ci.product_id = p.id
		WHERE ci.collection_id = $1
		ORDER BY ci.sort_order ASC
	`
	rows, err := r.db.Query(ctx, query, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin collection items: %w", err)
	}
	defer rows.Close()

	var res []AdminCollectionItem
	for rows.Next() {
		var i AdminCollectionItem
		if err := rows.Scan(&i.ProductID, &i.Title, &i.Status, &i.SortOrder); err != nil {
			return nil, err
		}
		res = append(res, i)
	}
	return res, nil
}

func (r *Repository) ReplaceCollectionItems(ctx context.Context, collectionID uuid.UUID, productIDs []uuid.UUID) error {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, "SELECT id FROM collections WHERE id = $1 FOR UPDATE", collectionID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCollectionNotFound
		}
		return fmt.Errorf("failed to lock collection: %w", err)
	}

	if len(productIDs) > 0 {
		var count int
		err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE id = ANY($1)", productIDs).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to count products: %w", err)
		}
		if count != len(productIDs) {
			return ErrProductNotFound
		}
	}

	_, err = r.db.Exec(ctx, "DELETE FROM collection_items WHERE collection_id = $1", collectionID)
	if err != nil {
		return fmt.Errorf("failed to delete old items: %w", err)
	}

	now := time.Now()
	for i, pid := range productIDs {
		_, err := r.db.Exec(ctx, "INSERT INTO collection_items (collection_id, product_id, sort_order, created_at) VALUES ($1, $2, $3, $4)", collectionID, pid, i, now)
		if err != nil {
			return fmt.Errorf("failed to insert collection item at index %d: %w", i, err)
		}
	}
	return nil
}

func (r *Repository) GetPublicCollectionItems(ctx context.Context, collectionID uuid.UUID) ([]products.Product, error) {
	query := fmt.Sprintf(`
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		INNER JOIN collection_items ci ON ci.product_id = p.id
		WHERE ci.collection_id = $1
		  AND %s
		ORDER BY ci.sort_order ASC, p.id ASC
	`, products.CanonicalStorefrontEligibilitySQL("p", "s"))

	rows, err := r.db.Query(ctx, query, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get public collection items: %w", err)
	}
	defer rows.Close()

	var res []products.Product
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
		res = append(res, p)
	}

	return res, nil
}
