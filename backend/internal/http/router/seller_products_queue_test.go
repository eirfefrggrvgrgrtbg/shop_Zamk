package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// SELLER.PRODUCTS.UX.1: the seller product list tells the truth about what a buyer can order. Real router, disposable
// database. The availability filter and the visibility flags are checked against the Shop's own public catalog.

func TestSellerProductsAvailability(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		JWT:  config.JWTConfig{AccessTokenSecret: "test-secret", RefreshTokenSecret: "test-secret-refresh", AccessTokenTTLMinutes: 60, RefreshTokenTTLDays: 7},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
	}
	pg, err := testutil.NewTrackedClient(t, ctx, testDBURL)
	require.NoError(t, err)
	defer pg.Close()
	testutil.AssertTestDatabase(t, pg.Pool)
	rc, err := redis.NewClient(ctx, "localhost:6379", "", 9)
	require.NoError(t, err)
	defer rc.Close()
	r, cancel := app.BuildRouter(ctx, cfg, pg, rc, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	defer cancel()
	tokens := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	exec := func(sql string, args ...any) {
		_, err := pg.Pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	user := func(role string) (uuid.UUID, string) {
		id := uuid.New()
		exec(`INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at) VALUES ($1,$2,$3,'Тест','hash',$4,'active',now(),now())`, id, id.String()+"@t.test", "7999"+id.String()[:7], role)
		tok, err := tokens.GenerateAccessToken(id, id.String()+"@t.test", role)
		require.NoError(t, err)
		return id, tok
	}
	call := func(method, path, token string, body any) (int, string) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req := httptest.NewRequest(method, path, rd)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr.Code, rr.Body.String()
	}

	cat := uuid.New()
	exec(`INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1,'Cat',$2,now(),now())`, cat, uuid.NewString())
	type seller struct {
		id    uuid.UUID
		token string
	}
	mkSeller := func(name string) seller {
		sid := uuid.New()
		exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1,$2,$3,$4,'active',now(),now())`, sid, name, uuid.NewString(), uuid.NewString()+"@ex.com")
		uid, tok := user("seller")
		exec(`INSERT INTO seller_users (id, seller_id, user_id) VALUES ($1,$2,$3)`, uuid.New(), sid, uid)
		return seller{sid, tok}
	}
	a, b := mkSeller("Seller A"), mkSeller("Seller B")

	// variant: free stock = total - reserved; active toggles the SKU
	type variant struct {
		total, reserved int
		inactive        bool
	}
	mkProduct := func(s seller, title, status string, minutesAgo int, vs ...variant) uuid.UUID {
		pid := uuid.New()
		exec(`INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,150000,$6, now() - make_interval(mins => $7), now())`, pid, s.id, cat, title, uuid.NewString(), status, minutesAgo)
		for i, v := range vs {
			vid := uuid.New()
			exec(`INSERT INTO product_variants (id, product_id, size, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1,$2,$3,$4,$4,$5,150000,$6,now(),now())`, vid, pid, []string{"S", "M", "L"}[i], uuid.NewString(), uuid.NewString(), !v.inactive)
			exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1,$2,$3,$4,$5,$6)`, uuid.New(), pid, vid, s.id, v.total, v.reserved)
		}
		return pid
	}
	ok := variant{total: 5}
	pAvail := mkProduct(a, "A-Доступный", "published", 1, ok, variant{total: 5}, variant{total: 5})            // 3 SKUs, 15 free: JOIN must not triplicate
	pNoStock := mkProduct(a, "A-Без остатка", "published", 2, variant{total: 3, reserved: 3})                  // published, nothing free
	pOneUnit := mkProduct(a, "A-Одна единица", "published", 3, variant{total: 1})                              // 1 free < storefront minimum 2
	pPartial := mkProduct(a, "A-Часть SKU", "published", 4, variant{total: 4}, variant{total: 2, reserved: 2}) // one SKU sold out
	pInactiveOnly := mkProduct(a, "A-SKU выключен", "published", 5, variant{total: 9, inactive: true})
	pDraft := mkProduct(a, "A-Черновик", "draft", 6, ok)
	pPending := mkProduct(a, "A-На модерации", "pending_moderation", 7, ok)
	pReview := mkProduct(a, "A-На проверке", "in_review", 8, ok)
	pHidden := mkProduct(a, "A-Скрыт", "hidden", 9, ok)
	pApproved := mkProduct(a, "A-Одобрен без остатка", "approved", 10, variant{total: 0})
	pRejected := mkProduct(a, "A-Отклонён", "rejected", 11, ok)
	pBOnly := mkProduct(b, "B-Доступный", "published", 12, ok, ok)

	type variantDTO struct {
		IsActive       bool `json:"isActive"`
		AvailableStock int  `json:"availableStock"`
	}
	type productDTO struct {
		ID                string       `json:"id"`
		Title             string       `json:"title"`
		Status            string       `json:"status"`
		ActualVisibility  bool         `json:"actualVisibility"`
		VisibilityReasons []string     `json:"visibilityReasons"`
		AvailableStock    int          `json:"availableStock"`
		Variants          []variantDTO `json:"variants"`
	}
	type listDTO struct {
		Items      []productDTO `json:"items"`
		TotalCount int          `json:"totalCount"`
		Summary    *struct {
			Total          int `json:"total"`
			Published      int `json:"published"`
			Purchasable    int `json:"purchasable"`
			AwaitingReview int `json:"awaitingReview"`
		} `json:"summary"`
	}
	list := func(tok, q string) listDTO {
		code, body := call("GET", "/api/seller/products/"+q, tok, nil)
		require.Equal(t, http.StatusOK, code, body)
		var l listDTO
		require.NoError(t, json.Unmarshal([]byte(body), &l))
		return l
	}
	titles := func(l listDTO) []string {
		out := []string{}
		for _, p := range l.Items {
			out = append(out, p.Title)
		}
		return out
	}
	byTitle := func(l listDTO) map[string]productDTO {
		m := map[string]productDTO{}
		for _, p := range l.Items {
			m[p.Title] = p
		}
		return m
	}

	t.Run("A: the seller sees only own products, one row per product, with the real total and summary", func(t *testing.T) {
		l := list(a.token, "")
		assert.Equal(t, 11, l.TotalCount)
		assert.Len(t, l.Items, 11, "variants and stock rows must not duplicate products")
		for _, p := range l.Items {
			assert.True(t, strings.HasPrefix(p.Title, "A-"), p.Title)
		}
		require.NotNil(t, l.Summary)
		assert.Equal(t, 11, l.Summary.Total)
		assert.Equal(t, 5, l.Summary.Published)
		assert.Equal(t, 2, l.Summary.Purchasable) // A-Доступный and A-Часть SKU
		assert.Equal(t, 2, l.Summary.AwaitingReview)
	})

	t.Run("B: availability flags and reasons follow the canonical storefront rules", func(t *testing.T) {
		m := byTitle(list(a.token, ""))
		assert.True(t, m["A-Доступный"].ActualVisibility)
		assert.Empty(t, m["A-Доступный"].VisibilityReasons)
		assert.Equal(t, 15, m["A-Доступный"].AvailableStock)
		assert.Len(t, m["A-Доступный"].Variants, 3)

		assert.False(t, m["A-Без остатка"].ActualVisibility)
		assert.Equal(t, []string{"low_stock"}, m["A-Без остатка"].VisibilityReasons)
		assert.False(t, m["A-Одна единица"].ActualVisibility, "one free unit is below the storefront minimum")
		assert.Equal(t, []string{"low_stock"}, m["A-Одна единица"].VisibilityReasons)

		part := m["A-Часть SKU"]
		assert.True(t, part.ActualVisibility, "one SKU sold out, the product is still orderable")
		sellable := 0
		for _, v := range part.Variants {
			if v.IsActive && v.AvailableStock > 0 {
				sellable++
			}
		}
		assert.Equal(t, 1, sellable)
		assert.Equal(t, 2, len(part.Variants))

		assert.Contains(t, m["A-SKU выключен"].VisibilityReasons, "no_active_variants")
		assert.Contains(t, m["A-Черновик"].VisibilityReasons, "moderation_required")
		assert.Contains(t, m["A-На модерации"].VisibilityReasons, "moderation_required")
		assert.Contains(t, m["A-Скрыт"].VisibilityReasons, "product_hidden")
		assert.NotContains(t, m["A-Доступный"].VisibilityReasons, "seller_inactive", "the seller's real status is applied")
		assert.NotContains(t, m["A-Скрыт"].VisibilityReasons, "seller_inactive")
	})

	t.Run("C: the availability filter equals the Shop public catalog, with the matching count", func(t *testing.T) {
		avail := list(a.token, "?availability=available")
		assert.ElementsMatch(t, []string{"A-Доступный", "A-Часть SKU"}, titles(avail))
		assert.Equal(t, 2, avail.TotalCount)
		un := list(a.token, "?availability=unavailable")
		assert.Equal(t, 9, un.TotalCount)
		assert.Equal(t, 11, avail.TotalCount+un.TotalCount)

		code, body := call("GET", "/api/public/products?limit=100", "", nil)
		require.Equal(t, http.StatusOK, code, body)
		var pub struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &pub))
		shop := map[string]bool{}
		for _, it := range pub.Items {
			shop[it.ID] = true
		}
		for title, id := range map[string]uuid.UUID{"A-Доступный": pAvail, "A-Часть SKU": pPartial, "A-Без остатка": pNoStock, "A-Одна единица": pOneUnit, "A-SKU выключен": pInactiveOnly, "A-Черновик": pDraft, "A-На модерации": pPending, "A-На проверке": pReview, "A-Скрыт": pHidden, "A-Одобрен без остатка": pApproved, "A-Отклонён": pRejected} {
			p := byTitle(list(a.token, ""))[title]
			assert.Equal(t, shop[id.String()], p.ActualVisibility, "seller view and Shop disagree on "+title)
		}
		assert.True(t, shop[pBOnly.String()], "sanity: the Shop does list seller B's available product")
	})

	t.Run("D: status filters, including the closed list and moderation", func(t *testing.T) {
		assert.ElementsMatch(t, []string{"A-Доступный", "A-Без остатка", "A-Одна единица", "A-Часть SKU", "A-SKU выключен"}, titles(list(a.token, "?status=published")))
		assert.Equal(t, []string{"A-Скрыт"}, titles(list(a.token, "?status=hidden")))
		assert.Equal(t, []string{"A-Черновик"}, titles(list(a.token, "?status=draft")))
		assert.ElementsMatch(t, []string{"A-На модерации", "A-На проверке"}, titles(list(a.token, "?status=moderation")))
		l := list(a.token, "?status=published&availability=unavailable")
		assert.ElementsMatch(t, []string{"A-Без остатка", "A-Одна единица", "A-SKU выключен"}, titles(l))
		assert.Equal(t, 3, l.TotalCount)
		assert.Equal(t, 11, list(a.token, "?status=all&availability=all").TotalCount)
		code, _ := call("GET", "/api/seller/products/?status=%27%3B%20DROP%20TABLE%20products", a.token, nil)
		assert.Equal(t, http.StatusBadRequest, code)
		code, _ = call("GET", "/api/seller/products/?availability=maybe", a.token, nil)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("E: limit, offset and totalCount work together with filters, no skips or repeats", func(t *testing.T) {
		seen := map[string]int{}
		for off := 0; off < 11; off += 4 {
			l := list(a.token, "?limit=4&offset="+string(rune('0'+off/10))+string(rune('0'+off%10)))
			assert.Equal(t, 11, l.TotalCount)
			for _, p := range l.Items {
				seen[p.Title]++
			}
		}
		assert.Len(t, seen, 11)
		for title, n := range seen {
			assert.Equal(t, 1, n, title)
		}
		l := list(a.token, "?availability=unavailable&limit=4&offset=8")
		assert.Equal(t, 9, l.TotalCount)
		assert.Len(t, l.Items, 1)
		assert.Equal(t, []string{"A-Одна единица"}, titles(list(a.token, "?q=Одна")))
		assert.Equal(t, 1, list(a.token, "?q=Одна").TotalCount)
	})

	t.Run("F: seller B sees none of A's products and cannot read or change them by id", func(t *testing.T) {
		lb := list(b.token, "")
		assert.Equal(t, []string{"B-Доступный"}, titles(lb))
		assert.Equal(t, 1, lb.TotalCount)
		require.NotNil(t, lb.Summary)
		assert.Equal(t, 1, lb.Summary.Total)

		path := "/api/seller/products/" + pAvail.String()
		code, body := call("GET", path, b.token, nil)
		assert.Equal(t, http.StatusNotFound, code)
		assert.NotContains(t, body, "A-Доступный")
		code, _ = call("PATCH", path, b.token, map[string]any{"title": "Взломано"})
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = call("POST", path+"/submit-moderation", b.token, nil)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusForbidden}, code)
		code, _ = call("POST", path+"/archive", b.token, nil)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusForbidden}, code)
		code, _ = call("DELETE", path, b.token, nil)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusForbidden, http.StatusConflict}, code)
		var title, status string
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT title, status FROM products WHERE id=$1`, pAvail).Scan(&title, &status))
		assert.Equal(t, "A-Доступный", title)
		assert.Equal(t, "published", status)
	})

	t.Run("G: the seller cannot publish, approve or change stock through the API", func(t *testing.T) {
		path := "/api/seller/products/" + pDraft.String()
		code, _ := call("PATCH", path, a.token, map[string]any{"status": "published", "publishedAt": "2026-01-01T00:00:00Z", "totalStock": 999, "reservedStock": 0,
			"variants": []map[string]any{{"totalStock": 999, "availableStock": 999}}})
		_ = code
		var status string
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT status FROM products WHERE id=$1`, pDraft).Scan(&status))
		assert.Equal(t, "draft", status, "a seller PATCH must not change the moderation status")
		var total int
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(total_stock),0) FROM inventory_items WHERE product_id=$1`, pDraft).Scan(&total))
		assert.Equal(t, 5, total, "a seller PATCH must not write warehouse stock")

		for _, route := range []string{"/api/admin/moderation/products/" + pPending.String() + "/approve", "/api/admin/moderation/products/" + pPending.String() + "/start-review"} {
			code, _ := call("POST", route, a.token, map[string]any{})
			assert.Contains(t, []int{http.StatusForbidden, http.StatusUnauthorized}, code, route)
		}
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT status FROM products WHERE id=$1`, pPending).Scan(&status))
		assert.Equal(t, "pending_moderation", status)
		code, _ = call("PUT", "/api/seller/inventory/"+uuid.NewString(), a.token, map[string]any{"totalStock": 100})
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, code, "there is no seller write route for warehouse stock")
	})

	t.Run("H: anonymous and buyers are rejected; no buyer data in the seller product list", func(t *testing.T) {
		_, buyerTok := user("customer")
		code, _ := call("GET", "/api/seller/products/", "", nil)
		assert.Equal(t, http.StatusUnauthorized, code)
		code, _ = call("GET", "/api/seller/products/", buyerTok, nil)
		assert.Equal(t, http.StatusForbidden, code)
		_, body := call("GET", "/api/seller/products/", a.token, nil)
		for _, forbidden := range []string{"customerPhone", "customerEmail", "deliveryAddress", "receiving"} {
			assert.NotContains(t, body, forbidden)
		}
	})
}
