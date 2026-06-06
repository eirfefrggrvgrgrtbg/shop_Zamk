package ratelimit

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type Middleware struct {
	limiter              *Limiter
	enabled              bool
	failOpenOnRedisError bool
	logger               *slog.Logger
}

type Rule struct {
	Group  string
	Limit  int
	Window time.Duration
	Key    KeyFunc
}

func NewMiddleware(limiter *Limiter, enabled bool, failOpenOnRedisError bool, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &Middleware{
		limiter:              limiter,
		enabled:              enabled,
		failOpenOnRedisError: failOpenOnRedisError,
		logger:               logger,
	}
}

func (m *Middleware) Limit(rule Rule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.enabled || m.limiter == nil || rule.Limit <= 0 || rule.Window <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			keyFunc := rule.Key
			if keyFunc == nil {
				keyFunc = IPKey(rule.Group)
			}
			key := keyFunc(r)

			result, err := m.limiter.Allow(r.Context(), key, rule.Limit, rule.Window)
			if err != nil {
				if errors.Is(err, ErrLimited) {
					m.logLimited(r, rule, key, result)
					writeRateLimited(w, result.RetryAfter)
					return
				}
				if m.failOpenOnRedisError {
					m.logger.Warn("rate limiter redis error; allowing request",
						"group", rule.Group,
						"key", SafeKeyLabel(key),
						"ip", clientIP(r),
						"userID", userIDFromContext(r),
						"limit", rule.Limit,
						"window", rule.Window.String(),
						"requestID", requestID(r),
						"error", err.Error(),
					)
					next.ServeHTTP(w, r)
					return
				}
				m.logger.Error("rate limiter redis error; rejecting request",
					"group", rule.Group,
					"key", SafeKeyLabel(key),
					"ip", clientIP(r),
					"userID", userIDFromContext(r),
					"limit", rule.Limit,
					"window", rule.Window.String(),
					"requestID", requestID(r),
					"error", err.Error(),
				)
				writeRateLimited(w, rule.Window)
				return
			}

			if !result.Allowed {
				m.logLimited(r, rule, key, result)
				writeRateLimited(w, result.RetryAfter)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) logLimited(r *http.Request, rule Rule, key string, result Result) {
	m.logger.Warn("request rate limited",
		"group", rule.Group,
		"key", SafeKeyLabel(key),
		"ip", clientIP(r),
		"userID", userIDFromContext(r),
		"limit", rule.Limit,
		"window", rule.Window.String(),
		"retryAfter", result.RetryAfter.String(),
		"requestID", requestID(r),
	)
}

func writeRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		seconds := int(retryAfter.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "rate_limited",
			"message": "Too many requests. Please try again later.",
		},
	})
}

func requestID(r *http.Request) string {
	if value := r.Context().Value("requestID"); value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}
	if value := r.Header.Get("X-Request-ID"); value != "" {
		return value
	}
	return ""
}
