package favorites

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid user context")
		return
	}

	productsList, err := h.service.ListFavorites(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list favorites")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(productsList)
}

func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}
	userID := val.(uuid.UUID)

	productIDStr := chi.URLParam(r, "productId")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid product ID")
		return
	}

	err = h.service.AddFavorite(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found or not published")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to add favorite")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}
	userID := val.(uuid.UUID)

	productIDStr := chi.URLParam(r, "productId")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid product ID")
		return
	}

	err = h.service.RemoveFavorite(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrFavoriteNotFound) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) // Idempotent
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to remove favorite")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
