package payments

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	resp, err := h.service.CreatePayment(r.Context(), userID, orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		if errors.Is(err, ErrOrderNotAwaitingPayment) {
			h.writeError(w, http.StatusBadRequest, "invalid_order_status", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create payment")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HandleTBankWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to read body")
		return
	}
	defer r.Body.Close()

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if err := h.service.HandleWebhook(r.Context(), headers, body); err != nil {
		if errors.Is(err, ErrInvalidSignature) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		if errors.Is(err, ErrPaymentAlreadyProcessed) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process webhook: " + err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) ListAdminPayments(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	payments, err := h.service.repo.ListAdminPayments(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list payments")
		return
	}

	resp := AdminPaymentListResponse{
		Items:      payments,
		TotalCount: len(payments),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminPayment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	paymentID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid payment ID")
		return
	}

	payment, err := h.service.repo.GetAdminPayment(r.Context(), paymentID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Payment not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get payment")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
