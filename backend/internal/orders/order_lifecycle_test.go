package orders_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// ORDER.LIFECYCLE.1: Comprehensive verification of order cancellation, payment expiration,
// reservation release, fulfillment cascading, first-payment claims, concurrency, connection pool safety,
// and real race conditions between payment confirmation and expiration (PAY.RECON.1 compatibility).

type lifecycleFixture struct {
	t             *testing.T
	ctx           context.Context
	db            *pgxpool.Pool
	svc           *orders.Service
	invSvc        *inventory.Service
	paymentsSvc   *payments.Service
	paymentsRepo  *payments.Repository
	tbankProvider *payments.TBankProvider
	seller        uuid.UUID
	cat           uuid.UUID
	brand         uuid.UUID
	dm            uuid.UUID
}

func newLifecycleFixture(t *testing.T) *lifecycleFixture {
	return newLifecycleFixtureWithPool(t, 0)
}

func newLifecycleFixtureWithPool(t *testing.T, maxConns int) *lifecycleFixture {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.DisposableDatabaseURL(t)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	if maxConns > 0 {
		cfg.MaxConns = int32(maxConns)
	}
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(db.Close)
	testutil.MustBeDisposableDatabase(t, db)

	f := &lifecycleFixture{
		t:      t,
		ctx:    ctx,
		db:     db,
		seller: uuid.New(),
		cat:    uuid.New(),
		brand:  uuid.New(),
		dm:     uuid.New(),
	}
	f.exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Lifecycle Brand', $2, 'lc@ex.com', 'active')`, f.seller, "lc-"+f.seller.String()[:8])
	f.exec(`INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', $2)`, f.cat, "lc-cat-"+f.cat.String()[:8])
	f.exec(`INSERT INTO brands (id, name, slug) VALUES ($1, 'br', $2)`, f.brand, "lc-br-"+f.brand.String()[:8])
	f.exec(`INSERT INTO delivery_methods (id, code, name, price_cents, is_active) VALUES ($1, $2, 'Курьер', 39000, true)`, f.dm, "lc_"+f.dm.String()[:8])

	pg := &postgres.Client{Pool: db}
	f.invSvc = inventory.NewService(inventory.NewRepository(db), nil, pg)
	appCfg := &config.Config{
		App: config.AppConfig{PaymentStuckPendingMinutes: 30},
		Worker: config.WorkerConfig{
			MarketplaceCommissionBPS:       1500,
			OrderPaymentTimeoutMinutes:     30,
			OrderExpirationIntervalSeconds: 30,
		},
	}
	ordersRepo := orders.NewRepository(db)
	f.svc = orders.NewService(ordersRepo, cart.NewRepository(db), f.invSvc, pg, appCfg)

	f.tbankProvider = payments.NewTBankProvider("TERM", "SECRET", "", "", "", true, "O", "mock")
	f.paymentsRepo = payments.NewRepository(db)
	notifRepo := notifications.NewRepository(pg)
	notifSvc := notifications.NewService(notifRepo, nil, nil)
	f.paymentsSvc = payments.NewService(f.paymentsRepo, ordersRepo, f.invSvc, f.tbankProvider, pg, notifSvc, nil, appCfg)

	return f
}

func (f *lifecycleFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *lifecycleFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&n), sql)
	return n
}

func (f *lifecycleFixture) str(sql string, args ...any) string {
	f.t.Helper()
	var s string
	require.NoError(f.t, f.db.QueryRow(f.ctx, sql, args...).Scan(&s), sql)
	return s
}

func (f *lifecycleFixture) variant(price int64, stock int) (uuid.UUID, uuid.UUID) {
	p, v := uuid.New(), uuid.New()
	f.exec(`INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents) VALUES ($1,$2,$3,$4,'Куртка',$5,'desc','published',$6)`, p, f.seller, f.cat, f.brand, "lc-p-"+p.String()[:8], price)
	f.exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, price_cents, is_active) VALUES ($1,$2,$3,$3,$4,true)`, v, p, "LC-"+v.String()[:8], price)
	f.exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,$4,0)`, p, v, f.seller, stock)
	return p, v
}

