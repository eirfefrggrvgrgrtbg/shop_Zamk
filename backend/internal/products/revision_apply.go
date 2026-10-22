package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	// ErrRevisionNotPending: the revision was already approved, rejected or superseded (repeat approval is a no-op refusal).
	ErrRevisionNotPending = errors.New("revision is not pending")
	// ErrRevisionConflict: the live variants changed after the revision was submitted, so applying it would overwrite them.
	ErrRevisionConflict = errors.New("revision conflicts with the current product state")
	// ErrRevisionNotApplicable wraps any refusal to apply an approved revision to the current product state.
	ErrRevisionNotApplicable = errors.New("revision cannot be applied to the current product state")
)

// ProductRevisionChanges is the part of a published-product edit that lives outside the products row.
// It is stored inside the revision snapshot next to the product fields. A nil field means "not changed by
// this revision", so approval leaves that part of the live product untouched.
type ProductRevisionChanges struct {
	// Variants is the full target variant list (ids already resolved against the live variants at submission).
	Variants *[]ProductVariant `json:"revisionVariants,omitempty"`
	// Images is the full target gallery (canonical ids or same-product staged uploads).
	Images *[]DesiredProductImage `json:"revisionImages,omitempty"`
	// VariantBase records updated_at of every live variant at submission; approval refuses if any of them moved.
	VariantBase map[uuid.UUID]time.Time `json:"revisionVariantBase,omitempty"`
}

// PendingRevisionView is what Admin sees of a pending revision of a published product.
type PendingRevisionView struct {
	ID              uuid.UUID        `json:"id"`
	CreatedAt       time.Time        `json:"createdAt"`
	Variants        []ProductVariant `json:"variants,omitempty"`
	VariantsChanged bool             `json:"variantsChanged"`
	ImagesChanged   bool             `json:"imagesChanged"`
	ImagesCount     int              `json:"imagesCount"`
	// Images is the proposed gallery in order (existing photos as stored, new ones from their uploads).
	Images []ProductImage `json:"images,omitempty"`
	// ImagesMissing counts proposed photos that no longer resolve (lost upload); approval would be refused.
	ImagesMissing int `json:"imagesMissing,omitempty"`
	// ChangedFields lists the product fields (title, description, prices…) the revision changes; filled for Admin.
	ChangedFields []RevisionFieldChange `json:"changedFields,omitempty"`
	scalars       revisionScalars
}

func parseRevisionChanges(snapBytes []byte) (ProductRevisionChanges, error) {
	var ch ProductRevisionChanges
	if len(snapBytes) == 0 {
		return ch, nil
	}
	if err := json.Unmarshal(snapBytes, &ch); err != nil {
		return ch, fmt.Errorf("failed to decode revision changes: %w", err)
	}
	return ch, nil
}

// addRevisionChangesToSnapshot merges the change set into the snapshot map under its own keys.
func addRevisionChangesToSnapshot(snap map[string]interface{}, ch ProductRevisionChanges) error {
	b, err := json.Marshal(ch)
	if err != nil {
		return err
	}
	var extra map[string]interface{}
	if err := json.Unmarshal(b, &extra); err != nil {
		return err
	}
	for k, v := range extra {
		snap[k] = v
	}
	return nil
}

func variantBase(variants []ProductVariant) map[uuid.UUID]time.Time {
	base := make(map[uuid.UUID]time.Time, len(variants))
	for _, v := range variants {
		base[v.ID] = v.UpdatedAt
	}
	return base
}

