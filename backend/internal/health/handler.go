package health

import (
	"encoding/json"
	"net/http"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

type Handler struct {
	pg    *postgres.Client
	redis *redis.Client
}

func NewHandler(pg *postgres.Client, redis *redis.Client) *Handler {
	return &Handler{
		pg:    pg,
		redis: redis,
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "zamk-api",
	})
}

func (h *Handler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pgStatus := "ok"
	if err := h.pg.Ping(r.Context()); err != nil {
		pgStatus = "down"
	}

	redisStatus := "ok"
	if err := h.redis.Ping(r.Context()); err != nil {
		redisStatus = "down"
	}

	statusCode := http.StatusOK
	status := "ok"
	if pgStatus == "down" || redisStatus == "down" {
		statusCode = http.StatusServiceUnavailable
		status = "error"
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   status,
		"postgres": pgStatus,
		"redis":    redisStatus,
	})
}
