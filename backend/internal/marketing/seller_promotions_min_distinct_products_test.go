package marketing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// Helper to create an additional variant for an EXISTING product in an order.
func createAdditionalVariantForProduct(t *testing.T, client *postgres.Client, orderID, sellerID, prodID uuid.UUID, priceCents int64, quantity int) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	variantID := uuid.New()
	barcode := "BARCODE-" + variantID.String()[:8]
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, now(), now())
	`, variantID, prodID, "SKU-"+variantID.String()[:8], "SSKU-"+variantID.String()[:8], barcode, priceCents)
	require.NoError(t, err)

	var fulID uuid.UUID
	err = client.Pool.QueryRow(ctx, `SELECT id FROM order_fulfillments WHERE order_id = $1 AND seller_id = $2`, orderID, sellerID).Scan(&fulID)
	require.NoError(t, err)

	itemID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item Variant', 'slug-var', $7, $8, $9, now())
	`, itemID, orderID, fulID, prodID, variantID, sellerID, priceCents, quantity, priceCents*int64(quantity))
	require.NoError(t, err)

	invID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 100, 10, now(), now())
	`, invID, prodID, variantID, sellerID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, quantity, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, 'active', now() + interval '30 minutes', now())
	`, resID, invID, prodID, variantID, quantity)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_reservations (id, order_id, reservation_id, created_at)
		VALUES ($1, $2, $3, now())
	`, uuid.New(), orderID, resID)
	require.NoError(t, err)

	return itemID, variantID
}

// ============================================================================
// PROMO.2F BACKEND TEST MATRIX: A through AB
// ============================================================================

// Case A: Migration 000103 UP applied cleanly and check constraint exists.
func TestMinDistinct_CaseA_MigrationUpAndConstraints(t *testing.T) {
	client, _, _ := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	// Verify column exists
	var exists bool
	err := client.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'promo_codes' AND column_name = 'min_distinct_products'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "column min_distinct_products must exist")

	// Verify check constraint exists
	var chkExists bool
	err = client.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.table_constraints
			WHERE table_name = 'promo_codes' AND constraint_name = 'chk_promo_codes_min_distinct_products'
		)
	`).Scan(&chkExists)
	require.NoError(t, err)
	assert.True(t, chkExists, "constraint chk_promo_codes_min_distinct_products must exist")

	// Attempt inserting min_distinct_products = 0 directly via SQL -> must violate check constraint
	sellerID := createTestSeller(t, client)
	campID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, seller_id, title, funding_mode, status, discount_type, seller_discount_bps, created_at, updated_at)
		VALUES ($1, $2, 'Test Camp', 'seller', 'active', 'percent', 1000, now(), now())
	`, campID, sellerID)
	require.NoError(t, err)
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, []uuid.UUID{campID})

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, product_scope, min_distinct_products, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'percent', 1000, true, 'ENTIRE_STORE', 0, now(), now())
	`, uuid.New(), campID, sellerID, uniqueCode("CHK0"))
	assert.Error(t, err, "inserting min_distinct_products = 0 must fail check constraint")

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, product_scope, min_distinct_products, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'percent', 1000, true, 'ENTIRE_STORE', -1, now(), now())
	`, uuid.New(), campID, sellerID, uniqueCode("CHKN"))
	assert.Error(t, err, "inserting min_distinct_products = -1 must fail check constraint")
}

// Case B & C: Migration 000103 DOWN rolls back and re-UP works cleanly.
func TestMinDistinct_CaseBC_MigrationReversibility(t *testing.T) {
	client, _, _ := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	root := findRepoRoot()
	downPath := filepath.Join(root, "migrations", "000103_add_min_distinct_products_to_promo_codes.down.sql")
	upPath := filepath.Join(root, "migrations", "000103_add_min_distinct_products_to_promo_codes.up.sql")

	downContent, err := os.ReadFile(downPath)
	require.NoError(t, err)
	upContent, err := os.ReadFile(upPath)
	require.NoError(t, err)

	// Execute DOWN
	_, err = client.Pool.Exec(ctx, string(downContent))
	require.NoError(t, err, "migration down must execute cleanly")

	// Verify column dropped
	var hasCol bool
	err = client.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'promo_codes' AND column_name = 'min_distinct_products'
		)
	`).Scan(&hasCol)
	require.NoError(t, err)
	assert.False(t, hasCol, "min_distinct_products must not exist after down migration")

	// Re-execute UP
	_, err = client.Pool.Exec(ctx, string(upContent))
	require.NoError(t, err, "migration up must re-execute cleanly")

	err = client.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'promo_codes' AND column_name = 'min_distinct_products'
		)
	`).Scan(&hasCol)
	require.NoError(t, err)
	assert.True(t, hasCol, "min_distinct_products must exist after re-up migration")
}

// Case D: NULL min_distinct_products preserves legacy behavior (passes with 1 product).
func TestMinDistinct_CaseD_NullPreservesLegacyBehavior(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-d")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("MDPD")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: nil,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)
	assert.Nil(t, res.MinDistinctProducts)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case E: Exact minimum passes (MinDistinctProducts = 1 with 1 product, MinDistinctProducts = 2 with 2 products).
func TestMinDistinct_CaseE_ExactMinimumPasses(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-e")
	orderID := createTestOrder(t, client, userID)

	// Create 2 distinct products
	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 1)
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 60000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPE")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 60000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(11000), calc.TotalSellerDiscountCents) // (50000 + 60000) * 10%
		return nil
	})
	require.NoError(t, err)
}

// Case F: Below minimum fails with ErrPromoMinDistinctProducts.
func TestMinDistinct_CaseF_BelowMinimumFails(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-f")
	orderID := createTestOrder(t, client, userID)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPF")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts)
		return nil
	})
	require.NoError(t, err)
}

// Case G: Product variants of the SAME product count as 1 distinct product!
func TestMinDistinct_CaseG_ProductVariantsDoNotDefineDistinctness(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-g")
	orderID := createTestOrder(t, client, userID)

	// Product A / Variant 1 (qty 2)
	itemID1, prodID, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 2)
	// Product A / Variant 2 (qty 3) - same prodID!
	itemID2, varID2 := createAdditionalVariantForProduct(t, client, orderID, sellerID, prodID, 50000, 3)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPG")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Items list contains 2 variants of the SAME product, total qty = 5, distinct products = 1
	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 2},
		{OrderItemID: itemID2, ProductID: prodID, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 3},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts, "2 variants of the same product must count as 1 distinct product and fail minDistinct=2")
		return nil
	})
	require.NoError(t, err)
}

// Case H: Different products pass regardless of variants (Product A / Variant 1 + Product B / Variant 1).
func TestMinDistinct_CaseH_DifferentProductsPass(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-h")
	orderID := createTestOrder(t, client, userID)

	// Product A / Variant 1
	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 40000, 1)
	// Product B / Variant 1
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 60000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPH")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 40000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 60000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case I: Excluded products do NOT count toward distinct products.
func TestMinDistinct_CaseI_ExcludedProductsDoNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-i")
	orderID := createTestOrder(t, client, userID)

	// Product A and Product B
	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 1)
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPI")
	scopeEntire := marketing.ProductScopeEntireStore
	// Product A is excluded
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
		ProductScope:        &scopeEntire,
		ExcludedProductIDs:  []uuid.UUID{prodID1},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts, "excluded product A must not count toward distinct products")
		return nil
	})
	require.NoError(t, err)
}

// Case J: Excluded categories do NOT count toward distinct products.
func TestMinDistinct_CaseJ_ExcludedCategoriesDoNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-j")
	orderID := createTestOrder(t, client, userID)

	cat1 := uuid.New()
	_, err := client.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat1', $2, now(), now())`, cat1, uuid.New().String())
	require.NoError(t, err)

	cat2 := uuid.New()
	_, err = client.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat2', $2, now(), now())`, cat2, uuid.New().String())
	require.NoError(t, err)

	// Product A in cat1, Product B in cat2
	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, cat1, 50000, 1)
	_, _ = client.Pool.Exec(ctx, `UPDATE products SET category_id = $1 WHERE id = $2`, cat1, prodID1)

	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, cat2, 50000, 1)
	_, _ = client.Pool.Exec(ctx, `UPDATE products SET category_id = $1 WHERE id = $2`, cat2, prodID2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	minDistinct := 2
	code := uniqueCode("MDPJ")
	scopeEntire := marketing.ProductScopeEntireStore
	// Exclude category 1
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
		ProductScope:        &scopeEntire,
		ExcludedCategoryIDs: []uuid.UUID{cat1},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts, "product in excluded category must not count toward distinct products")
		return nil
	})
	require.NoError(t, err)
}

