package behavior

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HandleIngest(w http.ResponseWriter, r *http.Request) {
	// Strict JSON decoding to disallow unknown fields
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req EventIngestionRequest
	if err := decoder.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			h.writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body exceeds maximum allowed size of 256 KiB")
			return
		}
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body or forbidden fields: "+err.Error())
		return
	}

	var userID *uuid.UUID
	if val := r.Context().Value("userID"); val != nil {
		if uid, ok := val.(uuid.UUID); ok {
			userID = &uid
		}
	}

	resp, err := h.service.IngestEvents(r.Context(), userID, req)
	if err != nil {
		var structErr *StructuralError
		if errors.As(err, &structErr) {
			h.writeError(w, http.StatusBadRequest, structErr.Code, structErr.Message)
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to ingest events")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}
