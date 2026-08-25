package supplies

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func (h *Handler) MarkArrived(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	supplyIDStr := chi.URLParam(r, "supplyId")
	supplyID, err := uuid.Parse(supplyIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid supply ID")
		return
	}

	err = h.svc.MarkSupplyArrived(r.Context(), adminID, supplyID)
	if err != nil {
		if errors.Is(err, ErrInvalidStatus) {
			h.logger.Error("supply arrival failed", "event", "supply_arrival_invalid_status", "error", err.Error(), "supply_id", supplyID.String())
			h.writeError(w, http.StatusBadRequest, "invalid_status", "Supply is not in shipped status")
			return
		}
		if errors.Is(err, ErrSupplyNotFound) {
			h.writeError(w, http.StatusNotFound, "supply_not_found", "Supply not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.logger.Info("supply marked as arrived", "event", "supply_arrived", "supply_id", supplyID.String())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LookupSupply(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || (role != "admin" && role != "super_admin") {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	qrToken := strings.TrimSpace(r.URL.Query().Get("qr_token"))
	if qrToken == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_receiving_code", "Введите номер поставки, грузоместа или отсканируйте QR-код.")
		return
	}
	header, err := h.svc.repo.GetSupplyByQRToken(r.Context(), qrToken)
	if err != nil {
		if errors.Is(err, ErrSupplyNotFound) {
			h.writeError(w, http.StatusNotFound, "supply_not_found", "Поставка или грузоместо не найдено.")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	supply, err := h.svc.repo.GetSupplyByID(r.Context(), header.ID)
	if err != nil {
		if errors.Is(err, ErrSupplyNotFound) {
			h.writeError(w, http.StatusNotFound, "supply_not_found", "Поставка или грузоместо не найдено.")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supply)
}

func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
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
	reqID := middleware.GetReqID(r.Context())

	qrToken := strings.TrimSpace(r.URL.Query().Get("qr_token"))
	if qrToken == "" && r.Body != nil && r.ContentLength > 0 {
		var body struct {
			QRToken string `json:"qr_token"`
			Token   string `json:"token"`
			Code    string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if body.QRToken != "" {
				qrToken = strings.TrimSpace(body.QRToken)
			} else if body.Token != "" {
				qrToken = strings.TrimSpace(body.Token)
			} else if body.Code != "" {
				qrToken = strings.TrimSpace(body.Code)
			}
		}
	}

	if qrToken == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_receiving_code", "Введите номер поставки, грузоместа или отсканируйте QR-код.")
		return
	}

	session, err := h.svc.StartReceivingSession(r.Context(), userID, qrToken)
	if err != nil {
		if errors.Is(err, ErrSupplyNotFound) {
			h.logger.Warn("supply receiving lookup failed not found",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_not_found",
				"status", http.StatusNotFound,
			)
			h.writeError(w, http.StatusNotFound, "supply_not_found", "Поставка или грузоместо не найдено.")
			return
		}
		if errors.Is(err, ErrSupplyNotArrived) {
			h.logger.Warn("supply receiving lookup failed not arrived",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_not_arrived",
				"status", http.StatusBadRequest,
			)
			h.writeError(w, http.StatusBadRequest, "supply_not_arrived", "Поставка ещё не прибыла на склад.")
			return
		}
		if errors.Is(err, ErrSupplyNotReadyForReceiving) {
			h.logger.Warn("supply receiving lookup failed not ready",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_not_ready_for_receiving",
				"status", http.StatusBadRequest,
			)
			h.writeError(w, http.StatusBadRequest, "supply_not_ready_for_receiving", "Поставка ещё не готова к приёмке.")
			return
		}
		if errors.Is(err, ErrSupplyAlreadyCompleted) {
			h.logger.Warn("supply receiving lookup failed already completed",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_already_completed",
				"status", http.StatusBadRequest,
			)
			h.writeError(w, http.StatusBadRequest, "supply_already_completed", "Приёмка по этой поставке уже завершена.")
			return
		}
		if errors.Is(err, ErrSupplyCancelled) {
			h.logger.Warn("supply receiving lookup failed cancelled",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_cancelled",
				"status", http.StatusBadRequest,
			)
			h.writeError(w, http.StatusBadRequest, "supply_cancelled", "Поставка отменена.")
			return
		}
		if errors.Is(err, ErrNoExpectedUnitsRemain) {
			h.logger.Warn("supply receiving lookup failed no expected units remain",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "no_expected_units_remain",
				"status", http.StatusBadRequest,
			)
			h.writeError(w, http.StatusBadRequest, "no_expected_units_remain", "Все ожидаемые товарные единицы по этой поставке уже приняты.")
			return
		}
		if errors.Is(err, ErrSupplyUnitIdentityMismatch) {
			h.logger.Error("supply receiving lookup failed identity mismatch",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_unit_identity_mismatch",
				"status", http.StatusUnprocessableEntity,
			)
			h.writeError(w, http.StatusUnprocessableEntity, "supply_unit_identity_mismatch", "Идентификаторы товарных единиц не совпадают с составом поставки.")
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			h.logger.Warn("supply receiving lookup failed invalid status",
				"event", "supply_receiving_lookup_failed",
				"request_id", reqID,
				"admin_id", userID.String(),
				"error_code", "supply_not_ready_for_receiving",
				"status", http.StatusBadRequest,
			)
			h.writeError(w, http.StatusBadRequest, "supply_not_ready_for_receiving", "Поставка ещё не готова к приёмке.")
			return
		}

		h.logger.Error("supply receiving lookup failed",
			"event", "supply_receiving_lookup_failed",
			"request_id", reqID,
			"admin_id", userID.String(),
			"error_code", "internal_error",
			"status", http.StatusInternalServerError,
		)
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
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

	var req RecordReceivingScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	err = h.svc.RecordScan(r.Context(), userID, sessionID, req)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			h.writeError(w, http.StatusNotFound, "session_not_found", err.Error())
			return
		}
		if errors.Is(err, ErrItemNotFound) {
			h.writeError(w, http.StatusNotFound, "item_not_found", err.Error())
			return
		}
		if errors.Is(err, ErrReceivingSessionFinalized) {
			h.writeError(w, http.StatusConflict, "receiving_session_finalized", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) FinalizeSession(w http.ResponseWriter, r *http.Request) {
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

	var req FinalizeReceivingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	err = h.svc.FinalizeReceiving(r.Context(), userID, sessionID, req)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			h.writeError(w, http.StatusNotFound, "session_not_found", err.Error())
			return
		}
		if errors.Is(err, ErrReceivingSessionFinalized) || err.Error() == "session is not active" {
			h.writeError(w, http.StatusConflict, "receiving_session_finalized", "session is not active")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
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

		h.logger.Error("supply receiving unit scan failed",
			"event", "supply_receiving_unit_scan_failed",
			"session_id", sessionID.String(),
			"error", err.Error(),
		)
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
		h.logger.Error("failed to list recent serialized scans",
			"event", "supply_receiving_list_scans_failed",
			"session_id", sessionID.String(),
			"error", err.Error(),
		)
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

		h.logger.Error("failed to undo serialized scan",
			"event", "supply_receiving_undo_scan_failed",
			"session_id", sessionID.String(),
			"scan_id", scanID.String(),
			"error", err.Error(),
		)
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
