package fulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingDeliveryBehaviorWriter struct {
	real *behavior.Service
	err  error
}

func (f *failingDeliveryBehaviorWriter) InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []behavior.BehavioralEvent) error {
	return f.err
}

func (f *failingDeliveryBehaviorWriter) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]behavior.ProductBehaviorSnapshot, error) {
	return f.real.ResolveCategoriesTx(ctx, tx, productIDs)
}

type emptyDeliverySnapshotWriter struct {
	real *behavior.Service
}

func (e *emptyDeliverySnapshotWriter) InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []behavior.BehavioralEvent) error {
	return e.real.InsertServerEventsTx(ctx, tx, events)
}

func (e *emptyDeliverySnapshotWriter) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]behavior.ProductBehaviorSnapshot, error) {
	return make(map[uuid.UUID]behavior.ProductBehaviorSnapshot), nil
}

type deliveryFulfillmentSpec struct {
	Quantity int
}

type deliveryFulfillmentLeg struct {
	SellerID      uuid.UUID
	CategoryID    uuid.UUID
	ProductID     uuid.UUID
	VariantID     uuid.UUID
	FulfillmentID uuid.UUID
	OrderItemID   uuid.UUID
	ShipmentID    uuid.UUID
	Quantity      int
}

type deliveryEventFixture struct {
	Client   *postgres.Client
	AdminID  uuid.UUID
	UserID   uuid.UUID
	OrderID  uuid.UUID
	Legs     []deliveryFulfillmentLeg
}

func newDeliveryServerEventsEnv(t *testing.T, writerOverride behavior.EventWriter) (*postgres.Client, *Service, *behavior.Service) {
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
	behaviorSvc := behavior.NewService(behavior.NewRepository(client))

	var writer behavior.EventWriter = behaviorSvc
	if writerOverride != nil {
		writer = writerOverride
	}

	svc := NewService(repo, ordersRepo, client, nil, nil, writer)
	return client, svc, behaviorSvc
}

func seedDeliveryEventFixture(t *testing.T, client *postgres.Client, specs []deliveryFulfillmentSpec) *deliveryEventFixture {
	t.Helper()
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	fix := &deliveryEventFixture{
		Client:  client,
		AdminID: uuid.New(),
		UserID:  uuid.New(),
		OrderID: uuid.New(),
		Legs:    make([]deliveryFulfillmentLeg, len(specs)),
	}

	var sellerIDs, catIDs, prodIDs, varIDs, fulIDs, oiIDs, shipIDs []uuid.UUID
	for i, sp := range specs {
		leg := deliveryFulfillmentLeg{
			SellerID:      uuid.New(),
			CategoryID:    uuid.New(),
			ProductID:     uuid.New(),
			VariantID:     uuid.New(),
			FulfillmentID: uuid.New(),
			OrderItemID:   uuid.New(),
			ShipmentID:    uuid.New(),
			Quantity:      sp.Quantity,
		}
		fix.Legs[i] = leg
		sellerIDs = append(sellerIDs, leg.SellerID)
		catIDs = append(catIDs, leg.CategoryID)
		prodIDs = append(prodIDs, leg.ProductID)
		varIDs = append(varIDs, leg.VariantID)
		fulIDs = append(fulIDs, leg.FulfillmentID)
		oiIDs = append(oiIDs, leg.OrderItemID)
		shipIDs = append(shipIDs, leg.ShipmentID)
	}

	// Register scoped cleanup BEFORE any DB mutation and check every error
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, err := client.Pool.Exec(cleanCtx, "DELETE FROM behavioral_events WHERE order_id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM shipment_events WHERE shipment_id = ANY($1)", shipIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM shipments WHERE id = ANY($1)", shipIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_status_history WHERE order_id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_items WHERE id = ANY($1)", oiIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_fulfillments WHERE id = ANY($1)", fulIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM orders WHERE id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM product_variants WHERE id = ANY($1)", varIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM products WHERE id = ANY($1)", prodIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM categories WHERE id = ANY($1)", catIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM sellers WHERE id = ANY($1)", sellerIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{fix.AdminID, fix.UserID})
		require.NoError(t, err)
	})

	_, err := client.Pool.Exec(ctx, "INSERT INTO users (id, role, name, email, password_hash) VALUES ($1, 'admin', 'Test Admin', $2, 'hash')", fix.AdminID, "admin-"+fix.AdminID.String()+"@example.com")
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO users (id, role, name, email, password_hash) VALUES ($1, 'customer', 'Test User', $2, 'hash')", fix.UserID, "user-"+fix.UserID.String()+"@example.com")
	require.NoError(t, err)

	var totalCents int64
	for _, sp := range specs {
		totalCents += int64(sp.Quantity) * 1000
	}

	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'shipped', $3, 'Test User', '12345', 't@t.t', 'Addr')", fix.OrderID, fix.UserID, totalCents)
	require.NoError(t, err)

	for i, leg := range fix.Legs {
		_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Seller', $2, 's@t.t', 'active')", leg.SellerID, "seller-"+leg.SellerID.String())
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, slug, name, parent_id, is_active, sort_order) VALUES ($1, $2, 'Cat', NULL, true, 0)", leg.CategoryID, "cat-"+leg.CategoryID.String())
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, status, price_cents, category_id) VALUES ($1, $2, 'test product', $3, 'published', 1000, $4)", leg.ProductID, leg.SellerID, "prod-"+leg.ProductID.String(), leg.CategoryID)
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, size, color, sku, price_cents) VALUES ($1, $2, 'M', 'Black', $3, 1000)", leg.VariantID, leg.ProductID, "SKU-"+leg.VariantID.String())
		require.NoError(t, err)

		subCents := int64(leg.Quantity) * 1000
		_, err = client.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents) VALUES ($1, $2, $3, 'shipped', $4, 1000, $5)", leg.FulfillmentID, fix.OrderID, leg.SellerID, subCents, subCents*9/10)
		require.NoError(t, err)

		createdAt := time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		_, err = client.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'item', 'slug', 'M', 'Black', 'SKU', 1000, $7, $8, $9)",
			leg.OrderItemID, fix.OrderID, leg.FulfillmentID, leg.ProductID, leg.VariantID, leg.SellerID, leg.Quantity, subCents, createdAt)
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at) VALUES ($1, $2, $3, 'shipped', now() - interval '1 hour')", leg.ShipmentID, fix.OrderID, leg.FulfillmentID)
		require.NoError(t, err)
	}

	return fix
}

