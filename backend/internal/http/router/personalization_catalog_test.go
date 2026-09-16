package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersonalizationCatalog_A_Through_AA(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	// AA. Database isolation invariant
	var dbName string
	err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "tests must strictly run against zamk_test")

	// Base IDs
	sellerUserID := uuid.New()
	adminUserID := uuid.New()
	customerEmptyID := uuid.New()
	customerAffinityID := uuid.New()

	createTestUser := func(id uuid.UUID, role string) string {
		email := fmt.Sprintf("%s-%s@zamk.local", role, id.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test User', 'hash', $4, 'active', now(), now())
		`, id, email, "+7999"+id.String()[:7], role)
		require.NoError(t, err)

		tok, err := tokenService.GenerateAccessToken(id, email, role)
		require.NoError(t, err)
		return tok
	}

	tokSeller := createTestUser(sellerUserID, "seller")
	tokAdmin := createTestUser(adminUserID, "admin")
	tokCustomerEmpty := createTestUser(customerEmptyID, "customer")
	tokCustomerAffinity := createTestUser(customerAffinityID, "customer")

	// Create seller
	sellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Per Test Brand', $2, $3, 'active', now(), now())
	`, sellerID, "per-brand-"+sellerID.String()[:8], "per-"+sellerID.String()[:8]+"@test.local")
	require.NoError(t, err)

	// Create 3 Categories
	cat1ID := uuid.New()
	cat2ID := uuid.New()
	cat3ID := uuid.New()
	for _, cat := range []struct {
		id   uuid.UUID
		name string
	}{
		{cat1ID, "Cat 1"},
		{cat2ID, "Cat 2"},
		{cat3ID, "Cat 3"},
	} {
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, now(), now())
		`, cat.id, cat.name, "cat-"+cat.id.String()[:8])
		require.NoError(t, err)
	}

	// Create 3 Brands
	brand1ID := uuid.New()
	brand2ID := uuid.New()
	brand3ID := uuid.New()
	for _, br := range []struct {
		id   uuid.UUID
		name string
	}{
		{brand1ID, "Brand 1"},
		{brand2ID, "Brand 2"},
		{brand3ID, "Brand 3"},
	} {
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, now(), now())
		`, br.id, br.name, "brand-"+br.id.String()[:8])
		require.NoError(t, err)
	}

	// Helper to insert a product with free stock
	createProduct := func(pID uuid.UUID, title string, cID, bID uuid.UUID, priceCents int64, pubAt, createdAt time.Time, totalStock, reservedStock int) {
		slug := fmt.Sprintf("p-%s", pID.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status,
				submitted_at, approved_at, published_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 'RUB', 'published', $8, $8, $8, $9, $9)
		`, pID, sellerID, cID, bID, title, slug, priceCents, pubAt, createdAt)
		require.NoError(t, err)

		vID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, $5, true, $6, $6)
		`, vID, pID, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8], priceCents, createdAt)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		`, uuid.New(), pID, vID, sellerID, totalStock, reservedStock)
		require.NoError(t, err)
	}

	now := time.Now().Truncate(time.Second)

	pSeedFav1 := uuid.New()
	pSeedFav2 := uuid.New()
	pSeedFav3 := uuid.New()
	pSeedView1 := uuid.New()

	createProduct(pSeedFav1, "Seed Fav 1", cat1ID, brand1ID, 10000, now.Add(-100*time.Hour), now.Add(-100*time.Hour), 5, 0)
	createProduct(pSeedFav2, "Seed Fav 2", cat1ID, brand1ID, 11000, now.Add(-99*time.Hour), now.Add(-99*time.Hour), 5, 0)
	createProduct(pSeedFav3, "Seed Fav 3", cat2ID, brand2ID, 12000, now.Add(-98*time.Hour), now.Add(-98*time.Hour), 5, 0)
	createProduct(pSeedView1, "Seed View 1", cat3ID, brand3ID, 13000, now.Add(-97*time.Hour), now.Add(-97*time.Hour), 5, 0)

	// Add favorites for customerAffinity
	for _, pid := range []uuid.UUID{pSeedFav1, pSeedFav2, pSeedFav3} {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO customer_favorites (id, user_id, product_id, created_at)
			VALUES ($1, $2, $3, now())
		`, uuid.New(), customerAffinityID, pid)
		require.NoError(t, err)
	}

	// Add views for customerAffinity
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO customer_product_views (user_id, product_id, view_count, last_viewed_at)
		VALUES ($1, $2, 1, now())
	`, customerAffinityID, pSeedView1)
	require.NoError(t, err)

	cat4ID := uuid.New()
	brand4ID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Cat 4', $2, true, now(), now())`, cat4ID, "cat-"+cat4ID.String()[:8])
	require.NoError(t, err)
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO brands (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Brand 4', $2, true, now(), now())`, brand4ID, "brand-"+brand4ID.String()[:8])
	require.NoError(t, err)

	pT1_Rank1Cat_Rank1Brand := uuid.New()
	pT1_Rank1Cat_Rank2Brand := uuid.New()
	pT1_Rank2Cat_Rank1Brand := uuid.New()
	pT2_Brand1 := uuid.New()
	pT3_Cat1 := uuid.New()
	pT4_ViewedCat_ViewedBrand := uuid.New()
	pT5_ViewedBrand := uuid.New()
	pT6_ViewedCat := uuid.New()
	pT7_UnmatchedA := uuid.New()
	pT7_UnmatchedB := uuid.New()

	pCat1A_Excluded := uuid.New()
	pCat1A_Included := uuid.New()

	timePubNewer := now.Add(-10 * time.Minute)
	timePubOlder := now.Add(-50 * time.Minute)

	createProduct(pT1_Rank1Cat_Rank1Brand, "Target T1 Rank11", cat1ID, brand1ID, 1000, now.Add(-10*time.Minute), now.Add(-10*time.Minute), 10, 0)
	createProduct(pT1_Rank1Cat_Rank2Brand, "Target T1 Rank12", cat1ID, brand2ID, 2000, now.Add(-5*time.Minute), now.Add(-5*time.Minute), 10, 0)
	createProduct(pT1_Rank2Cat_Rank1Brand, "Target T1 Rank21", cat2ID, brand1ID, 3000, now.Add(-2*time.Minute), now.Add(-2*time.Minute), 10, 0)
	createProduct(pT2_Brand1, "Target T2 Brand1", cat4ID, brand1ID, 4000, now.Add(-15*time.Minute), now.Add(-15*time.Minute), 10, 0)
	createProduct(pT3_Cat1, "Target T3 Cat1", cat1ID, brand4ID, 5000, now.Add(-16*time.Minute), now.Add(-16*time.Minute), 10, 0)
	createProduct(pT4_ViewedCat_ViewedBrand, "Target T4 DualViewed", cat3ID, brand3ID, 6000, now.Add(-17*time.Minute), now.Add(-17*time.Minute), 10, 0)
	createProduct(pT5_ViewedBrand, "Target T5 ViewedBrand", cat4ID, brand3ID, 7000, now.Add(-18*time.Minute), now.Add(-18*time.Minute), 10, 0)
	createProduct(pT6_ViewedCat, "Target T6 ViewedCat", cat3ID, brand4ID, 8000, now.Add(-19*time.Minute), now.Add(-19*time.Minute), 10, 0)
	createProduct(pT7_UnmatchedA, "Target T7 Newer", cat4ID, brand4ID, 9000, timePubNewer, timePubNewer, 10, 0)
	createProduct(pT7_UnmatchedB, "Target T7 Older", cat4ID, brand4ID, 9500, timePubOlder, timePubOlder, 10, 0)

	createProduct(pCat1A_Excluded, "CAT1A Excluded", cat4ID, brand4ID, 50000, now, now, 1, 0)
	createProduct(pCat1A_Included, "CAT1A Included", cat4ID, brand4ID, 50001, now, now, 3, 1)

	doGet := func(url string, token string) (*httptest.ResponseRecorder, publicCatalogResponse) {
		req := httptest.NewRequest("GET", url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp publicCatalogResponse
		if w.Code == http.StatusOK {
			_ = json.NewDecoder(w.Body).Decode(&resp)
		}
		return w, resp
	}

	t.Run("A_AuthenticatedCustomer_Returns200", func(t *testing.T) {
		w, resp := doGet("/api/customer/catalog?limit=5", tokCustomerEmpty)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, resp.Items)
	})

	t.Run("B_Unauthenticated_Returns401", func(t *testing.T) {
		w, _ := doGet("/api/customer/catalog", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("C_SellerOrAdmin_Returns403", func(t *testing.T) {
		wSeller, _ := doGet("/api/customer/catalog", tokSeller)
		assert.Equal(t, http.StatusForbidden, wSeller.Code)

		wAdmin, _ := doGet("/api/customer/catalog", tokAdmin)
		assert.Equal(t, http.StatusForbidden, wAdmin.Code)
	})

	t.Run("D_PublicCatalog_RetainsAnonymousOrder", func(t *testing.T) {
		_, pubResp := doGet("/api/public/products?limit=100", "")
		_, custResp := doGet("/api/customer/catalog?limit=100", tokCustomerAffinity)

		assert.Equal(t, pubResp.TotalCount, custResp.TotalCount, "TotalCount must match between public and customer")
		assert.Equal(t, len(pubResp.Items), len(custResp.Items), "Item count must match")

		for i := 0; i < len(pubResp.Items)-1; i++ {
			p1 := pubResp.Items[i]
			p2 := pubResp.Items[i+1]
			if p1.PublishedAt != nil && p2.PublishedAt != nil {
				assert.True(t, p1.PublishedAt.After(*p2.PublishedAt) || p1.PublishedAt.Equal(*p2.PublishedAt), "Public order must be published_at DESC")
			}
		}
	})

	t.Run("E_EmptyCustomer_IdenticalToAnonymous", func(t *testing.T) {
		_, pubResp := doGet("/api/public/products?limit=100", "")
		_, emptyResp := doGet("/api/customer/catalog?limit=100", tokCustomerEmpty)

		require.Equal(t, len(pubResp.Items), len(emptyResp.Items))
		for i := range pubResp.Items {
			assert.Equal(t, pubResp.Items[i].ID, emptyResp.Items[i].ID, "Position %d must match between anonymous and empty customer", i)
		}
	})

	t.Run("F_Exact7TierOrder", func(t *testing.T) {
		url := fmt.Sprintf("/api/customer/catalog?sellerId=%s&limit=50", sellerID.String())
		_, resp := doGet(url, tokCustomerAffinity)

		idToPos := make(map[string]int)
		for idx, item := range resp.Items {
			idToPos[item.ID] = idx
		}

		posT1, ok1 := idToPos[pT1_Rank1Cat_Rank1Brand.String()]
		posT2, ok2 := idToPos[pT2_Brand1.String()]
		posT3, ok3 := idToPos[pT3_Cat1.String()]
		posT4, ok4 := idToPos[pT4_ViewedCat_ViewedBrand.String()]
		posT5, ok5 := idToPos[pT5_ViewedBrand.String()]
		posT6, ok6 := idToPos[pT6_ViewedCat.String()]
		posT7, ok7 := idToPos[pT7_UnmatchedA.String()]

		require.True(t, ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7, "All 7 tier items must be present")

		assert.True(t, posT1 < posT2, "Tier 1 (%d) must precede Tier 2 (%d)", posT1, posT2)
		assert.True(t, posT2 < posT3, "Tier 2 (%d) must precede Tier 3 (%d)", posT2, posT3)
		assert.True(t, posT3 < posT4, "Tier 3 (%d) must precede Tier 4 (%d)", posT3, posT4)
		assert.True(t, posT4 < posT5, "Tier 4 (%d) must precede Tier 5 (%d)", posT4, posT5)
		assert.True(t, posT5 < posT6, "Tier 5 (%d) must precede Tier 6 (%d)", posT5, posT6)
		assert.True(t, posT6 < posT7, "Tier 6 (%d) must precede Tier 7 (%d)", posT6, posT7)
	})

	t.Run("G_ProfileRank_CategoryRankTieBreaker", func(t *testing.T) {
		url := fmt.Sprintf("/api/customer/catalog?sellerId=%s&limit=50", sellerID.String())
		_, resp := doGet(url, tokCustomerAffinity)

		idToPos := make(map[string]int)
		for idx, item := range resp.Items {
			idToPos[item.ID] = idx
		}

		posCatRank1, ok1 := idToPos[pT1_Rank1Cat_Rank1Brand.String()]
		posCatRank2, ok2 := idToPos[pT1_Rank2Cat_Rank1Brand.String()]
		require.True(t, ok1 && ok2)

		assert.True(t, posCatRank1 < posCatRank2, "Cat Rank 1 (%d) must precede Cat Rank 2 (%d)", posCatRank1, posCatRank2)
	})

	t.Run("H_ProfileRank_BrandRankTieBreaker", func(t *testing.T) {
		url := fmt.Sprintf("/api/customer/catalog?sellerId=%s&limit=50", sellerID.String())
		_, resp := doGet(url, tokCustomerAffinity)

		idToPos := make(map[string]int)
		for idx, item := range resp.Items {
			idToPos[item.ID] = idx
		}

		posBrandRank1, ok1 := idToPos[pT1_Rank1Cat_Rank1Brand.String()]
		posBrandRank2, ok2 := idToPos[pT1_Rank1Cat_Rank2Brand.String()]
		require.True(t, ok1 && ok2)

		assert.True(t, posBrandRank1 < posBrandRank2, "Brand Rank 1 (%d) must precede Brand Rank 2 (%d)", posBrandRank1, posBrandRank2)
	})

	t.Run("I_WithinTier_TieBreaker_PublishedAtDesc", func(t *testing.T) {
		url := fmt.Sprintf("/api/customer/catalog?categoryId=%s&limit=50", cat4ID.String())
		_, resp := doGet(url, tokCustomerAffinity)

		idToPos := make(map[string]int)
		for idx, item := range resp.Items {
			idToPos[item.ID] = idx
		}

		posNewer, okNewer := idToPos[pT7_UnmatchedA.String()]
		posOlder, okOlder := idToPos[pT7_UnmatchedB.String()]
		require.True(t, okNewer, "pT7_UnmatchedA must be present")
		require.True(t, okOlder, "pT7_UnmatchedB must be present")

		assert.True(t, posNewer < posOlder, "In Tier 7, newer published_at (%d) must precede older published_at (%d)", posNewer, posOlder)
	})

	t.Run("J_TotalCatalogPreservation", func(t *testing.T) {
		fetchAll := func(url string, tok string) map[string]bool {
			ids := make(map[string]bool)
			offset := 0
			limit := 100
			for {
				_, resp := doGet(fmt.Sprintf("%s?limit=%d&offset=%d", url, limit, offset), tok)
				for _, item := range resp.Items {
					ids[item.ID] = true
				}
				if len(resp.Items) < limit || len(ids) >= resp.TotalCount {
					break
				}
				offset += limit
			}
			return ids
		}

		pubSet := fetchAll("/api/public/products", "")
		custSet := fetchAll("/api/customer/catalog", tokCustomerAffinity)

		assert.Equal(t, len(pubSet), len(custSet))
		assert.Equal(t, pubSet, custSet, "Product candidate set across all pages must be 100% identical")
	})

	t.Run("K_FavoritedProductsAppearInCatalog", func(t *testing.T) {
		_, custResp := doGet("/api/customer/catalog?limit=100", tokCustomerAffinity)

		foundSeedFav1 := false
		for _, item := range custResp.Items {
			if item.ID == pSeedFav1.String() {
				foundSeedFav1 = true
				break
			}
		}
		assert.True(t, foundSeedFav1, "Favorited product must be included in catalog")
	})

	t.Run("L_ViewedProductsAppearInCatalog", func(t *testing.T) {
		_, custResp := doGet("/api/customer/catalog?limit=100", tokCustomerAffinity)

		foundSeedView1 := false
		for _, item := range custResp.Items {
			if item.ID == pSeedView1.String() {
				foundSeedView1 = true
				break
			}
		}
		assert.True(t, foundSeedView1, "Viewed product must be included in catalog")
	})

	t.Run("M_N_CAT1A_FreeStockRule", func(t *testing.T) {
		_, custResp := doGet("/api/customer/catalog?limit=100", tokCustomerAffinity)

		custIDs := make(map[string]bool)
		for _, item := range custResp.Items {
			custIDs[item.ID] = true
		}

		assert.False(t, custIDs[pCat1A_Excluded.String()], "Free=1 product MUST be excluded from customer catalog")
		assert.True(t, custIDs[pCat1A_Included.String()], "Free=2 product MUST be included in customer catalog")

		_, pubResp := doGet("/api/public/products?limit=100", "")
		pubIDs := make(map[string]bool)
		for _, item := range pubResp.Items {
			pubIDs[item.ID] = true
		}

		assert.False(t, pubIDs[pCat1A_Excluded.String()], "Free=1 product MUST be excluded from public catalog")
		assert.True(t, pubIDs[pCat1A_Included.String()], "Free=2 product MUST be included in public catalog")
	})

	t.Run("O_ExplicitSort_PriceAsc", func(t *testing.T) {
		_, resp := doGet("/api/customer/catalog?sort=price_asc&limit=100", tokCustomerAffinity)
		require.NotEmpty(t, resp.Items)

		for i := 0; i < len(resp.Items)-1; i++ {
			p1 := resp.Items[i]
			p2 := resp.Items[i+1]
			assert.True(t, p1.PriceCents <= p2.PriceCents, "price_asc: %d <= %d", p1.PriceCents, p2.PriceCents)
			if p1.PriceCents == p2.PriceCents {
				assert.True(t, p1.ID < p2.ID, "price_asc tie-breaker: %s < %s", p1.ID, p2.ID)
			}
		}
	})

	t.Run("P_ExplicitSort_PriceDesc", func(t *testing.T) {
		_, resp := doGet("/api/customer/catalog?sort=price_desc&limit=100", tokCustomerAffinity)
		require.NotEmpty(t, resp.Items)

		for i := 0; i < len(resp.Items)-1; i++ {
			p1 := resp.Items[i]
			p2 := resp.Items[i+1]
			assert.True(t, p1.PriceCents >= p2.PriceCents, "price_desc: %d >= %d", p1.PriceCents, p2.PriceCents)
			if p1.PriceCents == p2.PriceCents {
				assert.True(t, p1.ID < p2.ID, "price_desc tie-breaker: %s < %s", p1.ID, p2.ID)
			}
		}
	})

	t.Run("Q_ExplicitSort_Newest", func(t *testing.T) {
		_, resp := doGet("/api/customer/catalog?sort=newest&limit=100", tokCustomerAffinity)
		_, pubResp := doGet("/api/public/products?sort=newest&limit=100", "")

		require.Equal(t, len(pubResp.Items), len(resp.Items))
		for i := range resp.Items {
			assert.Equal(t, pubResp.Items[i].ID, resp.Items[i].ID, "sort=newest must match public sort=newest exactly at %d", i)
		}
	})

	t.Run("R_QueryFilter_PreservesTiers", func(t *testing.T) {
		_, resp := doGet("/api/customer/catalog?q=Target&limit=100", tokCustomerAffinity)
		require.NotEmpty(t, resp.Items)

		idToPos := make(map[string]int)
		for idx, item := range resp.Items {
			idToPos[item.ID] = idx
		}

		posT1 := idToPos[pT1_Rank1Cat_Rank1Brand.String()]
		posT2 := idToPos[pT2_Brand1.String()]
		posT7 := idToPos[pT7_UnmatchedA.String()]

		assert.True(t, posT1 < posT2, "Under q filter, Tier 1 (%d) must precede Tier 2 (%d)", posT1, posT2)
		assert.True(t, posT2 < posT7, "Under q filter, Tier 2 (%d) must precede Tier 7 (%d)", posT2, posT7)
	})

	t.Run("S_CategoryFilter", func(t *testing.T) {
		url := fmt.Sprintf("/api/customer/catalog?categoryId=%s&limit=100", cat1ID.String())
		_, resp := doGet(url, tokCustomerAffinity)
		require.NotEmpty(t, resp.Items)

		idToPos := make(map[string]int)
		for idx, item := range resp.Items {
			idToPos[item.ID] = idx
		}

		posT1 := idToPos[pT1_Rank1Cat_Rank1Brand.String()]
		posT3 := idToPos[pT3_Cat1.String()]
		assert.True(t, posT1 < posT3, "Under category filter, Tier 1 (%d) must precede Tier 3 (%d)", posT1, posT3)
	})

	t.Run("T_BrandFilter", func(t *testing.T) {
		url := fmt.Sprintf("/api/customer/catalog?brandId=%s&limit=100", brand1ID.String())
		_, resp := doGet(url, tokCustomerAffinity)
		require.NotEmpty(t, resp.Items)

		idToPos := make(map[string]int)
		for idx, item := range resp.Items {
			idToPos[item.ID] = idx
		}

		posT1 := idToPos[pT1_Rank1Cat_Rank1Brand.String()]
		posT2 := idToPos[pT2_Brand1.String()]
		assert.True(t, posT1 < posT2, "Under brand filter, Tier 1 (%d) must precede Tier 2 (%d)", posT1, posT2)
	})

	t.Run("U_InStockFilter", func(t *testing.T) {
		_, pubResp := doGet("/api/public/products?inStock=true&limit=100", "")
		_, custResp := doGet("/api/customer/catalog?inStock=true&limit=100", tokCustomerAffinity)

		assert.Equal(t, pubResp.TotalCount, custResp.TotalCount)
	})

	t.Run("V_PriceRangeFilter", func(t *testing.T) {
		_, pubResp := doGet("/api/public/products?minPriceCents=1500&maxPriceCents=6500&limit=100", "")
		_, custResp := doGet("/api/customer/catalog?minPriceCents=1500&maxPriceCents=6500&limit=100", tokCustomerAffinity)

		assert.Equal(t, pubResp.TotalCount, custResp.TotalCount)
		for _, item := range custResp.Items {
			assert.True(t, item.PriceCents >= 1500 && item.PriceCents <= 6500)
		}
	})

	t.Run("W_DeterministicPagination", func(t *testing.T) {
		_, p1 := doGet("/api/customer/catalog?limit=3&offset=0", tokCustomerAffinity)
		_, p2 := doGet("/api/customer/catalog?limit=3&offset=3", tokCustomerAffinity)
		_, full := doGet("/api/customer/catalog?limit=6&offset=0", tokCustomerAffinity)

		require.Len(t, p1.Items, 3)
		require.Len(t, p2.Items, 3)
		require.Len(t, full.Items, 6)

		for i := 0; i < 3; i++ {
			assert.Equal(t, full.Items[i].ID, p1.Items[i].ID, "P1 item %d must match full item %d", i, i)
		}
		for i := 0; i < 3; i++ {
			assert.Equal(t, full.Items[3+i].ID, p2.Items[i].ID, "P2 item %d must match full item %d", i, 3+i)
		}

		p1Map := make(map[string]bool)
		for _, item := range p1.Items {
			p1Map[item.ID] = true
		}
		for _, item := range p2.Items {
			assert.False(t, p1Map[item.ID], "P2 item %s must not appear in P1", item.ID)
		}
	})

	t.Run("X_LargeCatalogSanity", func(t *testing.T) {
		start := time.Now()
		w, resp := doGet("/api/customer/catalog?limit=50", tokCustomerAffinity)
		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, resp.Items)
		assert.Less(t, duration, 500*time.Millisecond, "Personalized catalog query should execute in under 500ms")
	})

	t.Run("Y_RedundantCustomerProductsRouteRemoved", func(t *testing.T) {
		w, _ := doGet("/api/customer/products", tokCustomerAffinity)
		assert.Equal(t, http.StatusNotFound, w.Code, "GET /api/customer/products must return 404 since it is removed")
	})

	t.Run("Z_InvalidParameters_Return400", func(t *testing.T) {
		w, _ := doGet("/api/customer/catalog?categoryId=invalid-uuid", tokCustomerAffinity)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		wBrand, _ := doGet("/api/customer/catalog?brandId=invalid-uuid", tokCustomerAffinity)
		assert.Equal(t, http.StatusBadRequest, wBrand.Code)

		wMinPrice, _ := doGet("/api/customer/catalog?minPriceCents=invalid", tokCustomerAffinity)
		assert.Equal(t, http.StatusBadRequest, wMinPrice.Code)

		wMaxPrice, _ := doGet("/api/customer/catalog?maxPriceCents=-50", tokCustomerAffinity)
		assert.Equal(t, http.StatusBadRequest, wMaxPrice.Code)
	})
}

func TestPersonalizationCatalog_NoMatchProfileFallback(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	var dbName string
	err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName)

	customerID := uuid.New()
	email := fmt.Sprintf("nomatch-%s@zamk.local", customerID.String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'NoMatch User', 'hash', 'customer', 'active', now(), now())
	`, customerID, email, "+7999"+customerID.String()[:7])
	require.NoError(t, err)

	token, err := tokenService.GenerateAccessToken(customerID, email, "customer")
	require.NoError(t, err)

	isolatedCatID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Isolated Cat', $2, true, now(), now())`, isolatedCatID, "iso-cat-"+isolatedCatID.String()[:8])
	require.NoError(t, err)

	isolatedBrandID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO brands (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Isolated Brand', $2, true, now(), now())`, isolatedBrandID, "iso-brand-"+isolatedBrandID.String()[:8])
	require.NoError(t, err)

	dummySellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Iso Seller', $2, $3, 'active', now(), now())
	`, dummySellerID, "iso-seller-"+dummySellerID.String()[:8], "iso-"+dummySellerID.String()[:8]+"@test.local")
	require.NoError(t, err)

	dummyProdID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status,
			submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'Iso Product', $5, 1000, 'RUB', 'draft', now(), now(), now(), now(), now())
	`, dummyProdID, dummySellerID, isolatedCatID, isolatedBrandID, "iso-p-"+dummyProdID.String()[:8])
	require.NoError(t, err)

	// Customer has favorites and views on this isolated category/brand
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO customer_favorites (id, user_id, product_id, created_at)
		VALUES ($1, $2, $3, now())
	`, uuid.New(), customerID, dummyProdID)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO customer_product_views (user_id, product_id, view_count, last_viewed_at)
		VALUES ($1, $2, 5, now())
	`, customerID, dummyProdID)
	require.NoError(t, err)

	doGet := func(url string, tok string) (*httptest.ResponseRecorder, publicCatalogResponse) {
		req := httptest.NewRequest("GET", url, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp publicCatalogResponse
		if w.Code == http.StatusOK {
			_ = json.NewDecoder(w.Body).Decode(&resp)
		}
		return w, resp
	}

	wPub, pubResp := doGet("/api/public/products?limit=50", "")
	require.Equal(t, http.StatusOK, wPub.Code)

	wCust, custResp := doGet("/api/customer/catalog?limit=50", token)
	require.Equal(t, http.StatusOK, wCust.Code)

	require.Equal(t, pubResp.TotalCount, custResp.TotalCount)
	require.Equal(t, len(pubResp.Items), len(custResp.Items))
	for i := range pubResp.Items {
		assert.Equal(t, pubResp.Items[i].ID, custResp.Items[i].ID, "Position %d must match between anonymous default and non-empty profile with zero eligible matches", i)
	}
}

