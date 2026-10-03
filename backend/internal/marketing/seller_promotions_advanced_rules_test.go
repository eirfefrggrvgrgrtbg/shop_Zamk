package marketing_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// Helper to create test product belonging to a seller
func createTestProductForSeller(t *testing.T, client *postgres.Client, sellerID uuid.UUID, priceCents int64) uuid.UUID {
	ctx := context.Background()
	testutil.AssertTestDatabase(t, client.Pool)

	catID := uuid.New()
	_, err := client.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = client.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, $5, 'published', now(), now())`, prodID, sellerID, catID, uuid.New().String(), priceCents)
	require.NoError(t, err)
	return prodID
}

// ============================================================================
// MATRIX A: legacy/default ENTIRE_STORE promo unchanged
// ============================================================================
func TestMatrixA_LegacyDefaultEntireStorePromoUnchanged(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-a")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1) // 1,000 RUB
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 200000, 1) // 2,000 RUB

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("LEGACY")
	req := marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000, // 10%
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, marketing.ProductScopeEntireStore, res.ProductScope)
	assert.Nil(t, res.MaxDiscountCents)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(30000), calc.TotalSellerDiscountCents) // 100 + 200 = 300 RUB
		assert.Len(t, calc.PromotedLines, 2)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX B: SELECTED_PRODUCTS discounts selected product
// MATRIX C: SELECTED_PRODUCTS does not discount unselected product
// ============================================================================
func TestMatrixB_C_SelectedProductsDiscountsOnlySelected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-bc")
	orderID := createTestOrder(t, client, userID)

	itemAID, prodAID, varAID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1) // 1,000 RUB
	itemBID, prodBID, varBID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 200000, 1) // 2,000 RUB

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("SELPROD")
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000, // 10%
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prodAID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, marketing.ProductScopeSelectedProducts, res.ProductScope)
	assert.Equal(t, []uuid.UUID{prodAID}, res.IncludedProductIDs)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemAID, ProductID: prodAID, ProductVariantID: varAID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: itemBID, ProductID: prodBID, ProductVariantID: varBID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		// Total discount: only 10% on prodA = 100 RUB (10,000 cents)
		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, itemAID, calc.PromotedLines[0].OrderItemID)
		assert.Equal(t, int64(10000), calc.PromotedLines[0].TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX D: ENTIRE_STORE exclusion removes excluded product
// ============================================================================
func TestMatrixD_EntireStoreExclusionRemovesExcludedProduct(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-d")
	orderID := createTestOrder(t, client, userID)

	itemAID, prodAID, varAID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)
	itemBID, prodBID, varBID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 200000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("EXCLUDE")
	scope := marketing.ProductScopeEntireStore
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000, // 10%
		ProductScope:       &scope,
		ExcludedProductIDs: []uuid.UUID{prodBID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, marketing.ProductScopeEntireStore, res.ProductScope)
	assert.Equal(t, []uuid.UUID{prodBID}, res.ExcludedProductIDs)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemAID, ProductID: prodAID, ProductVariantID: varAID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: itemBID, ProductID: prodBID, ProductVariantID: varBID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, itemAID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX E: SELECTED_PRODUCTS exclusion overrides inclusion / contradictory payload rejected
// ============================================================================
func TestMatrixE_ContradictoryPayloadRejectedAtCreation(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	prodID := createTestProductForSeller(t, client, sellerID, 100000)

	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, nil)

	ctx := context.Background()
	code := uniqueCode("CONFLICT")
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prodID},
		ExcludedProductIDs: []uuid.UUID{prodID}, // Contradictory!
	}

	_, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrProductConflict)
}

// ============================================================================
// MATRIX F: SELECTED_PRODUCTS with no valid eligible cart line fails closed
// ============================================================================
func TestMatrixF_SelectedProductsWithNoEligibleLineFailsClosed(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-f")
	orderID := createTestOrder(t, client, userID)

	itemUnselID, prodUnselID, varUnselID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)
	prodOtherID := createTestProductForSeller(t, client, sellerID, 200000)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("NOELIG")
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prodOtherID}, // Not in cart
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemUnselID, ProductID: prodUnselID, ProductVariantID: varUnselID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoNotApplicable)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX G: foreign Seller product target rejected at creation
// ============================================================================
func TestMatrixG_ForeignSellerProductTargetRejectedAtCreation(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	seller1ID := createTestSeller(t, client)
	seller2ID := createTestSeller(t, client)
	foreignProdID := createTestProductForSeller(t, client, seller2ID, 100000)

	defer cleanupExactFixtures(client, []uuid.UUID{seller1ID, seller2ID}, nil, nil, nil)

	ctx := context.Background()
	code := uniqueCode("FOREIGN")
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{foreignProdID},
	}

	_, err := svc.CreateSellerPromotion(ctx, seller1ID, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrProductNotOwnedBySeller)
}

// ============================================================================
// MATRIX H: duplicate target rejected at creation
// ============================================================================
func TestMatrixH_DuplicateTargetRejectedAtCreation(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	prodID := createTestProductForSeller(t, client, sellerID, 100000)

	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, nil)

	ctx := context.Background()
	code := uniqueCode("DUP")
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prodID, prodID}, // Duplicate!
	}

	_, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrInvalidProductScope)
}

// ============================================================================
// MATRIX I: selected product from correct Seller accepted
// ============================================================================
func TestMatrixI_SelectedProductFromCorrectSellerAccepted(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	prodID := createTestProductForSeller(t, client, sellerID, 100000)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("CORRECT")
	scope := marketing.ProductScopeSelectedProducts
	req := marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prodID},
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, []uuid.UUID{prodID}, res.IncludedProductIDs)

	// Verify target table directly
	targets, err := repo.GetPromoCodeProductTargets(ctx, res.ID)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, prodID, targets[0].ProductID)
	assert.Equal(t, marketing.TargetTypeInclude, targets[0].TargetType)
}

// ============================================================================
// MATRIX J: multi-seller order only discounts promo Seller eligible lines
// ============================================================================
func TestMatrixJ_MultiSellerOrderOnlyDiscountsPromoSellerLines(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	seller1ID := createTestSeller(t, client)
	seller2ID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-j")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, seller1ID, uuid.Nil, 100000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, seller2ID, uuid.Nil, 200000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{seller1ID, seller2ID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("MULTISELLER")
	req := marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1500, // 15%
	}

	res, err := svc.CreateSellerPromotion(ctx, seller1ID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: seller1ID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: seller2ID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents) // 15% of 1000 RUB = 150 RUB
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, item1ID, calc.PromotedLines[0].OrderItemID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX K: excluded line does not count toward minimum eligible subtotal
// ============================================================================
func TestMatrixK_ExcludedLineDoesNotCountTowardMinSubtotal(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-k")
	orderID := createTestOrder(t, client, userID)

	itemAID, prodAID, varAID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 60000, 1)  // 600 RUB
	itemBID, prodBID, varBID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 80000, 1)  // 800 RUB

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("MINSUB")
	scope := marketing.ProductScopeEntireStore
	req := marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinOrderSubtotalCents: 100000, // 1,000 RUB required
		ProductScope:          &scope,
		ExcludedProductIDs:    []uuid.UUID{prodBID}, // prodB excluded
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemAID, ProductID: prodAID, ProductVariantID: varAID, SellerID: sellerID, BaseUnitPriceCents: 60000, Quantity: 1},
		{OrderItemID: itemBID, ProductID: prodBID, ProductVariantID: varBID, SellerID: sellerID, BaseUnitPriceCents: 80000, Quantity: 1},
	}

	// Cart total is 1400 RUB, but eligible subtotal is only 600 RUB < 1000 RUB
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoMinSubtotal)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX L: max discount below calculated caps exact order discount
// MATRIX M: max discount above calculated changes nothing
// MATRIX N: max discount NULL changes nothing
// ============================================================================
func TestMatrixL_M_N_MaxDiscountBehavior(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-lmn")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 1000000, 1) // 10,000 RUB

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()

	// L: Cap at 1,000 RUB (100,000 cents) below 20% of 10,000 = 2,000 RUB
	capL := int64(100000)
	codeL := uniqueCode("CAPL")
	resL, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             codeL,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 2000, // 20%
		MaxDiscountCents: &capL,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, resL.CampaignID)

	// M: Cap at 5,000 RUB (500,000 cents) above 20% of 10,000 = 2,000 RUB
	capM := int64(500000)
	codeM := uniqueCode("CAPM")
	resM, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             codeM,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 2000, // 20%
		MaxDiscountCents: &capM,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, resM.CampaignID)

	// N: Cap NULL
	codeN := uniqueCode("CAPN")
	resN, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             codeN,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 2000, // 20%
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, resN.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 1000000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		// Test L: capped at exactly 100,000 cents
		calcL, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, codeL, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(100000), calcL.TotalSellerDiscountCents)

		// Test M: cap above calculated produces full 200,000 cents
		calcM, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, codeM, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(200000), calcM.TotalSellerDiscountCents)

		// Test N: NULL cap produces full 200,000 cents
		calcN, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, codeN, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(200000), calcN.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX O: capped discount allocation is cent-exact across multiple lines
// MATRIX P: deterministic remainder allocation
// ============================================================================
func TestMatrixO_P_CappedDiscountAllocationCentExactAndDeterministic(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-op")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)
	item3ID, prod3ID, var3ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()

	// 20% on 3,000 RUB = 600 RUB (60,000 cents). Cap at 10,001 cents.
	capOP := int64(10001)
	code := uniqueCode("EXACTCENT")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 2000,
		MaxDiscountCents: &capOP,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
		{OrderItemID: item3ID, ProductID: prod3ID, ProductVariantID: var3ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)

		// O: Total discount must equal EXACTLY 10,001 cents
		assert.Equal(t, int64(10001), calc.TotalSellerDiscountCents)

		var sumAllocated int64
		for _, line := range calc.PromotedLines {
			sumAllocated += line.TotalSellerDiscountCents
		}
		assert.Equal(t, int64(10001), sumAllocated)

		// P: Deterministic remainder: 10001 / 3 = 3334, 3334, 3333 or similar exact integer split
		assert.Len(t, calc.PromotedLines, 3)
		assert.True(t, calc.PromotedLines[0].TotalSellerDiscountCents >= 3333)
		assert.True(t, calc.PromotedLines[1].TotalSellerDiscountCents >= 3333)
		assert.True(t, calc.PromotedLines[2].TotalSellerDiscountCents >= 3333)

		// Rerun same calculation: must be identical
		calc2, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, calc.PromotedLines, calc2.PromotedLines)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX Q: FIXED promo + max cap behaves correctly
// ============================================================================
func TestMatrixQ_FixedPromoPlusMaxCap(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-q")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 200000, 1) // 2,000 RUB

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()

	// Fixed 1,000 RUB capped at 600 RUB (60,000 cents)
	capQ := int64(60000)
	code := uniqueCode("FIXCAP")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                    code,
		DiscountType:            marketing.DiscountTypeFixed,
		DiscountValueFixedCents: 100000, // 1,000 RUB
		MaxDiscountCents:        &capQ,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(60000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX R: max_discount_cents <= 0 rejected when supplied
// ============================================================================
func TestMatrixR_MaxDiscountLessThanZeroRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)

	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, nil)

	ctx := context.Background()
	negCap := int64(-500)
	req := marketing.CreateSellerPromoRequest{
		Code:             uniqueCode("NEG"),
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		MaxDiscountCents: &negCap,
	}

	_, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrInvalidMaxDiscount)

	zeroCap := int64(0)
	reqZero := marketing.CreateSellerPromoRequest{
		Code:             uniqueCode("ZERO"),
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		MaxDiscountCents: &zeroCap,
	}
	_, err = svc.CreateSellerPromotion(ctx, sellerID, reqZero)
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrInvalidMaxDiscount)
}

// ============================================================================
// MATRIX S: advanced fields immutable after creation
// ============================================================================
func TestMatrixS_AdvancedFieldsImmutableAfterCreation(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("IMMUT")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	_, router := setupMarketingHTTPHandler(svc)

	// Attempt to patch productScope
	patchBody := []byte(`{"productScope": "SELECTED_PRODUCTS"}`)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/seller/promotions/%s", res.ID), bytes.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(newTestContextWithSeller(sellerID))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Attempt to patch maxDiscountCents
	patchBody2 := []byte(`{"maxDiscountCents": 50000}`)
	req2 := httptest.NewRequest("PATCH", fmt.Sprintf("/api/seller/promotions/%s", res.ID), bytes.NewReader(patchBody2))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(newTestContextWithSeller(sellerID))

	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusBadRequest, rr2.Code)
}

// ============================================================================
// MATRIX T: pause/reactivate still works with targeting and max cap
// ============================================================================
func TestMatrixT_PauseReactivateStillWorks(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-t")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	capT := int64(5000)
	scope := marketing.ProductScopeSelectedProducts
	code := uniqueCode("PAUSE")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prodID},
		MaxDiscountCents:   &capT,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Pause
	falseVal := false
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, res.ID, marketing.UpdateSellerPromoRequest{
		IsActive: &falseVal,
	})
	require.NoError(t, err)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.Error(t, err)
		assert.ErrorIs(t, err, marketing.ErrPromoInactive)
		return nil
	})
	require.NoError(t, err)

	// Reactivate
	trueVal := true
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, res.ID, marketing.UpdateSellerPromoRequest{
		IsActive: &trueVal,
	})
	require.NoError(t, err)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(5000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX U: payment success consumes usage exactly once
// MATRIX V: terminal payment failure releases existing reservation exactly once
// ============================================================================
func TestMatrixU_V_PaymentSuccessConsumesUsageAndFailureReleases(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	user1 := createTestUser(t, client, "usr-u1")
	user2 := createTestUser(t, client, "usr-u2")
	order1 := createTestOrder(t, client, user1)
	order2 := createTestOrder(t, client, user2)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, order1, sellerID, uuid.Nil, 100000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, order2, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user1, user2}, []uuid.UUID{order1, order2}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("USAGE")
	scope := marketing.ProductScopeSelectedProducts
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prod1ID, prod2ID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items1 := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}
	items2 := []marketing.PromotedOrderItemInput{
		{OrderItemID: item2ID, ProductID: prod2ID, ProductVariantID: var2ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	// 1) U: Order 1 reserves then consumes (payment success)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user1, code, items1, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, order1, user1, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, order1)
	})
	require.NoError(t, err)

	usage1, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage1.Status)

	// 2) V: Order 2 reserves then releases (terminal failure)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user2, code, items2, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, order2, user2, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	usage2Reserve, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order2)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage2Reserve.Status)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, order2, "terminal failure")
	})
	require.NoError(t, err)

	usage2Release, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order2)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage2Release.Status)
}

// ============================================================================
// MATRIX W: retry / unknown / duplicate consume handling
// ============================================================================
func TestMatrixW_IdempotentConsumeAndRelease(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-w")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("RETRY")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Consume once
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	// Repeated consume on already completed order does not double count
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX X: order_item_promotions snapshots remain authoritative
// ============================================================================
func TestMatrixX_OrderItemPromotionsSnapshotsRemainAuthoritative(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-x")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("SNAP")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		err = svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
		require.NoError(t, err)
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	// Verify order_item_promotions snapshot row exists
	var snapCampaignID, snapPromoID uuid.UUID
	var snapSellerDisc int64
	err = client.Pool.QueryRow(ctx, `
		SELECT campaign_id, promo_code_id, total_seller_discount_cents
		FROM order_item_promotions
		WHERE order_item_id = $1
	`, itemID).Scan(&snapCampaignID, &snapPromoID, &snapSellerDisc)
	require.NoError(t, err)
	assert.Equal(t, res.CampaignID, snapCampaignID)
	assert.Equal(t, res.ID, snapPromoID)
	assert.Equal(t, int64(10000), snapSellerDisc)
}

// ============================================================================
// MATRIX Y: existing MARKETING.1D2 promos work after migration defaults
// ============================================================================
func TestMatrixY_ExistingPromosWorkAfterMigrationDefaults(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-y")
	orderID := createTestOrder(t, client, userID)

	itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	// Simulate row inserted prior to migration (with default ENTIRE_STORE and NULL max_discount_cents)
	code := uniqueCode("MIG98")
	campID := uuid.New()
	campaignIDs = append(campaignIDs, campID)

	_, err := client.Pool.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, seller_id, title, funding_mode, status, discount_type, seller_discount_bps, created_at, updated_at)
		VALUES ($1, $2, 'Old Campaign', 'seller', 'active', 'percent', 1000, now(), now())
	`, campID, sellerID)
	require.NoError(t, err)

	promoID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, product_scope, max_discount_cents, created_at, updated_at)
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
		assert.Equal(t, promoID, calc.PromoCodeID)
		return nil
	})
	require.NoError(t, err)
}

