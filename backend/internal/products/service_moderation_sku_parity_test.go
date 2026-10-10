package products_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestSubmitProductToModeration_InactiveVariantSKUParity(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create category and color/size
	catID := uuid.New()
	_, err := db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required, is_active) VALUES ($1, 'Худи', $2, false, true)", catID, "cat-hoodie-"+uuid.New().String())
	require.NoError(t, err)

	var colorID uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT id FROM colors WHERE is_active = true LIMIT 1").Scan(&colorID)
	require.NoError(t, err)

	var sizeValID uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT id FROM size_values WHERE is_active = true LIMIT 1").Scan(&sizeValID)
	require.NoError(t, err)

	slug := "prod-long-" + uuid.New().String()
	priceVal := int64(250000)
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Лонг",
		Slug:       &slug,
		CategoryID: &catID,
		PriceCents: priceVal,
	})
	require.NoError(t, err)

	// Add 3 valid cropped images (1 main)
	repo := products.NewRepository(db.Pool)
	img1 := uuid.New()
	require.NoError(t, repo.AddProductImage(ctx, &products.ProductImage{
		ID:        img1,
		ProductID: p.ID,
		ImageURL:  "https://storage.zamk.test/img1.jpg",
		IsMain:    true,
	}))
	require.NoError(t, repo.UpdateProductImageCrop(ctx, img1, 0, 0, 1.0, 1.0, "https://storage.zamk.test/rend1.jpg", "rend1.jpg"))

	img2 := uuid.New()
	require.NoError(t, repo.AddProductImage(ctx, &products.ProductImage{
		ID:        img2,
		ProductID: p.ID,
		ImageURL:  "https://storage.zamk.test/img2.jpg",
		IsMain:    false,
	}))
	require.NoError(t, repo.UpdateProductImageCrop(ctx, img2, 0, 0, 1.0, 1.0, "https://storage.zamk.test/rend2.jpg", "rend2.jpg"))

	img3 := uuid.New()
	require.NoError(t, repo.AddProductImage(ctx, &products.ProductImage{
		ID:        img3,
		ProductID: p.ID,
		ImageURL:  "https://storage.zamk.test/img3.jpg",
		IsMain:    false,
	}))
	require.NoError(t, repo.UpdateProductImageCrop(ctx, img3, 0, 0, 1.0, 1.0, "https://storage.zamk.test/rend3.jpg", "rend3.jpg"))

	// Case A: Product with ONLY active variant WITHOUT SKU -> Fails with ErrProductSKURequired
	activeVarNoSKUID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, size_value_id, color_id, price_cents, is_active)
		VALUES ($1, $2, NULL, NULL, $3, $4, 250000, true)
	`, activeVarNoSKUID, p.ID, sizeValID, colorID)
	require.NoError(t, err)

	err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
	assert.ErrorIs(t, err, products.ErrProductSKURequired, "Active variant without seller SKU must return ErrProductSKURequired")

	// Case B: Add valid SellerSKU to active variant, and add an INACTIVE variant WITHOUT SKU
	// -> Moderation succeeds, inactive variant without SKU does NOT block!
	_, err = db.Pool.Exec(ctx, "UPDATE product_variants SET seller_sku = 'SKU-ACTIVE-01' WHERE id = $1", activeVarNoSKUID)
	require.NoError(t, err)

	inactiveVarNoSKUID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, size_value_id, color_id, price_cents, is_active)
		VALUES ($1, $2, NULL, NULL, $3, $4, NULL, false)
	`, inactiveVarNoSKUID, p.ID, sizeValID, colorID)
	require.NoError(t, err)

	err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
	require.NoError(t, err, "Inactive variant without SKU or price must NOT block moderation")

	// Verify product status is now pending_moderation
	refreshed, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, products.StatusPendingModeration, refreshed.Status)
}

func TestListSellerProducts_ReturnsCategoryName(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	catID := uuid.New()
	catName := "Худи"
	_, err := db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required, is_active) VALUES ($1, $2, $3, false, true)", catID, catName, fmt.Sprintf("cat-hoodie-%s", uuid.New()))
	require.NoError(t, err)

	slug := "prod-long-" + uuid.New().String()
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Лонг",
		Slug:       &slug,
		CategoryID: &catID,
	})
	require.NoError(t, err)

	resp, err := svc.ListSellerProducts(ctx, sellerUserID, "", 50, 0)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	var found *products.Product
	for i := range resp.Items {
		if resp.Items[i].ID == p.ID {
			found = &resp.Items[i]
			break
		}
	}
	require.NotNil(t, found, "Created product must be in seller products list")
	require.NotNil(t, found.CategoryName, "CategoryName must be populated in ListSellerProducts")
	assert.Equal(t, "Худи", *found.CategoryName)
}
