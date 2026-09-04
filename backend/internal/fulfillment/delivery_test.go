package fulfillment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
)

func TestDelivery_Success(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 2)

	var prodID, variantID uuid.UUID
	err := f.db.QueryRow(ctx, `SELECT product_id, product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&prodID, &variantID)
	require.NoError(t, err)

	f.createInventoryItem(t, ctx, prodID, variantID, 10, 5)

	// Create allocation and unit in shipped state
	f.createAllocation(t, ctx, itemID, true)
	f.createAllocation(t, ctx, itemID, true)
	_, err = f.db.Exec(ctx, `
		UPDATE inventory_units
		SET status = 'shipped'
		WHERE id IN (SELECT inventory_unit_id FROM order_item_allocations WHERE order_item_id = $1)
	`, itemID)
	require.NoError(t, err)

	// Create converted reservation
	resID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, quantity, status, order_id, created_at, expires_at)
		SELECT $1, id, product_id, product_variant_id, 2, 'converted', $2, now(), now() + interval '1 hour'
		FROM inventory_items WHERE product_variant_id = $3
	`, resID, orderID, variantID)
	require.NoError(t, err)

	// Create shipped shipment
	shippedAt := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Microsecond)
	shipmentID := uuid.New()
	carrier := "CDEK"
	trkNum := "TRK-98765"
	trkUrl := "https://cdek.ru/trk/98765"
	_, err = f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', $4, $5, $6, $7, now(), now())
	`, shipmentID, orderID, fulfillmentID, carrier, trkNum, trkUrl, shippedAt)
	require.NoError(t, err)

	// Capture BEFORE delivery baseline
	var preTotalStock, preReservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&preTotalStock, &preReservedStock)
	require.NoError(t, err)
	assert.Equal(t, 10, preTotalStock)
	assert.Equal(t, 5, preReservedStock)

	type allocBaseline struct {
		id         uuid.UUID
		pickedAt   time.Time
		releasedAt *time.Time
	}
	var preAllocs []allocBaseline
	rowsPre, err := f.db.Query(ctx, `SELECT id, picked_at, released_at FROM order_item_allocations WHERE order_item_id = $1 ORDER BY id ASC`, itemID)
	require.NoError(t, err)
	for rowsPre.Next() {
		var ab allocBaseline
		require.NoError(t, rowsPre.Scan(&ab.id, &ab.pickedAt, &ab.releasedAt))
		preAllocs = append(preAllocs, ab)
	}
	rowsPre.Close()
	require.Len(t, preAllocs, 2)

	var preResStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, resID).Scan(&preResStatus)
	require.NoError(t, err)
	assert.Equal(t, "converted", preResStatus)

	comment := "Вручено лично в руки"
	res, err := f.svc.DeliverShipment(ctx, f.adminID, shipmentID, fulfillment.DeliverShipmentRequest{
		Comment: &comment,
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, shipmentID, res.ShipmentID)
	assert.Equal(t, fulfillmentID, res.FulfillmentID)
	assert.Equal(t, orderID, res.OrderID)
	assert.Equal(t, "delivered", res.ShipmentStatus)
	assert.Equal(t, "delivered", res.FulfillmentStatus)
	assert.Equal(t, "delivered", res.OrderStatus)
	assert.WithinDuration(t, time.Now(), res.DeliveredAt, 5*time.Second)

	// 1. Verify shipment state in DB
	var dbStatus, dbCarrier, dbTrkNum, dbTrkUrl string
	var dbShippedAt, dbDeliveredAt time.Time
	err = f.db.QueryRow(ctx, `
		SELECT status, carrier, tracking_number, tracking_url, shipped_at, delivered_at
		FROM shipments WHERE id = $1
	`, shipmentID).Scan(&dbStatus, &dbCarrier, &dbTrkNum, &dbTrkUrl, &dbShippedAt, &dbDeliveredAt)
	require.NoError(t, err)
	assert.Equal(t, "delivered", dbStatus)
	assert.Equal(t, carrier, dbCarrier)
	assert.Equal(t, trkNum, dbTrkNum)
	assert.Equal(t, trkUrl, dbTrkUrl)
	assert.WithinDuration(t, shippedAt, dbShippedAt, time.Second, "shipped_at must remain unchanged")
	assert.WithinDuration(t, time.Now(), dbDeliveredAt, 5*time.Second)

	// 2. Verify fulfillment in DB
	var dbFStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&dbFStatus)
	require.NoError(t, err)
	assert.Equal(t, "delivered", dbFStatus)

	// 3. Verify order in DB
	var dbOStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&dbOStatus)
	require.NoError(t, err)
	assert.Equal(t, "delivered", dbOStatus)

	// 4. Verify shipment_events: exactly 1 record
	var eventCount int
	var fromStatus, toStatus, evComment string
	var actorID uuid.UUID
	err = f.db.QueryRow(ctx, `
		SELECT count(*), max(from_status), max(to_status), max(comment), max(actor_user_id::text)::uuid
		FROM shipment_events WHERE shipment_id = $1
	`, shipmentID).Scan(&eventCount, &fromStatus, &toStatus, &evComment, &actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount)
	assert.Equal(t, "shipped", fromStatus)
	assert.Equal(t, "delivered", toStatus)
	assert.Equal(t, comment, evComment)
	assert.Equal(t, f.adminID, actorID)

	// 5. Zero Inventory Mutation Invariants
	var postTotalStock, postReservedStock int
	err = f.db.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantID).Scan(&postTotalStock, &postReservedStock)
	require.NoError(t, err)
	assert.Equal(t, preTotalStock, postTotalStock, "delivery must not mutate total_stock")
	assert.Equal(t, preReservedStock, postReservedStock, "delivery must not mutate reserved_stock")

	// 6. Inventory units remain 'shipped'
	rows, err := f.db.Query(ctx, `
		SELECT u.status
		FROM inventory_units u
		JOIN order_item_allocations a ON a.inventory_unit_id = u.id
		WHERE a.order_item_id = $1
	`, itemID)
	require.NoError(t, err)
	defer rows.Close()
	var unitCount int
	for rows.Next() {
		var uStatus string
		require.NoError(t, rows.Scan(&uStatus))
		assert.Equal(t, "shipped", uStatus)
		unitCount++
	}
	assert.Equal(t, 2, unitCount)

	// 7. Reservations remain 'converted'
	var postResStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, resID).Scan(&postResStatus)
	require.NoError(t, err)
	assert.Equal(t, preResStatus, postResStatus)

	// 8. Allocation lineage remains intact (picked_at unchanged, released_at IS NULL)
	rowsPost, err := f.db.Query(ctx, `SELECT id, picked_at, released_at FROM order_item_allocations WHERE order_item_id = $1 ORDER BY id ASC`, itemID)
	require.NoError(t, err)
	var idx int
	for rowsPost.Next() {
		var id uuid.UUID
		var pickedAt time.Time
		var releasedAt *time.Time
		require.NoError(t, rowsPost.Scan(&id, &pickedAt, &releasedAt))
		assert.Equal(t, preAllocs[idx].id, id)
		assert.WithinDuration(t, preAllocs[idx].pickedAt, pickedAt, time.Millisecond, "picked_at must be unchanged")
		assert.Nil(t, releasedAt, "released_at must remain nil")
		idx++
	}
	rowsPost.Close()
	assert.Equal(t, 2, idx)
}

func TestDelivery_MultiFulfillment_ParentStatusMatrix(t *testing.T) {
	ctx := context.Background()

	createSecondSeller := func(t *testing.T, f *pickingFixture) uuid.UUID {
		seller2ID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Seller 2', $2, $3, 'active', now(), now())
		`, seller2ID, uuid.New().String(), uuid.New().String()+"@ex.com")
		require.NoError(t, err)
		return seller2ID
	}

	t.Run("delivered + shipped sibling -> parent shipped", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()
		s2 := createSecondSeller(t, f)

		orderID, f1 := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
		f2 := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'shipped', 1000, 900, 900)
		`, f2, orderID, s2)
		require.NoError(t, err)

		s1 := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'shipped', now(), now(), now())
		`, s1, orderID, f1)
		require.NoError(t, err)

		res, err := f.svc.DeliverShipment(ctx, f.adminID, s1, fulfillment.DeliverShipmentRequest{})
		require.NoError(t, err)
		assert.Equal(t, "shipped", res.OrderStatus, "parent order must remain shipped while sibling is shipped")

		var dbOStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&dbOStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", dbOStatus)
	})

	t.Run("delivered + packed sibling -> parent packed", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()
		s2 := createSecondSeller(t, f)

		orderID, f1 := f.createOrderAndFulfillment(t, ctx, "packed", "shipped")
		f2 := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'packed', 1000, 900, 900)
		`, f2, orderID, s2)
		require.NoError(t, err)

		s1 := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'shipped', now(), now(), now())
		`, s1, orderID, f1)
		require.NoError(t, err)

		res, err := f.svc.DeliverShipment(ctx, f.adminID, s1, fulfillment.DeliverShipmentRequest{})
		require.NoError(t, err)
		assert.Equal(t, "packed", res.OrderStatus, "parent order must be packed when one is delivered and other is packed")
	})

	t.Run("delivered + assembling sibling -> parent assembling", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()
		s2 := createSecondSeller(t, f)

		orderID, f1 := f.createOrderAndFulfillment(t, ctx, "assembling", "shipped")
		f2 := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'assembling', 1000, 900, 900)
		`, f2, orderID, s2)
		require.NoError(t, err)

		s1 := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'shipped', now(), now(), now())
		`, s1, orderID, f1)
		require.NoError(t, err)

		res, err := f.svc.DeliverShipment(ctx, f.adminID, s1, fulfillment.DeliverShipmentRequest{})
		require.NoError(t, err)
		assert.Equal(t, "assembling", res.OrderStatus, "parent order must be assembling when one is delivered and other is assembling")
	})

	t.Run("all delivered -> parent delivered", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()
		s2 := createSecondSeller(t, f)

		orderID, f1 := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
		f2 := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'delivered', 1000, 900, 900)
		`, f2, orderID, s2)
		require.NoError(t, err)

		s1 := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'shipped', now(), now(), now())
		`, s1, orderID, f1)
		require.NoError(t, err)

		res, err := f.svc.DeliverShipment(ctx, f.adminID, s1, fulfillment.DeliverShipmentRequest{})
		require.NoError(t, err)
		assert.Equal(t, "delivered", res.OrderStatus, "parent order must be delivered when all fulfillments are delivered")
	})
}

