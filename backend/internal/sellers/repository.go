package sellers

import (
	"context"
	"errors"
	"fmt"

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

// WithTx returns a new Repository bound to the provided transaction
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) CreateSeller(ctx context.Context, s *Seller) error {
	query := `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, logo_object_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		s.ID, s.BrandName, s.Slug, s.Description, s.ContactEmail, s.ContactPhone, s.Status, s.LogoURL, s.LogoObjectKey, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create seller: %w", err)
	}
	return nil
}

func (r *Repository) CreateSellerUser(ctx context.Context, su *SellerUser) error {
	query := `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query,
		su.ID, su.SellerID, su.UserID, su.Role, su.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create seller user: %w", err)
	}
	return nil
}

func (r *Repository) GetSellerByID(ctx context.Context, id uuid.UUID) (*Seller, error) {
	query := `
		SELECT id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, logo_object_key, created_at, updated_at
		FROM sellers
		WHERE id = $1
	`
	var s Seller
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to get seller by id: %w", err)
	}
	return &s, nil
}

func (r *Repository) GetSellerBySlug(ctx context.Context, slug string) (*Seller, error) {
	query := `
		SELECT id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, logo_object_key, created_at, updated_at
		FROM sellers
		WHERE slug = $1
	`
	var s Seller
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to get seller by slug: %w", err)
	}
	return &s, nil
}

func (r *Repository) GetSellerByUserID(ctx context.Context, userID uuid.UUID) (*Seller, *SellerUser, error) {
	query := `
		SELECT s.id, s.brand_name, s.slug, s.description, s.contact_email, s.contact_phone, s.status, s.logo_url, s.logo_object_key, s.created_at, s.updated_at,
		       su.id, su.seller_id, su.user_id, su.role, su.created_at
		FROM sellers s
		JOIN seller_users su ON s.id = su.seller_id
		WHERE su.user_id = $1
	`
	var s Seller
	var su SellerUser
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.CreatedAt, &s.UpdatedAt,
		&su.ID, &su.SellerID, &su.UserID, &su.Role, &su.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrSellerUserNotFound
		}
		return nil, nil, fmt.Errorf("failed to get seller by user id: %w", err)
	}
	return &s, &su, nil
}

func (r *Repository) UpdateSellerStatus(ctx context.Context, id uuid.UUID, status SellerStatus) error {
	query := `
		UPDATE sellers
		SET status = $1, updated_at = now()
		WHERE id = $2
	`
	res, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update seller status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSellerNotFound
	}
	return nil
}

func (r *Repository) ListSellers(ctx context.Context, limit, offset int) ([]Seller, error) {
	query := `
		SELECT id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, logo_object_key, created_at, updated_at
		FROM sellers
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sellers: %w", err)
	}
	defer rows.Close()

	var items []Seller
	for rows.Next() {
		var s Seller
		if err := rows.Scan(
			&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan seller: %w", err)
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateSellerLogo(ctx context.Context, sellerID uuid.UUID, logoURL string, logoObjectKey string) error {
	query := `
		UPDATE sellers
		SET logo_url = $1, logo_object_key = $2, updated_at = now()
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, logoURL, logoObjectKey, sellerID)
	if err != nil {
		return fmt.Errorf("failed to update seller logo: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSellerNotFound
	}
	return nil
}
