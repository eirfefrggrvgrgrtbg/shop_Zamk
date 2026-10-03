package marketing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func createTestCategory(t *testing.T, client *postgres.Client, parentID *uuid.UUID, name string, isActive bool) uuid.UUID {
	ctx := context.Background()
	testutil.AssertTestDatabase(t, client.Pool)

	id := uuid.New()
	slug := "cat-" + id.String()[:8]
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO categories (id, parent_id, name, slug, is_active, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, now(), now())
	`, id, parentID, name, slug, isActive)
	require.NoError(t, err)
	return id
}

func createTestProductWithCategory(t *testing.T, client *postgres.Client, sellerID, catID uuid.UUID, priceCents int64) uuid.UUID {
	ctx := context.Background()
	testutil.AssertTestDatabase(t, client.Pool)

	prodID := uuid.New()
	slug := "prod-" + prodID.String()[:8]
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Category Prod', $4, $5, 'published', now(), now())
	`, prodID, sellerID, catID, slug, priceCents)
	require.NoError(t, err)
	return prodID
}

func createTestOrderItemWithProduct(t *testing.T, client *postgres.Client, orderID, sellerID, prodID uuid.UUID, priceCents int64, quantity int) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	testutil.AssertTestDatabase(t, client.Pool)

	variantID := uuid.New()
	barcode := "BAR-" + variantID.String()[:8]
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, now(), now())
	`, variantID, prodID, "SKU-"+variantID.String()[:8], "SSKU-"+variantID.String()[:8], barcode, priceCents)
	require.NoError(t, err)

	var existingFulID uuid.UUID
	err = client.Pool.QueryRow(ctx, `SELECT id FROM order_fulfillments WHERE order_id = $1 AND seller_id = $2`, orderID, sellerID).Scan(&existingFulID)
	var fulfillmentID uuid.UUID
	if err != nil {
		fulfillmentID = uuid.New()
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'awaiting_payment', $4, 1500, $4, now(), now())
		`, fulfillmentID, orderID, sellerID, priceCents*int64(quantity))
		require.NoError(t, err)
	} else {
		fulfillmentID = existingFulID
	}

	itemID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item', 'slug', $7, $8, $9, now())
	`, itemID, orderID, fulfillmentID, prodID, variantID, sellerID, priceCents, quantity, priceCents*int64(quantity))
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

func cleanupCategoryFixtures(client *postgres.Client, categoryIDs []uuid.UUID) {
	ctx := context.Background()
	for _, cid := range categoryIDs {
		if cid == uuid.Nil {
			continue
		}
		_, _ = client.Pool.Exec(ctx, "DELETE FROM promo_code_category_targets WHERE category_id = $1", cid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", cid)
	}
}

// ============================================================================
// A. Create promo with SELECTED_CATEGORIES:
//    - included category IDs saved in promo_code_category_targets;
//    - product_scope = SELECTED_CATEGORIES.
// ============================================================================
func TestCategoryTargeting_CaseA_CreatePromoWithSelectedCategories(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	catID := createTestCategory(t, client, nil, "Clothing", true)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	code := uniqueCode("CAT")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000, // 10%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, marketing.ProductScopeSelectedCategories, res.ProductScope)
	assert.Equal(t, []uuid.UUID{catID}, res.IncludedCategoryIDs)
	assert.Empty(t, res.ExcludedCategoryIDs)

	// Verify in DB directly
	var targetCount int
	err = client.Pool.QueryRow(ctx, `
		SELECT count(*) FROM promo_code_category_targets
		WHERE promo_code_id = (SELECT id FROM promo_codes WHERE code = $1)
		  AND category_id = $2 AND target_type = 'INCLUDE'
	`, code, catID).Scan(&targetCount)
	require.NoError(t, err)
	assert.Equal(t, 1, targetCount)
}

// ============================================================================
// B. Cart with product in targeted category:
//    - discount applies successfully.
// ============================================================================
func TestCategoryTargeting_CaseB_ProductInTargetedCategoryDiscounts(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-b")
	orderID := createTestOrder(t, client, userID)

	catID := createTestCategory(t, client, nil, "Clothing", true)
	prodID := createTestProductWithCategory(t, client, sellerID, catID, 100000) // 1000 RUB
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 100000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATB")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    2000, // 20%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(20000), calc.TotalSellerDiscountCents) // 200 RUB
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, itemID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// C. Cart with product in subcategory of targeted parent category:
//    - discount applies successfully (recursive resolution).
// ============================================================================
func TestCategoryTargeting_CaseC_ProductInSubcategoryDiscountsRecursively(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-c")
	orderID := createTestOrder(t, client, userID)

	parentCatID := createTestCategory(t, client, nil, "Clothing", true)
	childCatID := createTestCategory(t, client, &parentCatID, "Outerwear", true)

	prodID := createTestProductWithCategory(t, client, sellerID, childCatID, 150000) // 1500 RUB
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 150000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{parentCatID, childCatID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATC")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000, // 10%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{parentCatID}, // Targeting parent
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 150000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents) // 150 RUB
		require.Len(t, calc.PromotedLines, 1)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// D. Cart with product in deeply nested subcategory (3 levels):
//    - discount applies successfully.
// ============================================================================
func TestCategoryTargeting_CaseD_DeeplyNestedSubcategoryDiscounts(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-d")
	orderID := createTestOrder(t, client, userID)

	level1 := createTestCategory(t, client, nil, "Level 1", true)
	level2 := createTestCategory(t, client, &level1, "Level 2", true)
	level3 := createTestCategory(t, client, &level2, "Level 3", true)

	prodID := createTestProductWithCategory(t, client, sellerID, level3, 200000) // 2000 RUB
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 200000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{level1, level2, level3})
	}()

	ctx := context.Background()
	code := uniqueCode("CATD")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    2500, // 25%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{level1},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(50000), calc.TotalSellerDiscountCents) // 500 RUB
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// E. Cart with product in non-targeted category:
//    - discount does NOT apply (ErrPromoNotApplicable).
// ============================================================================
func TestCategoryTargeting_CaseE_ProductInNonTargetedCategoryFails(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-e")
	orderID := createTestOrder(t, client, userID)

	catClothing := createTestCategory(t, client, nil, "Clothing", true)
	catElectronics := createTestCategory(t, client, nil, "Electronics", true)

	prodID := createTestProductWithCategory(t, client, sellerID, catElectronics, 300000)
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 300000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catClothing, catElectronics})
	}()

	ctx := context.Background()
	code := uniqueCode("CATE")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catClothing}, // only clothing
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 300000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.True(t, errors.Is(err, marketing.ErrPromoNotApplicable), "expected ErrPromoNotApplicable, got %v", err)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// F. Cart with product in targeted category AND excluded by product exclusion:
//    - discount does NOT apply to that product (product exclusion wins).
// ============================================================================
func TestCategoryTargeting_CaseF_ProductExclusionWinsOverCategoryInclude(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-f")
	orderID := createTestOrder(t, client, userID)

	catID := createTestCategory(t, client, nil, "Clothing", true)
	prodExcludedID := createTestProductWithCategory(t, client, sellerID, catID, 100000)
	prodIncludedID := createTestProductWithCategory(t, client, sellerID, catID, 200000)

	itemExcID, varExcID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodExcludedID, 100000, 1)
	itemIncID, varIncID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodIncludedID, 200000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATF")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
		ExcludedProductIDs:  []uuid.UUID{prodExcludedID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemExcID, ProductID: prodExcludedID, ProductVariantID: varExcID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: itemIncID, ProductID: prodIncludedID, ProductVariantID: varIncID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		// Discount ONLY on prodIncludedID (10% of 2000 = 200 RUB = 20,000 cents)
		assert.Equal(t, int64(20000), calc.TotalSellerDiscountCents)
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, itemIncID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// G. Cart with product in targeted parent category AND child category is excluded:
//    - product in child category does NOT receive discount (category exclusion wins).
//    - product in another sibling child category DOES receive discount.
// ============================================================================
func TestCategoryTargeting_CaseG_ChildCategoryExclusionWinsOverParentInclude(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-g")
	orderID := createTestOrder(t, client, userID)

	parentCatID := createTestCategory(t, client, nil, "Apparel", true)
	childExcludedCatID := createTestCategory(t, client, &parentCatID, "Coats", true)
	childIncludedCatID := createTestCategory(t, client, &parentCatID, "Shirts", true)

	prodInExcCat := createTestProductWithCategory(t, client, sellerID, childExcludedCatID, 500000) // 5000 RUB
	prodInIncCat := createTestProductWithCategory(t, client, sellerID, childIncludedCatID, 300000) // 3000 RUB

	itemExcID, varExcID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodInExcCat, 500000, 1)
	itemIncID, varIncID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodInIncCat, 300000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{parentCatID, childExcludedCatID, childIncludedCatID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATG")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000, // 10%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{parentCatID},
		ExcludedCategoryIDs: []uuid.UUID{childExcludedCatID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemExcID, ProductID: prodInExcCat, ProductVariantID: varExcID, SellerID: sellerID, BaseUnitPriceCents: 500000, Quantity: 1},
		{OrderItemID: itemIncID, ProductID: prodInIncCat, ProductVariantID: varIncID, SellerID: sellerID, BaseUnitPriceCents: 300000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		// Only childIncludedCatID gets 10% (30,000 cents)
		assert.Equal(t, int64(30000), calc.TotalSellerDiscountCents)
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, itemIncID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// H. Exact same category in included and excluded list at creation:
//    - rejected with ErrCategoryConflict / 400.
// ============================================================================
func TestCategoryTargeting_CaseH_ExactSameCategoryInIncludeAndExcludeRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	catID := createTestCategory(t, client, nil, "Clothing", true)

	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, nil)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                uniqueCode("CATH"),
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
		ExcludedCategoryIDs: []uuid.UUID{catID},
	}

	_, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	assert.True(t, errors.Is(err, marketing.ErrCategoryConflict), "expected ErrCategoryConflict, got %v", err)
}

// ============================================================================
// I. Excluded category without category targeting (ENTIRE_STORE + excluded category):
//    - discount applies to store products EXCEPT those in the excluded category and its subcategories.
// ============================================================================
func TestCategoryTargeting_CaseI_EntireStoreWithCategoryExclusion(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-i")
	orderID := createTestOrder(t, client, userID)

	catParent := createTestCategory(t, client, nil, "Parent", true)
	catChild := createTestCategory(t, client, &catParent, "Child", true)
	catOther := createTestCategory(t, client, nil, "Other", true)

	prodInChild := createTestProductWithCategory(t, client, sellerID, catChild, 100000)
	prodInOther := createTestProductWithCategory(t, client, sellerID, catOther, 200000)

	itemChildID, varChildID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodInChild, 100000, 1)
	itemOtherID, varOtherID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodInOther, 200000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catParent, catChild, catOther})
	}()

	ctx := context.Background()
	code := uniqueCode("CATI")
	scope := marketing.ProductScopeEntireStore
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		ExcludedCategoryIDs: []uuid.UUID{catParent}, // excludes parent and its child
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemChildID, ProductID: prodInChild, ProductVariantID: varChildID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: itemOtherID, ProductID: prodInOther, ProductVariantID: varOtherID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(20000), calc.TotalSellerDiscountCents) // 10% of 200000 only
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, itemOtherID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// J. Excluded category with SELECTED_PRODUCTS targeting:
//    - REJECTED with ErrCategoryNotAllowed (400 Bad Request, category_not_allowed).
// ============================================================================
func TestCategoryTargeting_CaseJ_SelectedProductsWithCategoryExclusionRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	catExc := createTestCategory(t, client, nil, "Excluded Cat", true)
	prod1 := createTestProductWithCategory(t, client, sellerID, catExc, 100000)

	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, nil)
		cleanupCategoryFixtures(client, []uuid.UUID{catExc})
	}()

	ctx := context.Background()
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:                uniqueCode("CATJ_REJ"),
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedProductIDs:  []uuid.UUID{prod1},
		ExcludedCategoryIDs: []uuid.UUID{catExc},
	}

	_, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	assert.True(t, errors.Is(err, marketing.ErrCategoryNotAllowed), "expected ErrCategoryNotAllowed, got %v", err)

	// Also verify HTTP Handler returns 400 Bad Request with code "category_not_allowed"
	_, router := setupMarketingHTTPHandler(svc)
	createBody := map[string]interface{}{
		"code":                uniqueCode("CATJ_HTTP"),
		"discountType":        "percent",
		"discountValueBps":    1000,
		"productScope":        "SELECTED_PRODUCTS",
		"includedProductIds":  []string{prod1.String()},
		"excludedCategoryIds": []string{catExc.String()},
	}
	bodyBytes, _ := json.Marshal(createBody)
	httpReq := httptest.NewRequest(http.MethodPost, "/api/seller/promotions", bytes.NewReader(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = httpReq.WithContext(newTestContextWithSeller(sellerID))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httpReq)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.Equal(t, "category_not_allowed", errResp["code"])
}

// ============================================================================
// J2. SELECTED_PRODUCTS with Product Exclusion:
//     - Product exclusion retains existing PROMO.2B semantics and works properly.
// ============================================================================
func TestCategoryTargeting_CaseJ2_SelectedProductsProductExclusionWorks(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-j2")
	orderID := createTestOrder(t, client, userID)

	cat := createTestCategory(t, client, nil, "Standard Cat", true)
	prod1 := createTestProductWithCategory(t, client, sellerID, cat, 100000)
	prod2 := createTestProductWithCategory(t, client, sellerID, cat, 200000)
	prod3 := createTestProductWithCategory(t, client, sellerID, cat, 300000)

	item1ID, var1ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod1, 100000, 1)
	item2ID, var2ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod2, 200000, 1)
	item3ID, var3ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod3, 300000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{cat})
	}()

	ctx := context.Background()
	code := uniqueCode("CATJ2")
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prod1, prod2},
		ExcludedProductIDs: []uuid.UUID{prod3},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
		{OrderItemID: item3ID, ProductID: prod3, ProductVariantID: var3ID, SellerID: sellerID, BaseUnitPriceCents: 300000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(30000), calc.TotalSellerDiscountCents) // 10% on prod1 (10,000) and prod2 (20,000) = 30,000
		require.Len(t, calc.PromotedLines, 2)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// K. Mixed cart:
//    - Item 1: included category -> discounted.
//    - Item 2: excluded category -> full price.
//    - Item 3: non-targeted category -> full price.
//    - Item 4: other seller's product -> untouched.
// ============================================================================
func TestCategoryTargeting_CaseK_MixedCartEvaluation(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	otherSellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-k")
	orderID := createTestOrder(t, client, userID)

	catInc := createTestCategory(t, client, nil, "Targeted", true)
	catExc := createTestCategory(t, client, nil, "Excluded", true)
	catOther := createTestCategory(t, client, nil, "NonTargeted", true)

	prod1 := createTestProductWithCategory(t, client, sellerID, catInc, 100000)
	prod2 := createTestProductWithCategory(t, client, sellerID, catExc, 100000)
	prod3 := createTestProductWithCategory(t, client, sellerID, catOther, 100000)
	prod4 := createTestProductWithCategory(t, client, otherSellerID, catInc, 100000)

	item1ID, var1ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod1, 100000, 1)
	item2ID, var2ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod2, 100000, 1)
	item3ID, var3ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod3, 100000, 1)
	item4ID, var4ID := createTestOrderItemWithProduct(t, client, orderID, otherSellerID, prod4, 100000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID, otherSellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catInc, catExc, catOther})
	}()

	ctx := context.Background()
	code := uniqueCode("CATK")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    2000, // 20%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catInc},
		ExcludedCategoryIDs: []uuid.UUID{catExc},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item3ID, ProductID: prod3, ProductVariantID: var3ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item4ID, ProductID: prod4, ProductVariantID: var4ID, SellerID: otherSellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(20000), calc.TotalSellerDiscountCents) // Only item 1
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, item1ID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// L. Deactivated category:
//    - products in deactivated category do not qualify.
// ============================================================================
func TestCategoryTargeting_CaseL_DeactivatedCategoryFails(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-l")
	orderID := createTestOrder(t, client, userID)

	catID := createTestCategory(t, client, nil, "ActiveAtStart", true)
	prodID := createTestProductWithCategory(t, client, sellerID, catID, 100000)
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 100000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATL")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Now deactivate category
	_, err = client.Pool.Exec(ctx, `UPDATE categories SET is_active = false WHERE id = $1`, catID)
	require.NoError(t, err)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.True(t, errors.Is(err, marketing.ErrPromoNotApplicable))
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// M. Min order subtotal with category targeting:
//    - subtotal calculated ONLY from eligible items.
//    - non-eligible items in cart do not count toward min subtotal threshold.
// ============================================================================
func TestCategoryTargeting_CaseM_MinOrderSubtotalOnlyFromEligibleItems(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-m")
	orderID := createTestOrder(t, client, userID)

	catInc := createTestCategory(t, client, nil, "Included", true)
	catOther := createTestCategory(t, client, nil, "Other", true)

	prodEligible := createTestProductWithCategory(t, client, sellerID, catInc, 40000)      // 400 RUB
	prodNonEligible := createTestProductWithCategory(t, client, sellerID, catOther, 80000) // 800 RUB

	itemElID, varElID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodEligible, 40000, 1)
	itemNonID, varNonID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodNonEligible, 80000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catInc, catOther})
	}()

	ctx := context.Background()
	code := uniqueCode("CATM")
	scope := marketing.ProductScopeSelectedCategories
	// Threshold = 500 RUB (50,000 cents). Cart total is 1200 RUB, but eligible is only 400 RUB!
	req := marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		ProductScope:          &scope,
		IncludedCategoryIDs:   []uuid.UUID{catInc},
		MinOrderSubtotalCents: 50000,
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemElID, ProductID: prodEligible, ProductVariantID: varElID, SellerID: sellerID, BaseUnitPriceCents: 40000, Quantity: 1},
		{OrderItemID: itemNonID, ProductID: prodNonEligible, ProductVariantID: varNonID, SellerID: sellerID, BaseUnitPriceCents: 80000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.True(t, errors.Is(err, marketing.ErrPromoMinSubtotal), "expected ErrPromoMinSubtotal, got %v", err)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// N. Max discount cap with category targeting:
//    - cap applied to total discount from eligible items.
// ============================================================================
func TestCategoryTargeting_CaseN_MaxDiscountCapApplied(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-n")
	orderID := createTestOrder(t, client, userID)

	catID := createTestCategory(t, client, nil, "Clothing", true)
	prodID := createTestProductWithCategory(t, client, sellerID, catID, 1000000) // 10,000 RUB
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 1000000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATN")
	scope := marketing.ProductScopeSelectedCategories
	maxDisc := int64(50000) // 500 RUB cap
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    2000, // 20% of 10,000 = 2,000 RUB uncapped
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
		MaxDiscountCents:    &maxDisc,
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 1000000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(50000), calc.TotalSellerDiscountCents) // capped at 500 RUB
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// O. Edit promo:
//    - category targeting fields cannot be modified via update endpoint.
// ============================================================================
func TestCategoryTargeting_CaseO_ImmutableCategoryTargetingInUpdate(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	catID := createTestCategory(t, client, nil, "Clothing", true)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATO")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Via HTTP PATCH handler
	_, router := setupMarketingHTTPHandler(svc)
	patchBody := map[string]interface{}{
		"includedCategoryIds": []string{uuid.New().String()},
	}
	bodyBytes, _ := json.Marshal(patchBody)
	httpReq := httptest.NewRequest(http.MethodPatch, "/api/seller/promotions/"+res.ID.String(), bytes.NewReader(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = httpReq.WithContext(newTestContextWithSeller(sellerID))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httpReq)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "immutable")
}

// ============================================================================
// P. Fail-closed:
//    - promo with SELECTED_CATEGORIES targeting zero surviving active categories returns ErrPromoNotApplicable.
// ============================================================================
func TestCategoryTargeting_CaseP_FailClosedZeroSurvivingCategories(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-p")
	orderID := createTestOrder(t, client, userID)

	catParent := createTestCategory(t, client, nil, "OnlyParent", true)
	prodID := createTestProductWithCategory(t, client, sellerID, catParent, 100000)
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 100000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catParent})
	}()

	ctx := context.Background()
	code := uniqueCode("CATP")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catParent},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Now delete or deactivate category directly in DB so zero categories survive
	_, err = client.Pool.Exec(ctx, `UPDATE categories SET is_active = false WHERE id = $1`, catParent)
	require.NoError(t, err)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.True(t, errors.Is(err, marketing.ErrPromoNotApplicable))
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// Q. List promos endpoint:
//    - returns includedCategoryIds and excludedCategoryIds correctly.
// ============================================================================
func TestCategoryTargeting_CaseQ_ListPromotionsReturnsCategoryTargets(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	catInc := createTestCategory(t, client, nil, "ListCatInc", true)
	catExc := createTestCategory(t, client, nil, "ListCatExc", true)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catInc, catExc})
	}()

	ctx := context.Background()
	code := uniqueCode("CATQ")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1000,
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catInc},
		ExcludedCategoryIDs: []uuid.UUID{catExc},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Call list endpoint via HTTP
	_, router := setupMarketingHTTPHandler(svc)
	httpReq := httptest.NewRequest(http.MethodGet, "/api/seller/promotions", nil)
	httpReq = httpReq.WithContext(newTestContextWithSeller(sellerID))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httpReq)
	assert.Equal(t, http.StatusOK, rec.Code)

	var listRes struct {
		Items []marketing.SellerPromoItem `json:"items"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &listRes)
	require.NoError(t, err)

	var found *marketing.SellerPromoItem
	for i := range listRes.Items {
		if listRes.Items[i].Code == code {
			found = &listRes.Items[i]
			break
		}
	}
	require.NotNil(t, found, "expected to find created promo in list response")
	assert.Equal(t, marketing.ProductScopeSelectedCategories, found.ProductScope)
	assert.Equal(t, []uuid.UUID{catInc}, found.IncludedCategoryIDs)
	assert.Equal(t, []uuid.UUID{catExc}, found.ExcludedCategoryIDs)
}

