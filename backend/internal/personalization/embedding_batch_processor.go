package personalization

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

var (
	ErrBatchProcessorNilSource          = errors.New("embedding batch processor: product ID source must not be nil")
	ErrBatchProcessorNilGenerator       = errors.New("embedding batch processor: embedding generator must not be nil")
	ErrBatchProcessorNonAdvancingCursor = errors.New("embedding batch processor: pagination cursor did not advance")
)

const (
	DefaultBatchProcessorBatchSize         = 50
	MinBatchProcessorBatchSize             = 1
	MaxBatchProcessorBatchSize             = 500

	DefaultBatchProcessorMaxConcurrency    = 5
	MinBatchProcessorMaxConcurrency        = 1
	MaxBatchProcessorAllowedConcurrency    = 20

	DefaultBatchProcessorMaxFailureSamples = 10
	MinBatchProcessorMaxFailureSamples     = 1
	MaxBatchProcessorAllowedFailureSamples = 50

	maxFailureErrorLen = 256
)

// PublishedProductIDSource abstracts keyset-based retrieval of published product IDs.
type PublishedProductIDSource interface {
	ListPublishedProductIDsAfter(ctx context.Context, afterProductID *uuid.UUID, limit int) ([]uuid.UUID, error)
}

// SingleProductEmbeddingGenerator abstracts single-product embedding generation and persistence.
type SingleProductEmbeddingGenerator interface {
	GenerateProductEmbedding(ctx context.Context, productID uuid.UUID) (*GenerationResult, error)
}

// BatchProcessorConfig defines batch and concurrency limits for embedding operations.
type BatchProcessorConfig struct {
	BatchSize         int `json:"batchSize"`
	MaxConcurrency    int `json:"maxConcurrency"`
	MaxFailureSamples int `json:"maxFailureSamples"`
}

// Normalize enforces bounded and non-zero parameters for batch operations.
func (c *BatchProcessorConfig) Normalize() {
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultBatchProcessorBatchSize
	} else if c.BatchSize > MaxBatchProcessorBatchSize {
		c.BatchSize = MaxBatchProcessorBatchSize
	}

	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = DefaultBatchProcessorMaxConcurrency
	} else if c.MaxConcurrency > MaxBatchProcessorAllowedConcurrency {
		c.MaxConcurrency = MaxBatchProcessorAllowedConcurrency
	}

	if c.MaxFailureSamples <= 0 {
		c.MaxFailureSamples = DefaultBatchProcessorMaxFailureSamples
	} else if c.MaxFailureSamples > MaxBatchProcessorAllowedFailureSamples {
		c.MaxFailureSamples = MaxBatchProcessorAllowedFailureSamples
	}
}

// DefaultBatchProcessorConfig returns production-safe default configurations.
func DefaultBatchProcessorConfig() BatchProcessorConfig {
	return BatchProcessorConfig{
		BatchSize:         DefaultBatchProcessorBatchSize,
		MaxConcurrency:    DefaultBatchProcessorMaxConcurrency,
		MaxFailureSamples: DefaultBatchProcessorMaxFailureSamples,
	}
}

// ProductFailure provides diagnostic context for an isolated product failure without exposing secrets or vectors.
type ProductFailure struct {
	ProductID uuid.UUID `json:"productId"`
	Error     string    `json:"error"`
}

// BatchStats aggregates execution metrics for a single slice of product IDs.
type BatchStats struct {
	Scanned                 int              `json:"scanned"`
	Generated               int              `json:"generated"`
	SkippedCurrent          int              `json:"skippedCurrent"`
	SkippedNotEligible      int              `json:"skippedNotEligible"`
	DiscardedContentChanged int              `json:"discardedContentChanged"`
	DiscardedNotEligible    int              `json:"discardedNotEligible"`
	Failed                  int              `json:"failed"`
	Failures                []ProductFailure `json:"failures,omitempty"`
}

// SweepStats aggregates execution metrics across an entire catalog sweep.
type SweepStats struct {
	BatchesProcessed        int              `json:"batchesProcessed"`
	Scanned                 int              `json:"scanned"`
	Generated               int              `json:"generated"`
	SkippedCurrent          int              `json:"skippedCurrent"`
	SkippedNotEligible      int              `json:"skippedNotEligible"`
	DiscardedContentChanged int              `json:"discardedContentChanged"`
	DiscardedNotEligible    int              `json:"discardedNotEligible"`
	Failed                  int              `json:"failed"`
	Failures                []ProductFailure `json:"failures,omitempty"`
}

// EmbeddingBatchProcessor executes bounded concurrent embedding generation over product batches and full catalog sweeps.
type EmbeddingBatchProcessor struct {
	idSource  PublishedProductIDSource
	generator SingleProductEmbeddingGenerator
	cfg       BatchProcessorConfig
}

// NewEmbeddingBatchProcessor constructs a batch processor with injected dependencies and normalized limits.
func NewEmbeddingBatchProcessor(
	idSource PublishedProductIDSource,
	generator SingleProductEmbeddingGenerator,
	cfg BatchProcessorConfig,
) *EmbeddingBatchProcessor {
	cfg.Normalize()
	return &EmbeddingBatchProcessor{
		idSource:  idSource,
		generator: generator,
		cfg:       cfg,
	}
}

// NewDefaultEmbeddingBatchProcessor constructs a batch processor using repository and orchestrator defaults.
func NewDefaultEmbeddingBatchProcessor(
	repo *Repository,
	orchestrator *ProductEmbeddingOrchestrator,
	cfg BatchProcessorConfig,
) *EmbeddingBatchProcessor {
	return NewEmbeddingBatchProcessor(repo, orchestrator, cfg)
}

