package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

const (
	MinCleanupBackoff           = 1 * time.Minute
	MaxCleanupBackoff           = 1 * time.Hour
	DefaultCleanupLeaseDuration = 2 * time.Minute
	DefaultCleanupPollInterval  = 5 * time.Second
	DefaultTTLSweepInterval     = 10 * time.Minute
	DefaultTTLSweepBatchSize    = 100
	DefaultTTLUploadingMaxAge   = 1 * time.Hour
	DefaultTTLReadyMaxAge       = 24 * time.Hour
	DefaultTTLConsumedMaxAge    = 24 * time.Hour
)

type MediaCleanupConfig struct {
	LeaseDuration      time.Duration
	PollInterval       time.Duration
	TTLSweepInterval   time.Duration
	TTLSweepBatchSize  int
	TTLUploadingMaxAge time.Duration
	TTLReadyMaxAge     time.Duration
	TTLConsumedMaxAge  time.Duration
}

func DefaultMediaCleanupConfig() MediaCleanupConfig {
	return MediaCleanupConfig{
		LeaseDuration:      DefaultCleanupLeaseDuration,
		PollInterval:       DefaultCleanupPollInterval,
		TTLSweepInterval:   DefaultTTLSweepInterval,
		TTLSweepBatchSize:  DefaultTTLSweepBatchSize,
		TTLUploadingMaxAge: DefaultTTLUploadingMaxAge,
		TTLReadyMaxAge:     DefaultTTLReadyMaxAge,
		TTLConsumedMaxAge:  DefaultTTLConsumedMaxAge,
	}
}

// MediaCleanupRepository defines the repository interface required by MediaCleanupWorker.
type MediaCleanupRepository interface {
	ClaimMediaCleanupJob(ctx context.Context, leaseDuration time.Duration) (*products.ProductMediaCleanupJob, error)
	FinalizeMediaCleanupSuccess(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID) (products.CleanupFinalizeResult, error)
	FinalizeMediaCleanupFailure(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID, lastError string, backoff time.Duration) error
	ExpireStaleStagedMedia(ctx context.Context, uploadingAge, readyAge, consumedAge time.Duration, limit int) (int, error)
}

// CalculateCleanupBackoff calculates bounded exponential backoff based on attempts:
// attempt 0 -> 1m
// attempt 1 -> 2m
// attempt 2 -> 4m
// attempt 3 -> 8m
// ...
// capped at 1h.
func CalculateCleanupBackoff(attempts int) time.Duration {
	if attempts <= 0 {
		return MinCleanupBackoff
	}
	if attempts > 6 {
		return MaxCleanupBackoff
	}
	backoff := MinCleanupBackoff * time.Duration(1<<attempts)
	if backoff > MaxCleanupBackoff || backoff <= 0 {
		return MaxCleanupBackoff
	}
	return backoff
}

// IsNotFoundError returns true only for explicit, typed missing-object sentinels:
// - storage.ErrObjectNotFound
// - minio.ErrorResponse with exact Code == "NoSuchKey"
// Generic string matching and HTTP status codes alone are deliberately avoided to prevent misclassifying non-object errors like NoSuchBucket.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrObjectNotFound) {
		return true
	}
	var errResp minio.ErrorResponse
	if errors.As(err, &errResp) {
		if errResp.Code == "NoSuchKey" {
			return true
		}
	}
	return false
}

type MediaCleanupWorker struct {
	provider Provider
	repo     MediaCleanupRepository
	logger   *slog.Logger
	cfg      MediaCleanupConfig
	wg       sync.WaitGroup
}

func NewMediaCleanupWorker(
	provider Provider,
	repo MediaCleanupRepository,
	logger *slog.Logger,
	cfg MediaCleanupConfig,
) *MediaCleanupWorker {
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = DefaultCleanupLeaseDuration
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultCleanupPollInterval
	}
	if cfg.TTLSweepInterval <= 0 {
		cfg.TTLSweepInterval = DefaultTTLSweepInterval
	}
	if cfg.TTLSweepBatchSize <= 0 {
		cfg.TTLSweepBatchSize = DefaultTTLSweepBatchSize
	}
	if cfg.TTLUploadingMaxAge <= 0 {
		cfg.TTLUploadingMaxAge = DefaultTTLUploadingMaxAge
	}
	if cfg.TTLReadyMaxAge <= 0 {
		cfg.TTLReadyMaxAge = DefaultTTLReadyMaxAge
	}
	if cfg.TTLConsumedMaxAge <= 0 {
		cfg.TTLConsumedMaxAge = DefaultTTLConsumedMaxAge
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &MediaCleanupWorker{
		provider: provider,
		repo:     repo,
		logger:   logger,
		cfg:      cfg,
	}
}

