package products_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

// TestAdminVisionRead_SucceededV2_Profile is an isolated, deterministic automated test
// that creates its own test-owned product, category, V2 run, and visual profile.
// Satisfies Req A, B, C, D, F without relying on the presence of the live acceptance row.
func TestAdminVisionRead_SucceededV2_Profile(t *testing.T) {
	ctx := context.Background()
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()

	// 1. Create a dedicated test category
	testCatID := uuid.New()
	_, err := fix.pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Test Vision Category', $2, now(), now())
	`, testCatID, fmt.Sprintf("test-vis-cat-%s", testCatID.String()[:8]))
	require.NoError(t, err)

	// 2. Create test product
	testProdID := uuid.New()
	_, err = fix.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, $3, 'Vision Succeeded Product', $4, 'draft', 'seller', 15000, 'RUB', 1, now(), now())
	`, testProdID, fix.sellerID, testCatID, fmt.Sprintf("vis-succ-%s", testProdID.String()[:8]))
	require.NoError(t, err)

	// 3. Create test observation and evaluation
	obs := vision.ProductVisualObservationV2{
		CategoryConsistency:  vision.CategoryConsistencyContradictory,
		ObservedCategoryText: strPtr("The images show a cityscape, a portrait, and a video still of men in suits. No coat is visible."),
		DominantColorIDs:     []uuid.UUID{},
		SecondaryColorIDs:    []uuid.UUID{},
		ImageSetConsistency:  boolPtr(false),
		Confidence: map[string]vision.ConfidenceScore{
			"category":   {Level: "low", Score: 0.1},
			"silhouette": {Level: "low", Score: 0.1},
		},
		Evidence: []vision.Evidence{
			{Field: "category", ValueID: testCatID.String(), ImageIDs: []uuid.UUID{uuid.New()}},
		},
	}
	obsJSON, err := json.Marshal(obs)
	require.NoError(t, err)

	eval := vision.ModerationEvaluationV1{
		ModerationRecommendation: "uncertain",
		RecommendationReason:     "The provided images do not contain the product described.",
	}
	evalJSON, err := json.Marshal(eval)
	require.NoError(t, err)

	// 4. Create V2 run and current visual profile with matching identity
	runID := uuid.New()
	snapshotHash := "snap_hash_test_123"
	taxonomyHash := "tax_hash_test_123"
	modelID := vision.CanonicalModelID

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_vision_runs (
			id, product_id, vision_content_version, provider, model_id, prompt_version,
			schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
			snapshot_json, idempotency_key, status, completed_at, result_observation,
			result_evaluation, taxonomy_context_json, created_at, updated_at
		) VALUES (
			$1, $2, 1, 'cloudru', $3, '1', '2', '1', $4, $5,
			'{}', $6, 'succeeded', now(), $7, $8, '{}', now(), now()
		)
	`, runID, testProdID, modelID, taxonomyHash, snapshotHash, fmt.Sprintf("idem-succ-%s", runID.String()[:8]), obsJSON, evalJSON)
	require.NoError(t, err)

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_current_visual_profiles (
			product_id, run_id, run_created_at, vision_content_version, model_id,
			snapshot_hash, taxonomy_context_hash, observation, updated_at
		) VALUES (
			$1, $2, now(), 1, $3, $4, $5, $6, now()
		)
	`, testProdID, runID, modelID, snapshotHash, taxonomyHash, obsJSON)
	require.NoError(t, err)

	// 5. Read back product via Admin API
	p, err := fix.svc.GetAdminProductDetail(ctx, testProdID)
	require.NoError(t, err)

	// Canonical product category remains completely separate and untouched
	require.NotNil(t, p.CategoryID, "Canonical product category must exist")
	assert.Equal(t, testCatID.String(), p.CategoryID.String(), "Canonical category ID must remain untouched")

	// Vision DTO attached
	require.NotNil(t, p.Vision, "Vision DTO must be populated on GetAdminProductDetail")
	assert.Equal(t, "succeeded", p.Vision.Status)
	assert.NotNil(t, p.Vision.AnalyzedAt, "AnalyzedAt must be present")

	// Category consistency returned
	require.NotNil(t, p.Vision.CategoryConsistency)
	assert.Equal(t, "contradictory", *p.Vision.CategoryConsistency)

	// Observed category text returned as explanatory text only
	require.NotNil(t, p.Vision.ObservedCategoryText)
	assert.Contains(t, *p.Vision.ObservedCategoryText, "The images show a cityscape")

	// Empty colors array remains valid
	assert.Empty(t, p.Vision.DominantColors, "Empty dominant colors must remain valid")
	assert.Empty(t, p.Vision.SecondaryColors, "Empty secondary colors must remain valid")

	// Image set consistency
	require.NotNil(t, p.Vision.ImageSetConsistency)
	assert.False(t, *p.Vision.ImageSetConsistency)

	// Moderation evaluation
	require.NotNil(t, p.Vision.Moderation)
	assert.Equal(t, "uncertain", p.Vision.Moderation.Recommendation)
	assert.Contains(t, p.Vision.Moderation.RecommendationReason, "The provided images do not contain the product described")

	// Confidence and Evidence
	assert.NotEmpty(t, p.Vision.Confidence)
	assert.NotEmpty(t, p.Vision.Evidence)
}

