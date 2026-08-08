package payments_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAdminPaymentDetailAuthorization(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)
	tokenAdminWithPerm := GenerateToken(data, data.AdminWithPermID, "admin")
	tokenAdminNoPerm := GenerateToken(data, data.AdminNoPermID, "admin")
	tokenCustomer := GenerateToken(data, data.CustomerID, "customer")

	// Create a payment
	orderID := uuid.New()
	_, err := data.Pool.Exec(context.Background(), `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, $3, 'awaiting_payment', 10000, 'RUB', 'Test Address', 'Delivery', 0, 'Test User', 'test@example.com', '+79000000000', now(), now())
	`, orderID, data.CustomerID, "ORD-"+uuid.New().String()[:6])
	if err != nil { t.Fatalf("failed to insert order: %v", err) }
	
	pID := uuid.New()
	pAuthNum := "PAY-" + uuid.New().String()[:6]
	_, err = data.Pool.Exec(context.Background(), `INSERT INTO payments (id, order_id, status, provider, payment_method, amount_cents, currency, payment_number, created_at, updated_at, idempotency_key) 
		VALUES ($1, $2, 'pending', 'tbank', 'sbp', 10000, 'RUB', $3, now(), now(), $4)`, pID, orderID, pAuthNum, uuid.New().String())
	if err != nil { t.Fatalf("failed to insert payment: %v", err) }

	tests := []struct {
		name         string
		token        string
		expectedCode int
	}{
		{"no_token", "", http.StatusUnauthorized},
		{"customer", tokenCustomer, http.StatusForbidden},
		{"admin_no_perm", tokenAdminNoPerm, http.StatusForbidden},
		{"admin_with_perm", tokenAdminWithPerm, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/admin/payments/"+pID.String(), nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rr := httptest.NewRecorder()
			data.Router.ServeHTTP(rr, req)
			assert.Equal(t, tt.expectedCode, rr.Code)
		})
	}
}
