package selleranalytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSellerAnalytics_InventoryIdentityAndStockConsistency(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer db.Close()

	verifyTestDBSafety(t, db)

	repo := NewRepository(db)
	svc := NewService(repo)
	router := setupCohortTestRouter(svc, repo)

	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc).UTC()
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC()

	sellerID, userID := insertSellerUser(t, db)
	sellerForeign, _ := insertSellerUser(t, db)

	var colorBlackID, colorWhiteID uuid.UUID
	err = db.QueryRow(ctx, "SELECT id FROM colors WHERE name_ru = 'Черный' LIMIT 1").Scan(&colorBlackID)
	if err != nil {
		colorBlackID = uuid.New()
		_, err = db.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, 'BLK', 'Черный', '#000000', true)", colorBlackID)
		require.NoError(t, err)
	}

	err = db.QueryRow(ctx, "SELECT id FROM colors WHERE name_ru = 'Белый' LIMIT 1").Scan(&colorWhiteID)
	if err != nil {
		colorWhiteID = uuid.New()
		_, err = db.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, 'WHT', 'Белый', '#FFFFFF', true)", colorWhiteID)
		require.NoError(t, err)
	}

	var sizeMID, sizeLID uuid.UUID
	err = db.QueryRow(ctx, "SELECT id FROM size_values WHERE value = 'M' LIMIT 1").Scan(&sizeMID)
	if err != nil {
		sizeMID = uuid.New()
		var gridID uuid.UUID
		err = db.QueryRow(ctx, "SELECT id FROM size_grids LIMIT 1").Scan(&gridID)
		if err != nil {
			gridID = uuid.New()
			_, err = db.Exec(ctx, "INSERT INTO size_grids (id, name) VALUES ($1, 'Standard')", gridID)
			require.NoError(t, err)
		}
		_, err = db.Exec(ctx, "INSERT INTO size_values (id, size_grid_id, value, sort_order) VALUES ($1, $2, 'M', 1)", sizeMID, gridID)
		require.NoError(t, err)
	}

	err = db.QueryRow(ctx, "SELECT id FROM size_values WHERE value = 'L' LIMIT 1").Scan(&sizeLID)
	if err != nil {
		sizeLID = uuid.New()
		var gridID uuid.UUID
		_ = db.QueryRow(ctx, "SELECT id FROM size_grids LIMIT 1").Scan(&gridID)
		_, err = db.Exec(ctx, "INSERT INTO size_values (id, size_grid_id, value, sort_order) VALUES ($1, $2, 'L', 2)", sizeLID, gridID)
		require.NoError(t, err)
	}

	// Create Product
	var catID uuid.UUID
	err = db.QueryRow(ctx, "SELECT id FROM categories LIMIT 1").Scan(&catID)
	if err != nil {
		catID = uuid.New()
		_, err = db.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Test Cat', $2)", catID, catID.String())
		require.NoError(t, err)
	}

	pID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents)
		VALUES ($1, $2, $3, 'Шелковое платье ZAMK', $4, 'published', 250000)`,
		pID, sellerID, catID, pID.String())
	require.NoError(t, err)

	// Scenario A: Modern variant with canonical size/color and seller SKU
	vA := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, size_value_id, color_id, price_cents, is_active)
		VALUES ($1, $2, '', 'DRS-SLK-BLK-M', '', '', $3, $4, 250000, true)`,
		vA, pID, sizeMID, colorBlackID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 25, 2)`,
		uuid.New(), pID, vA, sellerID)
	require.NoError(t, err)

	// Scenario B: Modern unsold variant with NULL legacy metadata
	vB := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, size_value_id, color_id, price_cents, is_active)
		VALUES ($1, $2, NULL, 'DRS-SLK-WHT-L', NULL, NULL, $3, $4, 250000, true)`,
		vB, pID, sizeLID, colorWhiteID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 2, 0)`, // available = 2 (low stock without sales)
		uuid.New(), pID, vB, sellerID)
	require.NoError(t, err)

	// Scenario C: Legacy variant with older text metadata and legacy sku
	vC := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, size_value_id, color_id, price_cents, is_active)
		VALUES ($1, $2, 'LEGACY-SKU-1', '', 'S', 'Красный', NULL, NULL, 250000, true)`,
		vC, pID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 15, 0)`,
		uuid.New(), pID, vC, sellerID)
	require.NoError(t, err)

	// Scenario D: Variant missing all metadata
	vD := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, size_value_id, color_id, price_cents, is_active)
		VALUES ($1, $2, '', '', '', '', NULL, NULL, 250000, true)`,
		vD, pID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 0)`,
		uuid.New(), pID, vD, sellerID)
	require.NoError(t, err)

	// Scenario H: Zero available stock variant
	vH := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, 'SKU-ZERO', 'SKU-ZERO', 'XL', 'Черный', 250000, true)`,
		vH, pID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 0, 0)`,
		uuid.New(), pID, vH, sellerID)
	require.NoError(t, err)

	// Scenario J: Low available stock WITH positive sales (10 sales in 30 days, available = 2 -> dos = 6.0 <= 7)
	vJ := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, 'SKU-LOW-ACTIVE', 'SKU-LOW-ACTIVE', 'XS', 'Черный', 250000, true)`,
		vJ, pID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 2, 0)`,
		uuid.New(), pID, vJ, sellerID)
	require.NoError(t, err)

	insertDeliveredOrderWithItem(t, db, sellerID, pID, vJ, 250000, 10, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

	// Scenario K: Inbound supply that must NOT increase available stock
	supplyID := uuid.New()
	supplyNum := "SUPP-" + uuid.New().String()[:8]
	_, err = db.Exec(ctx, `INSERT INTO seller_supplies (id, seller_id, supply_number, status, handoff_method, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped_by_seller', 'carrier_delivery', now(), now())`,
		supplyID, sellerID, supplyNum)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 19, 0, now(), now())`,
		uuid.New(), supplyID, vA)
	require.NoError(t, err)

	// Scenario B (Phase 4B.2): High physical stock, urgent forecast (Available 30, velocity 50/day)
	vHighStockFast := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, 'SKU-FAST', 'SKU-FAST', 'M', 'Черный', 250000, true)`,
		vHighStockFast, pID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 30, 0)`, // available = 30
		uuid.New(), pID, vHighStockFast, sellerID)
	require.NoError(t, err)

	// In 30 days, 1500 units sold -> 50 units/day -> daysOfStock = 30 / 50 = 0.6 days <= 7
	insertDeliveredOrderWithItem(t, db, sellerID, pID, vHighStockFast, 250000, 1500, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())

	// Scenario F (Phase 4B.2): Reserved inventory (On hand 10, Reserved 3 -> Available 7)
	vReserved := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, 'SKU-RES-7', 'SKU-RES-7', 'L', 'Черный', 250000, true)`,
		vReserved, pID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 10, 3)`, // on_hand = 10, reserved = 3, available = 7
		uuid.New(), pID, vReserved, sellerID)
	require.NoError(t, err)

	// Foreign Seller product and inventory (Scenario J)
	pForeign := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents)
		VALUES ($1, $2, $3, 'Foreign Secret Product', $4, 'published', 100000)`,
		pForeign, sellerForeign, catID, pForeign.String())
	require.NoError(t, err)

	vForeign := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, 'FOREIGN-SECRET-SKU', 'FOREIGN-SECRET-SKU', 'M', 'Черный', 100000, true)`,
		vForeign, pForeign)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 50, 0)`,
		uuid.New(), pForeign, vForeign, sellerForeign)
	require.NoError(t, err)

	// Scenario K (Phase 4B.2): Malformed cross-seller inventory reference (references foreign variant)
	// Must fail-closed without leaking foreign title, SKU or attributes
	vForeignCross := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, 'FOREIGN-LEAK-SKU', 'FOREIGN-LEAK-SKU', 'M', 'Черный', 100000, true)`,
		vForeignCross, pForeign)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, 5, 0)`,
		uuid.New(), pID, vForeignCross, sellerID)
	require.NoError(t, err)

	// Execute API call for Seller
	req := httptest.NewRequest("GET", "/inventory?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339), nil)
	req.Header.Set("X-Test-User-ID", userID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	resp, err := svc.GetInventory(ctx, sellerID, from, to)
	require.NoError(t, err)

	// Total 9 items belonging to Seller A (including 1 malformed inventory row)
	require.Len(t, resp.Items, 9, "Must contain exactly the 9 inventory records belonging to Seller")

	itemMap := make(map[string]InventoryRow)
	for _, item := range resp.Items {
		itemMap[item.VariantID] = item
	}

	// 1. Scenario A: Available 2, no sales -> both independent signals TRUE
	rowB, ok := itemMap[vB.String()]
	require.True(t, ok)
	assert.Equal(t, "Шелковое платье ZAMK", rowB.ProductTitle)
	assert.Equal(t, "DRS-SLK-WHT-L", rowB.SKU)
	assert.Equal(t, "L Белый", rowB.DisplayName)
	assert.Equal(t, 2, rowB.Available)
	assert.Equal(t, 0, rowB.UnitsSold)
	assert.True(t, rowB.IsPhysicalLowStock, "Scenario A: Available 2 <= 10 must have IsPhysicalLowStock = true")
	assert.True(t, rowB.HasNoSalesInPeriod, "Scenario A: 0 units sold must have HasNoSalesInPeriod = true")
	assert.False(t, rowB.IsForecastLowCoverage, "Scenario A: no sales -> forecast low coverage false")
	assert.Nil(t, rowB.DaysOfStock, "Scenario A: forecast unavailable for zero sales")
	assert.Equal(t, "no_sales", rowB.StockCoverageState)

	// 2. Scenario B: Available 30, velocity 50/day -> forecast low coverage TRUE, physical low stock FALSE
	rowFast, ok := itemMap[vHighStockFast.String()]
	require.True(t, ok)
	assert.Equal(t, 30, rowFast.Available)
	assert.Equal(t, 1500, rowFast.UnitsSold)
	assert.InDelta(t, 50.0, rowFast.SalesVelocity, 0.01)
	require.NotNil(t, rowFast.DaysOfStock)
	assert.InDelta(t, 0.6, *rowFast.DaysOfStock, 0.01)
	assert.False(t, rowFast.IsPhysicalLowStock, "Scenario B: Available 30 > 10 must have IsPhysicalLowStock = false")
	assert.False(t, rowFast.HasNoSalesInPeriod, "Scenario B: 1500 sales -> HasNoSalesInPeriod = false")
	assert.True(t, rowFast.IsForecastLowCoverage, "Scenario B: 0.6 days <= 7 must have IsForecastLowCoverage = true")

	// 3. Scenario C: Available 2 <= 10, days of stock 6 <= 7 -> both stock warnings TRUE
	rowJ, ok := itemMap[vJ.String()]
	require.True(t, ok)
	assert.Equal(t, 2, rowJ.Available)
	assert.Equal(t, 10, rowJ.UnitsSold)
	assert.InDelta(t, 10.0/30.0, rowJ.SalesVelocity, 0.001)
	require.NotNil(t, rowJ.DaysOfStock)
	assert.InDelta(t, 6.0, *rowJ.DaysOfStock, 0.01)
	assert.True(t, rowJ.IsPhysicalLowStock, "Scenario C: Available 2 <= 10 -> IsPhysicalLowStock = true")
	assert.True(t, rowJ.IsForecastLowCoverage, "Scenario C: 6.0 days <= 7 -> IsForecastLowCoverage = true")
	assert.False(t, rowJ.HasNoSalesInPeriod, "Scenario C: 10 sales -> HasNoSalesInPeriod = false")

	// 4. Scenario D: Available 0 -> out-of-stock TRUE and physical low stock FALSE
	rowH, ok := itemMap[vH.String()]
	require.True(t, ok)
	assert.Equal(t, 0, rowH.Available)
	assert.Equal(t, "out_of_stock", rowH.StockCoverageState)
	assert.False(t, rowH.IsPhysicalLowStock, "Scenario D: 0 stock is out-of-stock, NOT physical low stock")
	assert.False(t, rowH.IsForecastLowCoverage, "Scenario D: 0 stock cannot have forecast low coverage")
	assert.Nil(t, rowH.DaysOfStock, "Scenario D: 0 stock has nil DaysOfStock to prevent misleading 0 days forecast")

	// 5. Scenario E: Available 23 (>10), no sales -> no-sales TRUE without low-stock warning
	rowA, ok := itemMap[vA.String()]
	require.True(t, ok)
	assert.Equal(t, 23, rowA.Available)
	assert.Equal(t, 0, rowA.UnitsSold)
	assert.False(t, rowA.IsPhysicalLowStock, "Scenario E: Available 23 > 10 -> IsPhysicalLowStock = false")
	assert.True(t, rowA.HasNoSalesInPeriod, "Scenario E: 0 sales -> HasNoSalesInPeriod = true")
	assert.Nil(t, rowA.DaysOfStock, "Scenario E: 0 sales -> nil DaysOfStock")

	// 6. Scenario F: On hand 10, reserved 3 -> available 7, physical low stock TRUE
	rowRes, ok := itemMap[vReserved.String()]
	require.True(t, ok)
	assert.Equal(t, 10, rowRes.OnHand)
	assert.Equal(t, 3, rowRes.Reserved)
	assert.Equal(t, 7, rowRes.Available)
	assert.True(t, rowRes.IsPhysicalLowStock, "Scenario F: Available 7 <= 10 -> IsPhysicalLowStock = true")

	// 7. Scenario G: Inbound +19 does not increase available
	assert.Equal(t, 19, rowA.Inbound, "Inbound is 19")
	assert.Equal(t, 23, rowA.Available, "Available remains 23 (25 on hand - 2 reserved), not 42")

	// 8. Scenario H: Changing 7-, 30- and 90-day periods does not change physical low stock for an unchanged warehouse snapshot
	from7 := to.Add(-7 * 24 * time.Hour)
	resp7, err := svc.GetInventory(ctx, sellerID, from7, to)
	require.NoError(t, err)
	from90 := to.Add(-90 * 24 * time.Hour)
	resp90, err := svc.GetInventory(ctx, sellerID, from90, to)
	require.NoError(t, err)

	itemMap7 := make(map[string]InventoryRow)
	for _, it := range resp7.Items { itemMap7[it.VariantID] = it }
	itemMap90 := make(map[string]InventoryRow)
	for _, it := range resp90.Items { itemMap90[it.VariantID] = it }

	for _, it := range resp.Items {
		it7 := itemMap7[it.VariantID]
		it90 := itemMap90[it.VariantID]
		assert.Equal(t, it.Available, it7.Available, "Period change must not alter physical available stock")
		assert.Equal(t, it.Available, it90.Available, "Period change must not alter physical available stock")
		assert.Equal(t, it.IsPhysicalLowStock, it7.IsPhysicalLowStock, "Period change must not alter IsPhysicalLowStock")
		assert.Equal(t, it.IsPhysicalLowStock, it90.IsPhysicalLowStock, "Period change must not alter IsPhysicalLowStock")
	}

	// 9. Scenario I: No-sales and physical-low counts can overlap without inflating stock unit totals
	physLowCount := 0
	noSalesCount := 0
	totalAvailableUnits := 0
	for _, it := range resp.Items {
		totalAvailableUnits += it.Available
		if it.IsPhysicalLowStock { physLowCount++ }
		if it.Available > 0 && it.HasNoSalesInPeriod { noSalesCount++ }
	}
	assert.True(t, physLowCount > 0)
	assert.True(t, noSalesCount > 0)
	// vB is counted in BOTH physLowCount and noSalesCount:
	assert.True(t, rowB.IsPhysicalLowStock && rowB.HasNoSalesInPeriod, "vB overlaps in both counts")
	assert.Equal(t, 23 + 2 + 15 + 10 + 0 + 2 + 30 + 7 + 5, totalAvailableUnits, "Total stock units is exact sum of available, not inflated")

	// 10. Scenario J: Seller A cannot receive Seller B's stock, SKU or variant metadata
	// Note: vForeign in inventory_items for sellerID was added as malformed cross-ref; verify Seller B's own inventory row (50 units) is NOT visible
	for _, it := range resp.Items {
		assert.NotEqual(t, 50, it.OnHand, "Seller B's 50-stock row must NOT be visible to Seller A")
	}

	// 11. Scenario K: Malformed cross-seller inventory/catalog reference cannot leak foreign metadata
	rowCross := itemMap[vForeignCross.String()]
	assert.Equal(t, "Шелковое платье ZAMK", rowCross.ProductTitle, "Product title belongs to Seller A")
	assert.Equal(t, "", rowCross.SKU, "Foreign SKU must NOT leak to Seller A")
	assert.Equal(t, "", rowCross.DisplayName, "Foreign attributes must NOT leak to Seller A")
	assert.NotEqual(t, "FOREIGN-LEAK-SKU", rowCross.SKU)

	// 12. Scenario L & M: Legacy and archived variants remain displayed safely with canonical resolution
	rowC, ok := itemMap[vC.String()]
	require.True(t, ok)
	assert.Equal(t, "Шелковое платье ZAMK", rowC.ProductTitle)
	assert.Equal(t, "LEGACY-SKU-1", rowC.SKU)
	assert.Equal(t, "S Красный", rowC.DisplayName)
	assert.Equal(t, 15, rowC.Available)

	rowD, ok := itemMap[vD.String()]
	require.True(t, ok)
	assert.Equal(t, "Шелковое платье ZAMK", rowD.ProductTitle)
	assert.Equal(t, "", rowD.SKU)
}
