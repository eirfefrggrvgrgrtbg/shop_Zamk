package marketing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// ============================================================================
// PROMO.2D BACKEND TEST MATRIX: A through AI
// ============================================================================

// Case A: NULL min quantity preserves legacy behavior.
func TestMinQuantity_CaseA_NullPreservesLegacyBehavior(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-a")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("MQA")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000, // 10%
		MinEligibleQuantity: nil,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)
	assert.Nil(t, res.MinEligibleQuantity)

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

// Case B: exact minimum passes.
func TestMinQuantity_CaseB_ExactMinimumPasses(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-b")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 3)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	code := uniqueCode("MQB")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 3},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		// 3 * 50000 = 150000 cents * 10% = 15000 cents
		assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case C: below minimum fails with stable applicability error.
func TestMinQuantity_CaseC_BelowMinimumFails(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-c")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	code := uniqueCode("MQC")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case D: multiple eligible lines sum quantities.
func TestMinQuantity_CaseD_MultipleEligibleLinesSumQuantities(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-d")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 40000, 2)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 60000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 4
	code := uniqueCode("MQD")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 40000, Quantity: 2},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 60000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		// Total subtotal = 2*40000 + 2*60000 = 200000; 10% = 20000
		assert.Equal(t, int64(20000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case E: multiple units of same product/variant count.
func TestMinQuantity_CaseE_MultipleUnitsOfSameProductVariantCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-e")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 30000, 5)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 5
	code := uniqueCode("MQE")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 30000, Quantity: 5},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case F: excluded product quantity does not count.
func TestMinQuantity_CaseF_ExcludedProductQuantityDoesNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-f")
	orderID := createTestOrder(t, client, userID)

	itemEligibleID, prodEligibleID, varEligibleID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 2)
	itemExcludedID, prodExcludedID, varExcludedID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 5)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	code := uniqueCode("MQF")
	scope := marketing.ProductScopeEntireStore
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		ExcludedProductIDs:  []uuid.UUID{prodExcludedID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemEligibleID, ProductID: prodEligibleID, ProductVariantID: varEligibleID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 2},
		{OrderItemID: itemExcludedID, ProductID: prodExcludedID, ProductVariantID: varExcludedID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 5},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case G: excluded category quantity does not count.
func TestMinQuantity_CaseG_ExcludedCategoryQuantityDoesNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-g")
	orderID := createTestOrder(t, client, userID)

	catIncID := createTestCategory(t, client, nil, "CatIncG", true)
	catExcID := createTestCategory(t, client, nil, "CatExcG", true)
	prodIncID := createTestProductWithCategory(t, client, sellerID, catIncID, 50000)
	prodExcID := createTestProductWithCategory(t, client, sellerID, catExcID, 50000)

	itemIncID, varIncID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodIncID, 50000, 1)
	itemExcID, varExcID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodExcID, 50000, 5)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catIncID, catExcID})
	}()

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQG")
	scope := marketing.ProductScopeSelectedCategories
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catIncID},
		ExcludedCategoryIDs: []uuid.UUID{catExcID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemIncID, ProductID: prodIncID, ProductVariantID: varIncID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemExcID, ProductID: prodExcID, ProductVariantID: varExcID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 5},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case H: unrelated/out-of-scope category quantity does not count.
func TestMinQuantity_CaseH_OutOfScopeCategoryQuantityDoesNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-h")
	orderID := createTestOrder(t, client, userID)

	catAID := createTestCategory(t, client, nil, "CatAH", true)
	catBID := createTestCategory(t, client, nil, "CatBH", true)
	prodAID := createTestProductWithCategory(t, client, sellerID, catAID, 50000)
	prodBID := createTestProductWithCategory(t, client, sellerID, catBID, 50000)

	itemAID, varAID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodAID, 50000, 1)
	itemBID, varBID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodBID, 50000, 4)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catAID, catBID})
	}()

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQH")
	scope := marketing.ProductScopeSelectedCategories
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catAID}, // Only catA is included; catB is out-of-scope
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemAID, ProductID: prodAID, ProductVariantID: varAID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemBID, ProductID: prodBID, ProductVariantID: varBID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 4},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case I: foreign seller quantity does not count.
func TestMinQuantity_CaseI_ForeignSellerQuantityDoesNotCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	seller1ID := createTestSeller(t, client)
	seller2ID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-i")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, seller1ID, uuid.Nil, 50000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, seller2ID, uuid.Nil, 50000, 3)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{seller1ID, seller2ID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQI")
	res, err := svc.CreateSellerPromotion(ctx, seller1ID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: seller1ID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: seller2ID, BaseUnitPriceCents: 50000, Quantity: 3},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case J: ENTIRE_STORE works.
