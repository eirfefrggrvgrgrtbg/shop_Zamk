package personalization

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Active embedding specification constants for v1 orchestration.
const (
	ActiveEmbeddingProvider           = OpenAIProviderName
	ActiveEmbeddingModel              = DefaultOpenAIModel
	ActiveEmbeddingDimensions         = DefaultOpenAIDimensions
	ActiveEmbeddingInputSchemaVersion = CanonicalEmbeddingSchemaVersion
)

// GenerationOutcome represents the deterministic business outcome of an embedding generation attempt.
type GenerationOutcome string

const (
	OutcomeGenerated                   GenerationOutcome = "generated"
	OutcomeSkippedCurrent              GenerationOutcome = "skipped_current"
	OutcomeSkippedNotEligible          GenerationOutcome = "skipped_not_eligible"
	OutcomeDiscardedContentChanged     GenerationOutcome = "discarded_content_changed"
	OutcomeDiscardedProductNotEligible GenerationOutcome = "discarded_product_not_eligible"
)

// GenerationResult provides full visibility into the execution and persistence of the embedding operation.
type GenerationResult struct {
	ProductID   uuid.UUID         `json:"productId"`
	Outcome     GenerationOutcome `json:"outcome"`
	APICalled   bool              `json:"apiCalled"`
	Saved       bool              `json:"saved"`
	ContentHash string            `json:"contentHash,omitempty"`
}

// CanonicalProductSnapshot captures the catalog eligibility and canonical embedding input at an instant in time.
type CanonicalProductSnapshot struct {
	Status     string
	IsEligible bool
	Input      *CanonicalEmbeddingInput
}

// ProductSnapshotLoader retrieves the current canonical snapshot and eligibility of a product.
type ProductSnapshotLoader interface {
	GetProductCanonicalSnapshot(ctx context.Context, productID uuid.UUID) (*CanonicalProductSnapshot, error)
}

// EmbeddingRepository handles metadata inspection and atomic persistence of product embeddings.
type EmbeddingRepository interface {
	GetProductEmbeddingMetadata(ctx context.Context, productID uuid.UUID) (*ProductEmbeddingMetadata, error)
	UpsertProductEmbedding(ctx context.Context, params UpsertProductEmbeddingParams) error
	DeleteProductEmbeddingIfMatch(
		ctx context.Context,
		productID uuid.UUID,
		provider string,
		model string,
		dimensions int,
		inputSchemaVersion int,
		contentHash string,
	) (bool, error)
}

// ProductEmbeddingOrchestrator manages the single-product embedding generation workflow,
// enforcing strict eligibility, metadata-only staleness checks, and pre-persistence race verification.
type ProductEmbeddingOrchestrator struct {
	loader ProductSnapshotLoader
	repo   EmbeddingRepository
	client EmbeddingClient
}

// NewProductEmbeddingOrchestrator creates a new orchestrator with injected dependencies.
func NewProductEmbeddingOrchestrator(
	loader ProductSnapshotLoader,
	repo EmbeddingRepository,
	client EmbeddingClient,
) *ProductEmbeddingOrchestrator {
	return &ProductEmbeddingOrchestrator{
		loader: loader,
		repo:   repo,
		client: client,
	}
}

// NewDefaultProductEmbeddingOrchestrator creates an orchestrator where *Repository serves as both loader and repo.
func NewDefaultProductEmbeddingOrchestrator(
	repo *Repository,
	client EmbeddingClient,
) *ProductEmbeddingOrchestrator {
	return NewProductEmbeddingOrchestrator(repo, repo, client)
}