func (f *lifecycleFixture) buyerWithCart(productID, variantID uuid.UUID, qty int) uuid.UUID {
	u, c := uuid.New(), uuid.New()
	f.exec(`INSERT INTO users (id, name, email, password_hash, role) VALUES ($1,'Покупатель',$2,'h','customer')`, u, u.String()[:8]+"@buyer.test")
	f.exec(`INSERT INTO carts (id, user_id) VALUES ($1,$2)`, c, u)
	f.exec(`INSERT INTO cart_items (id, cart_id, product_id, product_variant_id, quantity) VALUES (gen_random_uuid(),$1,$2,$3,$4)`, c, productID, variantID, qty)
	return u
}

func (f *lifecycleFixture) createOrder(buyerID uuid.UUID) uuid.UUID {
	f.t.Helper()
	req := orders.CreateOrderRequest{
		CustomerName:     "Тест Покупатель",
		CustomerPhone:    "+79991112233",
		CustomerEmail:    "buyer@test.ru",
		DeliveryAddress:  "ул. Пушкина, д. 10",
		DeliveryMethodID: f.dm,
	}
	key := uuid.New()
	res, err := f.svc.CreateOrder(f.ctx, buyerID, req, &key)
	require.NoError(f.t, err)
	return res.ID
}

func (f *lifecycleFixture) reservedStock(variantID uuid.UUID) int {
	return f.count(`SELECT reserved_stock FROM inventory_items WHERE product_variant_id=$1`, variantID)
}

func (f *lifecycleFixture) totalStock(variantID uuid.UUID) int {
	return f.count(`SELECT total_stock FROM inventory_items WHERE product_variant_id=$1`, variantID)
}

func (f *lifecycleFixture) signedWebhook(orderID uuid.UUID, providerPaymentID string, amountCents int64, status string) []byte {
	f.t.Helper()
	pid, err := strconv.ParseInt(providerPaymentID, 10, 64)
	require.NoError(f.t, err)
	data := map[string]any{
		"TerminalKey": "TERM",
		"OrderId":     orderID.String(),
		"Success":     true,
		"Status":      status,
		"PaymentId":   pid,
		"Amount":      amountCents,
		"ErrorCode":   "0",
	}
	params := map[string]string{
		"TerminalKey": "TERM",
		"OrderId":     orderID.String(),
		"Success":     "true",
		"Status":      status,
		"PaymentId":   strconv.FormatInt(pid, 10),
		"Amount":      strconv.FormatInt(amountCents, 10),
		"ErrorCode":   "0",
		"Password":    "SECRET",
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(params[k])
	}
	h := sha256.Sum256([]byte(sb.String()))
	data["Token"] = fmt.Sprintf("%x", h)
	b, err := json.Marshal(data)
	require.NoError(f.t, err)
	return b
}

