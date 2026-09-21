package storage_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type fakeStorageProvider struct {
	mu            sync.Mutex
	uploads       []fakeUpload
	failUpload    bool
	failUploadErr error
	onAfterUpload func(objectKey string)
}

type fakeUpload struct {
	ObjectKey   string
	Size        int64
	ContentType string
	Bytes       []byte
}

func (f *fakeStorageProvider) UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*storage.StoredObject, error) {
	f.mu.Lock()
	if f.failUpload {
		err := f.failUploadErr
		if err == nil {
			err = errors.New("simulated S3 upload failure")
		}
		f.mu.Unlock()
		return nil, err
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(reader)
	data := buf.Bytes()
	f.uploads = append(f.uploads, fakeUpload{
		ObjectKey:   objectKey,
		Size:        objectSize,
		ContentType: contentType,
		Bytes:       data,
	})
	hook := f.onAfterUpload
	f.mu.Unlock()

	if hook != nil {
		hook(objectKey)
	}

	return &storage.StoredObject{
		ObjectURL: f.BuildPublicURL(objectKey),
		ObjectKey: objectKey,
		Size:      objectSize,
	}, nil
}

func (f *fakeStorageProvider) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	return []byte("fake content"), nil
}

func (f *fakeStorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	return nil
}

func (f *fakeStorageProvider) BuildPublicURL(objectKey string) string {
	return "http://localhost:9000/media/" + objectKey
}

func (f *fakeStorageProvider) getUploadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.uploads)
}

func (f *fakeStorageProvider) getLastUpload() (fakeUpload, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.uploads) == 0 {
		return fakeUpload{}, false
	}
	return f.uploads[len(f.uploads)-1], true
}

func (f *fakeStorageProvider) setFailUpload(fail bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failUpload = fail
	f.failUploadErr = err
}