// ============================================================================
// MATRIX W: Scoped + Capped Seller Promo Payment Lifecycle & Invariants
// ============================================================================
func TestMatrixW_ScopedCappedPromo_PaymentLifecycleTransitions(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	user1 := createTestUser(t, client, "usr-w1")
	user2 := createTestUser(t, client, "usr-w2")
	order1 := createTestOrder(t, client, user1)
	order2 := createTestOrder(t, client, user2)

	prod1ID := createTestProductForSeller(t, client, sellerID, 800000)
	prod2ID := createTestProductForSeller(t, client, sellerID, 200000)

	item1_1ID, _, var1_1ID := createTestOrderItemInFulfillment(t, client, order1, sellerID, uuid.Nil, 800000, 1)
	item1_2ID, _, var1_2ID := createTestOrderItemInFulfillment(t, client, order1, sellerID, uuid.Nil, 200000, 1)
	_, err := client.Pool.Exec(ctx, "UPDATE order_items SET product_id = $1 WHERE id = $2", prod1ID, item1_1ID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE order_items SET product_id = $1 WHERE id = $2", prod2ID, item1_2ID)
	require.NoError(t, err)

	item2_1ID, _, var2_1ID := createTestOrderItemInFulfillment(t, client, order2, sellerID, uuid.Nil, 800000, 1)
	item2_2ID, _, var2_2ID := createTestOrderItemInFulfillment(t, client, order2, sellerID, uuid.Nil, 200000, 1)
	_, err = client.Pool.Exec(ctx, "UPDATE order_items SET product_id = $1 WHERE id = $2", prod1ID, item2_1ID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE order_items SET product_id = $1 WHERE id = $2", prod2ID, item2_2ID)
	require.NoError(t, err)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user1, user2}, []uuid.UUID{order1, order2}, campaignIDs)

	code := uniqueCode("SCOPECAPPAY")
	scope := marketing.ProductScopeSelectedProducts
	capCents := int64(100000) // 1 000 RUB cap

	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   2000, // 20%
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prod1ID},
		MaxDiscountCents:   &capCents,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	items1 := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1_1ID, ProductID: prod1ID, ProductVariantID: var1_1ID, SellerID: sellerID, BaseUnitPriceCents: 800000, Quantity: 1},
		{OrderItemID: item1_2ID, ProductID: prod2ID, ProductVariantID: var1_2ID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	// ------------------------------------------------------------------------
	// Phase 1: Order 1 Checkout & Reservation (Step A)
	// ------------------------------------------------------------------------
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user1, code, items1, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(100000), calc.TotalSellerDiscountCents)
		assert.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, item1_1ID, calc.PromotedLines[0].OrderItemID)
		return svc.ReserveCheckoutPromoTx(ctx, tx, order1, user1, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	usage1, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage1.Status, "Step A: Promo reservation must exist and be 'reserved'")

	order1Total := int64(900000)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = $1 WHERE id = $2", order1Total, order1)
	require.NoError(t, err)

	// ------------------------------------------------------------------------
	// Phase 2: Provider Init ambiguous/unknown error (Step B)
	// ------------------------------------------------------------------------
	provPaymentID1 := "tb-hook-w1-" + uuid.NewString()
	shouldFailInit := true

	mockProv1 := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			if shouldFailInit {
				return payments.ProviderCreatePaymentResult{}, errors.New("ambiguous provider timeout")
			}
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID1,
				PaymentURL:        "https://pay.tbank.ru/w1",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			statusStr := string(body)
			normStatus := "pending"
			if statusStr == "CONFIRMED" {
				normStatus = "succeeded"
			} else if statusStr == "REJECTED" {
				normStatus = "cancelled"
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID1,
				OrderID:           order1.String(),
				Status:            normStatus,
				ProviderStatus:    statusStr,
				AmountCents:       order1Total,
				EventKey:          "key-" + statusStr + "-" + uuid.NewString(),
			}, nil
		},
	}
	paySvc1 := setupTestPaymentsService(t, client, svc, mockProv1)

	_, err = paySvc1.CreatePayment(ctx, user1, order1, "card")
	require.Error(t, err, "Ambiguous provider error simulated")

	usage1, err = repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage1.Status, "Step B: Promo usage remains reserved on ambiguous provider error")

	// Verify cancellation remains blocked while payment attempt is outcome-uncertain
	ordersSvc := setupTestOrdersService(t, client, svc)
	err = ordersSvc.CancelCustomerOrder(ctx, user1, order1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, orders.ErrOrderNotCancellable), "Cancellation must be blocked while payment outcome is uncertain")

	// ------------------------------------------------------------------------
	// Phase 3: Provider delivers AUTH_FAIL (Step C: Non-terminal retryable event)
	// ------------------------------------------------------------------------
	err = paySvc1.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	var pStatus, oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", order1).Scan(&pStatus)
	require.NoError(t, err)
	assert.True(t, pStatus == "pending" || pStatus == "created", "Payment remains non-terminal on AUTH_FAIL")

	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", order1).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "awaiting_payment", oStatus, "Order remains awaiting_payment on AUTH_FAIL")

	usage1, err = repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage1.Status, "Step C: Promo usage remains reserved on AUTH_FAIL")

	// ------------------------------------------------------------------------
	// Phase 4: AUTH_FAIL -> CONFIRMED (Step D)
	// ------------------------------------------------------------------------
	err = paySvc1.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", order1).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", order1).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)

	usage1, err = repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage1.Status, "Step D: Promo usage consumed on CONFIRMED")

	var snapCampaignID, snapPromoID uuid.UUID
	var snapSellerDisc int64
	err = client.Pool.QueryRow(ctx, `
		SELECT campaign_id, promo_code_id, total_seller_discount_cents
		FROM order_item_promotions
		WHERE order_item_id = $1
	`, item1_1ID).Scan(&snapCampaignID, &snapPromoID, &snapSellerDisc)
	require.NoError(t, err)
	assert.Equal(t, res.CampaignID, snapCampaignID)
	assert.Equal(t, res.ID, snapPromoID)
	assert.Equal(t, int64(100000), snapSellerDisc, "Snapshot discount matches exact 1000 RUB cap")

	// ------------------------------------------------------------------------
	// Phase 5: Stale AUTH_FAIL arrives after CONFIRMED (Step F1)
	// ------------------------------------------------------------------------
	err = paySvc1.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", order1).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus, "Payment must not be downgraded by stale AUTH_FAIL")

	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", order1).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus, "Order must remain paid")

	usage1, err = repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order1)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage1.Status, "Promo usage must not be downgraded by stale AUTH_FAIL")

	// ------------------------------------------------------------------------
	// Phase 6: Order 2: AUTH_FAIL -> REJECTED (Step E) & Stale AUTH_FAIL (Step F2)
	// ------------------------------------------------------------------------
	items2 := []marketing.PromotedOrderItemInput{
		{OrderItemID: item2_1ID, ProductID: prod1ID, ProductVariantID: var2_1ID, SellerID: sellerID, BaseUnitPriceCents: 800000, Quantity: 1},
		{OrderItemID: item2_2ID, ProductID: prod2ID, ProductVariantID: var2_2ID, SellerID: sellerID, BaseUnitPriceCents: 200000, Quantity: 1},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user2, code, items2, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(100000), calc.TotalSellerDiscountCents)
		return svc.ReserveCheckoutPromoTx(ctx, tx, order2, user2, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	order2Total := int64(900000)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = $1 WHERE id = $2", order2Total, order2)
	require.NoError(t, err)

	provPaymentID2 := "tb-hook-w2-" + uuid.NewString()
	mockProv2 := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID2,
				PaymentURL:        "https://pay.tbank.ru/w2",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			statusStr := string(body)
			normStatus := "pending"
			if statusStr == "REJECTED" {
				normStatus = "cancelled"
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID2,
				OrderID:           order2.String(),
				Status:            normStatus,
				ProviderStatus:    statusStr,
				AmountCents:       order2Total,
				EventKey:          "key-" + statusStr + "-" + uuid.NewString(),
			}, nil
		},
	}
	paySvc2 := setupTestPaymentsService(t, client, svc, mockProv2)

	_, err = paySvc2.CreatePayment(ctx, user2, order2, "card")
	require.NoError(t, err)

	err = paySvc2.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	usage2, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order2)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage2.Status, "Promo usage remains reserved on AUTH_FAIL")

	err = paySvc2.HandleWebhook(ctx, nil, []byte("REJECTED"))
	require.NoError(t, err)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", order2).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)

	usage2, err = repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order2)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage2.Status, "Step E: Promo usage released on REJECTED")

	err = paySvc2.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", order2).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus, "Step F2: Payment must not be reopened by stale AUTH_FAIL")

	usage2, err = repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, order2)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage2.Status, "Promo usage must remain released")
}

