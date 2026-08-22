package products

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) GenerateSKUs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	var req GenerateSKUsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	skus, err := h.service.GenerateSKUs(r.Context(), userID, req.Count)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to generate skus")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GenerateSKUsResponse{SKUs: skus})
}
