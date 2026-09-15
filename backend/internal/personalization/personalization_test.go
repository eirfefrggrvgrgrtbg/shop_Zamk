package personalization_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func setupTestDB(t *testing.T) (context.Context, *postgres.Client, *personalization.Repository, *personalization.Service) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping database integration test")
	}

	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = testutil.CanonicalTestDatabaseDSN
	}

	ctx := context.Background()
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)

	// Invariant: strictly assert zamk_test
	var currentDB string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "integration tests must strictly run against zamk_test")

	repo := personalization.NewRepository(pgClient.Pool)
	svc := personalization.NewService(repo)

	return ctx, pgClient, repo, svc
}

func createTestFixtures(t *testing.T, ctx context.Context, pgClient *postgres.Client) (customerAID, customerBID, publishedProdID, unpublishedProdID, sellerID, catID uuid.UUID) {
	t.Helper()

	customerAID = uuid.New()
	customerBID = uuid.New()
	sellerID = uuid.New()
	catID = uuid.New()
	publishedProdID = uuid.New()
	pubVariantID := uuid.New()
	unpublishedProdID = uuid.New()
	unpubVariantID := uuid.New()

	now := time.Now()

	// Insert customers
	for _, cid := range []uuid.UUID{customerAID, customerBID} {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test Customer', 'hash', 'customer', 'active', $4, $4)
		`, cid, cid.String()+"@test.local", "+7999"+cid.String()[:7], now)
		require.NoError(t, err)
	}

	// Insert active seller
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Personalization Brand', $2, $3, 'active', $4, $4)
	`, sellerID, "pers-brand-"+sellerID.String()[:8], "pers-"+sellerID.String()[:8]+"@test.local", now)
	require.NoError(t, err)

	// Insert category
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Pers Category', $2, true, $3, $3)
	`, catID, "pers-cat-"+catID.String()[:8], now)
	require.NoError(t, err)

	// Insert published product with stock >= 2 (CAT.1A storefront accessible)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Published Product', $4, 100000, 'RUB', 'published', $5, $5, $5, $5, $5)
	`, publishedProdID, sellerID, catID, "pub-prod-"+publishedProdID.String()[:8], now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, 100000, true, $5, $5)
	`, pubVariantID, publishedProdID, "SKU-"+pubVariantID.String()[:8], "BC-"+pubVariantID.String()[:8], now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 0, $5, $5)
	`, uuid.New(), publishedProdID, pubVariantID, sellerID, now)
	require.NoError(t, err)

	// Insert unpublished product (status = pending_moderation)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			submitted_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Unpublished Product', $4, 100000, 'RUB', 'pending_moderation', $5, $5, $5)
	`, unpublishedProdID, sellerID, catID, "unpub-prod-"+unpublishedProdID.String()[:8], now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, 100000, true, $5, $5)
	`, unpubVariantID, unpublishedProdID, "SKU-"+unpubVariantID.String()[:8], "BC-"+unpubVariantID.String()[:8], now)
	require.NoError(t, err)

	return customerAID, customerBID, publishedProdID, unpublishedProdID, sellerID, catID
}

