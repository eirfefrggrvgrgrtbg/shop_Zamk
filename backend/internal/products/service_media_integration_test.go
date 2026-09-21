package products_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-chi/chi/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

// Helper: insert a ready staging row
func insertStagingRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, stagingID, sellerID, productID uuid.UUID, status string, objKey string) {
	t.Helper()
	imageURL := fmt.Sprintf("https://storage.zamk.test/%s", objKey)
	var consumedAt *time.Time
	if status == "consumed" {
		now := time.Now().UTC()
		consumedAt = &now
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO product_media_staging (
			id, seller_id, product_id, client_media_id, status,
			object_key, image_url, content_sha256, byte_size, width, height,
			created_at, consumed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 1024, 800, 1000, NOW(), $8)
	`, stagingID, sellerID, productID, uuid.New(), status, objKey, imageURL, consumedAt)
	require.NoError(t, err)
}

// Helper: insert canonical product image directly
func insertCanonicalImage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, imgID, productID uuid.UUID, objKey string, sortOrder int, isMain bool, colorID *uuid.UUID, rendObjKey *string) {
	t.Helper()
	imageURL := fmt.Sprintf("https://storage.zamk.test/%s", objKey)
	var rendURL *string
	if rendObjKey != nil {
		u := fmt.Sprintf("https://storage.zamk.test/%s", *rendObjKey)
		rendURL = &u
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO product_images (
			id, product_id, image_url, object_key, rendition_url, rendition_object_key,
			sort_order, color_id, is_main, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
	`, imgID, productID, imageURL, objKey, rendURL, rendObjKey, sortOrder, colorID, isMain)
	require.NoError(t, err)
}

