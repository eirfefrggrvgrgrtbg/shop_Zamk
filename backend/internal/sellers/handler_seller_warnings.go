package sellers

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// GetMyWarnings returns warnings for the currently authenticated seller
func (h *Handler) GetMyWarnings(w http.ResponseWriter, r *http.Request) {
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

	meRes, err := h.service.GetSellerMe(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrSellerUserNotFound) {
			h.respondError(w, http.StatusNotFound, "seller profile not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to get seller profile")
		return
	}

	warnings, err := h.service.ListWarnings(r.Context(), meRes.Seller.ID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list warnings")
		return
	}

	h.respondJSON(w, http.StatusOK, warnings)
}

// GetMyViolations returns violations for the currently authenticated seller
func (h *Handler) GetMyViolations(w http.ResponseWriter, r *http.Request) {
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

	meRes, err := h.service.GetSellerMe(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrSellerUserNotFound) {
			h.respondError(w, http.StatusNotFound, "seller profile not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to get seller profile")
		return
	}

	violations, err := h.service.ListViolations(r.Context(), meRes.Seller.ID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list violations")
		return
	}

	h.respondJSON(w, http.StatusOK, violations)
}
