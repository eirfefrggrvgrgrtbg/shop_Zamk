package storage_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type mockStorageProvider struct {
	mu          sync.Mutex
	deleteCalls []string
	deleteHook  func(ctx context.Context, key string) error
}

func (m *mockStorageProvider) UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*storage.StoredObject, error) {
	return &storage.StoredObject{ObjectKey: objectKey, ObjectURL: "http://test/" + objectKey, Size: objectSize}, nil
}

func (m *mockStorageProvider) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	return []byte("data"), nil
}

func (m *mockStorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	m.mu.Lock()
	m.deleteCalls = append(m.deleteCalls, objectKey)
	hook := m.deleteHook
	m.mu.Unlock()

	if hook != nil {
		return hook(ctx, objectKey)
	}
	return nil
}

func (m *mockStorageProvider) BuildPublicURL(objectKey string) string {
	return "http://test/" + objectKey
}

func (m *mockStorageProvider) getDeleteCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]string, len(m.deleteCalls))
	copy(res, m.deleteCalls)
	return res
}

type scopedCleanupRepo struct {
	repo          storage.MediaCleanupRepository
	allowedPrefix string
}

func (s *scopedCleanupRepo) ClaimMediaCleanupJob(ctx context.Context, leaseDuration time.Duration) (*products.ProductMediaCleanupJob, error) {
	job, err := s.repo.ClaimMediaCleanupJob(ctx, leaseDuration)
	if err != nil || job == nil {
		return job, err
	}
	if strings.HasPrefix(job.ObjectKey, s.allowedPrefix) {
		return job, nil
	}
	_ = s.repo.FinalizeMediaCleanupFailure(ctx, job.ID, job.Generation, *job.LeaseToken, "scoped test skipped external job", 0)
	return nil, nil
}

func (s *scopedCleanupRepo) FinalizeMediaCleanupSuccess(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID) (products.CleanupFinalizeResult, error) {
	return s.repo.FinalizeMediaCleanupSuccess(ctx, jobID, generation, leaseToken)
}

func (s *scopedCleanupRepo) FinalizeMediaCleanupFailure(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID, lastError string, backoff time.Duration) error {
	return s.repo.FinalizeMediaCleanupFailure(ctx, jobID, generation, leaseToken, lastError, backoff)
}

func (s *scopedCleanupRepo) ExpireStaleStagedMedia(ctx context.Context, uploadingAge, readyAge, consumedAge time.Duration, limit int) (int, error) {
	return s.repo.ExpireStaleStagedMedia(ctx, uploadingAge, readyAge, consumedAge, limit)
}

func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"typed storage.ErrObjectNotFound", storage.ErrObjectNotFound, true},
		{"wrapped ErrObjectNotFound", fmt.Errorf("wrapped: %w", storage.ErrObjectNotFound), true},
		{"typed minio NoSuchKey code", minio.ErrorResponse{Code: "NoSuchKey"}, true},
		{"wrapped minio NoSuchKey", fmt.Errorf("s3 delete: %w", minio.ErrorResponse{Code: "NoSuchKey"}), true},
		{"typed minio NoSuchBucket 404", minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound}, false},
		{"typed minio 404 without NoSuchKey", minio.ErrorResponse{StatusCode: http.StatusNotFound}, false},
		{"generic storage host not found", errors.New("storage host not found"), false},
		{"generic text containing 404", errors.New("upstream returned 404 during transport"), false},
		{"generic text containing nosuchkey", errors.New("nosuchkey found in message text"), false},
		{"typed minio 500 internal server error", minio.ErrorResponse{StatusCode: http.StatusInternalServerError, Code: "InternalError"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := storage.IsNotFoundError(tc.err)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestCalculateCleanupBackoff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		attempts int
		expected time.Duration
	}{
		{-1, 1 * time.Minute},
		{0, 1 * time.Minute},
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{3, 8 * time.Minute},
		{4, 16 * time.Minute},
		{5, 32 * time.Minute},
		{6, 1 * time.Hour},
		{7, 1 * time.Hour},
		{10, 1 * time.Hour},
	}

	for _, tc := range cases {
		actual := storage.CalculateCleanupBackoff(tc.attempts)
		require.Equal(t, tc.expected, actual, "attempts=%d", tc.attempts)
	}
}

