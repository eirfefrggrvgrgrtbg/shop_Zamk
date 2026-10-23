package payments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// PAY.P1: paying after checkout on a disposable database. A customer without paid orders pays one order at a time
// (first-payment claim), so a second order is refused before anything is reserved while the first payment is in
// progress, and a payment start that still meets the claim answers 409, never 500. Without T-Bank credentials no
// request leaves the process: card is unavailable, the explicit T-Pay mock keeps working.

// noNetwork fails the test on any outgoing provider request and counts attempts.
type noNetwork struct {
	t     *testing.T
	calls atomic.Int32
}

func (n *noNetwork) RoundTrip(req *http.Request) (*http.Response, error) {
	n.calls.Add(1)
	n.t.Errorf("unexpected outgoing request to %s", req.URL.Host)
	return nil, errors.New("network disabled in test")
}

type payFixture struct {
	t        *testing.T
	ctx      context.Context
	db       *pgxpool.Pool
	orderH   *orders.Handler
	ordersSvc *orders.Service
	svc      *Service
	payH     *Handler
	provider *TBankProvider
	net      *noNetwork
	seller   uuid.UUID
	cat      uuid.UUID
	brand    uuid.UUID
	dm       uuid.UUID
}

// newPayFixture builds the real orders and payments services on a disposable database. The provider has the
// given terminal credentials and T-Pay in mock mode, as in the demo.
func newPayFixture(t *testing.T, terminalKey, password string) *payFixture {
	t.Helper()
	ctx := context.Background()
	db, err := pgxpool.New(ctx, testutil.DisposableDatabaseURL(t))
	require.NoError(t, err)
	t.Cleanup(db.Close)
	testutil.MustBeDisposableDatabase(t, db)

	f := &payFixture{t: t, ctx: ctx, db: db, seller: uuid.New(), cat: uuid.New(), brand: uuid.New(), dm: uuid.New()}
	f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Pay Brand', $2, 'p@ex.com', 'active')`, f.seller, "pp-"+f.seller.String()[:8])
	f.exec(`INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', $2)`, f.cat, "pp-cat-"+f.cat.String()[:8])
	f.exec(`INSERT INTO brands (id, name, slug) VALUES ($1, 'br', $2)`, f.brand, "pp-br-"+f.brand.String()[:8])
	f.exec(`INSERT INTO delivery_methods (id, code, name, price_cents, is_active) VALUES ($1, $2, 'Курьер', 39000, true)`, f.dm, "pp_"+f.dm.String()[:8])

	pg := &postgres.Client{Pool: db}
	cfg := &config.Config{Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500, OrderPaymentTimeoutMinutes: 30}, App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	ordersRepo := orders.NewRepository(db)
	invSvc := inventory.NewService(inventory.NewRepository(db), nil, pg)
	f.ordersSvc = orders.NewService(ordersRepo, cart.NewRepository(db), invSvc, pg, cfg)
	f.orderH = orders.NewHandler(f.ordersSvc)

	f.net = &noNetwork{t: t}
	f.provider = NewTBankProvider(terminalKey, password, "", "", "", false, "O", "mock")
	f.provider.client = &http.Client{Transport: f.net}
	notifSvc := notifications.NewService(notifications.NewRepository(pg), nil, nil)
	f.svc = NewService(NewRepository(db), ordersRepo, invSvc, f.provider, pg, notifSvc, nil, cfg)
	f.payH = NewHandler(f.svc, "local")
	return f
}

func (f *payFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *payFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&n), sql)
	return n
}

// variant creates a published product with one variant and stock.
func (f *payFixture) variant(price int64, stock int) (uuid.UUID, uuid.UUID) {
	p, v := uuid.New(), uuid.New()
	f.exec(`INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents) VALUES ($1,$2,$3,$4,'Платье',$5,'d','published',$6)`, p, f.seller, f.cat, f.brand, "pp-p-"+p.String()[:8], price)
	f.exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, price_cents, is_active) VALUES ($1,$2,$3,$3,$4,true)`, v, p, "PP-"+v.String()[:8], price)
	f.exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,$4,0)`, p, v, f.seller, stock)
	return p, v
}

// buyer creates a customer with an empty cart.
func (f *payFixture) buyer() uuid.UUID {
	u := uuid.New()
	f.exec(`INSERT INTO users (id, name, email, password_hash, role) VALUES ($1,'Покупатель',$2,'h','customer')`, u, u.String()[:8]+"@buyer.test")
	f.exec(`INSERT INTO carts (id, user_id) VALUES (gen_random_uuid(),$1)`, u)
	return u
}

