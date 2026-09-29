package personalization_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
)

func makeDummyVector(dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32(i) * 0.001
	}
	return v
}

func makeEmbeddingJSON(vec []float32) string {
	type embData struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	}
	type resp struct {
		Object string    `json:"object"`
		Data   []embData `json:"data"`
		Model  string    `json:"model"`
	}
	r := resp{
		Object: "list",
		Data: []embData{
			{Object: "embedding", Index: 0, Embedding: vec},
		},
		Model: "text-embedding-3-small",
	}
	bytes, _ := json.Marshal(r)
	return string(bytes)
}

func TestOpenAIEmbeddingClient(t *testing.T) {
	const secretKey = "test-secret-key-xyz-98765"

	// A, B, C, D, E: Valid response, request contract and authorization verification
	t.Run("ValidRequestAndResponse_A_B_C_D_E", func(t *testing.T) {
		var receivedAuthHeader string
		var receivedModel string
		var receivedDimensions int
		var receivedEncodingFormat string
		var receivedInput string

		expectedInput := "schema_version=1\ntitle=Test Product\ncategory_path=Apparel"

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuthHeader = r.Header.Get("Authorization")
			bodyBytes, _ := io.ReadAll(r.Body)

			var req struct {
				Model          string `json:"model"`
				Input          string `json:"input"`
				Dimensions     int    `json:"dimensions"`
				EncodingFormat string `json:"encoding_format"`
			}
			_ = json.Unmarshal(bodyBytes, &req)
			receivedModel = req.Model
			receivedDimensions = req.Dimensions
			receivedEncodingFormat = req.EncodingFormat
			receivedInput = req.Input

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, makeEmbeddingJSON(makeDummyVector(1536)))
		}))
		defer srv.Close()

		client, err := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:  secretKey,
			BaseURL: srv.URL,
		})
		if err != nil {
			t.Fatalf("unexpected client creation error: %v", err)
		}

		ctx := context.Background()
		vec, err := client.Embed(ctx, expectedInput)
		if err != nil {
			t.Fatalf("unexpected Embed error: %v", err)
		}

		// A. valid response -> vector length 1536
		if len(vec) != 1536 {
			t.Fatalf("Test A failed: expected vector length 1536, got %d", len(vec))
		}

		// B. request model = text-embedding-3-small
		if receivedModel != "text-embedding-3-small" {
			t.Fatalf("Test B failed: expected model 'text-embedding-3-small', got %q", receivedModel)
		}

		// C. request dimensions = 1536, encoding_format = float
		if receivedDimensions != 1536 || receivedEncodingFormat != "float" {
			t.Fatalf("Test C failed: expected dimensions 1536 and float encoding, got %d and %q", receivedDimensions, receivedEncodingFormat)
		}

		// D. request input exact canonical text bytes
		if receivedInput != expectedInput {
			t.Fatalf("Test D failed: expected input %q, got %q", expectedInput, receivedInput)
		}

		// E. Authorization header uses configured key (verify match without printing key to logs)
		expectedAuth := "Bearer " + secretKey
		if receivedAuthHeader != expectedAuth {
			t.Fatalf("Test E failed: Authorization header mismatch (header present: %v)", receivedAuthHeader != "")
		}
	})

	// F. missing key -> config error
	t.Run("MissingAPIKey_F", func(t *testing.T) {
		_, err := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey: "",
		})
		if !errors.Is(err, personalization.ErrOpenAIMissingAPIKey) {
			t.Fatalf("Test F failed: expected ErrOpenAIMissingAPIKey, got %v", err)
		}
	})

	// G. 400 -> no retry
	t.Run("Status400_NoRetry_G", func(t *testing.T) {
		var requestCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Bad Request"}}`)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    srv.URL,
			MaxRetries: 3,
			Backoff:    5 * time.Millisecond,
		})

		_, err := client.Embed(context.Background(), "test")
		if err == nil {
			t.Fatalf("Test G failed: expected error on 400, got nil")
		}
		if count := atomic.LoadInt32(&requestCount); count != 1 {
			t.Fatalf("Test G failed: expected exactly 1 attempt (no retry), got %d", count)
		}
	})

	// H. 401 -> no retry
	t.Run("Status401_NoRetry_H", func(t *testing.T) {
		var requestCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"Unauthorized key"}}`)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    srv.URL,
			MaxRetries: 3,
			Backoff:    5 * time.Millisecond,
		})

		_, err := client.Embed(context.Background(), "test")
		if err == nil {
			t.Fatalf("Test H failed: expected error on 401, got nil")
		}
		if count := atomic.LoadInt32(&requestCount); count != 1 {
			t.Fatalf("Test H failed: expected exactly 1 attempt (no retry), got %d", count)
		}
	})

	// I. 429 -> bounded retry then success
	t.Run("Status429_RetryThenSuccess_I", func(t *testing.T) {
		var requestCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&requestCount, 1)
			if count < 3 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"error":{"message":"Rate limit reached"}}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, makeEmbeddingJSON(makeDummyVector(1536)))
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    srv.URL,
			MaxRetries: 4,
			Backoff:    5 * time.Millisecond,
		})

		vec, err := client.Embed(context.Background(), "test")
		if err != nil {
			t.Fatalf("Test I failed: expected success after retrying 429, got %v", err)
		}
		if len(vec) != 1536 {
			t.Fatalf("Test I failed: expected vector length 1536, got %d", len(vec))
		}
		if count := atomic.LoadInt32(&requestCount); count != 3 {
			t.Fatalf("Test I failed: expected exactly 3 requests, got %d", count)
		}
	})

	// J. 500 -> bounded retry then success
	t.Run("Status500_RetryThenSuccess_J", func(t *testing.T) {
		var requestCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&requestCount, 1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"error":{"message":"Internal server error"}}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, makeEmbeddingJSON(makeDummyVector(1536)))
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    srv.URL,
			MaxRetries: 3,
			Backoff:    5 * time.Millisecond,
		})

		vec, err := client.Embed(context.Background(), "test")
		if err != nil {
			t.Fatalf("Test J failed: expected success after retrying 500, got %v", err)
		}
		if len(vec) != 1536 {
			t.Fatalf("Test J failed: expected vector length 1536, got %d", len(vec))
		}
		if count := atomic.LoadInt32(&requestCount); count != 2 {
			t.Fatalf("Test J failed: expected exactly 2 requests, got %d", count)
		}
	})

	// K. 503 exhausted -> error
	t.Run("Status503_Exhausted_K", func(t *testing.T) {
		var requestCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":{"message":"Service Unavailable"}}`)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    srv.URL,
			MaxRetries: 3,
			Backoff:    5 * time.Millisecond,
		})

		_, err := client.Embed(context.Background(), "test")
		if err == nil {
			t.Fatalf("Test K failed: expected error after exhausted 503 retries, got nil")
		}
		if count := atomic.LoadInt32(&requestCount); count != 3 {
			t.Fatalf("Test K failed: expected exactly 3 retry attempts, got %d", count)
		}
	})

	// L. malformed JSON -> error
	t.Run("MalformedJSON_L", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{broken json response`)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:  secretKey,
			BaseURL: srv.URL,
		})

		_, err := client.Embed(context.Background(), "test")
		if !errors.Is(err, personalization.ErrOpenAIMalformedResponse) {
			t.Fatalf("Test L failed: expected ErrOpenAIMalformedResponse, got %v", err)
		}
	})

	// M. empty data -> error
	t.Run("EmptyData_M", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"object":"list","data":[]}`)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:  secretKey,
			BaseURL: srv.URL,
		})

		_, err := client.Embed(context.Background(), "test")
		if !errors.Is(err, personalization.ErrOpenAIEmptyData) {
			t.Fatalf("Test M failed: expected ErrOpenAIEmptyData, got %v", err)
		}
	})

	// N. multiple embeddings unexpected -> error
	t.Run("MultipleEmbeddings_N", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			raw := fmt.Sprintf(`{"object":"list","data":[{"index":0,"embedding":%s},{"index":1,"embedding":%s}]}`,
				vecToJSON(makeDummyVector(1536)), vecToJSON(makeDummyVector(1536)))
			_, _ = io.WriteString(w, raw)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:  secretKey,
			BaseURL: srv.URL,
		})

		_, err := client.Embed(context.Background(), "test")
		if !errors.Is(err, personalization.ErrOpenAIMultipleEmbeddings) {
			t.Fatalf("Test N failed: expected ErrOpenAIMultipleEmbeddings, got %v", err)
		}
	})

	// O. vector length != 1536 -> error
	t.Run("WrongDimensions_O", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, makeEmbeddingJSON(makeDummyVector(512)))
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:  secretKey,
			BaseURL: srv.URL,
		})

		_, err := client.Embed(context.Background(), "test")
		if !errors.Is(err, personalization.ErrOpenAIInvalidDimensions) {
			t.Fatalf("Test O failed: expected ErrOpenAIInvalidDimensions, got %v", err)
		}
	})

	// P. NaN/Inf -> error
	t.Run("NonFiniteVector_P", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("[1e39")
		for i := 1; i < 1536; i++ {
			sb.WriteString(",0.0")
		}
		sb.WriteString("]")
		vecWithInfJSON := fmt.Sprintf(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":%s}],"model":"text-embedding-3-small"}`, sb.String())

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, vecWithInfJSON)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:  secretKey,
			BaseURL: srv.URL,
		})

		_, err := client.Embed(context.Background(), "test")
		if !errors.Is(err, personalization.ErrOpenAINonFiniteVectorElement) {
			t.Fatalf("Test P failed: expected ErrOpenAINonFiniteVectorElement, got %v", err)
		}
	})

	// Q. caller context cancelled -> stops retries/request
	t.Run("CallerContextCancelled_Q", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    srv.URL,
			MaxRetries: 5,
			Backoff:    500 * time.Millisecond,
		})

		ctx, cancel := context.WithCancel(context.Background())
		// Cancel context immediately
		cancel()

		_, err := client.Embed(ctx, "test")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Test Q failed: expected context.Canceled, got %v", err)
		}
	})

	// R. API key never appears in returned error string
	t.Run("APIKeyNeverAppearsInError_R", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			// Malicious/leaky server echoes back the key
			_, _ = io.WriteString(w, fmt.Sprintf(`{"error":{"message":"Invalid key %s"}}`, secretKey))
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:  secretKey,
			BaseURL: srv.URL,
		})

		_, err := client.Embed(context.Background(), "test")
		if err == nil {
			t.Fatalf("Test R failed: expected error on 401, got nil")
		}
		if strings.Contains(err.Error(), secretKey) {
			t.Fatalf("Test R failed: secret key leaked in error string: %s", strings.ReplaceAll(err.Error(), secretKey, "[REDACTED]"))
		}
		if !strings.Contains(err.Error(), "[REDACTED]") {
			t.Fatalf("Test R failed: expected secret to be replaced with [REDACTED], got: %s", err.Error())
		}
	})

	// S. Large error body is bounded, status is present, API key is redacted/absent
	t.Run("LargeErrorBody_Bounded_S", func(t *testing.T) {
		largeBody := fmt.Sprintf("prefix error info with key %s ", secretKey) + strings.Repeat("A", 1024*1024) // 1MB
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, largeBody)
		}))
		defer srv.Close()

		client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    srv.URL,
			MaxRetries: 1,
		})

		_, err := client.Embed(context.Background(), "test")
		if err == nil {
			t.Fatalf("Test S failed: expected error, got nil")
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "HTTP 500") {
			t.Fatalf("Test S failed: expected HTTP 500 status in error, got %q", errMsg)
		}
		if strings.Contains(errMsg, secretKey) {
			t.Fatalf("Test S failed: API key leaked in error message")
		}
		// Error message length should be bounded (prefix + at most MaxOpenAIErrorBodyLength 512 bytes)
		// Diagnostic message should not exceed 1KB total
		if len(errMsg) > 1024 {
			t.Fatalf("Test S failed: error message too large (%d bytes), expected bounded <= 1024", len(errMsg))
		}
	})

	// T. Transport error retry via fake RoundTripper
	t.Run("TransportError_RetryThenSuccess_T", func(t *testing.T) {
		var attemptCount int32
		fakeTransport := &roundTripperFunc{
			fn: func(req *http.Request) (*http.Response, error) {
				count := atomic.AddInt32(&attemptCount, 1)
				if count == 1 {
					return nil, errors.New("temporary network transport error: connection reset by peer")
				}
				// 2nd attempt succeeds with valid 1536 vector response
				respBody := makeEmbeddingJSON(makeDummyVector(1536))
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(respBody)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Request:    req,
				}, nil
			},
		}

		customHTTPClient := &http.Client{
			Transport: fakeTransport,
		}

		client, err := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
			APIKey:     secretKey,
			BaseURL:    "https://fake.openai.local/v1",
			MaxRetries: 3,
			Backoff:    5 * time.Millisecond,
			HTTPClient: customHTTPClient,
		})
		if err != nil {
			t.Fatalf("unexpected client creation error: %v", err)
		}

		vec, err := client.Embed(context.Background(), "test canonical input")
		if err != nil {
			t.Fatalf("Test T failed: expected retry to succeed, got %v", err)
		}
		if len(vec) != 1536 {
			t.Fatalf("Test T failed: expected 1536 dimensions, got %d", len(vec))
		}
		if count := atomic.LoadInt32(&attemptCount); count != 2 {
			t.Fatalf("Test T failed: expected exactly 2 attempts (1 failure + 1 success), got %d", count)
		}
	})

	// U. Table test: 400, 401, 403, 404 statuses make exactly 1 attempt (no retry)
	t.Run("NonRetriableStatuses_TableTest_U", func(t *testing.T) {
		nonRetriableStatuses := []struct {
			name   string
			status int
		}{
			{"BadRequest_400", http.StatusBadRequest},
			{"Unauthorized_401", http.StatusUnauthorized},
			{"Forbidden_403", http.StatusForbidden},
			{"NotFound_404", http.StatusNotFound},
		}

		for _, tc := range nonRetriableStatuses {
			t.Run(tc.name, func(t *testing.T) {
				var requestCount int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&requestCount, 1)
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, fmt.Sprintf(`{"error":{"message":"error for status %d"}}`, tc.status))
				}))
				defer srv.Close()

				client, _ := personalization.NewOpenAIEmbeddingClient(personalization.OpenAIClientConfig{
					APIKey:     secretKey,
					BaseURL:    srv.URL,
					MaxRetries: 3,
					Backoff:    5 * time.Millisecond,
				})

				_, err := client.Embed(context.Background(), "test input")
				if err == nil {
					t.Fatalf("expected error for HTTP %d, got nil", tc.status)
				}
				if count := atomic.LoadInt32(&requestCount); count != 1 {
					t.Fatalf("expected exactly 1 attempt for HTTP %d, got %d", tc.status, count)
				}
				if !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", tc.status)) {
					t.Fatalf("expected error string to contain 'HTTP %d', got %q", tc.status, err.Error())
				}
			})
		}
	})
}

type roundTripperFunc struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (r *roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.fn(req)
}

func vecToJSON(v []float32) string {
	b, _ := json.Marshal(v)
	return string(b)
}