func TestMinQuantity_CaseJ_EntireStoreWorks(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-j")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 40000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 60000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	code := uniqueCode("MQJ")
	scope := marketing.ProductScopeEntireStore
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 40000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 60000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(16000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case K: SELECTED_PRODUCTS works.
func TestMinQuantity_CaseK_SelectedProductsWorks(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-k")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 2)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 50000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQK")
	scope := marketing.ProductScopeSelectedProducts
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		IncludedProductIDs:  []uuid.UUID{prod1ID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 2},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		// Only prod1 is included, quantity is 2 >= 2; discount is 10% of 2*50000 = 10000 cents
		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
		assert.Len(t, calc.PromotedLines, 1)
		return nil
	})
	require.NoError(t, err)
}

// Case L: SELECTED_CATEGORIES works.
func TestMinQuantity_CaseL_SelectedCategoriesWorks(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-l")
	orderID := createTestOrder(t, client, userID)

	catID := createTestCategory(t, client, nil, "CatL", true)
	prodID := createTestProductWithCategory(t, client, sellerID, catID, 50000)
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 50000, 3)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	minQty := 3
	code := uniqueCode("MQL")
	scope := marketing.ProductScopeSelectedCategories
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 3},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case M: parent category descendants count.
func TestMinQuantity_CaseM_ParentCategoryDescendantsCount(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-m")
	orderID := createTestOrder(t, client, userID)

	parentCatID := createTestCategory(t, client, nil, "ParentCatM", true)
	childCatID := createTestCategory(t, client, &parentCatID, "ChildCatM", true)
	prodID := createTestProductWithCategory(t, client, sellerID, childCatID, 50000)
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 50000, 3)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{childCatID, parentCatID})
	}()

	ctx := context.Background()
	minQty := 3
	code := uniqueCode("MQM")
	scope := marketing.ProductScopeSelectedCategories
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{parentCatID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 3},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case N: descendant category exclusion removes units.
func TestMinQuantity_CaseN_DescendantCategoryExclusionRemovesUnits(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-n")
	orderID := createTestOrder(t, client, userID)

	parentCatID := createTestCategory(t, client, nil, "ParentCatN", true)
	childCatID := createTestCategory(t, client, &parentCatID, "ChildCatN", true)

	prodParentID := createTestProductWithCategory(t, client, sellerID, parentCatID, 50000)
	prodChildID := createTestProductWithCategory(t, client, sellerID, childCatID, 50000)

	itemParentID, varParentID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodParentID, 50000, 1)
	itemChildID, varChildID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodChildID, 50000, 5)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{childCatID, parentCatID})
	}()

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQN")
	scope := marketing.ProductScopeSelectedCategories
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{parentCatID},
		ExcludedCategoryIDs: []uuid.UUID{childCatID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemParentID, ProductID: prodParentID, ProductVariantID: varParentID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemChildID, ProductID: prodChildID, ProductVariantID: varChildID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 5},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case O: product exclusion remains final veto.
func TestMinQuantity_CaseO_ProductExclusionRemainsFinalVeto(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-o")
	orderID := createTestOrder(t, client, userID)

	catID := createTestCategory(t, client, nil, "CatO", true)
	prodNormalID := createTestProductWithCategory(t, client, sellerID, catID, 50000)
	prodExcludedID := createTestProductWithCategory(t, client, sellerID, catID, 50000)

	itemNormalID, varNormalID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodNormalID, 50000, 1)
	itemExcludedID, varExcludedID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodExcludedID, 50000, 5)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQO")
	scope := marketing.ProductScopeSelectedCategories
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
		ExcludedProductIDs:  []uuid.UUID{prodExcludedID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemNormalID, ProductID: prodNormalID, ProductVariantID: varNormalID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 1},
		{OrderItemID: itemExcludedID, ProductID: prodExcludedID, ProductVariantID: varExcludedID, SellerID: sellerID, BaseUnitPriceCents: 50000, Quantity: 5},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case P: min subtotal + min quantity: both pass => applies.
func TestMinQuantity_CaseP_BothPassApplies(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-p")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 3)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	minSub := int64(200000) // 2,000 RUB
	code := uniqueCode("MQP")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinOrderSubtotalCents: minSub,
		MinEligibleQuantity:   &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 3},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(30000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case Q: subtotal passes / quantity fails => rejects.
func TestMinQuantity_CaseQ_SubtotalPassesQuantityFails(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-q")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 500000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	minSub := int64(200000)
	code := uniqueCode("MQQ")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinOrderSubtotalCents: minSub,
		MinEligibleQuantity:   &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 500000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinQuantity)
		return nil
	})
	require.NoError(t, err)
}

