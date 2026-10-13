package vision

import (
	"sort"

	"github.com/google/uuid"
)

const (
	CanonicalSchemaNameV1 = "vision_product_analysis_v1"
	CanonicalSchemaNameV2 = "vision_product_analysis_v2"

	CanonicalSchemaName = CanonicalSchemaNameV2
)

// CanonicalJSONSchema produces the baseline strict JSON schema V2 for static callers.
func CanonicalJSONSchema() map[string]interface{} {
	return BuildCanonicalJSONSchema(VisionAnalysisRequest{})
}

// BuildCanonicalJSONSchema produces the provider-native strict JSON schema V2
// describing the compound Vision output (ProductVisualObservationV2 + ModerationEvaluationV1),
// dynamically bound to the immutable runtime inputs (snapshot images, dictionary colors).
func BuildCanonicalJSONSchema(req VisionAnalysisRequest) map[string]interface{} {
	// Extract and sort snapshot image IDs for evidence binding
	imageIDs := make([]string, 0, len(req.Images))
	for _, img := range req.Images {
		imageIDs = append(imageIDs, img.ImageID.String())
	}
	if len(imageIDs) == 0 {
		for _, img := range req.Snapshot.Images {
			imageIDs = append(imageIDs, img.ImageID.String())
		}
	}
	sort.Strings(imageIDs)

	var evidenceImageItemSchema map[string]interface{}
	if len(imageIDs) > 0 {
		evidenceImageItemSchema = map[string]interface{}{
			"type": "string",
			"enum": imageIDs,
		}
	} else {
		evidenceImageItemSchema = map[string]interface{}{
			"type":      "string",
			"maxLength": 36,
		}
	}

	// Extract and sort canonical color UUIDs for color binding
	var colorUUIDs []uuid.UUID
	if req.TaxonomyContext != nil && len(req.TaxonomyContext.Colors) > 0 {
		colorUUIDs = make([]uuid.UUID, len(req.TaxonomyContext.Colors))
		for i, c := range req.TaxonomyContext.Colors {
			colorUUIDs[i] = c.ID
		}
	} else if len(req.AllowedColorIDs) > 0 {
		colorUUIDs = req.AllowedColorIDs
	}

	colorIDs := make([]string, 0, len(colorUUIDs))
	for _, id := range colorUUIDs {
		colorIDs = append(colorIDs, id.String())
	}
	sort.Strings(colorIDs)

	var colorItemSchema map[string]interface{}
	if len(colorIDs) > 0 {
		colorItemSchema = map[string]interface{}{
			"type": "string",
			"enum": colorIDs,
		}
	} else {
		colorItemSchema = map[string]interface{}{
			"type":      "string",
			"maxLength": 36,
		}
	}

	confidenceScoreSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"level": map[string]interface{}{
				"type": "string",
				"enum": []string{"high", "medium", "low", "abstained"},
			},
			"score": map[string]interface{}{
				"type":    "number",
				"minimum": 0.0,
				"maximum": 1.0,
			},
		},
		"required":             []string{"level", "score"},
		"additionalProperties": false,
	}

	evidenceItemSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"field": map[string]interface{}{
				"type": "string",
				"enum": []string{
					"category",
					"dominantColors",
					"secondaryColors",
					"silhouette",
					"pattern",
					"texture",
					"details",
					"ocrText",
				},
			},
			"valueId": map[string]interface{}{
				"type":      "string",
				"maxLength": 64,
			},
			"imageIds": map[string]interface{}{
				"type":     "array",
				"items":    evidenceImageItemSchema,
				"maxItems": 10,
			},
		},
		"required":             []string{"field", "valueId", "imageIds"},
		"additionalProperties": false,
	}

	contradictionItemSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"field": map[string]interface{}{
				"type":      "string",
				"maxLength": 64,
			},
			"declared": map[string]interface{}{
				"type":      "string",
				"maxLength": 200,
			},
			"observed": map[string]interface{}{
				"type":      "string",
				"maxLength": 200,
			},
			"reason": map[string]interface{}{
				"type":      "string",
				"maxLength": 500,
			},
		},
		"required":             []string{"field", "declared", "observed", "reason"},
		"additionalProperties": false,
	}

	visualObservationSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"categoryConsistency": map[string]interface{}{
				"type": "string",
				"enum": []string{"consistent", "contradictory", "uncertain"},
			},
			"observedCategoryText": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"type":      "string",
						"maxLength": 100,
					},
					map[string]interface{}{"type": "null"},
				},
			},
			"dominantColorIds": map[string]interface{}{
				"type":     "array",
				"items":    colorItemSchema,
				"maxItems": 10,
			},
			"secondaryColorIds": map[string]interface{}{
				"type":     "array",
				"items":    colorItemSchema,
				"maxItems": 10,
			},
			"silhouetteId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"type": "string",
						"enum": VocabularySilhouettes,
					},
					map[string]interface{}{"type": "null"},
				},
			},
			"fitAppearanceId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{"type": "string", "maxLength": 64},
					map[string]interface{}{"type": "null"},
				},
			},
			"lengthId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{"type": "string", "maxLength": 64},
					map[string]interface{}{"type": "null"},
				},
			},
			"sleeveIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 64,
				},
				"maxItems": 10,
			},
			"necklineId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{"type": "string", "maxLength": 64},
					map[string]interface{}{"type": "null"},
				},
			},
			"closureId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{"type": "string", "maxLength": 64},
					map[string]interface{}{"type": "null"},
				},
			},
			"patternId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"type": "string",
						"enum": VocabularyPatterns,
					},
					map[string]interface{}{"type": "null"},
				},
			},
			"visibleTextureId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"type": "string",
						"enum": VocabularyTextures,
					},
					map[string]interface{}{"type": "null"},
				},
			},
			"decorativeDetailIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 64,
				},
				"maxItems": 20,
			},
			"styleDescriptorIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 64,
				},
				"maxItems": 20,
			},
			"formalityId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{"type": "string", "maxLength": 64},
					map[string]interface{}{"type": "null"},
				},
			},
			"seasonalCueIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 64,
				},
				"maxItems": 10,
			},
			"imageSetConsistency": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{"type": "boolean"},
					map[string]interface{}{"type": "null"},
				},
			},
			"ocrText": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 200,
				},
				"maxItems": 50,
			},
			"confidence": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"category":   confidenceScoreSchema,
					"silhouette": confidenceScoreSchema,
					"pattern":    confidenceScoreSchema,
					"texture":    confidenceScoreSchema,
				},
				"required":             []string{"category", "silhouette", "pattern", "texture"},
				"additionalProperties": false,
			},
			"evidence": map[string]interface{}{
				"type":     "array",
				"items":    evidenceItemSchema,
				"maxItems": 20,
			},
		},
		"required": []string{
			"categoryConsistency",
			"observedCategoryText",
			"dominantColorIds",
			"secondaryColorIds",
			"silhouetteId",
			"fitAppearanceId",
			"lengthId",
			"sleeveIds",
			"necklineId",
			"closureId",
			"patternId",
			"visibleTextureId",
			"decorativeDetailIds",
			"styleDescriptorIds",
			"formalityId",
			"seasonalCueIds",
			"imageSetConsistency",
			"ocrText",
			"confidence",
			"evidence",
		},
		"additionalProperties": false,
	}

	moderationEvaluationSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"contradictions": map[string]interface{}{
				"type":     "array",
				"items":    contradictionItemSchema,
				"maxItems": 20,
			},
			"qualityIssues": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": VocabularyQualityIssues,
				},
				"maxItems": 10,
			},
			"ocrFindings": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 200,
				},
				"maxItems": 50,
			},
			"moderationRecommendation": map[string]interface{}{
				"type": "string",
				"enum": []string{"clear", "review_recommended", "uncertain"},
			},
			"recommendationReason": map[string]interface{}{
				"type":      "string",
				"maxLength": 500,
			},
		},
		"required": []string{
			"contradictions",
			"qualityIssues",
			"ocrFindings",
			"moderationRecommendation",
			"recommendationReason",
		},
		"additionalProperties": false,
	}

	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"visualObservation":    visualObservationSchema,
			"moderationEvaluation": moderationEvaluationSchema,
		},
		"required": []string{
			"visualObservation",
			"moderationEvaluation",
		},
		"additionalProperties": false,
	}
}
