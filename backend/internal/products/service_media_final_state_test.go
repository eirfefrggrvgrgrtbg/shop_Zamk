package products_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestMediaFinalState_Matrix(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Confirm test database safety
	var currentDB string
	err := db.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "integration tests must ONLY run against zamk_test")

	// Get sellerID for current user
	var sellerID uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	// Create scoped category and colors
	catID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Final State Media Cat', $2, false)", catID, "cat-media-"+catID.String()[:8])
	require.NoError(t, err)

	colorBlackID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Черный', '#000000', true)", colorBlackID, "BLK-"+colorBlackID.String()[:8])
	require.NoError(t, err)

	colorYellowID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Желтый', '#FFFF00', true)", colorYellowID, "YEL-"+colorYellowID.String()[:8])
	require.NoError(t, err)

	colorInactiveID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Неактивный', '#CCCCCC', false)", colorInactiveID, "INA-"+colorInactiveID.String()[:8])
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM colors WHERE id IN ($1, $2, $3)", colorBlackID, colorYellowID, colorInactiveID)
		db.Pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID)
	})

	// Helpers
	createDraft := func(variants []products.ProductVariantRequest) products.Product {
		slug := "p-" + uuid.New().String()
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Matrix Test Product",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   variants,
		})
		require.NoError(t, err)
		return p
	}

	stageImage := func(productID uuid.UUID) uuid.UUID {
		stagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, productID, "ready", fmt.Sprintf("staged_%s.jpg", stagedID.String()))
		return stagedID
	}

	addCanonicalCroppedImage := func(productID uuid.UUID, isMain bool, colorID *uuid.UUID, sortOrder int) uuid.UUID {
		imgID := uuid.New()
		repo := products.NewRepository(db.Pool)
		img := &products.ProductImage{
			ID:        imgID,
			ProductID: productID,
			ImageURL:  fmt.Sprintf("https://storage.zamk.test/img_%s.jpg", imgID.String()),
			IsMain:    isMain,
			ColorID:   colorID,
			SortOrder: sortOrder,
		}
		require.NoError(t, repo.AddProductImage(ctx, img))
		rendKey := fmt.Sprintf("rend_%s.jpg", imgID.String())
		require.NoError(t, repo.UpdateProductImageCrop(ctx, imgID, 0, 0, 1.0, 1.0, "https://storage.zamk.test/"+rendKey, rendKey))
		return imgID
	}

	isMainTrue := true
	isMainFalse := false

	// ==========================================
	// GENERAL_GALLERY Tests
	// ==========================================
	t.Run("A_no_color_product_all_generic_images_PASS", func(t *testing.T) {
		sku := "sku-gen-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, PriceCents: ptr(int64(2000))}, // No colorID
		})
		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: nil},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: nil},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 2)
		assert.Nil(t, pUpdated.Images[0].ColorID)
		assert.Nil(t, pUpdated.Images[1].ColorID)
	})

	t.Run("B_no_color_product_colored_image_invalid_image_color", func(t *testing.T) {
		sku := "sku-gen-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, PriceCents: ptr(int64(2000))},
		})
		staged1 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidImageColor)
	})

	// ==========================================
	// GENERAL_GALLERY Tests (with and without colors)
	// ==========================================
	t.Run("C1_one_active_color_all_generic_images_PASS", func(t *testing.T) {
		sku := "sku-blk-gen-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})
		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: nil},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: nil},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 2)
		assert.Nil(t, pUpdated.Images[0].ColorID)
		assert.Nil(t, pUpdated.Images[1].ColorID)
	})

	t.Run("C2_two_active_colors_all_generic_images_PASS", func(t *testing.T) {
		sku1 := "sku-b-gen-" + uuid.New().String()
		sku2 := "sku-y-gen-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku1, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &sku2, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: nil},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: nil},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 2)
		assert.Nil(t, pUpdated.Images[0].ColorID)
		assert.Nil(t, pUpdated.Images[1].ColorID)
	})

	// ==========================================
	// COLORWAY_GALLERIES Tests
	// ==========================================
	t.Run("C_one_active_color_all_images_that_color_PASS", func(t *testing.T) {
		sku := "sku-blk-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})
		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: &colorBlackID},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 2)
		assert.Equal(t, colorBlackID, *pUpdated.Images[0].ColorID)
		assert.Equal(t, colorBlackID, *pUpdated.Images[1].ColorID)
	})

	t.Run("D_two_active_colors_correctly_assigned_images_PASS", func(t *testing.T) {
		sku1 := "sku-b-" + uuid.New().String()
		sku2 := "sku-y-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku1, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &sku2, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: &colorYellowID},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 2)
		assert.Equal(t, colorBlackID, *pUpdated.Images[0].ColorID)
		assert.Equal(t, colorYellowID, *pUpdated.Images[1].ColorID)
	})

	t.Run("E_generic_and_colored_mix_invalid_media_mode", func(t *testing.T) {
		sku := "sku-mix-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})
		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: nil}, // Generic photo in colorway!
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidMediaMode)
	})

	t.Run("F_image_references_inactive_color_invalid_image_color", func(t *testing.T) {
		sku := "sku-ina-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})
		staged1 := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorInactiveID},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidImageColor)
	})

	t.Run("G_image_references_soft_deleted_variant_color_invalid_image_color", func(t *testing.T) {
		skuB := "sku-b-" + uuid.New().String()
		skuY := "sku-y-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})

		// Soft-delete Yellow variant by updating variants to Black only
		updVarReq := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updVarReq)
		require.NoError(t, err)

		// Now attempt to add an image referencing the soft-deleted Yellow color
		staged := stageImage(p.ID)
		updImgReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged, IsMain: &isMainTrue, ColorID: &colorYellowID},
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updImgReq)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidImageColor)
	})

	// ==========================================
	// PATCH Final State (Omitted variants/images)
	// ==========================================
	t.Run("H_variants_changed_images_nil_removed_color_still_has_image_FAIL", func(t *testing.T) {
		skuB := "sku-b-" + uuid.New().String()
		skuY := "sku-y-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})

		// Add canonical images for Black and Yellow
		_ = addCanonicalCroppedImage(p.ID, true, &colorBlackID, 0)
		_ = addCanonicalCroppedImage(p.ID, false, &colorYellowID, 1)

		// Seller removes Yellow from variants, but sends images: nil
		updReq := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			},
			Images: nil, // Omitted
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidImageColor)
	})

	t.Run("I_images_changed_variants_nil_current_active_colors_used", func(t *testing.T) {
		skuB := "sku-b-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})

		staged := stageImage(p.ID)
		updReq := products.UpdateProductRequest{
			Variants: nil, // Omitted
			Images: []products.ProductImageRequest{
				{ID: &staged, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 1)
		assert.Equal(t, colorBlackID, *pUpdated.Images[0].ColorID)
	})

	t.Run("J_both_variants_and_images_changed_validate_combined_final_state", func(t *testing.T) {
		skuB := "sku-b-" + uuid.New().String()
		skuY := "sku-y-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})

		stagedB := stageImage(p.ID)
		stagedY := stageImage(p.ID)

		// Add Yellow variant AND add images for both Black and Yellow
		updReq := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
				{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
			},
			Images: []products.ProductImageRequest{
				{ID: &stagedB, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &stagedY, IsMain: &isMainFalse, ColorID: &colorYellowID},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Variants, 2)
		assert.Len(t, pUpdated.Images, 2)
	})

	t.Run("K_omitted_existing_image_semantics_declarative", func(t *testing.T) {
		sku := "sku-k-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})

		img1 := addCanonicalCroppedImage(p.ID, true, &colorBlackID, 0)
		_ = addCanonicalCroppedImage(p.ID, false, &colorBlackID, 1)

		// Send only img1 in images list (omitting img2 -> img2 deleted)
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &img1, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 1)
		assert.Equal(t, img1, pUpdated.Images[0].ID)
	})

	// ==========================================
	// Draft progressive authoring rules
	// ==========================================
	t.Run("L_zero_images_draft_allowed", func(t *testing.T) {
		sku := "sku-l-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Empty(t, pUpdated.Images)
	})

	t.Run("M_one_image_draft_allowed", func(t *testing.T) {
		sku := "sku-m-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})
		staged := stageImage(p.ID)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 1)
	})

	t.Run("N_colorway_draft_missing_images_for_one_active_color_allowed", func(t *testing.T) {
		skuB := "sku-b-" + uuid.New().String()
		skuY := "sku-y-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		stagedB := stageImage(p.ID)

		// Draft has Black image, Yellow has 0 images -> Draft save is ALLOWED
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedB, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 1)
	})

	// ==========================================
	// Moderation readiness rules
	// ==========================================
	t.Run("O_GENERAL_total_less_than_3_readiness_failure", func(t *testing.T) {
		sku := "sku-o-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, PriceCents: ptr(int64(2000))},
		})
		_ = addCanonicalCroppedImage(p.ID, true, nil, 0)
		_ = addCanonicalCroppedImage(p.ID, false, nil, 1)

		err := svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrProductMediaRequired)
	})

	t.Run("P_GENERAL_greater_or_equal_3_PASS", func(t *testing.T) {
		sku := "sku-p-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, PriceCents: ptr(int64(2000))},
		})
		_ = addCanonicalCroppedImage(p.ID, true, nil, 0)
		_ = addCanonicalCroppedImage(p.ID, false, nil, 1)
		_ = addCanonicalCroppedImage(p.ID, false, nil, 2)

		err := svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
		require.NoError(t, err)
	})

	t.Run("P2_GENERAL_with_active_colors_gte_3_moderation_PASS", func(t *testing.T) {
		skuB := "sku-p2b-" + uuid.New().String()
		skuY := "sku-p2y-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		// 3 generic images (color_id == nil) on a 2-color product
		_ = addCanonicalCroppedImage(p.ID, true, nil, 0)
		_ = addCanonicalCroppedImage(p.ID, false, nil, 1)
		_ = addCanonicalCroppedImage(p.ID, false, nil, 2)

		err := svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
		require.NoError(t, err, "GENERAL gallery mode must be valid for moderation on products with color dimension")
	})

	t.Run("Q_COLORWAY_total_gte_3_but_active_color_has_0_missing_color_images", func(t *testing.T) {
		skuB := "sku-qb-" + uuid.New().String()
		skuY := "sku-qy-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		// 3 images for Black, 0 for Yellow
		_ = addCanonicalCroppedImage(p.ID, true, &colorBlackID, 0)
		_ = addCanonicalCroppedImage(p.ID, false, &colorBlackID, 1)
		_ = addCanonicalCroppedImage(p.ID, false, &colorBlackID, 2)

		err := svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrMissingColorImages)
	})

	t.Run("R_COLORWAY_gte_3_and_every_active_color_covered_PASS", func(t *testing.T) {
		skuB := "sku-rb-" + uuid.New().String()
		skuY := "sku-ry-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		// 2 images for Black, 1 for Yellow -> total 3, both active colors covered
		_ = addCanonicalCroppedImage(p.ID, true, &colorBlackID, 0)
		_ = addCanonicalCroppedImage(p.ID, false, &colorBlackID, 1)
		_ = addCanonicalCroppedImage(p.ID, false, &colorYellowID, 2)

		err := svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, products.SubmitProductModerationRequest{})
		require.NoError(t, err)
	})

	// ==========================================
	// GENERAL Gallery Color Mutation Robustness
	// ==========================================
	t.Run("M_GENERAL_removes_or_adds_colors_without_invalidating_generic_images", func(t *testing.T) {
		skuB := "sku-mb-" + uuid.New().String()
		skuY := "sku-my-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		_ = addCanonicalCroppedImage(p.ID, true, nil, 0)
		_ = addCanonicalCroppedImage(p.ID, false, nil, 1)

		// Removing Yellow variant while keeping generic images (images: nil) must SUCCEED
		updReq := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			},
			Images: nil, // Omitted
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Len(t, pUpdated.Images, 2)
		assert.Nil(t, pUpdated.Images[0].ColorID)
	})

	// ==========================================
	// isMain Invariant
	// ==========================================
	t.Run("S_T_U_single_global_isMain_enforced", func(t *testing.T) {
		sku := "sku-s-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
		})
		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)

		// Multiple isMain -> rejected
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &staged2, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidMediaSet)

		// Zero isMain -> rejected
		updReq2 := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainFalse, ColorID: &colorBlackID},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: &colorBlackID},
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq2)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidMediaSet)
	})

	// ==========================================
	// Published Product Revision Path
	// ==========================================
	t.Run("V_W_X_revision_path_uses_same_validator", func(t *testing.T) {
		skuB := "sku-rev-b-" + uuid.New().String()
		skuY := "sku-rev-y-" + uuid.New().String()
		p := createDraft([]products.ProductVariantRequest{
			{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2500))},
		})
		_ = addCanonicalCroppedImage(p.ID, true, &colorBlackID, 0)
		_ = addCanonicalCroppedImage(p.ID, false, &colorBlackID, 1)
		_ = addCanonicalCroppedImage(p.ID, false, &colorYellowID, 2)

		// Move product to published status
		_, err := db.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", p.ID)
		require.NoError(t, err)

		// W1. Revision can switch to GENERAL gallery (generic images on product with colors)
		stagedGen1 := stageImage(p.ID)
		stagedGen2 := stageImage(p.ID)
		revReqGen := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedGen1, IsMain: &isMainTrue, ColorID: nil},
				{ID: &stagedGen2, IsMain: &isMainFalse, ColorID: nil},
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, revReqGen)
		require.NoError(t, err, "Revision must support GENERAL_GALLERY for products with colors")

		// W2. Revision cannot bypass hybrid validation
		stagedHybrid1 := stageImage(p.ID)
		stagedHybrid2 := stageImage(p.ID)
		revReqHybrid := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedHybrid1, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &stagedHybrid2, IsMain: &isMainFalse, ColorID: nil}, // Generic + Color mix!
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, revReqHybrid)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidMediaMode)

		// X. Revision cannot retain image for removed active color
		revReqOrphan := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2500))},
			},
			Images: nil, // Omitted, so DB has image for Yellow
		}
		_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, revReqOrphan)
		require.Error(t, err)
		assert.ErrorIs(t, err, products.ErrInvalidImageColor)
	})
}

