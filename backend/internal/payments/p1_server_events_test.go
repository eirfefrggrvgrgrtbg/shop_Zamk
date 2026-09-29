package payments

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingPaymentBehaviorWriter struct {
	real *behavior.Service
	err  error
}

func (f *failingPaymentBehaviorWriter) InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []behavior.BehavioralEvent) error {
	return f.err
}

func (f *failingPaymentBehaviorWriter) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]behavior.ProductBehaviorSnapshot, error) {
	return f.real.ResolveCategoriesTx(ctx, tx, productIDs)
}

type emptySnapshotBehaviorWriter struct {
	real *behavior.Service
}

func (e *emptySnapshotBehaviorWriter) InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []behavior.BehavioralEvent) error {
	return e.real.InsertServerEventsTx(ctx, tx, events)
}

func (e *emptySnapshotBehaviorWriter) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]behavior.ProductBehaviorSnapshot, error) {
	// Simulates a missing product entry in returned snapshot map
	return make(map[uuid.UUID]behavior.ProductBehaviorSnapshot), nil
}

type paymentEventTestItemSpec struct {
	Quantity int
}

type paymentEventTestItem struct {
	CategoryID    uuid.UUID
	ProductID     uuid.UUID
	VariantID     uuid.UUID
	OrderItemID   uuid.UUID
	InvItemID     uuid.UUID
	ReservationID uuid.UUID
	OrderResID    uuid.UUID
	Quantity      int
}

type paymentEventFixture struct {
	Client        *postgres.Client
	BehaviorSvc   *behavior.Service
	SellerID      uuid.UUID
	UserID        uuid.UUID
	OrderID       uuid.UUID
	PaymentID     uuid.UUID
	FulfillmentID uuid.UUID
	Items         []paymentEventTestItem
}

func newPaymentServerEventsEnv(t *testing.T, writerOverride behavior.EventWriter) (*postgres.Client, *Service, *behavior.Service) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	client, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
	})
	testutil.AssertTestDatabase(t, client.Pool)

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	invSvc := inventory.NewService(inventory.NewRepository(client.Pool), nil, client)
	notifSvc := notifications.NewService(notifications.NewRepository(client), nil, nil)
	behaviorSvc := behavior.NewService(behavior.NewRepository(client))
	tbankProvider := NewTBankProvider("STUB", "STUB", "", "", "", true, "O", "mock")
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}

	var writer behavior.EventWriter = behaviorSvc
	if writerOverride != nil {
		writer = writerOverride
	}

	svc := NewService(repo, ordersRepo, invSvc, tbankProvider, client, notifSvc, writer, cfg)
	return client, svc, behaviorSvc
}