func TestMediaCleanupExecution(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	db, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, db.Pool)
	var dbName string
	err = db.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "integration test must run on zamk_test")

	// Upfront fixture generation
	sellerID := uuid.New()
	productID := uuid.New()
	brandID := uuid.New()
	catID := uuid.New()
	cleanupKeyPrefix := fmt.Sprintf("test-cleanup-%s", uuid.New())

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_staging WHERE seller_id = $1", sellerID); err != nil {
			t.Errorf("cleanup staging failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE $1", cleanupKeyPrefix+"%"); err != nil {
			t.Errorf("cleanup jobs failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM products WHERE id = $1", productID); err != nil {
			t.Errorf("cleanup product failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM categories WHERE id = $1", catID); err != nil {
			t.Errorf("cleanup category failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM brands WHERE id = $1", brandID); err != nil {
			t.Errorf("cleanup brand failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM sellers WHERE id = $1", sellerID); err != nil {
			t.Errorf("cleanup seller failed: %v", err)
		}
		db.Close()
	})

	// Seed DB dependencies
	_, err = db.Pool.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'Test Brand', $2)", brandID, uuid.New().String())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Test Cat', $2)", catID, uuid.New().String())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug) VALUES ($1, 'Test Seller', $2)", sellerID, uuid.New().String())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'P', $5, 'draft', 100, 'RUB')", productID, sellerID, brandID, catID, uuid.New().String())
	require.NoError(t, err)

	productsRepo := products.NewRepository(db.Pool)

	t.Run("1. Eligible job: claim -> DeleteObject called -> finalize success -> job gone", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-1.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.NoError(t, err)
		require.True(t, processed)

		require.Contains(t, mockStorage.getDeleteCalls(), key)

		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "job must be deleted from database")
	})

	t.Run("2a. DeleteObject missing-object: storage.ErrObjectNotFound -> treated as success -> job removed", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-2a.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, key string) error {
				return storage.ErrObjectNotFound
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.NoError(t, err)
		require.True(t, processed)

		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "missing object must be treated as success and removed from queue")
	})

	t.Run("2b. DeleteObject missing-object: minio NoSuchKey -> treated as success -> job removed", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-2b.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, key string) error {
				return minio.ErrorResponse{Code: "NoSuchKey"}
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.NoError(t, err)
		require.True(t, processed)

		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "NoSuchKey must be treated as idempotent success and removed from queue")
	})

	t.Run("2c. DeleteObject NoSuchBucket 404: failure -> job retained -> backoff applied", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-2c.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, key string) error {
				return minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound}
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.Error(t, err)
		require.True(t, processed)

		var attempts int
		var lastError *string
		var nextAttempt time.Time
		var leaseToken *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT attempts, last_error, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE object_key = $1", key).
			Scan(&attempts, &lastError, &nextAttempt, &leaseToken)
		require.NoError(t, err)
		require.Equal(t, 1, attempts, "attempts must increment on NoSuchBucket 404 failure")
		require.NotNil(t, lastError)
		require.True(t, nextAttempt.After(time.Now().Add(30*time.Second)), "backoff must be applied")
		require.Nil(t, leaseToken, "lease must be released")
	})

	t.Run("3. DeleteObject generic host not found: failure -> job retained -> backoff applied", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-3.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, key string) error {
				return errors.New("storage host not found")
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.Error(t, err)
		require.True(t, processed)

		var attempts int
		var lastError *string
		var nextAttempt time.Time
		var leaseToken *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT attempts, last_error, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE object_key = $1", key).
			Scan(&attempts, &lastError, &nextAttempt, &leaseToken)
		require.NoError(t, err)
		require.Equal(t, 1, attempts, "attempts must increment on host-not-found failure")
		require.NotNil(t, lastError)
		require.Contains(t, *lastError, "storage host not found")
		require.True(t, nextAttempt.After(time.Now().Add(30*time.Second)), "next_attempt_at must be in future (backoff applied)")
		require.Nil(t, leaseToken, "lease token must be cleared on failure")
	})

	t.Run("4. DeleteObject generic text containing 404: failure -> job retained -> backoff applied", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-4.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, key string) error {
				return errors.New("upstream returned 404 during transport")
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.Error(t, err)
		require.True(t, processed)

		var attempts int
		var lastError *string
		var nextAttempt time.Time
		var leaseToken *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT attempts, last_error, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE object_key = $1", key).
			Scan(&attempts, &lastError, &nextAttempt, &leaseToken)
		require.NoError(t, err)
		require.Equal(t, 1, attempts, "attempts must increment on un-typed generic 404 transport failure")
		require.NotNil(t, lastError)
		require.Contains(t, *lastError, "upstream returned 404 during transport")
		require.True(t, nextAttempt.After(time.Now().Add(30*time.Second)), "backoff must be applied")
		require.Nil(t, leaseToken, "lease must be released")
	})

	t.Run("5. DeleteObject transient 500: attempts incremented -> backoff applied", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-5.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, key string) error {
				return errors.New("500 internal server error")
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.Error(t, err)
		require.True(t, processed)

		var attempts int
		var lastError *string
		var nextAttempt time.Time
		var leaseToken *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT attempts, last_error, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE object_key = $1", key).
			Scan(&attempts, &lastError, &nextAttempt, &leaseToken)
		require.NoError(t, err)
		require.Equal(t, 1, attempts)
		require.NotNil(t, lastError)
		require.Contains(t, *lastError, "500 internal server error")
		require.True(t, nextAttempt.After(time.Now().Add(30*time.Second)), "next_attempt_at must be in future")
		require.Nil(t, leaseToken, "lease token must be cleared on failure")
	})

	t.Run("6. Repeated failure: backoff grows exponentially", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-6.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET attempts = 2, next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, key string) error {
				return errors.New("temporary s3 timeout")
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.Error(t, err)
		require.True(t, processed)

		var attempts int
		var nextAttempt time.Time
		err = db.Pool.QueryRow(ctx, "SELECT attempts, next_attempt_at FROM product_media_cleanup_jobs WHERE object_key = $1", key).
			Scan(&attempts, &nextAttempt)
		require.NoError(t, err)
		require.Equal(t, 3, attempts)
		// attempt 2 backoff was base << 2 = 4 minutes
		require.True(t, nextAttempt.After(time.Now().Add(3*time.Minute)))
	})

	t.Run("7. Generation advances while DeleteObject is in flight: old worker success does NOT delete newer job", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-7.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, k string) error {
				enqErr := productsRepo.EnqueueMediaCleanup(ctx, k)
				require.NoError(t, enqErr)
				return nil
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.NoError(t, err)
		require.True(t, processed)

		var gen int64
		var leaseToken *uuid.UUID
		var nextAttempt time.Time
		err = db.Pool.QueryRow(ctx, "SELECT generation, lease_token, next_attempt_at FROM product_media_cleanup_jobs WHERE object_key = $1", key).
			Scan(&gen, &leaseToken, &nextAttempt)
		require.NoError(t, err)
		require.Equal(t, int64(2), gen, "job must survive with generation 2")
		require.Nil(t, leaseToken, "lease token must be cleared so next claim succeeds")
		require.True(t, nextAttempt.Before(time.Now().Add(5*time.Second)), "job must be promptly eligible")
	})

	t.Run("8. Generation advances while old worker fails: old backoff does NOT bury newer generation", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-8.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, k string) error {
				enqErr := productsRepo.EnqueueMediaCleanup(ctx, k)
				require.NoError(t, enqErr)
				return errors.New("network failure during delete")
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.Error(t, err)
		require.True(t, processed)

		var gen int64
		var attempts int
		var nextAttempt time.Time
		var leaseToken *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT generation, attempts, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE object_key = $1", key).
			Scan(&gen, &attempts, &nextAttempt, &leaseToken)
		require.NoError(t, err)
		require.Equal(t, int64(2), gen)
		require.Equal(t, 0, attempts, "attempts must not increment on old generation failure")
		require.True(t, nextAttempt.Before(time.Now().Add(5*time.Second)), "old failure backoff must not bury newer generation")
		require.Nil(t, leaseToken, "lease must be released")
	})

	t.Run("9. Lost lease: worker does not delete/overwrite DB job owned by another claim", func(t *testing.T) {
		key := fmt.Sprintf("%s/job-9.jpg", cleanupKeyPrefix)
		err := productsRepo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '1 minute' WHERE object_key = $1", key)
		require.NoError(t, err)

		mockStorage := &mockStorageProvider{
			deleteHook: func(ctx context.Context, k string) error {
				_, changeErr := db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET lease_token = gen_random_uuid() WHERE object_key = $1", k)
				require.NoError(t, changeErr)
				return nil
			},
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.NoError(t, err)
		require.True(t, processed)

		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "job must not be deleted when lease is lost")
	})

	t.Run("10. Deterministic no eligible job: processed=false and DeleteObject count=0", func(t *testing.T) {
		mockStorage := &mockStorageProvider{}
		// Make sure all existing jobs in this test fixture prefix have future next_attempt_at
		_, err := db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() + interval '10 days' WHERE object_key LIKE $1", cleanupKeyPrefix+"%")
		require.NoError(t, err)

		// Wrap with scoped repository seam ensuring this worker only claims fixture-scoped jobs
		scopedRepo := &scopedCleanupRepo{repo: productsRepo, allowedPrefix: cleanupKeyPrefix}
		worker := storage.NewMediaCleanupWorker(mockStorage, scopedRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

		processed, err := worker.ProcessOneMediaCleanupJob(ctx)
		require.NoError(t, err)
		require.False(t, processed, "must return processed == false when no eligible jobs exist")
		require.Equal(t, 0, len(mockStorage.getDeleteCalls()), "DeleteObject must not be called when no job is claimed")
	})
}

