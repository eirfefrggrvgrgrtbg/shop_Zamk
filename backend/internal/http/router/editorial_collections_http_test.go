package router_test

// MARKETING.M1.5 — editorial collections through the real router on zamk_test:
// RBAC (storefront.manage), create/edit/items/order, and what customers get from the public API.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestEditorialCollections_AdminAndPublic(t *testing.T) {
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

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)
	newAdmin := func(perms []string) string {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'M15', 'hash', 'admin', 'active', NOW(), NOW())`, id, id.String()+"@test.com", "7998"+id.String()[:7])
		require.NoError(t, err)
		_, err = setupRouterStaffWithPermissions(ctx, pgClient.Pool, id, "M15Role", perms)
		require.NoError(t, err)
		tok, err := tokenService.GenerateAccessToken(id, id.String()+"@test.com", "admin")
		require.NoError(t, err)
		return tok
	}
	editor := newAdmin([]string{"storefront.manage"})
	outsider := newAdmin([]string{"marketing.campaigns.read", "marketing.campaigns.write", "products.read"})

	call := func(token, method, path string, body any) (int, []byte) {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req := httptest.NewRequest(method, path, &buf)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.Bytes()
	}

	// Fixtures: two storefront-eligible products and one product customers cannot see.
	loadIDs := func(query string, limit int) []uuid.UUID {
		rows, err := pgClient.Pool.Query(ctx, query, limit)
		require.NoError(t, err)
		defer rows.Close()
		var ids []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			require.NoError(t, rows.Scan(&id))
			ids = append(ids, id)
		}
		return ids
	}
	eligibility := products.CanonicalStorefrontEligibilitySQL("p", "s")
	eligible := loadIDs(`SELECT p.id FROM products p JOIN sellers s ON s.id = p.seller_id WHERE `+eligibility+` ORDER BY p.created_at DESC LIMIT $1`, 2)
	require.Len(t, eligible, 2, "zamk_test must contain two storefront-eligible products")
	hiddenIDs := loadIDs(`SELECT p.id FROM products p JOIN sellers s ON s.id = p.seller_id WHERE NOT (`+eligibility+`) LIMIT $1`, 1)
	require.Len(t, hiddenIDs, 1, "zamk_test must contain a product that is not visible to customers")
	hidden := hiddenIDs[0]

	slug := "m15-" + uuid.NewString()[:8]
	var collectionID string
	defer func() {
		if collectionID != "" {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM collections WHERE id = $1`, collectionID)
		}
	}()

	t.Run("RBAC: only storefront.manage may manage collections", func(t *testing.T) {
		body := map[string]any{"slug": slug, "title": "M15 rbac", "isActive": false}
		code, _ := call(outsider, http.MethodPost, "/api/admin/merchandising/collections", body)
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = call(outsider, http.MethodGet, "/api/admin/merchandising/collections", nil)
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = call("", http.MethodGet, "/api/admin/merchandising/collections", nil)
		assert.Equal(t, http.StatusUnauthorized, code)
	})

	t.Run("create as draft, then edit", func(t *testing.T) {
		code, out := call(editor, http.MethodPost, "/api/admin/merchandising/collections",
			map[string]any{"slug": slug, "title": "M15 title", "description": "short", "isActive": false})
		require.Equal(t, http.StatusOK, code, string(out))
		var c map[string]any
		require.NoError(t, json.Unmarshal(out, &c))
		collectionID = c["id"].(string)

		code, out = call(editor, http.MethodPost, "/api/admin/merchandising/collections",
			map[string]any{"slug": slug, "title": "dup", "isActive": false})
		assert.Equal(t, http.StatusConflict, code, string(out))

		code, out = call(editor, http.MethodPatch, "/api/admin/merchandising/collections/"+collectionID,
			map[string]any{"slug": slug, "title": "M15 edited", "description": "edited", "isActive": false})
		require.Equal(t, http.StatusOK, code, string(out))
		assert.Contains(t, string(out), "M15 edited")
	})

	t.Run("draft is invisible to customers", func(t *testing.T) {
		code, _ := call("", http.MethodGet, "/api/public/collections/"+slug, nil)
		assert.Equal(t, http.StatusNotFound, code)
		_, list := call("", http.MethodGet, "/api/public/collections", nil)
		assert.NotContains(t, string(list), slug)
	})

	t.Run("items keep the requested order and hide unpublished products from customers", func(t *testing.T) {
		code, out := call(editor, http.MethodPut, "/api/admin/merchandising/collections/"+collectionID+"/items",
			map[string]any{"productIds": []uuid.UUID{eligible[1], hidden, eligible[0]}})
		require.Equal(t, http.StatusNoContent, code, string(out))

		code, out = call(editor, http.MethodGet, "/api/admin/merchandising/collections/"+collectionID, nil)
		require.Equal(t, http.StatusOK, code)
		var admin struct {
			Items []struct {
				ProductID string `json:"productId"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(out, &admin))
		require.Len(t, admin.Items, 3, "admin still sees the unpublished product")
		assert.Equal(t, eligible[1].String(), admin.Items[0].ProductID)
		assert.Equal(t, hidden.String(), admin.Items[1].ProductID)

		// publish
		code, out = call(editor, http.MethodPatch, "/api/admin/merchandising/collections/"+collectionID,
			map[string]any{"slug": slug, "title": "M15 edited", "isActive": true})
		require.Equal(t, http.StatusOK, code, string(out))

		code, out = call("", http.MethodGet, "/api/public/collections/"+slug, nil)
		require.Equal(t, http.StatusOK, code, string(out))
		var pub struct {
			Products []struct {
				ID string `json:"id"`
			} `json:"products"`
		}
		require.NoError(t, json.Unmarshal(out, &pub))
		require.Len(t, pub.Products, 2, "unpublished product must not reach customers")
		assert.Equal(t, eligible[1].String(), pub.Products[0].ID)
		assert.Equal(t, eligible[0].String(), pub.Products[1].ID)
		assert.NotContains(t, string(out), hidden.String())
	})

	t.Run("a product may be in several collections; removal and unpublishing work", func(t *testing.T) {
		slug2 := "m15-" + uuid.NewString()[:8]
		code, out := call(editor, http.MethodPost, "/api/admin/merchandising/collections",
			map[string]any{"slug": slug2, "title": "M15 second", "isActive": true})
		require.Equal(t, http.StatusOK, code, string(out))
		var c2 map[string]any
		require.NoError(t, json.Unmarshal(out, &c2))
		id2 := c2["id"].(string)
		defer func() { _, _ = pgClient.Pool.Exec(ctx, `DELETE FROM collections WHERE id = $1`, id2) }()

		code, _ = call(editor, http.MethodPut, "/api/admin/merchandising/collections/"+id2+"/items",
			map[string]any{"productIds": []uuid.UUID{eligible[0]}})
		require.Equal(t, http.StatusNoContent, code)

		// removal from the first collection leaves the second intact
		code, _ = call(editor, http.MethodPut, "/api/admin/merchandising/collections/"+collectionID+"/items",
			map[string]any{"productIds": []uuid.UUID{eligible[1]}})
		require.Equal(t, http.StatusNoContent, code)
		_, out = call("", http.MethodGet, "/api/public/collections/"+slug2, nil)
		assert.Contains(t, string(out), eligible[0].String())
		_, out = call("", http.MethodGet, "/api/public/collections/"+slug, nil)
		assert.NotContains(t, string(out), eligible[0].String())

		// unpublish
		code, _ = call(editor, http.MethodPatch, "/api/admin/merchandising/collections/"+collectionID,
			map[string]any{"slug": slug, "title": "M15 edited", "isActive": false})
		require.Equal(t, http.StatusOK, code)
		code, _ = call("", http.MethodGet, "/api/public/collections/"+slug, nil)
		assert.Equal(t, http.StatusNotFound, code)
	})
}
