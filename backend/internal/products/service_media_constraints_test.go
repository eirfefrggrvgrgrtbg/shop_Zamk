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

func TestModerationSubmission_MediaValidation(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create a category
	catID := uuid.New()
	_, err := db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Test Cat', $2, false)", catID, "test-cat-"+uuid.New().String())
	require.NoError(t, err)

	// 2. Create a draft product
	slug := "draft-" + uuid.New().String()
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Draft Product",
		Slug:       &slug,
		CategoryID: &catID,
	})
	require.NoError(t, err)

	// 3. Attempt submit without images -> fails
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one image is required")

	// 4. Add an image without crop/rendition
	repo := products.NewRepository(db.Pool)
	img1 := &products.ProductImage{
		ID:        uuid.New(),
		ProductID: p.ID,
		ImageURL:  "https://storage.zamk.test/orig1.jpg",
		IsMain:    false,
	}
	err = repo.AddProductImage(ctx, img1)
	require.NoError(t, err)

	// Attempt submit -> fails because image lacks rendition
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all images must have explicit 4:5 renditions")

	// 5. Update image with crop & rendition, but is_main is false
	err = repo.UpdateProductImageCrop(ctx, img1.ID, 0, 0, 1.0, 1.0, "https://storage.zamk.test/rend1.jpg", "rend1.jpg")
	require.NoError(t, err)

	// Attempt submit -> fails because no main image
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a main image is required")

	// 6. Set as main
	_, err = db.Pool.Exec(ctx, "UPDATE product_images SET is_main = true WHERE id = $1", img1.ID)
	require.NoError(t, err)

	// Attempt submit -> passes media validation (may fail on missing required attributes/variants if strict, or succeed)
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
	// The error should NOT be media-related anymore
	if err != nil {
		assert.NotErrorIs(t, err, products.ErrProductMediaRequired)
		assert.NotErrorIs(t, err, products.ErrProductMediaNotReady)
		assert.NotErrorIs(t, err, products.ErrProductMainImageMissing)
	}
}

func TestUpdateProductForSeller_PreservesImageCropsAndMain(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	catID := uuid.New()
	_, err := db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Test Cat', $2, false)", catID, "test-cat-"+uuid.New().String())
	require.NoError(t, err)

	slug := "draft-" + uuid.New().String()
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Draft Product With Media",
		Slug:       &slug,
		CategoryID: &catID,
	})
	require.NoError(t, err)

	repo := products.NewRepository(db.Pool)
	imgID := uuid.New()
	img := &products.ProductImage{
		ID:        imgID,
		ProductID: p.ID,
		ImageURL:  "https://storage.zamk.test/orig.jpg",
		IsMain:    true,
	}
	err = repo.AddProductImage(ctx, img)
	require.NoError(t, err)

	// Set crop and rendition
	err = repo.UpdateProductImageCrop(ctx, imgID, 0.1, 0.1, 0.8, 0.8, "https://storage.zamk.test/rend.jpg", "rend.jpg")
	require.NoError(t, err)

	// Now simulate wizard saving draft (e.g. updating description or variant prices)
	desc := "Updated description from wizard"
	sort0 := 0
	updateReq := products.UpdateProductRequest{
		Description: &desc,
		Images: []products.ProductImageRequest{
			{
				ID:        &imgID,
				ImageURL:  "https://storage.zamk.test/orig.jpg",
				SortOrder: &sort0,
			},
		},
	}

	pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updateReq)
	require.NoError(t, err)
	require.Len(t, pUpdated.Images, 1)

	updatedImg := pUpdated.Images[0]
	assert.Equal(t, imgID, updatedImg.ID)
	assert.True(t, updatedImg.IsMain, "is_main should be preserved across draft saves")
	require.NotNil(t, updatedImg.CropWidth, "CropWidth should be preserved")
	assert.Equal(t, 0.8, *updatedImg.CropWidth)
	require.NotNil(t, updatedImg.RenditionURL, "RenditionURL should be preserved")
	assert.Equal(t, "https://storage.zamk.test/rend.jpg", *updatedImg.RenditionURL)
}

func TestSubmitProductToModeration_TypedErrors(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. No category
	slug1 := "draft-" + uuid.New().String()
	p1, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title: "No Category",
		Slug:  &slug1,
	})
	require.NoError(t, err)
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p1.ID, products.SubmitProductModerationRequest{})
	assert.ErrorIs(t, err, products.ErrProductCategoryRequired)

	// 2. Category present, no images
	catID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Cat Typed', $2, false)", catID, "cat-typed-"+uuid.New().String())
	require.NoError(t, err)

	slug2 := "draft-" + uuid.New().String()
	p2, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "No Images",
		Slug:       &slug2,
		CategoryID: &catID,
	})
	require.NoError(t, err)
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p2.ID, products.SubmitProductModerationRequest{})
	assert.ErrorIs(t, err, products.ErrProductMediaRequired)

	// 3. Image without crop
	repo := products.NewRepository(db.Pool)
	imgID := uuid.New()
	err = repo.AddProductImage(ctx, &products.ProductImage{
		ID:        imgID,
		ProductID: p2.ID,
		ImageURL:  "https://storage.zamk.test/uncropped.jpg",
		IsMain:    false,
	})
	require.NoError(t, err)
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p2.ID, products.SubmitProductModerationRequest{})
	assert.ErrorIs(t, err, products.ErrProductMediaNotReady)

	// 4. Image with crop, but no main
	err = repo.UpdateProductImageCrop(ctx, imgID, 0, 0, 1.0, 1.0, "https://storage.zamk.test/rend.jpg", "rend.jpg")
	require.NoError(t, err)
	err = svc.SubmitProductToModeration(ctx, sellerUserID, p2.ID, products.SubmitProductModerationRequest{})
	assert.ErrorIs(t, err, products.ErrProductMainImageMissing)
}
