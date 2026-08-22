package products_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestPriceOnlyUpdateIntegration(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Setup another seller
	_, _, foreignUserID := setupBlockATestDB(t)

	// Create Product
	priceA := int64(1000)
	req := products.CreateProductRequest{
		Title: "Price Test Product",
		Slug: func(s string) *string { return &s }(uuid.New().String()),
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:  func(s string) *string { return &s }("SKU-PRICE-1"),
				PriceCents: &priceA,
			},
		},
	}
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
	require.NoError(t, err)
	require.Len(t, p.Variants, 1)
	variantID := p.Variants[0].ID

	// Publish product and set revisions count to some known state
	_, err = db.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", p.ID)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO product_revisions (id, product_id, status, content_snapshot) VALUES ($1, $2, 'approved', '{}')", uuid.New(), p.ID)
	require.NoError(t, err)

	// Get base revision count
	var revCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_revisions WHERE product_id = $1", p.ID).Scan(&revCount)
	require.NoError(t, err)

	// Price only update to B
	priceB := int64(1500)
	updateReq := products.UpdateProductPricesRequest{
		Variants: []products.VariantPriceUpdateRequest{
			{ID: variantID, PriceCents: priceB},
		},
	}
	
	err = svc.UpdateProductPrices(ctx, sellerUserID, p.ID, updateReq)
	require.NoError(t, err)

	// Assertions
	pUpdated, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "published", string(pUpdated.Status))
	assert.Equal(t, priceB, *pUpdated.Variants[0].PriceCents)

	var newRevCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_revisions WHERE product_id = $1", p.ID).Scan(&newRevCount)
	require.NoError(t, err)
	assert.Equal(t, revCount, newRevCount)

	// Test foreign Seller Product -> rejected
	err = svc.UpdateProductPrices(ctx, foreignUserID, p.ID, updateReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "product not found")

	// Test foreign Product Variant -> rejected
	reqForeign := products.CreateProductRequest{
		Title: "Foreign Product",
		Slug: func(s string) *string { return &s }(uuid.New().String()),
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:  func(s string) *string { return &s }("SKU-PRICE-FOREIGN"),
				PriceCents: &priceA,
			},
		},
	}
	pForeign, err := svc.CreateProductForSeller(ctx, foreignUserID, reqForeign)
	require.NoError(t, err)
	foreignVariantID := pForeign.Variants[0].ID

	updateReqForeignVar := products.UpdateProductPricesRequest{
		Variants: []products.VariantPriceUpdateRequest{
			{ID: foreignVariantID, PriceCents: 2000},
		},
	}
	err = svc.UpdateProductPrices(ctx, sellerUserID, p.ID, updateReqForeignVar)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to product")

	// Test price <= 0 -> rejected
	
	updateReqNeg := products.UpdateProductPricesRequest{
		Variants: []products.VariantPriceUpdateRequest{
			{ID: variantID, PriceCents: -100},
		},
	}
	err = svc.UpdateProductPrices(ctx, sellerUserID, p.ID, updateReqNeg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be greater than 0")
}
