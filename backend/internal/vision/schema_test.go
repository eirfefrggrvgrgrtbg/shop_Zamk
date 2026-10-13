package vision_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

func TestCanonicalJSONSchemaStructure(t *testing.T) {
	if vision.CanonicalSchemaName != "vision_product_analysis_v2" {
		t.Fatalf("expected CanonicalSchemaName 'vision_product_analysis_v2', got %q", vision.CanonicalSchemaName)
	}

	schema := vision.CanonicalJSONSchema()

	// 1. Root structure
	if schema["type"] != "object" {
		t.Fatalf("expected root schema type 'object', got %v", schema["type"])
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("expected root schema additionalProperties false")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected root schema required list of strings")
	}
	hasObs, hasMod := false, false
	for _, req := range required {
		if req == "visualObservation" {
			hasObs = true
		}
		if req == "moderationEvaluation" {
			hasMod = true
		}
	}
	if !hasObs || !hasMod {
		t.Fatalf("root required missing visualObservation or moderationEvaluation: %v", required)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected root properties map")
	}

	// 2. VisualObservation structure
	obsSchema, ok := props["visualObservation"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected visualObservation schema map")
	}
	if obsSchema["additionalProperties"] != false {
		t.Fatalf("expected visualObservation additionalProperties false")
	}
	obsProps, ok := obsSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected visualObservation properties map")
	}

	// Verify observedCategoryId is REMOVED
	if _, exists := obsProps["observedCategoryId"]; exists {
		t.Fatalf("observedCategoryId must NOT exist in V2 schema")
	}

	// Verify categoryConsistency
	consistencySchema, ok := obsProps["categoryConsistency"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing categoryConsistency property")
	}
	if consistencySchema["type"] != "string" {
		t.Fatalf("expected categoryConsistency type string, got %v", consistencySchema["type"])
	}
	consistencyEnums, ok := consistencySchema["enum"].([]string)
	if !ok || len(consistencyEnums) != 3 {
		t.Fatalf("expected 3 enum values for categoryConsistency, got %v", consistencyEnums)
	}

	// Verify observedCategoryText nullable
	obsCatTextSchema, ok := obsProps["observedCategoryText"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing observedCategoryText property")
	}
	anyOfCatText, ok := obsCatTextSchema["anyOf"].([]interface{})
	if !ok || len(anyOfCatText) < 2 {
		t.Fatalf("expected observedCategoryText anyOf with string and null")
	}

	// Verify closed enum in silhouetteId
	silSchema, ok := obsProps["silhouetteId"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing silhouetteId property")
	}
	anyOfSil, ok := silSchema["anyOf"].([]interface{})
	if !ok || len(anyOfSil) < 2 {
		t.Fatalf("expected silhouetteId anyOf with null")
	}
	strSilObj := anyOfSil[0].(map[string]interface{})
	silEnums := strSilObj["enum"].([]string)
	if len(silEnums) != len(vision.VocabularySilhouettes) {
		t.Fatalf("expected closed silhouette enum matching vocabulary: %v", silEnums)
	}

	// Verify confidence score schema
	confSchema, ok := obsProps["confidence"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing confidence property")
	}
	if confSchema["additionalProperties"] != false {
		t.Fatalf("expected confidence additionalProperties false")
	}
	confProps := confSchema["properties"].(map[string]interface{})
	catConf := confProps["category"].(map[string]interface{})
	catConfProps := catConf["properties"].(map[string]interface{})
	levelProp := catConfProps["level"].(map[string]interface{})
	levels := levelProp["enum"].([]string)
	hasAbstained := false
	for _, l := range levels {
		if l == "abstained" {
			hasAbstained = true
		}
	}
	if !hasAbstained {
		t.Fatalf("expected confidence level enum to include 'abstained'")
	}
	scoreProp := catConfProps["score"].(map[string]interface{})
	if scoreProp["minimum"] != 0.0 || scoreProp["maximum"] != 1.0 {
		t.Fatalf("expected score bounds 0.0 to 1.0, got min %v, max %v", scoreProp["minimum"], scoreProp["maximum"])
	}

	// Verify evidence item schema
	evSchema, ok := obsProps["evidence"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing evidence property")
	}
	evItem := evSchema["items"].(map[string]interface{})
	if evItem["additionalProperties"] != false {
		t.Fatalf("expected evidence item additionalProperties false")
	}
	evRequired := evItem["required"].([]string)
	if len(evRequired) != 3 {
		t.Fatalf("expected evidence required: field, valueId, imageIds, got: %v", evRequired)
	}

	// 3. ModerationEvaluation structure
	modSchema, ok := props["moderationEvaluation"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected moderationEvaluation schema map")
	}
	if modSchema["additionalProperties"] != false {
		t.Fatalf("expected moderationEvaluation additionalProperties false")
	}
	modProps, ok := modSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected moderationEvaluation properties map")
	}

	// Verify closed enum in moderationRecommendation
	recSchema := modProps["moderationRecommendation"].(map[string]interface{})
	recEnums := recSchema["enum"].([]string)
	if len(recEnums) != 3 {
		t.Fatalf("expected clear, review_recommended, uncertain in recommendation: %v", recEnums)
	}

	// Verify contradictions schema
	contraSchema := modProps["contradictions"].(map[string]interface{})
	contraItem := contraSchema["items"].(map[string]interface{})
	if contraItem["additionalProperties"] != false {
		t.Fatalf("expected contradiction item additionalProperties false")
	}
	contraRequired := contraItem["required"].([]string)
	if len(contraRequired) != 4 {
		t.Fatalf("expected contradiction item required: field, declared, observed, reason")
	}
}

