package selleranalytics

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercises GET /products through the real Handler, Service and Repository.
// setupTestDB refuses to run unless current_database() is 'zamk_test'.
func TestProductsHTTP_PaginationValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	handler := NewHandler(NewService(repo), repo)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sellerID := insertSeller(t, db)
	var userID uuid.UUID
	require.NoError(t, db.QueryRow(context.Background(), "SELECT user_id FROM seller_users WHERE seller_id = $1", sellerID).Scan(&userID))

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		pID, vID := insertProduct(t, db, sellerID)
		insertOrderWithLedger(t, db, sellerID, pID, vID, int64(100000*(i+1)), 8000, int64(92000*(i+1)), 1, true, now)
	}

	base := "/products?from=" + now.Add(-24*time.Hour).Format(time.RFC3339) +
		"&to=" + now.Add(24*time.Hour).Format(time.RFC3339)

	get := func(extra string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", base+extra, nil)
		req = req.WithContext(context.WithValue(req.Context(), "userID", userID))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	decode := func(t *testing.T, rec *httptest.ResponseRecorder) ProductsResponse {
		var resp ProductsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp
	}

	invalid := map[string]string{
		"limit=-1":     "&limit=-1",
		"offset=-1":    "&offset=-1",
		"limit=abc":    "&limit=abc",
		"offset=abc":   "&offset=abc",
		"limit=1.5":    "&limit=1.5",
		"limit=99..99": "&limit=99999999999999999999",
	}
	for name, q := range invalid {
		t.Run("400 for "+name, func(t *testing.T) {
			assert.Equal(t, http.StatusBadRequest, get(q).Code)
		})
	}

	t.Run("limit=50 returns the page", func(t *testing.T) {
		rec := get("&limit=50")
		require.Equal(t, http.StatusOK, rec.Code)
		resp := decode(t, rec)
		assert.Len(t, resp.Items, 3)
		assert.Equal(t, 3, resp.TotalCount)
	})

	t.Run("defaults without limit/offset", func(t *testing.T) {
		rec := get("")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Len(t, decode(t, rec).Items, 3)
	})

	t.Run("limit=1 offset=1 returns a single item", func(t *testing.T) {
		rec := get("&limit=1&offset=1")
		require.Equal(t, http.StatusOK, rec.Code)
		resp := decode(t, rec)
		assert.Len(t, resp.Items, 1)
		assert.Equal(t, 3, resp.TotalCount)
	})

	t.Run("limit=0 keeps empty page behaviour", func(t *testing.T) {
		rec := get("&limit=0")
		require.Equal(t, http.StatusOK, rec.Code)
		resp := decode(t, rec)
		assert.Empty(t, resp.Items)
		assert.Equal(t, 3, resp.TotalCount)
	})

	t.Run("large valid offset returns empty page", func(t *testing.T) {
		rec := get("&offset=1000000")
		require.Equal(t, http.StatusOK, rec.Code)
		resp := decode(t, rec)
		assert.Empty(t, resp.Items)
		assert.Equal(t, 3, resp.TotalCount)
	})

	t.Run("offset+limit overflow returns empty page", func(t *testing.T) {
		rec := get("&limit=50&offset=" + strconv.Itoa(math.MaxInt))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, decode(t, rec).Items)

		rec = get("&limit=" + strconv.Itoa(math.MaxInt) + "&offset=" + strconv.Itoa(math.MaxInt))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, decode(t, rec).Items)
	})
}
