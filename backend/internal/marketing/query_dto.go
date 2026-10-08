package marketing

import "time"

type QueryPeriod struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type QueryFilter struct {
	Dimension string   `json:"dimension"`
	Operator  string   `json:"operator"` // eq, neq, in, contains
	Values    []string `json:"values"`
}

type QuerySort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // asc, desc
}

type QueryRequest struct {
	Version    int           `json:"version"`
	Period     QueryPeriod   `json:"period"`
	Dimensions []string      `json:"dimensions"`
	Metrics    []string      `json:"metrics"`
	Filters    []QueryFilter `json:"filters,omitempty"`
	Sort       []QuerySort   `json:"sort,omitempty"`
	Limit      *int          `json:"limit,omitempty"`
}

type QueryColumn struct {
	Key  string `json:"key"`
	Kind string `json:"kind"` // dimension | metric
	Type string `json:"type"` // string, date, number, money, percentage
}

type QueryResponse struct {
	Version  int                       `json:"version"`
	Query    QueryRequest              `json:"query"`
	Columns  []QueryColumn             `json:"columns"`
	Rows     []map[string]interface{}  `json:"rows"`
	Coverage map[string]MetricCoverage `json:"coverage"`
	Warnings []string                  `json:"warnings"`
}
