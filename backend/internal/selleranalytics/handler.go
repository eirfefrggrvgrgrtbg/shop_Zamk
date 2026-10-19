package selleranalytics

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, "", fmt.Errorf("both 'from' and 'to' parameters are required")
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, "", fmt.Errorf("invalid 'from' timestamp: %w", err)
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, "", fmt.Errorf("invalid 'to' timestamp: %w", err)
	}
	return from, to, tz, nil
}

func validateTimePeriod(r *http.Request) (time.Time, time.Time, string, error) {
	from, to, tz, err := parseTimeParams(r)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, "", fmt.Errorf("invalid period: 'to' cannot be before 'from'")
	}
	if to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, "", fmt.Errorf("date range exceeds maximum allowed limit of 366 days")
	}
	return from, to, tz, nil
}

// defaultProductsLimit is the page size used when 'limit' is not provided.
const defaultProductsLimit = 50

// parsePagination strictly parses the optional 'limit' and 'offset' query values.
// Empty values fall back to the defaults (50 and 0); non-integer, out-of-range or
// negative values are rejected.
func parsePagination(limitStr, offsetStr string) (int, int, error) {
	limit := defaultProductsLimit
	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 0 {
			return 0, 0, fmt.Errorf("invalid 'limit': must be a non-negative integer")
		}
		limit = v
	}
	offset := 0
	if offsetStr != "" {
		v, err := strconv.Atoi(offsetStr)
		if err != nil || v < 0 {
			return 0, 0, fmt.Errorf("invalid 'offset': must be a non-negative integer")
		}
		offset = v
	}
	return limit, offset, nil
}

func parseAsOfParam(r *http.Request) (time.Time, error) {
	asOfStr := r.URL.Query().Get("asOf")
	if asOfStr == "" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, asOfStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid 'asOf' timestamp: %w", err)
	}
	if t.After(time.Now().UTC().Add(time.Minute)) {
		return time.Time{}, fmt.Errorf("'asOf' timestamp cannot be in the future")
	}
	return t, nil
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

	from, to, tz, err := validateTimePeriod(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	from, to, _, err := validateTimePeriod(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	sort := r.URL.Query().Get("sort")
	if sort == "" { sort = "gross_sales" }
	order := r.URL.Query().Get("order")
	if order == "" { order = "desc" }

	limit, offset, err := parsePagination(limitStr, offsetStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	asOf, err := parseAsOfParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.svc.GetProducts(r.Context(), sellerID, from, to, limit, offset, sort, order, asOf)
	if err != nil {
		if errors.Is(err, ErrCohortDataAnomaly) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
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

	from, to, tz, err := validateTimePeriod(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	asOf, err := parseAsOfParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.svc.GetProductDetail(r.Context(), sellerID, productID, from, to, tz, asOf)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrCohortDataAnomaly) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
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

	from, to, _, err := validateTimePeriod(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
