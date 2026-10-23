package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// CHECKOUT.E2E.1: the customer checkout (cart → order) on a disposable database: one order with server prices and
// reservations; out-of-stock, unavailable products and changed prices are refused cleanly with no order and no
// reservation; repeated and concurrent submits never create a second order or reservation.

type checkoutFixture struct {
	t      *testing.T
	ctx    context.Context
	db     *pgxpool.Pool
	svc    *Service
	h      *Handler
	seller uuid.UUID
	cat    uuid.UUID
	brand  uuid.UUID
	dm     uuid.UUID
}

func newCheckoutFixture(t *testing.T) *checkoutFixture {
	t.Helper()
	ctx := context.Background()
	db, err := pgxpool.New(ctx, testutil.DisposableDatabaseURL(t))
	require.NoError(t, err)
	t.Cleanup(db.Close)
	testutil.MustBeDisposableDatabase(t, db)
	f := &checkoutFixture{t: t, ctx: ctx, db: db, seller: uuid.New(), cat: uuid.New(), brand: uuid.New(), dm: uuid.New()}
	f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Checkout Brand', $2, 'c@ex.com', 'active')`, f.seller, "co-"+f.seller.String()[:8])
	f.exec(`INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', $2)`, f.cat, "co-cat-"+f.cat.String()[:8])
	f.exec(`INSERT INTO brands (id, name, slug) VALUES ($1, 'br', $2)`, f.brand, "co-br-"+f.brand.String()[:8])
	f.exec(`INSERT INTO delivery_methods (id, code, name, price_cents, is_active) VALUES ($1, $2, 'Курьер', 39000, true)`, f.dm, "co_"+f.dm.String()[:8])
	pg := &postgres.Client{Pool: db}
	invSvc := inventory.NewService(inventory.NewRepository(db), nil, pg)
	f.svc = NewService(NewRepository(db), cart.NewRepository(db), invSvc, pg, &config.Config{Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500}})
	f.h = NewHandler(f.svc)
	return f
}

func (f *checkoutFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *checkoutFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&n), sql)
	return n
}

// variant creates a published product with one variant (price in kopecks) and stock.
func (f *checkoutFixture) variant(price int64, stock int) (uuid.UUID, uuid.UUID) {
	p, v := uuid.New(), uuid.New()
	f.exec(`INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents) VALUES ($1,$2,$3,$4,'Платье',$5,'d','published',$6)`, p, f.seller, f.cat, f.brand, "co-p-"+p.String()[:8], price)
	f.exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, price_cents, is_active) VALUES ($1,$2,$3,$3,$4,true)`, v, p, "CO-"+v.String()[:8], price)
	f.exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,$4,0)`, p, v, f.seller, stock)
	return p, v
}

// buyer creates a customer with a cart holding the given (product, variant, quantity) lines.
func (f *checkoutFixture) buyer(lines ...[3]any) uuid.UUID {
	u, c := uuid.New(), uuid.New()
	f.exec(`INSERT INTO users (id, name, email, password_hash, role) VALUES ($1,'Покупатель',$2,'h','customer')`, u, u.String()[:8]+"@buyer.test")
	f.exec(`INSERT INTO carts (id, user_id) VALUES ($1,$2)`, c, u)
	for _, l := range lines {
		f.exec(`INSERT INTO cart_items (id, cart_id, product_id, product_variant_id, quantity) VALUES (gen_random_uuid(),$1,$2,$3,$4)`, c, l[0], l[1], l[2])
	}
	return u
}

func (f *checkoutFixture) req() CreateOrderRequest {
	return CreateOrderRequest{CustomerName: "Анна Покупатель", CustomerPhone: "+79990000001", CustomerEmail: "anna@buyer.test", DeliveryAddress: "г. Москва, ул. Тестовая, д. 1", DeliveryMethodID: f.dm}
}

