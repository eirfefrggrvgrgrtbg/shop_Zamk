package orders_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type allocFixture struct {
	db       *pgxpool.Pool
	repo     *orders.Repository
	sellerID uuid.UUID
	buyerID  uuid.UUID
	catID    uuid.UUID
	prodID   uuid.UUID
}

func setupAllocFixture(t *testing.T, ctx context.Context) *allocFixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	require.NotEmpty(t, dbURL, "test database URL must not be empty")

	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "failed to connect to test database")

	// Safety check: must strictly be zamk_test
	testutil.AssertTestDatabase(t, db)

	repo := orders.NewRepository(db)

	suffix := uuid.New().String()[:8]
	sellerUserID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()

	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Seller User', $2, 'hash', 'seller', 'active', now(), now())
	`, sellerUserID, fmt.Sprintf("seller-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Buyer User', $2, 'hash', 'customer', 'active', now(), now())
	`, buyerID, fmt.Sprintf("buyer-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Alloc Brand', $2, $3, 'active', now(), now())
	`, sellerID, fmt.Sprintf("alloc-seller-%s", suffix), fmt.Sprintf("contact-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Alloc Cat', $2, now(), now())
	`, catID, fmt.Sprintf("alloc-cat-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Alloc Prod', $4, 1000, 'published', now(), now())
	`, prodID, sellerID, catID, fmt.Sprintf("alloc-prod-%s", suffix))
	require.NoError(t, err)

	return &allocFixture{
		db:       db,
		repo:     repo,
		sellerID: sellerID,
		buyerID:  buyerID,
		catID:    catID,
		prodID:   prodID,
	}
}

func (f *allocFixture) createVariant(t *testing.T, ctx context.Context, skuPrefix string) uuid.UUID {
	t.Helper()
	variantID := uuid.New()
	suffix := uuid.New().String()[:8]
	_, err := f.db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1000, true, now(), now())
	`, variantID, f.prodID, fmt.Sprintf("%s-%s", skuPrefix, suffix), fmt.Sprintf("SSKU-%s", suffix), fmt.Sprintf("BC-%s", suffix))
	require.NoError(t, err)

	// Create inventory_items entry to track aggregate stock
	_, err = f.db.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 100, 0, now(), now())
	`, uuid.New(), f.prodID, variantID, f.sellerID)
	require.NoError(t, err)

	return variantID
}

func (f *allocFixture) createOrderAndItem(t *testing.T, ctx context.Context, variantID uuid.UUID, quantity int) (uuid.UUID, uuid.UUID) {
	t.Helper()
	orderID := uuid.New()
	orderItemID := uuid.New()
	fulfillmentID := uuid.New()
	suffix := uuid.New().String()[:8]
	subtotal := int64(quantity * 1000)

	_, err := f.db.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 'awaiting_payment', 1000, 'Customer', '+79990000000', 'cust@example.com', 'Warehouse 1', now(), now())
	`, orderID, f.buyerID, fmt.Sprintf("ORD-%s", suffix))
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'awaiting_payment', $4, 900, $5, now(), now())
	`, fulfillmentID, orderID, f.sellerID, subtotal, subtotal)
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Order Item Title', 'item-slug', 1000, $7, $8, now())
	`, orderItemID, orderID, fulfillmentID, f.prodID, variantID, f.sellerID, quantity, subtotal)
	require.NoError(t, err)

	return orderID, orderItemID
}

func (f *allocFixture) createWarehouseUnits(t *testing.T, ctx context.Context, variantID uuid.UUID, count int) []uuid.UUID {
	t.Helper()
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	suffix := uuid.New().String()[:8]

	_, err := f.db.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
	`, supplyID, f.sellerID, fmt.Sprintf("SUP-%s", suffix))
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, supplyItemID, supplyID, variantID, count)
	require.NoError(t, err)

	var unitIDs []uuid.UUID
	for i := 1; i <= count; i++ {
		unitID := uuid.New()
		unitCode := fmt.Sprintf("ZMU-%s-%03d", suffix, i)
		_, err := f.db.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'warehouse', now(), now())
		`, unitID, unitCode, variantID, supplyID, supplyItemID, i)
		require.NoError(t, err)
		unitIDs = append(unitIDs, unitID)
	}

	return unitIDs
}

