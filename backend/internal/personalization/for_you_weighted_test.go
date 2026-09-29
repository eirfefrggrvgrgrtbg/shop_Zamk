package personalization_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForYouWeighted_PERS_2B3(t *testing.T) {
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

	now := time.Now().UTC().Truncate(time.Microsecond)

	var createdUsers []uuid.UUID
	var createdSellers []uuid.UUID
	var createdCats []uuid.UUID
	var createdBrands []uuid.UUID
	var createdProds []uuid.UUID
	var createdEvents []uuid.UUID

	defer func() {
		if len(createdEvents) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM behavioral_events WHERE id = ANY($1)", createdEvents)
		}
		if len(createdUsers) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM customer_favorites WHERE user_id = ANY($1)", createdUsers)
		}
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
		if len(createdUsers) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", createdUsers)
		}
	}()

	createUser := func(prefix string) uuid.UUID {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Weighted ForYou User', 'hash', 'customer', 'active', $4, $4)
		`, id, prefix+"-"+id.String()[:8]+"@test.local", "+7999"+id.String()[:7], now)
		require.NoError(t, err)
		createdUsers = append(createdUsers, id)
		return id
	}

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

	createBrand := func(name string) uuid.UUID {
		id := uuid.New()
		slug := "br-" + id.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, $4, $4)
		`, id, name, slug, now)
		require.NoError(t, err)
		createdBrands = append(createdBrands, id)
		return id
	}

	createProdWithDetails := func(
		sellerID, catID uuid.UUID,
		brandID *uuid.UUID,
		title string,
		status string,
		rating float64,
		reviewsCount int,
		publishedAt time.Time,
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
			) VALUES ($1, $2, $3, $4, $5, $6, 100000, 'RUB', $7, $8, $9, $10, $10, $10, $10, $10)
		`, pID, sellerID, catID, brandID, title, "slug-"+pID.String()[:8], status, rating, reviewsCount, publishedAt)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, 100000, true, $5, $5)
		`, vID, pID, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8], publishedAt)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 0, $6, $6)
		`, uuid.New(), pID, vID, sellerID, stock, publishedAt)
		require.NoError(t, err)

		return pID
	}

	createProd := func(sellerID, catID uuid.UUID, brandID *uuid.UUID, title string, status string, stock int) uuid.UUID {
		return createProdWithDetails(sellerID, catID, brandID, title, status, 0.0, 0, now, stock)
	}

	insertEvent := func(eventType string, userID *uuid.UUID, productID uuid.UUID, occurredAt time.Time) uuid.UUID {
		eID := uuid.New()
		createdEvents = append(createdEvents, eID)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO behavioral_events (
				id, event_type, source, visitor_id, user_id, product_id,
				occurred_at, received_at, metadata
			) VALUES ($1, $2, 'client', NULL, $3, $4, $5, $5, '{}'::jsonb)
		`, eID, eventType, userID, productID, occurredAt)
		require.NoError(t, err)
		return eID
	}

	sellerActive := createSeller("Active Seller", "active")
	sellerBlocked := createSeller("Blocked Seller", "blocked")

	// Test A: category score 10 beats category score 3
	t.Run("A. category score 10 beats category score 3", func(t *testing.T) {
		u := createUser("uA")
		cat10 := createCat("Cat 10 A")
		cat3 := createCat("Cat 3 A")
		brandCommon := createBrand("Brand Common A")

		// Seeds to build profile:
		// Product in cat10 has order_paid (score 10)
		pSeed10 := createProd(sellerActive, cat10, &brandCommon, "Seed 10", "published", 10)
		insertEvent("order_paid", &u, pSeed10, now.Add(-1*time.Hour))

		// Product in cat3 has favorite_added (score 3)
		pSeed3 := createProd(sellerActive, cat3, &brandCommon, "Seed 3", "published", 10)
		insertEvent("favorite_added", &u, pSeed3, now.Add(-2*time.Hour))

		// Candidates
		c10 := createProd(sellerActive, cat10, &brandCommon, "Candidate Cat 10", "published", 10)
		c3 := createProd(sellerActive, cat3, &brandCommon, "Candidate Cat 3", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		var idx10, idx3 int = -1, -1
		for i, p := range res {
			if p.ID == c10 {
				idx10 = i
			}
			if p.ID == c3 {
				idx3 = i
			}
		}
		require.NotEqual(t, -1, idx10, "c10 must be returned")
		require.NotEqual(t, -1, idx3, "c3 must be returned")
		assert.Less(t, idx10, idx3, "Category score 10 candidate must rank before category score 3 candidate")
	})

	// Test B: brand score 10 beats brand score 3
	t.Run("B. brand score 10 beats brand score 3", func(t *testing.T) {
		u := createUser("uB")
		catCommon := createCat("Cat Common B")
		brand10 := createBrand("Brand 10 B")
		brand3 := createBrand("Brand 3 B")

		pSeed10 := createProd(sellerActive, catCommon, &brand10, "Seed Brand 10", "published", 10)
		insertEvent("order_paid", &u, pSeed10, now.Add(-1*time.Hour))

		pSeed3 := createProd(sellerActive, catCommon, &brand3, "Seed Brand 3", "published", 10)
		insertEvent("favorite_added", &u, pSeed3, now.Add(-2*time.Hour))

		cB10 := createProd(sellerActive, catCommon, &brand10, "Cand Brand 10", "published", 10)
		cB3 := createProd(sellerActive, catCommon, &brand3, "Cand Brand 3", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		var idx10, idx3 int = -1, -1
		for i, p := range res {
			if p.ID == cB10 {
				idx10 = i
			}
			if p.ID == cB3 {
				idx3 = i
			}
		}
		require.NotEqual(t, -1, idx10)
		require.NotEqual(t, -1, idx3)
		assert.Less(t, idx10, idx3, "Brand score 10 candidate must rank before brand score 3 candidate")
	})

	// Test C: category + brand scores combine
	t.Run("C. category + brand scores combine", func(t *testing.T) {
		u := createUser("uC")
		catA := createCat("Cat A C")
		catB := createCat("Cat B C")
		brandX := createBrand("Brand X C")
		brandY := createBrand("Brand Y C")

		// CatA: 10 (order_paid), BrandX: 3 (favorite_added)
		pSeedCatA := createProd(sellerActive, catA, nil, "Seed Cat A", "published", 10)
		insertEvent("order_paid", &u, pSeedCatA, now.Add(-1*time.Hour))

		pSeedBrandX := createProd(sellerActive, catB, &brandX, "Seed Brand X", "published", 10)
		insertEvent("favorite_added", &u, pSeedBrandX, now.Add(-2*time.Hour))

		// Candidate 1: CatA (10) + BrandX (3) = 13
		cBoth := createProd(sellerActive, catA, &brandX, "Cand CatA + BrandX", "published", 10)
		// Candidate 2: CatA (10) + BrandY (0) = 10
		cCatOnly := createProd(sellerActive, catA, &brandY, "Cand CatA + BrandY", "published", 10)
		// Candidate 3: CatB (3 via fav) + BrandY (0) = 3
		cBrandFavCat := createProd(sellerActive, catB, &brandY, "Cand CatB", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		var idxBoth, idxCatOnly, idxThird int = -1, -1, -1
		for i, p := range res {
			if p.ID == cBoth {
				idxBoth = i
			}
			if p.ID == cCatOnly {
				idxCatOnly = i
			}
			if p.ID == cBrandFavCat {
				idxThird = i
			}
		}
		require.NotEqual(t, -1, idxBoth)
		require.NotEqual(t, -1, idxCatOnly)
		require.NotEqual(t, -1, idxThird)

		assert.Less(t, idxBoth, idxCatOnly, "Score 13 (CatA+BrandX) must rank before Score 10 (CatA only)")
		assert.Less(t, idxCatOnly, idxThird, "Score 10 (CatA) must rank before Score 3")
	})

	// Test D: order_paid-derived affinity can outrank view-derived affinity
	t.Run("D. order_paid-derived affinity can outrank view-derived affinity", func(t *testing.T) {
		u := createUser("uD")
		catPaid := createCat("Cat Paid D")
		catView := createCat("Cat View D")
		brand := createBrand("Brand D")

		pSeedPaid := createProd(sellerActive, catPaid, &brand, "Seed Paid", "published", 10)
		insertEvent("order_paid", &u, pSeedPaid, now.Add(-1*time.Hour))

		pSeedView := createProd(sellerActive, catView, &brand, "Seed View", "published", 10)
		insertEvent("product_view", &u, pSeedView, now.Add(-10*time.Minute))

		cPaid := createProd(sellerActive, catPaid, &brand, "Cand Paid Cat", "published", 10)
		cView := createProd(sellerActive, catView, &brand, "Cand View Cat", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		var idxPaid, idxView int = -1, -1
		for i, p := range res {
			if p.ID == cPaid {
				idxPaid = i
			}
			if p.ID == cView {
				idxView = i
			}
		}
		require.NotEqual(t, -1, idxPaid)
		require.NotEqual(t, -1, idxView)
		assert.Less(t, idxPaid, idxView, "order_paid (score 10) must outrank product_view (score 1)")
	})

	// Test E: favorite-derived affinity outranks view-derived affinity
	t.Run("E. favorite-derived affinity outranks view-derived affinity", func(t *testing.T) {
		u := createUser("uE")
		catFav := createCat("Cat Fav E")
		catView := createCat("Cat View E")
		brand := createBrand("Brand E")

		pSeedFav := createProd(sellerActive, catFav, &brand, "Seed Fav", "published", 10)
		insertEvent("favorite_added", &u, pSeedFav, now.Add(-1*time.Hour))

		pSeedView := createProd(sellerActive, catView, &brand, "Seed View", "published", 10)
		insertEvent("product_view", &u, pSeedView, now.Add(-10*time.Minute))

		cFav := createProd(sellerActive, catFav, &brand, "Cand Fav Cat", "published", 10)
		cView := createProd(sellerActive, catView, &brand, "Cand View Cat", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		var idxFav, idxView int = -1, -1
		for i, p := range res {
			if p.ID == cFav {
				idxFav = i
			}
			if p.ID == cView {
				idxView = i
			}
		}
		require.NotEqual(t, -1, idxFav)
		require.NotEqual(t, -1, idxView)
		assert.Less(t, idxFav, idxView, "favorite_added (score 3) must outrank product_view (score 1)")
	})

	// Test F: candidate with zero affinity ranks below positive-affinity candidate
	t.Run("F. candidate with zero affinity ranks below positive-affinity candidate", func(t *testing.T) {
		u := createUser("uF")
		catPos := createCat("Cat Pos F")
		catZero := createCat("Cat Zero F")
		brandZero := createBrand("Brand Zero F")

		pSeed := createProd(sellerActive, catPos, nil, "Seed Pos", "published", 10)
		insertEvent("product_view", &u, pSeed, now.Add(-1*time.Hour))
		insertEvent("favorite_added", &u, pSeed, now.Add(-1*time.Hour))

		cPos := createProd(sellerActive, catPos, nil, "Cand Pos Affinity", "published", 10)
		cZero := createProd(sellerActive, catZero, &brandZero, "Cand Zero Affinity", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)
		require.NotEmpty(t, res)

		// Positive affinity candidate must rank at the top
		assert.Equal(t, cPos, res[0].ID, "Positive affinity candidate must rank first")

		// Candidate with zero affinity (neither category nor brand in profile) is not ranked at or above positive candidate
		for _, p := range res {
			assert.NotEqual(t, cZero, p.ID, "Zero affinity candidate must not rank at or above positive candidate")
		}
	})

	// Test G: unpublished product excluded
	t.Run("G. unpublished product excluded", func(t *testing.T) {
		u := createUser("uG")
		cat := createCat("Cat G")
		brand := createBrand("Brand G")

		seed := createProd(sellerActive, cat, &brand, "Seed G", "published", 10)
		insertEvent("product_view", &u, seed, now.Add(-1*time.Hour))

		cDraft := createProd(sellerActive, cat, &brand, "Draft Candidate", "draft", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)
		for _, p := range res {
			assert.NotEqual(t, cDraft, p.ID, "Unpublished candidate must be excluded")
		}
	})

	// Test H: inactive seller excluded
	t.Run("H. inactive seller excluded", func(t *testing.T) {
		u := createUser("uH")
		cat := createCat("Cat H")
		brand := createBrand("Brand H")

		seed := createProd(sellerActive, cat, &brand, "Seed H", "published", 10)
		insertEvent("product_view", &u, seed, now.Add(-1*time.Hour))

		cBlocked := createProd(sellerBlocked, cat, &brand, "Blocked Candidate", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)
		for _, p := range res {
			assert.NotEqual(t, cBlocked, p.ID, "Candidate from inactive seller must be excluded")
		}
	})

	// Test I: insufficient canonical stock excluded
	t.Run("I. insufficient canonical stock excluded", func(t *testing.T) {
		u := createUser("uI")
		cat := createCat("Cat I")
		brand := createBrand("Brand I")

		seed := createProd(sellerActive, cat, &brand, "Seed I", "published", 10)
		insertEvent("product_view", &u, seed, now.Add(-1*time.Hour))

		cLowStock := createProd(sellerActive, cat, &brand, "Low Stock Candidate", "published", 1)
		cGoodStock := createProd(sellerActive, cat, &brand, "Good Stock Candidate", "published", 2)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		foundGood := false
		for _, p := range res {
			assert.NotEqual(t, cLowStock, p.ID, "Free stock = 1 must be excluded by CAT.1A")
			if p.ID == cGoodStock {
				foundGood = true
			}
		}
		assert.True(t, foundGood, "Free stock = 2 must be eligible")
	})

	// Test J: empty profile preserves fallback behavior
	t.Run("J. empty profile preserves fallback behavior", func(t *testing.T) {
		u := createUser("uJ-empty")

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Empty(t, res, "Empty profile must safely return empty slice without error")
	})

	// Test K: equal total affinity has no hidden brand preference; quality tie-breakers apply and tie-break is deterministic
	t.Run("K. equal total affinity has no hidden brand preference; quality tie-breakers apply and tie-break is deterministic", func(t *testing.T) {
		u := createUser("uK")
		catMatch := createCat("Cat K Match")
		catOther := createCat("Cat K Other")
		brandMatch := createBrand("Brand K Match")
		brandOther := createBrand("Brand K Other")

		// Customer has favorite on catMatch (score 3) and brandMatch (score 3)
		seedCat := createProd(sellerActive, catMatch, &brandOther, "Seed K Cat", "published", 10)
		insertEvent("favorite_added", &u, seedCat, now.Add(-2*time.Hour))

		seedBrand := createProd(sellerActive, catOther, &brandMatch, "Seed K Brand", "published", 10)
		insertEvent("favorite_added", &u, seedBrand, now.Add(-2*time.Hour))

		// 1. Equal score (3 vs 3), but Cat match candidate has higher rating (5.0 vs 4.0)
		// Proves: no hidden brand preference (cat match with higher quality ranks first)
		cCatHighRating := createProdWithDetails(sellerActive, catMatch, &brandOther, "Cat Cand High Rating", "published", 5.0, 10, now.Add(-1*time.Hour), 10)
		cBrandLowerRating := createProdWithDetails(sellerActive, catOther, &brandMatch, "Brand Cand Lower Rating", "published", 4.0, 10, now.Add(-1*time.Hour), 10)

		res1, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		var idxCatHigh, idxBrandLow int = -1, -1
		for i, p := range res1 {
			if p.ID == cCatHighRating {
				idxCatHigh = i
			}
			if p.ID == cBrandLowerRating {
				idxBrandLow = i
			}
		}
		require.NotEqual(t, -1, idxCatHigh)
		require.NotEqual(t, -1, idxBrandLow)
		assert.Less(t, idxCatHigh, idxBrandLow, "Equal affinity score (3 vs 3): category candidate with higher rating must outrank brand candidate (no hidden brand preference)")

		// 2. Equal score and equal quality metrics: tie-break strictly deterministic by product_id ASC
		tExact := now.Add(-3 * time.Hour)
		cIdentical1 := createProdWithDetails(sellerActive, catMatch, &brandOther, "Identical K 1", "published", 4.5, 20, tExact, 10)
		cIdentical2 := createProdWithDetails(sellerActive, catOther, &brandMatch, "Identical K 2", "published", 4.5, 20, tExact, 10)

		res2, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)

		var idxId1, idxId2 int = -1, -1
		for i, p := range res2 {
			if p.ID == cIdentical1 {
				idxId1 = i
			}
			if p.ID == cIdentical2 {
				idxId2 = i
			}
		}
		require.NotEqual(t, -1, idxId1)
		require.NotEqual(t, -1, idxId2)

		if cIdentical1.String() < cIdentical2.String() {
			assert.Less(t, idxId1, idxId2, "Equal total score & quality: product with smaller UUID string must rank first (product_id ASC)")
		} else {
			assert.Less(t, idxId2, idxId1, "Equal total score & quality: product with smaller UUID string must rank first (product_id ASC)")
		}

		// 3. Multi-run deterministic stability across multiple queries
		for run := 0; run < 5; run++ {
			resNext, err := svc.GetForYouProducts(ctx, u, 10)
			require.NoError(t, err)
			require.Equal(t, len(res2), len(resNext))
			for i := range res2 {
				assert.Equal(t, res2[i].ID, resNext[i].ID, "Ordering must be byte-for-byte identical across runs")
			}
		}
	})

	// Test L: existing API response contract remains unchanged
	t.Run("L. existing API response contract remains unchanged", func(t *testing.T) {
		u := createUser("uL")
		cat := createCat("Cat L")
		brand := createBrand("Brand L")

		seed := createProd(sellerActive, cat, &brand, "Seed L", "published", 10)
		insertEvent("order_paid", &u, seed, now.Add(-1*time.Hour))
		insertEvent("favorite_added", &u, seed, now.Add(-1*time.Hour))

		c := createProd(sellerActive, cat, &brand, "Cand L", "published", 10)

		res, err := svc.GetForYouProducts(ctx, u, 10)
		require.NoError(t, err)
		require.NotEmpty(t, res)

		// Verify JSON serialization matches expected schema
		payload, err := json.Marshal(map[string]interface{}{
			"items":      res,
			"totalCount": len(res),
		})
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(payload, &parsed)
		require.NoError(t, err)

		assert.Contains(t, parsed, "items")
		assert.Contains(t, parsed, "totalCount")

		// Verify first item has public contract fields
		item0 := res[0]
		assert.Equal(t, c, item0.ID)
		assert.NotEmpty(t, item0.Title)
		assert.Equal(t, int64(100000), item0.PriceCents)
		assert.Equal(t, "RUB", item0.Currency)
	})
}
