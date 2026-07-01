package addresses

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

func (h *Handler) ListAddresses(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid user context")
		return
	}

	addresses, err := h.service.ListAddresses(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list addresses")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if addresses == nil {
		addresses = []Address{} // return empty array instead of null
	}
	json.NewEncoder(w).Encode(addresses)
}

func (h *Handler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid user context")
		return
	}

	var req CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Basic validation could go here

	addr, err := h.service.CreateAddress(r.Context(), userID, req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create address")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(addr)
}

func (h *Handler) UpdateAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid user context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid address ID")
		return
	}

	var req UpdateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	addr, err := h.service.UpdateAddress(r.Context(), id, userID, req)
	if err != nil {
		if errors.Is(err, ErrAddressNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Address not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update address")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(addr)
}

func (h *Handler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid user context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid address ID")
		return
	}

	err = h.service.DeleteAddress(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, ErrAddressNotFound) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete address")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) SetDefault(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid user context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid address ID")
		return
	}

	err = h.service.SetDefault(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, ErrAddressNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Address not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to set default address")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) getUserID(r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
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
