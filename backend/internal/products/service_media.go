package products

import (
	"fmt"

	"github.com/google/uuid"
)

// validateFinalProductMediaState enforces the canonical media invariant on the product's final state.
//
// Invariants:
// 1. Structural integrity:
//    - 0..8 images allowed (draft progressive authoring allows 0 images).
//    - No duplicate image IDs.
//    - If images > 0, exactly one global isMain=true image required.
// 2. GENERAL_GALLERY (Product has no color dimension, i.e. 0 active variant colors):
//    - All images must have color_id == nil.
//    - Any image with color_id != nil is rejected with ErrInvalidImageColor.
// 3. COLORWAY_GALLERIES (Product has color dimension, i.e. >= 1 active variant color):
//    - No hybrid state: cannot mix generic images (color_id == nil) and color-specific images (color_id != nil).
//      Violation returns ErrInvalidMediaMode.
//    - Every color-specific image must belong to an ACTIVE variant color (where is_active == true).
//      Soft-deleted or non-existent colors return ErrInvalidImageColor.
func validateFinalProductMediaState(
	finalActiveVariants []ProductVariant,
	finalImages []DesiredProductImage,
) error {
	if len(finalImages) == 0 {
		return nil
	}

	if len(finalImages) > 8 {
		return fmt.Errorf("%w: maximum 8 images allowed, got %d", ErrInvalidMediaSet, len(finalImages))
	}

	seenIDs := make(map[uuid.UUID]bool, len(finalImages))
	mainCount := 0

	for _, img := range finalImages {
		if img.ID == uuid.Nil {
			return fmt.Errorf("%w: image id cannot be nil", ErrInvalidMediaReference)
		}
		if seenIDs[img.ID] {
			return fmt.Errorf("%w: duplicate image id %s", ErrInvalidMediaSet, img.ID)
		}
		seenIDs[img.ID] = true
		if img.IsMain {
			mainCount++
		}
	}

	if mainCount != 1 {
		return fmt.Errorf("%w: exactly one main image required, got %d", ErrInvalidMediaSet, mainCount)
	}

	// Build active color lookup strictly from active variants (is_active == true)
	activeColors := make(map[uuid.UUID]bool)
	for _, v := range finalActiveVariants {
		if v.IsActive && v.ColorID != nil && *v.ColorID != uuid.Nil {
			activeColors[*v.ColorID] = true
		}
	}
	hasColorDimension := len(activeColors) > 0

	var genericCount int
	var colorCount int
	for _, img := range finalImages {
		if img.ColorID == nil || *img.ColorID == uuid.Nil {
			genericCount++
		} else {
			colorCount++
			if !activeColors[*img.ColorID] {
				return fmt.Errorf("%w: color %s is not an active product color", ErrInvalidImageColor, img.ColorID)
			}
		}
	}

	// If product has NO color dimension, all images must be generic
	if !hasColorDimension {
		if colorCount > 0 {
			return fmt.Errorf("%w: cannot attach color to image on a product without color dimension", ErrInvalidImageColor)
		}
		return nil
	}

	// Product HAS color dimension:
	// Reject hybrid mix: cannot mix generic and color-specific images
	if genericCount > 0 && colorCount > 0 {
		return fmt.Errorf("%w: product cannot mix generic images and colorway-specific images", ErrInvalidMediaMode)
	}

	return nil
}
