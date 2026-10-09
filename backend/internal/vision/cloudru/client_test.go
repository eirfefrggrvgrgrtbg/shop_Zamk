package cloudru_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
						"content": "{\"visualObservation\":{\"patternId\":\"vision.pattern.striped\"},\"moderationEvaluation\":{\"moderationRecommendation\":\"clear\"}}"
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
