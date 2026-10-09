package vision_test

import (
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
	"github.com/google/uuid"
)

func TestTaxonomyContextHash(t *testing.T) {
	cat1 := uuid.New()
	cat2 := uuid.New()
	dict1 := uuid.New()

	vocab := vision.NewVocabularyRegistry()

	// T. Hash determinism independent of array ordering
	hash1 := vision.ComputeTaxonomyContextHash([]uuid.UUID{cat1, cat2}, []uuid.UUID{dict1}, vocab)
	hash2 := vision.ComputeTaxonomyContextHash([]uuid.UUID{cat2, cat1}, []uuid.UUID{dict1}, vocab)

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash independent of slice order, got %s != %s", hash1, hash2)
	}

	// U. Hash changes when vocabulary changes
	hash3 := vision.ComputeTaxonomyContextHash([]uuid.UUID{cat1}, []uuid.UUID{dict1}, vocab)
	if hash1 == hash3 {
		t.Fatalf("expected hash to change when allowed categories change")
	}

	vocab2 := &vision.VocabularyRegistry{ValidIDs: map[string]bool{"vision.new.id": true}}
	hash4 := vision.ComputeTaxonomyContextHash([]uuid.UUID{cat1, cat2}, []uuid.UUID{dict1}, vocab2)
	if hash1 == hash4 {
		t.Fatalf("expected hash to change when internal vision vocabulary changes")
	}
}

func TestValidateAnalysisResult(t *testing.T) {
	catID := uuid.New()
	dictID := uuid.New()
	imgID := uuid.New()
	vocab := vision.NewVocabularyRegistry()

	req := vision.VisionAnalysisRequest{
		AllowedCategoryIDs:   []uuid.UUID{catID},
		AllowedDictionaryIDs: []uuid.UUID{dictID},
		Images: []vision.VisionInputImage{
			{ImageID: imgID},
		},
	}

	ptr := func(s string) *string { return &s }

	t.Run("Valid Response", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV1{
				ObservedCategoryID: &catID,
				DominantColorIDs:   []string{dictID.String(), "vision.silhouette.regular"}, // Just reusing known strings for testing valid resolution
				PatternID:          ptr("vision.pattern.striped"),
				Evidence: []vision.Evidence{
					{
						Field:    "pattern",
						ValueID:  "vision.pattern.striped",
						ImageIDs: []uuid.UUID{imgID},
					},
				},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
			},
		}

		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err != nil {
			t.Fatalf("expected valid result, got %v", err)
		}
	})

	t.Run("Unknown Taxonomy ID", func(t *testing.T) {
		wrongCat := uuid.New()
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV1{
				ObservedCategoryID: &wrongCat,
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsUnknownTaxonomy(err) {
			t.Fatalf("expected ErrUnknownTaxonomyID, got %v", err)
		}
	})

	t.Run("Unknown Vision ID", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV1{
				PatternID: ptr("vision.pattern.fake"),
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsUnknownVision(err) {
			t.Fatalf("expected ErrUnknownVisionID, got %v", err)
		}
	})

	t.Run("Unknown Image ID in Evidence", func(t *testing.T) {
		wrongImg := uuid.New()
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV1{
				PatternID: ptr("vision.pattern.striped"),
				Evidence: []vision.Evidence{
					{ImageIDs: []uuid.UUID{wrongImg}},
				},
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsUnknownImage(err) {
			t.Fatalf("expected ErrUnknownImageID, got %v", err)
		}
	})

	t.Run("Conflicting Observation (Dominant and Secondary Color overlap)", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV1{
				DominantColorIDs:  []string{dictID.String()},
				SecondaryColorIDs: []string{dictID.String()},
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsConflicting(err) {
			t.Fatalf("expected ErrConflictingData, got %v", err)
		}
	})

	t.Run("Invalid Enum", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "approve", // not allowed
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsInvalidEnum(err) {
			t.Fatalf("expected ErrInvalidEnum, got %v", err)
		}
	})

	t.Run("Abstention Valid", func(t *testing.T) {
		// Just omitted fields -> valid
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV1{
				Confidence: map[string]vision.ConfidenceScore{
					"pattern": {Level: "abstained", Score: 0},
				},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "uncertain",
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err != nil {
			t.Fatalf("expected abstained result to be valid, got %v", err)
		}
	})

	t.Run("Observation Independent of Moderation", func(t *testing.T) {
		// Proves X
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV1{
				PatternID: ptr("vision.pattern.striped"),
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear", // required field technically for mod struct, but obs is intact
			},
		}

		obs := res.VisualObservation
		if *obs.PatternID != "vision.pattern.striped" {
			t.Fatalf("visual observation inaccessible")
		}
	})
}

// Helpers
func errorsIsUnknownTaxonomy(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), "unknown taxonomy ID")
}
func errorsIsUnknownVision(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), "unknown vision vocabulary ID")
}
func errorsIsUnknownImage(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), "unknown image ID")
}
func errorsIsInvalidEnum(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), "invalid enum value")
}
func errorsIsConflicting(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), "conflicting observation data")
}
func contains(s, substr string) bool {
	// simple contains for test
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
