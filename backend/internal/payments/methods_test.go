package payments

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
)

func methodsService(env string, p Provider) *Service {
	cfg := &config.Config{}
	cfg.App.Env = env
	return &Service{provider: p, cfg: cfg}
}

func methodNames(r AvailablePaymentMethods) string {
	names := []string{}
	for _, m := range r.Methods {
		names = append(names, m.Method)
	}
	return strings.Join(names, ",")
}

func TestAvailablePaymentMethods(t *testing.T) {
	t.Run("no credentials, mock tpay: card hidden, mock preselected", func(t *testing.T) {
		r := methodsService("local", NewTBankProvider("", "", "", "", "", false, "O", "mock")).AvailablePaymentMethods()
		if methodNames(r) != "tpay" || r.Default != "tpay" || !r.Methods[0].Mock {
			t.Fatalf("got %+v", r)
		}
	})
	t.Run("no credentials, real tpay mode: nothing available", func(t *testing.T) {
		r := methodsService("local", NewTBankProvider("", "", "", "", "", true, "O", "quick_widget")).AvailablePaymentMethods()
		if len(r.Methods) != 0 || r.Default != "" {
			t.Fatalf("got %+v", r)
		}
	})
	t.Run("real credentials: card default, no mock switch", func(t *testing.T) {
		r := methodsService("production", NewTBankProvider("TERMINAL", "SECRET", "", "", "", true, "O", "quick_widget")).AvailablePaymentMethods()
		if methodNames(r) != "card,tpay" || r.Default != "card" {
			t.Fatalf("got %+v", r)
		}
	})
	t.Run("production never reports mock", func(t *testing.T) {
		r := methodsService("production", NewTBankProvider("", "", "", "", "", false, "O", "mock")).AvailablePaymentMethods()
		if len(r.Methods) != 0 {
			t.Fatalf("got %+v", r)
		}
	})
	t.Run("response has no secrets", func(t *testing.T) {
		h := NewHandler(methodsService("local", NewTBankProvider("TERMINAL", "SECRET", "", "", "", false, "O", "mock")), "local")
		w := httptest.NewRecorder()
		h.GetAvailableMethods(w, httptest.NewRequest("GET", "/api/public/payment-methods", nil))
		body := w.Body.String()
		if strings.Contains(body, "TERMINAL") || strings.Contains(body, "SECRET") {
			t.Fatalf("leak: %s", body)
		}
		var out AvailablePaymentMethods
		if err := json.Unmarshal([]byte(body), &out); err != nil || out.Default != "tpay" {
			t.Fatalf("got %s %v", body, err)
		}
	})
}
