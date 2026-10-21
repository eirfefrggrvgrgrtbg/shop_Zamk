package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/jackc/pgx/v5"
)

// memProvider is an in-memory object store: uploads are kept, downloads return the real bytes,
// and objects can be removed to simulate a lost file.
type memProvider struct {
	fakeStorageProvider
	omu     sync.Mutex
	objects map[string][]byte
}

func newMemProvider() *memProvider { return &memProvider{objects: map[string][]byte{}} }

func (m *memProvider) UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*storage.StoredObject, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}
	m.omu.Lock()
	m.objects[objectKey] = buf.Bytes()
	m.omu.Unlock()
	return &storage.StoredObject{ObjectURL: m.BuildPublicURL(objectKey), ObjectKey: objectKey, Size: objectSize}, nil
}

func (m *memProvider) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	m.omu.Lock()
	defer m.omu.Unlock()
	b, ok := m.objects[objectKey]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return b, nil
}

func (m *memProvider) DownloadObjectBounded(ctx context.Context, objectKey string, maxBytes int64) ([]byte, error) {
	return m.DownloadObject(ctx, objectKey)
}

func (m *memProvider) exists(ctx context.Context, key string) (bool, error) {
	m.omu.Lock()
	defer m.omu.Unlock()
	_, ok := m.objects[key]
	return ok, nil
}

func (m *memProvider) remove(key string) {
	m.omu.Lock()
	delete(m.objects, key)
	m.omu.Unlock()
}