func TestPersonalizationCatalog_CrossUserIsolation(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	custAID := uuid.New()
	emailA := fmt.Sprintf("custA-%s@zamk.local", custAID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'User A', 'hash', 'customer', 'active', now(), now())`, custAID, emailA, "+7999"+custAID.String()[:7])
	tokA, _ := tokenService.GenerateAccessToken(custAID, emailA, "customer")

	custBID := uuid.New()
	emailB := fmt.Sprintf("custB-%s@zamk.local", custBID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'User B', 'hash', 'customer', 'active', now(), now())`, custBID, emailB, "+7999"+custBID.String()[:7])
	tokB, _ := tokenService.GenerateAccessToken(custBID, emailB, "customer")

	sellerID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Iso Sellers', $2, $3, 'active', now(), now())`, sellerID, "iso-sellers-"+sellerID.String()[:8], "iso-"+sellerID.String()[:8]+"@test.local")

	catAID := uuid.New()
	catBID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Cat A', $2, true, now(), now())`, catAID, "cat-a-"+catAID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Cat B', $2, true, now(), now())`, catBID, "cat-b-"+catBID.String()[:8])

	prodAID := uuid.New()
	prodBID := uuid.New()
	now := time.Now().Truncate(time.Second)

	createProd := func(id uuid.UUID, title string, cID uuid.UUID, pubAt time.Time) {
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, submitted_at, approved_at, published_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 5000, 'RUB', 'published', $6, $6, $6, $6, $6)`, id, sellerID, cID, title, "p-"+id.String()[:8], pubAt)
		vID := uuid.New()
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, 5000, true, now(), now())`, vID, id, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8])
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, now(), now())`, uuid.New(), id, vID, sellerID)
	}

	createProd(prodAID, "Product A", catAID, now.Add(-5*time.Minute))
	createProd(prodBID, "Product B", catBID, now.Add(-2*time.Minute))

	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_favorites (id, user_id, product_id, created_at) VALUES ($1, $2, $3, now())`, uuid.New(), custAID, prodAID)
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_favorites (id, user_id, product_id, created_at) VALUES ($1, $2, $3, now())`, uuid.New(), custBID, prodBID)

	doGet := func(url string, tok string) (*httptest.ResponseRecorder, publicCatalogResponse) {
		req := httptest.NewRequest("GET", url, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp publicCatalogResponse
		if w.Code == http.StatusOK {
			_ = json.NewDecoder(w.Body).Decode(&resp)
		}
		return w, resp
	}

	_, respA := doGet(fmt.Sprintf("/api/customer/catalog?sellerId=%s", sellerID.String()), tokA)
	require.Len(t, respA.Items, 2)
	assert.Equal(t, prodAID.String(), respA.Items[0].ID, "Customer A must see Product A first")
	assert.Equal(t, prodBID.String(), respA.Items[1].ID, "Customer A must see Product B second")

	_, respB := doGet(fmt.Sprintf("/api/customer/catalog?sellerId=%s", sellerID.String()), tokB)
	require.Len(t, respB.Items, 2)
	assert.Equal(t, prodBID.String(), respB.Items[0].ID, "Customer B must see Product B first")
	assert.Equal(t, prodAID.String(), respB.Items[1].ID, "Customer B must see Product A second")
}

