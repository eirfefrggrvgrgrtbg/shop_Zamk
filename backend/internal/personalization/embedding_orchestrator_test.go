package personalization_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeProductSnapshotLoader struct {
	getSnapshotFunc func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error)
	callsCount      int32
}

func (f *fakeProductSnapshotLoader) GetProductCanonicalSnapshot(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
	atomic.AddInt32(&f.callsCount, 1)
	if f.getSnapshotFunc != nil {
		return f.getSnapshotFunc(ctx, productID)
	}
	return nil, nil
}

type fakeEmbeddingRepository struct {
	getMetadataFunc       func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error)
	upsertFunc            func(ctx context.Context, params personalization.UpsertProductEmbeddingParams) error
	deleteIfMatchFunc     func(ctx context.Context, productID uuid.UUID, provider, model string, dimensions, inputSchemaVersion int, contentHash string) (bool, error)
	getMetadataCalls      int32
	upsertCalls           int32
	deleteIfMatchCalls    int32
	lastUpsertParams      personalization.UpsertProductEmbeddingParams
	lastDeleteIfMatchArgs struct {
		productID          uuid.UUID
		provider           string
		model              string
		dimensions         int
		inputSchemaVersion int
		contentHash        string
	}
}

func (f *fakeEmbeddingRepository) GetProductEmbeddingMetadata(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
	atomic.AddInt32(&f.getMetadataCalls, 1)
	if f.getMetadataFunc != nil {
		return f.getMetadataFunc(ctx, productID)
	}
	return nil, personalization.ErrEmbeddingNotFound
}

func (f *fakeEmbeddingRepository) UpsertProductEmbedding(ctx context.Context, params personalization.UpsertProductEmbeddingParams) error {
	atomic.AddInt32(&f.upsertCalls, 1)
	f.lastUpsertParams = params
	if f.upsertFunc != nil {
		return f.upsertFunc(ctx, params)
	}
	return nil
}

func (f *fakeEmbeddingRepository) DeleteProductEmbeddingIfMatch(
	ctx context.Context,
	productID uuid.UUID,
	provider string,
	model string,
	dimensions int,
	inputSchemaVersion int,
	contentHash string,
) (bool, error) {
	atomic.AddInt32(&f.deleteIfMatchCalls, 1)
	f.lastDeleteIfMatchArgs = struct {
		productID          uuid.UUID
		provider           string
		model              string
		dimensions         int
		inputSchemaVersion int
		contentHash        string
	}{
		productID:          productID,
		provider:           provider,
		model:              model,
		dimensions:         dimensions,
		inputSchemaVersion: inputSchemaVersion,
		contentHash:        contentHash,
	}
	if f.deleteIfMatchFunc != nil {
		return f.deleteIfMatchFunc(ctx, productID, provider, model, dimensions, inputSchemaVersion, contentHash)
	}
	return true, nil
}

type fakeEmbeddingClient struct {
	embedFunc  func(ctx context.Context, input string) ([]float32, error)
	callsCount int32
	lastInput  string
}

func (f *fakeEmbeddingClient) Embed(ctx context.Context, input string) ([]float32, error) {
	atomic.AddInt32(&f.callsCount, 1)
	f.lastInput = input
	if f.embedFunc != nil {
		return f.embedFunc(ctx, input)
	}
	return makeValidFloatVector(1536), nil
}

