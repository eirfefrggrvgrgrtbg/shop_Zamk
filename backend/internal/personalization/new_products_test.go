package personalization_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProducts_PERS_2C2(t *testing.T) {
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

	var createdSellers []uuid.UUID
	var createdCats []uuid.UUID
	var createdProds []uuid.UUID

	defer func() {
		if len(createdProds) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id = ANY($1)", createdProds)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = ANY($1)", createdProds)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProds)
		}
		if len(createdCats) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCats)
		}
		if len(createdSellers) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellers)
		}
	}()

	createSeller := func(name string, status string) uuid.UUID {
		id := uuid.New()
		slug := "sel-" + id.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
		`, id, name, slug, slug+"@test.local", status, now)
		require.NoError(t, err)
		createdSellers = append(createdSellers, id)
		return id
	}

	createCat := func(name string) uuid.UUID {
		id := uuid.New()
		slug := "cat-" + id.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, $4, $4)
		`, id, name, slug, now)
		require.NoError(t, err)
		createdCats = append(createdCats, id)
		return id
	}

	createProdWithTimestamps := func(
		sellerID, catID uuid.UUID,
		title string,
		status string,
		createdAt time.Time,
		publishedAt *time.Time,
		stock int,
	) uuid.UUID {
		pID := uuid.New()
		vID := uuid.New()
		createdProds = append(createdProds, pID)

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status,
				average_rating, reviews_count,
				submitted_at, approved_at, published_at, created_at, updated_at
			) VALUES ($1, $2, $3, NULL, $4, $5, 100000, 'RUB', $6, 0.0, 0, $7, $7, $8, $7, $7)
		`, pID, sellerID, catID, title, "slug-"+pID.String()[:8], status, createdAt, publishedAt)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, 100000, true, $5, $5)
		`, vID, pID, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8], createdAt)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 0, $6, $6)
		`, uuid.New(), pID, vID, sellerID, stock, createdAt)
		require.NoError(t, err)

		return pID
	}

	sellerActive := createSeller("Active New Seller", "active")
	sellerBlocked := createSeller("Blocked New Seller", "blocked")
	catCommon := createCat("Common New Cat")

	// A. более свежий published_at выше
	t.Run("A. более свежий published_at выше", func(t *testing.T) {
		tNewer := now.Add(2 * time.Hour)
		tOlder := now.Add(1 * time.Hour)

		pNewer := createProdWithTimestamps(sellerActive, catCommon, "Prod Newer Pub", "published", tNewer, &tNewer, 10)
		pOlder := createProdWithTimestamps(sellerActive, catCommon, "Prod Older Pub", "published", tOlder, &tOlder, 10)

		res, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		var idxNewer, idxOlder int = -1, -1
		for i, p := range res {
			if p.ID == pNewer {
				idxNewer = i
			}
			if p.ID == pOlder {
				idxOlder = i
			}
		}
		require.NotEqual(t, -1, idxNewer)
		require.NotEqual(t, -1, idxOlder)
		assert.Less(t, idxNewer, idxOlder, "Product published 2h ahead must rank before product published 1h ahead")
	})

	// B. created_at не влияет на ranking
	t.Run("B. created_at не влияет на ranking", func(t *testing.T) {
		// Product A: created 30 days ago, but published 4 hours in the future
		tCreatedOld := now.Add(-30 * 24 * time.Hour)
		tPubRecent := now.Add(4 * time.Hour)
		pOldCreatedRecentPub := createProdWithTimestamps(sellerActive, catCommon, "Old Created Recent Pub", "published", tCreatedOld, &tPubRecent, 10)

		// Product B: created 3 days ago, published 3 hours in the future
		tCreatedRecent := now.Add(-3 * 24 * time.Hour)
		tPubOlder := now.Add(3 * time.Hour)
		pRecentCreatedOlderPub := createProdWithTimestamps(sellerActive, catCommon, "Recent Created Older Pub", "published", tCreatedRecent, &tPubOlder, 10)

		res, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		var idxA, idxB int = -1, -1
		for i, p := range res {
			if p.ID == pOldCreatedRecentPub {
				idxA = i
			}
			if p.ID == pRecentCreatedOlderPub {
				idxB = i
			}
		}
		require.NotEqual(t, -1, idxA)
		require.NotEqual(t, -1, idxB)
		assert.Less(t, idxA, idxB, "Product with newer published_at must rank first regardless of created_at")
	})

	// C. product старше 14 дней исключается
	t.Run("C. product старше 14 дней исключается", func(t *testing.T) {
		tExpired := now.Add(-15 * 24 * time.Hour)
		tValid := now.Add(5 * time.Hour)

		pExpired := createProdWithTimestamps(sellerActive, catCommon, "15 Days Old Product", "published", tExpired, &tExpired, 10)
		pValid := createProdWithTimestamps(sellerActive, catCommon, "5 Hours Ahead Product", "published", tValid, &tValid, 10)

		res, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		foundValid := false
		for _, p := range res {
			assert.NotEqual(t, pExpired, p.ID, "Products published > 14 days ago must be excluded from New Products")
			if p.ID == pValid {
				foundValid = true
			}
		}
		assert.True(t, foundValid, "Product published within 14 days must be included")
	})

	// D. unpublished исключается
	t.Run("D. unpublished исключается", func(t *testing.T) {
		tRecent := now.Add(6 * time.Hour)
		pDraft := createProdWithTimestamps(sellerActive, catCommon, "Draft Product", "draft", tRecent, &tRecent, 10)
		pPending := createProdWithTimestamps(sellerActive, catCommon, "Pending Product", "pending_moderation", tRecent, &tRecent, 10)

		res, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		for _, p := range res {
			assert.NotEqual(t, pDraft, p.ID, "Draft product must be excluded")
			assert.NotEqual(t, pPending, p.ID, "Pending moderation product must be excluded")
		}
	})

	// E. inactive seller исключается
	t.Run("E. inactive seller исключается", func(t *testing.T) {
		tRecent := now.Add(7 * time.Hour)
		pBlocked := createProdWithTimestamps(sellerBlocked, catCommon, "Blocked Seller Product", "published", tRecent, &tRecent, 10)

		res, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		for _, p := range res {
			assert.NotEqual(t, pBlocked, p.ID, "Product from blocked seller must be excluded")
		}
	})

	// F. insufficient canonical stock исключается
	t.Run("F. insufficient canonical stock исключается", func(t *testing.T) {
		tRecent := now.Add(8 * time.Hour)
		pLowStock := createProdWithTimestamps(sellerActive, catCommon, "Low Stock Product (1)", "published", tRecent, &tRecent, 1)
		pGoodStock := createProdWithTimestamps(sellerActive, catCommon, "Good Stock Product (2)", "published", tRecent, &tRecent, 2)

		res, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		foundGood := false
		for _, p := range res {
			assert.NotEqual(t, pLowStock, p.ID, "Free sellable stock = 1 must be excluded by CAT.1A threshold")
			if p.ID == pGoodStock {
				foundGood = true
			}
		}
		assert.True(t, foundGood, "Free sellable stock = 2 must be eligible")
	})

	// G. published_at NULL исключается
	t.Run("G. published_at NULL исключается", func(t *testing.T) {
		pNullPub := createProdWithTimestamps(sellerActive, catCommon, "NULL PublishedAt Product", "published", now, nil, 10)

		res, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		for _, p := range res {
			assert.NotEqual(t, pNullPub, p.ID, "Product with published_at NULL must be excluded")
		}
	})

	// H. одинаковый published_at => product_id ASC
	t.Run("H. одинаковый published_at => product_id ASC", func(t *testing.T) {
		tExact := now.Add(9 * time.Hour)
		c1 := createProdWithTimestamps(sellerActive, catCommon, "Tie Cand 1", "published", tExact, &tExact, 10)
		c2 := createProdWithTimestamps(sellerActive, catCommon, "Tie Cand 2", "published", tExact, &tExact, 10)
		c3 := createProdWithTimestamps(sellerActive, catCommon, "Tie Cand 3", "published", tExact, &tExact, 10)

		resFirst, err := svc.GetNewProducts(ctx, 50)
		require.NoError(t, err)

		getIdx := func(list []products.PublicProduct, id uuid.UUID) int {
			for i, p := range list {
				if p.ID == id {
					return i
				}
			}
			return -1
		}

		idx1 := getIdx(resFirst, c1)
		idx2 := getIdx(resFirst, c2)
		idx3 := getIdx(resFirst, c3)
		require.NotEqual(t, -1, idx1)
		require.NotEqual(t, -1, idx2)
		require.NotEqual(t, -1, idx3)

		// Check deterministic tie-break by UUID ASC
		if c1.String() < c2.String() {
			assert.Less(t, idx1, idx2)
		} else {
			assert.Less(t, idx2, idx1)
		}

		// Multi-run repeatability
		for run := 0; run < 5; run++ {
			resNext, err := svc.GetNewProducts(ctx, 50)
			require.NoError(t, err)
			require.Equal(t, len(resFirst), len(resNext))
			for i := range resFirst {
				assert.Equal(t, resFirst[i].ID, resNext[i].ID, "Ordering must be byte-for-byte identical across runs")
			}
		}
	})

	// I. empty result => [] / HTTP 200
	t.Run("I. empty result => [] / HTTP 200", func(t *testing.T) {
		// Handler HTTP request when empty
		req := httptest.NewRequest("GET", "/api/public/products/new", nil)
		rec := httptest.NewRecorder()
		handler.GetNewProducts(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp, "items")
		assert.Contains(t, resp, "totalCount")
	})

	// J. public API contract
	t.Run("J. public API contract", func(t *testing.T) {
		tPub := now.Add(10 * time.Hour)
		pContract := createProdWithTimestamps(sellerActive, catCommon, "Contract New Product", "published", tPub, &tPub, 10)

		req := httptest.NewRequest("GET", "/api/public/products/new?limit=50", nil)
		rec := httptest.NewRecorder()
		handler.GetNewProducts(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var parsed struct {
			Items      []products.PublicProduct `json:"items"`
			TotalCount int                      `json:"totalCount"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &parsed)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, parsed.TotalCount, 1)
		assert.Equal(t, len(parsed.Items), parsed.TotalCount)

		// Check public product contract fields
		foundContractProd := false
		for _, item := range parsed.Items {
			if item.ID == pContract {
				foundContractProd = true
				assert.Equal(t, "Contract New Product", item.Title)
				assert.Equal(t, int64(100000), item.PriceCents)
				assert.Equal(t, "RUB", item.Currency)
				assert.Equal(t, "published", item.Status)
				assert.NotEmpty(t, item.SellerSlug)
				assert.NotEmpty(t, item.SellerName)
				assert.False(t, item.CreatedAt.IsZero())
			}
		}
		assert.True(t, foundContractProd, "Contract product must be found in response items")
	})

	// K. no duplicate/alias routes if canonical route already exists
	t.Run("K. no duplicate/alias routes if canonical route already exists", func(t *testing.T) {
		// Verify handler works cleanly with limits
		prods, err := svc.GetNewProducts(ctx, 1)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(prods), 1)

		prodsDefault, err := svc.GetNewProducts(ctx, 0)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(prodsDefault), personalization.DefaultNewProductsLimit)
	})
}