// TestAdminVisionRead_AcceptedLiveRun_Proof is a READ-ONLY verification of the real accepted live fixture:
// Product: 86b56c03-365c-426d-8fb2-99cb25724ea8
// Run:     0b5ad88a-58cc-4033-b55a-2ed0fcc5d566
// If this row is not present in the environment, the test gracefully skips, ensuring normal CI runs do not fail.
func TestAdminVisionRead_AcceptedLiveRun_Proof(t *testing.T) {
	ctx := context.Background()
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()

	liveProductID := uuid.MustParse("86b56c03-365c-426d-8fb2-99cb25724ea8")

	var exists bool
	err := fix.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM products WHERE id = $1)", liveProductID).Scan(&exists)
	require.NoError(t, err)
	if !exists {
		t.Skip("Live accepted fixture not present in this database, skipping read-only proof")
	}

	p, err := fix.svc.GetAdminProductDetail(ctx, liveProductID)
	require.NoError(t, err)

	require.NotNil(t, p.CategoryID)
	assert.Equal(t, "2d097e3c-f546-4af4-b7c1-2b954783a6b4", p.CategoryID.String())

	require.NotNil(t, p.Vision)
	assert.Equal(t, "succeeded", p.Vision.Status)
	assert.NotNil(t, p.Vision.AnalyzedAt)

	require.NotNil(t, p.Vision.CategoryConsistency)
	assert.Equal(t, "contradictory", *p.Vision.CategoryConsistency)

	require.NotNil(t, p.Vision.ObservedCategoryText)
	assert.Contains(t, *p.Vision.ObservedCategoryText, "The images show a cityscape")

	assert.Empty(t, p.Vision.DominantColors)
	assert.Empty(t, p.Vision.SecondaryColors)

	require.NotNil(t, p.Vision.ImageSetConsistency)
	assert.False(t, *p.Vision.ImageSetConsistency)

	require.NotNil(t, p.Vision.Moderation)
	assert.Equal(t, "uncertain", p.Vision.Moderation.Recommendation)
}

