package inventory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminInventory_ZMUContextHandoff(t *testing.T) {
	testutil.AssertTestDatabase(t, testDB)
	ctx := context.Background()

	repo := inventory.NewRepository(testDB)
	sellersRepo := sellers.NewRepository(testDB)
	pgClient, err := postgres.NewClient(ctx, testutil.GetTestDatabaseURL())
	require.NoError(t, err)
	service := inventory.NewService(repo, sellersRepo, pgClient)

	// Fixture prefix for safety
	pfx := "zmu_tst_" + uuid.NewString()[:8]

	// 1. Create Seller User, Seller, and SellerUser mapping
	sellerUserID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name)
		VALUES ($1, $2, 'hash', 'seller', 'ZMU Test Seller')
	`, sellerUserID, pfx+"_seller@zamk.local")
	require.NoError(t, err)

	sellerID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'ZMU Test Brand', $2, $3, 'active')
	`, sellerID, pfx+"-brand", pfx+"_seller@zamk.local")
	require.NoError(t, err)

	sellerUserRoleID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role)
		VALUES ($1, $2, $3, 'owner')
	`, sellerUserRoleID, sellerID, sellerUserID)
	require.NoError(t, err)

	// 2. Create Product 1 (Wool Coat) with Variant 1
	prod1ID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id)
		VALUES ($1, 'Dev Premium Wool Coat', $2, 2500000, 'published', $3)
	`, prod1ID, pfx+"-wool-coat", sellerID)
	require.NoError(t, err)

	var1ID := uuid.New()
	sku1 := "SKU-COAT-M-BLK"
	_, err = testDB.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, size, color, is_active)
		VALUES ($1, $2, $3, 'M', 'Black', true)
	`, var1ID, prod1ID, sku1)
	require.NoError(t, err)

	// Aggregate inventory row for Variant 1
	inv1ID := uuid.New()
	err = repo.CreateItem(ctx, &inventory.Item{
		ID:               inv1ID,
		ProductID:        prod1ID,
		ProductVariantID: var1ID,
		SellerID:         sellerID,
		TotalStock:       15,
		ReservedStock:    2,
	})
	require.NoError(t, err)

	// 3. Create Product 2 (Silk Scarf) with Variant 2
	prod2ID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id)
		VALUES ($1, 'Dev Silk Scarf', $2, 500000, 'published', $3)
	`, prod2ID, pfx+"-silk-scarf", sellerID)
	require.NoError(t, err)

	var2ID := uuid.New()
	sku2 := "SKU-SCARF-RED"
	_, err = testDB.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, size, color, is_active)
		VALUES ($1, $2, $3, 'One Size', 'Red', true)
	`, var2ID, prod2ID, sku2)
	require.NoError(t, err)

	// Aggregate inventory row for Variant 2
	inv2ID := uuid.New()
	err = repo.CreateItem(ctx, &inventory.Item{
		ID:               inv2ID,
		ProductID:        prod2ID,
		ProductVariantID: var2ID,
		SellerID:         sellerID,
		TotalStock:       8,
		ReservedStock:    0,
	})
	require.NoError(t, err)

	// Supply & Supply Item for origin foreign keys
	supplyID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, supply_number, status, handoff_method, created_at, updated_at)
		VALUES ($1, $2, $3, 'completed', 'zamk_dropoff', NOW(), NOW())
	`, supplyID, sellerID, pfx+"-SUP-01")
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 10, 10, 0, 0, 0, NOW(), NOW())
	`, supplyItemID, supplyID, var1ID)
	require.NoError(t, err)

	genZMU := func() string {
		chars := []rune("23456789ABCDEFGHJKMNPQRSTUVWXYZ")
		u := uuid.New()
		res := make([]rune, 16)
		for i := 0; i < 16; i++ {
			res[i] = chars[int(u[i])%len(chars)]
		}
		return "ZMU-" + string(res)
	}

	// 4. Physical Units for Variant 1 with different statuses:
	// A. Warehouse ZMU
	unitWarehouseID := uuid.New()
	zmuWarehouse := genZMU()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, 'warehouse', NOW(), NOW())
	`, unitWarehouseID, zmuWarehouse, var1ID, supplyID, supplyItemID)
	require.NoError(t, err)

	// B. Shipped ZMU
	unitShippedID := uuid.New()
	zmuShipped := genZMU()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 2, 'shipped', NOW(), NOW())
	`, unitShippedID, zmuShipped, var1ID, supplyID, supplyItemID)
	require.NoError(t, err)

	// C. Damaged ZMU
	unitDamagedID := uuid.New()
	zmuDamaged := genZMU()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 3, 'damaged', NOW(), NOW())
	`, unitDamagedID, zmuDamaged, var1ID, supplyID, supplyItemID)
	require.NoError(t, err)

	// D. Written-off ZMU
	unitWrittenOffID := uuid.New()
	zmuWrittenOff := genZMU()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 4, 'written_off', NOW(), NOW())
	`, unitWrittenOffID, zmuWrittenOff, var1ID, supplyID, supplyItemID)
	require.NoError(t, err)

	// TEST A: Exact Warehouse ZMU query
	t.Run("exact warehouse ZMU resolves owning inventory variant", func(t *testing.T) {
		resp, err := service.ListAdminInventory(ctx, zmuWarehouse, "", "", false, 50, 0)
		require.NoError(t, err)

		// Unit Context must be populated
		require.NotNil(t, resp.UnitContext, "UnitContext must be populated for exact ZMU")
		assert.Equal(t, zmuWarehouse, resp.UnitContext.UnitCode)
		assert.Equal(t, "warehouse", resp.UnitContext.Status)
		assert.Equal(t, "На складе", resp.UnitContext.StatusLabel)
		assert.Equal(t, "Dev Premium Wool Coat", resp.UnitContext.ProductTitle)
		assert.Equal(t, var1ID, resp.UnitContext.VariantID)

		// Owning aggregate inventory row must be returned
		require.Len(t, resp.Items, 1)
		assert.Equal(t, var1ID, resp.Items[0].ProductVariantID)
		assert.Equal(t, 15, resp.Items[0].TotalStock)
		assert.Equal(t, 2, resp.Items[0].ReservedStock)
		assert.Equal(t, 13, resp.Items[0].AvailableStock)
	})

	// TEST B: Exact Shipped ZMU query (valid physical history)
	t.Run("exact shipped ZMU resolves owning inventory variant and status", func(t *testing.T) {
		resp, err := service.ListAdminInventory(ctx, zmuShipped, "", "", false, 50, 0)
		require.NoError(t, err)

		require.NotNil(t, resp.UnitContext)
		assert.Equal(t, zmuShipped, resp.UnitContext.UnitCode)
		assert.Equal(t, "shipped", resp.UnitContext.Status)
		assert.Equal(t, "Отгружен", resp.UnitContext.StatusLabel)
		assert.Equal(t, "Dev Premium Wool Coat", resp.UnitContext.ProductTitle)

		require.Len(t, resp.Items, 1)
		assert.Equal(t, var1ID, resp.Items[0].ProductVariantID)
	})

	// TEST C: Exact Damaged ZMU query
	t.Run("exact damaged ZMU resolves owning inventory variant and status", func(t *testing.T) {
		resp, err := service.ListAdminInventory(ctx, zmuDamaged, "", "", false, 50, 0)
		require.NoError(t, err)

		require.NotNil(t, resp.UnitContext)
		assert.Equal(t, zmuDamaged, resp.UnitContext.UnitCode)
		assert.Equal(t, "damaged", resp.UnitContext.Status)
		assert.Equal(t, "Поврежден", resp.UnitContext.StatusLabel)

		require.Len(t, resp.Items, 1)
		assert.Equal(t, var1ID, resp.Items[0].ProductVariantID)
	})

	// TEST D: Unknown ZMU query -> gracefully empty
	t.Run("unknown ZMU returns empty without error", func(t *testing.T) {
		resp, err := service.ListAdminInventory(ctx, "ZMU-NONEXISTENT9999", "", "", false, 50, 0)
		require.NoError(t, err)

		assert.Nil(t, resp.UnitContext, "UnitContext must be nil for non-existent ZMU")
		assert.Empty(t, resp.Items)
		assert.Equal(t, 0, resp.TotalCount)
	})

	// TEST E: Case-insensitive lowercase ZMU query
	t.Run("lowercase ZMU input resolves canonical unit", func(t *testing.T) {
		resp, err := service.ListAdminInventory(ctx, strings.ToLower(zmuWarehouse), "", "", false, 50, 0)
		require.NoError(t, err)

		require.NotNil(t, resp.UnitContext)
		assert.Equal(t, zmuWarehouse, resp.UnitContext.UnitCode)
		assert.Equal(t, "На складе", resp.UnitContext.StatusLabel)
		require.Len(t, resp.Items, 1)
	})

	// TEST F: Existing Product Title Search Unchanged
	t.Run("existing title search is unchanged", func(t *testing.T) {
		resp, err := service.ListAdminInventory(ctx, "Silk Scarf", "", "", false, 50, 0)
		require.NoError(t, err)

		assert.Nil(t, resp.UnitContext)
		require.Len(t, resp.Items, 1)
		assert.Equal(t, "Dev Silk Scarf", resp.Items[0].ProductTitle)
		assert.Equal(t, var2ID, resp.Items[0].ProductVariantID)
	})

	// TEST G: Existing SKU Search Unchanged
	t.Run("existing SKU search is unchanged", func(t *testing.T) {
		resp, err := service.ListAdminInventory(ctx, "SKU-COAT", "", "", false, 50, 0)
		require.NoError(t, err)

		assert.Nil(t, resp.UnitContext)
		require.Len(t, resp.Items, 1)
		assert.Equal(t, "Dev Premium Wool Coat", resp.Items[0].ProductTitle)
	})

	// TEST H: Verify no inventory mutation happened
	t.Run("no inventory or stock mutation occurs during searches", func(t *testing.T) {
		var currentTotal, currentReserved int
		err := testDB.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE id = $1", inv1ID).
			Scan(&currentTotal, &currentReserved)
		require.NoError(t, err)
		assert.Equal(t, 15, currentTotal, "Total stock must remain untouched")
		assert.Equal(t, 2, currentReserved, "Reserved stock must remain untouched")

		var currentUnitStatus string
		err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE unit_code = $1", zmuShipped).
			Scan(&currentUnitStatus)
		require.NoError(t, err)
		assert.Equal(t, "shipped", currentUnitStatus, "Unit status must remain exact")
	})

	// Safety cleanup: delete ONLY rows created by this exact test run in strict reverse foreign-key dependency order
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx, "DELETE FROM inventory_units WHERE id = ANY($1)", []uuid.UUID{unitWarehouseID, unitShippedID, unitDamagedID, unitWrittenOffID})
		_, _ = testDB.Exec(ctx, "DELETE FROM seller_supply_items WHERE id = $1", supplyItemID)
		_, _ = testDB.Exec(ctx, "DELETE FROM seller_supplies WHERE id = $1", supplyID)
		_, _ = testDB.Exec(ctx, "DELETE FROM inventory_items WHERE id = ANY($1)", []uuid.UUID{inv1ID, inv2ID})
		_, _ = testDB.Exec(ctx, "DELETE FROM product_variants WHERE id = ANY($1)", []uuid.UUID{var1ID, var2ID})
		_, _ = testDB.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", []uuid.UUID{prod1ID, prod2ID})
		_, _ = testDB.Exec(ctx, "DELETE FROM seller_users WHERE id = $1", sellerUserRoleID)
		_, _ = testDB.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = testDB.Exec(ctx, "DELETE FROM users WHERE id = $1", sellerUserID)
	})
}
