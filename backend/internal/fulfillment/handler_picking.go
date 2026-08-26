package fulfillment

import (
	"encoding/json"
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
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	adminID, ok := adminIDVal.(uuid.UUID)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	fIDStr := chi.URLParam(r, "id")
	fulfillmentID, err := uuid.Parse(fIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid fulfillment ID"}`, http.StatusBadRequest)
		return
	}

	var req ScanPickingCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	res, err := h.svc.ScanPickingCode(r.Context(), adminID, fulfillmentID, req.Code)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		switch err {
		case ErrPickingNotAllowed:
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"picking_not_allowed"}`))
		case ErrInvariantViolation:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"invariant_violation"}`))
		case ErrUnitNotInWarehouse:
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"unit_not_in_warehouse"}`))
		case ErrUnitNotAllocatedToFulfillment:
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"unit_not_allocated_to_fulfillment"}`))
		case ErrUnitAllocatedToOtherOrder:
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"unit_allocated_to_other_order"}`))
		case ErrAmbiguousPickingCode:
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"ambiguous_picking_code"}`))
		case ErrCodeNotFound:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"picking_code_not_found"}`))
		case ErrCannotPickSerializedWithBarcode:
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"cannot_pick_serialized_with_barcode"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal server error"}`))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
