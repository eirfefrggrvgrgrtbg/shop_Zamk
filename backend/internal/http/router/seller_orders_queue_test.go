package router_test

import (
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

// SELLER.ORDERS.UX.1: the seller order queue. Real router, disposable database: every seller sees only their own
// lines, the stage and return state are per item, the filters and totalCount are server-side.

func TestSellerOrdersQueue(t *testing.T) {
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
	get := func(path, token string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
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
		id, prod uuid.UUID
		token    string
	}
	mkSeller := func(name string) seller {
		sid, pid := uuid.New(), uuid.New()
		exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1,$2,$3,$4,'active',now(),now())`, sid, name, uuid.NewString(), uuid.NewString()+"@ex.com")
		exec(`INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1,$2,$3,'Prod',$4,1000,'published',now(),now())`, pid, sid, cat, uuid.NewString())
		uid, tok := user("seller")
		exec(`INSERT INTO seller_users (id, seller_id, user_id) VALUES ($1,$2,$3)`, uuid.New(), sid, uid)
		return seller{sid, pid, tok}
	}
	a, b, empty := mkSeller("Seller A"), mkSeller("Seller B"), mkSeller("Seller Empty")
	buyer, buyerTok := user("customer")

	// a line of a seller in an order; the order status is the fulfillment status of every seller in it
	type line struct {
		s     seller
		title string
		qty   int
		cents int64
	}
	mkOrder := func(number, status string, minutesAgo int, lines ...line) (uuid.UUID, map[string]uuid.UUID) {
		oid := uuid.New()
		exec(`INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, order_number, created_at)
			VALUES ($1,$2,$3,5000,'Покупатель Секретов','+7 999 123-45-67','buyer-secret@e.test','г. Секретск, ул. Тайная, 1',$4, now() - make_interval(mins => $5))`, oid, buyer, status, number, minutesAgo)
		fuls := map[uuid.UUID]uuid.UUID{}
		items := map[string]uuid.UUID{}
		for _, l := range lines {
			f, ok := fuls[l.s.id]
			if !ok {
				f = uuid.New()
				fuls[l.s.id] = f
				exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents) VALUES ($1,$2,$3,$4,1000,900,900)`, f, oid, l.s.id, status)
			}
			v := uuid.New()
			exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1,$2,$3,$3,$4,1000,true,now(),now())`, v, l.s.prod, uuid.NewString(), uuid.NewString())
			iid := uuid.New()
			exec(`INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1,$2,$3,$4,$5,$6,'slug',$7,$8,$9,$10)`,
				iid, oid, l.s.prod, v, l.s.id, l.title, l.cents, l.qty, l.cents*int64(l.qty), f)
			items[l.title] = iid
		}
		return oid, items
	}
	mkReturn := func(oid uuid.UUID, status string, item uuid.UUID, qty int) {
		rid := uuid.New()
		var ful uuid.UUID
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT order_fulfillment_id FROM order_items WHERE id = $1`, item).Scan(&ful))
		exec(`INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason) VALUES ($1,$2,$3,$4,$5,'size')`, rid, oid, ful, buyer, status)
		exec(`INSERT INTO return_items (id, return_id, order_item_id, quantity) VALUES ($1,$2,$3,$4)`, uuid.New(), rid, item, qty)
	}

	// A's queue (newest first): paid, assembling, shipped(mixed with B), delivered clean, delivered with a partial return,
	// delivered with a return in progress, cancelled
	oPaid, _ := mkOrder("ORD-Q-PAID", "paid", 1, line{a, "A-Платье", 1, 1000})
	oAsm, _ := mkOrder("ORD-Q-ASM", "assembling", 2, line{a, "A-Куртка", 1, 2000})
	oMixed, _ := mkOrder("ORD-Q-MIX", "shipped", 3, line{a, "A-Шарф", 2, 500}, line{b, "B-ТАЙНЫЕ-Ботинки", 1, 7777})
	oDel, _ := mkOrder("ORD-Q-DEL", "delivered", 4, line{a, "A-Джинсы", 1, 3000})
	oPart, partItems := mkOrder("ORD-Q-PART", "delivered", 5, line{a, "A-Футболка", 3, 1000}, line{a, "A-Кепка", 1, 900})
	mkReturn(oPart, "refunded", partItems["A-Футболка"], 1)
	oOpen, openItems := mkOrder("ORD-Q-OPEN", "delivered", 6, line{a, "A-Брюки", 1, 4000})
	mkReturn(oOpen, "requested", openItems["A-Брюки"], 1)
	oRej, rejItems := mkOrder("ORD-Q-REJ", "delivered", 7, line{a, "A-Носки", 1, 100})
	mkReturn(oRej, "rejected", rejItems["A-Носки"], 1)
	oCanc, _ := mkOrder("ORD-Q-CANC", "cancelled", 8, line{a, "A-Пальто", 1, 9000})
	_ = []uuid.UUID{oPaid, oAsm, oDel, oCanc}
	oBOnly, _ := mkOrder("ORD-Q-BONLY", "paid", 9, line{b, "B-Одиночный", 1, 1234})

	type itemDTO struct {
		Title                 string `json:"title"`
		Quantity              int    `json:"quantity"`
		FulfillmentStage      string `json:"fulfillmentStage"`
		ReturnRequestedUnits  int    `json:"returnRequestedUnits"`
		ReturnInProgressUnits int    `json:"returnInProgressUnits"`
		ReturnRefundedUnits   int    `json:"returnRefundedUnits"`
	}
	type orderDTO struct {
		ID                string    `json:"id"`
		OrderNumber       string    `json:"orderNumber"`
		FulfillmentStage  string    `json:"fulfillmentStage"`
		NextStepOwner     string    `json:"nextStepOwner"`
		HasReturns        bool      `json:"hasReturns"`
		SellerUnits       int       `json:"sellerUnits"`
		SellerGrossAmount int64     `json:"sellerGrossAmount"`
		Items             []itemDTO `json:"items"`
	}
	type listDTO struct {
		Items      []orderDTO `json:"items"`
		TotalCount int        `json:"totalCount"`
		Limit      int        `json:"limit"`
		Offset     int        `json:"offset"`
		Filter     string     `json:"filter"`
	}
	list := func(tok, query string) listDTO {
		code, body := get("/api/seller/orders/"+query, tok)
		require.Equal(t, http.StatusOK, code, body)
		var l listDTO
		require.NoError(t, json.Unmarshal([]byte(body), &l))
		return l
	}
	numbers := func(l listDTO) []string {
		out := []string{}
		for _, o := range l.Items {
			out = append(out, o.OrderNumber)
		}
		return out
	}

	t.Run("A: seller with orders sees all own orders, newest first, with the real total", func(t *testing.T) {
		l := list(a.token, "")
		assert.Equal(t, 8, l.TotalCount)
		assert.Equal(t, []string{"ORD-Q-PAID", "ORD-Q-ASM", "ORD-Q-MIX", "ORD-Q-DEL", "ORD-Q-PART", "ORD-Q-OPEN", "ORD-Q-REJ", "ORD-Q-CANC"}, numbers(l))
		assert.Equal(t, "all", l.Filter)
	})

	t.Run("B: seller without orders gets an empty list", func(t *testing.T) {
		l := list(empty.token, "")
		assert.Equal(t, 0, l.TotalCount)
		assert.Empty(t, l.Items)
	})

	t.Run("C: a mixed order shows each seller only their own lines and amounts", func(t *testing.T) {
		_, bodyA := get("/api/seller/orders/"+oMixed.String(), a.token)
		assert.Contains(t, bodyA, "A-Шарф")
		assert.NotContains(t, bodyA, "B-ТАЙНЫЕ")
		assert.NotContains(t, bodyA, "7777")
		var oa orderDTO
		require.NoError(t, json.Unmarshal([]byte(bodyA), &oa))
		assert.Equal(t, 2, oa.SellerUnits)
		assert.EqualValues(t, 1000, oa.SellerGrossAmount)
		require.Len(t, oa.Items, 1)

		_, bodyB := get("/api/seller/orders/"+oMixed.String(), b.token)
		assert.Contains(t, bodyB, "B-ТАЙНЫЕ")
		assert.NotContains(t, bodyB, "A-Шарф")
		lb := list(b.token, "")
		assert.Equal(t, 2, lb.TotalCount) // the mixed order and B's own
		for _, o := range lb.Items {
			assert.EqualValues(t, map[string]int64{"ORD-Q-MIX": 7777, "ORD-Q-BONLY": 1234}[o.OrderNumber], o.SellerGrossAmount)
		}
	})

	t.Run("D: stages and who acts next come from the real state", func(t *testing.T) {
		l := list(a.token, "")
		stages := map[string]string{}
		owners := map[string]string{}
		for _, o := range l.Items {
			stages[o.OrderNumber] = o.FulfillmentStage
			owners[o.OrderNumber] = o.NextStepOwner
			for _, it := range o.Items {
				assert.Equal(t, o.FulfillmentStage, it.FulfillmentStage)
			}
		}
		assert.Equal(t, "paid", stages["ORD-Q-PAID"])
		assert.Equal(t, "processing", stages["ORD-Q-ASM"])
		assert.Equal(t, "shipped", stages["ORD-Q-MIX"])
		assert.Equal(t, "delivered", stages["ORD-Q-DEL"])
		assert.Equal(t, "cancelled", stages["ORD-Q-CANC"])
		assert.Equal(t, "zamk", owners["ORD-Q-ASM"])
		assert.Equal(t, "zamk", owners["ORD-Q-MIX"])
		assert.Equal(t, "none", owners["ORD-Q-DEL"])
	})

	t.Run("E: a partial return is per item and does not change the order stage", func(t *testing.T) {
		_, body := get("/api/seller/orders/"+oPart.String(), a.token)
		var o orderDTO
		require.NoError(t, json.Unmarshal([]byte(body), &o))
		assert.Equal(t, "delivered", o.FulfillmentStage)
		assert.True(t, o.HasReturns)
		byTitle := map[string]itemDTO{}
		for _, it := range o.Items {
			byTitle[it.Title] = it
		}
		assert.Equal(t, 1, byTitle["A-Футболка"].ReturnRefundedUnits)
		assert.Equal(t, 0, byTitle["A-Футболка"].ReturnRequestedUnits)
		assert.Equal(t, 0, byTitle["A-Кепка"].ReturnRefundedUnits)
		assert.Equal(t, "delivered", byTitle["A-Кепка"].FulfillmentStage)

		_, body = get("/api/seller/orders/"+oOpen.String(), a.token)
		require.NoError(t, json.Unmarshal([]byte(body), &o))
		assert.Equal(t, "delivered", o.FulfillmentStage)
		assert.Equal(t, 1, o.Items[0].ReturnRequestedUnits, "a request is not a refund")
		assert.Equal(t, 0, o.Items[0].ReturnRefundedUnits)
	})

	t.Run("F: a delivered order without a return, and one whose return was rejected, have no return", func(t *testing.T) {
		for _, id := range []uuid.UUID{oDel, oRej} {
			_, body := get("/api/seller/orders/"+id.String(), a.token)
			var o orderDTO
			require.NoError(t, json.Unmarshal([]byte(body), &o))
			assert.False(t, o.HasReturns)
			assert.Equal(t, "delivered", o.FulfillmentStage)
		}
	})

	t.Run("G: filters and pagination are server-side and the total follows the filter", func(t *testing.T) {
		assert.Equal(t, []string{"ORD-Q-PAID"}, numbers(list(a.token, "?filter=new")))
		assert.Equal(t, []string{"ORD-Q-ASM"}, numbers(list(a.token, "?filter=processing")))
		assert.Equal(t, []string{"ORD-Q-MIX"}, numbers(list(a.token, "?filter=shipped")))
		assert.Equal(t, []string{"ORD-Q-DEL", "ORD-Q-PART", "ORD-Q-OPEN", "ORD-Q-REJ"}, numbers(list(a.token, "?filter=delivered")))
		assert.Equal(t, []string{"ORD-Q-PART", "ORD-Q-OPEN"}, numbers(list(a.token, "?filter=returns")))

		p1 := list(a.token, "?limit=3&offset=0")
		p2 := list(a.token, "?limit=3&offset=3")
		p3 := list(a.token, "?limit=3&offset=6")
		assert.Equal(t, 8, p1.TotalCount)
		assert.Equal(t, 8, p3.TotalCount)
		assert.Equal(t, 3, p1.Limit)
		assert.Equal(t, 3, p2.Limit)
		assert.Len(t, p1.Items, 3)
		assert.Len(t, p2.Items, 3)
		assert.Len(t, p3.Items, 2)
		d := list(a.token, "?filter=delivered&limit=2&offset=2")
		assert.Equal(t, 4, d.TotalCount)
		assert.Equal(t, []string{"ORD-Q-OPEN", "ORD-Q-REJ"}, numbers(d))

		code, _ := get("/api/seller/orders/?filter=%27%3B%20DROP%20TABLE%20orders", a.token)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("H: no access to other sellers' orders, no ID guessing, no Seller API for buyers or anonymous", func(t *testing.T) {
		code, body := get("/api/seller/orders/"+oBOnly.String(), a.token)
		assert.Equal(t, http.StatusNotFound, code)
		assert.NotContains(t, body, "B-Одиночный")
		code, _ = get("/api/seller/orders/"+uuid.NewString(), a.token)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = get("/api/seller/orders/"+oMixed.String(), empty.token)
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, 1, len(list(b.token, "?filter=new").Items)) // only B's own paid order, not A's paid one
		code, _ = get("/api/seller/orders/", buyerTok)
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = get("/api/seller/orders/", "")
		assert.Equal(t, http.StatusUnauthorized, code)
	})

	t.Run("I: the seller DTO carries no buyer contact data and no warehouse internals", func(t *testing.T) {
		_, body := get("/api/seller/orders/", a.token)
		for _, forbidden := range []string{"Секретов", "+7 999 123", "buyer-secret", "Секретск", "customerName", "customerPhone", "customerEmail", "deliveryAddress", "orderFulfillmentId", "receiving", "picked"} {
			assert.False(t, strings.Contains(body, forbidden), forbidden)
		}
	})

	t.Run("J: the shipment endpoint never returns another seller's parcel", func(t *testing.T) {
		oShip, _ := mkOrder("ORD-Q-SHIP", "shipped", 10, line{a, "A-Шапка", 1, 100}, line{b, "B-Шапка", 1, 100})
		var fa, fb uuid.UUID
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT id FROM order_fulfillments WHERE order_id=$1 AND seller_id=$2`, oShip, a.id).Scan(&fa))
		require.NoError(t, pg.Pool.QueryRow(ctx, `SELECT id FROM order_fulfillments WHERE order_id=$1 AND seller_id=$2`, oShip, b.id).Scan(&fb))
		exec(`INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, created_at) VALUES ($1,$2,$3,'shipped','CarrierA','TRK-A-ONLY', now() - interval '1 minute')`, uuid.New(), oShip, fa)
		exec(`INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, created_at) VALUES ($1,$2,$3,'shipped','CarrierB','TRK-B-ONLY', now())`, uuid.New(), oShip, fb)
		_, bodyA := get("/api/seller/orders/"+oShip.String()+"/shipment", a.token)
		_, bodyB := get("/api/seller/orders/"+oShip.String()+"/shipment", b.token)
		assert.Contains(t, bodyA, "TRK-A-ONLY")
		assert.NotContains(t, bodyA, "TRK-B-ONLY")
		assert.Contains(t, bodyB, "TRK-B-ONLY")
		assert.NotContains(t, bodyB, "TRK-A-ONLY")
	})
}