// GetProductCanonicalSnapshot loads product status and, if published, retrieves canonical embedding text and hash.
// Satisfies ProductSnapshotLoader on *Repository.
func (r *Repository) GetProductCanonicalSnapshot(ctx context.Context, productID uuid.UUID) (*CanonicalProductSnapshot, error) {
	if productID == uuid.Nil {
		return nil, ErrEmbeddingNilProductID
	}

	var status string
	err := r.db.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", productID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to query product status for canonical snapshot: %w", err)
	}

	if status != "published" {
		return &CanonicalProductSnapshot{
			Status:     status,
			IsEligible: false,
			Input:      nil,
		}, nil
	}

	input, err := r.GetProductEmbeddingInput(ctx, productID)
	if err != nil {
		return nil, err
	}

	return &CanonicalProductSnapshot{
		Status:     status,
		IsEligible: true,
		Input:      input,
	}, nil
}

// GenerateProductEmbedding orchestrates the embedding lifecycle for a single product.
func (o *ProductEmbeddingOrchestrator) GenerateProductEmbedding(ctx context.Context, productID uuid.UUID) (*GenerationResult, error) {
	if productID == uuid.Nil {
		return nil, ErrEmbeddingNilProductID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 1. First Canonical Snapshot & Eligibility Verification
	firstSnapshot, err := o.loader.GetProductCanonicalSnapshot(ctx, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			return &GenerationResult{
				ProductID: productID,
				Outcome:   OutcomeSkippedNotEligible,
				APICalled: false,
				Saved:     false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get first canonical product snapshot: %w", err)
	}

	if !firstSnapshot.IsEligible || firstSnapshot.Input == nil {
		return &GenerationResult{
			ProductID: productID,
			Outcome:   OutcomeSkippedNotEligible,
			APICalled: false,
			Saved:     false,
		}, nil
	}

	firstInput := firstSnapshot.Input
	firstHash := firstInput.ContentHash

	// 2. Metadata-only Current / Stale Check
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	meta, err := o.repo.GetProductEmbeddingMetadata(ctx, productID)
	if err != nil && !errors.Is(err, ErrEmbeddingNotFound) {
		return nil, fmt.Errorf("failed to check existing product embedding metadata: %w", err)
	}

	if err == nil && isCurrentSpecAndHash(meta, firstHash) {
		return &GenerationResult{
			ProductID:   productID,
			Outcome:     OutcomeSkippedCurrent,
			APICalled:   false,
			Saved:       false,
			ContentHash: firstHash,
		}, nil
	}

	// 3. Provider Embedding Generation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	vec, err := o.client.Embed(ctx, firstInput.Text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding from provider: %w", err)
	}

	if len(vec) != ActiveEmbeddingDimensions {
		return nil, fmt.Errorf("provider returned vector of dimension %d, expected %d", len(vec), ActiveEmbeddingDimensions)
	}

	// 4. Critical Race Protection: Second Canonical Snapshot Reload
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	secondSnapshot, err := o.loader.GetProductCanonicalSnapshot(ctx, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			// Product was deleted while Embed was in flight
			return &GenerationResult{
				ProductID: productID,
				Outcome:   OutcomeDiscardedProductNotEligible,
				APICalled: true,
				Saved:     false,
			}, nil
		}
		return nil, fmt.Errorf("failed to reload canonical product snapshot: %w", err)
	}

	if !secondSnapshot.IsEligible || secondSnapshot.Input == nil {
		// Product transitioned to unpublished/ineligible while Embed was in flight
		return &GenerationResult{
			ProductID: productID,
			Outcome:   OutcomeDiscardedProductNotEligible,
			APICalled: true,
			Saved:     false,
		}, nil
	}

	secondInput := secondSnapshot.Input
	if secondInput.ContentHash != firstHash || secondInput.SchemaVersion != firstInput.SchemaVersion {
		// Catalog data changed while Embed was in flight: discard vector without secondary API call
		return &GenerationResult{
			ProductID:   productID,
			Outcome:     OutcomeDiscardedContentChanged,
			APICalled:   true,
			Saved:       false,
			ContentHash: secondInput.ContentHash,
		}, nil
	}

	// 5. Safe Persistence
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	upsertParams := UpsertProductEmbeddingParams{
		ProductID:          productID,
		Provider:           ActiveEmbeddingProvider,
		Model:              ActiveEmbeddingModel,
		Dimensions:         ActiveEmbeddingDimensions,
		InputSchemaVersion: ActiveEmbeddingInputSchemaVersion,
		ContentHash:        secondInput.ContentHash,
		Embedding:          vec,
	}

	if err := o.repo.UpsertProductEmbedding(ctx, upsertParams); err != nil {
		return nil, fmt.Errorf("failed to persist product embedding: %w", err)
	}

	// 6. Post-Write Verification (Third Snapshot) & Stale-Write Race Protection
	// Detect TOCTOU gap where catalog mutation or status change committed right before/during Upsert.
	thirdSnapshot, err := o.loader.GetProductCanonicalSnapshot(ctx, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			// Product disappeared right after upsert: conditionally cleanup our row
			_, _ = o.repo.DeleteProductEmbeddingIfMatch(
				ctx,
				productID,
				ActiveEmbeddingProvider,
				ActiveEmbeddingModel,
				ActiveEmbeddingDimensions,
				ActiveEmbeddingInputSchemaVersion,
				secondInput.ContentHash,
			)
			return &GenerationResult{
				ProductID: productID,
				Outcome:   OutcomeDiscardedProductNotEligible,
				APICalled: true,
				Saved:     false,
			}, nil
		}
		// Eventual-recovery semantics: if third snapshot encounters a system error (DB/network failure),
		// we surface the error without blind deletion, because we cannot ascertain staleness.
		// A future worker sweep will inspect metadata and heal/reconcile any inconsistency.
		return nil, fmt.Errorf("failed to verify canonical product snapshot post-write: %w", err)
	}

	if !thirdSnapshot.IsEligible || thirdSnapshot.Input == nil {
		// Product became unpublished/ineligible right after upsert: conditionally cleanup our row
		_, _ = o.repo.DeleteProductEmbeddingIfMatch(
			ctx,
			productID,
			ActiveEmbeddingProvider,
			ActiveEmbeddingModel,
			ActiveEmbeddingDimensions,
			ActiveEmbeddingInputSchemaVersion,
			secondInput.ContentHash,
		)
		return &GenerationResult{
			ProductID: productID,
			Outcome:   OutcomeDiscardedProductNotEligible,
			APICalled: true,
			Saved:     false,
		}, nil
	}

	thirdInput := thirdSnapshot.Input
	if thirdInput.ContentHash != secondInput.ContentHash || thirdInput.SchemaVersion != secondInput.SchemaVersion {
		// Content changed right after upsert: conditionally cleanup our row
		_, _ = o.repo.DeleteProductEmbeddingIfMatch(
			ctx,
			productID,
			ActiveEmbeddingProvider,
			ActiveEmbeddingModel,
			ActiveEmbeddingDimensions,
			ActiveEmbeddingInputSchemaVersion,
			secondInput.ContentHash,
		)
		return &GenerationResult{
			ProductID:   productID,
			Outcome:     OutcomeDiscardedContentChanged,
			APICalled:   true,
			Saved:       false,
			ContentHash: thirdInput.ContentHash,
		}, nil
	}

	// Third snapshot verified: persisted embedding matches current catalog content.
	return &GenerationResult{
		ProductID:   productID,
		Outcome:     OutcomeGenerated,
		APICalled:   true,
		Saved:       true,
		ContentHash: thirdInput.ContentHash,
	}, nil
}

func isCurrentSpecAndHash(meta *ProductEmbeddingMetadata, currentHash string) bool {
	if meta == nil {
		return false
	}
	return meta.Provider == ActiveEmbeddingProvider &&
		meta.Model == ActiveEmbeddingModel &&
		meta.Dimensions == ActiveEmbeddingDimensions &&
		meta.InputSchemaVersion == ActiveEmbeddingInputSchemaVersion &&
		meta.ContentHash == currentHash
}