func TestProductEmbeddingOrchestrator_Matrix(t *testing.T) {
	ctx := context.Background()
	prodID := uuid.New()
	hash1 := strings.Repeat("a", 64)
	hash2 := strings.Repeat("b", 64)

	makePublishedSnapshot := func(hash string) *personalization.CanonicalProductSnapshot {
		return &personalization.CanonicalProductSnapshot{
			Status:     "published",
			IsEligible: true,
			Input: &personalization.CanonicalEmbeddingInput{
				SchemaVersion: 1,
				Text:          "schema_version=1\ntitle=Test Product",
				ContentHash:   hash,
			},
		}
	}

	// A. missing embedding -> Embed called once -> saved
	t.Run("A_MissingEmbedding_GeneratesAndSaves", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return nil, personalization.ErrEmbeddingNotFound
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Outcome != personalization.OutcomeGenerated {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeGenerated, res.Outcome)
		}
		if !res.APICalled || !res.Saved {
			t.Fatalf("expected APICalled=true and Saved=true, got APICalled=%v Saved=%v", res.APICalled, res.Saved)
		}
		if client.callsCount != 1 {
			t.Fatalf("expected exactly 1 Embed call, got %d", client.callsCount)
		}
		if repo.upsertCalls != 1 {
			t.Fatalf("expected exactly 1 Upsert call, got %d", repo.upsertCalls)
		}
	})

	// B. current exact metadata/hash -> Embed not called -> no write
	t.Run("B_CurrentExactMetadata_Skipped", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return &personalization.ProductEmbeddingMetadata{
					ProductID:          prodID,
					Provider:           "openai",
					Model:              "text-embedding-3-small",
					Dimensions:         1536,
					InputSchemaVersion: 1,
					ContentHash:        hash1,
					GeneratedAt:        time.Now(),
				}, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Outcome != personalization.OutcomeSkippedCurrent {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeSkippedCurrent, res.Outcome)
		}
		if res.APICalled || res.Saved {
			t.Fatalf("expected APICalled=false and Saved=false, got APICalled=%v Saved=%v", res.APICalled, res.Saved)
		}
		if client.callsCount != 0 {
			t.Fatalf("expected 0 Embed calls, got %d", client.callsCount)
		}
		if repo.upsertCalls != 0 {
			t.Fatalf("expected 0 Upsert calls, got %d", repo.upsertCalls)
		}
	})

	// C. hash differs -> Embed once -> saved new hash/vector
	t.Run("C_HashDiffers_RegeneratesAndSaves", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash2), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return &personalization.ProductEmbeddingMetadata{
					ProductID:          prodID,
					Provider:           "openai",
					Model:              "text-embedding-3-small",
					Dimensions:         1536,
					InputSchemaVersion: 1,
					ContentHash:        hash1, // old hash
					GeneratedAt:        time.Now(),
				}, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Outcome != personalization.OutcomeGenerated {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeGenerated, res.Outcome)
		}
		if client.callsCount != 1 || repo.upsertCalls != 1 {
			t.Fatalf("expected 1 Embed call and 1 Upsert call")
		}
		if repo.lastUpsertParams.ContentHash != hash2 {
			t.Fatalf("expected saved hash %s, got %s", hash2, repo.lastUpsertParams.ContentHash)
		}
	})

	// D. provider differs -> regenerate
	t.Run("D_ProviderDiffers_Regenerates", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return &personalization.ProductEmbeddingMetadata{
					ProductID:          prodID,
					Provider:           "azure-openai", // differs from openai
					Model:              "text-embedding-3-small",
					Dimensions:         1536,
					InputSchemaVersion: 1,
					ContentHash:        hash1,
				}, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil || res.Outcome != personalization.OutcomeGenerated || client.callsCount != 1 {
			t.Fatalf("expected regeneration on provider mismatch")
		}
	})

	// E. model differs -> regenerate
	t.Run("E_ModelDiffers_Regenerates", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return &personalization.ProductEmbeddingMetadata{
					ProductID:          prodID,
					Provider:           "openai",
					Model:              "text-embedding-ada-002", // differs from text-embedding-3-small
					Dimensions:         1536,
					InputSchemaVersion: 1,
					ContentHash:        hash1,
				}, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil || res.Outcome != personalization.OutcomeGenerated || client.callsCount != 1 {
			t.Fatalf("expected regeneration on model mismatch")
		}
	})

	// F. dimensions differs -> regenerate
	t.Run("F_DimensionsDiffers_Regenerates", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return &personalization.ProductEmbeddingMetadata{
					ProductID:          prodID,
					Provider:           "openai",
					Model:              "text-embedding-3-small",
					Dimensions:         512, // differs from 1536
					InputSchemaVersion: 1,
					ContentHash:        hash1,
				}, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil || res.Outcome != personalization.OutcomeGenerated || client.callsCount != 1 {
			t.Fatalf("expected regeneration on dimensions mismatch")
		}
	})

	// G. input_schema_version differs -> regenerate
	t.Run("G_SchemaVersionDiffers_Regenerates", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return &personalization.ProductEmbeddingMetadata{
					ProductID:          prodID,
					Provider:           "openai",
					Model:              "text-embedding-3-small",
					Dimensions:         1536,
					InputSchemaVersion: 2, // differs from active 1
					ContentHash:        hash1,
				}, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil || res.Outcome != personalization.OutcomeGenerated || client.callsCount != 1 {
			t.Fatalf("expected regeneration on schema version mismatch")
		}
	})

	// H. canonical hash changes while Embed is in flight -> vector discarded -> no upsert
	// R. discarded race does NOT immediately call provider a second time
	t.Run("H_and_R_ContentChangedRace_VectorDiscarded_NoSecondCall", func(t *testing.T) {
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count == 1 {
					return makePublishedSnapshot(hash1), nil
				}
				// 2nd call: product content has changed to hash2!
				return makePublishedSnapshot(hash2), nil
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("expected nil error on normal race condition, got: %v", err)
		}

		if res.Outcome != personalization.OutcomeDiscardedContentChanged {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeDiscardedContentChanged, res.Outcome)
		}
		if !res.APICalled {
			t.Fatalf("expected APICalled=true")
		}
		if res.Saved {
			t.Fatalf("expected Saved=false (discarded)")
		}
		if repo.upsertCalls != 0 {
			t.Fatalf("expected NO upsert call on content changed race, got %d", repo.upsertCalls)
		}
		// R check: Embed called strictly once, no second attempt
		if client.callsCount != 1 {
			t.Fatalf("Test R failed: expected strictly 1 Embed call, got %d", client.callsCount)
		}
	})

	// I. product becomes unpublished while Embed is in flight -> vector discarded -> no upsert
	t.Run("I_ProductBecomesUnpublished_VectorDiscarded", func(t *testing.T) {
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count == 1 {
					return makePublishedSnapshot(hash1), nil
				}
				// 2nd call: product transitioned to draft!
				return &personalization.CanonicalProductSnapshot{
					Status:     "draft",
					IsEligible: false,
					Input:      nil,
				}, nil
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if res.Outcome != personalization.OutcomeDiscardedProductNotEligible {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeDiscardedProductNotEligible, res.Outcome)
		}
		if repo.upsertCalls != 0 {
			t.Fatalf("expected NO upsert, got %d", repo.upsertCalls)
		}
	})

	// J. product disappears after Embed -> no upsert
	t.Run("J_ProductDisappearsAfterEmbed_NoUpsert", func(t *testing.T) {
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count == 1 {
					return makePublishedSnapshot(hash1), nil
				}
				// 2nd call: product deleted from DB
				return nil, personalization.ErrProductNotFound
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("expected nil error on product disappearing, got: %v", err)
		}
		if res.Outcome != personalization.OutcomeDiscardedProductNotEligible {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeDiscardedProductNotEligible, res.Outcome)
		}
		if repo.upsertCalls != 0 {
			t.Fatalf("expected NO upsert, got %d", repo.upsertCalls)
		}
	})

	// K. first canonical load error -> no Embed
	t.Run("K_FirstCanonicalLoadError_NoEmbed", func(t *testing.T) {
		expectedErr := errors.New("db connection failure")
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return nil, expectedErr
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		_, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err == nil || !errors.Is(err, expectedErr) {
			t.Fatalf("expected load error %v, got %v", expectedErr, err)
		}
		if client.callsCount != 0 {
			t.Fatalf("expected 0 Embed calls, got %d", client.callsCount)
		}
	})

	// L. Embed error -> no upsert
	t.Run("L_EmbedError_NoUpsert", func(t *testing.T) {
		expectedErr := errors.New("openai 500 error")
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{
			embedFunc: func(ctx context.Context, input string) ([]float32, error) {
				return nil, expectedErr
			},
		}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		_, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err == nil || !errors.Is(err, expectedErr) {
			t.Fatalf("expected embed error %v, got %v", expectedErr, err)
		}
		if repo.upsertCalls != 0 {
			t.Fatalf("expected 0 upsert calls, got %d", repo.upsertCalls)
		}
	})

	// M. second canonical load error -> no upsert
	t.Run("M_SecondCanonicalLoadError_NoUpsert", func(t *testing.T) {
		expectedErr := errors.New("db network error on reload")
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count == 1 {
					return makePublishedSnapshot(hash1), nil
				}
				return nil, expectedErr
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		_, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err == nil || !errors.Is(err, expectedErr) {
			t.Fatalf("expected second load error %v, got %v", expectedErr, err)
		}
		if repo.upsertCalls != 0 {
			t.Fatalf("expected 0 upsert calls, got %d", repo.upsertCalls)
		}
	})

	// N. upsert error -> surfaced
	t.Run("N_UpsertError_Surfaced", func(t *testing.T) {
		expectedErr := errors.New("db deadlock on upsert")
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			upsertFunc: func(ctx context.Context, params personalization.UpsertProductEmbeddingParams) error {
				return expectedErr
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		_, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err == nil || !errors.Is(err, expectedErr) {
			t.Fatalf("expected upsert error %v, got %v", expectedErr, err)
		}
	})

	// O. cancelled context -> provider/reload/persistence stop appropriately
	t.Run("O_CancelledContext_StopsExecution", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)

		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before call

		_, err := orch.GenerateProductEmbedding(cancelledCtx, prodID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
		if client.callsCount != 0 || repo.upsertCalls != 0 {
			t.Fatalf("expected 0 operations on pre-cancelled context")
		}
	})

	// P. persisted metadata exactly: openai / text-embedding-3-small / 1536 / schema 1 / current hash
	t.Run("P_PersistedMetadata_ExactMatch", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{}
		dummyVec := makeValidFloatVector(1536)
		dummyVec[0] = 0.999
		client := &fakeEmbeddingClient{
			embedFunc: func(ctx context.Context, input string) ([]float32, error) {
				return dummyVec, nil
			},
		}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		_, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		p := repo.lastUpsertParams
		if p.ProductID != prodID {
			t.Errorf("expected productID %s, got %s", prodID, p.ProductID)
		}
		if p.Provider != "openai" {
			t.Errorf("expected provider 'openai', got %q", p.Provider)
		}
		if p.Model != "text-embedding-3-small" {
			t.Errorf("expected model 'text-embedding-3-small', got %q", p.Model)
		}
		if p.Dimensions != 1536 {
			t.Errorf("expected dimensions 1536, got %d", p.Dimensions)
		}
		if p.InputSchemaVersion != 1 {
			t.Errorf("expected input schema version 1, got %d", p.InputSchemaVersion)
		}
		if p.ContentHash != hash1 {
			t.Errorf("expected content hash %s, got %s", hash1, p.ContentHash)
		}
		if len(p.Embedding) != 1536 || p.Embedding[0] != 0.999 {
			t.Errorf("expected vector to be passed exactly to upsert")
		}
	})

	// Q. current check uses metadata-only repository path, not full vector read
	t.Run("Q_CurrentCheck_UsesMetadataOnlyPath", func(t *testing.T) {
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				return makePublishedSnapshot(hash1), nil
			},
		}
		repo := &fakeEmbeddingRepository{
			getMetadataFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.ProductEmbeddingMetadata, error) {
				return &personalization.ProductEmbeddingMetadata{
					ProductID:          prodID,
					Provider:           "openai",
					Model:              "text-embedding-3-small",
					Dimensions:         1536,
					InputSchemaVersion: 1,
					ContentHash:        hash1,
				}, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != personalization.OutcomeSkippedCurrent {
			t.Fatalf("expected skipped_current")
		}
		if repo.getMetadataCalls != 1 {
			t.Fatalf("expected exactly 1 metadata call, got %d", repo.getMetadataCalls)
		}
	})

	// Upfront Ineligibility: draft, moderation, deleted/absent -> skipped_not_eligible, no Embed
	t.Run("UpfrontIneligible_Skipped", func(t *testing.T) {
		statuses := []string{"draft", "pending_moderation", "rejected", "hidden", "blocked", "out_of_stock"}
		for _, st := range statuses {
			t.Run(st, func(t *testing.T) {
				loader := &fakeProductSnapshotLoader{
					getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
						return &personalization.CanonicalProductSnapshot{
							Status:     st,
							IsEligible: false,
							Input:      nil,
						}, nil
					},
				}
				repo := &fakeEmbeddingRepository{}
				client := &fakeEmbeddingClient{}

				orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
				res, err := orch.GenerateProductEmbedding(ctx, prodID)
				if err != nil {
					t.Fatalf("unexpected error for status %s: %v", st, err)
				}
				if res.Outcome != personalization.OutcomeSkippedNotEligible {
					t.Fatalf("expected outcome %q for status %s, got %q", personalization.OutcomeSkippedNotEligible, st, res.Outcome)
				}
				if client.callsCount != 0 || repo.upsertCalls != 0 {
					t.Fatalf("expected 0 Embed and 0 Upsert calls for status %s", st)
				}
			})
		}

		// Absent / deleted upfront -> skipped_not_eligible
		t.Run("AbsentUpfront", func(t *testing.T) {
			loader := &fakeProductSnapshotLoader{
				getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
					return nil, personalization.ErrProductNotFound
				},
			}
			repo := &fakeEmbeddingRepository{}
			client := &fakeEmbeddingClient{}

			orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
			res, err := orch.GenerateProductEmbedding(ctx, prodID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Outcome != personalization.OutcomeSkippedNotEligible {
				t.Fatalf("expected OutcomeSkippedNotEligible, got: %q", res.Outcome)
			}
			if client.callsCount != 0 || repo.upsertCalls != 0 {
				t.Fatalf("expected 0 Embed and 0 Upsert calls")
			}
		})
	})

	// S. Content changes AFTER second snapshot but before/during upsert completion
	// -> third snapshot detects change -> conditional delete removes our stale row -> Saved=false
	t.Run("S_ContentChangesAfterUpsert_ThirdSnapshotDetects_ConditionalDelete", func(t *testing.T) {
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count <= 2 {
					return makePublishedSnapshot(hash1), nil
				}
				// 3rd snapshot: content changed to hash2!
				return makePublishedSnapshot(hash2), nil
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Outcome != personalization.OutcomeDiscardedContentChanged {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeDiscardedContentChanged, res.Outcome)
		}
		if !res.APICalled {
			t.Fatalf("expected APICalled=true")
		}
		if res.Saved {
			t.Fatalf("expected Saved=false after third snapshot detection")
		}
		if repo.upsertCalls != 1 {
			t.Fatalf("expected 1 upsert call, got %d", repo.upsertCalls)
		}
		if repo.deleteIfMatchCalls != 1 {
			t.Fatalf("expected exactly 1 conditional delete call, got %d", repo.deleteIfMatchCalls)
		}
		if repo.lastDeleteIfMatchArgs.contentHash != hash1 {
			t.Fatalf("expected conditional delete to target our written hash %s, got %s", hash1, repo.lastDeleteIfMatchArgs.contentHash)
		}
	})

	// T. Product becomes unpublished after upsert -> conditional cleanup -> no stale row remains
	t.Run("T_ProductUnpublishedAfterUpsert_ConditionalCleanup", func(t *testing.T) {
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count <= 2 {
					return makePublishedSnapshot(hash1), nil
				}
				// 3rd snapshot: product transitioned to draft
				return &personalization.CanonicalProductSnapshot{
					Status:     "draft",
					IsEligible: false,
					Input:      nil,
				}, nil
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Outcome != personalization.OutcomeDiscardedProductNotEligible {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeDiscardedProductNotEligible, res.Outcome)
		}
		if res.Saved {
			t.Fatalf("expected Saved=false after unpublish")
		}
		if repo.deleteIfMatchCalls != 1 {
			t.Fatalf("expected 1 conditional delete call, got %d", repo.deleteIfMatchCalls)
		}
		if repo.lastDeleteIfMatchArgs.contentHash != hash1 {
			t.Fatalf("expected conditional delete to target hash %s", hash1)
		}
	})

	// U. Product disappears after upsert -> normal discarded/ineligible outcome -> conditional cleanup attempted
	t.Run("U_ProductDisappearsAfterUpsert_ConditionalCleanup", func(t *testing.T) {
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count <= 2 {
					return makePublishedSnapshot(hash1), nil
				}
				// 3rd snapshot: product deleted
				return nil, personalization.ErrProductNotFound
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Outcome != personalization.OutcomeDiscardedProductNotEligible {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeDiscardedProductNotEligible, res.Outcome)
		}
		if res.Saved {
			t.Fatalf("expected Saved=false")
		}
		if repo.deleteIfMatchCalls != 1 {
			t.Fatalf("expected 1 conditional delete call, got %d", repo.deleteIfMatchCalls)
		}
	})

	// V. Concurrent newer embedding replaces our row before cleanup -> conditional DELETE does NOT delete newer row
	t.Run("V_ConcurrentNewerEmbedding_ConditionalDeleteDoesNotDeleteNewerRow", func(t *testing.T) {
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count <= 2 {
					return makePublishedSnapshot(hash1), nil
				}
				// 3rd snapshot: content changed to hash2
				return makePublishedSnapshot(hash2), nil
			},
		}

		var rowInDBHash = hash1
		repo := &fakeEmbeddingRepository{
			upsertFunc: func(ctx context.Context, params personalization.UpsertProductEmbeddingParams) error {
				rowInDBHash = params.ContentHash
				return nil
			},
			deleteIfMatchFunc: func(ctx context.Context, productID uuid.UUID, provider, model string, dimensions, inputSchemaVersion int, contentHash string) (bool, error) {
				// If row in DB matches the conditional delete hash, delete it.
				// If another concurrent worker already wrote hash2, hash1 won't match!
				if rowInDBHash == contentHash {
					rowInDBHash = ""
					return true, nil
				}
				// No match: newer row is left untouched!
				return false, nil
			},
		}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)

		// Simulate concurrent worker writing newer embedding (hash2) before cleanup runs
		// Hook into client or simulate right after Embed:
		client.embedFunc = func(ctx context.Context, input string) ([]float32, error) {
			return makeValidFloatVector(1536), nil
		}

		// Run orchestrator
		// Before cleanup runs, another worker writes hash2:
		// We can intercept by setting rowInDBHash to hash2 right before deleteIfMatch is invoked
		var originalDeleteIfMatch = repo.deleteIfMatchFunc
		repo.deleteIfMatchFunc = func(ctx context.Context, productID uuid.UUID, provider, model string, dimensions, inputSchemaVersion int, contentHash string) (bool, error) {
			// Concurrent worker wrote hash2!
			rowInDBHash = hash2
			return originalDeleteIfMatch(ctx, productID, provider, model, dimensions, inputSchemaVersion, contentHash)
		}

		res, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Outcome != personalization.OutcomeDiscardedContentChanged {
			t.Fatalf("expected outcome %q, got %q", personalization.OutcomeDiscardedContentChanged, res.Outcome)
		}
		if res.Saved {
			t.Fatalf("expected Saved=false")
		}
		if repo.deleteIfMatchCalls != 1 {
			t.Fatalf("expected exactly 1 conditional delete call, got %d", repo.deleteIfMatchCalls)
		}
		// Confirm conditional delete targeted hash1:
		if repo.lastDeleteIfMatchArgs.contentHash != hash1 {
			t.Fatalf("expected conditional delete to target hash1, got %s", repo.lastDeleteIfMatchArgs.contentHash)
		}
		// Confirm the newer row (hash2) was NOT deleted!
		if rowInDBHash != hash2 {
			t.Fatalf("FATAL: concurrent newer embedding was deleted! Expected %s to remain, got %q", hash2, rowInDBHash)
		}
	})

	// W. Third snapshot system error after successful upsert -> error surfaced -> no blind delete
	t.Run("W_ThirdSnapshotSystemError_Surfaced_NoBlindDelete", func(t *testing.T) {
		expectedErr := errors.New("db query timeout on third snapshot")
		var snapshotCount int32
		loader := &fakeProductSnapshotLoader{
			getSnapshotFunc: func(ctx context.Context, productID uuid.UUID) (*personalization.CanonicalProductSnapshot, error) {
				count := atomic.AddInt32(&snapshotCount, 1)
				if count <= 2 {
					return makePublishedSnapshot(hash1), nil
				}
				// 3rd snapshot fails with system error
				return nil, expectedErr
			},
		}
		repo := &fakeEmbeddingRepository{}
		client := &fakeEmbeddingClient{}

		orch := personalization.NewProductEmbeddingOrchestrator(loader, repo, client)
		_, err := orch.GenerateProductEmbedding(ctx, prodID)
		if err == nil || !errors.Is(err, expectedErr) {
			t.Fatalf("expected error %v, got %v", expectedErr, err)
		}
		// Eventual-recovery semantics: NO blind delete allowed!
		if repo.deleteIfMatchCalls != 0 {
			t.Fatalf("Test W failed: blind delete was called on system error! Expected 0 delete calls, got %d", repo.deleteIfMatchCalls)
		}
	})
}

