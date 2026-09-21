package products_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestProductIdempotency_Handler(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Guard test DB
	var dbName string
	err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName)

	var mu sync.Mutex
	var createdProductIDs []uuid.UUID
	t.Cleanup(func() {
		mu.Lock()
		ids := make([]uuid.UUID, len(createdProductIDs))
		copy(ids, createdProductIDs)
		mu.Unlock()
		if len(ids) == 0 {
			return
		}
		queries := []string{
			"DELETE FROM variant_attribute_values WHERE product_variant_id IN (SELECT id FROM product_variants WHERE product_id = ANY($1))",
			"DELETE FROM product_variants WHERE product_id = ANY($1)",
			"DELETE FROM product_attribute_values WHERE product_id = ANY($1)",
			"DELETE FROM product_material_composition WHERE product_id = ANY($1)",
			"DELETE FROM product_size_chart_rows WHERE size_chart_id IN (SELECT id FROM product_size_charts WHERE product_id = ANY($1))",
			"DELETE FROM product_size_charts WHERE product_id = ANY($1)",
			"DELETE FROM product_images WHERE product_id = ANY($1)",
			"DELETE FROM product_revisions WHERE product_id = ANY($1)",
			"DELETE FROM products WHERE id = ANY($1)",
		}
		for _, q := range queries {
			if _, err := pool.Exec(context.Background(), q, ids); err != nil {
				t.Errorf("cleanup failed for query %s: %v", q, err)
			}
		}
	})

	catID := uuid.New()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID); err != nil {
			t.Errorf("failed to clean up category %s: %v", catID, err)
		}
	})

	// Execute category insert AFTER cleanup registration
	_, err = pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Handler Test Cat', $2)", catID, "handler-cat-"+uuid.New().String())
	require.NoError(t, err)

	handler := products.NewHandler(svc, nil)

	t.Run("malformed Idempotency-Key returns 400 invalid_idempotency_key", func(t *testing.T) {
		reqBody := products.CreateProductRequest{
			Title:      "Malformed Key Test",
			CategoryID: &catID,
			PriceCents: 1500,
			Currency:   "RUB",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		httpReq := httptest.NewRequest(http.MethodPost, "/api/seller/products", bytes.NewReader(bodyBytes))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Idempotency-Key", "not-a-valid-uuid")
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.CreateProduct(rec, httpReq)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		errObj, ok := resp["error"].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "invalid_idempotency_key", errObj["code"])
	})

	t.Run("missing key preserves legacy create behavior", func(t *testing.T) {
		reqBody := products.CreateProductRequest{
			Title:      "Legacy No-Key Test",
			CategoryID: &catID,
			PriceCents: 1500,
			Currency:   "RUB",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		httpReq := httptest.NewRequest(http.MethodPost, "/api/seller/products", bytes.NewReader(bodyBytes))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), "userID", sellerUserID))

		rec := httptest.NewRecorder()
		handler.CreateProduct(rec, httpReq)

		require.Equal(t, http.StatusCreated, rec.Code)
		var prod products.Product
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &prod))
		require.NotEqual(t, uuid.Nil, prod.ID)

		mu.Lock()
		createdProductIDs = append(createdProductIDs, prod.ID)
		mu.Unlock()
	})

	t.Run("same key with different payload returns 409 idempotency_conflict", func(t *testing.T) {
		idemKey := uuid.New()
		reqBody1 := products.CreateProductRequest{
			Title:      "Idem Conflict Test 1",
			CategoryID: &catID,
			PriceCents: 1500,
			Currency:   "RUB",
		}
		bodyBytes1, _ := json.Marshal(reqBody1)

		httpReq1 := httptest.NewRequest(http.MethodPost, "/api/seller/products", bytes.NewReader(bodyBytes1))
		httpReq1.Header.Set("Content-Type", "application/json")
		httpReq1.Header.Set("Idempotency-Key", idemKey.String())
		httpReq1 = httpReq1.WithContext(context.WithValue(httpReq1.Context(), "userID", sellerUserID))

		rec1 := httptest.NewRecorder()
		handler.CreateProduct(rec1, httpReq1)
		require.Equal(t, http.StatusCreated, rec1.Code)
		var prod1 products.Product
		require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &prod1))

		mu.Lock()
		createdProductIDs = append(createdProductIDs, prod1.ID)
		mu.Unlock()

		// Call 2 with different payload and same key
		reqBody2 := products.CreateProductRequest{
			Title:      "Idem Conflict Test 2 Differing",
			CategoryID: &catID,
			PriceCents: 2500,
			Currency:   "RUB",
		}
		bodyBytes2, _ := json.Marshal(reqBody2)

		httpReq2 := httptest.NewRequest(http.MethodPost, "/api/seller/products", bytes.NewReader(bodyBytes2))
		httpReq2.Header.Set("Content-Type", "application/json")
		httpReq2.Header.Set("Idempotency-Key", idemKey.String())
		httpReq2 = httpReq2.WithContext(context.WithValue(httpReq2.Context(), "userID", sellerUserID))

		rec2 := httptest.NewRecorder()
		handler.CreateProduct(rec2, httpReq2)

		require.Equal(t, http.StatusConflict, rec2.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp))
		errObj, ok := resp["error"].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "idempotency_conflict", errObj["code"])

		// Ensure no sensitive hash or seller info is leaked
		bodyStr := rec2.Body.String()
		require.NotContains(t, bodyStr, sellerUserID.String())
		require.NotContains(t, bodyStr, "hash")
	})
}