func (f *fakeStorageProvider) setOnAfterUpload(fn func(objectKey string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onAfterUpload = fn
}

func createTestJPEG(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

func buildStageMultipartRequest(url string, clientMediaID string, filename, contentType string, data []byte) (*http.Request, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if clientMediaID != "" {
		if err := writer.WriteField("clientMediaId", clientMediaID); err != nil {
			return nil, err
		}
	}

	if data != nil {
		partHeader := make(map[string][]string)
		partHeader["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="image"; filename="%s"`, filename)}
		if contentType != "" {
			partHeader["Content-Type"] = []string{contentType}
		}
		part, err := writer.CreatePart(partHeader)
		if err != nil {
			return nil, err
		}
		if _, err = part.Write(data); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestStagedProductImageUploadEndpoint(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	}

	ctx := context.Background()
	db, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	// Invariant Check
	testutil.AssertTestDatabase(t, db.Pool)
	var dbName string
	err = db.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "integration tests must use zamk_test database")

	// IDs
	userID := uuid.New()
	userID2 := uuid.New()
	sellerID := uuid.New()
	sellerID2 := uuid.New()
	brandID := uuid.New()
	catID := uuid.New()

	userEmail := fmt.Sprintf("stagetest-%s@zamk.ru", userID)
	userEmail2 := fmt.Sprintf("stagetest-%s@zamk.ru", userID2)
	brandSlug := fmt.Sprintf("brand-stage-%s", brandID)
	catSlug := fmt.Sprintf("cat-stage-%s", catID)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		queries := []struct {
			name string
			sql  string
			args []any
		}{
			{"product_media_cleanup_jobs", "DELETE FROM product_media_cleanup_jobs WHERE object_key LIKE 'products/' || $1 || '/%' OR object_key LIKE 'products/' || $2 || '/%'", []any{sellerID.String(), sellerID2.String()}},
			{"product_media_staging", "DELETE FROM product_media_staging WHERE seller_id IN ($1, $2)", []any{sellerID, sellerID2}},
			{"product_images", "DELETE FROM product_images WHERE product_id IN (SELECT id FROM products WHERE seller_id IN ($1, $2))", []any{sellerID, sellerID2}},
			{"products", "DELETE FROM products WHERE seller_id IN ($1, $2)", []any{sellerID, sellerID2}},
			{"categories", "DELETE FROM categories WHERE id = $1", []any{catID}},
			{"brands", "DELETE FROM brands WHERE id = $1", []any{brandID}},
			{"seller_users", "DELETE FROM seller_users WHERE user_id IN ($1, $2)", []any{userID, userID2}},
			{"sellers", "DELETE FROM sellers WHERE id IN ($1, $2)", []any{sellerID, sellerID2}},
			{"users", "DELETE FROM users WHERE id IN ($1, $2)", []any{userID, userID2}},
		}
		for _, q := range queries {
			if _, err := db.Pool.Exec(cleanupCtx, q.sql, q.args...); err != nil {
				t.Errorf("cleanup failed for %s: %v", q.name, err)
			}
		}
		db.Close()
	})

	// Fixtures
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, role) VALUES ($1, $2, 'hash', 'Stage Seller 1', 'seller')", userID, userEmail)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Stage Brand 1', 'active', $2)", sellerID, userEmail)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, role) VALUES ($1, $2, 'hash', 'Stage Seller 2', 'seller')", userID2, userEmail2)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, status, contact_email) VALUES ($1, 'Stage Brand 2', 'active', $2)", sellerID2, userEmail2)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID2, userID2)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'Stage Test Brand', $2, true)", brandID, brandSlug)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Stage Test Cat', $2)", catID, catSlug)
	require.NoError(t, err)

	createProduct := func(pID uuid.UUID, sID uuid.UUID, status string) {
		slug := fmt.Sprintf("prod-%s", pID)
		_, err := db.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, $4, 'Stage Prod', $5, $6, 100, 'RUB')", pID, sID, brandID, catID, slug, status)
		require.NoError(t, err)
	}

	prodID1 := uuid.New()
	createProduct(prodID1, sellerID, "draft")

	prodID2 := uuid.New()
	createProduct(prodID2, sellerID2, "draft")

	blockedProdID := uuid.New()
	createProduct(blockedProdID, sellerID, "blocked")

	fakeStorage := &fakeStorageProvider{}
	productsRepo := products.NewRepository(db.Pool)
	catalogRepo := catalog.NewRepository(db.Pool)
	sellersRepo := sellers.NewRepository(db.Pool)
	storageService := storage.NewService(fakeStorage, productsRepo, catalogRepo, sellersRepo, db)
	handlerCfg := &config.S3Config{UploadMaxSizeMB: 10}
	storageHandler := storage.NewHandler(storageService, handlerCfg)

	// Setup router
	r := chi.NewRouter()
	r.Post("/api/seller/products/{id}/images/stage", storageHandler.StageSellerProductImage)

	validImageBytes := createTestJPEG(800, 1000, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	validImageSHA := sha256.Sum256(validImageBytes)
	validImageSHAHex := hex.EncodeToString(validImageSHA[:])

	differentImageBytes := createTestJPEG(800, 1000, color.RGBA{R: 50, G: 100, B: 200, A: 255})

	// 1. VALID NEW UPLOAD (Case A)
	t.Run("1. valid new upload moves to ready with correct metadata", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		initialUploads := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
		var resp storage.StageProductImageResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, clientMediaID, resp.ClientMediaID)
		assert.NotEmpty(t, resp.StagedMediaID)
		assert.Equal(t, "ready", resp.Status)
		expectedKey := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodID1, resp.StagedMediaID)
		assert.Contains(t, resp.ImageURL, expectedKey)

		// Verify S3 provider
		assert.Equal(t, initialUploads+1, fakeStorage.getUploadCount())
		lastUpload, ok := fakeStorage.getLastUpload()
		require.True(t, ok)
		assert.Equal(t, expectedKey, lastUpload.ObjectKey)
		assert.Equal(t, "image/jpeg", lastUpload.ContentType)

		// Verify DB row
		var status, sha string
		var wDim, hDim int
		err = db.Pool.QueryRow(ctx, "SELECT status, content_sha256, width, height FROM product_media_staging WHERE id = $1", resp.StagedMediaID).Scan(&status, &sha, &wDim, &hDim)
		require.NoError(t, err)
		assert.Equal(t, "ready", status)
		assert.Equal(t, validImageSHAHex, sha)
		assert.Equal(t, 800, wDim)
		assert.Equal(t, 1000, hDim)
	})

	// 2. SAME clientMediaId + SAME CONTENT + READY (Case B)
	t.Run("2. same clientMediaId + same content + ready returns same row without S3 upload", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)

		// First upload
		req1, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req1 = req1.WithContext(context.WithValue(req1.Context(), "userID", userID))
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		require.Equal(t, http.StatusOK, w1.Code)
		var resp1 storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &resp1))

		uploadCountAfterFirst := fakeStorage.getUploadCount()

		// Second upload (identical retry)
		req2, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req2 = req2.WithContext(context.WithValue(req2.Context(), "userID", userID))
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusOK, w2.Code)
		var resp2 storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))

		assert.Equal(t, resp1.StagedMediaID, resp2.StagedMediaID, "must return identical stagedMediaId")
		assert.Equal(t, "ready", resp2.Status)

		// Provider MUST NOT have uploaded again
		assert.Equal(t, uploadCountAfterFirst, fakeStorage.getUploadCount(), "Case B must cause 0 additional S3 uploads")

		// DB must have exactly 1 staging row
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE client_media_id = $1", clientMediaID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	// 3. SAME clientMediaId + SAME CONTENT + UPLOADING (Case C)
	t.Run("3. same clientMediaId + same content + uploading re-uploads to deterministic key and marks ready", func(t *testing.T) {
		clientMediaID := uuid.New()
		stagedID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		expectedKey := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodID1, stagedID)

		// Seed a row in 'uploading' state directly
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO product_media_staging (
				id, seller_id, product_id, client_media_id, status,
				object_key, image_url, content_sha256, byte_size, width, height
			) VALUES ($1, $2, $3, $4, 'uploading', $5, $6, $7, $8, 800, 1000)
		`, stagedID, sellerID, prodID1, clientMediaID, expectedKey, "http://localhost:9000/media/"+expectedKey, validImageSHAHex, len(validImageBytes))
		require.NoError(t, err)

		initialUploads := fakeStorage.getUploadCount()

		// Call stage endpoint with same clientMediaId and same content
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var resp storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		assert.Equal(t, stagedID, resp.StagedMediaID)
		assert.Equal(t, "ready", resp.Status)
		assert.Equal(t, initialUploads+1, fakeStorage.getUploadCount(), "Case C must re-upload")

		// Verify row transitioned to ready in DB
		var status string
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE id = $1", stagedID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "ready", status)
	})

	// 4. SAME clientMediaId + DIFFERENT CONTENT (Case D)
	t.Run("4. same clientMediaId + different content returns 409 conflict and does not call S3", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)

		// First upload
		req1, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req1 = req1.WithContext(context.WithValue(req1.Context(), "userID", userID))
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		require.Equal(t, http.StatusOK, w1.Code)

		uploadCountBeforeConflict := fakeStorage.getUploadCount()

		// Second upload with same clientMediaID but DIFFERENT content
		req2, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", differentImageBytes)
		require.NoError(t, err)
		req2 = req2.WithContext(context.WithValue(req2.Context(), "userID", userID))
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusConflict, w2.Code, "must return 409 conflict: %s", w2.Body.String())
		assert.Contains(t, w2.Body.String(), "staged_media_conflict")
		assert.Equal(t, uploadCountBeforeConflict, fakeStorage.getUploadCount(), "conflict must not invoke S3")

		// Verify original row in DB has original SHA
		var sha string
		err = db.Pool.QueryRow(ctx, "SELECT content_sha256 FROM product_media_staging WHERE client_media_id = $1", clientMediaID).Scan(&sha)
		require.NoError(t, err)
		assert.Equal(t, validImageSHAHex, sha, "original SHA must remain unaltered")
	})

	// 5. S3 UPLOAD FAILURE
	t.Run("5. S3 upload failure leaves row uploading and does not mark ready", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)

		fakeStorage.setFailUpload(true, errors.New("s3 connection timeout"))
		defer fakeStorage.setFailUpload(false, nil)

		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Verify row in DB exists and status is 'uploading'
		var status string
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE client_media_id = $1", clientMediaID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "uploading", status, "row must remain in uploading state on S3 failure")
	})

	// 6. RETRY AFTER PREVIOUS S3 FAILURE
	t.Run("6. retry after S3 failure recovers via Case C and becomes ready", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)

		// 1. Fail first attempt
		fakeStorage.setFailUpload(true, errors.New("s3 network glitch"))
		req1, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req1 = req1.WithContext(context.WithValue(req1.Context(), "userID", userID))
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		require.Equal(t, http.StatusInternalServerError, w1.Code)

		// 2. Clear failure and retry
		fakeStorage.setFailUpload(false, nil)
		req2, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req2 = req2.WithContext(context.WithValue(req2.Context(), "userID", userID))
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		require.Equal(t, http.StatusOK, w2.Code, "retry must succeed: %s", w2.Body.String())
		var resp storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
		assert.Equal(t, "ready", resp.Status)

		var status string
		err = db.Pool.QueryRow(ctx, "SELECT status FROM product_media_staging WHERE client_media_id = $1", clientMediaID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "ready", status)
	})

	// 7. MARK READY IDEMPOTENCY
	t.Run("7. mark ready is idempotent when row is already ready", func(t *testing.T) {
		clientMediaID := uuid.New()
		stagedID := uuid.New()
		expectedKey := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodID1, stagedID)

		_, err := db.Pool.Exec(ctx, `
			INSERT INTO product_media_staging (
				id, seller_id, product_id, client_media_id, status,
				object_key, image_url, content_sha256, byte_size, width, height
			) VALUES ($1, $2, $3, $4, 'ready', $5, $6, $7, $8, 800, 1000)
		`, stagedID, sellerID, prodID1, clientMediaID, expectedKey, "http://localhost:9000/media/"+expectedKey, validImageSHAHex, len(validImageBytes))
		require.NoError(t, err)

		// Mark ready on already-ready row must succeed without error
		err = productsRepo.MarkStagedMediaReadyForSellerProduct(ctx, stagedID, sellerID, prodID1)
		require.NoError(t, err)
	})

	// 8. WRONG SELLER (Information Leak Guard)
	t.Run("8. wrong seller is rejected with 404 not_found indistinguishable from nonexistent product", func(t *testing.T) {
		clientMediaID := uuid.New()
		// Product 1 belongs to Seller 1, but request sent with userID2 (Seller 2)
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID2))

		uploadCountBefore := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code, "foreign product must return 404 to avoid info leak")
		assert.Contains(t, w.Body.String(), "not_found")
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount(), "must not upload to S3 on foreign product")

		// Verify no staging row was created
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE client_media_id = $1", clientMediaID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	// 9. NONEXISTENT PRODUCT
	t.Run("9. nonexistent product is rejected before S3 upload with 404 not_found", func(t *testing.T) {
		clientMediaID := uuid.New()
		nonExistentProdID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", nonExistentProdID)
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		uploadCountBefore := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "not_found")
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount(), "must not upload to S3 on nonexistent product")
	})

	// 10. NON-EDITABLE PRODUCT
	t.Run("10. non-editable product is rejected before S3 upload", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", blockedProdID)
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		uploadCountBefore := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount(), "must not upload to S3 on non-editable product")
	})

	// 11. MALFORMED CLIENT MEDIA ID
	t.Run("11. malformed clientMediaId returns 400", func(t *testing.T) {
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		req, err := buildStageMultipartRequest(url, "not-a-valid-uuid", "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		uploadCountBefore := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_client_media_id")
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount())
	})

	// 12. UNSUPPORTED IMAGE TYPE
	t.Run("12. unsupported image type returns 400", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "note.txt", "text/plain", []byte("some text content"))
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		uploadCountBefore := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_file_type")
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount())
	})

	// 13. INVALID DIMENSIONS
	t.Run("13a. landscape image is rejected with portrait required error", func(t *testing.T) {
		landscapeBytes := createTestJPEG(1000, 800, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "landscape.jpg", "image/jpeg", landscapeBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		uploadCountBefore := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "product_media_portrait_required")
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount())
	})

	t.Run("13b. small image is rejected with too small error", func(t *testing.T) {
		smallBytes := createTestJPEG(400, 500, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "small.jpg", "image/jpeg", smallBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		uploadCountBefore := fakeStorage.getUploadCount()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "product_media_too_small")
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount())
	})

	// 14. CANONICAL NON-MUTATION PROOF
	t.Run("14. canonical product media is completely untouched throughout staging", func(t *testing.T) {
		// Capture before
		var imageCountBefore int
		var mainURLBefore *string
		err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE product_id = $1", prodID1).Scan(&imageCountBefore)
		require.NoError(t, err)
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url FROM products WHERE id = $1", prodID1).Scan(&mainURLBefore)
		require.NoError(t, err)

		// Perform multiple stages and retries
		for i := 0; i < 3; i++ {
			cID := uuid.New()
			url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
			req, err := buildStageMultipartRequest(url, cID.String(), "photo.jpg", "image/jpeg", validImageBytes)
			require.NoError(t, err)
			req = req.WithContext(context.WithValue(req.Context(), "userID", userID))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
		}

		// Capture after
		var imageCountAfter int
		var mainURLAfter *string
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE product_id = $1", prodID1).Scan(&imageCountAfter)
		require.NoError(t, err)
		err = db.Pool.QueryRow(ctx, "SELECT main_image_url FROM products WHERE id = $1", prodID1).Scan(&mainURLAfter)
		require.NoError(t, err)

		assert.Equal(t, imageCountBefore, imageCountAfter, "product_images count MUST NOT change")
		assert.Equal(t, mainURLBefore, mainURLAfter, "products.main_image_url MUST NOT change")
	})

	// 15. CONSUMED STAGED IDENTITY (Case E)
	t.Run("15. consumed staged identity returns consumed without re-uploading", func(t *testing.T) {
		clientMediaID := uuid.New()
		stagedID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		expectedKey := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodID1, stagedID)

		// Seed a consumed staging row directly
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO product_media_staging (
				id, seller_id, product_id, client_media_id, status,
				object_key, image_url, content_sha256, byte_size, width, height, consumed_at
			) VALUES ($1, $2, $3, $4, 'consumed', $5, $6, $7, $8, 800, 1000, NOW())
		`, stagedID, sellerID, prodID1, clientMediaID, expectedKey, "http://localhost:9000/media/"+expectedKey, validImageSHAHex, len(validImageBytes))
		require.NoError(t, err)

		uploadCountBefore := fakeStorage.getUploadCount()

		// Call stage endpoint
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var resp storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		assert.Equal(t, stagedID, resp.StagedMediaID)
		assert.Equal(t, "consumed", resp.Status)
		assert.Equal(t, uploadCountBefore, fakeStorage.getUploadCount(), "consumed state must not upload to S3")
	})

	// 16. JPEG BYTES WITH MISLEADING PNG HEADER
	t.Run("16. JPEG bytes with misleading PNG header normalizes to canonical JPEG object", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)
		// Upload valid JPEG bytes, but client claims it is photo.png with image/png
		req, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.png", "image/png", validImageBytes)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
		var resp storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		// Deterministic key MUST use .jpg, NOT .png, and be based on resp.StagedMediaID
		expectedKey := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodID1, resp.StagedMediaID)
		assert.Equal(t, "http://localhost:9000/media/"+expectedKey, resp.ImageURL)
		assert.True(t, strings.HasSuffix(resp.ImageURL, ".jpg"), "URL must have .jpg extension derived from actual bytes")

		// Provider must have received image/jpeg content type
		lastUpload, ok := fakeStorage.getLastUpload()
		require.True(t, ok)
		assert.Equal(t, expectedKey, lastUpload.ObjectKey)
		assert.Equal(t, "image/jpeg", lastUpload.ContentType, "S3 upload Content-Type must be byte-derived image/jpeg")

		// DB row must have .jpg object key
		var objKey string
		err = db.Pool.QueryRow(ctx, "SELECT object_key FROM product_media_staging WHERE id = $1", resp.StagedMediaID).Scan(&objKey)
		require.NoError(t, err)
		assert.Equal(t, expectedKey, objKey)
	})

	// 17. IDENTICAL BYTES RETRY WITH CHANGED CLIENT CONTENT-TYPE
	t.Run("17. identical bytes retry with changed Content-Type produces consistent key and no duplicate upload", func(t *testing.T) {
		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodID1)

		// First upload: sends with image/jpeg
		req1, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req1 = req1.WithContext(context.WithValue(req1.Context(), "userID", userID))
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		require.Equal(t, http.StatusOK, w1.Code)
		var resp1 storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &resp1))

		uploadCountAfterFirst := fakeStorage.getUploadCount()

		// Second upload: sends same clientMediaID and same bytes with misleading image/png
		req2, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.png", "image/png", validImageBytes)
		require.NoError(t, err)
		req2 = req2.WithContext(context.WithValue(req2.Context(), "userID", userID))
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusOK, w2.Code)
		var resp2 storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))

		assert.Equal(t, resp1.StagedMediaID, resp2.StagedMediaID)
		assert.Equal(t, resp1.ImageURL, resp2.ImageURL)
		assert.Equal(t, uploadCountAfterFirst, fakeStorage.getUploadCount(), "retry must not cause additional upload")
	})

	// 18. STAGED MEDIA RESOURCE CAP
	t.Run("18. active staging quota limits uploading+ready items to MaxActiveStagedMediaPerProduct", func(t *testing.T) {
		prodQuotaID := uuid.New()
		createProduct(prodQuotaID, sellerID, "draft")

		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodQuotaID)
		var stagedClientIDs []uuid.UUID

		// Stage up to MaxActiveStagedMediaPerProduct (16)
		for i := 0; i < products.MaxActiveStagedMediaPerProduct; i++ {
			cID := uuid.New()
			stagedClientIDs = append(stagedClientIDs, cID)
			// Vary color slightly so each image has unique SHA
			b := createTestJPEG(800, 1000, color.RGBA{R: uint8(10 + i*5), G: 100, B: 50, A: 255})
			req, err := buildStageMultipartRequest(url, cID.String(), "photo.jpg", "image/jpeg", b)
			require.NoError(t, err)
			req = req.WithContext(context.WithValue(req.Context(), "userID", userID))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, "item %d should succeed", i+1)
		}

		// Verify count in DB is 16
		count, err := productsRepo.CountActiveStagedMediaForSellerProduct(ctx, sellerID, prodQuotaID)
		require.NoError(t, err)
		require.Equal(t, products.MaxActiveStagedMediaPerProduct, count)

		uploadCountAtCap := fakeStorage.getUploadCount()

		// Attempt 17th with NEW clientMediaID -> MUST BE REJECTED
		cID17 := uuid.New()
		b17 := createTestJPEG(800, 1000, color.RGBA{R: 200, G: 200, B: 50, A: 255})
		req17, err := buildStageMultipartRequest(url, cID17.String(), "photo.jpg", "image/jpeg", b17)
		require.NoError(t, err)
		req17 = req17.WithContext(context.WithValue(req17.Context(), "userID", userID))
		w17 := httptest.NewRecorder()
		r.ServeHTTP(w17, req17)

		assert.Equal(t, http.StatusBadRequest, w17.Code, "17th upload must be rejected with 400")
		assert.Contains(t, w17.Body.String(), "staged_media_quota_exceeded")
		assert.Equal(t, uploadCountAtCap, fakeStorage.getUploadCount(), "quota rejection must happen BEFORE S3 upload")

		// 19. IDEMPOTENT RETRY AT QUOTA IS ALLOWED
		firstCID := stagedClientIDs[0]
		bFirst := createTestJPEG(800, 1000, color.RGBA{R: 10, G: 100, B: 50, A: 255})
		reqRetry, err := buildStageMultipartRequest(url, firstCID.String(), "photo.jpg", "image/jpeg", bFirst)
		require.NoError(t, err)
		reqRetry = reqRetry.WithContext(context.WithValue(reqRetry.Context(), "userID", userID))
		wRetry := httptest.NewRecorder()
		r.ServeHTTP(wRetry, reqRetry)

		assert.Equal(t, http.StatusOK, wRetry.Code, "retry of existing clientMediaId at quota must succeed")
		assert.Equal(t, uploadCountAtCap, fakeStorage.getUploadCount(), "retry must not cause S3 upload")

		// 20. CONSUMED ROWS EXCLUDED FROM ACTIVE QUOTA
		// Mark one row as consumed
		_, err = db.Pool.Exec(ctx, "UPDATE product_media_staging SET status = 'consumed', consumed_at = NOW() WHERE client_media_id = $1", firstCID)
		require.NoError(t, err)

		// Active count is now 15
		countAfterConsumed, err := productsRepo.CountActiveStagedMediaForSellerProduct(ctx, sellerID, prodQuotaID)
		require.NoError(t, err)
		require.Equal(t, products.MaxActiveStagedMediaPerProduct-1, countAfterConsumed)

		// Now 17th item should succeed because active count < 16
		req17Allowed, err := buildStageMultipartRequest(url, cID17.String(), "photo.jpg", "image/jpeg", b17)
		require.NoError(t, err)
		req17Allowed = req17Allowed.WithContext(context.WithValue(req17Allowed.Context(), "userID", userID))
		w17Allowed := httptest.NewRecorder()
		r.ServeHTTP(w17Allowed, req17Allowed)

		assert.Equal(t, http.StatusOK, w17Allowed.Code, "17th upload must succeed after a row is consumed: %s", w17Allowed.Body.String())
	})

	// 21. CONCURRENT QUOTA TEST (Section 6)
	t.Run("21. concurrent quota race: exactly one claim succeeds, active count is 16, never 17", func(t *testing.T) {
		prodConcurrentID := uuid.New()
		createProduct(prodConcurrentID, sellerID, "draft")

		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodConcurrentID)

		// Pre-populate 15 active rows
		for i := 0; i < products.MaxActiveStagedMediaPerProduct-1; i++ {
			cID := uuid.New()
			b := createTestJPEG(800, 1000, color.RGBA{R: uint8(20 + i*4), G: 120, B: 60, A: 255})
			req, err := buildStageMultipartRequest(url, cID.String(), "photo.jpg", "image/jpeg", b)
			require.NoError(t, err)
			req = req.WithContext(context.WithValue(req.Context(), "userID", userID))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
		}

		// Verify count is 15
		count, err := productsRepo.CountActiveStagedMediaForSellerProduct(ctx, sellerID, prodConcurrentID)
		require.NoError(t, err)
		require.Equal(t, 15, count)

		uploadCountBeforeRace := fakeStorage.getUploadCount()

		// Prepare 2 concurrent requests with distinct clientMediaIDs
		cID1 := uuid.New()
		b1 := createTestJPEG(800, 1000, color.RGBA{R: 210, G: 120, B: 60, A: 255})
		req1, err := buildStageMultipartRequest(url, cID1.String(), "photo1.jpg", "image/jpeg", b1)
		require.NoError(t, err)
		req1 = req1.WithContext(context.WithValue(req1.Context(), "userID", userID))

		cID2 := uuid.New()
		b2 := createTestJPEG(800, 1000, color.RGBA{R: 220, G: 120, B: 60, A: 255})
		req2, err := buildStageMultipartRequest(url, cID2.String(), "photo2.jpg", "image/jpeg", b2)
		require.NoError(t, err)
		req2 = req2.WithContext(context.WithValue(req2.Context(), "userID", userID))

		startChan := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		var code1, code2 int
		var body1, body2 string

		go func() {
			defer wg.Done()
			<-startChan
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req1)
			code1 = w.Code
			body1 = w.Body.String()
		}()

		go func() {
			defer wg.Done()
			<-startChan
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req2)
			code2 = w.Code
			body2 = w.Body.String()
		}()

		close(startChan) // release barrier simultaneously
		wg.Wait()

		// Invariant: Exactly one 200 and one 400
		if code1 == http.StatusOK {
			assert.Equal(t, http.StatusBadRequest, code2, "second request must fail with 400: %s", body2)
			assert.Contains(t, body2, "staged_media_quota_exceeded")
		} else {
			assert.Equal(t, http.StatusBadRequest, code1, "first request must fail with 400: %s", body1)
			assert.Contains(t, body1, "staged_media_quota_exceeded")
			assert.Equal(t, http.StatusOK, code2, "second request must succeed with 200: %s", body2)
		}

		// Invariant: Active count == 16, NEVER 17
		finalCount, err := productsRepo.CountActiveStagedMediaForSellerProduct(ctx, sellerID, prodConcurrentID)
		require.NoError(t, err)
		assert.Equal(t, 16, finalCount, "active count must be exactly 16")

		// S3 upload count increased by exactly 1 (losing request never hit S3)
		assert.Equal(t, uploadCountBeforeRace+1, fakeStorage.getUploadCount(), "only the winning claim must perform S3 upload")

		// 22. IDEMPOTENT RETRY AT CAP UNDER LOCK (Section 7)
		// Request with existing clientMediaID (the one that won) + same SHA succeeds at cap 16
		var winningCID uuid.UUID
		var winningBytes []byte
		if code1 == http.StatusOK {
			winningCID = cID1
			winningBytes = b1
		} else {
			winningCID = cID2
			winningBytes = b2
		}

		reqRetry, err := buildStageMultipartRequest(url, winningCID.String(), "photo_retry.jpg", "image/jpeg", winningBytes)
		require.NoError(t, err)
		reqRetry = reqRetry.WithContext(context.WithValue(reqRetry.Context(), "userID", userID))
		wRetry := httptest.NewRecorder()
		r.ServeHTTP(wRetry, reqRetry)

		assert.Equal(t, http.StatusOK, wRetry.Code, "existing clientMediaId retry must succeed even when product is at cap 16")
		assert.Equal(t, uploadCountBeforeRace+1, fakeStorage.getUploadCount(), "idempotent retry at cap must not perform additional S3 upload")

		// Request with NEW clientMediaID at cap 16 must fail with quota exceeded
		cIDNew := uuid.New()
		bNew := createTestJPEG(800, 1000, color.RGBA{R: 230, G: 120, B: 60, A: 255})
		reqNew, err := buildStageMultipartRequest(url, cIDNew.String(), "photo_new.jpg", "image/jpeg", bNew)
		require.NoError(t, err)
		reqNew = reqNew.WithContext(context.WithValue(reqNew.Context(), "userID", userID))
		wNew := httptest.NewRecorder()
		r.ServeHTTP(wNew, reqNew)

		assert.Equal(t, http.StatusBadRequest, wNew.Code, "new clientMediaId at cap 16 must fail")
		assert.Contains(t, wNew.Body.String(), "staged_media_quota_exceeded")
		assert.Equal(t, uploadCountBeforeRace+1, fakeStorage.getUploadCount(), "rejected new claim must not perform S3 upload")
	})

	// 23. DELETE / STAGE CLAIM RACE TEST (Section 8)
	t.Run("23. product delete vs stage claim lock serialization", func(t *testing.T) {
		// Part A: Staging claim commits before Product delete
		// -> DeleteDraftProductTx sees the staging row and enqueues its object_key into product_media_cleanup_jobs
		prodRaceA := uuid.New()
		createProduct(prodRaceA, sellerID, "draft")

		cIDA := uuid.New()
		urlA := fmt.Sprintf("/api/seller/products/%s/images/stage", prodRaceA)
		reqA, err := buildStageMultipartRequest(urlA, cIDA.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		reqA = reqA.WithContext(context.WithValue(reqA.Context(), "userID", userID))
		wA := httptest.NewRecorder()
		r.ServeHTTP(wA, reqA)
		require.Equal(t, http.StatusOK, wA.Code)

		var respA storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(wA.Body.Bytes(), &respA))
		expectedKeyA := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodRaceA, respA.StagedMediaID)

		// Now execute DeleteDraftProductTx
		err = db.RunInTx(ctx, func(tx pgx.Tx) error {
			txRepo := productsRepo.WithTx(tx)
			return txRepo.DeleteDraftProductTx(ctx, prodRaceA, sellerID)
		})
		require.NoError(t, err)

		// Verify that DeleteDraftProductTx collected the staged key and enqueued it
		var cleanupCountA int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", expectedKeyA).Scan(&cleanupCountA)
		require.NoError(t, err)
		assert.Equal(t, 1, cleanupCountA, "DeleteDraftProductTx must enqueue staged media object key for cleanup")

		// Part B: Product delete holds lock -> Stage claim is blocked, then finds Product deleted and aborts before S3
		prodRaceB := uuid.New()
		createProduct(prodRaceB, sellerID, "draft")

		// Begin transaction on a dedicated connection and acquire lock on prodRaceB
		txB, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = txB.Rollback(ctx)
		}()

		var statusB string
		err = txB.QueryRow(ctx, "SELECT status FROM products WHERE id = $1 AND seller_id = $2 FOR UPDATE", prodRaceB, sellerID).Scan(&statusB)
		require.NoError(t, err)

		// Product is now locked by txB!
		// Spawn stage request in goroutine. It will block in ClaimStagedMediaSlotForSellerProduct on the product lock.
		cidB := uuid.New()
		urlB := fmt.Sprintf("/api/seller/products/%s/images/stage", prodRaceB)
		reqB, err := buildStageMultipartRequest(urlB, cidB.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		reqB = reqB.WithContext(context.WithValue(reqB.Context(), "userID", userID))

		uploadCountBeforePartB := fakeStorage.getUploadCount()
		stageDone := make(chan struct{})
		var stageCodeB int
		var stageBodyB string

		go func() {
			defer close(stageDone)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, reqB)
			stageCodeB = w.Code
			stageBodyB = w.Body.String()
		}()

		// Give goroutine a moment to block on the DB lock
		time.Sleep(50 * time.Millisecond)

		// Now txB deletes the product and commits
		_, err = txB.Exec(ctx, "DELETE FROM products WHERE id = $1 AND seller_id = $2", prodRaceB, sellerID)
		require.NoError(t, err)
		err = txB.Commit(ctx)
		require.NoError(t, err)

		// Wait for stage request to finish
		<-stageDone

		// Assert: Staging claim unblocks, finds product missing, returns 404 not_found, and NEVER called S3
		assert.Equal(t, http.StatusNotFound, stageCodeB, "stage claim after product delete must return 404: %s", stageBodyB)
		assert.Contains(t, stageBodyB, "not_found")
		assert.Equal(t, uploadCountBeforePartB, fakeStorage.getUploadCount(), "stage claim on deleted product must NOT touch S3")
	})

	// 24. POST-UPLOAD ROW-MISSING TEST (Section 9)
	t.Run("24. S3 upload succeeds but staging row disappears before mark-ready -> enqueues cleanup job and fails request", func(t *testing.T) {
		prodMissingID := uuid.New()
		createProduct(prodMissingID, sellerID, "draft")

		cIDMissing := uuid.New()
		urlMissing := fmt.Sprintf("/api/seller/products/%s/images/stage", prodMissingID)

		var capturedKeyMissing string
		// Set hook in fakeStorage: right after upload succeeds, delete the staging row before MarkStagedMediaReady runs
		fakeStorage.setOnAfterUpload(func(objectKey string) {
			capturedKeyMissing = objectKey
			_, delErr := db.Pool.Exec(context.Background(), "DELETE FROM product_media_staging WHERE object_key = $1", objectKey)
			require.NoError(t, delErr)
		})
		defer fakeStorage.setOnAfterUpload(nil)

		reqMissing, err := buildStageMultipartRequest(urlMissing, cIDMissing.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		reqMissing = reqMissing.WithContext(context.WithValue(reqMissing.Context(), "userID", userID))

		wMissing := httptest.NewRecorder()
		r.ServeHTTP(wMissing, reqMissing)

		// 1. Request must return failure
		assert.Equal(t, http.StatusInternalServerError, wMissing.Code, "request must fail when staging row disappeared: %s", wMissing.Body.String())
		assert.NotEmpty(t, capturedKeyMissing)

		// 2. product_media_cleanup_jobs MUST contain exactly one job for this deterministic key
		var cleanupAfter int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", capturedKeyMissing).Scan(&cleanupAfter)
		require.NoError(t, err)
		assert.Equal(t, 1, cleanupAfter, "disappeared row after upload must be durably enqueued into product_media_cleanup_jobs")

		// 3. Canonical media remains unchanged
		var imgCount int
		var mainURL *string
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_images WHERE product_id = $1", prodMissingID).Scan(&imgCount)
		require.NoError(t, err)
		assert.Equal(t, 0, imgCount, "canonical product_images must remain 0")

		err = db.Pool.QueryRow(ctx, "SELECT main_image_url FROM products WHERE id = $1", prodMissingID).Scan(&mainURL)
		require.NoError(t, err)
		assert.Nil(t, mainURL, "products.main_image_url must remain nil")
	})

	// 25. CONSUMED TOMBSTONE REMOVAL SAFETY (Section 4)
	t.Run("25. consumed tombstone removal safety: reusing clientMediaId creates new generation with new objectKey", func(t *testing.T) {
		prodTombstoneID := uuid.New()
		createProduct(prodTombstoneID, sellerID, "draft")

		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodTombstoneID)

		// 1. First stage using clientMediaId X
		req1, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req1 = req1.WithContext(context.WithValue(req1.Context(), "userID", userID))
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		require.Equal(t, http.StatusOK, w1.Code)

		var resp1 storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &resp1))
		oldStagedMediaID := resp1.StagedMediaID
		oldObjectKey := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodTombstoneID, oldStagedMediaID)
		assert.Equal(t, "http://localhost:9000/media/"+oldObjectKey, resp1.ImageURL)

		// 2. Simulate it becoming consumed into canonical media
		err = productsRepo.MarkStagedMediaConsumedForSellerProduct(ctx, oldStagedMediaID, sellerID, prodTombstoneID)
		require.NoError(t, err)

		// 3. Simulate future consumed metadata TTL cleanup: delete only the product_media_staging row
		_, err = db.Pool.Exec(ctx, "DELETE FROM product_media_staging WHERE id = $1", oldStagedMediaID)
		require.NoError(t, err)

		// 4. Call stage again with the SAME clientMediaId X
		req2, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		req2 = req2.WithContext(context.WithValue(req2.Context(), "userID", userID))
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusOK, w2.Code)

		var resp2 storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))

		// 5. Invariant asserts:
		// - new stagedMediaId != oldStagedMediaId
		// - new objectKey != oldObjectKey
		// - old object key is never overwritten
		// - new stage succeeds as a NEW generation
		assert.NotEqual(t, oldStagedMediaID, resp2.StagedMediaID, "new generation must receive distinct stagedMediaId")
		newObjectKey := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodTombstoneID, resp2.StagedMediaID)
		assert.NotEqual(t, oldObjectKey, newObjectKey, "new generation must receive distinct objectKey")
		assert.Equal(t, "http://localhost:9000/media/"+newObjectKey, resp2.ImageURL)
		assert.Equal(t, "ready", resp2.Status)

		// Verify DB contains only the new generation
		var count int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_staging WHERE id = $1", resp2.StagedMediaID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	// 26. OLD CLEANUP JOB MUST NOT TARGET NEW GENERATION (Section 5)
	t.Run("26. old cleanup job never targets new generation with same clientMediaId", func(t *testing.T) {
		prodCleanupIsoID := uuid.New()
		createProduct(prodCleanupIsoID, sellerID, "draft")

		clientMediaID := uuid.New()
		url := fmt.Sprintf("/api/seller/products/%s/images/stage", prodCleanupIsoID)

		// 1. Stage generation A using clientMediaId
		reqA, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		reqA = reqA.WithContext(context.WithValue(reqA.Context(), "userID", userID))
		wA := httptest.NewRecorder()
		r.ServeHTTP(wA, reqA)
		require.Equal(t, http.StatusOK, wA.Code)

		var respA storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(wA.Body.Bytes(), &respA))
		objectKeyA := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodCleanupIsoID, respA.StagedMediaID)

		// 2. Enqueue cleanup job for A (e.g. from deletion or abort)
		err = productsRepo.EnqueueMediaCleanup(ctx, objectKeyA)
		require.NoError(t, err)

		// 3. Remove old staging metadata / simulate old generation lifecycle completion
		_, err = db.Pool.Exec(ctx, "DELETE FROM product_media_staging WHERE id = $1", respA.StagedMediaID)
		require.NoError(t, err)

		// 4. Stage SAME clientMediaId again -> generation B
		reqB, err := buildStageMultipartRequest(url, clientMediaID.String(), "photo.jpg", "image/jpeg", validImageBytes)
		require.NoError(t, err)
		reqB = reqB.WithContext(context.WithValue(reqB.Context(), "userID", userID))
		wB := httptest.NewRecorder()
		r.ServeHTTP(wB, reqB)
		require.Equal(t, http.StatusOK, wB.Code)

		var respB storage.StageProductImageResponse
		require.NoError(t, json.Unmarshal(wB.Body.Bytes(), &respB))
		objectKeyB := fmt.Sprintf("products/%s/%s/staged/%s.jpg", sellerID, prodCleanupIsoID, respB.StagedMediaID)

		// 5. Assert:
		// A != B
		// and existing cleanup job still references only A
		assert.NotEqual(t, objectKeyA, objectKeyB, "generation A and B must have different object keys")
		assert.NotEqual(t, respA.StagedMediaID, respB.StagedMediaID, "generation A and B must have different staged IDs")

		var countA, countB int
		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", objectKeyA).Scan(&countA)
		require.NoError(t, err)
		assert.Equal(t, 1, countA, "cleanup job must exist for old generation A")

		err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_media_cleanup_jobs WHERE object_key = $1", objectKeyB).Scan(&countB)
		require.NoError(t, err)
		assert.Equal(t, 0, countB, "no cleanup job must exist for new generation B")
	})
}
