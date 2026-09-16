package router_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

type publicCatalogProduct struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	PriceCents  int64      `json:"priceCents"`
	PublishedAt *time.Time `json:"publishedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type publicCatalogResponse struct {
	Items      []publicCatalogProduct `json:"items"`
	TotalCount int                    `json:"totalCount"`
}

func setupCatalogDeterminismTestEnv(t *testing.T) (context.Context, http.Handler, func(), *postgres.Client) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping router integration test")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://127.0.0.1:3000"},
		},
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
	}

	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)

	var dbName string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "tests must strictly run against zamk_test")

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)

	cleanup := func() {
		cancel()
		redisClient.Close()
		pgClient.Close()
	}

	return ctx, r, cleanup, pgClient
}

func TestCatalogDeterminism(t *testing.T) {
	ctx, r, cleanup, pgClient := setupCatalogDeterminismTestEnv(t)
	defer cleanup()

	// Initial cleanup of any previous test fixtures
	_, _ = pgClient.Pool.Exec(ctx, `
		DELETE FROM inventory_items WHERE product_id IN (
			SELECT id FROM products WHERE slug LIKE 'coat-%' OR slug LIKE 'lower-price-coat-%' OR slug LIKE 'higher-price-coat-%' OR slug LIKE 'newer-coat-%' OR slug LIKE 'draft-coat-%' OR slug LIKE 'free-1-coat-%' OR slug LIKE 'free-2-coat-%' OR slug LIKE 'suspended-seller-coat-%'
		)
	`)
	_, _ = pgClient.Pool.Exec(ctx, `
		DELETE FROM product_variants WHERE product_id IN (
			SELECT id FROM products WHERE slug LIKE 'coat-%' OR slug LIKE 'lower-price-coat-%' OR slug LIKE 'higher-price-coat-%' OR slug LIKE 'newer-coat-%' OR slug LIKE 'draft-coat-%' OR slug LIKE 'free-1-coat-%' OR slug LIKE 'free-2-coat-%' OR slug LIKE 'suspended-seller-coat-%'
		)
	`)
	_, _ = pgClient.Pool.Exec(ctx, `
		DELETE FROM products WHERE slug LIKE 'coat-%' OR slug LIKE 'lower-price-coat-%' OR slug LIKE 'higher-price-coat-%' OR slug LIKE 'newer-coat-%' OR slug LIKE 'draft-coat-%' OR slug LIKE 'free-1-coat-%' OR slug LIKE 'free-2-coat-%' OR slug LIKE 'suspended-seller-coat-%'
	`)

	// 1. Create active seller
	sellerUserID := uuid.New()
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Active Seller', 'hash', 'seller', 'active', now(), now())
	`, sellerUserID, "seller-det-"+sellerUserID.String()[:8]+"@test.com", "+7991"+sellerUserID.String()[:7])
	require.NoError(t, err)

	activeSellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Active Brand', $2, 'active@test.com', 'active', now(), now())
	`, activeSellerID, "active-brand-"+activeSellerID.String()[:8])
	require.NoError(t, err)

	// 2. Create inactive seller for filter regression
	inactiveSellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Inactive Brand', $2, 'inactive@test.com', 'blocked', now(), now())
	`, inactiveSellerID, "inactive-brand-"+inactiveSellerID.String()[:8])
	require.NoError(t, err)

	// 3. Create isolated category
	catID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug)
		VALUES ($1, 'Determinism Category', $2)
	`, catID, "det-cat-"+catID.String()[:8])
	require.NoError(t, err)

	// Fixed timestamps for identical primary sort testing
	fixedTime := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)

	// Distinct UUIDs generated per run and sorted ascending:
	// id1 < id2 < id3 < id4
	var rawIDs []uuid.UUID
	for i := 0; i < 4; i++ {
		rawIDs = append(rawIDs, uuid.New())
	}
	sort.Slice(rawIDs, func(i, j int) bool {
		return rawIDs[i].String() < rawIDs[j].String()
	})
	id1, id2, id3, id4 := rawIDs[0], rawIDs[1], rawIDs[2], rawIDs[3]

	var createdProductIDs []uuid.UUID
	createdProductIDs = append(createdProductIDs, id1, id2, id3, id4)

	createProductFixture := func(prodID uuid.UUID, sID uuid.UUID, title string, priceCents int64, status string, pubAt, createdAt time.Time, totalStock, reservedStock int) {
		slug := fmt.Sprintf("%s-%s", title, prodID.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				published_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'RUB', $7, $8, $9, $9)
		`, prodID, sID, catID, title, slug, priceCents, status, pubAt, createdAt)
		require.NoError(t, err)

		variantID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, $4, $5, true, $6, $6)
		`, variantID, prodID, "SKU-"+variantID.String()[:8], "BC-"+variantID.String()[:8], priceCents, createdAt)
		require.NoError(t, err)

		if totalStock > 0 || reservedStock > 0 {
			_, err = pgClient.Pool.Exec(ctx, `
				INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
			`, uuid.New(), prodID, variantID, sID, totalStock, reservedStock, createdAt)
			require.NoError(t, err)
		}
	}

	// Create 4 products with identical sort fields (price=10000, published_at=fixedTime, created_at=fixedTime)
	for _, id := range []uuid.UUID{id3, id1, id4, id2} { // inserted intentionally out of order
		createProductFixture(id, activeSellerID, "coat", 10000, "published", fixedTime, fixedTime, 10, 0)
	}

	// Cleanup defer
	defer func() {
		for _, pid := range createdProductIDs {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_id = $1", pid)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = $1", pid)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = $1", pid)
		}
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id IN ($1, $2)", activeSellerID, inactiveSellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", catID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", sellerUserID)
	}()

	fetchProducts := func(queryURL string) publicCatalogResponse {
		req := httptest.NewRequest(http.MethodGet, queryURL, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp publicCatalogResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		return resp
	}

	t.Run("A. DEFAULT: identical primary fields order deterministically by product ID ASC", func(t *testing.T) {
		resp := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s", catID))
		require.Len(t, resp.Items, 4)
		assert.Equal(t, id1.String(), resp.Items[0].ID)
		assert.Equal(t, id2.String(), resp.Items[1].ID)
		assert.Equal(t, id3.String(), resp.Items[2].ID)
		assert.Equal(t, id4.String(), resp.Items[3].ID)
	})

	t.Run("B. Repeated DEFAULT calls with unchanged data return exact identical order", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s", catID))
			require.Len(t, resp.Items, 4)
			assert.Equal(t, id1.String(), resp.Items[0].ID)
			assert.Equal(t, id2.String(), resp.Items[1].ID)
			assert.Equal(t, id3.String(), resp.Items[2].ID)
			assert.Equal(t, id4.String(), resp.Items[3].ID)
		}
	})

	t.Run("C. DEFAULT pagination: no duplicate IDs between pages and no missing IDs", func(t *testing.T) {
		// Page 1: limit=2, offset=0
		page1 := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s&limit=2&offset=0", catID))
		require.Len(t, page1.Items, 2)
		assert.Equal(t, id1.String(), page1.Items[0].ID)
		assert.Equal(t, id2.String(), page1.Items[1].ID)

		// Page 2: limit=2, offset=2
		page2 := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s&limit=2&offset=2", catID))
		require.Len(t, page2.Items, 2)
		assert.Equal(t, id3.String(), page2.Items[0].ID)
		assert.Equal(t, id4.String(), page2.Items[1].ID)

		// Single-item pages: limit=1
		seen := make(map[string]bool)
		for off := 0; off < 4; off++ {
			p := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s&limit=1&offset=%d", catID, off))
			require.Len(t, p.Items, 1)
			assert.False(t, seen[p.Items[0].ID], "duplicate ID across pages: %s", p.Items[0].ID)
			seen[p.Items[0].ID] = true
		}
		assert.True(t, seen[id1.String()])
		assert.True(t, seen[id2.String()])
		assert.True(t, seen[id3.String()])
		assert.True(t, seen[id4.String()])
	})

	var idLower, idHigher uuid.UUID

	t.Run("D. price_asc: equal price -> deterministic p.id ASC, lower price comes first", func(t *testing.T) {
		// Add lower priced product
		idLower = uuid.New()
		createProductFixture(idLower, activeSellerID, "lower-price-coat", 5000, "published", fixedTime, fixedTime, 10, 0)
		createdProductIDs = append(createdProductIDs, idLower)

		resp := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s&sort=price_asc", catID))
		require.Len(t, resp.Items, 5)

		// Lower price (5000) must come first
		assert.Equal(t, idLower.String(), resp.Items[0].ID)
		assert.Equal(t, int64(5000), resp.Items[0].PriceCents)

		// Equal price (10000) must be ordered deterministically by p.id ASC
		assert.Equal(t, id1.String(), resp.Items[1].ID)
		assert.Equal(t, id2.String(), resp.Items[2].ID)
		assert.Equal(t, id3.String(), resp.Items[3].ID)
		assert.Equal(t, id4.String(), resp.Items[4].ID)
	})

	t.Run("E. price_desc: equal price -> deterministic p.id ASC, higher price comes first", func(t *testing.T) {
		// Add higher priced product
		idHigher = uuid.New()
		createProductFixture(idHigher, activeSellerID, "higher-price-coat", 50000, "published", fixedTime, fixedTime, 10, 0)
		createdProductIDs = append(createdProductIDs, idHigher)

		resp := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s&sort=price_desc", catID))
		require.Len(t, resp.Items, 6)

		// Higher price (50000) must come first
		assert.Equal(t, idHigher.String(), resp.Items[0].ID)
		assert.Equal(t, int64(50000), resp.Items[0].PriceCents)

		// Next are equal price (10000) products, ordered deterministically by p.id ASC (NOT reversed)
		assert.Equal(t, id1.String(), resp.Items[1].ID)
		assert.Equal(t, id2.String(), resp.Items[2].ID)
		assert.Equal(t, id3.String(), resp.Items[3].ID)
		assert.Equal(t, id4.String(), resp.Items[4].ID)

		// Lowest price (5000) comes last
		assert.Equal(t, int64(5000), resp.Items[5].PriceCents)
	})

	t.Run("F. newest: equal timestamps -> deterministic p.id ASC, newer timestamp comes first", func(t *testing.T) {
		// Add newer published product
		idNewer := uuid.New()
		newerTime := fixedTime.Add(2 * time.Hour)
		createProductFixture(idNewer, activeSellerID, "newer-coat", 10000, "published", newerTime, newerTime, 10, 0)
		createdProductIDs = append(createdProductIDs, idNewer)

		resp := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s&sort=newest", catID))
		require.Len(t, resp.Items, 7)

		// Newer timestamp must come first
		assert.Equal(t, idNewer.String(), resp.Items[0].ID)

		// All items with identical timestamp (fixedTime) must be ordered deterministically by p.id ASC
		sameTimeIDs := []uuid.UUID{id1, id2, id3, id4, idLower, idHigher}
		sort.Slice(sameTimeIDs, func(i, j int) bool {
			return sameTimeIDs[i].String() < sameTimeIDs[j].String()
		})
		for i, expectedID := range sameTimeIDs {
			assert.Equal(t, expectedID.String(), resp.Items[1+i].ID, "mismatch at position %d", 1+i)
		}
	})

	t.Run("G. Filter Regression: unpublished, inactive seller, CAT.1A free=1 excluded, free=2 eligible", func(t *testing.T) {
		// 1. Unpublished product (draft)
		idDraft := uuid.New()
		createProductFixture(idDraft, activeSellerID, "draft-coat", 10000, "draft", fixedTime, fixedTime, 10, 0)
		createdProductIDs = append(createdProductIDs, idDraft)

		// 2. Inactive seller product
		idInactiveSeller := uuid.New()
		createProductFixture(idInactiveSeller, inactiveSellerID, "suspended-seller-coat", 10000, "published", fixedTime, fixedTime, 10, 0)
		createdProductIDs = append(createdProductIDs, idInactiveSeller)

		// 3. CAT.1A free=1 product (total=1, reserved=0 -> free=1 < MinStorefrontFreeSellableUnits(2) -> excluded)
		idFree1 := uuid.New()
		createProductFixture(idFree1, activeSellerID, "free-1-coat", 10000, "published", fixedTime, fixedTime, 1, 0)
		createdProductIDs = append(createdProductIDs, idFree1)

		// 4. CAT.1A free=2 product (total=2, reserved=0 -> free=2 >= MinStorefrontFreeSellableUnits(2) -> eligible)
		idFree2 := uuid.New()
		createProductFixture(idFree2, activeSellerID, "free-2-coat", 10000, "published", fixedTime, fixedTime, 2, 0)
		createdProductIDs = append(createdProductIDs, idFree2)

		resp := fetchProducts(fmt.Sprintf("/api/public/products?categoryId=%s", catID))

		returnedIDs := make(map[string]bool)
		for _, item := range resp.Items {
			returnedIDs[item.ID] = true
		}

		// Verify exclusions
		assert.False(t, returnedIDs[idDraft.String()], "unpublished draft product must be excluded")
		assert.False(t, returnedIDs[idInactiveSeller.String()], "inactive seller product must be excluded")
		assert.False(t, returnedIDs[idFree1.String()], "CAT.1A free=1 product must be excluded")

		// Verify inclusion
		assert.True(t, returnedIDs[idFree2.String()], "CAT.1A free=2 product must be eligible")
	})
}