// Case R: quantity passes / subtotal fails => rejects.
func TestMinQuantity_CaseR_QuantityPassesSubtotalFails(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-r")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 10000, 4)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	minSub := int64(200000) // 2,000 RUB required, but cart is 4 * 100 RUB = 400 RUB
	code := uniqueCode("MQR")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinOrderSubtotalCents: minSub,
		MinEligibleQuantity:   &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 10000, Quantity: 4},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinSubtotal)
		return nil
	})
	require.NoError(t, err)
}

// Case S: PERCENT result unchanged after quantity passes.
func TestMinQuantity_CaseS_PercentResultUnchanged(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-s")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQS")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000, // 10%
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(20000), calc.TotalSellerDiscountCents) // 10% of 200,000 = 20,000
		return nil
	})
	require.NoError(t, err)
}

// Case T: FIXED allocation unchanged after quantity passes.
func TestMinQuantity_CaseT_FixedAllocationUnchanged(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-t")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQT")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                    code,
		DiscountType:            marketing.DiscountTypeFixed,
		DiscountValueFixedCents: 50000, // 500 RUB
		MinEligibleQuantity:     &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(50000), calc.TotalSellerDiscountCents)
		assert.Equal(t, int64(25000), calc.PromotedLines[0].TotalSellerDiscountCents)
		assert.Equal(t, int64(25000), calc.PromotedLines[1].TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case U: max discount unchanged.
func TestMinQuantity_CaseU_MaxDiscountUnchanged(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-u")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 200000, 2) // 400,000 cents

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	maxDisc := int64(100000) // 1,000 RUB cap
	code := uniqueCode("MQU")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    5000, // 50% = 200,000 cents, capped at 100,000 cents
		MaxDiscountCents:    &maxDisc,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(100000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// Case V: deterministic cent allocation unchanged.
func TestMinQuantity_CaseV_DeterministicCentAllocationUnchanged(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-v")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 33300, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 33300, 1)
	item3ID, prod3ID, var3ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 33400, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 3
	code := uniqueCode("MQV")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                    code,
		DiscountType:            marketing.DiscountTypeFixed,
		DiscountValueFixedCents: 1000, // 10 RUB = 1000 cents
		MinEligibleQuantity:     &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 33300, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 33300, Quantity: 1},
		{OrderItemID: item3ID, ProductID: prod3ID, ProductVariantID: var3ID, SellerID: sellerID, BaseUnitPriceCents: 33400, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(1000), calc.TotalSellerDiscountCents)
		var sum int64
		for _, line := range calc.PromotedLines {
			sum += line.TotalSellerDiscountCents
		}
		assert.Equal(t, int64(1000), sum)
		return nil
	})
	require.NoError(t, err)
}

// Case W: multi-quantity order_item_promotions snapshot remains correct.
func TestMinQuantity_CaseW_MultiQuantityOrderItemPromotionsSnapshot(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-w")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQW")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		err = svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
		require.NoError(t, err)
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	var snapSellerDisc int64
	err = client.Pool.QueryRow(ctx, `
		SELECT total_seller_discount_cents FROM order_item_promotions WHERE order_item_id = $1
	`, itemID).Scan(&snapSellerDisc)
	require.NoError(t, err)
	assert.Equal(t, int64(20000), snapSellerDisc)
}

// Case X: reservation freezes accepted result.
func TestMinQuantity_CaseX_ReservationFreezesAcceptedResult(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-x")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQX")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)
	assert.Equal(t, int64(20000), usage.SellerDiscountCents)
}

// Case Y: payment retry does not re-evaluate quantity.
func TestMinQuantity_CaseY_PaymentRetryDoesNotReevaluateQuantity(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-y")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQY")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Simulate payment retry: consume directly without evaluating items/quantities
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// Case Z: ambiguous Init/unknown does not re-evaluate.
func TestMinQuantity_CaseZ_AmbiguousInitDoesNotReevaluate(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-z")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQZ")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Ambiguous init outcome leaves reservation untouched in RESERVED status
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)
}

// Case AA: AUTH_FAIL does not re-evaluate.
func TestMinQuantity_CaseAA_AuthFailDoesNotReevaluate(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-aa")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQAA")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Reservation preserved despite auth failure
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)
}

