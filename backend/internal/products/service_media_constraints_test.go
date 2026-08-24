package products_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestMediaMainImageConstraint(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	req := products.CreateProductRequest{
		Title: "Media Main Test",
		Slug: func(s string) *string { return &s }(uuid.New().String()),
	}
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
	require.NoError(t, err)

	repo := products.NewRepository(db.Pool)

	// Create two images directly
	img1 := &products.ProductImage{
		ID:        uuid.New(),
		ProductID: p.ID,
		ImageURL:  "url1",
		IsMain:    true,
	}
	err = repo.AddProductImage(ctx, img1)
	require.NoError(t, err)

	img2 := &products.ProductImage{
		ID:        uuid.New(),
		ProductID: p.ID,
		ImageURL:  "url2",
		IsMain:    true, // Try to add another main
	}
	err = repo.AddProductImage(ctx, img2)
	require.Error(t, err, "Should fail because of unique constraint on is_main")

	// Set img2 to not main, it should succeed
	img2.IsMain = false
	err = repo.AddProductImage(ctx, img2)
	require.NoError(t, err)

	// Try to update img2 to main directly via SQL
	_, err = db.Pool.Exec(ctx, "UPDATE product_images SET is_main = true WHERE id = $1", img2.ID)
	require.Error(t, err, "Should fail because img1 is still main")

	// Call clear other main images
	err = repo.ClearOtherMainImages(ctx, p.ID, img2.ID)
	require.NoError(t, err)

	// Now update img2 to main should succeed
	_, err = db.Pool.Exec(ctx, "UPDATE product_images SET is_main = true WHERE id = $1", img2.ID)
	require.NoError(t, err)

	// Verify
	images, err := repo.GetProductImages(ctx, p.ID)
	require.NoError(t, err)

	var i1, i2 *products.ProductImage
	for i, img := range images {
		if img.ID == img1.ID { i1 = &images[i] }
		if img.ID == img2.ID { i2 = &images[i] }
	}
	require.NotNil(t, i1)
	require.NotNil(t, i2)

	assert.False(t, i1.IsMain)
	assert.True(t, i2.IsMain)
}
