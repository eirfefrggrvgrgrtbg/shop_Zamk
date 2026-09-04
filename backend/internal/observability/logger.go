package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

var sensitiveKeySubstrings = []string{
	"authorization",
	"cookie",
	"set-cookie",
	"password",
	"secret",
	"token",
	"jwt",
	"card_number",
	"cvv",
}

// isSensitiveKey checks whether an attribute key represents sensitive data.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeySubstrings {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// sanitizeAttr redacts sensitive attribute values.
func sanitizeAttr(_ []string, a slog.Attr) slog.Attr {
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

// ContextEnrichingHandler is an slog.Handler that automatically enriches records
// with request_id, trace_id, span_id, and level from the context.
type ContextEnrichingHandler struct {
	next slog.Handler
}

func (h *ContextEnrichingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *ContextEnrichingHandler) Handle(ctx context.Context, r slog.Record) error {
	var extraAttrs []slog.Attr

	// Ensure level is explicitly present in attributes for downstream Loki processors
	extraAttrs = append(extraAttrs, slog.String("level", r.Level.String()))

	if reqID := RequestIDFromContext(ctx); reqID != "" {
		extraAttrs = append(extraAttrs, slog.String("request_id", reqID))
	}

	if route := RouteFromContext(ctx); route != "" {
		extraAttrs = append(extraAttrs, slog.String("route", route))
	}

	span := trace.SpanFromContext(ctx)
	if sc := span.SpanContext(); sc.IsValid() {
		extraAttrs = append(extraAttrs,
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	if len(extraAttrs) > 0 {
		r.AddAttrs(extraAttrs...)
	}

	return h.next.Handle(ctx, r)
}

func (h *ContextEnrichingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextEnrichingHandler{next: h.next.WithAttrs(attrs)}
}

func (h *ContextEnrichingHandler) WithGroup(name string) slog.Handler {
	return &ContextEnrichingHandler{next: h.next.WithGroup(name)}
}

// MultiHandler multiplexes slog records to multiple underlying handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: newHandlers}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: newHandlers}
}

// NewLogger creates the canonical structured slog.Logger.
// It writes structured JSON to out (defaults to os.Stdout) and forwards
// structured logs to the OpenTelemetry LoggerProvider if available.
func NewLogger(cfg Config, lp *sdklog.LoggerProvider, out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}

	jsonHandler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: sanitizeAttr,
	})

	var baseHandler slog.Handler = jsonHandler

	if lp != nil {
		otelHandler := otelslog.NewHandler(cfg.ServiceName,
			otelslog.WithLoggerProvider(lp),
		)
		baseHandler = &MultiHandler{
			handlers: []slog.Handler{jsonHandler, otelHandler},
		}
	}

	enrichedHandler := &ContextEnrichingHandler{next: baseHandler}

	logger := slog.New(enrichedHandler).With(
		slog.String("service_name", cfg.ServiceName),
		slog.String("environment", cfg.Environment),
	)

	return logger
}

// LogError logs an unexpected error with contextual request and trace correlation.
func LogError(ctx context.Context, logger *slog.Logger, msg string, err error, attrs ...any) {
	if logger == nil {
		logger = slog.Default()
	}

	allAttrs := make([]any, 0, len(attrs)+2)
	if err != nil {
		allAttrs = append(allAttrs, slog.String("error", err.Error()))
	}
	allAttrs = append(allAttrs, attrs...)

	logger.ErrorContext(ctx, msg, allAttrs...)
}
