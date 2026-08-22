package products_test

import (
	"context"
	"testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestVariantIdentityPreservation(t *testing.T) {
	dbClient, svc, sellerUserID := setupTestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	catID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Edit Category', $2, false)", catID, "edit-" + uuid.New().String())
	require.NoError(t, err)

	req := products.CreateProductRequest{
		Title: "Edit Product",
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:   func(s string) *string { return &s }("SEM-EDIT-1"),
				PriceCents:  func(i int64) *int64 { return &i }(1000),
			},
		},
	}
	
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
	require.NoError(t, err)
	
	fetched, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
	require.NoError(t, err)
	
	originalVariantID := fetched.Variants[0].ID

	// Update the product with the same variant ID
	updateReq := products.UpdateProductRequest{
		Title: func(s string) *string { return &s }("Updated Title"),
		Variants: []products.ProductVariantRequest{
			{
				ID:          &originalVariantID,
				SellerSKU:   func(s string) *string { return &s }("SEM-EDIT-1"),
				PriceCents:  func(i int64) *int64 { return &i }(2000),
			},
		},
	}
	
	_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updateReq)
	require.NoError(t, err)
	
	fetched2, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
	require.NoError(t, err)
	
	require.Equal(t, 1, len(fetched2.Variants))
	require.Equal(t, originalVariantID, fetched2.Variants[0].ID)
	require.Equal(t, int64(2000), *fetched2.Variants[0].PriceCents)
}