func seedPaymentEventFixture(t *testing.T, client *postgres.Client, specs []paymentEventTestItemSpec) *paymentEventFixture {
	t.Helper()
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	fix := &paymentEventFixture{
		Client:        client,
		SellerID:      uuid.New(),
		UserID:        uuid.New(),
		OrderID:       uuid.New(),
		PaymentID:     uuid.New(),
		FulfillmentID: uuid.New(),
		Items:         make([]paymentEventTestItem, len(specs)),
	}

	var catIDs, prodIDs, varIDs, oiIDs, invIDs, resIDs, orderResIDs []uuid.UUID
	for i, sp := range specs {
		it := paymentEventTestItem{
			CategoryID:    uuid.New(),
			ProductID:     uuid.New(),
			VariantID:     uuid.New(),
			OrderItemID:   uuid.New(),
			InvItemID:     uuid.New(),
			ReservationID: uuid.New(),
			OrderResID:    uuid.New(),
			Quantity:      sp.Quantity,
		}
		fix.Items[i] = it
		catIDs = append(catIDs, it.CategoryID)
		prodIDs = append(prodIDs, it.ProductID)
		varIDs = append(varIDs, it.VariantID)
		oiIDs = append(oiIDs, it.OrderItemID)
		invIDs = append(invIDs, it.InvItemID)
		resIDs = append(resIDs, it.ReservationID)
		orderResIDs = append(orderResIDs, it.OrderResID)
	}

	// Register scoped cleanup BEFORE any DB mutation and check every error
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, err := client.Pool.Exec(cleanCtx, "DELETE FROM behavioral_events WHERE order_id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM notifications WHERE entity_id = $1 OR entity_id = $2", fix.OrderID, fix.FulfillmentID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_status_history WHERE order_id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_reservations WHERE id = ANY($1)", orderResIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM reservations WHERE id = ANY($1)", resIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM inventory_items WHERE id = ANY($1)", invIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_items WHERE id = ANY($1)", oiIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_fulfillments WHERE id = $1", fix.FulfillmentID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM payment_events WHERE payment_id = $1", fix.PaymentID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM payments WHERE id = $1", fix.PaymentID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM orders WHERE id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM product_variants WHERE id = ANY($1)", varIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM products WHERE id = ANY($1)", prodIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM categories WHERE id = ANY($1)", catIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM sellers WHERE id = $1", fix.SellerID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM users WHERE id = $1", fix.UserID)
		require.NoError(t, err)
	})

	_, err := client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Seller', $2, 'seller@t.t', 'active')", fix.SellerID, "seller-"+fix.SellerID.String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO users (id, role, name, email, password_hash) VALUES ($1, 'customer', 'Test User', $2, 'hash')", fix.UserID, "user-"+fix.UserID.String()+"@example.com")
	require.NoError(t, err)

	var totalCents int64
	for _, sp := range specs {
		totalCents += int64(sp.Quantity) * 1000
	}

	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'awaiting_payment', $3, 'Test User', '12345', 't@t.t', 'Addr')", fix.OrderID, fix.UserID, totalCents)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO payments (id, order_id, amount_cents, currency, provider, status, idempotency_key, integration_mode) VALUES ($1, $2, $3, 'RUB', 'tbank', 'pending', $4, 'mock')", fix.PaymentID, fix.OrderID, totalCents, "idem-"+fix.PaymentID.String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents) VALUES ($1, $2, $3, 'awaiting_payment', $4, 1000, $5)", fix.FulfillmentID, fix.OrderID, fix.SellerID, totalCents, totalCents*9/10)
	require.NoError(t, err)

	for i, it := range fix.Items {
		_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, slug, name, parent_id, is_active, sort_order) VALUES ($1, $2, 'Cat', NULL, true, 0)", it.CategoryID, "cat-"+it.CategoryID.String())
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, status, price_cents, category_id) VALUES ($1, $2, 'test product', $3, 'published', 1000, $4)", it.ProductID, fix.SellerID, "prod-"+it.ProductID.String(), it.CategoryID)
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, size, color, sku, price_cents) VALUES ($1, $2, 'M', 'Black', $3, 1000)", it.VariantID, it.ProductID, "SKU-"+it.VariantID.String())
		require.NoError(t, err)

		createdAt := time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		_, err = client.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'item', 'slug', 'M', 'Black', 'SKU', 1000, $7, $8, $9)",
			it.OrderItemID, fix.OrderID, fix.FulfillmentID, it.ProductID, it.VariantID, fix.SellerID, it.Quantity, int64(it.Quantity)*1000, createdAt)
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 20, $5)", it.InvItemID, it.ProductID, it.VariantID, fix.SellerID, it.Quantity)
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, quantity, expires_at, status) VALUES ($1, $2, $3, $4, $5, now() + interval '1 hour', 'active')", it.ReservationID, it.InvItemID, it.ProductID, it.VariantID, it.Quantity)
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO order_reservations (id, order_id, reservation_id) VALUES ($1, $2, $3)", it.OrderResID, fix.OrderID, it.ReservationID)
		require.NoError(t, err)
	}

	return fix
}

type persistedServerEventRow struct {
	ID          uuid.UUID
	EventType   string
	Source      string
	VisitorID   *uuid.UUID
	UserID      *uuid.UUID
	ProductID   *uuid.UUID
	VariantID   *uuid.UUID
	CategoryID  *uuid.UUID
	OrderID     *uuid.UUID
	ReturnID    *uuid.UUID
	OrderItemID *uuid.UUID
	Quantity    *int
	OccurredAt  time.Time
	Metadata    json.RawMessage
}

func queryOrderBehavioralEvents(t *testing.T, client *postgres.Client, orderID uuid.UUID, eventType string) []persistedServerEventRow {
	t.Helper()
	rows, err := client.Pool.Query(context.Background(), `
		SELECT id, event_type, source, visitor_id, user_id, product_id, variant_id, category_id, order_id, return_id, order_item_id, quantity, occurred_at, metadata
		FROM behavioral_events
		WHERE order_id = $1 AND event_type = $2
		ORDER BY order_item_id ASC
	`, orderID, eventType)
	require.NoError(t, err)
	defer rows.Close()

	var res []persistedServerEventRow
	for rows.Next() {
		var r persistedServerEventRow
		err := rows.Scan(
			&r.ID, &r.EventType, &r.Source, &r.VisitorID, &r.UserID,
			&r.ProductID, &r.VariantID, &r.CategoryID, &r.OrderID, &r.ReturnID,
			&r.OrderItemID, &r.Quantity, &r.OccurredAt, &r.Metadata,
		)
		require.NoError(t, err)
		res = append(res, r)
	}
	require.NoError(t, rows.Err())
	return res
}

