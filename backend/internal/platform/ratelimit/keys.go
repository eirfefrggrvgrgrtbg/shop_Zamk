package ratelimit

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type KeyFunc func(r *http.Request) string

func LoginKey(r *http.Request) string {
	email := normalizedEmailFromJSON(r, "email")
	if email != "" {
		return joinKey("auth:login", clientIP(r), hashPart(email))
	}
	return joinKey("auth:login", clientIP(r))
}

func RegisterKey(r *http.Request) string {
	email := normalizedEmailFromJSON(r, "email")
	if email != "" {
		_, domain, ok := strings.Cut(email, "@")
		if ok && domain != "" {
			return joinKey("auth:register", clientIP(r), hashPart(domain))
		}
	}
	return joinKey("auth:register", clientIP(r))
}

func RefreshKey(r *http.Request) string {
	if cookie, err := r.Cookie("zamk_refresh_token"); err == nil && cookie.Value != "" {
		return joinKey("auth:refresh", clientIP(r), hashPart(cookie.Value))
	}
	return joinKey("auth:refresh", clientIP(r))
}

func UserIPKey(group string) KeyFunc {
	return func(r *http.Request) string {
		userID := userIDFromContext(r)
		if userID != "" {
			return joinKey(group, userID, clientIP(r))
		}
		return joinKey(group, clientIP(r))
	}
}

func IPKey(group string) KeyFunc {
	return func(r *http.Request) string {
		return joinKey(group, clientIP(r))
	}
}

func SafeKeyLabel(key string) string {
	return hashPart(key)
}

func normalizedEmailFromJSON(r *http.Request, field string) string {
	value, ok := JSONStringField(r, field)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func userIDFromContext(r *http.Request) string {
	raw := r.Context().Value("userID")
	switch v := raw.(type) {
	case uuid.UUID:
		return v.String()
	case string:
		return v
	default:
		return ""
	}
}

func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return sanitizePart(ip)
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		if first = strings.TrimSpace(first); first != "" {
			return sanitizePart(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return sanitizePart(host)
	}
	return sanitizePart(r.RemoteAddr)
}

func joinKey(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			clean = append(clean, sanitizePart(part))
		}
	}
	return "rl:" + strings.Join(clean, ":")
}

func sanitizePart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ":", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	value = strings.ReplaceAll(value, "\r", "_")
	if value == "" {
		return "unknown"
	}
	return value
}

func hashPart(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}
