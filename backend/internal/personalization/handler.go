package personalization

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RecordProductView records an authenticated customer's view of a published storefront product.
func (h *Handler) RecordProductView(w http.ResponseWriter, r *http.Request) {
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

	productIDStr := chi.URLParam(r, "productId")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid product ID")
		return
	}

	err = h.service.RecordProductView(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotAccessible) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found or not published")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to record product view")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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

func (h *Handler) GetRecentlyViewedProducts(w http.ResponseWriter, r *http.Request) {
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

	limitStr := r.URL.Query().Get("limit")
	limit := 12 // Default to 12
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	items, err := h.service.GetRecentlyViewedProducts(r.Context(), userID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get recently viewed products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items":      items,
		"totalCount": len(items),
	})
}

// GetForYouProducts retrieves personalized product recommendations for the authenticated customer.
func (h *Handler) GetForYouProducts(w http.ResponseWriter, r *http.Request) {
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

	limit := DefaultForYouLimit
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			if parsedLimit > MaxForYouLimit {
				limit = MaxForYouLimit
			} else {
				limit = parsedLimit
			}
		}
	}

	items, err := h.service.GetForYouProducts(r.Context(), userID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get for-you recommendations")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items":      items,
		"totalCount": len(items),
	})
}

// GetSimilarProducts retrieves similar products for the given product ID without authentication.
func (h *Handler) GetSimilarProducts(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid product ID")
		return
	}

	limit := 8
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	items, err := h.service.GetSimilarProducts(r.Context(), productID, limit)
	if err != nil {
		if errors.Is(err, ErrProductNotAccessible) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found or not accessible")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get similar products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items":      items,
		"totalCount": len(items),
	})
}
