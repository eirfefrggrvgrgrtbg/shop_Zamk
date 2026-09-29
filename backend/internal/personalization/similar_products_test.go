package personalization_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimilarProducts_PERS_2D1(t *testing.T) {
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
	defer pgClient.Close()

	// Invariant: strictly assert zamk_test
	var currentDB string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "integration tests must strictly run against zamk_test")

	repo := personalization.NewRepository(pgClient.Pool)
	svc := personalization.NewService(repo)
	handler := personalization.NewHandler(svc)

	var dbNow time.Time
	err = pgClient.Pool.QueryRow(ctx, "SELECT now()").Scan(&dbNow)
	require.NoError(t, err)

	now := dbNow.UTC().Truncate(time.Microsecond)
	t0 := now.Add(-10 * time.Hour)
	t1 := now.Add(-5 * time.Hour)
	t2 := now.Add(-1 * time.Hour)

	var createdSellers []uuid.UUID
	var createdCats []uuid.UUID
	var createdBrands []uuid.UUID
	var createdProds []uuid.UUID

	defer func() {
		if len(createdProds) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id = ANY($1)", createdProds)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = ANY($1)", createdProds)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProds)
		}
		if len(createdBrands) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM brands WHERE id = ANY($1)", createdBrands)
		}
		if len(createdCats) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCats)
		}
		if len(createdSellers) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellers)
		}
	}()

	insertSeller := func(name string, status string) uuid.UUID {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
		`, id, name, "seller-"+id.String()[:8], "seller-"+id.String()[:8]+"@test.local", status, now)
		require.NoError(t, err)
		createdSellers = append(createdSellers, id)
		return id
	}

	insertCategory := func(name string) uuid.UUID {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, $4, $4)
		`, id, name, "cat-"+id.String()[:8], now)
		require.NoError(t, err)
		createdCats = append(createdCats, id)
		return id
	}

	insertBrand := func(name string) uuid.UUID {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, $4, $4)
		`, id, name, "brand-"+id.String()[:8], now)
		require.NoError(t, err)
		createdBrands = append(createdBrands, id)
		return id
	}

	insertProduct := func(
		sellerID uuid.UUID,
		catID *uuid.UUID,
		brandID *uuid.UUID,
		title string,
		priceCents int64,
		rating float64,
		reviewsCount int,
		publishedAt *time.Time,
		status string,
		stock int,
	) uuid.UUID {
		prodID := uuid.New()
		slug := "sim-" + prodID.String()[:8]

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status,
				average_rating, reviews_count,
				submitted_at, approved_at, published_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 'RUB', $8, $9, $10, $11, $11, $12, $11, $11)
		`, prodID, sellerID, catID, brandID, title, slug, priceCents, status, rating, reviewsCount, now, publishedAt)
		require.NoError(t, err)
		createdProds = append(createdProds, prodID)

		varID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, $5, true, $6, $6)
		`, varID, prodID, "SKU-"+varID.String()[:8], "BC-"+varID.String()[:8], priceCents, now)
		require.NoError(t, err)

		if stock > 0 {
			_, err = pgClient.Pool.Exec(ctx, `
				INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 0, $6, $6)
			`, uuid.New(), prodID, varID, sellerID, stock, now)
			require.NoError(t, err)
		}

		return prodID
	}

	activeSellerID := insertSeller("Active Seller", "active")
	blockedSellerID := insertSeller("Blocked Seller", "blocked")

	catMain := insertCategory("Main Similar Category")
	catOther := insertCategory("Other Similar Category")

	brandA := insertBrand("Brand Alpha")
	brandB := insertBrand("Brand Beta")

	// Target product: Brand A, Category Main, Price 100_000, Rating 4.0, Stock 10
	targetProdID := insertProduct(activeSellerID, &catMain, &brandA, "Target Product", 100000, 4.0, 10, &t0, "published", 10)

	// Eligible candidates in catMain:
	// Candidate 1: Same Brand A, Price 105_000 (diff: 5_000)
	cSameBrandClose := insertProduct(activeSellerID, &catMain, &brandA, "Same Brand Close", 105000, 4.0, 10, &t0, "published", 10)
	// Candidate 2: Same Brand A, Price 150_000 (diff: 50_000)
	cSameBrandFar := insertProduct(activeSellerID, &catMain, &brandA, "Same Brand Far", 150000, 4.0, 10, &t0, "published", 10)
	// Candidate 3: Diff Brand B, Price 100_000 (diff: 0)
	cDiffBrandExact := insertProduct(activeSellerID, &catMain, &brandB, "Diff Brand Exact", 100000, 4.0, 10, &t0, "published", 10)
	// Candidate 4: Diff Brand B, Price 120_000 (diff: 20_000)
	cDiffBrandFar := insertProduct(activeSellerID, &catMain, &brandB, "Diff Brand Far", 120000, 4.0, 10, &t0, "published", 10)

	// Ineligible candidates in catMain or other:
	// Candidate 5: Other category (catOther), Brand A, Price 100_000, Rating 5.0
	cOtherCategory := insertProduct(activeSellerID, &catOther, &brandA, "Other Category Prod", 100000, 5.0, 50, &t0, "published", 10)
	// Candidate 6: Unpublished (pending_moderation)
	cUnpublished := insertProduct(activeSellerID, &catMain, &brandA, "Unpublished Prod", 101000, 5.0, 50, &t0, "pending_moderation", 10)
	// Candidate 7: Inactive (blocked) seller
	cInactiveSeller := insertProduct(blockedSellerID, &catMain, &brandA, "Blocked Seller Prod", 101000, 5.0, 50, &t0, "published", 10)
	// Candidate 8: Low stock (stock = 1, required >= 2)
	cLowStock := insertProduct(activeSellerID, &catMain, &brandA, "Low Stock Prod", 101000, 5.0, 50, &t0, "published", 1)

	// Run focused checks A through M
	res, err := svc.GetSimilarProducts(ctx, targetProdID, 10)
	require.NoError(t, err)

	t.Run("A. current product is never returned in similar products", func(t *testing.T) {
		for _, p := range res {
			assert.NotEqual(t, targetProdID, p.ID, "Target product must never appear in similar products")
		}
	})

	t.Run("B. product of a different category is not returned", func(t *testing.T) {
		for _, p := range res {
			assert.NotEqual(t, cOtherCategory, p.ID, "Product from another category must not appear")
		}
	})

	t.Run("C. same-brand candidate is ranked higher than different-brand candidate", func(t *testing.T) {
		require.True(t, len(res) >= 3)
		// Both same-brand candidates (cSameBrandClose, cSameBrandFar) rank before different-brand
		// even though cDiffBrandExact has exact price match (diff 0 vs 5_000)
		assert.Equal(t, cSameBrandClose, res[0].ID)
		assert.Equal(t, cSameBrandFar, res[1].ID)
		assert.Equal(t, cDiffBrandExact, res[2].ID)
	})

	t.Run("D. within same-brand, closer price ranks higher", func(t *testing.T) {
		require.True(t, len(res) >= 2)
		// 105_000 (diff 5_000) ranks before 150_000 (diff 50_000)
		assert.Equal(t, cSameBrandClose, res[0].ID)
		assert.Equal(t, cSameBrandFar, res[1].ID)
	})

	t.Run("E. different-brand candidates are also sorted by price distance", func(t *testing.T) {
		require.True(t, len(res) >= 4)
		// 100_000 (diff 0) ranks before 120_000 (diff 20_000)
		assert.Equal(t, cDiffBrandExact, res[2].ID)
		assert.Equal(t, cDiffBrandFar, res[3].ID)
	})

	t.Run("F. unpublished candidate is excluded", func(t *testing.T) {
		for _, p := range res {
			assert.NotEqual(t, cUnpublished, p.ID, "Unpublished product must be excluded")
		}
	})

	t.Run("G. inactive seller candidate is excluded", func(t *testing.T) {
		for _, p := range res {
			assert.NotEqual(t, cInactiveSeller, p.ID, "Product from inactive seller must be excluded")
		}
	})

	t.Run("H. insufficient canonical stock candidate is excluded", func(t *testing.T) {
		for _, p := range res {
			assert.NotEqual(t, cLowStock, p.ID, "Product with stock < MinStorefrontFreeSellableUnits must be excluded")
		}
	})

	t.Run("I. target without brand works (same-brand bonus absent, ranked by price distance)", func(t *testing.T) {
		catNoBrand := insertCategory("Target No Brand Category")
		targetNoBrand := insertProduct(activeSellerID, &catNoBrand, nil, "Target No Brand", 100000, 4.0, 10, &t0, "published", 10)
		candWithBrand := insertProduct(activeSellerID, &catNoBrand, &brandA, "Cand With Brand", 105000, 4.0, 10, &t0, "published", 10)
		candNoBrand := insertProduct(activeSellerID, &catNoBrand, nil, "Cand Without Brand", 102000, 4.0, 10, &t0, "published", 10)

		resNoBrand, err := svc.GetSimilarProducts(ctx, targetNoBrand, 10)
		require.NoError(t, err)
		require.Len(t, resNoBrand, 2)
		// No same-brand bonus -> sorted by price distance (102k diff 2k before 105k diff 5k)
		assert.Equal(t, candNoBrand, resNoBrand[0].ID)
		assert.Equal(t, candWithBrand, resNoBrand[1].ID)
	})

	t.Run("J. target without category returns empty result, not error", func(t *testing.T) {
		targetNoCat := insertProduct(activeSellerID, nil, &brandA, "Target Without Category", 100000, 4.0, 10, &t0, "published", 10)
		resNoCat, err := svc.GetSimilarProducts(ctx, targetNoCat, 10)
		require.NoError(t, err)
		assert.NotNil(t, resNoCat)
		assert.Empty(t, resNoCat)
	})

	t.Run("K. equal similarity uses quality tie-breakers (rating -> reviews -> published_at)", func(t *testing.T) {
		catTie := insertCategory("Tie Breaker Category")
		srcTie := insertProduct(activeSellerID, &catTie, &brandA, "Source Tie", 100000, 4.0, 10, &t0, "published", 10)

		// 1. High rating (5.0, 5 reviews, t1)
		c1HighRating := insertProduct(activeSellerID, &catTie, &brandA, "High Rating", 100000, 5.0, 5, &t1, "published", 10)
		// 2. Lower rating, high reviews (4.5, 50 reviews, t1)
		c2HighReviews := insertProduct(activeSellerID, &catTie, &brandA, "High Reviews", 100000, 4.5, 50, &t1, "published", 10)
		// 3. Same rating, lower reviews, newer published (4.5, 20 reviews, t2)
		c3NewerPub := insertProduct(activeSellerID, &catTie, &brandA, "Newer Pub", 100000, 4.5, 20, &t2, "published", 10)
		// 4. Same rating, lower reviews, older published (4.5, 20 reviews, t1)
		c4OlderPub := insertProduct(activeSellerID, &catTie, &brandA, "Older Pub", 100000, 4.5, 20, &t1, "published", 10)

		resTie, err := svc.GetSimilarProducts(ctx, srcTie, 10)
		require.NoError(t, err)
		require.Len(t, resTie, 4)

		assert.Equal(t, c1HighRating, resTie[0].ID, "1st tie-breaker: rating DESC")
		assert.Equal(t, c2HighReviews, resTie[1].ID, "2nd tie-breaker: reviews_count DESC")
		assert.Equal(t, c3NewerPub, resTie[2].ID, "3rd tie-breaker: published_at DESC (newer)")
		assert.Equal(t, c4OlderPub, resTie[3].ID, "4th tie-breaker: published_at DESC (older)")
	})

	t.Run("L. complete tie uses deterministic product_id ASC", func(t *testing.T) {
		catFullTie := insertCategory("Full Tie Category")
		srcFullTie := insertProduct(activeSellerID, &catFullTie, &brandA, "Source Full Tie", 100000, 4.0, 10, &t0, "published", 10)

		// Create 3 candidates with identical attributes: same brand, same price, same rating, same reviews, same published_at
		candA := insertProduct(activeSellerID, &catFullTie, &brandA, "Cand Full Tie A", 100000, 4.5, 10, &t1, "published", 10)
		candB := insertProduct(activeSellerID, &catFullTie, &brandA, "Cand Full Tie B", 100000, 4.5, 10, &t1, "published", 10)
		candC := insertProduct(activeSellerID, &catFullTie, &brandA, "Cand Full Tie C", 100000, 4.5, 10, &t1, "published", 10)

		resFullTie, err := svc.GetSimilarProducts(ctx, srcFullTie, 10)
		require.NoError(t, err)
		require.Len(t, resFullTie, 3)

		// Must be ordered by product_id ASC (UUID compare)
		expectedOrder := []uuid.UUID{candA, candB, candC}
		for i := 0; i < len(expectedOrder)-1; i++ {
			for j := i + 1; j < len(expectedOrder); j++ {
				if expectedOrder[i].String() > expectedOrder[j].String() {
					expectedOrder[i], expectedOrder[j] = expectedOrder[j], expectedOrder[i]
				}
			}
		}

		assert.Equal(t, expectedOrder[0], resFullTie[0].ID)
		assert.Equal(t, expectedOrder[1], resFullTie[1].ID)
		assert.Equal(t, expectedOrder[2], resFullTie[2].ID)
	})

	t.Run("M. existing API response contract is unchanged", func(t *testing.T) {
		r := chi.NewRouter()
		r.Get("/api/public/products/{productId}/similar", handler.GetSimilarProducts)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/public/products/%s/similar", targetProdID), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp struct {
			Items      []products.PublicProduct `json:"items"`
			TotalCount int                      `json:"totalCount"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, len(resp.Items), resp.TotalCount)
		assert.NotEmpty(t, resp.Items)

		item := resp.Items[0]
		assert.NotEmpty(t, item.ID)
		assert.NotEmpty(t, item.Title)
		assert.NotEmpty(t, item.Slug)
		assert.NotZero(t, item.PriceCents)
		assert.Equal(t, "RUB", item.Currency)
		assert.NotEmpty(t, item.SellerID)
		assert.NotEmpty(t, item.SellerSlug)
		assert.NotEmpty(t, item.SellerName)
	})
}
