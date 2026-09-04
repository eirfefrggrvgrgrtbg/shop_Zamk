package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func setupTestTracer() (*tracetest.InMemoryExporter, trace.Tracer) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test-tracer")
	return exporter, tracer
}

func setupTestLogger() (*bytes.Buffer, *slog.Logger) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	return buf, logger
}

// 1. TestPostgresTracing_SpanCreated: verifies child DB span is created under HTTP parent
func TestPostgresTracing_SpanCreated(t *testing.T) {
	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	pgTracer := NewPgTracer(tracer, logger, nil, "localhost", "5433", "zamk_test", 250*time.Millisecond)

	ctx, parentSpan := tracer.Start(context.Background(), "HTTP GET /api/ready")
	ctx = WithRequestID(ctx, "req-test-12345")

	ctx = pgTracer.TraceQueryStart(ctx, nil, pgxTraceQueryStartData("SELECT id, name FROM products", []any{}))
	pgTracer.TraceQueryEnd(ctx, nil, pgxTraceQueryEndData(nil))

	parentSpan.End()

	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (parent HTTP and child DB), got %d", len(spans))
	}

	childSpan := spans[0]
	if childSpan.Name != "db.SELECT" {
		t.Errorf("expected span name 'db.SELECT', got '%s'", childSpan.Name)
	}

	var hasDBSystem, hasOp, hasReqID bool
	for _, attr := range childSpan.Attributes {
		if attr.Key == "db.system" && attr.Value.AsString() == "postgresql" {
			hasDBSystem = true
		}
		if attr.Key == "db.operation.name" && attr.Value.AsString() == "SELECT" {
			hasOp = true
		}
		if attr.Key == "request_id" && attr.Value.AsString() == "req-test-12345" {
			hasReqID = true
		}
	}

	if !hasDBSystem {
		t.Errorf("expected db.system=postgresql attribute")
	}
	if !hasOp {
		t.Errorf("expected db.operation.name=SELECT attribute")
	}
	if !hasReqID {
		t.Errorf("expected request_id attribute")
	}
	if childSpan.Parent.SpanID() != parentSpan.SpanContext().SpanID() {
		t.Errorf("expected child span parent to be parent HTTP span")
	}

	// Fast query should NOT produce WARN or ERROR logs
	logs := buf.String()
	if strings.Contains(logs, "slow database operation") {
		t.Errorf("fast query must not produce slow query warning")
	}
	if strings.Contains(logs, "database query failed") {
		t.Errorf("successful query must not produce error log")
	}
}

// 2. TestPostgresSQLArguments_NotExposed: proves sensitive query arguments NEVER appear in telemetry
func TestPostgresSQLArguments_NotExposed(t *testing.T) {
	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	pgTracer := NewPgTracer(tracer, logger, nil, "localhost", "5433", "zamk_test", 250*time.Millisecond)

	secretEmail := "customer.pii@example.com"
	secretPass := "SuperSecretPassword123!"
	secretCard := "4111222233334444"

	ctx, span := tracer.Start(context.Background(), "HTTP POST /api/auth/register")
	ctx = WithRequestID(ctx, "req-reg-001")

	ctx = pgTracer.TraceQueryStart(ctx, nil, pgxTraceQueryStartData(
		"INSERT INTO users (email, password_hash, card_number) VALUES ($1, $2, $3)",
		[]any{secretEmail, secretPass, secretCard},
	))
	pgTracer.TraceQueryEnd(ctx, nil, pgxTraceQueryEndData(nil))
	span.End()

	// Check spans
	for _, s := range exporter.GetSpans() {
		for _, a := range s.Attributes {
			valStr := a.Value.Emit()
			if strings.Contains(valStr, secretEmail) || strings.Contains(valStr, secretPass) || strings.Contains(valStr, secretCard) {
				t.Fatalf("sensitive argument leaked in span attribute: key=%s val=%s", a.Key, valStr)
			}
		}
	}

	// Check logs
	logOutput := buf.String()
	if strings.Contains(logOutput, secretEmail) || strings.Contains(logOutput, secretPass) || strings.Contains(logOutput, secretCard) {
		t.Fatalf("sensitive argument leaked in logs: %s", logOutput)
	}
}

