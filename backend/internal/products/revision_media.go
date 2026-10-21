package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	// ErrStagedMediaRenditionMissing: a new photo of a revision has no 4:5 rendition yet (seller has not cropped it).
	ErrStagedMediaRenditionMissing = errors.New("new photo has no 4:5 rendition")
	// ErrRevisionMediaObjectMissing: a new photo's file (original or rendition) is absent from object storage.
	ErrRevisionMediaObjectMissing = errors.New("photo file is missing from storage")
)

// MediaObjectVerifier reports whether an object exists in storage. It is optional: without it the approval
// relies on the database state alone (staged rows are only 'ready' after their upload succeeded).
type MediaObjectVerifier func(ctx context.Context, objectKey string) (bool, error)

// WithMediaObjectVerifier enables the storage existence check done before a revision with new photos is applied.
func (s *Service) WithMediaObjectVerifier(v MediaObjectVerifier) *Service {
	s.mediaVerifier = v
	return s
}

// GetStagedMediaForCrop returns a seller's staged photo that can still be cropped (status 'ready').
func (r *Repository) GetStagedMediaForCrop(ctx context.Context, sellerID, productID, id uuid.UUID) (*ProductMediaStaging, error) {
	var row ProductMediaStaging
	err := r.db.QueryRow(ctx, `
		SELECT id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at,
			crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key
		FROM product_media_staging
		WHERE id = $1 AND seller_id = $2 AND product_id = $3
	`, id, sellerID, productID).Scan(
		&row.ID, &row.SellerID, &row.ProductID, &row.ClientMediaID, &row.Status,
		&row.ObjectKey, &row.ImageURL, &row.ContentSHA256, &row.ByteSize, &row.Width, &row.Height,
		&row.CreatedAt, &row.ConsumedAt,
		&row.CropX, &row.CropY, &row.CropWidth, &row.CropHeight, &row.RenditionURL, &row.RenditionObjectKey,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStagedMediaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get staged media for crop: %w", err)
	}
	switch row.Status {
	case ProductMediaStagingReady:
		return &row, nil
	case ProductMediaStagingConsumed:
		return nil, ErrStagedMediaAlreadyConsumed
	default:
		return nil, ErrStagedMediaNotReady
	}
}

// UpdateStagedMediaCrop stores the 4:5 crop and rendition of a staged photo. The rendition object must already be
// uploaded: the row only ever points at files that exist.
func (r *Repository) UpdateStagedMediaCrop(ctx context.Context, sellerID, productID, id uuid.UUID, cropX, cropY, cropWidth, cropHeight float64, renditionURL, renditionObjectKey string) error {
	res, err := r.db.Exec(ctx, `
		UPDATE product_media_staging
		SET crop_x = $1, crop_y = $2, crop_width = $3, crop_height = $4, rendition_url = $5, rendition_object_key = $6
		WHERE id = $7 AND seller_id = $8 AND product_id = $9 AND status = $10
	`, cropX, cropY, cropWidth, cropHeight, renditionURL, renditionObjectKey, id, sellerID, productID, ProductMediaStagingReady)
	if err != nil {
		return fmt.Errorf("failed to store staged media crop: %w", err)
	}
	if res.RowsAffected() != 1 {
		return ErrStagedMediaNotFound
	}
	return nil
}

type revisionImageState struct {
	images []ProductImage // resolved, in gallery order
	// problems per desired id
	notFound   []uuid.UUID
	notReady   []uuid.UUID
	noRendered []uuid.UUID
}

