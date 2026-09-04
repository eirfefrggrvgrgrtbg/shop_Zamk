package inventory_test

import (
	"context"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminInventory_UnitTraceability(t *testing.T) {
	testutil.AssertTestDatabase(t, testDB)
	ctx := context.Background()

	repo := inventory.NewRepository(testDB)
	pfx := "trace_" + uuid.NewString()[:8]

	// 1. Create seller
	sellerID := uuid.New()
	_, err := testDB.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'Traceability Brand', $2, $3, 'active')
	`, sellerID, pfx+"-brand", pfx+"@zamk.local")
	require.NoError(t, err)

	// 2. Create product & variant
	prodID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id)
		VALUES ($1, 'Traceability Blazer', $2, 2500000, 'published', $3)
	`, prodID, pfx+"-blazer", sellerID)
	require.NoError(t, err)

	varID := uuid.New()
	sku := "SKU-TRC-" + uuid.NewString()[:6]
	barcode := "ZMK-TRC-" + uuid.NewString()[:6]
	_, err = testDB.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode, size, color)
		VALUES ($1, $2, $3, $4, 'M', 'Charcoal')
	`, varID, prodID, sku, barcode)
	require.NoError(t, err)

	// 3. Create supply
	supplyID := uuid.New()
	supplyNum := "SUP-TRC-" + uuid.NewString()[:6]
	_, err = testDB.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, supply_number, status, handoff_method, created_at, updated_at)
		VALUES ($1, $2, $3, 'completed', 'courier', now() - interval '5 days', now() - interval '5 days')
	`, supplyID, sellerID, supplyNum)
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, NOW(), NOW())
	`, supplyItemID, supplyID, varID)
	require.NoError(t, err)

	// 4. Create user for receiving/staff and orders
	staffID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name)
		VALUES ($1, $2, 'hash', 'admin', 'Warehouse Staff')
	`, staffID, pfx+"-staff@zamk.local")
	require.NoError(t, err)

	custID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name)
		VALUES ($1, $2, 'hash', 'customer', 'Customer One')
	`, custID, pfx+"-cust@zamk.local")
	require.NoError(t, err)

	recSessionID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO supply_receiving_sessions (id, supply_id, status, started_at, started_by_staff_id, completed_at, created_at, updated_at)
		VALUES ($1, $2, 'completed', now() - interval '5 days', $3, now() - interval '5 days', now() - interval '5 days', now() - interval '5 days')
	`, recSessionID, supplyID, staffID)
	require.NoError(t, err)

	recItemID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO supply_receiving_items (id, session_id, supply_item_id, variant_id, sku, product_title, expected_quantity, scanned_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'Traceability Blazer', 10, 10, NOW(), NOW())
	`, recItemID, recSessionID, supplyItemID, varID, sku)
	require.NoError(t, err)

	// CASE A: Received warehouse ZMU (free, on floor)
	codeA := "ZMU-CASE-A-" + uuid.NewString()[:6]
	unitIDA := uuid.New()
	timeA := time.Now().Add(-5 * 24 * time.Hour)
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 1, $6, $6)
	`, unitIDA, varID, codeA, supplyID, supplyItemID, timeA)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO supply_receiving_scans (id, session_id, supply_receiving_item_id, staff_id, inventory_unit_id, quantity, condition, created_at)
		VALUES ($1, $2, $3, $4, $5, 1, 'ok', $6)
	`, uuid.New(), recSessionID, recItemID, staffID, unitIDA, timeA.Add(5*time.Minute))
	require.NoError(t, err)

	t.Run("Case A: Received warehouse ZMU timeline", func(t *testing.T) {
		res, err := repo.GetAdminInventoryUnitTraceability(ctx, codeA)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, codeA, res.Identity.UnitCode)
		assert.Equal(t, "warehouse", res.CurrentState.Status)
		assert.Equal(t, "free", res.CurrentState.Availability)
		assert.False(t, res.CurrentState.IsStaleAllocation)
		assert.Nil(t, res.CurrentContext.LiveAllocation)
		assert.Nil(t, res.CurrentContext.StaleAllocation)
		assert.False(t, res.HasPartialHistory)

		require.Len(t, res.Timeline, 2)
		assert.Equal(t, "received", res.Timeline[0].Type)
		assert.Equal(t, "Принята на склад", res.Timeline[0].EventName)
		assert.Equal(t, "staff", res.Timeline[0].ActorRole)
		assert.Equal(t, "Warehouse Staff", res.Timeline[0].ActorName)

		assert.Equal(t, "inbound_created", res.Timeline[1].Type)
		assert.Equal(t, "Ожидается поступление", res.Timeline[1].EventName)
	})

	// CASE B: Live allocated + picked ZMU
	codeB := "ZMU-CASE-B-" + uuid.NewString()[:6]
	unitIDB := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 2, $6, $6)
	`, unitIDB, varID, codeB, supplyID, supplyItemID, timeA)
	require.NoError(t, err)

	orderIDB := uuid.New()
	orderNumB := "ORD-B-" + uuid.NewString()[:6]
	_, err = testDB.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'assembling', 2500000, 'Customer', '+79991112233', 'cust@zamk.local', 'Delivery Address', now() - interval '2 hours', now() - interval '2 hours')
	`, orderIDB, custID, orderNumB)
	require.NoError(t, err)

	orderItemIDB := uuid.New()
	fulfillIDB := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'assembling', now() - interval '2 hours', now() - interval '2 hours')
	`, fulfillIDB, orderIDB, sellerID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Traceability Blazer', 'traceability-blazer', 2500000, 1, 2500000)
	`, orderItemIDB, orderIDB, fulfillIDB, prodID, varID, sellerID)
	require.NoError(t, err)

	allocIDB := uuid.New()
	pickedAtB := time.Now().Add(-1 * time.Hour)
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at, picked_at)
		VALUES ($1, $2, $3, now() - interval '2 hours', $4)
	`, allocIDB, orderItemIDB, unitIDB, pickedAtB)
	require.NoError(t, err)

	t.Run("Case B: Live allocated + picked ZMU", func(t *testing.T) {
		res, err := repo.GetAdminInventoryUnitTraceability(ctx, codeB)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, "warehouse", res.CurrentState.Status)
		assert.Equal(t, "picked", res.CurrentState.Availability)
		assert.False(t, res.CurrentState.IsStaleAllocation)
		require.NotNil(t, res.CurrentContext.LiveAllocation)
		assert.Equal(t, orderNumB, res.CurrentContext.LiveAllocation.OrderNumber)

		// Timeline should contain picked event at top (excluding inbound/received)
		assert.Equal(t, "picked", res.Timeline[0].Type)
		assert.Equal(t, "Собрана на складе", res.Timeline[0].EventName)
		assert.Equal(t, "allocation_created", res.Timeline[1].Type)
		assert.Equal(t, "Назначена заказу", res.Timeline[1].EventName)
	})

	// CASE C: Shipped and delivered ZMU
	codeC := "ZMU-CASE-C-" + uuid.NewString()[:6]
	unitIDC := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', $4, $5, 3, $6, $6)
	`, unitIDC, varID, codeC, supplyID, supplyItemID, timeA)
	require.NoError(t, err)

	orderIDC := uuid.New()
	orderNumC := "ORD-C-" + uuid.NewString()[:6]
	_, err = testDB.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 2500000, 'Happy Buyer', '+79991112233', 'buyer@zamk.local', 'Delivery Address', now() - interval '3 days', now() - interval '1 day')
	`, orderIDC, custID, orderNumC)
	require.NoError(t, err)

	fulfillIDC := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, packed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '2 days', now() - interval '3 days', now() - interval '1 day')
	`, fulfillIDC, orderIDC, sellerID)
	require.NoError(t, err)

	orderItemIDC := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Traceability Blazer', 'traceability-blazer', 2500000, 1, 2500000)
	`, orderItemIDC, orderIDC, fulfillIDC, prodID, varID, sellerID)
	require.NoError(t, err)

	allocIDC := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at, picked_at)
		VALUES ($1, $2, $3, now() - interval '3 days', now() - interval '2 days' - interval '1 hour')
	`, allocIDC, orderItemIDC, unitIDC)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '2 days', now() - interval '1 day', now() - interval '2 days', now() - interval '1 day')
	`, uuid.New(), orderIDC, fulfillIDC)
	require.NoError(t, err)

	t.Run("Case C: Shipped/delivered ZMU", func(t *testing.T) {
		res, err := repo.GetAdminInventoryUnitTraceability(ctx, codeC)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, "shipped", res.CurrentState.Status)
		assert.Equal(t, "unavailable_shipped", res.CurrentState.Availability)
		assert.False(t, res.CurrentState.IsStaleAllocation)

		types := make([]string, len(res.Timeline))
		for i, ev := range res.Timeline {
			types[i] = ev.Type
		}
		assert.Contains(t, types, "delivered")
		assert.Contains(t, types, "shipped")
		assert.Contains(t, types, "packed")
		assert.Contains(t, types, "picked")
		assert.Contains(t, types, "allocation_created")
	})

	// CASE D & E: Returned + restocked ZMU, with stale historical allocation
	codeDE := "ZMU-CASE-DE-" + uuid.NewString()[:6]
	unitIDDE := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 4, $6, $6)
	`, unitIDDE, varID, codeDE, supplyID, supplyItemID, timeA)
	require.NoError(t, err)

	orderIDDE := uuid.New()
	orderNumDE := "ORD-DE-" + uuid.NewString()[:6]
	_, err = testDB.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 2500000, 'Return Customer', '+79991112233', 'retcust@zamk.local', 'Delivery Address', now() - interval '4 days', now() - interval '2 days')
	`, orderIDDE, custID, orderNumDE)
	require.NoError(t, err)

	fulfillIDDE := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '4 days', now() - interval '2 days')
	`, fulfillIDDE, orderIDDE, sellerID)
	require.NoError(t, err)

	orderItemIDDE := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Traceability Blazer', 'traceability-blazer', 2500000, 1, 2500000)
	`, orderItemIDDE, orderIDDE, fulfillIDDE, prodID, varID, sellerID)
	require.NoError(t, err)

	// Allocation on terminal order has released_at = NULL -> stale!
	allocIDDE := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at, picked_at)
		VALUES ($1, $2, $3, now() - interval '4 days', now() - interval '3 days')
	`, allocIDDE, orderItemIDDE, unitIDDE)
	require.NoError(t, err)

	returnIDDE := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO returns (id, order_id, user_id, fulfillment_id, status, reason, created_at, approved_at, receiving_started_at)
		VALUES ($1, $2, $3, $4, 'refunded', 'damaged', now() - interval '2 days', now() - interval '36 hours', now() - interval '24 hours')
	`, returnIDDE, orderIDDE, custID, fulfillIDDE)
	require.NoError(t, err)

	returnItemIDDE := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, condition, restock)
		VALUES ($1, $2, $3, 1, 'damaged', 'new', true)
	`, returnItemIDDE, returnIDDE, orderItemIDDE)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, disposition)
		VALUES ($1, $2, $3, now() - interval '23 hours', 'restock')
	`, uuid.New(), returnItemIDDE, allocIDDE)
	require.NoError(t, err)

	t.Run("Case D: Returned + restocked ZMU", func(t *testing.T) {
		res, err := repo.GetAdminInventoryUnitTraceability(ctx, codeDE)
		require.NoError(t, err)
		require.NotNil(t, res)

		types := make([]string, len(res.Timeline))
		for i, ev := range res.Timeline {
			types[i] = ev.Type
		}
		assert.Contains(t, types, "return_received")
		assert.Contains(t, types, "return_unit_scanned")
		assert.Contains(t, types, "return_receiving_started")
		assert.Contains(t, types, "return_approved")
		assert.Contains(t, types, "return_requested")
	})

	t.Run("Case E: Stale historical allocation does not appear as current owner and timeline has stable persisted timestamps", func(t *testing.T) {
		res1, err := repo.GetAdminInventoryUnitTraceability(ctx, codeDE)
		require.NoError(t, err)
		require.NotNil(t, res1)

		// Operationally free on warehouse, but flagged as stale in CurrentState
		assert.Equal(t, "warehouse", res1.CurrentState.Status)
		assert.Equal(t, "free", res1.CurrentState.Availability)
		assert.True(t, res1.CurrentState.IsStaleAllocation)
		assert.Equal(t, "stale_active_allocation", res1.CurrentState.HealthIssue)

		// NOT current live owner
		assert.Nil(t, res1.CurrentContext.LiveAllocation)
		require.NotNil(t, res1.CurrentContext.StaleAllocation)
		assert.Equal(t, orderNumDE, res1.CurrentContext.StaleAllocation.OrderNumber)

		// Timeline contains only actual recorded historical events (no synthetic time.Now())
		for _, ev := range res1.Timeline {
			assert.NotEqual(t, "allocation_stale_detected", ev.Type, "Derived diagnostic must not masquerade as historical timeline event with fake time")
		}

		// Repeated read after a delay yields 100% identical timestamps
		time.Sleep(10 * time.Millisecond)
		res2, err := repo.GetAdminInventoryUnitTraceability(ctx, codeDE)
		require.NoError(t, err)
		require.Equal(t, len(res1.Timeline), len(res2.Timeline))
		for i := range res1.Timeline {
			assert.Equal(t, res1.Timeline[i].ID, res2.Timeline[i].ID)
			assert.True(t, res1.Timeline[i].Timestamp.Equal(res2.Timeline[i].Timestamp), "Timeline event timestamps must remain completely stable across repeated reads")
		}
	})

	// CASE F: Partial lineage returns honest partial history (created from supply without receiving scan)
	codeF := "ZMU-CASE-F-" + uuid.NewString()[:6]
	unitIDF := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 5, now() - interval '10 days', now() - interval '10 days')
	`, unitIDF, varID, codeF, supplyID, supplyItemID)
	require.NoError(t, err)

	t.Run("Case F: Partial lineage returns honest partial history", func(t *testing.T) {
		res, err := repo.GetAdminInventoryUnitTraceability(ctx, codeF)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.True(t, res.HasPartialHistory)
		assert.Equal(t, "warehouse", res.CurrentState.Status)
		assert.Equal(t, "free", res.CurrentState.Availability)
		assert.Nil(t, res.Origin.ReceivedAt)
		require.Len(t, res.Timeline, 1)
		assert.Equal(t, "inbound_created", res.Timeline[0].Type)
	})

	t.Run("Case G: Simultaneous events preserve business causal order (prerequisite before consequence)", func(t *testing.T) {
		codeG := "ZMU-CASE-G-" + uuid.NewString()[:6]
		unitIDG := uuid.New()
		fixedTime := time.Now().Add(-2 * 24 * time.Hour).Truncate(time.Second)

		_, err := testDB.Exec(ctx, `
			INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
			VALUES ($1, $2, $3, 'warehouse', $4, $5, 6, $6, $6)
		`, unitIDG, varID, codeG, supplyID, supplyItemID, fixedTime)
		require.NoError(t, err)

		// Insert allocation where allocated_at and picked_at are identical
		orderIDG := uuid.New()
		orderNumG := "ORD-G-" + uuid.NewString()[:6]
		_, err = testDB.Exec(ctx, `
			INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
			VALUES ($1, $2, $3, 'assembling', 2500000, 'Order G Cust', '+79991112233', 'custg@zamk.local', 'Address G', $4, $4)
		`, orderIDG, custID, orderNumG, fixedTime)
		require.NoError(t, err)

		fulfillIDG := uuid.New()
		_, err = testDB.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'assembling', $4, $4)
		`, fulfillIDG, orderIDG, sellerID, fixedTime)
		require.NoError(t, err)

		orderItemIDG := uuid.New()
		_, err = testDB.Exec(ctx, `
			INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
			VALUES ($1, $2, $3, $4, $5, $6, 'Traceability Blazer', 'traceability-blazer', 2500000, 1, 2500000)
		`, orderItemIDG, orderIDG, fulfillIDG, prodID, varID, sellerID)
		require.NoError(t, err)

		allocIDG := uuid.New()
		_, err = testDB.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at, picked_at)
			VALUES ($1, $2, $3, $4, $4)
		`, allocIDG, orderItemIDG, unitIDG, fixedTime)
		require.NoError(t, err)

		res, err := repo.GetAdminInventoryUnitTraceability(ctx, codeG)
		require.NoError(t, err)
		require.NotNil(t, res)

		var allocIdx, pickIdx int = -1, -1
		for i, ev := range res.Timeline {
			if ev.Type == "allocation_created" {
				allocIdx = i
			}
			if ev.Type == "picked" {
				pickIdx = i
			}
		}
		require.NotEqual(t, -1, allocIdx)
		require.NotEqual(t, -1, pickIdx)
		// Prerequisite (allocation) must appear before consequence (picked) when timestamps are equal
		assert.Less(t, allocIdx, pickIdx, "allocation_created must precede picked when timestamps are equal")
	})
}
