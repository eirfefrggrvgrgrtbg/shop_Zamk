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

func (r *Repository) UpdateSellerProfile(ctx context.Context, sellerID uuid.UUID, req *UpdateSellerProfileRequest) error {
	query := `
		UPDATE sellers
		SET
			brand_name    = COALESCE($1, brand_name),
			description   = COALESCE($2, description),
			contact_email = COALESCE($3, contact_email),
			contact_phone = COALESCE($4, contact_phone),
			slug          = COALESCE($5, slug),
			updated_at    = now()
		WHERE id = $6
	`
	res, err := r.db.Exec(ctx, query,
		req.BrandName, req.Description, req.ContactEmail, req.ContactPhone, req.Slug,
		sellerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update seller profile: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSellerNotFound
	}
	return nil
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

// CountSellers returns total number of sellers.
func (r *Repository) CountSellers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM sellers`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sellers: %w", err)
	}
	return count, nil
}

// GetSellerDetailByID returns a full seller aggregate with owner info and counts.
func (r *Repository) GetSellerDetailByID(ctx context.Context, sellerID uuid.UUID) (*SellerDetail, error) {
	query := `
		SELECT
			s.id, s.brand_name, s.slug, s.description, s.contact_email, s.contact_phone, s.logo_url, s.status, s.created_at, s.updated_at,
			u.id, u.name, u.email, u.status,
			(SELECT COUNT(*) FROM seller_warnings sw WHERE sw.seller_id = s.id AND sw.status = 'active')  AS warnings_active,
			(SELECT COUNT(*) FROM seller_violations sv WHERE sv.seller_id = s.id AND sv.status = 'active') AS violations_active,
			(SELECT COUNT(*) FROM seller_violations sv WHERE sv.seller_id = s.id AND sv.status = 'active' AND sv.counts_for_penalty = TRUE) AS active_penalty_violations
		FROM sellers s
		JOIN seller_users su ON su.seller_id = s.id
		JOIN users u ON u.id = su.user_id
		WHERE s.id = $1
		LIMIT 1
	`
	var d SellerDetail
	err := r.db.QueryRow(ctx, query, sellerID).Scan(
		&d.ID, &d.BrandName, &d.Slug, &d.Description, &d.ContactEmail, &d.ContactPhone, &d.LogoURL, &d.Status, &d.CreatedAt, &d.UpdatedAt,
		&d.OwnerID, &d.OwnerName, &d.OwnerEmail, &d.OwnerStatus,
		&d.WarningsActive, &d.ViolationsActive, &d.ActivePenaltyViolations,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to get seller detail: %w", err)
	}
	return &d, nil
}

// WriteStatusHistory inserts a seller_status_history row.
func (r *Repository) WriteStatusHistory(ctx context.Context, sellerID uuid.UUID, oldStatus *string, newStatus string, reason *string, actorUserID *uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO seller_status_history (seller_id, old_status, new_status, reason, actor_user_id)
		VALUES ($1, $2, $3, $4, $5)
	`, sellerID, oldStatus, newStatus, reason, actorUserID)
	if err != nil {
		return fmt.Errorf("failed to write status history: %w", err)
	}
	return nil
}

