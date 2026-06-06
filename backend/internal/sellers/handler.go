package sellers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateSellerByAdmin(w http.ResponseWriter, r *http.Request) {
	var req CreateSellerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Basic validation could be done here (e.g. using validator package).
	// For MVP we just do simple checks.
	if req.BrandName == "" || req.ContactEmail == "" || req.OwnerEmail == "" || req.TemporaryPassword == "" {
		h.respondError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	res, err := h.service.CreateSellerByAdmin(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) || errors.Is(err, ErrDuplicateEmail) {
			h.respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to create seller")
		return
	}

	h.respondJSON(w, http.StatusCreated, res)
}

func (h *Handler) ListSellers(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	res, err := h.service.ListSellers(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list sellers")
		return
	}
	h.respondJSON(w, http.StatusOK, res)
}

func (h *Handler) UpdateSellerStatus(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}

	var req UpdateSellerStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.UpdateSellerStatus(r.Context(), sellerID, &req); err != nil {
		if errors.Is(err, ErrSellerNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetSellerMe(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	res, err := h.service.GetSellerMe(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrSellerUserNotFound) {
			h.respondError(w, http.StatusNotFound, "seller profile not found for user")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to get seller profile")
		return
	}

	h.respondJSON(w, http.StatusOK, res)
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}
