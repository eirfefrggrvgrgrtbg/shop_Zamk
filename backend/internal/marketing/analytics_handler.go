package marketing

import (
	"errors"
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

func (h *Handler) GetProductAnalytics(w http.ResponseWriter, r *http.Request) {
	req := ProductAnalyticsRequest{}

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
	if !to.After(from) {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "'to' date must be after 'from' date")
		return
	}

	req.From = from
	req.To = to

	if catID := r.URL.Query().Get("categoryId"); catID != "" {
		req.CategoryID = &catID
	}
	if desID := r.URL.Query().Get("designerId"); desID != "" {
		req.DesignerID = &desID
	}
	if search := r.URL.Query().Get("search"); search != "" {
		req.Search = &search
	}
	if sort := r.URL.Query().Get("sort"); sort != "" {
		req.Sort = &sort
	}
	if dir := r.URL.Query().Get("direction"); dir != "" {
		req.Direction = &dir
	}

	res, err := h.service.Analytics.GetProductAnalytics(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidSort) {
			h.writeError(w, http.StatusBadRequest, "invalid_sort", "Unsupported sort parameter")
			return
		}
		if errors.Is(err, ErrFavoritesCoverageIncomplete) {
			h.writeError(w, http.StatusBadRequest, "favorites_coverage_incomplete", "Sorting by favorites requires verified tracking coverage")
			return
		}
		h.logger.Error("failed to get product analytics", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch product analytics")
		return
	}

	h.writeJSON(w, http.StatusOK, res)
}

func (h *Handler) GetDesignerAnalytics(w http.ResponseWriter, r *http.Request) {
	req := DesignerAnalyticsRequest{}

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
	if !to.After(from) {
		h.writeError(w, http.StatusBadRequest, "invalid_date", "'to' date must be after 'from' date")
		return
	}

	req.From = from
	req.To = to

	if catID := r.URL.Query().Get("categoryId"); catID != "" {
		req.CategoryID = &catID
	}
	if search := r.URL.Query().Get("search"); search != "" {
		req.Search = &search
	}
	if sort := r.URL.Query().Get("sort"); sort != "" {
		req.Sort = &sort
	}

	res, err := h.service.Analytics.GetDesignerAnalytics(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidSort) {
			h.writeError(w, http.StatusBadRequest, "invalid_sort", "Unsupported sort parameter")
			return
		}
		if errors.Is(err, ErrFavoritesCoverageIncomplete) {
			h.writeError(w, http.StatusBadRequest, "favorites_coverage_incomplete", "Sorting by favorites requires verified tracking coverage")
			return
		}
		h.logger.Error("failed to get designer analytics", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch designer analytics")
		return
	}

	h.writeJSON(w, http.StatusOK, res)
}
