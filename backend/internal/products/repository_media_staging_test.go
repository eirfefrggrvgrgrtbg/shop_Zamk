package products_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestProductMediaStaging(t *testing.T) {
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
	productID2 := uuid.New()
	productIDOtherSeller := uuid.New()
	cleanupKeyPrefix := fmt.Sprintf("cleanup-test/%s", uuid.New())

	userEmail := fmt.Sprintf("stagingtest-%s@zamk.ru", userID)
	userEmail2 := fmt.Sprintf("stagingtest-%s@zamk.ru", userID2)
	brandSlug := fmt.Sprintf("brand-%s", brandID)
	catSlug := fmt.Sprintf("cat-%s", catID)
	prodSlug := fmt.Sprintf("prod-%s", productID)
	prodSlug2 := fmt.Sprintf("prod-%s", productID2)
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
				name: "product media staging",
				sql:  "DELETE FROM product_media_staging WHERE seller_id IN ($1, $2)",
				args: []any{sellerID, sellerID2},
			},
			{
				name: "products",
				sql:  "DELETE FROM products WHERE id IN ($1, $2, $3)",
				args: []any{productID, productID2, productIDOtherSeller},
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

	// 3. Perform fixture insertions with strict error assertions
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name) VALUES ($1, $2, 'hash', 'Test User')", userID, userEmail)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Staging Test Seller', 'active', $2)", sellerID, userEmail)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name) VALUES ($1, $2, 'hash', 'Test User 2')", userID2, userEmail2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Staging Test Seller 2', 'active', $2)", sellerID2, userEmail2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'Staging Test Brand', $2, true)", brandID, brandSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Staging Test Category', $2)", catID, catSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'Test Product', $5, 'draft', 100, 'RUB')", productID, sellerID, brandID, catID, prodSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'Test Product 2', $5, 'draft', 200, 'RUB')", productID2, sellerID, brandID, catID, prodSlug2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'Test Product Other', $5, 'draft', 300, 'RUB')", productIDOtherSeller, sellerID2, brandID, catID, prodSlugOther)
	require.NoError(t, err)

	repo := products.NewRepository(db.Pool)

	t.Run("1. ownership-safe create", func(t *testing.T) {
		// A. seller A + product A -> succeeds
		clientMediaID := uuid.New()
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/key.jpg",
			ImageURL:      "http://test/key.jpg",
			ContentSHA256: strings.Repeat("a", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		res, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, res.ID)
		require.Equal(t, products.ProductMediaStagingUploading, res.Status)

		// B. seller B + product A -> rejected with ErrUnauthorized
		clientMediaIDCross := uuid.New()
		smCross := &products.ProductMediaStaging{
			SellerID:      sellerID2,
			ProductID:     productID,
			ClientMediaID: clientMediaIDCross,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/cross.jpg",
			ImageURL:      "http://test/cross.jpg",
			ContentSHA256: strings.Repeat("a", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		_, err = repo.CreateOrGetStagedMedia(ctx, smCross)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrUnauthorized), "expected ErrUnauthorized for cross-seller create")

		// Verify no staging row was created for seller B + product A
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE seller_id = $1 AND product_id = $2 AND client_media_id = $3", sellerID2, productID, clientMediaIDCross).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "expected no staging row created for unauthorized seller")

		// C. nonexistent product -> rejected with ErrProductNotFound
		smNonexistent := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     uuid.New(),
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/nonexistent.jpg",
			ImageURL:      "http://test/nonexistent.jpg",
			ContentSHA256: strings.Repeat("a", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		_, err = repo.CreateOrGetStagedMedia(ctx, smNonexistent)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrProductNotFound), "expected ErrProductNotFound for nonexistent product")
	})

	t.Run("2. duplicate seller+product+clientMediaId resolved according to contract", func(t *testing.T) {
		clientMediaID := uuid.New()
		sha1 := strings.Repeat("1", 64)
		sm1 := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/key1.jpg",
			ImageURL:      "http://test/key1.jpg",
			ContentSHA256: sha1,
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		res1, err := repo.CreateOrGetStagedMedia(ctx, sm1)
		require.NoError(t, err)

		// Same payload => returns same row
		res2, err := repo.CreateOrGetStagedMedia(ctx, sm1)
		require.NoError(t, err)
		require.Equal(t, res1.ID, res2.ID, "expected same identity on idempotent retry")

		// Same clientMediaID but different content_sha256 => conflict error
		sm2 := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/key2.jpg",
			ImageURL:      "http://test/key2.jpg",
			ContentSHA256: strings.Repeat("2", 64),
			ByteSize:      2048,
			Width:         800,
			Height:        1000,
		}
		_, err = repo.CreateOrGetStagedMedia(ctx, sm2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "conflict")
	})

	t.Run("3. same clientMediaId on another product does not collide", func(t *testing.T) {
		clientMediaID := uuid.New()

		sm1 := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/key.jpg",
			ImageURL:      "http://test/key.jpg",
			ContentSHA256: strings.Repeat("a", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		res1, err := repo.CreateOrGetStagedMedia(ctx, sm1)
		require.NoError(t, err)

		sm2 := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID2,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/key2.jpg",
			ImageURL:      "http://test/key2.jpg",
			ContentSHA256: strings.Repeat("b", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		res2, err := repo.CreateOrGetStagedMedia(ctx, sm2)
		require.NoError(t, err)
		require.NotEqual(t, res1.ID, res2.ID)
	})

	t.Run("4. invalid status rejected", func(t *testing.T) {
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingStatus("invalid_status"),
			ObjectKey:     "test.jpg",
			ImageURL:      "http://test.jpg",
			ContentSHA256: strings.Repeat("c", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		_, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.Error(t, err)
	})

	t.Run("5. byte_size <= 0 rejected", func(t *testing.T) {
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test.jpg",
			ImageURL:      "http://test.jpg",
			ContentSHA256: strings.Repeat("c", 64),
			ByteSize:      0,
			Width:         800,
			Height:        1000,
		}
		_, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.Error(t, err)
	})

	t.Run("6. width/height <= 0 rejected", func(t *testing.T) {
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test.jpg",
			ImageURL:      "http://test.jpg",
			ContentSHA256: strings.Repeat("c", 64),
			ByteSize:      1024,
			Width:         0,
			Height:        1000,
		}
		_, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.Error(t, err)
	})

	t.Run("7. consumed requires consumed_at and non-consumed cannot have consumed_at", func(t *testing.T) {
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingConsumed,
			ObjectKey:     "test.jpg",
			ImageURL:      "http://test.jpg",
			ContentSHA256: strings.Repeat("c", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
			ConsumedAt:    nil,
		}
		_, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.Error(t, err)

		now := time.Now()
		sm2 := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: uuid.New(),
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test.jpg",
			ImageURL:      "http://test.jpg",
			ContentSHA256: strings.Repeat("c", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
			ConsumedAt:    &now,
		}
		_, err = repo.CreateOrGetStagedMedia(ctx, sm2)
		require.Error(t, err)
	})

	t.Run("8. ownership-scoped access: GetStagedMediaByIDForSellerProduct", func(t *testing.T) {
		clientMediaID := uuid.New()
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/ownership.jpg",
			ImageURL:      "http://test/ownership.jpg",
			ContentSHA256: strings.Repeat("f", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		created, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)

		// A. same staged ID + correct seller + correct product -> found
		found, err := repo.GetStagedMediaByIDForSellerProduct(ctx, created.ID, sellerID, productID)
		require.NoError(t, err)
		require.Equal(t, created.ID, found.ID)
		require.Equal(t, sellerID, found.SellerID)
		require.Equal(t, productID, found.ProductID)

		// B. same staged ID + wrong seller -> not found
		_, err = repo.GetStagedMediaByIDForSellerProduct(ctx, created.ID, sellerID2, productID)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrStagedMediaNotFound))

		// C. same staged ID + wrong product -> not found
		_, err = repo.GetStagedMediaByIDForSellerProduct(ctx, created.ID, sellerID, productID2)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrStagedMediaNotFound))
	})

	t.Run("9. ownership-scoped locking: GetStagedMediaForUpdateForSellerProduct", func(t *testing.T) {
		clientMediaID := uuid.New()
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "test/locking.jpg",
			ImageURL:      "http://test/locking.jpg",
			ContentSHA256: strings.Repeat("d", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		created, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)

		tx, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txRepo := repo.WithTx(tx)

		// A. Lock with correct seller and product -> found
		locked, err := txRepo.GetStagedMediaForUpdateForSellerProduct(ctx, created.ID, sellerID, productID)
		require.NoError(t, err)
		require.Equal(t, created.ID, locked.ID)

		// B. Lock with wrong seller -> not found
		_, err = txRepo.GetStagedMediaForUpdateForSellerProduct(ctx, created.ID, sellerID2, productID)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrStagedMediaNotFound))

		// C. Lock with wrong product -> not found
		_, err = txRepo.GetStagedMediaForUpdateForSellerProduct(ctx, created.ID, sellerID, productID2)
		require.Error(t, err)
		require.True(t, errors.Is(err, products.ErrStagedMediaNotFound))

		require.NoError(t, tx.Commit(ctx))
	})

	t.Run("10. cleanup object_key uniqueness and strength", func(t *testing.T) {
		key := fmt.Sprintf("%s/image.jpg", cleanupKeyPrefix)
		err := repo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)

		// Enqueueing again should NOT error, due to ON CONFLICT DO NOTHING
		err = repo.EnqueueMediaCleanup(ctx, key)
		require.NoError(t, err)

		// exactly ONE cleanup job row exists
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", key).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "expected exactly 1 cleanup job row for duplicate enqueue")
	})

	t.Run("11. lifecycle transitions using scoped mutators", func(t *testing.T) {
		clientMediaID := uuid.New()
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			Status:        products.ProductMediaStagingUploading,
			ObjectKey:     "lifecycle.jpg",
			ImageURL:      "http://lifecycle.jpg",
			ContentSHA256: strings.Repeat("e", 64),
			ByteSize:      1024,
			Width:         800,
			Height:        1000,
		}
		res, err := repo.CreateOrGetStagedMedia(ctx, sm)
		require.NoError(t, err)

		// Scoped mark ready
		err = repo.MarkStagedMediaReadyForSellerProduct(ctx, res.ID, sellerID, productID)
		require.NoError(t, err)

		// Scoped mark consumed
		err = repo.MarkStagedMediaConsumedForSellerProduct(ctx, res.ID, sellerID, productID)
		require.NoError(t, err)

		final, err := repo.GetStagedMediaByIDForSellerProduct(ctx, res.ID, sellerID, productID)
		require.NoError(t, err)
		require.Equal(t, products.ProductMediaStagingConsumed, final.Status)
		require.NotNil(t, final.ConsumedAt)
	})

	t.Run("12. ClaimStagedMediaSlotForSellerProduct transactional behavior", func(t *testing.T) {
		clientMediaID := uuid.New()
		sm := &products.ProductMediaStaging{
			SellerID:      sellerID,
			ProductID:     productID,
			ClientMediaID: clientMediaID,
			ObjectKey:     "claim_test.jpg",
			ImageURL:      "http://claim_test.jpg",
			ContentSHA256: strings.Repeat("f", 64),
			ByteSize:      2048,
			Width:         800,
			Height:        1000,
		}

		// A. Valid claim in tx
		err := db.RunInTx(ctx, func(tx pgx.Tx) error {
			txRepo := repo.WithTx(tx)
			res, err := txRepo.ClaimStagedMediaSlotForSellerProduct(ctx, sm, "active")
			if err != nil {
				return err
			}
			require.Equal(t, products.ProductMediaStagingUploading, res.Status)
			require.Equal(t, clientMediaID, res.ClientMediaID)
			return nil
		})
		require.NoError(t, err)

		// B. Idempotent retry with same SHA in tx returns existing row
		err = db.RunInTx(ctx, func(tx pgx.Tx) error {
			txRepo := repo.WithTx(tx)
			res, err := txRepo.ClaimStagedMediaSlotForSellerProduct(ctx, sm, "active")
			if err != nil {
				return err
			}
			require.Equal(t, products.ProductMediaStagingUploading, res.Status)
			return nil
		})
		require.NoError(t, err)

		// C. Retry with different SHA returns ErrStagedMediaConflict
		smConflict := *sm
		smConflict.ContentSHA256 = strings.Repeat("0", 64)
		err = db.RunInTx(ctx, func(tx pgx.Tx) error {
			txRepo := repo.WithTx(tx)
			_, err := txRepo.ClaimStagedMediaSlotForSellerProduct(ctx, &smConflict, "active")
			return err
		})
		require.ErrorIs(t, err, products.ErrStagedMediaConflict)

		// D. Nonexistent product returns ErrProductNotFound
		smNonexistent := *sm
		smNonexistent.ProductID = uuid.New()
		err = db.RunInTx(ctx, func(tx pgx.Tx) error {
			txRepo := repo.WithTx(tx)
			_, err := txRepo.ClaimStagedMediaSlotForSellerProduct(ctx, &smNonexistent, "active")
			return err
		})
		require.ErrorIs(t, err, products.ErrProductNotFound)

		// E. Foreign product returns ErrProductNotFound (no info leak)
		smForeign := *sm
		smForeign.ProductID = productIDOtherSeller
		err = db.RunInTx(ctx, func(tx pgx.Tx) error {
			txRepo := repo.WithTx(tx)
			_, err := txRepo.ClaimStagedMediaSlotForSellerProduct(ctx, &smForeign, "active")
			return err
		})
		require.ErrorIs(t, err, products.ErrProductNotFound)

		// F. Blocked seller status returns ErrProductNotEditable
		err = db.RunInTx(ctx, func(tx pgx.Tx) error {
			txRepo := repo.WithTx(tx)
			_, err := txRepo.ClaimStagedMediaSlotForSellerProduct(ctx, sm, "blocked")
			return err
		})
		require.ErrorIs(t, err, products.ErrProductNotEditable)
	})

	t.Run("Queue Acceptance Matrix", func(t *testing.T) {
		qPrefix := fmt.Sprintf("%s/queue-matrix-%s", cleanupKeyPrefix, uuid.New())
		t.Cleanup(func() {
			cleanupCtx := context.Background()
			if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE $1", qPrefix+"%"); err != nil {
				t.Errorf("cleanup failed for queue acceptance jobs: %v", err)
			}
		})

		// A. first enqueue: generation == 1
		keyA := fmt.Sprintf("%s/key-a.jpg", qPrefix)
		err := repo.EnqueueMediaCleanup(ctx, keyA)
		require.NoError(t, err)

		var jobAId uuid.UUID
		var genA int64
		var attA int
		var nextA time.Time
		var errA *string
		err = db.Pool.QueryRow(ctx, "SELECT id, generation, attempts, next_attempt_at, last_error FROM product_media_cleanup_jobs WHERE object_key = $1", keyA).
			Scan(&jobAId, &genA, &attA, &nextA, &errA)
		require.NoError(t, err)
		require.Equal(t, int64(1), genA, "first enqueue must have generation 1")
		require.Equal(t, 0, attA, "first enqueue must have attempts 0")
		require.Nil(t, errA, "first enqueue must have nil last_error")
		require.True(t, nextA.Before(time.Now().Add(5*time.Second)), "first enqueue must be promptly eligible")

		// B. duplicate enqueue same object_key: same DB row, generation == 2, next_attempt_at becomes promptly eligible
		err = repo.EnqueueMediaCleanup(ctx, keyA)
		require.NoError(t, err)

		var jobBId uuid.UUID
		var genB int64
		var nextB time.Time
		err = db.Pool.QueryRow(ctx, "SELECT id, generation, next_attempt_at FROM product_media_cleanup_jobs WHERE object_key = $1", keyA).
			Scan(&jobBId, &genB, &nextB)
		require.NoError(t, err)
		require.Equal(t, jobAId, jobBId, "duplicate enqueue must maintain same DB row")
		require.Equal(t, int64(2), genB, "duplicate enqueue must increment generation")
		require.True(t, nextB.Before(time.Now().Add(5*time.Second)), "next_attempt_at must become promptly eligible")

		// C. claim: lease_token set, lease_until set, second worker cannot claim same live lease
		keyC := fmt.Sprintf("%s/key-c.jpg", qPrefix)
		err = repo.EnqueueMediaCleanup(ctx, keyC)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '10 minutes' WHERE object_key = $1", keyC)
		require.NoError(t, err)

		jobC, err := repo.ClaimMediaCleanupJob(ctx, 5*time.Minute)
		require.NoError(t, err)
		require.NotNil(t, jobC)
		require.Equal(t, keyC, jobC.ObjectKey)
		require.NotNil(t, jobC.LeaseToken)
		require.NotNil(t, jobC.LeaseUntil)
		require.True(t, jobC.LeaseUntil.After(time.Now()), "lease_until must be in the future")

		// Second worker cannot claim the same live lease
		jobC2, err := repo.ClaimMediaCleanupJob(ctx, 5*time.Minute)
		require.NoError(t, err)
		if jobC2 != nil {
			require.NotEqual(t, jobC.ID, jobC2.ID, "second worker must not claim same live leased row")
		}

		// D. re-enqueue while generation 1 is leased: generation becomes 2, original lease may remain until old worker finalizes
		err = repo.EnqueueMediaCleanup(ctx, keyC)
		require.NoError(t, err)

		var genC2 int64
		var leaseTokenC *uuid.UUID
		var leaseUntilC *time.Time
		err = db.Pool.QueryRow(ctx, "SELECT generation, lease_token, lease_until FROM product_media_cleanup_jobs WHERE id = $1", jobC.ID).
			Scan(&genC2, &leaseTokenC, &leaseUntilC)
		require.NoError(t, err)
		require.Equal(t, int64(2), genC2, "re-enqueue during active lease must advance generation to 2")
		require.NotNil(t, leaseTokenC)
		require.Equal(t, *jobC.LeaseToken, *leaseTokenC, "original lease token must remain active")
		require.NotNil(t, leaseUntilC)

		// E. old worker success with claimed generation 1:
		// MUST NOT delete generation 2, returns CleanupFinalizeSkipped, lease released, job promptly eligible again
		resE, err := repo.FinalizeMediaCleanupSuccess(ctx, jobC.ID, 1, *jobC.LeaseToken)
		require.NoError(t, err)
		require.Equal(t, products.CleanupFinalizeSkipped, resE, "old worker success must be skipped due to advanced generation")

		var countC int
		var leaseTokenAfterE *uuid.UUID
		var nextAfterE time.Time
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*), lease_token, next_attempt_at FROM product_media_cleanup_jobs WHERE id = $1 GROUP BY lease_token, next_attempt_at", jobC.ID).
			Scan(&countC, &leaseTokenAfterE, &nextAfterE)
		require.NoError(t, err)
		require.Equal(t, 1, countC, "generation 2 row must survive")
		require.Nil(t, leaseTokenAfterE, "lease must be released")
		require.True(t, nextAfterE.Before(time.Now().Add(5*time.Second)), "job must be promptly eligible again")

		// F. claim generation 2 + success: row deleted
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '10 minutes' WHERE id = $1", jobC.ID)
		require.NoError(t, err)

		jobCGen2, err := repo.ClaimMediaCleanupJob(ctx, 5*time.Minute)
		require.NoError(t, err)
		require.NotNil(t, jobCGen2)
		require.Equal(t, jobC.ID, jobCGen2.ID)
		require.Equal(t, int64(2), jobCGen2.Generation)

		resF, err := repo.FinalizeMediaCleanupSuccess(ctx, jobCGen2.ID, 2, *jobCGen2.LeaseToken)
		require.NoError(t, err)
		require.Equal(t, products.CleanupFinalizeDeleted, resF)

		var countAfterF int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE id = $1", jobC.ID).Scan(&countAfterF)
		require.NoError(t, err)
		require.Equal(t, 0, countAfterF, "row must be deleted after successful finalization of matching generation")

		// G. WRONG lease token on success: must NOT delete row, must return lost-lease/not-owned result
		keyG := fmt.Sprintf("%s/key-g.jpg", qPrefix)
		err = repo.EnqueueMediaCleanup(ctx, keyG)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '10 minutes' WHERE object_key = $1", keyG)
		require.NoError(t, err)

		jobG, err := repo.ClaimMediaCleanupJob(ctx, 5*time.Minute)
		require.NoError(t, err)
		require.NotNil(t, jobG)

		wrongToken := uuid.New()
		resG, err := repo.FinalizeMediaCleanupSuccess(ctx, jobG.ID, jobG.Generation, wrongToken)
		require.NoError(t, err)
		require.Equal(t, products.CleanupFinalizeLost, resG, "must return lost_lease on wrong token")

		var countG int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE id = $1", jobG.ID).Scan(&countG)
		require.NoError(t, err)
		require.Equal(t, 1, countG, "row must not be deleted on wrong lease token")

		// H. WRONG lease token on failure: must NOT change attempts/backoff/lease owned by another worker
		var origAttemptsG int
		var origNextG time.Time
		var origLeaseG *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT attempts, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE id = $1", jobG.ID).
			Scan(&origAttemptsG, &origNextG, &origLeaseG)
		require.NoError(t, err)

		err = repo.FinalizeMediaCleanupFailure(ctx, jobG.ID, jobG.Generation, wrongToken, "fake failure", 1*time.Hour)
		require.NoError(t, err)

		var afterAttemptsG int
		var afterNextG time.Time
		var afterLeaseG *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT attempts, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE id = $1", jobG.ID).
			Scan(&afterAttemptsG, &afterNextG, &afterLeaseG)
		require.NoError(t, err)
		require.Equal(t, origAttemptsG, afterAttemptsG, "attempts must not change on wrong lease token")
		require.Equal(t, origNextG.Unix(), afterNextG.Unix(), "backoff must not change on wrong lease token")
		require.Equal(t, *origLeaseG, *afterLeaseG, "lease token must remain unchanged on wrong lease token")

		// I. failure with SAME generation: attempts += 1, last_error saved, next_attempt_at reflects requested retry, lease cleared
		err = repo.FinalizeMediaCleanupFailure(ctx, jobG.ID, jobG.Generation, *jobG.LeaseToken, "real failure", 30*time.Minute)
		require.NoError(t, err)

		var attemptsI int
		var lastErrI *string
		var nextI time.Time
		var leaseI *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT attempts, last_error, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE id = $1", jobG.ID).
			Scan(&attemptsI, &lastErrI, &nextI, &leaseI)
		require.NoError(t, err)
		require.Equal(t, origAttemptsG+1, attemptsI, "attempts must increment")
		require.NotNil(t, lastErrI)
		require.Equal(t, "real failure", *lastErrI, "last error must be recorded")
		require.True(t, nextI.After(time.Now().Add(25*time.Minute)), "next_attempt_at must reflect backoff")
		require.Nil(t, leaseI, "lease must be cleared on failure")

		// J. failure while generation advanced:
		// newer generation remains, old failure MUST NOT apply old backoff to newer enqueue, lease released, job promptly eligible
		keyJ := fmt.Sprintf("%s/key-j.jpg", qPrefix)
		err = repo.EnqueueMediaCleanup(ctx, keyJ)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '10 minutes' WHERE object_key = $1", keyJ)
		require.NoError(t, err)

		jobJ, err := repo.ClaimMediaCleanupJob(ctx, 5*time.Minute)
		require.NoError(t, err)
		require.NotNil(t, jobJ)
		require.Equal(t, int64(1), jobJ.Generation)

		// advance generation
		err = repo.EnqueueMediaCleanup(ctx, keyJ)
		require.NoError(t, err)

		// old worker reports failure with gen 1 and large backoff (e.g. 24 hours)
		err = repo.FinalizeMediaCleanupFailure(ctx, jobJ.ID, 1, *jobJ.LeaseToken, "old failure to discard", 24*time.Hour)
		require.NoError(t, err)

		var genJ int64
		var attemptsJ int
		var lastErrJ *string
		var nextJ time.Time
		var leaseJ *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT generation, attempts, last_error, next_attempt_at, lease_token FROM product_media_cleanup_jobs WHERE id = $1", jobJ.ID).
			Scan(&genJ, &attemptsJ, &lastErrJ, &nextJ, &leaseJ)
		require.NoError(t, err)
		require.Equal(t, int64(2), genJ, "newer generation remains")
		require.Equal(t, 0, attemptsJ, "attempts must not increment from old generation failure")
		require.Nil(t, lastErrJ, "old failure error must not overwrite newer enqueue")
		require.True(t, nextJ.Before(time.Now().Add(5*time.Second)), "old backoff must not apply; job must be promptly eligible")
		require.Nil(t, leaseJ, "lease must be released")
	})

	t.Run("Real Re-Enqueue Concurrency Test", func(t *testing.T) {
		concurPrefix := fmt.Sprintf("%s/concur-%s", cleanupKeyPrefix, uuid.New())
		t.Cleanup(func() {
			cleanupCtx := context.Background()
			if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE $1", concurPrefix+"%"); err != nil {
				t.Errorf("cleanup failed for concurrency test jobs: %v", err)
			}
		})

		concurKey := fmt.Sprintf("%s/concur-test.jpg", concurPrefix)
		err := repo.EnqueueMediaCleanup(ctx, concurKey)
		require.NoError(t, err)

		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '10 minutes' WHERE object_key = $1", concurKey)
		require.NoError(t, err)

		job, err := repo.ClaimMediaCleanupJob(ctx, 1*time.Minute)
		require.NoError(t, err)
		require.NotNil(t, job)
		require.Equal(t, concurKey, job.ObjectKey)
		require.Equal(t, int64(1), job.Generation)

		// Concurrent re-enqueue while lease is active
		enqueueStarted := make(chan struct{})
		enqueueDone := make(chan struct{})
		var enqueueErr error

		go func() {
			close(enqueueStarted)
			enqueueErr = repo.EnqueueMediaCleanup(ctx, concurKey)
			close(enqueueDone)
		}()

		<-enqueueStarted
		<-enqueueDone
		require.NoError(t, enqueueErr)

		// Old worker finalizes success with claimed generation 1
		res, err := repo.FinalizeMediaCleanupSuccess(ctx, job.ID, 1, *job.LeaseToken)
		require.NoError(t, err)
		require.Equal(t, products.CleanupFinalizeSkipped, res, "old worker success must skip deletion")

		// Verify row survives with generation 2, lease cleared, promptly eligible
		var gen int64
		var leaseToken *uuid.UUID
		var nextAttempt time.Time
		err = db.Pool.QueryRow(ctx, "SELECT generation, lease_token, next_attempt_at FROM product_media_cleanup_jobs WHERE id = $1", job.ID).
			Scan(&gen, &leaseToken, &nextAttempt)
		require.NoError(t, err)
		require.Equal(t, int64(2), gen, "row must survive with generation 2")
		require.Nil(t, leaseToken, "lease must be released")
		require.True(t, nextAttempt.Before(time.Now().Add(5*time.Second)), "job must be promptly eligible")

		// Job becomes claimable again
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_cleanup_jobs SET next_attempt_at = NOW() - interval '10 minutes' WHERE id = $1", job.ID)
		require.NoError(t, err)

		jobClaimedAgain, err := repo.ClaimMediaCleanupJob(ctx, 1*time.Minute)
		require.NoError(t, err)
		require.NotNil(t, jobClaimedAgain)
		require.Equal(t, job.ID, jobClaimedAgain.ID)
		require.Equal(t, int64(2), jobClaimedAgain.Generation)

		// Now finalize gen 2 succeeds and deletes
		res2, err := repo.FinalizeMediaCleanupSuccess(ctx, jobClaimedAgain.ID, 2, *jobClaimedAgain.LeaseToken)
		require.NoError(t, err)
		require.Equal(t, products.CleanupFinalizeDeleted, res2)
	})

	t.Run("TTL Acceptance Matrix", func(t *testing.T) {
		ttlPrefix := fmt.Sprintf("%s/ttl-matrix-%s", cleanupKeyPrefix, uuid.New())
		sha64 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

		t.Cleanup(func() {
			cleanupCtx := context.Background()
			if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_images WHERE object_key LIKE $1", ttlPrefix+"%"); err != nil {
				t.Errorf("cleanup failed for test product_images: %v", err)
			}
			if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_staging WHERE object_key LIKE $1", ttlPrefix+"%"); err != nil {
				t.Errorf("cleanup failed for test product_media_staging: %v", err)
			}
			if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE $1", ttlPrefix+"%"); err != nil {
				t.Errorf("cleanup failed for test product_media_cleanup_jobs: %v", err)
			}
		})

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

		// A. stale uploading (>1h): row removed, cleanup job created
		keyA := fmt.Sprintf("%s/stale-uploading.jpg", ttlPrefix)
		idA := insertStaging(products.ProductMediaStagingUploading, keyA, time.Now().Add(-2*time.Hour), nil)

		// B. fresh uploading (<1h): row untouched, NO cleanup job
		keyB := fmt.Sprintf("%s/fresh-uploading.jpg", ttlPrefix)
		idB := insertStaging(products.ProductMediaStagingUploading, keyB, time.Now().Add(-10*time.Minute), nil)

		// C. stale ready (>24h): row removed, cleanup job created
		keyC := fmt.Sprintf("%s/stale-ready.jpg", ttlPrefix)
		idC := insertStaging(products.ProductMediaStagingReady, keyC, time.Now().Add(-25*time.Hour), nil)

		// D. fresh ready (<24h): row untouched, NO cleanup job
		keyD := fmt.Sprintf("%s/fresh-ready.jpg", ttlPrefix)
		idD := insertStaging(products.ProductMediaStagingReady, keyD, time.Now().Add(-2*time.Hour), nil)

		// E. stale consumed (>24h from consumed_at): staging metadata removed, NO cleanup job
		keyE := fmt.Sprintf("%s/stale-consumed.jpg", ttlPrefix)
		tConsumedE := time.Now().Add(-25 * time.Hour)
		idE := insertStaging(products.ProductMediaStagingConsumed, keyE, time.Now().Add(-30*time.Hour), &tConsumedE)

		// F. fresh consumed: untouched
		keyF := fmt.Sprintf("%s/fresh-consumed.jpg", ttlPrefix)
		tConsumedF := time.Now().Add(-1 * time.Hour)
		idF := insertStaging(products.ProductMediaStagingConsumed, keyF, time.Now().Add(-10*time.Hour), &tConsumedF)

		// G. consumed canonical safety:
		// create canonical product_images row using the same object_key as consumed
		// staging metadata may expire, but NO cleanup job may be created for that key
		// canonical Product image remains intact
		keyG := fmt.Sprintf("%s/consumed-canon.jpg", ttlPrefix)
		tConsumedG := time.Now().Add(-25 * time.Hour)
		idG := insertStaging(products.ProductMediaStagingConsumed, keyG, time.Now().Add(-30*time.Hour), &tConsumedG)
		canonImgID := uuid.New()
		_, err := db.Pool.Exec(ctx, `INSERT INTO product_images (id, product_id, image_url, object_key, sort_order, is_main)
			VALUES ($1, $2, 'http://test-canon.jpg', $3, 0, false)`, canonImgID, productID, keyG)
		require.NoError(t, err)

		// H. TTL encounters object_key already present in cleanup queue: still one queue row, generation increments
		keyH := fmt.Sprintf("%s/already-queued.jpg", ttlPrefix)
		err = repo.EnqueueMediaCleanup(ctx, keyH)
		require.NoError(t, err)
		var genHBefore int64
		err = db.Pool.QueryRow(ctx, "SELECT generation FROM product_media_cleanup_jobs WHERE object_key = $1", keyH).Scan(&genHBefore)
		require.NoError(t, err)
		require.Equal(t, int64(1), genHBefore)
		idH := insertStaging(products.ProductMediaStagingReady, keyH, time.Now().Add(-25*time.Hour), nil)

		// Run TTL expiration batch
		expiredCount, err := repo.ExpireStaleStagedMedia(ctx, 1*time.Hour, 24*time.Hour, 24*time.Hour, 50)
		require.NoError(t, err)
		// Expected expired: A (stale uploading), C (stale ready), E (stale consumed), G (stale consumed), H (stale ready) = 5 rows
		require.Equal(t, 5, expiredCount)

		// Assertions for A: removed from staging, present in cleanup jobs
		var countStagingA int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idA).Scan(&countStagingA)
		require.NoError(t, err)
		require.Equal(t, 0, countStagingA, "stale uploading must be removed from staging")
		var countJobA int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", keyA).Scan(&countJobA)
		require.NoError(t, err)
		require.Equal(t, 1, countJobA, "stale uploading must have cleanup job created")

		// Assertions for B: fresh uploading untouched in staging, NO cleanup job
		var countStagingB int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idB).Scan(&countStagingB)
		require.NoError(t, err)
		require.Equal(t, 1, countStagingB, "fresh uploading must remain in staging")
		var countJobB int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", keyB).Scan(&countJobB)
		require.NoError(t, err)
		require.Equal(t, 0, countJobB, "fresh uploading must not have cleanup job")

		// Assertions for C: stale ready removed from staging, cleanup job created
		var countStagingC int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idC).Scan(&countStagingC)
		require.NoError(t, err)
		require.Equal(t, 0, countStagingC, "stale ready must be removed from staging")
		var countJobC int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", keyC).Scan(&countJobC)
		require.NoError(t, err)
		require.Equal(t, 1, countJobC, "stale ready must have cleanup job created")

		// Assertions for D: fresh ready untouched in staging, NO cleanup job
		var countStagingD int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idD).Scan(&countStagingD)
		require.NoError(t, err)
		require.Equal(t, 1, countStagingD, "fresh ready must remain in staging")
		var countJobD int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", keyD).Scan(&countJobD)
		require.NoError(t, err)
		require.Equal(t, 0, countJobD, "fresh ready must not have cleanup job")

		// Assertions for E: stale consumed removed from staging, NO cleanup job
		var countStagingE int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idE).Scan(&countStagingE)
		require.NoError(t, err)
		require.Equal(t, 0, countStagingE, "stale consumed must be removed from staging")
		var countJobE int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", keyE).Scan(&countJobE)
		require.NoError(t, err)
		require.Equal(t, 0, countJobE, "stale consumed must NOT create cleanup job")

		// Assertions for F: fresh consumed untouched
		var countStagingF int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idF).Scan(&countStagingF)
		require.NoError(t, err)
		require.Equal(t, 1, countStagingF, "fresh consumed must remain in staging")

		// Assertions for G: consumed canonical safety
		var countStagingG int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idG).Scan(&countStagingG)
		require.NoError(t, err)
		require.Equal(t, 0, countStagingG, "consumed staging metadata must expire")
		var countJobG int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", keyG).Scan(&countJobG)
		require.NoError(t, err)
		require.Equal(t, 0, countJobG, "NO cleanup job must be created for consumed key shared with canonical image")
		var countCanonG int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", canonImgID).Scan(&countCanonG)
		require.NoError(t, err)
		require.Equal(t, 1, countCanonG, "canonical product image must remain intact")

		// Assertions for H: TTL encounters existing cleanup job: still one queue row, generation increments
		var countJobH int
		var genHAfter int64
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*), generation FROM product_media_cleanup_jobs WHERE object_key = $1 GROUP BY generation", keyH).
			Scan(&countJobH, &genHAfter)
		require.NoError(t, err)
		require.Equal(t, 1, countJobH, "still exactly one cleanup job row")
		require.Equal(t, int64(2), genHAfter, "generation must increment to 2")
		var countStagingH int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", idH).Scan(&countStagingH)
		require.NoError(t, err)
		require.Equal(t, 0, countStagingH, "staging row must be deleted")

		// I. concurrent TTL processors:
		// same stale staging row is processed once, not double-deleted, queue intent is not duplicated/lost.
		// Uses FOR UPDATE SKIP LOCKED path.
		numConcurrentRows := 6
		concurTTLKeys := make([]string, numConcurrentRows)
		concurTTLRowIDs := make([]uuid.UUID, numConcurrentRows)
		for i := 0; i < numConcurrentRows; i++ {
			concurTTLKeys[i] = fmt.Sprintf("%s/concur-ttl-%d.jpg", ttlPrefix, i)
			concurTTLRowIDs[i] = insertStaging(products.ProductMediaStagingUploading, concurTTLKeys[i], time.Now().Add(-2*time.Hour), nil)
		}

		numWorkers := 3
		results := make(chan int, numWorkers)
		errs := make(chan error, numWorkers)

		for w := 0; w < numWorkers; w++ {
			go func() {
				c, e := repo.ExpireStaleStagedMedia(ctx, 1*time.Hour, 24*time.Hour, 24*time.Hour, 10)
				if e != nil {
					errs <- e
					return
				}
				results <- c
			}()
		}

		totalExpired := 0
		for w := 0; w < numWorkers; w++ {
			select {
			case e := <-errs:
				require.NoError(t, e)
			case c := <-results:
				totalExpired += c
			}
		}

		require.Equal(t, numConcurrentRows, totalExpired, "concurrent TTL workers must process all rows exactly once without double-deletion")

		// Verify all staging rows gone
		for _, rid := range concurTTLRowIDs {
			var cnt int
			err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", rid).Scan(&cnt)
			require.NoError(t, err)
			require.Equal(t, 0, cnt, "staged row must be deleted")
		}

		// Verify cleanup jobs created once per distinct object key
		for _, k := range concurTTLKeys {
			var cnt int
			var g int64
			err = db.Pool.QueryRow(ctx, "SELECT COUNT(*), generation FROM product_media_cleanup_jobs WHERE object_key = $1 GROUP BY generation", k).
				Scan(&cnt, &g)
			require.NoError(t, err)
			require.Equal(t, 1, cnt, "cleanup job must exist exactly once")
			require.Equal(t, int64(1), g, "generation must be 1")
		}
	})
}
