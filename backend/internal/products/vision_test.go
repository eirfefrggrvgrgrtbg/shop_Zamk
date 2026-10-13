package products_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

type mockFailScheduler struct {
	err error
}

func (m *mockFailScheduler) EnsureVisionRun(ctx context.Context, params vision.ScheduleVisionRunParams) (*vision.VisionRun, bool, error) {
	return nil, false, m.err
}

func (m *mockFailScheduler) ScheduleCurrentProductVision(ctx context.Context, productID uuid.UUID) (*vision.VisionRun, error) {
	return nil, m.err
}

func setupVisionProductFixture(t *testing.T) (revisionPromotionFixture, *vision.Store) {
	t.Helper()
	fix := setupRevisionPromotionFixture(t)
	ctx := context.Background()

	// Insert 3 valid product images to satisfy moderation validation rules
	for i := 1; i <= 3; i++ {
		imgID := uuid.New()
		objKey := fmt.Sprintf("products/%s/%s/%s.jpg", fix.sellerID, fix.productID, imgID)
		_, err := fix.pool.Exec(ctx, `
			INSERT INTO product_images (id, product_id, image_url, object_key, rendition_url, rendition_object_key, crop_x, crop_y, crop_width, crop_height, sort_order, is_main, created_at)
			VALUES ($1, $2, $3, $4, $3, $4, 0.0, 0.0, 1.0, 1.0, $5, $6, now())
		`, imgID, fix.productID, "http://test/"+objKey, objKey, i, i == 1)
		require.NoError(t, err)
	}

	// Insert active variant
	varID := uuid.New()
	sku := fmt.Sprintf("SKU-%s", varID.String()[:8])
	_, err := fix.pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 25000, true, now(), now())
	`, varID, fix.productID, sku)
	require.NoError(t, err)

	visionStore := vision.NewStore(fix.pool)
	fix.svc.WithVisionScheduler(visionStore)
	return fix, visionStore
}

func TestVision_Scheduling_Blocker13(t *testing.T) {
	ctx := context.Background()

	// 1. Seller initial moderation submit: business transition succeeds, one run is ensured.
	t.Run("1. Seller initial moderation submit ensures run", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// Set product to draft status
		_, err := fix.pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", fix.productID)
		require.NoError(t, err)

		err = fix.svc.SubmitProductToModeration(ctx, fix.sellerUser, fix.productID, products.SubmitProductModerationRequest{})
		require.NoError(t, err)

		var status string
		err = fix.pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", fix.productID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "pending_moderation", status)

		var count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "One vision run must be scheduled")
	})

	// 2. Vision scheduler failure: moderation submit STILL succeeds.
	t.Run("2. Vision scheduler failure: moderation submit STILL succeeds", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		_, err := fix.pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", fix.productID)
		require.NoError(t, err)

		fix.svc.WithVisionScheduler(&mockFailScheduler{err: errors.New("vision scheduling down")})

		err = fix.svc.SubmitProductToModeration(ctx, fix.sellerUser, fix.productID, products.SubmitProductModerationRequest{})
		require.NoError(t, err, "Moderation submit must not fail when vision scheduler fails")

		var status string
		err = fix.pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", fix.productID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "pending_moderation", status)
	})

	// 3. Repeated scheduling: exact same identity does not duplicate.
	t.Run("3. Repeated scheduling: exact same identity does not duplicate", func(t *testing.T) {
		fix, visionStore := setupVisionProductFixture(t)
		defer fix.pool.Close()

		_, err := visionStore.ScheduleCurrentProductVision(ctx, fix.productID)
		require.NoError(t, err)

		_, err = visionStore.ScheduleCurrentProductVision(ctx, fix.productID)
		require.NoError(t, err)

		var count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Duplicate scheduling must not create duplicate run")
	})

	// 4. Initial approval: does not create duplicate paid identity if same snapshot already scheduled.
	t.Run("4. Initial approval does not create duplicate run if unchanged", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		_, err := fix.pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", fix.productID)
		require.NoError(t, err)

		// Submit
		err = fix.svc.SubmitProductToModeration(ctx, fix.sellerUser, fix.productID, products.SubmitProductModerationRequest{})
		require.NoError(t, err)

		// Approve
		err = fix.svc.ApproveProduct(ctx, fix.adminUser, fix.productID, nil)
		require.NoError(t, err)

		var count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Initial approval must not duplicate identical snapshot run")
	})

	// 5. Published revision pending: does NOT schedule old live state as if it were the revision.
	t.Run("5. Published revision pending: does not schedule old live state as revision", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// Product is published at version 5
		newTitle := "Wool Coat V2 - Luxury Edition"
		contSelling := true
		updReq := products.UpdateProductRequest{
			Title:           &newTitle,
			ContinueSelling: &contSelling,
		}
		updProd, err := fix.svc.UpdateProductForSeller(ctx, fix.sellerUser, fix.productID, updReq)
		require.NoError(t, err)
		require.NotNil(t, updProd.LiveRevisionID)

		// Live product still version 5. No version 6 run created yet until promotion!
		var v6Count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1 AND vision_content_version = 6", fix.productID).Scan(&v6Count)
		require.NoError(t, err)
		assert.Equal(t, 0, v6Count)
	})

	// 6. Revision promotion: N+1 current live product gets new run.
	t.Run("6. Revision promotion: N+1 current live product gets new run", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		newTitle := "Wool Coat V2 - Promoted"
		contSelling := true
		updReq := products.UpdateProductRequest{
			Title:           &newTitle,
			ContinueSelling: &contSelling,
		}
		_, err := fix.svc.UpdateProductForSeller(ctx, fix.sellerUser, fix.productID, updReq)
		require.NoError(t, err)

		// Approve revision (promotes to published and bumps vision_content_version to 6)
		err = fix.svc.ApproveProduct(ctx, fix.adminUser, fix.productID, nil)
		require.NoError(t, err)

		var v6Count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1 AND vision_content_version = 6", fix.productID).Scan(&v6Count)
		require.NoError(t, err)
		assert.Equal(t, 1, v6Count, "Revision promotion must schedule run for version 6")
	})

	// 7. Scheduling failure after revision approval: approval STILL succeeds.
	t.Run("7. Scheduling failure after revision approval: approval STILL succeeds", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		newTitle := "Wool Coat V2 - Failure Test"
		contSelling := true
		updReq := products.UpdateProductRequest{
			Title:           &newTitle,
			ContinueSelling: &contSelling,
		}
		updProd, err := fix.svc.UpdateProductForSeller(ctx, fix.sellerUser, fix.productID, updReq)
		require.NoError(t, err)

		// Set failing scheduler
		fix.svc.WithVisionScheduler(&mockFailScheduler{err: errors.New("vision down")})

		// Approve revision
		err = fix.svc.ApproveProduct(ctx, fix.adminUser, fix.productID, nil)
		require.NoError(t, err, "Approval must succeed even if vision scheduling fails")

		var revStatus string
		err = fix.pool.QueryRow(ctx, "SELECT status FROM product_revisions WHERE id = $1", *updProd.LiveRevisionID).Scan(&revStatus)
		require.NoError(t, err)
		assert.Equal(t, "approved", revStatus)
	})
}

func TestVision_Reconciliation_Blocker14(t *testing.T) {
	ctx := context.Background()

	// Drain any products missing current runs so each test isolates its own product
	drainFix, _ := setupVisionProductFixture(t)
	for {
		drained, err := drainFix.svc.ReconcileMissingVisionRuns(ctx, 500)
		require.NoError(t, err)
		if drained == 0 {
			break
		}
	}
	drainFix.pool.Close()

	// A. missing CURRENT inference identity -> scheduled
	t.Run("A. Missing CURRENT inference identity -> scheduled", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		reconciled, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, reconciled, 1)

		var count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	// B. existing exact current identity -> no duplicate
	t.Run("B. Existing exact current identity -> no duplicate", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// First reconcile
		_, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)

		// Second reconcile
		reconciled, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)
		assert.Equal(t, 0, reconciled)

		var count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	// C. old model run exists -> current model run still scheduled
	t.Run("C. Old model run exists -> current model run still scheduled", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// Reconcile to create run
		_, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)

		// Tamper existing run model_id to an old model
		_, err = fix.pool.Exec(ctx, "UPDATE product_vision_runs SET model_id = 'old-model-v0', idempotency_key = $1 WHERE product_id = $2", fmt.Sprintf("old-idem-%s", uuid.New()), fix.productID)
		require.NoError(t, err)

		// Reconcile again: must detect absence of CURRENT model and schedule it
		reconciled, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)
		assert.Equal(t, 1, reconciled)

		var count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1 AND model_id = $2", fix.productID, vision.CanonicalModelID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	// D. old prompt/schema/vocabulary run exists -> current identity still scheduled
	t.Run("D. Old prompt/schema/vocabulary run exists -> current identity still scheduled", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		_, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)

		// Tamper existing run to old prompt version
		_, err = fix.pool.Exec(ctx, "UPDATE product_vision_runs SET prompt_version = 'old-prompt-v0', idempotency_key = $1 WHERE product_id = $2", fmt.Sprintf("old-idem-prompt-%s", uuid.New()), fix.productID)
		require.NoError(t, err)

		// Reconcile again
		reconciled, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)
		assert.Equal(t, 1, reconciled)

		var count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1 AND prompt_version = $2", fix.productID, vision.PromptVersion).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	// E. old product content version -> current version scheduled
	t.Run("E. Old product content version -> current version scheduled", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		_, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)

		// Bump product content version to 6
		_, err = fix.pool.Exec(ctx, "UPDATE products SET vision_content_version = 6 WHERE id = $1", fix.productID)
		require.NoError(t, err)

		// Reconcile: must schedule version 6
		reconciled, err := fix.svc.ReconcileMissingVisionRuns(ctx, 100)
		require.NoError(t, err)
		assert.Equal(t, 1, reconciled)

		var v6Count int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1 AND vision_content_version = 6", fix.productID).Scan(&v6Count)
		require.NoError(t, err)
		assert.Equal(t, 1, v6Count)
	})

	// F. bounded limit respected & G. deterministic order
	t.Run("F & G. Bounded limit respected & deterministic order", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// First, reconcile all existing published products so only new ones remain unreconciled
		_, _ = fix.svc.ReconcileMissingVisionRuns(ctx, 500)

		// Create 3 published products without vision runs using deterministic zero-prefixed UUIDs
		pIDs := []uuid.UUID{
			uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		}
		for i, pID := range pIDs {
			_, _ = fix.pool.Exec(ctx, "DELETE FROM product_vision_runs WHERE product_id = $1", pID)
			_, _ = fix.pool.Exec(ctx, "DELETE FROM products WHERE id = $1", pID)

			pSlug := fmt.Sprintf("rec-prod-%d", i)
			_, err := fix.pool.Exec(ctx, `
				INSERT INTO products (id, seller_id, category_id, title, slug, description, status, source, currency, price_cents, vision_content_version, created_at, updated_at)
				VALUES ($1, $2, $3, 'Test Product', $4, 'Desc', 'published', 'seller', 'RUB', 10000, 1, now(), now())
			`, pID, fix.sellerID, fix.categoryID, pSlug)
			require.NoError(t, err)
		}

		// Reconcile with limit = 1
		reconciled, err := fix.svc.ReconcileMissingVisionRuns(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, reconciled, "Limit of 1 must be respected")

		// Verify the first in ORDER BY p.id ASC was scheduled
		var firstScheduled bool
		err = fix.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM product_vision_runs WHERE product_id = $1)", pIDs[0]).Scan(&firstScheduled)
		require.NoError(t, err)
		assert.True(t, firstScheduled, "First product by ascending ID must be scheduled")
	})
}

func TestVision_RuntimeWiring_A1(t *testing.T) {
	ctx := context.Background()

	// 1. VISION_ENABLED=true: products.Service wired with vision scheduler schedules run on moderation submit
	t.Run("1. VISION_ENABLED=true schedules run on moderation submit", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// Product in draft state
		_, err := fix.pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", fix.productID)
		require.NoError(t, err)

		// Submit to moderation
		err = fix.svc.SubmitProductToModeration(ctx, fix.sellerUser, fix.productID, products.SubmitProductModerationRequest{})
		require.NoError(t, err)

		var status string
		err = fix.pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", fix.productID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "pending_moderation", status)

		var runCount int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&runCount)
		require.NoError(t, err)
		assert.Equal(t, 1, runCount, "A vision run must be scheduled when vision scheduler is wired")
	})

	// 2. VISION_ENABLED=true: schedules run on revision promotion
	t.Run("2. VISION_ENABLED=true schedules run on revision promotion", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		newTitle := "Revision Product Title"
		contSelling := true
		updReq := products.UpdateProductRequest{
			Title:           &newTitle,
			ContinueSelling: &contSelling,
		}
		_, err := fix.svc.UpdateProductForSeller(ctx, fix.sellerUser, fix.productID, updReq)
		require.NoError(t, err)

		// Approve revision (promotes to published and triggers vision scheduling)
		err = fix.svc.ApproveProduct(ctx, fix.adminUser, fix.productID, nil)
		require.NoError(t, err)

		var runCount int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1 AND vision_content_version = 6", fix.productID).Scan(&runCount)
		require.NoError(t, err)
		assert.Equal(t, 1, runCount, "A vision run must be scheduled for promoted revision")
	})

	// 3. VISION_ENABLED=false: no scheduler wired, submit does not panic and creates zero vision runs
	t.Run("3. VISION_ENABLED=false does not schedule on submit and never panics", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// Disconnect scheduler to simulate VISION_ENABLED=false
		fix.svc.WithVisionScheduler(nil)

		// Product in draft state
		_, err := fix.pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", fix.productID)
		require.NoError(t, err)

		// Submit to moderation
		err = fix.svc.SubmitProductToModeration(ctx, fix.sellerUser, fix.productID, products.SubmitProductModerationRequest{})
		require.NoError(t, err, "Submit to moderation must succeed when vision is disabled")

		var status string
		err = fix.pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", fix.productID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "pending_moderation", status)

		var runCount int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&runCount)
		require.NoError(t, err)
		assert.Equal(t, 0, runCount, "Zero vision runs must be scheduled when vision is disabled")
	})

	// 4. VISION_ENABLED=false: no scheduler wired, approve revision does not panic and creates zero vision runs
	t.Run("4. VISION_ENABLED=false does not schedule on publish and never panics", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		// Disconnect scheduler to simulate VISION_ENABLED=false
		fix.svc.WithVisionScheduler(nil)

		newTitle := "Disabled Vision Revision Title"
		contSelling := true
		updReq := products.UpdateProductRequest{
			Title:           &newTitle,
			ContinueSelling: &contSelling,
		}
		_, err := fix.svc.UpdateProductForSeller(ctx, fix.sellerUser, fix.productID, updReq)
		require.NoError(t, err)

		err = fix.svc.ApproveProduct(ctx, fix.adminUser, fix.productID, nil)
		require.NoError(t, err, "Approve must succeed when vision is disabled")

		var runCount int
		err = fix.pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", fix.productID).Scan(&runCount)
		require.NoError(t, err)
		assert.Equal(t, 0, runCount, "Zero vision runs must be scheduled when vision is disabled")
	})

	// 5. API synchronous path performs zero provider calls: run row is pending, attempts=0, no observation
	t.Run("5. API path performs no provider inference synchronously", func(t *testing.T) {
		fix, _ := setupVisionProductFixture(t)
		defer fix.pool.Close()

		_, err := fix.pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", fix.productID)
		require.NoError(t, err)

		err = fix.svc.SubmitProductToModeration(ctx, fix.sellerUser, fix.productID, products.SubmitProductModerationRequest{})
		require.NoError(t, err)

		var runStatus string
		var attempts int
		var claimedAt, completedAt *string
		var obsJSON, evalJSON []byte
		err = fix.pool.QueryRow(ctx, `
			SELECT status, attempts, claimed_at::text, completed_at::text, result_observation, result_evaluation
			FROM product_vision_runs
			WHERE product_id = $1
		`, fix.productID).Scan(&runStatus, &attempts, &claimedAt, &completedAt, &obsJSON, &evalJSON)
		require.NoError(t, err)

		assert.Equal(t, vision.RunStatusPending, runStatus, "Run status must remain pending (not processing, not succeeded)")
		assert.Equal(t, 0, attempts, "Attempts must be 0 (no worker processing has occurred)")
		assert.Nil(t, claimedAt, "claimed_at must be null")
		assert.Nil(t, completedAt, "completed_at must be null")
		assert.Empty(t, obsJSON, "result_observation must be empty (no synchronous inference)")
		assert.Empty(t, evalJSON, "result_evaluation must be empty (no synchronous evaluation)")
	})
}
