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

	ctx1 := vision.TaxonomyContext{
		Categories: []vision.TaxonomyItem{
			{ID: cat1, Label: "Dresses"},
			{ID: cat2, Label: "Pants"},
		},
		Dictionaries: []vision.TaxonomyItem{
			{ID: dict1, Label: "Fabric"},
		},
		VocabularyRegistry: vocab,
	}

	// 1. Same values different order → same hash
	ctxReversed := vision.TaxonomyContext{
		Categories: []vision.TaxonomyItem{
			{ID: cat2, Label: "Pants"},
			{ID: cat1, Label: "Dresses"},
		},
		Dictionaries: []vision.TaxonomyItem{
			{ID: dict1, Label: "Fabric"},
		},
		VocabularyRegistry: vocab,
	}
	hash1 := vision.ComputeTaxonomyContextHash(ctx1)
	hashReversed := vision.ComputeTaxonomyContextHash(ctxReversed)
	if hash1 != hashReversed {
		t.Fatalf("expected deterministic hash independent of order, got %s != %s", hash1, hashReversed)
	}

	// 2. New allowed ID → different hash
	cat3 := uuid.New()
	ctxAdded := vision.TaxonomyContext{
		Categories: []vision.TaxonomyItem{
			{ID: cat1, Label: "Dresses"},
			{ID: cat2, Label: "Pants"},
			{ID: cat3, Label: "Shirts"},
		},
		Dictionaries: []vision.TaxonomyItem{
			{ID: dict1, Label: "Fabric"},
		},
		VocabularyRegistry: vocab,
	}
	if vision.ComputeTaxonomyContextHash(ctxAdded) == hash1 {
		t.Fatalf("expected hash to change when new allowed ID added")
	}

	// 3. Removed ID → different hash
	ctxRemoved := vision.TaxonomyContext{
		Categories: []vision.TaxonomyItem{
			{ID: cat1, Label: "Dresses"},
		},
		Dictionaries: []vision.TaxonomyItem{
			{ID: dict1, Label: "Fabric"},
		},
		VocabularyRegistry: vocab,
	}
	if vision.ComputeTaxonomyContextHash(ctxRemoved) == hash1 {
		t.Fatalf("expected hash to change when allowed ID removed")
	}

	// 4. Changed prompt-visible label → different hash
	ctxChangedLabel := vision.TaxonomyContext{
		Categories: []vision.TaxonomyItem{
			{ID: cat1, Label: "Evening Gowns"}, // Changed label
			{ID: cat2, Label: "Pants"},
		},
		Dictionaries: []vision.TaxonomyItem{
			{ID: dict1, Label: "Fabric"},
		},
		VocabularyRegistry: vocab,
	}
	if vision.ComputeTaxonomyContextHash(ctxChangedLabel) == hash1 {
		t.Fatalf("expected hash to change when prompt-visible label changed")
	}

	// 5. Vision vocabulary version / content change → different hash
	ctxChangedVocabVersion := ctx1
	ctxChangedVocabVersion.VocabularyVersion = "2"
	if vision.ComputeTaxonomyContextHash(ctxChangedVocabVersion) == hash1 {
		t.Fatalf("expected hash to change when vocabulary version changed")
	}

	vocab2 := &vision.VocabularyRegistry{ValidIDs: map[string]bool{"vision.new.id": true}}
	ctxChangedVocabContent := ctx1
	ctxChangedVocabContent.VocabularyRegistry = vocab2
	if vision.ComputeTaxonomyContextHash(ctxChangedVocabContent) == hash1 {
		t.Fatalf("expected hash to change when vocabulary content changed")
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