func TestPATCHMediaMatrix(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Get sellerID for current user
	var sellerID uuid.UUID
	err := db.Pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	// Create test category and color
	catID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Media Cat', $2, false)", catID, "cat-"+catID.String())
	require.NoError(t, err)

	colorID1 := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Color One', '#111111', true)", colorID1, "C1-"+colorID1.String()[:8])
	require.NoError(t, err)

	colorID2 := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Color Two', '#222222', true)", colorID2, "C2-"+colorID2.String()[:8])
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM colors WHERE id IN ($1, $2)", colorID1, colorID2)
		db.Pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID)
	})

	// Helper to create fresh draft product with a variant
	createDraftWithVariant := func(colorID *uuid.UUID) products.Product {
		slug := "p-" + uuid.New().String()
		sku := "sku-" + uuid.New().String()
		var variants []products.ProductVariantRequest
		if colorID != nil {
			variants = []products.ProductVariantRequest{
				{
					SellerSKU:  &sku,
					ColorID:    colorID,
					PriceCents: ptr(int64(1500)),
				},
			}
		}
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
			Title:      "Matrix Test Product",
			Slug:       &slug,
			CategoryID: &catID,
			Variants:   variants,
		})
		require.NoError(t, err)
		return p
	}

	t.Run("1_images_omitted_untouched", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		imgID := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, imgID, p.ID, "obj1.jpg", 0, true, nil, nil)
		_, err := db.Pool.Exec(ctx, "UPDATE products SET main_image_url = 'https://storage.zamk.test/obj1.jpg', main_image_object_key = 'obj1.jpg' WHERE id = $1", p.ID)
		require.NoError(t, err)

		newTitle := "Updated Title Omitted Images"
		updReq := products.UpdateProductRequest{
			Title:  &newTitle,
			Images: nil, // Omitted
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Equal(t, newTitle, pUpdated.Title)

		// Assert canonical images untouched
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE product_id = $1", p.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Assert main image untouched
		var mainURL, mainKey *string
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", p.ID).Scan(&mainURL, &mainKey)
		require.NoError(t, err)
		require.NotNil(t, mainURL)
		assert.Equal(t, "https://storage.zamk.test/obj1.jpg", *mainURL)
		require.NotNil(t, mainKey)
		assert.Equal(t, "obj1.jpg", *mainKey)
	})

	t.Run("2_and_20_explicit_empty_images_clears_all_and_sets_main_null", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		imgID := uuid.New()
		rendKey := "rend1.jpg"
		insertCanonicalImage(t, ctx, db.Pool, imgID, p.ID, "obj1.jpg", 0, true, nil, &rendKey)
		_, err := db.Pool.Exec(ctx, "UPDATE products SET main_image_url = 'https://storage.zamk.test/obj1.jpg', main_image_object_key = 'obj1.jpg' WHERE id = $1", p.ID)
		require.NoError(t, err)

		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{}, // Explicit empty []
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		assert.Empty(t, pUpdated.Images)
		assert.Nil(t, pUpdated.MainImageURL)
		assert.Nil(t, pUpdated.MainImageObjectKey)

		// DB: images cleared
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE product_id = $1", p.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		// DB: products table main image fields NULL
		var mainURL, mainKey *string
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", p.ID).Scan(&mainURL, &mainKey)
		require.NoError(t, err)
		assert.Nil(t, mainURL)
		assert.Nil(t, mainKey)

		// Cleanup jobs queued for both orig and rendition
		var jobCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key IN ('obj1.jpg', 'rend1.jpg')").Scan(&jobCount)
		require.NoError(t, err)
		assert.Equal(t, 2, jobCount)
	})

	t.Run("3_and_4_reorder_canonical_array_order_wins_over_client_sortOrder", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		img1 := uuid.New()
		img2 := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, img1, p.ID, "obj1.jpg", 0, true, nil, nil)
		insertCanonicalImage(t, ctx, db.Pool, img2, p.ID, "obj2.jpg", 1, false, nil, nil)

		// Client sends img2 first with sortOrder 999, img1 second with sortOrder 0
		sort999 := 999
		sort0 := 0
		isMainTrue := true
		isMainFalse := false
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &img2, SortOrder: &sort999, IsMain: &isMainTrue},
				{ID: &img1, SortOrder: &sort0, IsMain: &isMainFalse},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		require.Len(t, pUpdated.Images, 2)

		// Array index determines sort_order: img2 index 0 -> sort_order 0, img1 index 1 -> sort_order 1
		var so2, so1 int
		err = db.Pool.QueryRow(ctx, "SELECT sort_order FROM product_images WHERE id = $1", img2).Scan(&so2)
		require.NoError(t, err)
		assert.Equal(t, 0, so2)

		err = db.Pool.QueryRow(ctx, "SELECT sort_order FROM product_images WHERE id = $1", img1).Scan(&so1)
		require.NoError(t, err)
		assert.Equal(t, 1, so1)
	})

	t.Run("5_change_existing_image_color_binding", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		imgID := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, imgID, p.ID, "obj1.jpg", 0, true, nil, nil)

		// Update image to bind to colorID1 (which is valid on the product variant)
		isMain := true
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &imgID, ColorID: &colorID1, IsMain: &isMain},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		require.Len(t, pUpdated.Images, 1)
		require.NotNil(t, pUpdated.Images[0].ColorID)
		assert.Equal(t, colorID1, *pUpdated.Images[0].ColorID)

		var boundColor *uuid.UUID
		err = db.Pool.QueryRow(ctx, "SELECT color_id FROM product_images WHERE id = $1", imgID).Scan(&boundColor)
		require.NoError(t, err)
		require.NotNil(t, boundColor)
		assert.Equal(t, colorID1, *boundColor)
	})

	t.Run("6_11_12_promote_ready_staged_image_preserves_uuid_and_marks_consumed", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		stagedID := uuid.New()
		objKey := fmt.Sprintf("staged-%s.jpg", stagedID)
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, p.ID, "ready", objKey)

		isMain := true
		alt := "Alt text for staged"
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedID, AltText: &alt, IsMain: &isMain},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		require.Len(t, pUpdated.Images, 1)

		// 11. Staged UUID preserved as canonical UUID
		assert.Equal(t, stagedID, pUpdated.Images[0].ID)

		// 12. Staged row becomes consumed, not deleted
		var stgStatus string
		var consumedAt *time.Time
		err = db.Pool.QueryRow(ctx, "SELECT status, consumed_at FROM product_media_staging WHERE id = $1", stagedID).Scan(&stgStatus, &consumedAt)
		require.NoError(t, err)
		assert.Equal(t, "consumed", stgStatus)
		assert.NotNil(t, consumedAt)

		// Canonical row exists
		var canCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1 AND product_id = $2", stagedID, p.ID).Scan(&canCount)
		require.NoError(t, err)
		assert.Equal(t, 1, canCount)
	})

	t.Run("7_8_9_mixed_set_and_omitted_canonical_with_rendition_cleanup", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		existingImg1 := uuid.New()
		existingImg2ToDrop := uuid.New()
		rendKey := "rend-drop.jpg"

		insertCanonicalImage(t, ctx, db.Pool, existingImg1, p.ID, "keep.jpg", 0, true, nil, nil)
		insertCanonicalImage(t, ctx, db.Pool, existingImg2ToDrop, p.ID, "drop.jpg", 1, false, nil, &rendKey)

		stagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, p.ID, "ready", "promote.jpg")

		isMainTrue := true
		isMainFalse := false
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &existingImg1, IsMain: &isMainTrue},
				{ID: &stagedID, IsMain: &isMainFalse},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)
		require.Len(t, pUpdated.Images, 2)

		// Dropped canonical image deleted
		var droppedCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", existingImg2ToDrop).Scan(&droppedCount)
		require.NoError(t, err)
		assert.Equal(t, 0, droppedCount)

		// Cleanup jobs queued for both drop.jpg and rend-drop.jpg
		var cleanupCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key IN ('drop.jpg', 'rend-drop.jpg')").Scan(&cleanupCount)
		require.NoError(t, err)
		assert.Equal(t, 2, cleanupCount)
	})

	t.Run("10_exactly_one_main_derived_on_products_row", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		img1 := uuid.New()
		img2 := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, img1, p.ID, "main.jpg", 0, true, nil, nil)
		insertCanonicalImage(t, ctx, db.Pool, img2, p.ID, "secondary.jpg", 1, false, nil, nil)

		isMainFalse := false
		isMainTrue := true
		// Make img2 the main image
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &img1, IsMain: &isMainFalse},
				{ID: &img2, IsMain: &isMainTrue},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.NoError(t, err)

		assert.Equal(t, "https://storage.zamk.test/secondary.jpg", *pUpdated.MainImageURL)
		assert.Equal(t, "secondary.jpg", *pUpdated.MainImageObjectKey)

		var mainURL, mainKey *string
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", p.ID).Scan(&mainURL, &mainKey)
		require.NoError(t, err)
		assert.Equal(t, "https://storage.zamk.test/secondary.jpg", *mainURL)
		assert.Equal(t, "secondary.jpg", *mainKey)
	})

	t.Run("13_uploading_staged_rejected_with_ErrStagedMediaNotReady", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		stagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, p.ID, "uploading", "upl.jpg")

		isMain := true
		updReq := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedID, IsMain: &isMain},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReq)
		require.ErrorIs(t, err, products.ErrStagedMediaNotReady)

		// Zero mutation: staging row still uploading
		var stgStatus string
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE id = $1", stagedID).Scan(&stgStatus)
		require.NoError(t, err)
		assert.Equal(t, "uploading", stgStatus)
	})

	t.Run("14_15_16_foreign_or_missing_ID_returns_ErrInvalidMediaReference", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)

		// Foreign staged row (belongs to other seller/product)
		foreignProductID := uuid.New()
		foreignSellerID := uuid.New()
		_, err := db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, status, contact_email) VALUES ($1, 'Foreign Brand 1', $2, 'active', 'foreign1@test.com')", foreignSellerID, "f-brand-"+foreignSellerID.String())
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, status, price_cents, currency) VALUES ($1, $2, 'Foreign Prod 1', $3, 'draft', 1000, 'RUB')", foreignProductID, foreignSellerID, "f-prod-"+foreignProductID.String())
		require.NoError(t, err)
		t.Cleanup(func() {
			db.Pool.Exec(context.Background(), "DELETE FROM product_media_staging WHERE seller_id = $1", foreignSellerID)
			db.Pool.Exec(context.Background(), "DELETE FROM product_images WHERE product_id = $1", foreignProductID)
			db.Pool.Exec(context.Background(), "DELETE FROM products WHERE id = $1", foreignProductID)
			db.Pool.Exec(context.Background(), "DELETE FROM sellers WHERE id = $1", foreignSellerID)
		})
		foreignStagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, foreignStagedID, foreignSellerID, foreignProductID, "ready", "f-stg.jpg")

		// Foreign canonical image (belongs to other product)
		foreignCanID := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, foreignCanID, foreignProductID, "f-can.jpg", 0, true, nil, nil)

		missingID := uuid.New()

		isMain := true

		// 14. Foreign staged
		_, err1 := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{{ID: &foreignStagedID, IsMain: &isMain}},
		})
		require.ErrorIs(t, err1, products.ErrInvalidMediaReference)

		// 15. Foreign canonical
		_, err2 := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{{ID: &foreignCanID, IsMain: &isMain}},
		})
		require.ErrorIs(t, err2, products.ErrInvalidMediaReference)

		// 16. Missing ID
		_, err3 := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{{ID: &missingID, IsMain: &isMain}},
		})
		require.ErrorIs(t, err3, products.ErrInvalidMediaReference)

		// All 3 errors return the EXACT same sentinel ErrInvalidMediaReference
		assert.Equal(t, err1.Error(), err2.Error())
		assert.Equal(t, err2.Error(), err3.Error())
	})

	t.Run("17_nil_or_nilUUID_item_rejected_as_ErrInvalidMediaReference", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		nilUUID := uuid.Nil
		isMain := true

		// Item with nil ID
		_, err1 := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{{ID: nil, IsMain: &isMain}},
		})
		require.ErrorIs(t, err1, products.ErrInvalidMediaReference)

		// Item with uuid.Nil
		_, err2 := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{{ID: &nilUUID, IsMain: &isMain}},
		})
		require.ErrorIs(t, err2, products.ErrInvalidMediaReference)
	})

	t.Run("18_duplicate_image_ids_rejected", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		imgID := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, imgID, p.ID, "dup.jpg", 0, true, nil, nil)

		isMain := true
		isMainFalse := false
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &imgID, IsMain: &isMain},
				{ID: &imgID, IsMain: &isMainFalse},
			},
		})
		require.ErrorIs(t, err, products.ErrInvalidMediaSet)
		assert.Contains(t, err.Error(), "duplicate image id")
	})

	t.Run("19_more_than_8_images_rejected", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		var imgs []products.ProductImageRequest
		isMainTrue := true
		isMainFalse := false
		for i := 0; i < 9; i++ {
			id := uuid.New()
			insertCanonicalImage(t, ctx, db.Pool, id, p.ID, fmt.Sprintf("o%d.jpg", i), i, i == 0, nil, nil)
			m := &isMainFalse
			if i == 0 {
				m = &isMainTrue
			}
			imgs = append(imgs, products.ProductImageRequest{ID: &id, IsMain: m})
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: imgs,
		})
		require.ErrorIs(t, err, products.ErrInvalidMediaSet)
		assert.Contains(t, err.Error(), "maximum 8 images allowed")
	})

	t.Run("21_22_main_image_count_validation", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		img1 := uuid.New()
		img2 := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, img1, p.ID, "m1.jpg", 0, true, nil, nil)
		insertCanonicalImage(t, ctx, db.Pool, img2, p.ID, "m2.jpg", 1, false, nil, nil)

		isMainFalse := false
		isMainTrue := true

		// 21: Non-empty + zero main -> rejected
		_, err1 := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &img1, IsMain: &isMainFalse},
				{ID: &img2, IsMain: &isMainFalse},
			},
		})
		require.ErrorIs(t, err1, products.ErrInvalidMediaSet)
		assert.Contains(t, err1.Error(), "got 0")

		// 22: Non-empty + two mains -> rejected
		_, err2 := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &img1, IsMain: &isMainTrue},
				{ID: &img2, IsMain: &isMainTrue},
			},
		})
		require.ErrorIs(t, err2, products.ErrInvalidMediaSet)
		assert.Contains(t, err2.Error(), "got 2")
	})

	t.Run("23_24_color_validation_against_final_requested_variants", func(t *testing.T) {
		// Product initially has NO variants
		p := createDraftWithVariant(nil)
		stagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, p.ID, "ready", "c-val.jpg")

		sku := "new-sku-" + uuid.New().String()
		isMain := true

		// 23: Image ColorID matches the new variant sent in req.Variants -> accepted!
		updReqOK := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:  &sku,
					ColorID:    &colorID1,
					PriceCents: ptr(int64(2000)),
				},
			},
			Images: []products.ProductImageRequest{
				{ID: &stagedID, ColorID: &colorID1, IsMain: &isMain},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReqOK)
		require.NoError(t, err)
		require.Len(t, pUpdated.Images, 1)
		assert.Equal(t, colorID1, *pUpdated.Images[0].ColorID)

		// 24: Image ColorID (colorID2) valid globally in `colors` table, but NOT present in final variants -> rejected!
		stagedID2 := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID2, sellerID, p.ID, "ready", "c-val2.jpg")
		updReqFail := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedID2, ColorID: &colorID2, IsMain: &isMain},
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updReqFail)
		require.ErrorIs(t, err, products.ErrInvalidImageColor)
	})

	t.Run("25_26_color_validation_when_req_Variants_omitted", func(t *testing.T) {
		// Product has existing variant with colorID1
		p := createDraftWithVariant(&colorID1)
		stagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, p.ID, "ready", "c-omitted.jpg")

		isMain := true

		// 25: req.Variants omitted + image ColorID matching existing variant (colorID1) -> accepted
		updOK := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedID, ColorID: &colorID1, IsMain: &isMain},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updOK)
		require.NoError(t, err)

		// 26: req.Variants omitted + image ColorID (colorID2) outside existing variants -> rejected
		stagedID2 := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID2, sellerID, p.ID, "ready", "c-omitted2.jpg")
		updFail := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedID2, ColorID: &colorID2, IsMain: &isMain},
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, updFail)
		require.ErrorIs(t, err, products.ErrInvalidImageColor)
	})

	t.Run("27_28_retry_same_patch_succeeds_through_canonical_consumed_path", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		stagedID := uuid.New()
		insertStagingRow(t, ctx, db.Pool, stagedID, sellerID, p.ID, "ready", "retry.jpg")

		isMain := true
		req := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &stagedID, IsMain: &isMain},
			},
		}

		// First call promotes staged -> canonical
		p1, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, req)
		require.NoError(t, err)
		require.Len(t, p1.Images, 1)
		assert.Equal(t, stagedID, p1.Images[0].ID)

		// 28: Staging is now consumed, canonical exists with same UUID. Retrying exact same request succeeds!
		p2, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, req)
		require.NoError(t, err)
		require.Len(t, p2.Images, 1)
		assert.Equal(t, stagedID, p2.Images[0].ID)
	})

	t.Run("29_staging_metadata_swept_and_canonical_exists_retry_succeeds", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		imgID := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, imgID, p.ID, "swept.jpg", 0, true, nil, nil)

		// Staging row never existed or was swept by TTL
		var stgCount int
		err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", imgID).Scan(&stgCount)
		require.NoError(t, err)
		assert.Equal(t, 0, stgCount)

		isMain := true
		req := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &imgID, IsMain: &isMain},
			},
		}
		pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, req)
		require.NoError(t, err)
		require.Len(t, pUpdated.Images, 1)
		assert.Equal(t, imgID, pUpdated.Images[0].ID)
	})

	t.Run("30_canonical_plus_ready_same_uuid_returns_ErrMediaIntegrityViolation", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		id := uuid.New()
		insertCanonicalImage(t, ctx, db.Pool, id, p.ID, "can30.jpg", 0, true, nil, nil)
		insertStagingRow(t, ctx, db.Pool, id, sellerID, p.ID, "ready", "stg30.jpg")

		isMain := true
		req := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &id, IsMain: &isMain},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, req)
		require.ErrorIs(t, err, products.ErrMediaIntegrityViolation)
	})

	t.Run("31_consumed_staging_without_canonical_returns_ErrMediaIntegrityViolation", func(t *testing.T) {
		p := createDraftWithVariant(&colorID1)
		id := uuid.New()
		insertStagingRow(t, ctx, db.Pool, id, sellerID, p.ID, "consumed", "stg31.jpg")

		isMain := true
		req := products.UpdateProductRequest{
			Images: []products.ProductImageRequest{
				{ID: &id, IsMain: &isMain},
			},
		}
		_, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, req)
		require.ErrorIs(t, err, products.ErrMediaIntegrityViolation)

		// Zero mutation
		var canCount int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", id).Scan(&canCount)
		require.NoError(t, err)
		assert.Equal(t, 0, canCount)
	})
}

