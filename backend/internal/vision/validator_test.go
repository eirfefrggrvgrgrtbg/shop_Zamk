package vision_test

import (
	"encoding/json"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
	"github.com/google/uuid"
)

func TestTaxonomyContextHash(t *testing.T) {
	cat1 := uuid.New()
	cat2 := uuid.New()
	dict1 := uuid.New()
	col1 := uuid.New()
	col2 := uuid.New()
	hexBlack := "#000000"
	hexWhite := "#FFFFFF"
	vocab := vision.NewVocabularyRegistry()

	ctx1 := vision.TaxonomyContext{
		Categories: []vision.TaxonomyItem{
			{ID: cat1, Label: "Dresses"},
			{ID: cat2, Label: "Pants"},
		},
		Dictionaries: []vision.TaxonomyItem{
			{ID: dict1, Label: "Fabric"},
		},
		Colors: []vision.ColorTaxonomyItem{
			{ID: col1, Code: "BLK", Label: "Черный", Hex: &hexBlack},
			{ID: col2, Code: "WHT", Label: "Белый", Hex: &hexWhite},
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
		Colors: []vision.ColorTaxonomyItem{
			{ID: col2, Code: "WHT", Label: "Белый", Hex: &hexWhite},
			{ID: col1, Code: "BLK", Label: "Черный", Hex: &hexBlack},
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
		Colors: []vision.ColorTaxonomyItem{
			{ID: col1, Code: "BLK", Label: "Черный", Hex: &hexBlack},
			{ID: col2, Code: "WHT", Label: "Белый", Hex: &hexWhite},
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
		Colors: []vision.ColorTaxonomyItem{
			{ID: col1, Code: "BLK", Label: "Черный", Hex: &hexBlack},
			{ID: col2, Code: "WHT", Label: "Белый", Hex: &hexWhite},
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
		Colors: []vision.ColorTaxonomyItem{
			{ID: col1, Code: "BLK", Label: "Черный", Hex: &hexBlack},
			{ID: col2, Code: "WHT", Label: "Белый", Hex: &hexWhite},
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

	// 6. Changed Color in TaxonomyContext → different hash
	col3 := uuid.New()
	ctxAddedColor := ctx1
	ctxAddedColor.Colors = append(ctxAddedColor.Colors, vision.ColorTaxonomyItem{ID: col3, Code: "RED", Label: "Красный"})
	if vision.ComputeTaxonomyContextHash(ctxAddedColor) == hash1 {
		t.Fatalf("expected hash to change when color is added")
	}

	ctxChangedColorLabel := ctx1
	ctxChangedColorLabel.Colors = []vision.ColorTaxonomyItem{
		{ID: col1, Code: "BLK", Label: "Антрацит", Hex: &hexBlack},
		{ID: col2, Code: "WHT", Label: "Белый", Hex: &hexWhite},
	}
	if vision.ComputeTaxonomyContextHash(ctxChangedColorLabel) == hash1 {
		t.Fatalf("expected hash to change when color label is changed")
	}
}

func TestValidateAnalysisResult(t *testing.T) {
	catID := uuid.New()
	seasonDictID := uuid.New()
	finishDictID := uuid.New()
	color1ID := uuid.New()
	color2ID := uuid.New()
	imgID := uuid.New()
	hexBlack := "#000000"
	hexRed := "#FF0000"
	vocab := vision.NewVocabularyRegistry()

	req := vision.VisionAnalysisRequest{
		AllowedCategoryIDs:   []uuid.UUID{catID},
		AllowedDictionaryIDs: []uuid.UUID{seasonDictID, finishDictID},
		AllowedColorIDs:      []uuid.UUID{color1ID, color2ID},
		TaxonomyContext: &vision.TaxonomyContext{
			Categories: []vision.TaxonomyItem{{ID: catID, Label: "Платья"}},
			Dictionaries: []vision.TaxonomyItem{
				{ID: seasonDictID, Label: "Весна"},
				{ID: finishDictID, Label: "Матовая"},
			},
			Colors: []vision.ColorTaxonomyItem{
				{ID: color1ID, Code: "BLK", Label: "Черный", Hex: &hexBlack},
				{ID: color2ID, Code: "RED", Label: "Красный", Hex: &hexRed},
			},
		},
		Images: []vision.VisionInputImage{
			{ImageID: imgID},
		},
	}

	ptr := func(s string) *string { return &s }

	t.Run("Valid Response", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency:  vision.CategoryConsistencyConsistent,
				ObservedCategoryText: ptr("striped dress"),
				DominantColorIDs:      []uuid.UUID{color1ID},
				SecondaryColorIDs:     []uuid.UUID{color2ID},
				PatternID:             ptr("vision.pattern.striped"),
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

	t.Run("Valid Consistency Enums (contradictory, uncertain)", func(t *testing.T) {
		for _, consistency := range []vision.CategoryConsistency{vision.CategoryConsistencyContradictory, vision.CategoryConsistencyUncertain} {
			res := &vision.VisionAnalysisResult{
				VisualObservation: vision.ProductVisualObservationV2{
					CategoryConsistency:  consistency,
					ObservedCategoryText: ptr("hoodie"),
				},
				ModerationEvaluation: vision.ModerationEvaluationV1{
					ModerationRecommendation: "clear",
				},
			}
			if err := vision.ValidateAnalysisResult(req, res, vocab); err != nil {
				t.Fatalf("expected %s to be valid, got %v", consistency, err)
			}
		}
	})

	t.Run("Pseudo-Null ObservedCategoryText Rejected", func(t *testing.T) {
		pseudoNulls := []string{"null", "NULL", "none", "unknown", "n/a", "N/A", "", "   "}
		for _, pn := range pseudoNulls {
			res := &vision.VisionAnalysisResult{
				VisualObservation: vision.ProductVisualObservationV2{
					CategoryConsistency:  vision.CategoryConsistencyConsistent,
					ObservedCategoryText: ptr(pn),
				},
				ModerationEvaluation: vision.ModerationEvaluationV1{
					ModerationRecommendation: "clear",
				},
			}
			err := vision.ValidateAnalysisResult(req, res, vocab)
			if err == nil || !errorsIsInvalidEnum(err) {
				t.Fatalf("expected ErrInvalidEnum for pseudo-null %q, got %v", pn, err)
			}
		}
	})

	t.Run("Canonical Color UUID Accepted", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
				DominantColorIDs:    []uuid.UUID{color1ID},
				SecondaryColorIDs:   []uuid.UUID{color2ID},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err != nil {
			t.Fatalf("expected canonical color UUIDs to be accepted, got %v", err)
		}
	})

	t.Run("Season Dictionary UUID Rejected as Color", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
				DominantColorIDs:    []uuid.UUID{seasonDictID},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsUnknownTaxonomy(err) {
			t.Fatalf("expected ErrUnknownTaxonomyID when season dictionary UUID is passed as color, got %v", err)
		}
	})

	t.Run("Finish Dictionary UUID Rejected as Color", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
				DominantColorIDs:    []uuid.UUID{finishDictID},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsUnknownTaxonomy(err) {
			t.Fatalf("expected ErrUnknownTaxonomyID when finish dictionary UUID is passed as color, got %v", err)
		}
	})

	t.Run("Random UUID Rejected as Color", func(t *testing.T) {
		randomID := uuid.New()
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
				DominantColorIDs:    []uuid.UUID{randomID},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsUnknownTaxonomy(err) {
			t.Fatalf("expected ErrUnknownTaxonomyID for random UUID, got %v", err)
		}
	})

	t.Run("Empty Color Arrays Accepted as Abstention", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
				DominantColorIDs:    []uuid.UUID{},
				SecondaryColorIDs:   []uuid.UUID{},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err != nil {
			t.Fatalf("expected empty color arrays to be accepted as abstention, got %v", err)
		}
	})

	t.Run("Vision String Color Token Rejected on Deserialization", func(t *testing.T) {
		rawJSON := `{"visualObservation":{"categoryConsistency":"consistent","dominantColorIds":["vision.color.black"]}}`
		var res vision.VisionAnalysisResult
		err := json.Unmarshal([]byte(rawJSON), &res)
		if err == nil {
			t.Fatalf("expected unmarshal error when string token is passed to uuid.UUID color field, got nil")
		}
	})

	t.Run("Conflicting Observation (Dominant and Secondary Color overlap)", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
				DominantColorIDs:     []uuid.UUID{color1ID},
				SecondaryColorIDs:    []uuid.UUID{color1ID},
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsConflicting(err) {
			t.Fatalf("expected ErrConflictingData, got %v", err)
		}
	})

	t.Run("Invalid Moderation Enum", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "approve", // not allowed
			},
		}
		err := vision.ValidateAnalysisResult(req, res, vocab)
		if err == nil || !errorsIsInvalidEnum(err) {
			t.Fatalf("expected ErrInvalidEnum, got %v", err)
		}
	})

	t.Run("Invalid Category Consistency Enum", func(t *testing.T) {
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistency("invalid_consistency"),
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
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
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency:  vision.CategoryConsistencyUncertain,
				ObservedCategoryText: nil,
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
		res := &vision.VisionAnalysisResult{
			VisualObservation: vision.ProductVisualObservationV2{
				CategoryConsistency: vision.CategoryConsistencyConsistent,
				PatternID:           ptr("vision.pattern.striped"),
			},
			ModerationEvaluation: vision.ModerationEvaluationV1{
				ModerationRecommendation: "clear",
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
