package vision

import (
	"context"
	"errors"
)

var (
	ErrProviderTimeout   = errors.New("provider timeout: request timed out")
	ErrProviderRateLimit = errors.New("rate limit exceeded (429)")
	ErrProviderServer    = errors.New("server error (5xx)")
	ErrProviderNetwork   = errors.New("provider_network_error: connection or network failure")
	ErrProviderClient    = errors.New("provider error HTTP 400")
	ErrProviderAuth      = errors.New("provider_auth_error: authentication or configuration failure")
	ErrMalformedResponse = errors.New("malformed JSON response")
)

// Analyzer defines the pure domain boundary for Vision processing.
// Implementations (like Cloud.ru Qwen) must not know about storage or persistence.
type Analyzer interface {
	AnalyzeProduct(ctx context.Context, req VisionAnalysisRequest) (*VisionAnalysisResult, error)
}
