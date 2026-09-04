package inventory_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminInventory_VariantDetail_PhysicalUnitsClassification(t *testing.T) {
	testutil.AssertTestDatabase(t, testDB)
	ctx := context.Background()

	repo := inventory.NewRepository(testDB)
	pfx := "vardet_" + uuid.NewString()[:8]

	// 1. Create seller
	sellerID := uuid.New()
	_, err := testDB.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'Variant Detail Brand', $2, $3, 'active')
	`, sellerID, pfx+"-brand", pfx+"@zamk.local")
	require.NoError(t, err)

	// 2. Create product & variant
	prodID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id)
		VALUES ($1, 'Test Detail Coat', $2, 3000000, 'published', $3)
	`, prodID, pfx+"-coat", sellerID)
	require.NoError(t, err)

	varID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode, size, color)
		VALUES ($1, $2, $3, $4, 'L', 'Navy')
	`, varID, prodID, "SKU-"+uuid.NewString()[:8], "ZMK-"+uuid.NewString()[:8])
	require.NoError(t, err)

	// 3. Create supply
	supplyID := uuid.New()
	supplyNum := "SUP-" + uuid.NewString()[:8]
	_, err = testDB.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, supply_number, status, handoff_method, created_at, updated_at)
		VALUES ($1, $2, $3, 'completed', 'pickup', NOW(), NOW())
	`, supplyID, sellerID, supplyNum)
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, NOW(), NOW())
	`, supplyItemID, supplyID, varID)
	require.NoError(t, err)

	// 4. Create inventory item (aggregate total: 10, reserved: 2)
	invItemID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 2)
	`, invItemID, prodID, varID, sellerID)
	require.NoError(t, err)

	// 5. Create customer user for orders
	userID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name)
		VALUES ($1, $2, 'hash', 'customer', 'Customer 1')
	`, userID, pfx+"_cust@zamk.local")
	require.NoError(t, err)

	// 6. Create Live Order (paid)
	liveOrderID := uuid.New()
	liveOrderNum := "ORD-" + uuid.NewString()[:8]
	_, err = testDB.Exec(ctx, `
		INSERT INTO orders (id, order_number, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, $3, 'paid', 3000000, 'Customer', '+79991112233', 'cust@zamk.local', 'Delivery Address')
	`, liveOrderID, liveOrderNum, userID)
	require.NoError(t, err)

	liveFulfillmentID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'paid')
	`, liveFulfillmentID, liveOrderID, sellerID)
	require.NoError(t, err)

	liveOrderItemID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Test Detail Coat', 'test-detail-coat', 3000000, 1, 3000000)
	`, liveOrderItemID, liveOrderID, liveFulfillmentID, prodID, varID, sellerID)
	require.NoError(t, err)

	// 7. Create Delivered Order (terminal)
	termOrderID := uuid.New()
	termOrderNum := "ORD-" + uuid.NewString()[:8]
	_, err = testDB.Exec(ctx, `
		INSERT INTO orders (id, order_number, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, $3, 'delivered', 3000000, 'Customer', '+79991112233', 'cust@zamk.local', 'Delivery Address')
	`, termOrderID, termOrderNum, userID)
	require.NoError(t, err)

	termFulfillmentID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, termFulfillmentID, termOrderID, sellerID)
	require.NoError(t, err)

	termOrderItemID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Test Detail Coat', 'test-detail-coat', 3000000, 1, 3000000)
	`, termOrderItemID, termOrderID, termFulfillmentID, prodID, varID, sellerID)
	require.NoError(t, err)

	// Unit 1: Warehouse free
	uFreeID := uuid.New()
	uFreeCode := "ZMU-" + strings.ToUpper(uuid.NewString()[:8]) + "FR"
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 1)
	`, uFreeID, varID, uFreeCode, supplyID, supplyItemID)
	require.NoError(t, err)

	// Unit 2: Warehouse live allocated (not picked)
	uLiveAllocID := uuid.New()
	uLiveAllocCode := "ZMU-" + strings.ToUpper(uuid.NewString()[:8]) + "LV"
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 2)
	`, uLiveAllocID, varID, uLiveAllocCode, supplyID, supplyItemID)
	require.NoError(t, err)

	liveAllocID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id)
		VALUES ($1, $2, $3)
	`, liveAllocID, liveOrderItemID, uLiveAllocID)
	require.NoError(t, err)

	// Unit 3: Warehouse stale allocation on delivered order
	uStaleID := uuid.New()
	uStaleCode := "ZMU-" + strings.ToUpper(uuid.NewString()[:8]) + "ST"
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index)
		VALUES ($1, $2, $3, 'warehouse', $4, $5, 3)
	`, uStaleID, varID, uStaleCode, supplyID, supplyItemID)
	require.NoError(t, err)

	staleAllocID := uuid.New()
	pickedTime := time.Now().Add(-24 * time.Hour)
	_, err = testDB.Exec(ctx, `
		INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at)
		VALUES ($1, $2, $3, $4)
	`, staleAllocID, termOrderItemID, uStaleID, pickedTime)
	require.NoError(t, err)

	// Unit 4: Expected
	uExpID := uuid.New()
	uExpCode := "ZMU-" + strings.ToUpper(uuid.NewString()[:8]) + "EX"
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index)
		VALUES ($1, $2, $3, 'expected', $4, $5, 4)
	`, uExpID, varID, uExpCode, supplyID, supplyItemID)
	require.NoError(t, err)

	// Unit 5: Damaged
	uDamID := uuid.New()
	uDamCode := "ZMU-" + strings.ToUpper(uuid.NewString()[:8]) + "DM"
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index)
		VALUES ($1, $2, $3, 'damaged', $4, $5, 5)
	`, uDamID, varID, uDamCode, supplyID, supplyItemID)
	require.NoError(t, err)

	// Unit 6: Shipped
	uShipID := uuid.New()
	uShipCode := "ZMU-" + strings.ToUpper(uuid.NewString()[:8]) + "SH"
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index)
		VALUES ($1, $2, $3, 'shipped', $4, $5, 6)
	`, uShipID, varID, uShipCode, supplyID, supplyItemID)
	require.NoError(t, err)

	// Fetch detail
	item, err := repo.GetAdminInventoryItemRich(ctx, invItemID)
	require.NoError(t, err)
	require.NotNil(t, item)

	// Verify top-level summaries
	assert.Equal(t, 10, item.Aggregate.Total)
	assert.Equal(t, 2, item.Aggregate.Reserved)
	assert.Equal(t, 8, item.Aggregate.Available)
	assert.Equal(t, 3, item.Physical.Warehouse)
	assert.Equal(t, 1, item.Physical.Allocated)
	assert.Equal(t, 2, item.Physical.Free) // 1 pure free + 1 stale allocated
	assert.Equal(t, 1, item.Physical.StaleAllocated)
	assert.Equal(t, 1, item.Physical.Expected)
	assert.Equal(t, 1, item.Physical.Damaged)
	assert.Equal(t, 1, item.Physical.Shipped)

	// Verify health
	assert.Equal(t, "mixed", item.AccountingMode)
	assert.Equal(t, "warning", item.Health.Status)
	assert.Contains(t, item.Health.Issues, "stale_active_allocation")

	// Verify physical units array
	require.Len(t, item.PhysicalUnits, 6)

	unitMap := make(map[string]inventory.AdminInventoryPhysicalUnit)
	for _, u := range item.PhysicalUnits {
		unitMap[u.UnitCode] = u
	}

	// 1. Free unit
	u1, ok := unitMap[uFreeCode]
	require.True(t, ok)
	assert.Equal(t, "warehouse", u1.Status)
	assert.Equal(t, "free", u1.Availability)
	assert.False(t, u1.IsStaleAllocation)
	assert.Nil(t, u1.LiveAllocation)
	assert.Nil(t, u1.StaleAllocation)
	require.NotNil(t, u1.SupplyLineage)
	assert.Equal(t, supplyNum, u1.SupplyLineage.SupplyNumber)

	// 2. Live allocated unit
	u2, ok := unitMap[uLiveAllocCode]
	require.True(t, ok)
	assert.Equal(t, "warehouse", u2.Status)
	assert.Equal(t, "allocated", u2.Availability)
	assert.False(t, u2.IsStaleAllocation)
	require.NotNil(t, u2.LiveAllocation)
	assert.Equal(t, liveOrderNum, u2.LiveAllocation.OrderNumber)
	assert.Equal(t, "paid", u2.LiveAllocation.OrderStatus)
	assert.Nil(t, u2.StaleAllocation)

	// 3. Stale allocated unit
	u3, ok := unitMap[uStaleCode]
	require.True(t, ok)
	assert.Equal(t, "warehouse", u3.Status)
	assert.Equal(t, "free", u3.Availability) // Operationally free!
	assert.True(t, u3.IsStaleAllocation)     // Flagged as stale!
	assert.Nil(t, u3.LiveAllocation)         // No live owner!
	require.NotNil(t, u3.StaleAllocation)    // Historical context present!
	assert.Equal(t, termOrderNum, u3.StaleAllocation.OrderNumber)
	assert.Equal(t, "delivered", u3.StaleAllocation.OrderStatus)

	// 4. Expected
	u4, ok := unitMap[uExpCode]
	require.True(t, ok)
	assert.Equal(t, "expected", u4.Status)
	assert.Equal(t, "unavailable_expected", u4.Availability)

	// 5. Damaged
	u5, ok := unitMap[uDamCode]
	require.True(t, ok)
	assert.Equal(t, "damaged", u5.Status)
	assert.Equal(t, "unavailable_damaged", u5.Availability)

	// 6. Shipped
	u6, ok := unitMap[uShipCode]
	require.True(t, ok)
	assert.Equal(t, "shipped", u6.Status)
	assert.Equal(t, "unavailable_shipped", u6.Availability)
	assert.False(t, u6.IsStaleAllocation)
}
