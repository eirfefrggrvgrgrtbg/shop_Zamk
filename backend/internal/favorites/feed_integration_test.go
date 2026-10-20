package favorites_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/favorites"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
)

func mkSeller(t *testing.T, c *postgres.Client, status string) uuid.UUID {
	id := uuid.New()
	_, err := c.Pool.Exec(context.Background(), `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, 't@t.com', $4, now(), now())`, id, "FeedSeller "+id.String()[:6], "fs-"+id.String()[:8], status)
	require.NoError(t, err)
	t.Cleanup(func() { c.Pool.Exec(context.Background(), "DELETE FROM sellers WHERE id=$1", id) })
	return id
}

// mkProduct inserts a product of brand with the given status and free stock; ageMin orders published_at.
func mkProduct(t *testing.T, c *postgres.Client, seller, brand uuid.UUID, status string, stock, ageMin int) uuid.UUID {
	ctx := context.Background()
	id := uuid.New()
	_, err := c.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, brand_id, title, slug, status, source, currency, price_cents, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'Feed item', $4, $5, 'draft', 'RUB', 1000, $6, $6, now())`,
		id, seller, brand, id.String(), status, time.Now().Add(-time.Duration(ageMin)*time.Minute))
	require.NoError(t, err)
	t.Cleanup(func() { c.Pool.Exec(ctx, "DELETE FROM products WHERE id=$1", id) })
	if stock > 0 {
		v := uuid.New()
		_, err = c.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, is_active) VALUES ($1, $2, true)", v, id)
		require.NoError(t, err)
		_, err = c.Pool.Exec(ctx, "INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES ($1,$2,$3,$4,$5,0)", uuid.New(), id, seller, v, stock)
		require.NoError(t, err)
	}
	return id
}

type feedResp struct {
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
	TotalCount int    `json:"totalCount"`
	Order      string `json:"order"`
}

func feed(t *testing.T, h *favorites.Handler, u *uuid.UUID, query string) (int, feedResp) {
	r := chi.NewRouter()
	r.Get("/favorite-brands/products", h.ListFavoriteBrandProducts)
	req := httptest.NewRequest(http.MethodGet, "/favorite-brands/products"+query, nil)
	if u != nil {
		req = req.WithContext(context.WithValue(req.Context(), "userID", *u))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out feedResp
	if w.Code == 200 {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	}
	return w.Code, out
}

func ids(f feedResp) []string {
	out := []string{}
	for _, i := range f.Items {
		out = append(out, i.ID)
	}
	return out
}

func TestFavoriteBrandFeed(t *testing.T) {
	c, _ := setup(t)
	defer c.Close()
	svc := favorites.NewService(favorites.NewRepository(c.Pool)).
		WithProducts(products.NewService(products.NewRepository(c.Pool), sellers.NewRepository(c.Pool), c, nil, nil))
	h := favorites.NewHandler(svc)

	a, b, other := mkUser(t, c), mkUser(t, c), mkUser(t, c)
	brandA, brandB, brandC := mkBrand(t, c, true), mkBrand(t, c, true), mkBrand(t, c, true)
	brandOff := mkBrand(t, c, true)
	active, blocked := mkSeller(t, c, "active"), mkSeller(t, c, "blocked")

	a1 := mkProduct(t, c, active, brandA, "published", 5, 1) // newest
	b1 := mkProduct(t, c, active, brandB, "published", 5, 2)
	a2 := mkProduct(t, c, active, brandA, "published", 5, 3)
	b2 := mkProduct(t, c, active, brandB, "published", 5, 4)
	a3 := mkProduct(t, c, active, brandA, "published", 5, 5)
	mkProduct(t, c, active, brandA, "draft", 5, 0)                  // unpublished
	mkProduct(t, c, active, brandA, "published", 0, 0)              // no stock
	mkProduct(t, c, active, brandA, "published", 1, 0)              // below storefront minimum
	mkProduct(t, c, blocked, brandA, "published", 5, 0)             // seller not active
	mkProduct(t, c, active, brandC, "published", 5, 0)              // not a favorite
	offProd := mkProduct(t, c, active, brandOff, "published", 5, 0) // newest of all

	for _, br := range []uuid.UUID{brandA, brandB, brandOff} {
		require.Equal(t, 200, call(h, http.MethodPost, "/favorite-brands/"+br.String(), &a).Code)
	}
	require.Equal(t, 200, call(h, http.MethodPost, "/favorite-brands/"+brandC.String(), &other).Code)

	t.Run("anonymous is rejected", func(t *testing.T) {
		code, _ := feed(t, h, nil, "")
		require.Equal(t, 401, code)
	})

	t.Run("buyer without favorites gets an empty feed", func(t *testing.T) {
		code, f := feed(t, h, &b, "")
		require.Equal(t, 200, code)
		require.Empty(t, f.Items)
		require.Equal(t, 0, f.TotalCount)
	})

	t.Run("multi-brand feed, eligible only, newest first", func(t *testing.T) {
		_, f := feed(t, h, &a, "")
		require.Equal(t, []string{offProd.String(), a1.String(), b1.String(), a2.String(), b2.String(), a3.String()}, ids(f))
		require.Equal(t, 6, f.TotalCount)
		require.Equal(t, "catalog_default", f.Order)
	})

	t.Run("deactivated brand leaves the feed", func(t *testing.T) {
		_, err := c.Pool.Exec(context.Background(), `UPDATE brands SET is_active=false WHERE id=$1`, brandOff)
		require.NoError(t, err)
		_, f := feed(t, h, &a, "")
		require.Equal(t, 5, f.TotalCount)
		require.NotContains(t, ids(f), offProd.String())
	})

	t.Run("pagination has no gaps or duplicates", func(t *testing.T) {
		seen := map[string]bool{}
		var all []string
		for off := 0; off < 6; off += 2 {
			_, f := feed(t, h, &a, fmt.Sprintf("?limit=2&offset=%d", off))
			require.Equal(t, 5, f.TotalCount)
			for _, id := range ids(f) {
				require.False(t, seen[id], "duplicate %s", id)
				seen[id] = true
				all = append(all, id)
			}
		}
		require.Equal(t, []string{a1.String(), b1.String(), a2.String(), b2.String(), a3.String()}, all)
	})

	t.Run("other buyer's favorites are not visible", func(t *testing.T) {
		_, f := feed(t, h, &other, "")
		require.Equal(t, 1, f.TotalCount) // only the single brandC product
		for _, id := range ids(f) {
			require.NotContains(t, []string{a1.String(), b1.String(), a2.String()}, id)
		}
	})

	t.Run("removing a brand removes its products", func(t *testing.T) {
		require.Equal(t, 200, call(h, http.MethodDelete, "/favorite-brands/"+brandB.String(), &a).Code)
		_, f := feed(t, h, &a, "")
		require.Equal(t, []string{a1.String(), a2.String(), a3.String()}, ids(f))
	})
}
