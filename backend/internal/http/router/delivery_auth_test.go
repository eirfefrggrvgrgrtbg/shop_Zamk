package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestAdminDeliveryRouter_RBAC(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
	}
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	insertUser := func(role string) uuid.UUID {
		id := uuid.New()
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test Admin', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		require.NoError(t, err)
		return id
	}

	insertAdminWithPerms := func(userID uuid.UUID, perms []string) {
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'DeliveryRole')`, roleID, code)
		require.NoError(t, err)
		for _, p := range perms {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
			require.NoError(t, err)
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	// 1. Prepare data
	sellerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Delivery Brand', $2, $3, 'active', now(), now())
	`, sellerID, uuid.New().String(), uuid.New().String()+"@ex.com")
	require.NoError(t, err)

	orderID := uuid.New()
	buyerID := insertUser("customer")
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'shipped', 1000, 'N', 'P', 'E', 'A')
	`, orderID, buyerID)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'shipped', 1000, 900, 900)
	`, fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	shipmentID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', now(), now(), now())
	`, shipmentID, orderID, fulfillmentID)
	require.NoError(t, err)

	// Create users with various permission sets
	adminWithUpdatePerm := insertUser("admin")
	insertAdminWithPerms(adminWithUpdatePerm, []string{"shipments.update_status"})
	adminWithUpdateToken := makeToken(adminWithUpdatePerm, "admin")

	adminWithReadOnly := insertUser("admin")
	insertAdminWithPerms(adminWithReadOnly, []string{"shipments.read"})
	adminReadOnlyToken := makeToken(adminWithReadOnly, "admin")

	sellerUser := insertUser("seller")
	sellerToken := makeToken(sellerUser, "seller")

	customerUser := insertUser("customer")
	customerToken := makeToken(customerUser, "customer")

	// 1. Unauthenticated -> 401
	t.Run("unauthenticated -> 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+shipmentID.String()+"/deliver", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	// 2. Customer -> 403
	t.Run("customer -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+shipmentID.String()+"/deliver", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 3. Seller -> 403
	t.Run("seller -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+shipmentID.String()+"/deliver", nil)
		req.Header.Set("Authorization", "Bearer "+sellerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 4. Admin with only shipments.read -> 403
	t.Run("admin with shipments.read only -> 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+shipmentID.String()+"/deliver", nil)
		req.Header.Set("Authorization", "Bearer "+adminReadOnlyToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	// 5. Invalid UUID -> 400
	t.Run("invalid uuid -> 400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/not-a-uuid/deliver", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithUpdateToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	// 6. Non-existent shipment -> 404
	t.Run("non-existent shipment -> 404", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+uuid.New().String()+"/deliver", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithUpdateToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	// 7. Successful delivery with shipments.update_status -> 200 OK
	t.Run("delivery success with shipments.update_status -> 200", func(t *testing.T) {
		body, _ := json.Marshal(fulfillment.DeliverShipmentRequest{
			Comment: func(s string) *string { return &s }("Доставлено курьером"),
		})
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+shipmentID.String()+"/deliver", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminWithUpdateToken)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res fulfillment.DeliveryResult
		err := json.NewDecoder(rr.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, shipmentID, res.ShipmentID)
		assert.Equal(t, fulfillmentID, res.FulfillmentID)
		assert.Equal(t, "delivered", res.ShipmentStatus)
		assert.Equal(t, "delivered", res.FulfillmentStatus)
		assert.Equal(t, "delivered", res.OrderStatus)
	})

	// 8. Already delivered -> 409 shipment_already_delivered
	t.Run("already delivered -> 409 shipment_already_delivered", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/shipments/"+shipmentID.String()+"/deliver", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithUpdateToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)

		var res struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(rr.Body).Decode(&res)
		assert.Equal(t, "shipment_already_delivered", res.Error.Code)
	})
}