// SupersedePendingRevisions closes older pending revisions when the seller submits a newer full target state,
// so a later approval can never promote a stale one.
func (r *Repository) SupersedePendingRevisions(ctx context.Context, productID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE product_revisions SET status = 'rejected', updated_at = now()
		WHERE product_id = $1 AND status = 'pending'
	`, productID)
	if err != nil {
		return fmt.Errorf("failed to supersede pending revisions: %w", err)
	}
	return nil
}

// applyRevisionVariants applies the revision's variant matrix. Must run inside the approval transaction.
// The C5 deactivation guard runs again inside MergeProductVariants against the current warehouse state.
func (r *Repository) applyRevisionVariants(ctx context.Context, productID uuid.UUID, ch ProductRevisionChanges) error {
	if ch.Variants == nil {
		return nil
	}
	// Lock the live variant rows so a concurrent price/variant write cannot land between the check and the merge.
	if _, err := r.db.Exec(ctx, `SELECT id FROM product_variants WHERE product_id = $1 ORDER BY id FOR UPDATE`, productID); err != nil {
		return fmt.Errorf("failed to lock product variants: %w", err)
	}
	current, err := r.GetProductVariants(ctx, productID)
	if err != nil {
		return err
	}
	seen := make(map[uuid.UUID]bool, len(current))
	for _, cv := range current {
		if seen[cv.ID] {
			continue
		}
		seen[cv.ID] = true
		baseAt, ok := ch.VariantBase[cv.ID]
		if !ok || !baseAt.Equal(cv.UpdatedAt) {
			return ErrRevisionConflict
		}
	}
	if len(seen) != len(ch.VariantBase) {
		return ErrRevisionConflict
	}

	variants := *ch.Variants
	// Same rule as submitting a draft for moderation: every sold variant has a price and a seller SKU.
	if len(variants) == 0 {
		return ErrProductVariantsRequired
	}
	for i := range variants {
		variants[i].ProductID = productID
		if variants[i].PriceCents == nil || *variants[i].PriceCents <= 0 {
			return ErrProductPriceInvalid
		}
		if variants[i].SellerSKU == nil || strings.TrimSpace(*variants[i].SellerSKU) == "" {
			return ErrProductSKURequired
		}
	}
	if err := r.MergeProductVariants(ctx, productID, variants); err != nil {
		return err
	}
	for _, v := range variants {
		if len(v.Attributes) == 0 {
			continue
		}
		if err := r.InsertVariantAttributeValues(ctx, v.ID, v.Attributes); err != nil {
			return err
		}
	}
	return nil
}

// applyRevisionImages reconciles the gallery and re-validates the final media/variant state.
func (r *Repository) applyRevisionImages(ctx context.Context, productID uuid.UUID, ch ProductRevisionChanges) error {
	if ch.Variants == nil && ch.Images == nil {
		return nil
	}
	if ch.Images != nil {
		// Every new photo must exist and have its 4:5 rendition before anything is promoted.
		if err := r.requireRevisionPhotosReady(ctx, productID, *ch.Images); err != nil {
			return err
		}
		var sellerID uuid.UUID
		if err := r.db.QueryRow(ctx, `SELECT seller_id FROM products WHERE id = $1`, productID).Scan(&sellerID); err != nil {
			return fmt.Errorf("failed to get product seller: %w", err)
		}
		if err := r.ReconcileProductImagesTx(ctx, sellerID, productID, *ch.Images); err != nil {
			return err
		}
	}

	dbVariants, err := r.GetProductVariants(ctx, productID)
	if err != nil {
		return err
	}
	var active []ProductVariant
	for _, v := range dbVariants {
		if v.IsActive {
			active = append(active, v)
		}
	}
	dbImages, err := r.GetProductImages(ctx, productID)
	if err != nil {
		return err
	}
	finalImages := make([]DesiredProductImage, len(dbImages))
	for i, img := range dbImages {
		finalImages[i] = DesiredProductImage{ID: img.ID, AltText: img.AltText, ColorID: img.ColorID, IsMain: img.IsMain, SortOrder: img.SortOrder}
	}
	return validateFinalProductMediaState(active, finalImages)
}

// GetPendingRevisionView returns the Admin view of the product's pending revision, or nil.
func (r *Repository) GetPendingRevisionView(ctx context.Context, productID uuid.UUID) (*PendingRevisionView, error) {
	var view PendingRevisionView
	var snapBytes []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, created_at, content_snapshot FROM product_revisions
		WHERE product_id = $1 AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1
	`, productID).Scan(&view.ID, &view.CreatedAt, &snapBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending revision: %w", err)
	}
	ch, err := parseRevisionChanges(snapBytes)
	if err != nil {
		return nil, err
	}
	view.scalars = parseRevisionScalars(snapBytes)
	if ch.Images != nil {
		view.ImagesChanged = true
		view.ImagesCount = len(*ch.Images)
		st, err := r.resolveRevisionImages(ctx, productID, *ch.Images)
		if err != nil {
			return nil, err
		}
		view.Images = st.images
		view.ImagesMissing = len(st.notFound) + len(st.notReady)
	}
	if ch.Variants != nil {
		view.VariantsChanged = true
		view.Variants = *ch.Variants
		if err := r.fillVariantLabels(ctx, view.Variants); err != nil {
			return nil, err
		}
	}
	return &view, nil
}

// fillVariantLabels resolves dictionary color/size names for variants that are not stored yet.
func (r *Repository) fillVariantLabels(ctx context.Context, variants []ProductVariant) error {
	for i := range variants {
		v := &variants[i]
		if v.ColorID != nil {
			var name string
			if err := r.db.QueryRow(ctx, `SELECT name_ru FROM colors WHERE id = $1`, *v.ColorID).Scan(&name); err == nil {
				v.ColorName = &name
				v.Color = &name
			}
		}
		if v.SizeValueID != nil {
			var size string
			if err := r.db.QueryRow(ctx, `SELECT value FROM size_values WHERE id = $1`, *v.SizeValueID).Scan(&size); err == nil {
				v.Size = &size
			}
		}
	}
	return nil
}

// overlayRevisionVariants returns the revision's variant list with the read-only warehouse state of the live
// variants it keeps; live variants the revision removes are listed as inactive.
func overlayRevisionVariants(live, revision []ProductVariant) []ProductVariant {
	liveByID := make(map[uuid.UUID]ProductVariant, len(live))
	for _, lv := range live {
		liveByID[lv.ID] = lv
	}
	kept := make(map[uuid.UUID]bool, len(revision))
	out := make([]ProductVariant, 0, len(revision)+len(live))
	for _, rv := range revision {
		if lv, ok := liveByID[rv.ID]; ok {
			kept[rv.ID] = true
			rv.HasInventoryRecord = lv.HasInventoryRecord
			rv.TotalStock = lv.TotalStock
			rv.ReservedStock = lv.ReservedStock
			rv.AvailableStock = lv.AvailableStock
			rv.InStock = lv.InStock
			if rv.ColorHex == nil {
				rv.ColorHex = lv.ColorHex
			}
		}
		rv.IsActive = true
		out = append(out, rv)
	}
	for _, lv := range live {
		if !kept[lv.ID] {
			lv.IsActive = false
			out = append(out, lv)
		}
	}
	return out
}

// mainImageOf returns the main photo's URL and object key of a gallery, or the given fallbacks.
func mainImageOf(images []ProductImage, fallbackURL, fallbackKey *string) (*string, *string) {
	for _, img := range images {
		if img.IsMain {
			url := img.ImageURL
			return &url, img.ObjectKey
		}
	}
	return fallbackURL, fallbackKey
}
