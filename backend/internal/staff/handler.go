package staff

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

// Handler serves the /api/admin/* staff-related endpoints.
type Handler struct {
	service   *Service
	auditRepo *AuditRepository
	userRepo  *users.Repository
}

func NewHandler(svc *Service, auditRepo *AuditRepository, userRepo *users.Repository) *Handler {
	return &Handler{service: svc, auditRepo: auditRepo, userRepo: userRepo}
}

// GetAdminMe returns the authenticated admin user together with their staff access data.
func (h *Handler) GetAdminMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromCtx(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	user, err := h.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get user")
		return
	}

	access, err := h.service.GetStaffAccess(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get staff access")
		return
	}

	type staffResp struct {
		RoleCode    string   `json:"roleCode"`
		RoleName    string   `json:"roleName"`
		Status      string   `json:"status"`
		Permissions []string `json:"permissions"`
	}

	resp := struct {
		User  any        `json:"user"`
		Staff *staffResp `json:"staff"`
	}{
		User: map[string]any{
			"id":     user.ID,
			"name":   user.Name,
			"email":  user.Email,
			"role":   user.Role,
			"status": user.Status,
		},
	}

	if access != nil {
		resp.Staff = &staffResp{
			RoleCode:    access.Role.Code,
			RoleName:    access.Role.Name,
			Status:      access.Member.Status,
			Permissions: access.Permissions,
		}
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// GetStaffRoles returns all staff roles with their permissions.
func (h *Handler) GetStaffRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list roles")
		return
	}

	rolePerms, err := h.service.ListRolesWithPermissions(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list role permissions")
		return
	}

	type roleItem struct {
		StaffRole
		Permissions []string `json:"permissions"`
	}

	items := make([]roleItem, 0, len(roles))
	for _, role := range roles {
		perms := rolePerms[role.Code]
		if perms == nil {
			perms = []string{}
		}
		items = append(items, roleItem{StaffRole: role, Permissions: perms})
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GetAuditLogs returns paginated audit logs.
func (h *Handler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	logs, total, err := h.auditRepo.ListAuditLogs(r.Context(), limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list audit logs")
		return
	}
	if logs == nil {
		logs = []AuditLog{}
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"items":  logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ListStaffMembers returns all staff members.
// GET /api/admin/staff/members
func (h *Handler) ListStaffMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.service.ListStaffMembers(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list staff members")
		return
	}
	if members == nil {
		members = []StaffMemberView{}
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"items": members})
}

// CreateStaffMember creates a new admin user with a staff role.
// POST /api/admin/staff/members
func (h *Handler) CreateStaffMember(w http.ResponseWriter, r *http.Request) {
	actorID, ok := userIDFromCtx(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	var req struct {
		Name              string `json:"name"`
		Email             string `json:"email"`
		Phone             string `json:"phone"`
		RoleCode          string `json:"roleCode"`
		TemporaryPassword string `json:"temporaryPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if req.Email == "" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "email is required")
		return
	}
	if req.RoleCode == "" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "roleCode is required")
		return
	}
	if len(req.TemporaryPassword) < 8 {
		h.writeError(w, http.StatusBadRequest, "validation_error", "temporaryPassword must be at least 8 characters")
		return
	}

	result, err := h.service.CreateStaffMember(r.Context(), CreateStaffMemberInput{
		Name:              req.Name,
		Email:             req.Email,
		Phone:             req.Phone,
		RoleCode:          req.RoleCode,
		TemporaryPassword: req.TemporaryPassword,
		CreatedByUserID:   actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicateEmail):
			h.writeError(w, http.StatusConflict, "duplicate_email", "Email already in use")
		case errors.Is(err, ErrRoleNotFound):
			h.writeError(w, http.StatusBadRequest, "role_not_found", "Unknown role code")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create staff member")
		}
		return
	}

	// Audit — never include password in metadata
	_ = h.auditRepo.RecordAudit(r.Context(), AuditEvent{
		ActorUserID: actorID,
		Action:      "staff.create_access",
		EntityType:  "staff_member",
		EntityID:    &result.UserID,
		IP:          r.RemoteAddr,
		Metadata:    map[string]any{"email": result.Email, "roleCode": result.RoleCode},
	})

	h.writeJSON(w, http.StatusCreated, map[string]any{
		"userId":                    result.UserID,
		"email":                     result.Email,
		"roleCode":                  result.RoleCode,
		"temporaryPasswordReturned": false,
	})
}

// UpdateStaffRole updates a staff member's role.
// PATCH /api/admin/staff/members/{userId}/role
func (h *Handler) UpdateStaffRole(w http.ResponseWriter, r *http.Request) {
	actorID, ok := userIDFromCtx(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid userId")
		return
	}

	var req struct {
		RoleCode string `json:"roleCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if req.RoleCode == "" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "roleCode is required")
		return
	}

	if err := h.service.UpdateStaffRole(r.Context(), UpdateStaffRoleInput{
		TargetUserID: targetID,
		NewRoleCode:  req.RoleCode,
		ActorUserID:  actorID,
	}); err != nil {
		switch {
		case errors.Is(err, ErrTargetNotStaff):
			h.writeError(w, http.StatusNotFound, "not_found", "Staff member not found")
		case errors.Is(err, ErrCannotDemoteOwner):
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, ErrCannotPromoteToOwner):
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, ErrRoleNotFound):
			h.writeError(w, http.StatusBadRequest, "role_not_found", "Unknown role code")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update role")
		}
		return
	}

	_ = h.auditRepo.RecordAudit(r.Context(), AuditEvent{
		ActorUserID: actorID,
		Action:      "staff.role_update",
		EntityType:  "staff_member",
		EntityID:    &targetID,
		IP:          r.RemoteAddr,
		Metadata:    map[string]any{"newRoleCode": req.RoleCode},
	})

	h.writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// UpdateStaffStatus updates a staff member's status.
// PATCH /api/admin/staff/members/{userId}/status
func (h *Handler) UpdateStaffStatus(w http.ResponseWriter, r *http.Request) {
	actorID, ok := userIDFromCtx(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid userId")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if req.Status != "active" && req.Status != "blocked" && req.Status != "archived" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "status must be active, blocked, or archived")
		return
	}

	if err := h.service.UpdateStaffStatus(r.Context(), UpdateStaffStatusInput{
		TargetUserID: targetID,
		NewStatus:    req.Status,
		ActorUserID:  actorID,
	}); err != nil {
		switch {
		case errors.Is(err, ErrTargetNotStaff):
			h.writeError(w, http.StatusNotFound, "not_found", "Staff member not found")
		case errors.Is(err, ErrCannotBlockLastOwner):
			h.writeError(w, http.StatusConflict, "last_owner", err.Error())
		case errors.Is(err, ErrCannotDemoteOwner):
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update status")
		}
		return
	}

	_ = h.auditRepo.RecordAudit(r.Context(), AuditEvent{
		ActorUserID: actorID,
		Action:      "staff.status_update",
		EntityType:  "staff_member",
		EntityID:    &targetID,
		IP:          r.RemoteAddr,
		Metadata:    map[string]any{"newStatus": req.Status},
	})

	h.writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ResetStaffPassword resets a staff member's password.
// POST /api/admin/staff/members/{userId}/reset-password
func (h *Handler) ResetStaffPassword(w http.ResponseWriter, r *http.Request) {
	actorID, ok := userIDFromCtx(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid userId")
		return
	}

	var req struct {
		TemporaryPassword string `json:"temporaryPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if len(req.TemporaryPassword) < 8 {
		h.writeError(w, http.StatusBadRequest, "validation_error", "temporaryPassword must be at least 8 characters")
		return
	}

	if err := h.service.ResetStaffPassword(r.Context(), ResetStaffPasswordInput{
		TargetUserID:      targetID,
		TemporaryPassword: req.TemporaryPassword,
	}); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to reset password")
		return
	}

	// Audit — NO password in metadata
	_ = h.auditRepo.RecordAudit(r.Context(), AuditEvent{
		ActorUserID: actorID,
		Action:      "staff.reset_password",
		EntityType:  "staff_member",
		EntityID:    &targetID,
		IP:          r.RemoteAddr,
		Metadata:    map[string]any{},
	})

	h.writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func userIDFromCtx(r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, map[string]string{"error": code, "message": message})
}
