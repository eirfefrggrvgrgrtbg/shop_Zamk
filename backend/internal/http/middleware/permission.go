package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

// RequirePermission checks that the authenticated admin user has the given RBAC permission.
// It is designed to be chained after AuthMiddleware + RequireRole("admin").
// The permission check is purely advisory in Phase B — this middleware is exported but not yet
// wired to any existing admin routes (to avoid lockout). It will be applied in Phase D.
func RequirePermission(staffSvc *staff.Service, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			val := r.Context().Value("userID")
			if val == nil {
				writePermError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
				return
			}
			userID, ok := val.(uuid.UUID)
			if !ok {
				writePermError(w, http.StatusUnauthorized, "unauthorized", "Invalid user context")
				return
			}

			ctx := context.WithValue(r.Context(), "checkedPermission", permission)

			allowed, err := staffSvc.HasPermission(ctx, userID, permission)
			if err != nil {
				writePermError(w, http.StatusInternalServerError, "internal_error", "Permission check failed")
				return
			}
			if !allowed {
				writePermError(w, http.StatusForbidden, "insufficient_permissions", "Недостаточно прав")
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writePermError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
