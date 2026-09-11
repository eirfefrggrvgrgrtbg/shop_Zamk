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

func TestProductVariantTruth(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupProductPublicationTestEnv(t)
	defer cleanup()

	var (
		createdSellerIDs         []uuid.UUID
		createdCategoryIDs       []uuid.UUID
		createdProductIDs        []uuid.UUID
		createdVariantIDs        []uuid.UUID
		createdUserIDs           []uuid.UUID
		createdDeliveryMethodIDs []uuid.UUID
		createdOrderIDs          []uuid.UUID
	)

	t.Cleanup(func() {
		var dbName string
		if err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err == nil && dbName == "zamk_test" {
			if len(createdOrderIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM order_items WHERE order_id = ANY($1)", createdOrderIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM orders WHERE id = ANY($1)", createdOrderIDs)
			}
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
			if len(createdDeliveryMethodIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM delivery_methods WHERE id = ANY($1)", createdDeliveryMethodIDs)
			}
			if len(createdCategoryIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCategoryIDs)
			}
			if len(createdSellerIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
			}
		}
	})

	// 1. Create Seller
	sellerID := uuid.New()
	sellerSlug := "pv-seller-" + sellerID.String()[:8]
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'PV Test Brand', $2, $3, 'active', now(), now())
	`, sellerID, sellerSlug, sellerSlug+"@test.com")
	require.NoError(t, err)
	createdSellerIDs = append(createdSellerIDs, sellerID)

	// 2. Create Category
	catID := uuid.New()
	catSlug := "pv-cat-" + catID.String()[:8]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'PV Category', $2, true, now(), now())
	`, catID, catSlug)
	require.NoError(t, err)
	createdCategoryIDs = append(createdCategoryIDs, catID)

	// 3. Resolve / Insert Color (Красный)
	var redColorID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM colors WHERE name_ru = 'Красный' LIMIT 1").Scan(&redColorID)
	if err != nil {
		redColorID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active)
			VALUES ($1, 'red', 'Красный', '#FF0000', 1, true)
		`, redColorID)
		require.NoError(t, err)
	}

	// 4. Resolve / Insert Size System & Size Values (L, XL)
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

	var sizeLID, sizeXLID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM size_values WHERE size_system_id = $1 AND value = 'L' LIMIT 1", sizeSystemID).Scan(&sizeLID)
	if err != nil {
		sizeLID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO size_values (id, size_system_id, value, sort_order, is_active)
			VALUES ($1, $2, 'L', 1, true)
		`, sizeLID, sizeSystemID)
		require.NoError(t, err)
	}

	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM size_values WHERE size_system_id = $1 AND value = 'XL' LIMIT 1", sizeSystemID).Scan(&sizeXLID)
	if err != nil {
		sizeXLID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO size_values (id, size_system_id, value, sort_order, is_active)
			VALUES ($1, $2, 'XL', 2, true)
		`, sizeXLID, sizeSystemID)
		require.NoError(t, err)
	}

	// 5. Ensure Delivery Method exists for orders
	var deliveryMethodID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, "SELECT id FROM delivery_methods WHERE is_active = true LIMIT 1").Scan(&deliveryMethodID)
	if err != nil {
		deliveryMethodID = uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
			VALUES ($1, 'pv-std-delivery', 'Standard Delivery', 50000, true, now(), now())
		`, deliveryMethodID)
		require.NoError(t, err)
		createdDeliveryMethodIDs = append(createdDeliveryMethodIDs, deliveryMethodID)
	}

	// 6. Create Customer
	customerID := uuid.New()
	customerEmail := "pv-cust-" + customerID.String()[:8] + "@test.com"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'PV Customer', 'hash', 'customer', 'active', now(), now())
	`, customerID, customerEmail, "+7991"+customerID.String()[:7])
	require.NoError(t, err)
	createdUserIDs = append(createdUserIDs, customerID)

	customerToken, err := tokenService.GenerateAccessToken(customerID, customerEmail, "customer")
	require.NoError(t, err)

	// 7. Setup Modern Product with Red / L (stock=2) and Red / XL (stock=0)
	prodID := uuid.New()
	prodSlug := "pv-hoodie-" + prodID.String()[:8]
	now := time.Now()

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, title, slug, price_cents, currency, status,
			submitted_at, approved_at, published_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'Худи Modern', $4, 122200, 'RUB', 'published', $5, $5, $5, $5, $5)
	`, prodID, sellerID, catID, prodSlug, now)
	require.NoError(t, err)
	createdProductIDs = append(createdProductIDs, prodID)

	// General image (color_id IS NULL)
	genImgID := uuid.New()
	genImgURL := "http://minio.local/general.jpg"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, $3, 0, true, NULL, now())
	`, genImgID, prodID, genImgURL)
	require.NoError(t, err)

	// Color-specific image for Red
	redImgID := uuid.New()
	redImgURL := "http://minio.local/red.jpg"
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
		VALUES ($1, $2, $3, 0, false, $4, now())
	`, redImgID, prodID, redImgURL, redColorID)
	require.NoError(t, err)

	// Variant 1: Red / L
	varRedLID := uuid.New()
	redLSKU := "SKU-RED-L-" + varRedLID.String()[:6]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (
			id, product_id, color_id, size_value_id, seller_sku, barcode, price_cents, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 122200, true, now(), now())
	`, varRedLID, prodID, redColorID, sizeLID, redLSKU, "BC-"+varRedLID.String()[:8])
	require.NoError(t, err)
	createdVariantIDs = append(createdVariantIDs, varRedLID)

	// Inventory for Red / L: total 2, reserved 0 -> free 2
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 2, 0, now(), now())
	`, uuid.New(), prodID, varRedLID, sellerID)
	require.NoError(t, err)

	// Variant 2: Red / XL
	varRedXLID := uuid.New()
	redXLSKU := "SKU-RED-XL-" + varRedXLID.String()[:6]
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO product_variants (
			id, product_id, color_id, size_value_id, seller_sku, barcode, price_cents, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 122200, true, now(), now())
	`, varRedXLID, prodID, redColorID, sizeXLID, redXLSKU, "BC-"+varRedXLID.String()[:8])
	require.NoError(t, err)
	createdVariantIDs = append(createdVariantIDs, varRedXLID)

	// Inventory for Red / XL: 0 stock
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 0, 0, now(), now())
	`, uuid.New(), prodID, varRedXLID, sellerID)
	require.NoError(t, err)

	// -------------------------------------------------------------
	// TEST A: Public Product DTO Wire JSON
	// -------------------------------------------------------------
	t.Run("A. Public Product DTO returns canonical colorName, colorHex, size, inStock in wire JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", prodID), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var rawResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &rawResp)
		require.NoError(t, err)

		rawVariants, ok := rawResp["variants"].([]any)
		require.True(t, ok, "variants array must exist in wire JSON")
		require.Len(t, rawVariants, 2)

		var vLFound, vXLFound bool
		for _, rawV := range rawVariants {
			vMap, ok := rawV.(map[string]any)
			require.True(t, ok)
			vIDStr, _ := vMap["id"].(string)

			if vIDStr == varRedLID.String() {
				vLFound = true
				assert.Equal(t, redLSKU, vMap["sellerSku"])
				assert.Equal(t, redLSKU, vMap["sku"])
				assert.Equal(t, redColorID.String(), vMap["colorId"])
				assert.Equal(t, "Красный", vMap["colorName"])
				assert.Equal(t, "Красный", vMap["color"])
				assert.Equal(t, "#FF0000", vMap["colorHex"])
				assert.Equal(t, sizeLID.String(), vMap["sizeValueId"])
				assert.Equal(t, "L", vMap["size"])
				assert.Equal(t, true, vMap["isActive"])
				assert.Equal(t, true, vMap["inStock"])
			}
			if vIDStr == varRedXLID.String() {
				vXLFound = true
				assert.Equal(t, redXLSKU, vMap["sellerSku"])
				assert.Equal(t, redXLSKU, vMap["sku"])
				assert.Equal(t, redColorID.String(), vMap["colorId"])
				assert.Equal(t, "Красный", vMap["colorName"])
				assert.Equal(t, "Красный", vMap["color"])
				assert.Equal(t, "#FF0000", vMap["colorHex"])
				assert.Equal(t, sizeXLID.String(), vMap["sizeValueId"])
				assert.Equal(t, "XL", vMap["size"])
				assert.Equal(t, true, vMap["isActive"])
				assert.Equal(t, false, vMap["inStock"])
			}
		}
		require.True(t, vLFound, "Red / L variant must be present in wire JSON")
		require.True(t, vXLFound, "Red / XL variant must be present in wire JSON")
	})

	// -------------------------------------------------------------
	// TEST B: Cart Exact Variant Wire JSON
	// -------------------------------------------------------------
	t.Run("B. Cart returns exact variant identity, size, color, sellerSku, and color-specific imageUrl in wire JSON", func(t *testing.T) {
		// Add Red / L to cart
		addPayload := fmt.Sprintf(`{"productId":"%s","productVariantId":"%s","quantity":1}`, prodID, varRedLID)
		addReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader([]byte(addPayload)))
		addReq.Header.Set("Authorization", "Bearer "+customerToken)
		addReq.Header.Set("Content-Type", "application/json")
		addRec := httptest.NewRecorder()
		r.ServeHTTP(addRec, addReq)
		require.Equal(t, http.StatusCreated, addRec.Code)

		// Get Cart
		cartReq := httptest.NewRequest(http.MethodGet, "/api/customer/cart", nil)
		cartReq.Header.Set("Authorization", "Bearer "+customerToken)
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		require.Equal(t, http.StatusOK, cartRec.Code)

		var rawCart map[string]any
		err := json.Unmarshal(cartRec.Body.Bytes(), &rawCart)
		require.NoError(t, err)

		rawItems, ok := rawCart["items"].([]any)
		require.True(t, ok, "items array must exist in cart wire JSON")
		require.Len(t, rawItems, 1)

		item, ok := rawItems[0].(map[string]any)
		require.True(t, ok)

		assert.Equal(t, varRedLID.String(), item["productVariantId"])
		assert.Equal(t, prodID.String(), item["productId"])
		assert.Equal(t, "Худи Modern", item["title"])
		assert.Equal(t, "L", item["size"])
		assert.Equal(t, "Красный", item["color"])
		assert.Equal(t, redLSKU, item["sellerSku"])
		assert.Equal(t, redImgURL, item["imageUrl"])
		assert.Equal(t, true, item["inStock"])
		assert.Equal(t, float64(122200), item["priceCents"])
		assert.Equal(t, float64(1), item["quantity"])
	})

	// -------------------------------------------------------------
	// TEST C: Cart Zero-Stock Identity (buyability guard)
	// -------------------------------------------------------------
	t.Run("C. Adding out-of-stock variant (Red / XL) to cart is rejected", func(t *testing.T) {
		addPayload := fmt.Sprintf(`{"productId":"%s","productVariantId":"%s","quantity":1}`, prodID, varRedXLID)
		addReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader([]byte(addPayload)))
		addReq.Header.Set("Authorization", "Bearer "+customerToken)
		addReq.Header.Set("Content-Type", "application/json")
		addRec := httptest.NewRecorder()
		r.ServeHTTP(addRec, addReq)
		assert.Equal(t, http.StatusBadRequest, addRec.Code)
	})

	// -------------------------------------------------------------
	// TEST D: Order Snapshot
	// -------------------------------------------------------------
	t.Run("D. Order creation snapshots canonical variant_size, variant_color, sku, and image_url", func(t *testing.T) {
		orderPayload := fmt.Sprintf(`{
			"customerName": "Test Customer",
			"customerPhone": "+79991112233",
			"customerEmail": "%s",
			"deliveryAddress": "ул. Пушкина, д. 10",
			"deliveryMethodId": "%s"
		}`, customerEmail, deliveryMethodID)

		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader([]byte(orderPayload)))
		orderReq.Header.Set("Authorization", "Bearer "+customerToken)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		require.Equal(t, http.StatusCreated, orderRec.Code)

		var orderResp struct {
			ID uuid.UUID `json:"id"`
		}
		err := json.NewDecoder(orderRec.Body).Decode(&orderResp)
		require.NoError(t, err)
		createdOrderIDs = append(createdOrderIDs, orderResp.ID)

		// Inspect DB order_items directly
		var (
			pvID     uuid.UUID
			varSize  *string
			varColor *string
			sku      *string
			imgURL   *string
		)
		err = pgClient.Pool.QueryRow(ctx, `
			SELECT product_variant_id, variant_size, variant_color, sku, image_url
			FROM order_items
			WHERE order_id = $1
		`, orderResp.ID).Scan(&pvID, &varSize, &varColor, &sku, &imgURL)
		require.NoError(t, err)

		assert.Equal(t, varRedLID, pvID)
		require.NotNil(t, varSize, "order_item.variant_size must not be NULL")
		assert.Equal(t, "L", *varSize)
		require.NotNil(t, varColor, "order_item.variant_color must not be NULL")
		assert.Equal(t, "Красный", *varColor)
		require.NotNil(t, sku, "order_item.sku must not be NULL")
		assert.Equal(t, redLSKU, *sku)
		require.NotNil(t, imgURL, "order_item.image_url must not be NULL")
		assert.Equal(t, redImgURL, *imgURL, "order snapshot must record color-specific image")
	})

	// -------------------------------------------------------------
	// TEST E: Legacy Regression
	// -------------------------------------------------------------
	t.Run("E. Legacy textual variant resolves display fields, cart, and order snapshot correctly", func(t *testing.T) {
		legacyProdID := uuid.New()
		legacySlug := "pv-legacy-" + legacyProdID.String()[:8]

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				submitted_at, approved_at, published_at, created_at, updated_at
			) VALUES ($1, $2, $3, 'Vintage Wool Coat', $4, 800000, 'RUB', 'published', now(), now(), now(), now(), now())
		`, legacyProdID, sellerID, catID, legacySlug)
		require.NoError(t, err)
		createdProductIDs = append(createdProductIDs, legacyProdID)

		legacyGenImgURL := "http://minio.local/legacy-general.jpg"
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, color_id, created_at)
			VALUES ($1, $2, $3, 0, true, NULL, now())
		`, uuid.New(), legacyProdID, legacyGenImgURL)
		require.NoError(t, err)

		// Legacy row: size, color, sku as textual columns; color_id, size_value_id, seller_sku are NULL
		legacyVarID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (
				id, product_id, size, color, sku, barcode, price_cents, is_active, created_at, updated_at
			) VALUES ($1, $2, 'M', 'Graphite', 'LEGACY-SKU-M', $3, 800000, true, now(), now())
		`, legacyVarID, legacyProdID, "BC-"+legacyVarID.String()[:8])
		require.NoError(t, err)
		createdVariantIDs = append(createdVariantIDs, legacyVarID)

		// Stock = 2
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 2, 0, now(), now())
		`, uuid.New(), legacyProdID, legacyVarID, sellerID)
		require.NoError(t, err)

		// E.1 Public PDP Wire JSON
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public/products/%s", legacyProdID), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var rawPubResp map[string]any
		err = json.Unmarshal(rec.Body.Bytes(), &rawPubResp)
		require.NoError(t, err)

		rawPubVars, ok := rawPubResp["variants"].([]any)
		require.True(t, ok)
		require.Len(t, rawPubVars, 1)

		legVarMap, ok := rawPubVars[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "M", legVarMap["size"])
		assert.Equal(t, "Graphite", legVarMap["color"])
		assert.Equal(t, "Graphite", legVarMap["colorName"])
		assert.Equal(t, "LEGACY-SKU-M", legVarMap["sellerSku"])
		assert.Equal(t, "LEGACY-SKU-M", legVarMap["sku"])
		assert.Equal(t, true, legVarMap["inStock"])
		assert.Equal(t, true, legVarMap["isActive"])

		// E.2 Cart
		// Customer 2 for clean cart
		cust2ID := uuid.New()
		cust2Email := "pv-cust2-" + cust2ID.String()[:8] + "@test.com"
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Legacy Customer', 'hash', 'customer', 'active', now(), now())
		`, cust2ID, cust2Email, "+7992"+cust2ID.String()[:7])
		require.NoError(t, err)
		createdUserIDs = append(createdUserIDs, cust2ID)

		cust2Token, err := tokenService.GenerateAccessToken(cust2ID, cust2Email, "customer")
		require.NoError(t, err)

		addPayload := fmt.Sprintf(`{"productId":"%s","productVariantId":"%s","quantity":1}`, legacyProdID, legacyVarID)
		addReq := httptest.NewRequest(http.MethodPost, "/api/customer/cart/items", bytes.NewReader([]byte(addPayload)))
		addReq.Header.Set("Authorization", "Bearer "+cust2Token)
		addReq.Header.Set("Content-Type", "application/json")
		addRec := httptest.NewRecorder()
		r.ServeHTTP(addRec, addReq)
		require.Equal(t, http.StatusCreated, addRec.Code)

		cartReq := httptest.NewRequest(http.MethodGet, "/api/customer/cart", nil)
		cartReq.Header.Set("Authorization", "Bearer "+cust2Token)
		cartRec := httptest.NewRecorder()
		r.ServeHTTP(cartRec, cartReq)
		require.Equal(t, http.StatusOK, cartRec.Code)

		var rawCartResp map[string]any
		err = json.Unmarshal(cartRec.Body.Bytes(), &rawCartResp)
		require.NoError(t, err)

		rawCartItems, ok := rawCartResp["items"].([]any)
		require.True(t, ok)
		require.Len(t, rawCartItems, 1)

		itemMap, ok := rawCartItems[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "M", itemMap["size"])
		assert.Equal(t, "Graphite", itemMap["color"])
		assert.Equal(t, "LEGACY-SKU-M", itemMap["sellerSku"])
		assert.Equal(t, legacyGenImgURL, itemMap["imageUrl"])
		assert.Equal(t, true, itemMap["inStock"])

		// E.3 Order
		orderPayload := fmt.Sprintf(`{
			"customerName": "Legacy Customer",
			"customerPhone": "+79998887766",
			"customerEmail": "%s",
			"deliveryAddress": "ул. Чехова, д. 5",
			"deliveryMethodId": "%s"
		}`, cust2Email, deliveryMethodID)

		orderReq := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader([]byte(orderPayload)))
		orderReq.Header.Set("Authorization", "Bearer "+cust2Token)
		orderReq.Header.Set("Content-Type", "application/json")
		orderRec := httptest.NewRecorder()
		r.ServeHTTP(orderRec, orderReq)
		require.Equal(t, http.StatusCreated, orderRec.Code)

		var orderResp struct {
			ID uuid.UUID `json:"id"`
		}
		err = json.NewDecoder(orderRec.Body).Decode(&orderResp)
		require.NoError(t, err)
		createdOrderIDs = append(createdOrderIDs, orderResp.ID)

		var (
			pvID     uuid.UUID
			varSize  *string
			varColor *string
			sku      *string
			imgURL   *string
		)
		err = pgClient.Pool.QueryRow(ctx, `
			SELECT product_variant_id, variant_size, variant_color, sku, image_url
			FROM order_items
			WHERE order_id = $1
		`, orderResp.ID).Scan(&pvID, &varSize, &varColor, &sku, &imgURL)
		require.NoError(t, err)

		assert.Equal(t, legacyVarID, pvID)
		require.NotNil(t, varSize)
		assert.Equal(t, "M", *varSize)
		require.NotNil(t, varColor)
		assert.Equal(t, "Graphite", *varColor)
		require.NotNil(t, imgURL)
		assert.Equal(t, legacyGenImgURL, *imgURL)
	})
}