// ProcessOneMediaCleanupJob executes a single cleanup queue iteration:
// 1. Claims one job with configured lease duration
// 2. Calls Provider.DeleteObject outside any PostgreSQL transaction
// 3. Normalizes missing objects to idempotent success ONLY for typed not-found errors
// 4. Finalizes success or failure while strictly respecting generation and lease semantics
func (w *MediaCleanupWorker) ProcessOneMediaCleanupJob(ctx context.Context) (bool, error) {
	job, err := w.repo.ClaimMediaCleanupJob(ctx, w.cfg.LeaseDuration)
	if err != nil {
		return false, fmt.Errorf("failed to claim media cleanup job: %w", err)
	}
	if job == nil {
		return false, nil
	}

	jobID := job.ID
	objectKey := job.ObjectKey
	generation := job.Generation
	leaseToken := *job.LeaseToken

	// OUTSIDE any DB transaction: call storage provider
	delErr := w.provider.DeleteObject(ctx, objectKey)
	if IsNotFoundError(delErr) {
		w.logger.Info("media cleanup object not found in storage, treating as idempotent success", "object_key", objectKey)
		delErr = nil
	}

	if delErr == nil {
		res, err := w.repo.FinalizeMediaCleanupSuccess(ctx, jobID, generation, leaseToken)
		if err != nil {
			return true, fmt.Errorf("failed to finalize media cleanup success: %w", err)
		}
		switch res {
		case products.CleanupFinalizeDeleted:
			w.logger.Info("media cleanup job deleted successfully", "job_id", jobID, "object_key", objectKey, "generation", generation)
		case products.CleanupFinalizeSkipped:
			w.logger.Info("media cleanup job preserved due to advanced generation", "job_id", jobID, "object_key", objectKey, "generation", generation)
		case products.CleanupFinalizeLost:
			w.logger.Warn("media cleanup job lost lease before finalize", "job_id", jobID, "object_key", objectKey, "generation", generation)
		}
		return true, nil
	}

	backoff := CalculateCleanupBackoff(job.Attempts)
	w.logger.Warn("media cleanup DeleteObject failed, recording failure with backoff",
		"job_id", jobID,
		"object_key", objectKey,
		"generation", generation,
		"attempts", job.Attempts,
		"backoff", backoff,
		"error", delErr,
	)

	failErr := w.repo.FinalizeMediaCleanupFailure(ctx, jobID, generation, leaseToken, delErr.Error(), backoff)
	if failErr != nil {
		return true, fmt.Errorf("failed to finalize media cleanup failure: %w (delete error: %v)", failErr, delErr)
	}

	return true, delErr
}

// ExpireStaleStagedMediaSweep executes a single batch-draining TTL sweep across product_media_staging:
// - repeats in bounded batches until returned count < batch size or context cancelled
func (w *MediaCleanupWorker) ExpireStaleStagedMediaSweep(ctx context.Context) (int, error) {
	totalExpired := 0
	for {
		select {
		case <-ctx.Done():
			return totalExpired, ctx.Err()
		default:
		}

		count, err := w.repo.ExpireStaleStagedMedia(
			ctx,
			w.cfg.TTLUploadingMaxAge,
			w.cfg.TTLReadyMaxAge,
			w.cfg.TTLConsumedMaxAge,
			w.cfg.TTLSweepBatchSize,
		)
		if err != nil {
			return totalExpired, fmt.Errorf("failed to expire stale staged media batch: %w", err)
		}

		totalExpired += count
		if count < w.cfg.TTLSweepBatchSize {
			break
		}
	}
	return totalExpired, nil
}

// Start launches both the cleanup queue loop and TTL scheduler in supervised background goroutines.
func (w *MediaCleanupWorker) Start(ctx context.Context) {
	w.wg.Add(2)
	go func() {
		defer w.wg.Done()
		w.StartCleanupLoop(ctx)
	}()
	go func() {
		defer w.wg.Done()
		w.StartTTLScheduler(ctx)
	}()
}

// Wait blocks until both the cleanup loop and TTL scheduler have exited.
func (w *MediaCleanupWorker) Wait() {
	w.wg.Wait()
}

// Run starts the worker loops and blocks until the context is canceled and all goroutines complete.
func (w *MediaCleanupWorker) Run(ctx context.Context) error {
	w.Start(ctx)
	w.Wait()
	return ctx.Err()
}

func (w *MediaCleanupWorker) StartCleanupLoop(ctx context.Context) {
	w.logger.Info("starting media cleanup worker loop", "lease_duration", w.cfg.LeaseDuration, "poll_interval", w.cfg.PollInterval)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("stopping media cleanup worker loop")
			return
		default:
		}

		processed, err := w.ProcessOneMediaCleanupJob(ctx)
		if err != nil {
			w.logger.Error("media cleanup worker job error", "error", err)
		}

		if processed {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		} else {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.cfg.PollInterval):
			}
		}
	}
}

func (w *MediaCleanupWorker) StartTTLScheduler(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.TTLSweepInterval)
	defer ticker.Stop()

	// Initial sweep on startup
	if total, err := w.ExpireStaleStagedMediaSweep(ctx); err != nil {
		w.logger.Error("initial media ttl sweep error", "error", err)
	} else if total > 0 {
		w.logger.Info("initial media ttl sweep completed", "expired_count", total)
	}

	w.RunTTLSchedulerLoop(ctx, ticker.C)
}

// RunTTLSchedulerLoop runs the scheduler loop driven by tickChan.
// It is exposed to allow deterministic testing of error handling across ticks without wall-clock sleep.
func (w *MediaCleanupWorker) RunTTLSchedulerLoop(ctx context.Context, tickChan <-chan time.Time) {
	w.logger.Info("starting media ttl scheduler loop", "interval", w.cfg.TTLSweepInterval, "batch_size", w.cfg.TTLSweepBatchSize)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("stopping media ttl scheduler loop")
			return
		case <-tickChan:
			if total, err := w.ExpireStaleStagedMediaSweep(ctx); err != nil {
				w.logger.Error("media ttl sweep error", "error", err)
			} else if total > 0 {
				w.logger.Info("media ttl sweep completed", "expired_count", total)
			}
		}
	}
}