// GetStatusHistory returns status history ordered by created_at desc.
func (r *Repository) GetStatusHistory(ctx context.Context, sellerID uuid.UUID) ([]SellerStatusHistoryItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, old_status, new_status, reason, actor_user_id, created_at
		FROM seller_status_history
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get status history: %w", err)
	}
	defer rows.Close()

	var items []SellerStatusHistoryItem
	for rows.Next() {
		var item SellerStatusHistoryItem
		if err := rows.Scan(&item.ID, &item.OldStatus, &item.NewStatus, &item.Reason, &item.ActorUserID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan status history: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateWarning inserts a seller_warnings row.
func (r *Repository) CreateWarning(ctx context.Context, w CreateWarningInput) (*WarningResponse, error) {
	var wr WarningResponse
	err := r.db.QueryRow(ctx, `
		INSERT INTO seller_warnings (seller_id, type, title, message, severity, status, actor_user_id)
		VALUES ($1, $2, $3, $4, $5, 'active', $6)
		RETURNING id, seller_id, type, title, message, severity, status, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
	`, w.SellerID, w.Type, w.Title, w.Message, w.Severity, w.ActorUserID).Scan(
		&wr.ID, &wr.SellerID, &wr.Type, &wr.Title, &wr.Message, &wr.Severity, &wr.Status,
		&wr.ActorUserID, &wr.CreatedAt, &wr.ResolvedAt, &wr.ResolvedBy, &wr.ResolutionNote,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create warning: %w", err)
	}
	return &wr, nil
}

// ListWarnings returns all warnings for a seller ordered by created_at desc.
func (r *Repository) ListWarnings(ctx context.Context, sellerID uuid.UUID) ([]WarningResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, seller_id, type, title, message, severity, status, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
		FROM seller_warnings
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list warnings: %w", err)
	}
	defer rows.Close()

	var items []WarningResponse
	for rows.Next() {
		var wr WarningResponse
		if err := rows.Scan(
			&wr.ID, &wr.SellerID, &wr.Type, &wr.Title, &wr.Message, &wr.Severity, &wr.Status,
			&wr.ActorUserID, &wr.CreatedAt, &wr.ResolvedAt, &wr.ResolvedBy, &wr.ResolutionNote,
		); err != nil {
			return nil, fmt.Errorf("failed to scan warning: %w", err)
		}
		items = append(items, wr)
	}
	return items, rows.Err()
}

// UpdateWarningStatus updates warning status and resolved fields.
func (r *Repository) UpdateWarningStatus(ctx context.Context, warningID uuid.UUID, status string, resolvedBy *uuid.UUID, note *string) error {
	res, err := r.db.Exec(ctx, `
		UPDATE seller_warnings
		SET status = $1, resolved_at = CASE WHEN $1 IN ('resolved','cancelled') THEN NOW() ELSE resolved_at END,
		    resolved_by = $2, resolution_note = COALESCE($3, resolution_note)
		WHERE id = $4
	`, status, resolvedBy, note, warningID)
	if err != nil {
		return fmt.Errorf("failed to update warning status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrWarningNotFound
	}
	return nil
}

// CreateViolation inserts a seller_violations row.
func (r *Repository) CreateViolation(ctx context.Context, v CreateViolationInput) (*ViolationResponse, error) {
	var vr ViolationResponse
	err := r.db.QueryRow(ctx, `
		INSERT INTO seller_violations (seller_id, type, title, description, severity, status, counts_for_penalty, actor_user_id)
		VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)
		RETURNING id, seller_id, type, title, description, severity, status, counts_for_penalty, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
	`, v.SellerID, v.Type, v.Title, v.Description, v.Severity, v.CountsForPenalty, v.ActorUserID).Scan(
		&vr.ID, &vr.SellerID, &vr.Type, &vr.Title, &vr.Description, &vr.Severity, &vr.Status,
		&vr.CountsForPenalty, &vr.ActorUserID, &vr.CreatedAt, &vr.ResolvedAt, &vr.ResolvedBy, &vr.ResolutionNote,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create violation: %w", err)
	}
	return &vr, nil
}

// ListViolations returns all violations for a seller ordered by created_at desc.
func (r *Repository) ListViolations(ctx context.Context, sellerID uuid.UUID) ([]ViolationResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, seller_id, type, title, description, severity, status, counts_for_penalty, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
		FROM seller_violations
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list violations: %w", err)
	}
	defer rows.Close()

	var items []ViolationResponse
	for rows.Next() {
		var vr ViolationResponse
		if err := rows.Scan(
			&vr.ID, &vr.SellerID, &vr.Type, &vr.Title, &vr.Description, &vr.Severity, &vr.Status,
			&vr.CountsForPenalty, &vr.ActorUserID, &vr.CreatedAt, &vr.ResolvedAt, &vr.ResolvedBy, &vr.ResolutionNote,
		); err != nil {
			return nil, fmt.Errorf("failed to scan violation: %w", err)
		}
		items = append(items, vr)
	}
	return items, rows.Err()
}

// UpdateViolationStatus updates violation status and resolved fields.
func (r *Repository) UpdateViolationStatus(ctx context.Context, violationID uuid.UUID, status string, resolvedBy *uuid.UUID, note *string) error {
	res, err := r.db.Exec(ctx, `
		UPDATE seller_violations
		SET status = $1, resolved_at = CASE WHEN $1 IN ('resolved','cancelled') THEN NOW() ELSE resolved_at END,
		    resolved_by = $2, resolution_note = COALESCE($3, resolution_note)
		WHERE id = $4
	`, status, resolvedBy, note, violationID)
	if err != nil {
		return fmt.Errorf("failed to update violation status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrViolationNotFound
	}
	return nil
}

// CountActivePenaltyViolations counts active violations where counts_for_penalty=true.
func (r *Repository) CountActivePenaltyViolations(ctx context.Context, sellerID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM seller_violations
		WHERE seller_id = $1 AND status = 'active' AND counts_for_penalty = TRUE
	`, sellerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count penalty violations: %w", err)
	}
	return count, nil
}
