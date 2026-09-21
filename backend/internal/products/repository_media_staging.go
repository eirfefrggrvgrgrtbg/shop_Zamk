package products

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// INVARIANT & DEPENDENCY NOTE:
// Because product_media_staging uses ON DELETE CASCADE,
// the stage endpoint MUST NOT be exposed in production before product/seller
// hard-delete paths durably preserve cleanup intent for staged S3 objects.

type ProductMediaStagingStatus string

const (
	ProductMediaStagingUploading ProductMediaStagingStatus = "uploading"
	ProductMediaStagingReady     ProductMediaStagingStatus = "ready"
	ProductMediaStagingConsumed  ProductMediaStagingStatus = "consumed"
)

var (
	ErrStagedMediaNotFound = errors.New("staged media not found")
)

type ProductMediaStaging struct {
	ID            uuid.UUID
	SellerID      uuid.UUID
	ProductID     uuid.UUID
	ClientMediaID uuid.UUID
	Status        ProductMediaStagingStatus
	ObjectKey     string
	ImageURL      string
	ContentSHA256 string
	ByteSize      int64
	Width         int
	Height        int
	CreatedAt     time.Time
	ConsumedAt    *time.Time
}

type ProductMediaCleanupJob struct {
	ID            uuid.UUID
	ObjectKey     string
	CreatedAt     time.Time
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
}

// CreateOrGetStagedMedia inserts an 'uploading' staged media row while enforcing that
// product.seller_id == s.SellerID.
// If a row with the same (seller_id, product_id, client_media_id) already exists,
// it is returned instead (idempotency), BUT it must match content_sha256 or it returns an error.
// If the product does not exist, ErrProductNotFound is returned.
// If the product belongs to another seller, ErrUnauthorized is returned.
func (r *Repository) CreateOrGetStagedMedia(ctx context.Context, s *ProductMediaStaging) (*ProductMediaStaging, error) {
	query := `
		INSERT INTO product_media_staging (
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
		)
		SELECT
			$1, p.seller_id, p.id, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13
		FROM products p
		WHERE p.id = $3 AND p.seller_id = $2
		ON CONFLICT (seller_id, product_id, client_media_id) DO NOTHING
		RETURNING
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
	`

	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	if s.Status == "" {
		s.Status = ProductMediaStagingUploading
	}

	var row ProductMediaStaging
	err := r.db.QueryRow(ctx, query,
		s.ID, s.SellerID, s.ProductID, s.ClientMediaID, s.Status,
		s.ObjectKey, s.ImageURL, s.ContentSHA256, s.ByteSize, s.Width, s.Height,
		s.CreatedAt, s.ConsumedAt,
	).Scan(
		&row.ID, &row.SellerID, &row.ProductID, &row.ClientMediaID, &row.Status,
		&row.ObjectKey, &row.ImageURL, &row.ContentSHA256, &row.ByteSize, &row.Width, &row.Height,
		&row.CreatedAt, &row.ConsumedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		// Either the row already exists (ON CONFLICT DO NOTHING),
		// or the SELECT produced 0 rows (product not found or seller mismatch).

		// 1. Check if the staged media row already exists for this (seller_id, product_id, client_media_id)
		fetchQuery := `
			SELECT
				id, seller_id, product_id, client_media_id, status,
				object_key, image_url, content_sha256, byte_size, width, height,
				created_at, consumed_at
			FROM product_media_staging
			WHERE seller_id = $1 AND product_id = $2 AND client_media_id = $3
		`
		fetchErr := r.db.QueryRow(ctx, fetchQuery, s.SellerID, s.ProductID, s.ClientMediaID).Scan(
			&row.ID, &row.SellerID, &row.ProductID, &row.ClientMediaID, &row.Status,
			&row.ObjectKey, &row.ImageURL, &row.ContentSHA256, &row.ByteSize, &row.Width, &row.Height,
			&row.CreatedAt, &row.ConsumedAt,
		)
		if fetchErr == nil {
			if row.ContentSHA256 != s.ContentSHA256 {
				return nil, fmt.Errorf("idempotency conflict: existing staged media has different content_sha256")
			}
			return &row, nil
		}
		if !errors.Is(fetchErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("failed to fetch existing staged media: %w", fetchErr)
		}

		// 2. The staging row does not exist. This means the INSERT produced 0 rows because
		// the product either does not exist or does not belong to the seller.
		var actualSellerID uuid.UUID
		checkErr := r.db.QueryRow(ctx, "SELECT seller_id FROM products WHERE id = $1", s.ProductID).Scan(&actualSellerID)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		} else if checkErr != nil {
			return nil, fmt.Errorf("failed to check product ownership: %w", checkErr)
		}

		if actualSellerID != s.SellerID {
			return nil, ErrUnauthorized
		}

		return nil, ErrProductNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to insert staged media: %w", err)
	}

	return &row, nil
}