// A, D, 8: Single item success, exact fields, and exact canonical timestamp equality.
func TestOrderPaidServerEvents_SingleItemAndExactFieldsAndTimestamp(t *testing.T) {
	client, svc, _ := newPaymentServerEventsEnv(t, nil)
	fix := seedPaymentEventFixture(t, client, []paymentEventTestItemSpec{{Quantity: 1}})
	ctx := context.Background()

	err := svc.ProcessMockPaymentAction(ctx, fix.PaymentID, "confirm")
	require.NoError(t, err)

	var orderUpdatedAt time.Time
	err = client.Pool.QueryRow(ctx, "SELECT updated_at FROM orders WHERE id = $1", fix.OrderID).Scan(&orderUpdatedAt)
	require.NoError(t, err)

	events := queryOrderBehavioralEvents(t, client, fix.OrderID, "order_paid")
	require.Len(t, events, 1)

	ev := events[0]
	it := fix.Items[0]
	expectedID := behavior.DeterministicServerEventID("order_paid", fix.OrderID, it.OrderItemID)

	assert.Equal(t, expectedID, ev.ID)
	assert.Equal(t, "order_paid", ev.EventType)
	assert.Equal(t, "server", ev.Source)
	assert.Nil(t, ev.VisitorID)
	require.NotNil(t, ev.UserID)
	assert.Equal(t, fix.UserID, *ev.UserID)
	require.NotNil(t, ev.OrderID)
	assert.Equal(t, fix.OrderID, *ev.OrderID)
	require.NotNil(t, ev.OrderItemID)
	assert.Equal(t, it.OrderItemID, *ev.OrderItemID)
	require.NotNil(t, ev.ProductID)
	assert.Equal(t, it.ProductID, *ev.ProductID)
	require.NotNil(t, ev.VariantID)
	assert.Equal(t, it.VariantID, *ev.VariantID)
	require.NotNil(t, ev.CategoryID)
	assert.Equal(t, it.CategoryID, *ev.CategoryID)
	assert.Nil(t, ev.ReturnID)
	require.NotNil(t, ev.Quantity)
	assert.Equal(t, 1, *ev.Quantity)
	assert.JSONEq(t, `{}`, string(ev.Metadata))
	assert.True(t, ev.OccurredAt.Equal(orderUpdatedAt), "behavioral_events.occurred_at (%v) must equal orders.updated_at (%v)", ev.OccurredAt, orderUpdatedAt)
}

// B: Two distinct items -> exactly 2 order_paid rows.
func TestOrderPaidServerEvents_TwoDistinctItems(t *testing.T) {
	client, svc, _ := newPaymentServerEventsEnv(t, nil)
	fix := seedPaymentEventFixture(t, client, []paymentEventTestItemSpec{{Quantity: 1}, {Quantity: 2}})
	ctx := context.Background()

	err := svc.ProcessMockPaymentAction(ctx, fix.PaymentID, "confirm")
	require.NoError(t, err)

	var orderUpdatedAt time.Time
	err = client.Pool.QueryRow(ctx, "SELECT updated_at FROM orders WHERE id = $1", fix.OrderID).Scan(&orderUpdatedAt)
	require.NoError(t, err)

	events := queryOrderBehavioralEvents(t, client, fix.OrderID, "order_paid")
	require.Len(t, events, 2)

	byItemID := make(map[uuid.UUID]persistedServerEventRow)
	for _, ev := range events {
		require.NotNil(t, ev.OrderItemID)
		byItemID[*ev.OrderItemID] = ev
	}

	for _, it := range fix.Items {
		ev, ok := byItemID[it.OrderItemID]
		require.True(t, ok, "missing event for order_item_id %s", it.OrderItemID)
		assert.Equal(t, behavior.DeterministicServerEventID("order_paid", fix.OrderID, it.OrderItemID), ev.ID)
		assert.Equal(t, it.ProductID, *ev.ProductID)
		assert.Equal(t, it.VariantID, *ev.VariantID)
		assert.Equal(t, it.CategoryID, *ev.CategoryID)
		assert.Equal(t, it.Quantity, *ev.Quantity)
		assert.True(t, ev.OccurredAt.Equal(orderUpdatedAt))
	}
}

// C: Quantity = 3 -> one row with quantity = 3.
func TestOrderPaidServerEvents_QuantityThree(t *testing.T) {
	client, svc, _ := newPaymentServerEventsEnv(t, nil)
	fix := seedPaymentEventFixture(t, client, []paymentEventTestItemSpec{{Quantity: 3}})
	ctx := context.Background()

	err := svc.ProcessMockPaymentAction(ctx, fix.PaymentID, "confirm")
	require.NoError(t, err)

	events := queryOrderBehavioralEvents(t, client, fix.OrderID, "order_paid")
	require.Len(t, events, 1)
	require.NotNil(t, events[0].Quantity)
	assert.Equal(t, 3, *events[0].Quantity)
}

