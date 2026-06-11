package fulfillment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
)

type FulfillmentListResponse struct {
	Items      []Fulfillment `json:"items"`
	TotalCount int           `json:"totalCount"`
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}

func (h *Handler) ListSellerFulfillments(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	statusParam := r.URL.Query().Get("status")
	var status *string
	if statusParam != "" {
		status = &statusParam
	}

	fulfillments, err := h.svc.ListSellerFulfillments(r.Context(), sellerID, page.Limit, page.Offset, status)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FulfillmentListResponse{
		Items:      fulfillments,
		TotalCount: len(fulfillments), // Just returning length as total for simplicity in this phase
	})
}

func (h *Handler) GetSellerFulfillment(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	f, err := h.svc.GetSellerFulfillment(r.Context(), sellerID, fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *Handler) ListAdminFulfillments(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	statusParam := r.URL.Query().Get("status")
	var status *string
	if statusParam != "" {
		status = &statusParam
	}

	fulfillments, err := h.svc.ListAdminFulfillments(r.Context(), page.Limit, page.Offset, status)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FulfillmentListResponse{
		Items:      fulfillments,
		TotalCount: len(fulfillments),
	})
}

func (h *Handler) GetAdminFulfillment(w http.ResponseWriter, r *http.Request) {
	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	f, err := h.svc.GetAdminFulfillment(r.Context(), fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *Handler) GetAdminOrderFulfillments(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid Order ID")
		return
	}

	fulfillments, err := h.svc.GetOrderFulfillments(r.Context(), orderID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fulfillments)
}

func (h *Handler) GetCustomerOrderFulfillments(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	customerID := val.(uuid.UUID)

	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid Order ID")
		return
	}

	fulfillments, err := h.svc.CustomerGetOrderFulfillments(r.Context(), customerID, orderID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fulfillments)
}
