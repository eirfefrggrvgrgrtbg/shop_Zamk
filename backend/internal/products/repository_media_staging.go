package products

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
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
	ErrStagedMediaNotFound        = errors.New("staged media not found")
	ErrStagedMediaConflict        = errors.New("idempotency conflict: existing staged media has different content_sha256")
	ErrStagedMediaQuotaExceeded   = errors.New("active staged media quota exceeded for product")
	ErrStagedMediaAlreadyConsumed = errors.New("staged media is already consumed")
)

const MaxActiveStagedMediaPerProduct = 16

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
	Generation    int64
	LeaseToken    *uuid.UUID
	LeaseUntil    *time.Time
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
		  AND (
		      SELECT COUNT(*)
		      FROM product_media_staging s2
		      WHERE s2.seller_id = $2 AND s2.product_id = $3 AND s2.status IN ($14, $15)
		  ) < $16
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
		ProductMediaStagingUploading, ProductMediaStagingReady, MaxActiveStagedMediaPerProduct,
	).Scan(
		&row.ID, &row.SellerID, &row.ProductID, &row.ClientMediaID, &row.Status,
		&row.ObjectKey, &row.ImageURL, &row.ContentSHA256, &row.ByteSize, &row.Width, &row.Height,
		&row.CreatedAt, &row.ConsumedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		// Either the row already exists (ON CONFLICT DO NOTHING),
		// or the SELECT produced 0 rows (product not found, seller mismatch, or quota reached).

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
				return nil, ErrStagedMediaConflict
			}
			return &row, nil
		}
		if !errors.Is(fetchErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("failed to fetch existing staged media: %w", fetchErr)
		}

		// 2. The staging row does not exist. Check product existence and ownership.
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

		// 3. Check active staging quota (uploading + ready)
		activeCount, countErr := r.CountActiveStagedMediaForSellerProduct(ctx, s.SellerID, s.ProductID)
		if countErr == nil && activeCount >= MaxActiveStagedMediaPerProduct {
			return nil, ErrStagedMediaQuotaExceeded
		}

		return nil, ErrProductNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to insert staged media: %w", err)
	}

	return &row, nil
}

// CountActiveStagedMediaForSellerProduct counts non-consumed ('uploading' or 'ready') staged media items.
func (r *Repository) CountActiveStagedMediaForSellerProduct(ctx context.Context, sellerID, productID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM product_media_staging
		WHERE seller_id = $1 AND product_id = $2 AND status IN ($3, $4)
	`
	var count int
	err := r.db.QueryRow(ctx, query, sellerID, productID, ProductMediaStagingUploading, ProductMediaStagingReady).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active staged media: %w", err)
	}
	return count, nil
}

// ClaimStagedMediaSlotForSellerProduct atomically locks the product row, validates editability,
// checks existing staging idempotency, enforces active staging quota, and inserts a new 'uploading'
// staging row. It must be called within an active transaction (e.g. via repo.WithTx(tx)).
func (r *Repository) ClaimStagedMediaSlotForSellerProduct(
	ctx context.Context,
	s *ProductMediaStaging,
	sellerStatus sellers.SellerStatus,
) (*ProductMediaStaging, error) {
	// 1. Lock exact owned Product row:
	// SELECT id, status FROM products WHERE id = $productID AND seller_id = $sellerID FOR UPDATE
	var prodID uuid.UUID
	var prodStatus string
	err := r.db.QueryRow(ctx, `
		SELECT id, status
		FROM products
		WHERE id = $1 AND seller_id = $2
		FOR UPDATE
	`, s.ProductID, s.SellerID).Scan(&prodID, &prodStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to lock product for staging claim: %w", err)
	}

	// 2. Revalidate current Product editability under this lock:
	if !CanEditProduct(sellerStatus, prodStatus) {
		return nil, ErrProductNotEditable
	}

	// 3. Resolve existing staging row by: seller_id, product_id, client_media_id
	// IMPORTANT: existing identity lookup happens BEFORE quota rejection.
	fetchQuery := `
		SELECT
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
		FROM product_media_staging
		WHERE seller_id = $1 AND product_id = $2 AND client_media_id = $3
	`
	var existingRow ProductMediaStaging
	fetchErr := r.db.QueryRow(ctx, fetchQuery, s.SellerID, s.ProductID, s.ClientMediaID).Scan(
		&existingRow.ID, &existingRow.SellerID, &existingRow.ProductID, &existingRow.ClientMediaID, &existingRow.Status,
		&existingRow.ObjectKey, &existingRow.ImageURL, &existingRow.ContentSHA256, &existingRow.ByteSize, &existingRow.Width, &existingRow.Height,
		&existingRow.CreatedAt, &existingRow.ConsumedAt,
	)
	if fetchErr == nil {
		if existingRow.ContentSHA256 != s.ContentSHA256 {
			return nil, ErrStagedMediaConflict
		}
		return &existingRow, nil
	} else if !errors.Is(fetchErr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to fetch existing staged media: %w", fetchErr)
	}

	// 4. If no existing row: COUNT active staging rows where status IN ('uploading','ready')
	var activeCount int
	countErr := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM product_media_staging
		WHERE seller_id = $1 AND product_id = $2 AND status IN ($3, $4)
	`, s.SellerID, s.ProductID, ProductMediaStagingUploading, ProductMediaStagingReady).Scan(&activeCount)
	if countErr != nil {
		return nil, fmt.Errorf("failed to count active staged media: %w", countErr)
	}

	// 5. If count >= MaxActiveStagedMediaPerProduct: return ErrStagedMediaQuotaExceeded
	if activeCount >= MaxActiveStagedMediaPerProduct {
		return nil, ErrStagedMediaQuotaExceeded
	}

	// 6. Otherwise insert new 'uploading' row
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	s.Status = ProductMediaStagingUploading

	insertQuery := `
		INSERT INTO product_media_staging (
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, NULL
		)
		RETURNING
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
	`
	var insertedRow ProductMediaStaging
	err = r.db.QueryRow(
		ctx, insertQuery,
		s.ID, s.SellerID, s.ProductID, s.ClientMediaID, s.Status,
		s.ObjectKey, s.ImageURL, s.ContentSHA256, s.ByteSize, s.Width, s.Height,
		s.CreatedAt,
	).Scan(
		&insertedRow.ID, &insertedRow.SellerID, &insertedRow.ProductID, &insertedRow.ClientMediaID, &insertedRow.Status,
		&insertedRow.ObjectKey, &insertedRow.ImageURL, &insertedRow.ContentSHA256, &insertedRow.ByteSize, &insertedRow.Width, &insertedRow.Height,
		&insertedRow.CreatedAt, &insertedRow.ConsumedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert staged media: %w", err)
	}

	return &insertedRow, nil
}

