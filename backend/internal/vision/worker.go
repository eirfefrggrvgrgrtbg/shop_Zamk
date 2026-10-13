package vision

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Worker struct {
	processor    *Processor
	store        *Store
	logger       *slog.Logger
	pollInterval time.Duration
	claimTimeout time.Duration
	maxAttempts  int
	batchSize    int
}

func NewWorker(processor *Processor, store *Store, logger *slog.Logger, pollInterval, claimTimeout time.Duration, maxAttempts, batchSize int) *Worker {
	return &Worker{
		processor:    processor,
		store:        store,
		logger:       logger,
		pollInterval: pollInterval,
		claimTimeout: claimTimeout,
		maxAttempts:  maxAttempts,
		batchSize:    batchSize,
	}
}

func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.RunIteration(ctx)
			}
		}
	}()
}

func (w *Worker) RunIteration(ctx context.Context) (int, error) {
	// 1. Recover stale runs
	recovered, err := w.store.ResetStaleRuns(ctx, w.claimTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Error("vision worker failed to reset stale runs", "error", err)
	} else if recovered > 0 {
		w.logger.Warn("vision worker recovered stale runs", "count", recovered)
	}

	// 2. Claim pending runs
	runs, err := w.store.ClaimPendingRuns(ctx, w.batchSize)
	if err != nil {
		w.logger.Error("vision worker failed to claim runs", "error", err)
		return 0, err
	}

	if len(runs) == 0 {
		return 0, nil // nothing to do
	}

	w.logger.Info("vision worker processing batch", "count", len(runs))

	// 3. Process sequentially (small concurrency as per instructions)
	processedCount := 0
	for _, run := range runs {
		if ctx.Err() != nil {
			return processedCount, ctx.Err()
		}

		err := w.processor.ProcessRun(ctx, run)
		if err != nil {
			w.logger.Error("vision worker failed to process run", "run_id", run.ID, "error", err)
		} else {
			w.logger.Info("vision worker successfully processed run", "run_id", run.ID, "product_id", run.ProductID)
			processedCount++
		}
	}
	return processedCount, nil
}