// TestProductEmbeddingOrchestrator_RealDBIntegration runs against zamk_test using the real Repository.
func TestProductEmbeddingOrchestrator_RealDBIntegration(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, safeTestDBConn)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// Invariant: strictly assert zamk_test before any mutation
	var currentDB string
	if err := pool.QueryRow(ctx, "SELECT current_database();").Scan(&currentDB); err != nil {
		pool.Close()
		t.Fatalf("failed to query current_database: %v", err)
	}
	if currentDB != "zamk_test" {
		pool.Close()
		t.Fatalf("FATAL DB SAFETY VIOLATION: expected 'zamk_test', got %q", currentDB)
	}

	repo := personalization.NewRepository(pool)

	sellerUserID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()
	pubProductID := uuid.New()
	draftProductID := uuid.New()

	allProductIDs := []uuid.UUID{pubProductID, draftProductID}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, pid := range allProductIDs {
			if _, err := pool.Exec(cleanupCtx, `DELETE FROM products WHERE id = $1`, pid); err != nil {
				t.Errorf("cleanup product %s failed: %v", pid, err)
			}
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM categories WHERE id = $1`, catID); err != nil {
			t.Errorf("cleanup category %s failed: %v", catID, err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM sellers WHERE id = $1`, sellerID); err != nil {
			t.Errorf("cleanup seller %s failed: %v", sellerID, err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, sellerUserID); err != nil {
			t.Errorf("cleanup user %s failed: %v", sellerUserID, err)
		}
		pool.Close()
	})

	// Seed Parent Rows
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, status, role, name, first_name) VALUES ($1, $2, 'hash', 'active', 'seller', 'Name', 'Name')`,
		sellerUserID, "orch_seller_"+sellerUserID.String()+"@test.com")
	if err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, status) VALUES ($1, 'OrchBrand', $2, 'active')`,
		sellerID, "orch-brand-"+sellerID.String())
	if err != nil {
		t.Fatalf("seed seller failed: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'OrchCategory', $2)`,
		catID, "orch-cat-"+catID.String())
	if err != nil {
		t.Fatalf("seed category failed: %v", err)
	}

	// Seed published product
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, status, source, price_cents, currency)
		VALUES ($1, $2, $3, 'Orchestrator Test Product', $4, 'published', 'api', 1000, 'RUB')
	`, pubProductID, sellerID, catID, "orch-prod-"+pubProductID.String())
	if err != nil {
		t.Fatalf("seed published product failed: %v", err)
	}

	// Seed draft product
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, status, source, price_cents, currency)
		VALUES ($1, $2, $3, 'Draft Test Product', $4, 'draft', 'api', 1000, 'RUB')
	`, draftProductID, sellerID, catID, "draft-prod-"+draftProductID.String())
	if err != nil {
		t.Fatalf("seed draft product failed: %v", err)
	}

	client := &fakeEmbeddingClient{}
	orch := personalization.NewDefaultProductEmbeddingOrchestrator(repo, client)

	// 1. Draft product is skipped upfront
	draftRes, err := orch.GenerateProductEmbedding(ctx, draftProductID)
	if err != nil {
		t.Fatalf("unexpected error for draft product: %v", err)
	}
	if draftRes.Outcome != personalization.OutcomeSkippedNotEligible {
		t.Fatalf("expected OutcomeSkippedNotEligible, got: %q", draftRes.Outcome)
	}
	if client.callsCount != 0 {
		t.Fatalf("expected 0 Embed calls for draft product, got %d", client.callsCount)
	}

	// 2. Published product is generated and persisted
	pubRes, err := orch.GenerateProductEmbedding(ctx, pubProductID)
	if err != nil {
		t.Fatalf("failed to generate embedding for published product: %v", err)
	}
	if pubRes.Outcome != personalization.OutcomeGenerated {
		t.Fatalf("expected OutcomeGenerated, got: %q", pubRes.Outcome)
	}
	if !pubRes.APICalled || !pubRes.Saved {
		t.Fatalf("expected APICalled=true and Saved=true")
	}
	if client.callsCount != 1 {
		t.Fatalf("expected 1 Embed call, got %d", client.callsCount)
	}

	// Verify row in DB exists with full vector
	fullEmb, err := repo.GetProductEmbedding(ctx, pubProductID)
	if err != nil {
		t.Fatalf("failed to read persisted embedding from DB: %v", err)
	}
	if fullEmb.Dimensions != 1536 || len(fullEmb.Embedding) != 1536 {
		t.Fatalf("expected 1536 dimensions, got len %d", len(fullEmb.Embedding))
	}
	if fullEmb.ContentHash != pubRes.ContentHash {
		t.Fatalf("expected hash %s, got %s", pubRes.ContentHash, fullEmb.ContentHash)
	}

	// 3. Second run for same published product is skipped as current
	secondRes, err := orch.GenerateProductEmbedding(ctx, pubProductID)
	if err != nil {
		t.Fatalf("failed second run: %v", err)
	}
	if secondRes.Outcome != personalization.OutcomeSkippedCurrent {
		t.Fatalf("expected OutcomeSkippedCurrent, got: %q", secondRes.Outcome)
	}
	if secondRes.APICalled || secondRes.Saved {
		t.Fatalf("expected APICalled=false and Saved=false on second run")
	}
	// Embed calls should still be 1 (zero new calls!)
	if client.callsCount != 1 {
		t.Fatalf("expected no new Embed calls on current check, got %d", client.callsCount)
	}
}
