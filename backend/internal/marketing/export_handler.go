package marketing

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (h *Handler) ExportQuery(w http.ResponseWriter, r *http.Request) {
	var req ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		if valErr, ok := err.(*QueryValidationError); ok && valErr != nil {
			h.writeError(w, http.StatusBadRequest, valErr.Code, valErr.Message)
			return
		}
		h.writeError(w, http.StatusBadRequest, "invalid_export_request", err.Error())
		return
	}

	data, contentType, filename, err := h.service.Analytics.ExportQuery(r.Context(), req)
	if err != nil {
		if valErr, ok := err.(*QueryValidationError); ok && valErr != nil {
			h.writeError(w, http.StatusBadRequest, valErr.Code, valErr.Message)
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to export query", "error", err)
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
