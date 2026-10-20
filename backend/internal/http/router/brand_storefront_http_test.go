package router_test

// MARKETING.M1.8 — public brand storefronts through the real router on zamk_test:
// Resolves brand by ID/slug (and seller slug), returns published items & totalCount,
// enforces storefront eligibility (hides drafts/unpublished), handles 404s, and protects privacy.

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
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
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()
	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	defer cancel()

	// 1. Brand with multiple published products (via seller or brand)
	elig := products.CanonicalStorefrontEligibilitySQL("p", "s")
	var sellerID uuid.UUID
	var sellerSlug string
	var brandName string
	err = pgClient.Pool.QueryRow(ctx, `
		SELECT s.id, s.slug, s.brand_name
		FROM sellers s
		JOIN products p ON p.seller_id = s.id
		WHERE `+elig+`
		GROUP BY s.id, s.slug, s.brand_name
		HAVING count(*) >= 2
		ORDER BY count(*) DESC
		LIMIT 1`).Scan(&sellerID, &sellerSlug, &brandName)
	require.NoError(t, err, "need seller with at least 2 eligible products")

	// Also check if seller has an active primary brand in brands table
	var linkedBrandID *uuid.UUID
	var linkedBrandSlug *string
	var bID uuid.UUID
	var bSlug string
	bErr := pgClient.Pool.QueryRow(ctx, `
		SELECT b.id, b.slug
		FROM brands b
		JOIN seller_brands sb ON sb.brand_id = b.id
		WHERE sb.seller_id = $1 AND sb.status = 'active'
		LIMIT 1`, sellerID).Scan(&bID, &bSlug)
	if bErr == nil {
		linkedBrandID = &bID
		linkedBrandSlug = &bSlug
	}

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
		var eligibleProdID uuid.UUID
		err := pgClient.Pool.QueryRow(ctx, `
			SELECT p.id
			FROM products p
			JOIN sellers s ON p.seller_id = s.id
			WHERE `+elig+`
			LIMIT 1`).Scan(&eligibleProdID)
		require.NoError(t, err)

		singleBrandID := uuid.New()
		singleSlug := fmt.Sprintf("single-prod-brand-%s", singleBrandID.String()[:8])
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO brands (id, name, slug, description, is_active, created_at, updated_at)
			VALUES ($1, 'Single Item Brand', $2, 'Solo piece studio', true, now(), now())`,
			singleBrandID, singleSlug)
		require.NoError(t, err)

		var prevBrandID *uuid.UUID
		_ = pgClient.Pool.QueryRow(ctx, `SELECT brand_id FROM products WHERE id = $1`, eligibleProdID).Scan(&prevBrandID)
		_, err = pgClient.Pool.Exec(ctx, `UPDATE products SET brand_id = $1 WHERE id = $2`, singleBrandID, eligibleProdID)
		require.NoError(t, err)

		defer func() {
			_, _ = pgClient.Pool.Exec(ctx, `UPDATE products SET brand_id = $1 WHERE id = $2`, prevBrandID, eligibleProdID)
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM brands WHERE id = $1`, singleBrandID)
		}()

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