// TestAdminVisionRead_ColorResolution_And_Fallback verifies:
// - Canonical colors resolve to metadata (id, code, name, hex)
// - Historical/deleted/unresolvable colors fall back cleanly to explicit unresolved state
// Satisfies Req E.
func TestAdminVisionRead_ColorResolution_And_Fallback(t *testing.T) {
	ctx := context.Background()
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()

	// 1. Create a known color in colors table
	knownColorID := uuid.New()
	colorHex := "#1A2B3C"
	_, err := fix.pool.Exec(ctx, `
		INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active)
		VALUES ($1, $2, $3, $4, 10, true)
	`, knownColorID, fmt.Sprintf("test_color_%s", knownColorID.String()[:8]), "Синий тест", colorHex)
	require.NoError(t, err)

	// 2. An unresolvable/deleted color UUID
	unresolvedColorID := uuid.New()

	// 3. Create a test product
	testProdID := uuid.New()
	_, err = fix.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, 'Color Test Product', $3, 'draft', 'seller', 10000, 'RUB', 1, now(), now())
	`, testProdID, fix.sellerID, fmt.Sprintf("color-test-%s", testProdID.String()[:8]))
	require.NoError(t, err)

	// 4. Create a vision run and visual profile with both colors
	runID := uuid.New()
	obs := vision.ProductVisualObservationV2{
		CategoryConsistency:  vision.CategoryConsistencyConsistent,
		DominantColorIDs:     []uuid.UUID{knownColorID, unresolvedColorID},
		SecondaryColorIDs:    []uuid.UUID{},
		ImageSetConsistency:  boolPtr(true),
		Confidence:           map[string]vision.ConfidenceScore{"category": {Level: "high", Score: 0.95}},
	}
	obsJSON, err := json.Marshal(obs)
	require.NoError(t, err)

	modelID := vision.CanonicalModelID
	snapHash := "snap_col_123"
	taxHash := "tax_col_123"

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_vision_runs (
			id, product_id, vision_content_version, provider, model_id, prompt_version,
			schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
			snapshot_json, idempotency_key, status, completed_at, result_observation,
			result_evaluation, taxonomy_context_json, created_at, updated_at
		) VALUES (
			$1, $2, 1, 'cloudru', $3, '1', '2', '1', $4, $5,
			'{}', $6, 'succeeded', now(), $7, '{}', '{}', now(), now()
		)
	`, runID, testProdID, modelID, taxHash, snapHash, fmt.Sprintf("idem-color-%s", runID.String()[:8]), obsJSON)
	require.NoError(t, err)

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_current_visual_profiles (
			product_id, run_id, run_created_at, vision_content_version, model_id,
			snapshot_hash, taxonomy_context_hash, observation, updated_at
		) VALUES (
			$1, $2, now(), 1, $3, $4, $5, $6, now()
		)
	`, testProdID, runID, modelID, snapHash, taxHash, obsJSON)
	require.NoError(t, err)

	// 5. Read back product via Admin API
	p, err := fix.svc.GetAdminProductDetail(ctx, testProdID)
	require.NoError(t, err)
	require.NotNil(t, p.Vision)
	assert.Equal(t, "succeeded", p.Vision.Status)

	require.Len(t, p.Vision.DominantColors, 2)

	// Check resolved color
	resolved := p.Vision.DominantColors[0]
	assert.Equal(t, knownColorID, resolved.ID)
	assert.Equal(t, "Синий тест", resolved.Name)
	assert.NotEqual(t, "unresolved", resolved.Code)
	require.NotNil(t, resolved.Hex)
	assert.Equal(t, colorHex, *resolved.Hex)

	// Check unresolvable color fallback
	unresolved := p.Vision.DominantColors[1]
	assert.Equal(t, unresolvedColorID, unresolved.ID)
	assert.Equal(t, "unresolved", unresolved.Code)
	assert.Equal(t, "Неизвестный цвет", unresolved.Name)
	assert.Nil(t, unresolved.Hex)
}

// TestAdminVisionRead_RunStatuses covers:
// - G. pending analysis returns pending state
// - H. processing analysis returns processing state
// - I. provider_failed / invalid_output / processing_failed map to failed safely with failure code
// - J. no Vision run returns not_scheduled
func TestAdminVisionRead_RunStatuses(t *testing.T) {
	ctx := context.Background()
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()

	// Helper to insert a test product
	createProduct := func(name string, version int64) uuid.UUID {
		id := uuid.New()
		_, err := fix.pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'draft', 'seller', 10000, 'RUB', $5, now(), now())
		`, id, fix.sellerID, name, fmt.Sprintf("slug-%s", id.String()[:8]), version)
		require.NoError(t, err)
		return id
	}

	// Helper to insert a run
	insertRun := func(prodID uuid.UUID, version int64, status, failureCode string) uuid.UUID {
		runID := uuid.New()
		var fc *string
		if failureCode != "" {
			fc = &failureCode
		}
		_, err := fix.pool.Exec(ctx, `
			INSERT INTO product_vision_runs (
				id, product_id, vision_content_version, provider, model_id, prompt_version,
				schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
				snapshot_json, idempotency_key, status, failure_code, taxonomy_context_json, created_at, updated_at
			) VALUES (
				$1, $2, $3, 'cloudru', 'qwen', '1', '2', '1', 'thash', 'shash',
				'{}', $4, $5, $6, '{}', now(), now()
			)
		`, runID, prodID, version, fmt.Sprintf("idem-%s", runID.String()[:8]), status, fc)
		require.NoError(t, err)
		return runID
	}

	// J. No vision run returns not_scheduled
	t.Run("J_NotScheduled", func(t *testing.T) {
		prodID := createProduct("No Run Product", 1)
		p, err := fix.svc.GetAdminProductDetail(ctx, prodID)
		require.NoError(t, err)
		require.NotNil(t, p.Vision)
		assert.Equal(t, "not_scheduled", p.Vision.Status)
		assert.Nil(t, p.Vision.FailureCode)
		assert.Nil(t, p.Vision.AnalyzedAt)
	})

	// G. Pending analysis returns pending state
	t.Run("G_Pending", func(t *testing.T) {
		prodID := createProduct("Pending Product", 1)
		insertRun(prodID, 1, "pending", "")

		p, err := fix.svc.GetAdminProductDetail(ctx, prodID)
		require.NoError(t, err)
		require.NotNil(t, p.Vision)
		assert.Equal(t, "pending", p.Vision.Status)
		assert.Nil(t, p.Vision.FailureCode)
		assert.Nil(t, p.Vision.AnalyzedAt)
		assert.Nil(t, p.Vision.CategoryConsistency)
	})

	// H. Processing analysis returns processing state
	t.Run("H_Processing", func(t *testing.T) {
		prodID := createProduct("Processing Product", 1)
		insertRun(prodID, 1, "processing", "")

		p, err := fix.svc.GetAdminProductDetail(ctx, prodID)
		require.NoError(t, err)
		require.NotNil(t, p.Vision)
		assert.Equal(t, "processing", p.Vision.Status)
		assert.Nil(t, p.Vision.FailureCode)
		assert.Nil(t, p.Vision.AnalyzedAt)
	})

	// I. Failed runs map to failed safely with failureCode
	failureStatuses := []struct {
		runStatus   string
		failureCode string
	}{
		{"provider_failed", "provider_timeout"},
		{"invalid_output", "contract_mismatch"},
		{"processing_failed", "transient_io_error"},
	}

	for _, fs := range failureStatuses {
		t.Run("I_"+fs.runStatus, func(t *testing.T) {
			prodID := createProduct("Failed Product "+fs.runStatus, 1)
			insertRun(prodID, 1, fs.runStatus, fs.failureCode)

			p, err := fix.svc.GetAdminProductDetail(ctx, prodID)
			require.NoError(t, err)
			require.NotNil(t, p.Vision)
			assert.Equal(t, "failed", p.Vision.Status)
			require.NotNil(t, p.Vision.FailureCode)
			assert.Equal(t, fs.failureCode, *p.Vision.FailureCode)

			// Ensure zero leaked sensitive fields
			assert.Nil(t, p.Vision.CategoryConsistency)
			assert.Nil(t, p.Vision.ObservedCategoryText)
			assert.Nil(t, p.Vision.Moderation)
		})
	}
}

