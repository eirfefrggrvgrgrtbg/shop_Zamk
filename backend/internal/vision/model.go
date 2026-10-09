package vision

import (
	"github.com/google/uuid"
)

// ProductVisionSnapshot represents the immutable logical product state.
// It does not contain runtime image bytes, only stable references.
type ProductVisionSnapshot struct {
	ProductID          uuid.UUID
	ProductRevisionID  *uuid.UUID // Live revision identity if available
	Title              string
	Description        string
	DeclaredCategoryID uuid.UUID
	DeclaredCategory   string
	DeclaredColors     []uuid.UUID
	DeclaredOptions    map[string]string // Visually relevant variant options
	Images             []SnapshotImage
}

type SnapshotImage struct {
	ImageID   uuid.UUID
	ObjectKey string // Canonical stable media identity
	SortOrder int
	IsMain    bool
}

// VisionInputImage contains the runtime payload sent to the provider.
type VisionInputImage struct {
	ImageID  uuid.UUID
	MIMEType string
	Bytes    []byte
}

// VisionAnalysisRequest represents the full runtime request for the Analyzer.
type VisionAnalysisRequest struct {
	Snapshot              ProductVisionSnapshot
	Images                []VisionInputImage
	AllowedCategoryIDs    []uuid.UUID
	AllowedDictionaryIDs  []uuid.UUID
	VocabularyContextHash string // Deterministic hash of taxonomy + vocabulary context
}

// ConfidenceScore distinguishes between known, uncertain, and abstained states.
type ConfidenceScore struct {
	Level string  `json:"level"` // "high", "medium", "low", "abstained"
	Score float64 `json:"score"` // 0.0 to 1.0 where applicable
}

// Evidence ties a visual observation directly to a supplied image.
type Evidence struct {
	Field    string      `json:"field"`
	ValueID  string      `json:"valueId"`
	ImageIDs []uuid.UUID `json:"imageIds"`
}

// ProductVisualObservationV1 captures pure visual facts.
type ProductVisualObservationV1 struct {
	ObservedCategoryID   *uuid.UUID `json:"observedCategoryId,omitempty"`
	DominantColorIDs     []string   `json:"dominantColorIds,omitempty"` // Mixed UUID or Vision string ID
	SecondaryColorIDs    []string   `json:"secondaryColorIds,omitempty"`
	SilhouetteID         *string    `json:"silhouetteId,omitempty"`
	FitAppearanceID      *string    `json:"fitAppearanceId,omitempty"`
	LengthID             *string    `json:"lengthId,omitempty"`
	SleeveIDs            []string   `json:"sleeveIds,omitempty"`
	NecklineID           *string    `json:"necklineId,omitempty"`
	ClosureID            *string    `json:"closureId,omitempty"`
	PatternID            *string    `json:"patternId,omitempty"`
	VisibleTextureID     *string    `json:"visibleTextureId,omitempty"`
	DecorativeDetailIDs  []string   `json:"decorativeDetailIds,omitempty"`
	StyleDescriptorIDs   []string   `json:"styleDescriptorIds,omitempty"`
	FormalityID          *string    `json:"formalityId,omitempty"`
	SeasonalCueIDs       []string   `json:"seasonalCueIds,omitempty"`
	ImageSetConsistency  *bool      `json:"imageSetConsistency,omitempty"`
	OCRText              []string   `json:"ocrText,omitempty"`

	Confidence map[string]ConfidenceScore `json:"confidence,omitempty"`
	Evidence   []Evidence                 `json:"evidence,omitempty"`
}

// ModerationContradiction flags a mismatch between facts and seller claims.
type ModerationContradiction struct {
	Field    string `json:"field"`
	Declared string `json:"declared"`
	Observed string `json:"observed"`
	Reason   string `json:"reason"`
}

// ModerationEvaluationV1 captures policy and validation results.
type ModerationEvaluationV1 struct {
	Contradictions           []ModerationContradiction `json:"contradictions,omitempty"`
	QualityIssues            []string                  `json:"qualityIssues,omitempty"`
	OCRFindings              []string                  `json:"ocrFindings,omitempty"`
	ModerationRecommendation string                    `json:"moderationRecommendation"` // "clear", "review_recommended", "uncertain"
	RecommendationReason     string                    `json:"recommendationReason,omitempty"`
}

// UsageMetadata captures provider costs/tokens.
type UsageMetadata struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// VisionAnalysisResult is the final compound payload returned by the Analyzer.
type VisionAnalysisResult struct {
	VisualObservation    ProductVisualObservationV1 `json:"visualObservation"`
	ModerationEvaluation ModerationEvaluationV1     `json:"moderationEvaluation"`
	Usage                *UsageMetadata             `json:"-"` // Not typically stored directly in JSON blob
}