func TestCrossSubdocumentRollback_TestA(t *testing.T) {
	// Proves that if media reconciliation succeeds in Step 4,
	// but a LATER subdocument (Step 5: Attributes FK failure) fails,
	// the entire PostgreSQL transaction rolls back!
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	var sellerID uuid.UUID
	err := db.Pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Rollback Cat A', $2, false)", catID, "cat-"+catID.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID)
	})

	slug := "p-rb-a-" + uuid.New().String()
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Rollback Test Product A",
		Slug:       &slug,
		CategoryID: &catID,
	})
	require.NoError(t, err)

	// Prepare initial canonical image
	oldImgID := uuid.New()
	insertCanonicalImage(t, ctx, db.Pool, oldImgID, p.ID, "orig_a.jpg", 0, true, nil, nil)
	_, err = db.Pool.Exec(ctx, "UPDATE products SET main_image_url = 'https://storage.zamk.test/orig_a.jpg', main_image_object_key = 'orig_a.jpg' WHERE id = $1", p.ID)
	require.NoError(t, err)

	// Prepare new ready staging row
	newStagedID := uuid.New()
	insertStagingRow(t, ctx, db.Pool, newStagedID, sellerID, p.ID, "ready", "new_stg_a.jpg")

	// We pass:
	// - Images: replace oldImg with newStagedID (which would succeed in Step 4 and queue cleanup for orig_a.jpg)
	// - Attributes: random nonexistent AttributeDefinitionID (which FAILS with FK violation in Step 5!)
	nonexistentAttrDefID := uuid.New()
	isMain := true
	req := products.UpdateProductRequest{
		Images: []products.ProductImageRequest{
			{ID: &newStagedID, IsMain: &isMain},
		},
		Attributes: []products.ProductAttributeValueRequest{
			{
				AttributeDefinitionID: nonexistentAttrDefID,
				TextValue:             ptr("Should fail with FK error"),
			},
		},
	}

	_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, req)
	require.Error(t, err, "UpdateProductForSeller must fail due to FK error in attributes")

	// VERIFY COMPLETE ROLLBACK:
	// 1. Canonical images: old image is STILL THERE, new image was NOT inserted
	var canIDs []uuid.UUID
	rows, err := db.Pool.Query(ctx, "SELECT id FROM product_images WHERE product_id = $1", p.ID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		canIDs = append(canIDs, id)
	}
	assert.Equal(t, []uuid.UUID{oldImgID}, canIDs, "Canonical images must remain exactly the old image")

	// 2. Staging row: status is STILL 'ready', was NOT marked 'consumed'
	var stgStatus string
	var consumedAt *time.Time
	err = db.Pool.QueryRow(ctx, "SELECT status, consumed_at FROM product_media_staging WHERE id = $1", newStagedID).Scan(&stgStatus, &consumedAt)
	require.NoError(t, err)
	assert.Equal(t, "ready", stgStatus, "Staging row must remain ready due to rollback")
	assert.Nil(t, consumedAt, "consumed_at must be nil")

	// 3. Cleanup jobs: cleanup for orig_a.jpg was NOT enqueued
	var cleanupCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = 'orig_a.jpg'").Scan(&cleanupCount)
	require.NoError(t, err)
	assert.Equal(t, 0, cleanupCount, "Cleanup job must be rolled back")

	// 4. Products table: main_image fields were NOT updated
	var mainURL, mainKey *string
	err = db.Pool.QueryRow(ctx, "SELECT main_image_url, main_image_object_key FROM products WHERE id = $1", p.ID).Scan(&mainURL, &mainKey)
	require.NoError(t, err)
	assert.Equal(t, "https://storage.zamk.test/orig_a.jpg", *mainURL)
	assert.Equal(t, "orig_a.jpg", *mainKey)
}

