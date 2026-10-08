package marketing

import (
	"fmt"
	"strings"
	"time"
)

type QueryValidationError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
}

func (e *QueryValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewValidationError(code, message string) *QueryValidationError {
	return &QueryValidationError{
		Code:    code,
		Message: message,
	}
}

var allowedDimensionsMap = map[string]bool{
	"day":      true,
	"source":   true,
	"campaign": true,
	"product":  true,
	"designer": true,
	"category": true,
}

var allowedMetricsMap = map[string]bool{
	"sessions":       true,
	"orders":         true,
	"sold_units":     true,
	"revenue":        true,
	"product_views":  true,
	"favorites":      true,
	"add_to_cart":    true,
	"conversion":     true,
	"returns":        true,
	"returned_units": true,
}

var allowedOperatorsMap = map[string]bool{
	"eq":       true,
	"neq":      true,
	"in":       true,
	"contains": true,
}

func ValidateQueryRequest(req QueryRequest) *QueryValidationError {
	if req.Version != 1 {
		return NewValidationError("unsupported_version", "only version 1 is supported")
	}

	// Period validation
	if req.Period.From.IsZero() || req.Period.To.IsZero() {
		return NewValidationError("invalid_period", "from and to timestamps must be provided")
	}

	if !req.Period.From.Before(req.Period.To) {
		return NewValidationError("invalid_period", "from timestamp must be strictly before to timestamp")
	}

	if req.Period.To.Sub(req.Period.From) > 366*24*time.Hour {
		return NewValidationError("period_too_long", "query period cannot exceed 366 days")
	}

	// Dimensions validation
	if len(req.Dimensions) > 2 {
		return NewValidationError("too_many_dimensions", "maximum 2 dimensions allowed")
	}

	selectedDims := make(map[string]bool)
	for _, d := range req.Dimensions {
		if !allowedDimensionsMap[d] {
			return NewValidationError("invalid_dimension", fmt.Sprintf("unrecognized dimension: %s", d))
		}
		if selectedDims[d] {
			return NewValidationError("duplicate_dimension", fmt.Sprintf("duplicate dimension: %s", d))
		}
		selectedDims[d] = true
	}

	// Metrics validation
	if len(req.Metrics) == 0 {
		return NewValidationError("missing_metrics", "at least 1 metric must be requested")
	}

	if len(req.Metrics) > 8 {
		return NewValidationError("too_many_metrics", "maximum 8 metrics allowed")
	}

	selectedMetrics := make(map[string]bool)
	for _, m := range req.Metrics {
		if !allowedMetricsMap[m] {
			return NewValidationError("invalid_metric", fmt.Sprintf("unrecognized metric: %s", m))
		}
		if selectedMetrics[m] {
			return NewValidationError("duplicate_metric", fmt.Sprintf("duplicate metric: %s", m))
		}
		selectedMetrics[m] = true
	}

	// Check metric-dimension compatibility (Blocker 4 fail-closed)
	registryMetrics := GetRegistryMetrics()
	for _, m := range req.Metrics {
		def := findMetric(registryMetrics, m)
		if def != nil && def.IsAvailableWith != nil {
			if !def.IsAvailableWith(req.Dimensions) {
				return NewValidationError("unsupported_combination", fmt.Sprintf("metric %s is unsupported with dimensions %v", m, req.Dimensions))
			}
		}
	}

	// Filters validation
	if len(req.Filters) > 10 {
		return NewValidationError("too_many_filters", "maximum 10 filters allowed")
	}

	for _, f := range req.Filters {
		if !allowedDimensionsMap[f.Dimension] {
			return NewValidationError("invalid_filter_dimension", fmt.Sprintf("filter dimension not allowed: %s", f.Dimension))
		}

		// Explicit V1 rule: filter dimension must be in selected dimensions
		if !selectedDims[f.Dimension] {
			return NewValidationError("unsupported_filter_dimension", fmt.Sprintf("filter dimension %s must be present in selected dimensions", f.Dimension))
		}

		if !allowedOperatorsMap[f.Operator] {
			return NewValidationError("invalid_filter_operator", fmt.Sprintf("filter operator not allowed: %s", f.Operator))
		}

		if len(f.Values) == 0 {
			return NewValidationError("empty_filter_values", "filter values list cannot be empty")
		}

		for _, v := range f.Values {
			if strings.TrimSpace(v) == "" {
				return NewValidationError("empty_filter_values", "filter value cannot be empty string")
			}
		}
	}

	// Sort validation
	if len(req.Sort) > 2 {
		return NewValidationError("too_many_sort_keys", "maximum 2 sort keys allowed")
	}

	for _, s := range req.Sort {
		if !selectedDims[s.Field] && !selectedMetrics[s.Field] {
			return NewValidationError("invalid_sort_field", fmt.Sprintf("sort field %s must be in selected dimensions or metrics", s.Field))
		}

		dirLower := strings.ToLower(s.Direction)
		if dirLower != "asc" && dirLower != "desc" {
			return NewValidationError("invalid_sort_direction", fmt.Sprintf("sort direction must be asc or desc: %s", s.Direction))
		}
	}

	// Limit validation
	if req.Limit != nil {
		if *req.Limit <= 0 || *req.Limit > 1000 {
			return NewValidationError("invalid_limit", "limit must be strictly between 1 and 1000")
		}
	}

	return nil
}
