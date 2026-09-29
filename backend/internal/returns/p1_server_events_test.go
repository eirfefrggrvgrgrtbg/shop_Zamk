package returns

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
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

type failingReturnBehaviorWriter struct {
	real *behavior.Service
	err  error
}

func (f *failingReturnBehaviorWriter) InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []behavior.BehavioralEvent) error {
	return f.err
}

func (f *failingReturnBehaviorWriter) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]behavior.ProductBehaviorSnapshot, error) {
	return f.real.ResolveCategoriesTx(ctx, tx, productIDs)
}

type emptyReturnSnapshotWriter struct {
	real *behavior.Service
}

func (e *emptyReturnSnapshotWriter) InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []behavior.BehavioralEvent) error {
	return e.real.InsertServerEventsTx(ctx, tx, events)
}

func (e *emptyReturnSnapshotWriter) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]behavior.ProductBehaviorSnapshot, error) {
	return make(map[uuid.UUID]behavior.ProductBehaviorSnapshot), nil
}

type returnTestItemSpec struct {
	OrderQty int
}

type returnTestItem struct {
	CategoryID  uuid.UUID
	ProductID   uuid.UUID
	VariantID   uuid.UUID
	OrderItemID uuid.UUID
	OrderQty    int
}

type returnEventFixture struct {
	Client        *postgres.Client
	UserID        uuid.UUID
	SellerID      uuid.UUID
	OrderID       uuid.UUID
	FulfillmentID uuid.UUID
	ShipmentID    uuid.UUID
	Items         []returnTestItem
}

func newReturnServerEventsEnv(t *testing.T, writerOverride behavior.EventWriter) (*postgres.Client, *Service, *behavior.Service) {
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
	notifSvc := notifications.NewService(nil, nil, nil)
	invSvc := inventory.NewService(nil, nil, client)

	var writer behavior.EventWriter = behaviorSvc
	if writerOverride != nil {
		writer = writerOverride
	}

	svc := NewService(repo, ordersRepo, invSvc, client, nil, nil, 14, notifSvc, nil, nil, writer)
	return client, svc, behaviorSvc
}

