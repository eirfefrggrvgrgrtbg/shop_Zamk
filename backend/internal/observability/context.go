package observability

import (
	"context"
	"fmt"
	"regexp"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	requestIDKey contextKey = "zamk.request_id"
	routeKey     contextKey = "zamk.route"
	actorIDKey   contextKey = "zamk.actor_id"
	actorRoleKey contextKey = "zamk.actor_role"
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

// WithActor returns a new context with the given actor ID and role.
// Never pass customer PII (phone, email, full address) or secrets.
func WithActor(ctx context.Context, actorID, actorRole string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, actorIDKey, actorID)
	return context.WithValue(ctx, actorRoleKey, actorRole)
}

// ActorFromContext extracts actor ID and actor role from the context.
// Never extracts customer PII (email, name, phone) or tokens.
func ActorFromContext(ctx context.Context) (actorID, actorRole string) {
	if ctx == nil {
		return "", ""
	}
	if id, ok := ctx.Value(actorIDKey).(string); ok {
		actorID = id
	}
	if role, ok := ctx.Value(actorRoleKey).(string); ok {
		actorRole = role
	}
	// Safe bridge for standard authentication middleware context keys ("userID", "role")
	if actorID == "" {
		if stringer, ok := ctx.Value("userID").(fmt.Stringer); ok && stringer != nil {
			actorID = stringer.String()
		} else if str, ok := ctx.Value("userID").(string); ok {
			actorID = str
		}
	}
	if actorRole == "" {
		if roleStr, ok := ctx.Value("role").(string); ok {
			actorRole = roleStr
		}
	}
	return actorID, actorRole
}

var (
	// canonical ZMU format: "ZMU-" followed by identifier
	canonicalZMURegex = regexp.MustCompile(`^ZMU-[A-Za-z0-9_-]{1,32}$`)
	// canonical ZMK / SKU namespace
	canonicalZMKRegex     = regexp.MustCompile(`^ZMK-[A-Za-z0-9_-]{1,32}$`)
	canonicalSKURegex     = regexp.MustCompile(`^(?:[A-Za-z0-9]+-)?SKU-[A-Za-z0-9_-]{1,32}$`)
	canonicalBarcodeRegex = regexp.MustCompile(`^BARCODE-[A-Za-z0-9_-]{1,32}$`)
	// numeric EAN: 8, 12, 13, 14 digits
	canonicalEANRegex = regexp.MustCompile(`^(?:[0-9]{8}|[0-9]{12}|[0-9]{13}|[0-9]{14})$`)
)

// IsCanonicalScannerCode returns true if code matches domain-canonical formats:
// - Exact ZMU format or namespace: "ZMU-" + identifier
// - Canonical ZMK namespace: "ZMK-" + identifier
// - Canonical SKU namespace: "SKU-" + identifier
// - Canonical BARCODE namespace: "BARCODE-" + identifier
// - Canonical numeric EAN/UPC barcodes: 8, 12, 13, or 14 digits
// Any arbitrary string (e.g. PRIVATE_INTERNAL_VALUE_123, SQL injections, PII) returns false.
func IsCanonicalScannerCode(code string) bool {
	return canonicalZMURegex.MatchString(code) ||
		canonicalZMKRegex.MatchString(code) ||
		canonicalSKURegex.MatchString(code) ||
		canonicalBarcodeRegex.MatchString(code) ||
		canonicalEANRegex.MatchString(code)
}
