package supplies

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListSupplies(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || role != "seller" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	supplies, err := h.svc.repo.GetSuppliesBySeller(r.Context(), sellerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supplies)
}

func (h *Handler) GetSupply(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okRole || !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid supply ID", http.StatusBadRequest)
		return
	}

	supply, err := h.svc.repo.GetSupplyByID(r.Context(), id)
	if err != nil {
		if err == ErrSupplyNotFound {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if role == "seller" && supply.SellerID != sellerID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supply)
}

func (h *Handler) CreateSupply(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || role != "seller" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateSupplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	supply, err := h.svc.CreateSupply(r.Context(), sellerID, req)
	if err != nil {
		if err == ErrInvalidQuantities {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supply)
}

func (h *Handler) MarkShipped(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || role != "seller" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid supply ID", http.StatusBadRequest)
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = h.svc.MarkShipped(r.Context(), sellerID, id)
	if err != nil {
		if err == ErrUnauthorized {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if err == ErrInvalidStatus {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