func TestMediaErrorContract_HTTPAndService(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Guard test DB
	var currentDB string
	err := db.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "integration tests must ONLY run against zamk_test")

	// Get sellerID for current user
	var sellerID uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Contract Cat', $2, false)", catID, "cat-contract-"+catID.String()[:8])
	require.NoError(t, err)

	colorBlackID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Черный', '#000000', true)", colorBlackID, "BLK-"+colorBlackID.String()[:8])
	require.NoError(t, err)

	colorInactiveID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Неактивный', '#CCCCCC', false)", colorInactiveID, "INA-"+colorInactiveID.String()[:8])
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM colors WHERE id IN ($1, $2)", colorBlackID, colorInactiveID)
		db.Pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID)
	})

	handler := products.NewHandler(svc, nil)

	stageImage := func(productID uuid.UUID) uuid.UUID {
		stagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, productID, "ready", fmt.Sprintf("staged_contract_%s.jpg", stagedID.String()))
		return stagedID
	}

	addCroppedImg := func(productID uuid.UUID, isMain bool, colorID *uuid.UUID, sortOrder int) uuid.UUID {
		imgID := uuid.New()
		repo := products.NewRepository(db.Pool)
		img := &products.ProductImage{
			ID:        imgID,
			ProductID: productID,
			ImageURL:  fmt.Sprintf("https://storage.zamk.test/img_%s.jpg", imgID.String()),
			IsMain:    isMain,
			ColorID:   colorID,
			SortOrder: sortOrder,
		}
		require.NoError(t, repo.AddProductImage(ctx, img))
		rendKey := fmt.Sprintf("rend_%s.jpg", imgID.String())
		require.NoError(t, repo.UpdateProductImageCrop(ctx, imgID, 0, 0, 1.0, 1.0, "https://storage.zamk.test/"+rendKey, rendKey))
		return imgID
	}

	isMainTrue := true
	isMainFalse := false

	// A. Product has NO color dimension. Image has color_id != NULL -> invalid_image_color (400)
	t.Run("Case_A_no_color_product_colored_image_returns_400_invalid_image_color", func(t *testing.T) {
		slug := "p-a-" + uuid.New().String()
		sku := "sku-a-" + uuid.New().String()
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "No Color Product",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   []products.ProductVariantRequest{{SellerSKU: &sku, PriceCents: ptr(int64(2000))}},
		})
		require.NoError(t, err)

		staged := stageImage(p.ID)
		body, _ := json.Marshal(products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		})

		httpReq := httptest.NewRequest(http.MethodPut, "/api/seller/products/"+p.ID.String(), bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.UpdateProduct(rec, httpReq)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var resp map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "invalid_image_color", resp["error"]["code"])
	})

	// B. Product HAS color dimension. Images contain mixed generic and color-specific -> invalid_media_mode (400)
	t.Run("Case_B_hybrid_media_mode_returns_400_invalid_media_mode", func(t *testing.T) {
		slug := "p-b-" + uuid.New().String()
		sku := "sku-b-" + uuid.New().String()
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Color Product",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   []products.ProductVariantRequest{{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2000))}},
		})
		require.NoError(t, err)

		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)
		body, _ := json.Marshal(products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &staged2, IsMain: &isMainFalse, ColorID: nil}, // Generic
			},
		})

		httpReq := httptest.NewRequest(http.MethodPut, "/api/seller/products/"+p.ID.String(), bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.UpdateProduct(rec, httpReq)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var resp map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "invalid_media_mode", resp["error"]["code"])
	})

	// C. Product HAS color dimension. Image references inactive/unknown color -> invalid_image_color (400)
	t.Run("Case_C_inactive_or_unknown_color_returns_400_invalid_image_color", func(t *testing.T) {
		slug := "p-c-" + uuid.New().String()
		sku := "sku-c-" + uuid.New().String()
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Color Product C",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   []products.ProductVariantRequest{{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2000))}},
		})
		require.NoError(t, err)

		staged := stageImage(p.ID)
		body, _ := json.Marshal(products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged, IsMain: &isMainTrue, ColorID: &colorInactiveID},
			},
		})

		httpReq := httptest.NewRequest(http.MethodPut, "/api/seller/products/"+p.ID.String(), bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.UpdateProduct(rec, httpReq)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var resp map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "invalid_image_color", resp["error"]["code"])
	})

	// D. Submit to moderation: active color exists but has 0 images -> missing_color_images (422)
	t.Run("Case_D_submit_moderation_missing_color_images_returns_422", func(t *testing.T) {
		slug := "p-d-" + uuid.New().String()
		skuB := "sku-db-" + uuid.New().String()
		skuY := "sku-dy-" + uuid.New().String()

		colorYellowID := uuid.New()
		_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Желтый', '#FFFF00', true)", colorYellowID, "YEL-"+colorYellowID.String()[:8])
		require.NoError(t, err)
		defer db.Pool.Exec(context.Background(), "DELETE FROM colors WHERE id = $1", colorYellowID)

		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Two Colors Product D",
			Slug:       &slug,
			CategoryID: &catID,
			Variants: []products.ProductVariantRequest{
				{SellerSKU: &skuB, ColorID: &colorBlackID, PriceCents: ptr(int64(2000))},
				{SellerSKU: &skuY, ColorID: &colorYellowID, PriceCents: ptr(int64(2000))},
			},
		})
		require.NoError(t, err)

		// 3 images for Black, 0 for Yellow
		_ = addCroppedImg(p.ID, true, &colorBlackID, 0)
		_ = addCroppedImg(p.ID, false, &colorBlackID, 1)
		_ = addCroppedImg(p.ID, false, &colorBlackID, 2)

		httpReq := httptest.NewRequest(http.MethodPost, "/api/seller/products/"+p.ID.String()+"/submit-moderation", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.SubmitForModeration(rec, httpReq)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var resp map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "missing_color_images", resp["error"]["code"])
	})

	// E. Media structure invalid: duplicate IDs, 0 isMain, 2 isMain, >8 images -> invalid_media_set (400)
	t.Run("Case_E_invalid_media_set_returns_400_invalid_media_set", func(t *testing.T) {
		slug := "p-e-" + uuid.New().String()
		sku := "sku-e-" + uuid.New().String()
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Media Set Product E",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   []products.ProductVariantRequest{{SellerSKU: &sku, ColorID: &colorBlackID, PriceCents: ptr(int64(2000))}},
		})
		require.NoError(t, err)

		staged1 := stageImage(p.ID)
		staged2 := stageImage(p.ID)
		// Both isMain = true
		body, _ := json.Marshal(products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &staged1, IsMain: &isMainTrue, ColorID: &colorBlackID},
				{ID: &staged2, IsMain: &isMainTrue, ColorID: &colorBlackID},
			},
		})

		httpReq := httptest.NewRequest(http.MethodPut, "/api/seller/products/"+p.ID.String(), bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.UpdateProduct(rec, httpReq)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var resp map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "invalid_media_set", resp["error"]["code"])
	})

	// F. Submit to moderation with total images <3 -> product_media_required (422)
	t.Run("Case_F_submit_moderation_less_than_3_images_returns_422_product_media_required", func(t *testing.T) {
		slug := "p-f-" + uuid.New().String()
		sku := "sku-f-" + uuid.New().String()
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Less Than 3 Product F",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   []products.ProductVariantRequest{{SellerSKU: &sku, PriceCents: ptr(int64(2000))}},
		})
		require.NoError(t, err)

		_ = addCroppedImg(p.ID, true, nil, 0)
		_ = addCroppedImg(p.ID, false, nil, 1)

		httpReq := httptest.NewRequest(http.MethodPost, "/api/seller/products/"+p.ID.String()+"/submit-moderation", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.SubmitForModeration(rec, httpReq)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var resp map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "product_media_required", resp["error"]["code"])
	})

	// E2. Submit to moderation with invalid media set (e.g. multiple main images) -> invalid_media_set (400)
	t.Run("Case_E2_submit_moderation_invalid_media_set_returns_400", func(t *testing.T) {
		slug := "p-e2-" + uuid.New().String()
		sku := "sku-e2-" + uuid.New().String()
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Invalid Media Set Moderation E2",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   []products.ProductVariantRequest{{SellerSKU: &sku, PriceCents: ptr(int64(2000))}},
		})
		require.NoError(t, err)

		// 9 cropped images (>8 maximum allowed)
		_ = addCroppedImg(p.ID, true, nil, 0)
		for i := 1; i <= 8; i++ {
			_ = addCroppedImg(p.ID, false, nil, i)
		}

		httpReq := httptest.NewRequest(http.MethodPost, "/api/seller/products/"+p.ID.String()+"/submit-moderation", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.SubmitForModeration(rec, httpReq)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var resp map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "invalid_media_set", resp["error"]["code"])
	})
}