func TestPersonalizationCatalog_RawViewCountIrrelevant(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	custID := uuid.New()
	email := fmt.Sprintf("viewcount-%s@zamk.local", custID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'VC User', 'hash', 'customer', 'active', now(), now())`, custID, email, "+7999"+custID.String()[:7])
	tok, _ := tokenService.GenerateAccessToken(custID, email, "customer")

	sellerID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'VC Seller', $2, $3, 'active', now(), now())`, sellerID, "vc-seller-"+sellerID.String()[:8], "vc-"+sellerID.String()[:8]+"@test.local")

	cat1 := uuid.New()
	cat2 := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Cat VC 1', $2, true, now(), now())`, cat1, "cat-vc-1-"+cat1.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Cat VC 2', $2, true, now(), now())`, cat2, "cat-vc-2-"+cat2.String()[:8])

	now := time.Now().Truncate(time.Second)
	p1 := uuid.New()
	p2 := uuid.New()

	createProd := func(id uuid.UUID, title string, cID uuid.UUID, pubAt time.Time) {
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, submitted_at, approved_at, published_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 5000, 'RUB', 'published', $6, $6, $6, $6, $6)`, id, sellerID, cID, title, "p-"+id.String()[:8], pubAt)
		vID := uuid.New()
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, 5000, true, now(), now())`, vID, id, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8])
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, now(), now())`, uuid.New(), id, vID, sellerID)
	}

	createProd(p1, "Prod VC 1", cat1, now.Add(-10*time.Minute))
	createProd(p2, "Prod VC 2", cat2, now.Add(-5*time.Minute))

	tView1 := now.Add(-2 * time.Hour)
	tView2 := now.Add(-1 * time.Hour)
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_product_views (user_id, product_id, view_count, last_viewed_at) VALUES ($1, $2, 1, $3)`, custID, p1, tView1)
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_product_views (user_id, product_id, view_count, last_viewed_at) VALUES ($1, $2, 1, $3)`, custID, p2, tView2)

	doGet := func(url string, tok string) (*httptest.ResponseRecorder, publicCatalogResponse) {
		req := httptest.NewRequest("GET", url, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp publicCatalogResponse
		if w.Code == http.StatusOK {
			_ = json.NewDecoder(w.Body).Decode(&resp)
		}
		return w, resp
	}

	_, respBefore := doGet(fmt.Sprintf("/api/customer/catalog?sellerId=%s", sellerID.String()), tok)
	require.Len(t, respBefore.Items, 2)

	_, err := pgClient.Pool.Exec(ctx, `UPDATE customer_product_views SET view_count = 100 WHERE user_id = $1 AND product_id = $2`, custID, p1)
	require.NoError(t, err)

	_, respAfter := doGet(fmt.Sprintf("/api/customer/catalog?sellerId=%s", sellerID.String()), tok)
	require.Len(t, respAfter.Items, 2)

	assert.Equal(t, respBefore.Items[0].ID, respAfter.Items[0].ID, "Inflating view_count from 1 to 100 must not change order")
	assert.Equal(t, respBefore.Items[1].ID, respAfter.Items[1].ID, "Inflating view_count from 1 to 100 must not change order")
}

