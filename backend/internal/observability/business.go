package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// BusinessEvent represents a canonical operational business domain event.
// It is recorded both as a structured log to Loki and as a span event in Tempo.
type BusinessEvent struct {
	EventName  string
	Domain     string
	Action     string
	Result     string
	ActorID    string
	ActorRole  string
	Attributes []slog.Attr
	Level      slog.Level
}

// EmitBusinessEvent emits a canonical operational business event.
// Requirements:
// 1. Emits structured log via slog with event_name, domain, action, result, actor, and entity context.
// 2. If an active OpenTelemetry span exists, records span.AddEvent with safe bounded attributes.
// 3. Gracefully succeeds when no active span exists.
// 4. Inherits actor_id / actor_role from context if omitted on the event.
// 5. Automatically redacts sensitive keys (passwords, tokens, cards, etc.).
func EmitBusinessEvent(ctx context.Context, logger *slog.Logger, event BusinessEvent) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}

	// 1. Resolve actor context if missing
	ctxActorID, ctxActorRole := ActorFromContext(ctx)
	if event.ActorID == "" && ctxActorID != "" {
		event.ActorID = ctxActorID
	}
	if event.ActorRole == "" && ctxActorRole != "" {
		event.ActorRole = ctxActorRole
	}

	// 2. Default level is INFO
	level := event.Level
	if level == 0 {
		level = slog.LevelInfo
	}

	// 3. Build structured log attributes
	logAttrs := make([]slog.Attr, 0, len(event.Attributes)+8)
	logAttrs = append(logAttrs,
		slog.String("event_name", event.EventName),
		slog.String("domain", event.Domain),
		slog.String("component", event.Domain),
		slog.String("action", event.Action),
		slog.String("operation", event.Action),
		slog.String("result", event.Result),
	)

	if event.ActorID != "" {
		logAttrs = append(logAttrs, slog.String("actor_id", event.ActorID))
	}
	if event.ActorRole != "" {
		logAttrs = append(logAttrs, slog.String("actor_role", event.ActorRole))
	}

	// Sanitize and append custom attributes
	for _, attr := range event.Attributes {
		if isSensitiveKey(attr.Key) {
			logAttrs = append(logAttrs, slog.String(attr.Key, "[REDACTED]"))
		} else {
			logAttrs = append(logAttrs, attr)
		}
	}

	logger.LogAttrs(ctx, level, event.EventName, logAttrs...)

	// 4. Record OpenTelemetry span event if span is recording
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		otelAttrs := make([]attribute.KeyValue, 0, len(logAttrs))
		otelAttrs = append(otelAttrs,
			attribute.String("event.name", event.EventName),
			attribute.String("event.domain", event.Domain),
			attribute.String("event.action", event.Action),
			attribute.String("event.result", event.Result),
		)
		if event.ActorID != "" {
			otelAttrs = append(otelAttrs, attribute.String("actor.id", event.ActorID))
		}
		if event.ActorRole != "" {
			otelAttrs = append(otelAttrs, attribute.String("actor.role", event.ActorRole))
		}

		for _, attr := range event.Attributes {
			if isSensitiveKey(attr.Key) {
				otelAttrs = append(otelAttrs, attribute.String(attr.Key, "[REDACTED]"))
			} else {
				otelAttrs = append(otelAttrs, slogAttrToOtelAttr(attr))
			}
		}

		span.AddEvent(event.EventName, trace.WithAttributes(otelAttrs...))
	}
}

// slogAttrToOtelAttr converts an slog.Attr to an OpenTelemetry attribute.KeyValue.
func slogAttrToOtelAttr(a slog.Attr) attribute.KeyValue {
	switch a.Value.Kind() {
	case slog.KindString:
		return attribute.String(a.Key, a.Value.String())
	case slog.KindInt64:
		return attribute.Int64(a.Key, a.Value.Int64())
	case slog.KindUint64:
		return attribute.Int64(a.Key, int64(a.Value.Uint64()))
	case slog.KindFloat64:
		return attribute.Float64(a.Key, a.Value.Float64())
	case slog.KindBool:
		return attribute.Bool(a.Key, a.Value.Bool())
	default:
		return attribute.String(a.Key, a.Value.String())
	}
}
