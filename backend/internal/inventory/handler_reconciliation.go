package inventory

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) StartReconciliation(w http.ResponseWriter, r *http.Request) {
	var req StartReconciliationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.VariantID == uuid.Nil {
		h.writeError(w, http.StatusBadRequest, "invalid_variant_id", "variantId is required")
		return
	}

	adminIDVal := r.Context().Value("userID")
	if adminIDVal == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user id required")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "invalid user id")
		return
	}
	sessionID := uuid.New()

	err := h.service.StartReconciliationSession(r.Context(), sessionID, req.VariantID, adminID)
	if err != nil {
		if errors.Is(err, ErrReconciliationAlreadyActive) {
			h.writeError(w, http.StatusConflict, "reconciliation_already_active", err.Error())
			return
		}
		if errors.Is(err, ErrLegacyReconciliationNotAllowed) {
			h.writeError(w, http.StatusBadRequest, "legacy_reconciliation_not_allowed", err.Error())
			return
		}
		if errors.Is(err, ErrInventoryItemNotFound) {
			h.writeError(w, http.StatusNotFound, "variant_not_found", "variant not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	session, err := h.service.GetReconciliationSessionByID(r.Context(), sessionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (h *Handler) GetReconciliation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid session id")
		return
	}

	session, err := h.service.GetReconciliationSessionByID(r.Context(), sessionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if session == nil {
		h.writeError(w, http.StatusNotFound, "not_found", "session not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (h *Handler) ScanReconciliation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid session id")
		return
	}

	var req struct {
		RawCode string `json:"rawCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	adminID := r.Context().Value("userID").(uuid.UUID)

	res, err := h.service.ProcessReconciliationScan(r.Context(), sessionID, req.RawCode, adminID)
	if err != nil {
		if errors.Is(err, ErrReconciliationNotInProgress) {
			h.writeError(w, http.StatusBadRequest, "reconciliation_not_in_progress", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) MoveReconciliationToReview(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid session id")
		return
	}

	adminID := r.Context().Value("userID").(uuid.UUID)
	err = h.service.MoveReconciliationToReview(r.Context(), sessionID, adminID)
	if err != nil {
		if errors.Is(err, ErrInvalidReconciliationState) {
			h.writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) CancelReconciliation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid session id")
		return
	}

	adminID := r.Context().Value("userID").(uuid.UUID)
	err = h.service.CancelReconciliationSession(r.Context(), sessionID, adminID)
	if err != nil {
		if errors.Is(err, ErrInvalidReconciliationState) {
			h.writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) CompleteReconciliation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid session id")
		return
	}

	adminID := r.Context().Value("userID").(uuid.UUID)
	err = h.service.CompleteReconciliationSession(r.Context(), sessionID, adminID)
	if err != nil {
		if errors.Is(err, ErrInvalidReconciliationState) {
			h.writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) GetReconciliationReview(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid session id")
		return
	}

	review, err := h.service.GetReconciliationReview(r.Context(), sessionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

func (h *Handler) GetActiveReconciliation(w http.ResponseWriter, r *http.Request) {
	variantIDStr := r.URL.Query().Get("variantId")
	variantID, err := uuid.Parse(variantIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid variant id")
		return
	}

	session, err := h.service.GetActiveReconciliationSession(r.Context(), variantID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session) // returns null if no active session
}

func (h *Handler) ListReconciliations(w http.ResponseWriter, r *http.Request) {
	variantIDStr := r.URL.Query().Get("variantId")
	variantID, err := uuid.Parse(variantIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid variant id")
		return
	}

	limit := 10
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	sessions, err := h.service.ListReconciliationSessionsByVariant(r.Context(), variantID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListReconciliationSessionsResponse{Items: sessions})
}