// addToCart puts one line into the customer's cart.
func (f *payFixture) addToCart(u, p, v uuid.UUID, qty int) {
	f.exec(`INSERT INTO cart_items (id, cart_id, product_id, product_variant_id, quantity) SELECT gen_random_uuid(), id, $2, $3, $4 FROM carts WHERE user_id=$1`, u, p, v, qty)
}

func (f *payFixture) cartLines(u uuid.UUID) int {
	return f.count(`SELECT count(*) FROM cart_items ci JOIN carts c ON c.id=ci.cart_id WHERE c.user_id=$1`, u)
}
func (f *payFixture) ordersOf(u uuid.UUID) int {
	return f.count(`SELECT count(*) FROM orders WHERE user_id=$1`, u)
}
func (f *payFixture) reserved(v uuid.UUID) int {
	return f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id=$1`, v)
}
func (f *payFixture) activeReservations(v uuid.UUID) int {
	return f.count(`SELECT count(*) FROM reservations WHERE product_variant_id=$1 AND status='active'`, v)
}
func (f *payFixture) paymentsOf(orderID uuid.UUID) int {
	return f.count(`SELECT count(*) FROM payments WHERE order_id=$1`, orderID)
}
func (f *payFixture) claims(u uuid.UUID) int {
	return f.count(`SELECT count(*) FROM customer_first_payment_claims WHERE user_id=$1`, u)
}

type apiResult struct {
	Status  int
	Code    string
	Message string
	Body    map[string]any
}

func decodeResult(w *httptest.ResponseRecorder) apiResult {
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	res := apiResult{Status: w.Code, Body: body}
	if e, ok := body["error"].(map[string]any); ok {
		res.Code, _ = e["code"].(string)
		res.Message, _ = e["message"].(string)
	}
	return res
}

// order submits the checkout through the real orders handler.
func (f *payFixture) order(u uuid.UUID, key string) apiResult {
	b, _ := json.Marshal(orders.CreateOrderRequest{CustomerName: "Анна Покупатель", CustomerPhone: "+79990000001", CustomerEmail: "anna@buyer.test", DeliveryAddress: "г. Москва, ул. Тестовая, д. 1", DeliveryMethodID: f.dm})
	r := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(b))
	r = r.WithContext(context.WithValue(r.Context(), "userID", u))
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	f.orderH.CreateOrder(w, r)
	return decodeResult(w)
}

func (f *payFixture) mustOrder(u uuid.UUID, key string) (uuid.UUID, string) {
	f.t.Helper()
	res := f.order(u, key)
	require.Equal(f.t, http.StatusCreated, res.Status, "%+v", res)
	id, err := uuid.Parse(res.Body["id"].(string))
	require.NoError(f.t, err)
	number, _ := res.Body["orderNumber"].(string)
	return id, number
}

// pay starts a payment through the real payments handler.
func (f *payFixture) pay(u, orderID uuid.UUID, method string) apiResult {
	b, _ := json.Marshal(CreatePaymentRequest{Method: method})
	r := httptest.NewRequest(http.MethodPost, "/api/customer/orders/"+orderID.String()+"/payments", bytes.NewReader(b))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orderID.String())
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	r = r.WithContext(context.WithValue(ctx, "userID", u))
	w := httptest.NewRecorder()
	f.payH.CreatePayment(w, r)
	return decodeResult(w)
}

func (f *payFixture) mustPay(u, orderID uuid.UUID) uuid.UUID {
	f.t.Helper()
	res := f.pay(u, orderID, "tpay")
	require.Equal(f.t, http.StatusCreated, res.Status, "%+v", res)
	require.Equal(f.t, "mock", res.Body["integrationMode"])
	id, err := uuid.Parse(res.Body["paymentId"].(string))
	require.NoError(f.t, err)
	require.Equal(f.t, "/dev/payments/mock/"+id.String(), res.Body["paymentUrl"])
	return id
}

func requireRussian(t *testing.T, msg string) {
	t.Helper()
	require.NotEmpty(t, msg)
	require.Regexp(t, `[А-Яа-яЁё]`, msg, "message must be in Russian: %q", msg)
	for _, leak := range []string{"another order", "sql", "pgx", "claim", "uuid"} {
		require.NotContains(t, strings.ToLower(msg), leak)
	}
}

func TestPayP1(t *testing.T) {
	t.Run("one order and a mock payment start: pending payment, mock page, first-payment claim on this order", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 5)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		orderID, _ := f.mustOrder(u, uuid.NewString())
		paymentID := f.mustPay(u, orderID)

		require.Equal(t, 1, f.paymentsOf(orderID))
		require.Equal(t, "pending", func() string {
			var s string
			require.NoError(t, f.db.QueryRow(f.ctx, `SELECT status FROM payments WHERE id=$1`, paymentID).Scan(&s))
			return s
		}())
		require.Equal(t, 1, f.count(`SELECT count(*) FROM customer_first_payment_claims WHERE user_id=$1 AND order_id=$2`, u, orderID))
		require.Equal(t, 1, f.reserved(v))
		require.Equal(t, int32(0), f.net.calls.Load())
	})

	t.Run("second order while the first payment is unfinished: 409 before anything is reserved, cart kept", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 5)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		firstID, firstNumber := f.mustOrder(u, uuid.NewString())
		f.mustPay(u, firstID)

		f.addToCart(u, p, v, 2)
		res := f.order(u, uuid.NewString())
		require.Equal(t, http.StatusConflict, res.Status, "%+v", res)
		require.Equal(t, "payment_in_progress", res.Code)
		requireRussian(t, res.Message)
		require.Contains(t, res.Message, firstNumber, "the message names the order to pay")

		require.Equal(t, 1, f.ordersOf(u), "no extra order")
		require.Equal(t, 1, f.reserved(v), "nothing reserved for the refused order")
		require.Equal(t, 1, f.activeReservations(v))
		require.Equal(t, 1, f.cartLines(u), "the cart stays as it was")
	})

	t.Run("repeat of the first checkout with the same key returns the same order while its payment is in progress", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 5)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		key := uuid.NewString()
		firstID, _ := f.mustOrder(u, key)
		paymentID := f.mustPay(u, firstID)

		againID, _ := f.mustOrder(u, key)
		require.Equal(t, firstID, againID)
		// A repeated payment start for the same order reuses the pending payment.
		res := f.pay(u, firstID, "tpay")
		require.Equal(t, http.StatusCreated, res.Status, "%+v", res)
		require.Equal(t, paymentID.String(), res.Body["paymentId"])

		require.Equal(t, 1, f.ordersOf(u))
		require.Equal(t, 1, f.paymentsOf(firstID))
		require.Equal(t, 1, f.reserved(v))
	})

	t.Run("after the first payment is rejected the customer orders and pays again", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 5)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		firstID, _ := f.mustOrder(u, uuid.NewString())
		paymentID := f.mustPay(u, firstID)
		require.NoError(t, f.svc.ProcessMockPaymentAction(f.ctx, paymentID, "reject"))
		require.Equal(t, 0, f.claims(u), "a rejected payment releases the claim")

		f.addToCart(u, p, v, 1)
		secondID, _ := f.mustOrder(u, uuid.NewString())
		f.mustPay(u, secondID)
		require.Equal(t, 1, f.paymentsOf(secondID))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM customer_first_payment_claims WHERE user_id=$1 AND order_id=$2`, u, secondID))
	})

	t.Run("an order created before the first payment started: its payment answers 409, not 500, and stays payable", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 5)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		firstID, _ := f.mustOrder(u, uuid.NewString())
		f.addToCart(u, p, v, 1)
		secondID, _ := f.mustOrder(u, uuid.NewString()) // no payment yet, so no claim: allowed
		firstPayment := f.mustPay(u, firstID)

		res := f.pay(u, secondID, "tpay")
		require.Equal(t, http.StatusConflict, res.Status, "%+v", res)
		require.Equal(t, "payment_in_progress", res.Code)
		requireRussian(t, res.Message)
		require.Equal(t, 0, f.paymentsOf(secondID), "no payment row for the refused start")
		require.Equal(t, 2, f.reserved(v), "each order keeps exactly its own reservation")
		require.Equal(t, 2, f.activeReservations(v))

		require.NoError(t, f.svc.ProcessMockPaymentAction(f.ctx, firstPayment, "cancel"))
		f.mustPay(u, secondID)
		require.Equal(t, 1, f.paymentsOf(secondID))
	})

	t.Run("concurrent: second-order submits while the first payment is in progress all get 409, none is created", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 10)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		firstID, _ := f.mustOrder(u, uuid.NewString())
		f.mustPay(u, firstID)
		f.addToCart(u, p, v, 1)

		const n = 6
		results := make([]apiResult, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i] = f.order(u, uuid.NewString())
			}(i)
		}
		wg.Wait()
		for _, r := range results {
			require.Equal(t, http.StatusConflict, r.Status, "%+v", r)
			require.Equal(t, "payment_in_progress", r.Code)
		}
		require.Equal(t, 1, f.ordersOf(u))
		require.Equal(t, 1, f.reserved(v))
	})

	t.Run("concurrent payment starts for two unpaid orders: exactly one starts, the other gets 409, one claim", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 10)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		a, _ := f.mustOrder(u, uuid.NewString())
		f.addToCart(u, p, v, 1)
		b, _ := f.mustOrder(u, uuid.NewString())

		results := make([]apiResult, 2)
		var wg sync.WaitGroup
		for i, id := range []uuid.UUID{a, b} {
			wg.Add(1)
			go func(i int, id uuid.UUID) {
				defer wg.Done()
				results[i] = f.pay(u, id, "tpay")
			}(i, id)
		}
		wg.Wait()
		statuses := []int{results[0].Status, results[1].Status}
		require.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, statuses, "%+v", results)
		require.Equal(t, 1, f.claims(u))
		require.Equal(t, 1, f.paymentsOf(a)+f.paymentsOf(b), "no double payment")
		require.Equal(t, 2, f.reserved(v), "no double reservation")
	})

	t.Run("empty T-Bank configuration: card is unavailable with a Russian message, no request, nothing created", func(t *testing.T) {
		f := newPayFixture(t, "", "")
		p, v := f.variant(500000, 5)
		u := f.buyer()
		f.addToCart(u, p, v, 1)
		orderID, _ := f.mustOrder(u, uuid.NewString())

		res := f.pay(u, orderID, "card")
		require.Equal(t, http.StatusBadRequest, res.Status, "%+v", res)
		require.Equal(t, "payment_method_unavailable", res.Code)
		requireRussian(t, res.Message)
		require.Equal(t, 0, f.paymentsOf(orderID))
		require.Equal(t, 0, f.claims(u))

		// The same order is then paid with the explicitly allowed mock.
		f.mustPay(u, orderID)
		require.Equal(t, int32(0), f.net.calls.Load(), "no request reached T-Bank")
	})
}

