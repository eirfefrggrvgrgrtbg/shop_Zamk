package personalization_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/google/uuid"
)

// fakeIDSource implements personalization.PublishedProductIDSource for unit testing.
type fakeIDSource struct {
	mu           sync.Mutex
	pages        [][]uuid.UUID
	pageErrors   map[int]error
	queries      []*uuid.UUID
	limits       []int
	pageIndex    int
	listErr      error
	delayPerCall time.Duration
	repeatPage   bool
}

func (f *fakeIDSource) ListPublishedProductIDsAfter(ctx context.Context, afterProductID *uuid.UUID, limit int) ([]uuid.UUID, error) {
	if f.delayPerCall > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(f.delayPerCall):
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listErr != nil {
		return nil, f.listErr
	}

	var copiedCursor *uuid.UUID
	if afterProductID != nil {
		c := *afterProductID
		copiedCursor = &c
	}
	f.queries = append(f.queries, copiedCursor)
	f.limits = append(f.limits, limit)

	if f.pageErrors != nil {
		if err, ok := f.pageErrors[f.pageIndex]; ok {
			f.pageIndex++
			return nil, err
		}
	}

	if f.repeatPage && len(f.pages) > 0 {
		return f.pages[0], nil
	}

	if f.pageIndex >= len(f.pages) {
		return []uuid.UUID{}, nil
	}

	page := f.pages[f.pageIndex]
	f.pageIndex++
	return page, nil
}

// fakeGenerator implements personalization.SingleProductEmbeddingGenerator for unit testing.
type fakeGenerator struct {
	mu                     sync.Mutex
	results                map[uuid.UUID]*personalization.GenerationResult
	errors                 map[uuid.UUID]error
	panics                 map[uuid.UUID]string
	delays                 map[uuid.UUID]time.Duration
	defaultDelay           time.Duration
	invokedIDs             []uuid.UUID
	activeConcurrency      int32
	maxObservedConcurrency int32
	onStart                func(ctx context.Context, productID uuid.UUID)
}

func newFakeGenerator() *fakeGenerator {
	return &fakeGenerator{
		results: make(map[uuid.UUID]*personalization.GenerationResult),
		errors:  make(map[uuid.UUID]error),
		panics:  make(map[uuid.UUID]string),
		delays:  make(map[uuid.UUID]time.Duration),
	}
}

