package supplies

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func (h *Handler) MarkArrived(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	adminID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	supplyIDStr := chi.URLParam(r, "supplyId")
	supplyID, err := uuid.Parse(supplyIDStr)
	if err != nil {
		http.Error(w, "Invalid supply ID", http.StatusBadRequest)
		return
	}

	err = h.svc.MarkSupplyArrived(r.Context(), adminID, supplyID)
	if err != nil {
		if err == ErrInvalidStatus {
			h.logger.Error("supply arrival failed", "event", "supply_arrival_invalid_status", "error", err.Error(), "supply_id", supplyID.String())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info("supply marked as arrived", "event", "supply_arrived", "supply_id", supplyID.String())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LookupSupply(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	qrToken := r.URL.Query().Get("qr_token")
	if qrToken == "" {
		http.Error(w, "qr_token is required", http.StatusBadRequest)
		return
	}
	supply, err := h.svc.repo.GetSupplyByQRToken(r.Context(), qrToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supply)
}

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
	reqID := middleware.GetReqID(r.Context())

	qrToken := r.URL.Query().Get("qr_token")
	if qrToken == "" {
		http.Error(w, "qr_token is required", http.StatusBadRequest)
		return
	}

	session, err := h.svc.StartReceivingSession(r.Context(), userID, qrToken)
	if err != nil {
		if err == ErrInvalidStatus {
			h.logger.Error("supply receiving lookup failed due to invalid status",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_invalid_status",
				"status", http.StatusBadRequest,
			)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err == ErrSupplyNotArrived {
			h.logger.Error("supply receiving lookup failed not arrived",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_not_arrived",
				"status", http.StatusBadRequest,
			)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err == ErrSupplyNotFound {
			h.logger.Error("supply receiving lookup failed not found",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_not_found",
				"status", http.StatusNotFound,
			)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		h.logger.Error("supply receiving lookup failed",
			"event", "supply_receiving_lookup_failed",
			"request_id", reqID,
			"admin_id", userID.String(),
			"error_code", "internal_error",
			"status", http.StatusInternalServerError,
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	expectedUnits := 0
	for _, item := range session.Items {
		expectedUnits += item.ExpectedQuantity
	}

	h.logger.Info("supply receiving started",
		"event", "supply_receiving_started",
		"supply_id", session.SupplyID.String(),
		"session_id", session.ID.String(),
		"expected_units", expectedUnits,
	)

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
		if errors.Is(err, ErrSerializedFinalizeNotSupported) {
			h.writeError(w, http.StatusUnprocessableEntity, "serialized_finalize_not_supported", "serialized unit finalization is not enabled yet")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info("supply receiving completed",
		"event", "supply_receiving_completed",
		"session_id", sessionID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RecordSerializedScan(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid session ID")
		return
	}

	var req RecordSerializedScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.UnitCode == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "unitCode is required")
		return
	}

	res, err := h.svc.RecordSerializedScan(r.Context(), userID, sessionID, req)
	if err != nil {
		if errors.Is(err, ErrSerializedUnitCodeRequired) {
			h.writeError(w, http.StatusBadRequest, "serialized_unit_code_required", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidReceivingCondition) {
			h.writeError(w, http.StatusBadRequest, "invalid_receiving_condition", err.Error())
			return
		}
		if errors.Is(err, ErrUnitNotFound) {
			h.writeError(w, http.StatusNotFound, "unit_not_found", err.Error())
			return
		}
		if errors.Is(err, ErrUnitNotInSupply) {
			h.writeError(w, http.StatusConflict, "unit_not_in_supply", err.Error())
			return
		}
		if errors.Is(err, ErrUnitAlreadyScanned) {
			h.writeError(w, http.StatusConflict, "unit_already_scanned", err.Error())
			return
		}
		if errors.Is(err, ErrUnitAlreadyReceived) {
			h.writeError(w, http.StatusConflict, "unit_already_received", err.Error())
			return
		}
		if errors.Is(err, ErrReceivingSessionFinalized) {
			h.writeError(w, http.StatusConflict, "receiving_session_finalized", err.Error())
			return
		}
		if errors.Is(err, ErrSupplyUnitIdentityMismatch) {
			h.writeError(w, http.StatusUnprocessableEntity, "supply_unit_identity_mismatch", err.Error())
			return
		}
		if errors.Is(err, ErrSupplyNotSerialized) {
			h.writeError(w, http.StatusUnprocessableEntity, "supply_not_serialized", err.Error())
			return
		}
		if errors.Is(err, ErrSessionNotFound) {
			h.writeError(w, http.StatusNotFound, "session_not_found", err.Error())
			return
		}
		if errors.Is(err, ErrSupplyNotFound) {
			h.writeError(w, http.StatusNotFound, "supply_not_found", err.Error())
			return
		}
		if errors.Is(err, ErrItemNotFound) {
			h.writeError(w, http.StatusNotFound, "item_not_found", err.Error())
			return
		}

		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) GetRecentSerializedScans(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid session ID")
		return
	}

	limit := 10
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if parsedLimit, parseErr := strconv.Atoi(limitStr); parseErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	scans, err := h.svc.ListRecentSerializedScans(r.Context(), userID, sessionID, limit)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			h.writeError(w, http.StatusNotFound, "session_not_found", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scans)
}

func (h *Handler) UndoSerializedScan(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid session ID")
		return
	}

	scanIDStr := chi.URLParam(r, "scanId")
	scanID, err := uuid.Parse(scanIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid scan ID")
		return
	}

	res, err := h.svc.UndoSerializedScan(r.Context(), userID, sessionID, scanID)
	if err != nil {
		if errors.Is(err, ErrScanNotFound) {
			h.writeError(w, http.StatusNotFound, "scan_not_found", err.Error())
			return
		}
		if errors.Is(err, ErrScanAlreadyVoided) {
			h.writeError(w, http.StatusConflict, "scan_already_voided", err.Error())
			return
		}
		if errors.Is(err, ErrScanNotInSession) {
			h.writeError(w, http.StatusNotFound, "scan_not_in_session", err.Error())
			return
		}
		if errors.Is(err, ErrReceivingSessionFinalized) {
			h.writeError(w, http.StatusConflict, "receiving_session_finalized", err.Error())
			return
		}
		if errors.Is(err, ErrSessionNotFound) {
			h.writeError(w, http.StatusNotFound, "session_not_found", err.Error())
			return
		}

		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