type mockEmptyRepo struct{}

func (m *mockEmptyRepo) ClaimMediaCleanupJob(ctx context.Context, leaseDuration time.Duration) (*products.ProductMediaCleanupJob, error) {
	return nil, nil
}
func (m *mockEmptyRepo) FinalizeMediaCleanupSuccess(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID) (products.CleanupFinalizeResult, error) {
	return products.CleanupFinalizeDeleted, nil
}
func (m *mockEmptyRepo) FinalizeMediaCleanupFailure(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID, lastError string, backoff time.Duration) error {
	return nil
}
func (m *mockEmptyRepo) ExpireStaleStagedMedia(ctx context.Context, uploadingAge, readyAge, consumedAge time.Duration, limit int) (int, error) {
	return 0, nil
}

func TestNoEligibleJobDeterministicUnit(t *testing.T) {
	t.Parallel()

	mockStorage := &mockStorageProvider{}
	emptyRepo := &mockEmptyRepo{}
	worker := storage.NewMediaCleanupWorker(mockStorage, emptyRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

	processed, err := worker.ProcessOneMediaCleanupJob(context.Background())
	require.NoError(t, err)
	require.False(t, processed, "must return processed == false when repo returns no job")
	require.Equal(t, 0, len(mockStorage.getDeleteCalls()), "DeleteObject call count must be 0")
}

func TestMediaTTLSweepExecution(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	db, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, db.Pool)

	sellerID := uuid.New()
	productID := uuid.New()
	brandID := uuid.New()
	catID := uuid.New()
	ttlPrefix := fmt.Sprintf("test-ttl-%s", uuid.New())
	sha64 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_staging WHERE object_key LIKE $1", ttlPrefix+"%"); err != nil {
			t.Errorf("cleanup staging failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE $1", ttlPrefix+"%"); err != nil {
			t.Errorf("cleanup jobs failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM products WHERE id = $1", productID); err != nil {
			t.Errorf("cleanup product failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM categories WHERE id = $1", catID); err != nil {
			t.Errorf("cleanup category failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM brands WHERE id = $1", brandID); err != nil {
			t.Errorf("cleanup brand failed: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM sellers WHERE id = $1", sellerID); err != nil {
			t.Errorf("cleanup seller failed: %v", err)
		}
		db.Close()
	})

	_, err = db.Pool.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'B', $2)", brandID, uuid.New().String())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'C', $2)", catID, uuid.New().String())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug) VALUES ($1, 'S', $2)", sellerID, uuid.New().String())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'P', $5, 'draft', 100, 'RUB')", productID, sellerID, brandID, catID, uuid.New().String())
	require.NoError(t, err)

	productsRepo := products.NewRepository(db.Pool)
	mockStorage := &mockStorageProvider{}

	insertStaging := func(status products.ProductMediaStagingStatus, key string, createdAt time.Time, consumedAt *time.Time) uuid.UUID {
		rowID := uuid.New()
		q := `INSERT INTO product_media_staging (
			id, seller_id, product_id, client_media_id, status,
			object_key, content_sha256, byte_size, width, height,
			image_url, created_at, consumed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 1024, 800, 600, 'http://test.jpg', $8, $9)`
		_, err := db.Pool.Exec(ctx, q, rowID, sellerID, productID, uuid.New(), status, key, sha64, createdAt, consumedAt)
		require.NoError(t, err)
		return rowID
	}

	t.Run("1. TTL sweep calls repository expiration using 1h/24h/24h and drains batches", func(t *testing.T) {
		staleKeys := make([]string, 5)
		for i := 0; i < 2; i++ {
			staleKeys[i] = fmt.Sprintf("%s/stale-up-%d.jpg", ttlPrefix, i)
			insertStaging(products.ProductMediaStagingUploading, staleKeys[i], time.Now().Add(-2*time.Hour), nil)
		}
		for i := 2; i < 4; i++ {
			staleKeys[i] = fmt.Sprintf("%s/stale-ready-%d.jpg", ttlPrefix, i)
			insertStaging(products.ProductMediaStagingReady, staleKeys[i], time.Now().Add(-25*time.Hour), nil)
		}
		tCons := time.Now().Add(-25 * time.Hour)
		staleKeys[4] = fmt.Sprintf("%s/stale-consumed-0.jpg", ttlPrefix)
		insertStaging(products.ProductMediaStagingConsumed, staleKeys[4], time.Now().Add(-30*time.Hour), &tCons)

		freshUpKey := fmt.Sprintf("%s/fresh-up.jpg", ttlPrefix)
		insertStaging(products.ProductMediaStagingUploading, freshUpKey, time.Now().Add(-10*time.Minute), nil)
		freshReadyKey := fmt.Sprintf("%s/fresh-ready.jpg", ttlPrefix)
		insertStaging(products.ProductMediaStagingReady, freshReadyKey, time.Now().Add(-2*time.Hour), nil)

		cfg := storage.MediaCleanupConfig{
			TTLSweepBatchSize:  2,
			TTLUploadingMaxAge: 1 * time.Hour,
			TTLReadyMaxAge:     24 * time.Hour,
			TTLConsumedMaxAge:  24 * time.Hour,
		}
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), cfg)

		totalExpired, err := worker.ExpireStaleStagedMediaSweep(ctx)
		require.NoError(t, err)
		require.Equal(t, 5, totalExpired, "must drain multiple batches until all 5 stale rows are expired")

		var freshCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE object_key IN ($1, $2)", freshUpKey, freshReadyKey).Scan(&freshCount)
		require.NoError(t, err)
		require.Equal(t, 2, freshCount, "fresh rows must not be expired")
	})

	t.Run("2. Context cancellation stops TTL sweep", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		cfg := storage.DefaultMediaCleanupConfig()
		worker := storage.NewMediaCleanupWorker(mockStorage, productsRepo, slog.Default(), cfg)

		_, err := worker.ExpireStaleStagedMediaSweep(cancelCtx)
		require.ErrorIs(t, err, context.Canceled)
	})
}

