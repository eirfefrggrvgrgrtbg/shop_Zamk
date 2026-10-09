package cloudru

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

var (
	ErrMissingAPIKey       = errors.New("cloudru: missing API key")
	ErrProviderTimeout     = errors.New("cloudru: provider timeout")
	ErrProviderRateLimit   = errors.New("cloudru: rate limit exceeded (429)")
	ErrProviderServer      = errors.New("cloudru: server error (5xx)")
	ErrMalformedResponse   = errors.New("cloudru: malformed JSON response")
	ErrUnsupportedMIMEType = errors.New("cloudru: unsupported image MIME type")
)

const (
	DefaultBaseURL = "https://api.cloud.ru/v1"
	DefaultModel   = "qwen/qwen3-vl-235b-a22b-instruct"
)

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	Timeout    time.Duration
	MaxRetries int // Orchestrator handles this, client just makes one call
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrMissingAPIKey
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (c *Client) AnalyzeProduct(ctx context.Context, req vision.VisionAnalysisRequest) (*vision.VisionAnalysisResult, error) {
	payload, err := c.buildPayload(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			return nil, ErrProviderTimeout
		}
		return nil, fmt.Errorf("transport error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: %s", ErrProviderRateLimit, string(body))
		}
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("%w: HTTP %d: %s", ErrProviderServer, resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("provider error HTTP %d: %s", resp.StatusCode, string(body))
	}

	var providerResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&providerResp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedResponse, err)
	}

	if len(providerResp.Choices) == 0 {
		return nil, fmt.Errorf("%w: empty choices", ErrMalformedResponse)
	}

	content := providerResp.Choices[0].Message.Content
	var res vision.VisionAnalysisResult
	if err := json.Unmarshal([]byte(content), &res); err != nil {
		return nil, fmt.Errorf("%w: failed to parse structured output: %v", ErrMalformedResponse, err)
	}

	// Capture usage if present
	if providerResp.Usage.TotalTokens > 0 {
		res.Usage = &vision.UsageMetadata{
			PromptTokens:     providerResp.Usage.PromptTokens,
			CompletionTokens: providerResp.Usage.CompletionTokens,
			TotalTokens:      providerResp.Usage.TotalTokens,
		}
	}

	return &res, nil
}

func (c *Client) buildPayload(req vision.VisionAnalysisRequest) (map[string]interface{}, error) {
	// Construct the content block with text and images
	content := []map[string]interface{}{
		{
			"type": "text",
			"text": c.buildPromptText(req),
		},
	}

	for _, img := range req.Images {
		if img.MIMEType != "image/jpeg" && img.MIMEType != "image/png" && img.MIMEType != "image/webp" {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedMIMEType, img.MIMEType)
		}
		encoded := base64.StdEncoding.EncodeToString(img.Bytes)
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": fmt.Sprintf("data:%s;base64,%s", img.MIMEType, encoded),
			},
		})
	}

	// Use provider-native strict structured output constrained to the canonical schema.
	payload := map[string]interface{}{
		"model": c.cfg.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": content,
			},
		},
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   vision.CanonicalSchemaName,
				"strict": true,
				"schema": vision.CanonicalJSONSchema(),
			},
		},
		"temperature": 0.0,
	}

	return payload, nil
}

func (c *Client) buildPromptText(req vision.VisionAnalysisRequest) string {
	// In a real implementation this would be a robust template
	return fmt.Sprintf(`Observe the provided product images and output JSON.
Product Title: %s
Description: %s

CRITICAL INSTRUCTIONS:
- Observe only visible facts.
- Abstain when uncertain.
- Never invent IDs. Use only supplied taxonomy/vocabulary IDs.
- Never infer exact material composition. Distinguish appearance from fact.
- Evidence must reference exactly the supplied image IDs.
- Output ONLY valid JSON matching the requested structure.
`, req.Snapshot.Title, req.Snapshot.Description)
}