func TestPersonalizationCatalog_StorefrontEligibility(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	custID := uuid.New()
	email := fmt.Sprintf("elig-%s@zamk.local", custID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'Elig User', 'hash', 'customer', 'active', now(), now())`, custID, email, "+7999"+custID.String()[:7])
	tok, _ := tokenService.GenerateAccessToken(custID, email, "customer")

	activeSellerID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Active Seller', $2, $3, 'active', now(), now())`, activeSellerID, "act-seller-"+activeSellerID.String()[:8], "act-"+activeSellerID.String()[:8]+"@test.local")

	suspendedSellerID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Suspended Seller', $2, $3, 'suspended', now(), now())`, suspendedSellerID, "susp-seller-"+suspendedSellerID.String()[:8], "susp-"+suspendedSellerID.String()[:8]+"@test.local")

	catID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Elig Cat', $2, true, now(), now())`, catID, "cat-elig-"+catID.String()[:8])

	draftProdID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, created_at, updated_at) VALUES ($1, $2, $3, 'Draft Product', $4, 5000, 'RUB', 'draft', now(), now())`, draftProdID, activeSellerID, catID, "draft-"+draftProdID.String()[:8])
	vDraft := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, 5000, true, now(), now())`, vDraft, draftProdID, "SKU-"+vDraft.String()[:8], "BC-"+vDraft.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, now(), now())`, uuid.New(), draftProdID, vDraft, activeSellerID)

	suspProdID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, submitted_at, approved_at, published_at, created_at, updated_at) VALUES ($1, $2, $3, 'Suspended Product', $4, 5000, 'RUB', 'published', now(), now(), now(), now(), now())`, suspProdID, suspendedSellerID, catID, "susp-"+suspProdID.String()[:8])
	vSusp := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, 5000, true, now(), now())`, vSusp, suspProdID, "SKU-"+vSusp.String()[:8], "BC-"+vSusp.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, now(), now())`, uuid.New(), suspProdID, vSusp, suspendedSellerID)

	seedProdID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, submitted_at, approved_at, published_at, created_at, updated_at) VALUES ($1, $2, $3, 'Seed Product', $4, 5000, 'RUB', 'published', now(), now(), now(), now(), now())`, seedProdID, activeSellerID, catID, "seed-"+seedProdID.String()[:8])
	vSeed := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, 5000, true, now(), now())`, vSeed, seedProdID, "SKU-"+vSeed.String()[:8], "BC-"+vSeed.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, now(), now())`, uuid.New(), seedProdID, vSeed, activeSellerID)
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_favorites (id, user_id, product_id, created_at) VALUES ($1, $2, $3, now())`, uuid.New(), custID, seedProdID)

	doGet := func(url string, tok string) (*httptest.ResponseRecorder, publicCatalogResponse) {
		req := httptest.NewRequest("GET", url, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp publicCatalogResponse
		if w.Code == http.StatusOK {
			_ = json.NewDecoder(w.Body).Decode(&resp)
		}
		return w, resp
	}

	_, resp := doGet(fmt.Sprintf("/api/customer/catalog?categoryId=%s", catID.String()), tok)
	for _, item := range resp.Items {
		assert.NotEqual(t, draftProdID.String(), item.ID, "Unpublished product MUST be absent from customer catalog")
		assert.NotEqual(t, suspProdID.String(), item.ID, "Suspended seller product MUST be absent from customer catalog")
	}
}

