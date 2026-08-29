package orders

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdminOrderPaymentStatus(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Skipping test: TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	userID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Payment Test User', $2, 'hash', 'customer', 'active', false, now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID, "paytest-"+userID.String()[:8]+"@zamk.local"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Paid + Shipped order with succeeded payment
	shippedOrderID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'shipped', 1299000, 'RUB', 'Test Buyer', '9991234567', 'buyer@test.local', 'Test Address', now(), now())
	`, shippedOrderID, userID)
	if err != nil {
		t.Fatalf("failed to create shipped order: %v", err)
	}

	paymentID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at, paid_at)
		VALUES ($1, $2, 'tbank', 'test-prov-1', 'succeeded', 1299000, 'RUB', 'https://pay.url', $3, $4, 'tpay', 'mock', now(), now(), now())
	`, paymentID, shippedOrderID, uuid.New().String(), "PAY-TEST-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	// 2. Pending order with pending payment
	pendingOrderID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'awaiting_payment', 500000, 'RUB', 'Pending Buyer', '9991234568', 'pending@test.local', 'Test Address 2', now(), now())
	`, pendingOrderID, userID)
	if err != nil {
		t.Fatalf("failed to create pending order: %v", err)
	}

	pendingPaymentID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at)
		VALUES ($1, $2, 'tbank', 'test-prov-2', 'pending', 500000, 'RUB', 'https://pay.url/2', $3, $4, 'tpay', 'mock', now(), now())
	`, pendingPaymentID, pendingOrderID, uuid.New().String(), "PAY-TEST-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("failed to create pending payment: %v", err)
	}

	// 3. Shipped order with NO payment rows -> must be pending (NOT paid)
	shippedNoPaymentOrderID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'shipped', 700000, 'RUB', 'No Pay Buyer', '9991234569', 'nopay@test.local', 'Test Address 3', now(), now())
	`, shippedNoPaymentOrderID, userID)
	if err != nil {
		t.Fatalf("failed to create shipped order without payment: %v", err)
	}

	// 4. Packed order with failed payment -> must be failed
	packedFailedOrderID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'packed', 800000, 'RUB', 'Failed Buyer', '9991234570', 'failed@test.local', 'Test Address 4', now(), now())
	`, packedFailedOrderID, userID)
	if err != nil {
		t.Fatalf("failed to create packed failed order: %v", err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at)
		VALUES ($1, $2, 'tbank', 'test-prov-failed', 'failed', 800000, 'RUB', 'https://pay.url/f', $3, $4, 'tpay', 'mock', now(), now())
	`, uuid.New(), packedFailedOrderID, uuid.New().String(), "PAY-TEST-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("failed to create failed payment: %v", err)
	}

	// 5. Delivered order with cancelled payment -> must be cancelled
	deliveredCancelledOrderID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'delivered', 900000, 'RUB', 'Cancelled Buyer', '9991234571', 'cancelled@test.local', 'Test Address 5', now(), now())
	`, deliveredCancelledOrderID, userID)
	if err != nil {
		t.Fatalf("failed to create delivered cancelled order: %v", err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at)
		VALUES ($1, $2, 'tbank', 'test-prov-canc', 'cancelled', 900000, 'RUB', 'https://pay.url/c', $3, $4, 'tpay', 'mock', now(), now())
	`, uuid.New(), deliveredCancelledOrderID, uuid.New().String(), "PAY-TEST-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("failed to create cancelled payment: %v", err)
	}

	// 6. Multiple attempts with any succeeded payment -> must be paid
	multiAttemptOrderID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'assembling', 1100000, 'RUB', 'Multi Attempt Buyer', '9991234572', 'multi@test.local', 'Test Address 6', now(), now())
	`, multiAttemptOrderID, userID)
	if err != nil {
		t.Fatalf("failed to create multi attempt order: %v", err)
	}
	// Attempt 1: Failed
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at)
		VALUES ($1, $2, 'tbank', 'test-prov-multi-1', 'failed', 1100000, 'RUB', 'https://pay.url/m1', $3, $4, 'tpay', 'mock', now() - interval '5 minutes', now() - interval '5 minutes')
	`, uuid.New(), multiAttemptOrderID, uuid.New().String(), "PAY-TEST-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("failed to create multi attempt 1 payment: %v", err)
	}
	// Attempt 2: Succeeded
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at, paid_at)
		VALUES ($1, $2, 'tbank', 'test-prov-multi-2', 'succeeded', 1100000, 'RUB', 'https://pay.url/m2', $3, $4, 'tpay', 'mock', now(), now(), now())
	`, uuid.New(), multiAttemptOrderID, uuid.New().String(), "PAY-TEST-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("failed to create multi attempt 2 payment: %v", err)
	}

	// 7. Delivered order with pending payment -> must remain pending (not inferred from delivered)
	deliveredPendingOrderID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'delivered', 1500000, 'RUB', 'Delivered Pending Buyer', '9991234573', 'delpending@test.local', 'Test Address 7', now(), now())
	`, deliveredPendingOrderID, userID)
	if err != nil {
		t.Fatalf("failed to create delivered pending order: %v", err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at)
		VALUES ($1, $2, 'tbank', 'test-prov-del-pend', 'pending', 1500000, 'RUB', 'https://pay.url/dp', $3, $4, 'tpay', 'mock', now(), now())
	`, uuid.New(), deliveredPendingOrderID, uuid.New().String(), "PAY-TEST-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("failed to create delivered pending payment: %v", err)
	}

	// === VERIFY GetAdminOrderDetail ===
	// 1. Shipped + succeeded -> paid
	detailShipped, err := repo.GetAdminOrderDetail(ctx, shippedOrderID)
	if err != nil {
		t.Fatalf("failed to get shipped order detail: %v", err)
	}
	if detailShipped.PaymentStatus != "paid" {
		t.Errorf("expected payment status 'paid' for shipped order, got '%s'", detailShipped.PaymentStatus)
	}

	// 2. Pending -> pending
	detailPending, err := repo.GetAdminOrderDetail(ctx, pendingOrderID)
	if err != nil {
		t.Fatalf("failed to get pending order detail: %v", err)
	}
	if detailPending.PaymentStatus != "pending" {
		t.Errorf("expected payment status 'pending', got '%s'", detailPending.PaymentStatus)
	}

	// 3. Shipped + NO payment row -> pending (NOT paid)
	detailNoPay, err := repo.GetAdminOrderDetail(ctx, shippedNoPaymentOrderID)
	if err != nil {
		t.Fatalf("failed to get shipped no pay order detail: %v", err)
	}
	if detailNoPay.PaymentStatus != "pending" {
		t.Errorf("expected payment status 'pending' (NOT paid) for shipped order without payment, got '%s'", detailNoPay.PaymentStatus)
	}

	// 4. Packed + failed -> failed
	detailFailed, err := repo.GetAdminOrderDetail(ctx, packedFailedOrderID)
	if err != nil {
		t.Fatalf("failed to get packed failed order detail: %v", err)
	}
	if detailFailed.PaymentStatus != "failed" {
		t.Errorf("expected payment status 'failed', got '%s'", detailFailed.PaymentStatus)
	}

	// 5. Delivered + cancelled -> cancelled
	detailCancelled, err := repo.GetAdminOrderDetail(ctx, deliveredCancelledOrderID)
	if err != nil {
		t.Fatalf("failed to get delivered cancelled order detail: %v", err)
	}
	if detailCancelled.PaymentStatus != "cancelled" {
		t.Errorf("expected payment status 'cancelled', got '%s'", detailCancelled.PaymentStatus)
	}

	// 6. Multi attempts (failed + succeeded) -> paid
	detailMulti, err := repo.GetAdminOrderDetail(ctx, multiAttemptOrderID)
	if err != nil {
		t.Fatalf("failed to get multi attempt order detail: %v", err)
	}
	if detailMulti.PaymentStatus != "paid" {
		t.Errorf("expected payment status 'paid' for multi-attempt order with 1 succeeded, got '%s'", detailMulti.PaymentStatus)
	}

	// 7. Delivered + pending -> pending
	detailDelPend, err := repo.GetAdminOrderDetail(ctx, deliveredPendingOrderID)
	if err != nil {
		t.Fatalf("failed to get delivered pending order detail: %v", err)
	}
	if detailDelPend.PaymentStatus != "pending" {
		t.Errorf("expected payment status 'pending' for delivered order with pending payment, got '%s'", detailDelPend.PaymentStatus)
	}

	// === VERIFY ListAdminOrders ===
	list, _, err := repo.ListAdminOrders(ctx, "", "", "", "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("failed to list admin orders: %v", err)
	}

	listMap := make(map[uuid.UUID]string)
	for _, o := range list {
		listMap[o.ID] = o.PaymentStatus
	}

	if listMap[shippedOrderID] != "paid" {
		t.Errorf("list: expected shipped order paymentStatus 'paid', got '%s'", listMap[shippedOrderID])
	}
	if listMap[shippedNoPaymentOrderID] != "pending" {
		t.Errorf("list: expected shipped order without payment paymentStatus 'pending', got '%s'", listMap[shippedNoPaymentOrderID])
	}
	if listMap[packedFailedOrderID] != "failed" {
		t.Errorf("list: expected packed failed order paymentStatus 'failed', got '%s'", listMap[packedFailedOrderID])
	}
	if listMap[deliveredCancelledOrderID] != "cancelled" {
		t.Errorf("list: expected delivered cancelled order paymentStatus 'cancelled', got '%s'", listMap[deliveredCancelledOrderID])
	}
	if listMap[multiAttemptOrderID] != "paid" {
		t.Errorf("list: expected multi attempt order paymentStatus 'paid', got '%s'", listMap[multiAttemptOrderID])
	}
	if listMap[deliveredPendingOrderID] != "pending" {
		t.Errorf("list: expected delivered pending order paymentStatus 'pending', got '%s'", listMap[deliveredPendingOrderID])
	}

	// === VERIFY ListAdminOrders Filters ===
	// Filter: paymentStatus = "paid"
	paidList, _, err := repo.ListAdminOrders(ctx, "", "", "paid", "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("failed to list paid orders: %v", err)
	}
	paidIDs := make(map[uuid.UUID]bool)
	for _, o := range paidList {
		paidIDs[o.ID] = true
	}
	if !paidIDs[shippedOrderID] {
		t.Error("filter paid: expected shippedOrderID in paid list")
	}
	if !paidIDs[multiAttemptOrderID] {
		t.Error("filter paid: expected multiAttemptOrderID in paid list")
	}
	if paidIDs[shippedNoPaymentOrderID] {
		t.Error("filter paid: shippedNoPaymentOrderID must NOT be in paid list")
	}
	if paidIDs[packedFailedOrderID] {
		t.Error("filter paid: packedFailedOrderID must NOT be in paid list")
	}

	// Filter: paymentStatus = "failed"
	failedList, _, err := repo.ListAdminOrders(ctx, "", "", "failed", "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("failed to list failed orders: %v", err)
	}
	failedIDs := make(map[uuid.UUID]bool)
	for _, o := range failedList {
		failedIDs[o.ID] = true
	}
	if !failedIDs[packedFailedOrderID] {
		t.Error("filter failed: expected packedFailedOrderID in failed list")
	}
	if failedIDs[multiAttemptOrderID] {
		t.Error("filter failed: multiAttemptOrderID (has succeeded) must NOT be in failed list")
	}

	// Filter: paymentStatus = "pending"
	pendingList, _, err := repo.ListAdminOrders(ctx, "", "", "pending", "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("failed to list pending orders: %v", err)
	}
	pendingIDs := make(map[uuid.UUID]bool)
	for _, o := range pendingList {
		pendingIDs[o.ID] = true
	}
	if !pendingIDs[shippedNoPaymentOrderID] {
		t.Error("filter pending: expected shippedNoPaymentOrderID in pending list")
	}
	if !pendingIDs[deliveredPendingOrderID] {
		t.Error("filter pending: expected deliveredPendingOrderID in pending list")
	}
	if pendingIDs[shippedOrderID] {
		t.Error("filter pending: shippedOrderID (paid) must NOT be in pending list")
	}
}

