package orders

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Handler struct {
	service   *Service
	validator *validator.Validate
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
	}
}

// ---------------------------------------------------------
// Customer Operations
// ---------------------------------------------------------

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	order, err := h.service.CreateOrder(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, ErrEmptyCart) || errors.Is(err, ErrProductNotPublished) || errors.Is(err, ErrVariantNotFound) || errors.Is(err, ErrInsufficientStock) {
			h.writeError(w, http.StatusBadRequest, "invalid_order", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) CancelCustomerOrder(w http.ResponseWriter, r *http.Request) {
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

	if err := h.service.CancelCustomerOrder(r.Context(), userID, orderID); err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		if errors.Is(err, ErrOrderNotCancellable) {
			h.writeError(w, http.StatusBadRequest, "not_cancellable", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to cancel order")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetCustomerOrder(w http.ResponseWriter, r *http.Request) {
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

	order, err := h.service.GetCustomerOrder(r.Context(), userID, orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get order")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) ListCustomerOrders(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	orders, err := h.service.ListCustomerOrders(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list orders")
		return
	}

	resp := OrderListResponse{
		Items:      orders,
		TotalCount: len(orders),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------
// Admin Operations
// ---------------------------------------------------------

func (h *Handler) GetAdminOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	order, err := h.service.GetAdminOrder(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get order")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) ListAdminOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.service.ListAdminOrders(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list orders")
		return
	}

	resp := OrderListResponse{
		Items:      orders,
		TotalCount: len(orders),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	var req UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.service.UpdateOrderStatus(r.Context(), adminID, orderID, req); err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		if errors.Is(err, ErrManualPaidNotAllowed) {
			h.writeError(w, http.StatusBadRequest, "invalid_status", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update order status")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------
// Seller Operations
// ---------------------------------------------------------

func (h *Handler) GetSellerOrder(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	order, err := h.service.GetSellerOrder(r.Context(), sellerID, orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get order")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) ListSellerOrders(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	orders, err := h.service.ListSellerOrders(r.Context(), sellerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list orders")
		return
	}

	resp := SellerOrderListResponse{
		Items:      orders,
		TotalCount: len(orders),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------
// Helper
// ---------------------------------------------------------

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