// resolveRevisionImages turns the revision's desired gallery into displayable images: existing product images
// as stored, new photos from their staging rows (marked Pending).
func (r *Repository) resolveRevisionImages(ctx context.Context, productID uuid.UUID, desired []DesiredProductImage) (revisionImageState, error) {
	var st revisionImageState
	canonical, err := r.GetProductImages(ctx, productID)
	if err != nil {
		return st, err
	}
	canonicalByID := make(map[uuid.UUID]ProductImage, len(canonical))
	for _, img := range canonical {
		canonicalByID[img.ID] = img
	}

	ids := make([]uuid.UUID, 0, len(desired))
	for _, d := range desired {
		if _, ok := canonicalByID[d.ID]; !ok {
			ids = append(ids, d.ID)
		}
	}
	staged := make(map[uuid.UUID]ProductMediaStaging, len(ids))
	if len(ids) > 0 {
		rows, err := r.db.Query(ctx, `
			SELECT id, status, object_key, image_url, width, height, created_at,
				crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key
			FROM product_media_staging
			WHERE product_id = $1 AND id = ANY($2)
		`, productID, ids)
		if err != nil {
			return st, fmt.Errorf("failed to read staged revision photos: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var s ProductMediaStaging
			if err := rows.Scan(&s.ID, &s.Status, &s.ObjectKey, &s.ImageURL, &s.Width, &s.Height, &s.CreatedAt,
				&s.CropX, &s.CropY, &s.CropWidth, &s.CropHeight, &s.RenditionURL, &s.RenditionObjectKey); err != nil {
				return st, fmt.Errorf("failed to scan staged revision photo: %w", err)
			}
			staged[s.ID] = s
		}
		if err := rows.Err(); err != nil {
			return st, err
		}
	}

	for _, d := range desired {
		var img ProductImage
		if c, ok := canonicalByID[d.ID]; ok {
			img = c
		} else if s, ok := staged[d.ID]; ok {
			switch s.Status {
			case ProductMediaStagingReady:
			case ProductMediaStagingUploading:
				st.notReady = append(st.notReady, d.ID)
				continue
			default: // consumed without a canonical row: the revision no longer matches the stored state
				st.notFound = append(st.notFound, d.ID)
				continue
			}
			key := s.ObjectKey
			w, h := s.Width, s.Height
			img = ProductImage{
				ID: s.ID, ProductID: productID, ImageURL: s.ImageURL, ObjectKey: &key,
				Width: &w, Height: &h, CreatedAt: s.CreatedAt, Pending: true,
				CropX: s.CropX, CropY: s.CropY, CropWidth: s.CropWidth, CropHeight: s.CropHeight,
				RenditionURL: s.RenditionURL, RenditionObjectKey: s.RenditionObjectKey,
			}
			if s.RenditionURL == nil || s.RenditionObjectKey == nil || s.CropWidth == nil {
				st.noRendered = append(st.noRendered, d.ID)
			}
		} else {
			st.notFound = append(st.notFound, d.ID)
			continue
		}
		img.AltText = d.AltText
		img.ColorID = d.ColorID
		img.IsMain = d.IsMain
		img.SortOrder = d.SortOrder
		st.images = append(st.images, img)
	}
	sort.SliceStable(st.images, func(i, j int) bool { return st.images[i].SortOrder < st.images[j].SortOrder })
	return st, nil
}

// ValidateRevisionMediaReferences is the early (submission-time) check that every photo of the proposed gallery
// is either an existing image of the product or a finished upload of the same seller and product.
func (r *Repository) ValidateRevisionMediaReferences(ctx context.Context, productID uuid.UUID, desired []DesiredProductImage) error {
	st, err := r.resolveRevisionImages(ctx, productID, desired)
	if err != nil {
		return err
	}
	if len(st.notReady) > 0 {
		return ErrStagedMediaNotReady
	}
	if len(st.notFound) > 0 {
		return ErrInvalidMediaReference
	}
	return nil
}

// requireRevisionPhotosReady is the approval-time check: every new photo must exist and carry a 4:5 rendition.
// Existing (already published) photos are not required to be re-cropped.
func (r *Repository) requireRevisionPhotosReady(ctx context.Context, productID uuid.UUID, desired []DesiredProductImage) error {
	st, err := r.resolveRevisionImages(ctx, productID, desired)
	if err != nil {
		return err
	}
	if len(st.notReady) > 0 {
		return ErrStagedMediaNotReady
	}
	if len(st.notFound) > 0 {
		return ErrInvalidMediaReference
	}
	if len(st.noRendered) > 0 {
		return ErrStagedMediaRenditionMissing
	}
	return nil
}

// verifyRevisionMedia runs before the approval transaction. Object storage is not part of the PostgreSQL
// transaction, so the existence of every new file is checked first: approval is refused instead of publishing
// a broken URL. The files are immutable (unique staged keys) and protected from the TTL sweep while the
// revision is pending, so the check cannot be invalidated between here and the commit.
func (s *Service) verifyRevisionMedia(ctx context.Context, productID uuid.UUID, snapshot map[string]interface{}) error {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	changes, err := parseRevisionChanges(b)
	if err != nil {
		return err
	}
	if changes.Images == nil {
		return nil
	}
	st, err := s.repo.resolveRevisionImages(ctx, productID, *changes.Images)
	if err != nil {
		return err
	}
	switch {
	case len(st.notReady) > 0:
		return fmt.Errorf("%w: %w", ErrRevisionNotApplicable, ErrStagedMediaNotReady)
	case len(st.notFound) > 0:
		return fmt.Errorf("%w: %w", ErrRevisionNotApplicable, ErrInvalidMediaReference)
	case len(st.noRendered) > 0:
		return fmt.Errorf("%w: %w", ErrRevisionNotApplicable, ErrStagedMediaRenditionMissing)
	}
	if s.mediaVerifier == nil {
		return nil
	}
	for _, img := range st.images {
		if !img.Pending {
			continue
		}
		for _, key := range []*string{img.ObjectKey, img.RenditionObjectKey} {
			if key == nil || *key == "" {
				return fmt.Errorf("%w: %w", ErrRevisionNotApplicable, ErrRevisionMediaObjectMissing)
			}
			ok, err := s.mediaVerifier(ctx, *key)
			if err != nil {
				return fmt.Errorf("failed to verify photo file: %w", err)
			}
			if !ok {
				return fmt.Errorf("%w: %w", ErrRevisionNotApplicable, ErrRevisionMediaObjectMissing)
			}
		}
	}
	return nil
}