type persistedDeliveredEventRow struct {
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

func queryOrderDeliveredEvents(t *testing.T, client *postgres.Client, orderID uuid.UUID) []persistedDeliveredEventRow {
	t.Helper()
	rows, err := client.Pool.Query(context.Background(), `
		SELECT id, event_type, source, visitor_id, user_id, product_id, variant_id, category_id, order_id, return_id, order_item_id, quantity, occurred_at, metadata
		FROM behavioral_events
		WHERE order_id = $1 AND event_type = 'order_delivered'
		ORDER BY order_item_id ASC
	`, orderID)
	require.NoError(t, err)
	defer rows.Close()

	var res []persistedDeliveredEventRow
	for rows.Next() {
		var r persistedDeliveredEventRow
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

// A, D, E: Single fulfillment final delivery -> item-level order_delivered, exact fields, and exact canonical timestamp equality.
func TestOrderDeliveredServerEvents_SingleFulfillmentExactFieldsAndTimestamp(t *testing.T) {
	client, svc, _ := newDeliveryServerEventsEnv(t, nil)
	fix := seedDeliveryEventFixture(t, client, []deliveryFulfillmentSpec{{Quantity: 2}})
	ctx := context.Background()

	leg := fix.Legs[0]
	res, err := svc.DeliverShipment(ctx, fix.AdminID, leg.ShipmentID, DeliverShipmentRequest{})
	require.NoError(t, err)
	assert.Equal(t, "delivered", res.OrderStatus)

	var orderUpdatedAt time.Time
	err = client.Pool.QueryRow(ctx, "SELECT updated_at FROM orders WHERE id = $1", fix.OrderID).Scan(&orderUpdatedAt)
	require.NoError(t, err)

	events := queryOrderDeliveredEvents(t, client, fix.OrderID)
	require.Len(t, events, 1)

	ev := events[0]
	expectedID := behavior.DeterministicServerEventID("order_delivered", fix.OrderID, leg.OrderItemID)

	assert.Equal(t, expectedID, ev.ID)
	assert.Equal(t, "order_delivered", ev.EventType)
	assert.Equal(t, "server", ev.Source)
	assert.Nil(t, ev.VisitorID)
	require.NotNil(t, ev.UserID)
	assert.Equal(t, fix.UserID, *ev.UserID)
	require.NotNil(t, ev.OrderID)
	assert.Equal(t, fix.OrderID, *ev.OrderID)
	require.NotNil(t, ev.OrderItemID)
	assert.Equal(t, leg.OrderItemID, *ev.OrderItemID)
	require.NotNil(t, ev.ProductID)
	assert.Equal(t, leg.ProductID, *ev.ProductID)
	require.NotNil(t, ev.VariantID)
	assert.Equal(t, leg.VariantID, *ev.VariantID)
	require.NotNil(t, ev.CategoryID)
	assert.Equal(t, leg.CategoryID, *ev.CategoryID)
	assert.Nil(t, ev.ReturnID)
	require.NotNil(t, ev.Quantity)
	assert.Equal(t, 2, *ev.Quantity)
	assert.JSONEq(t, `{}`, string(ev.Metadata))
	assert.True(t, ev.OccurredAt.Equal(orderUpdatedAt), "behavioral_events.occurred_at (%v) must equal orders.updated_at (%v)", ev.OccurredAt, orderUpdatedAt)
}

// B & C: Two fulfillments -> deliver first emits ZERO events; deliver second/final emits one event per canonical order item.
func TestOrderDeliveredServerEvents_MultiFulfillmentPartialThenFinal(t *testing.T) {
	client, svc, _ := newDeliveryServerEventsEnv(t, nil)
	fix := seedDeliveryEventFixture(t, client, []deliveryFulfillmentSpec{{Quantity: 1}, {Quantity: 3}})
	ctx := context.Background()

	// B: Deliver first fulfillment -> parent remains 'shipped' -> ZERO order_delivered
	res1, err := svc.DeliverShipment(ctx, fix.AdminID, fix.Legs[0].ShipmentID, DeliverShipmentRequest{})
	require.NoError(t, err)
	assert.Equal(t, "shipped", res1.OrderStatus)

	eventsAfterFirst := queryOrderDeliveredEvents(t, client, fix.OrderID)
	assert.Empty(t, eventsAfterFirst, "partial delivery must emit zero order_delivered events")

	// C: Deliver second/final fulfillment -> parent becomes 'delivered' -> one event per canonical order item (2 total)
	res2, err := svc.DeliverShipment(ctx, fix.AdminID, fix.Legs[1].ShipmentID, DeliverShipmentRequest{})
	require.NoError(t, err)
	assert.Equal(t, "delivered", res2.OrderStatus)

	var orderUpdatedAt time.Time
	err = client.Pool.QueryRow(ctx, "SELECT updated_at FROM orders WHERE id = $1", fix.OrderID).Scan(&orderUpdatedAt)
	require.NoError(t, err)

	eventsAfterFinal := queryOrderDeliveredEvents(t, client, fix.OrderID)
	require.Len(t, eventsAfterFinal, 2)

	byItemID := make(map[uuid.UUID]persistedDeliveredEventRow)
	for _, ev := range eventsAfterFinal {
		require.NotNil(t, ev.OrderItemID)
		byItemID[*ev.OrderItemID] = ev
	}

	for _, leg := range fix.Legs {
		ev, ok := byItemID[leg.OrderItemID]
		require.True(t, ok, "missing order_delivered event for item %s", leg.OrderItemID)
		assert.Equal(t, behavior.DeterministicServerEventID("order_delivered", fix.OrderID, leg.OrderItemID), ev.ID)
		assert.Equal(t, leg.ProductID, *ev.ProductID)
		assert.Equal(t, leg.VariantID, *ev.VariantID)
		assert.Equal(t, leg.CategoryID, *ev.CategoryID)
		assert.Equal(t, leg.Quantity, *ev.Quantity)
		assert.True(t, ev.OccurredAt.Equal(orderUpdatedAt))
	}
}

// F: Retry / replay -> no duplicate behavioral row.
func TestOrderDeliveredServerEvents_RetryReplayIdempotency(t *testing.T) {
	client, svc, _ := newDeliveryServerEventsEnv(t, nil)
	fix := seedDeliveryEventFixture(t, client, []deliveryFulfillmentSpec{{Quantity: 1}})
	ctx := context.Background()

	leg := fix.Legs[0]
	_, err := svc.DeliverShipment(ctx, fix.AdminID, leg.ShipmentID, DeliverShipmentRequest{})
	require.NoError(t, err)

	// Replay 1: Second DeliverShipment call returns ErrShipmentAlreadyDelivered and does not duplicate
	_, err = svc.DeliverShipment(ctx, fix.AdminID, leg.ShipmentID, DeliverShipmentRequest{})
	require.ErrorIs(t, err, ErrShipmentAlreadyDelivered)

	// Replay 2: Direct replay of emitOrderDeliveredEventsTx with same canonical order & item IDs inside a transaction
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		order, err := svc.ordersRepo.GetOrderForUpdateTx(ctx, tx, fix.OrderID)
		if err != nil {
			return err
		}
		return svc.emitOrderDeliveredEventsTx(ctx, tx, order, order.UpdatedAt)
	})
	require.NoError(t, err)

	events := queryOrderDeliveredEvents(t, client, fix.OrderID)
	require.Len(t, events, 1)
}

// G: Concurrent final-delivery race -> at most one canonical event per item and correct parent truth.
func TestOrderDeliveredServerEvents_ConcurrentFinalDeliveryRace(t *testing.T) {
	client, svc, _ := newDeliveryServerEventsEnv(t, nil)
	fix := seedDeliveryEventFixture(t, client, []deliveryFulfillmentSpec{{Quantity: 1}, {Quantity: 2}})
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, errs[idx] = svc.DeliverShipment(ctx, fix.AdminID, fix.Legs[idx].ShipmentID, DeliverShipmentRequest{})
		}(i)
	}
	close(start)
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	var ordStatus string
	require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", fix.OrderID).Scan(&ordStatus))
	assert.Equal(t, "delivered", ordStatus)

	events := queryOrderDeliveredEvents(t, client, fix.OrderID)
	require.Len(t, events, 2, "must have exactly 1 canonical event per item")
}

