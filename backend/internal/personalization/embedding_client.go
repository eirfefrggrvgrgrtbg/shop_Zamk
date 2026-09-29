package personalization

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	OpenAIProviderName       = "openai"
	DefaultOpenAIModel       = "text-embedding-3-small"
	DefaultOpenAIDimensions  = 1536
	DefaultOpenAIBaseURL     = "https://api.openai.com/v1"
	DefaultOpenAITimeout     = 15 * time.Second
	DefaultOpenAIMaxRetries  = 3
	DefaultOpenAIBackoff     = 100 * time.Millisecond
	MaxOpenAIErrorBodyLength = 512
)

var (
	ErrOpenAIMissingAPIKey          = errors.New("openai: missing API key")
	ErrOpenAIEmptyInput             = errors.New("openai: input text is empty")
	ErrOpenAIEmptyData              = errors.New("openai: response data is empty")
	ErrOpenAIMultipleEmbeddings     = errors.New("openai: unexpected multiple embeddings returned for single input")
	ErrOpenAIEmptyVector            = errors.New("openai: embedding vector is empty")
	ErrOpenAIInvalidDimensions      = errors.New("openai: invalid embedding dimensions")
	ErrOpenAINonFiniteVectorElement = errors.New("openai: vector contains non-finite element (NaN or Inf)")
	ErrOpenAIMalformedResponse      = errors.New("openai: malformed response JSON")
)

// EmbeddingClient is the internal interface for generating vector embeddings.
type EmbeddingClient interface {
	Embed(ctx context.Context, input string) ([]float32, error)
}

// OpenAIClientConfig holds options for creating an OpenAI embedding client.
type OpenAIClientConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
	Timeout    time.Duration
	MaxRetries int
	Backoff    time.Duration
	HTTPClient *http.Client
}

// OpenAIEmbeddingClient handles vector generation against the OpenAI Embeddings API.
type OpenAIEmbeddingClient struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	timeout    time.Duration
	maxRetries int
	backoff    time.Duration
	httpClient *http.Client
}

var _ EmbeddingClient = (*OpenAIEmbeddingClient)(nil)

// NewOpenAIEmbeddingClient creates an initialized OpenAI client.
// Returns an error if APIKey is missing.
func NewOpenAIEmbeddingClient(cfg OpenAIClientConfig) (*OpenAIEmbeddingClient, error) {
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		return nil, ErrOpenAIMissingAPIKey
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultOpenAIModel
	}

	dimensions := cfg.Dimensions
	if dimensions <= 0 {
		dimensions = DefaultOpenAIDimensions
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultOpenAITimeout
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultOpenAIMaxRetries
	}

	backoff := cfg.Backoff
	if backoff <= 0 {
		backoff = DefaultOpenAIBackoff
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &OpenAIEmbeddingClient{
		apiKey:     key,
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
		timeout:    timeout,
		maxRetries: maxRetries,
		backoff:    backoff,
		httpClient: httpClient,
	}, nil
}

type openAIEmbeddingRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Dimensions     int    `json:"dimensions"`
	EncodingFormat string `json:"encoding_format"`
}

type openAIEmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type openAIEmbeddingResponse struct {
	Object string                `json:"object"`
	Data   []openAIEmbeddingData `json:"data"`
	Model  string                `json:"model"`
}

// Embed generates an embedding vector for the provided text using OpenAI.
// Retries transient HTTP errors with bounded backoff while respecting caller cancellation.
func (c *OpenAIEmbeddingClient) Embed(ctx context.Context, input string) ([]float32, error) {
	if strings.TrimSpace(input) == "" {
		return nil, ErrOpenAIEmptyInput
	}

	reqBody := openAIEmbeddingRequest{
		Model:          c.model,
		Input:          input,
		Dimensions:     c.dimensions,
		EncodingFormat: "float",
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request failed: %w", err)
	}

	endpoint := c.baseURL + "/embeddings"

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		// Respect caller cancellation before attempt
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		vec, shouldRetry, retryAfter, err := c.doAttempt(ctx, endpoint, reqBytes)
		if err == nil {
			return vec, nil
		}

		lastErr = err
		if !shouldRetry || attempt == c.maxRetries-1 {
			break
		}

		delay := c.backoff * time.Duration(1<<attempt)
		if retryAfter > 0 {
			delay = retryAfter
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, c.sanitize(lastErr)
}

func (c *OpenAIEmbeddingClient) doAttempt(ctx context.Context, endpoint string, reqBytes []byte) ([]float32, bool, time.Duration, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, false, 0, fmt.Errorf("openai: create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, 0, ctx.Err()
		}
		return nil, true, 0, fmt.Errorf("openai: transport error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var res openAIEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, false, 0, fmt.Errorf("%w: %v", ErrOpenAIMalformedResponse, err)
		}
		vec, err := c.validateResponse(&res)
		if err != nil {
			return nil, false, 0, err
		}
		return vec, false, 0, nil
	}

	bodyPreview := c.readLimitedBody(resp.Body)
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))

	switch resp.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return nil, true, retryAfter, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, bodyPreview)
	default:
		// 400, 401, 403, 404 -> permanent / configuration / bad request -> no retry
		return nil, false, 0, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, bodyPreview)
	}
}

func (c *OpenAIEmbeddingClient) validateResponse(res *openAIEmbeddingResponse) ([]float32, error) {
	if len(res.Data) == 0 {
		return nil, ErrOpenAIEmptyData
	}
	if len(res.Data) > 1 {
		return nil, ErrOpenAIMultipleEmbeddings
	}

	embRaw := res.Data[0].Embedding
	if len(embRaw) == 0 {
		return nil, ErrOpenAIEmptyVector
	}
	if len(embRaw) != c.dimensions {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrOpenAIInvalidDimensions, c.dimensions, len(embRaw))
	}

	vec := make([]float32, len(embRaw))
	for i, v64 := range embRaw {
		if math.IsNaN(v64) || math.IsInf(v64, 0) {
			return nil, fmt.Errorf("%w: index %d has non-finite value %v", ErrOpenAINonFiniteVectorElement, i, v64)
		}
		v32 := float32(v64)
		if math.IsNaN(float64(v32)) || math.IsInf(float64(v32), 0) {
			return nil, fmt.Errorf("%w: index %d has non-finite float32 value %v", ErrOpenAINonFiniteVectorElement, i, v32)
		}
		vec[i] = v32
	}

	return vec, nil
}

func (c *OpenAIEmbeddingClient) readLimitedBody(r io.Reader) string {
	lr := io.LimitReader(r, MaxOpenAIErrorBodyLength)
	data, _ := io.ReadAll(lr)
	return strings.TrimSpace(string(data))
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
		if secs > 5 {
			secs = 5
		}
		return time.Duration(secs) * time.Second
	}
	return 0
}

func (c *OpenAIEmbeddingClient) sanitize(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if c.apiKey != "" && strings.Contains(msg, c.apiKey) {
		msg = strings.ReplaceAll(msg, c.apiKey, "[REDACTED]")
		return errors.New(msg)
	}
	return err
}
