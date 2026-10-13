package vision_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

type mockCallAnalyzer struct {
	res       *vision.VisionAnalysisResult
	err       error
	callCount int
}

func (m *mockCallAnalyzer) AnalyzeProduct(ctx context.Context, req vision.VisionAnalysisRequest) (*vision.VisionAnalysisResult, error) {
	m.callCount++
	return m.res, m.err
}

type mockLoaderWithErr struct {
	images []vision.VisionInputImage
	err    error
}

func (m *mockLoaderWithErr) LoadImages(ctx context.Context, snapshot vision.ProductVisionSnapshot) ([]vision.VisionInputImage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.images, nil
}

func helperSetupRun(t *testing.T, pool *pgxpool.Pool, store *vision.Store, fix testFixture) vision.VisionRun {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx, "UPDATE products SET status = 'published', vision_content_version = 1 WHERE id = $1", fix.productID)
	require.NoError(t, err)

	tc := vision.TaxonomyContext{
		Categories:         []vision.TaxonomyItem{{ID: fix.categoryID, Label: "Dresses"}},
		VocabularyRegistry: vision.NewVocabularyRegistry(),
		PromptVersion:      vision.PromptVersion,
		SchemaVersion:      vision.SchemaVersion,
		VocabularyVersion:  vision.VocabularyVersion,
	}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	snap, err := store.ReadCurrentSnapshotTx(ctx, tx, fix.productID)
	require.NoError(t, err)
	_ = tx.Commit(ctx)

	run, _, err := store.EnsureRawVisionRun(ctx, vision.BuildCanonicalVisionRun(vision.ScheduleVisionRunParams{
		Snapshot:        snap,
		TaxonomyContext: tc,
		Provider:        vision.CanonicalProvider,
		ModelID:         vision.CanonicalModelID,
	}))
	require.NoError(t, err)

	var claimed vision.VisionRun
	var snapBytes, taxBytes []byte
	err = pool.QueryRow(ctx, `
		UPDATE product_vision_runs
		SET status = 'processing', claimed_at = now(), attempts = attempts + 1, updated_at = now()
		WHERE id = $1
		RETURNING id, product_id, product_revision_id, vision_content_version, provider, model_id, prompt_version, schema_version,
		          vocabulary_version, taxonomy_context_hash, taxonomy_context_json, snapshot_hash, snapshot_json,
		          idempotency_key, status, attempts, claimed_at, next_attempt_at, completed_at,
		          created_at, updated_at
	`, run.ID).Scan(
		&claimed.ID, &claimed.ProductID, &claimed.ProductRevisionID, &claimed.VisionContentVersion, &claimed.Provider, &claimed.ModelID, &claimed.PromptVersion, &claimed.SchemaVersion,
		&claimed.VocabularyVersion, &claimed.TaxonomyContextHash, &taxBytes, &claimed.SnapshotHash, &snapBytes,
		&claimed.IdempotencyKey, &claimed.Status, &claimed.Attempts, &claimed.ClaimedAt, &claimed.NextAttemptAt, &claimed.CompletedAt,
		&claimed.CreatedAt, &claimed.UpdatedAt,
	)
	require.NoError(t, err)
	if len(snapBytes) > 0 {
		_ = json.Unmarshal(snapBytes, &claimed.SnapshotJSON)
	}
	if len(taxBytes) > 0 {
		var taxCtx vision.TaxonomyContext
		if err := json.Unmarshal(taxBytes, &taxCtx); err == nil {
			claimed.TaxonomyContext = &taxCtx
		}
	}
	return claimed
}

func helperValidResult(_ uuid.UUID) *vision.VisionAnalysisResult {
	obsText := "hoodie"
	return &vision.VisionAnalysisResult{
		VisualObservation: vision.ProductVisualObservationV2{
			CategoryConsistency:  vision.CategoryConsistencyConsistent,
			ObservedCategoryText: &obsText,
		},
		ModerationEvaluation: vision.ModerationEvaluationV1{
			ModerationRecommendation: "clear",
		},
	}
}

