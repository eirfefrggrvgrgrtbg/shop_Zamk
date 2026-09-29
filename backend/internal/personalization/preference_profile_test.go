package personalization_test

import (
	"context"
	"fmt"
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

func TestPreferenceProfile_CanonicalSignals(t *testing.T) {
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
			VALUES ($1, $2, $3, 'Profile Test User', 'hash', 'customer', 'active', $4, $4)
		`, id, prefix+"-"+id.String()[:8]+"@test.local", "+7999"+id.String()[:7], now)
		require.NoError(t, err)
		createdUsers = append(createdUsers, id)
		return id
	}

	createSeller := func(name string) uuid.UUID {
		id := uuid.New()
		slug := "sel-" + id.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'active', $5, $5)
		`, id, name, slug, slug+"@test.local", now)
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

	createProd := func(sellerID, catID uuid.UUID, brandID *uuid.UUID, title string) uuid.UUID {
		pID := uuid.New()
		vID := uuid.New()
		createdProds = append(createdProds, pID)

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status,
				submitted_at, approved_at, published_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 100000, 'RUB', 'published', $7, $7, $7, $7, $7)
		`, pID, sellerID, catID, brandID, title, "slug-"+pID.String()[:8], now)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, 100000, true, $5, $5)
		`, vID, pID, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8], now)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 10, 0, $5, $5)
		`, uuid.New(), pID, vID, sellerID, now)
		require.NoError(t, err)

		return pID
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

	insertLegacyFavorite := func(userID, productID uuid.UUID, createdAt time.Time) {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO customer_favorites (id, user_id, product_id, created_at)
			VALUES ($1, $2, $3, $4)
		`, uuid.New(), userID, productID, createdAt)
		require.NoError(t, err)
	}

	seller := createSeller("Default Seller")

	t.Run("1. view creates category and brand affinity with score = 1", func(t *testing.T) {
		u := createUser("u1")
		cat := createCat("Cat 1")
		brand := createBrand("Brand 1")
		p := createProd(seller, cat, &brand, "Prod 1")

		insertEvent("product_view", &u, p, now.Add(-5*time.Minute))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		require.Len(t, profile.Categories, 1)
		assert.Equal(t, cat, profile.Categories[0].CategoryID)
		assert.Equal(t, int64(1), profile.Categories[0].Score)
		assert.Equal(t, int64(1), profile.Categories[0].DistinctProductCount)

		require.Len(t, profile.Brands, 1)
		assert.Equal(t, brand, profile.Brands[0].BrandID)
		assert.Equal(t, int64(1), profile.Brands[0].Score)
		assert.Equal(t, int64(1), profile.Brands[0].DistinctProductCount)
	})

	t.Run("2. multiple views sum up", func(t *testing.T) {
		u := createUser("u2")
		cat := createCat("Cat 2")
		brand := createBrand("Brand 2")
		p1 := createProd(seller, cat, &brand, "Prod 2A")
		p2 := createProd(seller, cat, &brand, "Prod 2B")

		insertEvent("product_view", &u, p1, now.Add(-10*time.Minute))
		insertEvent("product_view", &u, p1, now.Add(-8*time.Minute))
		insertEvent("product_view", &u, p2, now.Add(-5*time.Minute))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		require.Len(t, profile.Categories, 1)

		// 3 views -> score = 3, distinct products = 2
		assert.Equal(t, cat, profile.Categories[0].CategoryID)
		assert.Equal(t, int64(3), profile.Categories[0].Score)
		assert.Equal(t, int64(2), profile.Categories[0].DistinctProductCount)
	})

	t.Run("3. favorite weighs stronger than view", func(t *testing.T) {
		u := createUser("u3")
		catView := createCat("Cat View")
		brandView := createBrand("Brand View")
		pView := createProd(seller, catView, &brandView, "Prod View")

		catFav := createCat("Cat Fav")
		brandFav := createBrand("Brand Fav")
		pFav := createProd(seller, catFav, &brandFav, "Prod Fav")

		// 1 view (weight 1)
		insertEvent("product_view", &u, pView, now.Add(-1*time.Hour))
		// 1 active favorite (weight 3)
		insertEvent("favorite_added", &u, pFav, now.Add(-2*time.Hour))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		require.Len(t, profile.Categories, 2)

		// Fav has score 3 > View score 1 -> Fav category must rank first
		assert.Equal(t, catFav, profile.Categories[0].CategoryID)
		assert.Equal(t, int64(personalization.WeightFavorite), profile.Categories[0].Score)

		assert.Equal(t, catView, profile.Categories[1].CategoryID)
		assert.Equal(t, int64(personalization.WeightProductView), profile.Categories[1].Score)
	})

	t.Run("4. removed favorite is not counted", func(t *testing.T) {
		u := createUser("u4")
		cat := createCat("Cat Rem Fav")
		brand := createBrand("Brand Rem Fav")
		p := createProd(seller, cat, &brand, "Prod Rem Fav")

		// Added, then later removed
		tAdded := now.Add(-2 * time.Hour)
		tRemoved := now.Add(-1 * time.Hour)
		insertEvent("favorite_added", &u, p, tAdded)
		insertEvent("favorite_removed", &u, p, tRemoved)

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)

		assert.Empty(t, profile.Categories, "removed favorite must produce no active affinity")
		assert.Empty(t, profile.Brands)
		assert.Empty(t, profile.FavoriteCategories)
	})

	t.Run("5. order_paid weighs stronger than favorite", func(t *testing.T) {
		u := createUser("u5")
		catFav := createCat("Cat Fav Only")
		brandFav := createBrand("Brand Fav Only")
		pFav := createProd(seller, catFav, &brandFav, "Prod Fav")

		catPaid := createCat("Cat Paid Only")
		brandPaid := createBrand("Brand Paid Only")
		pPaid := createProd(seller, catPaid, &brandPaid, "Prod Paid")

		// 1 favorite (weight 3)
		insertEvent("favorite_added", &u, pFav, now.Add(-10*time.Minute))
		// 1 order_paid (weight 10)
		insertEvent("order_paid", &u, pPaid, now.Add(-2*time.Hour))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		require.Len(t, profile.Categories, 2)

		// order_paid (score 10) > favorite (score 3) -> Paid category must rank first
		assert.Equal(t, catPaid, profile.Categories[0].CategoryID)
		assert.Equal(t, int64(personalization.WeightOrderPaid), profile.Categories[0].Score)

		assert.Equal(t, catFav, profile.Categories[1].CategoryID)
		assert.Equal(t, int64(personalization.WeightFavorite), profile.Categories[1].Score)
	})

	t.Run("6. other user events do not affect current user profile", func(t *testing.T) {
		uA := createUser("u6A")
		uB := createUser("u6B")
		catB := createCat("Cat B Only")
		brandB := createBrand("Brand B Only")
		pB := createProd(seller, catB, &brandB, "Prod B Only")

		insertEvent("order_paid", &uB, pB, now.Add(-10*time.Minute))
		insertEvent("favorite_added", &uB, pB, now.Add(-10*time.Minute))

		profileA, err := svc.GetCustomerPreferenceProfile(ctx, uA, 5)
		require.NoError(t, err)
		assert.Empty(t, profileA.Categories)
		assert.Empty(t, profileA.Brands)
	})

	t.Run("7. anonymous events do not affect authenticated user profile", func(t *testing.T) {
		u := createUser("u7")
		catAnon := createCat("Cat Anon")
		brandAnon := createBrand("Brand Anon")
		pAnon := createProd(seller, catAnon, &brandAnon, "Prod Anon")

		// Insert event with user_id = NULL
		insertEvent("product_view", nil, pAnon, now.Add(-5*time.Minute))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		assert.Empty(t, profile.Categories)
		assert.Empty(t, profile.Brands)
	})

	t.Run("8. product_view older than 30 days lookback is excluded", func(t *testing.T) {
		u := createUser("u8")
		catOld := createCat("Cat Old View")
		brandOld := createBrand("Brand Old View")
		pOld := createProd(seller, catOld, &brandOld, "Prod Old View")

		insertEvent("product_view", &u, pOld, now.Add(-31*24*time.Hour))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		assert.Empty(t, profile.Categories)
		assert.Empty(t, profile.Brands)
		assert.Empty(t, profile.ViewedCategories)
	})

	t.Run("9. order_paid older than 180 days lookback is excluded", func(t *testing.T) {
		u := createUser("u9")
		catOldPaid := createCat("Cat Old Paid")
		brandOldPaid := createBrand("Brand Old Paid")
		pOldPaid := createProd(seller, catOldPaid, &brandOldPaid, "Prod Old Paid")

		insertEvent("order_paid", &u, pOldPaid, now.Add(-181*24*time.Hour))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		assert.Empty(t, profile.Categories)
		assert.Empty(t, profile.Brands)
	})

	t.Run("10. deterministic tie ordering on equal score and timestamp", func(t *testing.T) {
		u := createUser("u10")
		catA := createCat("Cat Tie A")
		catB := createCat("Cat Tie B")
		catC := createCat("Cat Tie C")

		brand := createBrand("Brand Tie Common")

		pA := createProd(seller, catA, &brand, "Prod Tie A")
		pB := createProd(seller, catB, &brand, "Prod Tie B")
		pC := createProd(seller, catC, &brand, "Prod Tie C")

		exactTime := now.Add(-1 * time.Hour)
		insertEvent("product_view", &u, pA, exactTime)
		insertEvent("product_view", &u, pB, exactTime)
		insertEvent("product_view", &u, pC, exactTime)

		res1, err := svc.GetCustomerPreferenceProfile(ctx, u, 10)
		require.NoError(t, err)
		require.Len(t, res1.Categories, 3)

		// Expected tie breaker: category_id ASC
		expectedCats := []uuid.UUID{catA, catB, catC}
		for i := 0; i < len(expectedCats)-1; i++ {
			for j := i + 1; j < len(expectedCats); j++ {
				if expectedCats[i].String() > expectedCats[j].String() {
					expectedCats[i], expectedCats[j] = expectedCats[j], expectedCats[i]
				}
			}
		}

		for i := 0; i < 3; i++ {
			assert.Equal(t, expectedCats[i], res1.Categories[i].CategoryID, fmt.Sprintf("tie break index %d must match category_id ASC", i))
		}

		// Verify determinism across 5 runs
		for run := 0; run < 5; run++ {
			resNext, err := svc.GetCustomerPreferenceProfile(ctx, u, 10)
			require.NoError(t, err)
			require.Len(t, resNext.Categories, 3)
			for i := 0; i < 3; i++ {
				assert.Equal(t, res1.Categories[i].CategoryID, resNext.Categories[i].CategoryID, "ordering must be completely deterministic")
			}
		}
	})

	t.Run("11. Scenario A: customer_favorites row + favorite_added event => score contribution = 3, not 6", func(t *testing.T) {
		u := createUser("u11-fav-dedup")
		cat := createCat("Cat Fav Dedup")
		brand := createBrand("Brand Fav Dedup")
		p := createProd(seller, cat, &brand, "Prod Fav Dedup")

		// Both legacy customer_favorites row AND behavioral favorite_added event exist (e.g. dual-write or migration state)
		insertLegacyFavorite(u, p, now.Add(-3*time.Hour))
		insertEvent("favorite_added", &u, p, now.Add(-2*time.Hour))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		// Must have exactly 1 category and 1 brand affinity with score = 3 (NOT 6)
		require.Len(t, profile.Categories, 1)
		assert.Equal(t, cat, profile.Categories[0].CategoryID)
		assert.Equal(t, int64(personalization.WeightFavorite), profile.Categories[0].Score, "score must be 3, not 6")
		assert.Equal(t, int64(1), profile.Categories[0].DistinctProductCount)

		require.Len(t, profile.Brands, 1)
		assert.Equal(t, brand, profile.Brands[0].BrandID)
		assert.Equal(t, int64(personalization.WeightFavorite), profile.Brands[0].Score, "score must be 3, not 6")

		// Legacy favorite categories must also have exactly 1 entry
		require.Len(t, profile.FavoriteCategories, 1)
		assert.Equal(t, cat, profile.FavoriteCategories[0].CategoryID)
		assert.Equal(t, int64(1), profile.FavoriteCategories[0].DistinctProductCount)
	})

	t.Run("12. Scenario B: customer_favorites row + later favorite_removed event => score contribution = 0", func(t *testing.T) {
		u := createUser("u12-fav-removed")
		cat := createCat("Cat Fav Removed")
		brand := createBrand("Brand Fav Removed")
		p := createProd(seller, cat, &brand, "Prod Fav Removed")

		// Legacy customer_favorites row exists, but a later favorite_removed event occurred
		insertLegacyFavorite(u, p, now.Add(-3*time.Hour))
		insertEvent("favorite_removed", &u, p, now.Add(-1*time.Hour))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		// Must be empty: removed favorite overrides legacy row
		assert.Empty(t, profile.Categories, "favorite_removed event must cancel out legacy customer_favorites row")
		assert.Empty(t, profile.Brands)
		assert.Empty(t, profile.FavoriteCategories)
		assert.Empty(t, profile.FavoriteBrands)
	})

	t.Run("13. Scenario C: legacy customer_favorites row + no behavioral favorite events => contribution = 3", func(t *testing.T) {
		u := createUser("u13-legacy-fav")
		cat := createCat("Cat Legacy Fav")
		brand := createBrand("Brand Legacy Fav")
		p := createProd(seller, cat, &brand, "Prod Legacy Fav")

		// Only customer_favorites row, no behavioral events
		insertLegacyFavorite(u, p, now.Add(-10*time.Hour))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 5)
		require.NoError(t, err)
		require.NotNil(t, profile)

		// Must produce score = 3
		require.Len(t, profile.Categories, 1)
		assert.Equal(t, cat, profile.Categories[0].CategoryID)
		assert.Equal(t, int64(personalization.WeightFavorite), profile.Categories[0].Score)
		assert.Equal(t, int64(1), profile.Categories[0].DistinctProductCount)

		require.Len(t, profile.Brands, 1)
		assert.Equal(t, brand, profile.Brands[0].BrandID)
		assert.Equal(t, int64(personalization.WeightFavorite), profile.Brands[0].Score)

		require.Len(t, profile.FavoriteCategories, 1)
		assert.Equal(t, cat, profile.FavoriteCategories[0].CategoryID)
		require.Len(t, profile.FavoriteBrands, 1)
		assert.Equal(t, brand, profile.FavoriteBrands[0].BrandID)
	})

	t.Run("14. legacy profile isolation: order_paid never enters legacy arrays", func(t *testing.T) {
		u := createUser("u14-legacy-isolation")
		catPaid := createCat("Cat Paid Only")
		brandPaid := createBrand("Brand Paid Only")
		pPaid := createProd(seller, catPaid, &brandPaid, "Prod Paid Only")

		catView := createCat("Cat View Only")
		brandView := createBrand("Brand View Only")
		pView := createProd(seller, catView, &brandView, "Prod View Only")

		catFav := createCat("Cat Fav Only")
		brandFav := createBrand("Brand Fav Only")
		pFav := createProd(seller, catFav, &brandFav, "Prod Fav Only")

		// Add view, favorite, and paid
		insertEvent("product_view", &u, pView, now.Add(-2*time.Hour))
		insertEvent("favorite_added", &u, pFav, now.Add(-1*time.Hour))
		insertEvent("order_paid", &u, pPaid, now.Add(-30*time.Minute))

		profile, err := svc.GetCustomerPreferenceProfile(ctx, u, 10)
		require.NoError(t, err)
		require.NotNil(t, profile)

		// Weighted Categories profile contains all 3: paid (10) > fav (3) > view (1)
		require.Len(t, profile.Categories, 3)
		assert.Equal(t, catPaid, profile.Categories[0].CategoryID)
		assert.Equal(t, int64(10), profile.Categories[0].Score)
		assert.Equal(t, catFav, profile.Categories[1].CategoryID)
		assert.Equal(t, int64(3), profile.Categories[1].Score)
		assert.Equal(t, catView, profile.Categories[2].CategoryID)
		assert.Equal(t, int64(1), profile.Categories[2].Score)

		// Legacy ViewedCategories contains ONLY catView (NEVER catPaid or catFav)
		require.Len(t, profile.ViewedCategories, 1)
		assert.Equal(t, catView, profile.ViewedCategories[0].CategoryID)
		require.Len(t, profile.ViewedBrands, 1)
		assert.Equal(t, brandView, profile.ViewedBrands[0].BrandID)

		// Legacy FavoriteCategories contains ONLY catFav (NEVER catPaid or catView)
		require.Len(t, profile.FavoriteCategories, 1)
		assert.Equal(t, catFav, profile.FavoriteCategories[0].CategoryID)
		require.Len(t, profile.FavoriteBrands, 1)
		assert.Equal(t, brandFav, profile.FavoriteBrands[0].BrandID)
	})
}