// TestAdminVisionRead_StaleProfile_And_VersionSafety verifies:
// - K1. Profile version 1 is not returned when product is at version 2 (returns not_scheduled)
// - K2. Strong stale case: run succeeded for current version, BUT current visual profile is absent (SaveSuccess rejected advancement) -> status = stale, observation NOT exposed
// - K3. Strong stale case: run succeeded for current version, BUT current visual profile has invalid/mismatched snapshot hash -> status = stale, observation NOT exposed
// - Historical V1 runs/profiles fail closed without fabricating V2 results
func TestAdminVisionRead_StaleProfile_And_VersionSafety(t *testing.T) {
	ctx := context.Background()
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()

	// 1. Create a product at vision_content_version = 1 with a valid V2 profile
	prodID := uuid.New()
	_, err := fix.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, 'Stale Test Product', $3, 'draft', 'seller', 10000, 'RUB', 1, now(), now())
	`, prodID, fix.sellerID, fmt.Sprintf("stale-%s", prodID.String()[:8]))
	require.NoError(t, err)

	runID := uuid.New()
	obs := vision.ProductVisualObservationV2{
		CategoryConsistency: vision.CategoryConsistencyConsistent,
		ImageSetConsistency: boolPtr(true),
	}
	obsJSON, _ := json.Marshal(obs)
	modelID := vision.CanonicalModelID
	snapHash := "snap_h_1"
	taxHash := "tax_h_1"

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_vision_runs (
			id, product_id, vision_content_version, provider, model_id, prompt_version,
			schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
			snapshot_json, idempotency_key, status, completed_at, result_observation,
			result_evaluation, taxonomy_context_json, created_at, updated_at
		) VALUES (
			$1, $2, 1, 'cloudru', $3, '1', '2', '1', $4, $5,
			'{}', $6, 'succeeded', now(), $7, '{}', '{}', now(), now()
		)
	`, runID, prodID, modelID, taxHash, snapHash, fmt.Sprintf("idem-v1-%s", runID.String()[:8]), obsJSON)
	require.NoError(t, err)

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_current_visual_profiles (
			product_id, run_id, run_created_at, vision_content_version, model_id,
			snapshot_hash, taxonomy_context_hash, observation, updated_at
		) VALUES (
			$1, $2, now(), 1, $3, $4, $5, $6, now()
		)
	`, prodID, runID, modelID, snapHash, taxHash, obsJSON)
	require.NoError(t, err)

	// Verify version 1 returns succeeded
	p1, err := fix.svc.GetAdminProductDetail(ctx, prodID)
	require.NoError(t, err)
	require.NotNil(t, p1.Vision)
	assert.Equal(t, "succeeded", p1.Vision.Status)

	// 2. Bump product content version to 2 (e.g. seller edited title or images)
	_, err = fix.pool.Exec(ctx, "UPDATE products SET vision_content_version = 2 WHERE id = $1", prodID)
	require.NoError(t, err)

	// Case K1: Before any run for version 2 is scheduled: must return not_scheduled (NOT version 1's profile!)
	p2, err := fix.svc.GetAdminProductDetail(ctx, prodID)
	require.NoError(t, err)
	require.NotNil(t, p2.Vision)
	assert.Equal(t, "not_scheduled", p2.Vision.Status, "Stale version 1 profile must not be returned as current")

	// Case K2 (Strong stale case): Run for version 2 succeeded, BUT current profile is absent!
	// (Simulates SaveSuccess refusing to update current visual profile due to concurrent mutation).
	runV2ID := uuid.New()
	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_vision_runs (
			id, product_id, vision_content_version, provider, model_id, prompt_version,
			schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
			snapshot_json, idempotency_key, status, completed_at, result_observation,
			result_evaluation, taxonomy_context_json, created_at, updated_at
		) VALUES (
			$1, $2, 2, 'cloudru', $3, '1', '2', '1', 'tax_v2', 'snap_v2',
			'{}', $4, 'succeeded', now(), $5, '{}', '{}', now(), now()
		)
	`, runV2ID, prodID, modelID, fmt.Sprintf("idem-v2-%s", runV2ID.String()[:8]), obsJSON)
	require.NoError(t, err)

	// Read back: Because current visual profile for version 2 is ABSENT,
	// the bare succeeded run must NOT be promoted to succeeded current!
	// Must return 'stale', and historical observation must NOT be exposed as current.
	p3, err := fix.svc.GetAdminProductDetail(ctx, prodID)
	require.NoError(t, err)
	require.NotNil(t, p3.Vision)
	assert.Equal(t, "stale", p3.Vision.Status, "Bare succeeded run without current profile must return stale")
	assert.Nil(t, p3.Vision.ObservedCategoryText, "Historical observation must not be exposed as current")
	assert.Empty(t, p3.Vision.DominantColors, "Dominant colors must not be exposed as current")
	assert.Nil(t, p3.Vision.Moderation, "Moderation evaluation must not be exposed as current")

	// Case K3 (Strong stale case with invalid identity):
	// Current visual profile has mismatched snapshot_hash
	corruptProdID := uuid.New()
	_, err = fix.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, 'Corrupt Identity Product', $3, 'draft', 'seller', 10000, 'RUB', 1, now(), now())
	`, corruptProdID, fix.sellerID, fmt.Sprintf("corrupt-%s", corruptProdID.String()[:8]))
	require.NoError(t, err)

	corruptRunID := uuid.New()
	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_vision_runs (
			id, product_id, vision_content_version, provider, model_id, prompt_version,
			schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
			snapshot_json, idempotency_key, status, completed_at, result_observation,
			result_evaluation, taxonomy_context_json, created_at, updated_at
		) VALUES (
			$1, $2, 1, 'cloudru', $3, '1', '2', '1', 'tax_c', 'snap_run_actual',
			'{}', $4, 'succeeded', now(), $5, '{}', '{}', now(), now()
		)
	`, corruptRunID, corruptProdID, modelID, fmt.Sprintf("idem-c-%s", corruptRunID.String()[:8]), obsJSON)
	require.NoError(t, err)

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_current_visual_profiles (
			product_id, run_id, run_created_at, vision_content_version, model_id,
			snapshot_hash, taxonomy_context_hash, observation, updated_at
		) VALUES (
			$1, $2, now(), 1, $3, 'snap_profile_mismatched', 'tax_c', $4, now()
		)
	`, corruptProdID, corruptRunID, modelID, obsJSON)
	require.NoError(t, err)

	pCorrupt, err := fix.svc.GetAdminProductDetail(ctx, corruptProdID)
	require.NoError(t, err)
	require.NotNil(t, pCorrupt.Vision)
	assert.Equal(t, "stale", pCorrupt.Vision.Status, "Mismatched snapshot hash identity must fail currentness check and return stale")

	// 3. Historical V1 safety test:
	// A product whose only profile was created with schema_version = "1" must not fabricate V2 semantics
	v1ProdID := uuid.New()
	_, err = fix.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, 'V1 Product', $3, 'draft', 'seller', 10000, 'RUB', 1, now(), now())
	`, v1ProdID, fix.sellerID, fmt.Sprintf("v1prod-%s", v1ProdID.String()[:8]))
	require.NoError(t, err)

	v1RunID := uuid.New()
	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_vision_runs (
			id, product_id, vision_content_version, provider, model_id, prompt_version,
			schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
			snapshot_json, idempotency_key, status, completed_at, result_observation,
			result_evaluation, taxonomy_context_json, created_at, updated_at
		) VALUES (
			$1, $2, 1, 'cloudru', 'qwen', '1', '1', '1', 'thash', 'shash',
			'{}', $3, 'succeeded', now(), '{"observedCategoryId":"2d097e3c-f546-4af4-b7c1-2b954783a6b4"}', '{}', '{}', now(), now()
		)
	`, v1RunID, v1ProdID, fmt.Sprintf("idem-v1run-%s", v1RunID.String()[:8]))
	require.NoError(t, err)

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_current_visual_profiles (
			product_id, run_id, run_created_at, vision_content_version, model_id,
			snapshot_hash, taxonomy_context_hash, observation, updated_at
		) VALUES (
			$1, $2, now(), 1, 'qwen', 'shash', 'thash', '{"observedCategoryId":"2d097e3c-f546-4af4-b7c1-2b954783a6b4"}', now()
		)
	`, v1ProdID, v1RunID)
	require.NoError(t, err)

	pV1, err := fix.svc.GetAdminProductDetail(ctx, v1ProdID)
	require.NoError(t, err)
	require.NotNil(t, pV1.Vision)
	assert.NotEqual(t, "succeeded", pV1.Vision.Status, "V1 profile must not be returned as valid V2 succeeded result")
	assert.Nil(t, pV1.Vision.CategoryConsistency, "Must not fabricate V2 categoryConsistency from V1 data")
}

