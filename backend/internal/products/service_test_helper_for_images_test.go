package products_test

import (
	"context"
	"testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func injectValidMainImage(t *testing.T, ctx context.Context, repo *products.Repository, productID uuid.UUID) {
	images, err := repo.GetProductImages(ctx, productID)
	require.NoError(t, err)
	for _, img := range images {
		if img.IsMain {
			return // already has a main image
		}
	}

	img := &products.ProductImage{
		ID:        uuid.New(),
		ProductID: productID,
		ImageURL:  "test_image_url",
		IsMain:    true,
	}
	err = repo.AddProductImage(ctx, img)
	require.NoError(t, err)
	
	err = repo.UpdateProductImageCrop(ctx, img.ID, 0.1, 0.1, 0.8, 1.0, "rend.jpg", "rend.jpg")
	require.NoError(t, err)
}
