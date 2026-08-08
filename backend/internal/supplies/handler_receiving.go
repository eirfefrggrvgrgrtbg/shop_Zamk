package supplies

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	qrToken := r.URL.Query().Get("qr_token")
	if qrToken == "" {
		http.Error(w, "qr_token is required", http.StatusBadRequest)
		return
	}

	session, err := h.svc.StartReceivingSession(r.Context(), userID, qrToken)
	if err != nil {
		if err == ErrInvalidStatus {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err == ErrSupplyNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (h *Handler) RecordScan(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	var req RecordReceivingScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.svc.RecordScan(r.Context(), userID, sessionID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) FinalizeSession(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	var req FinalizeReceivingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.svc.FinalizeReceiving(r.Context(), userID, sessionID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
