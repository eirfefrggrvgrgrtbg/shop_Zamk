package favorites_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/favorites"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (*postgres.Client, *favorites.Handler) {
	if testing.Short() {
		t.Skip("integration test")
	}
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	}
	require.True(t, strings.Contains(dbURL, "zamk_test"), "MUST run only against zamk_test")
	client, err := postgres.NewClient(context.Background(), dbURL)
	require.NoError(t, err)
	var name string
	require.NoError(t, client.Pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&name))
	require.Equal(t, "zamk_test", name)
	h := favorites.NewHandler(favorites.NewService(favorites.NewRepository(client.Pool)))
	return client, h
}

func mkUser(t *testing.T, c *postgres.Client) uuid.UUID {
	id := uuid.New()
	_, err := c.Pool.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, name, role, status) VALUES ($1, $2, 'x', 'Fav Test', 'customer', 'active')`,
		id, id.String()+"@fav.test")
	require.NoError(t, err)
	t.Cleanup(func() { c.Pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", id) })
	return id
}

func mkBrand(t *testing.T, c *postgres.Client, active bool) uuid.UUID {
	id := uuid.New()
	_, err := c.Pool.Exec(context.Background(),
		`INSERT INTO brands (id, name, slug, is_active) VALUES ($1, $2, $3, $4)`,
		id, "FavBrand "+id.String()[:6], "fav-"+id.String()[:8], active)
	require.NoError(t, err)
	t.Cleanup(func() { c.Pool.Exec(context.Background(), "DELETE FROM brands WHERE id=$1", id) })
	return id
}

func call(h *favorites.Handler, method, path string, user *uuid.UUID) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/favorite-brands", h.ListFavoriteBrands)
	r.Post("/favorite-brands/{brandId}", h.AddFavoriteBrand)
	r.Delete("/favorite-brands/{brandId}", h.RemoveFavoriteBrand)
	req := httptest.NewRequest(method, path, nil)
	if user != nil {
		req = req.WithContext(context.WithValue(req.Context(), "userID", *user))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type listResp struct {
	Items []map[string]any `json:"items"`
}

func list(t *testing.T, h *favorites.Handler, u uuid.UUID) listResp {
	w := call(h, http.MethodGet, "/favorite-brands", &u)
	require.Equal(t, 200, w.Code)
	var out listResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func TestFavoriteBrands(t *testing.T) {
	c, h := setup(t)
	defer c.Close()
	a, b := mkUser(t, c), mkUser(t, c)
	brand := mkBrand(t, c, true)
	path := "/favorite-brands/" + brand.String()

	t.Run("anonymous is rejected on every route", func(t *testing.T) {
		require.Equal(t, 401, call(h, http.MethodGet, "/favorite-brands", nil).Code)
		require.Equal(t, 401, call(h, http.MethodPost, path, nil).Code)
		require.Equal(t, 401, call(h, http.MethodDelete, path, nil).Code)
	})

	t.Run("add, repeat add, list, isolation, remove", func(t *testing.T) {
		require.Equal(t, 200, call(h, http.MethodPost, path, &a).Code)
		require.Equal(t, 200, call(h, http.MethodPost, path, &a).Code)
		var n int
		require.NoError(t, c.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM favorite_brands WHERE user_id=$1 AND brand_id=$2`, a, brand).Scan(&n))
		require.Equal(t, 1, n)

		got := list(t, h, a)
		require.Len(t, got.Items, 1)
		require.Equal(t, brand.String(), got.Items[0]["id"])
		_, leaked := got.Items[0]["sellerId"]
		require.False(t, leaked)
		require.Empty(t, list(t, h, b).Items, "other buyer sees nothing")

		// another buyer's DELETE must not touch a's row
		require.Equal(t, 200, call(h, http.MethodDelete, path, &b).Code)
		require.Len(t, list(t, h, a).Items, 1)

		require.Equal(t, 200, call(h, http.MethodDelete, path, &a).Code)
		require.Equal(t, 200, call(h, http.MethodDelete, path, &a).Code, "idempotent delete")
		require.Empty(t, list(t, h, a).Items)
	})

	t.Run("invalid, unknown and inactive brands", func(t *testing.T) {
		require.Equal(t, 400, call(h, http.MethodPost, "/favorite-brands/not-a-uuid", &a).Code)
		require.Equal(t, 404, call(h, http.MethodPost, "/favorite-brands/"+uuid.NewString(), &a).Code)
		inactive := mkBrand(t, c, false)
		require.Equal(t, 404, call(h, http.MethodPost, "/favorite-brands/"+inactive.String(), &a).Code)
	})

	t.Run("brand deactivated later disappears from list; deleted brand cascades", func(t *testing.T) {
		br := mkBrand(t, c, true)
		p := "/favorite-brands/" + br.String()
		require.Equal(t, 200, call(h, http.MethodPost, p, &a).Code)
		require.Len(t, list(t, h, a).Items, 1)
		_, err := c.Pool.Exec(context.Background(), `UPDATE brands SET is_active=false WHERE id=$1`, br)
		require.NoError(t, err)
		require.Empty(t, list(t, h, a).Items)
		_, err = c.Pool.Exec(context.Background(), `DELETE FROM brands WHERE id=$1`, br)
		require.NoError(t, err)
		var n int
		require.NoError(t, c.Pool.QueryRow(context.Background(), `SELECT count(*) FROM favorite_brands WHERE brand_id=$1`, br).Scan(&n))
		require.Equal(t, 0, n)
	})

	t.Run("concurrent adds create exactly one row", func(t *testing.T) {
		br := mkBrand(t, c, true)
		p := "/favorite-brands/" + br.String()
		var wg sync.WaitGroup
		codes := make([]int, 20)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codes[i] = call(h, http.MethodPost, p, &b).Code
			}(i)
		}
		wg.Wait()
		for _, code := range codes {
			require.Equal(t, 200, code)
		}
		var n int
		require.NoError(t, c.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM favorite_brands WHERE user_id=$1 AND brand_id=$2`, b, br).Scan(&n))
		require.Equal(t, 1, n)
	})
}