// Case AB: terminal failure releases normally.
func TestMinQuantity_CaseAB_TerminalFailureReleasesNormally(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-ab")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQAB")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "terminal failure")
	})
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)
}

// Case AC: successful payment consumes normally.
func TestMinQuantity_CaseAC_SuccessfulPaymentConsumesNormally(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-ac")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 2)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	minQty := 2
	code := uniqueCode("MQAC")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 2},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// Case AD: create request persists min quantity.
func TestMinQuantity_CaseAD_CreateRequestPersistsMinQuantity(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)

	ctx := context.Background()
	minQty := 7
	code := uniqueCode("MQAD")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	require.NotNil(t, res.MinEligibleQuantity)
	assert.Equal(t, 7, *res.MinEligibleQuantity)

	// Verify directly in DB
	var dbMinQty *int
	err = client.Pool.QueryRow(ctx, `SELECT min_eligible_quantity FROM promo_codes WHERE code = $1`, code).Scan(&dbMinQty)
	require.NoError(t, err)
	require.NotNil(t, dbMinQty)
	assert.Equal(t, 7, *dbMinQty)
}

// Case AE: Seller response returns min quantity.
func TestMinQuantity_CaseAE_SellerResponseReturnsMinQuantity(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)

	ctx := context.Background()
	minQty := 4
	code := uniqueCode("MQAE")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	promos, err := svc.ListSellerPromotions(ctx, sellerID)
	require.NoError(t, err)

	var found *marketing.SellerPromoResponse
	for _, p := range promos {
		if p.ID == res.ID {
			found = &p
			break
		}
	}
	require.NotNil(t, found)
	require.NotNil(t, found.MinEligibleQuantity)
	assert.Equal(t, 4, *found.MinEligibleQuantity)
}

// Case AF: update cannot mutate min quantity.
func TestMinQuantity_CaseAF_UpdateCannotMutateMinQuantity(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)

	ctx := context.Background()
	minQty := 5
	code := uniqueCode("MQAF")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Update mutable fields only
	newVal := false
	updated, err := svc.UpdateSellerPromotion(ctx, sellerID, res.ID, marketing.UpdateSellerPromoRequest{
		IsActive: &newVal,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.MinEligibleQuantity)
	assert.Equal(t, 5, *updated.MinEligibleQuantity)

	// Verify directly in DB that min_eligible_quantity is still 5
	var dbMinQty *int
	err = client.Pool.QueryRow(ctx, `SELECT min_eligible_quantity FROM promo_codes WHERE code = $1`, code).Scan(&dbMinQty)
	require.NoError(t, err)
	require.NotNil(t, dbMinQty)
	assert.Equal(t, 5, *dbMinQty)
}

// Case AG: zero rejected.
func TestMinQuantity_CaseAG_ZeroRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	ctx := context.Background()
	minQty := 0
	code := uniqueCode("MQAG")

	_, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrInvalidMinQuantity)
}

// Case AH: negative rejected.
func TestMinQuantity_CaseAH_NegativeRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	ctx := context.Background()
	minQty := -3
	code := uniqueCode("MQAH")

	_, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &minQty,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrInvalidMinQuantity)
}

// Case AI: legacy NULL row remains valid.
func TestMinQuantity_CaseAI_LegacyNullRowRemainsValid(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-mq-ai")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	campID := uuid.New()
	code := uniqueCode("MQAI")

	// Directly insert campaign and promo code with NULL min_eligible_quantity
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, seller_id, title, funding_mode, status, discount_type, seller_discount_bps, created_at, updated_at)
		VALUES ($1, $2, 'Legacy Campaign', 'seller', 'active', 'percent', 1000, now(), now())
	`, campID, sellerID)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, campID)

	promoID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, product_scope, min_eligible_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'percent', 1000, true, 'ENTIRE_STORE', NULL, now(), now())
	`, promoID, campID, sellerID, code)
	require.NoError(t, err)

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

// HTTP Handler test for stable error status codes
func TestMinQuantity_HTTPHandlerErrors(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	h := marketing.NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Zero min quantity -> 400 invalid_min_quantity
	zeroQty := 0
	body, _ := json.Marshal(marketing.CreateSellerPromoRequest{
		Code:                uniqueCode("HTTP0"),
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		MinEligibleQuantity: &zeroQty,
	})

	req := httptest.NewRequest(http.MethodPost, "/seller/promotions", bytes.NewReader(body))
	req = req.WithContext(newTestContextWithSeller(sellerID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.CreateSellerPromotion(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.Equal(t, "invalid_min_quantity", errResp["code"])
}
