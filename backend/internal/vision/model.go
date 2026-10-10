package vision

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ProductVisionSnapshot represents the immutable logical product state.
// It does not contain runtime image bytes, only stable references.
type ProductVisionSnapshot struct {
	ProductID            uuid.UUID         `json:"productId"`
	ProductRevisionID    *uuid.UUID        `json:"productRevisionId,omitempty"` // Live revision identity if available
	VisionContentVersion int64             `json:"visionContentVersion"`
	Title                string            `json:"title"`
	Description        string            `json:"description"`
	DeclaredCategoryID uuid.UUID         `json:"declaredCategoryId"`
	DeclaredCategory   string            `json:"declaredCategory"`
	DeclaredColors     []uuid.UUID       `json:"declaredColors"`
	DeclaredOptions    map[string]string `json:"declaredOptions"` // Visually relevant variant options
	Images             []SnapshotImage   `json:"images"`
}

type SnapshotImage struct {
	ImageID            uuid.UUID  `json:"imageId"`
	ObjectKey          string     `json:"objectKey"` // Canonical stable source media identity
	RenditionObjectKey string     `json:"renditionObjectKey,omitempty"`
	CropX              *float64   `json:"cropX,omitempty"`
	CropY              *float64   `json:"cropY,omitempty"`
	CropWidth          *float64   `json:"cropWidth,omitempty"`
	CropHeight         *float64   `json:"cropHeight,omitempty"`
	SortOrder          int        `json:"sortOrder"`
	IsMain             bool       `json:"isMain"`
	ColorID            *uuid.UUID `json:"colorId,omitempty"`
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
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// VisionAnalysisResult is the final compound payload returned by the Analyzer.
type VisionAnalysisResult struct {
	VisualObservation    ProductVisualObservationV1 `json:"visualObservation"`
	ModerationEvaluation ModerationEvaluationV1     `json:"moderationEvaluation"`
	Usage                *UsageMetadata             `json:"usage,omitempty"`
}

const (
	RunStatusPending          = "pending"
	RunStatusProcessing       = "processing"
	RunStatusSucceeded        = "succeeded"
	RunStatusProviderFailed   = "provider_failed"
	RunStatusInvalidOutput    = "invalid_output"
	RunStatusProcessingFailed = "processing_failed" // Worker crash / max attempts exceeded
)

var (
	ErrRunNotFound              = errors.New("vision run not found")
	ErrInvalidRunTransition     = errors.New("invalid vision run state transition")
	ErrProductNotFound          = errors.New("product not found")
	ErrCorruptStoredJSON        = errors.New("corrupt stored vision json")
	ErrMaxAttemptsExceeded      = errors.New("max retry attempts exceeded")
	ErrMismatchedSnapshotHash   = errors.New("snapshot hash does not match canonical snapshot json")
	ErrMismatchedTaxonomyHash   = errors.New("taxonomy context hash does not match canonical context")
	ErrMismatchedIdempotencyKey = errors.New("idempotency key does not match canonical input values")
)

// ScheduleVisionRunParams encapsulates the canonical inputs for a Vision run.
// Callers do not manually craft hashes or idempotency keys; identity is derived deterministically.
type ScheduleVisionRunParams struct {
	RunID             uuid.UUID
	Snapshot          ProductVisionSnapshot
	TaxonomyContext   TaxonomyContext
	Provider          string
	ModelID           string
	PromptVersion     string
	SchemaVersion     string
	VocabularyVersion string
}

// BuildCanonicalVisionRun constructs a complete, valid VisionRun where all hashes and
// idempotency keys are derived from the canonical inputs.
func BuildCanonicalVisionRun(params ScheduleVisionRunParams) VisionRun {
	runID := params.RunID
	if runID == uuid.Nil {
		runID = uuid.New()
	}

	promptVer := params.PromptVersion
	if promptVer == "" {
		promptVer = PromptVersion
	}
	schemaVer := params.SchemaVersion
	if schemaVer == "" {
		schemaVer = SchemaVersion
	}
	vocabVer := params.VocabularyVersion
	if vocabVer == "" {
		vocabVer = VocabularyVersion
	}

	tc := params.TaxonomyContext
	tc.PromptVersion = promptVer
	tc.SchemaVersion = schemaVer
	tc.VocabularyVersion = vocabVer

	taxHash := ComputeTaxonomyContextHash(tc)
	snapHash := ComputeSnapshotHash(params.Snapshot)
	idemKey := ComputeIdempotencyKey(params.Provider, params.ModelID, promptVer, schemaVer, taxHash, snapHash)

	return VisionRun{
		ID:                   runID,
		ProductID:            params.Snapshot.ProductID,
		ProductRevisionID:    params.Snapshot.ProductRevisionID,
		VisionContentVersion: params.Snapshot.VisionContentVersion,
		Provider:             params.Provider,
		ModelID:              params.ModelID,
		PromptVersion:        promptVer,
		SchemaVersion:        schemaVer,
		VocabularyVersion:    vocabVer,
		TaxonomyContextHash:  taxHash,
		SnapshotHash:         snapHash,
		SnapshotJSON:         params.Snapshot,
		IdempotencyKey:       idemKey,
		Status:               RunStatusPending,
	}
}

type VisionRun struct {
	ID                   uuid.UUID
	ProductID            uuid.UUID
	ProductRevisionID    *uuid.UUID
	VisionContentVersion int64
	Provider             string
	ModelID              string
	PromptVersion        string
	SchemaVersion        string
	VocabularyVersion    string
	TaxonomyContext      *TaxonomyContext
	TaxonomyContextHash  string
	SnapshotHash         string
	SnapshotJSON         ProductVisionSnapshot
	IdempotencyKey       string
	Status               string
	Attempts             int
	ClaimedAt            *time.Time
	NextAttemptAt        *time.Time
	CompletedAt          *time.Time
	ResultObservation    *ProductVisualObservationV1
	ResultEvaluation     *ModerationEvaluationV1
	UsageMetadata        *UsageMetadata
	LatencyMs            *int
	CostCents            *int
	FailureCode          *string
	ErrorMessage         *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CurrentVisualProfile is the cheap personalization-focused read model.
// ModerationEvaluation is strictly kept in product_vision_runs to prevent
// accidental coupling between personalization and moderation state.
type CurrentVisualProfile struct {
	ProductID            uuid.UUID                  `json:"productId"`
	RunID                uuid.UUID                  `json:"runId"`
	RunCreatedAt         time.Time                  `json:"runCreatedAt"`
	VisionContentVersion int64                      `json:"visionContentVersion"`
	ModelID              string                     `json:"modelId"`
	SnapshotHash         string                     `json:"snapshotHash"`
	TaxonomyContextHash  string                     `json:"taxonomyContextHash"`
	Observation          ProductVisualObservationV1 `json:"observation"`
	UpdatedAt            time.Time                  `json:"updatedAt"`
}
