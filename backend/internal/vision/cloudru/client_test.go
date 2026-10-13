package cloudru_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision/cloudru"
	"github.com/google/uuid"
)

func TestCloudRuClient(t *testing.T) {
	img1ID := uuid.New()
	img2ID := uuid.New()
	reqData := vision.VisionAnalysisRequest{
		Snapshot: vision.ProductVisionSnapshot{
			Title:       "Test T-Shirt",
			Description: "A nice test shirt",
		},
		Images: []vision.VisionInputImage{
			{ImageID: img1ID, MIMEType: "image/jpeg", Bytes: []byte("first-image-bytes")},
			{ImageID: img2ID, MIMEType: "image/png", Bytes: []byte("second-image-bytes")},
		},
	}

	t.Run("Outbound Request Contract (Model, json_schema, strict, order)", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}
			var bodyMap map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
				t.Fatalf("failed to unmarshal request json: %v", err)
			}

			// 1. Exact model ID
			if bodyMap["model"] != cloudru.DefaultModel {
				t.Errorf("expected model %s, got %v", cloudru.DefaultModel, bodyMap["model"])
			}

			// 2. response_format is schema-constrained, NOT generic legacy mode
			respFormat, ok := bodyMap["response_format"].(map[string]interface{})
			if !ok {
				t.Fatalf("missing response_format map")
			}
			if respFormat["type"] != "json_schema" {
				t.Fatalf("expected response_format.type == 'json_schema', got %v", respFormat["type"])
			}

			// 3. json_schema container
			jsonSchemaObj, ok := respFormat["json_schema"].(map[string]interface{})
			if !ok {
				t.Fatalf("missing json_schema object in response_format")
			}
			if jsonSchemaObj["name"] != vision.CanonicalSchemaName {
				t.Errorf("expected schema name %s, got %v", vision.CanonicalSchemaName, jsonSchemaObj["name"])
			}
			if jsonSchemaObj["strict"] != true {
				t.Errorf("expected strict == true, got %v", jsonSchemaObj["strict"])
			}

			// 4. Schema properties
			schemaMap, ok := jsonSchemaObj["schema"].(map[string]interface{})
			if !ok {
				t.Fatalf("missing schema in json_schema")
			}
			if schemaMap["additionalProperties"] != false {
				t.Errorf("expected root additionalProperties false")
			}
			schemaProps, ok := schemaMap["properties"].(map[string]interface{})
			if !ok {
				t.Fatalf("missing properties in schema")
			}
			if _, hasObs := schemaProps["visualObservation"]; !hasObs {
				t.Errorf("missing visualObservation in schema properties")
			}
			if _, hasMod := schemaProps["moderationEvaluation"]; !hasMod {
				t.Errorf("missing moderationEvaluation in schema properties")
			}

			// 5. Image payload ordering deterministic
			messages := bodyMap["messages"].([]interface{})
			firstMsg := messages[0].(map[string]interface{})
			contents := firstMsg["content"].([]interface{})

			// First content item is prompt text, subsequent items are images
			if len(contents) != 3 {
				t.Fatalf("expected 3 content elements (1 text + 2 images), got %d", len(contents))
			}
			imgElem1 := contents[1].(map[string]interface{})
			imgElem2 := contents[2].(map[string]interface{})

			url1 := imgElem1["image_url"].(map[string]interface{})["url"].(string)
			url2 := imgElem2["image_url"].(map[string]interface{})["url"].(string)

			expectedBase64_1 := base64.StdEncoding.EncodeToString([]byte("first-image-bytes"))
			expectedBase64_2 := base64.StdEncoding.EncodeToString([]byte("second-image-bytes"))

			if !strings.Contains(url1, expectedBase64_1) {
				t.Errorf("first image content out of order or incorrect")
			}
			if !strings.Contains(url2, expectedBase64_2) {
				t.Errorf("second image content out of order or incorrect")
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "{\"visualObservation\":{\"categoryConsistency\":\"consistent\",\"observedCategoryText\":\"t-shirt\",\"patternId\":\"vision.pattern.striped\"},\"moderationEvaluation\":{\"moderationRecommendation\":\"clear\"}}"
					}
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 5,
					"total_tokens": 15
				}
			}`))
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake"})
		res, err := client.AnalyzeProduct(context.Background(), reqData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.VisualObservation.CategoryConsistency != vision.CategoryConsistencyConsistent {
			t.Fatalf("expected categoryConsistency 'consistent', got %v", res.VisualObservation.CategoryConsistency)
		}
		if res.VisualObservation.ObservedCategoryText == nil || *res.VisualObservation.ObservedCategoryText != "t-shirt" {
			t.Fatalf("expected observedCategoryText 't-shirt', got %v", res.VisualObservation.ObservedCategoryText)
		}
		if res.VisualObservation.PatternID == nil || *res.VisualObservation.PatternID != "vision.pattern.striped" {
			t.Fatalf("missing parsed visual observation data")
		}
		if res.Usage == nil || res.Usage.TotalTokens != 15 {
			t.Fatalf("usage metadata not parsed correctly")
		}
	})

	t.Run("Missing Usage Remains Unknown", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices": [{"message": {"content": "{}"}}]}`))
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake"})
		res, err := client.AnalyzeProduct(context.Background(), reqData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Usage != nil {
			t.Fatalf("expected usage to be nil")
		}
	})

	t.Run("Malformed JSON Response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{ bad json }`))
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake"})
		_, err := client.AnalyzeProduct(context.Background(), reqData)
		if err == nil || !strings.Contains(err.Error(), "malformed JSON response") {
			t.Fatalf("expected malformed JSON error, got %v", err)
		}
	})

	t.Run("429 Rate Limit", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`rate limit`))
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake"})
		_, err := client.AnalyzeProduct(context.Background(), reqData)
		if err == nil || !strings.Contains(err.Error(), "rate limit exceeded (429)") {
			t.Fatalf("expected 429 error, got %v", err)
		}
	})

	t.Run("5xx Server Error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`bad gateway`))
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake"})
		_, err := client.AnalyzeProduct(context.Background(), reqData)
		if err == nil || !strings.Contains(err.Error(), "server error (5xx)") {
			t.Fatalf("expected 5xx error, got %v", err)
		}
	})

	t.Run("Non-2xx Generic", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake"})
		_, err := client.AnalyzeProduct(context.Background(), reqData)
		if err == nil || !strings.Contains(err.Error(), "provider error HTTP 400") {
			t.Fatalf("expected 400 error, got %v", err)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake", Timeout: 1 * time.Second})
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // immediately cancel

		_, err := client.AnalyzeProduct(ctx, reqData)
		if err == nil || !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("expected context cancellation error, got %v", err)
		}
	})

	t.Run("Client Timeout", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
		}))
		defer ts.Close()

		client, _ := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake", Timeout: 10 * time.Millisecond})
		_, err := client.AnalyzeProduct(context.Background(), reqData)
		if err == nil || !strings.Contains(err.Error(), "provider timeout") {
			t.Fatalf("expected provider timeout error, got %v", err)
		}
	})
}

