package observability

import (
	"context"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics manages operational HTTP metrics using low-cardinality labels.
type HTTPMetrics struct {
	requestsTotal   metric.Int64Counter
	requestDuration metric.Float64Histogram
}

// NewHTTPMetrics initializes HTTP metrics registered with the given MeterProvider.
func NewHTTPMetrics(meter metric.Meter) (*HTTPMetrics, error) {
	requestsTotal, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total count of processed HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	return &HTTPMetrics{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
	}, nil
}

// Record records request count and latency for an HTTP request.
// Only bounded, low-cardinality labels (method, route template, status code) are permitted.
func (m *HTTPMetrics) Record(ctx context.Context, method, route string, statusCode int, duration time.Duration) {
	if m == nil {
		return
	}

	if route == "" {
		route = "unmatched"
	}

	attrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("route", route),
		attribute.String("status", strconv.Itoa(statusCode)),
	)

	m.requestsTotal.Add(ctx, 1, attrs)
	m.requestDuration.Record(ctx, duration.Seconds(), attrs)
}