func runInTx(ctx context.Context, db *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestPhysicalUnitAllocation_ExactQuantityAndAtomicity(t *testing.T) {
	ctx := context.Background()
	f := setupAllocFixture(t, ctx)
	defer f.db.Close()

	variantA := f.createVariant(t, ctx, "SKU-A")
	zmuUnits := f.createWarehouseUnits(t, ctx, variantA, 5)
	require.Len(t, zmuUnits, 5)

	_, item1ID := f.createOrderAndItem(t, ctx, variantA, 2)
	_, item2ID := f.createOrderAndItem(t, ctx, variantA, 2)
	_, item3ID := f.createOrderAndItem(t, ctx, variantA, 2)

	// Step 1: Allocate 2 units for item 1
	var allocated1 []uuid.UUID
	err := runInTx(ctx, f.db, func(tx pgx.Tx) error {
		var err error
		allocated1, err = f.repo.AllocateUnitsForOrderItem(ctx, tx, item1ID, 2, nil)
		return err
	})
	require.NoError(t, err)
	assert.Len(t, allocated1, 2)
	assert.NotEqual(t, allocated1[0], allocated1[1])

	activeAllocs1, err := f.repo.ListActiveAllocationsForOrderItem(ctx, item1ID)
	require.NoError(t, err)
	assert.Len(t, activeAllocs1, 2)

	// Step 2: Allocate 2 units for item 2
	var allocated2 []uuid.UUID
	err = runInTx(ctx, f.db, func(tx pgx.Tx) error {
		var err error
		allocated2, err = f.repo.AllocateUnitsForOrderItem(ctx, tx, item2ID, 2, nil)
		return err
	})
	require.NoError(t, err)
	assert.Len(t, allocated2, 2)
	assert.NotEqual(t, allocated2[0], allocated2[1])

	// Ensure item 1 and item 2 have disjoint units
	for _, u1 := range allocated1 {
		for _, u2 := range allocated2 {
			assert.NotEqual(t, u1, u2, "units allocated to different items must be distinct")
		}
	}

	// Step 3: Try to allocate 2 units for item 3 when only 1 unit remains
	err = runInTx(ctx, f.db, func(tx pgx.Tx) error {
		_, err := f.repo.AllocateUnitsForOrderItem(ctx, tx, item3ID, 2, nil)
		return err
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, orders.ErrInsufficientWarehouseUnits))

	// Verify ZERO allocation rows exist for item 3
	activeAllocs3, err := f.repo.ListActiveAllocationsForOrderItem(ctx, item3ID)
	require.NoError(t, err)
	assert.Empty(t, activeAllocs3, "failed allocation must not leave partial rows")

	// Step 4: Verify physical status of all units remains 'warehouse'
	for _, uid := range zmuUnits {
		var status string
		err := f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uid).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", status, "physical unit status must remain warehouse")
	}

	// Step 5: Verify inventory_items.reserved_stock was not modified
	var reservedStock int
	err = f.db.QueryRow(ctx, `SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1`, variantA).Scan(&reservedStock)
	require.NoError(t, err)
	assert.Equal(t, 0, reservedStock, "M4.1.1 must not modify aggregate reserved_stock")
}

func TestPhysicalUnitAllocation_VariantAuthorityAndMismatch(t *testing.T) {
	ctx := context.Background()
	f := setupAllocFixture(t, ctx)
	defer f.db.Close()

	variantA := f.createVariant(t, ctx, "SKU-VAR-A")
	variantB := f.createVariant(t, ctx, "SKU-VAR-B")

	// Create warehouse units ONLY for variant B
	unitsB := f.createWarehouseUnits(t, ctx, variantB, 3)
	require.Len(t, unitsB, 3)

	// Create order item requesting variant A
	_, itemAID := f.createOrderAndItem(t, ctx, variantA, 1)

	// Attempt allocation for item A -> must fail because 0 units of variant A exist
	err := runInTx(ctx, f.db, func(tx pgx.Tx) error {
		_, err := f.repo.AllocateUnitsForOrderItem(ctx, tx, itemAID, 1, nil)
		return err
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, orders.ErrInsufficientWarehouseUnits))

	activeAllocsA, err := f.repo.ListActiveAllocationsForOrderItem(ctx, itemAID)
	require.NoError(t, err)
	assert.Empty(t, activeAllocsA, "variant mismatch must yield zero allocations")

	// Now add units for variant A
	unitsA := f.createWarehouseUnits(t, ctx, variantA, 2)
	require.Len(t, unitsA, 2)

	// Allocate for item A -> should succeed and only select from unitsA
	var allocatedA []uuid.UUID
	err = runInTx(ctx, f.db, func(tx pgx.Tx) error {
		var err error
		allocatedA, err = f.repo.AllocateUnitsForOrderItem(ctx, tx, itemAID, 1, nil)
		return err
	})
	require.NoError(t, err)
	require.Len(t, allocatedA, 1)
	assert.Contains(t, unitsA, allocatedA[0], "allocated unit must belong to variant A")
	assert.NotContains(t, unitsB, allocatedA[0], "allocated unit must NOT belong to variant B")
}

