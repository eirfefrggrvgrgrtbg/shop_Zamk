package products_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestMediaRoundtripIntegration(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Get a color
	var colorID uuid.UUID
	err := db.Pool.QueryRow(ctx, "SELECT id FROM colors WHERE code = 'BLACK'").Scan(&colorID)
	require.NoError(t, err)

	// Create Product with media
	req := products.CreateProductRequest{
		Title: "Media Test Product",
		Slug: func(s string) *string { return &s }(uuid.New().String()),
		Images: []products.ProductImageRequest{
			{
				ImageURL:       "https://storage.zamk.test/products/common1.jpg",
				SortOrder: func(i int) *int { return &i }(1),
				ColorID:   nil, // Common image
			},
			{
				ImageURL:       "https://storage.zamk.test/products/black1.jpg",
				SortOrder: func(i int) *int { return &i }(2),
				ColorID:   &colorID, // Color-specific image
			},
		},
	}
	
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
	require.NoError(t, err)

	// Read and assert
	pRead, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
	require.NoError(t, err)
	require.Len(t, pRead.Images, 2)
	
	var commonImg, colorImg *products.ProductImage
	for i, img := range pRead.Images {
		if img.ColorID == nil {
			commonImg = &pRead.Images[i]
		} else {
			colorImg = &pRead.Images[i]
		}
	}
	
	require.NotNil(t, commonImg)
	require.NotNil(t, colorImg)
	assert.Equal(t, "https://storage.zamk.test/products/common1.jpg", commonImg.ImageURL)
	assert.Equal(t, 1, commonImg.SortOrder)
	
	assert.Equal(t, "https://storage.zamk.test/products/black1.jpg", colorImg.ImageURL)
	assert.Equal(t, colorID, *colorImg.ColorID)
	assert.Equal(t, 2, colorImg.SortOrder)

	// Update and reorder
	updateReq := products.UpdateProductRequest{
		Title: func(s string) *string { return &s }("Media Test Product 2"),
		Images: []products.ProductImageRequest{
			{
				ImageURL:       "https://storage.zamk.test/products/black1.jpg",
				SortOrder: func(i int) *int { return &i }(1), // Now first
				ColorID:   &colorID,
			},
			{
				ImageURL:       "https://storage.zamk.test/products/common1.jpg",
				SortOrder: func(i int) *int { return &i }(2), // Now second
				ColorID:   nil,
			},
			{
				ImageURL:       "https://storage.zamk.test/products/common2.jpg",
				SortOrder: func(i int) *int { return &i }(3),
				ColorID:   nil, // New common image
			},
		},
	}

	pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updateReq)
	require.NoError(t, err)
	
	pRead2, err := svc.GetSellerProduct(ctx, sellerUserID, pUpdated.ID)
	require.NoError(t, err)
	require.Len(t, pRead2.Images, 3)

	var readImg1, readImg2, readImg3 *products.ProductImage
	for i, img := range pRead2.Images {
		if img.SortOrder == 1 { readImg1 = &pRead2.Images[i] }
		if img.SortOrder == 2 { readImg2 = &pRead2.Images[i] }
		if img.SortOrder == 3 { readImg3 = &pRead2.Images[i] }
	}
	require.NotNil(t, readImg1)
	require.NotNil(t, readImg2)
	require.NotNil(t, readImg3)

	assert.Equal(t, colorID, *readImg1.ColorID)
	assert.Nil(t, readImg2.ColorID)
	assert.Nil(t, readImg3.ColorID)
	assert.Equal(t, "https://storage.zamk.test/products/common2.jpg", readImg3.ImageURL)
}