func TestPersonalizationCatalog_SearchFilterContract(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	custID := uuid.New()
	email := fmt.Sprintf("search-%s@zamk.local", custID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'Search User', 'hash', 'customer', 'active', now(), now())`, custID, email, "+7999"+custID.String()[:7])
	tok, _ := tokenService.GenerateAccessToken(custID, email, "customer")

	sellerID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Search Seller', $2, $3, 'active', now(), now())`, sellerID, "search-seller-"+sellerID.String()[:8], "search-"+sellerID.String()[:8]+"@test.local")

	favCatID := uuid.New()
	otherCatID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Fav Search Cat', $2, true, now(), now())`, favCatID, "fav-cat-"+favCatID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Other Search Cat', $2, true, now(), now())`, otherCatID, "oth-cat-"+otherCatID.String()[:8])

	now := time.Now().Truncate(time.Second)

	createProd := func(id uuid.UUID, title string, cID uuid.UUID, pubAt time.Time) {
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, submitted_at, approved_at, published_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 5000, 'RUB', 'published', $6, $6, $6, $6, $6)`, id, sellerID, cID, title, "p-"+id.String()[:8], pubAt)
		vID := uuid.New()
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, 5000, true, now(), now())`, vID, id, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8])
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, now(), now())`, uuid.New(), id, vID, sellerID)
	}

	pSearchFavMatch := uuid.New()
	pSearchNonFavMatch := uuid.New()
	pNonSearchFavMatch := uuid.New()

	searchTerm := "SearchTerm" + uuid.New().String()[:8]

	createProd(pSearchFavMatch, "Coat "+searchTerm, favCatID, now.Add(-10*time.Minute))
	createProd(pSearchNonFavMatch, "Pants "+searchTerm, otherCatID, now.Add(-5*time.Minute))
	createProd(pNonSearchFavMatch, "Regular Wool Sweater", favCatID, now.Add(-2*time.Minute))

	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_favorites (id, user_id, product_id, created_at) VALUES ($1, $2, $3, now())`, uuid.New(), custID, pSearchFavMatch)

	doGet := func(url string, tok string) (*httptest.ResponseRecorder, publicCatalogResponse) {
		req := httptest.NewRequest("GET", url, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp publicCatalogResponse
		if w.Code == http.StatusOK {
			_ = json.NewDecoder(w.Body).Decode(&resp)
		}
		return w, resp
	}

	_, resp := doGet("/api/customer/catalog?q="+searchTerm, tok)
	require.Len(t, resp.Items, 2)

	for _, item := range resp.Items {
		assert.NotEqual(t, pNonSearchFavMatch.String(), item.ID, "Product not matching text search must NEVER appear")
	}

	assert.Equal(t, pSearchFavMatch.String(), resp.Items[0].ID, "Affinity match must be first among search matches")
	assert.Equal(t, pSearchNonFavMatch.String(), resp.Items[1].ID, "Non-affinity match must be second among search matches")
}

func TestPersonalizationCatalog_ClientCannotSelectAnotherCustomer(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupPersonalizationRouterTestEnv(t)
	defer cleanup()

	custAID := uuid.New()
	emailA := fmt.Sprintf("spoofA-%s@zamk.local", custAID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'User A', 'hash', 'customer', 'active', now(), now())`, custAID, emailA, "+7999"+custAID.String()[:7])
	tokA, _ := tokenService.GenerateAccessToken(custAID, emailA, "customer")

	custBID := uuid.New()
	emailB := fmt.Sprintf("spoofB-%s@zamk.local", custBID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1, $2, $3, 'User B', 'hash', 'customer', 'active', now(), now())`, custBID, emailB, "+7999"+custBID.String()[:7])

	sellerID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Spoof Seller', $2, $3, 'active', now(), now())`, sellerID, "spoof-seller-"+sellerID.String()[:8], "spoof-"+sellerID.String()[:8]+"@test.local")

	catAID := uuid.New()
	catBID := uuid.New()
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Cat Spoof A', $2, true, now(), now())`, catAID, "cat-sp-a-"+catAID.String()[:8])
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, is_active, created_at, updated_at) VALUES ($1, 'Cat Spoof B', $2, true, now(), now())`, catBID, "cat-sp-b-"+catBID.String()[:8])

	now := time.Now().Truncate(time.Second)
	pA := uuid.New()
	pB := uuid.New()

	createProd := func(id uuid.UUID, title string, cID uuid.UUID, pubAt time.Time) {
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, submitted_at, approved_at, published_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 5000, 'RUB', 'published', $6, $6, $6, $6, $6)`, id, sellerID, cID, title, "p-"+id.String()[:8], pubAt)
		vID := uuid.New()
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, 5000, true, now(), now())`, vID, id, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8])
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, now(), now())`, uuid.New(), id, vID, sellerID)
	}

	createProd(pA, "Product Spoof A", catAID, now.Add(-10*time.Minute))
	createProd(pB, "Product Spoof B", catBID, now.Add(-5*time.Minute))

	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_favorites (id, user_id, product_id, created_at) VALUES ($1, $2, $3, now())`, uuid.New(), custAID, pA)
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO customer_favorites (id, user_id, product_id, created_at) VALUES ($1, $2, $3, now())`, uuid.New(), custBID, pB)

	doGet := func(url string, tok string) (*httptest.ResponseRecorder, publicCatalogResponse) {
		req := httptest.NewRequest("GET", url, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp publicCatalogResponse
		if w.Code == http.StatusOK {
			_ = json.NewDecoder(w.Body).Decode(&resp)
		}
		return w, resp
	}

	urlWithUserID := fmt.Sprintf("/api/customer/catalog?sellerId=%s&user_id=%s", sellerID.String(), custBID.String())
	w1, resp1 := doGet(urlWithUserID, tokA)
	require.Equal(t, http.StatusOK, w1.Code)
	require.Len(t, resp1.Items, 2)
	assert.Equal(t, pA.String(), resp1.Items[0].ID, "Customer A identity must be used; ?user_id must have zero effect")

	urlWithCustID := fmt.Sprintf("/api/customer/catalog?sellerId=%s&customer_id=%s", sellerID.String(), custBID.String())
	w2, resp2 := doGet(urlWithCustID, tokA)
	require.Equal(t, http.StatusOK, w2.Code)
	require.Len(t, resp2.Items, 2)
	assert.Equal(t, pA.String(), resp2.Items[0].ID, "Customer A identity must be used; ?customer_id must have zero effect")
}
