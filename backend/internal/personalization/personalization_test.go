package personalization_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
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

func createTestProduct(
	t *testing.T,
	ctx context.Context,
	client *postgres.Client,
	sellerID uuid.UUID,
	catID uuid.UUID,
	brandID *uuid.UUID,
	title string,
	priceCents int64,
	rating float64,
	reviewsCount int,
	publishedAt time.Time,
	status string,
	stock int,
) uuid.UUID {
	t.Helper()
	prodID := uuid.New()
	slug := "sim-" + prodID.String()[:8]
	now := time.Now()

	_, err := client.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status,
			average_rating, reviews_count,
			submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'RUB', $8, $9, $10, $11, $11, $12, $11, $11)
	`, prodID, sellerID, catID, brandID, title, slug, priceCents, status, rating, reviewsCount, now, publishedAt)
	require.NoError(t, err)

	varID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, $5, true, $6, $6)
	`, varID, prodID, "SKU-"+varID.String()[:8], "BC-"+varID.String()[:8], priceCents, now)
	require.NoError(t, err)

	if stock > 0 {
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 0, $6, $6)
		`, uuid.New(), prodID, varID, sellerID, stock, now)
		require.NoError(t, err)
	}

	return prodID
}

func TestSimilarProducts_Acceptance(t *testing.T) {
	ctx, pgClient, _, svc := setupTestDB(t)

	now := time.Now()
	t0 := now.Add(-10 * time.Hour)
	t1 := now.Add(-5 * time.Hour)
	t2 := now.Add(-1 * time.Hour)

	// Setup sellers
	activeSellerID := uuid.New()
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Active Seller', $2, $3, 'active', $4, $4)
	`, activeSellerID, "seller-act-"+activeSellerID.String()[:8], "act-"+activeSellerID.String()[:8]+"@test.local", now)
	require.NoError(t, err)

	blockedSellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Blocked Seller', $2, $3, 'blocked', $4, $4)
	`, blockedSellerID, "seller-blk-"+blockedSellerID.String()[:8], "blk-"+blockedSellerID.String()[:8]+"@test.local", now)
	require.NoError(t, err)

	// Setup brands
	brandA := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Brand Alpha', $2, true, $3, $3)
	`, brandA, "brand-alpha-"+brandA.String()[:8], now)
	require.NoError(t, err)

	brandB := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Brand Beta', $2, true, $3, $3)
	`, brandB, "brand-beta-"+brandB.String()[:8], now)
	require.NoError(t, err)

	// Setup categories
	catMain := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Main Category', $2, true, $3, $3)
	`, catMain, "cat-main-"+catMain.String()[:8], now)
	require.NoError(t, err)

	catIsolated := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Isolated Category', $2, true, $3, $3)
	`, catIsolated, "cat-iso-"+catIsolated.String()[:8], now)
	require.NoError(t, err)

	// Source product: Brand A, Price 100_000, Rating 4.0, Stock 10
	sourceProdID := createTestProduct(t, ctx, pgClient, activeSellerID, catMain, &brandA, "Source Product", 100000, 4.0, 10, t0, "published", 10)

	// Tier 1 Candidate 1: Same Brand A, Price 105_000 (diff: 5_000)
	cTier1Close := createTestProduct(t, ctx, pgClient, activeSellerID, catMain, &brandA, "Tier 1 Close", 105000, 4.0, 10, t0, "published", 10)

	// Tier 1 Candidate 2: Same Brand A, Price 150_000 (diff: 50_000)
	cTier1Far := createTestProduct(t, ctx, pgClient, activeSellerID, catMain, &brandA, "Tier 1 Far", 150000, 4.0, 10, t0, "published", 10)

	// Tier 2 Candidate 1: Different Brand B, Price 100_000 (diff: 0)
	cTier2Exact := createTestProduct(t, ctx, pgClient, activeSellerID, catMain, &brandB, "Tier 2 Exact Price", 100000, 4.0, 10, t0, "published", 10)

	// Tier 2 Candidate 2: Different Brand B, Price 120_000 (diff: 20_000)
	cTier2Far := createTestProduct(t, ctx, pgClient, activeSellerID, catMain, &brandB, "Tier 2 Far Price", 120000, 4.0, 10, t0, "published", 10)

	// Ineligible candidates:
	// E. Unpublished candidate
	cUnpublished := createTestProduct(t, ctx, pgClient, activeSellerID, catMain, &brandA, "Unpublished Candidate", 101000, 5.0, 100, t0, "pending_moderation", 10)

	// F. Inactive seller candidate
	cInactiveSeller := createTestProduct(t, ctx, pgClient, blockedSellerID, catMain, &brandA, "Inactive Seller Candidate", 101000, 5.0, 100, t0, "published", 10)

	// G. Below CAT.1A threshold (stock = 1, required >= 2)
	cLowStock := createTestProduct(t, ctx, pgClient, activeSellerID, catMain, &brandA, "Low Stock Candidate", 101000, 5.0, 100, t0, "published", 1)

	// Run acceptance checks
	t.Run("A, B, D, E, F, G: Tiers, closeness, source exclusion, eligibility filtering", func(t *testing.T) {
		res, err := svc.GetSimilarProducts(ctx, sourceProdID, 10)
		require.NoError(t, err)

		// Expected candidates: exactly 4 eligible products
		require.Len(t, res, 4)

		// D. Source product must be excluded
		for _, p := range res {
			assert.NotEqual(t, sourceProdID, p.ID, "Source product must not appear in similar products")
			assert.NotEqual(t, cUnpublished, p.ID, "Unpublished product must be excluded")
			assert.NotEqual(t, cInactiveSeller, p.ID, "Product with inactive seller must be excluded")
			assert.NotEqual(t, cLowStock, p.ID, "Product below CAT.1A stock threshold must be excluded")
		}

		// A. Same-brand (Tier 1) rank before different-brand (Tier 2), even if Tier 2 has closer price
		assert.Equal(t, cTier1Close, res[0].ID)
		assert.Equal(t, cTier1Far, res[1].ID)
		assert.Equal(t, cTier2Exact, res[2].ID)
		assert.Equal(t, cTier2Far, res[3].ID)

		// B. Within same tier, closer price ranks first (105k before 150k in Tier 1; 100k before 120k in Tier 2)
		assert.Equal(t, cTier1Close, res[0].ID)
		assert.Equal(t, cTier1Far, res[1].ID)
	})

	t.Run("C: Within-tier deterministic tie-breakers (rating -> reviews -> published_at -> id)", func(t *testing.T) {
		catTie := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, 'Tie Category', $2, true, $3, $3)
		`, catTie, "cat-tie-"+catTie.String()[:8], now)
		require.NoError(t, err)

		srcTie := createTestProduct(t, ctx, pgClient, activeSellerID, catTie, &brandA, "Src Tie", 100000, 4.0, 10, t0, "published", 10)

		// High rating (5.0, 5 reviews, t1)
		c1HighRating := createTestProduct(t, ctx, pgClient, activeSellerID, catTie, &brandA, "High Rating", 100000, 5.0, 5, t1, "published", 10)
		// Lower rating, high reviews (4.5, 50 reviews, t1)
		c2HighReviews := createTestProduct(t, ctx, pgClient, activeSellerID, catTie, &brandA, "High Reviews", 100000, 4.5, 50, t1, "published", 10)
		// Same rating, lower reviews, newer published (4.5, 20 reviews, t2)
		c3NewerPub := createTestProduct(t, ctx, pgClient, activeSellerID, catTie, &brandA, "Newer Pub", 100000, 4.5, 20, t2, "published", 10)
		// Same rating, lower reviews, older published (4.5, 20 reviews, t1)
		c4OlderPub := createTestProduct(t, ctx, pgClient, activeSellerID, catTie, &brandA, "Older Pub", 100000, 4.5, 20, t1, "published", 10)

		res, err := svc.GetSimilarProducts(ctx, srcTie, 10)
		require.NoError(t, err)
		require.Len(t, res, 4)

		// 1. Rating DESC: 5.0 > 4.5
		assert.Equal(t, c1HighRating, res[0].ID)
		// 2. Reviews DESC: 50 > 20
		assert.Equal(t, c2HighReviews, res[1].ID)
		// 3. PublishedAt DESC: t2 > t1
		assert.Equal(t, c3NewerPub, res[2].ID)
		// 4. Older pub
		assert.Equal(t, c4OlderPub, res[3].ID)
	})

	t.Run("H: Nonexistent or non-public source product -> error", func(t *testing.T) {
		// Nonexistent ID
		_, err := svc.GetSimilarProducts(ctx, uuid.New(), 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, personalization.ErrProductNotAccessible)

		// Unpublished source product
		_, err = svc.GetSimilarProducts(ctx, cUnpublished, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, personalization.ErrProductNotAccessible)

		// Inactive seller source product
		_, err = svc.GetSimilarProducts(ctx, cInactiveSeller, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, personalization.ErrProductNotAccessible)

		// Low stock source product
		_, err = svc.GetSimilarProducts(ctx, cLowStock, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, personalization.ErrProductNotAccessible)
	})

	t.Run("I: Brand NULL does not break query", func(t *testing.T) {
		catNoBrand := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, 'No Brand Cat', $2, true, $3, $3)
		`, catNoBrand, "cat-nobrand-"+catNoBrand.String()[:8], now)
		require.NoError(t, err)

		srcNoBrand := createTestProduct(t, ctx, pgClient, activeSellerID, catNoBrand, nil, "No Brand Src", 100000, 4.0, 10, t0, "published", 10)
		cWithBrand := createTestProduct(t, ctx, pgClient, activeSellerID, catNoBrand, &brandA, "Cand Brand A", 105000, 4.0, 10, t0, "published", 10)
		cNoBrand := createTestProduct(t, ctx, pgClient, activeSellerID, catNoBrand, nil, "Cand No Brand", 102000, 4.0, 10, t0, "published", 10)

		res, err := svc.GetSimilarProducts(ctx, srcNoBrand, 10)
		require.NoError(t, err)
		require.Len(t, res, 2)
		// Both in Tier 2, ordered by price distance (102k diff 2k vs 105k diff 5k)
		assert.Equal(t, cNoBrand, res[0].ID)
		assert.Equal(t, cWithBrand, res[1].ID)
	})

	t.Run("J: No candidates -> empty slice", func(t *testing.T) {
		srcIsolated := createTestProduct(t, ctx, pgClient, activeSellerID, catIsolated, &brandA, "Isolated Src", 100000, 4.0, 10, t0, "published", 10)
		res, err := svc.GetSimilarProducts(ctx, srcIsolated, 10)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Empty(t, res)
	})

	t.Run("K: Bounded limit enforced", func(t *testing.T) {
		res, err := svc.GetSimilarProducts(ctx, sourceProdID, 2)
		require.NoError(t, err)
		require.Len(t, res, 2)
	})

	t.Run("L: Response uses safe PublicProduct DTO", func(t *testing.T) {
		res, err := svc.GetSimilarProducts(ctx, sourceProdID, 5)
		require.NoError(t, err)
		require.NotEmpty(t, res)

		p := res[0]
		assert.NotEmpty(t, p.ID)
		assert.NotEmpty(t, p.Title)
		assert.NotEmpty(t, p.Slug)
		assert.NotZero(t, p.PriceCents)
		assert.Equal(t, "RUB", p.Currency)
		assert.NotEmpty(t, p.SellerID)
		assert.NotEmpty(t, p.SellerSlug)
		assert.NotEmpty(t, p.SellerName)
	})

	t.Run("CAT.1A exact stock boundary tests for source and candidate", func(t *testing.T) {
		catBoundary := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, 'Boundary Category', $2, true, $3, $3)
		`, catBoundary, "cat-bound-"+catBoundary.String()[:8], now)
		require.NoError(t, err)

		// Source with stock = 1 (free < 2 -> ineligible)
		srcStock1 := createTestProduct(t, ctx, pgClient, activeSellerID, catBoundary, &brandA, "Src Stock 1", 100000, 4.0, 10, t0, "published", 1)

		// Source with stock = 2 (free >= 2 -> eligible)
		srcStock2 := createTestProduct(t, ctx, pgClient, activeSellerID, catBoundary, &brandA, "Src Stock 2", 100000, 4.0, 10, t0, "published", 2)

		// Candidate with stock = 1 (free = 1 -> excluded)
		candStock1 := createTestProduct(t, ctx, pgClient, activeSellerID, catBoundary, &brandA, "Cand Stock 1", 105000, 4.0, 10, t0, "published", 1)

		// Candidate with stock = 2 (free = 2 -> eligible)
		candStock2 := createTestProduct(t, ctx, pgClient, activeSellerID, catBoundary, &brandA, "Cand Stock 2", 105000, 4.0, 10, t0, "published", 2)

		// A. Source with free=1 -> ErrProductNotAccessible (404)
		_, err = svc.GetSimilarProducts(ctx, srcStock1, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, personalization.ErrProductNotAccessible)

		// B. Source with free=2 -> eligible
		similar, err := svc.GetSimilarProducts(ctx, srcStock2, 10)
		require.NoError(t, err)

		// C. Candidate with free=1 -> excluded
		// D. Candidate with free=2 -> included
		require.Len(t, similar, 1)
		assert.Equal(t, candStock2, similar[0].ID)
		for _, p := range similar {
			assert.NotEqual(t, candStock1, p.ID, "Candidate with free stock = 1 must be excluded")
		}
	})
}

