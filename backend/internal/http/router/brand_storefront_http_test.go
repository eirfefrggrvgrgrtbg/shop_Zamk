package router_test

// MARKETING.M1.8 — public brand storefronts through the real router on zamk_test:
// Resolves brand by ID/slug (and seller slug), returns published items & totalCount,
// enforces storefront eligibility (hides drafts/unpublished), handles 404s, and protects privacy.

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestBrandStorefront_PublicEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	ctx := context.Background()
	cfg := &config.Config{
		JWT:  config.JWTConfig{AccessTokenSecret: "test-secret", RefreshTokenSecret: "test-secret-refresh", AccessTokenTTLMinutes: 60, RefreshTokenTTLDays: 7},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
	}
	pgClient, err := testutil.NewTrackedClient(t, ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()
	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	defer cancel()

	// 1. Own isolated storefront: a seller with its primary brand and 3 published, in-stock products (never the
	// sellers that happen to exist in the shared zamk_test).
	fx := seedStorefrontSeller(t, pgClient.Pool, 3)
	sellerID, sellerSlug, brandName := fx.SellerID, fx.SellerSlug, fx.BrandName
	_ = brandName
	linkedBrandID, linkedBrandSlug := &fx.BrandID, &fx.BrandSlug

	t.Run("1. Brand with multiple published products by seller slug", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/public/brands/"+sellerSlug, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

		brandMap, ok := body["brand"].(map[string]any)
		require.True(t, ok)
		assert.NotEmpty(t, brandMap["name"])
		assert.Equal(t, sellerSlug, brandMap["slug"])

		items, ok := body["items"].([]any)
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(items), 2)
		totalCount, ok := body["totalCount"].(float64)
		require.True(t, ok)
		assert.GreaterOrEqual(t, int(totalCount), 2)

		// Privacy check: no internal secrets leaked
		assert.Nil(t, brandMap["logoObjectKey"])
		assert.Nil(t, brandMap["contactEmail"])
		assert.Nil(t, brandMap["contactPhone"])
	})

	if linkedBrandID != nil {
		t.Run("2. Brand with multiple products by brand UUID", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/public/brands/"+linkedBrandID.String(), nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

			brandMap, ok := body["brand"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, linkedBrandID.String(), brandMap["id"])

			items, ok := body["items"].([]any)
			require.True(t, ok)
			assert.GreaterOrEqual(t, len(items), 1)
		})

		if linkedBrandSlug != nil {
			t.Run("2b. Brand lookup by brand slug", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/api/public/brands/"+*linkedBrandSlug, nil)
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)

				require.Equal(t, http.StatusOK, rec.Code)
				var body map[string]any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

				brandMap, ok := body["brand"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, *linkedBrandSlug, brandMap["slug"])
			})
		}
	}

	t.Run("3. Brand with no products", func(t *testing.T) {
		emptyBrandID := uuid.New()
		emptySlug := fmt.Sprintf("empty-brand-%s", emptyBrandID.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO brands (id, name, slug, description, is_active, created_at, updated_at)
			VALUES ($1, 'Empty Archive Brand', $2, 'Specialized archive collection', true, now(), now())`,
			emptyBrandID, emptySlug)
		require.NoError(t, err)
		defer func() {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM brands WHERE id = $1`, emptyBrandID)
		}()

		req := httptest.NewRequest("GET", "/api/public/brands/"+emptySlug, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

		brandMap := body["brand"].(map[string]any)
		assert.Equal(t, "Empty Archive Brand", brandMap["name"])
		assert.Equal(t, "Specialized archive collection", brandMap["description"])

		items := body["items"].([]any)
		assert.Empty(t, items)
		assert.Equal(t, float64(0), body["totalCount"])
	})

	t.Run("3b. Brand with exactly 1 published product", func(t *testing.T) {
		single := seedStorefrontSeller(t, pgClient.Pool, 1) // own one-product brand; no shared product is re-branded
		singleSlug := single.BrandSlug

		req := httptest.NewRequest("GET", "/api/public/brands/"+singleSlug, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

		items := body["items"].([]any)
		assert.Len(t, items, 1)
		assert.Equal(t, float64(1), body["totalCount"])
	})

	t.Run("4. Unknown brand returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/public/brands/nonexistent-unknown-brand-404", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		errMap, ok := body["error"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "not_found", errMap["code"])
	})

	t.Run("5. Unpublished / draft products are strictly hidden", func(t *testing.T) {
		// Create a brand with ONLY a draft product
		brandID := uuid.New()
		bSlug := fmt.Sprintf("draft-only-brand-%s", brandID.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
			VALUES ($1, 'Draft Only Brand', $2, true, now(), now())`, brandID, bSlug)
		require.NoError(t, err)
		defer func() {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM products WHERE brand_id = $1`, brandID)
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM brands WHERE id = $1`, brandID)
		}()

		// Add draft product to seller
		draftProdID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, brand_id, title, slug, status, currency, price_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'Secret Draft Jacket', $4, 'draft', 'RUB', 1500000, now(), now())`,
			draftProdID, sellerID, brandID, "secret-draft-jacket-"+draftProdID.String()[:8])
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/api/public/brands/"+bSlug, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

		items := body["items"].([]any)
		assert.Empty(t, items, "draft product must not be exposed in public storefront")
		assert.Equal(t, float64(0), body["totalCount"])
	})
}