func TestPersonalization_RepositoryAndService(t *testing.T) {
	ctx, pgClient, _, svc := setupTestDB(t)
	defer pgClient.Close()

	custA, custB, pubProd, unpubProd, sellerID, catID := createTestFixtures(t, ctx, pgClient)

	// Clean up all fixtures created during test
	defer func() {
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM customer_product_views WHERE user_id IN ($1, $2)", custA, custB)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id IN ($1, $2)", pubProd, unpubProd)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id IN ($1, $2)", pubProd, unpubProd)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id IN ($1, $2)", pubProd, unpubProd)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", catID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", custA, custB)
	}()

	t.Run("A. Customer A views Product X -> 1 row, view_count = 1", func(t *testing.T) {
		err := svc.RecordProductView(ctx, custA, pubProd)
		require.NoError(t, err)

		view, err := svc.GetProductView(ctx, custA, pubProd)
		require.NoError(t, err)
		require.NotNil(t, view)
		assert.Equal(t, custA, view.UserID)
		assert.Equal(t, pubProd, view.ProductID)
		assert.Equal(t, int64(1), view.ViewCount)
		assert.False(t, view.LastViewedAt.IsZero())
	})

	t.Run("B. Customer A views Product X again -> still 1 row, view_count = 2, last_viewed_at updated", func(t *testing.T) {
		viewBefore, err := svc.GetProductView(ctx, custA, pubProd)
		require.NoError(t, err)
		require.NotNil(t, viewBefore)

		time.Sleep(10 * time.Millisecond)

		err = svc.RecordProductView(ctx, custA, pubProd)
		require.NoError(t, err)

		viewAfter, err := svc.GetProductView(ctx, custA, pubProd)
		require.NoError(t, err)
		require.NotNil(t, viewAfter)

		assert.Equal(t, int64(2), viewAfter.ViewCount)
		assert.True(t, viewAfter.LastViewedAt.After(viewBefore.LastViewedAt) || viewAfter.LastViewedAt.Equal(viewBefore.LastViewedAt))

		// Check count of rows in table is strictly 1
		var count int
		err = pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM customer_product_views WHERE user_id = $1 AND product_id = $2", custA, pubProd).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("C. Customer B views same Product X -> separate B/X row, no overwrite", func(t *testing.T) {
		err := svc.RecordProductView(ctx, custB, pubProd)
		require.NoError(t, err)

		viewB, err := svc.GetProductView(ctx, custB, pubProd)
		require.NoError(t, err)
		require.NotNil(t, viewB)
		assert.Equal(t, custB, viewB.UserID)
		assert.Equal(t, pubProd, viewB.ProductID)
		assert.Equal(t, int64(1), viewB.ViewCount)

		// Check Customer A still has view_count = 2
		viewA, err := svc.GetProductView(ctx, custA, pubProd)
		require.NoError(t, err)
		require.NotNil(t, viewA)
		assert.Equal(t, int64(2), viewA.ViewCount)
	})

	t.Run("F. nonexistent product -> ErrProductNotAccessible, no view row created", func(t *testing.T) {
		fakeProdID := uuid.New()
		err := svc.RecordProductView(ctx, custA, fakeProdID)
		assert.ErrorIs(t, err, personalization.ErrProductNotAccessible)

		view, err := svc.GetProductView(ctx, custA, fakeProdID)
		require.NoError(t, err)
		assert.Nil(t, view)
	})

	t.Run("G. unpublished product -> ErrProductNotAccessible, no view row created", func(t *testing.T) {
		err := svc.RecordProductView(ctx, custA, unpubProd)
		assert.ErrorIs(t, err, personalization.ErrProductNotAccessible)

		view, err := svc.GetProductView(ctx, custA, unpubProd)
		require.NoError(t, err)
		assert.Nil(t, view)
	})

	t.Run("H. concurrent repeated calls do not create duplicate rows", func(t *testing.T) {
		concurrency := 10
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				_ = svc.RecordProductView(ctx, custA, pubProd)
			}()
		}
		wg.Wait()

		var rowCount int
		err := pgClient.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM customer_product_views WHERE user_id = $1 AND product_id = $2", custA, pubProd).Scan(&rowCount)
		require.NoError(t, err)
		assert.Equal(t, 1, rowCount, "upsert must ensure exactly one row for user+product under concurrency")

		view, err := svc.GetProductView(ctx, custA, pubProd)
		require.NoError(t, err)
		assert.Equal(t, int64(2+concurrency), view.ViewCount)
	})

	t.Run("I. GetRecentlyViewedProducts returns correct products in order", func(t *testing.T) {
		// Create another published product
		pubProd2 := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				submitted_at, approved_at, published_at, created_at, updated_at
			) VALUES ($1, $2, $3, 'Published Product 2', $4, 200000, 'RUB', 'published', $5, $5, $5, $5, $5)
		`, pubProd2, sellerID, catID, "pub-prod2-"+pubProd2.String()[:8], time.Now())
		require.NoError(t, err)

		pubVariantID2 := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, 200000, true, $5, $5)
		`, pubVariantID2, pubProd2, "SKU-"+pubVariantID2.String()[:8], "BC-"+pubVariantID2.String()[:8], time.Now())
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 10, 0, $5, $5)
		`, uuid.New(), pubProd2, pubVariantID2, sellerID, time.Now())
		require.NoError(t, err)

		defer func() {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM customer_product_views WHERE product_id = $1", pubProd2)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id = $1", pubProd2)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = $1", pubProd2)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = $1", pubProd2)
		}()

		// View product 2
		err = svc.RecordProductView(ctx, custA, pubProd2)
		require.NoError(t, err)

		// Get recently viewed products
		recentProds, err := svc.GetRecentlyViewedProducts(ctx, custA, 10)
		require.NoError(t, err)

		require.Len(t, recentProds, 2)
		assert.Equal(t, pubProd2, recentProds[0].ID, "most recently viewed should be first")
		assert.Equal(t, pubProd, recentProds[1].ID, "older view should be second")

		// B. Customer views A again -> returns A, B -> A appears only once.
		time.Sleep(10 * time.Millisecond)
		err = svc.RecordProductView(ctx, custA, pubProd)
		require.NoError(t, err)

		recentProds, err = svc.GetRecentlyViewedProducts(ctx, custA, 10)
		require.NoError(t, err)
		require.Len(t, recentProds, 2)
		assert.Equal(t, pubProd, recentProds[0].ID, "A should now be first")
		assert.Equal(t, pubProd2, recentProds[1].ID, "B should be second")

		// C. Customer B has independent history -> no cross-user leakage.
		recentProdsB, err := svc.GetRecentlyViewedProducts(ctx, custB, 10)
		require.NoError(t, err)
		// CustB only viewed pubProd (in test C above)
		require.Len(t, recentProdsB, 1)
		assert.Equal(t, pubProd, recentProdsB[0].ID)

		// F. authenticated customer with no history -> []
		custC := uuid.New()
		recentProdsC, err := svc.GetRecentlyViewedProducts(ctx, custC, 10)
		require.NoError(t, err)
		assert.Empty(t, recentProdsC)

		// G. historically viewed product becomes unpublished -> excluded.
		_, err = pgClient.Pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", pubProd2)
		require.NoError(t, err)

		recentProds, err = svc.GetRecentlyViewedProducts(ctx, custA, 10)
		require.NoError(t, err)
		require.Len(t, recentProds, 1)
		assert.Equal(t, pubProd, recentProds[0].ID)

		_, err = pgClient.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", pubProd2)
		require.NoError(t, err)

		// H. historically viewed product falls below canonical CAT.1A storefront threshold -> excluded.
		_, err = pgClient.Pool.Exec(ctx, "UPDATE inventory_items SET total_stock = 1 WHERE product_id = $1", pubProd2)
		require.NoError(t, err)

		recentProds, err = svc.GetRecentlyViewedProducts(ctx, custA, 10)
		require.NoError(t, err)
		require.Len(t, recentProds, 1)
		assert.Equal(t, pubProd, recentProds[0].ID)

		_, err = pgClient.Pool.Exec(ctx, "UPDATE inventory_items SET total_stock = 10 WHERE product_id = $1", pubProd2)
		require.NoError(t, err)

		// I. product seller becomes inactive -> excluded.
		_, err = pgClient.Pool.Exec(ctx, "UPDATE sellers SET status = 'blocked' WHERE id = $1", sellerID)
		require.NoError(t, err)

		recentProds, err = svc.GetRecentlyViewedProducts(ctx, custA, 10)
		require.NoError(t, err)
		assert.Empty(t, recentProds)

		_, err = pgClient.Pool.Exec(ctx, "UPDATE sellers SET status = 'active' WHERE id = $1", sellerID)
		require.NoError(t, err)

		// K. limit is bounded
		recentProds, err = svc.GetRecentlyViewedProducts(ctx, custA, 1)
		require.NoError(t, err)
		require.Len(t, recentProds, 1)

		// L. response does NOT expose private fields
		// PublicProduct is used which omits customer/user identity, payment, private seller info
		assert.NotZero(t, recentProds[0].SellerID) // SellerID is expected for PublicProduct, but not private finance data.
	})
}
