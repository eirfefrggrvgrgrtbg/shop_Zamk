package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCartBrandConsistency verifies that Cart items deterministically resolve
// their brand name from the canonical product brand truth (products.brand_id -> brands.name):
// 1. Product with canonical brand -> returns brandName == "DEV BRAND"
// 2. Product without brand (brand_id == null) -> returns brandName == nil (omitted/empty)
// 3. Seller name isolation -> brandName NEVER substitutes seller.brand_name or seller shop name
// 4. Persistence across re-fetches -> deterministic brand truth across multiple reads
// 5. Variant visual truth intact -> imageUrl, size, color remain unaffected
func TestCartBrandConsistency(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupProductPublicationTestEnv(t)
	defer cleanup()

	var (
		createdSellerIDs   []uuid.UUID
		createdCategoryIDs []uuid.UUID
		createdBrandIDs    []uuid.UUID
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
			if len(createdBrandIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM brands WHERE id = ANY($1)", createdBrandIDs)
			}
			if len(createdCategoryIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCategoryIDs)
			}
			if len(createdSellerIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
			}
		}
	})

	// 1. Seller with distinct seller brand_name to test isolation
	sellerID := uuid.New()
	sellerSlug := "brand-seller-" + sellerID.String()[:8]
	sellerShopName := "SELLER COMMERCIAL ENTITY " + sellerID.String()[:4]
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', now(), now())
	`, sellerID, sellerShopName, sellerSlug, sellerSlug+"@test.com")
	require.NoError(t, err)
	createdSellerIDs = append(createdSellerIDs, sellerID)

	// 2. Category
	catID := uuid.New()
	catSlug := "brand-cat-" + catID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Brand Consistency Cat', $2, true, now(), now())
	`, catID, catSlug)
	require.NoError(t, err)
	createdCategoryIDs = append(createdCategoryIDs, catID)

	// 3. Canonical Brand: "DEV BRAND"
	brandID := uuid.New()
	brandSlug := "dev-brand-" + brandID.String()[:8]
	canonicalBrandName := "DEV BRAND"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO brands (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, true, now(), now())
	`, brandID, canonicalBrandName, brandSlug)
	require.NoError(t, err)
	createdBrandIDs = append(createdBrandIDs, brandID)

	// 4. Color & Size
	var colorID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM colors LIMIT 1").Scan(&colorID)
	if err != nil {
		colorID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active)
			VALUES ($1, 'black_test', 'Черный', '#000000', 1, true)
		`, colorID)
		require.NoError(t, err)
	}

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

	var sizeID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM size_values WHERE size_system_id = $1 LIMIT 1", sizeSystemID).Scan(&sizeID)
	if err != nil {
		sizeID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO size_values (id, size_system_id, value, sort_order, is_active)
			VALUES ($1, $2, 'L', 1, true)
		`, sizeID, sizeSystemID)
		require.NoError(t, err)
	}

	// 5. Product A: has canonical brand
	productAID := uuid.New()
	productASlug := "prod-with-brand-" + productAID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status, main_image_url)
		VALUES ($1, $2, $3, $4, 'Product With Brand', $5, 120000, 'RUB', 'published', 'https://cdn.zamk.test/main_a.jpg')
	`, productAID, sellerID, catID, brandID, productASlug)
	require.NoError(t, err)
	createdProductIDs = append(createdProductIDs, productAID)

	variantAID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, color_id, size_value_id, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'SKU-PROD-A-01', $5, 120000, true, now(), now())
	`, variantAID, productAID, colorID, sizeID, "BC-"+variantAID.String()[:8])
	require.NoError(t, err)
	createdVariantIDs = append(createdVariantIDs, variantAID)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 0, now(), now())
	`, uuid.New(), productAID, variantAID, sellerID)
	require.NoError(t, err)

	// 6. Product B: NO brand (brand_id == NULL)
	productBID := uuid.New()
	productBSlug := "prod-no-brand-" + productBID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, price_cents, currency, status, main_image_url)
		VALUES ($1, $2, $3, NULL, 'Product Without Brand', $4, 80000, 'RUB', 'published', 'https://cdn.zamk.test/main_b.jpg')
	`, productBID, sellerID, catID, productBSlug)
	require.NoError(t, err)
	createdProductIDs = append(createdProductIDs, productBID)

	variantBID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, color_id, size_value_id, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'SKU-PROD-B-01', $5, 80000, true, now(), now())
	`, variantBID, productBID, colorID, sizeID, "BC-"+variantBID.String()[:8])
	require.NoError(t, err)
	createdVariantIDs = append(createdVariantIDs, variantBID)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10, 0, now(), now())
	`, uuid.New(), productBID, variantBID, sellerID)
	require.NoError(t, err)

	// 7. Customer user + token
	userID := uuid.New()
	userEmail := fmt.Sprintf("cust-brand-%s@test.com", userID.String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Customer Brand', 'hash', 'customer', 'active', now(), now())
	`, userID, userEmail, "+7999"+userID.String()[:7])
	require.NoError(t, err)
	createdUserIDs = append(createdUserIDs, userID)

	token, err := tokenService.GenerateAccessToken(userID, userEmail, "customer")
	require.NoError(t, err)

	authReq := func(req *http.Request) *http.Request {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	// 8. Add Product A (with brand) to cart
	addBodyA, _ := json.Marshal(map[string]interface{}{
		"productId":        productAID.String(),
		"productVariantId": variantAID.String(),
		"quantity":         1,
	})
	addReqA := authReq(httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(addBodyA)))
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, addReqA)
	require.Equal(t, http.StatusCreated, wA.Code)

	// 9. Add Product B (without brand) to cart
	addBodyB, _ := json.Marshal(map[string]interface{}{
		"productId":        productBID.String(),
		"productVariantId": variantBID.String(),
		"quantity":         2,
	})
	addReqB := authReq(httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader(addBodyB)))
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, addReqB)
	require.Equal(t, http.StatusCreated, wB.Code)

	// 10. Fetch Cart and verify brand consistency
	getReq := authReq(httptest.NewRequest(http.MethodGet, "/api/customer/cart", nil))
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, getReq)
	require.Equal(t, http.StatusOK, wGet.Code)

	var cartResp struct {
		ID    string `json:"id"`
		Items []struct {
			ID               string  `json:"id"`
			ProductID        string  `json:"productId"`
			ProductVariantID string  `json:"productVariantId"`
			Quantity         int     `json:"quantity"`
			Title            string  `json:"title"`
			BrandName        *string `json:"brandName"`
			ImageURL         *string `json:"imageUrl"`
		} `json:"items"`
	}
	err = json.Unmarshal(wGet.Body.Bytes(), &cartResp)
	require.NoError(t, err)
	require.Len(t, cartResp.Items, 2)

	// Find items
	var itemA, itemB *struct {
		ID               string  `json:"id"`
		ProductID        string  `json:"productId"`
		ProductVariantID string  `json:"productVariantId"`
		Quantity         int     `json:"quantity"`
		Title            string  `json:"title"`
		BrandName        *string `json:"brandName"`
		ImageURL         *string `json:"imageUrl"`
	}

	for i := range cartResp.Items {
		if cartResp.Items[i].ProductID == productAID.String() {
			itemA = &cartResp.Items[i]
		} else if cartResp.Items[i].ProductID == productBID.String() {
			itemB = &cartResp.Items[i]
		}
	}

	require.NotNil(t, itemA, "Item A should be in cart")
	require.NotNil(t, itemB, "Item B should be in cart")

	// Assertion 1: Product A has canonical brand name
	require.NotNil(t, itemA.BrandName, "Product A brandName should not be nil")
	assert.Equal(t, canonicalBrandName, *itemA.BrandName)

	// Assertion 2: Product B has NO brand (brandName is nil / omitted)
	assert.Nil(t, itemB.BrandName, "Product B brandName must be nil when product has no brand_id")

	// Assertion 3: Brand name must NOT fall back to seller name or seller shop name
	if itemB.BrandName != nil {
		assert.NotEqual(t, sellerShopName, *itemB.BrandName)
	}

	// Assertion 4: Image URL truth remains intact
	assert.NotNil(t, itemA.ImageURL)
	assert.Equal(t, "https://cdn.zamk.test/main_a.jpg", *itemA.ImageURL)

	// 11. Test Persistence across re-fetch
	reGetReq := authReq(httptest.NewRequest(http.MethodGet, "/api/customer/cart", nil))
	wReGet := httptest.NewRecorder()
	r.ServeHTTP(wReGet, reGetReq)
	require.Equal(t, http.StatusOK, wReGet.Code)

	var reCartResp struct {
		Items []struct {
			ProductID string  `json:"productId"`
			BrandName *string `json:"brandName"`
		} `json:"items"`
	}
	err = json.Unmarshal(wReGet.Body.Bytes(), &reCartResp)
	require.NoError(t, err)

	for _, it := range reCartResp.Items {
		if it.ProductID == productAID.String() {
			require.NotNil(t, it.BrandName)
			assert.Equal(t, canonicalBrandName, *it.BrandName)
		} else if it.ProductID == productBID.String() {
			assert.Nil(t, it.BrandName)
		}
	}
}
