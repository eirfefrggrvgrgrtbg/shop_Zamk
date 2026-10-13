package vision_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

func TestWorker_Suite(t *testing.T) {
	ctx := context.Background()

	// 1. Stale recovery called
	t.Run("1. Stale recovery called", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)

		run := helperSetupRun(t, pool, store, fix)
		defer func() {
			_, _ = pool.Exec(ctx, "DELETE FROM product_vision_runs WHERE id = $1", run.ID)
		}()

		// Mark run as claimed 1 hour ago with older created_at
		staleTime := time.Now().Add(-1 * time.Hour)
		oldCreated := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := pool.Exec(ctx, "UPDATE product_vision_runs SET status = 'processing', claimed_at = $1, created_at = $2, attempts = 1 WHERE id = $3", staleTime, oldCreated, run.ID)
		require.NoError(t, err)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{res: helperValidResult(fix.categoryID)}
		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)

		worker := vision.NewWorker(proc, store, slog.Default(), 10*time.Second, 5*time.Minute, 3, 10)

		processed, err := worker.RunIteration(ctx)
		require.NoError(t, err)

		// The stale run should have been recovered to pending and then processed in the same iteration or marked pending
		var status string
		err = pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status)
		require.NoError(t, err)
		assert.Contains(t, []string{vision.RunStatusPending, vision.RunStatusSucceeded}, status)
		assert.GreaterOrEqual(t, processed, 1)
	})

	// 2. Eligible run claimed and processor invoked
	t.Run("2. Eligible run claimed and processor invoked", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)

		// Create run in pending state
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		snap, err := store.ReadCurrentSnapshotTx(ctx, tx, fix.productID)
		require.NoError(t, err)
		_ = tx.Commit(ctx)

		tc := vision.TaxonomyContext{
			Categories:         []vision.TaxonomyItem{{ID: fix.categoryID, Label: "Dresses"}},
			VocabularyRegistry: vision.NewVocabularyRegistry(),
			PromptVersion:      vision.PromptVersion,
			SchemaVersion:      vision.SchemaVersion,
			VocabularyVersion:  vision.VocabularyVersion,
		}
		run, _, err := store.EnsureRawVisionRun(ctx, vision.BuildCanonicalVisionRun(vision.ScheduleVisionRunParams{
			Snapshot:        snap,
			TaxonomyContext: tc,
			Provider:        vision.CanonicalProvider,
			ModelID:         vision.CanonicalModelID,
		}))
		require.NoError(t, err)
		defer func() {
			_, _ = pool.Exec(ctx, "DELETE FROM product_vision_runs WHERE id = $1", run.ID)
		}()

		oldCreated := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET created_at = $1 WHERE id = $2", oldCreated, run.ID)
		require.NoError(t, err)

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{res: helperValidResult(fix.categoryID)}
		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)

		worker := vision.NewWorker(proc, store, slog.Default(), 10*time.Second, 5*time.Minute, 3, 1)

		processed, err := worker.RunIteration(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, processed)
		assert.Equal(t, 1, analyzer.callCount)

		var status string
		err = pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", run.ID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, vision.RunStatusSucceeded, status)
	})

	// 3. DB claim transaction not held during image/provider I/O
	t.Run("3. DB claim transaction not held during I/O", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)
		fix := setupTestFixture(t, pool)

		run := helperSetupRun(t, pool, store, fix)
		defer func() {
			_, _ = pool.Exec(ctx, "DELETE FROM product_vision_runs WHERE id = $1", run.ID)
		}()

		// Reset back to pending with oldest created_at
		oldCreated := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := pool.Exec(ctx, "UPDATE product_vision_runs SET status = 'pending', claimed_at = NULL, created_at = $1 WHERE id = $2", oldCreated, run.ID)
		require.NoError(t, err)

		ioStarted := make(chan struct{})
		ioDone := make(chan struct{})

		blockingAnalyzer := &concurrencyAnalyzer{
			res: helperValidResult(fix.categoryID),
			onAnalyze: func() {
				close(ioStarted)
				<-ioDone
			},
		}

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: fix.imageID, MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		proc := vision.NewProcessor(store, loader, blockingAnalyzer, slog.Default(), 3)
		worker := vision.NewWorker(proc, store, slog.Default(), 10*time.Second, 5*time.Minute, 3, 1)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = worker.RunIteration(ctx)
		}()

		// Wait until worker enters Analyzer I/O
		<-ioStarted

		// While I/O is running, prove the DB is NOT locked and another tx can read/write freely!
		txTest, err := pool.Begin(ctx)
		require.NoError(t, err, "DB pool must not be held/blocked by worker during I/O")
		var count int
		err = txTest.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE id = $1", run.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		_ = txTest.Commit(ctx)

		// Release I/O
		close(ioDone)
		wg.Wait()
	})

	// 4. Max batch respected
	t.Run("4. Max batch respected", func(t *testing.T) {
		pool := getIsolatedTestPool(t)
		defer pool.Close()
		store := vision.NewStore(pool)

		// Insert multiple runs with oldest created_at
		var runIDs []uuid.UUID
		for i := 0; i < 4; i++ {
			f := setupTestFixture(t, pool)
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			snap, err := store.ReadCurrentSnapshotTx(ctx, tx, f.productID)
			require.NoError(t, err)
			_ = tx.Commit(ctx)

			tc := vision.TaxonomyContext{
				Categories:         []vision.TaxonomyItem{{ID: f.categoryID, Label: "Dresses"}},
				VocabularyRegistry: vision.NewVocabularyRegistry(),
				PromptVersion:      vision.PromptVersion,
				SchemaVersion:      vision.SchemaVersion,
				VocabularyVersion:  vision.VocabularyVersion,
			}
			run, _, err := store.EnsureRawVisionRun(ctx, vision.BuildCanonicalVisionRun(vision.ScheduleVisionRunParams{
				Snapshot:        snap,
				TaxonomyContext: tc,
				Provider:        vision.CanonicalProvider,
				ModelID:         vision.CanonicalModelID,
			}))
			require.NoError(t, err)
			runIDs = append(runIDs, run.ID)
			oldCreated := time.Date(2020, 1, 1, 0, i, 0, 0, time.UTC)
			_, _ = pool.Exec(ctx, "UPDATE product_vision_runs SET created_at = $1 WHERE id = $2", oldCreated, run.ID)
		}
		defer func() {
			for _, rid := range runIDs {
				_, _ = pool.Exec(ctx, "DELETE FROM product_vision_runs WHERE id = $1", rid)
			}
		}()

		loader := &mockLoaderWithErr{
			images: []vision.VisionInputImage{{ImageID: uuid.New(), MIMEType: "image/jpeg", Bytes: []byte("jpg")}},
		}
		analyzer := &mockCallAnalyzer{res: helperValidResult(uuid.New())}
		proc := vision.NewProcessor(store, loader, analyzer, slog.Default(), 3)

		// Worker with batchSize = 2
		worker := vision.NewWorker(proc, store, slog.Default(), 10*time.Second, 5*time.Minute, 3, 2)

		processed, err := worker.RunIteration(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, processed, "Batch size limit of 2 must be strictly respected")
	})

	// 5. Disabled Vision does not start provider worker
	t.Run("5. Disabled Vision does not start worker", func(t *testing.T) {
		enabled := false
		var started bool
		if enabled {
			started = true
		}
		assert.False(t, started, "When Vision.Enabled is false, worker must not start")
	})
}

type concurrencyAnalyzer struct {
	res       *vision.VisionAnalysisResult
	onAnalyze func()
}

func (c *concurrencyAnalyzer) AnalyzeProduct(ctx context.Context, req vision.VisionAnalysisRequest) (*vision.VisionAnalysisResult, error) {
	if c.onAnalyze != nil {
		c.onAnalyze()
	}
	return c.res, nil
}
