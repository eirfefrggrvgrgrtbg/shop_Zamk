package pagination

import (
	"net/http"
	"strconv"
)

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

type Params struct {
	Limit  int
	Offset int
}

func FromRequest(r *http.Request) Params {
	query := r.URL.Query()
	limit := parsePositiveInt(query.Get("limit"), DefaultLimit)
	if limit > MaxLimit {
		limit = MaxLimit
	}
	offset := parseNonNegativeInt(query.Get("offset"), 0)

	return Params{Limit: limit, Offset: offset}
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseNonNegativeInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}
