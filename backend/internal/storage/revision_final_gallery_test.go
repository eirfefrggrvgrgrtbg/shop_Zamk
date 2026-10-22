package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// CATALOG.C5.4: approval of a published product's revision evaluates the RESULTING gallery: a live product without a
// main photo is approved once the revision adds a valid 4:5 photo; a revision that removes every photo is refused.
func TestRevisionFinalGalleryChecks(t *testing.T) {
	ctx := context.Background()
	db, err := postgres.NewClient(ctx, testutil.GetTestDatabaseURL())
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, db.Pool)

	sellerID, userID, adminID, brandID, catID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	noPhoto, withPhoto := uuid.New(), uuid.New()
	tag := uuid.NewString()[:8]
	t.Cleanup(func() {
		c := context.Background()
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE 'products/' || $1 || '/%'", []any{sellerID.String()}},
			{"DELETE FROM product_media_staging WHERE seller_id = $1", []any{sellerID}},
			{"UPDATE products SET live_revision_id = NULL WHERE seller_id = $1", []any{sellerID}},
			{"DELETE FROM product_revisions WHERE product_id = ANY($1)", []any{[]uuid.UUID{noPhoto, withPhoto}}},
			{"DELETE FROM product_moderation_logs WHERE product_id = ANY($1)", []any{[]uuid.UUID{noPhoto, withPhoto}}},
			{"DELETE FROM product_images WHERE product_id = ANY($1)", []any{[]uuid.UUID{noPhoto, withPhoto}}},
			{"DELETE FROM inventory_items WHERE product_id = ANY($1)", []any{[]uuid.UUID{noPhoto, withPhoto}}},
			{"DELETE FROM product_variants WHERE product_id = ANY($1)", []any{[]uuid.UUID{noPhoto, withPhoto}}},
			{"DELETE FROM products WHERE seller_id = $1", []any{sellerID}},
			{"DELETE FROM categories WHERE id = $1", []any{catID}},
			{"DELETE FROM brands WHERE id = $1", []any{brandID}},
			{"DELETE FROM seller_users WHERE user_id = $1", []any{userID}},
			{"DELETE FROM sellers WHERE id = $1", []any{sellerID}},
			{"DELETE FROM users WHERE id IN ($1, $2)", []any{userID, adminID}},
		} {
			if _, err := db.Pool.Exec(c, q.sql, q.args...); err != nil {
				t.Errorf("cleanup failed: %s: %v", q.sql, err)
			}
		}
		db.Close()
	})
	mk := func(sql string, args ...any) {
		_, err := db.Pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	mk("INSERT INTO users (id, email, password_hash, name, role) VALUES ($1, $2, 'h', 'FG Seller', 'seller'), ($3, $4, 'h', 'FG Admin', 'admin')",
		userID, fmt.Sprintf("fg-s-%s@zamk.ru", tag), adminID, fmt.Sprintf("fg-a-%s@zamk.ru", tag))
	mk("INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'FG Brand', 'active', $2)", sellerID, fmt.Sprintf("fg-s-%s@zamk.ru", tag))
	mk("INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	mk("INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'FG Brand Name', $2, true)", brandID, "fg-brand-"+tag)
	mk("INSERT INTO categories (id, name, slug) VALUES ($1, 'FG Cat', $2)", catID, "fg-cat-"+tag)
	for _, pid := range []uuid.UUID{noPhoto, withPhoto} {
		mk(`INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency, description)
			VALUES ($1, $2, $3, $4, 'FG Prod', $5, 'published', 100000, 'RUB', 'Футболка для проверки итоговой галереи.')`, pid, sellerID, brandID, catID, "fg-"+pid.String()[:8])
		vid := uuid.New()
		mk("INSERT INTO product_variants (id, product_id, sku, seller_sku, price_cents, is_active, barcode) VALUES ($1, $2, $3, $3, 100000, true, $4)",
			vid, pid, "FG-"+pid.String()[:8], "ZMK-fg"+pid.String()[:8])
		// FBO fixture: ZAMK holds stock, so the product is visible in the shop
		mk("INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(), $1, $2, $3, 5, 0)", pid, sellerID, vid)
	}
	oldImg := uuid.New()
	oldKey := fmt.Sprintf("products/%s/%s/%s.jpg", sellerID, withPhoto, oldImg)
	mk(`INSERT INTO product_images (id, product_id, image_url, object_key, sort_order, is_main, width, height, crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key)
		VALUES ($1, $2, $3, $4, 0, true, 800, 1000, 0, 0, 1, 1, $3, $4)`, oldImg, withPhoto, "http://x/"+oldKey, oldKey)
	mk("UPDATE products SET main_image_url = $2 WHERE id = $1", withPhoto, "http://x/"+oldKey)

	mem := newMemProvider()
	productsRepo := products.NewRepository(db.Pool)
	sellersRepo := sellers.NewRepository(db.Pool)
	storageSvc := storage.NewService(mem, productsRepo, catalog.NewRepository(db.Pool), sellersRepo, db)
	prodSvc := products.NewService(productsRepo, sellersRepo, db, nil, nil).WithMediaObjectVerifier(mem.exists)
	yes := true
	jpeg := createTestJPEG(800, 1000, color.RGBA{R: 40, G: 40, B: 40, A: 255})
	status := func(id uuid.UUID) string {
		var s string
		require.NoError(t, db.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", id).Scan(&s))
		return s
	}
	publicImages := func(id uuid.UUID) int {
		p, err := prodSvc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err)
		return len(p.Images)
	}
	checkFailed := func(id uuid.UUID) []string {
		p, err := prodSvc.GetAdminProductDetail(ctx, id)
		require.NoError(t, err)
		var out []string
		for _, c := range p.ModerationChecks {
			if !c.Pass {
				out = append(out, c.Code)
			}
		}
		return out
	}

	t.Run("9. live product without a main photo: the revision adds a cropped 4:5 photo and is approved", func(t *testing.T) {
		require.Equal(t, []string{products.CheckMainImage}, checkFailed(noPhoto), "the live product alone lacks a photo")
		resp, err := storageSvc.StageSellerProductImage(ctx, userID, noPhoto, uuid.New(), bytes.NewReader(jpeg), "p.jpg", int64(len(jpeg)), "image/jpeg", 10)
		require.NoError(t, err)
		_, err = storageSvc.CropSellerProductImage(ctx, userID, noPhoto, resp.StagedMediaID, storage.CropImageRequest{})
		require.NoError(t, err)
		id, main := resp.StagedMediaID, true
		_, err = prodSvc.UpdateProductForSeller(ctx, userID, noPhoto, products.UpdateProductRequest{ContinueSelling: &yes, Images: []products.ProductImageRequest{{ID: &id, ImageURL: "x", IsMain: &main}}})
		require.NoError(t, err)
		require.Empty(t, checkFailed(noPhoto), "Admin evaluates the proposed gallery")
		require.Equal(t, 0, publicImages(noPhoto), "the photo is not public before approval")
		require.NoError(t, prodSvc.ApproveProduct(ctx, adminID, noPhoto, nil))
		require.Equal(t, 1, publicImages(noPhoto))
		require.Equal(t, products.StatusPublished, status(noPhoto))
	})

	t.Run("10. live product with a photo: a revision removing every photo is refused and changes nothing", func(t *testing.T) {
		require.Empty(t, checkFailed(withPhoto))
		_, err := prodSvc.UpdateProductForSeller(ctx, userID, withPhoto, products.UpdateProductRequest{ContinueSelling: &yes, Images: []products.ProductImageRequest{}})
		require.NoError(t, err)
		require.Equal(t, []string{products.CheckMainImage}, checkFailed(withPhoto), "the RESULT has no photo")
		err = prodSvc.ApproveProduct(ctx, adminID, withPhoto, nil)
		require.ErrorIs(t, err, products.ErrModerationChecksFailed)
		require.Equal(t, 1, publicImages(withPhoto), "the live gallery is kept")
		require.Equal(t, products.StatusPublished, status(withPhoto))
		require.NoError(t, prodSvc.RejectProduct(ctx, adminID, withPhoto, "нужно хотя бы одно фото"))
		require.Equal(t, 1, publicImages(withPhoto))
	})
}
