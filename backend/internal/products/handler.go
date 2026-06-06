package products

import (
	"context"
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

func (h *Handler) getUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return uuid.Nil, false
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid user context")
		return uuid.Nil, false
	}
	return userID, true
}

func (h *Handler) parseUUIDParam(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	idStr := chi.URLParam(r, param)
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID format")
		return uuid.Nil, false
	}
	return id, true
}

// ---------------------------------------------------------
// Seller Handlers
// ---------------------------------------------------------

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	prod, err := h.service.CreateProductForSeller(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Product slug already exists")
			return
		}
		if errors.Is(err, ErrSellerNotFound) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Seller profile not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(prod)
}

func (h *Handler) ListSellerProducts(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	page := pagination.FromRequest(r)
	resp, err := h.service.ListSellerProducts(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetSellerProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	prod, err := h.service.GetSellerProduct(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prod)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	prod, err := h.service.UpdateProductForSeller(r.Context(), userID, productID, req)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Product slug already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prod)
}

func (h *Handler) DeleteDraftProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	err := h.service.DeleteSellerDraftProduct(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found or not in draft/rejected state")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete product")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SubmitForModeration(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req SubmitProductModerationRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	err := h.service.SubmitProductToModeration(r.Context(), userID, productID, req)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to submit product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------------------------------------------------------
// Admin Handlers
// ---------------------------------------------------------

func (h *Handler) ListAdminProducts(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	resp, err := h.service.ListAdminProducts(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ListModerationProducts(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	resp, err := h.service.ListProductsForModeration(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) AdminApproveProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.ApproveProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminPublishProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.PublishProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminHideProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.HideProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminBlockProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.BlockProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminRejectProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req RejectProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	err := h.service.RejectProduct(r.Context(), userID, productID, req.Comment)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to reject product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleAdminModerationAction(w http.ResponseWriter, r *http.Request, action func(context.Context, uuid.UUID, uuid.UUID, *string) error) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req AdminProductModerationRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	err := action(r.Context(), userID, productID, req.Comment)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to perform moderation action")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------------------------------------------------------
// Public Handlers
// ---------------------------------------------------------

func (h *Handler) ListPublicProducts(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	resp, err := h.service.ListPublicProducts(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetPublicProduct(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	if idOrSlug == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Missing id or slug")
		return
	}

	prod, err := h.service.GetPublicProduct(r.Context(), idOrSlug)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prod)
}
