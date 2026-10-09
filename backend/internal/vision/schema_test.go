package vision_test

import (
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

func TestCanonicalJSONSchemaStructure(t *testing.T) {
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