// ============================================================================
// TARGET DELETE: SELECTED_PRODUCTS with deleted target fails closed
// ============================================================================
func TestTargetDelete_ZeroIncludesFailsClosed(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestUser(t, client, "usr-target-del")
	orderID := createTestOrder(t, client, userID)

	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, orderID, sellerID, uuid.Nil, 100000, 1)

	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)

	ctx := context.Background()
	code := uniqueCode("DELTGT")
	scope := marketing.ProductScopeSelectedProducts

	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:               code,
		DiscountType:       marketing.DiscountTypePercent,
		DiscountValueBps:   1000,
		ProductScope:       &scope,
		IncludedProductIDs: []uuid.UUID{prod1ID},
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// Verify target link was created
	var count int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM promo_code_product_targets WHERE promo_code_id = $1", res.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Now simulate deleting the target link (e.g. FK cascade or target unlinked/deleted)
	_, err = client.Pool.Exec(ctx, "DELETE FROM promo_code_product_targets WHERE promo_code_id = $1", res.ID)
	require.NoError(t, err)

	items := []marketing.PromotedOrderItemInput{
		{OrderItemID: item1ID, ProductID: prod1ID, ProductVariantID: var1ID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
	}

	// ValidateAndCalculateCheckoutPromoTx MUST fail closed with ErrPromoNotApplicable
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		assert.Nil(t, calc, "Calc must be nil when zero includes remain")
		assert.ErrorIs(t, err, marketing.ErrPromoNotApplicable, "Promo must fail closed with ErrPromoNotApplicable, never ENTIRE_STORE")
		return err
	})
	require.Error(t, err)

	// Assert NO reservation exists in promo_code_usages
	var usageCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM promo_code_usages WHERE promo_code_id = $1", res.ID).Scan(&usageCount)
	require.NoError(t, err)
	assert.Equal(t, 0, usageCount, "No usage reservation must exist after fail-closed check")
}