func TestOrderLifecycle(t *testing.T) {
	t.Run("A. Expired awaiting payment order cancels, releases reserves and fulfillments, frees claim", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(150000, 10)
		buyerID := f.buyerWithCart(p, v, 2)
		orderID := f.createOrder(buyerID)

		require.Equal(t, 2, f.reservedStock(v))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM reservations WHERE product_variant_id=$1 AND status='active'`, v))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id=$1 AND status='awaiting_payment'`, orderID))

		// Set order created_at to 35 minutes ago
		f.exec(`UPDATE orders SET created_at = now() - interval '35 minutes' WHERE id=$1`, orderID)
		// Insert first-payment claim
		f.exec(`INSERT INTO customer_first_payment_claims (user_id, order_id, created_at, updated_at) VALUES ($1,$2,now(),now()) ON CONFLICT (user_id) DO NOTHING`, buyerID, orderID)

		now := time.Now()
		cutoff := now.Add(-30 * time.Minute)
		res, err := f.svc.ExpireAwaitingPaymentOrders(f.ctx, cutoff, 100)
		require.NoError(t, err)
		require.Equal(t, 1, res.Expired)
		require.Equal(t, 1, res.ReleasedReservations)

		// Assert Order status
		require.Equal(t, "cancelled", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM order_status_history WHERE order_id=$1 AND to_status='cancelled'`, orderID))

		// Assert Fulfillment status cascaded to cancelled
		require.Equal(t, 0, f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id=$1 AND status='awaiting_payment'`, orderID))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id=$1 AND status='cancelled'`, orderID))

		// Assert Inventory Reservation released and reserved_stock restored
		require.Equal(t, 0, f.reservedStock(v))
		require.Equal(t, 10, f.totalStock(v))
		require.Equal(t, 0, f.count(`SELECT count(*) FROM reservations WHERE product_variant_id=$1 AND status='active'`, v))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM reservations WHERE product_variant_id=$1 AND status='released'`, v))

		// Assert customer first-payment claim deleted
		require.Equal(t, 0, f.count(`SELECT count(*) FROM customer_first_payment_claims WHERE order_id=$1`, orderID))

		// Idempotent second expiration run
		res2, err2 := f.svc.ExpireAwaitingPaymentOrders(f.ctx, cutoff, 100)
		require.NoError(t, err2)
		require.Equal(t, 0, res2.Expired)
		require.Equal(t, 0, f.reservedStock(v))
	})

	t.Run("B. Customer cancels unpaid order: safe release, idempotence, correct state", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(200000, 5)
		buyerID := f.buyerWithCart(p, v, 1)
		orderID := f.createOrder(buyerID)

		require.Equal(t, 1, f.reservedStock(v))
		f.exec(`INSERT INTO customer_first_payment_claims (user_id, order_id, created_at, updated_at) VALUES ($1,$2,now(),now()) ON CONFLICT (user_id) DO NOTHING`, buyerID, orderID)

		err := f.svc.CancelCustomerOrder(f.ctx, buyerID, orderID)
		require.NoError(t, err)

		// Verify order is cancelled
		require.Equal(t, "cancelled", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, 1, f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id=$1 AND status='cancelled'`, orderID))
		require.Equal(t, 0, f.reservedStock(v))
		require.Equal(t, 0, f.count(`SELECT count(*) FROM customer_first_payment_claims WHERE order_id=$1`, orderID))

		// Second cancel attempt: returns ErrOrderAlreadyCancelled idempotently
		err2 := f.svc.CancelCustomerOrder(f.ctx, buyerID, orderID)
		require.ErrorIs(t, err2, orders.ErrOrderAlreadyCancelled)
		require.Equal(t, 0, f.reservedStock(v), "stock must not become negative")
		require.Equal(t, 5, f.totalStock(v))
	})

	t.Run("C. In-flight active payment blocks customer cancellation and expiration", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(100000, 3)
		buyerID := f.buyerWithCart(p, v, 1)
		orderID := f.createOrder(buyerID)

		// Insert active payment in 'pending' status
		payID := uuid.New()
		f.exec(`INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, created_at, updated_at)
			VALUES ($1, $2, 'tbank', 'pending', 100000, 'RUB', $3, now(), now())`, payID, orderID, uuid.NewString())

		// Customer cancel must be refused
		err := f.svc.CancelCustomerOrder(f.ctx, buyerID, orderID)
		require.Error(t, err)
		require.ErrorIs(t, err, orders.ErrOrderNotCancellable)
		require.Equal(t, "awaiting_payment", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, 1, f.reservedStock(v))

		// Expiration must skip order with active payment
		f.exec(`UPDATE orders SET created_at = now() - interval '40 minutes' WHERE id=$1`, orderID)
		res, err := f.svc.ExpireAwaitingPaymentOrders(f.ctx, time.Now().Add(-30*time.Minute), 100)
		require.NoError(t, err)
		require.Equal(t, 0, res.Expired)
		require.Equal(t, "awaiting_payment", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, 1, f.reservedStock(v))

		// When payment terminates in failed status, expiration proceeds
		f.exec(`UPDATE payments SET status = 'failed', failed_at = now() WHERE id=$1`, payID)
		res2, err2 := f.svc.ExpireAwaitingPaymentOrders(f.ctx, time.Now().Add(-30*time.Minute), 100)
		require.NoError(t, err2)
		require.Equal(t, 1, res2.Expired)
		require.Equal(t, "cancelled", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, 0, f.reservedStock(v))
	})

	t.Run("D. Concurrent workers processing expiration: zero duplicates, skip-locked safety", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(50000, 50)

		var orderIDs []uuid.UUID
		for i := 0; i < 6; i++ {
			b := f.buyerWithCart(p, v, 1)
			oid := f.createOrder(b)
			orderIDs = append(orderIDs, oid)
		}
		require.Equal(t, 6, f.reservedStock(v))

		// Backdate all orders to 40 minutes ago
		f.exec(`UPDATE orders SET created_at = now() - interval '40 minutes'`)

		cutoff := time.Now().Add(-30 * time.Minute)
		var wg sync.WaitGroup
		expiredCounts := make([]int, 3)

		for w := 0; w < 3; w++ {
			workerIdx := w
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := f.svc.ExpireAwaitingPaymentOrders(f.ctx, cutoff, 10)
				if err == nil && res != nil {
					expiredCounts[workerIdx] = res.Expired
				}
			}()
		}
		wg.Wait()

		totalExpired := 0
		for _, c := range expiredCounts {
			totalExpired += c
		}
		require.Equal(t, 6, totalExpired, "all 6 orders should be expired exactly once across workers")
		require.Equal(t, 0, f.reservedStock(v), "all reservations must be released")
		require.Equal(t, 6, f.count(`SELECT count(*) FROM orders WHERE status='cancelled'`))
		require.Equal(t, 6, f.count(`SELECT count(*) FROM order_fulfillments WHERE status='cancelled'`))
	})

	t.Run("E. Small connection pool (maxConns=2) executes cancellation and expiration without pool starvation", func(t *testing.T) {
		// This reproduces the technical risk: previously GetActiveOrderReservations checked out a second connection
		// from the pool while in tx, causing deadlock on small pools.
		f := newLifecycleFixtureWithPool(t, 2)
		p, v := f.variant(120000, 20)

		b1 := f.buyerWithCart(p, v, 1)
		o1 := f.createOrder(b1)

		b2 := f.buyerWithCart(p, v, 1)
		o2 := f.createOrder(b2)
		f.exec(`UPDATE orders SET created_at = now() - interval '40 minutes' WHERE id=$1`, o2)

		var wg sync.WaitGroup
		errs := make(chan error, 2)

		// Goroutine 1: customer cancellation
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.svc.CancelCustomerOrder(f.ctx, b1, o1)
		}()

		// Goroutine 2: worker expiration
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.ExpireAwaitingPaymentOrders(f.ctx, time.Now().Add(-30*time.Minute), 10)
			errs <- err
		}()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("DEADLOCK / POOL STARVATION: Cancel/Expire timed out on 2-connection pool")
		}

		require.Equal(t, 0, f.reservedStock(v))
		require.Equal(t, "cancelled", f.str(`SELECT status FROM orders WHERE id=$1`, o1))
		require.Equal(t, "cancelled", f.str(`SELECT status FROM orders WHERE id=$1`, o2))
	})

	t.Run("F. Paid order is never touched by expiration, converted reservations are preserved", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(300000, 5)
		buyerID := f.buyerWithCart(p, v, 1)
		orderID := f.createOrder(buyerID)

		// Convert reservation to converted and order to paid
		var resID uuid.UUID
		require.NoError(t, f.db.QueryRow(f.ctx, `SELECT reservation_id FROM order_reservations WHERE order_id=$1`, orderID).Scan(&resID))
		tx, err := f.db.Begin(f.ctx)
		require.NoError(t, err)
		require.NoError(t, f.invSvc.ConvertReservationToSaleTx(f.ctx, tx, resID))
		require.NoError(t, tx.Commit(f.ctx))
		f.exec(`UPDATE orders SET status = 'paid', created_at = now() - interval '50 minutes' WHERE id=$1`, orderID)
		f.exec(`UPDATE order_fulfillments SET status = 'paid' WHERE order_id=$1`, orderID)

		// Run expiration
		res, err := f.svc.ExpireAwaitingPaymentOrders(f.ctx, time.Now().Add(-30*time.Minute), 100)
		require.NoError(t, err)
		require.Equal(t, 0, res.Expired)

		// Paid order must remain paid
		require.Equal(t, "paid", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, "paid", f.str(`SELECT status FROM order_fulfillments WHERE order_id=$1`, orderID))
		require.Equal(t, "converted", f.str(`SELECT status FROM reservations WHERE id=$1`, resID))
	})

	t.Run("G. First payment claim freed upon cancel/expiration allows customer to order/pay next", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(100000, 5)
		buyerID := f.buyerWithCart(p, v, 1)
		orderID := f.createOrder(buyerID)

		// Customer holds claim for orderID
		f.exec(`INSERT INTO customer_first_payment_claims (user_id, order_id, created_at, updated_at) VALUES ($1,$2,now(),now())`, buyerID, orderID)

		// Second order for same customer is blocked while order1 is awaiting payment
		f.exec(`INSERT INTO cart_items (id, cart_id, product_id, product_variant_id, quantity)
			SELECT gen_random_uuid(), c.id, $2, $3, 1 FROM carts c WHERE c.user_id = $1`, buyerID, p, v)
		req := orders.CreateOrderRequest{
			CustomerName:     "Тест Покупатель",
			CustomerPhone:    "+79991112233",
			CustomerEmail:    "buyer@test.ru",
			DeliveryAddress:  "ул. Пушкина, д. 10",
			DeliveryMethodID: f.dm,
		}
		key := uuid.New()
		_, err := f.svc.CreateOrder(f.ctx, buyerID, req, &key)
		require.Error(t, err, "another order is awaiting payment must block new order creation")

		// Cancel order1
		require.NoError(t, f.svc.CancelCustomerOrder(f.ctx, buyerID, orderID))

		// Claim for order1 is gone
		require.Equal(t, 0, f.count(`SELECT count(*) FROM customer_first_payment_claims WHERE order_id=$1`, orderID))

		// Now customer can create Order 2 and acquire claim
		order2, err := f.svc.CreateOrder(f.ctx, buyerID, req, &key)
		require.NoError(t, err)
		require.NotNil(t, order2)

		f.exec(`INSERT INTO customer_first_payment_claims (user_id, order_id, created_at, updated_at) VALUES ($1,$2,now(),now())`, buyerID, order2.ID)
		require.Equal(t, 1, f.count(`SELECT count(*) FROM customer_first_payment_claims WHERE user_id=$1 AND order_id=$2`, buyerID, order2.ID))
	})

	t.Run("H. Race Outcome 1: Payment confirmed first -> order paid, reservation converted, worker expiration skips it", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(150000, 5)
		buyerID := f.buyerWithCart(p, v, 1)
		orderID := f.createOrder(buyerID)

		require.Equal(t, 1, f.reservedStock(v))
		var resID uuid.UUID
		require.NoError(t, f.db.QueryRow(f.ctx, `SELECT reservation_id FROM order_reservations WHERE order_id=$1`, orderID).Scan(&resID))

		// Initialize payment via real payments service
		payResp, err := f.paymentsSvc.CreatePayment(f.ctx, buyerID, orderID, "tpay")
		require.NoError(t, err)
		require.NotNil(t, payResp)

		// 1. Bank payment confirmation arrives first
		providerID := f.str(`SELECT provider_payment_id FROM payments WHERE id=$1`, payResp.PaymentID)
		webhookBody := f.signedWebhook(orderID, providerID, 150000+39000, "CONFIRMED")
		err = f.paymentsSvc.HandleWebhook(f.ctx, nil, webhookBody)
		require.NoError(t, err)

		// Verify order is paid and reservation converted
		require.Equal(t, "paid", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, "paid", f.str(`SELECT status FROM order_fulfillments WHERE order_id=$1`, orderID))
		require.Equal(t, "converted", f.str(`SELECT status FROM reservations WHERE id=$1`, resID))
		require.Equal(t, 1, f.reservedStock(v), "committed stock remains held until shipment")

		// 2. Worker expiration runs now on 40-minute-old timestamp
		f.exec(`UPDATE orders SET created_at = now() - interval '40 minutes' WHERE id=$1`, orderID)
		res, err := f.svc.ExpireAwaitingPaymentOrders(f.ctx, time.Now().Add(-30*time.Minute), 100)
		require.NoError(t, err)
		require.Equal(t, 0, res.Expired, "paid order must not be expired by worker")

		// Assert order and stock remain untouched
		require.Equal(t, "paid", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, "converted", f.str(`SELECT status FROM reservations WHERE id=$1`, resID))
		require.Equal(t, 1, f.reservedStock(v))
		require.Equal(t, 5, f.totalStock(v))
	})

	t.Run("I. Race Outcome 2: Expiration first -> order cancelled, reservation released; late bank confirmation rejected, PAY.RECON.1 detected", func(t *testing.T) {
		f := newLifecycleFixture(t)
		p, v := f.variant(150000, 5)
		buyerID := f.buyerWithCart(p, v, 1)
		orderID := f.createOrder(buyerID)

		require.Equal(t, 1, f.reservedStock(v))
		var resID uuid.UUID
		require.NoError(t, f.db.QueryRow(f.ctx, `SELECT reservation_id FROM order_reservations WHERE order_id=$1`, orderID).Scan(&resID))

		// Initialize payment
		payResp, err := f.paymentsSvc.CreatePayment(f.ctx, buyerID, orderID, "tpay")
		require.NoError(t, err)

		// Suppose client/gateway timed out locally before bank reported
		f.exec(`UPDATE payments SET status = 'failed', failed_at = now() WHERE id=$1`, payResp.PaymentID)
		f.exec(`UPDATE orders SET created_at = now() - interval '40 minutes' WHERE id=$1`, orderID)

		// 1. Worker expiration runs first
		expRes, err := f.svc.ExpireAwaitingPaymentOrders(f.ctx, time.Now().Add(-30*time.Minute), 100)
		require.NoError(t, err)
		require.Equal(t, 1, expRes.Expired)
		require.Equal(t, 1, expRes.ReleasedReservations)

		// Assert order is cancelled, fulfillment cancelled, reservation released
		require.Equal(t, "cancelled", f.str(`SELECT status FROM orders WHERE id=$1`, orderID))
		require.Equal(t, "cancelled", f.str(`SELECT status FROM order_fulfillments WHERE order_id=$1`, orderID))
		require.Equal(t, "released", f.str(`SELECT status FROM reservations WHERE id=$1`, resID))
		require.Equal(t, 0, f.reservedStock(v))
		require.Equal(t, 5, f.totalStock(v))

		// 2. Now a late bank confirmation arrives for the cancelled order
		providerID := f.str(`SELECT provider_payment_id FROM payments WHERE id=$1`, payResp.PaymentID)
		webhookBody := f.signedWebhook(orderID, providerID, 150000+39000, "CONFIRMED")
		err = f.paymentsSvc.HandleWebhook(f.ctx, nil, webhookBody)
		require.NoError(t, err) // Handled safely without error

		// Assert order is NOT paid, reservation is NOT converted, stock is NOT double-modified
		require.Equal(t, "cancelled", f.str(`SELECT status FROM orders WHERE id=$1`, orderID), "order must remain cancelled")
		require.Equal(t, "cancelled", f.str(`SELECT status FROM order_fulfillments WHERE order_id=$1`, orderID))
		require.Equal(t, "released", f.str(`SELECT status FROM reservations WHERE id=$1`, resID))
		require.Equal(t, 0, f.reservedStock(v))
		require.Equal(t, 5, f.totalStock(v))

		// 3. Assert PAY.RECON.1 detects critical discrepancy for Admin
		adminDetail, err := f.paymentsSvc.GetAdminPaymentDetail(f.ctx, payResp.PaymentID)
		require.NoError(t, err)
		var problemCodes []string
		for _, prob := range adminDetail.Problems {
			problemCodes = append(problemCodes, prob.Code)
		}
		require.Contains(t, problemCodes, "BANK_CONFIRMED_PAYMENT_NOT_SUCCEEDED", "late bank confirmation on cancelled order must flag critical discrepancy")

		// Assert listed in admin payments filter
		conflicts, _, err := f.paymentsSvc.ListAdminPayments(f.ctx, "", "", "", "", "", "", "BANK_CONFIRMED_PAYMENT_NOT_SUCCEEDED", "", "", 0, 0, false, "", "", 50, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(conflicts))
	})
}