func TestPhysicalUnitAllocation_ReleaseAndHistoryPreservation(t *testing.T) {
	ctx := context.Background()
	f := setupAllocFixture(t, ctx)
	defer f.db.Close()

	variantA := f.createVariant(t, ctx, "SKU-REL")
	units := f.createWarehouseUnits(t, ctx, variantA, 2)
	require.Len(t, units, 2)

	_, item1ID := f.createOrderAndItem(t, ctx, variantA, 2)
	_, item2ID := f.createOrderAndItem(t, ctx, variantA, 2)

	// Step 1: Allocate for item 1
	var allocated []uuid.UUID
	err := runInTx(ctx, f.db, func(tx pgx.Tx) error {
		var err error
		allocated, err = f.repo.AllocateUnitsForOrderItem(ctx, tx, item1ID, 2, nil)
		return err
	})
	require.NoError(t, err)
	assert.Len(t, allocated, 2)

	// Step 2: Release allocations for item 1 with reason "customer_cancelled"
	releaseReason := "customer_cancelled"
	err = runInTx(ctx, f.db, func(tx pgx.Tx) error {
		return f.repo.ReleaseAllocationsForOrderItem(ctx, tx, item1ID, releaseReason)
	})
	require.NoError(t, err)

	// Step 3: Verify active allocations is now 0
	activeAllocs, err := f.repo.ListActiveAllocationsForOrderItem(ctx, item1ID)
	require.NoError(t, err)
	assert.Empty(t, activeAllocs, "active allocations must be empty after release")

	// Step 4: Verify historical rows exist and have released_at and release_reason
	allAllocs, err := f.repo.ListAllAllocationsForOrderItem(ctx, item1ID)
	require.NoError(t, err)
	require.Len(t, allAllocs, 2, "history rows must be retained")
	for _, a := range allAllocs {
		assert.NotNil(t, a.ReleasedAt, "released_at must be populated")
		require.NotNil(t, a.ReleaseReason)
		assert.Equal(t, releaseReason, *a.ReleaseReason)
	}

	// Step 5: Verify units are eligible again for item 2
	var allocated2 []uuid.UUID
	err = runInTx(ctx, f.db, func(tx pgx.Tx) error {
		var err error
		allocated2, err = f.repo.AllocateUnitsForOrderItem(ctx, tx, item2ID, 2, nil)
		return err
	})
	require.NoError(t, err)
	assert.Len(t, allocated2, 2)

	// Step 6: Verify physical status of units remains 'warehouse'
	for _, uid := range units {
		var status string
		err := f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, uid).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "warehouse", status)
	}
}

func TestPhysicalUnitAllocation_TrueConcurrencyProtection(t *testing.T) {
	ctx := context.Background()
	f := setupAllocFixture(t, ctx)
	defer f.db.Close()

	variant := f.createVariant(t, ctx, "SKU-CONCUR")

	// Exactly 1 physical unit in warehouse
	units := f.createWarehouseUnits(t, ctx, variant, 1)
	require.Len(t, units, 1)
	singleUnitID := units[0]

	// Create two competing order items
	_, itemAID := f.createOrderAndItem(t, ctx, variant, 1)
	_, itemBID := f.createOrderAndItem(t, ctx, variant, 1)

	var wg sync.WaitGroup
	type allocResult struct {
		orderItemID uuid.UUID
		unitIDs     []uuid.UUID
		err         error
	}

	results := make(chan allocResult, 2)
	startBarrier := make(chan struct{})

	runAttempt := func(orderItemID uuid.UUID) {
		defer wg.Done()
		<-startBarrier // synchronize start

		err := runInTx(ctx, f.db, func(tx pgx.Tx) error {
			unitIDs, err := f.repo.AllocateUnitsForOrderItem(ctx, tx, orderItemID, 1, nil)
			if err != nil {
				return err
			}
			results <- allocResult{orderItemID: orderItemID, unitIDs: unitIDs, err: nil}
			return nil
		})
		if err != nil {
			results <- allocResult{orderItemID: orderItemID, unitIDs: nil, err: err}
		}
	}

	wg.Add(2)
	go runAttempt(itemAID)
	go runAttempt(itemBID)

	close(startBarrier)
	wg.Wait()
	close(results)

	var successCount int
	var failCount int
	var winnerItemID uuid.UUID

	for res := range results {
		if res.err == nil {
			successCount++
			winnerItemID = res.orderItemID
			assert.Equal(t, []uuid.UUID{singleUnitID}, res.unitIDs)
		} else {
			failCount++
			assert.True(t, errors.Is(res.err, orders.ErrInsufficientWarehouseUnits), "expected ErrInsufficientWarehouseUnits, got %v", res.err)
		}
	}

	assert.Equal(t, 1, successCount, "exactly one concurrent allocation must succeed")
	assert.Equal(t, 1, failCount, "exactly one concurrent allocation must fail")

	// Verify authoritative active allocations in DB
	var totalActiveAllocations int
	err := f.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM order_item_allocations
		WHERE inventory_unit_id = $1 AND released_at IS NULL
	`, singleUnitID).Scan(&totalActiveAllocations)
	require.NoError(t, err)
	assert.Equal(t, 1, totalActiveAllocations, "only one active allocation record must exist for the physical unit")

	// Check winner has active allocation
	winnerAllocs, err := f.repo.ListActiveAllocationsForOrderItem(ctx, winnerItemID)
	require.NoError(t, err)
	assert.Len(t, winnerAllocs, 1)

	// Physical unit status remains warehouse
	var status string
	err = f.db.QueryRow(ctx, `SELECT status FROM inventory_units WHERE id = $1`, singleUnitID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "warehouse", status)
}
