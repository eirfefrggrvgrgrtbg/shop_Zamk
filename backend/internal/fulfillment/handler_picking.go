package fulfillment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ScanPickingCodeRequest struct {
	Code string `json:"code"`
}

func (h *Handler) ScanPickingCode(w http.ResponseWriter, r *http.Request) {
	adminIDVal := r.Context().Value("userID")
	if adminIDVal == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	fIDStr := chi.URLParam(r, "id")
	fulfillmentID, err := uuid.Parse(fIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid fulfillment ID")
		return
	}

	var req ScanPickingCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
		return
	}

	res, err := h.svc.ScanPickingCode(r.Context(), adminID, fulfillmentID, req.Code)
	if err != nil {
		if errors.Is(err, ErrPickingNotAllowed) {
			h.writeError(w, http.StatusConflict, "picking_not_allowed", "Picking is not allowed for this fulfillment state")
			return
		}
		if errors.Is(err, ErrUnitNotInWarehouse) {
			h.writeError(w, http.StatusConflict, "unit_not_in_warehouse", "Unit is not in warehouse")
			return
		}
		if errors.Is(err, ErrUnitNotAllocatedToFulfillment) {
			h.writeError(w, http.StatusConflict, "unit_not_allocated_to_fulfillment", "Unit is not allocated to this fulfillment")
			return
		}
		if errors.Is(err, ErrUnitAllocatedToOtherOrder) {
			h.writeError(w, http.StatusConflict, "unit_allocated_to_other_order", "Unit is allocated to another order")
			return
		}
		if errors.Is(err, ErrAmbiguousPickingCode) {
			h.writeError(w, http.StatusConflict, "ambiguous_picking_code", "Ambiguous picking code")
			return
		}
		if errors.Is(err, ErrCodeNotFound) {
			h.writeError(w, http.StatusNotFound, "picking_code_not_found", "Picking code not found")
			return
		}
		if errors.Is(err, ErrCannotPickSerializedWithBarcode) {
			h.writeError(w, http.StatusConflict, "cannot_pick_serialized_with_barcode", "Cannot pick serialized item with variant barcode")
			return
		}
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "fulfillment_not_found", "Fulfillment not found")
			return
		}
		if errors.Is(err, ErrInvariantViolation) {
			h.writeError(w, http.StatusInternalServerError, "invariant_violation", "Invariant violation in allocation data")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) GetPickingOrder(w http.ResponseWriter, r *http.Request) {
	fIDStr := chi.URLParam(r, "id")
	fulfillmentID, err := uuid.Parse(fIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid fulfillment ID")
		return
	}

	po, err := h.svc.GetPickingOrder(r.Context(), fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "fulfillment_not_found", "Fulfillment not found")
			return
		}
		if errors.Is(err, ErrPickingNotAllowed) {
			h.writeError(w, http.StatusConflict, "picking_not_allowed", "Picking is not allowed for this fulfillment state")
			return
		}
		if errors.Is(err, ErrInvariantViolation) {
			h.writeError(w, http.StatusInternalServerError, "invariant_violation", "Invariant violation in allocation data")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(po)
}
