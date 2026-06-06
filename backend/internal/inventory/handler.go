package inventory

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
// Admin Operations
// ---------------------------------------------------------

func (h *Handler) ListAdminInventory(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	resp, err := h.service.ListAdminInventory(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list inventory")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminInventoryItem(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid inventory item ID")
		return
	}

	item, err := h.service.GetAdminInventoryItem(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrInventoryItemNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Inventory item not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get inventory item")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) ReceiveStock(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID := val.(uuid.UUID)

	var req ReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	item, err := h.service.ReceiveStock(r.Context(), adminID, req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to receive stock")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID := val.(uuid.UUID)

	var req AdjustmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	item, err := h.service.AdjustStock(r.Context(), adminID, req)
	if err != nil {
		if errors.Is(err, ErrNegativeStock) || errors.Is(err, ErrStockBelowReserved) || errors.Is(err, ErrInvalidQuantity) {
			h.writeError(w, http.StatusBadRequest, "invalid_adjustment", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to adjust stock")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) WriteOffStock(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID := val.(uuid.UUID)

	var req WriteOffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	item, err := h.service.WriteOffStock(r.Context(), adminID, req)
	if err != nil {
		if errors.Is(err, ErrNegativeStock) || errors.Is(err, ErrStockBelowReserved) {
			h.writeError(w, http.StatusBadRequest, "invalid_write_off", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to write-off stock")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) ListMovements(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid inventory item ID")
		return
	}

	page := pagination.FromRequest(r)
	resp, err := h.service.ListMovements(r.Context(), id, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list movements")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------
// Seller Operations
// ---------------------------------------------------------

func (h *Handler) ListSellerInventory(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	resp, err := h.service.ListSellerInventory(r.Context(), sellerID, page.Limit, page.Offset)
	if err != nil {
		if errors.Is(err, ErrSellerMismatch) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Not a seller")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list inventory")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetSellerInventoryItem(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid inventory item ID")
		return
	}

	item, err := h.service.GetSellerInventoryItem(r.Context(), sellerID, id)
	if err != nil {
		if errors.Is(err, ErrSellerMismatch) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Cannot access this inventory item")
			return
		}
		if errors.Is(err, ErrInventoryItemNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Inventory item not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get inventory item")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) ListSellerMovements(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid inventory item ID")
		return
	}

	page := pagination.FromRequest(r)
	resp, err := h.service.ListSellerMovements(r.Context(), sellerID, id, page.Limit, page.Offset)
	if err != nil {
		if errors.Is(err, ErrSellerMismatch) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Cannot access this inventory item")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list movements")
		return
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