func seedReturnEventFixture(t *testing.T, client *postgres.Client, specs []returnTestItemSpec) *returnEventFixture {
	t.Helper()
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	fix := &returnEventFixture{
		Client:        client,
		UserID:        uuid.New(),
		SellerID:      uuid.New(),
		OrderID:       uuid.New(),
		FulfillmentID: uuid.New(),
		ShipmentID:    uuid.New(),
		Items:         make([]returnTestItem, len(specs)),
	}

	var catIDs, prodIDs, varIDs, oiIDs []uuid.UUID
	for i, sp := range specs {
		it := returnTestItem{
			CategoryID:  uuid.New(),
			ProductID:   uuid.New(),
			VariantID:   uuid.New(),
			OrderItemID: uuid.New(),
			OrderQty:    sp.OrderQty,
		}
		fix.Items[i] = it
		catIDs = append(catIDs, it.CategoryID)
		prodIDs = append(prodIDs, it.ProductID)
		varIDs = append(varIDs, it.VariantID)
		oiIDs = append(oiIDs, it.OrderItemID)
	}

	// Register scoped cleanup BEFORE any DB mutation and check every error
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, err := client.Pool.Exec(cleanCtx, "DELETE FROM behavioral_events WHERE order_id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM notifications WHERE entity_id IN (SELECT id FROM returns WHERE order_id = $1)", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM return_responsibility_allocation_history WHERE return_item_id IN (SELECT id FROM return_items WHERE return_id IN (SELECT id FROM returns WHERE order_id = $1))", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM return_responsibility_allocations WHERE return_item_id IN (SELECT id FROM return_items WHERE return_id IN (SELECT id FROM returns WHERE order_id = $1))", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM return_items WHERE return_id IN (SELECT id FROM returns WHERE order_id = $1)", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM returns WHERE order_id = $1", fix.OrderID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM shipments WHERE id = $1", fix.ShipmentID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_items WHERE id = ANY($1)", oiIDs)
		require.NoError(t, err)
		_, err = client.Pool.Exec(cleanCtx, "DELETE FROM order_fulfillments WHERE id = $1", fix.FulfillmentID)
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

	_, err := client.Pool.Exec(ctx, "INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'Test User', 'hash', 'customer')", fix.UserID, "user-"+fix.UserID.String()+"@example.com")
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Seller', $2, 'seller@t.t', 'active')", fix.SellerID, "seller-"+fix.SellerID.String())
	require.NoError(t, err)

	var totalCents int64
	for _, sp := range specs {
		totalCents += int64(sp.OrderQty) * 1000
	}

	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', $3, 'Test User', '12345', 't@t.t', 'Addr')", fix.OrderID, fix.UserID, totalCents)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, seller_amount_cents, commission_bps) VALUES ($1, $2, $3, 'delivered', $4, $5, 900)", fix.FulfillmentID, fix.OrderID, fix.SellerID, totalCents, totalCents*9/10)
	require.NoError(t, err)

	for i, it := range fix.Items {
		_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, slug, name, parent_id, is_active, sort_order) VALUES ($1, $2, 'Cat', NULL, true, 0)", it.CategoryID, "cat-"+it.CategoryID.String())
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, slug, title, status, price_cents) VALUES ($1, $2, $3, $4, 'Prod', 'approved', 1000)", it.ProductID, fix.SellerID, it.CategoryID, "prod-"+it.ProductID.String())
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, $3, 1000)", it.VariantID, it.ProductID, "SKU-"+it.VariantID.String())
		require.NoError(t, err)

		subCents := int64(it.OrderQty) * 1000
		createdAt := time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		_, err = client.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'item', 'slug', 'M', 'Black', 'SKU', 1000, $7, $8, $9)",
			it.OrderItemID, fix.OrderID, fix.FulfillmentID, it.ProductID, it.VariantID, fix.SellerID, it.OrderQty, subCents, createdAt)
		require.NoError(t, err)
	}

	_, err = client.Pool.Exec(ctx, "INSERT INTO shipments (id, order_id, fulfillment_id, status, delivered_at) VALUES ($1, $2, $3, 'delivered', now())", fix.ShipmentID, fix.OrderID, fix.FulfillmentID)
	require.NoError(t, err)

	return fix
}