// H & 4: Failing behavior writer and missing product in snapshot roll back shipment, fulfillment, and parent order delivery transition.
func TestOrderDeliveredServerEvents_WriterFailureAndMissingProductRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("forced_behavior_writer_failure_rolls_back_delivery", func(t *testing.T) {
		client, _, realBehaviorSvc := newDeliveryServerEventsEnv(t, nil)
		failWriter := &failingDeliveryBehaviorWriter{
			real: realBehaviorSvc,
			err:  errors.New("simulated delivery behavioral writer failure"),
		}
		_, failingSvc, _ := newDeliveryServerEventsEnv(t, failWriter)
		fix := seedDeliveryEventFixture(t, client, []deliveryFulfillmentSpec{{Quantity: 1}})
		leg := fix.Legs[0]

		_, err := failingSvc.DeliverShipment(ctx, fix.AdminID, leg.ShipmentID, DeliverShipmentRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated delivery behavioral writer failure")

		var shipStatus, fulStatus, ordStatus string
		var deliveredAt *time.Time
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status, delivered_at FROM shipments WHERE id = $1", leg.ShipmentID).Scan(&shipStatus, &deliveredAt))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM order_fulfillments WHERE id = $1", leg.FulfillmentID).Scan(&fulStatus))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", fix.OrderID).Scan(&ordStatus))

		assert.Equal(t, "shipped", shipStatus)
		assert.Nil(t, deliveredAt)
		assert.Equal(t, "shipped", fulStatus)
		assert.Equal(t, "shipped", ordStatus)

		events := queryOrderDeliveredEvents(t, client, fix.OrderID)
		assert.Empty(t, events)
	})

	t.Run("missing_product_in_snapshot_rolls_back_delivery", func(t *testing.T) {
		client, _, realBehaviorSvc := newDeliveryServerEventsEnv(t, nil)
		emptyWriter := &emptyDeliverySnapshotWriter{real: realBehaviorSvc}
		_, svcWithEmptySnap, _ := newDeliveryServerEventsEnv(t, emptyWriter)
		fix := seedDeliveryEventFixture(t, client, []deliveryFulfillmentSpec{{Quantity: 1}})
		leg := fix.Legs[0]

		_, err := svcWithEmptySnap.DeliverShipment(ctx, fix.AdminID, leg.ShipmentID, DeliverShipmentRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "canonical product")

		var shipStatus, fulStatus, ordStatus string
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM shipments WHERE id = $1", leg.ShipmentID).Scan(&shipStatus))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM order_fulfillments WHERE id = $1", leg.FulfillmentID).Scan(&fulStatus))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", fix.OrderID).Scan(&ordStatus))

		assert.Equal(t, "shipped", shipStatus)
		assert.Equal(t, "shipped", fulStatus)
		assert.Equal(t, "shipped", ordStatus)

		events := queryOrderDeliveredEvents(t, client, fix.OrderID)
		assert.Empty(t, events)
	})
}