func (f *fakeGenerator) GenerateProductEmbedding(ctx context.Context, productID uuid.UUID) (*personalization.GenerationResult, error) {
	current := atomic.AddInt32(&f.activeConcurrency, 1)
	defer atomic.AddInt32(&f.activeConcurrency, -1)

	// Track max observed concurrency
	for {
		max := atomic.LoadInt32(&f.maxObservedConcurrency)
		if current <= max || atomic.CompareAndSwapInt32(&f.maxObservedConcurrency, max, current) {
			break
		}
	}

	f.mu.Lock()
	f.invokedIDs = append(f.invokedIDs, productID)
	delay := f.defaultDelay
	if d, ok := f.delays[productID]; ok {
		delay = d
	}
	panicMsg, shouldPanic := f.panics[productID]
	res, hasRes := f.results[productID]
	err, hasErr := f.errors[productID]
	onStartFn := f.onStart
	f.mu.Unlock()

	if onStartFn != nil {
		onStartFn(ctx, productID)
	}

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if shouldPanic {
		panic(panicMsg)
	}

	if hasErr {
		return nil, err
	}

	if hasRes {
		return res, nil
	}

	// Default fallback: valid generated outcome
	return &personalization.GenerationResult{
		ProductID:   productID,
		Outcome:     personalization.OutcomeGenerated,
		APICalled:   true,
		Saved:       true,
		ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, nil
}

func TestEmbeddingBatchProcessor_ConfigNormalization(t *testing.T) {
	t.Run("ZeroAndNegativeValues_FallbackToSafeDefaults", func(t *testing.T) {
		cfg := personalization.BatchProcessorConfig{
			BatchSize:         0,
			MaxConcurrency:    -1,
			MaxFailureSamples: 0,
		}
		cfg.Normalize()

		if cfg.BatchSize != personalization.DefaultBatchProcessorBatchSize {
			t.Errorf("expected BatchSize %d, got %d", personalization.DefaultBatchProcessorBatchSize, cfg.BatchSize)
		}
		if cfg.MaxConcurrency != personalization.DefaultBatchProcessorMaxConcurrency {
			t.Errorf("expected MaxConcurrency %d, got %d", personalization.DefaultBatchProcessorMaxConcurrency, cfg.MaxConcurrency)
		}
		if cfg.MaxFailureSamples != personalization.DefaultBatchProcessorMaxFailureSamples {
			t.Errorf("expected MaxFailureSamples %d, got %d", personalization.DefaultBatchProcessorMaxFailureSamples, cfg.MaxFailureSamples)
		}
	})

	t.Run("OversizedValues_CappedAtLimits", func(t *testing.T) {
		cfg := personalization.BatchProcessorConfig{
			BatchSize:         1000,
			MaxConcurrency:    100,
			MaxFailureSamples: 500,
		}
		cfg.Normalize()

		if cfg.BatchSize != personalization.MaxBatchProcessorBatchSize {
			t.Errorf("expected BatchSize capped at %d, got %d", personalization.MaxBatchProcessorBatchSize, cfg.BatchSize)
		}
		if cfg.MaxConcurrency != personalization.MaxBatchProcessorAllowedConcurrency {
			t.Errorf("expected MaxConcurrency capped at %d, got %d", personalization.MaxBatchProcessorAllowedConcurrency, cfg.MaxConcurrency)
		}
		if cfg.MaxFailureSamples != personalization.MaxBatchProcessorAllowedFailureSamples {
			t.Errorf("expected MaxFailureSamples capped at %d, got %d", personalization.MaxBatchProcessorAllowedFailureSamples, cfg.MaxFailureSamples)
		}
	})

	t.Run("ValidCustomValues_Preserved", func(t *testing.T) {
		cfg := personalization.BatchProcessorConfig{
			BatchSize:         25,
			MaxConcurrency:    8,
			MaxFailureSamples: 15,
		}
		cfg.Normalize()

		if cfg.BatchSize != 25 || cfg.MaxConcurrency != 8 || cfg.MaxFailureSamples != 15 {
			t.Errorf("unexpected normalized config: %+v", cfg)
		}
	})
}

func TestEmbeddingBatchProcessor_ProcessBatch_EmptyList(t *testing.T) {
	generator := newFakeGenerator()
	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, personalization.DefaultBatchProcessorConfig())

	stats, err := processor.ProcessBatch(context.Background(), []uuid.UUID{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != 0 || stats.Generated != 0 || stats.Failed != 0 {
		t.Errorf("expected zero stats, got: %+v", stats)
	}
	if len(generator.invokedIDs) != 0 {
		t.Errorf("expected 0 generator invocations, got %d", len(generator.invokedIDs))
	}
}

func TestEmbeddingBatchProcessor_ProcessSweep_EmptyCatalog(t *testing.T) {
	idSource := &fakeIDSource{pages: [][]uuid.UUID{}}
	generator := newFakeGenerator()
	processor := personalization.NewEmbeddingBatchProcessor(idSource, generator, personalization.DefaultBatchProcessorConfig())

	stats, err := processor.ProcessSweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.BatchesProcessed != 0 || stats.Scanned != 0 || stats.Generated != 0 {
		t.Errorf("expected zero sweep stats, got: %+v", stats)
	}
	if len(generator.invokedIDs) != 0 {
		t.Errorf("expected 0 generator invocations, got %d", len(generator.invokedIDs))
	}
}

func TestEmbeddingBatchProcessor_ProcessBatch_SingleProductGenerated(t *testing.T) {
	pid := uuid.New()
	generator := newFakeGenerator()
	generator.results[pid] = &personalization.GenerationResult{
		ProductID:   pid,
		Outcome:     personalization.OutcomeGenerated,
		APICalled:   true,
		Saved:       true,
		ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, personalization.DefaultBatchProcessorConfig())

	stats, err := processor.ProcessBatch(context.Background(), []uuid.UUID{pid})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != 1 {
		t.Errorf("expected Scanned 1, got %d", stats.Scanned)
	}
	if stats.Generated != 1 {
		t.Errorf("expected Generated 1, got %d", stats.Generated)
	}
	if stats.Failed != 0 || len(stats.Failures) != 0 {
		t.Errorf("expected zero failures, got %d failures", stats.Failed)
	}
}

func TestEmbeddingBatchProcessor_ProcessBatch_RespectsMaxConcurrency(t *testing.T) {
	const totalProducts = 12
	const maxConcurrency = 3

	pids := make([]uuid.UUID, totalProducts)
	for i := range pids {
		pids[i] = uuid.New()
	}

	generator := newFakeGenerator()
	generator.defaultDelay = 25 * time.Millisecond

	cfg := personalization.BatchProcessorConfig{
		BatchSize:         totalProducts,
		MaxConcurrency:    maxConcurrency,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, cfg)

	start := time.Now()
	stats, err := processor.ProcessBatch(context.Background(), pids)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != totalProducts {
		t.Errorf("expected Scanned %d, got %d", totalProducts, stats.Scanned)
	}
	if stats.Generated != totalProducts {
		t.Errorf("expected Generated %d, got %d", totalProducts, stats.Generated)
	}

	observedConcurrency := atomic.LoadInt32(&generator.maxObservedConcurrency)
	if observedConcurrency > maxConcurrency {
		t.Fatalf("VIOLATION: observed concurrency %d exceeded MaxConcurrency limit %d", observedConcurrency, maxConcurrency)
	}
	if observedConcurrency <= 1 {
		t.Fatalf("expected concurrent execution (>1), but observed max concurrency was %d", observedConcurrency)
	}

	// 12 items with concurrency 3 and 25ms delay should take ~100ms, definitely < 300ms (which sequential would take)
	if elapsed < 50*time.Millisecond || elapsed > 1*time.Second {
		t.Logf("concurrency execution time: %v (observed concurrency: %d)", elapsed, observedConcurrency)
	}
}

func TestEmbeddingBatchProcessor_ProcessBatch_MixOfOutcomes(t *testing.T) {
	prodGenerated := uuid.New()
	prodSkippedCur := uuid.New()
	prodSkippedNotElig := uuid.New()
	prodDiscardedChanged := uuid.New()
	prodDiscardedNotElig := uuid.New()
	prodFailed := uuid.New()

	generator := newFakeGenerator()
	generator.results[prodGenerated] = &personalization.GenerationResult{
		ProductID: prodGenerated, Outcome: personalization.OutcomeGenerated, APICalled: true, Saved: true,
	}
	generator.results[prodSkippedCur] = &personalization.GenerationResult{
		ProductID: prodSkippedCur, Outcome: personalization.OutcomeSkippedCurrent, APICalled: false, Saved: false,
	}
	generator.results[prodSkippedNotElig] = &personalization.GenerationResult{
		ProductID: prodSkippedNotElig, Outcome: personalization.OutcomeSkippedNotEligible, APICalled: false, Saved: false,
	}
	generator.results[prodDiscardedChanged] = &personalization.GenerationResult{
		ProductID: prodDiscardedChanged, Outcome: personalization.OutcomeDiscardedContentChanged, APICalled: true, Saved: false,
	}
	generator.results[prodDiscardedNotElig] = &personalization.GenerationResult{
		ProductID: prodDiscardedNotElig, Outcome: personalization.OutcomeDiscardedProductNotEligible, APICalled: true, Saved: false,
	}
	generator.errors[prodFailed] = errors.New("upstream provider connection reset")

	pids := []uuid.UUID{
		prodGenerated,
		prodSkippedCur,
		prodSkippedNotElig,
		prodDiscardedChanged,
		prodDiscardedNotElig,
		prodFailed,
	}

	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, personalization.DefaultBatchProcessorConfig())

	stats, err := processor.ProcessBatch(context.Background(), pids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != 6 {
		t.Errorf("expected Scanned 6, got %d", stats.Scanned)
	}
	if stats.Generated != 1 {
		t.Errorf("expected Generated 1, got %d", stats.Generated)
	}
	if stats.SkippedCurrent != 1 {
		t.Errorf("expected SkippedCurrent 1, got %d", stats.SkippedCurrent)
	}
	if stats.SkippedNotEligible != 1 {
		t.Errorf("expected SkippedNotEligible 1, got %d", stats.SkippedNotEligible)
	}
	if stats.DiscardedContentChanged != 1 {
		t.Errorf("expected DiscardedContentChanged 1, got %d", stats.DiscardedContentChanged)
	}
	if stats.DiscardedNotEligible != 1 {
		t.Errorf("expected DiscardedNotEligible 1, got %d", stats.DiscardedNotEligible)
	}
	if stats.Failed != 1 {
		t.Errorf("expected Failed 1, got %d", stats.Failed)
	}

	// Verify exact sum invariant: Scanned == sum of all outcome buckets
	sum := stats.Generated + stats.SkippedCurrent + stats.SkippedNotEligible +
		stats.DiscardedContentChanged + stats.DiscardedNotEligible + stats.Failed
	if stats.Scanned != sum {
		t.Errorf("invariant violation: Scanned (%d) != sum of outcomes (%d)", stats.Scanned, sum)
	}

	if len(stats.Failures) != 1 {
		t.Fatalf("expected 1 failure sample, got %d", len(stats.Failures))
	}
	if stats.Failures[0].ProductID != prodFailed {
		t.Errorf("expected failure product ID %s, got %s", prodFailed, stats.Failures[0].ProductID)
	}
	if stats.Failures[0].Error != "upstream provider connection reset" {
		t.Errorf("expected failure error message 'upstream provider connection reset', got %q", stats.Failures[0].Error)
	}
}

func TestEmbeddingBatchProcessor_ProcessBatch_IsolatedFailureDoesNotHalt(t *testing.T) {
	pids := make([]uuid.UUID, 5)
	for i := range pids {
		pids[i] = uuid.New()
	}

	generator := newFakeGenerator()
	failingPID := pids[2]
	generator.errors[failingPID] = errors.New("isolated 500 error")

	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, personalization.DefaultBatchProcessorConfig())

	stats, err := processor.ProcessBatch(context.Background(), pids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != 5 {
		t.Errorf("expected Scanned 5, got %d", stats.Scanned)
	}
	if stats.Generated != 4 {
		t.Errorf("expected Generated 4, got %d", stats.Generated)
	}
	if stats.Failed != 1 {
		t.Errorf("expected Failed 1, got %d", stats.Failed)
	}
	if len(stats.Failures) != 1 || stats.Failures[0].ProductID != failingPID {
		t.Errorf("expected failure sample for %s, got %+v", failingPID, stats.Failures)
	}
}

func TestEmbeddingBatchProcessor_ProcessBatch_ContextCancellation(t *testing.T) {
	const totalProducts = 20
	pids := make([]uuid.UUID, totalProducts)
	for i := range pids {
		pids[i] = uuid.New()
	}

	generator := newFakeGenerator()
	generator.defaultDelay = 40 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	cfg := personalization.BatchProcessorConfig{
		BatchSize:         totalProducts,
		MaxConcurrency:    2,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, cfg)

	// Cancel context shortly after start
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	stats, err := processor.ProcessBatch(ctx, pids)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	// Should have stopped launching before exhausting all 20 items
	if stats.Scanned >= totalProducts {
		t.Errorf("expected Scanned < %d due to cancellation, got %d", totalProducts, stats.Scanned)
	}
}

func TestEmbeddingBatchProcessor_ProcessSweep_KeysetPaginationAcrossPages(t *testing.T) {
	const totalProducts = 7
	const batchSize = 3

	pids := make([]uuid.UUID, totalProducts)
	for i := range pids {
		pids[i] = uuid.New()
	}

	// Pages:
	// Page 0: [pid0, pid1, pid2]
	// Page 1: [pid3, pid4, pid5]
	// Page 2: [pid6]
	// Page 3: []
	idSource := &fakeIDSource{
		pages: [][]uuid.UUID{
			pids[0:3],
			pids[3:6],
			pids[6:7],
			{},
		},
	}

	generator := newFakeGenerator()
	cfg := personalization.BatchProcessorConfig{
		BatchSize:         batchSize,
		MaxConcurrency:    2,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(idSource, generator, cfg)

	stats, err := processor.ProcessSweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.BatchesProcessed != 3 {
		t.Errorf("expected 3 batches processed, got %d", stats.BatchesProcessed)
	}
	if stats.Scanned != totalProducts {
		t.Errorf("expected Scanned %d, got %d", totalProducts, stats.Scanned)
	}
	if stats.Generated != totalProducts {
		t.Errorf("expected Generated %d, got %d", totalProducts, stats.Generated)
	}
	if stats.Failed != 0 {
		t.Errorf("expected Failed 0, got %d", stats.Failed)
	}

	// Verify keyset cursor sequence:
	// Query 0: cursor = nil
	// Query 1: cursor = &pids[2]
	// Query 2: cursor = &pids[5]
	// Query 3: cursor = &pids[6]
	if len(idSource.queries) != 4 {
		t.Fatalf("expected 4 queries to idSource, got %d", len(idSource.queries))
	}
	if idSource.queries[0] != nil {
		t.Errorf("expected first query cursor to be nil, got: %v", idSource.queries[0])
	}
	if idSource.queries[1] == nil || *idSource.queries[1] != pids[2] {
		t.Errorf("expected second query cursor to be %s, got: %v", pids[2], idSource.queries[1])
	}
	if idSource.queries[2] == nil || *idSource.queries[2] != pids[5] {
		t.Errorf("expected third query cursor to be %s, got: %v", pids[5], idSource.queries[2])
	}
	if idSource.queries[3] == nil || *idSource.queries[3] != pids[6] {
		t.Errorf("expected fourth query cursor to be %s, got: %v", pids[6], idSource.queries[3])
	}
}

func TestEmbeddingBatchProcessor_ProcessSweep_ContextCancellation(t *testing.T) {
	pids := make([]uuid.UUID, 10)
	for i := range pids {
		pids[i] = uuid.New()
	}

	idSource := &fakeIDSource{
		pages: [][]uuid.UUID{
			pids[0:5],
			pids[5:10],
		},
		delayPerCall: 50 * time.Millisecond,
	}

	generator := newFakeGenerator()
	ctx, cancel := context.WithCancel(context.Background())

	cfg := personalization.BatchProcessorConfig{
		BatchSize:         5,
		MaxConcurrency:    2,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(idSource, generator, cfg)

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err := processor.ProcessSweep(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestEmbeddingBatchProcessor_FailureSampleBounded(t *testing.T) {
	const totalFailures = 25
	const maxSamples = 5

	pids := make([]uuid.UUID, totalFailures)
	for i := range pids {
		pids[i] = uuid.New()
	}

	generator := newFakeGenerator()
	for _, pid := range pids {
		generator.errors[pid] = fmt.Errorf("error for product %s", pid)
	}

	cfg := personalization.BatchProcessorConfig{
		BatchSize:         totalFailures,
		MaxConcurrency:    5,
		MaxFailureSamples: maxSamples,
	}
	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, cfg)

	stats, err := processor.ProcessBatch(context.Background(), pids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != totalFailures {
		t.Errorf("expected Scanned %d, got %d", totalFailures, stats.Scanned)
	}
	if stats.Failed != totalFailures {
		t.Errorf("expected Failed %d, got %d", totalFailures, stats.Failed)
	}
	if len(stats.Failures) != maxSamples {
		t.Errorf("expected Failures capped at %d, got %d", maxSamples, len(stats.Failures))
	}
}

func TestEmbeddingBatchProcessor_PanicRecovery(t *testing.T) {
	pid1 := uuid.New()
	pidPanic := uuid.New()
	pid3 := uuid.New()

	generator := newFakeGenerator()
	generator.panics[pidPanic] = "unhandled nil pointer in custom plugin"

	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, personalization.DefaultBatchProcessorConfig())

	stats, err := processor.ProcessBatch(context.Background(), []uuid.UUID{pid1, pidPanic, pid3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != 3 {
		t.Errorf("expected Scanned 3, got %d", stats.Scanned)
	}
	if stats.Generated != 2 {
		t.Errorf("expected Generated 2, got %d", stats.Generated)
	}
	if stats.Failed != 1 {
		t.Errorf("expected Failed 1, got %d", stats.Failed)
	}
	if len(stats.Failures) != 1 {
		t.Fatalf("expected 1 failure sample, got %d", len(stats.Failures))
	}
	if stats.Failures[0].ProductID != pidPanic {
		t.Errorf("expected failure product ID %s, got %s", pidPanic, stats.Failures[0].ProductID)
	}
}

func TestEmbeddingBatchProcessor_NilDependencies(t *testing.T) {
	cfg := personalization.DefaultBatchProcessorConfig()

	t.Run("NilGenerator_ProcessBatchReturnsError", func(t *testing.T) {
		proc := personalization.NewEmbeddingBatchProcessor(nil, nil, cfg)
		_, err := proc.ProcessBatch(context.Background(), []uuid.UUID{uuid.New()})
		if !errors.Is(err, personalization.ErrBatchProcessorNilGenerator) {
			t.Fatalf("expected ErrBatchProcessorNilGenerator, got: %v", err)
		}
	})

	t.Run("NilSource_ProcessSweepReturnsError", func(t *testing.T) {
		proc := personalization.NewEmbeddingBatchProcessor(nil, newFakeGenerator(), cfg)
		_, err := proc.ProcessSweep(context.Background())
		if !errors.Is(err, personalization.ErrBatchProcessorNilSource) {
			t.Fatalf("expected ErrBatchProcessorNilSource, got: %v", err)
		}
	})
}

func TestEmbeddingBatchProcessor_ProcessSweep_DBIntegration(t *testing.T) {
	pool, repo := setupEmbeddingTestDB(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	generator := newFakeGenerator()
	cfg := personalization.BatchProcessorConfig{
		BatchSize:         200,
		MaxConcurrency:    10,
		MaxFailureSamples: 10,
	}

	processor := personalization.NewEmbeddingBatchProcessor(repo, generator, cfg)

	stats, err := processor.ProcessSweep(ctx)
	if err != nil {
		t.Fatalf("unexpected error during DB sweep: %v", err)
	}

	if stats.BatchesProcessed == 0 {
		t.Errorf("expected at least 1 batch processed, got 0")
	}
	if stats.Scanned == 0 {
		t.Errorf("expected scanned > 0, got 0")
	}
	if stats.Generated != stats.Scanned {
		t.Errorf("expected Generated (%d) == Scanned (%d)", stats.Generated, stats.Scanned)
	}
	if stats.Failed != 0 {
		t.Errorf("expected 0 failures, got %d", stats.Failed)
	}
}

type memoryIDSource struct {
	mu       sync.Mutex
	products []uuid.UUID
}

func (m *memoryIDSource) ListPublishedProductIDsAfter(ctx context.Context, afterProductID *uuid.UUID, limit int) ([]uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []uuid.UUID
	for _, id := range m.products {
		if afterProductID == nil || id.String() > afterProductID.String() {
			result = append(result, id)
			if len(result) == limit {
				break
			}
		}
	}
	if result == nil {
		return []uuid.UUID{}, nil
	}
	return result, nil
}

// 1. SAME PROCESSOR CONCURRENT USE
func TestEmbeddingBatchProcessor_ConcurrentSweeps_SameInstance(t *testing.T) {
	const totalProducts = 20
	pids := make([]uuid.UUID, totalProducts)
	for i := range pids {
		pids[i] = uuid.New()
	}
	sort.Slice(pids, func(i, j int) bool {
		return pids[i].String() < pids[j].String()
	})

	idSource := &memoryIDSource{products: pids}
	generator := newFakeGenerator()
	generator.defaultDelay = 5 * time.Millisecond

	cfg := personalization.BatchProcessorConfig{
		BatchSize:         5,
		MaxConcurrency:    3,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(idSource, generator, cfg)

	const concurrentSweeps = 3
	var wg sync.WaitGroup
	errs := make([]error, concurrentSweeps)
	results := make([]*personalization.SweepStats, concurrentSweeps)

	for i := 0; i < concurrentSweeps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := processor.ProcessSweep(context.Background())
			errs[idx] = err
			results[idx] = res
		}(i)
	}

	wg.Wait()

	for i := 0; i < concurrentSweeps; i++ {
		if errs[i] != nil {
			t.Fatalf("sweep %d failed unexpectedly: %v", i, errs[i])
		}
		stats := results[i]
		if stats == nil {
			t.Fatalf("sweep %d returned nil stats", i)
		}
		if stats.Scanned != totalProducts {
			t.Errorf("sweep %d: expected scanned %d, got %d", i, totalProducts, stats.Scanned)
		}
		if stats.Generated != totalProducts {
			t.Errorf("sweep %d: expected generated %d, got %d", i, totalProducts, stats.Generated)
		}
		if stats.Failed != 0 {
			t.Errorf("sweep %d: expected failed 0, got %d", i, stats.Failed)
		}
		if stats.BatchesProcessed != 4 {
			t.Errorf("sweep %d: expected 4 batches processed, got %d", i, stats.BatchesProcessed)
		}
	}
}

// 2. SOURCE / PAGINATION FAILURE
func TestEmbeddingBatchProcessor_ProcessSweep_SourceErrorOnSecondPage(t *testing.T) {
	page1 := []uuid.UUID{uuid.New(), uuid.New()}
	page3 := []uuid.UUID{uuid.New(), uuid.New()}

	expectedErr := errors.New("simulated database disconnect on page 2")
	idSource := &fakeIDSource{
		pages: [][]uuid.UUID{
			page1,
			page3,
		},
		pageErrors: map[int]error{
			1: expectedErr,
		},
	}

	generator := newFakeGenerator()
	cfg := personalization.BatchProcessorConfig{
		BatchSize:         2,
		MaxConcurrency:    2,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(idSource, generator, cfg)

	stats, err := processor.ProcessSweep(context.Background())
	if err == nil {
		t.Fatalf("expected error from source on page 2, got nil")
	}
	if !strings.Contains(err.Error(), "simulated database disconnect on page 2") {
		t.Fatalf("expected error to contain source error, got: %v", err)
	}

	if stats == nil {
		t.Fatalf("expected non-nil partial stats")
	}
	if stats.BatchesProcessed != 1 {
		t.Errorf("expected 1 batch processed, got %d", stats.BatchesProcessed)
	}
	if stats.Scanned != 2 {
		t.Errorf("expected 2 scanned items, got %d", stats.Scanned)
	}
	if stats.Generated != 2 {
		t.Errorf("expected 2 generated items, got %d", stats.Generated)
	}

	if idSource.pageIndex != 2 {
		t.Errorf("expected exactly 2 page calls, got %d (page 3 was erroneously queried)", idSource.pageIndex)
	}
}

// 3. NON-ADVANCING CURSOR GUARD
func TestEmbeddingBatchProcessor_ProcessSweep_NonAdvancingCursorGuard(t *testing.T) {
	pid := uuid.New()
	idSource := &fakeIDSource{
		pages:      [][]uuid.UUID{{pid}},
		repeatPage: true,
	}

	generator := newFakeGenerator()
	processor := personalization.NewEmbeddingBatchProcessor(idSource, generator, personalization.DefaultBatchProcessorConfig())

	stats, err := processor.ProcessSweep(context.Background())
	if err == nil {
		t.Fatalf("expected ErrBatchProcessorNonAdvancingCursor, got nil")
	}
	if !errors.Is(err, personalization.ErrBatchProcessorNonAdvancingCursor) {
		t.Fatalf("expected ErrBatchProcessorNonAdvancingCursor, got: %v", err)
	}

	if stats == nil {
		t.Fatalf("expected non-nil stats")
	}
	if stats.BatchesProcessed != 1 {
		t.Errorf("expected 1 batch processed before loop guard triggered, got %d", stats.BatchesProcessed)
	}
	if stats.Scanned != 1 {
		t.Errorf("expected 1 scanned, got %d", stats.Scanned)
	}
}

// 4. SERIAL CONCURRENCY (MaxConcurrency = 1)
func TestEmbeddingBatchProcessor_ProcessBatch_SerialConcurrency(t *testing.T) {
	const totalProducts = 8
	pids := make([]uuid.UUID, totalProducts)
	for i := range pids {
		pids[i] = uuid.New()
	}

	generator := newFakeGenerator()
	generator.defaultDelay = 15 * time.Millisecond

	cfg := personalization.BatchProcessorConfig{
		BatchSize:         totalProducts,
		MaxConcurrency:    1,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, cfg)

	stats, err := processor.ProcessBatch(context.Background(), pids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != totalProducts || stats.Generated != totalProducts {
		t.Fatalf("expected %d scanned and generated, got %+v", totalProducts, stats)
	}

	observedConcurrency := atomic.LoadInt32(&generator.maxObservedConcurrency)
	if observedConcurrency != 1 {
		t.Fatalf("VIOLATION: observed concurrency was %d, expected strictly 1 with MaxConcurrency=1", observedConcurrency)
	}
}

// 5. BATCH SIZE PROOF
func TestEmbeddingBatchProcessor_ProcessSweep_BatchSizePassedToSource(t *testing.T) {
	const configuredBatchSize = 37

	pids := make([]uuid.UUID, 50)
	for i := range pids {
		pids[i] = uuid.New()
	}

	idSource := &fakeIDSource{
		pages: [][]uuid.UUID{
			pids[0:37],
			pids[37:50],
			{},
		},
	}

	generator := newFakeGenerator()
	cfg := personalization.BatchProcessorConfig{
		BatchSize:         configuredBatchSize,
		MaxConcurrency:    3,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(idSource, generator, cfg)

	stats, err := processor.ProcessSweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Scanned != 50 {
		t.Errorf("expected 50 scanned, got %d", stats.Scanned)
	}

	if len(idSource.limits) != 3 {
		t.Fatalf("expected 3 source queries, got %d", len(idSource.limits))
	}
	for i, limit := range idSource.limits {
		if limit != configuredBatchSize {
			t.Errorf("query %d: expected limit %d, got %d", i, configuredBatchSize, limit)
		}
	}
}

// 6. CANCELLATION WHILE ACTIVE JOB IN-FLIGHT
func TestEmbeddingBatchProcessor_ProcessBatch_CancellationWithActiveJob(t *testing.T) {
	const totalProducts = 10
	pids := make([]uuid.UUID, totalProducts)
	for i := range pids {
		pids[i] = uuid.New()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	generator := newFakeGenerator()
	firstJobStarted := make(chan struct{}, 1)
	var activeSawCancellation atomic.Bool

	generator.onStart = func(callCtx context.Context, pid uuid.UUID) {
		select {
		case firstJobStarted <- struct{}{}:
			cancel()
		default:
		}

		select {
		case <-callCtx.Done():
			activeSawCancellation.Store(true)
		case <-time.After(100 * time.Millisecond):
		}
	}

	cfg := personalization.BatchProcessorConfig{
		BatchSize:         totalProducts,
		MaxConcurrency:    2,
		MaxFailureSamples: 10,
	}
	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, cfg)

	stats, err := processor.ProcessBatch(ctx, pids)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got: %v", err)
	}

	if !activeSawCancellation.Load() {
		t.Errorf("expected active generator job to receive cancelled context")
	}

	if stats.Scanned >= totalProducts {
		t.Errorf("expected Scanned < %d due to cancellation halting new launches, got %d", totalProducts, stats.Scanned)
	}
}

// 7. FAILURE DIAGNOSTICS BOUNDED AND SCRUBBED
func TestEmbeddingBatchProcessor_ProcessBatch_FailureDiagnosticsBoundedAndScrubbed(t *testing.T) {
	const totalProducts = 15
	pids := make([]uuid.UUID, totalProducts)
	for i := range pids {
		pids[i] = uuid.New()
	}

	generator := newFakeGenerator()
	longSensitiveError := "upstream provider error: sql='SELECT * FROM users WHERE secret_token=\"sk-proj-xyz\"' " +
		"vector=[0.123456789, 0.987654321, 0.111111111, 0.222222222, 0.333333333, 0.444444444, " +
		"0.555555555, 0.666666666, 0.777777777, 0.888888888, 0.999999999, 0.000000000, 0.121212121, " +
		"0.232323232, 0.343434343, 0.454545454, 0.565656565, 0.676767676, 0.787878787, 0.898989898] " +
		"status=500 detail=internal_database_unreachable_during_high_load"

	for _, pid := range pids {
		generator.errors[pid] = errors.New(longSensitiveError)
	}

	const maxSamples = 3
	cfg := personalization.BatchProcessorConfig{
		BatchSize:         totalProducts,
		MaxConcurrency:    4,
		MaxFailureSamples: maxSamples,
	}
	processor := personalization.NewEmbeddingBatchProcessor(nil, generator, cfg)

	stats, err := processor.ProcessBatch(context.Background(), pids)
	if err != nil {
		t.Fatalf("unexpected error from ProcessBatch: %v", err)
	}

	if stats.Failed != totalProducts {
		t.Errorf("expected Failed %d, got %d", totalProducts, stats.Failed)
	}

	if len(stats.Failures) != maxSamples {
		t.Fatalf("expected exactly %d failure samples, got %d", maxSamples, len(stats.Failures))
	}

	for i, f := range stats.Failures {
		if len(f.Error) > 256 {
			t.Errorf("failure %d length %d exceeds max 256 characters", i, len(f.Error))
		}
		if !strings.HasSuffix(f.Error, "...") {
			t.Errorf("failure %d expected trailing '...' truncation marker, got: %q", i, f.Error)
		}
	}
}
