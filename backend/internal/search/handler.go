package search

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

type Handler struct {
	svc      *Service
	staffSvc *staff.Service
	logger   *slog.Logger
}

func NewHandler(svc *Service, staffSvc *staff.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, staffSvc: staffSvc, logger: logger}
}

func (h *Handler) HandleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	if h.staffSvc == nil {
		if h.logger != nil {
			h.logger.ErrorContext(r.Context(), "search handler: missing staff service dependency")
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

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

	access, err := h.staffSvc.GetStaffAccess(r.Context(), userID)
	if err != nil {
		if h.logger != nil {
			h.logger.ErrorContext(r.Context(), "search handler: get staff access failed", "error", err, "user_id", userID)
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	if access == nil || access.Member == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав")
		return
	}

	if access.Member.Status != string(staff.StatusActive) {
		h.writeError(w, http.StatusForbidden, "forbidden", "Сотрудник заблокирован или неактивен")
		return
	}

	var perms AllowedPermissions
	for _, p := range access.Permissions {
		switch p {
		case "orders.read":
			perms.CanReadOrders = true
		case "returns.read":
			perms.CanReadReturns = true
		case "inventory.read":
			perms.CanReadInventory = true
		case "products.read":
			perms.CanReadProducts = true
		case "users.read":
			perms.CanReadUsers = true
		}
	}

	q := r.URL.Query().Get("q")
	results, err := h.svc.GlobalSearch(r.Context(), q, perms)
	if err != nil {
		if errors.Is(err, ErrQueryTooShort) {
			h.writeError(w, http.StatusBadRequest, "query_too_short", "Поисковый запрос должен содержать минимум 2 символа")
			return
		}
		if h.logger != nil {
			h.logger.ErrorContext(r.Context(), "global search failed", "error", err, "query", q)
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	if results == nil {
		results = []GlobalSearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(GlobalSearchResponse{
		Results: results,
	})
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
