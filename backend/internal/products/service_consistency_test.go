package products_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductPricePersistenceAndReadback(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	priceRubles1222 := int64(122200) // 1222.00 RUB in kopecks

	// 1. Create product where seller only sets variant price 1222.00 RUB and product-level priceCents is 0
	createReq := products.CreateProductRequest{
		Title:      "1222 RUB Product",
		Slug:       func(s string) *string { return &s }(uuid.New().String()),
		PriceCents: 0,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:  func(s string) *string { return &s }("SKU-1222-VAR1"),
				PriceCents: &priceRubles1222,
			},
		},
	}

	pCreated, err := svc.CreateProductForSeller(ctx, sellerUserID, createReq)
	require.NoError(t, err)
	assert.Equal(t, priceRubles1222, pCreated.PriceCents, "CreateProductForSeller must persist and return variant price at product level")
	require.Len(t, pCreated.Variants, 1)
	assert.Equal(t, priceRubles1222, *pCreated.Variants[0].PriceCents)

	// 2. Read back via GetSellerProduct
	pSellerGet, err := svc.GetSellerProduct(ctx, sellerUserID, pCreated.ID)
	require.NoError(t, err)
	assert.Equal(t, priceRubles1222, pSellerGet.PriceCents, "GetSellerProduct must return 1222.00 RUB")
	require.Len(t, pSellerGet.Variants, 1)
	assert.Equal(t, priceRubles1222, *pSellerGet.Variants[0].PriceCents)

	// 3. Read back via ListSellerProducts
	pSellerList, err := svc.ListSellerProducts(ctx, sellerUserID, 50, 0)
	require.NoError(t, err)
	var foundSellerProd *products.Product
	for i := range pSellerList.Items {
		if pSellerList.Items[i].ID == pCreated.ID {
			foundSellerProd = &pSellerList.Items[i]
			break
		}
	}
	require.NotNil(t, foundSellerProd, "Created product must be in ListSellerProducts")
	assert.Equal(t, priceRubles1222, foundSellerProd.PriceCents, "ListSellerProducts must return 1222.00 RUB")

	// 4. Read back via GetAdminProductDetail
	pAdminGet, err := svc.GetAdminProductDetail(ctx, pCreated.ID)
	require.NoError(t, err)
	assert.Equal(t, priceRubles1222, pAdminGet.PriceCents, "GetAdminProductDetail must return 1222.00 RUB")

	// 5. Read back via ListAdminProducts
	pAdminList, err := svc.ListAdminProducts(ctx, products.AdminProductFilter{
		SellerID: &pCreated.SellerID,
	}, 50, 0)
	require.NoError(t, err)
	var foundAdminProd *products.Product
	for i := range pAdminList.Items {
		if pAdminList.Items[i].ID == pCreated.ID {
			foundAdminProd = &pAdminList.Items[i]
			break
		}
	}
	require.NotNil(t, foundAdminProd, "Created product must be in ListAdminProducts")
	assert.Equal(t, priceRubles1222, foundAdminProd.PriceCents, "ListAdminProducts must return 1222.00 RUB")

	// 6. Update variant price to 1555.00 RUB via UpdateProductPrices
	newPrice := int64(155500)
	variantID := pCreated.Variants[0].ID
	updatePriceReq := products.UpdateProductPricesRequest{
		Variants: []products.VariantPriceUpdateRequest{
			{ID: variantID, PriceCents: newPrice},
		},
	}
	err = svc.UpdateProductPrices(ctx, sellerUserID, pCreated.ID, updatePriceReq)
	require.NoError(t, err)

	// Verify updated price reads back across all APIs
	pUpdatedSellerGet, err := svc.GetSellerProduct(ctx, sellerUserID, pCreated.ID)
	require.NoError(t, err)
	assert.Equal(t, newPrice, pUpdatedSellerGet.PriceCents, "GetSellerProduct must return updated 1555.00 RUB")
	assert.Equal(t, newPrice, *pUpdatedSellerGet.Variants[0].PriceCents)

	pUpdatedAdminGet, err := svc.GetAdminProductDetail(ctx, pCreated.ID)
	require.NoError(t, err)
	assert.Equal(t, newPrice, pUpdatedAdminGet.PriceCents, "GetAdminProductDetail must return updated 1555.00 RUB")
}

func TestModerationFirstAttemptPersistsAndExposesStatus(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	adminUserID := uuid.New()
	_, err := db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'admin', 'Admin User')", adminUserID, fmt.Sprintf("admin-%s@test.com", adminUserID))
	require.NoError(t, err)

	priceCents := int64(250000)

	// Create Product
	createReq := products.CreateProductRequest{
		Title:      "Moderation Target Product",
		Slug:       func(s string) *string { return &s }(uuid.New().String()),
		PriceCents: priceCents,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:  func(s string) *string { return &s }("SKU-MOD-1"),
				PriceCents: &priceCents,
			},
		},
	}
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, createReq)
	require.NoError(t, err)

	// Put product directly into pending_moderation
	now := p.CreatedAt
	_, err = db.Pool.Exec(ctx, "UPDATE products SET status = 'pending_moderation', submitted_at = $1 WHERE id = $2", now, p.ID)
	require.NoError(t, err)

	// Verify pending_moderation state
	pPending, err := svc.GetAdminProductDetail(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, products.StatusPendingModeration, pPending.Status)

	// Approve product on the VERY FIRST attempt directly from pending_moderation
	approveComment := "Approved on first attempt"
	err = svc.ApproveProduct(ctx, adminUserID, p.ID, &approveComment)
	require.NoError(t, err, "First moderation approval request must succeed without preliminary steps")

	// Verify state immediately reflects server truth on subsequent GETs
	pAdminApproved, err := svc.GetAdminProductDetail(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, products.StatusApproved, pAdminApproved.Status, "Product status must be approved in admin detail")
	assert.NotNil(t, pAdminApproved.ApprovedAt, "ApprovedAt must be populated")

	pSellerApproved, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, products.StatusApproved, pSellerApproved.Status, "Product status must be approved in seller detail")

	// Verify ListSellerProducts reflects approved status
	sellerList, err := svc.ListSellerProducts(ctx, sellerUserID, 50, 0)
	require.NoError(t, err)
	for _, item := range sellerList.Items {
		if item.ID == p.ID {
			assert.Equal(t, products.StatusApproved, item.Status, "Seller list must reflect approved status")
		}
	}

	// Verify ListAdminProducts with status=approved contains the product
	approvedFilterStatus := "approved"
	adminApprovedList, err := svc.ListAdminProducts(ctx, products.AdminProductFilter{
		Status:   &approvedFilterStatus,
		SellerID: &p.SellerID,
	}, 50, 0)
	require.NoError(t, err)
	foundApproved := false
	for _, item := range adminApprovedList.Items {
		if item.ID == p.ID {
			foundApproved = true
			assert.Equal(t, products.StatusApproved, item.Status)
		}
	}
	assert.True(t, foundApproved, "Approved product must be in ListAdminProducts with status=approved")

	// Verify moderation history has the log entry
	logs, err := svc.GetAdminProductModerationHistory(ctx, p.ID)
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	assert.Equal(t, products.StatusApproved, logs[len(logs)-1].ToStatus)
}