// Case K: Out-of-scope products (SELECTED_PRODUCTS) do NOT count toward distinct products.
func TestMinDistinct_CaseK_OutOfScopeProductsDoNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-k")
	orderID := createTestOrder(t, client, userID)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 1)
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPK")
	scopeSelected := marketing.ProductScopeSelectedProducts
	// Target ONLY prodID1
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
		ProductScope:        &scopeSelected,
		IncludedProductIDs:  []uuid.UUID{prodID1},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts, "out-of-scope prodID2 must not count toward distinct products")
		return nil
	})
	require.NoError(t, err)
}

// Case L: Foreign seller products in multi-seller order do NOT count.
func TestMinDistinct_CaseL_ForeignSellerProductsDoNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	seller1 := createTestSeller(t, client)
	seller2 := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-l")
	orderID := createTestOrder(t, client, userID)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, seller1, uuid.Nil, 50000, 1)
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, seller2, uuid.Nil, 50000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{seller1, seller2}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPL")
	res, err := svc.CreateSellerPromotion(ctx, seller1, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: seller1, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: seller2, BaseUnitPriceCents: 50000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts, "foreign seller items must not count toward seller1 promo distinct count")
		return nil
	})
	require.NoError(t, err)
}

// Case M: Combined rules (MinDistinctProducts + MinEligibleQuantity).
func TestMinDistinct_CaseM_CombinedDistinctAndQuantity(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-m")
	orderID := createTestOrder(t, client, userID)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 30000, 2)
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 30000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	minQty := 4
	code := uniqueCode("MDPM")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Subtest 1: 1 product with qty 4 -> fails minDistinct
	itemsSingleProd := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 30000, Quantity: 4},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, itemsSingleProd, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts)
		return nil
	})
	require.NoError(t, err)

	// Subtest 2: 2 products with qty 1 each (total 2) -> fails minQty
	itemsLowQty := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 30000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 30000, Quantity: 1},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, itemsLowQty, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)

	// Subtest 3: 2 products with qty 2 each (total 4) -> PASSES!
	itemsPass := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 30000, Quantity: 2},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 30000, Quantity: 2},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, itemsPass, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(12000), calc.TotalSellerDiscountCents) // (60000 + 60000) * 10%
		return nil
	})
	require.NoError(t, err)
}

