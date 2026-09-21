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
}
