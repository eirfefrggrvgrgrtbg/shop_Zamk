package payments

import (
	"encoding/json"
	"net/http"
)

// PaymentMethodInfo describes one payment method offered to the customer. It carries no keys or provider settings.
type PaymentMethodInfo struct {
	Method string `json:"method"`
	// Mock marks the local emulator (no real charge); it is never reported in production.
	Mock bool `json:"mock"`
}

type AvailablePaymentMethods struct {
	Methods []PaymentMethodInfo `json:"methods"`
	// Default is the method to preselect, empty when nothing is available.
	Default string `json:"default"`
}

var customerPaymentMethods = []string{"card", "tpay", "sbp"}

// AvailablePaymentMethods lists the methods CreatePayment would accept right now, derived from the same provider
// GetMode rules, so the list and the payment start cannot disagree. The mock T-Pay is preselected only outside
// production; otherwise the first available method in fixed order is the default.
func (s *Service) AvailablePaymentMethods() AvailablePaymentMethods {
	res := AvailablePaymentMethods{Methods: []PaymentMethodInfo{}}
	if s.provider == nil {
		return res
	}
	production := s.cfg != nil && s.cfg.App.Env == "production"
	for _, m := range customerPaymentMethods {
		mode := s.provider.GetMode(m)
		if mode == "unavailable" || mode == "unknown" {
			continue
		}
		isMock := m == "tpay" && mode == "mock"
		if isMock && production {
			continue
		}
		res.Methods = append(res.Methods, PaymentMethodInfo{Method: m, Mock: isMock})
	}
	for _, m := range res.Methods {
		if m.Mock {
			res.Default = m.Method
			return res
		}
	}
	if len(res.Methods) > 0 {
		res.Default = res.Methods[0].Method
	}
	return res
}

func (h *Handler) GetAvailableMethods(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(h.service.AvailablePaymentMethods())
}
