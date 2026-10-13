package vision

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrUnknownTaxonomyID = errors.New("vision: unknown taxonomy ID")
	ErrUnknownVisionID   = errors.New("vision: unknown vision vocabulary ID")
	ErrUnknownImageID    = errors.New("vision: unknown image ID in evidence")
	ErrInvalidEnum       = errors.New("vision: invalid enum value")
	ErrConflictingData   = errors.New("vision: conflicting observation data")
)

// ValidateAnalysisResult performs strict referential and logical validation.
// It assumes structural JSON unmarshaling already succeeded.
func ValidateAnalysisResult(req VisionAnalysisRequest, result *VisionAnalysisResult, vocab *VocabularyRegistry) error {
	obs := result.VisualObservation

	allowedColors := make(map[uuid.UUID]bool)
	if req.TaxonomyContext != nil {
		for _, col := range req.TaxonomyContext.Colors {
			allowedColors[col.ID] = true
		}
	}
	if len(allowedColors) == 0 {
		for _, id := range req.AllowedColorIDs {
			allowedColors[id] = true
		}
	}

	allowedImages := make(map[uuid.UUID]bool)
	for _, img := range req.Images {
		allowedImages[img.ImageID] = true
	}
	if len(allowedImages) == 0 {
		for _, img := range req.Snapshot.Images {
			allowedImages[img.ImageID] = true
		}
	}

	// 1. Validate Category Consistency
	switch obs.CategoryConsistency {
	case CategoryConsistencyConsistent, CategoryConsistencyContradictory, CategoryConsistencyUncertain:
		// valid
	default:
		return fmt.Errorf("%w: invalid categoryConsistency %q", ErrInvalidEnum, obs.CategoryConsistency)
	}

	// 2. Validate Observed Category Text (reject pseudo-null strings)
	if obs.ObservedCategoryText != nil {
		txt := strings.TrimSpace(strings.ToLower(*obs.ObservedCategoryText))
		if txt == "null" || txt == "none" || txt == "unknown" || txt == "n/a" || txt == "" {
			return fmt.Errorf("%w: observedCategoryText cannot be pseudo-null string %q, must be true JSON null or descriptive text", ErrInvalidEnum, *obs.ObservedCategoryText)
		}
	}

	// 2. Validate Colors (canonical colors.id only)
	if err := validateColorIDs(obs.DominantColorIDs, allowedColors); err != nil {
		return fmt.Errorf("dominant colors: %w", err)
	}
	if err := validateColorIDs(obs.SecondaryColorIDs, allowedColors); err != nil {
		return fmt.Errorf("secondary colors: %w", err)
	}

	// Check for conflicting colors
	domColors := make(map[uuid.UUID]bool)
	for _, c := range obs.DominantColorIDs {
		domColors[c] = true
	}
	for _, c := range obs.SecondaryColorIDs {
		if domColors[c] {
			return fmt.Errorf("%w: color %s cannot be both dominant and secondary", ErrConflictingData, c)
		}
	}

	// 3. Validate Vision IDs
	if err := validateVisionID(obs.SilhouetteID, vocab); err != nil {
		return fmt.Errorf("silhouette: %w", err)
	}
	if err := validateVisionID(obs.PatternID, vocab); err != nil {
		return fmt.Errorf("pattern: %w", err)
	}
	if err := validateVisionID(obs.VisibleTextureID, vocab); err != nil {
		return fmt.Errorf("texture: %w", err)
	}

	// 4. Validate Evidence
	for _, ev := range obs.Evidence {
		for _, imgID := range ev.ImageIDs {
			if !allowedImages[imgID] {
				return fmt.Errorf("%w: evidence field %q references unknown image %s", ErrUnknownImageID, ev.Field, imgID)
			}
		}
	}

	// 5. Validate Moderation
	mod := result.ModerationEvaluation
	for _, issue := range mod.QualityIssues {
		if !vocab.ValidIDs[issue] {
			return fmt.Errorf("%w: moderation quality issue %s", ErrUnknownVisionID, issue)
		}
	}

	switch mod.ModerationRecommendation {
	case "clear", "review_recommended", "uncertain":
		// valid
	default:
		return fmt.Errorf("%w: moderation recommendation %q", ErrInvalidEnum, mod.ModerationRecommendation)
	}

	return nil
}

func validateColorIDs(ids []uuid.UUID, allowedColors map[uuid.UUID]bool) error {
	for _, id := range ids {
		if !allowedColors[id] {
			return fmt.Errorf("%w: color ID %s is not a canonical color", ErrUnknownTaxonomyID, id)
		}
	}
	return nil
}

func validateVisionID(id *string, vocab *VocabularyRegistry) error {
	if id == nil {
		return nil
	}
	if !vocab.ValidIDs[*id] {
		return fmt.Errorf("%w: %s", ErrUnknownVisionID, *id)
	}
	return nil
}