// Case N: Combined rules (MinDistinctProducts + MinOrderSubtotal).
func TestMinDistinct_CaseN_CombinedDistinctAndSubtotal(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-n")
	orderID := createTestOrder(t, client, userID)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 40000, 1)
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 40000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	minSubtotal := int64(100000) // 1000 ₽
	code := uniqueCode("MDPN")
	promoRes, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinDistinctProducts:   &minDistinct,
		MinOrderSubtotalCents: minSubtotal,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, promoRes.CampaignID)

	// 2 products, but subtotal = 40000 + 40000 = 80000 < 100000 -> fails min order subtotal
	itemsLowSubtotal := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 40000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 40000, Quantity: 1},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, itemsLowSubtotal, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinSubtotal)
		return nil
	})
	require.NoError(t, err)

	// 2 products, subtotal = 60000 + 60000 = 120000 >= 100000 -> PASSES!
	itemsPass := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 60000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 60000, Quantity: 1},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, itemsPass, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(12000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case O: Combined rules (MinDistinctProducts + Audience REPEAT_CUSTOMERS).
func TestMinDistinct_CaseO_CombinedDistinctAndAudience(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userNew := createTestUser(t, client, "usr-mdp-onew")
	userRepeat := createTestUser(t, client, "usr-mdp-orep")

	// Make userRepeat a repeat customer via paid order with succeeded payment
	pastOrderID := createOrderWithStatus(t, client, userRepeat, "delivered")
	createPaymentWithStatus(t, client, pastOrderID, "succeeded")

	orderIDNew := createTestOrder(t, client, userNew)
	orderIDRep := createTestOrder(t, client, userRepeat)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderIDRep, sellerID, uuid.Nil, 50000, 1)
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderIDRep, sellerID, uuid.Nil, 50000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupAudienceFixtures(client, []uuid.UUID{userNew, userRepeat}, []uuid.UUID{pastOrderID, orderIDNew, orderIDRep}, campaignIDs, []uuid.UUID{sellerID})

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPO")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
		AudienceType:        marketing.AudienceRepeatCustomer,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// New user with 2 products -> fails audience check
	items2 := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userNew, code, items2, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoRepeatCustomerRequired)
		return nil
	})
	require.NoError(t, err)

	// Repeat user with 1 product -> fails minDistinct
	items1 := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userRepeat, code, items1, 1000, time.Now().UTC())
		assert.ErrorIs(t, err, marketing.ErrPromoMinDistinctProducts)
		return nil
	})
	require.NoError(t, err)

	// Repeat user with 2 products -> PASSES!
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userRepeat, code, items2, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case P: Pure gating economics and Hare-Niemeyer allocations remain unperturbed.
func TestMinDistinct_CaseP_PureGatingAllocations(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-p")
	orderID := createTestOrder(t, client, userID)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 10000, 1) // 100 ₽
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 20000, 1) // 200 ₽

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	code := uniqueCode("MDPP")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000, // 10%
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 10000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 20000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(3000), calc.TotalSellerDiscountCents) // 30 ₽ total
		// Allocations: 10% of 10000 = 1000, 10% of 20000 = 2000
		assert.Len(t, calc.PromotedLines, 2)
		assert.Equal(t, int64(1000), calc.PromotedLines[0].TotalSellerDiscountCents)
		assert.Equal(t, int64(2000), calc.PromotedLines[1].TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case Q: Max discount limit operates normally with MinDistinctProducts.
func TestMinDistinct_CaseQ_MaxDiscountUnaffected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mdp-q")
	orderID := createTestOrder(t, client, userID)

	itemID1, prodID1, varID1 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 10000, 1) // 100 ₽
	itemID2, prodID2, varID2 := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 10000, 1) // 100 ₽

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minDistinct := 2
	maxDisc := int64(5000) // 50 ₽ max discount
	code := uniqueCode("MDPQ")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                 code,
		DiscountType:         marketing.DiscountTypePercent,
		DiscountValueBps:     5000, // 50%
		MaxDiscountCents:     &maxDisc,
		MinDistinctProducts:  &minDistinct,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID1, ProductID: prodID1, ProductVariantID: varID1, SellerID: sellerID, BaseUnitPriceCents: 10000, Quantity: 1},
		{OrderItemID: itemID2, ProductID: prodID2, ProductVariantID: varID2, SellerID: sellerID, BaseUnitPriceCents: 10000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		// 50% of 20000 = 10000, capped at maxDiscount 5000
		assert.Equal(t, int64(5000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case R: Service-level creation validation (<= 0 rejected).
func TestMinDistinct_CaseR_ServiceCreateValidation(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	ctx := context.Background()

	zero := 0
	_, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                uniqueCode("VAL0"),
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &zero,
	})
	assert.ErrorIs(t, err, marketing.ErrInvalidMinDistinctProducts)

	negative := -3
	_, err = svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                uniqueCode("VALN"),
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &negative,
	})
	assert.ErrorIs(t, err, marketing.ErrInvalidMinDistinctProducts)
}

