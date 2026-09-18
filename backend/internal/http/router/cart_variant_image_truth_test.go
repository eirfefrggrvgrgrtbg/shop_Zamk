package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCartVariantImageTruth verifies that Cart items deterministically resolve
// their image URL based on the purchased variant's color according to canonical rules:
// 1. Variant White -> First matching White image in canonical order (sort_order ASC, created_at ASC)
// 2. Variant Black -> First matching Black image in canonical order (rendition_url if present)
// 3. Multiple images for same color -> respects lowest sort_order, ignoring is_main flag
// 4. Variant color has NO matching image -> falls back to first general image (colorId == null)
// 5. Product has NO matching color image AND NO general image -> falls back to product main_image_url
// 6. Product has NO images at all -> returns null/empty (Cart frontend falls back to placeholder)
// 7. Variant has NO color (color_id == null) -> returns first general image
// 8. Existing cart item -> deterministic persisted visual truth across re-fetches
func TestCartVariantImageTruth(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupProductPublicationTestEnv(t)
	defer cleanup()

	var (
		createdSellerIDs   []uuid.UUID
		createdCategoryIDs []uuid.UUID
		createdProductIDs  []uuid.UUID
		createdVariantIDs  []uuid.UUID
		createdUserIDs     []uuid.UUID
	)

	t.Cleanup(func() {
		var dbName string
		if err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err == nil && dbName == "zamk_test" {
			if len(createdUserIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM cart_items WHERE cart_id IN (SELECT id FROM carts WHERE user_id = ANY($1))", createdUserIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM carts WHERE user_id = ANY($1)", createdUserIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", createdUserIDs)
			}
			if len(createdVariantIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM cart_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE id = ANY($1)", createdVariantIDs)
			}
			if len(createdProductIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = ANY($1)", createdProductIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProductIDs)
			}
			if len(createdCategoryIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCategoryIDs)
			}
			if len(createdSellerIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
			}
		}
	})

	// 1. Seller & Category
	sellerID := uuid.New()
	sellerSlug := "img-truth-seller-" + sellerID.String()[:8]
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Image Truth Brand', $2, $3, 'active', now(), now())
	`, sellerID, sellerSlug, sellerSlug+"@test.com")
	require.NoError(t, err)
	createdSellerIDs = append(createdSellerIDs, sellerID)

	catID := uuid.New()
	catSlug := "img-truth-cat-" + catID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Image Truth Cat', $2, true, now(), now())
	`, catID, catSlug)
	require.NoError(t, err)
	createdCategoryIDs = append(createdCategoryIDs, catID)

	// 2. Colors: White, Black, Yellow, Green
	getColorOrInsert := func(code, nameRu, hex string, sortOrder int) uuid.UUID {
		var id uuid.UUID
		err := pgClient.Pool.QueryRow(ctx, "SELECT id FROM colors WHERE code = $1 LIMIT 1", code).Scan(&id)
		if err != nil {
			id = uuid.New()
			_, err = pgClient.Pool.Exec(ctx, `
				INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active)
				VALUES ($1, $2, $3, $4, $5, true)
			`, id, code, nameRu, hex, sortOrder)
			require.NoError(t, err)
		}
		return id
	}

	whiteColorID := getColorOrInsert("white_test", "Белый Тест", "#FFFFFF", 10)
	blackColorID := getColorOrInsert("black_test", "Черный Тест", "#000000", 20)
	yellowColorID := getColorOrInsert("yellow_test", "Желтый Тест", "#FFFF00", 30)
	greenColorID := getColorOrInsert("green_test", "Зеленый Тест", "#00FF00", 40)

	// 3. Size
	var sizeSystemID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM size_systems LIMIT 1").Scan(&sizeSystemID)
	if err != nil {
		sizeSystemID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO size_systems (id, code, name, is_active)
			VALUES ($1, 'INT', 'International', true)
		`, sizeSystemID)
		require.NoError(t, err)
	}

	var sizeMID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM size_values WHERE size_system_id = $1 AND value = 'M' LIMIT 1", sizeSystemID).Scan(&sizeMID)
	if err != nil {
		sizeMID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO size_values (id, size_system_id, value, sort_order, is_active)
			VALUES ($1, $2, 'M', 1, true)
		`, sizeMID, sizeSystemID)
		require.NoError(t, err)
	}

	now := time.Now()

	// -------------------------------------------------------------
	// PRODUCT 1: Rich media product
	// - main_image_url: http://cdn.test/p1-main.jpg
	// - product_images:
	//     1) General Image A: sort_order=5, color_id=NULL, url="http://cdn.test/p1-gen-5.jpg"
	//     2) General Image B: sort_order=1, color_id=NULL, url="http://cdn.test/p1-gen-1.jpg"
	//     3) White Image Late: sort_order=10, color_id=White, is_main=true, url="http://cdn.test/p1-white-10.jpg"
	//     4) White Image Early: sort_order=3, color_id=White, is_main=false, url="http://cdn.test/p1-white-3.jpg"
	//     5) Black Image: sort_order=2, color_id=Black, rendition_url="http://cdn.test/p1-black-rendition.jpg", image_url="http://cdn.test/p1-black-raw.jpg"
	// - variants:
	//     varWhite (White / M)
	//     varBlack (Black / M)
	//     varYellow (Yellow / M) -> has no Yellow images!
	//     varNoColor (NULL / M) -> variant has no color!
	// -------------------------------------------------------------
	p1ID := uuid.New()
	p1Slug := "p1-rich-" + p1ID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			main_image_url, submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Rich Media Hoodie', $4, 10000, 'RUB', 'published',
			'http://cdn.test/p1-main.jpg', $5, $5, $5, $5, $5)
	`, p1ID, sellerID, catID, p1Slug, now)
	require.NoError(t, err)
	createdProductIDs = append(createdProductIDs, p1ID)

	// Insert images for Product 1
	// General Image A (sort_order 5)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, 'http://cdn.test/p1-gen-5.jpg', 5, false, NULL, now())
	`, uuid.New(), p1ID)
	require.NoError(t, err)

	// General Image B (sort_order 1) - First general image
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, 'http://cdn.test/p1-gen-1.jpg', 1, false, NULL, now())
	`, uuid.New(), p1ID)
	require.NoError(t, err)

	// White Image Late (sort_order 10, is_main=true)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, 'http://cdn.test/p1-white-10.jpg', 10, true, $3, now())
	`, uuid.New(), p1ID, whiteColorID)
	require.NoError(t, err)

	// White Image Early (sort_order 3, is_main=false) - First White image by sort_order
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, 'http://cdn.test/p1-white-3.jpg', 3, false, $3, now())
	`, uuid.New(), p1ID, whiteColorID)
	require.NoError(t, err)

	// Black Image (sort_order 2, with rendition_url)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, rendition_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, 'http://cdn.test/p1-black-raw.jpg', 'http://cdn.test/p1-black-rendition.jpg', 2, false, $3, now())
	`, uuid.New(), p1ID, blackColorID)
	require.NoError(t, err)

	// Insert Variants for Product 1
	createVariantWithStock := func(prodID uuid.UUID, colorID *uuid.UUID, sku string) uuid.UUID {
		vID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, color_id, size_value_id, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 10000, true, now(), now())
		`, vID, prodID, colorID, sizeMID, sku, "BC-"+vID.String()[:8])
		require.NoError(t, err)
		createdVariantIDs = append(createdVariantIDs, vID)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 10, 0, now(), now())
		`, uuid.New(), prodID, vID, sellerID)
		require.NoError(t, err)
		return vID
	}

	varWhiteID := createVariantWithStock(p1ID, &whiteColorID, "SKU-WHITE-M")
	varBlackID := createVariantWithStock(p1ID, &blackColorID, "SKU-BLACK-M")
	varYellowID := createVariantWithStock(p1ID, &yellowColorID, "SKU-YELLOW-M")
	varNoColorID := createVariantWithStock(p1ID, nil, "SKU-NOCOLOR-M")

	// -------------------------------------------------------------
	// PRODUCT 2: Only White images, NO general images
	// - main_image_url: http://cdn.test/p2-main.jpg
	// - product_images:
	//     1) White Image: sort_order=1, color_id=White, url="http://cdn.test/p2-white.jpg"
	// - variant:
	//     varGreen (Green / M) -> no Green image and no general image
	// -------------------------------------------------------------
	p2ID := uuid.New()
	p2Slug := "p2-specific-" + p2ID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			main_image_url, submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Specific Color Hoodie', $4, 10000, 'RUB', 'published',
			'http://cdn.test/p2-main.jpg', $5, $5, $5, $5, $5)
	`, p2ID, sellerID, catID, p2Slug, now)
	require.NoError(t, err)
	createdProductIDs = append(createdProductIDs, p2ID)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, 'http://cdn.test/p2-white.jpg', 1, true, $3, now())
	`, uuid.New(), p2ID, whiteColorID)
	require.NoError(t, err)

	varGreenID := createVariantWithStock(p2ID, &greenColorID, "SKU-GREEN-M")

	// -------------------------------------------------------------
	// PRODUCT 3: Zero images and NO main_image_url
	// - main_image_url: NULL
	// - product_images: none
	// - variant:
	//     varNoImages (White / M)
	// -------------------------------------------------------------
	p3ID := uuid.New()
	p3Slug := "p3-no-images-" + p3ID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			main_image_url, submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'No Images Hoodie', $4, 10000, 'RUB', 'published',
			NULL, $5, $5, $5, $5, $5)
	`, p3ID, sellerID, catID, p3Slug, now)
	require.NoError(t, err)
	createdProductIDs = append(createdProductIDs, p3ID)

	varNoImagesID := createVariantWithStock(p3ID, &whiteColorID, "SKU-NOIMG-M")

	// Helper to add variant to cart and return the items array
	addToCartAndFetchItems := func(custToken string, prodID, varID uuid.UUID) []map[string]any {
		addPayload := fmt.Sprintf(`{"productId":"%s","productVariantId":"%s","quantity":1}`, prodID, varID)
		addReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader([]byte(addPayload)))
		addReq.Header.Set("Authorization", "Bearer "+custToken)
		addReq.Header.Set("Content-Type", "application/json")
		addRec := httptest.NewRecorder()
		r.ServeHTTP(addRec, addReq)
		require.Equal(t, http.StatusCreated, addRec.Code)

		cartReq := httptest.NewRequest(http.MethodGet, "/api/customer/cart", nil)
		cartReq.Header.Set("Authorization", "Bearer "+custToken)
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		require.Equal(t, http.StatusOK, cartRec.Code)

		var rawCart map[string]any
		err := json.Unmarshal(cartRec.Body.Bytes(), &rawCart)
		require.NoError(t, err)

		rawItems, ok := rawCart["items"].([]any)
		require.True(t, ok)

		var items []map[string]any
		for _, ri := range rawItems {
			items = append(items, ri.(map[string]any))
		}
		return items
	}

	// Helper to create a fresh customer token for isolated cart tests
	createFreshCustomer := func(tag string) (uuid.UUID, string) {
		uid := uuid.New()
		email := fmt.Sprintf("cust-%s-%s@test.com", tag, uid.String()[:6])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'hash', 'customer', 'active', now(), now())
		`, uid, email, "+7995"+uid.String()[:7], tag)
		require.NoError(t, err)
		createdUserIDs = append(createdUserIDs, uid)

		tok, err := tokenService.GenerateAccessToken(uid, email, "customer")
		require.NoError(t, err)
		return uid, tok
	}

	// -------------------------------------------------------------
	// SCENARIO 1 & 3: Variant White -> Returns first White image by canonical order (sort_order ASC, created_at ASC)
	// Even though White Image Late has is_main=true (sort_order=10),
	// White Image Early (sort_order=3) MUST be selected!
	// -------------------------------------------------------------
	t.Run("1_and_3. Variant White returns first White image by canonical sort_order, ignoring is_main override", func(t *testing.T) {
		_, token := createFreshCustomer("white-test")
		items := addToCartAndFetchItems(token, p1ID, varWhiteID)
		require.Len(t, items, 1)

		assert.Equal(t, varWhiteID.String(), items[0]["productVariantId"])
		assert.Equal(t, "http://cdn.test/p1-white-3.jpg", items[0]["imageUrl"], "must select lowest sort_order (3), NOT sort_order 10 despite is_main=true")
	})

	// -------------------------------------------------------------
	// SCENARIO 2: Variant Black -> Returns matching Black image (rendition_url prioritized over raw image_url)
	// -------------------------------------------------------------
	t.Run("2. Variant Black returns matching Black image with rendition_url", func(t *testing.T) {
		_, token := createFreshCustomer("black-test")
		items := addToCartAndFetchItems(token, p1ID, varBlackID)
		require.Len(t, items, 1)

		assert.Equal(t, varBlackID.String(), items[0]["productVariantId"])
		assert.Equal(t, "http://cdn.test/p1-black-rendition.jpg", items[0]["imageUrl"], "must select rendition_url when present")
	})

	// -------------------------------------------------------------
	// SCENARIO 4: Variant color has NO matching image -> Falls back to first general image (colorId == null)
	// Product 1 has general images sort_order=1 and sort_order=5.
	// Yellow variant has no yellow images, so it must pick sort_order=1 general image.
	// -------------------------------------------------------------
	t.Run("4. Variant color with no matching images falls back to first general image", func(t *testing.T) {
		_, token := createFreshCustomer("yellow-test")
		items := addToCartAndFetchItems(token, p1ID, varYellowID)
		require.Len(t, items, 1)

		assert.Equal(t, varYellowID.String(), items[0]["productVariantId"])
		assert.Equal(t, "http://cdn.test/p1-gen-1.jpg", items[0]["imageUrl"], "must select lowest sort_order general image")
	})

	// -------------------------------------------------------------
	// SCENARIO 5: Product has NO matching color image AND NO general image -> Falls back to product main_image_url
	// Product 2 only has White image. Green variant has no green image and no general image.
	// Must fall back to Product 2 main_image_url ("http://cdn.test/p2-main.jpg").
	// -------------------------------------------------------------
	t.Run("5. Variant with no matching color and no general image falls back to product main_image_url", func(t *testing.T) {
		_, token := createFreshCustomer("green-test")
		items := addToCartAndFetchItems(token, p2ID, varGreenID)
		require.Len(t, items, 1)

		assert.Equal(t, varGreenID.String(), items[0]["productVariantId"])
		assert.Equal(t, "http://cdn.test/p2-main.jpg", items[0]["imageUrl"], "must fall back to product main_image_url")
	})

	// -------------------------------------------------------------
	// SCENARIO 6: Product has NO images at all -> returns null/empty
	// -------------------------------------------------------------
	t.Run("6. Product with no images and no main_image_url returns nil imageUrl", func(t *testing.T) {
		_, token := createFreshCustomer("noimg-test")
		items := addToCartAndFetchItems(token, p3ID, varNoImagesID)
		require.Len(t, items, 1)

		assert.Equal(t, varNoImagesID.String(), items[0]["productVariantId"])
		assert.Nil(t, items[0]["imageUrl"], "must be nil when no images exist at all")
	})

	// -------------------------------------------------------------
	// SCENARIO 7: Variant has NO color (color_id == null) -> returns first general image
	// Product 1 varNoColor has color_id=NULL. Should pick general image 1 ("http://cdn.test/p1-gen-1.jpg").
	// -------------------------------------------------------------
	t.Run("7. Variant with no color_id returns first general image", func(t *testing.T) {
		_, token := createFreshCustomer("nocolor-test")
		items := addToCartAndFetchItems(token, p1ID, varNoColorID)
		require.Len(t, items, 1)

		assert.Equal(t, varNoColorID.String(), items[0]["productVariantId"])
		assert.Equal(t, "http://cdn.test/p1-gen-1.jpg", items[0]["imageUrl"], "must return first general image for uncolored variant")
	})

	// -------------------------------------------------------------
	// SCENARIO 8: Existing cart item visual truth is persisted and deterministic across fetches
	// Add variant to cart -> fetch 1 -> fetch 2 -> both return identical imageUrl.
	// -------------------------------------------------------------
	t.Run("8. Existing cart item returns deterministic, persisted variant visual truth across re-fetches", func(t *testing.T) {
		_, token := createFreshCustomer("persist-test")
		items1 := addToCartAndFetchItems(token, p1ID, varWhiteID)
		require.Len(t, items1, 1)
		img1 := items1[0]["imageUrl"]
		assert.Equal(t, "http://cdn.test/p1-white-3.jpg", img1)

		// Second fetch
		cartReq := httptest.NewRequest(http.MethodGet, "/api/customer/cart", nil)
		cartReq.Header.Set("Authorization", "Bearer "+token)
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		require.Equal(t, http.StatusOK, cartRec.Code)

		var rawCart map[string]any
		err := json.Unmarshal(cartRec.Body.Bytes(), &rawCart)
		require.NoError(t, err)
		rawItems := rawCart["items"].([]any)
		require.Len(t, rawItems, 1)
		img2 := rawItems[0].(map[string]any)["imageUrl"]
		assert.Equal(t, img1, img2, "persisted commerce identity must return identical image on subsequent fetch")
	})
}