func TestDelivery_PreconditionsAndRejections(t *testing.T) {
	ctx := context.Background()

	t.Run("shipment not found", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		_, err := f.svc.DeliverShipment(ctx, f.adminID, uuid.New(), fulfillment.DeliverShipmentRequest{})
		assert.ErrorIs(t, err, fulfillment.ErrShipmentNotFound)
	})

	t.Run("mismatched linked fulfillment rejected", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		_, fID1 := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
		orderID2, _ := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
		sID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'shipped', now(), now(), now())
		`, sID, orderID2, fID1)
		require.NoError(t, err)

		_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
		assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFound)
	})

	t.Run("non-shipped shipment rejected", func(t *testing.T) {
		invalidShipmentStates := []string{"pending", "assembling", "packed", "failed", "cancelled"}
		for _, st := range invalidShipmentStates {
			f := setupPickingFixture(t, ctx)
			defer f.db.Close()

			orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
			sID := uuid.New()
			_, err := f.db.Exec(ctx, `
				INSERT INTO shipments (id, order_id, fulfillment_id, status, created_at, updated_at)
				VALUES ($1, $2, $3, $4, now(), now())
			`, sID, orderID, fID, st)
			require.NoError(t, err)

			_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
			assert.ErrorIs(t, err, fulfillment.ErrDeliveryNotAllowed, "shipment status %s should be rejected", st)
		}
	})

	t.Run("already delivered shipment rejected", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		orderID, fID := f.createOrderAndFulfillment(t, ctx, "delivered", "delivered")
		sID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, delivered_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'delivered', now(), now(), now())
		`, sID, orderID, fID)
		require.NoError(t, err)

		_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
		assert.ErrorIs(t, err, fulfillment.ErrShipmentAlreadyDelivered)
	})

	t.Run("fulfillment not shipped rejected", func(t *testing.T) {
		invalidFulfillmentStates := []string{"awaiting_payment", "paid", "assembling", "packed", "cancelled"}
		for _, fst := range invalidFulfillmentStates {
			f := setupPickingFixture(t, ctx)
			defer f.db.Close()

			orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", fst)
			sID := uuid.New()
			_, err := f.db.Exec(ctx, `
				INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
				VALUES ($1, $2, $3, 'shipped', now(), now(), now())
			`, sID, orderID, fID)
			require.NoError(t, err)

			_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
			assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotShipped, "fulfillment status %s should be rejected", fst)
		}
	})

	t.Run("contradictory parent order states rejected", func(t *testing.T) {
		invalidParentStates := []struct {
			status      string
			expectedErr error
		}{
			{"cancelled", fulfillment.ErrOrderCancelled},
			{"delivered", fulfillment.ErrShipmentContradictoryState},
			{"awaiting_payment", fulfillment.ErrDeliveryNotAllowed},
			{"paid", fulfillment.ErrDeliveryNotAllowed},
		}

		for _, tc := range invalidParentStates {
			f := setupPickingFixture(t, ctx)
			defer f.db.Close()

			orderID, fID := f.createOrderAndFulfillment(t, ctx, tc.status, "shipped")
			sID := uuid.New()
			_, err := f.db.Exec(ctx, `
				INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
				VALUES ($1, $2, $3, 'shipped', now(), now(), now())
			`, sID, orderID, fID)
			require.NoError(t, err)

			_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
			assert.ErrorIs(t, err, tc.expectedErr, "parent status %s should be rejected with %v", tc.status, tc.expectedErr)
		}
	})

	t.Run("linkage mismatch order A vs fulfillment of order B rejected", func(t *testing.T) {
		f := setupPickingFixture(t, ctx)
		defer f.db.Close()

		orderA, _ := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
		orderB, fID_B := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")

		sID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'shipped', now(), now(), now())
		`, sID, orderA, fID_B)
		require.NoError(t, err)

		_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
		assert.ErrorIs(t, err, fulfillment.ErrFulfillmentNotFound, "cross-order fulfillment mismatch must be rejected")

		// Verify ZERO mutation
		var sStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM shipments WHERE id = $1`, sID).Scan(&sStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", sStatus)

		var fBStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fID_B).Scan(&fBStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", fBStatus)

		var oAStatus, oBStatus string
		err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderA).Scan(&oAStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", oAStatus)
		err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderB).Scan(&oBStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", oBStatus)
	})
}