// MarkStagedMediaReadyForSellerProduct updates status to 'ready' scoped strictly to seller and product.
func (r *Repository) MarkStagedMediaReadyForSellerProduct(ctx context.Context, id, sellerID, productID uuid.UUID) error {
	query := `UPDATE product_media_staging SET status = $1 WHERE id = $2 AND seller_id = $3 AND product_id = $4 AND status = $5`
	res, err := r.db.Exec(ctx, query, ProductMediaStagingReady, id, sellerID, productID, ProductMediaStagingUploading)
	if err != nil {
		return fmt.Errorf("failed to mark staged media ready: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("staged media not found or not in uploading state")
	}
	return nil
}

// MarkStagedMediaConsumedForSellerProduct updates status to 'consumed' and sets consumed_at, scoped strictly to seller and product.
func (r *Repository) MarkStagedMediaConsumedForSellerProduct(ctx context.Context, id, sellerID, productID uuid.UUID) error {
	query := `UPDATE product_media_staging SET status = $1, consumed_at = NOW() WHERE id = $2 AND seller_id = $3 AND product_id = $4 AND status = $5`
	res, err := r.db.Exec(ctx, query, ProductMediaStagingConsumed, id, sellerID, productID, ProductMediaStagingReady)
	if err != nil {
		return fmt.Errorf("failed to mark staged media consumed: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("staged media not found or not in ready state")
	}
	return nil
}

// GetStagedMediaByIDForSellerProduct retrieves a staged media item ensuring exact seller and product ownership.
func (r *Repository) GetStagedMediaByIDForSellerProduct(ctx context.Context, stagedMediaID, sellerID, productID uuid.UUID) (*ProductMediaStaging, error) {
	query := `
		SELECT
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
		FROM product_media_staging
		WHERE id = $1 AND seller_id = $2 AND product_id = $3
	`
	var row ProductMediaStaging
	err := r.db.QueryRow(ctx, query, stagedMediaID, sellerID, productID).Scan(
		&row.ID, &row.SellerID, &row.ProductID, &row.ClientMediaID, &row.Status,
		&row.ObjectKey, &row.ImageURL, &row.ContentSHA256, &row.ByteSize, &row.Width, &row.Height,
		&row.CreatedAt, &row.ConsumedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStagedMediaNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to get staged media: %w", err)
	}
	return &row, nil
}

// GetStagedMediaForUpdateForSellerProduct locks a staged media item using SELECT ... FOR UPDATE,
// ensuring exact seller and product ownership.
func (r *Repository) GetStagedMediaForUpdateForSellerProduct(ctx context.Context, stagedMediaID, sellerID, productID uuid.UUID) (*ProductMediaStaging, error) {
	query := `
		SELECT
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
		FROM product_media_staging
		WHERE id = $1 AND seller_id = $2 AND product_id = $3
		FOR UPDATE
	`
	var row ProductMediaStaging
	err := r.db.QueryRow(ctx, query, stagedMediaID, sellerID, productID).Scan(
		&row.ID, &row.SellerID, &row.ProductID, &row.ClientMediaID, &row.Status,
		&row.ObjectKey, &row.ImageURL, &row.ContentSHA256, &row.ByteSize, &row.Width, &row.Height,
		&row.CreatedAt, &row.ConsumedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStagedMediaNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to get staged media for update: %w", err)
	}
	return &row, nil
}

func (r *Repository) EnqueueMediaCleanup(ctx context.Context, objectKey string) error {
	query := `
		INSERT INTO product_media_cleanup_jobs (object_key, next_attempt_at)
		VALUES ($1, NOW())
		ON CONFLICT (object_key) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, objectKey)
	if err != nil {
		return fmt.Errorf("failed to enqueue media cleanup: %w", err)
	}
	return nil
}