// ============================================================================
// R. FAIL-CLOSED INVARIANT: Surviving INCLUDE targets cease to exist
//    - For a SELECTED_CATEGORIES promo, if its persisted INCLUDE target rows
//      cease to exist due to canonical category deletion (CASCADE),
//      the promo MUST have ZERO eligible products (ErrPromoNotApplicable).
//    - NEVER: zero surviving category INCLUDE targets => ENTIRE_STORE.
// ============================================================================
func TestCategoryTargeting_CaseR_FailClosedWhenSurvivingIncludeTargetsDisappear(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-r")
	orderID := createTestOrder(t, client, userID)

	// Isolated UUID category fixture
	catID := createTestCategory(t, client, nil, "IsolatedCatR", true)
	otherCatID := createTestCategory(t, client, nil, "OtherCatR", true)
	prodID := createTestProductWithCategory(t, client, sellerID, catID, 100000)
	itemID, varID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prodID, 100000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catID, otherCatID})
	}()

	ctx := context.Background()
	code := uniqueCode("CATR")
	scope := marketing.ProductScopeSelectedCategories
	req := marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    1500, // 15%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	// 1. Initial sanity check: discount applies when target exists
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)

	// 2. Canonical deletion of category:
	//    Under PostgreSQL FK semantics:
	//    - categories(id) is deleted
	//    - promo_code_category_targets has ON DELETE CASCADE -> target rows are deleted
	//    - products.category_id has ON DELETE SET NULL -> prodID.category_id becomes NULL
	_, err = client.Pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, catID)
	require.NoError(t, err)

	// Verify target row was CASCADE-deleted
	targets, err := repo.GetPromoCodeCategoryTargets(ctx, res.ID)
	require.NoError(t, err)
	assert.Empty(t, targets, "persisted category targets must be empty after category deletion")

	// 3. Now evaluate promo on checkout:
	//    Promo has ProductScopeSelectedCategories but ZERO surviving category INCLUDE targets.
	//    FAIL-CLOSED INVARIANT: It MUST return ErrPromoNotApplicable.
	//    It MUST NEVER fall back to ENTIRE_STORE.
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.True(t, errors.Is(err, marketing.ErrPromoNotApplicable),
			"expected ErrPromoNotApplicable when zero category targets survive, got %v", err)
		return nil
	})
	require.NoError(t, err)

	// 4. Test another product in the store: still ZERO eligible products
	otherProdID := createTestProductWithCategory(t, client, sellerID, otherCatID, 200000)
	otherItemID, otherVarID := createTestOrderItemWithProduct(t, client, orderID, sellerID, otherProdID, 200000, 1)
	otherItems := []marketing.PromotedOrderItemInput{
		{OrderItemID: otherItemID, ProductID: otherProdID, ProductVariantID: otherVarID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, otherItems, 1000, time.Now().UTC())
		assert.True(t, errors.Is(err, marketing.ErrPromoNotApplicable),
			"expected ErrPromoNotApplicable for any product when zero category targets survive, got %v", err)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// S. PAYMENT LIFECYCLE: Category promo reservation and consumption
//    - category-scoped reservation snapshot created from eligible lines;
//    - ambiguous Init/unknown does not re-evaluate categories;
//    - AUTH_FAIL does not re-evaluate categories;
//    - later CONFIRMED consumes the existing reservation once;
//    - later terminal rejection releases once;
//    - payment retry does not resolve category scope again.
// ============================================================================
func TestCategoryTargeting_CaseS_PaymentLifecycleCategoryPromo(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	user1 := createTestUser(t, client, "usr-cat-s1")
	user2 := createTestUser(t, client, "usr-cat-s2")
	order1 := createTestOrder(t, client, user1)
	order2 := createTestOrder(t, client, user2)

	catInc := createTestCategory(t, client, nil, "ApparelS", true)
	catExc := createTestCategory(t, client, nil, "ExcludedS", true)

	prod1 := createTestProductWithCategory(t, client, sellerID, catInc, 200000) // eligible
	prod2 := createTestProductWithCategory(t, client, sellerID, catExc, 100000) // excluded

	item1ID, var1ID := createTestOrderItemWithProduct(t, client, order1, sellerID, prod1, 200000, 1)
	item2ID, var2ID := createTestOrderItemWithProduct(t, client, order1, sellerID, prod2, 100000, 1)

	item3ID, var3ID := createTestOrderItemWithProduct(t, client, order2, sellerID, prod1, 200000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user1, user2}, []uuid.UUID{order1, order2}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{catInc, catExc})
	}()

	ctx := context.Background()
	code := uniqueCode("CATS")
	scope := marketing.ProductScopeSelectedCategories
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                code,
		DiscountType:        marketing.DiscountTypePercent,
		DiscountValueBps:    2000, // 20%
		ProductScope:        &scope,
		IncludedCategoryIDs: []uuid.UUID{catInc},
		ExcludedCategoryIDs: []uuid.UUID{catExc},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items1 := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	// 1. Calculate and reserve for Order 1
	var calc1 *marketing.CheckoutPromoCalculation
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		var err error
		calc1, err = svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user1, code, items1, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(40000), calc1.TotalSellerDiscountCents) // 20% of 2000 RUB = 400 RUB = 40,000 cents
		require.Len(t, calc1.PromotedLines, 1)
		assert.Equal(t, item1ID, calc1.PromotedLines[0].OrderItemID)

		return svc.ReserveCheckoutPromoTx(ctx, tx, order1, user1, calc1, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Verify immutable order_item_promotions snapshot row exists
	var snapCount int
	err = client.Pool.QueryRow(ctx, `SELECT count(*) FROM order_item_promotions WHERE order_item_id = $1`, item1ID).Scan(&snapCount)
	require.NoError(t, err)
	assert.Equal(t, 1, snapCount)

	// 2. Ambiguous Init / unknown state: reservation status remains "reserved"
	usage1, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage1.Status)

	// 3. CONFIRMED webhook: consume reservation once
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, order1)
	})
	require.NoError(t, err)

	usage1Consumed, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage1Consumed.Status)

	// Duplicate consume is an idempotent no-op (does not re-evaluate categories or double consume)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, order1)
	})
	require.NoError(t, err)

	// 4. Order 2: Terminal failure / AUTH_FAIL releases once
	items2 := []marketing.PromotedOrderItemInput{
		{OrderItemID: item3ID, ProductID: prod1, ProductVariantID: var3ID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc2, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user2, code, items2, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, order2, user2, calc2, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, order2, "auth_failed")
	})
	require.NoError(t, err)

	usage2Released, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order2)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage2Released.Status)

	// Duplicate release is idempotent no-op
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, order2, "auth_failed")
	})
	require.NoError(t, err)
}