func TestMeasureV2ContractSize(t *testing.T) {
	img1ID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	img2ID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	catID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	seasonID := uuid.New()
	finishID := uuid.New()
	dicts := []vision.TaxonomyItem{
		{ID: seasonID, Label: "Весна"},
		{ID: finishID, Label: "Матовая"},
	}

	hexBlack := "#000000"
	var colors []vision.ColorTaxonomyItem
	var colorIDs []uuid.UUID
	for i := 0; i < 122; i++ {
		id := uuid.New()
		colorIDs = append(colorIDs, id)
		colors = append(colors, vision.ColorTaxonomyItem{
			ID:    id,
			Code:  fmt.Sprintf("CLR-%03d", i),
			Label: "Черный",
			Hex:   &hexBlack,
		})
	}

	req := vision.VisionAnalysisRequest{
		Snapshot: vision.ProductVisionSnapshot{
			Title:              "Вечернее шелковое платье",
			Description:        "Элегантное шелковое платье макси",
			DeclaredCategoryID: catID,
			DeclaredCategory:   "Одежда → Женская одежда → Платья",
		},
		Images: []vision.VisionInputImage{
			{ImageID: img1ID, MIMEType: "image/jpeg", Bytes: []byte("img1")},
			{ImageID: img2ID, MIMEType: "image/jpeg", Bytes: []byte("img2")},
		},
		AllowedDictionaryIDs: []uuid.UUID{seasonID, finishID},
		AllowedColorIDs:      colorIDs,
		TaxonomyContext: &vision.TaxonomyContext{
			Dictionaries: dicts,
			Colors:       colors,
		},
	}

	schema := vision.BuildCanonicalJSONSchema(req)
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}

	var interceptedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		interceptedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"visualObservation\":{\"categoryConsistency\":\"consistent\",\"observedCategoryText\":\"dress\"},\"moderationEvaluation\":{\"moderationRecommendation\":\"clear\"}}"
				}
			}]
		}`))
	}))
	defer ts.Close()

	client, err := cloudru.NewClient(cloudru.Config{BaseURL: ts.URL, APIKey: "fake"})
	if err != nil {
		t.Fatalf("client err: %v", err)
	}

	// Make call with empty image bytes to measure request size excluding image base64
	reqEmptyImages := req
	reqEmptyImages.Images = []vision.VisionInputImage{
		{ImageID: img1ID, MIMEType: "image/jpeg", Bytes: []byte{}},
		{ImageID: img2ID, MIMEType: "image/jpeg", Bytes: []byte{}},
	}
	_, err = client.AnalyzeProduct(context.Background(), reqEmptyImages)
	if err != nil {
		t.Fatalf("analyze err: %v", err)
	}

	// Extract prompt text from intercepted request
	var bodyMap map[string]interface{}
	_ = json.Unmarshal(interceptedBody, &bodyMap)
	messages := bodyMap["messages"].([]interface{})
	firstMsg := messages[0].(map[string]interface{})
	contents := firstMsg["content"].([]interface{})
	promptText := contents[0].(map[string]interface{})["text"].(string)

	// Verify prompt contains canonical color mapping
	if !strings.Contains(promptText, "Available Canonical Colors:") {
		t.Fatalf("expected prompt to contain Available Canonical Colors section")
	}
	sampleColorLine := fmt.Sprintf("- %s: Черный (#000000)", colors[0].ID.String())
	if !strings.Contains(promptText, sampleColorLine) {
		t.Fatalf("expected prompt to contain canonical color line %q", sampleColorLine)
	}

	// Verify prompt excludes season/finish dictionary IDs as colors
	if strings.Contains(promptText, seasonID.String()) {
		t.Fatalf("season dictionary ID %s must NOT appear in prompt", seasonID)
	}
	if strings.Contains(promptText, finishID.String()) {
		t.Fatalf("finish dictionary ID %s must NOT appear in prompt", finishID)
	}

	t.Logf("V2_COLOR_COUNT=%d", len(colors))
	t.Logf("V2_SCHEMA_SIZE_BYTES=%d", len(schemaBytes))
	t.Logf("V2_PROMPT_SIZE_BYTES=%d", len([]byte(promptText)))
	t.Logf("V2_REQUEST_SIZE_EXCLUDING_BASE64=%d", len(interceptedBody))
	t.Logf("V2_IMAGE_ENUM_COUNT=%d", len(req.Images))
}
