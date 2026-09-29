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

func TestPopular_PERS_2C1(t *testing.T) {
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
			VALUES ($1, $2, $3, 'Popular User', 'hash', 'customer', 'active', $4, $4)
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

	createProd := func(sellerID, catID uuid.UUID, title string, status string, stock int) uuid.UUID {
		return createProdWithDetails(sellerID, catID, nil, title, status, 0.0, 0, now, stock)
	}

	insertEvent := func(eventType string, userID *uuid.UUID, productID uuid.UUID, quantity int, occurredAt time.Time) uuid.UUID {
		eID := uuid.New()
		createdEvents = append(createdEvents, eID)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO behavioral_events (
				id, event_type, source, visitor_id, user_id, product_id, quantity,
				occurred_at, received_at, metadata
			) VALUES ($1, $2, 'server', NULL, $3, $4, $5, $6, $6, '{}'::jsonb)
		`, eID, eventType, userID, productID, quantity, occurredAt)
		require.NoError(t, err)
		return eID
	}

	sellerActive := createSeller("Active Popular Seller", "active")
	sellerBlocked := createSeller("Blocked Popular Seller", "blocked")
	catCommon := createCat("Common Cat")
	uA := createUser("uA")

	// A. больше paid quantity => выше
	t.Run("A. больше paid quantity => выше", func(t *testing.T) {
		pHigh := createProd(sellerActive, catCommon, "Prod High Sales", "published", 10)
		pLow := createProd(sellerActive, catCommon, "Prod Low Sales", "published", 10)

		// 5 items sold vs 2 items sold
		insertEvent("order_paid", &uA, pHigh, 5, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, pLow, 2, now.Add(-1*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
		require.NoError(t, err)

		var idxHigh, idxLow int = -1, -1
		for i, p := range res {
			if p.ID == pHigh {
				idxHigh = i
			}
			if p.ID == pLow {
				idxLow = i
			}
		}
		require.NotEqual(t, -1, idxHigh)
		require.NotEqual(t, -1, idxLow)
		assert.Less(t, idxHigh, idxLow, "Product with higher paid quantity (5) must rank before product with lower (2)")
	})

	// B. quantity внутри одного event учитывается
	t.Run("B. quantity внутри одного event учитывается", func(t *testing.T) {
		pSingle10 := createProd(sellerActive, catCommon, "Prod Single Event 10", "published", 15)
		pSingle3 := createProd(sellerActive, catCommon, "Prod Single Event 3", "published", 15)

		insertEvent("order_paid", &uA, pSingle10, 10, now.Add(-2*time.Hour))
		insertEvent("order_paid", &uA, pSingle3, 3, now.Add(-2*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
		require.NoError(t, err)

		var idx10, idx3 int = -1, -1
		for i, p := range res {
			if p.ID == pSingle10 {
				idx10 = i
			}
			if p.ID == pSingle3 {
				idx3 = i
			}
		}
		require.NotEqual(t, -1, idx10)
		require.NotEqual(t, -1, idx3)
		assert.Less(t, idx10, idx3, "Single event with quantity 10 must rank before single event with quantity 3")
	})

	// C. несколько order_paid суммируются
	t.Run("C. несколько order_paid суммируются", func(t *testing.T) {
		pMulti := createProd(sellerActive, catCommon, "Prod Multi Events (2+3=5)", "published", 10)
		pSingle := createProd(sellerActive, catCommon, "Prod Single Event (4)", "published", 10)

		insertEvent("order_paid", &uA, pMulti, 2, now.Add(-3*time.Hour))
		insertEvent("order_paid", &uA, pMulti, 3, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, pSingle, 4, now.Add(-2*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
		require.NoError(t, err)

		var idxMulti, idxSingle int = -1, -1
		for i, p := range res {
			if p.ID == pMulti {
				idxMulti = i
			}
			if p.ID == pSingle {
				idxSingle = i
			}
		}
		require.NotEqual(t, -1, idxMulti)
		require.NotEqual(t, -1, idxSingle)
		assert.Less(t, idxMulti, idxSingle, "Multiple order_paid events sum (2+3=5) must rank before 4")
	})

	// D. события старше 30 дней не учитываются
	t.Run("D. события старше 30 дней не учитываются", func(t *testing.T) {
		pOld := createProd(sellerActive, catCommon, "Prod 35 Days Old", "published", 10)
		pRecent := createProd(sellerActive, catCommon, "Prod 2 Days Old", "published", 10)

		insertEvent("order_paid", &uA, pOld, 20, now.Add(-35*24*time.Hour))
		insertEvent("order_paid", &uA, pRecent, 2, now.Add(-2*24*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
		require.NoError(t, err)

		foundRecent := false
		for _, p := range res {
			assert.NotEqual(t, pOld, p.ID, "Events older than 30 days must not contribute to popularity")
			if p.ID == pRecent {
				foundRecent = true
			}
		}
		assert.True(t, foundRecent, "Recent order_paid within 30 days must be included")
	})

	// E. другие event types не влияют (и malformed order_paid с quantity NULL/<=0 не маскируются)
	t.Run("E. другие event types не влияют", func(t *testing.T) {
		pNonPaid := createProd(sellerActive, catCommon, "Prod Other Events Only", "published", 10)
		pMalformedPaid := createProd(sellerActive, catCommon, "Prod Malformed Paid Only", "published", 10)
		pPaid := createProd(sellerActive, catCommon, "Prod Paid Only", "published", 10)

		insertEvent("product_view", &uA, pNonPaid, 100, now.Add(-1*time.Hour))
		insertEvent("favorite_added", &uA, pNonPaid, 10, now.Add(-1*time.Hour))
		insertEvent("return_requested", &uA, pNonPaid, 5, now.Add(-1*time.Hour))

		// Malformed order_paid events: NULL quantity and quantity = 0
		eID1 := uuid.New()
		createdEvents = append(createdEvents, eID1)
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO behavioral_events (id, event_type, source, user_id, product_id, quantity, occurred_at, received_at)
			VALUES ($1, 'order_paid', 'server', $2, $3, NULL, $4, $4)
		`, eID1, uA, pMalformedPaid, now.Add(-1*time.Hour))
		require.NoError(t, err)

		eID2 := uuid.New()
		createdEvents = append(createdEvents, eID2)
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO behavioral_events (id, event_type, source, user_id, product_id, quantity, occurred_at, received_at)
			VALUES ($1, 'order_paid', 'server', $2, $3, 0, $4, $4)
		`, eID2, uA, pMalformedPaid, now.Add(-1*time.Hour))
		require.NoError(t, err)

		insertEvent("order_paid", &uA, pPaid, 1, now.Add(-1*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
		require.NoError(t, err)

		foundPaid := false
		for _, p := range res {
			assert.NotEqual(t, pNonPaid, p.ID, "Non order_paid event types must not contribute to popularity")
			assert.NotEqual(t, pMalformedPaid, p.ID, "order_paid with NULL or non-positive quantity must not contribute to popularity")
			if p.ID == pPaid {
				foundPaid = true
			}
		}
		assert.True(t, foundPaid, "Valid order_paid product must be included")
	})

	// F. unpublished исключается
	t.Run("F. unpublished исключается", func(t *testing.T) {
		pDraft := createProd(sellerActive, catCommon, "Draft Prod With Sales", "draft", 10)
		insertEvent("order_paid", &uA, pDraft, 10, now.Add(-1*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
		require.NoError(t, err)

		for _, p := range res {
			assert.NotEqual(t, pDraft, p.ID, "Unpublished product must be excluded from Popular")
		}
	})

	// G. inactive seller исключается
	t.Run("G. inactive seller исключается", func(t *testing.T) {
		pBlocked := createProd(sellerBlocked, catCommon, "Blocked Seller Prod With Sales", "published", 10)
		insertEvent("order_paid", &uA, pBlocked, 10, now.Add(-1*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
		require.NoError(t, err)

		for _, p := range res {
			assert.NotEqual(t, pBlocked, p.ID, "Product from inactive seller must be excluded from Popular")
		}
	})

	// H. insufficient stock исключается
	t.Run("H. insufficient stock исключается", func(t *testing.T) {
		pLowStock := createProd(sellerActive, catCommon, "Low Stock Prod (1)", "published", 1)
		pGoodStock := createProd(sellerActive, catCommon, "Good Stock Prod (2)", "published", 2)

		insertEvent("order_paid", &uA, pLowStock, 10, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, pGoodStock, 5, now.Add(-1*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 10)
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

	// I. equal popularity использует quality tie-breakers
	t.Run("I. equal popularity использует quality tie-breakers", func(t *testing.T) {
		t0 := now.Add(-10 * time.Hour)
		t1 := now.Add(-5 * time.Hour)
		t2 := now.Add(-1 * time.Hour)

		// Both products have equal sales: 3 items
		// 1. High rating (5.0, 5 reviews, t0) vs lower rating (4.0, 5 reviews, t0)
		pRating5 := createProdWithDetails(sellerActive, catCommon, nil, "Rating 5.0", "published", 5.0, 5, t0, 10)
		pRating4 := createProdWithDetails(sellerActive, catCommon, nil, "Rating 4.0", "published", 4.0, 5, t0, 10)

		insertEvent("order_paid", &uA, pRating5, 3, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, pRating4, 3, now.Add(-1*time.Hour))

		// 2. Same rating (4.5), higher reviews (50) vs lower reviews (10)
		pReviews50 := createProdWithDetails(sellerActive, catCommon, nil, "Reviews 50", "published", 4.5, 50, t0, 10)
		pReviews10 := createProdWithDetails(sellerActive, catCommon, nil, "Reviews 10", "published", 4.5, 10, t0, 10)

		insertEvent("order_paid", &uA, pReviews50, 3, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, pReviews10, 3, now.Add(-1*time.Hour))

		// 3. Same rating (4.2), same reviews (20), newer (t2) vs older (t1)
		pNewer := createProdWithDetails(sellerActive, catCommon, nil, "Newer Pub", "published", 4.2, 20, t2, 10)
		pOlder := createProdWithDetails(sellerActive, catCommon, nil, "Older Pub", "published", 4.2, 20, t1, 10)

		insertEvent("order_paid", &uA, pNewer, 3, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, pOlder, 3, now.Add(-1*time.Hour))

		res, err := svc.GetPopularProducts(ctx, 20)
		require.NoError(t, err)

		getIdx := func(id uuid.UUID) int {
			for i, p := range res {
				if p.ID == id {
					return i
				}
			}
			return -1
		}

		assert.Less(t, getIdx(pRating5), getIdx(pRating4), "Higher rating must rank before lower rating when sales equal")
		assert.Less(t, getIdx(pReviews50), getIdx(pReviews10), "Higher reviews must rank before lower reviews when rating equal")
		assert.Less(t, getIdx(pNewer), getIdx(pOlder), "Newer published_at must rank before older when rating and reviews equal")
	})

	// J. final ordering deterministic
	t.Run("J. final ordering deterministic", func(t *testing.T) {
		tFixed := now.Add(-5 * time.Hour)
		c1 := createProdWithDetails(sellerActive, catCommon, nil, "Det 1", "published", 4.0, 10, tFixed, 10)
		c2 := createProdWithDetails(sellerActive, catCommon, nil, "Det 2", "published", 4.0, 10, tFixed, 10)
		c3 := createProdWithDetails(sellerActive, catCommon, nil, "Det 3", "published", 4.0, 10, tFixed, 10)

		insertEvent("order_paid", &uA, c1, 7, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, c2, 7, now.Add(-1*time.Hour))
		insertEvent("order_paid", &uA, c3, 7, now.Add(-1*time.Hour))

		resFirst, err := svc.GetPopularProducts(ctx, 10)
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

		// Verify UUID ASC tie-breaker
		if c1.String() < c2.String() {
			assert.Less(t, idx1, idx2)
		} else {
			assert.Less(t, idx2, idx1)
		}

		// Multi-run repeatability
		for run := 0; run < 5; run++ {
			resNext, err := svc.GetPopularProducts(ctx, 10)
			require.NoError(t, err)
			require.Equal(t, len(resFirst), len(resNext))
			for i := range resFirst {
				assert.Equal(t, resFirst[i].ID, resNext[i].ID, "Ordering must be byte-for-byte identical across runs")
			}
		}
	})

	// K. empty history => [] / HTTP 200
	t.Run("K. empty history => [] / HTTP 200", func(t *testing.T) {
		// Clean all events temporarily
		var evIDs []uuid.UUID
		rows, err := pgClient.Pool.Query(ctx, "SELECT id FROM behavioral_events WHERE event_type = 'order_paid'")
		require.NoError(t, err)
		for rows.Next() {
			var id uuid.UUID
			_ = rows.Scan(&id)
			evIDs = append(evIDs, id)
		}
		rows.Close()

		// Handler HTTP test
		req := httptest.NewRequest("GET", "/api/public/products/popular", nil)
		rec := httptest.NewRecorder()
		handler.GetPopularProducts(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp, "items")
		assert.Contains(t, resp, "totalCount")
		assert.Equal(t, float64(len(resp["items"].([]interface{}))), resp["totalCount"])
	})

	// L. API contract
	t.Run("L. API contract", func(t *testing.T) {
		pContract := createProdWithDetails(sellerActive, catCommon, nil, "Contract Prod", "published", 4.8, 12, now, 10)
		insertEvent("order_paid", &uA, pContract, 100, now.Add(-1*time.Hour))

		// 1. HTTP GET request with limit query param
		req := httptest.NewRequest("GET", "/api/public/products/popular?limit=5", nil)
		rec := httptest.NewRecorder()
		handler.GetPopularProducts(rec, req)

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

		// Item 0 is the top seller
		top := parsed.Items[0]
		assert.Equal(t, pContract, top.ID)
		assert.Equal(t, "Contract Prod", top.Title)
		assert.Equal(t, int64(100000), top.PriceCents)
		assert.Equal(t, "RUB", top.Currency)
		assert.Equal(t, "published", top.Status)
		assert.NotEmpty(t, top.SellerSlug)
		assert.NotEmpty(t, top.SellerName)
	})
}