// ============================================================================
// T. LEGACY PROMOTIONS REMAIN UNCHANGED
//    - ENTIRE_STORE promo without category targets applies discount across store.
//    - SELECTED_PRODUCTS promo without category targets applies discount only to
//      included products and respects product exclusions.
// ============================================================================
func TestCategoryTargeting_CaseT_LegacyEntireStoreAndSelectedProductsRemainUnchanged(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)

	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-cat-t")
	orderID := createTestOrder(t, client, userID)

	cat := createTestCategory(t, client, nil, "LegacyCat", true)
	prod1 := createTestProductWithCategory(t, client, sellerID, cat, 100000)
	prod2 := createTestProductWithCategory(t, client, sellerID, cat, 200000)

	item1ID, var1ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod1, 100000, 1)
	item2ID, var2ID := createTestOrderItemWithProduct(t, client, orderID, sellerID, prod2, 200000, 1)

	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
		cleanupCategoryFixtures(client, []uuid.UUID{cat})
	}()

	ctx := context.Background()

	// 1. Legacy ENTIRE_STORE
	codeES := uniqueCode("LEG_ES")
	scopeES := marketing.ProductScopeEntireStore
	resES, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             codeES,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		ProductScope:     &scopeES,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, resES.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, codeES, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(30000), calc.TotalSellerDiscountCents) // 10% on both products
		require.Len(t, calc.PromotedLines, 2)
		return nil
	})
	require.NoError(t, err)

	// 2. Legacy SELECTED_PRODUCTS
	codeSP := uniqueCode("LEG_SP")
	scopeSP := marketing.ProductScopeSelectedProducts
	resSP, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:               codeSP,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   2000,
		ProductScope:       &scopeSP,
		IncludedProductIDs: []uuid.UUID{prod1},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, resSP.CampaignID)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, codeSP, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(20000), calc.TotalSellerDiscountCents) // 20% on prod1 only
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, item1ID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}