// Config returns the active normalized processor configuration.
func (p *EmbeddingBatchProcessor) Config() BatchProcessorConfig {
	return p.cfg
}

// ProcessBatch processes a slice of product IDs concurrently with bounded concurrency.
// Isolated item failures do not halt processing for the remaining products.
// Context cancellation stops launching new items and waits for active items to complete.
func (p *EmbeddingBatchProcessor) ProcessBatch(ctx context.Context, productIDs []uuid.UUID) (*BatchStats, error) {
	if p.generator == nil {
		return nil, ErrBatchProcessorNilGenerator
	}

	stats := &BatchStats{
		Failures: make([]ProductFailure, 0),
	}

	if len(productIDs) == 0 {
		return stats, nil
	}

	if err := ctx.Err(); err != nil {
		return stats, err
	}

	concurrency := p.cfg.MaxConcurrency
	if concurrency > len(productIDs) {
		concurrency = len(productIDs)
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

launchLoop:
	for _, pid := range productIDs {
		if ctx.Err() != nil {
			break launchLoop
		}

		select {
		case <-ctx.Done():
			break launchLoop
		case sem <- struct{}{}:
		}

		if ctx.Err() != nil {
			<-sem
			break launchLoop
		}

		wg.Add(1)
		go func(productID uuid.UUID) {
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					stats.Scanned++
					stats.Failed++
					if len(stats.Failures) < p.cfg.MaxFailureSamples {
						stats.Failures = append(stats.Failures, ProductFailure{
							ProductID: productID,
							Error:     fmt.Sprintf("panic during generation: %v", r),
						})
					}
					mu.Unlock()
				}
				<-sem
				wg.Done()
			}()

			res, err := p.generator.GenerateProductEmbedding(ctx, productID)

			mu.Lock()
			defer mu.Unlock()

			stats.Scanned++
			if err != nil {
				stats.Failed++
				if len(stats.Failures) < p.cfg.MaxFailureSamples {
					stats.Failures = append(stats.Failures, ProductFailure{
						ProductID: productID,
						Error:     boundedErrorString(err),
					})
				}
				return
			}

			if res == nil {
				stats.Failed++
				if len(stats.Failures) < p.cfg.MaxFailureSamples {
					stats.Failures = append(stats.Failures, ProductFailure{
						ProductID: productID,
						Error:     "nil generation result",
					})
				}
				return
			}

			switch res.Outcome {
			case OutcomeGenerated:
				stats.Generated++
			case OutcomeSkippedCurrent:
				stats.SkippedCurrent++
			case OutcomeSkippedNotEligible:
				stats.SkippedNotEligible++
			case OutcomeDiscardedContentChanged:
				stats.DiscardedContentChanged++
			case OutcomeDiscardedProductNotEligible:
				stats.DiscardedNotEligible++
			default:
				stats.Failed++
				if len(stats.Failures) < p.cfg.MaxFailureSamples {
					stats.Failures = append(stats.Failures, ProductFailure{
						ProductID: productID,
						Error:     fmt.Sprintf("unknown generation outcome: %s", res.Outcome),
					})
				}
			}
		}(pid)
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		return stats, err
	}

	return stats, nil
}

// ProcessSweep executes a complete, finite pass over all published products using keyset pagination.
// It iterates until no further published products remain or until context cancellation.
func (p *EmbeddingBatchProcessor) ProcessSweep(ctx context.Context) (*SweepStats, error) {
	if p.idSource == nil {
		return nil, ErrBatchProcessorNilSource
	}
	if p.generator == nil {
		return nil, ErrBatchProcessorNilGenerator
	}

	sweepStats := &SweepStats{
		Failures: make([]ProductFailure, 0),
	}

	if err := ctx.Err(); err != nil {
		return sweepStats, err
	}

	var cursor *uuid.UUID

	for {
		if err := ctx.Err(); err != nil {
			return sweepStats, err
		}

		ids, err := p.idSource.ListPublishedProductIDsAfter(ctx, cursor, p.cfg.BatchSize)
		if err != nil {
			return sweepStats, fmt.Errorf("failed to list published product IDs for sweep: %w", err)
		}

		if len(ids) == 0 {
			break
		}

		if cursor != nil {
			for _, id := range ids {
				if id == *cursor {
					// Non-advancing pagination cursor detected before duplicate processing
					return sweepStats, ErrBatchProcessorNonAdvancingCursor
				}
			}
		}

		batchStats, batchErr := p.ProcessBatch(ctx, ids)
		if batchStats != nil {
			sweepStats.BatchesProcessed++
			sweepStats.Scanned += batchStats.Scanned
			sweepStats.Generated += batchStats.Generated
			sweepStats.SkippedCurrent += batchStats.SkippedCurrent
			sweepStats.SkippedNotEligible += batchStats.SkippedNotEligible
			sweepStats.DiscardedContentChanged += batchStats.DiscardedContentChanged
			sweepStats.DiscardedNotEligible += batchStats.DiscardedNotEligible
			sweepStats.Failed += batchStats.Failed

			for _, f := range batchStats.Failures {
				if len(sweepStats.Failures) < p.cfg.MaxFailureSamples {
					sweepStats.Failures = append(sweepStats.Failures, f)
				}
			}
		}

		if batchErr != nil {
			return sweepStats, batchErr
		}

		lastID := ids[len(ids)-1]
		if cursor != nil && *cursor == lastID {
			// Protection against non-advancing pagination cursor
			return sweepStats, ErrBatchProcessorNonAdvancingCursor
		}
		cursor = &lastID
	}

	return sweepStats, nil
}

func boundedErrorString(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > maxFailureErrorLen {
		return s[:maxFailureErrorLen-3] + "..."
	}
	return s
}
