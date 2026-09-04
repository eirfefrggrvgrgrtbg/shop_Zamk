package observability

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// RedisHook implements goredis.Hook to trace, measure, and safely log Redis commands centrally.
// Crucially, command arguments and keys are NEVER recorded to prevent leaking sensitive identifiers or cached data.
type RedisHook struct {
	tracer        trace.Tracer
	logger        *slog.Logger
	metrics       *RedisMetrics
	serverAddress string
	slowThreshold time.Duration
}

// NewRedisHook creates a new Redis hook.
func NewRedisHook(
	tracer trace.Tracer,
	logger *slog.Logger,
	metrics *RedisMetrics,
	serverAddress string,
	slowThreshold time.Duration,
) *RedisHook {
	if slowThreshold <= 0 {
		slowThreshold = 50 * time.Millisecond
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisHook{
		tracer:        tracer,
		logger:        logger,
		metrics:       metrics,
		serverAddress: serverAddress,
		slowThreshold: slowThreshold,
	}
}

// DialHook passes through connection dialing.
func (h *RedisHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// ProcessHook intercepts and instruments single Redis commands.
func (h *RedisHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		op := strings.ToUpper(cmd.Name())
		if op == "" {
			op = "COMMAND"
		}

		spanName := "redis." + op
		var span trace.Span
		if h.tracer != nil {
			ctx, span = h.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))

			attrs := []attribute.KeyValue{
				semconv.DBSystemRedis,
				attribute.String("db.operation.name", op),
			}
			if h.serverAddress != "" {
				attrs = append(attrs, attribute.String("server.address", h.serverAddress))
			}
			if reqID := RequestIDFromContext(ctx); reqID != "" {
				attrs = append(attrs, attribute.String("request_id", reqID))
			}
			span.SetAttributes(attrs...)
		}

		startTime := time.Now()
		err := next(ctx, cmd)
		duration := time.Since(startTime)

		reqID := RequestIDFromContext(ctx)
		traceID := TraceIDFromContext(ctx)

		if err != nil {
			// Expected cache miss (redis.Nil) must NOT be logged as ERROR and must NOT fail span
			if errors.Is(err, goredis.Nil) {
				if span != nil && span.IsRecording() {
					span.SetStatus(codes.Ok, "")
				}
				if h.metrics != nil {
					h.metrics.RecordOperation(ctx, op, "miss", duration)
				}
			} else {
				// Unexpected Redis error
				if span != nil && span.IsRecording() {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.SetAttributes(attribute.String("error.type", err.Error()))
				}

				logAttrs := []any{
					"component", "redis",
					"operation", op,
					"duration_ms", duration.Milliseconds(),
				}
				if reqID != "" {
					logAttrs = append(logAttrs, "request_id", reqID)
				}
				if traceID != "" {
					logAttrs = append(logAttrs, "trace_id", traceID)
				}
				logAttrs = append(logAttrs, "error", err.Error())
				h.logger.Error("redis operation failed", logAttrs...)

				if h.metrics != nil {
					h.metrics.RecordOperation(ctx, op, "error", duration)
					h.metrics.RecordError(ctx, op)
				}
			}
		} else {
			if span != nil && span.IsRecording() {
				span.SetStatus(codes.Ok, "")
			}

			if duration >= h.slowThreshold {
				warnAttrs := []any{
					"component", "redis",
					"operation", op,
					"duration_ms", duration.Milliseconds(),
				}
				if reqID != "" {
					warnAttrs = append(warnAttrs, "request_id", reqID)
				}
				if traceID != "" {
					warnAttrs = append(warnAttrs, "trace_id", traceID)
				}
				h.logger.Warn("slow redis operation", warnAttrs...)
			}

			if h.metrics != nil {
				h.metrics.RecordOperation(ctx, op, "ok", duration)
			}
		}

		if span != nil {
			span.End()
		}

		return err
	}
}

// ProcessPipelineHook intercepts and instruments pipelined Redis commands.
func (h *RedisHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		op := "PIPELINE"
		spanName := "redis." + op
		var span trace.Span
		if h.tracer != nil {
			ctx, span = h.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))

			attrs := []attribute.KeyValue{
				semconv.DBSystemRedis,
				attribute.String("db.operation.name", op),
				attribute.Int("db.pipeline.size", len(cmds)),
			}
			if h.serverAddress != "" {
				attrs = append(attrs, attribute.String("server.address", h.serverAddress))
			}
			if reqID := RequestIDFromContext(ctx); reqID != "" {
				attrs = append(attrs, attribute.String("request_id", reqID))
			}
			span.SetAttributes(attrs...)
		}

		startTime := time.Now()
		err := next(ctx, cmds)
		duration := time.Since(startTime)

		reqID := RequestIDFromContext(ctx)
		traceID := TraceIDFromContext(ctx)

		if err != nil && !errors.Is(err, goredis.Nil) {
			if span != nil && span.IsRecording() {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			logAttrs := []any{
				"component", "redis",
				"operation", op,
				"duration_ms", duration.Milliseconds(),
			}
			if reqID != "" {
				logAttrs = append(logAttrs, "request_id", reqID)
			}
			if traceID != "" {
				logAttrs = append(logAttrs, "trace_id", traceID)
			}
			logAttrs = append(logAttrs, "error", err.Error())
			h.logger.Error("redis pipeline failed", logAttrs...)

			if h.metrics != nil {
				h.metrics.RecordOperation(ctx, op, "error", duration)
				h.metrics.RecordError(ctx, op)
			}
		} else {
			if span != nil && span.IsRecording() {
				span.SetStatus(codes.Ok, "")
			}
			if h.metrics != nil {
				h.metrics.RecordOperation(ctx, op, "ok", duration)
			}
		}

		if span != nil {
			span.End()
		}

		return err
	}
}