func TestDelivery_ConcurrencyAndIdempotency(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	sID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now(), now())
	`, sID, orderID, fID)
	require.NoError(t, err)

	const concurrency = 5
	var wg sync.WaitGroup
	results := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
			results[idx] = err
		}(i)
	}
	wg.Wait()

	var successCount, conflictCount int
	for _, err := range results {
		if err == nil {
			successCount++
		} else if err == fulfillment.ErrShipmentAlreadyDelivered || err == fulfillment.ErrDeliveryNotAllowed {
			conflictCount++
		}
	}

	assert.Equal(t, 1, successCount, "exactly one concurrent delivery should succeed")
	assert.Equal(t, concurrency-1, conflictCount, "all losers must receive conflict error")

	// Capture first delivered_at
	var firstDeliveredAt time.Time
	err = f.db.QueryRow(ctx, `SELECT delivered_at FROM shipments WHERE id = $1`, sID).Scan(&firstDeliveredAt)
	require.NoError(t, err)
	assert.False(t, firstDeliveredAt.IsZero())

	// Check DB state
	var eventCount int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1`, sID).Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount, "exactly 1 shipment_event must be recorded")

	// Repeated delivery attempt must return ErrShipmentAlreadyDelivered
	_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
	assert.ErrorIs(t, err, fulfillment.ErrShipmentAlreadyDelivered)

	// delivered_at must remain byte/time identical
	var secondDeliveredAt time.Time
	err = f.db.QueryRow(ctx, `SELECT delivered_at FROM shipments WHERE id = $1`, sID).Scan(&secondDeliveredAt)
	require.NoError(t, err)
	assert.Equal(t, firstDeliveredAt, secondDeliveredAt, "delivered_at must remain unchanged after repeated delivery")

	// shipment_events count must remain 1
	var eventCountPost int
	err = f.db.QueryRow(ctx, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1`, sID).Scan(&eventCountPost)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCountPost)
}

func TestDelivery_ConflictingMutationRace(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	sID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now(), now())
	`, sID, orderID, fID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	var deliverErr, updateErr error

	// Goroutine 1: DeliverShipment
	go func() {
		defer wg.Done()
		_, deliverErr = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
	}()

	// Goroutine 2: Generic Shipment status update attempt to 'failed'
	go func() {
		defer wg.Done()
		updateErr = f.svc.UpdateShipmentStatus(ctx, f.adminID, sID, fulfillment.UpdateShipmentStatusRequest{
			Status: "failed",
		})
	}()

	wg.Wait()

	// Verify DB state is completely coherent and never in a mixed contradictory state
	var finalShipmentStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM shipments WHERE id = $1`, sID).Scan(&finalShipmentStatus)
	require.NoError(t, err)

	var finalFulfillmentStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fID).Scan(&finalFulfillmentStatus)
	require.NoError(t, err)

	var finalOrderStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&finalOrderStatus)
	require.NoError(t, err)

	if finalShipmentStatus == "delivered" {
		assert.Equal(t, "delivered", finalFulfillmentStatus)
		assert.Equal(t, "delivered", finalOrderStatus)
		assert.NoError(t, deliverErr)
		assert.Error(t, updateErr)
	} else if finalShipmentStatus == "failed" {
		assert.NoError(t, updateErr)
		assert.Error(t, deliverErr)
	} else {
		t.Fatalf("unexpected shipment status %s", finalShipmentStatus)
	}

	// Invariant: Never allow mixed shipment=delivered + fulfillment=shipped or inverse
	if finalShipmentStatus == "delivered" {
		assert.Equal(t, "delivered", finalFulfillmentStatus, "fulfillment must be delivered if shipment is delivered")
	}
	if finalFulfillmentStatus == "delivered" {
		assert.Equal(t, "delivered", finalShipmentStatus, "shipment must be delivered if fulfillment is delivered")
	}
}

func TestDelivery_DeliveredThenFailedRejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	sID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now(), now())
	`, sID, orderID, fID)
	require.NoError(t, err)

	// Step 1: DeliverShipment succeeds
	_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
	require.NoError(t, err)

	// Step 2: Attempting to update shipment status to 'failed' must be rejected
	err = f.svc.UpdateShipmentStatus(ctx, f.adminID, sID, fulfillment.UpdateShipmentStatusRequest{
		Status: "failed",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot change status of delivered shipment")

	// Step 3: Verify all entities remain cleanly delivered
	var sStatus, fStatus, oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM shipments WHERE id = $1`, sID).Scan(&sStatus)
	require.NoError(t, err)
	assert.Equal(t, "delivered", sStatus)

	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "delivered", fStatus)

	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "delivered", oStatus)
}

func TestDelivery_FailedThenDeliverRejected(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	sID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now(), now())
	`, sID, orderID, fID)
	require.NoError(t, err)

	// Step 1: UpdateShipmentStatus to 'failed' succeeds
	err = f.svc.UpdateShipmentStatus(ctx, f.adminID, sID, fulfillment.UpdateShipmentStatusRequest{
		Status: "failed",
	})
	require.NoError(t, err)

	// Step 2: DeliverShipment on failed shipment must be rejected
	_, err = f.svc.DeliverShipment(ctx, f.adminID, sID, fulfillment.DeliverShipmentRequest{})
	assert.ErrorIs(t, err, fulfillment.ErrDeliveryNotAllowed)

	// Step 3: Verify shipment remains failed, fulfillment and order remain shipped (never delivered)
	var sStatus, fStatus, oStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM shipments WHERE id = $1`, sID).Scan(&sStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", sStatus)

	err = f.db.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1`, fID).Scan(&fStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", fStatus)

	err = f.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", oStatus)
}