// E: Retry / replay of same canonical payment fact -> no duplicate behavioral row.
func TestOrderPaidServerEvents_RetryReplayIdempotency(t *testing.T) {
	client, svc, _ := newPaymentServerEventsEnv(t, nil)
	fix := seedPaymentEventFixture(t, client, []paymentEventTestItemSpec{{Quantity: 2}})
	ctx := context.Background()

	err := svc.ProcessMockPaymentAction(ctx, fix.PaymentID, "confirm")
	require.NoError(t, err)

	// Replay 1: Second domain confirm call is a no-op / rejected and does not duplicate events
	_ = svc.ProcessMockPaymentAction(ctx, fix.PaymentID, "confirm")

	// Replay 2: Direct replay of emitOrderPaidEventsTx with same canonical order & item IDs inside a transaction
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		order, err := svc.ordersRepo.GetOrderForUpdateTx(ctx, tx, fix.OrderID)
		if err != nil {
			return err
		}
		return svc.emitOrderPaidEventsTx(ctx, tx, order, order.UpdatedAt)
	})
	require.NoError(t, err)

	events := queryOrderBehavioralEvents(t, client, fix.OrderID, "order_paid")
	require.Len(t, events, 1)
}

// F: Failed payment -> zero order_paid rows.
func TestOrderPaidServerEvents_FailedPaymentEmitsZeroEvents(t *testing.T) {
	client, svc, _ := newPaymentServerEventsEnv(t, nil)
	fix := seedPaymentEventFixture(t, client, []paymentEventTestItemSpec{{Quantity: 1}})
	ctx := context.Background()

	err := svc.ProcessMockPaymentAction(ctx, fix.PaymentID, "reject")
	require.NoError(t, err)

	var payStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", fix.PaymentID).Scan(&payStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", payStatus)

	events := queryOrderBehavioralEvents(t, client, fix.OrderID, "order_paid")
	assert.Empty(t, events)
}

// G & 4: Forced behavior writer failure and missing canonical product snapshot both roll back payment/order transition.
func TestOrderPaidServerEvents_WriterFailureAndMissingProductRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("forced_behavior_writer_failure_rolls_back_payment_and_order", func(t *testing.T) {
		client, _, realBehaviorSvc := newPaymentServerEventsEnv(t, nil)
		failWriter := &failingPaymentBehaviorWriter{
			real: realBehaviorSvc,
			err:  errors.New("simulated behavioral writer failure"),
		}
		_, failingSvc, _ := newPaymentServerEventsEnv(t, failWriter)
		fix := seedPaymentEventFixture(t, client, []paymentEventTestItemSpec{{Quantity: 1}})

		err := failingSvc.ProcessMockPaymentAction(ctx, fix.PaymentID, "confirm")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated behavioral writer failure")

		var payStatus, ordStatus, fulStatus, resStatus string
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", fix.PaymentID).Scan(&payStatus))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", fix.OrderID).Scan(&ordStatus))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM order_fulfillments WHERE id = $1", fix.FulfillmentID).Scan(&fulStatus))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM reservations WHERE id = $1", fix.Items[0].ReservationID).Scan(&resStatus))

		assert.Equal(t, "pending", payStatus)
		assert.Equal(t, "awaiting_payment", ordStatus)
		assert.Equal(t, "awaiting_payment", fulStatus)
		assert.Equal(t, "active", resStatus)

		events := queryOrderBehavioralEvents(t, client, fix.OrderID, "order_paid")
		assert.Empty(t, events)
	})

	t.Run("missing_product_in_snapshot_returns_invariant_error_and_rolls_back", func(t *testing.T) {
		client, _, realBehaviorSvc := newPaymentServerEventsEnv(t, nil)
		emptyWriter := &emptySnapshotBehaviorWriter{real: realBehaviorSvc}
		_, svcWithEmptySnap, _ := newPaymentServerEventsEnv(t, emptyWriter)
		fix := seedPaymentEventFixture(t, client, []paymentEventTestItemSpec{{Quantity: 1}})

		err := svcWithEmptySnap.ProcessMockPaymentAction(ctx, fix.PaymentID, "confirm")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "canonical product")

		var payStatus, ordStatus string
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", fix.PaymentID).Scan(&payStatus))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", fix.OrderID).Scan(&ordStatus))
		assert.Equal(t, "pending", payStatus)
		assert.Equal(t, "awaiting_payment", ordStatus)

		events := queryOrderBehavioralEvents(t, client, fix.OrderID, "order_paid")
		assert.Empty(t, events)
	})
}
