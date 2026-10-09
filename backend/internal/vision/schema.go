package vision

const (
	CanonicalSchemaName = "vision_product_analysis_v1"
)

// CanonicalJSONSchema produces the provider-native strict JSON schema
// describing the compound Vision output (ProductVisualObservationV1 + ModerationEvaluationV1).
func CanonicalJSONSchema() map[string]interface{} {
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
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 36,
				},
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
			"observedCategoryId": map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{"type": "string", "maxLength": 36},
					map[string]interface{}{"type": "null"},
				},
			},
			"dominantColorIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 64,
				},
				"maxItems": 10,
			},
			"secondaryColorIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"maxLength": 64,
				},
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
			"observedCategoryId",
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
