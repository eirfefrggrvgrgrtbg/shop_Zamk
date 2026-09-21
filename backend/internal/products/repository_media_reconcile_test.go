package products_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestReconcileProductImagesTx(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	// Canonical DB Safety Guard before any mutation
	testutil.AssertTestDatabase(t, db.Pool)
	var dbName string
	err = db.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "integration test must use zamk_test database")

	// 1. Generate ALL fixture IDs up front
	userID := uuid.New()
	userID2 := uuid.New()
	sellerID := uuid.New()
	sellerID2 := uuid.New()
	brandID := uuid.New()
	catID := uuid.New()
	productID := uuid.New()
	productIDOtherSeller := uuid.New()
	colorID1 := uuid.New()
	colorID2 := uuid.New()
	cleanupKeyPrefix := fmt.Sprintf("cleanup-recon/%s", uuid.New())

	userEmail := fmt.Sprintf("recontest-%s@zamk.ru", userID)
	userEmail2 := fmt.Sprintf("recontest-%s@zamk.ru", userID2)
	brandSlug := fmt.Sprintf("brand-%s", brandID)
	catSlug := fmt.Sprintf("cat-%s", catID)
	prodSlug := fmt.Sprintf("prod-%s", productID)
	prodSlugOther := fmt.Sprintf("prod-%s", productIDOtherSeller)

	// 2. Register t.Cleanup BEFORE the first DB mutation
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		queries := []struct {
			name string
			sql  string
			args []any
		}{
			{
				name: "cleanup jobs",
				sql:  "DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE $1",
				args: []any{cleanupKeyPrefix + "%"},
			},
			{
				name: "product images",
				sql:  "DELETE FROM product_images WHERE product_id IN ($1, $2)",
				args: []any{productID, productIDOtherSeller},
			},
			{
				name: "product media staging",
				sql:  "DELETE FROM product_media_staging WHERE seller_id IN ($1, $2)",
				args: []any{sellerID, sellerID2},
			},
			{
				name: "products",
				sql:  "DELETE FROM products WHERE id IN ($1, $2)",
				args: []any{productID, productIDOtherSeller},
			},
			{
				name: "colors",
				sql:  "DELETE FROM colors WHERE id IN ($1, $2)",
				args: []any{colorID1, colorID2},
			},
			{
				name: "categories",
				sql:  "DELETE FROM categories WHERE id = $1",
				args: []any{catID},
			},
			{
				name: "brands",
				sql:  "DELETE FROM brands WHERE id = $1",
				args: []any{brandID},
			},
			{
				name: "sellers",
				sql:  "DELETE FROM sellers WHERE id IN ($1, $2)",
				args: []any{sellerID, sellerID2},
			},
			{
				name: "users",
				sql:  "DELETE FROM users WHERE id IN ($1, $2)",
				args: []any{userID, userID2},
			},
		}

		for _, q := range queries {
			if _, err := db.Pool.Exec(cleanupCtx, q.sql, q.args...); err != nil {
				t.Errorf("cleanup failed for %s: %v", q.name, err)
			}
		}
		db.Close()
	})

	// 3. Insert baseline fixtures with strict error checking
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name) VALUES ($1, $2, 'hash', 'Test User')", userID, userEmail)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Recon Test Seller', 'active', $2)", sellerID, userEmail)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name) VALUES ($1, $2, 'hash', 'Test User 2')", userID2, userEmail2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Recon Test Seller 2', 'active', $2)", sellerID2, userEmail2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'Recon Test Brand', $2, true)", brandID, brandSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Recon Test Category', $2)", catID, catSlug)
	require.NoError(t, err)

	colorCode1 := fmt.Sprintf("color-1-%s", colorID1)
	colorCode2 := fmt.Sprintf("color-2-%s", colorID2)
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Color 1', '#111111', true)", colorID1, colorCode1)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Color 2', '#222222', true)", colorID2, colorCode2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'Test Product', $5, 'draft', 100, 'RUB')", productID, sellerID, brandID, catID, prodSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'Test Product Other', $5, 'draft', 200, 'RUB')", productIDOtherSeller, sellerID2, brandID, catID, prodSlugOther)
	require.NoError(t, err)

	repo := products.NewRepository(db.Pool)

	// Helper to run reconciliation in a standalone transaction
	runReconcile := func(desired []products.DesiredProductImage) error {
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		txRepo := repo.WithTx(tx)
		if err := txRepo.ReconcileProductImagesTx(ctx, sellerID, productID, desired); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// Helper to insert a canonical image directly
	insertCanonical := func(id uuid.UUID, imgURL string, objKey, rendKey *string, isMain bool, sortOrder int, colorID *uuid.UUID) {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO product_images (
				id, product_id, image_url, object_key, rendition_object_key,
				is_main, sort_order, color_id, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		`, id, productID, imgURL, objKey, rendKey, isMain, sortOrder, colorID)
		require.NoError(t, err)
	}

	// Helper to insert a staged media item directly
	insertStaged := func(id uuid.UUID, sID, pID uuid.UUID, status products.ProductMediaStagingStatus, objKey, imgURL string) {
		var consumedAt *time.Time
		if status == products.ProductMediaStagingConsumed {
			now := time.Now().UTC()
			consumedAt = &now
		}
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO product_media_staging (
				id, seller_id, product_id, client_media_id, status,
				object_key, image_url, content_sha256, byte_size, width, height,
				created_at, consumed_at
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 1024, 800, 1000,
				NOW(), $8
			)
		`, id, sID, pID, uuid.New(), status, objKey, imgURL, consumedAt)
		require.NoError(t, err)
	}

	// ---------------------------------------------------------
	// Test 1: desired empty -> clear all, cleanup queued, main fields NULL
	// ---------------------------------------------------------
	t.Run("1. desired empty: all canonical removed, cleanup enqueued, main NULL", func(t *testing.T) {
		imgID := uuid.New()
		objKey := fmt.Sprintf("%s/img1.jpg", cleanupKeyPrefix)
		rendKey := fmt.Sprintf("%s/rend1.jpg", cleanupKeyPrefix)
		insertCanonical(imgID, "http://img1.jpg", &objKey, &rendKey, true, 0, nil)

		// Set product main fields
		_, err = db.Pool.Exec(ctx, "UPDATE products SET main_image_url = 'http://img1.jpg', main_image_object_key = $1 WHERE id = $2", objKey, productID)
		require.NoError(t, err)

		err = runReconcile([]products.DesiredProductImage{})
		require.NoError(t, err)

		// Verify canonical images empty
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE product_id = $1", productID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// Verify cleanup queue received both object_key and rendition_object_key
		var objCount, rendCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", objKey).Scan(&objCount)
		require.NoError(t, err)
		require.Equal(t, 1, objCount)

		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", rendKey).Scan(&rendCount)
		require.NoError(t, err)
		require.Equal(t, 1, rendCount)

		// Verify product main fields are NULL
		var mainURL, mainKey *string
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", productID).Scan(&mainURL, &mainKey)
		require.NoError(t, err)
		require.Nil(t, mainURL)
		require.Nil(t, mainKey)
	})

	// ---------------------------------------------------------
	// Test 2: reorder existing canonical images
	// ---------------------------------------------------------
	t.Run("2. reorder existing canonical images", func(t *testing.T) {
		idA := uuid.New()
		idB := uuid.New()
		keyA := fmt.Sprintf("%s/order_a.jpg", cleanupKeyPrefix)
		keyB := fmt.Sprintf("%s/order_b.jpg", cleanupKeyPrefix)

		insertCanonical(idA, "http://a.jpg", &keyA, nil, true, 0, nil)
		insertCanonical(idB, "http://b.jpg", &keyB, nil, false, 1, nil)

		// Reorder: B becomes 0 and isMain, A becomes 1
		desired := []products.DesiredProductImage{
			{ID: idB, IsMain: true, SortOrder: 0},
			{ID: idA, IsMain: false, SortOrder: 1},
		}

		err := runReconcile(desired)
		require.NoError(t, err)

		// Verify sort_order and is_main updated, storage keys preserved
		var bOrder int
		var bMain bool
		var bKey *string
		err = db.Pool.QueryRow(ctx, "SELECT sort_order, is_main, object_key FROM product_images WHERE id = $1", idB).Scan(&bOrder, &bMain, &bKey)
		require.NoError(t, err)
		require.Equal(t, 0, bOrder)
		require.True(t, bMain)
		require.Equal(t, keyB, *bKey)

		var aOrder int
		var aMain bool
		var aKey *string
		err = db.Pool.QueryRow(ctx, "SELECT sort_order, is_main, object_key FROM product_images WHERE id = $1", idA).Scan(&aOrder, &aMain, &aKey)
		require.NoError(t, err)
		require.Equal(t, 1, aOrder)
		require.False(t, aMain)
		require.Equal(t, keyA, *aKey)

		// Clean up for next tests
		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 3: change canonical color binding
	// ---------------------------------------------------------
	t.Run("3. change canonical color binding", func(t *testing.T) {
		idA := uuid.New()
		keyA := fmt.Sprintf("%s/color_a.jpg", cleanupKeyPrefix)
		insertCanonical(idA, "http://color_a.jpg", &keyA, nil, true, 0, &colorID1)

		desired := []products.DesiredProductImage{
			{ID: idA, IsMain: true, SortOrder: 0, ColorID: &colorID2},
		}
		err := runReconcile(desired)
		require.NoError(t, err)

		var resColorID *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT color_id FROM product_images WHERE id = $1", idA).Scan(&resColorID)
		require.NoError(t, err)
		require.Equal(t, colorID2, *resColorID)

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 4: promote READY staged
	// ---------------------------------------------------------
	t.Run("4. promote READY staged", func(t *testing.T) {
		stagedID := uuid.New()
		key := fmt.Sprintf("%s/staged_promote.jpg", cleanupKeyPrefix)
		imgURL := "http://storage.zamk/staged_promote.jpg"
		insertStaged(stagedID, sellerID, productID, products.ProductMediaStagingReady, key, imgURL)

		desired := []products.DesiredProductImage{
			{ID: stagedID, IsMain: true, SortOrder: 0},
		}
		err := runReconcile(desired)
		require.NoError(t, err)

		// Canonical row created with exact UUID, key, url
		var canID uuid.UUID
		var canURL string
		var canKey *string
		var isMain bool
		err = db.Pool.QueryRow(ctx, "SELECT id, image_url, object_key, is_main FROM product_images WHERE id = $1", stagedID).Scan(&canID, &canURL, &canKey, &isMain)
		require.NoError(t, err)
		require.Equal(t, stagedID, canID)
		require.Equal(t, imgURL, canURL)
		require.Equal(t, key, *canKey)
		require.True(t, isMain)

		// Staging row status is consumed and consumed_at populated
		var stgStatus products.ProductMediaStagingStatus
		var consumedAt *time.Time
		err = db.Pool.QueryRow(ctx, "SELECT status, consumed_at FROM product_media_staging WHERE id = $1", stagedID).Scan(&stgStatus, &consumedAt)
		require.NoError(t, err)
		require.Equal(t, products.ProductMediaStagingConsumed, stgStatus)
		require.NotNil(t, consumedAt)

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 5: mixed existing + staged
	// ---------------------------------------------------------
	t.Run("5. mixed existing + staged", func(t *testing.T) {
		canID := uuid.New()
		canKey := fmt.Sprintf("%s/mixed_can.jpg", cleanupKeyPrefix)
		insertCanonical(canID, "http://mixed_can.jpg", &canKey, nil, true, 0, nil)

		stagedID := uuid.New()
		stagedKey := fmt.Sprintf("%s/mixed_staged.jpg", cleanupKeyPrefix)
		insertStaged(stagedID, sellerID, productID, products.ProductMediaStagingReady, stagedKey, "http://mixed_staged.jpg")

		desired := []products.DesiredProductImage{
			{ID: canID, IsMain: false, SortOrder: 0},
			{ID: stagedID, IsMain: true, SortOrder: 1},
		}
		err := runReconcile(desired)
		require.NoError(t, err)

		var total int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE product_id = $1", productID).Scan(&total)
		require.NoError(t, err)
		require.Equal(t, 2, total)

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 6: replace old canonical with staged
	// ---------------------------------------------------------
	t.Run("6. replace old canonical with staged", func(t *testing.T) {
		oldCanID := uuid.New()
		oldKey := fmt.Sprintf("%s/old_can.jpg", cleanupKeyPrefix)
		oldRendKey := fmt.Sprintf("%s/old_rend.jpg", cleanupKeyPrefix)
		insertCanonical(oldCanID, "http://old_can.jpg", &oldKey, &oldRendKey, true, 0, nil)

		stagedID := uuid.New()
		stagedKey := fmt.Sprintf("%s/new_staged.jpg", cleanupKeyPrefix)
		insertStaged(stagedID, sellerID, productID, products.ProductMediaStagingReady, stagedKey, "http://new_staged.jpg")

		desired := []products.DesiredProductImage{
			{ID: stagedID, IsMain: true, SortOrder: 0},
		}
		err := runReconcile(desired)
		require.NoError(t, err)

		// Old canonical is gone
		var oldRemaining int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", oldCanID).Scan(&oldRemaining)
		require.NoError(t, err)
		require.Equal(t, 0, oldRemaining)

		// Old keys queued for cleanup
		var qCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key IN ($1, $2)", oldKey, oldRendKey).Scan(&qCount)
		require.NoError(t, err)
		require.Equal(t, 2, qCount)

		// Staged promoted
		var newCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", stagedID).Scan(&newCount)
		require.NoError(t, err)
		require.Equal(t, 1, newCount)

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 7 & 8: omitted canonical object key & duplicate cleanup key
	// ---------------------------------------------------------
	t.Run("7 & 8. omitted canonical object key and duplicate cleanup generation semantics", func(t *testing.T) {
		sharedKey := fmt.Sprintf("%s/shared_dup.jpg", cleanupKeyPrefix)

		// Pre-enqueue the key once
		err := repo.EnqueueMediaCleanup(ctx, sharedKey)
		require.NoError(t, err)

		var gen1 int64
		err = db.Pool.QueryRow(ctx, "SELECT generation FROM product_media_cleanup_jobs WHERE object_key = $1", sharedKey).Scan(&gen1)
		require.NoError(t, err)
		require.Equal(t, int64(1), gen1)

		// Create canonical image with this same key
		canID := uuid.New()
		insertCanonical(canID, "http://dup.jpg", &sharedKey, nil, true, 0, nil)

		// Reconcile with empty desired (omits canID)
		err = runReconcile([]products.DesiredProductImage{})
		require.NoError(t, err)

		// Queue row must NOT duplicate (COUNT=1), but generation must increment (gen=2)
		var totalJobs int
		var gen2 int64
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*), MAX(generation) FROM product_media_cleanup_jobs WHERE object_key = $1", sharedKey).Scan(&totalJobs, &gen2)
		require.NoError(t, err)
		require.Equal(t, 1, totalJobs, "no duplicate physical queue rows")
		require.Equal(t, int64(2), gen2, "generation incremented")
	})

	// ---------------------------------------------------------
	// Test 9: main image derived onto products
	// ---------------------------------------------------------
	t.Run("9. main image: exactly one canonical is_main and products main fields derived", func(t *testing.T) {
		id1 := uuid.New()
		id2 := uuid.New()
		key1 := fmt.Sprintf("%s/main1.jpg", cleanupKeyPrefix)
		key2 := fmt.Sprintf("%s/main2.jpg", cleanupKeyPrefix)

		insertCanonical(id1, "http://main1.jpg", &key1, nil, true, 0, nil)
		insertCanonical(id2, "http://main2.jpg", &key2, nil, false, 1, nil)

		// Switch main to id2
		desired := []products.DesiredProductImage{
			{ID: id1, IsMain: false, SortOrder: 0},
			{ID: id2, IsMain: true, SortOrder: 1},
		}
		err := runReconcile(desired)
		require.NoError(t, err)

		var mainURL, mainKey *string
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", productID).Scan(&mainURL, &mainKey)
		require.NoError(t, err)
		require.Equal(t, "http://main2.jpg", *mainURL)
		require.Equal(t, key2, *mainKey)

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 10: uploading staging rejected
	// ---------------------------------------------------------
	t.Run("10. uploading staging: reject without mutation", func(t *testing.T) {
		stagedID := uuid.New()
		key := fmt.Sprintf("%s/uploading.jpg", cleanupKeyPrefix)
		insertStaged(stagedID, sellerID, productID, products.ProductMediaStagingUploading, key, "http://uploading.jpg")

		desired := []products.DesiredProductImage{
			{ID: stagedID, IsMain: true, SortOrder: 0},
		}
		err := runReconcile(desired)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrStagedMediaNotReady))

		// No canonical images inserted
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", stagedID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	// ---------------------------------------------------------
	// Test 11: unknown / foreign-or-missing ID rejected generically
	// ---------------------------------------------------------
	t.Run("11. unknown / foreign-or-missing ID: ErrInvalidMediaReference", func(t *testing.T) {
		// Nonexistent ID
		missingID := uuid.New()
		err := runReconcile([]products.DesiredProductImage{
			{ID: missingID, IsMain: true, SortOrder: 0},
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrInvalidMediaReference))

		// Foreign seller/product staged ID
		foreignStagedID := uuid.New()
		foreignKey := fmt.Sprintf("%s/foreign.jpg", cleanupKeyPrefix)
		insertStaged(foreignStagedID, sellerID2, productIDOtherSeller, products.ProductMediaStagingReady, foreignKey, "http://foreign.jpg")

		err = runReconcile([]products.DesiredProductImage{
			{ID: foreignStagedID, IsMain: true, SortOrder: 0},
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrInvalidMediaReference), "foreign ID must produce the same generic ErrInvalidMediaReference")
	})

	// ---------------------------------------------------------
	// Test 12: consumed staging + canonical same UUID: retry succeeds
	// ---------------------------------------------------------
	t.Run("12. consumed staging + canonical same UUID: retry succeeds", func(t *testing.T) {
		id := uuid.New()
		key := fmt.Sprintf("%s/retry_consumed.jpg", cleanupKeyPrefix)
		insertStaged(id, sellerID, productID, products.ProductMediaStagingConsumed, key, "http://retry_consumed.jpg")
		insertCanonical(id, "http://retry_consumed.jpg", &key, nil, true, 0, nil)

		desired := []products.DesiredProductImage{
			{ID: id, IsMain: true, SortOrder: 0},
		}
		err := runReconcile(desired)
		require.NoError(t, err, "retry with consumed staging and canonical must succeed")

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 13: staging metadata absent + canonical same UUID: retry succeeds
	// ---------------------------------------------------------
	t.Run("13. staging metadata absent + canonical same UUID: retry succeeds", func(t *testing.T) {
		id := uuid.New()
		key := fmt.Sprintf("%s/retry_absent.jpg", cleanupKeyPrefix)
		insertCanonical(id, "http://retry_absent.jpg", &key, nil, true, 0, nil)

		desired := []products.DesiredProductImage{
			{ID: id, IsMain: true, SortOrder: 0},
		}
		err := runReconcile(desired)
		require.NoError(t, err, "retry when staging is TTL-swept must succeed")

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 14: consumed staging without canonical: ErrMediaIntegrityViolation
	// ---------------------------------------------------------
	t.Run("14. consumed staging without canonical: ErrMediaIntegrityViolation", func(t *testing.T) {
		id := uuid.New()
		key := fmt.Sprintf("%s/consumed_missing.jpg", cleanupKeyPrefix)
		insertStaged(id, sellerID, productID, products.ProductMediaStagingConsumed, key, "http://consumed_missing.jpg")

		desired := []products.DesiredProductImage{
			{ID: id, IsMain: true, SortOrder: 0},
		}
		err := runReconcile(desired)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrMediaIntegrityViolation))
	})

	// ---------------------------------------------------------
	// Test 15-19: structural validations
	// ---------------------------------------------------------
	t.Run("15. duplicate desired IDs rejected", func(t *testing.T) {
		id := uuid.New()
		err := runReconcile([]products.DesiredProductImage{
			{ID: id, IsMain: true, SortOrder: 0},
			{ID: id, IsMain: false, SortOrder: 1},
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrInvalidMediaSet))
	})

	t.Run("16. >8 rejected", func(t *testing.T) {
		var desired []products.DesiredProductImage
		for i := 0; i < 9; i++ {
			desired = append(desired, products.DesiredProductImage{
				ID:        uuid.New(),
				IsMain:    i == 0,
				SortOrder: i,
			})
		}
		err := runReconcile(desired)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrInvalidMediaSet))
	})

	t.Run("17. non-empty zero-main rejected", func(t *testing.T) {
		id := uuid.New()
		err := runReconcile([]products.DesiredProductImage{
			{ID: id, IsMain: false, SortOrder: 0},
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrInvalidMediaSet))
	})

	t.Run("18. two-main rejected", func(t *testing.T) {
		id1 := uuid.New()
		id2 := uuid.New()
		err := runReconcile([]products.DesiredProductImage{
			{ID: id1, IsMain: true, SortOrder: 0},
			{ID: id2, IsMain: true, SortOrder: 1},
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrInvalidMediaSet))
	})

	t.Run("19. empty final set accepts zero-main", func(t *testing.T) {
		err := runReconcile([]products.DesiredProductImage{})
		require.NoError(t, err)
	})

	// ---------------------------------------------------------
	// Test 20: cleanup enqueue failure: rolls back EVERYTHING
	// ---------------------------------------------------------
	t.Run("20. cleanup enqueue failure rolls back all media changes", func(t *testing.T) {
		canID := uuid.New()
		key := fmt.Sprintf("%s/fail_cleanup.jpg", cleanupKeyPrefix)
		insertCanonical(canID, "http://fail_cleanup.jpg", &key, nil, true, 0, nil)

		tx, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		// Add constraint to fail cleanup job inserts in this tx
		_, err = tx.Exec(ctx, "ALTER TABLE product_media_cleanup_jobs ADD CONSTRAINT test_fail_cleanup CHECK (false) NOT VALID")
		require.NoError(t, err)

		txRepo := repo.WithTx(tx)
		// Try to omit canID (trigger cleanup enqueue)
		err = txRepo.ReconcileProductImagesTx(ctx, sellerID, productID, []products.DesiredProductImage{})
		require.Error(t, err)

		// Explicit rollback
		err = tx.Rollback(ctx)
		require.NoError(t, err)

		// Verify via pool: canonical image was NOT deleted
		var remaining int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", canID).Scan(&remaining)
		require.NoError(t, err)
		require.Equal(t, 1, remaining, "canonical image must survive rolled-back transaction")

		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
	})

	// ---------------------------------------------------------
	// Test 21: canonical insert/update failure: no partial final state
	// ---------------------------------------------------------
	t.Run("21. canonical insert/update failure: no partial state survives rollback", func(t *testing.T) {
		stagedID := uuid.New()
		key := fmt.Sprintf("%s/fail_insert.jpg", cleanupKeyPrefix)
		insertStaged(stagedID, sellerID, productID, products.ProductMediaStagingReady, key, "http://fail_insert.jpg")

		tx, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		// Add constraint to fail product_images insert in this tx
		_, err = tx.Exec(ctx, "ALTER TABLE product_images ADD CONSTRAINT test_fail_img_insert CHECK (false) NOT VALID")
		require.NoError(t, err)

		txRepo := repo.WithTx(tx)
		err = txRepo.ReconcileProductImagesTx(ctx, sellerID, productID, []products.DesiredProductImage{
			{ID: stagedID, IsMain: true, SortOrder: 0},
		})
		require.Error(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		// Staging row must remain 'ready', not 'consumed'
		var stgStatus products.ProductMediaStagingStatus
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE id = $1", stagedID).Scan(&stgStatus)
		require.NoError(t, err)
		require.Equal(t, products.ProductMediaStagingReady, stgStatus)
	})

	// ---------------------------------------------------------
	// Test 22: mark-consumed failure: no canonical promotion survives rollback
	// ---------------------------------------------------------
	t.Run("22. mark-consumed failure: no canonical promotion survives rollback", func(t *testing.T) {
		stagedID := uuid.New()
		key := fmt.Sprintf("%s/fail_consumed.jpg", cleanupKeyPrefix)
		insertStaged(stagedID, sellerID, productID, products.ProductMediaStagingReady, key, "http://fail_consumed.jpg")

		tx, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		// Add constraint to fail marking staging consumed in this tx
		_, err = tx.Exec(ctx, "ALTER TABLE product_media_staging ADD CONSTRAINT test_fail_staging_status CHECK (status != 'consumed') NOT VALID")
		require.NoError(t, err)

		txRepo := repo.WithTx(tx)
		err = txRepo.ReconcileProductImagesTx(ctx, sellerID, productID, []products.DesiredProductImage{
			{ID: stagedID, IsMain: true, SortOrder: 0},
		})
		require.Error(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		// Canonical image must NOT exist
		var canCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", stagedID).Scan(&canCount)
		require.NoError(t, err)
		require.Equal(t, 0, canCount, "canonical image must not survive mark-consumed failure")

		// Staging row remains 'ready'
		var stgStatus products.ProductMediaStagingStatus
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE id = $1", stagedID).Scan(&stgStatus)
		require.NoError(t, err)
		require.Equal(t, products.ProductMediaStagingReady, stgStatus)
	})

	// ---------------------------------------------------------
	// Test 23: canonical + same-scope READY staging same UUID
	// ---------------------------------------------------------
	t.Run("23. canonical + same-scope READY staging same UUID: ErrMediaIntegrityViolation", func(t *testing.T) {
		id := uuid.New()
		key := fmt.Sprintf("%s/active_ready.jpg", cleanupKeyPrefix)
		insertCanonical(id, "http://active_ready.jpg", &key, nil, true, 0, nil)
		insertStaged(id, sellerID, productID, products.ProductMediaStagingReady, key, "http://active_ready.jpg")

		// Set initial product main image fields
		_, err := db.Pool.Exec(ctx, "UPDATE products SET main_image_url = 'http://active_ready.jpg', main_image_object_key = $1 WHERE id = $2", key, productID)
		require.NoError(t, err)

		desired := []products.DesiredProductImage{
			{ID: id, IsMain: true, SortOrder: 0},
		}
		err = runReconcile(desired)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrMediaIntegrityViolation))

		// Verify canonical row unchanged
		var canCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", id).Scan(&canCount)
		require.NoError(t, err)
		require.Equal(t, 1, canCount)

		// Verify staging status still ready
		var stgStatus products.ProductMediaStagingStatus
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE id = $1", id).Scan(&stgStatus)
		require.NoError(t, err)
		require.Equal(t, products.ProductMediaStagingReady, stgStatus)

		// Verify no cleanup job created
		var cleanupCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&cleanupCount)
		require.NoError(t, err)
		require.Equal(t, 0, cleanupCount)

		// Verify product main fields unchanged
		var mainURL, mainKey *string
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", productID).Scan(&mainURL, &mainKey)
		require.NoError(t, err)
		require.Equal(t, "http://active_ready.jpg", *mainURL)
		require.Equal(t, key, *mainKey)

		// Clean up for next test
		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_media_staging WHERE id = $1", id)
	})

	// ---------------------------------------------------------
	// Test 24: canonical + same-scope UPLOADING staging same UUID
	// ---------------------------------------------------------
	t.Run("24. canonical + same-scope UPLOADING staging same UUID: ErrMediaIntegrityViolation", func(t *testing.T) {
		id := uuid.New()
		key := fmt.Sprintf("%s/active_uploading.jpg", cleanupKeyPrefix)
		insertCanonical(id, "http://active_uploading.jpg", &key, nil, true, 0, nil)
		insertStaged(id, sellerID, productID, products.ProductMediaStagingUploading, key, "http://active_uploading.jpg")

		// Set initial product main image fields
		_, err := db.Pool.Exec(ctx, "UPDATE products SET main_image_url = 'http://active_uploading.jpg', main_image_object_key = $1 WHERE id = $2", key, productID)
		require.NoError(t, err)

		desired := []products.DesiredProductImage{
			{ID: id, IsMain: true, SortOrder: 0},
		}
		err = runReconcile(desired)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrMediaIntegrityViolation))

		// Verify canonical row unchanged
		var canCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", id).Scan(&canCount)
		require.NoError(t, err)
		require.Equal(t, 1, canCount)

		// Verify staging status remains uploading
		var stgStatus products.ProductMediaStagingStatus
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE id = $1", id).Scan(&stgStatus)
		require.NoError(t, err)
		require.Equal(t, products.ProductMediaStagingUploading, stgStatus)

		// Verify no cleanup job created
		var cleanupCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&cleanupCount)
		require.NoError(t, err)
		require.Equal(t, 0, cleanupCount)

		// Verify product main fields unchanged
		var mainURL, mainKey *string
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", productID).Scan(&mainURL, &mainKey)
		require.NoError(t, err)
		require.Equal(t, "http://active_uploading.jpg", *mainURL)
		require.Equal(t, key, *mainKey)

		// Clean up
		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
		_, _ = db.Pool.Exec(ctx, "DELETE FROM product_media_staging WHERE id = $1", id)
	})
}
