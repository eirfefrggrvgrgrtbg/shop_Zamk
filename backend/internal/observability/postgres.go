package observability

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type pgTraceCtxKey struct{}

type pgQueryState struct {
	startTime time.Time
	operation string
}

// PgErrorInfo captures safe diagnostic metadata from PostgreSQL errors.
type PgErrorInfo struct {
	SQLState       string
	Severity       string
	ConstraintName string
	TableName      string
	Message        string
}

// ClassifyPgError inspects an error for underlying pgconn.PgError metadata.
func ClassifyPgError(err error) (*PgErrorInfo, bool) {
	if err == nil {
		return nil, false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &PgErrorInfo{
			SQLState:       pgErr.Code,
			Severity:       pgErr.Severity,
			ConstraintName: pgErr.ConstraintName,
			TableName:      pgErr.TableName,
			Message:        pgErr.Message,
		}, true
	}
	return nil, false
}

// PgTracer implements pgx.QueryTracer to trace, log, and measure PostgreSQL queries centrally.
type PgTracer struct {
	tracer        trace.Tracer
	logger        *slog.Logger
	slowThreshold time.Duration
	metrics       *DBMetrics
	serverAddress string
	serverPort    string
	databaseName  string
}

// NewPgTracer initializes a new PostgreSQL tracer.
func NewPgTracer(
	tracer trace.Tracer,
	logger *slog.Logger,
	metrics *DBMetrics,
	serverAddress string,
	serverPort string,
	databaseName string,
	slowThreshold time.Duration,
) *PgTracer {
	if slowThreshold <= 0 {
		slowThreshold = 250 * time.Millisecond
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PgTracer{
		tracer:        tracer,
		logger:        logger,
		metrics:       metrics,
		serverAddress: serverAddress,
		serverPort:    serverPort,
		databaseName:  databaseName,
		slowThreshold: slowThreshold,
	}
}

// TraceQueryStart starts a client span for the database operation.
// Crucially, query arguments are NEVER recorded in span attributes or telemetry.
func (t *PgTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	op := extractOperation(data.SQL)

	spanName := "db." + op
	var span trace.Span
	if t.tracer != nil {
		ctx, span = t.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))

		attrs := []attribute.KeyValue{
			semconv.DBSystemPostgreSQL,
			attribute.String("db.operation.name", op),
		}
		if t.serverAddress != "" {
			attrs = append(attrs, attribute.String("server.address", t.serverAddress))
		}
		if t.serverPort != "" {
			attrs = append(attrs, attribute.String("server.port", t.serverPort))
		}
		if t.databaseName != "" {
			attrs = append(attrs, attribute.String("db.namespace", t.databaseName))
		}
		if reqID := RequestIDFromContext(ctx); reqID != "" {
			attrs = append(attrs, attribute.String("request_id", reqID))
		}
		span.SetAttributes(attrs...)
	}

	state := &pgQueryState{
		startTime: time.Now(),
		operation: op,
	}
	return context.WithValue(ctx, pgTraceCtxKey{}, state)
}

// TraceQueryEnd ends the client span, records metrics, classifies errors, and logs slow or failed operations.
func (t *PgTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	state, _ := ctx.Value(pgTraceCtxKey{}).(*pgQueryState)
	var op string
	startTime := time.Now()
	if state != nil {
		op = state.operation
		startTime = state.startTime
	}
	if op == "" {
		op = "QUERY"
	}

	duration := time.Since(startTime)
	span := trace.SpanFromContext(ctx)
	reqID := RequestIDFromContext(ctx)
	traceID := TraceIDFromContext(ctx)

	if data.Err != nil {
		status := "error"
		sqlState := ""
		pgErrInfo, isPgErr := ClassifyPgError(data.Err)

		if span != nil && span.IsRecording() {
			if isPgErr {
				sqlState = pgErrInfo.SQLState
				span.SetAttributes(
					attribute.String("error.type", pgErrInfo.SQLState),
					attribute.String("sql_state", pgErrInfo.SQLState),
				)
				if pgErrInfo.TableName != "" {
					span.SetAttributes(attribute.String("db.collection.name", pgErrInfo.TableName))
				}
				if pgErrInfo.ConstraintName != "" {
					span.SetAttributes(attribute.String("db.constraint_name", pgErrInfo.ConstraintName))
				}
			}
			span.RecordError(data.Err)
			span.SetStatus(codes.Error, data.Err.Error())
		}

		// Structured diagnostic log for unexpected database errors
		logAttrs := []any{
			"component", "postgres",
			"operation", op,
			"duration_ms", duration.Milliseconds(),
		}
		if reqID != "" {
			logAttrs = append(logAttrs, "request_id", reqID)
		}
		if traceID != "" {
			logAttrs = append(logAttrs, "trace_id", traceID)
		}
		if isPgErr {
			logAttrs = append(logAttrs,
				"error_code", pgErrInfo.SQLState,
				"sql_state", pgErrInfo.SQLState,
				"severity", pgErrInfo.Severity,
			)
			if pgErrInfo.TableName != "" {
				logAttrs = append(logAttrs, "table_name", pgErrInfo.TableName)
			}
			if pgErrInfo.ConstraintName != "" {
				logAttrs = append(logAttrs, "constraint_name", pgErrInfo.ConstraintName)
			}
		}
		logAttrs = append(logAttrs, "error", data.Err.Error())
		t.logger.Error("database query failed", logAttrs...)

		if t.metrics != nil {
			t.metrics.RecordOperation(ctx, "postgresql", op, status, duration)
			if sqlState != "" {
				t.metrics.RecordError(ctx, "postgresql", sqlState)
			}
		}
	} else {
		if span != nil && span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}

		// Slow operation warning
		if duration >= t.slowThreshold {
			warnAttrs := []any{
				"component", "postgres",
				"operation", op,
				"duration_ms", duration.Milliseconds(),
			}
			if reqID != "" {
				warnAttrs = append(warnAttrs, "request_id", reqID)
			}
			if traceID != "" {
				warnAttrs = append(warnAttrs, "trace_id", traceID)
			}
			t.logger.Warn("slow database operation", warnAttrs...)
		}

		if t.metrics != nil {
			t.metrics.RecordOperation(ctx, "postgresql", op, "ok", duration)
		}
	}

	if span != nil {
		span.End()
	}
}

// extractOperation extracts a safe, low-cardinality SQL operation name.
func extractOperation(sql string) string {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" || trimmed == ";" {
		return "PING"
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "QUERY"
	}
	firstWord := strings.ToUpper(fields[0])
	switch firstWord {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "BEGIN", "COMMIT", "ROLLBACK", "CREATE", "ALTER", "DROP", "TRUNCATE":
		return firstWord
	default:
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				return r
			}
			return -1
		}, firstWord)
		if clean != "" {
			return strings.ToUpper(clean)
		}
		return "QUERY"
	}
}
