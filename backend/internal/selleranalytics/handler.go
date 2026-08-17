package selleranalytics

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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

func getSellerID(r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := getSellerID(r)
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
	sellerID, ok := getSellerID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	from, to, _, err := parseTimeParams(r)
	if err != nil {
		http.Error(w, "Invalid from/to parameters", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.GetProducts(r.Context(), sellerID, from, to)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleProductDetail(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := getSellerID(r)
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
	sellerID, ok := getSellerID(r)
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
