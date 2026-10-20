package catalog

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (r *Repository) ListCategories(ctx context.Context, activeOnly bool) ([]Category, error) {
	query := `
		SELECT id, parent_id, name, slug, description, sort_order, is_active, created_at, updated_at
		FROM categories
		WHERE ($1::boolean = false OR is_active = true)
		ORDER BY sort_order ASC, name ASC
	`
	rows, err := r.db.Query(ctx, query, activeOnly)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := r.attachSizeSystems(ctx, categories); err != nil {
		return nil, err
	}
	return categories, nil
}

// attachSizeSystems adds each category's default size system (single query, no N+1).
func (r *Repository) attachSizeSystems(ctx context.Context, categories []Category) error {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (css.category_id, sv.id)
			css.category_id, ss.code, ss.name, sv.value, sv.sort_order, css.is_default, css.sort_order
		FROM category_size_systems css
		JOIN size_systems ss ON ss.id = css.size_system_id AND ss.is_active = true
		JOIN size_values sv ON sv.size_system_id = ss.id AND sv.is_active = true
		ORDER BY css.category_id, sv.id
	`)
	if err != nil {
		return fmt.Errorf("failed to load category size systems: %w", err)
	}
	defer rows.Close()

	type entry struct {
		code, name, value    string
		valueOrder, sysOrder int
		isDefault            bool
	}
	byCat := map[uuid.UUID][]entry{}
	for rows.Next() {
		var catID uuid.UUID
		var e entry
		if err := rows.Scan(&catID, &e.code, &e.name, &e.value, &e.valueOrder, &e.isDefault, &e.sysOrder); err != nil {
			return err
		}
		byCat[catID] = append(byCat[catID], e)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range categories {
		entries := byCat[categories[i].ID]
		if len(entries) == 0 {
			continue
		}
		// pick one system: default first, then lowest sort order, then code
		best := entries[0]
		for _, e := range entries[1:] {
			if (e.isDefault && !best.isDefault) ||
				(e.isDefault == best.isDefault && (e.sysOrder < best.sysOrder || (e.sysOrder == best.sysOrder && e.code < best.code))) {
				best = e
			}
		}
		sys := &CategorySizeSystem{Code: best.code, Name: best.name, Values: []CategorySizeValue{}}
		for _, e := range entries {
			if e.code == best.code {
				sys.Values = append(sys.Values, CategorySizeValue{Value: e.value, SortOrder: e.valueOrder})
			}
		}
		sort.Slice(sys.Values, func(a, b int) bool {
			if sys.Values[a].SortOrder != sys.Values[b].SortOrder {
				return sys.Values[a].SortOrder < sys.Values[b].SortOrder
			}
			return sys.Values[a].Value < sys.Values[b].Value
		})
		categories[i].SizeSystem = sys
	}
	return nil
}

func (r *Repository) CreateBrand(ctx context.Context, b *Brand) error {
	query := `
		INSERT INTO brands (id, name, slug, description, logo_url, logo_object_key, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		b.ID, b.Name, b.Slug, b.Description, b.LogoURL, b.LogoObjectKey, b.IsActive, b.CreatedAt, b.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create brand: %w", err)
	}
	return nil
}

func (r *Repository) GetBrandBySlug(ctx context.Context, slug string) (*Brand, error) {
	query := `
		SELECT id, name, slug, description, logo_url, logo_object_key, is_active, created_at, updated_at
		FROM brands
		WHERE slug = $1
	`
	var b Brand
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&b.ID, &b.Name, &b.Slug, &b.Description, &b.LogoURL, &b.LogoObjectKey, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}
		return nil, fmt.Errorf("failed to get brand: %w", err)
	}
	return &b, nil
}

func (r *Repository) GetBrandByID(ctx context.Context, id uuid.UUID) (*Brand, error) {
	query := `
		SELECT id, name, slug, description, logo_url, logo_object_key, is_active, created_at, updated_at
		FROM brands
		WHERE id = $1
	`
	var b Brand
	err := r.db.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.Name, &b.Slug, &b.Description, &b.LogoURL, &b.LogoObjectKey, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}
		return nil, fmt.Errorf("failed to get brand by id: %w", err)
	}
	return &b, nil
}

func (r *Repository) ListBrands(ctx context.Context) ([]Brand, error) {
	query := `
		SELECT id, name, slug, description, logo_url, logo_object_key, is_active, created_at, updated_at
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
		if err := rows.Scan(&b.ID, &b.Name, &b.Slug, &b.Description, &b.LogoURL, &b.LogoObjectKey, &b.IsActive, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		brands = append(brands, b)
	}
	return brands, rows.Err()
}

func (r *Repository) UpdateBrandLogo(ctx context.Context, brandID uuid.UUID, logoURL string, logoObjectKey string) error {
	query := `
		UPDATE brands
		SET logo_url = $1, logo_object_key = $2, updated_at = now()
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, logoURL, logoObjectKey, brandID)
	if err != nil {
		return fmt.Errorf("failed to update brand logo: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrBrandNotFound
	}
	return nil
}