func TestCrossSubdocumentRollback_TestB(t *testing.T) {
	// Proves that if media reconciliation succeeds in Step 4,
	// but moderation log / revision write fails in Step 6,
	// the entire PostgreSQL transaction rolls back!
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	var sellerID uuid.UUID
	err := db.Pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Rollback Cat B', $2, false)", catID, "cat-"+catID.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID)
	})

	slug := "p-rb-b-" + uuid.New().String()
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Rollback Test Product B",
		Slug:       &slug,
		CategoryID: &catID,
	})
	require.NoError(t, err)

	// Set product to pending_moderation status so that editing triggers moderation reset & AddModerationLog!
	_, err = db.Pool.Exec(ctx, "UPDATE products SET status = 'pending_moderation' WHERE id = $1", p.ID)
	require.NoError(t, err)

	// Prepare ready staging row
	newStagedID := uuid.New()
	insertStagingRow(t, ctx, db.Pool, newStagedID, sellerID, p.ID, "ready", "new_stg_b.jpg")

	// Install a trigger on product_moderation_logs to simulate failure during AddModerationLog
	_, err = db.Pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION fail_test_b_mod_log() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'simulated moderation log failure';
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER trg_test_b_fail_mod_log
		BEFORE INSERT ON product_moderation_logs
		FOR EACH ROW EXECUTE FUNCTION fail_test_b_mod_log();
	`)
	require.NoError(t, err)

	defer func() {
		db.Pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS trg_test_b_fail_mod_log ON product_moderation_logs")
		db.Pool.Exec(context.Background(), "DROP FUNCTION IF EXISTS fail_test_b_mod_log()")
	}()

	isMain := true
	req := products.UpdateProductRequest{
		Images: []products.ProductImageRequest{
			{ID: &newStagedID, IsMain: &isMain},
		},
	}

	_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated moderation log failure")

	// VERIFY COMPLETE ROLLBACK:
	// 1. Canonical images: new image was NOT committed
	var canCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE id = $1", newStagedID).Scan(&canCount)
	require.NoError(t, err)
	assert.Equal(t, 0, canCount, "Promoted image must be rolled back")

	// 2. Staging row remains 'ready'
	var stgStatus string
	err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE id = $1", newStagedID).Scan(&stgStatus)
	require.NoError(t, err)
	assert.Equal(t, "ready", stgStatus, "Staging row must remain ready due to rollback")

	// 3. Product status remains 'pending_moderation', NOT changed to 'draft'
	var prodStatus string
	err = db.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", p.ID).Scan(&prodStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending_moderation", prodStatus, "Product status change must be rolled back")
}

func TestMediaIndistinguishableSecurity(t *testing.T) {
	// Explicitly proves that:
	// - Foreign staged UUID
	// - Foreign canonical UUID
	// - Nonexistent UUID
	// produce the EXACT same HTTP 400 Bad Request response with:
	// {"error":{"code":"invalid_media_reference","message":"Invalid media reference"}}
	// Zero ownership leak!
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	handler := products.NewHandler(svc, nil)

	catID := uuid.New()
	_, err := db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Sec Cat', $2, false)", catID, "cat-"+catID.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID)
	})

	slug := "p-sec-" + uuid.New().String()
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Security Test Product",
		Slug:       &slug,
		CategoryID: &catID,
	})
	require.NoError(t, err)

	foreignProductID := uuid.New()
	foreignSellerID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, status, contact_email) VALUES ($1, 'Foreign Brand Sec', $2, 'active', 'foreignsec@test.com')", foreignSellerID, "f-brand-"+foreignSellerID.String())
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, status, price_cents, currency) VALUES ($1, $2, 'Foreign Prod Sec', $3, 'draft', 1000, 'RUB')", foreignProductID, foreignSellerID, "f-prod-sec-"+foreignProductID.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM product_media_staging WHERE seller_id = $1", foreignSellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM product_images WHERE product_id = $1", foreignProductID)
		db.Pool.Exec(context.Background(), "DELETE FROM products WHERE id = $1", foreignProductID)
		db.Pool.Exec(context.Background(), "DELETE FROM sellers WHERE id = $1", foreignSellerID)
	})
	foreignStagedID := uuid.New()
	insertStagingRow(t, ctx, db.Pool, foreignStagedID, foreignSellerID, foreignProductID, "ready", "f-sec-stg.jpg")

	foreignCanID := uuid.New()
	insertCanonicalImage(t, ctx, db.Pool, foreignCanID, foreignProductID, "f-sec-can.jpg", 0, true, nil, nil)

	nonexistentID := uuid.New()

	executePATCH := func(imgID uuid.UUID) (int, string) {
		body := map[string]any{
			"images": []map[string]any{
				{
					"id":       imgID.String(),
					"imageUrl": "https://storage.zamk.test/dummy.jpg",
					"isMain":   true,
				},
			},
		}
		jsonBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/seller/products/%s", p.ID), bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = req.WithContext(context.WithValue(req.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.UpdateProduct(rec, req)
		return rec.Code, rec.Body.String()
	}

	code1, body1 := executePATCH(foreignStagedID)
	code2, body2 := executePATCH(foreignCanID)
	code3, body3 := executePATCH(nonexistentID)

	assert.Equal(t, http.StatusBadRequest, code1)
	assert.Equal(t, http.StatusBadRequest, code2)
	assert.Equal(t, http.StatusBadRequest, code3)

	assert.JSONEq(t, `{"error":{"code":"invalid_media_reference","message":"Invalid media reference"}}`, body1)
	assert.JSONEq(t, `{"error":{"code":"invalid_media_reference","message":"Invalid media reference"}}`, body2)
	assert.JSONEq(t, `{"error":{"code":"invalid_media_reference","message":"Invalid media reference"}}`, body3)

	assert.Equal(t, body1, body2, "Foreign staged and foreign canonical must have identical response bodies")
	assert.Equal(t, body2, body3, "Foreign canonical and nonexistent UUID must have identical response bodies")
}