func TestProcessor_Matrix(t *testing.T) {
	ctx := context.Background()

	// A. SUCCESS
	t.Run("A. SUCCESS: valid run/images/result -> succeeded, Analyzer called exactly once", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{
			res: helperValidResult(fix.categoryID),
		}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 1, analyzer.callCount)

		var status, failCode, failMsg string
		err = pool.QueryRow(ctx, "SELECT status, COALESCE(failure_code, ''), COALESCE(error_message, '') FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode, &failMsg)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusSucceeded, status, "failed with code=%s, msg=%s", failCode, failMsg)

		// Profile advanced
		var profileRunID uuid.UUID
		err = pool.QueryRow(ctx, "SELECT run_id FROM product_current_visual_profiles WHERE product_id = $1", fix.productID).Scan(&profileRunID)
		require.NoError(t, err)
		assert.Equal(t, run.ID, profileRunID)
	})

	// B. PRODUCT CHANGED DURING PROCESSING
	t.Run("B. PRODUCT CHANGED DURING PROCESSING -> historical succeeded, current profile does not advance", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		// Mutate product to version 2 before run completes
		_, err := pool.Exec(ctx, "UPDATE products SET vision_content_version = 2, title = 'Updated Title' WHERE id = $1", fix.productID)
		require.NoError(t, err)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{
			res: helperValidResult(fix.categoryID),
		}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err = proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status string
		err = pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusSucceeded, status)

		// Profile must NOT advance because DB product is at version 2
		var profileCount int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_current_visual_profiles WHERE product_id = $1", fix.productID).Scan(&profileCount)
		require.NoError(t, err)
		assert.Equal(t, 0, profileCount)
	})

	// C. UNKNOWN IMAGE ID
	t.Run("C. UNKNOWN IMAGE ID -> invalid_output, no retry", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		badResult := helperValidResult(fix.categoryID)
		unknownImg := uuid.New()
		badResult.VisualObservation.Evidence = []vision.Evidence{
			{Field: "silhouette", ValueID: "vision.silhouette.regular", ImageIDs: []uuid.UUID{unknownImg}},
		}

		analyzer := &mockCallAnalyzer{res: badResult}
		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeInvalidModelOutput, failCode)
	})

	// D. UNKNOWN TAXONOMY ID
	t.Run("D. UNKNOWN TAXONOMY ID -> invalid_output, no retry", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		badResult := helperValidResult(fix.categoryID)
		badResult.VisualObservation.CategoryConsistency = vision.CategoryConsistency("invalid_consistency")

		analyzer := &mockCallAnalyzer{res: badResult}
		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeInvalidModelOutput, failCode)
	})

	// E. INVALID MODEL SEMANTICS
	t.Run("E. INVALID MODEL SEMANTICS -> invalid_output, no semantic retry", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		badResult := helperValidResult(fix.categoryID)
		badResult.ModerationEvaluation.ModerationRecommendation = "bad_recommendation"

		analyzer := &mockCallAnalyzer{res: badResult}
		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeInvalidModelOutput, failCode)
	})

	// F. PROVIDER 429
	t.Run("F. PROVIDER 429 -> retry scheduled", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{err: vision.ErrProviderRateLimit}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		var nextAttempt *string
		err = pool.QueryRow(ctx, "SELECT status, failure_code, next_attempt_at::text FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode, &nextAttempt)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusPending, status)
		assert.Equal(t, vision.FailureCodeProviderTransient, failCode)
		assert.NotNil(t, nextAttempt)
	})

	// G. PROVIDER 5XX
	t.Run("G. PROVIDER 5XX -> retry scheduled", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{err: vision.ErrProviderServer}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusPending, status)
		assert.Equal(t, vision.FailureCodeProviderTransient, failCode)
	})

	// H. PROVIDER TIMEOUT
	t.Run("H. PROVIDER TIMEOUT -> retry scheduled", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{err: vision.ErrProviderTimeout}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusPending, status)
		assert.Equal(t, vision.FailureCodeProviderTransient, failCode)
	})

	// I. PROVIDER PERMANENT 4XX
	t.Run("I. PROVIDER PERMANENT 4XX -> terminal, no retry", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{err: vision.ErrProviderClient}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		var nextAttempt *string
		err = pool.QueryRow(ctx, "SELECT status, failure_code, next_attempt_at::text FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode, &nextAttempt)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusProviderFailed, status)
		assert.Equal(t, vision.FailureCodeProviderPermanent, failCode)
		assert.Nil(t, nextAttempt)
	})

	// J. STORAGE TRANSIENT
	t.Run("J. STORAGE TRANSIENT -> retry scheduled, failure classified storage, NOT provider", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{err: vision.ErrStorageTransient}
		analyzer := &mockCallAnalyzer{}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount, "Analyzer must not be called on storage error")

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusPending, status)
		assert.Equal(t, vision.FailureCodeStorageTransient, failCode)
	})

	// K. STORAGE NOT FOUND
	t.Run("K. STORAGE NOT FOUND -> terminal media_missing", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{err: vision.ErrMediaMissing}
		analyzer := &mockCallAnalyzer{}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeMediaMissing, failCode)
	})

	// L. UNSUPPORTED/CORRUPT IMAGE
	t.Run("L. UNSUPPORTED/CORRUPT IMAGE -> terminal media_invalid", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		loader := &mockLoaderWithErr{err: vision.ErrMediaInvalid}
		analyzer := &mockCallAnalyzer{}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeMediaInvalid, failCode)
	})

	// M. SNAPSHOT HASH MISMATCH
	t.Run("M. SNAPSHOT HASH MISMATCH -> Analyzer calls = 0", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		// Tamper with snapshot hash in the claimed struct
		run.SnapshotHash = "tampered_snapshot_hash"

		analyzer := &mockCallAnalyzer{}
		loader := &mockLoaderWithErr{}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeContractMismatch, failCode)
	})

	// N. TAXONOMY HASH MISMATCH
	t.Run("N. TAXONOMY HASH MISMATCH -> Analyzer calls = 0", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		run.TaxonomyContextHash = "tampered_taxonomy_hash"

		analyzer := &mockCallAnalyzer{}
		loader := &mockLoaderWithErr{}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeContractMismatch, failCode)
	})

	// O. MODEL MISMATCH
	t.Run("O. MODEL MISMATCH -> Analyzer calls = 0", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		// Run has model different from processor expected model
		proc := vision.NewProcessor(store, &mockLoaderWithErr{}, &mockCallAnalyzer{}, slog.Default(), 3)
		proc.WithExpectedContract(vision.CanonicalProvider, "different-model-id")

		analyzer := &mockCallAnalyzer{}
		proc = vision.NewProcessor(store, &mockLoaderWithErr{}, analyzer, slog.Default(), 3).WithExpectedContract(vision.CanonicalProvider, "different-model-id")
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeContractMismatch, failCode)
	})

	// P. PROMPT VERSION MISMATCH
	t.Run("P. PROMPT VERSION MISMATCH -> Analyzer calls = 0", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		run.PromptVersion = "old_prompt_v0"

		analyzer := &mockCallAnalyzer{}
		proc := vision.NewProcessor(store, &mockLoaderWithErr{}, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeContractMismatch, failCode)
	})

	// Q. SCHEMA VERSION MISMATCH
	t.Run("Q. SCHEMA VERSION MISMATCH -> Analyzer calls = 0", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		run.SchemaVersion = "old_schema_v0"

		analyzer := &mockCallAnalyzer{}
		proc := vision.NewProcessor(store, &mockLoaderWithErr{}, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeContractMismatch, failCode)
	})

	// R. VOCABULARY VERSION MISMATCH
	t.Run("R. VOCABULARY VERSION MISMATCH -> Analyzer calls = 0", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		run.VocabularyVersion = "old_vocab_v0"

		analyzer := &mockCallAnalyzer{}
		proc := vision.NewProcessor(store, &mockLoaderWithErr{}, analyzer, slog.Default(), 3)
		err := proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		assert.Equal(t, 0, analyzer.callCount)

		var status, failCode string
		err = pool.QueryRow(ctx, "SELECT status, failure_code FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusInvalidOutput, status)
		assert.Equal(t, vision.FailureCodeContractMismatch, failCode)
	})

	// S. ATTEMPTS EXHAUSTED
	t.Run("S. ATTEMPTS EXHAUSTED -> no further requeue/claim", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)
		run := helperSetupRun(t, pool, store, fix)

		// Set attempts to 3 (maxAttempts = 3)
		_, err := pool.Exec(ctx, "UPDATE product_vision_runs SET attempts = 3 WHERE id = $1", run.ID)
		require.NoError(t, err)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{err: vision.ErrProviderRateLimit}

		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)
		err = proc.ProcessRun(ctx, run)
		require.NoError(t, err)

		var status, failCode string
		var nextAttempt *string
		err = pool.QueryRow(ctx, "SELECT status, failure_code, next_attempt_at::text FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status, &failCode, &nextAttempt)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusProcessingFailed, status)
		assert.Equal(t, vision.FailureCodeProviderTransient, failCode)
		assert.Nil(t, nextAttempt)

		// Must not be claimed again
		claimed, err := store.ClaimPendingRuns(ctx, 10)
		require.NoError(t, err)
		for _, c := range claimed {
			assert.NotEqual(t, run.ID, c.ID)
		}
	})
}
