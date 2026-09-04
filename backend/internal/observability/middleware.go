package observability

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// responseWriterInterceptor intercepts the HTTP status code and bytes written.
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

func (w *responseWriterInterceptor) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseWriterInterceptor) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// Middleware creates a unified HTTP middleware for Request ID, OpenTelemetry tracing,
// operational metrics, structured access logging, and safe panic recovery.
func Middleware(provider *Provider, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// 1. Request ID Handling
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = r.Header.Get("X-Request-Id")
			}
			if !IsValidRequestID(reqID) {
				reqID = NewRequestID()
			}

			w.Header().Set("X-Request-ID", reqID)
			ctx := WithRequestID(r.Context(), reqID)

			// 2. OpenTelemetry Tracing
			var span trace.Span
			if provider != nil && provider.Tracer != nil {
				// Propagate incoming W3C trace context
				ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

				spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
				ctx, span = provider.Tracer.Start(ctx, spanName,
					trace.WithSpanKind(trace.SpanKindServer),
					trace.WithAttributes(
						semconv.HTTPRequestMethodKey.String(r.Method),
						attribute.String("environment", provider.cfg.Environment),
						attribute.String("component", "http"),
					),
				)
				defer span.End()
			}

			ww := &responseWriterInterceptor{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// 3. Panic Recovery
			defer func() {
				if rec := recover(); rec != nil {
					stack := string(debug.Stack())
					route := resolveRoutePattern(r)

					if span != nil {
						span.RecordError(fmt.Errorf("panic: %v", rec))
						span.SetStatus(codes.Error, "panic")
					}

					// Log structured error for panic
					if logger != nil {
						logger.ErrorContext(ctx, "http request panic recovered",
							slog.String("component", "http"),
							slog.String("operation", "panic_recovery"),
							slog.String("route", route),
							slog.String("panic", fmt.Sprintf("%v", rec)),
							slog.String("stack", stack),
						)
					}

					// Generic response to client (never expose stack trace)
					if !ww.wroteHeader {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"An internal error occurred"}}`))
						ww.statusCode = http.StatusInternalServerError
					}

					// Record metric for 500
					if provider != nil && provider.Metrics != nil {
						provider.Metrics.Record(ctx, r.Method, route, http.StatusInternalServerError, time.Since(startTime))
					}
				}
			}()

			// Run next handler
			next.ServeHTTP(ww, r.WithContext(ctx))

			duration := time.Since(startTime)
			route := resolveRoutePattern(r)
			ctx = WithRoute(ctx, route)

			// Update span with resolved route pattern and HTTP status code
			if span != nil {
				span.SetName(fmt.Sprintf("%s %s", r.Method, route))
				span.SetAttributes(
					semconv.HTTPRouteKey.String(route),
					semconv.HTTPResponseStatusCodeKey.Int(ww.statusCode),
				)
				if ww.statusCode >= 500 {
					span.SetStatus(codes.Error, http.StatusText(ww.statusCode))
				} else {
					span.SetStatus(codes.Ok, "")
				}
			}

			// 4. Record Operational HTTP Metrics
			if provider != nil && provider.Metrics != nil {
				provider.Metrics.Record(ctx, r.Method, route, ww.statusCode, duration)
			}

			// 5. Structured HTTP Access Log
			if logger != nil {
				logAttrs := []any{
					slog.String("component", "http"),
					slog.String("method", r.Method),
					slog.String("route", route),
					slog.Int("status", ww.statusCode),
					slog.Int64("duration_ms", duration.Milliseconds()),
					slog.Int64("bytes", ww.bytesWritten),
				}

				switch {
				case ww.statusCode >= 500:
					logger.ErrorContext(ctx, "http request completed", logAttrs...)
				case ww.statusCode >= 400:
					// Normal expected 4xx (404, 409, 400) logged as WARN, preserving semantic distinction
					logger.WarnContext(ctx, "http request completed", logAttrs...)
				default:
					logger.InfoContext(ctx, "http request completed", logAttrs...)
				}
			}
		})
	}
}

// resolveRoutePattern extracts the chi route template, or falls back to safe path if static.
func resolveRoutePattern(r *http.Request) string {
	if rCtx := chi.RouteContext(r.Context()); rCtx != nil {
		pattern := rCtx.RoutePattern()
		if pattern != "" {
			return pattern
		}
	}
	// Fallback: If URL path is a clean root or health path without variable segments
	path := r.URL.Path
	if path == "/" || path == "/api/v1/health" || path == "/api/v1/ready" {
		return path
	}
	if strings.HasPrefix(path, "/api/") {
		return "unmatched"
	}
	return "unknown"
}
