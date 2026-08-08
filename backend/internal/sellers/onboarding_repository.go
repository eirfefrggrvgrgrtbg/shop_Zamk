package sellers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateOnboardingApplication(ctx context.Context, app *SellerOnboardingApplication) error {
	payloadJSON, err := json.Marshal(app.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO seller_onboarding_applications (id, seller_id, status, current_step, payload, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = r.db.Exec(ctx, query,
		app.ID, app.SellerID, app.Status, app.CurrentStep, payloadJSON, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create onboarding application: %w", err)
	}
	return nil
}

func (r *Repository) GetOnboardingApplicationBySellerID(ctx context.Context, sellerID uuid.UUID) (*SellerOnboardingApplication, error) {
	query := `
		SELECT id, seller_id, status, current_step, payload, review_comment, submitted_at, reviewed_at, reviewed_by, created_at, updated_at
		FROM seller_onboarding_applications
		WHERE seller_id = $1
	`
	var app SellerOnboardingApplication
	var payloadBytes []byte

	err := r.db.QueryRow(ctx, query, sellerID).Scan(
		&app.ID, &app.SellerID, &app.Status, &app.CurrentStep, &payloadBytes, &app.ReviewComment, &app.SubmittedAt, &app.ReviewedAt, &app.ReviewedBy, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOnboardingNotFound
		}
		return nil, fmt.Errorf("failed to get onboarding application: %w", err)
	}

	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &app.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	}

	return &app, nil
}

func (r *Repository) GetOnboardingApplicationBySellerIDForUpdate(ctx context.Context, sellerID uuid.UUID) (*SellerOnboardingApplication, error) {
	query := `
		SELECT id, seller_id, status, current_step, payload, review_comment, submitted_at, reviewed_at, reviewed_by, created_at, updated_at
		FROM seller_onboarding_applications
		WHERE seller_id = $1
		FOR UPDATE
	`
	var app SellerOnboardingApplication
	var payloadBytes []byte

	err := r.db.QueryRow(ctx, query, sellerID).Scan(
		&app.ID, &app.SellerID, &app.Status, &app.CurrentStep, &payloadBytes, &app.ReviewComment, &app.SubmittedAt, &app.ReviewedAt, &app.ReviewedBy, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOnboardingNotFound
		}
		return nil, fmt.Errorf("failed to get onboarding application for update: %w", err)
	}

	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &app.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	}

	return &app, nil
}

func (r *Repository) GetOnboardingApplicationByID(ctx context.Context, id uuid.UUID) (*SellerOnboardingApplication, error) {
	query := `
		SELECT id, seller_id, status, current_step, payload, review_comment, submitted_at, reviewed_at, reviewed_by, created_at, updated_at
		FROM seller_onboarding_applications
		WHERE id = $1
	`
	var app SellerOnboardingApplication
	var payloadBytes []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&app.ID, &app.SellerID, &app.Status, &app.CurrentStep, &payloadBytes, &app.ReviewComment, &app.SubmittedAt, &app.ReviewedAt, &app.ReviewedBy, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOnboardingNotFound
		}
		return nil, fmt.Errorf("failed to get onboarding application by ID: %w", err)
	}

	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &app.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	}

	return &app, nil
}

func (r *Repository) GetOnboardingApplicationByIDForUpdate(ctx context.Context, id uuid.UUID) (*SellerOnboardingApplication, error) {
	query := `
		SELECT id, seller_id, status, current_step, payload, review_comment, submitted_at, reviewed_at, reviewed_by, created_at, updated_at
		FROM seller_onboarding_applications
		WHERE id = $1
		FOR UPDATE
	`
	var app SellerOnboardingApplication
	var payloadBytes []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&app.ID, &app.SellerID, &app.Status, &app.CurrentStep, &payloadBytes, &app.ReviewComment, &app.SubmittedAt, &app.ReviewedAt, &app.ReviewedBy, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOnboardingNotFound
		}
		return nil, fmt.Errorf("failed to get onboarding application by ID for update: %w", err)
	}

	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &app.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	}

	return &app, nil
}

func (r *Repository) ListOnboardingApplications(ctx context.Context, status string) ([]SellerOnboardingApplication, error) {
	query := `
		SELECT id, seller_id, status, current_step, payload, review_comment, submitted_at, reviewed_at, reviewed_by, created_at, updated_at
		FROM seller_onboarding_applications
	`
	var args []interface{}
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list onboarding applications: %w", err)
	}
	defer rows.Close()

	var apps []SellerOnboardingApplication
	for rows.Next() {
		var app SellerOnboardingApplication
		var payloadBytes []byte
		if err := rows.Scan(&app.ID, &app.SellerID, &app.Status, &app.CurrentStep, &payloadBytes, &app.ReviewComment, &app.SubmittedAt, &app.ReviewedAt, &app.ReviewedBy, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan onboarding application: %w", err)
		}
		if len(payloadBytes) > 0 {
			if err := json.Unmarshal(payloadBytes, &app.Payload); err != nil {
				return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
			}
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func (r *Repository) UpdateOnboardingApplication(ctx context.Context, app *SellerOnboardingApplication) error {
	payloadJSON, err := json.Marshal(app.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		UPDATE seller_onboarding_applications
		SET status = $1, current_step = $2, payload = $3, review_comment = $4, submitted_at = $5, reviewed_at = $6, reviewed_by = $7, updated_at = $8
		WHERE id = $9
	`
	_, err = r.db.Exec(ctx, query,
		app.Status, app.CurrentStep, payloadJSON, app.ReviewComment, app.SubmittedAt, app.ReviewedAt, app.ReviewedBy, app.UpdatedAt, app.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update onboarding application: %w", err)
	}
	return nil
}

func (r *Repository) CreateSellerBrand(ctx context.Context, sb *SellerBrand) error {
	query := `
		INSERT INTO seller_brands (id, seller_id, brand_id, is_primary, relationship_type, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		sb.ID, sb.SellerID, sb.BrandID, sb.IsPrimary, sb.RelationshipType, sb.Status, sb.CreatedAt, sb.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create seller brand: %w", err)
	}
	return nil
}

func (r *Repository) IsBrandSlugTaken(ctx context.Context, slug string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM brands WHERE slug = $1)`
	var exists bool
	if err := r.db.QueryRow(ctx, query, slug).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check brand slug: %w", err)
	}
	return exists, nil
}

func (r *Repository) CreateBrand(ctx context.Context, id uuid.UUID, name, slug, description string) error {
	query := `
		INSERT INTO brands (id, name, slug, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, $6)
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query, id, name, slug, description, now, now)
	if err != nil {
		return fmt.Errorf("failed to create brand: %w", err)
	}
	return nil
}

func (r *Repository) UpdateSeller(ctx context.Context, s *Seller) error {
	query := `
		UPDATE sellers
		SET brand_name = $1, slug = $2, description = $3, contact_email = $4, contact_phone = $5, status = $6, updated_at = $7
		WHERE id = $8
	`
	_, err := r.db.Exec(ctx, query,
		s.BrandName, s.Slug, s.Description, s.ContactEmail, s.ContactPhone, s.Status, time.Now(), s.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update seller: %w", err)
	}
	return nil
}