type mockTTLSweepRepo struct {
	mu           sync.Mutex
	sweepCalls   int
	failOnCalls  map[int]error
	successCount int
	sweepDone    chan struct{}
}

func (m *mockTTLSweepRepo) ClaimMediaCleanupJob(ctx context.Context, leaseDuration time.Duration) (*products.ProductMediaCleanupJob, error) {
	return nil, nil
}
func (m *mockTTLSweepRepo) FinalizeMediaCleanupSuccess(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID) (products.CleanupFinalizeResult, error) {
	return products.CleanupFinalizeDeleted, nil
}
func (m *mockTTLSweepRepo) FinalizeMediaCleanupFailure(ctx context.Context, jobID uuid.UUID, generation int64, leaseToken uuid.UUID, lastError string, backoff time.Duration) error {
	return nil
}
func (m *mockTTLSweepRepo) ExpireStaleStagedMedia(ctx context.Context, uploadingAge, readyAge, consumedAge time.Duration, limit int) (int, error) {
	m.mu.Lock()
	m.sweepCalls++
	callNum := m.sweepCalls
	err := m.failOnCalls[callNum]
	m.mu.Unlock()

	if m.sweepDone != nil {
		select {
		case m.sweepDone <- struct{}{}:
		default:
		}
	}

	if err != nil {
		return 0, err
	}
	return m.successCount, nil
}

