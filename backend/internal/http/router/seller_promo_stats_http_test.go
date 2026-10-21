package router_test

// MARKETING.M1.6 — access control of the seller promo statistics endpoints through the real router on zamk_test.

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestSellerPromoStats_AccessControl(t *testing.T) {
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
	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	type actor struct{ userID, sellerID uuid.UUID }
	var cleanupUsers, cleanupSellers []uuid.UUID
	var campaigns []string
	defer func() {
		for _, c := range campaigns {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM promo_codes WHERE campaign_id = $1`, c)
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM marketing_campaigns WHERE id = $1`, c)
		}
		for _, s := range cleanupSellers {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM seller_users WHERE seller_id = $1`, s)
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM sellers WHERE id = $1`, s)
		}
		for _, u := range cleanupUsers {
			_, _ = pgClient.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u)
		}
	}()
	newSeller := func() (actor, string) {
		a := actor{userID: uuid.New(), sellerID: uuid.New()}
		email := fmt.Sprintf("m16_%s@zamk.local", a.userID.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
			VALUES ($1, 'M16', $2, 'hash', 'seller', 'active', false, now(), now())`, a.userID, email)
		require.NoError(t, err)
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
			VALUES ($1, 'M16 Seller', $2, 'd', $3, '1', 'active', now(), now())`, a.sellerID, "m16-"+a.sellerID.String()[:8], email)
		require.NoError(t, err)
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO seller_users (id, seller_id, user_id, role, created_at) VALUES ($1, $2, $3, 'owner', now())`, uuid.New(), a.sellerID, a.userID)
		require.NoError(t, err)
		cleanupUsers = append(cleanupUsers, a.userID)
		cleanupSellers = append(cleanupSellers, a.sellerID)
		tok, err := tokenService.GenerateAccessToken(a.userID, email, "seller")
		require.NoError(t, err)
		return a, tok
	}
	_, tokenA := newSeller()
	_, tokenB := newSeller()
	custID := uuid.New()
	custEmail := fmt.Sprintf("m16c_%s@zamk.local", custID.String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'M16c', $2, 'hash', 'customer', 'active', false, now(), now())`, custID, custEmail)
	require.NoError(t, err)
	cleanupUsers = append(cleanupUsers, custID)
	customerToken, err := tokenService.GenerateAccessToken(custID, custEmail, "customer")
	require.NoError(t, err)

	call := func(token, method, path string, body string) (int, []byte) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.Bytes()
	}

	code, out := call(tokenA, http.MethodPost, "/api/seller/promotions/", fmt.Sprintf(`{"code":"M16%s","discountType":"percent","discountValueBps":1000}`, strings.ToUpper(uuid.NewString()[:6])))
	require.Equal(t, http.StatusCreated, code, string(out))
	var created struct{ ID, CampaignID string }
	require.NoError(t, json.Unmarshal(out, &created))
	campaigns = append(campaigns, created.CampaignID)

	t.Run("anonymous and customer tokens are rejected", func(t *testing.T) {
		code, _ := call("", http.MethodGet, "/api/seller/promotions/stats", "")
		assert.Equal(t, http.StatusUnauthorized, code)
		code, _ = call(customerToken, http.MethodGet, "/api/seller/promotions/stats", "")
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = call(customerToken, http.MethodGet, "/api/seller/promotions/"+created.ID+"/stats", "")
		assert.Equal(t, http.StatusForbidden, code)
	})

	t.Run("owner sees zeros for a code without orders", func(t *testing.T) {
		code, out := call(tokenA, http.MethodGet, "/api/seller/promotions/"+created.ID+"/stats", "")
		require.Equal(t, http.StatusOK, code, string(out))
		var d struct {
			PaidOrders   int   `json:"paidOrders"`
			SalesCents   int64 `json:"salesCents"`
			RecentOrders []any `json:"recentOrders"`
		}
		require.NoError(t, json.Unmarshal(out, &d))
		assert.Zero(t, d.PaidOrders+int(d.SalesCents))
		assert.Empty(t, d.RecentOrders)
		code, out = call(tokenA, http.MethodGet, "/api/seller/promotions/stats", "")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, string(out), created.ID)
	})

	t.Run("another seller cannot read it", func(t *testing.T) {
		code, _ := call(tokenB, http.MethodGet, "/api/seller/promotions/"+created.ID+"/stats", "")
		assert.Equal(t, http.StatusNotFound, code)
		code, out := call(tokenB, http.MethodGet, "/api/seller/promotions/stats", "")
		assert.Equal(t, http.StatusOK, code)
		assert.NotContains(t, string(out), created.ID)
	})
}
