package returns

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-chi/chi/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
)

func setupHTTPFixture(t *testing.T) (*postgres.Client, *Service) {
	ctx := context.Background()
	client, err := postgres.NewClient(ctx, "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable")
	require.NoError(t, err)

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	
	payRepo := payments.NewRepository(client.Pool)
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	paySvc := payments.NewService(payRepo, ordersRepo, nil, nil, client, nil, cfg)
	svc := NewService(repo, ordersRepo, nil, client, nil, paySvc, 30, nil)

	return client, svc
}

func TestAdminRefundReserve(t *testing.T) {
	client, svc := setupHTTPFixture(t)
	ctx := context.Background()

	h := NewHandler(svc)
	r := chi.NewRouter()
	
	// Need a payment fixture
	fix := payments.SetupFixture(t, client, "succeeded", "paid", 100000, false, "")

	// Inject fake userID for the admin/user
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqCtx := context.WithValue(r.Context(), "userID", fix.UserID)
			next.ServeHTTP(w, r.WithContext(reqCtx))
		})
	})
	r.Post("/api/admin/returns/{id}/refund", h.CreateAdminRefund)

	// Create test order and return
	orderID := fix.OrderID

	var fulfillmentID uuid.UUID
	_ = client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1 LIMIT 1", orderID).Scan(&fulfillmentID)
	if fulfillmentID == uuid.Nil {
		fulfillmentID = uuid.New()
		_, _ = client.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'delivered')", fulfillmentID, orderID, fix.SellerID)
	}

	returnID := uuid.New()
	_, err := client.Pool.Exec(ctx, "INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())", returnID, orderID, fulfillmentID, fix.UserID)
	productID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, status, price_cents, created_at, updated_at) VALUES ($1, $2, 'Test Product', $3, 'published', 10000, now(), now())", productID, fix.SellerID, uuid.New().String())
	require.NoError(t, err)

	variantID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'SKU', 10000, now(), now())", variantID, productID)
	require.NoError(t, err)

	oiID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, image_url, price_cents, quantity, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'Test Item', 'test-slug', 'M', 'Red', 'SKU', '', 10000, 1, 10000, now())", oiID, orderID, fix.FulfillmentID, productID, variantID, fix.SellerID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO return_items (id, return_id, order_item_id, quantity, restock, created_at) VALUES ($1, $2, $3, 1, false, now())", uuid.New(), returnID, oiID)
	require.NoError(t, err)

	t.Cleanup(func() {
		client.Pool.Exec(ctx, "DELETE FROM return_items WHERE return_id = $1", returnID)
		client.Pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", returnID)
		client.Pool.Exec(ctx, "DELETE FROM order_items WHERE id = $1", oiID)
		client.Pool.Exec(ctx, "DELETE FROM product_variants WHERE id = $1", variantID)
		client.Pool.Exec(ctx, "DELETE FROM products WHERE id = $1", productID)
	})

	_, err = client.Pool.Exec(ctx, "UPDATE order_items SET price_cents = 10000 WHERE id = $1", oiID)
	require.NoError(t, err)

	reqBody := `{"reason":"defective"}`
	req, _ := http.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/refund", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	
	var resp Refund
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, resp.ID)
	assert.Equal(t, "pending", resp.Status)
	
	// mock Refund returns these fields null/zero because it's a mock!
	// We'll just verify HTTP status mostly, as requested by prompt.
	// But let's check actual Refund in DB
	var dbAmount int64
	err = client.Pool.QueryRow(ctx, "SELECT amount_cents FROM refunds WHERE payment_id = $1", fix.PaymentID).Scan(&dbAmount)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), dbAmount)

	// Try 0
	_, err = client.Pool.Exec(ctx, "UPDATE order_items SET price_cents = 0 WHERE id = $1", oiID)
	require.NoError(t, err)
	req2, _ := http.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/refund", bytes.NewBufferString(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	_, err = client.Pool.Exec(ctx, "UPDATE returns SET status = 'item_received' WHERE id = $1", returnID)
	require.NoError(t, err)

	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusInternalServerError, rr2.Code)

	// Try > available
	_, err = client.Pool.Exec(ctx, "UPDATE order_items SET price_cents = 200000 WHERE id = $1", oiID)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE returns SET status = 'item_received' WHERE id = $1", returnID)
	require.NoError(t, err)
	
	req3, _ := http.NewRequest("POST", "/api/admin/returns/"+returnID.String()+"/refund", bytes.NewBufferString(reqBody))
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusInternalServerError, rr3.Code)

}
