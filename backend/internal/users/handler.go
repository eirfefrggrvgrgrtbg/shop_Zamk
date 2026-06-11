package users

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

type UpdateProfileRequest struct {
	Name  string `json:"name" validate:"required"`
	Phone string `json:"phone" validate:"required"`
}

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
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

	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User not found")
		return
	}

	if user.Role != RoleCustomer {
		h.writeError(w, http.StatusForbidden, "forbidden", "Only customers can access this endpoint")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        user.ID,
		"name":      user.Name,
		"email":     user.Email,
		"phone":     user.Phone,
		"status":    user.Status,
		"createdAt": user.CreatedAt,
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	err := h.repo.UpdateCustomerProfile(r.Context(), userID, req.Name, req.Phone)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update profile")
		return
	}

	// Fetch updated
	user, _ := h.repo.GetUserByID(r.Context(), userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        user.ID,
		"name":      user.Name,
		"email":     user.Email,
		"phone":     user.Phone,
		"status":    user.Status,
		"createdAt": user.CreatedAt,
	})
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