func TestTTLSchedulerErrorRecovery(t *testing.T) {
	t.Parallel()

	sweepSignal := make(chan struct{}, 10)
	mockRepo := &mockTTLSweepRepo{
		failOnCalls: map[int]error{
			1: errors.New("simulated database sweep error"),
		},
		successCount: 3,
		sweepDone:    sweepSignal,
	}

	mockStorage := &mockStorageProvider{}
	worker := storage.NewMediaCleanupWorker(mockStorage, mockRepo, slog.Default(), storage.DefaultMediaCleanupConfig())

	tickChan := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	schedulerDone := make(chan struct{})
	go func() {
		worker.RunTTLSchedulerLoop(ctx, tickChan)
		close(schedulerDone)
	}()

	// Tick 1: trigger sweep -> returns error
	tickChan <- time.Now()
	select {
	case <-sweepSignal:
		// Call 1 finished with error
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tick 1 sweep")
	}

	// Verify scheduler did NOT terminate after error
	select {
	case <-schedulerDone:
		t.Fatal("scheduler must not terminate after first sweep error")
	default:
	}

	// Tick 2: trigger next sweep -> succeeds
	tickChan <- time.Now()
	select {
	case <-sweepSignal:
		// Call 2 finished successfully
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tick 2 sweep")
	}

	// Verify scheduler is still alive
	select {
	case <-schedulerDone:
		t.Fatal("scheduler must still be running after tick 2")
	default:
	}

	// Clean shutdown
	cancel()
	select {
	case <-schedulerDone:
		// Clean exit
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for scheduler loop shutdown")
	}

	mockRepo.mu.Lock()
	defer mockRepo.mu.Unlock()
	require.Equal(t, 2, mockRepo.sweepCalls, "expected exactly 2 sweeps to have executed")
}

func TestWorkerDeterministicShutdown(t *testing.T) {
	t.Parallel()

	mockStorage := &mockStorageProvider{}
	emptyRepo := &mockEmptyRepo{}

	cfg := storage.MediaCleanupConfig{
		PollInterval:     100 * time.Millisecond,
		TTLSweepInterval: 100 * time.Millisecond,
	}
	worker := storage.NewMediaCleanupWorker(mockStorage, emptyRepo, slog.Default(), cfg)

	workerCtx, cancel := context.WithCancel(context.Background())
	worker.Start(workerCtx)

	// Deterministic shutdown: cancel context and wait for completion signal
	cancel()

	done := make(chan struct{})
	go func() {
		worker.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: both loops exited cleanly within deadline
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker shutdown")
	}
}
