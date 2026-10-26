package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCartAddItemVariantErrors pins the HTTP contract of POST /api/customer/cart/items:
// a variant that does not exist, is inactive or belongs to another product is rejected with 4xx
// (never 500, never a leaked internal error), stock limits still apply, and rejected SKUs never reach the cart.
func TestCartAddItemVariantErrors(t *testing.T) {
	ctx, r, cleanup, pgClient, tokenService := setupProductPublicationTestEnv(t)
	defer cleanup()

	var (
		sellerIDs, categoryIDs, productIDs, variantIDs, userIDs []uuid.UUID
	)
	t.Cleanup(func() {
		if testutil.VerifyTestDatabase(ctx, pgClient.Pool) != nil {
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
	inactiveVariant := newVariant(productA, false, 5)
	emptyVariant := newVariant(productA, true, 0)
	foreignVariant := newVariant(productB, true, 5)

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
	cartQty := func() map[string]int {
		w := call(http.MethodGet, "/api/customer/cart", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Items []struct {
				ProductVariantID string `json:"productVariantId"`
				Quantity         int    `json:"quantity"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		out := map[string]int{}
		for _, it := range resp.Items {
			out[it.ProductVariantID] = it.Quantity
		}
		return out
	}
	requireRejected := func(w *httptest.ResponseRecorder, wantCode string) {
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		var resp struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, wantCode, resp.Error.Code)
		body := strings.ToLower(w.Body.String())
		for _, leak := range []string{"sql", "pgx", "no rows", "violates", "relation "} {
			assert.NotContains(t, body, leak)
		}
	}

	t.Run("valid variant is added", func(t *testing.T) {
		w := add(productA, okVariant, 1)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		assert.Equal(t, 1, cartQty()[okVariant.String()])
	})

	t.Run("repeat add accumulates one line, stock still enforced", func(t *testing.T) {
		require.Equal(t, http.StatusCreated, add(productA, okVariant, 2).Code)
		assert.Equal(t, 3, cartQty()[okVariant.String()])
		requireRejected(add(productA, okVariant, 3), "invalid_item") // 3+3 > 5
		assert.Equal(t, 3, cartQty()[okVariant.String()])
	})

	t.Run("nonexistent variant is 4xx not 500", func(t *testing.T) {
		requireRejected(add(productA, uuid.New(), 1), "invalid_item")
	})

	t.Run("inactive variant is rejected", func(t *testing.T) {
		requireRejected(add(productA, inactiveVariant, 1), "invalid_item")
		assert.NotContains(t, cartQty(), inactiveVariant.String())
	})

	t.Run("variant of another product is rejected", func(t *testing.T) {
		requireRejected(add(productA, foreignVariant, 1), "invalid_item")
		assert.NotContains(t, cartQty(), foreignVariant.String())
	})

	t.Run("insufficient stock is rejected", func(t *testing.T) {
		requireRejected(add(productA, emptyVariant, 1), "invalid_item")
		assert.NotContains(t, cartQty(), emptyVariant.String())
	})

	t.Run("nonexistent product is 4xx not 500", func(t *testing.T) {
		requireRejected(add(uuid.New(), okVariant, 1), "invalid_item")
	})

	t.Run("invalid input is 400", func(t *testing.T) {
		w := add(productA, okVariant, 0)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