func TestAdminOrderTimeline(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Skipping test: TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	userID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Timeline User', $2, 'hash', 'customer', 'active', false, now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID, "timeuser-"+userID.String()[:8]+"@zamk.local"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	orderID := uuid.New()
	t0 := time.Now().Add(-1 * time.Hour)
	t1 := t0.Add(1 * time.Minute)
	t2 := t0.Add(5 * time.Minute)
	t3 := t0.Add(15 * time.Minute)
	t4 := t0.Add(30 * time.Minute)

	// 1. Order created
	_, err = db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'shipped', 250000, 'RUB', 'Timeline Buyer', '9991112233', 'timeline@test.local', 'Timeline Address', $3, $4)
	`, orderID, userID, t0, t4)
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// 2. Payment succeeded
	payNum := "PAY-TIME-" + uuid.New().String()[:8]
	_, err = db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at, paid_at)
		VALUES ($1, $2, 'tbank', 'test-timeline-pay', 'succeeded', 250000, 'RUB', 'https://pay.url/t', $3, $4, 'tpay', 'mock', $5, $5, $5)
	`, uuid.New(), orderID, uuid.New().String(), payNum, t1)
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	// 3. Status history records
	statuses := []struct {
		from, to string
		ts       time.Time
	}{
		{"", "awaiting_payment", t0},
		{"awaiting_payment", "paid", t1},
		{"paid", "assembling", t2},
		{"assembling", "packed", t3},
		{"packed", "shipped", t4},
	}
	for _, st := range statuses {
		var fromPtr *string
		if st.from != "" {
			fromPtr = &st.from
		}
		_, err = db.Exec(ctx, `
			INSERT INTO order_status_history (id, order_id, from_status, to_status, actor_user_id, comment, created_at)
			VALUES ($1, $2, $3, $4, NULL, 'test status transition', $5)
		`, uuid.New(), orderID, fromPtr, st.to, st.ts)
		if err != nil {
			t.Fatalf("failed to insert status history %s: %v", st.to, err)
		}
	}

	sellerID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, status, created_at, updated_at)
		VALUES ($1, 'Timeline Seller', $2, 'active', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, sellerID, "time-seller-"+sellerID.String()[:8]); err != nil {
		t.Fatalf("failed to create seller: %v", err)
	}

	// 4. Fulfillment
	fulfID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, packed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', $4, $5, $6)
	`, fulfID, orderID, sellerID, t3, t0, t4)
	if err != nil {
		t.Fatalf("failed to insert fulfillment: %v", err)
	}

	// 5. Shipment
	_, err = db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 'СДЭК', 'TRK-12345', $4, $4, $4)
	`, uuid.New(), orderID, fulfID, t4)
	if err != nil {
		t.Fatalf("failed to insert shipment: %v", err)
	}

	// Fetch detail with timeline
	detail, err := repo.GetAdminOrderDetail(ctx, orderID)
	if err != nil {
		t.Fatalf("failed to get order detail: %v", err)
	}

	if len(detail.Timeline) <= 1 {
		t.Fatalf("expected multiple timeline events, got only %d", len(detail.Timeline))
	}

	expectedTitles := []string{
		"Заказ создан",
		"Оплата подтверждена",
		"В сборке",
		"Упаковка завершена",
		"Отгружен со склада",
	}

	if len(detail.Timeline) != len(expectedTitles) {
		t.Fatalf("expected exactly %d timeline events, got %d: %+v", len(expectedTitles), len(detail.Timeline), detail.Timeline)
	}

	for i, expected := range expectedTitles {
		if detail.Timeline[i].Title != expected {
			t.Errorf("event [%d]: expected title '%s', got '%s'", i, expected, detail.Timeline[i].Title)
		}
		if i > 0 {
			if detail.Timeline[i].Timestamp.Before(detail.Timeline[i-1].Timestamp) {
				t.Errorf("event [%d] is not chronologically ordered after [%d]", i, i-1)
			}
		}
	}

	// Check context enriched
	var foundPaymentContext, foundShipmentContext bool
	expectedPayCtx := fmt.Sprintf("%s (tbank)", payNum)
	for _, ev := range detail.Timeline {
		if ev.Title == "Оплата подтверждена" && ev.Context != nil && *ev.Context == expectedPayCtx {
			foundPaymentContext = true
		}
		if ev.Title == "Отгружен со склада" && ev.Context != nil && *ev.Context == "СДЭК (TRK-12345)" {
			foundShipmentContext = true
		}
	}

	if !foundPaymentContext {
		t.Errorf("payment event missing expected context %s", expectedPayCtx)
	}
	if !foundShipmentContext {
		t.Error("shipment event missing expected context СДЭК (TRK-12345)")
	}
}