func TestDelivery_GenericPatchCannotBypassDelivery(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	sID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now(), now())
	`, sID, orderID, fID)
	require.NoError(t, err)

	// Attempt to set status to 'delivered' directly via generic status update
	err = f.svc.UpdateShipmentStatus(ctx, f.adminID, sID, fulfillment.UpdateShipmentStatusRequest{
		Status: "delivered",
	})
	assert.ErrorIs(t, err, fulfillment.ErrDispatchNotAllowed)

	var currentStatus string
	err = f.db.QueryRow(ctx, `SELECT status FROM shipments WHERE id = $1`, sID).Scan(&currentStatus)
	require.NoError(t, err)
	assert.Equal(t, "shipped", currentStatus, "shipment status must remain shipped after generic patch rejection")
}

func TestDelivery_HandlerAndRBAC(t *testing.T) {
	ctx := context.Background()
	f := setupPickingFixture(t, ctx)
	defer f.db.Close()

	handler := fulfillment.NewHandler(f.svc)

	orderID, fID := f.createOrderAndFulfillment(t, ctx, "shipped", "shipped")
	sID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now(), now())
	`, sID, orderID, fID)
	require.NoError(t, err)

	t.Run("unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+sID.String()+"/deliver", nil)
		rec := httptest.NewRecorder()

		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", sID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))

		handler.DeliverShipment(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid shipment id -> 400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/invalid-uuid/deliver", nil)
		rec := httptest.NewRecorder()

		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		req = req.WithContext(context.WithValue(req.Context(), "userID", f.adminID))

		handler.DeliverShipment(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("authenticated with valid id -> 200", func(t *testing.T) {
		body, _ := json.Marshal(fulfillment.DeliverShipmentRequest{
			Comment: func(s string) *string { return &s }("Доставлено курьером"),
		})
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+sID.String()+"/deliver", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", sID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		req = req.WithContext(context.WithValue(req.Context(), "userID", f.adminID))

		handler.DeliverShipment(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var res fulfillment.DeliveryResult
		err := json.NewDecoder(rec.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, "delivered", res.ShipmentStatus)
		assert.Equal(t, "delivered", res.FulfillmentStatus)
		assert.Equal(t, "delivered", res.OrderStatus)
	})

	t.Run("already delivered -> 409", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+sID.String()+"/deliver", nil)
		rec := httptest.NewRecorder()

		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", sID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		req = req.WithContext(context.WithValue(req.Context(), "userID", f.adminID))

		handler.DeliverShipment(rec, req)
		assert.Equal(t, http.StatusConflict, rec.Code)
	})
}