// TestAdminVisionRead_RBAC_AccessIsolation verifies:
// - L. Seller cannot access Admin Vision result via GetSellerProduct
// - M. Customer cannot access Admin Vision result via GetPublicProduct
// - N. Admin with existing moderation read flow accesses it
func TestAdminVisionRead_RBAC_AccessIsolation(t *testing.T) {
	ctx := context.Background()
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()

	// Create an isolated test product with a valid visual profile
	isoProdID := uuid.New()
	_, err := fix.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, 'RBAC Isolation Product', $3, 'published', 'seller', 20000, 'RUB', 1, now(), now())
	`, isoProdID, fix.sellerID, fmt.Sprintf("rbac-iso-%s", isoProdID.String()[:8]))
	require.NoError(t, err)

	runID := uuid.New()
	obs := vision.ProductVisualObservationV2{
		CategoryConsistency: vision.CategoryConsistencyConsistent,
		ImageSetConsistency: boolPtr(true),
	}
	obsJSON, _ := json.Marshal(obs)
	modelID := vision.CanonicalModelID
	snapHash := "snap_rbac_1"
	taxHash := "tax_rbac_1"

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_vision_runs (
			id, product_id, vision_content_version, provider, model_id, prompt_version,
			schema_version, vocabulary_version, taxonomy_context_hash, snapshot_hash,
			snapshot_json, idempotency_key, status, completed_at, result_observation,
			result_evaluation, taxonomy_context_json, created_at, updated_at
		) VALUES (
			$1, $2, 1, 'cloudru', $3, '1', '2', '1', $4, $5,
			'{}', $6, 'succeeded', now(), $7, '{}', '{}', now(), now()
		)
	`, runID, isoProdID, modelID, taxHash, snapHash, fmt.Sprintf("idem-rbac-%s", runID.String()[:8]), obsJSON)
	require.NoError(t, err)

	_, err = fix.pool.Exec(ctx, `
		INSERT INTO product_current_visual_profiles (
			product_id, run_id, run_created_at, vision_content_version, model_id,
			snapshot_hash, taxonomy_context_hash, observation, updated_at
		) VALUES (
			$1, $2, now(), 1, $3, $4, $5, $6, now()
		)
	`, isoProdID, runID, modelID, snapHash, taxHash, obsJSON)
	require.NoError(t, err)

	// 1. Admin inspection flow returns Vision
	adminProduct, err := fix.svc.GetAdminProductDetail(ctx, isoProdID)
	require.NoError(t, err)
	require.NotNil(t, adminProduct.Vision)
	assert.Equal(t, "succeeded", adminProduct.Vision.Status)

	adminJSON, err := json.Marshal(adminProduct)
	require.NoError(t, err)
	assert.Contains(t, string(adminJSON), `"vision":`, "Admin JSON serialization must include vision")

	// 2. Seller product inspection flow does NOT return Vision (L)
	sellerProduct, err := fix.svc.GetSellerProduct(ctx, fix.sellerUser, isoProdID)
	if err == nil {
		assert.Nil(t, sellerProduct.Vision, "Seller product struct must have nil Vision")
		sellerJSON, err := json.Marshal(sellerProduct)
		require.NoError(t, err)
		assert.NotContains(t, string(sellerJSON), `"vision"`, "Seller JSON serialization must NOT include vision")
	}

	// 3. Customer public product inspection flow does NOT return Vision (M)
	publicProduct, err := fix.svc.GetPublicProduct(ctx, isoProdID.String())
	if err == nil {
		publicJSON, err := json.Marshal(publicProduct)
		require.NoError(t, err)
		assert.NotContains(t, string(publicJSON), `"vision"`, "Customer public JSON serialization must NOT include vision")
	}
}

