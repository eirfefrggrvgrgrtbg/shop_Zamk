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

// FBO.DELIVERY.HANDOFF.1: the staff-only dispatch context carries the order's full delivery snapshot (orders.*), and
// nothing else — customers and sellers never get the new address fields. Real router, disposable database.

func TestDispatchContextDeliverySnapshot(t *testing.T) {
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
	rc, err := redis.NewClient(ctx, "localhost:6379", "", 9) // a logical database of its own: nothing shared is touched
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
	staff := func(perms ...string) string {
		id, tok := user("admin")
		_, err := setupRouterStaffWithPermissions(ctx, pg.Pool, id, "Role"+id.String()[:6], perms)
		require.NoError(t, err)
		return tok
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

	// data: two sellers, a category, one product each
	cat := uuid.New()
	exec(`INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1,'Cat',$2,now(),now())`, cat, uuid.NewString())
	mkSeller := func(name string) (uuid.UUID, uuid.UUID) {
		sid, pid := uuid.New(), uuid.New()
		exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1,$2,$3,$4,'active',now(),now())`, sid, name, uuid.NewString(), uuid.NewString()+"@ex.com")
		exec(`INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1,$2,$3,'Prod',$4,1000,'published',now(),now())`, pid, sid, cat, uuid.NewString())
		return sid, pid
	}
	sellerA, prodA := mkSeller("Seller A")
	sellerB, prodB := mkSeller("Seller B")
	variant := func(prod uuid.UUID) uuid.UUID {
		v := uuid.New()
		exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1,$2,$3,$3,$4,1000,true,now(),now())`, v, prod, uuid.NewString(), uuid.NewString())
		return v
	}
	const secretComment = "Домофон 4321, звонить за час"
	mkOrder := func(buyer uuid.UUID, number, name, phone, address string, postal, comment *string, sellers ...uuid.UUID) (uuid.UUID, []uuid.UUID) {
		oid := uuid.New()
		exec(`INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, delivery_postal_code, delivery_courier_comment, order_number)
			VALUES ($1,$2,'packed',1000,$3,$4,'e@e.test',$5,$6,$7,$8)`, oid, buyer, name, phone, address, postal, comment, number)
		var fuls []uuid.UUID
		for i, s := range sellers {
			f := uuid.New()
			exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at) VALUES ($1,$2,$3,'packed',1000,900,900, now() + make_interval(secs => $4::double precision))`, f, oid, s, float64(i))
			prod := map[uuid.UUID]uuid.UUID{sellerA: prodA, sellerB: prodB}[s]
			v := variant(prod)
			exec(`INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, picked_quantity) VALUES ($1,$2,$3,$4,$5,'Товар','slug',1000,1,1000,$6,1)`, uuid.New(), oid, prod, v, s, f)
			fuls = append(fuls, f)
		}
		return oid, fuls
	}
	shipment := func(oid, ful uuid.UUID, carrier, track string) {
		exec(`INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number) VALUES ($1,$2,$3,'pending',$4,$5)`, uuid.New(), oid, ful, carrier, track)
	}
	ptr := func(s string) *string { return &s }

	buyer1, buyer1Tok := user("customer")
	buyer2, _ := user("customer")
	// new order, two parcels (two sellers), different carriers
	o1, f1 := mkOrder(buyer1, "ORD-HO-1", "Анна Иванова", "+7 999 000-11-22", "г. Москва, ул. Тверская, д. 1, кв. 12", ptr("101000"), ptr(secretComment), sellerA, sellerB)
	shipment(o1, f1[0], "СДЭК", "TRK-HO-A")
	shipment(o1, f1[1], "Курьер ZAMK", "TRK-HO-B")
	// an old order: no postal code, no comment
	_, f2 := mkOrder(buyer2, "ORD-HO-2", "Борис Петров", "+7 900 111-22-33", "г. Казань, ул. Баумана, д. 5", nil, nil, sellerA)

	dispatch := staff("warehouse.dispatch")
	reader := staff("fulfillment.read")
	noRight := staff("orders.update_status")
	ordersOnly := staff("orders.read")
	_, sellerTok := user("seller")
	_, customerTok := user("customer")
	path := func(f uuid.UUID) string { return "/api/admin/fulfillments/" + f.String() + "/dispatch-context" }

	type ctxDTO struct {
		OrderID, OrderNumber, DeliveryAddress, CustomerName, CustomerPhone, RecipientName, RecipientPhone string
		DeliveryPostalCode, DeliveryCourierComment, Carrier, TrackingNumber                           *string
		ParcelNumber, ParcelsTotal                                                                    int
	}
	read := func(f uuid.UUID, tok string) ctxDTO {
		code, body := get(path(f), tok)
		require.Equal(t, http.StatusOK, code, body)
		var d ctxDTO
		require.NoError(t, json.Unmarshal([]byte(body), &d))
		return d
	}

	t.Run("A + C: staff with the dispatch right gets the full snapshot of each parcel of a two-parcel order", func(t *testing.T) {
		a, b := read(f1[0], dispatch), read(f1[1], dispatch)
		for _, d := range []ctxDTO{a, b} {
			assert.Equal(t, o1.String(), d.OrderID)
			assert.Equal(t, "ORD-HO-1", d.OrderNumber)
			assert.Equal(t, "Анна Иванова", d.RecipientName)
			assert.Equal(t, "+7 999 000-11-22", d.RecipientPhone)
			assert.Equal(t, "г. Москва, ул. Тверская, д. 1, кв. 12", d.DeliveryAddress)
			require.NotNil(t, d.DeliveryPostalCode)
			assert.Equal(t, "101000", *d.DeliveryPostalCode)
			require.NotNil(t, d.DeliveryCourierComment)
			assert.Equal(t, secretComment, *d.DeliveryCourierComment)
			assert.Equal(t, 2, d.ParcelsTotal)
		}
		assert.Equal(t, 1, a.ParcelNumber)
		assert.Equal(t, 2, b.ParcelNumber)
		assert.Equal(t, []string{"СДЭК", "TRK-HO-A"}, []string{*a.Carrier, *a.TrackingNumber})
		assert.Equal(t, []string{"Курьер ZAMK", "TRK-HO-B"}, []string{*b.Carrier, *b.TrackingNumber}, "each parcel has its own carrier and track")
	})

	t.Run("B + D: an old order has no postal code or comment and still works; orders are not mixed", func(t *testing.T) {
		d := read(f2[0], dispatch)
		assert.Equal(t, "ORD-HO-2", d.OrderNumber)
		assert.Equal(t, "Борис Петров", d.RecipientName)
		assert.Equal(t, "г. Казань, ул. Баумана, д. 5", d.DeliveryAddress)
		assert.Nil(t, d.DeliveryPostalCode)
		assert.Nil(t, d.DeliveryCourierComment)
		assert.Nil(t, d.Carrier)
		assert.Equal(t, 1, d.ParcelsTotal)
		_, raw := get(path(f2[0]), dispatch)
		assert.NotContains(t, raw, secretComment)
		assert.NotContains(t, raw, "101000")
		assert.NotContains(t, raw, "Анна")
	})

	t.Run("E: rights — fulfillment.read also reads; others get 403; no session 401; unknown parcel 404", func(t *testing.T) {
		assert.Equal(t, "ORD-HO-1", read(f1[0], reader).OrderNumber)
		for name, tok := range map[string]string{"staff without the right": noRight, "orders.read only": ordersOnly, "seller": sellerTok, "customer": customerTok, "owner-buyer": buyer1Tok} {
			code, body := get(path(f1[0]), tok)
			assert.Equal(t, http.StatusForbidden, code, name)
			assert.NotContains(t, body, secretComment, name)
			assert.NotContains(t, body, "101000", name)
		}
		code, body := get(path(f1[0]), "")
		assert.Equal(t, http.StatusUnauthorized, code)
		assert.NotContains(t, body, secretComment)
		code, _ = get(path(uuid.New()), dispatch)
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("G: the customer's own fulfillments API shows no delivery snapshot, no warehouse data, no recipient fields", func(t *testing.T) {
		code, body := get("/api/customer/orders/"+o1.String()+"/fulfillments", buyer1Tok)
		require.Equal(t, http.StatusOK, code, body)
		for _, k := range []string{"deliveryPostalCode", "deliveryCourierComment", "deliveryAddress", "recipientPhone", "barcode", "allocationMode", "allocatedUnits", secretComment} {
			assert.NotContains(t, body, k)
		}
		// someone else's order: no data at all
		code, body = get("/api/customer/orders/"+o1.String()+"/fulfillments", customerTok)
		assert.Equal(t, http.StatusNotFound, code)
		assert.NotContains(t, body, secretComment)
	})

	t.Run("F: the seller side gets none of the new fields", func(t *testing.T) {
		sellerUser, sellerToken := user("seller")
		exec(`INSERT INTO seller_users (id, seller_id, user_id) VALUES (gen_random_uuid(), $1, $2)`, sellerA, sellerUser)
		sellerToken, err := tokens.GenerateAccessToken(sellerUser, sellerUser.String()+"@t.test", "seller")
		require.NoError(t, err)
		for _, p := range []string{"/api/seller/orders/" + o1.String(), "/api/seller/orders/" + o1.String() + "/shipment", "/api/seller/orders/"} {
			_, body := get(p, sellerToken)
			for _, k := range []string{"deliveryPostalCode", "deliveryCourierComment", "postalCode", "courierComment", secretComment, "101000", "Тверская", "+7 999 000-11-22"} {
				assert.False(t, strings.Contains(body, k), "%s leaked %q: %s", p, k, body)
			}
		}
		code, _ := get(path(f1[0]), sellerToken)
		assert.Equal(t, http.StatusForbidden, code, "a seller cannot read the staff context, even of their own parcel")
	})

	t.Run("H: the snapshot does not follow the address book", func(t *testing.T) {
		before := func() string { _, b := get(path(f1[0]), dispatch); return b }()
		exec(`INSERT INTO customer_addresses (id, user_id, recipient_name, phone, city, street, house, postal_code, comment, is_default) VALUES ($1,$2,'Другой','+7 900 000-00-00','Тула','Новая','9','300000','новый комментарий',true)`, uuid.New(), buyer1)
		exec(`UPDATE customer_addresses SET city='Омск', postal_code='644000', comment='ещё один', recipient_name='Третий' WHERE user_id=$1`, buyer1)
		exec(`DELETE FROM customer_addresses WHERE user_id=$1`, buyer1)
		assert.JSONEq(t, before, func() string { _, b := get(path(f1[0]), dispatch); return b }())
	})

	t.Run("a long comment is returned in full (the screen wraps it)", func(t *testing.T) {
		long := strings.Repeat("очень длинный комментарий ", 18)
		long = strings.TrimSpace(long)
		o, f := mkOrder(buyer2, "ORD-HO-3", "Имя", "+7 900 111-22-33", "г. Тула, "+strings.Repeat("длинная ", 30), ptr("101000"), ptr(long), sellerA)
		_ = o
		d := read(f[0], dispatch)
		assert.Equal(t, long, *d.DeliveryCourierComment)
	})
}