type persistedReturnEventRow struct {
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

func queryReturnRequestedEvents(t *testing.T, client *postgres.Client, orderID uuid.UUID) []persistedReturnEventRow {
	t.Helper()
	rows, err := client.Pool.Query(context.Background(), `
		SELECT id, event_type, source, visitor_id, user_id, product_id, variant_id, category_id, order_id, return_id, order_item_id, quantity, occurred_at, metadata
		FROM behavioral_events
		WHERE order_id = $1 AND event_type = 'return_requested'
		ORDER BY order_item_id ASC
	`, orderID)
	require.NoError(t, err)
	defer rows.Close()

	var res []persistedReturnEventRow
	for rows.Next() {
		var r persistedReturnEventRow
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

// A, C, D: Order quantity 3, return quantity 1 -> one return_requested with quantity = 1, exact fields, and occurred_at == returns.created_at.
func TestReturnRequestedServerEvents_PartialQuantityExactFieldsAndTimestamp(t *testing.T) {
	client, svc, _ := newReturnServerEventsEnv(t, nil)
	fix := seedReturnEventFixture(t, client, []returnTestItemSpec{{OrderQty: 3}})
	ctx := context.Background()

	it := fix.Items[0]
	comment := "slightly large"
	res, err := svc.CreateReturn(ctx, fix.UserID, fix.OrderID, CreateReturnRequest{
		Reason:  "didnt_fit",
		Comment: &comment,
		Items: []CreateReturnItemRequest{
			{OrderItemID: it.OrderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Len(t, res[0].Items, 1)

	returnID := res[0].Return.ID
	returnItemID := res[0].Items[0].ID

	var returnCreatedAt time.Time
	err = client.Pool.QueryRow(ctx, "SELECT created_at FROM returns WHERE id = $1", returnID).Scan(&returnCreatedAt)
	require.NoError(t, err)

	events := queryReturnRequestedEvents(t, client, fix.OrderID)
	require.Len(t, events, 1)

	ev := events[0]
	expectedID := behavior.DeterministicServerEventID("return_requested", returnID, returnItemID)

	assert.Equal(t, expectedID, ev.ID)
	assert.Equal(t, "return_requested", ev.EventType)
	assert.Equal(t, "server", ev.Source)
	assert.Nil(t, ev.VisitorID)
	require.NotNil(t, ev.UserID)
	assert.Equal(t, fix.UserID, *ev.UserID)
	require.NotNil(t, ev.OrderID)
	assert.Equal(t, fix.OrderID, *ev.OrderID)
	require.NotNil(t, ev.ReturnID)
	assert.Equal(t, returnID, *ev.ReturnID)
	require.NotNil(t, ev.OrderItemID)
	assert.Equal(t, it.OrderItemID, *ev.OrderItemID)
	require.NotNil(t, ev.ProductID)
	assert.Equal(t, it.ProductID, *ev.ProductID)
	require.NotNil(t, ev.VariantID)
	assert.Equal(t, it.VariantID, *ev.VariantID)
	require.NotNil(t, ev.CategoryID)
	assert.Equal(t, it.CategoryID, *ev.CategoryID)
	require.NotNil(t, ev.Quantity)
	assert.Equal(t, 1, *ev.Quantity)
	assert.JSONEq(t, `{}`, string(ev.Metadata))
	assert.True(t, ev.OccurredAt.Equal(returnCreatedAt), "behavioral_events.occurred_at (%v) must equal returns.created_at (%v)", ev.OccurredAt, returnCreatedAt)
}

// B: Multiple returned order items -> event per returned item.
func TestReturnRequestedServerEvents_MultipleReturnedItems(t *testing.T) {
	client, svc, _ := newReturnServerEventsEnv(t, nil)
	fix := seedReturnEventFixture(t, client, []returnTestItemSpec{{OrderQty: 3}, {OrderQty: 2}})
	ctx := context.Background()

	comment := "returning two items"
	res, err := svc.CreateReturn(ctx, fix.UserID, fix.OrderID, CreateReturnRequest{
		Reason:  "didnt_fit",
		Comment: &comment,
		Items: []CreateReturnItemRequest{
			{OrderItemID: fix.Items[0].OrderItemID, Quantity: 1},
			{OrderItemID: fix.Items[1].OrderItemID, Quantity: 2},
		},
	})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Len(t, res[0].Items, 2)

	returnID := res[0].Return.ID
	var returnCreatedAt time.Time
	err = client.Pool.QueryRow(ctx, "SELECT created_at FROM returns WHERE id = $1", returnID).Scan(&returnCreatedAt)
	require.NoError(t, err)

	returnItemByOrderItem := make(map[uuid.UUID]CustomerReturnItemDetail)
	for _, ri := range res[0].Items {
		returnItemByOrderItem[ri.OrderItemID] = ri
	}

	events := queryReturnRequestedEvents(t, client, fix.OrderID)
	require.Len(t, events, 2)

	byOrderItem := make(map[uuid.UUID]persistedReturnEventRow)
	for _, ev := range events {
		require.NotNil(t, ev.OrderItemID)
		byOrderItem[*ev.OrderItemID] = ev
	}

	expectedQtys := []int{1, 2}
	for i, it := range fix.Items {
		ev, ok := byOrderItem[it.OrderItemID]
		require.True(t, ok, "missing return_requested event for order_item_id %s", it.OrderItemID)
		ri := returnItemByOrderItem[it.OrderItemID]
		assert.Equal(t, behavior.DeterministicServerEventID("return_requested", returnID, ri.ID), ev.ID)
		assert.Equal(t, returnID, *ev.ReturnID)
		assert.Equal(t, it.ProductID, *ev.ProductID)
		assert.Equal(t, it.VariantID, *ev.VariantID)
		assert.Equal(t, it.CategoryID, *ev.CategoryID)
		assert.Equal(t, expectedQtys[i], *ev.Quantity)
		assert.True(t, ev.OccurredAt.Equal(returnCreatedAt))
	}
}

// E: Validation failure -> zero return_requested.
func TestReturnRequestedServerEvents_ValidationFailureEmitsZeroEvents(t *testing.T) {
	client, svc, _ := newReturnServerEventsEnv(t, nil)
	fix := seedReturnEventFixture(t, client, []returnTestItemSpec{{OrderQty: 2}})
	ctx := context.Background()

	comment := "invalid qty"
	_, err := svc.CreateReturn(ctx, fix.UserID, fix.OrderID, CreateReturnRequest{
		Reason:  "didnt_fit",
		Comment: &comment,
		Items: []CreateReturnItemRequest{
			{OrderItemID: fix.Items[0].OrderItemID, Quantity: 99},
		},
	})
	require.Error(t, err)

	var returnCount int
	require.NoError(t, client.Pool.QueryRow(ctx, "SELECT count(*) FROM returns WHERE order_id = $1", fix.OrderID).Scan(&returnCount))
	assert.Equal(t, 0, returnCount)

	events := queryReturnRequestedEvents(t, client, fix.OrderID)
	assert.Empty(t, events)
}

// F: Same canonical retry/idempotency path -> no duplicate.
func TestReturnRequestedServerEvents_RetryReplayIdempotency(t *testing.T) {
	client, svc, _ := newReturnServerEventsEnv(t, nil)
	fix := seedReturnEventFixture(t, client, []returnTestItemSpec{{OrderQty: 1}})
	ctx := context.Background()

	comment := "replay test"
	res, err := svc.CreateReturn(ctx, fix.UserID, fix.OrderID, CreateReturnRequest{
		Reason:  "didnt_fit",
		Comment: &comment,
		Items: []CreateReturnItemRequest{
			{OrderItemID: fix.Items[0].OrderItemID, Quantity: 1},
		},
	})
	require.NoError(t, err)
	require.Len(t, res, 1)

	// Replay 1: Second CreateReturn for already-returned item is rejected by domain validation
	_, err = svc.CreateReturn(ctx, fix.UserID, fix.OrderID, CreateReturnRequest{
		Reason:  "didnt_fit",
		Comment: &comment,
		Items: []CreateReturnItemRequest{
			{OrderItemID: fix.Items[0].OrderItemID, Quantity: 1},
		},
	})
	require.Error(t, err)

	// Replay 2: Direct replay of emitReturnRequestedEventsTx with same canonical Return & ReturnItem IDs inside a transaction
	var canonicalItems []ReturnItem
	for _, d := range res[0].Items {
		canonicalItems = append(canonicalItems, ReturnItem{
			ID:          d.ID,
			ReturnID:    res[0].Return.ID,
			OrderItemID: d.OrderItemID,
			Quantity:    d.Quantity,
		})
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.emitReturnRequestedEventsTx(ctx, tx, &res[0].Return, canonicalItems)
	})
	require.NoError(t, err)

	events := queryReturnRequestedEvents(t, client, fix.OrderID)
	require.Len(t, events, 1)
}

// G, H, 4: Failing behavior writer, missing canonical order-item mapping, and missing product snapshot roll back completely.
func TestReturnRequestedServerEvents_RollbackOnFailureAndMissingOrderItemAndMissingProduct(t *testing.T) {
	ctx := context.Background()

	t.Run("forced_behavior_writer_failure_rolls_back_return_and_return_items", func(t *testing.T) {
		client, _, realBehaviorSvc := newReturnServerEventsEnv(t, nil)
		failWriter := &failingReturnBehaviorWriter{
			real: realBehaviorSvc,
			err:  errors.New("simulated return behavioral writer failure"),
		}
		_, failingSvc, _ := newReturnServerEventsEnv(t, failWriter)
		fix := seedReturnEventFixture(t, client, []returnTestItemSpec{{OrderQty: 2}})

		comment := "rollback test"
		_, err := failingSvc.CreateReturn(ctx, fix.UserID, fix.OrderID, CreateReturnRequest{
			Reason:  "didnt_fit",
			Comment: &comment,
			Items: []CreateReturnItemRequest{
				{OrderItemID: fix.Items[0].OrderItemID, Quantity: 1},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated return behavioral writer failure")

		var retCount, retItemCount int
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT count(*) FROM returns WHERE order_id = $1", fix.OrderID).Scan(&retCount))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT count(*) FROM return_items WHERE order_item_id = $1", fix.Items[0].OrderItemID).Scan(&retItemCount))
		assert.Equal(t, 0, retCount)
		assert.Equal(t, 0, retItemCount)

		events := queryReturnRequestedEvents(t, client, fix.OrderID)
		assert.Empty(t, events)
	})

	t.Run("missing_canonical_order_item_mapping_returns_invariant_error_and_rolls_back", func(t *testing.T) {
		client, svc, _ := newReturnServerEventsEnv(t, nil)
		fix := seedReturnEventFixture(t, client, []returnTestItemSpec{{OrderQty: 1}})

		missingOrderItemID := uuid.New()
		retID := uuid.New()
		err := client.RunInTx(ctx, func(tx pgx.Tx) error {
			ret := &Return{
				ID:            retID,
				OrderID:       fix.OrderID,
				FulfillmentID: fix.FulfillmentID,
				UserID:        fix.UserID,
				Status:        "requested",
				Reason:        "didnt_fit",
			}
			if err := svc.repo.CreateReturnTx(ctx, tx, ret, nil); err != nil {
				return err
			}
			badItems := []ReturnItem{
				{
					ID:          uuid.New(),
					ReturnID:    ret.ID,
					OrderItemID: missingOrderItemID,
					Quantity:    1,
				},
			}
			return svc.emitReturnRequestedEventsTx(ctx, tx, ret, badItems)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing order item")

		var retCount int
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT count(*) FROM returns WHERE order_id = $1", fix.OrderID).Scan(&retCount))
		assert.Equal(t, 0, retCount)

		events := queryReturnRequestedEvents(t, client, fix.OrderID)
		assert.Empty(t, events)
	})

	t.Run("missing_product_in_snapshot_rolls_back_return_and_return_items", func(t *testing.T) {
		client, _, realBehaviorSvc := newReturnServerEventsEnv(t, nil)
		emptyWriter := &emptyReturnSnapshotWriter{real: realBehaviorSvc}
		_, svcWithEmptySnap, _ := newReturnServerEventsEnv(t, emptyWriter)
		fix := seedReturnEventFixture(t, client, []returnTestItemSpec{{OrderQty: 2}})

		comment := "missing product test"
		_, err := svcWithEmptySnap.CreateReturn(ctx, fix.UserID, fix.OrderID, CreateReturnRequest{
			Reason:  "didnt_fit",
			Comment: &comment,
			Items: []CreateReturnItemRequest{
				{OrderItemID: fix.Items[0].OrderItemID, Quantity: 1},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "canonical product")

		var retCount, retItemCount int
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT count(*) FROM returns WHERE order_id = $1", fix.OrderID).Scan(&retCount))
		require.NoError(t, client.Pool.QueryRow(ctx, "SELECT count(*) FROM return_items WHERE order_item_id = $1", fix.Items[0].OrderItemID).Scan(&retItemCount))
		assert.Equal(t, 0, retCount)
		assert.Equal(t, 0, retItemCount)

		events := queryReturnRequestedEvents(t, client, fix.OrderID)
		assert.Empty(t, events)
	})
}
