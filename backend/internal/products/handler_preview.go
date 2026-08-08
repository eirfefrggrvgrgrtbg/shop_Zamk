package products

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetProductPreviewByToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Token is required")
		return
	}
	p, err := h.service.GetProductPreviewByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrInvalidPreviewToken) {
			h.writeError(w, http.StatusNotFound, "invalid_token", "Invalid preview token format")
			return
		}
		if errors.Is(err, ErrPreviewUnavailable) {
			h.writeError(w, http.StatusGone, "preview_expired", "Preview token expired or deleted")
			return
		}
		if errors.Is(err, ErrRedisUnavailable) {
			h.writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Redis storage is required")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get preview")
		return
	}

	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-store")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) AdminCreateProductPreviewLink(w http.ResponseWriter, r *http.Request) {
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	adminID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	token, err := h.service.CreateProductPreviewLink(r.Context(), adminID, productID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create preview link")
		return
	}

	shopURL := os.Getenv("SHOP_PUBLIC_URL")
	if shopURL == "" {
		shopURL = "http://127.0.0.1:3000"
	}
	shopURL = strings.TrimRight(shopURL, "/")

	expiresAt := time.Now().Add(15 * time.Minute).Format(time.RFC3339)

	resp := ProductPreviewLinkResponse{
		PageURL:        fmt.Sprintf("%s/preview/products/%s", shopURL, token),
		CatalogCardURL: fmt.Sprintf("%s/preview/products/%s/card", shopURL, token),
		ExpiresAt:      expiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) AdminStartProductReview(w http.ResponseWriter, r *http.Request) {
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	adminID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	err := h.service.StartProductReview(r.Context(), adminID, productID)
	if err != nil {
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to start review")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
