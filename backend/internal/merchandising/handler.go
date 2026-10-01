package merchandising

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

// Admin API
func (h *Handler) ListAdminCollections(w http.ResponseWriter, r *http.Request) {
	cols, err := h.service.ListAdminCollections(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list collections")
		return
	}
	if cols == nil {
		cols = []Collection{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cols)
}

func (h *Handler) CreateCollection(w http.ResponseWriter, r *http.Request) {
	var req CreateCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", "Validation failed")
		return
	}

	col, err := h.service.CreateCollection(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Slug already exists")
			return
		}
		if errors.Is(err, ErrInvalidDates) {
			h.writeError(w, http.StatusBadRequest, "invalid_dates", "Invalid dates")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create collection")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(col)
}

func (h *Handler) GetAdminCollection(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid collection ID")
		return
	}

	resp, err := h.service.GetAdminCollectionDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrCollectionNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Collection not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get collection")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateCollection(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid collection ID")
		return
	}

	var req UpdateCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", "Validation failed")
		return
	}

	col, err := h.service.UpdateCollection(r.Context(), id, &req)
	if err != nil {
		if errors.Is(err, ErrCollectionNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Collection not found")
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Slug already exists")
			return
		}
		if errors.Is(err, ErrInvalidDates) {
			h.writeError(w, http.StatusBadRequest, "invalid_dates", "Invalid dates")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update collection")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(col)
}

func (h *Handler) ReplaceCollectionItems(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid collection ID")
		return
	}

	var req ReplaceCollectionItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if err := h.service.ReplaceCollectionItems(r.Context(), id, req.ProductIDs); err != nil {
		if errors.Is(err, ErrCollectionNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Collection not found")
			return
		}
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusBadRequest, "product_not_found", "One or more products do not exist")
			return
		}
		if errors.Is(err, ErrDuplicateProductID) {
			h.writeError(w, http.StatusBadRequest, "duplicate_products", "Duplicate product IDs provided")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to replace collection items")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Public API
func (h *Handler) ListPublicCollections(w http.ResponseWriter, r *http.Request) {
	cols, err := h.service.ListPublicCollections(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list collections")
		return
	}
	if cols == nil {
		cols = []Collection{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cols)
}

func (h *Handler) GetPublicCollection(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		h.writeError(w, http.StatusBadRequest, "missing_slug", "Slug is required")
		return
	}

	resp, err := h.service.GetPublicCollectionDetail(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrCollectionNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Collection not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get collection")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
