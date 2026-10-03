package products_test

import (
	"context"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSellerProductsList_SearchAndPagination(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Create 3 distinct products for this seller
	price := int64(100000)

	p1Req := products.CreateProductRequest{
		Title:      "Exclusive Silk Blouse Alpha",
		Slug:       ptr(uuid.New().String()),
		PriceCents: price,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				SKU:       ptr("SKU-ALPHA-01"),
				SellerSKU: ptr("SSKU-ALPHA-01"),
				Barcode:   ptr("BAR-ALPHA-01"),
			},
		},
	}
	p1, err := svc.CreateProductForSeller(ctx, sellerUserID, p1Req)
	require.NoError(t, err)

	p2Req := products.CreateProductRequest{
		Title:      "Modern Denim Jacket Beta",
		Slug:       ptr(uuid.New().String()),
		PriceCents: price,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				SKU:       ptr("SKU-BETA-02"),
				SellerSKU: ptr("SSKU-BETA-02"),
				Barcode:   ptr("BAR-BETA-02"),
			},
		},
	}
	p2, err := svc.CreateProductForSeller(ctx, sellerUserID, p2Req)
	require.NoError(t, err)

	p3Req := products.CreateProductRequest{
		Title:      "Wool Trench Coat Gamma",
		Slug:       ptr(uuid.New().String()),
		PriceCents: price,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				SKU:       ptr("SKU-GAMMA-03"),
				SellerSKU: ptr("SSKU-GAMMA-03"),
				Barcode:   ptr("BAR-GAMMA-03"),
			},
		},
	}
	p3, err := svc.CreateProductForSeller(ctx, sellerUserID, p3Req)
	require.NoError(t, err)

	// Another seller's product to verify scoping isolation
	_, otherSvc, otherSellerUserID := setupBlockATestDB(t)
	otherReq := products.CreateProductRequest{
		Title:      "Other Seller Silk Item",
		Slug:       ptr(uuid.New().String()),
		PriceCents: price,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				SKU:       ptr("SKU-OTHER-99"),
				SellerSKU: ptr("SSKU-OTHER-99"),
			},
		},
	}
	_, err = otherSvc.CreateProductForSeller(ctx, otherSellerUserID, otherReq)
	require.NoError(t, err)

	t.Run("list without search returns all seller products and accurate totalCount", func(t *testing.T) {
		res, err := svc.ListSellerProducts(ctx, sellerUserID, "", 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, res.TotalCount)
		assert.Len(t, res.Items, 3)

		ids := []uuid.UUID{res.Items[0].ID, res.Items[1].ID, res.Items[2].ID}
		assert.Contains(t, ids, p1.ID)
		assert.Contains(t, ids, p2.ID)
		assert.Contains(t, ids, p3.ID)
	})

	t.Run("pagination with limit and offset works correctly", func(t *testing.T) {
		page1, err := svc.ListSellerProducts(ctx, sellerUserID, "", 2, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, page1.TotalCount)
		assert.Len(t, page1.Items, 2)

		page2, err := svc.ListSellerProducts(ctx, sellerUserID, "", 2, 2)
		require.NoError(t, err)
		assert.Equal(t, 3, page2.TotalCount)
		assert.Len(t, page2.Items, 1)

		// Page 1 and Page 2 shouldn't overlap
		assert.NotEqual(t, page1.Items[0].ID, page2.Items[0].ID)
		assert.NotEqual(t, page1.Items[1].ID, page2.Items[0].ID)
	})

	t.Run("search by title", func(t *testing.T) {
		res, err := svc.ListSellerProducts(ctx, sellerUserID, "Denim", 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalCount)
		require.Len(t, res.Items, 1)
		assert.Equal(t, p2.ID, res.Items[0].ID)
	})

	t.Run("search by product ID", func(t *testing.T) {
		res, err := svc.ListSellerProducts(ctx, sellerUserID, p3.ID.String(), 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalCount)
		require.Len(t, res.Items, 1)
		assert.Equal(t, p3.ID, res.Items[0].ID)
	})

	t.Run("search by variant SKU", func(t *testing.T) {
		res, err := svc.ListSellerProducts(ctx, sellerUserID, "SKU-ALPHA-01", 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalCount)
		require.Len(t, res.Items, 1)
		assert.Equal(t, p1.ID, res.Items[0].ID)
	})

	t.Run("search by variant seller SKU", func(t *testing.T) {
		res, err := svc.ListSellerProducts(ctx, sellerUserID, "SSKU-BETA-02", 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalCount)
		require.Len(t, res.Items, 1)
		assert.Equal(t, p2.ID, res.Items[0].ID)
	})

	t.Run("search by variant barcode", func(t *testing.T) {
		require.NotEmpty(t, p3.Variants)
		require.NotNil(t, p3.Variants[0].Barcode)
		res, err := svc.ListSellerProducts(ctx, sellerUserID, *p3.Variants[0].Barcode, 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalCount)
		require.Len(t, res.Items, 1)
		assert.Equal(t, p3.ID, res.Items[0].ID)
	})

	t.Run("search preserves seller scoping isolation", func(t *testing.T) {
		// Searching for "Silk" should only return p1 (Exclusive Silk Blouse Alpha), never other seller's Silk item
		res, err := svc.ListSellerProducts(ctx, sellerUserID, "Silk", 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalCount)
		require.Len(t, res.Items, 1)
		assert.Equal(t, p1.ID, res.Items[0].ID)
	})

	t.Run("search with no matches returns empty list and 0 count", func(t *testing.T) {
		res, err := svc.ListSellerProducts(ctx, sellerUserID, "NONEXISTENT_KEYWORD_XYZ", 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, res.TotalCount)
		assert.Empty(t, res.Items)
	})
}
