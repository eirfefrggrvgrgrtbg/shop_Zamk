package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	requestIDKey contextKey = "zamk.request_id"
	routeKey     contextKey = "zamk.route"
)

// WithRequestID returns a new context with the given request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Value(requestIDKey).(string); ok {
		return val
	}
	return ""
}

// WithRoute returns a new context with the resolved route pattern.
func WithRoute(ctx context.Context, route string) context.Context {
	return context.WithValue(ctx, routeKey, route)
}

// RouteFromContext extracts the route pattern from the context.
func RouteFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Value(routeKey).(string); ok {
		return val
	}
	return ""
}

// TraceIDFromContext extracts the trace ID from the active OpenTelemetry span.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		return sc.TraceID().String()
	}
	return ""
}

// SpanIDFromContext extracts the span ID from the active OpenTelemetry span.
func SpanIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		return sc.SpanID().String()
	}
	return ""
}
