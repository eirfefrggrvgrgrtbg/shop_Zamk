package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
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

// SHOP.CART.ELIGIBILITY.1: a new or larger cart line must pass the canonical storefront eligibility the public catalog
// applies; lines already in the cart stay and can always be shrunk or removed; checkout is limited by the real
// reservation, not by the catalog's visibility threshold. Real router and transactions, disposable database.

func TestCartEligibilityAndCheckout(t *testing.T) {
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
	call := func(method, path, token string, body any, headers ...string) (int, string) {
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
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr.Code, rr.Body.String()
	}
	errCode := func(body string) string {
		var e struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal([]byte(body), &e)
		return e.Error.Code
	}

	cat, dm := uuid.New(), uuid.New()
	exec(`INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1,'Cat',$2,now(),now())`, cat, uuid.NewString())
	exec(`INSERT INTO delivery_methods (id, code, name, price_cents, is_active) VALUES ($1,$2,'Курьер',39000,true)`, dm, "c-"+uuid.NewString()[:8])
	mkSeller := func(status string) uuid.UUID {
		sid := uuid.New()
		exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1,'S',$2,$3,$4,now(),now())`, sid, uuid.NewString(), uuid.NewString()+"@ex.com", status)
		return sid
	}
	active, blockedSeller := mkSeller("active"), mkSeller("blocked")

	type variant struct {
		total, reserved int
		inactive        bool
	}
	type prod struct {
		id   uuid.UUID
		vids []uuid.UUID
	}
	mk := func(seller uuid.UUID, status string, vs ...variant) prod {
		p := prod{id: uuid.New()}
		exec(`INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1,$2,$3,'Товар',$4,100000,$5,now(),now())`, p.id, seller, cat, uuid.NewString(), status)
		for i, v := range vs {
			vid := uuid.New()
			exec(`INSERT INTO product_variants (id, product_id, size, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1,$2,$3,$4,$4,$5,100000,$6,now(),now())`, vid, p.id, []string{"S", "M", "L"}[i], uuid.NewString(), uuid.NewString(), !v.inactive)
			exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1,$2,$3,$4,$5,$6)`, uuid.New(), p.id, vid, seller, v.total, v.reserved)
			p.vids = append(p.vids, vid)
		}
		return p
	}
	setStock := func(vid uuid.UUID, total, reserved int) {
		exec(`UPDATE inventory_items SET total_stock=$2, reserved_stock=$3 WHERE product_variant_id=$1`, vid, total, reserved)
	}
	add := func(tok string, p prod, i, qty int) (int, string) {
		return call("POST", "/api/customer/cart/items", tok, map[string]any{"productId": p.id, "productVariantId": p.vids[i], "quantity": qty})
	}
	type cartLine struct {
		ID                string `json:"id"`
		ProductVariantID  string `json:"productVariantId"`
		Quantity          int    `json:"quantity"`
		AvailableQuantity int    `json:"availableQuantity"`
		UnavailableReason string `json:"unavailableReason"`
	}
	getCart := func(tok string) []cartLine {
		code, body := call("GET", "/api/customer/cart", tok, nil)
		require.Equal(t, http.StatusOK, code, body)
		var c struct {
			Items []cartLine `json:"items"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &c))
		return c.Items
	}
	checkout := func(tok, key string) (int, string) {
		hdr := []string{}
		if key != "" {
			hdr = []string{"Idempotency-Key", key}
		}
		return call("POST", "/api/customer/orders", tok, map[string]any{
			"customerName": "Покупатель", "customerPhone": "+79990000000", "customerEmail": "b@e.test",
			"deliveryAddress": "г. Москва, ул. Тестовая, 1", "deliveryMethodId": dm,
		}, hdr...)
	}
	reserved := func(vid uuid.UUID) int {
		var n int
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id=$1`, vid).Scan(&n))
		return n
	}
	orderCount := func(buyer uuid.UUID) int {
		var n int
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id=$1`, buyer).Scan(&n))
		return n
	}
	internalLeak := func(t *testing.T, body string) {
		for _, w := range []string{"SQLSTATE", "inventory_items", "pgx", "reserved_stock", "total_stock", "violates", "goroutine"} {
			assert.NotContains(t, body, w)
		}
	}

	t.Run("A + B + O: an eligible product is added; one free unit (hidden by the canonical rule) is refused as new, without 500", func(t *testing.T) {
		_, tok := user("customer")
		ok := mk(active, "published", variant{total: 5}, variant{total: 0})
		code, body := add(tok, ok, 0, 1)
		assert.Equal(t, http.StatusCreated, code, body)

		one := mk(active, "published", variant{total: 1})
		code, body = add(tok, one, 0, 1)
		assert.Equal(t, http.StatusConflict, code, body)
		assert.Equal(t, "product_unavailable", errCode(body))
		internalLeak(t, body)
		// the public catalog hides the very same product
		code, _ = call("GET", "/api/public/products/"+one.id.String(), "", nil)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = call("GET", "/api/public/products/"+ok.id.String(), "", nil)
		assert.Equal(t, http.StatusOK, code)
		assert.Len(t, getCart(tok), 1)
	})

	t.Run("C + D: hidden, blocked, draft, rejected, archived products and an inactive store are refused as new lines", func(t *testing.T) {
		_, tok := user("customer")
		for _, status := range []string{"hidden", "blocked", "draft", "rejected", "pending_moderation", "archived", "out_of_stock"} {
			code, body := add(tok, mk(active, status, variant{total: 9}), 0, 1)
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusConflict}, code, status+": "+body)
			internalLeak(t, body)
		}
		code, body := add(tok, mk(blockedSeller, "published", variant{total: 9}), 0, 1)
		assert.Equal(t, http.StatusConflict, code, body)
		assert.Equal(t, "product_unavailable", errCode(body))
		assert.Empty(t, getCart(tok))
	})

	t.Run("E + F: a switched-off SKU and a sold-out SKU are refused although the product is available through others", func(t *testing.T) {
		_, tok := user("customer")
		p := mk(active, "published", variant{total: 6}, variant{total: 4, inactive: true}, variant{total: 3, reserved: 3})
		code, body := add(tok, p, 1, 1)
		assert.Equal(t, http.StatusBadRequest, code, body) // inactive SKU
		code, body = add(tok, p, 2, 1)
		assert.Equal(t, http.StatusBadRequest, code, body) // sold-out SKU of an available product
		assert.Equal(t, "invalid_item", errCode(body))
		code, body = add(tok, p, 0, 7)
		assert.Equal(t, http.StatusBadRequest, code, body) // more than the SKU has
		code, body = add(tok, p, 0, 1)
		assert.Equal(t, http.StatusCreated, code, body)
		assert.Len(t, getCart(tok), 1)
	})

	t.Run("G + H + I: an existing line survives a stock drop, can be shrunk and removed, but not grown", func(t *testing.T) {
		_, tok := user("customer")
		p := mk(active, "published", variant{total: 6})
		code, body := add(tok, p, 0, 3)
		require.Equal(t, http.StatusCreated, code, body)
		setStock(p.vids[0], 6, 6) // nothing free any more: the product disappears from the catalog
		lines := getCart(tok)
		require.Len(t, lines, 1, "the line must not be removed automatically")
		assert.Equal(t, 3, lines[0].Quantity)
		assert.NotEmpty(t, lines[0].UnavailableReason)

		code, body = call("PATCH", "/api/customer/cart/items/"+lines[0].ID, tok, map[string]any{"quantity": 4})
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusConflict}, code, body)
		code, body = add(tok, p, 0, 1)
		assert.Equal(t, http.StatusConflict, code, body)
		code, body = call("PATCH", "/api/customer/cart/items/"+lines[0].ID, tok, map[string]any{"quantity": 2})
		assert.Equal(t, http.StatusOK, code, "lowering the quantity of an unavailable line must work: "+body)
		assert.Equal(t, 2, getCart(tok)[0].Quantity)
		code, _ = call("DELETE", "/api/customer/cart/items/"+lines[0].ID, tok, nil)
		assert.Equal(t, http.StatusOK, code)
		assert.Empty(t, getCart(tok))
	})

	t.Run("J: checkout never sells more than is free", func(t *testing.T) {
		buyer, tok := user("customer")
		p := mk(active, "published", variant{total: 6})
		code, body := add(tok, p, 0, 3)
		require.Equal(t, http.StatusCreated, code, body)
		setStock(p.vids[0], 6, 4) // 2 free, 3 wanted
		code, body = checkout(tok, "")
		assert.Equal(t, http.StatusConflict, code, body)
		assert.Equal(t, "insufficient_stock", errCode(body))
		internalLeak(t, body)
		assert.Equal(t, 0, orderCount(buyer))
		assert.Equal(t, 4, reserved(p.vids[0]))
		assert.Len(t, getCart(tok), 1, "a refused checkout keeps the cart")
	})

	t.Run("J2: checkout refuses a switched-off SKU, an unpublished product and an inactive store", func(t *testing.T) {
		for name, mut := range map[string]func(p prod){
			"sku off":     func(p prod) { exec(`UPDATE product_variants SET is_active=false WHERE id=$1`, p.vids[0]) },
			"unpublished": func(p prod) { exec(`UPDATE products SET status='hidden' WHERE id=$1`, p.id) },
			"blocked":     func(p prod) { exec(`UPDATE products SET status='blocked' WHERE id=$1`, p.id) },
			"store closed": func(p prod) {
				exec(`UPDATE sellers SET status='blocked' WHERE id=(SELECT seller_id FROM products WHERE id=$1)`, p.id)
			},
		} {
			buyer, tok := user("customer")
			s := mkSeller("active")
			p := mk(s, "published", variant{total: 6})
			code, body := add(tok, p, 0, 1)
			require.Equal(t, http.StatusCreated, code, body)
			mut(p)
			code, body = checkout(tok, "")
			assert.Equal(t, http.StatusConflict, code, name+": "+body)
			assert.Equal(t, "item_unavailable", errCode(body), name)
			assert.Equal(t, 0, orderCount(buyer), name)
			assert.Equal(t, 0, reserved(p.vids[0]), name)
		}
	})

	t.Run("L: the last unit of an existing cart can still be bought, although the catalog now hides the product", func(t *testing.T) {
		buyer, tok := user("customer")
		p := mk(active, "published", variant{total: 4})
		code, body := add(tok, p, 0, 1)
		require.Equal(t, http.StatusCreated, code, body)
		setStock(p.vids[0], 4, 3) // one free unit left: below the catalog's visibility minimum
		code, _ = call("GET", "/api/public/products/"+p.id.String(), "", nil)
		assert.Equal(t, http.StatusNotFound, code, "hidden from the catalog")
		code, body = add(tok, p, 0, 1)
		assert.Equal(t, http.StatusConflict, code, "no new line, no larger line: "+body)
		code, body = checkout(tok, "")
		assert.Equal(t, http.StatusCreated, code, "the unit already in the cart is bought: "+body)
		assert.Equal(t, 1, orderCount(buyer))
		assert.Equal(t, 4, reserved(p.vids[0]), "exactly the last free unit is reserved")
	})

	t.Run("K: two buyers racing for the last unit: one order, one reservation, no oversell", func(t *testing.T) {
		p := mk(active, "published", variant{total: 5})
		type buyer struct {
			id  uuid.UUID
			tok string
		}
		var bs []buyer
		for i := 0; i < 4; i++ {
			id, tok := user("customer")
			code, body := add(tok, p, 0, 1)
			require.Equal(t, http.StatusCreated, code, body)
			bs = append(bs, buyer{id, tok})
		}
		setStock(p.vids[0], 5, 4) // exactly one free unit, four carts hold it
		codes := make([]int, len(bs))
		bodies := make([]string, len(bs))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range bs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				codes[i], bodies[i] = checkout(bs[i].tok, "")
			}(i)
		}
		close(start)
		wg.Wait()
		created, refused := 0, 0
		for i, c := range codes {
			switch c {
			case http.StatusCreated:
				created++
			case http.StatusConflict:
				refused++
				assert.Equal(t, "insufficient_stock", errCode(bodies[i]))
			default:
				t.Errorf("unexpected status %d: %s", c, bodies[i])
			}
		}
		assert.Equal(t, 1, created)
		assert.Equal(t, len(bs)-1, refused)
		assert.Equal(t, 5, reserved(p.vids[0]), "never more than total_stock")
		var orders int
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM order_items WHERE product_variant_id=$1`, p.vids[0]).Scan(&orders))
		assert.Equal(t, 1, orders)
	})

	t.Run("M: the idempotency key still returns the same order and reserves once", func(t *testing.T) {
		buyer, tok := user("customer")
		p := mk(active, "published", variant{total: 6})
		code, body := add(tok, p, 0, 2)
		require.Equal(t, http.StatusCreated, code, body)
		key := uuid.NewString()
		code1, body1 := checkout(tok, key)
		code2, body2 := checkout(tok, key)
		assert.Equal(t, http.StatusCreated, code1, body1)
		assert.Contains(t, []int{http.StatusCreated, http.StatusOK}, code2, body2)
		var o1, o2 struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal([]byte(body1), &o1))
		require.NoError(t, json.Unmarshal([]byte(body2), &o2))
		assert.Equal(t, o1.ID, o2.ID)
		assert.Equal(t, 1, orderCount(buyer))
		assert.Equal(t, 2, reserved(p.vids[0]))
	})

	t.Run("O: the seller's availability verdict and the cart agree for new lines", func(t *testing.T) {
		sellerUser, sellerTok := user("seller")
		s := mkSeller("active")
		exec(`INSERT INTO seller_users (id, seller_id, user_id) VALUES ($1,$2,$3)`, uuid.New(), s, sellerUser)
		_, tok := user("customer")
		good := mk(s, "published", variant{total: 6})
		oneUnit := mk(s, "published", variant{total: 1})
		hidden := mk(s, "hidden", variant{total: 6})
		code, body := call("GET", "/api/seller/products/?availability=available", sellerTok, nil)
		require.Equal(t, http.StatusOK, code, body)
		var l struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &l))
		sellerSays := map[string]bool{}
		for _, it := range l.Items {
			sellerSays[it.ID] = true
		}
		for name, p := range map[string]prod{"good": good, "one unit": oneUnit, "hidden": hidden} {
			code, _ := add(tok, p, 0, 1)
			assert.Equal(t, sellerSays[p.id.String()], code == http.StatusCreated, "seller and cart disagree on "+name)
		}
		assert.True(t, sellerSays[good.id.String()])
	})
}
