package selleranalytics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc  *Service
	repo *Repository
}

func NewHandler(svc *Service, repo *Repository) *Handler {
	return &Handler{svc: svc, repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/overview", h.handleOverview)
	r.Get("/products", h.handleProducts)
	r.Get("/products/{productId}", h.handleProductDetail)
	r.Get("/inventory", h.handleInventory)
}

func parseTimeParams(r *http.Request) (time.Time, time.Time, string, error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	tz := "Europe/Moscow"

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	return from, to, tz, nil
}

func (h *Handler) resolveSellerID(r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		return uuid.Nil, false
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, false
	}
	sellerID, err := h.repo.ResolveSellerID(r.Context(), userID)
	if err != nil {
		return uuid.Nil, false
	}
	return sellerID, true
}

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := h.resolveSellerID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	from, to, tz, err := parseTimeParams(r)
	if err != nil {
		http.Error(w, "Invalid from/to parameters", http.StatusBadRequest)
		return
	}

	if to.Before(from) {
		http.Error(w, "Invalid period", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.GetOverview(r.Context(), sellerID, from, to, tz)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleProducts(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := h.resolveSellerID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	from, to, _, err := parseTimeParams(r)
	if err != nil {
		http.Error(w, "Invalid from/to parameters", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	sort := r.URL.Query().Get("sort")
	if sort == "" { sort = "gross_sales" }
	order := r.URL.Query().Get("order")
	if order == "" { order = "desc" }

	limit := 50
	if limitStr != "" { fmt.Sscanf(limitStr, "%d", &limit) }
	offset := 0
	if offsetStr != "" { fmt.Sscanf(offsetStr, "%d", &offset) }

	resp, err := h.svc.GetProducts(r.Context(), sellerID, from, to, limit, offset, sort, order)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleProductDetail(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := h.resolveSellerID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	productIDStr := chi.URLParam(r, "productId")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	from, to, tz, err := parseTimeParams(r)
	if err != nil {
		http.Error(w, "Invalid from/to parameters", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.GetProductDetail(r.Context(), sellerID, productID, from, to, tz)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleInventory(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := h.resolveSellerID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	from, to, _, err := parseTimeParams(r)
	if err != nil {
		http.Error(w, "Invalid from/to parameters", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.GetInventory(r.Context(), sellerID, from, to)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
