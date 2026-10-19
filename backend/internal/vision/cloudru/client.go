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
	"regexp"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

var (
	ErrMissingAPIKey       = vision.ErrProviderAuth
	ErrProviderTimeout     = vision.ErrProviderTimeout
	ErrProviderRateLimit   = vision.ErrProviderRateLimit
	ErrProviderServer      = vision.ErrProviderServer
	ErrProviderNetwork     = vision.ErrProviderNetwork
	ErrProviderClient      = vision.ErrProviderClient
	ErrMalformedResponse   = vision.ErrMalformedResponse
	ErrUnsupportedMIMEType = errors.New("cloudru: unsupported image MIME type")
)

const (
	DefaultBaseURL = "https://foundation-models.api.cloud.ru/v1"
	DefaultModel   = "Qwen/Qwen3.6-35B-A3B"
)

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	Timeout    time.Duration
	MaxRetries int // Orchestrator handles this, client just makes one call
	HTTPClient *http.Client
}

type ProviderHTTPError = vision.ProviderHTTPError

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

	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{
			Timeout: cfg.Timeout,
		}
	}

	return &Client{
		cfg:        cfg,
		httpClient: hc,
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
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			return nil, ErrProviderTimeout
		}
		return nil, fmt.Errorf("%w: transport error: %w", ErrProviderNetwork, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		safeBody := c.sanitizeErrorBody(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &vision.ProviderHTTPError{
				StatusCode: resp.StatusCode,
				Body:       safeBody,
				Err:        ErrProviderRateLimit,
			}
		}
		if resp.StatusCode >= 500 {
			return nil, &vision.ProviderHTTPError{
				StatusCode: resp.StatusCode,
				Body:       safeBody,
				Err:        ErrProviderServer,
			}
		}
		return nil, &vision.ProviderHTTPError{
			StatusCode: resp.StatusCode,
			Body:       safeBody,
			Err:        ErrProviderClient,
		}
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
				"schema": vision.BuildCanonicalJSONSchema(req),
			},
		},
		"temperature": 0.0,
	}

	return payload, nil
}

func (c *Client) buildPromptText(req vision.VisionAnalysisRequest) string {
	var sb strings.Builder
	sb.WriteString("Observe the provided product images and output JSON matching the strict schema.\n\n")

	sb.WriteString("Product Information:\n")
	sb.WriteString(fmt.Sprintf("- Title: %s\n", req.Snapshot.Title))
	if req.Snapshot.Description != "" {
		sb.WriteString(fmt.Sprintf("- Description: %s\n", req.Snapshot.Description))
	}
	if req.Snapshot.DeclaredCategory != "" {
		sb.WriteString(fmt.Sprintf("- Declared Category: %s (ID: %s)\n", req.Snapshot.DeclaredCategory, req.Snapshot.DeclaredCategoryID))
	}

	sb.WriteString("\nAvailable Images for Evidence:\n")
	if len(req.Images) > 0 {
		for i, img := range req.Images {
			sb.WriteString(fmt.Sprintf("- Image %d: %s\n", i+1, img.ImageID.String()))
		}
	} else {
		for i, img := range req.Snapshot.Images {
			sb.WriteString(fmt.Sprintf("- Image %d: %s\n", i+1, img.ImageID.String()))
		}
	}

	if req.TaxonomyContext != nil && len(req.TaxonomyContext.Colors) > 0 {
		sb.WriteString("\nAvailable Canonical Colors:\n")
		for _, col := range req.TaxonomyContext.Colors {
			if col.Hex != nil && *col.Hex != "" {
				sb.WriteString(fmt.Sprintf("- %s: %s (%s)\n", col.ID.String(), col.Label, *col.Hex))
			} else {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", col.ID.String(), col.Label))
			}
		}
	}

	sb.WriteString(`
CRITICAL INSTRUCTIONS:
- Observe only visible facts from the provided images.
- Verify whether the visible product is consistent with the Declared Category. Set categoryConsistency to "consistent", "contradictory", or "uncertain".
- In observedCategoryText, describe in plain words the garment/item type visible in the images (e.g. "hoodie", "coat", "dress"), or set to null if uncertain. NEVER invent or output a replacement category UUID.
- In evidence, imageIds must strictly reference the exact Available Image UUIDs listed above. Never use placeholders like "img1" or arbitrary IDs.
- In dominantColorIds and secondaryColorIds, reference only the Available Canonical Colors listed above. If a color cannot be confidently identified, return an empty array [].
- In nullable fields, use true JSON null when abstaining or when the feature is absent/uncertain. Never output string literals "null", "none", or "unknown".
- Abstain when visual evidence is insufficient.
`)

	return sb.String()
}

var (
	bearerPattern = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9._~+/-]+=*`)
	secretPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|authorization)\s*[:=]\s*["']?[a-zA-Z0-9._~+/-]+`)
)

func (c *Client) sanitizeErrorBody(r io.Reader) string {
	if r == nil {
		return ""
	}
	limited, _ := io.ReadAll(io.LimitReader(r, 1024))
	bodyStr := strings.TrimSpace(string(limited))
	if bodyStr == "" {
		return ""
	}

	if c.cfg.APIKey != "" {
		bodyStr = strings.ReplaceAll(bodyStr, c.cfg.APIKey, "[REDACTED]")
	}

	bodyStr = bearerPattern.ReplaceAllString(bodyStr, "Bearer [REDACTED]")
	bodyStr = secretPattern.ReplaceAllStringFunc(bodyStr, func(match string) string {
		parts := strings.SplitN(match, ":", 2)
		if len(parts) == 2 {
			return parts[0] + ": [REDACTED]"
		}
		parts = strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "=[REDACTED]"
		}
		return "[REDACTED]"
	})

	bodyStr = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, bodyStr)

	for strings.Contains(bodyStr, "  ") {
		bodyStr = strings.ReplaceAll(bodyStr, "  ", " ")
	}

	return strings.TrimSpace(bodyStr)
}
