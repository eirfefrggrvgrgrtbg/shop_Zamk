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

func TestRecentlyViewed_BehavioralEvents(t *testing.T) {
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

	// Fixtures
	userA := uuid.New()
	userB := uuid.New()
	sellerActive := uuid.New()
	sellerBlocked := uuid.New()
	catID := uuid.New()

	for _, u := range []uuid.UUID{userA, userB} {
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Customer', 'hash', 'customer', 'active', $4, $4)
		`, u, u.String()+"@test.local", "+7999"+u.String()[:7], now)
		require.NoError(t, err)
	}

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Active Brand', $2, $3, 'active', $4, $4)
	`, sellerActive, "act-brand-"+sellerActive.String()[:8], "act-"+sellerActive.String()[:8]+"@test.local", now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Blocked Brand', $2, $3, 'blocked', $4, $4)
	`, sellerBlocked, "blk-brand-"+sellerBlocked.String()[:8], "blk-"+sellerBlocked.String()[:8]+"@test.local", now)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Test Category', $2, true, $3, $3)
	`, catID, "cat-"+catID.String()[:8], now)
	require.NoError(t, err)

	createdProducts := make([]uuid.UUID, 0)
	createdEvents := make([]uuid.UUID, 0)

	defer func() {
		if len(createdEvents) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM behavioral_events WHERE id = ANY($1)", createdEvents)
		}
		if len(createdProducts) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id = ANY($1)", createdProducts)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = ANY($1)", createdProducts)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProducts)
		}
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id IN ($1, $2)", sellerActive, sellerBlocked)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", catID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", userA, userB)
	}()

	createProduct := func(seller uuid.UUID, title, status string, stock int) uuid.UUID {
		pID := uuid.New()
		vID := uuid.New()
		createdProducts = append(createdProducts, pID)

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				submitted_at, approved_at, published_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, 100000, 'RUB', $6, $7, $7, $7, $7, $7)
		`, pID, seller, catID, title, "slug-"+pID.String()[:8], status, now)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, 100000, true, $5, $5)
		`, vID, pID, "SKU-"+vID.String()[:8], "BC-"+vID.String()[:8], now)
		require.NoError(t, err)

		if stock > 0 {
			_, err = pgClient.Pool.Exec(ctx, `
				INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 0, $6, $6)
			`, uuid.New(), pID, vID, seller, stock, now)
			require.NoError(t, err)
		}
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

	// Products
	prod1 := createProduct(sellerActive, "Product 1", "published", 10)
	prod2 := createProduct(sellerActive, "Product 2", "published", 10)
	prod3 := createProduct(sellerActive, "Product 3", "published", 10)

	t.Run("A. Two product_view events of different products -> both returned", func(t *testing.T) {
		t1 := now.Add(-10 * time.Minute)
		t2 := now.Add(-5 * time.Minute)

		insertEvent("product_view", &userA, prod1, t1)
		insertEvent("product_view", &userA, prod2, t2)

		items, err := svc.GetRecentlyViewedProducts(ctx, userA, 10)
		require.NoError(t, err)
		require.Len(t, items, 2)

		var ids []uuid.UUID
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		assert.Contains(t, ids, prod1)
		assert.Contains(t, ids, prod2)
	})

	t.Run("B. Single product viewed multiple times -> single result with MAX(occurred_at)", func(t *testing.T) {
		// prod1 viewed again at newer time
		t3 := now.Add(-1 * time.Minute)
		insertEvent("product_view", &userA, prod1, t3)

		items, err := svc.GetRecentlyViewedProducts(ctx, userA, 10)
		require.NoError(t, err)

		// prod1 should appear exactly once
		var prod1Count int
		for _, it := range items {
			if it.ID == prod1 {
				prod1Count++
			}
		}
		assert.Equal(t, 1, prod1Count, "product viewed multiple times must be deduplicated to exactly one item")
	})

	t.Run("C. Ordering newest first", func(t *testing.T) {
		// prod1 latest view: now - 1 min
		// prod2 latest view: now - 5 min
		// prod3 viewed at now - 30 sec (newest)
		tNewest := now.Add(-30 * time.Second)
		insertEvent("product_view", &userA, prod3, tNewest)

		items, err := svc.GetRecentlyViewedProducts(ctx, userA, 10)
		require.NoError(t, err)
		require.Len(t, items, 3)

		assert.Equal(t, prod3, items[0].ID, "newest view should be first")
		assert.Equal(t, prod1, items[1].ID, "second newest should be second")
		assert.Equal(t, prod2, items[2].ID, "oldest should be third")
	})

	t.Run("D. Event of another user -> does not leak into user history", func(t *testing.T) {
		prodOther := createProduct(sellerActive, "Product Other User", "published", 10)
		insertEvent("product_view", &userB, prodOther, now.Add(-10*time.Second))

		itemsA, err := svc.GetRecentlyViewedProducts(ctx, userA, 10)
		require.NoError(t, err)

		for _, it := range itemsA {
			assert.NotEqual(t, prodOther, it.ID, "userB events must never appear in userA recently viewed")
		}

		itemsB, err := svc.GetRecentlyViewedProducts(ctx, userB, 10)
		require.NoError(t, err)
		require.Len(t, itemsB, 1)
		assert.Equal(t, prodOther, itemsB[0].ID)
	})

	t.Run("E. Anonymous event (user_id IS NULL) -> does not appear for authenticated user", func(t *testing.T) {
		prodAnon := createProduct(sellerActive, "Product Anonymous", "published", 10)
		insertEvent("product_view", nil, prodAnon, now.Add(-5*time.Second))

		itemsA, err := svc.GetRecentlyViewedProducts(ctx, userA, 10)
		require.NoError(t, err)

		for _, it := range itemsA {
			assert.NotEqual(t, prodAnon, it.ID, "anonymous events must not appear in authenticated user history")
		}
	})

	t.Run("F. Events of different types (favorite_added, add_to_cart, order_paid) -> not in Recently Viewed", func(t *testing.T) {
		prodFav := createProduct(sellerActive, "Product Fav", "published", 10)
		prodCart := createProduct(sellerActive, "Product Cart", "published", 10)
		prodPaid := createProduct(sellerActive, "Product Paid", "published", 10)

		insertEvent("favorite_added", &userA, prodFav, now.Add(-2*time.Second))
		insertEvent("add_to_cart", &userA, prodCart, now.Add(-2*time.Second))
		insertEvent("order_paid", &userA, prodPaid, now.Add(-2*time.Second))

		itemsA, err := svc.GetRecentlyViewedProducts(ctx, userA, 20)
		require.NoError(t, err)

		for _, it := range itemsA {
			assert.NotEqual(t, prodFav, it.ID, "favorite_added must not count as product_view")
			assert.NotEqual(t, prodCart, it.ID, "add_to_cart must not count as product_view")
			assert.NotEqual(t, prodPaid, it.ID, "order_paid must not count as product_view")
		}
	})

	t.Run("G. Product viewed older than 30 days -> excluded", func(t *testing.T) {
		prodOld := createProduct(sellerActive, "Product Old View", "published", 10)
		t31DaysAgo := now.Add(-31 * 24 * time.Hour)
		insertEvent("product_view", &userA, prodOld, t31DaysAgo)

		itemsA, err := svc.GetRecentlyViewedProducts(ctx, userA, 20)
		require.NoError(t, err)

		for _, it := range itemsA {
			assert.NotEqual(t, prodOld, it.ID, "views older than 30 days must be excluded")
		}
	})

	t.Run("H. Unpublished product -> excluded", func(t *testing.T) {
		prodDraft := createProduct(sellerActive, "Product Draft", "draft", 10)
		insertEvent("product_view", &userA, prodDraft, now.Add(-1*time.Minute))

		itemsA, err := svc.GetRecentlyViewedProducts(ctx, userA, 20)
		require.NoError(t, err)

		for _, it := range itemsA {
			assert.NotEqual(t, prodDraft, it.ID, "unpublished products must be excluded")
		}
	})

	t.Run("I. Inactive seller -> excluded", func(t *testing.T) {
		prodBlockedSeller := createProduct(sellerBlocked, "Product Blocked Seller", "published", 10)
		insertEvent("product_view", &userA, prodBlockedSeller, now.Add(-1*time.Minute))

		itemsA, err := svc.GetRecentlyViewedProducts(ctx, userA, 20)
		require.NoError(t, err)

		for _, it := range itemsA {
			assert.NotEqual(t, prodBlockedSeller, it.ID, "products from inactive sellers must be excluded")
		}
	})

	t.Run("J. Unavailable product / insufficient canonical stock (free stock < 2) -> excluded", func(t *testing.T) {
		prodLowStock := createProduct(sellerActive, "Product Low Stock", "published", 1)
		prodNoStock := createProduct(sellerActive, "Product No Stock", "published", 0)

		insertEvent("product_view", &userA, prodLowStock, now.Add(-1*time.Minute))
		insertEvent("product_view", &userA, prodNoStock, now.Add(-1*time.Minute))

		itemsA, err := svc.GetRecentlyViewedProducts(ctx, userA, 20)
		require.NoError(t, err)

		for _, it := range itemsA {
			assert.NotEqual(t, prodLowStock, it.ID, "product with stock < 2 must be excluded under CAT.1A")
			assert.NotEqual(t, prodNoStock, it.ID, "product with 0 stock must be excluded")
		}
	})

	t.Run("K. Deterministic tie ordering", func(t *testing.T) {
		uTie := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Tie Customer', 'hash', 'customer', 'active', $4, $4)
		`, uTie, uTie.String()+"@test.local", "+7999"+uTie.String()[:7], now)
		require.NoError(t, err)
		defer func() {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", uTie)
		}()

		pA := createProduct(sellerActive, "Product Tie A", "published", 10)
		pB := createProduct(sellerActive, "Product Tie B", "published", 10)
		pC := createProduct(sellerActive, "Product Tie C", "published", 10)

		exactTime := now.Add(-2 * time.Hour)
		insertEvent("product_view", &uTie, pA, exactTime)
		insertEvent("product_view", &uTie, pB, exactTime)
		insertEvent("product_view", &uTie, pC, exactTime)

		res1, err := svc.GetRecentlyViewedProducts(ctx, uTie, 10)
		require.NoError(t, err)
		require.Len(t, res1, 3)

		// Expected tie break order: p.id ASC
		expectedOrder := []uuid.UUID{pA, pB, pC}
		// Sort expectedOrder by UUID string / byte value
		for i := 0; i < len(expectedOrder)-1; i++ {
			for j := i + 1; j < len(expectedOrder); j++ {
				if expectedOrder[i].String() > expectedOrder[j].String() {
					expectedOrder[i], expectedOrder[j] = expectedOrder[j], expectedOrder[i]
				}
			}
		}

		for i := 0; i < 3; i++ {
			assert.Equal(t, expectedOrder[i], res1[i].ID, fmt.Sprintf("tie break index %d must match p.id ASC", i))
		}

		// Verify determinism across 5 runs
		for run := 0; run < 5; run++ {
			resNext, err := svc.GetRecentlyViewedProducts(ctx, uTie, 10)
			require.NoError(t, err)
			require.Len(t, resNext, 3)
			for i := 0; i < 3; i++ {
				assert.Equal(t, res1[i].ID, resNext[i].ID, "tie ordering must be completely deterministic")
			}
		}
	})
}
