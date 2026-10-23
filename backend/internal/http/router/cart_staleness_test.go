package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCartStaleness pins the HTTP contract of POST /api/customer/cart/items:
// a variant that does not exist, is inactive or belongs to another product is rejected with 4xx
// (never 500, never a leaked internal error), stock limits still apply, and rejected SKUs never reach the cart.
func TestCartStaleness(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupProductPublicationTestEnv(t)
	defer cleanup()

	var (
		sellerIDs, categoryIDs, productIDs, variantIDs, userIDs []uuid.UUID
	)
	t.Cleanup(func() {
		var dbName string
		if err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err != nil || dbName != "zamk_test" {
			return
		}
		if len(userIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM cart_items WHERE cart_id IN (SELECT id FROM carts WHERE user_id = ANY($1))", userIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM carts WHERE user_id = ANY($1)", userIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", userIDs)
		}
		if len(variantIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_variant_id = ANY($1)", variantIDs)
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE id = ANY($1)", variantIDs)
		}
		if len(productIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", productIDs)
		}
		if len(categoryIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", categoryIDs)
		}
		if len(sellerIDs) > 0 {
			_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", sellerIDs)
		}
	})

	sellerID := uuid.New()
	slug := "cart-err-" + sellerID.String()[:8]
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Cart Err Brand', $2, $3, 'active', now(), now())
	`, sellerID, slug, slug+"@test.com")
	require.NoError(t, err)
	sellerIDs = append(sellerIDs, sellerID)

	catID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Cart Err Cat', $2, true, now(), now())
	`, catID, "cart-err-cat-"+catID.String()[:8])
	require.NoError(t, err)
	categoryIDs = append(categoryIDs, catID)

	newProduct := func(title string) uuid.UUID {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, currency, status, main_image_url)
			VALUES ($1, $2, $3, $4, $5, 10000, 'RUB', 'published', 'https://cdn.zamk.test/m.jpg')
		`, id, sellerID, catID, title, "cart-err-"+id.String()[:8])
		require.NoError(t, err)
		productIDs = append(productIDs, id)
		return id
	}
	newVariant := func(productID uuid.UUID, active bool, stock int) uuid.UUID {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 10000, $5, now(), now())
		`, id, productID, "SKU-ERR-"+id.String()[:8], "BC-"+id.String()[:8], active)
		require.NoError(t, err)
		variantIDs = append(variantIDs, id)
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 0, now(), now())
		`, uuid.New(), productID, id, sellerID, stock)
		require.NoError(t, err)
		return id
	}

	productA := newProduct("Cart Err A")
	productB := newProduct("Cart Err B")
	okVariant := newVariant(productA, true, 5)

	userID := uuid.New()
	email := fmt.Sprintf("cart-err-%s@test.com", userID.String()[:8])
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Cart Err', 'hash', 'customer', 'active', now(), now())
	`, userID, email, "+7998"+userID.String()[:7])
	require.NoError(t, err)
	userIDs = append(userIDs, userID)
	token, err := tokenService.GenerateAccessToken(userID, email, "customer")
	require.NoError(t, err)

	call := func(method, path string, body any) *httptest.ResponseRecorder {
		var rd *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		} else {
			rd = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, path, rd)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	add := func(productID, variantID uuid.UUID, qty int) *httptest.ResponseRecorder {
		return call(http.MethodPost, "/api/customer/cart/items", map[string]any{
			"productId": productID.String(), "productVariantId": variantID.String(), "quantity": qty,
		})
	}
	type line struct {
		ProductVariantID  string `json:"productVariantId"`
		Quantity          int    `json:"quantity"`
		PriceCents        int64  `json:"priceCents"`
		InStock           bool   `json:"inStock"`
		AvailableQuantity int    `json:"availableQuantity"`
		UnavailableReason string `json:"unavailableReason"`
		ID                string `json:"id"`
	}
	getLine := func(variantID uuid.UUID) line {
		w := call(http.MethodGet, "/api/customer/cart", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Items []line `json:"items"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		for _, it := range resp.Items {
			if it.ProductVariantID == variantID.String() {
				return it
			}
		}
		t.Fatalf("variant %s not in cart", variantID)
		return line{}
	}
	reserved := func(variantID uuid.UUID) int {
		var n int
		require.NoError(t, pgClient.Pool.QueryRow(ctx, "SELECT reserved_stock FROM inventory_items WHERE product_variant_id=$1", variantID).Scan(&n))
		return n
	}
	exec := func(q string, args ...any) {
		_, err := pgClient.Pool.Exec(ctx, q, args...)
		require.NoError(t, err)
	}

	require.Equal(t, http.StatusCreated, add(productA, okVariant, 3).Code)
	item := getLine(okVariant)

	t.Run("valid cart has no unavailable reason; viewing reserves nothing", func(t *testing.T) {
		l := getLine(okVariant)
		assert.Empty(t, l.UnavailableReason)
		assert.True(t, l.InStock)
		assert.Equal(t, 5, l.AvailableQuantity)
		getLine(okVariant)
		assert.Equal(t, 0, reserved(okVariant))
	})

	t.Run("variant deactivated then restored", func(t *testing.T) {
		exec("UPDATE product_variants SET is_active=false WHERE id=$1", okVariant)
		l := getLine(okVariant)
		assert.Equal(t, "variant_inactive", l.UnavailableReason)
		assert.False(t, l.InStock)
		assert.Equal(t, 3, l.Quantity, "line must stay visible, never removed silently")

		// raising is refused by the server, lowering is allowed
		w := call(http.MethodPatch, "/api/customer/cart/items/"+item.ID, map[string]any{"quantity": 4})
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		w = call(http.MethodPatch, "/api/customer/cart/items/"+item.ID, map[string]any{"quantity": 2})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, 2, getLine(okVariant).Quantity)

		exec("UPDATE product_variants SET is_active=true WHERE id=$1", okVariant)
		assert.Empty(t, getLine(okVariant).UnavailableReason)
	})

	t.Run("product unpublished then republished", func(t *testing.T) {
		exec("UPDATE products SET status='draft' WHERE id=$1", productA)
		l := getLine(okVariant)
		assert.Equal(t, "product_unpublished", l.UnavailableReason)
		w := call(http.MethodPatch, "/api/customer/cart/items/"+item.ID, map[string]any{"quantity": 3})
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		// direct API add of the same SKU is refused too
		assert.Equal(t, http.StatusBadRequest, add(productA, okVariant, 1).Code)
		exec("UPDATE products SET status='published' WHERE id=$1", productA)
		assert.Empty(t, getLine(okVariant).UnavailableReason)
	})

	t.Run("stock drops to zero and below cart quantity", func(t *testing.T) {
		exec("UPDATE inventory_items SET total_stock=0 WHERE product_variant_id=$1", okVariant)
		l := getLine(okVariant)
		assert.Equal(t, "out_of_stock", l.UnavailableReason)
		assert.False(t, l.InStock)
		assert.Equal(t, 0, l.AvailableQuantity)

		exec("UPDATE inventory_items SET total_stock=1 WHERE product_variant_id=$1", okVariant)
		l = getLine(okVariant)
		assert.Equal(t, "insufficient_stock", l.UnavailableReason)
		assert.Equal(t, 1, l.AvailableQuantity)
		assert.Equal(t, 2, l.Quantity)

		// buyer lowers the quantity to what is available -> line becomes purchasable
		w := call(http.MethodPatch, "/api/customer/cart/items/"+item.ID, map[string]any{"quantity": 1})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Empty(t, getLine(okVariant).UnavailableReason)

		exec("UPDATE inventory_items SET total_stock=5 WHERE product_variant_id=$1", okVariant)
		assert.Empty(t, getLine(okVariant).UnavailableReason)
		assert.Equal(t, 0, reserved(okVariant))
	})

	t.Run("price change is served live, reopening shows the current price", func(t *testing.T) {
		before := getLine(okVariant).PriceCents
		exec("UPDATE product_variants SET price_cents=$2 WHERE id=$1", okVariant, before+2500)
		assert.Equal(t, before+2500, getLine(okVariant).PriceCents)
		assert.Equal(t, before+2500, getLine(okVariant).PriceCents) // second open is stable
	})

	t.Run("variant moved to another product is refused on update", func(t *testing.T) {
		exec("UPDATE cart_items SET product_id=$2 WHERE id=$1", item.ID, productB)
		w := call(http.MethodPatch, "/api/customer/cart/items/"+item.ID, map[string]any{"quantity": 2})
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.NotContains(t, strings.ToLower(w.Body.String()), "sql")
	})
}
