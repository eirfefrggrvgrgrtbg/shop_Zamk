package returns

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
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

func (h *Handler) CreateCustomerReturn(w http.ResponseWriter, r *http.Request) {
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

	var req CreateReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ret, items, err := h.service.CreateReturn(r.Context(), userID, orderID, req)
	if err != nil {
		if errors.Is(err, ErrOrderNotDelivered) || errors.Is(err, ErrReturnWindowExpired) || errors.Is(err, ErrInvalidQuantity) {
			h.writeError(w, http.StatusBadRequest, "invalid_return", err.Error())
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Unauthorized")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create return")
		return
	}

	resp := ReturnResponse{Return: *ret, Items: items}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetCustomerReturn(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	ret, items, err := h.service.GetCustomerReturn(r.Context(), userID, returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Unauthorized")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get return")
		return
	}

	resp := ReturnResponse{Return: *ret, Items: items}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ListCustomerReturns(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	returns, err := h.service.ListCustomerReturns(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list returns")
		return
	}

	var items []ReturnResponse
	for _, ret := range returns {
		// Only list basic details to avoid N+1 queries. If detailed items are needed, user can query by ID.
		items = append(items, ReturnResponse{Return: ret, Items: []ReturnItem{}})
	}

	resp := ReturnListResponse{Items: items, TotalCount: len(items)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------
// Admin Operations
// ---------------------------------------------------------

func (h *Handler) ListAdminReturns(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	returns, err := h.service.ListAdminReturns(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list returns")
		return
	}

	var items []ReturnResponse
	for _, ret := range returns {
		items = append(items, ReturnResponse{Return: ret, Items: []ReturnItem{}})
	}

	resp := ReturnListResponse{Items: items, TotalCount: len(items)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminReturn(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	ret, items, err := h.service.GetAdminReturn(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get return")
		return
	}

	resp := ReturnResponse{Return: *ret, Items: items}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateAdminReturnStatus(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	var req UpdateReturnStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	err = h.service.UpdateReturnStatus(r.Context(), adminID, returnID, req)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusBadRequest, "invalid_transition", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update return status")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateAdminRefund(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	var req CreateRefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ref, err := h.service.CreateRefund(r.Context(), adminID, returnID, req)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrRefundExceedsPaid) {
			h.writeError(w, http.StatusBadRequest, "refund_exceeds_paid", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create refund")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ref)
}

func (h *Handler) ListAdminRefunds(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	refunds, err := h.service.ListAdminRefunds(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list refunds")
		return
	}

	resp := RefundListResponse{Items: refunds, TotalCount: len(refunds)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminRefund(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	refundID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid refund ID")
		return
	}

	ref, err := h.service.GetAdminRefund(r.Context(), refundID)
	if err != nil {
		if errors.Is(err, ErrRefundNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Refund not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get refund")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ref)
}

// ---------------------------------------------------------
// Seller Operations
// ---------------------------------------------------------

func (h *Handler) ListSellerReturns(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID") // In this system seller users are authenticated via user ID
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	items, err := h.service.ListSellerReturns(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Must be a seller")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list seller returns")
		return
	}

	resp := SellerReturnListResponse{Items: items, TotalCount: len(items)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetSellerReturn(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	items, err := h.service.GetSellerReturn(r.Context(), userID, returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found or has no matching items")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get seller return")
		return
	}

	resp := SellerReturnListResponse{Items: items, TotalCount: len(items)}
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
