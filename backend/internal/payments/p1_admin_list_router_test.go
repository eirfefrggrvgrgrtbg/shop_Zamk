package payments_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func GenerateToken(data *SetupData, userID uuid.UUID, role string) string {
	token, _ := data.TokenService.GenerateAccessToken(userID, "test@example.com", role)
	return token
}

func TestAdminPaymentsRejectInvalidFilters(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)
	token := GenerateToken(data, data.AdminWithPermID, "admin")

	tests := []struct {
		name        string
		queryString string
	}{
		{"invalid_status", "status=unknown"},
		{"invalid_provider", "provider=unknown"},
		{"invalid_payment_method", "paymentMethod=unknown"},
		{"invalid_integration_mode", "integrationMode=unknown"},
		{"invalid_refund_state", "refundState=unknown"},
		{"invalid_problem_code", "problemCode=UNKNOWN"},
		{"invalid_has_problem", "hasProblem=broken"},
		{"invalid_amount_from", "amountFromCents=abc"},
		{"negative_amount", "amountFromCents=-1"},
		{"reversed_amount_range", "amountFromCents=10000&amountToCents=5000"},
		{"invalid_date", "dateFrom=broken"},
		{"reversed_date_range", "dateFrom=2024-01-01T00:00:00Z&dateTo=2023-01-01T00:00:00Z"},
		{"invalid_sort", "sort=drop_table"},
		{"invalid_direction", "direction=sideways"},
		{"zero_limit", "limit=0"},
		{"too_large_limit", "limit=101"},
		{"negative_offset", "offset=-1"},
		{"invalid_limit", "limit=abc"},
		{"invalid_offset", "offset=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/admin/payments?"+tt.queryString, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			
			data.Router.ServeHTTP(rr, req)
			
			assert.Equal(t, http.StatusBadRequest, rr.Code)
			
			var errResp map[string]map[string]interface{}
			json.Unmarshal(rr.Body.Bytes(), &errResp)
			assert.Equal(t, "validation_error", errResp["error"]["code"])
		})
	}
}

func TestAdminPaymentsList_RealRouter(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)
	token := GenerateToken(data, data.AdminWithPermID, "admin")
	
	t.Run("auth_checks", func(t *testing.T) {
		// without token
		req, _ := http.NewRequest("GET", "/api/admin/payments", nil)
		rr := httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		// customer
		req, _ = http.NewRequest("GET", "/api/admin/payments", nil)
		req.Header.Set("Authorization", "Bearer "+GenerateToken(data, data.CustomerID, "customer"))
		rr = httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		// admin without perm
		req, _ = http.NewRequest("GET", "/api/admin/payments", nil)
		req.Header.Set("Authorization", "Bearer "+GenerateToken(data, data.AdminNoPermID, "admin"))
		rr = httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		// admin with perm
		req, _ = http.NewRequest("GET", "/api/admin/payments", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("search_and_filters", func(t *testing.T) {
		// Wait 1 ms to ensure created_at differs slightly from next queries if needed, 
		// though fixtures are already injected at `now`.
		time.Sleep(time.Millisecond)

		// search by orderNumber
		orderNum := "ORD-" + data.OrderID.String()[:8]
		req, _ := http.NewRequest("GET", "/api/admin/payments?q="+orderNum, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp payments.AdminPaymentListResponse
		json.Unmarshal(rr.Body.Bytes(), &resp)
		
		assert.GreaterOrEqual(t, len(resp.Items), 1)
		found := false
		for _, p := range resp.Items {
			if p.PaymentID == data.PaymentID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find payment by orderNumber")

		// filter by status=succeeded
		req, _ = http.NewRequest("GET", "/api/admin/payments?status=succeeded", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		json.Unmarshal(rr.Body.Bytes(), &resp)
		for _, p := range resp.Items {
			assert.Equal(t, "succeeded", p.Status)
		}

		// pagination
		req, _ = http.NewRequest("GET", "/api/admin/payments?limit=2&offset=0", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.LessOrEqual(t, len(resp.Items), 2)
		// We could have more than 2 total payments in DB from previous tests
		assert.True(t, resp.TotalCount >= len(resp.Items))
	})
}
