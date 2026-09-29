package behavior

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicServerEventID(t *testing.T) {
	orderID := uuid.New()
	orderItemID := uuid.New()
	returnID := uuid.New()
	returnItemID := uuid.New()

	// 1. order_paid: type + orderID + orderItemID
	paid1 := DeterministicServerEventID("order_paid", orderID, orderItemID)
	paid2 := DeterministicServerEventID("order_paid", orderID, orderItemID)
	assert.Equal(t, paid1, paid2, "order_paid deterministic ID must be stable for same (orderID, orderItemID)")
	assert.NotEqual(t, paid1, DeterministicServerEventID("order_paid", uuid.New(), orderItemID))
	assert.NotEqual(t, paid1, DeterministicServerEventID("order_paid", orderID, uuid.New()))

	// 2. order_delivered: type + orderID + orderItemID
	deliv1 := DeterministicServerEventID("order_delivered", orderID, orderItemID)
	deliv2 := DeterministicServerEventID("order_delivered", orderID, orderItemID)
	assert.Equal(t, deliv1, deliv2, "order_delivered deterministic ID must be stable for same (orderID, orderItemID)")
	assert.NotEqual(t, paid1, deliv1, "order_paid and order_delivered for same item must have distinct IDs")

	// 3. return_requested: type + returnID + returnItemID
	ret1 := DeterministicServerEventID("return_requested", returnID, returnItemID)
	ret2 := DeterministicServerEventID("return_requested", returnID, returnItemID)
	assert.Equal(t, ret1, ret2, "return_requested deterministic ID must be stable for same (returnID, returnItemID)")
	assert.NotEqual(t, ret1, DeterministicServerEventID("return_requested", uuid.New(), returnItemID))
	assert.NotEqual(t, ret1, DeterministicServerEventID("return_requested", returnID, uuid.New()))
}

func TestResolveCategoriesTx_DistinguishesNullCategoryFromMissingProduct(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	client, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	testutil.AssertTestDatabase(t, client.Pool)

	repo := NewRepository(client)
	svc := NewService(repo)

	sellerID := uuid.New()
	catID := uuid.New()
	prodWithCatID := uuid.New()
	prodNullCatID := uuid.New()
	missingProdID := uuid.New()

	t.Cleanup(func() {
		_, err := client.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", []uuid.UUID{prodWithCatID, prodNullCatID})
		require.NoError(t, err)
		_, err = client.Pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", catID)
		require.NoError(t, err)
		_, err = client.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		require.NoError(t, err)
	})

	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Seller', $2, 's@t.t', 'active')", sellerID, "seller-"+sellerID.String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, slug, name, parent_id, is_active, sort_order) VALUES ($1, $2, 'Cat', NULL, true, 0)", catID, "cat-"+catID.String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Prod Cat', $4, 'published', 1000)", prodWithCatID, sellerID, catID, "prod-cat-"+prodWithCatID.String())
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, NULL, 'Prod Null Cat', $3, 'published', 1000)", prodNullCatID, sellerID, "prod-null-"+prodNullCatID.String())
	require.NoError(t, err)

	// Case A: Existing products (one with category, one with NULL category) succeed
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		snaps, err := svc.ResolveCategoriesTx(ctx, tx, []uuid.UUID{prodWithCatID, prodNullCatID})
		require.NoError(t, err)

		s1, ok1 := snaps[prodWithCatID]
		require.True(t, ok1)
		require.NotNil(t, s1.CategoryID)
		assert.Equal(t, catID, *s1.CategoryID)

		s2, ok2 := snaps[prodNullCatID]
		require.True(t, ok2)
		assert.Nil(t, s2.CategoryID)
		return nil
	})
	require.NoError(t, err)

	// Case B: Missing product returns invariant error
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ResolveCategoriesTx(ctx, tx, []uuid.UUID{prodWithCatID, missingProdID})
		return err
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), missingProdID.String())
}

func TestInsertServerEventsTx_DeterministicIdempotency(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	client, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	testutil.AssertTestDatabase(t, client.Pool)

	svc := NewService(NewRepository(client))

	orderID := uuid.New()
	orderItemID := uuid.New()
	eventID := DeterministicServerEventID("order_paid", orderID, orderItemID)

	t.Cleanup(func() {
		_, err := client.Pool.Exec(ctx, "DELETE FROM behavioral_events WHERE id = $1", eventID)
		require.NoError(t, err)
	})

	qty := 2
	occurredAt := time.Now().UTC().Truncate(time.Microsecond)
	ev := BehavioralEvent{
		ID:          eventID,
		EventType:   "order_paid",
		Source:      SourceServer,
		VisitorID:   nil,
		OrderID:     &orderID,
		OrderItemID: &orderItemID,
		Quantity:    &qty,
		OccurredAt:  occurredAt,
		ReceivedAt:  occurredAt,
		Metadata:    []byte(`{}`),
	}

	// Insert twice with the same deterministic ID
	require.NoError(t, client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.InsertServerEventsTx(ctx, tx, []BehavioralEvent{ev})
	}))
	require.NoError(t, client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.InsertServerEventsTx(ctx, tx, []BehavioralEvent{ev})
	}))

	var count int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM behavioral_events WHERE id = $1", eventID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
