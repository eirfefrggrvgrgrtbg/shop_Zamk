package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
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

func (r *Repository) CreateCategory(ctx context.Context, c *Category) error {
	query := `
		INSERT INTO categories (id, parent_id, name, slug, description, sort_order, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		c.ID, c.ParentID, c.Name, c.Slug, c.Description, c.SortOrder, c.IsActive, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}
	return nil
}

func (r *Repository) GetCategoryBySlug(ctx context.Context, slug string) (*Category, error) {
	query := `
		SELECT id, parent_id, name, slug, description, sort_order, is_active, created_at, updated_at
		FROM categories
		WHERE slug = $1
	`
	var c Category
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.Description, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return &c, nil
}

func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	query := `
		SELECT id, parent_id, name, slug, description, sort_order, is_active, created_at, updated_at
		FROM categories
		ORDER BY sort_order ASC, name ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.Description, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (r *Repository) CreateBrand(ctx context.Context, b *Brand) error {
	query := `
		INSERT INTO brands (id, name, slug, description, logo_url, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		b.ID, b.Name, b.Slug, b.Description, b.LogoURL, b.IsActive, b.CreatedAt, b.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create brand: %w", err)
	}
	return nil
}

func (r *Repository) GetBrandBySlug(ctx context.Context, slug string) (*Brand, error) {
	query := `
		SELECT id, name, slug, description, logo_url, is_active, created_at, updated_at
		FROM brands
		WHERE slug = $1
	`
	var b Brand
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&b.ID, &b.Name, &b.Slug, &b.Description, &b.LogoURL, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}
		return nil, fmt.Errorf("failed to get brand: %w", err)
	}
	return &b, nil
}

func (r *Repository) ListBrands(ctx context.Context) ([]Brand, error) {
	query := `
		SELECT id, name, slug, description, logo_url, is_active, created_at, updated_at
		FROM brands
		ORDER BY name ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list brands: %w", err)
	}
	defer rows.Close()

	var brands []Brand
	for rows.Next() {
		var b Brand
		if err := rows.Scan(&b.ID, &b.Name, &b.Slug, &b.Description, &b.LogoURL, &b.IsActive, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		brands = append(brands, b)
	}
	return brands, rows.Err()
}
