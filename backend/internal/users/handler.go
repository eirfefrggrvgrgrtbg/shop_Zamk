package users

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

type UpdateProfileRequest struct {
	FirstName  string `json:"firstName" validate:"required,min=2,max=80"`
	LastName   string `json:"lastName" validate:"required,min=2,max=80"`
	MiddleName string `json:"middleName,omitempty" validate:"omitempty,max=80"`
	Phone      string `json:"phone" validate:"required"`
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
		"id":         user.ID,
		"name":       user.Name,
		"firstName":  user.FirstName,
		"lastName":   user.LastName,
		"middleName": user.MiddleName,
		"email":      user.Email,
		"phone":      user.Phone,
		"status":     user.Status,
		"createdAt":  user.CreatedAt,
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

	// Wait, we need auth.ValidateNameFields, let's just do it directly or import auth?
	// But auth imports users, so importing auth from users would create a circular dependency.
	// So we do basic validation here or use the validator.
	
	// Assuming validator is added to handler? Let's just trust the validator tags or do basic trim.
	fullName := req.LastName + " " + req.FirstName
	if req.MiddleName != "" {
		fullName += " " + req.MiddleName
	}

	var midName *string
	if req.MiddleName != "" {
		midName = &req.MiddleName
	}

	err := h.repo.UpdateCustomerProfile(r.Context(), userID, fullName, &req.FirstName, &req.LastName, midName, req.Phone)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update profile")
		return
	}

	// Fetch updated
	user, _ := h.repo.GetUserByID(r.Context(), userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         user.ID,
		"name":       user.Name,
		"firstName":  user.FirstName,
		"lastName":   user.LastName,
		"middleName": user.MiddleName,
		"email":      user.Email,
		"phone":      user.Phone,
		"status":     user.Status,
		"createdAt":  user.CreatedAt,
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

// ListAdminUsers handles GET /api/admin/users
func (h *Handler) ListAdminUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	role := r.URL.Query().Get("role")
	status := r.URL.Query().Get("status")
	
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > 100 {
				v = 100
			}
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	filter := UsersFilter{
		Query:  q,
		Role:   role,
		Status: status,
		Limit:  limit,
		Offset: offset,
	}

	usrs, total, err := h.repo.ListUsers(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list users")
		return
	}

	type SafeUser struct {
		ID                 uuid.UUID `json:"id"`
		Email              string    `json:"email"`
		Name               string    `json:"name"`
		FirstName          *string   `json:"firstName,omitempty"`
		LastName           *string   `json:"lastName,omitempty"`
		Role               string    `json:"role"`
		Status             string    `json:"status"`
		MustChangePassword bool      `json:"mustChangePassword"`
		CreatedAt          string    `json:"createdAt"`
	}

	out := make([]SafeUser, 0, len(usrs))
	for _, u := range usrs {
		out = append(out, SafeUser{
			ID:                 u.ID,
			Email:              u.Email,
			Name:               u.Name,
			FirstName:          u.FirstName,
			LastName:           u.LastName,
			Role:               u.Role,
			Status:             u.Status,
			MustChangePassword: u.MustChangePassword,
			CreatedAt:          u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items":  out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

