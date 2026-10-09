package vision

import (
	"errors"
	"fmt"

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

	// Build lookup sets for fast validation
	allowedCats := make(map[uuid.UUID]bool)
	for _, id := range req.AllowedCategoryIDs {
		allowedCats[id] = true
	}

	allowedDicts := make(map[string]bool)
	for _, id := range req.AllowedDictionaryIDs {
		allowedDicts[id.String()] = true
	}

	allowedImages := make(map[uuid.UUID]bool)
	for _, img := range req.Images {
		allowedImages[img.ImageID] = true
	}

	// 1. Validate Category
	if obs.ObservedCategoryID != nil {
		if !allowedCats[*obs.ObservedCategoryID] {
			return fmt.Errorf("%w: observed category %s not in allowed list", ErrUnknownTaxonomyID, obs.ObservedCategoryID)
		}
	}

	// 2. Validate Colors (can be either taxonomy UUIDs or vision IDs)
	if err := validateColorIDs(obs.DominantColorIDs, allowedDicts, vocab); err != nil {
		return fmt.Errorf("dominant colors: %w", err)
	}
	if err := validateColorIDs(obs.SecondaryColorIDs, allowedDicts, vocab); err != nil {
		return fmt.Errorf("secondary colors: %w", err)
	}

	// Check for conflicting colors
	domColors := make(map[string]bool)
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

func validateColorIDs(ids []string, allowedDicts map[string]bool, vocab *VocabularyRegistry) error {
	for _, id := range ids {
		if _, err := uuid.Parse(id); err == nil {
			if !allowedDicts[id] {
				return fmt.Errorf("%w: dict ID %s", ErrUnknownTaxonomyID, id)
			}
		} else {
			if !vocab.ValidIDs[id] {
				return fmt.Errorf("%w: string ID %s", ErrUnknownVisionID, id)
			}
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
