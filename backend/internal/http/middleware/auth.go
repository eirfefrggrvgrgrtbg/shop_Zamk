package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
)

func AuthMiddleware(tokenService *auth.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":{"code":"unauthorized","message":"Missing authorization header"}}`, http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, `{"error":{"code":"unauthorized","message":"Invalid authorization header format"}}`, http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims, err := tokenService.ValidateAccessToken(tokenString)
			if err != nil {
				http.Error(w, `{"error":{"code":"unauthorized","message":"Invalid or expired access token"}}`, http.StatusUnauthorized)
				return
			}

			sub, ok := claims["sub"].(string)
			if !ok {
				http.Error(w, `{"error":{"code":"unauthorized","message":"Invalid token claims"}}`, http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(sub)
			if err != nil {
				http.Error(w, `{"error":{"code":"unauthorized","message":"Invalid user ID in token"}}`, http.StatusUnauthorized)
				return
			}

			email, _ := claims["email"].(string)
			role, _ := claims["role"].(string)

			ctx := context.WithValue(r.Context(), "userID", userID)
			ctx = context.WithValue(ctx, "email", email)
			ctx = context.WithValue(ctx, "role", role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			val := r.Context().Value("role")
			if val == nil {
				http.Error(w, `{"error":{"code":"forbidden","message":"Role not found in context"}}`, http.StatusForbidden)
				return
			}

			userRole, ok := val.(string)
			if !ok {
				http.Error(w, `{"error":{"code":"forbidden","message":"Invalid role context"}}`, http.StatusForbidden)
				return
			}

			allowed := false
			for _, role := range allowedRoles {
				if userRole == role {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, `{"error":{"code":"forbidden","message":"Insufficient permissions"}}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
