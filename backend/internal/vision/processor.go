package vision

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type Processor struct {
	store            *Store
	loader           VisionImageLoader
	analyzer         Analyzer
	logger           *slog.Logger
	maxAttempts      int
	expectedProvider string
	expectedModelID  string
}

func NewProcessor(store *Store, loader VisionImageLoader, analyzer Analyzer, logger *slog.Logger, maxAttempts int) *Processor {
	return &Processor{
		store:            store,
		loader:           loader,
		analyzer:         analyzer,
		logger:           logger,
		maxAttempts:      maxAttempts,
		expectedProvider: CanonicalProvider,
		expectedModelID:  CanonicalModelID,
	}
}

func (p *Processor) WithExpectedContract(provider, modelID string) *Processor {
	p.expectedProvider = provider
	p.expectedModelID = modelID
	return p
}

func (p *Processor) ProcessRun(ctx context.Context, run VisionRun) error {
	start := time.Now()

	// 1. Verify snapshot hash (BLOCKER 6)
	snapHash := ComputeSnapshotHash(run.SnapshotJSON)
	if snapHash != run.SnapshotHash {
		err := fmt.Errorf("snapshot hash mismatch: expected %s, got %s", run.SnapshotHash, snapHash)
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}

	// 2. Verify taxonomy context & hash (BLOCKER 6, 9)
	if run.TaxonomyContext == nil {
		err := errors.New("taxonomy context is missing")
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}
	taxHash := ComputeTaxonomyContextHash(*run.TaxonomyContext)
	if taxHash != run.TaxonomyContextHash {
		err := fmt.Errorf("taxonomy context hash mismatch: expected %s, got %s", run.TaxonomyContextHash, taxHash)
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}

	// 3. Pre-call runtime contract checks (BLOCKER 6)
	// Must verify run contract BEFORE making any paid Analyzer call
	if p.expectedProvider != "" && run.Provider != p.expectedProvider {
		err := fmt.Errorf("provider mismatch: expected %s, got %s", p.expectedProvider, run.Provider)
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}
	if p.expectedModelID != "" && run.ModelID != p.expectedModelID {
		err := fmt.Errorf("model ID mismatch: expected %s, got %s", p.expectedModelID, run.ModelID)
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}
	if run.PromptVersion != PromptVersion {
		err := fmt.Errorf("prompt version mismatch: expected %s, got %s", PromptVersion, run.PromptVersion)
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}
	if run.SchemaVersion != SchemaVersion {
		err := fmt.Errorf("schema version mismatch: expected %s, got %s", SchemaVersion, run.SchemaVersion)
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}
	if run.VocabularyVersion != VocabularyVersion {
		err := fmt.Errorf("vocabulary version mismatch: expected %s, got %s", VocabularyVersion, run.VocabularyVersion)
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}
	if run.TaxonomyContext.PromptVersion != PromptVersion ||
		run.TaxonomyContext.SchemaVersion != SchemaVersion ||
		run.TaxonomyContext.VocabularyVersion != VocabularyVersion {
		err := errors.New("taxonomy context internal version mismatch")
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeContractMismatch, err.Error())
	}

	// 4. Load Images (BLOCKER 1, 2, 3)
	images, err := p.loader.LoadImages(ctx, run.SnapshotJSON)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		if errors.Is(err, ErrMediaMissing) {
			return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeMediaMissing, err.Error())
		}
		if errors.Is(err, ErrMediaInvalid) {
			return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeMediaInvalid, err.Error())
		}
		// Storage transient error -> requeue with FailureCodeStorageTransient (BLOCKER 5)
		return p.store.RequeueTransientFailure(ctx, run.ID, time.Now().Add(5*time.Minute), p.maxAttempts, FailureCodeStorageTransient, err.Error())
	}

	// 5. Build Request
	catIDs := make([]uuid.UUID, len(run.TaxonomyContext.Categories))
	for i, c := range run.TaxonomyContext.Categories {
		catIDs[i] = c.ID
	}
	dictIDs := make([]uuid.UUID, len(run.TaxonomyContext.Dictionaries))
	for i, d := range run.TaxonomyContext.Dictionaries {
		dictIDs[i] = d.ID
	}

	colorIDs := make([]uuid.UUID, len(run.TaxonomyContext.Colors))
	for i, c := range run.TaxonomyContext.Colors {
		colorIDs[i] = c.ID
	}

	req := VisionAnalysisRequest{
		Snapshot:              run.SnapshotJSON,
		Images:                images,
		AllowedCategoryIDs:    catIDs,
		AllowedDictionaryIDs:  dictIDs,
		AllowedColorIDs:       colorIDs,
		VocabularyContextHash: taxHash,
		TaxonomyContext:       run.TaxonomyContext,
	}

	// 6. Invoke Analyzer (BLOCKER 4)
	res, err := p.analyzer.AnalyzeProduct(ctx, req)
	latencyMs := int(time.Since(start).Milliseconds())

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}

		if isRetryableProviderError(err) {
			return p.store.RequeueTransientFailure(ctx, run.ID, time.Now().Add(5*time.Minute), p.maxAttempts, FailureCodeProviderTransient, err.Error())
		}

		// Permanent provider error (4xx client error, auth error, unsupported request, etc.) -> terminal
		return p.store.SaveProviderFailure(ctx, run.ID, FailureCodeProviderPermanent, err.Error(), nil, &latencyMs)
	}

	if res == nil {
		return p.store.SaveProviderFailure(ctx, run.ID, FailureCodeProviderPermanent, "analyzer returned nil result", nil, &latencyMs)
	}

	// 7. Validate Output (BLOCKER 4 - No semantic retry on invalid output)
	vocab := run.TaxonomyContext.VocabularyRegistry
	if vocab == nil {
		vocab = NewVocabularyRegistry()
	}
	if err := ValidateAnalysisResult(req, res, vocab); err != nil {
		return p.store.SaveInvalidOutput(ctx, run.ID, FailureCodeInvalidModelOutput, err.Error())
	}

	// 8. Save Success
	return p.store.SaveSuccess(ctx, run.ID, res.VisualObservation, res.ModerationEvaluation, res.Usage, &latencyMs, nil)
}

func isRetryableProviderError(err error) bool {
	if errors.Is(err, ErrProviderRateLimit) ||
		errors.Is(err, ErrProviderServer) ||
		errors.Is(err, ErrProviderTimeout) ||
		errors.Is(err, ErrProviderNetwork) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || os.IsTimeout(err) {
		return true
	}
	return false
}