// TestPayP1ProviderWithoutCredentials: no request leaves the process without a terminal key and password.
func TestPayP1ProviderWithoutCredentials(t *testing.T) {
	for _, creds := range [][2]string{{"", ""}, {"TERMINAL", ""}, {"", "SECRET"}, {"  ", "  "}} {
		net := &noNetwork{t: t}
		p := NewTBankProvider(creds[0], creds[1], "", "", "", true, "O", "quick_widget")
		p.client = &http.Client{Transport: net}

		require.Equal(t, "unavailable", p.GetMode("card"))
		require.Equal(t, "unavailable", p.GetMode("tpay"), "a non-mock T-Pay needs the terminal too")

		_, err := p.CreatePayment(context.Background(), CreatePaymentInput{OrderID: "o", AmountCents: 100, Method: "card", IntegrationMode: "hosted_form"})
		require.ErrorIs(t, err, ErrProviderRejected)
		require.ErrorIs(t, err, ErrProviderNotConfigured)

		_, err = p.CheckOrder(context.Background(), "o")
		require.ErrorIs(t, err, ErrProviderNotConfigured)

		// A notification signed with the empty password is not accepted.
		params := map[string]string{"TerminalKey": creds[0], "OrderId": "o", "Status": "CONFIRMED", "Success": "true", "PaymentId": "1", "Amount": "100"}
		params["Password"] = creds[1]
		keys := []string{"Amount", "OrderId", "Password", "PaymentId", "Status", "Success", "TerminalKey"}
		var concat string
		for _, k := range keys {
			concat += params[k]
		}
		sum := sha256.Sum256([]byte(concat))
		body, _ := json.Marshal(map[string]any{"TerminalKey": creds[0], "OrderId": "o", "Status": "CONFIRMED", "Success": true, "PaymentId": 1, "Amount": 100, "Token": hex.EncodeToString(sum[:])})
		require.Error(t, p.VerifyWebhook(context.Background(), nil, body))

		require.Equal(t, int32(0), net.calls.Load())
	}

	mock := NewTBankProvider("", "", "", "", "", false, "O", "mock")
	require.Equal(t, "mock", mock.GetMode("tpay"), "the explicit T-Pay mock needs no terminal")
	require.Equal(t, "unavailable", mock.GetMode("sbp"))

	stub := NewTBankProvider("STUB", "STUB", "", "", "", false, "O", "mock")
	require.Equal(t, "hosted_form", stub.GetMode("card"), "the explicit local stub terminal stays available")

	real := NewTBankProvider("TERMINAL", "SECRET", "", "", "", false, "O", "mock")
	require.Equal(t, "hosted_form", real.GetMode("card"))
}
