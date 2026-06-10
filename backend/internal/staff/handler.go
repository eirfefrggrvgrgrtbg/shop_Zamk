package staff

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

// Handler serves the /api/admin/me, /api/admin/staff/roles and /api/admin/audit-logs endpoints.
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