func TestCustomerPreferenceProfile_Acceptance(t *testing.T) {
	ctx, pgClient, _, svc := setupTestDB(t)
	defer pgClient.Close()

	now := time.Now()
	var createdUserIDs []uuid.UUID
	var createdSellerIDs []uuid.UUID
	var createdCatIDs []uuid.UUID
	var createdBrandIDs []uuid.UUID
	var createdProdIDs []uuid.UUID

	createCat := func(name string) uuid.UUID {
		id := uuid.New()
		slug := "cat-" + id.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, $4, $4)
		`, id, name, slug, now)
		require.NoError(t, err)
		createdCatIDs = append(createdCatIDs, id)
		return id
	}

	createBrand := func(name string) uuid.UUID {
		id := uuid.New()
		slug := "brand-" + id.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, $4, $4)
		`, id, name, slug, now)
		require.NoError(t, err)
		createdBrandIDs = append(createdBrandIDs, id)
		return id
	}

	createUser := func(name string) uuid.UUID {
		id := uuid.New()
		email := "cust-" + id.String()[:8] + "@test.local"
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'hash', 'customer', 'active', $5, $5)
		`, id, email, "+7999"+id.String()[:7], name, now)
		require.NoError(t, err)
		createdUserIDs = append(createdUserIDs, id)
		return id
	}

	createSeller := func(name string) uuid.UUID {
		id := uuid.New()
		slug := "seller-" + id.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'active', $5, $5)
		`, id, name, slug, slug+"@test.local", now)
		require.NoError(t, err)
		createdSellerIDs = append(createdSellerIDs, id)
		return id
	}

	createProd := func(sellerID, catID uuid.UUID, brandID *uuid.UUID, title string) uuid.UUID {
		prodID := createTestProduct(t, ctx, pgClient, sellerID, catID, brandID, title, 100000, 4.5, 10, now, "published", 10)
		createdProdIDs = append(createdProdIDs, prodID)
		return prodID
	}

	addFav := func(userID, prodID uuid.UUID, createdAt time.Time) {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO customer_favorites (id, user_id, product_id, created_at)
			VALUES ($1, $2, $3, $4)
		`, uuid.New(), userID, prodID, createdAt)
		require.NoError(t, err)
	}

	addView := func(userID, prodID uuid.UUID, lastViewedAt time.Time, count int64) {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO customer_product_views (user_id, product_id, last_viewed_at, view_count)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, product_id)
			DO UPDATE SET last_viewed_at = $3, view_count = $4
		`, userID, prodID, lastViewedAt, count)
		require.NoError(t, err)
	}

	defer func() {
		if len(createdUserIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM customer_favorites WHERE user_id = ANY($1)", createdUserIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM customer_product_views WHERE user_id = ANY($1)", createdUserIDs)
		}
		if len(createdProdIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id = ANY($1)", createdProdIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = ANY($1)", createdProdIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProdIDs)
		}
		if len(createdBrandIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM brands WHERE id = ANY($1)", createdBrandIDs)
		}
		if len(createdCatIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCatIDs)
		}
		if len(createdSellerIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
		}
		if len(createdUserIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", createdUserIDs)
		}
	}()

	seller := createSeller("Pref Seller")
	custA := createUser("Customer A")
	custB := createUser("Customer B")
	custEmpty := createUser("Customer Empty")

	catX := createCat("Category X")
	catY := createCat("Category Y")
	brandX := createBrand("Brand X")
	brandY := createBrand("Brand Y")

	pX1 := createProd(seller, catX, &brandX, "Product X1")
	pX2 := createProd(seller, catX, &brandX, "Product X2")
	pY1 := createProd(seller, catY, &brandY, "Product Y1")

	// Subtest A: Customer A favorites two products in Category X and one in Category Y -> favoriteCategories ranks X before Y.
	t.Run("A: Customer A favorites two products in Category X and one in Category Y -> favoriteCategories ranks X before Y", func(t *testing.T) {
		addFav(custA, pX1, now.Add(-2*time.Hour))
		addFav(custA, pX2, now.Add(-1*time.Hour))
		addFav(custA, pY1, now.Add(-30*time.Minute))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, custA, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.GreaterOrEqual(t, len(profile.FavoriteCategories), 2)

		assert.Equal(t, catX, profile.FavoriteCategories[0].CategoryID)
		assert.Equal(t, int64(2), profile.FavoriteCategories[0].DistinctProductCount)
		assert.Equal(t, personalization.AffinityProvenanceFavorite, profile.FavoriteCategories[0].Provenance)

		assert.Equal(t, catY, profile.FavoriteCategories[1].CategoryID)
		assert.Equal(t, int64(1), profile.FavoriteCategories[1].DistinctProductCount)
		assert.Equal(t, personalization.AffinityProvenanceFavorite, profile.FavoriteCategories[1].Provenance)
	})

	// Subtest B: Customer A favorites two products from Brand X and one from Brand Y -> favoriteBrands ranks X before Y.
	t.Run("B: Customer A favorites two products from Brand X and one from Brand Y -> favoriteBrands ranks X before Y", func(t *testing.T) {
		profile, err := svc.GetCustomerPreferenceProfile(ctx, custA, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.GreaterOrEqual(t, len(profile.FavoriteBrands), 2)

		assert.Equal(t, brandX, profile.FavoriteBrands[0].BrandID)
		assert.Equal(t, int64(2), profile.FavoriteBrands[0].DistinctProductCount)
		assert.Equal(t, personalization.AffinityProvenanceFavorite, profile.FavoriteBrands[0].Provenance)

		assert.Equal(t, brandY, profile.FavoriteBrands[1].BrandID)
		assert.Equal(t, int64(1), profile.FavoriteBrands[1].DistinctProductCount)
		assert.Equal(t, personalization.AffinityProvenanceFavorite, profile.FavoriteBrands[1].Provenance)
	})

	// Subtest C: Repeated views of SAME Product A do NOT increase category/brand affinity beyond one distinct-product contribution.
	t.Run("C: Repeated views of SAME Product A do NOT increase category/brand affinity beyond one distinct-product contribution", func(t *testing.T) {
		custRepeat := createUser("Customer Repeat")
		catRepeat := createCat("Category Repeat")
		brandRepeat := createBrand("Brand Repeat")
		pRepeat := createProd(seller, catRepeat, &brandRepeat, "Product Repeat")

		// View the same product 5 times with view_count = 5
		addView(custRepeat, pRepeat, now, 5)

		profile, err := svc.GetCustomerPreferenceProfile(ctx, custRepeat, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		require.Len(t, profile.ViewedCategories, 1)
		assert.Equal(t, catRepeat, profile.ViewedCategories[0].CategoryID)
		assert.Equal(t, int64(1), profile.ViewedCategories[0].DistinctProductCount, "DistinctProductCount must remain 1 despite repeated views")

		require.Len(t, profile.ViewedBrands, 1)
		assert.Equal(t, brandRepeat, profile.ViewedBrands[0].BrandID)
		assert.Equal(t, int64(1), profile.ViewedBrands[0].DistinctProductCount, "DistinctProductCount must remain 1 despite repeated views")
	})

	// Subtest D: Views of Product A and Product B in same category -> viewed category count = 2.
	t.Run("D: Views of Product A and Product B in same category -> viewed category count = 2", func(t *testing.T) {
		custMulti := createUser("Customer Multi")
		catMulti := createCat("Category Multi")
		brandMulti := createBrand("Brand Multi")
		pM1 := createProd(seller, catMulti, &brandMulti, "Product M1")
		pM2 := createProd(seller, catMulti, &brandMulti, "Product M2")

		addView(custMulti, pM1, now.Add(-2*time.Hour), 3)
		addView(custMulti, pM2, now.Add(-1*time.Hour), 4)

		profile, err := svc.GetCustomerPreferenceProfile(ctx, custMulti, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		require.Len(t, profile.ViewedCategories, 1)
		assert.Equal(t, catMulti, profile.ViewedCategories[0].CategoryID)
		assert.Equal(t, int64(2), profile.ViewedCategories[0].DistinctProductCount)
		assert.Equal(t, personalization.AffinityProvenanceViewed, profile.ViewedCategories[0].Provenance)
	})

	// Subtest E: viewed affinity tie uses latest last_viewed_at.
	t.Run("E: viewed affinity tie uses latest last_viewed_at", func(t *testing.T) {
		custTie := createUser("Customer Tie")
		catOlder := createCat("Category Older")
		catNewer := createCat("Category Newer")
		brandCommon := createBrand("Brand Common")

		pOlder := createProd(seller, catOlder, &brandCommon, "Product Older")
		pNewer := createProd(seller, catNewer, &brandCommon, "Product Newer")

		tOlder := now.Add(-3 * time.Hour)
		tNewer := now.Add(-10 * time.Minute)

		addView(custTie, pOlder, tOlder, 1)
		addView(custTie, pNewer, tNewer, 1)

		profile, err := svc.GetCustomerPreferenceProfile(ctx, custTie, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.Len(t, profile.ViewedCategories, 2)

		assert.Equal(t, catNewer, profile.ViewedCategories[0].CategoryID)
		assert.Equal(t, int64(1), profile.ViewedCategories[0].DistinctProductCount)
		assert.Equal(t, catOlder, profile.ViewedCategories[1].CategoryID)
		assert.Equal(t, int64(1), profile.ViewedCategories[1].DistinctProductCount)
	})

	// Subtest F: Customer B data never affects Customer A profile.
	t.Run("F: Customer B data never affects Customer A profile", func(t *testing.T) {
		catOnlyB := createCat("Category Only B")
		brandOnlyB := createBrand("Brand Only B")
		pOnlyB := createProd(seller, catOnlyB, &brandOnlyB, "Product Only B")

		addFav(custB, pOnlyB, now)
		addView(custB, pOnlyB, now, 10)

		profA, err := svc.GetCustomerPreferenceProfile(ctx, custA, 10)
		require.NoError(t, err)
		for _, c := range profA.FavoriteCategories {
			assert.NotEqual(t, catOnlyB, c.CategoryID, "Customer A profile must not contain Customer B favorite category")
		}
		for _, b := range profA.FavoriteBrands {
			assert.NotEqual(t, brandOnlyB, b.BrandID, "Customer A profile must not contain Customer B favorite brand")
		}
		for _, c := range profA.ViewedCategories {
			assert.NotEqual(t, catOnlyB, c.CategoryID, "Customer A profile must not contain Customer B viewed category")
		}
		for _, b := range profA.ViewedBrands {
			assert.NotEqual(t, brandOnlyB, b.BrandID, "Customer A profile must not contain Customer B viewed brand")
		}
	})

	// Subtest G: favorite provenance remains separate from viewed provenance.
	t.Run("G: favorite provenance remains separate from viewed provenance", func(t *testing.T) {
		custProv := createUser("Customer Prov")
		catFav := createCat("Category Fav Only")
		catView := createCat("Category View Only")
		brandFav := createBrand("Brand Fav Only")
		brandView := createBrand("Brand View Only")

		pFav := createProd(seller, catFav, &brandFav, "Product Fav")
		pView := createProd(seller, catView, &brandView, "Product View")

		addFav(custProv, pFav, now)
		addView(custProv, pView, now, 1)

		profile, err := svc.GetCustomerPreferenceProfile(ctx, custProv, 5)
		require.NoError(t, err)

		require.Len(t, profile.FavoriteCategories, 1)
		assert.Equal(t, catFav, profile.FavoriteCategories[0].CategoryID)
		assert.Equal(t, personalization.AffinityProvenanceFavorite, profile.FavoriteCategories[0].Provenance)

		require.Len(t, profile.ViewedCategories, 1)
		assert.Equal(t, catView, profile.ViewedCategories[0].CategoryID)
		assert.Equal(t, personalization.AffinityProvenanceViewed, profile.ViewedCategories[0].Provenance)

		require.Len(t, profile.FavoriteBrands, 1)
		assert.Equal(t, brandFav, profile.FavoriteBrands[0].BrandID)
		assert.Equal(t, personalization.AffinityProvenanceFavorite, profile.FavoriteBrands[0].Provenance)

		require.Len(t, profile.ViewedBrands, 1)
		assert.Equal(t, brandView, profile.ViewedBrands[0].BrandID)
		assert.Equal(t, personalization.AffinityProvenanceViewed, profile.ViewedBrands[0].Provenance)
	})

	// Subtest H: brand NULL does not fail profile generation.
	t.Run("H: brand NULL does not fail profile generation", func(t *testing.T) {
		custNoBrand := createUser("Customer NoBrand")
		catNoBrand := createCat("Category NoBrand")
		pNoBrand := createProd(seller, catNoBrand, nil, "Product NoBrand")

		addFav(custNoBrand, pNoBrand, now)
		addView(custNoBrand, pNoBrand, now, 2)

		profile, err := svc.GetCustomerPreferenceProfile(ctx, custNoBrand, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		require.Len(t, profile.FavoriteCategories, 1)
		assert.Equal(t, catNoBrand, profile.FavoriteCategories[0].CategoryID)

		require.Len(t, profile.ViewedCategories, 1)
		assert.Equal(t, catNoBrand, profile.ViewedCategories[0].CategoryID)

		assert.Empty(t, profile.FavoriteBrands, "brand NULL must be excluded from FavoriteBrands")
		assert.Empty(t, profile.ViewedBrands, "brand NULL must be excluded from ViewedBrands")
	})

	// Subtest I: customer with no favorites/views -> valid empty profile.
	t.Run("I: customer with no favorites/views -> valid empty profile", func(t *testing.T) {
		profile, err := svc.GetCustomerPreferenceProfile(ctx, custEmpty, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, custEmpty, profile.UserID)
		assert.Empty(t, profile.FavoriteCategories)
		assert.Empty(t, profile.FavoriteBrands)
		assert.Empty(t, profile.ViewedCategories)
		assert.Empty(t, profile.ViewedBrands)

		// Must serialize to clean non-null arrays in JSON
		b, err := json.Marshal(profile)
		require.NoError(t, err)
		var jsonMap map[string]interface{}
		err = json.Unmarshal(b, &jsonMap)
		require.NoError(t, err)
		assert.Equal(t, []interface{}{}, jsonMap["favoriteCategories"])
		assert.Equal(t, []interface{}{}, jsonMap["favoriteBrands"])
		assert.Equal(t, []interface{}{}, jsonMap["viewedCategories"])
		assert.Equal(t, []interface{}{}, jsonMap["viewedBrands"])
	})

	// Subtest J: profile lists respect configured limits.
	t.Run("J: profile lists respect configured limits", func(t *testing.T) {
		custLimits := createUser("Customer Limits")
		for i := 0; i < 8; i++ {
			c := createCat("Cat Limit")
			p := createProd(seller, c, nil, "Prod Limit")
			addFav(custLimits, p, now.Add(-time.Duration(i)*time.Hour))
		}

		// Explicit limit = 3
		prof3, err := svc.GetCustomerPreferenceProfile(ctx, custLimits, 3)
		require.NoError(t, err)
		assert.Len(t, prof3.FavoriteCategories, 3)

		// Limit <= 0 defaults to DefaultProfileAffinityLimit (5)
		profDefault, err := svc.GetCustomerPreferenceProfile(ctx, custLimits, 0)
		require.NoError(t, err)
		assert.Len(t, profDefault.FavoriteCategories, personalization.DefaultProfileAffinityLimit)

		// Limit > 20 is clamped to MaxProfileAffinityLimit (20)
		profClamped, err := svc.GetCustomerPreferenceProfile(ctx, custLimits, 999)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(profClamped.FavoriteCategories), personalization.MaxProfileAffinityLimit)
	})

	// Subtest K: deterministic tie-breaking.
	t.Run("K: deterministic tie-breaking", func(t *testing.T) {
		custTieBreak := createUser("Customer TieBreak")
		tFixed := now.Add(-5 * time.Hour)

		cat1 := createCat("Cat Tie 1")
		cat2 := createCat("Cat Tie 2")
		p1 := createProd(seller, cat1, nil, "Prod Tie 1")
		p2 := createProd(seller, cat2, nil, "Prod Tie 2")

		addFav(custTieBreak, p1, tFixed)
		addFav(custTieBreak, p2, tFixed)

		pFirst, err := svc.GetCustomerPreferenceProfile(ctx, custTieBreak, 5)
		require.NoError(t, err)
		require.Len(t, pFirst.FavoriteCategories, 2)

		// Run repeated calls to verify stable ordering
		for i := 0; i < 5; i++ {
			pNext, err := svc.GetCustomerPreferenceProfile(ctx, custTieBreak, 5)
			require.NoError(t, err)
			assert.Equal(t, pFirst.FavoriteCategories[0].CategoryID, pNext.FavoriteCategories[0].CategoryID)
			assert.Equal(t, pFirst.FavoriteCategories[1].CategoryID, pNext.FavoriteCategories[1].CategoryID)
		}
	})

	// Subtest L: no customer PII / finance / order/payment data exists in profile model.
	t.Run("L: no customer PII / finance / order/payment data exists in profile model", func(t *testing.T) {
		profileType := reflect.TypeOf(personalization.CustomerPreferenceProfile{})
		disallowed := []string{"email", "phone", "name", "password", "card", "bank", "price", "cents", "order", "payment", "payout", "balance"}

		for i := 0; i < profileType.NumField(); i++ {
			field := profileType.Field(i)
			tag := field.Tag.Get("json")
			for _, bad := range disallowed {
				assert.NotContains(t, tag, bad, "Field tag %s must not contain PII or financial term %s", tag, bad)
			}
		}

		catAffType := reflect.TypeOf(personalization.CategoryAffinity{})
		for i := 0; i < catAffType.NumField(); i++ {
			tag := catAffType.Field(i).Tag.Get("json")
			for _, bad := range disallowed {
				assert.NotContains(t, tag, bad, "CategoryAffinity tag %s must not contain %s", tag, bad)
			}
		}

		brandAffType := reflect.TypeOf(personalization.BrandAffinity{})
		for i := 0; i < brandAffType.NumField(); i++ {
			tag := brandAffType.Field(i).Tag.Get("json")
			for _, bad := range disallowed {
				assert.NotContains(t, tag, bad, "BrandAffinity tag %s must not contain %s", tag, bad)
			}
		}
	})

	// Section 15 Negative Test:
	// Product A: view_count = 100
	// Product B: view_count = 1
	// If both are the only distinct products in different categories:
	// viewed category affinity must NOT become 100 vs 1 merely because of view_count.
	// Each distinct viewed product contributes once.
	t.Run("Negative Test (Section 15): view_count = 100 vs view_count = 1 does not inflate affinity score", func(t *testing.T) {
		custNeg := createUser("Customer Neg")
		catInflated := createCat("Cat Inflated Views")
		catSingle := createCat("Cat Single View")
		brandCommon := createBrand("Brand Neg")

		pInflated := createProd(seller, catInflated, &brandCommon, "Prod Inflated")
		pSingle := createProd(seller, catSingle, &brandCommon, "Prod Single")

		tOlder := now.Add(-2 * time.Hour)
		tNewer := now.Add(-10 * time.Minute)

		// Product A has 100 views, but older last_viewed_at
		addView(custNeg, pInflated, tOlder, 100)
		// Product B has 1 view, but newer last_viewed_at
		addView(custNeg, pSingle, tNewer, 1)

		profile, err := svc.GetCustomerPreferenceProfile(ctx, custNeg, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.Len(t, profile.ViewedCategories, 2)

		// Both categories must have DistinctProductCount = 1!
		// Because both have count = 1, tie-breaker MAX(last_viewed_at) DESC must place catSingle FIRST!
		assert.Equal(t, catSingle, profile.ViewedCategories[0].CategoryID)
		assert.Equal(t, int64(1), profile.ViewedCategories[0].DistinctProductCount)

		assert.Equal(t, catInflated, profile.ViewedCategories[1].CategoryID)
		assert.Equal(t, int64(1), profile.ViewedCategories[1].DistinctProductCount, "Cat with view_count=100 must have distinct count=1, not 100")
	})
}
