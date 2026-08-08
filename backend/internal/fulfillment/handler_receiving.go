package fulfillment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) ResolveReceivingCode(w http.ResponseWriter, r *http.Request) {
	var req ResolveReceivingCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
		return
	}

	f, err := h.svc.ResolveReceivingCode(r.Context(), req.Code)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "fulfillment_not_found", "Сборка не найдена")
			return
		}
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *Handler) StartReceiving(w http.ResponseWriter, r *http.Request) {
	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	var staffID *uuid.UUID
	if val := r.Context().Value("userID"); val != nil {
		id := val.(uuid.UUID)
		staffID = &id
	}

	sess, err := h.svc.StartReceivingSession(r.Context(), staffID, fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "fulfillment_not_found", "Сборка не найдена")
			return
		}
		if errors.Is(err, ErrFulfillmentAlreadyReceived) {
			h.writeError(w, http.StatusConflict, "fulfillment_already_received", "Сборка уже принята")
			return
		}
		if errors.Is(err, ErrFulfillmentNotPacked) {
			h.writeError(w, http.StatusBadRequest, "fulfillment_not_packed", "Сборка ещё не упакована")
			return
		}
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess)
}

func (h *Handler) ScanReceivingItem(w http.ResponseWriter, r *http.Request) {
	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	var req ScanItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
		return
	}

	sess, err := h.svc.ScanReceivingItem(r.Context(), fulfillmentID, req)
	if err != nil {
		if errors.Is(err, ErrReceivingNotStarted) {
			h.writeError(w, http.StatusBadRequest, "receiving_not_started", "Приёмка ещё не начата")
			return
		}
		if errors.Is(err, ErrInvalidBarcode) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_barcode", "Штрихкод не совпадает ни с одной позицией в этой сборке")
			return
		}
		if errors.Is(err, ErrExcessQuantity) {
			h.writeError(w, http.StatusConflict, "excess_quantity", "Отсканированное количество превышает ожидаемое по сборке")
			return
		}
		if errors.Is(err, ErrVersionConflict) {
			h.writeError(w, http.StatusConflict, "version_conflict", "Конфликт версий приёмки")
			return
		}
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess)
}

func (h *Handler) ConfirmReceiving(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	staffID := val.(uuid.UUID)

	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	var req ConfirmReceivingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
		return
	}

	shipment, err := h.svc.ConfirmReceiving(r.Context(), staffID, fulfillmentID, req)
	if err != nil {
		if errors.Is(err, ErrFulfillmentAlreadyReceived) {
			h.writeError(w, http.StatusConflict, "fulfillment_already_received", "Сборка уже принята")
			return
		}
		if errors.Is(err, ErrShipmentExists) {
			h.writeError(w, http.StatusConflict, "shipment_already_exists", "Отправление для этой сборки уже существует")
			return
		}
		if errors.Is(err, ErrVersionConflict) {
			h.writeError(w, http.StatusConflict, "version_conflict", "Конфликт версий приёмки")
			return
		}
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "accepted",
		"shipmentId": shipment.ID,
		"shipment":   shipment,
	})
}

func (h *Handler) RecordDiscrepancy(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	staffID := val.(uuid.UUID)

	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	var req RecordDiscrepancyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
		return
	}

	if req.Reason == "" {
		h.writeError(w, http.StatusBadRequest, "missing_reason", "Укажите причину расхождения")
		return
	}

	err = h.svc.RecordDiscrepancy(r.Context(), staffID, fulfillmentID, req)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "discrepancy_recorded"})
}
