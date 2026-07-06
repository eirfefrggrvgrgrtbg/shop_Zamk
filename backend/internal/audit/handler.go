package audit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) HandleListLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	action := r.URL.Query().Get("action")
	entityType := r.URL.Query().Get("entityType")

	var actorID *uuid.UUID
	if val := r.URL.Query().Get("actorUserId"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			actorID = &id
		}
	}

	var entityID *uuid.UUID
	if val := r.URL.Query().Get("entityId"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			entityID = &id
		}
	}

	var dateFrom *time.Time
	if val := r.URL.Query().Get("dateFrom"); val != "" {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			dateFrom = &t
		}
	}

	var dateTo *time.Time
	if val := r.URL.Query().Get("dateTo"); val != "" {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			dateTo = &t
		}
	}

	p := pagination.FromRequest(r)

	filters := AuditLogFilters{
		Query:      query,
		ActorID:    actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		Limit:      p.Limit,
		Offset:     p.Offset,
	}

	res, err := h.service.ListLogs(r.Context(), filters)
	if err != nil {
		http.Error(w, "Failed to list audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
