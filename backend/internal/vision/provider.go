package vision

import (
	"context"
)

// Analyzer defines the pure domain boundary for Vision processing.
// Implementations (like Cloud.ru Qwen) must not know about storage or persistence.
type Analyzer interface {
	AnalyzeProduct(ctx context.Context, req VisionAnalysisRequest) (*VisionAnalysisResult, error)
}