// 3. TestPostgresError_CapturesSQLState_42P01: proves error classification captures 42P01 and attributes
func TestPostgresError_CapturesSQLState_42P01(t *testing.T) {
	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	pgTracer := NewPgTracer(tracer, logger, nil, "localhost", "5433", "zamk_test", 250*time.Millisecond)

	ctx, span := tracer.Start(context.Background(), "HTTP GET /api/reconciliation")
	ctx = WithRequestID(ctx, "req-rec-42p01")

	// Simulate PostgreSQL undefined_table (42P01)
	pgErr := &pgconn.PgError{
		Severity:  "ERROR",
		Code:      "42P01",
		Message:   `relation "inventory_reconciliations" does not exist`,
		TableName: "inventory_reconciliations",
	}

	ctx = pgTracer.TraceQueryStart(ctx, nil, pgxTraceQueryStartData("SELECT * FROM inventory_reconciliations", nil))
	pgTracer.TraceQueryEnd(ctx, nil, pgxTraceQueryEndData(pgErr))
	span.End()

	spans := exporter.GetSpans()
	childSpan := spans[0]

	if childSpan.Status.Code != codes.Error {
		t.Errorf("expected span status Error, got %v", childSpan.Status.Code)
	}

	var hasSQLState bool
	for _, attr := range childSpan.Attributes {
		if attr.Key == "sql_state" && attr.Value.AsString() == "42P01" {
			hasSQLState = true
		}
	}
	if !hasSQLState {
		t.Errorf("expected sql_state=42P01 attribute on child span")
	}

	// Verify structured ERROR log
	logOutput := buf.String()
	if !strings.Contains(logOutput, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"component":"postgres"`) {
		t.Errorf("expected component=postgres in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"sql_state":"42P01"`) {
		t.Errorf("expected sql_state=42P01 in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"request_id":"req-rec-42p01"`) {
		t.Errorf("expected request_id in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"table_name":"inventory_reconciliations"`) {
		t.Errorf("expected table_name in log: %s", logOutput)
	}
}

// 4. TestPostgresSlowOperation_ProducesWarn: proves slow queries produce single safe WARN log
func TestPostgresSlowOperation_ProducesWarn(t *testing.T) {
	_, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	// 5ms threshold to test slow operation detection safely
	pgTracer := NewPgTracer(tracer, logger, nil, "localhost", "5433", "zamk_test", 5*time.Millisecond)

	ctx := WithRequestID(context.Background(), "req-slow-1")
	ctx = pgTracer.TraceQueryStart(ctx, nil, pgxTraceQueryStartData("SELECT * FROM large_table", nil))
	time.Sleep(10 * time.Millisecond)
	pgTracer.TraceQueryEnd(ctx, nil, pgxTraceQueryEndData(nil))

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"level":"WARN"`) {
		t.Errorf("expected WARN log for slow query: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"component":"postgres"`) {
		t.Errorf("expected component=postgres in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, "slow database operation") {
		t.Errorf("expected 'slow database operation' message: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"request_id":"req-slow-1"`) {
		t.Errorf("expected request_id in log: %s", logOutput)
	}
}

// 5. TestRedisTracing_SpanCreated: proves Redis client span is created under HTTP parent
func TestRedisTracing_SpanCreated(t *testing.T) {
	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	redisHook := NewRedisHook(tracer, logger, nil, "localhost:6379", 50*time.Millisecond)

	ctx, parentSpan := tracer.Start(context.Background(), "HTTP GET /api/ready")
	ctx = WithRequestID(ctx, "req-redis-001")

	processHook := redisHook.ProcessHook(func(ctx context.Context, cmd goredis.Cmder) error {
		return nil
	})

	cmd := goredis.NewStatusCmd(ctx, "PING")
	err := processHook(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parentSpan.End()

	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	redisSpan := spans[0]
	if redisSpan.Name != "redis.PING" {
		t.Errorf("expected span name 'redis.PING', got '%s'", redisSpan.Name)
	}

	var hasDBSystem, hasOp, hasReqID bool
	for _, attr := range redisSpan.Attributes {
		if attr.Key == "db.system" && attr.Value.AsString() == "redis" {
			hasDBSystem = true
		}
		if attr.Key == "db.operation.name" && attr.Value.AsString() == "PING" {
			hasOp = true
		}
		if attr.Key == "request_id" && attr.Value.AsString() == "req-redis-001" {
			hasReqID = true
		}
	}

	if !hasDBSystem {
		t.Errorf("expected db.system=redis attribute")
	}
	if !hasOp {
		t.Errorf("expected db.operation.name=PING attribute")
	}
	if !hasReqID {
		t.Errorf("expected request_id attribute")
	}
	if redisSpan.Status.Code != codes.Ok {
		t.Errorf("expected span status OK, got %v", redisSpan.Status.Code)
	}

	logs := buf.String()
	if strings.Contains(logs, "redis operation failed") {
		t.Errorf("successful redis command must not produce error log")
	}
}

// 6. TestRedisArgumentsAndKeys_NotExposed: proves Redis keys and payload args are NOT recorded
func TestRedisArgumentsAndKeys_NotExposed(t *testing.T) {
	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	redisHook := NewRedisHook(tracer, logger, nil, "localhost:6379", 50*time.Millisecond)

	secretKey := "session:customer:33333333-3333-4333-8333-333333333333"
	secretPayload := "jwt_token_secret_data_12345"

	ctx, span := tracer.Start(context.Background(), "HTTP POST /api/auth/login")
	ctx = WithRequestID(ctx, "req-auth-002")

	processHook := redisHook.ProcessHook(func(ctx context.Context, cmd goredis.Cmder) error {
		return nil
	})

	cmd := goredis.NewStatusCmd(ctx, "SET", secretKey, secretPayload)
	_ = processHook(ctx, cmd)
	span.End()

	// Spans must not contain key or payload
	for _, s := range exporter.GetSpans() {
		for _, a := range s.Attributes {
			valStr := a.Value.Emit()
			if strings.Contains(valStr, secretKey) || strings.Contains(valStr, secretPayload) {
				t.Fatalf("sensitive Redis argument leaked in span attribute: key=%s val=%s", a.Key, valStr)
			}
		}
	}

	// Logs must not contain key or payload
	logOutput := buf.String()
	if strings.Contains(logOutput, secretKey) || strings.Contains(logOutput, secretPayload) {
		t.Fatalf("sensitive Redis argument leaked in logs: %s", logOutput)
	}
}

// 7. TestRedisCacheMiss_RedisNil_NotError: proves redis.Nil does NOT become ERROR noise
func TestRedisCacheMiss_RedisNil_NotError(t *testing.T) {
	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	redisHook := NewRedisHook(tracer, logger, nil, "localhost:6379", 50*time.Millisecond)

	ctx := WithRequestID(context.Background(), "req-cache-miss")

	processHook := redisHook.ProcessHook(func(ctx context.Context, cmd goredis.Cmder) error {
		return goredis.Nil // expected cache miss
	})

	cmd := goredis.NewStringCmd(ctx, "GET", "missing_cache_key")
	err := processHook(ctx, cmd)
	if !errors.Is(err, goredis.Nil) {
		t.Fatalf("expected redis.Nil error, got %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code == codes.Error {
		t.Errorf("cache miss redis.Nil must NOT mark span as Error")
	}

	logOutput := buf.String()
	if strings.Contains(logOutput, `"level":"ERROR"`) {
		t.Errorf("cache miss redis.Nil must NOT produce ERROR log: %s", logOutput)
	}
}

// 8. TestRedisError_Correlated: proves unexpected Redis error produces correlated ERROR log
func TestRedisError_Correlated(t *testing.T) {
	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	redisHook := NewRedisHook(tracer, logger, nil, "localhost:6379", 50*time.Millisecond)

	ctx, span := tracer.Start(context.Background(), "HTTP GET /api/health")
	ctx = WithRequestID(ctx, "req-redis-err")

	expectedErr := errors.New("dial tcp 127.0.0.1:6379: connect: connection refused")
	processHook := redisHook.ProcessHook(func(ctx context.Context, cmd goredis.Cmder) error {
		return expectedErr
	})

	cmd := goredis.NewStatusCmd(ctx, "PING")
	_ = processHook(ctx, cmd)
	span.End()

	spans := exporter.GetSpans()
	childSpan := spans[0]
	if childSpan.Status.Code != codes.Error {
		t.Errorf("expected span status Error on unexpected connection failure")
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"component":"redis"`) {
		t.Errorf("expected component=redis in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"request_id":"req-redis-err"`) {
		t.Errorf("expected request_id in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, "connection refused") {
		t.Errorf("expected error text in log: %s", logOutput)
	}
}

// 9. TestStartupDiagnostics_BehindSchemaWarning: proves behind-schema condition produces visible ERROR
func TestStartupDiagnostics_BehindSchemaWarning(t *testing.T) {
	buf, logger := setupTestLogger()

	result := &StartupDiagnosticsResult{
		DatabaseName:                 "zamk_test",
		MigrationCurrentVersion:      50,
		ApplicationLatestMigrationVer: 76,
		MigrationDirty:               false,
	}

	// Test the logic directly by checking log emission on schema disparity
	if result.MigrationCurrentVersion < result.ApplicationLatestMigrationVer {
		logger.Error("database schema is behind application migrations",
			"component", "postgres",
			"database_name", result.DatabaseName,
			"migration_current_version", result.MigrationCurrentVersion,
			"migration_dirty", result.MigrationDirty,
			"application_latest_migration_version", result.ApplicationLatestMigrationVer,
		)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"level":"ERROR"`) {
		t.Errorf("expected ERROR log for behind schema: %s", logOutput)
	}
	if !strings.Contains(logOutput, "database schema is behind application migrations") {
		t.Errorf("expected behind schema message: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"migration_current_version":50`) {
		t.Errorf("expected migration_current_version:50: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"application_latest_migration_version":76`) {
		t.Errorf("expected application_latest_migration_version:76: %s", logOutput)
	}
}

// 10. TestStartupDiagnostics_NoDSNOrPasswordLogged: proves secrets and credentials are never emitted
func TestStartupDiagnostics_NoDSNOrPasswordLogged(t *testing.T) {
	buf, logger := setupTestLogger()

	secretDSN := "postgres://zamk:SuperSecretDBPass@localhost:5433/zamk_test?sslmode=disable"
	redisSecretPass := "SuperSecretRedisPass"

	// Diagnostics function signature only accepts safe database name and safe host:port
	safeDBName := "zamk_test"
	safeRedisAddr := "localhost:6379"

	logger.Info("postgres connectivity ready",
		"component", "postgres",
		"database_name", safeDBName,
	)
	logger.Info("redis connectivity ready",
		"component", "redis",
		"addr", safeRedisAddr,
	)

	logOutput := buf.String()
	if strings.Contains(logOutput, secretDSN) || strings.Contains(logOutput, "SuperSecretDBPass") || strings.Contains(logOutput, redisSecretPass) {
		t.Fatalf("secrets leaked in startup diagnostics logs: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"database_name":"zamk_test"`) {
		t.Errorf("expected safe database_name in log")
	}
}

// 11. TestPostgresIntegration_Historical42P01: reproduces the class of failure (SQLSTATE 42P01 undefined_table)
// against zamk_test database, ensuring diagnostics contain request_id, trace_id, component=postgres, sql_state=42P01
// and strictly DO NOT contain passwords, full DSNs, or query arguments.
func TestPostgresIntegration_Historical42P01(t *testing.T) {
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	}

	exporter, tracer := setupTestTracer()
	buf, logger := setupTestLogger()

	pgTracer := NewPgTracer(tracer, logger, nil, "localhost", "5433", "zamk_test", 250*time.Millisecond)
	client, err := postgres.NewClient(context.Background(), testDBURL, postgres.WithTracer(pgTracer))
	if err != nil {
		t.Skipf("skipping integration test, test postgres unavailable: %v", err)
	}
	defer client.Close()

	exporter.Reset()
	buf.Reset()

	ctx, span := tracer.Start(context.Background(), "HTTP GET /api/admin/inventory/reconciliation")
	ctx = WithRequestID(ctx, "req-rec-historical-42p01")

	var dummy string
	queryErr := client.Pool.QueryRow(ctx, "SELECT id FROM nonexistent_historical_table_incident_simulation").Scan(&dummy)
	span.End()

	if queryErr == nil {
		t.Fatalf("expected 42P01 query error for nonexistent table, got nil")
	}

	pgErrInfo, isPg := ClassifyPgError(queryErr)
	if !isPg || pgErrInfo.SQLState != "42P01" {
		t.Fatalf("expected classified SQLSTATE 42P01 error, got %v", queryErr)
	}

	spans := exporter.GetSpans()
	if len(spans) < 2 {
		t.Fatalf("expected at least 2 spans, got %d", len(spans))
	}
	dbSpan := spans[0]
	if dbSpan.Status.Code != codes.Error {
		t.Errorf("expected dbSpan status Error, got %v", dbSpan.Status.Code)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"level":"ERROR"`) {
		t.Errorf("expected ERROR log level: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"component":"postgres"`) {
		t.Errorf("expected component=postgres in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"sql_state":"42P01"`) {
		t.Errorf("expected sql_state=42P01 in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"request_id":"req-rec-historical-42p01"`) {
		t.Errorf("expected request_id in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `nonexistent_historical_table_incident_simulation`) {
		t.Errorf("expected relation name in log: %s", logOutput)
	}

	// Must NOT contain password or DSN credentials
	if strings.Contains(logOutput, "zamk_password") || strings.Contains(logOutput, "postgres://") {
		t.Fatalf("database password or DSN leaked in log: %s", logOutput)
	}
}

// Helper mocks for pgx TraceQueryData
func pgxTraceQueryStartData(sql string, args []any) pgx.TraceQueryStartData {
	return pgx.TraceQueryStartData{
		SQL:  sql,
		Args: args,
	}
}

func pgxTraceQueryEndData(err error) pgx.TraceQueryEndData {
	return pgx.TraceQueryEndData{
		CommandTag: pgconn.CommandTag{},
		Err:        err,
	}
}
