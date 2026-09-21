package products

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DesiredProductImage represents a validated desired media item requested by a seller.
type DesiredProductImage struct {
	ID        uuid.UUID
	AltText   *string
	ColorID   *uuid.UUID
	IsMain    bool
	SortOrder int
}

type imageClassification int

const (
	classCanonical imageClassification = iota
	classPromote
)

type classifiedImage struct {
	desired   DesiredProductImage
	class     imageClassification
	staging   *ProductMediaStaging
	canonical *ProductImage
}

// ReconcileProductImagesTx reconciles desired product images against existing canonical images
// and staged media inside the caller's active PostgreSQL transaction.
//
// Invariants & Lock Order:
//  1. Locks exact owned Product row:
//     SELECT id FROM products WHERE id=$productID AND seller_id=$sellerID FOR UPDATE
//  2. Locks all existing canonical images:
//     SELECT ... FROM product_images WHERE product_id=$productID ORDER BY id FOR UPDATE
//  3. Locks requested same-scope staging rows:
//     SELECT ... FROM product_media_staging WHERE seller_id=$sellerID AND product_id=$productID AND id=ANY(...) ORDER BY id FOR UPDATE
//
// Foreign/nonexistent media IDs are rejected without distinguishing ownership details.
// Staged rows in status='consumed' without a canonical counterpart return ErrMediaIntegrityViolation.
func (r *Repository) ReconcileProductImagesTx(
	ctx context.Context,
	sellerID uuid.UUID,
	productID uuid.UUID,
	desired []DesiredProductImage,
) error {
	// ---------------------------------------------------------
	// 1. Structural Validation of Desired Set
	// ---------------------------------------------------------
	if len(desired) > 8 {
		return fmt.Errorf("%w: maximum 8 images allowed, got %d", ErrInvalidMediaSet, len(desired))
	}

	seenIDs := make(map[uuid.UUID]bool, len(desired))
	desiredIDs := make([]uuid.UUID, len(desired))
	mainCount := 0

	for i, d := range desired {
		if d.ID == uuid.Nil {
			return fmt.Errorf("%w: image id cannot be nil", ErrInvalidMediaReference)
		}
		if seenIDs[d.ID] {
			return fmt.Errorf("%w: duplicate image id %s", ErrInvalidMediaSet, d.ID)
		}
		seenIDs[d.ID] = true
		desiredIDs[i] = d.ID

		if d.SortOrder != i {
			return fmt.Errorf("%w: sort_order must be normalized 0..N-1, expected %d for index %d got %d", ErrInvalidMediaSet, i, i, d.SortOrder)
		}
		if d.IsMain {
			mainCount++
		}
	}

	if len(desired) == 0 {
		if mainCount != 0 {
			return fmt.Errorf("%w: empty images cannot have a main image", ErrInvalidMediaSet)
		}
	} else {
		if mainCount != 1 {
			return fmt.Errorf("%w: exactly one main image required, got %d", ErrInvalidMediaSet, mainCount)
		}
	}

	// ---------------------------------------------------------
	// 2. Lock Ordering (Strict 1 -> 2 -> 3)
	// ---------------------------------------------------------

	// Lock 1: Product row
	var lockedProductID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM products
		WHERE id = $1 AND seller_id = $2
		FOR UPDATE
	`, productID, sellerID).Scan(&lockedProductID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProductNotFound
	} else if err != nil {
		return fmt.Errorf("failed to lock product: %w", err)
	}

	// Lock 2: Canonical images (ordered by id)
	canonicalRows, err := r.db.Query(ctx, `
		SELECT
			id, product_id, image_url, object_key, rendition_url, rendition_object_key,
			alt_text, sort_order, color_id, width, height,
			crop_x, crop_y, crop_width, crop_height, is_main, created_at
		FROM product_images
		WHERE product_id = $1
		ORDER BY id
		FOR UPDATE
	`, productID)
	if err != nil {
		return fmt.Errorf("failed to lock canonical images: %w", err)
	}
	defer canonicalRows.Close()

	var canonicalImages []ProductImage
	canonicalByID := make(map[uuid.UUID]ProductImage)
	for canonicalRows.Next() {
		var img ProductImage
		if err := canonicalRows.Scan(
			&img.ID, &img.ProductID, &img.ImageURL, &img.ObjectKey, &img.RenditionURL, &img.RenditionObjectKey,
			&img.AltText, &img.SortOrder, &img.ColorID, &img.Width, &img.Height,
			&img.CropX, &img.CropY, &img.CropWidth, &img.CropHeight, &img.IsMain, &img.CreatedAt,
		); err != nil {
			return fmt.Errorf("failed to scan canonical image: %w", err)
		}
		canonicalImages = append(canonicalImages, img)
		canonicalByID[img.ID] = img
	}
	if err := canonicalRows.Err(); err != nil {
		return fmt.Errorf("failed during canonical images iteration: %w", err)
	}

	// Lock 3: Requested same-scope staging rows (ordered by id)
	stagedByID := make(map[uuid.UUID]ProductMediaStaging)
	if len(desiredIDs) > 0 {
		stagedRows, err := r.db.Query(ctx, `
			SELECT
				id, seller_id, product_id, client_media_id, status,
				object_key, image_url, content_sha256, byte_size, width, height,
				created_at, consumed_at
			FROM product_media_staging
			WHERE seller_id = $1 AND product_id = $2 AND id = ANY($3)
			ORDER BY id
			FOR UPDATE
		`, sellerID, productID, desiredIDs)
		if err != nil {
			return fmt.Errorf("failed to lock staged media: %w", err)
		}
		defer stagedRows.Close()

		for stagedRows.Next() {
			var stg ProductMediaStaging
			if err := stagedRows.Scan(
				&stg.ID, &stg.SellerID, &stg.ProductID, &stg.ClientMediaID, &stg.Status,
				&stg.ObjectKey, &stg.ImageURL, &stg.ContentSHA256, &stg.ByteSize, &stg.Width, &stg.Height,
				&stg.CreatedAt, &stg.ConsumedAt,
			); err != nil {
				return fmt.Errorf("failed to scan staged media: %w", err)
			}
			stagedByID[stg.ID] = stg
		}
		if err := stagedRows.Err(); err != nil {
			return fmt.Errorf("failed during staged media iteration: %w", err)
		}
	}

	// ---------------------------------------------------------
	// 3. Classify Desired IDs (Validation Before Mutation)
	// ---------------------------------------------------------
	classified := make([]classifiedImage, len(desired))

	for i, d := range desired {
		can, hasCanonical := canonicalByID[d.ID]
		stg, hasStaged := stagedByID[d.ID]

		if hasCanonical {
			if hasStaged {
				switch stg.Status {
				case ProductMediaStagingConsumed:
					// valid retry-safe canonical (Case D)
				case ProductMediaStagingReady, ProductMediaStagingUploading:
					// Active staging conflict with existing canonical image
					return ErrMediaIntegrityViolation
				default:
					return ErrMediaIntegrityViolation
				}
			}

			// staging absent (Case E) or consumed (Case D) is valid canonical
			canCopy := can
			classified[i] = classifiedImage{
				desired:   d,
				class:     classCanonical,
				canonical: &canCopy,
			}
			continue
		}

		// Canonical row does NOT exist:
		if !hasStaged {
			// Case F: neither same-scope canonical nor same-scope staging exists
			// (handles nonexistent IDs and foreign IDs identically)
			return ErrInvalidMediaReference
		}

		// Staging row exists for same Seller + Product, but no canonical:
		switch stg.Status {
		case ProductMediaStagingReady:
			// Case B: staged READY row -> promote
			stgCopy := stg
			classified[i] = classifiedImage{
				desired: d,
				class:   classPromote,
				staging: &stgCopy,
			}
		case ProductMediaStagingUploading:
			// Case C: staged UPLOADING row -> reject
			return ErrStagedMediaNotReady
		case ProductMediaStagingConsumed:
			// Case G: staged CONSUMED row without canonical row -> integrity violation
			return ErrMediaIntegrityViolation
		default:
			return fmt.Errorf("%w: unexpected staging status %s", ErrInvalidMediaReference, stg.Status)
		}
	}

	// ---------------------------------------------------------
	// 4. Orphaned Canonical Cleanup & Deletion
	// ---------------------------------------------------------
	desiredMap := make(map[uuid.UUID]bool, len(desired))
	for _, d := range desired {
		desiredMap[d.ID] = true
	}

	var orphaned []ProductImage
	for _, can := range canonicalImages {
		if !desiredMap[can.ID] {
			orphaned = append(orphaned, can)
		}
	}

	for _, o := range orphaned {
		if o.ObjectKey != nil && *o.ObjectKey != "" {
			if err := r.EnqueueMediaCleanup(ctx, *o.ObjectKey); err != nil {
				return fmt.Errorf("failed to enqueue cleanup for object key: %w", err)
			}
		}
		if o.RenditionObjectKey != nil && *o.RenditionObjectKey != "" {
			if err := r.EnqueueMediaCleanup(ctx, *o.RenditionObjectKey); err != nil {
				return fmt.Errorf("failed to enqueue cleanup for rendition object key: %w", err)
			}
		}
	}

	if len(orphaned) > 0 {
		orphanedIDs := make([]uuid.UUID, len(orphaned))
		for i, o := range orphaned {
			orphanedIDs[i] = o.ID
		}
		_, err := r.db.Exec(ctx, `
			DELETE FROM product_images
			WHERE product_id = $1 AND id = ANY($2)
		`, productID, orphanedIDs)
		if err != nil {
			return fmt.Errorf("failed to delete orphaned product images: %w", err)
		}
	}

	// ---------------------------------------------------------
	// 5. Reset is_main on Remaining Canonical Images
	// ---------------------------------------------------------
	// Reset is_main = false to avoid violating the partial unique index
	// (product_images_single_main_idx) during subsequent updates or inserts.
	_, err = r.db.Exec(ctx, `
		UPDATE product_images
		SET is_main = false
		WHERE product_id = $1 AND is_main = true
	`, productID)
	if err != nil {
		return fmt.Errorf("failed to reset existing is_main flags: %w", err)
	}

	// ---------------------------------------------------------
	// 6. Update Existing Canonical Images
	// ---------------------------------------------------------
	for _, c := range classified {
		if c.class == classCanonical {
			_, err := r.db.Exec(ctx, `
				UPDATE product_images
				SET alt_text = $1,
					color_id = $2,
					is_main = $3,
					sort_order = $4
				WHERE id = $5 AND product_id = $6
			`, c.desired.AltText, c.desired.ColorID, c.desired.IsMain, c.desired.SortOrder, c.desired.ID, productID)
			if err != nil {
				return fmt.Errorf("failed to update canonical image %s: %w", c.desired.ID, err)
			}
		}
	}

	// ---------------------------------------------------------
	// 7. Insert Promoted Staged Images & Mark Staging Consumed
	// ---------------------------------------------------------
	for _, c := range classified {
		if c.class == classPromote {
			var w, h *int
			if c.staging.Width > 0 {
				w = &c.staging.Width
			}
			if c.staging.Height > 0 {
				h = &c.staging.Height
			}
			var objKey *string
			if c.staging.ObjectKey != "" {
				objKey = &c.staging.ObjectKey
			}

			_, err := r.db.Exec(ctx, `
				INSERT INTO product_images (
					id, product_id, image_url, object_key, alt_text, sort_order,
					color_id, width, height, is_main, created_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
			`,
				c.desired.ID,
				productID,
				c.staging.ImageURL,
				objKey,
				c.desired.AltText,
				c.desired.SortOrder,
				c.desired.ColorID,
				w, h,
				c.desired.IsMain,
			)
			if err != nil {
				return fmt.Errorf("failed to insert promoted image %s: %w", c.desired.ID, err)
			}

			res, err := r.db.Exec(ctx, `
				UPDATE product_media_staging
				SET status = $1, consumed_at = NOW()
				WHERE id = $2 AND seller_id = $3 AND product_id = $4 AND status = $5
			`, ProductMediaStagingConsumed, c.desired.ID, sellerID, productID, ProductMediaStagingReady)
			if err != nil {
				return fmt.Errorf("failed to mark staged media consumed: %w", err)
			}
			if res.RowsAffected() != 1 {
				return fmt.Errorf("failed to mark staged media %s consumed: expected 1 row, got %d", c.desired.ID, res.RowsAffected())
			}
		}
	}

	// ---------------------------------------------------------
	// 8. Main Image Denormalization on Products Row
	// ---------------------------------------------------------
	if len(desired) == 0 {
		_, err := r.db.Exec(ctx, `
			UPDATE products
			SET main_image_url = NULL,
				main_image_object_key = NULL
			WHERE id = $1 AND seller_id = $2
		`, productID, sellerID)
		if err != nil {
			return fmt.Errorf("failed to clear product main image fields: %w", err)
		}
	} else {
		var mainImageURL *string
		var mainImageObjectKey *string

		for _, c := range classified {
			if c.desired.IsMain {
				if c.class == classCanonical {
					mainImageURL = &c.canonical.ImageURL
					mainImageObjectKey = c.canonical.ObjectKey
				} else if c.class == classPromote {
					mainImageURL = &c.staging.ImageURL
					if c.staging.ObjectKey != "" {
						mainImageObjectKey = &c.staging.ObjectKey
					}
				}
				break
			}
		}

		_, err := r.db.Exec(ctx, `
			UPDATE products
			SET main_image_url = $1,
				main_image_object_key = $2
			WHERE id = $3 AND seller_id = $4
		`, mainImageURL, mainImageObjectKey, productID, sellerID)
		if err != nil {
			return fmt.Errorf("failed to update product main image fields: %w", err)
		}
	}

	return nil
}
