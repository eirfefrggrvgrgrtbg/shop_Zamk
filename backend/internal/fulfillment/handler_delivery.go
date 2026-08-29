package fulfillment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) DeliverShipment(w http.ResponseWriter, r *http.Request) {
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

	sIDStr := chi.URLParam(r, "id")
	shipmentID, err := uuid.Parse(sIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid shipment ID")
		return
	}

	var req DeliverShipmentRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	res, err := h.svc.DeliverShipment(r.Context(), adminID, shipmentID, req)
	if err != nil {
		if errors.Is(err, ErrShipmentNotFound) {
			h.writeError(w, http.StatusNotFound, "shipment_not_found", "Shipment not found")
			return
		}
		if errors.Is(err, ErrShipmentNotLinkedToFulfillment) {
			h.writeError(w, http.StatusUnprocessableEntity, "shipment_not_linked_to_fulfillment", "Shipment is not linked to a fulfillment")
			return
		}
		if errors.Is(err, ErrShipmentAlreadyDelivered) {
			h.writeError(w, http.StatusConflict, "shipment_already_delivered", "Shipment is already delivered")
			return
		}
		if errors.Is(err, ErrDeliveryNotAllowed) {
			h.writeError(w, http.StatusConflict, "delivery_not_allowed", "Delivery is not allowed for this shipment or order state")
			return
		}
		if errors.Is(err, ErrFulfillmentNotShipped) {
			h.writeError(w, http.StatusConflict, "fulfillment_not_shipped", "Linked fulfillment is not in shipped status")
			return
		}
		if errors.Is(err, ErrShipmentContradictoryState) {
			h.writeError(w, http.StatusConflict, "contradictory_shipment_state", "Shipment or fulfillment is in a contradictory state")
			return
		}
		if errors.Is(err, ErrOrderCancelled) {
			h.writeError(w, http.StatusConflict, "order_cancelled", "Order is cancelled")
			return
		}
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "fulfillment_not_found", "Linked fulfillment not found")
			return
		}

		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}