func TestCanonicalJSONSchemaDynamicBinding(t *testing.T) {
	img1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	img2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	colorID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	seasonDictID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	finishDictID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	arbitraryID := uuid.MustParse("66666666-6666-6666-6666-666666666666")

	req := vision.VisionAnalysisRequest{
		Images: []vision.VisionInputImage{
			{ImageID: img2},
			{ImageID: img1},
		},
		AllowedDictionaryIDs: []uuid.UUID{seasonDictID, finishDictID},
		AllowedColorIDs:      []uuid.UUID{colorID},
		TaxonomyContext: &vision.TaxonomyContext{
			Dictionaries: []vision.TaxonomyItem{
				{ID: seasonDictID, Label: "Весна"},
				{ID: finishDictID, Label: "Матовая"},
			},
			Colors: []vision.ColorTaxonomyItem{
				{ID: colorID, Code: "BLK", Label: "Черный"},
			},
		},
	}

	schema := vision.BuildCanonicalJSONSchema(req)
	props := schema["properties"].(map[string]interface{})
	obsProps := props["visualObservation"].(map[string]interface{})["properties"].(map[string]interface{})

	// 1. Check evidence imageIds enum is bound to sorted img1, img2
	evSchema := obsProps["evidence"].(map[string]interface{})
	evItemProps := evSchema["items"].(map[string]interface{})["properties"].(map[string]interface{})
	imgIdsSchema := evItemProps["imageIds"].(map[string]interface{})
	imgIdItem := imgIdsSchema["items"].(map[string]interface{})
	imgEnums, ok := imgIdItem["enum"].([]string)
	if !ok || len(imgEnums) != 2 {
		t.Fatalf("expected 2 image UUIDs in enum, got %v", imgEnums)
	}
	if imgEnums[0] != img1.String() || imgEnums[1] != img2.String() {
		t.Fatalf("expected sorted image UUIDs in enum, got %v", imgEnums)
	}

	// 2. Check dominantColorIds and secondaryColorIds enum contains colorID and excludes season/finish/arbitrary IDs
	for _, field := range []string{"dominantColorIds", "secondaryColorIds"} {
		fieldSchema := obsProps[field].(map[string]interface{})
		fieldItems := fieldSchema["items"].(map[string]interface{})
		colorEnums, ok := fieldItems["enum"].([]string)
		if !ok {
			t.Fatalf("expected color enum list in %s", field)
		}
		if len(colorEnums) != 1 || colorEnums[0] != colorID.String() {
			t.Fatalf("expected only colorID in %s enum, got: %v", field, colorEnums)
		}

		for _, c := range colorEnums {
			if c == seasonDictID.String() {
				t.Fatalf("season dictionary UUID %s must NOT be in %s enum", seasonDictID, field)
			}
			if c == finishDictID.String() {
				t.Fatalf("finish dictionary UUID %s must NOT be in %s enum", finishDictID, field)
			}
			if c == arbitraryID.String() {
				t.Fatalf("arbitrary UUID %s must NOT be in %s enum", arbitraryID, field)
			}
		}
	}
}
