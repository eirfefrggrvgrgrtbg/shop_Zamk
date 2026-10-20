package router_test

// MARKETING.M1.7 — seller submissions to editorial collections through the real router on zamk_test:
// who may submit what, duplicates, review (accept/reject), concurrency and access control.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
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

func TestCollectionSubmissions(t *testing.T) {
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

	elig := products.CanonicalStorefrontEligibilitySQL("p", "s")
	// Two distinct sellers: sellerA owns >=3 storefront-eligible products, sellerB owns >=1.
	var sellerA, sellerB uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, `
		SELECT p.seller_id FROM products p JOIN sellers s ON s.id = p.seller_id
		WHERE `+elig+` GROUP BY p.seller_id HAVING count(*) >= 3 ORDER BY count(*) DESC LIMIT 1`).Scan(&sellerA)
	require.NoError(t, err, "zamk_test needs a seller with >=3 eligible products")

	err = pgClient.Pool.QueryRow(ctx, `
		SELECT p.seller_id FROM products p JOIN sellers s ON s.id = p.seller_id
		WHERE `+elig+` AND p.seller_id <> $1 GROUP BY p.seller_id HAVING count(*) >= 1 LIMIT 1`, sellerA).Scan(&sellerB)
	require.NoError(t, err, "zamk_test needs a second distinct seller with >=1 eligible product")
	eligibleOf := func(seller uuid.UUID) []uuid.UUID {
		rows, err := pgClient.Pool.Query(ctx, `SELECT p.id FROM products p JOIN sellers s ON s.id = p.seller_id WHERE p.seller_id = $1 AND `+elig+` ORDER BY p.created_at DESC LIMIT 4`, seller)
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
	aProducts, bProducts := eligibleOf(sellerA), eligibleOf(sellerB)
	pAccept, pReject, pHideBefore := aProducts[0], aProducts[1], aProducts[2]

	var cleanupUsers []uuid.UUID
	var cleanupCollections []string
	var cleanupSellerUsers []uuid.UUID
	defer func() {
		_, _ = pgClient.Pool.Exec(ctx, `UPDATE products SET status = 'published' WHERE id = ANY($1)`, []uuid.UUID{pAccept, pReject, pHideBefore})
		for _, c := range cleanupCollections {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM collections WHERE id = $1`, c)
		}
		for _, u := range cleanupSellerUsers {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM seller_users WHERE user_id = $1`, u)
		}
		for _, u := range cleanupUsers {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u)
		}
	}()
	newUser := func(role string) (uuid.UUID, string) {
		id := uuid.New()
		email := fmt.Sprintf("m17_%s@zamk.local", id.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
			VALUES ($1, 'M17 Reviewer', $2, 'hash', $3, 'active', false, now(), now())`, id, email, role)
		require.NoError(t, err)
		cleanupUsers = append(cleanupUsers, id)
		tok, err := tokenService.GenerateAccessToken(id, email, role)
		require.NoError(t, err)
		return id, tok
	}
	sellerToken := func(seller uuid.UUID) string {
		id, tok := newUser("seller")
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO seller_users (id, seller_id, user_id, role, created_at) VALUES ($1, $2, $3, 'owner', now())`, uuid.New(), seller, id)
		require.NoError(t, err)
		cleanupSellerUsers = append(cleanupSellerUsers, id)
		return tok
	}
	tokenA, tokenB := sellerToken(sellerA), sellerToken(sellerB)
	_, customerToken := newUser("customer")
	editorID, editorToken := func() (uuid.UUID, string) {
		id, tok := newUser("admin")
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, id, "M17Editor", []string{"storefront.manage"})
		require.NoError(t, err)
		return id, tok
	}()
	_, outsiderToken := func() (uuid.UUID, string) {
		id, tok := newUser("admin")
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, id, "M17Outsider", []string{"orders.read"})
		require.NoError(t, err)
		return id, tok
	}()

	call := func(token, method, path string, body any) (int, map[string]any) {
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
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	errCode := func(out map[string]any) string {
		if e, ok := out["error"].(map[string]any); ok {
			s, _ := e["code"].(string)
			return s
		}
		return ""
	}
	newCollection := func(active bool) string {
		slug := "m17-" + uuid.NewString()[:8]
		code, out := call(editorToken, http.MethodPost, "/api/admin/merchandising/collections", map[string]any{"slug": slug, "title": "M17 " + slug, "isActive": active})
		require.Equal(t, http.StatusOK, code, "%v", out)
		id := out["id"].(string)
		cleanupCollections = append(cleanupCollections, id)
		return id
	}
	published, draft := newCollection(true), newCollection(false)
	submit := func(token, collection string, product uuid.UUID, comment string) (int, map[string]any) {
		return call(token, http.MethodPost, "/api/seller/collections/submissions", map[string]any{"collectionId": collection, "productId": product, "comment": comment})
	}
	itemCount := func(collection string, product uuid.UUID) int {
		var n int
		require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT count(*) FROM collection_items WHERE collection_id = $1 AND product_id = $2`, collection, product).Scan(&n))
		return n
	}

	t.Run("seller sees only published collections", func(t *testing.T) {
		code, out := call(tokenA, http.MethodGet, "/api/seller/collections/", nil)
		require.Equal(t, http.StatusOK, code)
		body, _ := json.Marshal(out)
		assert.Contains(t, string(body), published)
		assert.NotContains(t, string(body), draft)
	})

	var acceptID, rejectID string
	t.Run("seller submits an own eligible product", func(t *testing.T) {
		code, out := submit(tokenA, published, pAccept, "  Новая капсула  ")
		require.Equal(t, http.StatusCreated, code, "%v", out)
		assert.Equal(t, "pending", out["status"])
		assert.Equal(t, "Новая капсула", out["sellerComment"])
		acceptID = out["id"].(string)
		assert.NotContains(t, out, "reviewedBy")
		code, out = submit(tokenA, published, pReject, "")
		require.Equal(t, http.StatusCreated, code, "%v", out)
		rejectID = out["id"].(string)
	})

	t.Run("invalid submissions are refused", func(t *testing.T) {
		code, out := submit(tokenA, published, pAccept, "again")
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "duplicate_submission", errCode(out), "no second active submission for the pair")

		code, out = submit(tokenA, published, bProducts[0], "")
		assert.Equal(t, http.StatusNotFound, code, "someone else's product")
		assert.Equal(t, "product_not_found", errCode(out))
		code, _ = submit(tokenA, published, uuid.New(), "")
		assert.Equal(t, http.StatusNotFound, code)

		code, out = submit(tokenA, draft, pHideBefore, "")
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "collection_unavailable", errCode(out))
		code, _ = submit(tokenA, uuid.NewString(), pHideBefore, "")
		assert.Equal(t, http.StatusNotFound, code)

		_, err := pgClient.Pool.Exec(ctx, `UPDATE products SET status = 'hidden' WHERE id = $1`, pHideBefore)
		require.NoError(t, err)
		code, out = submit(tokenA, published, pHideBefore, "")
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "product_not_eligible", errCode(out), "unpublished product")
		_, err = pgClient.Pool.Exec(ctx, `UPDATE products SET status = 'published' WHERE id = $1`, pHideBefore)
		require.NoError(t, err)

		long := make([]byte, 301)
		for i := range long {
			long[i] = 'a'
		}
		code, out = submit(tokenA, published, pHideBefore, string(long))
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "text_too_long", errCode(out))
	})

	t.Run("access control: sellers cannot review, others cannot see", func(t *testing.T) {
		for _, tok := range []string{tokenA, tokenB, customerToken, outsiderToken, ""} {
			code, _ := call(tok, http.MethodPost, "/api/admin/merchandising/submissions/"+acceptID+"/accept", nil)
			assert.Contains(t, []int{http.StatusForbidden, http.StatusUnauthorized}, code)
			code, _ = call(tok, http.MethodGet, "/api/admin/merchandising/submissions", nil)
			assert.Contains(t, []int{http.StatusForbidden, http.StatusUnauthorized}, code)
		}
		assert.Zero(t, itemCount(published, pAccept), "a seller must not publish into a collection")

		code, out := call(tokenB, http.MethodGet, "/api/seller/collections/submissions", nil)
		require.Equal(t, http.StatusOK, code)
		body, _ := json.Marshal(out)
		assert.NotContains(t, string(body), acceptID, "another seller does not see foreign submissions")
		code, _ = call(customerToken, http.MethodGet, "/api/seller/collections/", nil)
		assert.Equal(t, http.StatusForbidden, code)
	})

	t.Run("admin sees pending submissions with brand, product and collection", func(t *testing.T) {
		code, out := call(editorToken, http.MethodGet, "/api/admin/merchandising/submissions?status=pending", nil)
		require.Equal(t, http.StatusOK, code)
		found := false
		for _, it := range out["items"].([]any) {
			m := it.(map[string]any)
			if m["id"] == acceptID {
				found = true
				assert.NotEmpty(t, m["brandName"])
				assert.Equal(t, pAccept.String(), m["product"].(map[string]any)["id"])
				assert.Equal(t, published, m["collectionId"])
			}
		}
		assert.True(t, found)
		code, out = call(editorToken, http.MethodGet, "/api/admin/merchandising/submissions?status=bogus", nil)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "invalid_status", errCode(out))
	})

	t.Run("two simultaneous acceptances create exactly one item", func(t *testing.T) {
		var wg sync.WaitGroup
		codes := make([]int, 4)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodPost, "/api/admin/merchandising/submissions/"+acceptID+"/accept", nil)
				req.Header.Set("Authorization", "Bearer "+editorToken)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				codes[i] = w.Code
			}(i)
		}
		wg.Wait()
		ok, conflict := 0, 0
		for _, c := range codes {
			switch c {
			case http.StatusOK:
				ok++
			case http.StatusConflict:
				conflict++
			}
		}
		assert.Equal(t, 1, ok, "codes: %v", codes)
		assert.Equal(t, 3, conflict, "codes: %v", codes)
		assert.Equal(t, 1, itemCount(published, pAccept), "no duplicate collection_items")

		var status string
		var reviewer *uuid.UUID
		var reviewedAt *string
		require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT status, reviewed_by, reviewed_at::text FROM collection_submissions WHERE id = $1`, acceptID).Scan(&status, &reviewer, &reviewedAt))
		assert.Equal(t, "accepted", status)
		require.NotNil(t, reviewer)
		assert.Equal(t, editorID, *reviewer, "who reviewed is stored")
		assert.NotNil(t, reviewedAt, "when is stored")

		code, out := call("", http.MethodGet, "/api/public/collections/"+slugOf(t, pgClient, ctx, published), nil)
		require.Equal(t, http.StatusOK, code)
		body, _ := json.Marshal(out)
		assert.Contains(t, string(body), pAccept.String(), "accepted product is visible in the public collection")

		code, out = submit(tokenA, published, pAccept, "")
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "already_in_collection", errCode(out))
	})

	t.Run("reject keeps the product out and shows the reason to the seller", func(t *testing.T) {
		code, out := call(editorToken, http.MethodPost, "/api/admin/merchandising/submissions/"+rejectID+"/reject", map[string]any{"reason": "Не подходит по теме"})
		require.Equal(t, http.StatusOK, code, "%v", out)
		assert.Equal(t, "rejected", out["status"])
		assert.Zero(t, itemCount(published, pReject))

		code, out = call(editorToken, http.MethodPost, "/api/admin/merchandising/submissions/"+rejectID+"/accept", nil)
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "already_reviewed", errCode(out), "a reviewed submission cannot be reviewed again")

		code, out = call(tokenA, http.MethodGet, "/api/seller/collections/submissions", nil)
		require.Equal(t, http.StatusOK, code)
		var reason any
		for _, it := range out["items"].([]any) {
			if m := it.(map[string]any); m["id"] == rejectID {
				reason = m["rejectionReason"]
				assert.NotContains(t, m, "reviewerName")
			}
		}
		assert.Equal(t, "Не подходит по теме", reason)

		// after a rejection the seller may submit the same pair again
		code, _ = submit(tokenA, published, pReject, "")
		assert.Equal(t, http.StatusCreated, code)
	})

	t.Run("product or collection changing before review is handled", func(t *testing.T) {
		code, out := submit(tokenA, published, pHideBefore, "")
		require.Equal(t, http.StatusCreated, code, "%v", out)
		id := out["id"].(string)

		_, err := pgClient.Pool.Exec(ctx, `UPDATE products SET status = 'hidden' WHERE id = $1`, pHideBefore)
		require.NoError(t, err)
		code, out = call(editorToken, http.MethodPost, "/api/admin/merchandising/submissions/"+id+"/accept", nil)
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "product_not_eligible", errCode(out))
		assert.Zero(t, itemCount(published, pHideBefore))
		var status string
		require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT status FROM collection_submissions WHERE id = $1`, id).Scan(&status))
		assert.Equal(t, "pending", status, "a failed acceptance leaves the submission pending")
		_, err = pgClient.Pool.Exec(ctx, `UPDATE products SET status = 'published' WHERE id = $1`, pHideBefore)
		require.NoError(t, err)

		// the collection is unpublished before the decision
		_, err = pgClient.Pool.Exec(ctx, `UPDATE collections SET is_active = false WHERE id = $1`, published)
		require.NoError(t, err)
		code, out = call(editorToken, http.MethodPost, "/api/admin/merchandising/submissions/"+id+"/accept", nil)
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "collection_unavailable", errCode(out))
		_, err = pgClient.Pool.Exec(ctx, `UPDATE collections SET is_active = true WHERE id = $1`, published)
		require.NoError(t, err)

		// now acceptable; the new product is appended after the existing one (order kept)
		code, out = call(editorToken, http.MethodPost, "/api/admin/merchandising/submissions/"+id+"/accept", nil)
		require.Equal(t, http.StatusOK, code, "%v", out)
		var order []uuid.UUID
		irows, err := pgClient.Pool.Query(ctx, `SELECT product_id FROM collection_items WHERE collection_id = $1 ORDER BY sort_order`, published)
		require.NoError(t, err)
		for irows.Next() {
			var p uuid.UUID
			require.NoError(t, irows.Scan(&p))
			order = append(order, p)
		}
		irows.Close()
		assert.Equal(t, []uuid.UUID{pAccept, pHideBefore}, order)
	})

	t.Run("existing admin collection endpoints keep working", func(t *testing.T) {
		code, _ := call(editorToken, http.MethodPut, "/api/admin/merchandising/collections/"+published+"/items", map[string]any{"productIds": []uuid.UUID{pHideBefore, pAccept}})
		assert.Equal(t, http.StatusNoContent, code)
		code, out := call(editorToken, http.MethodGet, "/api/admin/merchandising/collections/"+published, nil)
		require.Equal(t, http.StatusOK, code)
		assert.Len(t, out["items"].([]any), 2)
	})
}

func slugOf(t *testing.T, c *postgres.Client, ctx context.Context, id string) string {
	var s string
	require.NoError(t, c.Pool.QueryRow(ctx, `SELECT slug FROM collections WHERE id = $1`, id).Scan(&s))
	return s
}