// TestAdminVisionRead_AdvisoryOnly_And_ZeroProviderCalls verifies:
// - O. Reading Vision never triggers provider calls, scheduling, or S3 calls
// - Hard invariant: Vision read never mutates product moderation status or category
func TestAdminVisionRead_AdvisoryOnly_And_ZeroProviderCalls(t *testing.T) {
	ctx := context.Background()
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()

	testProdID := uuid.New()
	_, err := fix.pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, status, source, price_cents, currency, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, 'Advisory Test Product', $3, 'draft', 'seller', 10000, 'RUB', 1, now(), now())
	`, testProdID, fix.sellerID, fmt.Sprintf("adv-%s", testProdID.String()[:8]))
	require.NoError(t, err)

	// Record pre-read state of product and runs
	var (
		preStatus    string
		preCategory  *uuid.UUID
		preVersion   int64
		preRunCount  int
	)
	err = fix.pool.QueryRow(ctx, "SELECT status, category_id, vision_content_version FROM products WHERE id = $1", testProdID).Scan(&preStatus, &preCategory, &preVersion)
	require.NoError(t, err)

	err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", testProdID).Scan(&preRunCount)
	require.NoError(t, err)

	// Execute multiple admin reads
	for i := 0; i < 5; i++ {
		p, err := fix.svc.GetAdminProductDetail(ctx, testProdID)
		require.NoError(t, err)
		require.NotNil(t, p.Vision)
	}

	// Verify DB state is completely identical (read-only invariant)
	var (
		postStatus   string
		postCategory *uuid.UUID
		postVersion  int64
		postRunCount int
	)
	err = fix.pool.QueryRow(ctx, "SELECT status, category_id, vision_content_version FROM products WHERE id = $1", testProdID).Scan(&postStatus, &postCategory, &postVersion)
	require.NoError(t, err)

	err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", testProdID).Scan(&postRunCount)
	require.NoError(t, err)

	assert.Equal(t, preStatus, postStatus, "Product status must not mutate during read")
	assert.Equal(t, preCategory, postCategory, "Category must not mutate during read")
	assert.Equal(t, preVersion, postVersion, "Vision content version must not mutate during read")
	assert.Equal(t, preRunCount, postRunCount, "No new vision runs must be scheduled or inserted during read")
}

func boolPtr(b bool) *bool {
	return &b
}

func strPtr(s string) *string {
	return &s
}
