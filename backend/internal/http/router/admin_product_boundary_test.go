package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestAdminProductOwnershipBoundary(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	defer pgClient.Close()

	// Strict DB Safety Guard
	testutil.AssertTestDatabase(t, pgClient.Pool)

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
		S3: config.S3Config{
			Endpoint:        "localhost",
			Port:            "9000",
			Bucket:          "zamk-products",
			AccessKey:       "zamk_minio",
			SecretKey:       "zamk_minio_password",
			UseSSL:          false,
			UploadMaxSizeMB: 10,
		},
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	// Fixtures
	sellerUserID := uuid.New()
	adminUserID := uuid.New()
	staffRoleID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()
	otherCatID := uuid.New()
	draftProdID := uuid.New()
	draftVarID := uuid.New()
	pubProdID := uuid.New()
	pubVarID := uuid.New()
	pendingProdID := uuid.New()
	pendingVarID := uuid.New()
	pubImageID := uuid.New()
	draftBarcode := fmt.Sprintf("98%011d", (time.Now().UnixNano()%100000000000)+10000000000)
	pubBarcode := fmt.Sprintf("98%011d", (time.Now().UnixNano()%100000000000)+20000000000)
	pendingBarcode := fmt.Sprintf("98%011d", (time.Now().UnixNano()%100000000000)+30000000000)
	pubImageURL := "https://cdn.zamk.local/products/legit_seller_photo.jpg"

	cleanup := func() {
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = ANY($1) OR id = $2", []uuid.UUID{draftProdID, pubProdID, pendingProdID}, pubImageID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_moderation_logs WHERE product_id = ANY($1)", []uuid.UUID{draftProdID, pubProdID, pendingProdID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = ANY($1) OR barcode = ANY($2)", []uuid.UUID{draftProdID, pubProdID, pendingProdID}, []string{draftBarcode, pubBarcode, pendingBarcode})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1) OR seller_id = $2", []uuid.UUID{draftProdID, pubProdID, pendingProdID}, sellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", []uuid.UUID{catID, otherCatID})
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM seller_users WHERE seller_id = $1", sellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = $1", adminUserID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = $1", staffRoleID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = $1", staffRoleID)
		_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{sellerUserID, adminUserID})
	}

	cleanup()
	t.Cleanup(func() {
		cleanup()
	})

	// 1. Insert Users
	insertUser := func(id uuid.UUID, email, role string) {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
			VALUES ($1, 'Test User', $2, 'hash', $3, 'active', false, now(), now())
		`, id, email, role)
		require.NoError(t, err)
	}

	sellerEmail := fmt.Sprintf("seller_bnd_%s@zamk.local", sellerUserID.String()[:8])
	adminEmail := fmt.Sprintf("admin_bnd_%s@zamk.local", adminUserID.String()[:8])

	insertUser(sellerUserID, sellerEmail, "seller")
	insertUser(adminUserID, adminEmail, "admin")

	// 2. Insert Staff Role with full permissions for Admin
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'SuperAdmin')`, staffRoleID, "super_admin_"+staffRoleID.String()[:8])
	require.NoError(t, err)

	for _, perm := range []string{"*", "products.read", "products.moderate", "products.hide", "products.approve", "products.reject", "products.publish", "products.block"} {
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, staffRoleID, perm)
		require.NoError(t, err)
	}

	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, adminUserID, staffRoleID)
	require.NoError(t, err)

	// 3. Insert Seller and link to seller user
	sellerSlug := fmt.Sprintf("seller-bnd-%s", sellerID.String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, 'Boundary Seller', $2, 'desc', $3, '123', 'active', now(), now())
	`, sellerID, sellerSlug, sellerEmail)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, sellerUserID)
	require.NoError(t, err)

	// 4. Categories & Products
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat 1', $2, now(), now())`, catID, "cat-1-"+catID.String()[:8])
	require.NoError(t, err)
	_, err = pgClient.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat 2', $2, now(), now())`, otherCatID, "cat-2-"+otherCatID.String()[:8])
	require.NoError(t, err)

	initialDraftTitle := "Draft Commercial Title"
	initialDraftDesc := "Draft Commercial Description"
	draftProdSlug := fmt.Sprintf("prod-draft-%s", draftProdID.String()[:8])

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, description, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 5000, 'draft', now(), now())
	`, draftProdID, sellerID, catID, initialDraftTitle, initialDraftDesc, draftProdSlug)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 5000, true, now(), now())
	`, draftVarID, draftProdID, "SKU-"+draftVarID.String()[:8], "DRAFT-SKU", draftBarcode)
	require.NoError(t, err)

	// Published product for moderation tests
	pubTitle := "Published Commercial Title"
	pubProdSlug := fmt.Sprintf("prod-pub-%s", pubProdID.String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, description, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Pub Desc', $5, 7500, 'published', now(), now())
	`, pubProdID, sellerID, catID, pubTitle, pubProdSlug)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 7500, true, now(), now())
	`, pubVarID, pubProdID, "SKU-"+pubVarID.String()[:8], "PUB-SKU", pubBarcode)
	require.NoError(t, err)

	// Pending product for moderation reject test
	pendingTitle := "Pending Moderation Title"
	pendingProdSlug := fmt.Sprintf("prod-pending-%s", pendingProdID.String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, description, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Pending Desc', $5, 6000, 'pending_moderation', now(), now())
	`, pendingProdID, sellerID, catID, pendingTitle, pendingProdSlug)
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 6000, true, now(), now())
	`, pendingVarID, pendingProdID, "SKU-"+pendingVarID.String()[:8], "PEND-SKU", pendingBarcode)
	require.NoError(t, err)

	// Legitimate Seller Image on published product
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, object_key, alt_text, sort_order, is_main, created_at)
		VALUES ($1, $2, $3, 'products/legit_seller_photo.jpg', 'Main Photo', 0, true, now())
	`, pubImageID, pubProdID, pubImageURL)
	require.NoError(t, err)

	// Tokens
	sellerToken, err := tokenService.GenerateAccessToken(sellerUserID, sellerEmail, "seller")
	require.NoError(t, err)
	adminToken, err := tokenService.GenerateAccessToken(adminUserID, adminEmail, "admin")
	require.NoError(t, err)

	execReq := func(method, url, token string, body []byte) *httptest.ResponseRecorder {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req := httptest.NewRequest(method, url, bodyReader)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	execMultipartReq := func(method, url, token, fieldName, fileName string, fileContent []byte) *httptest.ResponseRecorder {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile(fieldName, fileName)
		require.NoError(t, err)
		_, err = part.Write(fileContent)
		require.NoError(t, err)
		err = writer.Close()
		require.NoError(t, err)

		req := httptest.NewRequest(method, url, body)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	// =========================================================================
	// Test 1: Admin attempts to mutate title/description/category via /api/admin/products/{id}
	// Contract: Endpoint is UNAVAILABLE / REJECTED (405 Method Not Allowed)
	// Database must remain completely unchanged.
	// =========================================================================
	t.Run("AdminCannotMutateCommercialFieldsViaAdminEndpoint", func(t *testing.T) {
		payload := map[string]any{
			"title":       "Hijacked by Admin",
			"description": "Admin replaced seller content",
			"categoryId":  otherCatID.String(),
			"priceCents":  99999,
		}
		body, _ := json.Marshal(payload)

		adminURL := fmt.Sprintf("/api/admin/products/%s", draftProdID.String())
		res := execReq(http.MethodPatch, adminURL, adminToken, body)

		// Chi router returns 405 Method Not Allowed because PATCH on /products/{id} was removed
		assert.Equal(t, http.StatusMethodNotAllowed, res.Code, "PATCH /api/admin/products/{id} must be unavailable/rejected")

		// Verify database state is untouched
		var dbTitle, dbDesc string
		var dbCatID uuid.UUID
		var dbPrice int64
		err = pgClient.Pool.QueryRow(ctx, "SELECT title, description, category_id, price_cents FROM products WHERE id = $1", draftProdID).Scan(&dbTitle, &dbDesc, &dbCatID, &dbPrice)
		require.NoError(t, err)
		assert.Equal(t, initialDraftTitle, dbTitle, "title must NOT have changed")
		assert.Equal(t, initialDraftDesc, dbDesc, "description must NOT have changed")
		assert.Equal(t, catID, dbCatID, "category must NOT have changed")
		assert.Equal(t, int64(5000), dbPrice, "price must NOT have changed")
	})

	// =========================================================================
	// Test 2: Admin attempts to call seller's update endpoint /api/seller/products/{id}
	// Contract: Admin is rejected (401/403 forbidden) because Admin is not a seller owner.
	// =========================================================================
	t.Run("AdminCannotMutateViaSellerEndpoint", func(t *testing.T) {
		payload := map[string]any{
			"title": "Admin Bypassing Via Seller API",
		}
		body, _ := json.Marshal(payload)

		sellerURL := fmt.Sprintf("/api/seller/products/%s", draftProdID.String())
		res := execReq(http.MethodPatch, sellerURL, adminToken, body)

		assert.Equal(t, http.StatusForbidden, res.Code, "Admin must not have seller access")

		// Verify database remains untouched
		var dbTitle string
		err = pgClient.Pool.QueryRow(ctx, "SELECT title FROM products WHERE id = $1", draftProdID).Scan(&dbTitle)
		require.NoError(t, err)
		assert.Equal(t, initialDraftTitle, dbTitle)
	})

	// =========================================================================
	// Test 3: Seller legitimate product edit flow still works
	// =========================================================================
	t.Run("SellerLegitimateProductEditFlowSucceeds", func(t *testing.T) {
		newTitle := "Legitimate Seller Updated Title"
		newDesc := "Legitimate Seller Updated Description"
		payload := map[string]any{
			"title":       newTitle,
			"description": newDesc,
		}
		body, _ := json.Marshal(payload)

		sellerURL := fmt.Sprintf("/api/seller/products/%s", draftProdID.String())
		res := execReq(http.MethodPatch, sellerURL, sellerToken, body)

		assert.Equal(t, http.StatusOK, res.Code, "Seller must be able to update own product")

		// Verify database reflects seller's update
		var dbTitle, dbDesc string
		err = pgClient.Pool.QueryRow(ctx, "SELECT title, description FROM products WHERE id = $1", draftProdID).Scan(&dbTitle, &dbDesc)
		require.NoError(t, err)
		assert.Equal(t, newTitle, dbTitle)
		assert.Equal(t, newDesc, dbDesc)
	})

	// =========================================================================
	// Test 4: Admin legitimate inspection flow (GET /api/admin/products/{id})
	// =========================================================================
	t.Run("AdminLegitimateInspectionSucceeds", func(t *testing.T) {
		adminURL := fmt.Sprintf("/api/admin/products/%s", draftProdID.String())
		res := execReq(http.MethodGet, adminURL, adminToken, nil)

		assert.Equal(t, http.StatusOK, res.Code, "Admin must be able to inspect product")
		assert.Contains(t, res.Body.String(), "Legitimate Seller Updated Title")
	})

	// =========================================================================
	// Test 5: Admin legitimate moderation flow (POST /api/admin/moderation/products/{id}/hide)
	// =========================================================================
	t.Run("AdminLegitimateModerationSucceeds", func(t *testing.T) {
		hideURL := fmt.Sprintf("/api/admin/moderation/products/%s/hide", pubProdID.String())
		payload := map[string]any{
			"comment": "Скрыт по жалобе правообладателя",
		}
		body, _ := json.Marshal(payload)

		res := execReq(http.MethodPost, hideURL, adminToken, body)
		assert.Equal(t, http.StatusOK, res.Code, "Admin must be able to hide product")

		// Verify database reflects hidden status
		var dbStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", pubProdID).Scan(&dbStatus)
		require.NoError(t, err)
		assert.Equal(t, "hidden", dbStatus)
	})

	// =========================================================================
	// Test 6: Admin attempts to upload image via /api/admin/products/{id}/images/upload
	// Contract: Route is removed/unavailable (404/405). Zero product_images rows created.
	// =========================================================================
	t.Run("AdminCannotUploadImageViaAdminEndpoint", func(t *testing.T) {
		adminUploadURL := fmt.Sprintf("/api/admin/products/%s/images/upload", draftProdID.String())
		dummyFile := []byte("fake image payload")
		res := execMultipartReq(http.MethodPost, adminUploadURL, adminToken, "image", "photo.png", dummyFile)

		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, res.Code, "Admin image upload endpoint must not exist")

		// Verify zero images exist in DB for draftProdID
		var imgCount int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM product_images WHERE product_id = $1", draftProdID).Scan(&imgCount)
		require.NoError(t, err)
		assert.Equal(t, 0, imgCount, "no images must be created in DB")
	})

	// =========================================================================
	// Test 7: Admin attempts to upload image via seller's endpoint /api/seller/products/{id}/images/upload
	// Contract: Rejected with 403 Forbidden. Zero product_images rows created.
	// =========================================================================
	t.Run("AdminCannotUploadImageViaSellerEndpoint", func(t *testing.T) {
		sellerUploadURL := fmt.Sprintf("/api/seller/products/%s/images/upload", draftProdID.String())
		dummyFile := []byte("fake image payload")
		res := execMultipartReq(http.MethodPost, sellerUploadURL, adminToken, "image", "photo.png", dummyFile)

		assert.Equal(t, http.StatusForbidden, res.Code, "Admin must be rejected from seller image upload")

		var imgCount int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM product_images WHERE product_id = $1", draftProdID).Scan(&imgCount)
		require.NoError(t, err)
		assert.Equal(t, 0, imgCount, "no images must be created in DB")
	})

	// =========================================================================
	// Test 8: Admin cannot delete, reorder, crop, or set main image via Seller routes
	// Contract: All rejected with 403 Forbidden. Existing seller images remain intact.
	// =========================================================================
	t.Run("AdminCannotMutateSellerImagesViaSellerEndpoints", func(t *testing.T) {
		// 8a. Admin DELETE image
		delURL := fmt.Sprintf("/api/seller/products/%s/images/%s", pubProdID.String(), pubImageID.String())
		resDel := execReq(http.MethodDelete, delURL, adminToken, nil)
		assert.Equal(t, http.StatusForbidden, resDel.Code, "Admin must not be able to delete seller image")

		// 8b. Admin PUT reorder
		reorderURL := fmt.Sprintf("/api/seller/products/%s/images/reorder", pubProdID.String())
		reorderBody, _ := json.Marshal(map[string]any{"imageIds": []string{pubImageID.String()}})
		resReorder := execReq(http.MethodPut, reorderURL, adminToken, reorderBody)
		assert.Equal(t, http.StatusForbidden, resReorder.Code, "Admin must not be able to reorder seller images")

		// 8c. Admin POST crop
		cropURL := fmt.Sprintf("/api/seller/products/%s/images/%s/crop", pubProdID.String(), pubImageID.String())
		cropBody, _ := json.Marshal(map[string]any{"cropX": 0, "cropY": 0, "cropWidth": 100, "cropHeight": 100})
		resCrop := execReq(http.MethodPost, cropURL, adminToken, cropBody)
		assert.Equal(t, http.StatusForbidden, resCrop.Code, "Admin must not be able to crop seller image")

		// 8d. Admin POST set main
		mainURL := fmt.Sprintf("/api/seller/products/%s/images/%s/main", pubProdID.String(), pubImageID.String())
		resMain := execReq(http.MethodPost, mainURL, adminToken, nil)
		assert.Equal(t, http.StatusForbidden, resMain.Code, "Admin must not be able to set main seller image")

		// Verify database state of image is intact
		var dbImgURL string
		var dbIsMain bool
		err = pgClient.Pool.QueryRow(ctx, "SELECT image_url, is_main FROM product_images WHERE id = $1", pubImageID).Scan(&dbImgURL, &dbIsMain)
		require.NoError(t, err)
		assert.Equal(t, pubImageURL, dbImgURL, "image URL must remain intact")
		assert.True(t, dbIsMain, "image is_main must remain true")
	})

	// =========================================================================
	// Test 9: Admin legitimate inspection includes seller product images
	// Contract: 200 OK, response payload contains seller product images
	// =========================================================================
	t.Run("AdminLegitimateInspectionIncludesProductImages", func(t *testing.T) {
		adminURL := fmt.Sprintf("/api/admin/products/%s", pubProdID.String())
		res := execReq(http.MethodGet, adminURL, adminToken, nil)

		assert.Equal(t, http.StatusOK, res.Code, "Admin must be able to inspect product")
		assert.Contains(t, res.Body.String(), pubImageURL, "Admin inspection response must include product images")
	})

	// =========================================================================
	// Test 10: Admin legitimate moderation reject with photo feedback succeeds
	// Contract: 200 OK, status transitions to rejected, comment saved in audit log
	// =========================================================================
	t.Run("AdminModerationRejectWithPhotoFeedbackSucceeds", func(t *testing.T) {
		rejectURL := fmt.Sprintf("/api/admin/moderation/products/%s/reject", pendingProdID.String())
		payload := map[string]any{
			"comment": "Фотографии низкого качества или содержат водяные знаки сторонних ресурсов",
		}
		body, _ := json.Marshal(payload)

		res := execReq(http.MethodPost, rejectURL, adminToken, body)
		assert.Equal(t, http.StatusOK, res.Code, "Admin must be able to reject product with photo reason")

		// Verify product status is rejected
		var dbStatus string
		err = pgClient.Pool.QueryRow(ctx, "SELECT status FROM products WHERE id = $1", pendingProdID).Scan(&dbStatus)
		require.NoError(t, err)
		assert.Equal(t, "rejected", dbStatus)

		// Verify moderation log
		var logComment string
		err = pgClient.Pool.QueryRow(ctx, "SELECT comment FROM product_moderation_logs WHERE product_id = $1 ORDER BY created_at DESC LIMIT 1", pendingProdID).Scan(&logComment)
		require.NoError(t, err)
		assert.Equal(t, "Фотографии низкого качества или содержат водяные знаки сторонних ресурсов", logComment)
	})
}
