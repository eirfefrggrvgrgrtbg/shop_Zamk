package fulfillment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

func (h *Handler) DispatchFulfillment(w http.ResponseWriter, r *http.Request) {
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

	res, err := h.svc.DispatchFulfillment(r.Context(), adminID, fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "fulfillment_not_found", "Fulfillment not found")
			return
		}
		if errors.Is(err, ErrDispatchNotAllowed) {
			h.writeError(w, http.StatusConflict, "dispatch_not_allowed", "Dispatch is not allowed for this fulfillment or order state")
			return
		}
		if errors.Is(err, ErrFulfillmentNotFullyPicked) {
			h.writeError(w, http.StatusConflict, "fulfillment_not_fully_picked", "Fulfillment is not fully picked")
			return
		}
		if errors.Is(err, ErrInventoryUnitStateConflict) {
			h.writeError(w, http.StatusConflict, "inventory_unit_state_conflict", "One or more allocated inventory units are not in warehouse state")
			return
		}
		if errors.Is(err, ErrInsufficientTotalStock) {
			h.writeError(w, http.StatusConflict, "insufficient_total_stock", "Insufficient total stock for dispatch")
			return
		}
		if errors.Is(err, ErrInsufficientReservedStock) {
			h.writeError(w, http.StatusConflict, "insufficient_reserved_stock", "Insufficient reserved stock for dispatch")
			return
		}
		if errors.Is(err, ErrShipmentContradictoryState) {
			h.writeError(w, http.StatusConflict, "contradictory_shipment_state", "Shipment is in a contradictory state")
			return
		}
		if errors.Is(err, ErrInvariantViolation) {
			h.writeError(w, http.StatusInternalServerError, "invariant_violation", "Invariant violation in allocation data")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if h.auditRepo != nil {
		fid := fulfillmentID
		_ = h.auditRepo.RecordAudit(r.Context(), staff.AuditEvent{
			ActorUserID: adminID,
			Action:      "fulfillment.dispatched",
			EntityType:  "order_fulfillment",
			EntityID:    &fid,
			Metadata:    staff.SanitizeMetadata(map[string]any{"fromStatus": "packed", "toStatus": "shipped", "shipmentId": res.ShipmentID.String(), "actorRole": "admin"}),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}
