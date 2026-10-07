package marketing

import (
	"net/http"
	"time"
)

func (h *Handler) GetAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'from' date format")
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'to' date format")
		return
	}

	res, err := h.service.Analytics.GetOverview(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get analytics overview", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch analytics")
		return
	}

	h.writeJSON(w, http.StatusOK, res)
}

func (h *Handler) GetAnalyticsSources(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'from' date format")
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'to' date format")
		return
	}

	res, err := h.service.Analytics.GetSources(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get analytics sources", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch sources analytics")
		return
	}

	h.writeJSON(w, http.StatusOK, res)
}

func (h *Handler) GetAnalyticsCampaigns(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'from' date format")
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'to' date format")
		return
	}

	res, err := h.service.Analytics.GetCampaigns(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get analytics campaigns", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch campaigns analytics")
		return
	}

	h.writeJSON(w, http.StatusOK, res)
}

func (h *Handler) GetAnalyticsTrend(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'from' date format")
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "Invalid 'to' date format")
		return
	}

	res, err := h.service.Analytics.GetTrend(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get analytics trend", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch analytics trend")
		return
	}

	h.writeJSON(w, http.StatusOK, res)
}
