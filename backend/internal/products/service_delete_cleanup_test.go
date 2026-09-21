package products_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestProductHardDeleteMediaCleanupSafety(t *testing.T) {
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
	cleanupKeyPrefix := fmt.Sprintf("del-test/%s", uuid.New())

	userEmail := fmt.Sprintf("deltest-%s@zamk.ru", userID)
	userEmail2 := fmt.Sprintf("deltest-%s@zamk.ru", userID2)
	brandSlug := fmt.Sprintf("brand-%s", brandID)
	catSlug := fmt.Sprintf("cat-%s", catID)

	// Track created product IDs for cleanup
	var createdProductIDs []uuid.UUID

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
				name: "product media staging",
				sql:  "DELETE FROM product_media_staging WHERE seller_id IN ($1, $2)",
				args: []any{sellerID, sellerID2},
			},
			{
				name: "products",
				sql:  "DELETE FROM products WHERE seller_id IN ($1, $2)",
				args: []any{sellerID, sellerID2},
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
				name: "seller_users",
				sql:  "DELETE FROM seller_users WHERE user_id IN ($1, $2)",
				args: []any{userID, userID2},
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

	// 3. Perform fixture setup
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, role) VALUES ($1, $2, 'hash', 'Delete Test User', 'seller')", userID, userEmail)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Delete Test Seller', 'active', $2)", sellerID, userEmail)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, role) VALUES ($1, $2, 'hash', 'Delete Test User 2', 'seller')", userID2, userEmail2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Delete Test Seller 2', 'active', $2)", sellerID2, userEmail2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID2, userID2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'Delete Test Brand', $2, true)", brandID, brandSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Delete Test Category', $2)", catID, catSlug)
	require.NoError(t, err)

	repo := products.NewRepository(db.Pool)
	sellerRepo := sellers.NewRepository(db.Pool)
	svc := products.NewService(repo, sellerRepo, db, nil, nil)

	createTestProduct := func(status string, sID uuid.UUID) uuid.UUID {
		pID := uuid.New()
		slug := fmt.Sprintf("prod-%s", pID)
		_, err := db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'Del Product', $5, $6, 100, 'RUB')", pID, sID, brandID, catID, slug, status)
		require.NoError(t, err)
		createdProductIDs = append(createdProductIDs, pID)
		return pID
	}

	t.Run("1. Product with canonical product_image: delete -> product gone, image gone, cleanup job exists", func(t *testing.T) {
		prodID := createTestProduct("draft", sellerID)
		imageKey := fmt.Sprintf("%s/canonical-1.jpg", cleanupKeyPrefix)
		renditionKey := fmt.Sprintf("%s/rendition-1.jpg", cleanupKeyPrefix)
		imgID := uuid.New()

		_, err := db.Pool.Exec(ctx, "INSERT INTO product_images (id, product_id, image_url, object_key, rendition_url, rendition_object_key) VALUES ($1, $2, 'http://img1.jpg', $3, 'http://rend1.jpg', $4)", imgID, prodID, imageKey, renditionKey)
		require.NoError(t, err)

		err = svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.NoError(t, err)

		// Product is gone
		var pCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE id = $1", prodID).Scan(&pCount)
		require.NoError(t, err)
		require.Equal(t, 0, pCount)

		// Image row is gone
		var imgCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", imgID).Scan(&imgCount)
		require.NoError(t, err)
		require.Equal(t, 0, imgCount)

		// Cleanup jobs exist for both keys
		var jobCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key IN ($1, $2)", imageKey, renditionKey).Scan(&jobCount)
		require.NoError(t, err)
		require.Equal(t, 2, jobCount)
	})

	t.Run("2. Product with uploading staged media: delete -> product gone, staging gone, cleanup job survives", func(t *testing.T) {
		prodID := createTestProduct("draft", sellerID)
		stagedKey := fmt.Sprintf("%s/staged-uploading.jpg", cleanupKeyPrefix)
		clientMediaID := uuid.New()

		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     prodID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     stagedKey,
			ImageURL:      "http://staged1.jpg",
			ContentSHA256: strings.Repeat("1", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		stagedRes, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)

		err = svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.NoError(t, err)

		// Staging row gone via cascade
		var smCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", stagedRes.ID).Scan(&smCount)
		require.NoError(t, err)
		require.Equal(t, 0, smCount)

		// Cleanup job survives
		var jobCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", stagedKey).Scan(&jobCount)
		require.NoError(t, err)
		require.Equal(t, 1, jobCount)
	})

	t.Run("3. Product with ready staged media: same invariant", func(t *testing.T) {
		prodID := createTestProduct("draft", sellerID)
		stagedKey := fmt.Sprintf("%s/staged-ready.jpg", cleanupKeyPrefix)
		clientMediaID := uuid.New()

		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     prodID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     stagedKey,
			ImageURL:      "http://staged2.jpg",
			ContentSHA256: strings.Repeat("2", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		stagedRes, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)

		err = repo.MarkStagedMediaReadyForSellerProduct(ctx, stagedRes.ID, sellerID, prodID)
		require.NoError(t, err)

		err = svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.NoError(t, err)

		// Cleanup job survives
		var jobCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", stagedKey).Scan(&jobCount)
		require.NoError(t, err)
		require.Equal(t, 1, jobCount)
	})

	t.Run("4. Product with consumed staged media and canonical same object_key -> exactly one cleanup job", func(t *testing.T) {
		prodID := createTestProduct("rejected", sellerID) // also testing 'rejected' status deletion
		sharedKey := fmt.Sprintf("%s/shared-key.jpg", cleanupKeyPrefix)
		clientMediaID := uuid.New()

		// Staged row (consumed)
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     prodID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     sharedKey,
			ImageURL:      "http://shared.jpg",
			ContentSHA256: strings.Repeat("3", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		stagedRes, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)
		require.NoError(t, repo.MarkStagedMediaReadyForSellerProduct(ctx, stagedRes.ID, sellerID, prodID))
		require.NoError(t, repo.MarkStagedMediaConsumedForSellerProduct(ctx, stagedRes.ID, sellerID, prodID))

		// Canonical row with same object key
		_, err = db.Pool.Exec(ctx, "INSERT INTO product_images (id, product_id, image_url, object_key) VALUES ($1, $2, 'http://shared.jpg', $3)", uuid.New(), prodID, sharedKey)
		require.NoError(t, err)

		err = svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.NoError(t, err)

		// Exactly ONE cleanup job
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", sharedKey).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "expected exactly 1 cleanup job for shared object_key")
	})

	t.Run("5. Product with canonical + staged different keys: cleanup jobs for all distinct keys", func(t *testing.T) {
		prodID := createTestProduct("draft", sellerID)
		key1 := fmt.Sprintf("%s/distinct-canon.jpg", cleanupKeyPrefix)
		key2 := fmt.Sprintf("%s/distinct-staged.jpg", cleanupKeyPrefix)

		_, err := db.Pool.Exec(ctx, "INSERT INTO product_images (id, product_id, image_url, object_key) VALUES ($1, $2, 'http://canon.jpg', $3)", uuid.New(), prodID, key1)
		require.NoError(t, err)

		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     prodID,
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     key2,
			ImageURL:      "http://staged.jpg",
			ContentSHA256: strings.Repeat("4", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		_, err = repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)

		err = svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.NoError(t, err)

		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key IN ($1, $2)", key1, key2).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count, "expected cleanup jobs for all distinct keys")
	})

	t.Run("6. Product with no media: normal delete works", func(t *testing.T) {
		prodID := createTestProduct("draft", sellerID)

		err := svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.NoError(t, err)

		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE id = $1", prodID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("7. Wrong seller: cannot delete, no cleanup jobs created", func(t *testing.T) {
		prodID := createTestProduct("draft", sellerID)
		key := fmt.Sprintf("%s/wrong-seller.jpg", cleanupKeyPrefix)

		_, err := db.Pool.Exec(ctx, "INSERT INTO product_images (id, product_id, image_url, object_key) VALUES ($1, $2, 'http://ws.jpg', $3)", uuid.New(), prodID, key)
		require.NoError(t, err)

		// User 2 (Seller 2) attempts to delete Seller 1's product
		err = svc.DeleteSellerDraftProduct(ctx, userID2, prodID)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrProductNotFound))

		// Product still exists
		var pCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE id = $1", prodID).Scan(&pCount)
		require.NoError(t, err)
		require.Equal(t, 1, pCount)

		// No cleanup job created
		var jobCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&jobCount)
		require.NoError(t, err)
		require.Equal(t, 0, jobCount)
	})

	t.Run("8. Non-deletable Product status: rejection preserved, no cleanup jobs created", func(t *testing.T) {
		prodID := createTestProduct("published", sellerID)
		key := fmt.Sprintf("%s/published-img.jpg", cleanupKeyPrefix)

		_, err := db.Pool.Exec(ctx, "INSERT INTO product_images (id, product_id, image_url, object_key) VALUES ($1, $2, 'http://pub.jpg', $3)", uuid.New(), prodID, key)
		require.NoError(t, err)

		err = svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrProductNotFound))

		// Product still exists
		var pCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE id = $1", prodID).Scan(&pCount)
		require.NoError(t, err)
		require.Equal(t, 1, pCount)

		// No cleanup job created
		var jobCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&jobCount)
		require.NoError(t, err)
		require.Equal(t, 0, jobCount)
	})

	t.Run("9. Forced enqueue failure: product remains, media/staging rows remain", func(t *testing.T) {
		prodID := createTestProduct("draft", sellerID)
		stagedKey := fmt.Sprintf("%s/valid-staged.jpg", cleanupKeyPrefix)
		// Insert an oversized key in product_images that exceeds VARCHAR(1024) of product_media_cleanup_jobs
		oversizedKey := strings.Repeat("o", 1025)

		imgID := uuid.New()
		_, err := db.Pool.Exec(ctx, "INSERT INTO product_images (id, product_id, image_url, object_key) VALUES ($1, $2, 'http://over.jpg', $3)", imgID, prodID, oversizedKey)
		require.NoError(t, err)

		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     prodID,
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     stagedKey,
			ImageURL:      "http://staged-ok.jpg",
			ContentSHA256: strings.Repeat("5", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		stagedRes, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)

		// Deletion should fail on enqueueing the oversized key
		err = svc.DeleteSellerDraftProduct(ctx, userID, prodID)
		require.Error(t, err)

		// Transaction must rollback: Product remains!
		var pCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE id = $1", prodID).Scan(&pCount)
		require.NoError(t, err)
		require.Equal(t, 1, pCount, "product must remain when cleanup enqueue fails")

		// Image row remains!
		var imgCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", imgID).Scan(&imgCount)
		require.NoError(t, err)
		require.Equal(t, 1, imgCount, "image row must remain when cleanup enqueue fails")

		// Staging row remains!
		var smCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", stagedRes.ID).Scan(&smCount)
		require.NoError(t, err)
		require.Equal(t, 1, smCount, "staging row must remain when cleanup enqueue fails")

		// No cleanup job was committed for stagedKey
		var jobCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", stagedKey).Scan(&jobCount)
		require.NoError(t, err)
		require.Equal(t, 0, jobCount, "no partial cleanup jobs should be committed")
	})
}