func (f *checkoutFixture) reserved(v uuid.UUID) int {
	return f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id=$1`, v)
}
func (f *checkoutFixture) orders(u uuid.UUID) int {
	return f.count(`SELECT count(*) FROM orders WHERE user_id=$1`, u)
}
func (f *checkoutFixture) activeReservations(v uuid.UUID) int {
	return f.count(`SELECT count(*) FROM reservations WHERE product_variant_id=$1 AND status='active'`, v)
}

// post calls the real handler and returns status, error code and message.
func (f *checkoutFixture) post(u uuid.UUID, body CreateOrderRequest, key string) (int, string, string) {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(b))
	r = r.WithContext(context.WithValue(r.Context(), "userID", u))
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	f.h.CreateOrder(w, r)
	var resp struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp.Error.Code, resp.Error.Message
}

func TestCheckoutE2E(t *testing.T) {
	t.Run("happy path: one order, server price, delivery in the total, reservation, cart cleared", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p, v := f.variant(450000, 5)
		u := f.buyer([3]any{p, v, 2})
		key := uuid.New()
		o, err := f.svc.CreateOrder(f.ctx, u, f.req(), &key)
		require.NoError(t, err)
		require.Equal(t, "awaiting_payment", o.Status)
		require.Len(t, o.Items, 1)
		require.Equal(t, int64(450000), o.Items[0].PriceCents)
		require.Equal(t, 2, o.Items[0].Quantity)
		require.Equal(t, int64(2*450000+39000), o.TotalPriceCents)
		require.Equal(t, 2, f.reserved(v))
		require.Equal(t, 1, f.activeReservations(v))
		require.Equal(t, 0, f.count(`SELECT count(*) FROM cart_items ci JOIN carts c ON c.id=ci.cart_id WHERE c.user_id=$1`, u))
		got, err := f.svc.ListCustomerOrders(f.ctx, u, 10, 0)
		require.NoError(t, err)
		require.Len(t, got, 1, "the order is in the customer's history")
	})

	t.Run("repeat with the same key (network retry) returns the same order, nothing new", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p, v := f.variant(300000, 5)
		u := f.buyer([3]any{p, v, 1})
		key := uuid.New()
		o1, err := f.svc.CreateOrder(f.ctx, u, f.req(), &key)
		require.NoError(t, err)
		o2, err := f.svc.CreateOrder(f.ctx, u, f.req(), &key)
		require.NoError(t, err)
		require.Equal(t, o1.ID, o2.ID)
		require.Equal(t, 1, f.orders(u))
		require.Equal(t, 1, f.reserved(v))
	})

	for _, withKey := range []bool{true, false} {
		withKey := withKey
		t.Run("double click: two concurrent submits create exactly one order and one reservation"+map[bool]string{true: " (same key)", false: " (no key)"}[withKey], func(t *testing.T) {
			f := newCheckoutFixture(t)
			p, v := f.variant(300000, 5)
			u := f.buyer([3]any{p, v, 1})
			key := uuid.New()
			var wg sync.WaitGroup
			errs := make([]error, 2)
			ids := make([]uuid.UUID, 2)
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					var k *uuid.UUID
					if withKey {
						k = &key
					}
					o, err := f.svc.CreateOrder(f.ctx, u, f.req(), k)
					errs[i] = err
					if o != nil {
						ids[i] = o.ID
					}
				}(i)
			}
			wg.Wait()
			require.Equal(t, 1, f.orders(u), "errors: %v", errs)
			require.Equal(t, 1, f.reserved(v))
			require.Equal(t, 1, f.activeReservations(v))
			for i, err := range errs {
				if err != nil {
					require.ErrorIs(t, err, ErrEmptyCart, "the second submit sees the cart already ordered")
				} else if withKey {
					require.Equal(t, ids[1-i], ids[i], "same key returns the same order")
				}
			}
		})
	}

	t.Run("stock ran out just before checkout: 409 insufficient_stock, readable message, no order, no reservation", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p, v := f.variant(300000, 2)
		u := f.buyer([3]any{p, v, 2})
		f.exec(`UPDATE inventory_items SET reserved_stock = 1 WHERE product_variant_id=$1`, v) // someone else took one
		code, errCode, msg := f.post(u, f.req(), uuid.NewString())
		require.Equal(t, http.StatusConflict, code, msg)
		require.Equal(t, "insufficient_stock", errCode)
		require.Contains(t, msg, "Недостаточно товара")
		require.Equal(t, 0, f.orders(u))
		require.Equal(t, 1, f.reserved(v), "no extra reservation")
		require.Equal(t, 0, f.activeReservations(v))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM cart_items ci JOIN carts c ON c.id=ci.cart_id WHERE c.user_id=$1`, u), "the cart is kept for correction")
	})

	t.Run("variant became inactive / product unpublished: 409 item_unavailable, no order, no reservation", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p1, v1 := f.variant(300000, 5)
		p2, v2 := f.variant(300000, 5)
		u1 := f.buyer([3]any{p1, v1, 1})
		u2 := f.buyer([3]any{p2, v2, 1})
		f.exec(`UPDATE product_variants SET is_active=false WHERE id=$1`, v1)
		f.exec(`UPDATE products SET status='hidden' WHERE id=$1`, p2)
		for _, c := range []struct {
			u, v uuid.UUID
		}{{u1, v1}, {u2, v2}} {
			code, errCode, msg := f.post(c.u, f.req(), uuid.NewString())
			require.Equal(t, http.StatusConflict, code, msg)
			require.Equal(t, "item_unavailable", errCode)
			require.NotContains(t, strings.ToLower(msg), "variant not found")
			require.Equal(t, 0, f.orders(c.u))
			require.Equal(t, 0, f.reserved(c.v))
		}
	})

	t.Run("a multi-line cart where the second line fails reserves nothing (atomic)", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p1, v1 := f.variant(100000, 5)
		p2, v2 := f.variant(100000, 1)
		u := f.buyer([3]any{p1, v1, 1}, [3]any{p2, v2, 2})
		code, errCode, _ := f.post(u, f.req(), uuid.NewString())
		require.Equal(t, http.StatusConflict, code)
		require.Equal(t, "insufficient_stock", errCode)
		require.Equal(t, 0, f.reserved(v1), "the first line's reservation was rolled back")
		require.Equal(t, 0, f.orders(u))
	})

	t.Run("price changed after adding to cart: the order uses the current server price", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p, v := f.variant(300000, 5)
		u := f.buyer([3]any{p, v, 1})
		f.exec(`UPDATE product_variants SET price_cents=320000 WHERE id=$1`, v)
		f.exec(`UPDATE products SET price_cents=320000 WHERE id=$1`, p)
		o, err := f.svc.CreateOrder(f.ctx, u, f.req(), nil)
		require.NoError(t, err)
		require.Equal(t, int64(320000), o.Items[0].PriceCents)
		require.Equal(t, int64(320000+39000), o.TotalPriceCents)
	})

	t.Run("concurrent buyers of the last units: never oversold", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p, v := f.variant(300000, 3)
		var buyers []uuid.UUID
		for i := 0; i < 6; i++ {
			buyers = append(buyers, f.buyer([3]any{p, v, 1}))
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		ok, stock := 0, 0
		for _, u := range buyers {
			wg.Add(1)
			go func(u uuid.UUID) {
				defer wg.Done()
				code, errCode, _ := f.post(u, f.req(), uuid.NewString())
				mu.Lock()
				defer mu.Unlock()
				if code == http.StatusCreated {
					ok++
				} else if code == http.StatusConflict && errCode == "insufficient_stock" {
					stock++
				}
			}(u)
		}
		wg.Wait()
		require.Equal(t, 3, ok)
		require.Equal(t, 3, stock, "the others get a clean out-of-stock answer")
		require.Equal(t, 3, f.reserved(v))
		require.Equal(t, 3, f.count(`SELECT count(*) FROM orders WHERE user_id = ANY($1)`, buyers))
	})

	t.Run("concurrent carts with the same two SKUs in opposite order: no deadlock error", func(t *testing.T) {
		f := newCheckoutFixture(t)
		pa, va := f.variant(100000, 50)
		pb, vb := f.variant(100000, 50)
		var wg sync.WaitGroup
		var mu sync.Mutex
		var failures []string
		for i := 0; i < 8; i++ {
			var u uuid.UUID
			if i%2 == 0 {
				u = f.buyer([3]any{pa, va, 1}, [3]any{pb, vb, 1})
			} else {
				u = f.buyer([3]any{pb, vb, 1}, [3]any{pa, va, 1})
			}
			wg.Add(1)
			go func(u uuid.UUID) {
				defer wg.Done()
				code, errCode, msg := f.post(u, f.req(), uuid.NewString())
				if code != http.StatusCreated {
					mu.Lock()
					failures = append(failures, errCode+": "+msg)
					mu.Unlock()
				}
			}(u)
		}
		wg.Wait()
		require.Empty(t, failures)
		require.Equal(t, 8, f.reserved(va))
		require.Equal(t, 8, f.reserved(vb))
	})

	t.Run("backend refusals never leak internal error text", func(t *testing.T) {
		f := newCheckoutFixture(t)
		p, v := f.variant(300000, 5)
		u := f.buyer([3]any{p, v, 1})
		bad := f.req()
		bad.DeliveryMethodID = uuid.New() // unknown delivery method
		code, errCode, msg := f.post(u, bad, "")
		require.Equal(t, http.StatusBadRequest, code, msg)
		require.Equal(t, "invalid_delivery_method", errCode)
		bad = f.req()
		bad.CustomerEmail = "not-an-email"
		code, errCode, msg = f.post(u, bad, "")
		require.Equal(t, http.StatusBadRequest, code)
		require.Equal(t, "validation_error", errCode)
		require.NotContains(t, msg, "Key: '", "no validator internals")
		require.Equal(t, 0, f.orders(u))
		empty := f.buyer()
		code, errCode, _ = f.post(empty, f.req(), "")
		require.Equal(t, http.StatusBadRequest, code)
		require.Equal(t, "empty_cart", errCode)
		_ = errors.New
	})
}
