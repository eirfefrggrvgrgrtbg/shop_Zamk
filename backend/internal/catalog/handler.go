package catalog

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
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

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	cat, err := h.service.CreateCategory(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Category slug already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create category")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cat)
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.ListCategories(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list categories")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) CreateBrand(w http.ResponseWriter, r *http.Request) {
	var req CreateBrandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	brand, err := h.service.CreateBrand(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Brand slug already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create brand")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(brand)
}

func (h *Handler) ListBrands(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.ListBrands(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list brands")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
