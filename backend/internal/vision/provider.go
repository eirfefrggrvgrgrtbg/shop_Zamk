package vision

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrProviderTimeout   = errors.New("provider timeout: request timed out")
	ErrProviderRateLimit = errors.New("rate limit exceeded (429)")
	ErrProviderServer    = errors.New("server error (5xx)")
	ErrProviderNetwork   = errors.New("provider_network_error: connection or network failure")
	ErrProviderClient    = errors.New("provider client error")
	ErrProviderAuth      = errors.New("provider_auth_error: authentication or configuration failure")
	ErrMalformedResponse = errors.New("malformed JSON response")
)

// ProviderHTTPError preserves the actual provider HTTP status code and sanitized body
// while maintaining errors.Is compatibility with sentinel provider errors.
type ProviderHTTPError struct {
	StatusCode int
	Body       string
	Err        error
}

func (e *ProviderHTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("%v: HTTP %d: %s", e.Err, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("%v: HTTP %d", e.Err, e.StatusCode)
}

func (e *ProviderHTTPError) Unwrap() error {
	return e.Err
}

// Analyzer defines the pure domain boundary for Vision processing.
// Implementations (like Cloud.ru Qwen) must not know about storage or persistence.
type Analyzer interface {
	AnalyzeProduct(ctx context.Context, req VisionAnalysisRequest) (*VisionAnalysisResult, error)
}
