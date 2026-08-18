package testlab

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc    *Service
	appEnv string
}

func NewHandler(svc *Service, appEnv string) *Handler {
	return &Handler{
		svc:    svc,
		appEnv: appEnv,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.handleListScenarios)
	r.Post("/apply", h.handleRunScenario)
	r.Get("/{runId}", h.handleGetScenario)
	r.Delete("/{runId}", h.handleCleanup)
}

func (h *Handler) handleListScenarios(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		http.Error(w, `{"error":{"code":"forbidden","message":"Test Lab is disabled in production"}}`, http.StatusForbidden)
		return
	}
	
	// Hardcoded definitions for Stage 1
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"items":[{"code":"BASIC_SALES","name":"Basic Sales"},{"code":"NEVER_SOLD","name":"Never Sold"},{"code":"ZERO_CURRENT_PERIOD","name":"Zero Current Period"},{"code":"INVENTORY_AND_INBOUND","name":"Inventory and Inbound"}]}`))
}

func (h *Handler) handleGetScenario(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		http.Error(w, `{"error":{"code":"forbidden","message":"Test Lab is disabled in production"}}`, http.StatusForbidden)
		return
	}
	// Stub since memory isn't fully persisted for GET yet
	runID := chi.URLParam(r, "runId")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"runId":"` + runID + `","status":"stub"}`))
}

func (h *Handler) handleRunScenario(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		http.Error(w, `{"error":{"code":"forbidden","message":"Test Lab is disabled in production"}}`, http.StatusForbidden)
		return
	}

	val := r.Context().Value("userID")
	adminUserID, ok := val.(uuid.UUID)
	if !ok {
		http.Error(w, `{"error":{"code":"unauthorized","message":"Invalid user context"}}`, http.StatusUnauthorized)
		return
	}

	var cfg ScenarioConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, `{"error":{"code":"invalid_request","message":"Invalid JSON body"}}`, http.StatusBadRequest)
		return
	}

	run, err := h.svc.RunScenario(r.Context(), adminUserID, cfg)
	if err != nil {
		http.Error(w, `{"error":{"code":"internal_error","message":"`+err.Error()+`"}}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

func (h *Handler) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		http.Error(w, `{"error":{"code":"forbidden","message":"Test Lab is disabled in production"}}`, http.StatusForbidden)
		return
	}

	runID := chi.URLParam(r, "runId")
	if runID == "" {
		http.Error(w, `{"error":{"code":"invalid_request","message":"Missing runId parameter"}}`, http.StatusBadRequest)
		return
	}

	err := h.svc.Cleanup(r.Context(), runID)
	if err != nil {
		http.Error(w, `{"error":{"code":"internal_error","message":"`+err.Error()+`"}}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