// CATALOG.C5.2: new photos of a published product's revision are cropped to 4:5 while still staged, shown to the
// seller and Admin as the pending gallery, protected from the TTL sweep, and promoted only on approval.
func TestRevisionPhotos(t *testing.T) {
	ctx := context.Background()
	db, err := postgres.NewClient(ctx, testutil.GetTestDatabaseURL())
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, db.Pool)

	sellerID, strangerSellerID := uuid.New(), uuid.New()
	userID, strangerUserID, adminID := uuid.New(), uuid.New(), uuid.New()
	brandID, catID, productID := uuid.New(), uuid.New(), uuid.New()
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
			{"DELETE FROM product_revisions WHERE product_id = $1", []any{productID}},
			{"DELETE FROM product_images WHERE product_id = $1", []any{productID}},
			{"DELETE FROM product_variants WHERE product_id = $1", []any{productID}},
			{"DELETE FROM product_moderation_logs WHERE product_id = $1", []any{productID}},
			{"DELETE FROM products WHERE seller_id = $1", []any{sellerID}},
			{"DELETE FROM categories WHERE id = $1", []any{catID}},
			{"DELETE FROM brands WHERE id = $1", []any{brandID}},
			{"DELETE FROM seller_users WHERE user_id IN ($1, $2)", []any{userID, strangerUserID}},
			{"DELETE FROM sellers WHERE id IN ($1, $2)", []any{sellerID, strangerSellerID}},
			{"DELETE FROM users WHERE id IN ($1, $2, $3)", []any{userID, strangerUserID, adminID}},
		} {
			if _, err := db.Pool.Exec(c, q.sql, q.args...); err != nil {
				t.Errorf("cleanup failed: %s: %v", q.sql, err)
			}
		}
		db.Close()
	})

	mk := func(sql string, args ...any) {
		_, err := db.Pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	mk("INSERT INTO users (id, email, password_hash, name, role) VALUES ($1, $2, 'h', 'Rev Seller', 'seller'), ($3, $4, 'h', 'Stranger', 'seller'), ($5, $6, 'h', 'Rev Admin', 'admin')",
		userID, fmt.Sprintf("rp-s-%s@zamk.ru", tag), strangerUserID, fmt.Sprintf("rp-x-%s@zamk.ru", tag), adminID, fmt.Sprintf("rp-a-%s@zamk.ru", tag))
	mk("INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'RP Brand', 'active', $2), ($3, 'RP Stranger', 'active', $4)",
		sellerID, fmt.Sprintf("rp-s-%s@zamk.ru", tag), strangerSellerID, fmt.Sprintf("rp-x-%s@zamk.ru", tag))
	mk("INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner'), ($4, $5, $6, 'owner')", uuid.New(), sellerID, userID, uuid.New(), strangerSellerID, strangerUserID)
	mk("INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'RP Brand Name', $2, true)", brandID, "rp-brand-"+tag)
	mk("INSERT INTO categories (id, name, slug) VALUES ($1, 'RP Cat', $2)", catID, "rp-cat-"+tag)
	mk("INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency, description) VALUES ($1, $2, $3, $4, 'RP Prod', $5, 'published', 100000, 'RUB', 'd')", productID, sellerID, brandID, catID, "rp-prod-"+tag)
	mk("INSERT INTO product_variants (id, product_id, sku, seller_sku, price_cents, is_active, barcode) VALUES ($1, $2, $3, $3, 100000, true, $4)", uuid.New(), productID, "RP-"+tag, "ZMK-rp"+tag)

	oldImg := uuid.New()
	oldKey := fmt.Sprintf("products/%s/%s/%s.jpg", sellerID, productID, oldImg)
	oldRendition := fmt.Sprintf("products/%s/%s/%s_rendition.jpg", sellerID, productID, oldImg)
	mk(`INSERT INTO product_images (id, product_id, image_url, object_key, sort_order, is_main, width, height, crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key)
		VALUES ($1, $2, $3, $4, 0, true, 800, 1000, 0, 0, 1, 1, $5, $6)`, oldImg, productID, "http://x/"+oldKey, oldKey, "http://x/"+oldRendition, oldRendition)
	mk("UPDATE products SET main_image_url = $2, main_image_object_key = $3 WHERE id = $1", productID, "http://x/"+oldKey, oldKey)

	mem := newMemProvider()
	productsRepo := products.NewRepository(db.Pool)
	sellersRepo := sellers.NewRepository(db.Pool)
	storageSvc := storage.NewService(mem, productsRepo, catalog.NewRepository(db.Pool), sellersRepo, db)
	prodSvc := products.NewService(productsRepo, sellersRepo, db, nil, nil).WithMediaObjectVerifier(mem.exists)
	yes := true

	jpegBytes := createTestJPEG(800, 1000, color.RGBA{R: 30, G: 90, B: 160, A: 255})
	stage := func() uuid.UUID {
		resp, err := storageSvc.StageSellerProductImage(ctx, userID, productID, uuid.New(), bytes.NewReader(jpegBytes), "p.jpg", int64(len(jpegBytes)), "image/jpeg", 10)
		require.NoError(t, err)
		return resp.StagedMediaID
	}
	propose := func(ids ...uuid.UUID) error {
		var reqs []products.ProductImageRequest
		for i, id := range ids {
			id, main := id, i == 0
			reqs = append(reqs, products.ProductImageRequest{ID: &id, ImageURL: "x", IsMain: &main})
		}
		_, err := prodSvc.UpdateProductForSeller(ctx, userID, productID, products.UpdateProductRequest{ContinueSelling: &yes, Images: reqs})
		return err
	}
	galleryIDs := func() []uuid.UUID {
		rows, err := db.Pool.Query(ctx, "SELECT id FROM product_images WHERE product_id = $1 ORDER BY sort_order", productID)
		require.NoError(t, err)
		defer rows.Close()
		var out []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			require.NoError(t, rows.Scan(&id))
			out = append(out, id)
		}
		return out
	}
	revisions := func() map[string]int {
		out := map[string]int{}
		rows, err := db.Pool.Query(ctx, "SELECT status, count(*) FROM product_revisions WHERE product_id = $1 GROUP BY status", productID)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var s string
			var n int
			require.NoError(t, rows.Scan(&s, &n))
			out[s] = n
		}
		return out
	}
	productStatus := func() string {
		var s string
		require.NoError(t, db.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", productID).Scan(&s))
		return s
	}
	stagingRow := func(id uuid.UUID) (status string, renditionKey *string, found bool) {
		err := db.Pool.QueryRow(ctx, "SELECT status, rendition_object_key FROM product_media_staging WHERE id = $1", id).Scan(&status, &renditionKey)
		if err == pgx.ErrNoRows {
			return "", nil, false
		}
		require.NoError(t, err)
		return status, renditionKey, true
	}

	// ---- 1. new photo goes into a pending revision; the published gallery is untouched ----
	s1 := stage()
	require.NoError(t, propose(oldImg, s1))
	require.Equal(t, []uuid.UUID{oldImg}, galleryIDs(), "published gallery unchanged before approval")
	require.Equal(t, "published", productStatus(), "continueSelling keeps the product on sale")
	require.Equal(t, map[string]int{"pending": 1}, revisions())

	// ---- 2. a photo that is not a finished upload of this product is refused at submission ----
	require.ErrorIs(t, propose(oldImg, uuid.New()), products.ErrInvalidMediaReference)

	// ---- 3. approval is blocked while the new photo has no 4:5 rendition; nothing changes ----
	err = prodSvc.ApproveProduct(ctx, adminID, productID, nil)
	require.ErrorIs(t, err, products.ErrStagedMediaRenditionMissing)
	require.ErrorIs(t, err, products.ErrRevisionNotApplicable)
	require.Equal(t, []uuid.UUID{oldImg}, galleryIDs())
	require.Equal(t, map[string]int{"pending": 1}, revisions())

	// ---- 4. crop of a staged photo: ownership, aspect and storage ----
	_, err = storageSvc.CropSellerProductImage(ctx, strangerUserID, productID, s1, storage.CropImageRequest{})
	require.ErrorIs(t, err, storage.ErrProductNotOwned, "another seller cannot crop it")
	_, err = storageSvc.CropSellerProductImage(ctx, userID, productID, uuid.New(), storage.CropImageRequest{})
	require.ErrorIs(t, err, products.ErrStagedMediaNotFound, "unknown id")
	_, err = storageSvc.CropSellerProductImage(ctx, userID, productID, s1, storage.CropImageRequest{CropX: 0, CropY: 0, CropWidth: 1, CropHeight: 0.5})
	require.ErrorIs(t, err, storage.ErrInvalidCropAspect)
	_, _, found := stagingRow(s1)
	require.True(t, found)

	resp, err := storageSvc.CropSellerProductImage(ctx, userID, productID, s1, storage.CropImageRequest{})
	require.NoError(t, err)
	renditionKey := fmt.Sprintf("products/%s/%s/staged/%s_rendition.jpg", sellerID, productID, s1)
	require.Contains(t, resp.RenditionURL, renditionKey)
	ok, _ := mem.exists(ctx, renditionKey)
	require.True(t, ok, "rendition uploaded to storage")
	var cw float64
	var dbKey string
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT crop_width, rendition_object_key FROM product_media_staging WHERE id = $1", s1).Scan(&cw, &dbKey))
	require.Equal(t, renditionKey, dbKey)
	require.InDelta(t, 1.0, cw, 0.0001)
	// re-cropping the same photo reuses the same object key: no duplicate objects
	_, err = storageSvc.CropSellerProductImage(ctx, userID, productID, s1, storage.CropImageRequest{})
	require.NoError(t, err)

	// ---- 5. Seller sees the pending gallery; Admin sees what is proposed ----
	seller, err := prodSvc.GetSellerProduct(ctx, userID, productID)
	require.NoError(t, err)
	require.Len(t, seller.Images, 2)
	require.Equal(t, oldImg, seller.Images[0].ID)
	require.Equal(t, s1, seller.Images[1].ID)
	require.True(t, seller.Images[1].Pending)
	require.NotNil(t, seller.Images[1].RenditionURL)
	require.NotNil(t, seller.PendingRevision)
	admin, err := prodSvc.GetAdminProductDetail(ctx, productID)
	require.NoError(t, err)
	require.NotNil(t, admin.PendingRevision)
	require.Len(t, admin.PendingRevision.Images, 2)
	require.Len(t, admin.Images, 1, "Admin's live product still shows the published gallery")
	require.Zero(t, admin.PendingRevision.ImagesMissing)

	// ---- 6. the TTL sweep spares a photo of a pending revision, and takes it back after the decision ----
	mk("UPDATE product_media_staging SET created_at = now() - interval '5000 hours' WHERE id = $1", s1)
	n, err := productsRepo.ExpireStaleStagedMedia(ctx, 4000*time.Hour, 4000*time.Hour, 4000*time.Hour, 100)
	require.NoError(t, err)
	_, _, found = stagingRow(s1)
	require.True(t, found, "pending revision protects its photos (swept %d)", n)

	// ---- 7. a lost file blocks approval safely (storage is checked before the transaction) ----
	mem.remove(renditionKey)
	err = prodSvc.ApproveProduct(ctx, adminID, productID, nil)
	require.ErrorIs(t, err, products.ErrRevisionMediaObjectMissing)
	require.Equal(t, []uuid.UUID{oldImg}, galleryIDs())
	require.Equal(t, map[string]int{"pending": 1}, revisions())
	_, err = storageSvc.CropSellerProductImage(ctx, userID, productID, s1, storage.CropImageRequest{}) // seller repairs by re-cropping
	require.NoError(t, err)

	// ---- 8. an error during the application leaves nothing half-applied ----
	s2 := stage() // new, uncropped
	require.NoError(t, propose(s2, oldImg))
	rev, err := productsRepo.GetPendingRevision(ctx, productID)
	require.NoError(t, err)
	err = db.RunInTx(ctx, func(tx pgx.Tx) error {
		return productsRepo.WithTx(tx).PromoteProductRevision(ctx, productID, rev.ID)
	})
	require.ErrorIs(t, err, products.ErrStagedMediaRenditionMissing)
	require.Equal(t, []uuid.UUID{oldImg}, galleryIDs(), "rolled back")
	require.Equal(t, map[string]int{"pending": 1, "rejected": 1}, revisions(), "the proposal is still pending after the rollback (the earlier one was superseded)")

	// ---- 9. rejection keeps the published gallery; the proposal's photos go back under normal cleanup ----
	require.NoError(t, prodSvc.RejectProduct(ctx, adminID, productID, "not suitable"))
	require.Equal(t, []uuid.UUID{oldImg}, galleryIDs())
	require.Equal(t, "published", productStatus())
	mk("UPDATE product_media_staging SET created_at = now() - interval '5000 hours' WHERE id = $1", s2)
	_, err = productsRepo.ExpireStaleStagedMedia(ctx, 4000*time.Hour, 4000*time.Hour, 4000*time.Hour, 100)
	require.NoError(t, err)
	_, _, found = stagingRow(s2)
	require.False(t, found, "after the decision the staged photo is swept")
	var jobs int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM product_media_cleanup_jobs WHERE object_key LIKE '%' || $1 || '%'", s2.String()).Scan(&jobs))
	require.Equal(t, 1, jobs, "its object is queued for deletion")

	// ---- 10. a fresh proposal: both photos new, order and main flag preserved; approval promotes them ----
	s3, s4 := stage(), stage()
	for _, id := range []uuid.UUID{s3, s4} {
		_, err = storageSvc.CropSellerProductImage(ctx, userID, productID, id, storage.CropImageRequest{})
		require.NoError(t, err)
	}
	require.NoError(t, propose(s3, s4, oldImg))
	require.NoError(t, prodSvc.ApproveProduct(ctx, adminID, productID, nil))
	require.Equal(t, []uuid.UUID{s3, s4, oldImg}, galleryIDs(), "order follows the proposal")
	var mainID uuid.UUID
	var renditionURL *string
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT id FROM product_images WHERE product_id = $1 AND is_main", productID).Scan(&mainID))
	require.Equal(t, s3, mainID)
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT rendition_url FROM product_images WHERE id = $1", s3).Scan(&renditionURL))
	require.NotNil(t, renditionURL, "rendition travelled with the promoted photo")
	st, _, _ := stagingRow(s3)
	require.Equal(t, "consumed", st)
	require.Equal(t, map[string]int{"approved": 1, "rejected": 2}, revisions())

	// the old published photo is kept (it was part of the proposal); repeat approval changes nothing
	before := galleryIDs()
	err = prodSvc.ApproveProduct(ctx, adminID, productID, nil)
	require.ErrorIs(t, err, products.ErrInvalidStatusTransition)
	require.Equal(t, before, galleryIDs())

	// ---- 11. removing a photo from the proposal: dropped on approval, files queued for deletion only then ----
	require.NoError(t, propose(s3, oldImg))
	require.Len(t, galleryIDs(), 3, "still three until approval")
	require.NoError(t, prodSvc.ApproveProduct(ctx, adminID, productID, nil))
	require.Equal(t, []uuid.UUID{s3, oldImg}, galleryIDs())
	assert.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM product_media_cleanup_jobs WHERE object_key LIKE '%' || $1 || '%'", s4.String()).Scan(&jobs))
	assert.GreaterOrEqual(t, jobs, 1, "the dropped photo's files are queued for deferred deletion")
}