// Case S: HTTP Handler Create Validation (400 invalid_min_distinct_products).
func TestMinDistinct_CaseS_HTTPHandlerCreateValidation(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	_, r := setupMarketingHTTPHandler(svc)

	zero := 0
	body, _ := json.Marshal(map[string]interface{}{
		"code":                uniqueCode("HVAL0"),
		"discountType":        "percent",
		"discountValueBps":    1000,
		"minDistinctProducts": zero,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/seller/promotions", bytes.NewReader(body))
	req = req.WithContext(newTestContextWithSeller(sellerID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.Equal(t, "invalid_min_distinct_products", errResp["code"])
}

// Case T: HTTP Handler Immutability (PATCH with minDistinctProducts returns 400 immutable_field).
func TestMinDistinct_CaseT_HTTPHandlerImmutability(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	_, r := setupMarketingHTTPHandler(svc)
	ctx := context.Background()

	minDistinct := 2
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                uniqueCode("IMMUT"),
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, []uuid.UUID{res.CampaignID})

	patchBody, _ := json.Marshal(map[string]interface{}{
		"minDistinctProducts": 5,
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/seller/promotions/"+res.ID.String(), bytes.NewReader(patchBody))
	req = req.WithContext(newTestContextWithSeller(sellerID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.Equal(t, "immutable_field", errResp["code"])
}

// Case U: List and Get Promotions return minDistinctProducts.
func TestMinDistinct_CaseU_ListAndGetPromotions(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	ctx := context.Background()

	minDistinct := 4
	code := uniqueCode("LGPR")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinDistinctProducts: &minDistinct,
	})
	require.NoError(t, err)
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, []uuid.UUID{res.CampaignID})

	// Get promo code via repository
	got, err := repo.GetPromoCodeByCode(ctx, code)
	require.NoError(t, err)
	require.NotNil(t, got.MinDistinctProducts)
	assert.Equal(t, 4, *got.MinDistinctProducts)

	// List
	list, err := svc.ListSellerPromotions(ctx, sellerID)
	require.NoError(t, err)
	var found bool
	for _, p := range list {
		if p.ID == res.ID {
			found = true
			require.NotNil(t, p.MinDistinctProducts)
			assert.Equal(t, 4, *p.MinDistinctProducts)
		}
	}
	assert.True(t, found, "created promotion must be returned in list with minDistinctProducts")
}

// Case V: Database Safety - strict test DB verification and DEV DB immutability.
func TestMinDistinct_CaseV_DatabaseSafety(t *testing.T) {
	client, _, _ := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	var currentDB string
	err := client.Pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	assert.Equal(t, "zamk_test", currentDB, "destructive tests MUST run on zamk_test")
}

// Case W: Error code contract verification for checkout mapping.
func TestMinDistinct_CaseW_ErrorContract(t *testing.T) {
	assert.Equal(t, "promo_min_distinct_products", marketing.ErrPromoMinDistinctProducts.Error())
	assert.Equal(t, "min distinct products must be positive", marketing.ErrInvalidMinDistinctProducts.Error())
}

// Case X: DEV database remains strictly untouched at version 102 dirty=false.
func TestMinDistinct_CaseX_DevDatabaseUntouched(t *testing.T) {
	devURL := "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable"
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, devURL)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var devDB string
	err = conn.QueryRow(ctx, "SELECT current_database()").Scan(&devDB)
	require.NoError(t, err)
	assert.Equal(t, "zamk", devDB)

	var version int
	var dirty bool
	err = conn.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
	require.NoError(t, err)
	assert.Equal(t, 102, version, "DEV database MUST remain at version 102")
	assert.False(t, dirty, "DEV database MUST NOT be dirty")
}
