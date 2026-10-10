package vision_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func getIsolatedTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	// BLOCKER 12: DB Guard before any fixture mutation
	testutil.AssertTestDatabase(t, pool)

	return pool
}

type testFixture struct {
	sellerID   uuid.UUID
	userID     uuid.UUID
	adminID    uuid.UUID
	categoryID uuid.UUID
	productID  uuid.UUID
	imageID    uuid.UUID
	objectKey  string
	snap       vision.ProductVisionSnapshot
	snapHash   string
}

func setupTestFixture(t *testing.T, pool *pgxpool.Pool) testFixture {
	t.Helper()
	ctx := context.Background()

	sellerID := uuid.New()
	brandName := fmt.Sprintf("Brand-%s", sellerID.String()[:8])
	sellerSlug := fmt.Sprintf("brand-slug-%s", sellerID.String()[:8])
	_, err := pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', now(), now())
	`, sellerID, brandName, sellerSlug, "test@test.com")
	require.NoError(t, err)

	userID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'seller', 'Seller User', now(), now())
	`, userID, fmt.Sprintf("seller-%s@test.com", userID.String()[:8]))
	require.NoError(t, err)

	adminID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'admin', 'Admin User', now(), now())
	`, adminID, fmt.Sprintf("admin-%s@test.com", adminID.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, userID)
	require.NoError(t, err)

	categoryID := uuid.New()
	catSlug := fmt.Sprintf("cat-%s", categoryID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Dresses', $2, true, now(), now())
	`, categoryID, catSlug)
	require.NoError(t, err)

	productID := uuid.New()
	prodSlug := fmt.Sprintf("prod-%s", productID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, description, status, source, currency, price_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'Evening Silk Dress', $4, 'Fine silk dress', 'draft', 'seller', 'RUB', 15000, now(), now())
	`, productID, sellerID, categoryID, prodSlug)
	require.NoError(t, err)

	imageID := uuid.New()
	objectKey := fmt.Sprintf("products/%s/%s/%s.jpg", sellerID, productID, imageID)
	_, err = pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, object_key, sort_order, is_main, created_at)
		VALUES ($1, $2, $3, $4, 1, true, now())
	`, imageID, productID, "http://example.com/img.jpg", objectKey)
	require.NoError(t, err)

	// Add an active variant
	varID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, is_active, option_values, created_at, updated_at)
		VALUES ($1, $2, true, '{"Size": "M"}'::jsonb, now(), now())
	`, varID, productID)
	require.NoError(t, err)

	snap := vision.ProductVisionSnapshot{
		ProductID:          productID,
		Title:              "Evening Silk Dress",
		Description:        "Fine silk dress",
		DeclaredCategoryID: categoryID,
		DeclaredCategory:   "Dresses",
		DeclaredOptions:    map[string]string{"Size": "M"},
		Images: []vision.SnapshotImage{
			{
				ImageID:   imageID,
				ObjectKey: objectKey,
				SortOrder: 1,
				IsMain:    true,
			},
		},
	}
	snapHash := vision.ComputeSnapshotHash(snap)

	return testFixture{
		sellerID:   sellerID,
		userID:     userID,
		adminID:    adminID,
		categoryID: categoryID,
		productID:  productID,
		imageID:    imageID,
		objectKey:  objectKey,
		snap:       snap,
		snapHash:   snapHash,
	}
}

func makeTestParams(fix testFixture, modelID string) vision.ScheduleVisionRunParams {
	return vision.ScheduleVisionRunParams{
		RunID:    uuid.New(),
		Snapshot: fix.snap,
		TaxonomyContext: vision.TaxonomyContext{
			Categories: []vision.TaxonomyItem{{ID: fix.categoryID, Label: "Dresses"}},
		},
		Provider:          "cloudru",
		ModelID:           modelID,
		PromptVersion:     "v1",
		SchemaVersion:     "v1",
		VocabularyVersion: "v1",
	}
}

func TestStore_EnsureVisionRun_SequentialAndConcurrent(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	params := makeTestParams(fix, "qwen-test")

	// Test A: Sequential duplicate Ensure → one row, created=true then created=false
	firstRun, created1, err := store.EnsureVisionRun(ctx, params)
	require.NoError(t, err)
	require.True(t, created1)
	require.Equal(t, params.RunID, firstRun.ID)
	require.Equal(t, fix.snap.Title, firstRun.SnapshotJSON.Title)

	secondRun, created2, err := store.EnsureVisionRun(ctx, params)
	require.NoError(t, err)
	require.False(t, created2)
	require.Equal(t, firstRun.ID, secondRun.ID)

	// Test B: Concurrent Ensure from multiple goroutines → one DB row, all callers resolve same run ID
	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]*vision.VisionRun, numGoroutines)
	createdFlags := make([]bool, numGoroutines)
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p := params
			p.RunID = uuid.New()
			p.ModelID = "qwen-test-concurrent"
			res, created, err := store.EnsureVisionRun(ctx, p)
			results[idx] = res
			createdFlags[idx] = created
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	var canonID uuid.UUID
	createdCount := 0
	for i := 0; i < numGoroutines; i++ {
		require.NoError(t, errs[i])
		require.NotNil(t, results[i])
		if createdFlags[i] {
			createdCount++
			canonID = results[i].ID
		}
	}
	require.Equal(t, 1, createdCount, "Exactly one caller must receive created=true")
	for i := 0; i < numGoroutines; i++ {
		require.Equal(t, canonID, results[i].ID, "All concurrent callers must resolve to canonical run ID")
	}
}

func TestStore_IdentityIntegrity(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	taxCtx := vision.TaxonomyContext{
		Categories: []vision.TaxonomyItem{{ID: fix.categoryID, Label: "Dresses"}},
	}

	// 1. Same canonical input → same snapshot hash, context hash, and idempotency key
	params1 := vision.ScheduleVisionRunParams{
		Snapshot:        fix.snap,
		TaxonomyContext: taxCtx,
		Provider:        "cloudru",
		ModelID:         "qwen",
	}
	run1 := vision.BuildCanonicalVisionRun(params1)
	run2 := vision.BuildCanonicalVisionRun(params1)
	require.Equal(t, run1.SnapshotHash, run2.SnapshotHash)
	require.Equal(t, run1.TaxonomyContextHash, run2.TaxonomyContextHash)
	require.Equal(t, run1.IdempotencyKey, run2.IdempotencyKey)

	// 2. Caller cannot persist mismatched snapshot_json A + snapshot_hash B
	badSnapRun := run1
	badSnapRun.ID = uuid.New()
	badSnapRun.SnapshotHash = "tampered-snapshot-hash"
	_, _, err := store.EnsureRawVisionRun(ctx, badSnapRun)
	require.ErrorIs(t, err, vision.ErrMismatchedSnapshotHash, "Mismatched snapshot hash must be rejected")

	// 3. Caller cannot persist mismatched context A + taxonomy_context_hash B
	badTaxRun := run1
	badTaxRun.ID = uuid.New()
	badTaxRun.TaxonomyContext = &taxCtx
	badTaxRun.TaxonomyContextHash = "tampered-taxonomy-hash"
	_, _, err = store.EnsureRawVisionRun(ctx, badTaxRun)
	require.ErrorIs(t, err, vision.ErrMismatchedTaxonomyHash, "Mismatched taxonomy context hash must be rejected")

	// 4. Provider change changes idempotency key
	paramsProvider := params1
	paramsProvider.Provider = "different-provider"
	runProvider := vision.BuildCanonicalVisionRun(paramsProvider)
	require.NotEqual(t, run1.IdempotencyKey, runProvider.IdempotencyKey)

	// 5. Model change changes idempotency key
	paramsModel := params1
	paramsModel.ModelID = "different-model"
	runModel := vision.BuildCanonicalVisionRun(paramsModel)
	require.NotEqual(t, run1.IdempotencyKey, runModel.IdempotencyKey)

	// 6. Prompt/schema/vocabulary change changes key
	paramsPrompt := params1
	paramsPrompt.PromptVersion = "v2"
	runPrompt := vision.BuildCanonicalVisionRun(paramsPrompt)
	require.NotEqual(t, run1.IdempotencyKey, runPrompt.IdempotencyKey)

	paramsSchema := params1
	paramsSchema.SchemaVersion = "v2"
	runSchema := vision.BuildCanonicalVisionRun(paramsSchema)
	require.NotEqual(t, run1.IdempotencyKey, runSchema.IdempotencyKey)

	paramsVocab := params1
	paramsVocab.VocabularyVersion = "v2"
	runVocab := vision.BuildCanonicalVisionRun(paramsVocab)
	require.NotEqual(t, run1.IdempotencyKey, runVocab.IdempotencyKey)

	// 7. Product/media change changes snapshot hash and key
	snapModified := fix.snap
	snapModified.Title = "Altered Title"
	paramsMod := params1
	paramsMod.Snapshot = snapModified
	runMod := vision.BuildCanonicalVisionRun(paramsMod)
	require.NotEqual(t, run1.SnapshotHash, runMod.SnapshotHash)
	require.NotEqual(t, run1.IdempotencyKey, runMod.IdempotencyKey)
}

func TestStore_Claiming_ConcurrencyAndExclusion(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	// Drain any existing pending runs from previous tests so claimer queue is clean
	_, _ = store.ClaimPendingRuns(ctx, 1000)

	// Test C: Two concurrent claimers with ONE pending run → exactly one receives it
	p1 := makeTestParams(fix, "qwen-claim-c")
	run1, _, err := store.EnsureVisionRun(ctx, p1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	claimResults := make([][]vision.VisionRun, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			claimed, claimErr := store.ClaimPendingRuns(ctx, 1)
			if claimErr == nil {
				claimResults[idx] = claimed
			}
		}(i)
	}
	wg.Wait()

	totalClaimedRun1 := 0
	for i := 0; i < 2; i++ {
		for _, r := range claimResults[i] {
			if r.ID == run1.ID {
				totalClaimedRun1++
			}
		}
	}
	require.Equal(t, 1, totalClaimedRun1, "Exactly one claimer should have claimed run1")

	// Test D: Two pending runs + two claimers → both can progress without duplicate claim
	p2A := makeTestParams(fix, "qwen-claim-d-a")
	run2A, _, err := store.EnsureVisionRun(ctx, p2A)
	require.NoError(t, err)

	p2B := makeTestParams(fix, "qwen-claim-d-b")
	run2B, _, err := store.EnsureVisionRun(ctx, p2B)
	require.NoError(t, err)

	claimResultsD := make([][]vision.VisionRun, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			claimed, _ := store.ClaimPendingRuns(ctx, 1)
			claimResultsD[idx] = claimed
		}(i)
	}
	wg.Wait()

	claimedSet := make(map[uuid.UUID]bool)
	for i := 0; i < 2; i++ {
		for _, r := range claimResultsD[i] {
			if r.ID == run2A.ID || r.ID == run2B.ID {
				require.False(t, claimedSet[r.ID], "Run must not be claimed twice")
				claimedSet[r.ID] = true
			}
		}
	}
	require.Equal(t, 2, len(claimedSet), "Both runs should be claimed by the two workers")

	// Test E & F: Terminal succeeded and invalid_output cannot be claimed
	pE := makeTestParams(fix, "qwen-claim-e")
	runE, _, err := store.EnsureVisionRun(ctx, pE)
	require.NoError(t, err)

	// claim it to processing then complete
	claimed, err := store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	foundClaimed := false
	for _, r := range claimed {
		if r.ID == runE.ID {
			foundClaimed = true
		}
	}
	require.True(t, foundClaimed)

	obs := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.regular")}
	eval := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, runE.ID, obs, eval, nil, nil, nil)
	require.NoError(t, err)

	// Attempt claim: must not be claimed
	claimedAgain, err := store.ClaimPendingRuns(ctx, 100)
	require.NoError(t, err)
	for _, r := range claimedAgain {
		require.NotEqual(t, runE.ID, r.ID, "Succeeded run must not be claimed")
	}

	// invalid_output cannot be claimed
	pF := makeTestParams(fix, "qwen-claim-f")
	runF, _, err := store.EnsureVisionRun(ctx, pF)
	require.NoError(t, err)

	claimed, err = store.ClaimPendingRuns(ctx, 100)
	require.NoError(t, err)
	err = store.SaveInvalidOutput(ctx, runF.ID, "BAD_JSON", "unparseable response")
	require.NoError(t, err)

	claimedAgain, err = store.ClaimPendingRuns(ctx, 100)
	require.NoError(t, err)
	for _, r := range claimedAgain {
		require.NotEqual(t, runF.ID, r.ID, "Invalid_output run must not be claimed")
	}
}

func TestStore_TerminalStateImmutability_AndTransitions(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	params := makeTestParams(fix, "qwen-immut")
	run, _, err := store.EnsureVisionRun(ctx, params)
	require.NoError(t, err)

	// Run is in 'pending', not 'processing'. SaveSuccess must fail closed!
	obs := vision.ProductVisualObservationV1{}
	eval := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	require.ErrorIs(t, err, vision.ErrInvalidRunTransition)

	// Claim to processing
	claimed, err := store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	require.NotEmpty(t, claimed)

	// SaveSuccess: processing -> succeeded
	err = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	require.NoError(t, err)

	// Terminal run 'succeeded' cannot be rewritten
	err = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	require.ErrorIs(t, err, vision.ErrInvalidRunTransition)

	err = store.SaveInvalidOutput(ctx, run.ID, "ERR", "msg")
	require.ErrorIs(t, err, vision.ErrInvalidRunTransition)

	err = store.SaveProviderFailure(ctx, run.ID, "TIMEOUT", "msg", nil, nil)
	require.ErrorIs(t, err, vision.ErrInvalidRunTransition)
}

func TestStore_DeadWorkerRecovery_AndRetryStorage(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	// Clean up any stale processing runs from earlier test runs
	_, _ = pool.Exec(ctx, "UPDATE product_vision_runs SET status = 'succeeded' WHERE status = 'processing'")

	// Test G: Non-stale processing not recovered
	pG := makeTestParams(fix, "qwen-recov-g")
	run1, _, err := store.EnsureVisionRun(ctx, pG)
	require.NoError(t, err)

	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)

	recovered, err := store.ResetStaleRuns(ctx, 10*time.Minute, 3)
	require.NoError(t, err)
	require.Equal(t, 0, recovered, "Non-stale run must not be recovered")

	// Test H: Stale processing recovered back to pending
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET claimed_at = $1 WHERE id = $2", time.Now().Add(-1*time.Hour), run1.ID)
	require.NoError(t, err)

	recovered, err = store.ResetStaleRuns(ctx, 10*time.Minute, 3)
	require.NoError(t, err)
	require.Equal(t, 1, recovered, "Stale run must be recovered back to pending")

	var st string
	pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", run1.ID).Scan(&st)
	require.Equal(t, vision.RunStatusPending, st)

	// Stale with max attempts reached → transitions to 'processing_failed' (NOT provider_failed)
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET status = 'processing', claimed_at = $1, attempts = 3 WHERE id = $2", time.Now().Add(-1*time.Hour), run1.ID)
	require.NoError(t, err)

	_, err = store.ResetStaleRuns(ctx, 10*time.Minute, 3)
	require.NoError(t, err)

	pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", run1.ID).Scan(&st)
	require.Equal(t, vision.RunStatusProcessingFailed, st)

	// Test Retry storage: RequeueProviderFailure with nextAttemptAt
	pRetry := makeTestParams(fix, "qwen-retry")
	runRetry, _, err := store.EnsureVisionRun(ctx, pRetry)
	require.NoError(t, err)
	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)

	// Requeue with future nextAttemptAt
	futureTime := time.Now().Add(1 * time.Hour)
	err = store.RequeueProviderFailure(ctx, runRetry.ID, futureTime, 3, "RATE_LIMIT", "429 Too Many Requests")
	require.NoError(t, err)

	// Claim should NOT pick up future retry
	unclaimed, err := store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	for _, r := range unclaimed {
		require.NotEqual(t, runRetry.ID, r.ID, "Future retry time should not be claimed yet")
	}

	// Update nextAttemptAt to past
	pastTime := time.Now().Add(-1 * time.Minute)
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET next_attempt_at = $1 WHERE id = $2", pastTime, runRetry.ID)
	require.NoError(t, err)

	// Now eligible for claim
	claimedAgain, err := store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	foundRetry := false
	for _, r := range claimedAgain {
		if r.ID == runRetry.ID {
			foundRetry = true
			require.Equal(t, 2, r.Attempts)
		}
	}
	require.True(t, foundRetry, "Eligible retry run should be claimed")
}

func TestStore_CurrentProfile_AtomicPrecedenceAndRegressionPrevention(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	// Test I: Older completion regression prevented
	// Run A created at T0
	// Run B created at T1 (later)
	timeA := time.Now().Add(-10 * time.Minute)
	timeB := time.Now().Add(-5 * time.Minute)

	pA := makeTestParams(fix, "qwen-model-v1")
	pB := makeTestParams(fix, "qwen-model-v2")

	runA, _, err := store.EnsureVisionRun(ctx, pA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET created_at = $1 WHERE id = $2", timeA, runA.ID)
	require.NoError(t, err)

	runB, _, err := store.EnsureVisionRun(ctx, pB)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET created_at = $1 WHERE id = $2", timeB, runB.ID)
	require.NoError(t, err)

	// Claim both runs
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET status = 'processing' WHERE id IN ($1, $2)", runA.ID, runB.ID)
	require.NoError(t, err)

	obsB := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.boxy")}
	evalB := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}

	obsA := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.regular")}
	evalA := vision.ModerationEvaluationV1{ModerationRecommendation: "review_recommended"}

	// Step 1: B succeeds FIRST → B becomes current profile
	err = store.SaveSuccess(ctx, runB.ID, obsB, evalB, nil, nil, nil)
	require.NoError(t, err)

	prof, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, prof)
	require.Equal(t, runB.ID, prof.RunID, "Run B must be current profile")
	require.Equal(t, "vision.silhouette.boxy", *prof.Observation.SilhouetteID)

	// Step 2: Older Run A succeeds LATER → Run B must REMAIN current profile!
	err = store.SaveSuccess(ctx, runA.ID, obsA, evalA, nil, nil, nil)
	require.NoError(t, err)

	profAfterA, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profAfterA)
	require.Equal(t, runB.ID, profAfterA.RunID, "Run B must STILL be current profile after older Run A completed!")
	require.Equal(t, "vision.silhouette.boxy", *profAfterA.Observation.SilhouetteID)
}

func TestStore_DBAuthoritative_StaleSnapshotRejection(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	params := makeTestParams(fix, "qwen-stale")
	run, _, err := store.EnsureVisionRun(ctx, params)
	require.NoError(t, err)

	// Worker claims it
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET status = 'processing' WHERE id = $1", run.ID)
	require.NoError(t, err)

	// Seller updates the product in DB (e.g. changes title) before worker saves success
	_, err = pool.Exec(ctx, "UPDATE products SET title = 'Completely Different Title' WHERE id = $1", fix.productID)
	require.NoError(t, err)

	// Worker completes and calls SaveSuccess
	obs := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.fitted")}
	eval := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	require.NoError(t, err)

	// The run was historically recorded as succeeded
	var runStatus string
	pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", run.ID).Scan(&runStatus)
	require.Equal(t, vision.RunStatusSucceeded, runStatus)

	// BUT the current visual profile was NOT advanced because DB state was no longer current!
	prof, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.Nil(t, prof, "Current visual profile must NOT be created/advanced for stale snapshot")
}

func TestStore_MediaMutation_RaceSafety(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)
	prodRepo := products.NewRepository(pool)

	params := makeTestParams(fix, "qwen-race")
	run, _, err := store.EnsureVisionRun(ctx, params)
	require.NoError(t, err)

	// Mark run as processing
	_, err = pool.Exec(ctx, "UPDATE product_vision_runs SET status = 'processing' WHERE id = $1", run.ID)
	require.NoError(t, err)

	// Concurrently execute SaveSuccess and an image crop mutation via canonical repository method
	var wg sync.WaitGroup
	wg.Add(2)

	var saveErr, cropErr error
	obs := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.regular")}
	eval := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}

	go func() {
		defer wg.Done()
		saveErr = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	}()

	go func() {
		defer wg.Done()
		// Canonical repository media mutation
		cropErr = prodRepo.UpdateProductImageCrop(ctx, fix.imageID, 0.1, 0.1, 0.8, 0.8, "https://example.com/cropped.jpg", "rendition_cropped.jpg")
	}()

	wg.Wait()
	require.NoError(t, saveErr)
	require.NoError(t, cropErr)

	// Inspect final committed state:
	// Verify INVARIANT:
	// After media mutation bumps products.vision_content_version from 0 to 1,
	// any profile for version 0 is invalidated, and no run for version 1 has succeeded yet.
	// Therefore GetCurrentVisualProfile MUST return nil.
	prof, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.Nil(t, prof, "Stale visual profile must not be returned after media mutation bumped version")
}

func TestStore_CorruptStoredJSON_FailsClosed(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)

	params := makeTestParams(fix, "qwen-corrupt")
	run, _, err := store.EnsureVisionRun(ctx, params)
	require.NoError(t, err)

	// Directly insert corrupt observation JSON into product_current_visual_profiles
	_, err = pool.Exec(ctx, `
		INSERT INTO product_current_visual_profiles (
			product_id, run_id, run_created_at, vision_content_version, model_id, snapshot_hash, taxonomy_context_hash, observation, updated_at
		) VALUES (
			$1, $2, now(), 0, 'qwen', $3, 'tax1', '{"observedCategoryId": "invalid-uuid-string"}'::jsonb, now()
		)
	`, fix.productID, run.ID, fix.snapHash)
	require.NoError(t, err)

	// GetCurrentVisualProfile must fail closed (return error wrapping ErrCorruptStoredJSON, not synthesize empty profile)
	prof, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.Error(t, err)
	require.ErrorIs(t, err, vision.ErrCorruptStoredJSON)
	require.Nil(t, prof)
}

func TestStore_ScenarioA_B_C_D(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)
	prodRepo := products.NewRepository(pool)

	// SCENARIO A: Valid profile returned when version matches (cp.vision_content_version = p.vision_content_version)
	paramsA := makeTestParams(fix, "qwen-scen-a")
	runA, _, err := store.EnsureVisionRun(ctx, paramsA)
	require.NoError(t, err)

	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)

	obsA := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.regular")}
	evalA := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, runA.ID, obsA, evalA, nil, nil, nil)
	require.NoError(t, err)

	profA, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profA, "Scenario A: Profile must be returned when version matches")
	require.Equal(t, runA.ID, profA.RunID)
	require.Equal(t, int64(0), profA.VisionContentVersion)

	// SCENARIO B: Media mutation bumps version from 0 to 1 -> GetCurrentVisualProfile returns nil. Historical Run A preserved.
	err = prodRepo.UpdateProductImageCrop(ctx, fix.imageID, 0.1, 0.1, 0.9, 0.9, "https://example.com/crop.jpg", "crop.jpg")
	require.NoError(t, err)

	var currentVersion int64
	err = pool.QueryRow(ctx, "SELECT vision_content_version FROM products WHERE id = $1", fix.productID).Scan(&currentVersion)
	require.NoError(t, err)
	require.Equal(t, int64(1), currentVersion, "Media mutation must bump vision_content_version to 1")

	profB, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.Nil(t, profB, "Scenario B: GetCurrentVisualProfile must return nil after media mutation bumps version")

	// Verify historical Run A preserved in product_vision_runs
	var runAStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", runA.ID).Scan(&runAStatus)
	require.NoError(t, err)
	require.Equal(t, vision.RunStatusSucceeded, runAStatus, "Scenario B: Historical Run A must be preserved as succeeded")

	// SCENARIO C: New Run B at version 1 succeeds -> GetCurrentVisualProfile returns profile B with version 1.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	snapB, err := store.ReadCurrentSnapshotTx(ctx, tx, fix.productID)
	require.NoError(t, err)
	_ = tx.Commit(ctx)
	require.Equal(t, int64(1), snapB.VisionContentVersion)

	paramsB := vision.ScheduleVisionRunParams{
		RunID:    uuid.New(),
		Snapshot: snapB,
		TaxonomyContext: vision.TaxonomyContext{
			Categories: []vision.TaxonomyItem{{ID: fix.categoryID, Label: "Dresses"}},
		},
		Provider:          "cloudru",
		ModelID:           "qwen-scen-b",
		PromptVersion:     "v1",
		SchemaVersion:     "v1",
		VocabularyVersion: "v1",
	}
	runB, _, err := store.EnsureVisionRun(ctx, paramsB)
	require.NoError(t, err)

	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)

	obsB := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.fitted")}
	evalB := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, runB.ID, obsB, evalB, nil, nil, nil)
	require.NoError(t, err)

	profC, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profC, "Scenario C: Profile B must be returned")
	require.Equal(t, runB.ID, profC.RunID)
	require.Equal(t, int64(1), profC.VisionContentVersion)
	require.Equal(t, "vision.silhouette.fitted", *profC.Observation.SilhouetteID)

	// SCENARIO D: Older run A (version 0) completing later cannot regress version 1 profile.
	paramsStaleA := makeTestParams(fix, "qwen-stale-a")
	paramsStaleA.RunID = uuid.New()
	runStaleA, _, err := store.EnsureVisionRun(ctx, paramsStaleA)
	require.NoError(t, err)

	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)

	obsStale := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.stale")}
	evalStale := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, runStaleA.ID, obsStale, evalStale, nil, nil, nil)
	require.NoError(t, err)

	// Current profile must STILL be runB at version 1!
	profD, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profD)
	require.Equal(t, runB.ID, profD.RunID, "Scenario D: Older run at version 0 completing later must not regress profile B at version 1")
	require.Equal(t, int64(1), profD.VisionContentVersion)
	require.Equal(t, "vision.silhouette.fitted", *profD.Observation.SilhouetteID)
}

func TestStore_ScenarioE_PendingRevision_And_ScenarioF_RevisionPromotion(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)
	prodRepo := products.NewRepository(pool)
	sellerRepo := sellers.NewRepository(pool)
	dbClient := &postgres.Client{Pool: pool}
	svc := products.NewService(prodRepo, sellerRepo, dbClient, nil, nil)

	// Setup product in published status with version 1
	_, err := pool.Exec(ctx, "UPDATE products SET status = 'published', vision_content_version = 1 WHERE id = $1", fix.productID)
	require.NoError(t, err)

	// Establish current visual profile at version 1
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	snap1, err := store.ReadCurrentSnapshotTx(ctx, tx, fix.productID)
	require.NoError(t, err)
	_ = tx.Commit(ctx)
	require.Equal(t, int64(1), snap1.VisionContentVersion)

	params1 := vision.ScheduleVisionRunParams{
		RunID:    uuid.New(),
		Snapshot: snap1,
		TaxonomyContext: vision.TaxonomyContext{
			Categories: []vision.TaxonomyItem{{ID: fix.categoryID, Label: "Dresses"}},
		},
		Provider:          "cloudru",
		ModelID:           "qwen-rev-1",
		PromptVersion:     "v1",
		SchemaVersion:     "v1",
		VocabularyVersion: "v1",
	}
	run1, _, err := store.EnsureVisionRun(ctx, params1)
	require.NoError(t, err)
	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	obs1 := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.straight")}
	eval1 := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, run1.ID, obs1, eval1, nil, nil, nil)
	require.NoError(t, err)

	// Profile is active at version 1
	prof1, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, prof1)
	require.Equal(t, run1.ID, prof1.RunID)
	require.Equal(t, int64(1), prof1.VisionContentVersion)

	// SCENARIO E: Seller updates published product with ContinueSelling=true, creating pending revision
	contSelling := true
	newTitle := "Proposed Revised Silk Dress"
	updateReq := products.UpdateProductRequest{
		Title:           &newTitle,
		ContinueSelling: &contSelling,
	}
	updatedProd, err := svc.UpdateProductForSeller(ctx, fix.userID, fix.productID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updatedProd.LiveRevisionID)

	// Live products.vision_content_version MUST remain 1
	var liveVer int64
	err = pool.QueryRow(ctx, "SELECT vision_content_version FROM products WHERE id = $1", fix.productID).Scan(&liveVer)
	require.NoError(t, err)
	require.Equal(t, int64(1), liveVer, "Scenario E: Pending revision must NOT bump live vision_content_version")

	// Live visual profile MUST still be returned
	profLive, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profLive, "Scenario E: Live profile must remain current while revision is pending")
	require.Equal(t, run1.ID, profLive.RunID)
	require.Equal(t, int64(1), profLive.VisionContentVersion)

	// SCENARIO F: Admin approves revision through CANONICAL SERVICE METHOD -> bumps version to 2
	err = svc.ApproveProduct(ctx, fix.adminID, fix.productID, nil)
	require.NoError(t, err)

	var promotedVer int64
	var liveTitle string
	err = pool.QueryRow(ctx, "SELECT vision_content_version, title FROM products WHERE id = $1", fix.productID).Scan(&promotedVer, &liveTitle)
	require.NoError(t, err)
	require.Equal(t, int64(2), promotedVer, "Scenario F: Revision promotion must bump vision_content_version to N+1")
	require.Equal(t, "Proposed Revised Silk Dress", liveTitle)

	// GetCurrentVisualProfile MUST now return nil because old profile was for version 1
	profAfterPromo, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.Nil(t, profAfterPromo, "Scenario F: Old profile at version 1 must be invalidated after revision promotion")
}

func TestStore_ScenarioG_UnrelatedMutation(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)
	prodRepo := products.NewRepository(pool)

	// Establish current profile at version 0
	params := makeTestParams(fix, "qwen-unrelated")
	run, _, err := store.EnsureVisionRun(ctx, params)
	require.NoError(t, err)
	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	obs := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.a_line")}
	eval := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	require.NoError(t, err)

	prof, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, prof)
	require.Equal(t, int64(0), prof.VisionContentVersion)

	// 1. Unrelated mutation: Price update
	err = prodRepo.UpdateProductPriceCents(ctx, fix.productID, 99990)
	require.NoError(t, err)

	var verAfterPrice int64
	err = pool.QueryRow(ctx, "SELECT vision_content_version FROM products WHERE id = $1", fix.productID).Scan(&verAfterPrice)
	require.NoError(t, err)
	require.Equal(t, int64(0), verAfterPrice, "Scenario G: Price update must NOT bump vision_content_version")

	profAfterPrice, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profAfterPrice, "Scenario G: Visual profile must remain valid after price update")

	// 2. Unrelated mutation: Moderation status update
	p, err := prodRepo.GetProductByID(ctx, fix.productID)
	require.NoError(t, err)
	p.Status = products.StatusInReview
	err = prodRepo.UpdateProductStatus(ctx, p)
	require.NoError(t, err)

	var verAfterStatus int64
	err = pool.QueryRow(ctx, "SELECT vision_content_version FROM products WHERE id = $1", fix.productID).Scan(&verAfterStatus)
	require.NoError(t, err)
	require.Equal(t, int64(0), verAfterStatus, "Scenario G: Moderation status update must NOT bump vision_content_version")

	profAfterStatus, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profAfterStatus, "Scenario G: Visual profile must remain valid after status update")
}

func TestVision_RealServicePath_RevisionPromotion(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)
	prodRepo := products.NewRepository(pool)
	sellerRepo := sellers.NewRepository(pool)
	dbClient := &postgres.Client{Pool: pool}
	svc := products.NewService(prodRepo, sellerRepo, dbClient, nil, nil)

	// A. Create/persist published product live state A at version 0
	_, err := pool.Exec(ctx, "UPDATE products SET status = 'published', vision_content_version = 0 WHERE id = $1", fix.productID)
	require.NoError(t, err)

	// B. Have current visual profile for version 0
	runA, _, err := store.EnsureVisionRun(ctx, makeTestParams(fix, "qwen-run-a"))
	require.NoError(t, err)
	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	obsA := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.a_line")}
	evalA := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, runA.ID, obsA, evalA, nil, nil, nil)
	require.NoError(t, err)

	profA, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profA, "Step B: Profile A must be current for version 0")
	require.Equal(t, int64(0), profA.VisionContentVersion)

	// C. Seller creates pending revision B through canonical service path (ContinueSelling=true)
	newTitle := "Evening Silk Dress V2 - Autumn Collection"
	contSelling := true
	updateReq := products.UpdateProductRequest{
		Title:           &newTitle,
		ContinueSelling: &contSelling,
	}
	updatedProd, err := svc.UpdateProductForSeller(ctx, fix.userID, fix.productID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updatedProd.LiveRevisionID)

	// D. Assert:
	//    live product still A
	//    live version still 0
	//    GetCurrentVisualProfile still returns profile A.
	var liveTitle string
	var liveVer int64
	err = pool.QueryRow(ctx, "SELECT title, vision_content_version FROM products WHERE id = $1", fix.productID).Scan(&liveTitle, &liveVer)
	require.NoError(t, err)
	require.Equal(t, "Evening Silk Dress", liveTitle, "Step D: Live title must remain unchanged before approval")
	require.Equal(t, int64(0), liveVer, "Step D: Live version must remain N before approval")

	profStillA, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profStillA, "Step D: Profile A must remain current while revision is pending")
	require.Equal(t, runA.ID, profStillA.RunID)
	require.Equal(t, int64(0), profStillA.VisionContentVersion)

	// E. Admin approves through CANONICAL SERVICE METHOD used by HTTP handler.
	//    Zero direct calls to PromoteProductRevision!
	err = svc.ApproveProduct(ctx, fix.adminID, fix.productID, nil)
	require.NoError(t, err)

	// F. Assert:
	//    live product is B
	//    revision is approved/applied
	//    version == N+1 exactly (version == 1)
	//    GetCurrentVisualProfile == nil.
	var liveTitleAfterApprove string
	var liveVerAfterApprove int64
	err = pool.QueryRow(ctx, "SELECT title, vision_content_version FROM products WHERE id = $1", fix.productID).Scan(&liveTitleAfterApprove, &liveVerAfterApprove)
	require.NoError(t, err)
	require.Equal(t, newTitle, liveTitleAfterApprove, "Step F: Live title must be updated to revision B")
	require.Equal(t, int64(1), liveVerAfterApprove, "Step F: Live version must be exactly N+1 (1)")

	var revStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM product_revisions WHERE id = $1", *updatedProd.LiveRevisionID).Scan(&revStatus)
	require.NoError(t, err)
	require.Equal(t, "approved", revStatus, "Step F: Revision status must be approved")

	profAfterPromotion, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.Nil(t, profAfterPromotion, "Step F: Old profile at version 0 must be invalidated (return nil)")

	// G. New Vision run B succeeds for version N+1 (version 1)
	txB, err := pool.Begin(ctx)
	require.NoError(t, err)
	snapB, err := store.ReadCurrentSnapshotTx(ctx, txB, fix.productID)
	require.NoError(t, err)
	_ = txB.Commit(ctx)
	require.Equal(t, int64(1), snapB.VisionContentVersion)
	require.Equal(t, newTitle, snapB.Title)

	paramsB := vision.ScheduleVisionRunParams{
		RunID:    uuid.New(),
		Snapshot: snapB,
		TaxonomyContext: vision.TaxonomyContext{
			Categories: []vision.TaxonomyItem{{ID: fix.categoryID, Label: "Dresses"}},
		},
		ModelID:       "qwen-run-b",
		PromptVersion: "v1.0",
		SchemaVersion: "v1.0",
	}
	runB, _, err := store.EnsureVisionRun(ctx, paramsB)
	require.NoError(t, err)
	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	obsB := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.fit_and_flare")}
	evalB := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, runB.ID, obsB, evalB, nil, nil, nil)
	require.NoError(t, err)

	// H. GetCurrentVisualProfile returns B
	profB, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profB, "Step H: Profile B must be current for version 1")
	require.Equal(t, runB.ID, profB.RunID)
	require.Equal(t, int64(1), profB.VisionContentVersion)
}

func TestVision_RealServicePath_NegativeModeration(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)
	prodRepo := products.NewRepository(pool)
	sellerRepo := sellers.NewRepository(pool)
	dbClient := &postgres.Client{Pool: pool}
	svc := products.NewService(prodRepo, sellerRepo, dbClient, nil, nil)

	// 1. Establish published product with profile at version 0
	_, err := pool.Exec(ctx, "UPDATE products SET status = 'published', vision_content_version = 0 WHERE id = $1", fix.productID)
	require.NoError(t, err)

	run, _, err := store.EnsureVisionRun(ctx, makeTestParams(fix, "qwen-neg-test"))
	require.NoError(t, err)
	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	obs := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.a_line")}
	eval := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	require.NoError(t, err)

	profBefore, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profBefore)
	require.Equal(t, int64(0), profBefore.VisionContentVersion)

	// 2. Seller creates pending revision (ContinueSelling=false -> resets status to pending_moderation)
	contSellingFalse := false
	badTitle := "Proposed Inappropriate Title"
	updateReq := products.UpdateProductRequest{
		Title:           &badTitle,
		ContinueSelling: &contSellingFalse,
	}
	updatedProd, err := svc.UpdateProductForSeller(ctx, fix.userID, fix.productID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updatedProd.LiveRevisionID)

	// 3. Admin starts review: verify start-review does NOT bump version
	err = svc.StartProductReview(ctx, fix.adminID, fix.productID)
	require.NoError(t, err)

	var verAfterStartReview int64
	var titleAfterStartReview string
	err = pool.QueryRow(ctx, "SELECT vision_content_version, title FROM products WHERE id = $1", fix.productID).Scan(&verAfterStartReview, &titleAfterStartReview)
	require.NoError(t, err)
	require.Equal(t, int64(0), verAfterStartReview, "Start review must NOT bump version")
	require.Equal(t, "Evening Silk Dress", titleAfterStartReview, "Start review must NOT change title")

	// 4. Admin rejects through canonical moderation service method:
	rejectionComment := "Violates content policy"
	err = svc.RejectProduct(ctx, fix.adminID, fix.productID, rejectionComment)
	require.NoError(t, err)

	// 5. Assert:
	//    live content unchanged
	//    vision_content_version unchanged (0)
	//    old live current profile remains valid
	var verAfterReject int64
	var titleAfterReject string
	var statusAfterReject string
	err = pool.QueryRow(ctx, "SELECT vision_content_version, title, status FROM products WHERE id = $1", fix.productID).Scan(&verAfterReject, &titleAfterReject, &statusAfterReject)
	require.NoError(t, err)
	require.Equal(t, int64(0), verAfterReject, "Reject must NOT bump vision_content_version")
	require.Equal(t, "Evening Silk Dress", titleAfterReject, "Reject must NOT change live content")
	require.Equal(t, products.StatusRejected, statusAfterReject)

	var revStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM product_revisions WHERE id = $1", *updatedProd.LiveRevisionID).Scan(&revStatus)
	require.NoError(t, err)
	require.Equal(t, "rejected", revStatus, "Pending revision must be marked rejected")

	profAfterReject, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, profAfterReject, "Old live current profile must remain valid after rejection")
	require.Equal(t, run.ID, profAfterReject.RunID)
}

func TestVision_RealServicePath_InitialProductApproval(t *testing.T) {
	pool := getIsolatedTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := vision.NewStore(pool)
	fix := setupTestFixture(t, pool)
	prodRepo := products.NewRepository(pool)
	sellerRepo := sellers.NewRepository(pool)
	dbClient := &postgres.Client{Pool: pool}
	svc := products.NewService(prodRepo, sellerRepo, dbClient, nil, nil)

	// 1. Initial product created in draft/pending state without revision
	p, err := prodRepo.GetProductByID(ctx, fix.productID)
	require.NoError(t, err)
	require.Nil(t, p.LiveRevisionID)

	p.Status = products.StatusPendingModeration
	err = prodRepo.UpdateProductStatus(ctx, p)
	require.NoError(t, err)

	var verBefore int64
	err = pool.QueryRow(ctx, "SELECT vision_content_version FROM products WHERE id = $1", fix.productID).Scan(&verBefore)
	require.NoError(t, err)
	require.Equal(t, int64(0), verBefore)

	// 2. Admin approves initial product through canonical service method
	err = svc.ApproveProduct(ctx, fix.adminID, fix.productID, nil)
	require.NoError(t, err)

	// 3. Assert:
	//    status is published
	//    vision_content_version is still 0 (no double bump, not incremented)
	var verAfter int64
	var statusAfter string
	err = pool.QueryRow(ctx, "SELECT vision_content_version, status FROM products WHERE id = $1", fix.productID).Scan(&verAfter, &statusAfter)
	require.NoError(t, err)
	require.Equal(t, products.StatusPublished, statusAfter)
	require.Equal(t, int64(0), verAfter, "Initial product approval must NOT bump vision_content_version (remains 0)")

	// 4. Save visual profile for version 0 and verify it can be retrieved
	run, _, err := store.EnsureVisionRun(ctx, makeTestParams(fix, "qwen-initial-approve"))
	require.NoError(t, err)
	_, err = store.ClaimPendingRuns(ctx, 10)
	require.NoError(t, err)
	obs := vision.ProductVisualObservationV1{SilhouetteID: func(s string) *string { return &s }("vision.silhouette.a_line")}
	eval := vision.ModerationEvaluationV1{ModerationRecommendation: "clear"}
	err = store.SaveSuccess(ctx, run.ID, obs, eval, nil, nil, nil)
	require.NoError(t, err)

	prof, err := store.GetCurrentVisualProfile(ctx, fix.productID)
	require.NoError(t, err)
	require.NotNil(t, prof, "Visual profile for initial approved product must be readable as current")
	require.Equal(t, int64(0), prof.VisionContentVersion)
}