// MarkStagedMediaReadyForSellerProduct updates status to 'ready' scoped strictly to seller and product.
// If the row was already marked 'ready' (e.g. by a concurrent idempotent upload), it treats it as success.
// If the row definitively does not exist, it returns ErrStagedMediaNotFound.
func (r *Repository) MarkStagedMediaReadyForSellerProduct(ctx context.Context, id, sellerID, productID uuid.UUID) error {
	query := `UPDATE product_media_staging SET status = $1 WHERE id = $2 AND seller_id = $3 AND product_id = $4 AND status = $5`
	res, err := r.db.Exec(ctx, query, ProductMediaStagingReady, id, sellerID, productID, ProductMediaStagingUploading)
	if err != nil {
		return fmt.Errorf("failed to mark staged media ready: %w", err)
	}
	if res.RowsAffected() == 0 {
		var actualSellerID, actualProductID uuid.UUID
		var currentStatus ProductMediaStagingStatus
		checkErr := r.db.QueryRow(ctx, "SELECT seller_id, product_id, status FROM product_media_staging WHERE id = $1", id).Scan(&actualSellerID, &actualProductID, &currentStatus)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return ErrStagedMediaNotFound
		}
		if checkErr != nil {
			return fmt.Errorf("failed to verify staged media status: %w", checkErr)
		}
		if actualSellerID != sellerID || actualProductID != productID {
			return ErrUnauthorized
		}
		if currentStatus == ProductMediaStagingReady {
			return nil
		}
		if currentStatus == ProductMediaStagingConsumed {
			return ErrStagedMediaAlreadyConsumed
		}
		return fmt.Errorf("unexpected staged media status: %s", currentStatus)
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
		INSERT INTO product_media_cleanup_jobs (object_key, next_attempt_at, generation)
		VALUES ($1, NOW(), 1)
		ON CONFLICT (object_key) DO UPDATE SET
			generation = product_media_cleanup_jobs.generation + 1,
			next_attempt_at = NOW()
	`
	_, err := r.db.Exec(ctx, query, objectKey)
	if err != nil {
		return fmt.Errorf("failed to enqueue media cleanup: %w", err)
	}
	return nil
}

type CleanupFinalizeResult string

const (
	CleanupFinalizeDeleted CleanupFinalizeResult = "deleted"
	CleanupFinalizeSkipped CleanupFinalizeResult = "skipped_generation_changed"
	CleanupFinalizeLost    CleanupFinalizeResult = "lost_lease"
)

func (r *Repository) ClaimMediaCleanupJob(ctx context.Context, leaseDuration time.Duration) (*ProductMediaCleanupJob, error) {
	query := `
		UPDATE product_media_cleanup_jobs
		SET
			lease_token = gen_random_uuid(),
			lease_until = NOW() + make_interval(secs := $1)
		WHERE id = (
			SELECT id
			FROM product_media_cleanup_jobs
			WHERE next_attempt_at <= NOW()
			  AND (lease_until IS NULL OR lease_until < NOW())
			ORDER BY next_attempt_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, object_key, created_at, attempts, next_attempt_at, last_error, generation, lease_token, lease_until
	`
	var job ProductMediaCleanupJob
	err := r.db.QueryRow(ctx, query, leaseDuration.Seconds()).Scan(
		&job.ID, &job.ObjectKey, &job.CreatedAt, &job.Attempts, &job.NextAttemptAt,
		&job.LastError, &job.Generation, &job.LeaseToken, &job.LeaseUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no jobs available
	}
	if err != nil {
		return nil, fmt.Errorf("failed to claim media cleanup job: %w", err)
	}
	return &job, nil
}

func (r *Repository) ExpireStaleStagedMedia(ctx context.Context, uploadingStale time.Duration, readyStale time.Duration, consumedStale time.Duration, limit int) (int, error) {
	query := `
		WITH stale_batch AS (
			SELECT id, status, object_key
			FROM product_media_staging
			WHERE (status = 'uploading' AND created_at < NOW() - make_interval(secs := $1))
			   OR (status = 'ready' AND created_at < NOW() - make_interval(secs := $2))
			   OR (status = 'consumed' AND consumed_at < NOW() - make_interval(secs := $3))
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		),
		deleted AS (
			DELETE FROM product_media_staging
			WHERE id IN (SELECT id FROM stale_batch)
			RETURNING id, status, object_key
		),
		enqueue AS (
			INSERT INTO product_media_cleanup_jobs (object_key, next_attempt_at, generation)
			SELECT object_key, NOW(), 1
			FROM deleted
			WHERE status IN ('uploading', 'ready')
			ON CONFLICT (object_key) DO UPDATE SET
				generation = product_media_cleanup_jobs.generation + 1,
				next_attempt_at = NOW()
		)
		SELECT count(*) FROM deleted;
	`
	var count int
	err := r.db.QueryRow(ctx, query, uploadingStale.Seconds(), readyStale.Seconds(), consumedStale.Seconds(), limit).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to expire stale staged media: %w", err)
	}
	return count, nil
}

func (r *Repository) FinalizeMediaCleanupSuccess(ctx context.Context, id uuid.UUID, generation int64, leaseToken uuid.UUID) (CleanupFinalizeResult, error) {
	query := `
		WITH locked AS (
			SELECT id, generation, lease_token
			FROM product_media_cleanup_jobs
			WHERE id = $1 FOR UPDATE
		),
		deleted AS (
			DELETE FROM product_media_cleanup_jobs
			WHERE id = $1
			  AND (SELECT generation FROM locked) = $2
			  AND (SELECT lease_token FROM locked) = $3
			RETURNING id
		),
		updated AS (
			UPDATE product_media_cleanup_jobs
			SET lease_token = NULL, lease_until = NULL, next_attempt_at = LEAST(next_attempt_at, NOW())
			WHERE id = $1
			  AND (SELECT generation FROM locked) != $2
			  AND (SELECT lease_token FROM locked) = $3
			RETURNING id
		)
		SELECT
			CASE
				WHEN (SELECT id FROM locked) IS NULL THEN 'lost_lease'
				WHEN (SELECT lease_token FROM locked) IS NULL OR (SELECT lease_token FROM locked) != $3 THEN 'lost_lease'
				WHEN (SELECT id FROM deleted) IS NOT NULL THEN 'deleted'
				WHEN (SELECT id FROM updated) IS NOT NULL THEN 'skipped_generation_changed'
				ELSE 'lost_lease'
			END
	`
	var res string
	err := r.db.QueryRow(ctx, query, id, generation, leaseToken).Scan(&res)
	if err != nil {
		return "", fmt.Errorf("failed to finalize media cleanup success: %w", err)
	}
	return CleanupFinalizeResult(res), nil
}

func (r *Repository) FinalizeMediaCleanupFailure(ctx context.Context, id uuid.UUID, generation int64, leaseToken uuid.UUID, errMessage string, backoff time.Duration) error {
	query := `
		WITH locked AS (
			SELECT id, generation, lease_token
			FROM product_media_cleanup_jobs
			WHERE id = $1 FOR UPDATE
		),
		updated_same_gen AS (
			UPDATE product_media_cleanup_jobs
			SET attempts = attempts + 1,
			    last_error = $4,
			    next_attempt_at = NOW() + make_interval(secs := $5),
			    lease_token = NULL,
			    lease_until = NULL
			WHERE id = $1
			  AND (SELECT generation FROM locked) = $2
			  AND (SELECT lease_token FROM locked) = $3
			RETURNING id
		),
		updated_diff_gen AS (
			UPDATE product_media_cleanup_jobs
			SET lease_token = NULL,
			    lease_until = NULL,
			    next_attempt_at = LEAST(next_attempt_at, NOW())
			WHERE id = $1
			  AND (SELECT generation FROM locked) != $2
			  AND (SELECT lease_token FROM locked) = $3
			RETURNING id
		)
		SELECT 1
	`
	_, err := r.db.Exec(ctx, query, id, generation, leaseToken, errMessage, backoff.Seconds())
	if err != nil {
		return fmt.Errorf("failed to finalize media cleanup failure: %w", err)
	}
	return nil
}
