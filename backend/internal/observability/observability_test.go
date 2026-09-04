package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// 1. Request ID Tests
func TestRequestID_MissingGenerated(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	provider := &Provider{cfg: DefaultConfig()}
	mw := Middleware(provider, nil)

	var extractedID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	assert.NotEmpty(t, extractedID, "missing request ID should be generated")
	assert.Equal(t, extractedID, w.Header().Get("X-Request-ID"), "response header must match generated request ID")
	assert.True(t, IsValidRequestID(extractedID))
}

func TestRequestID_SafePreserved(t *testing.T) {
	safeID := "req-12345_custom-abc"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", safeID)
	w := httptest.NewRecorder()

	provider := &Provider{cfg: DefaultConfig()}
	mw := Middleware(provider, nil)

	var extractedID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	assert.Equal(t, safeID, extractedID, "safe incoming request ID must be preserved")
	assert.Equal(t, safeID, w.Header().Get("X-Request-ID"))
}

func TestRequestID_UnsafeReplaced(t *testing.T) {
	unsafeIDs := []string{
		"req/with/slashes",
		"req with spaces",
		"req<script>alert(1)</script>",
		strings.Repeat("a", 65), // oversized > 64 chars
		"",
	}

	for _, badID := range unsafeIDs {
		t.Run("bad_id_"+badID, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Request-ID", badID)
			w := httptest.NewRecorder()

			provider := &Provider{cfg: DefaultConfig()}
			mw := Middleware(provider, nil)

			var extractedID string
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				extractedID = RequestIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(w, req)

			assert.NotEmpty(t, extractedID)
			assert.NotEqual(t, badID, extractedID, "unsafe or oversized request ID must be replaced")
			assert.True(t, IsValidRequestID(extractedID))
			assert.Equal(t, extractedID, w.Header().Get("X-Request-ID"))
		})
	}
}

// 2. Structured Logging & Error Correlation Tests
func TestStructuredLogging_CompletionLogFields(t *testing.T) {
	logBuf := &bytes.Buffer{}
	cfg := DefaultConfig()
	logger := NewLogger(cfg, nil, logBuf)

	r := chi.NewRouter()
	provider := &Provider{cfg: cfg}
	r.Use(Middleware(provider, logger))

	r.Get("/orders/{id}", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"order_id":"ORD-123"}`))
	})

	req := httptest.NewRequest("GET", "/orders/ORD-123", nil)
	req.Header.Set("X-Request-ID", "test-req-corr-001")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := logBuf.String()
	require.NotEmpty(t, logOutput)

	var logEntry map[string]any
	err := json.Unmarshal(logBuf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "http request completed", logEntry["msg"])
	assert.Equal(t, "test-req-corr-001", logEntry["request_id"])
	assert.Equal(t, "/orders/{id}", logEntry["route"], "route pattern must be used instead of concrete ID")
	assert.Equal(t, "GET", logEntry["method"])
	assert.Equal(t, float64(200), logEntry["status"])
	assert.Contains(t, logEntry, "duration_ms")
	assert.Equal(t, "zamk-api", logEntry["service_name"])
	assert.Equal(t, "INFO", logEntry["level"])
}

func TestStructuredLogging_ConflictIsNotFake500(t *testing.T) {
	logBuf := &bytes.Buffer{}
	cfg := DefaultConfig()
	logger := NewLogger(cfg, nil, logBuf)

	r := chi.NewRouter()
	provider := &Provider{cfg: cfg}
	r.Use(Middleware(provider, logger))

	r.Post("/items", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		_, _ = w.Write([]byte(`{"error":"item already exists"}`))
	})

	req := httptest.NewRequest("POST", "/items", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var logEntry map[string]any
	err := json.Unmarshal(logBuf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "WARN", logEntry["level"], "409 domain conflict must not be logged as ERROR 500")
	assert.Equal(t, float64(409), logEntry["status"])
}

func TestStructuredLogging_500ContainsCorrelation(t *testing.T) {
	logBuf := &bytes.Buffer{}
	cfg := DefaultConfig()
	logger := NewLogger(cfg, nil, logBuf)

	r := chi.NewRouter()
	provider := &Provider{cfg: cfg}
	r.Use(Middleware(provider, logger))

	r.Get("/error", func(w http.ResponseWriter, req *http.Request) {
		LogError(req.Context(), logger, "database query failed", fmt.Errorf("connection timeout"), slog.String("component", "database"))
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := httptest.NewRequest("GET", "/error", nil)
	req.Header.Set("X-Request-ID", "req-err-500-test")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	lines := strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	require.GreaterOrEqual(t, len(lines), 2)

	// First log is the internal LogError call
	var internalLog map[string]any
	err := json.Unmarshal([]byte(lines[0]), &internalLog)
	require.NoError(t, err)
	assert.Equal(t, "ERROR", internalLog["level"])
	assert.Equal(t, "req-err-500-test", internalLog["request_id"])
	assert.Equal(t, "connection timeout", internalLog["error"])

	// Second log is the HTTP access completion log
	var accessLog map[string]any
	err = json.Unmarshal([]byte(lines[1]), &accessLog)
	require.NoError(t, err)
	assert.Equal(t, "ERROR", accessLog["level"])
	assert.Equal(t, float64(500), accessLog["status"])
	assert.Equal(t, "req-err-500-test", accessLog["request_id"])
}

// 3. Panic Recovery Tests
func TestPanicRecovery_SafeResponseAndDiagnosticLog(t *testing.T) {
	logBuf := &bytes.Buffer{}
	cfg := DefaultConfig()
	logger := NewLogger(cfg, nil, logBuf)

	r := chi.NewRouter()
	provider := &Provider{cfg: cfg}
	r.Use(Middleware(provider, logger))

	r.Get("/panic", func(w http.ResponseWriter, req *http.Request) {
		panic("nil pointer dereference simulation")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	req.Header.Set("X-Request-ID", "panic-req-001")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Client response verification
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "nil pointer dereference", "client response must not leak panic message")
	assert.NotContains(t, body, "goroutine", "client response must not leak stack trace")
	assert.Contains(t, body, `{"error":{"code":"internal_error"`)

	// Log verification
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "http request panic recovered")
	assert.Contains(t, logOutput, "panic-req-001")
	assert.Contains(t, logOutput, "nil pointer dereference simulation")
	assert.Contains(t, logOutput, "stack")
}

// 4. Redaction & Security Tests
func TestRedaction_SensitiveKeysRedacted(t *testing.T) {
	logBuf := &bytes.Buffer{}
	cfg := DefaultConfig()
	logger := NewLogger(cfg, nil, logBuf)

	logger.InfoContext(context.Background(), "user auth attempt",
		slog.String("authorization", "Bearer eyJhbGciOiJIUzI1NiIsIn..."),
		slog.String("password", "superSecret123"),
		slog.String("cookie", "session_token=abc"),
		slog.String("user_email", "normal@example.com"),
	)

	logOutput := logBuf.String()
	assert.NotContains(t, logOutput, "Bearer eyJ")
	assert.NotContains(t, logOutput, "superSecret123")
	assert.NotContains(t, logOutput, "session_token=abc")
	assert.Contains(t, logOutput, "[REDACTED]")
}

// 5. OpenTelemetry Tracing Tests
func TestTracing_SpanCreatedAndTraceIDInLogs(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	tracer := tp.Tracer("test-tracer")

	logBuf := &bytes.Buffer{}
	cfg := DefaultConfig()
	logger := NewLogger(cfg, nil, logBuf)

	provider := &Provider{
		TracerProvider: tp,
		Tracer:         tracer,
		cfg:            cfg,
	}

	r := chi.NewRouter()
	r.Use(Middleware(provider, logger))
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		traceID := TraceIDFromContext(req.Context())
		assert.NotEmpty(t, traceID, "trace ID must exist in request context")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/users/usr-789", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "GET /users/{id}", span.Name())

	var logEntry map[string]any
	err := json.Unmarshal(logBuf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, span.SpanContext().TraceID().String(), logEntry["trace_id"], "log must contain matching trace_id")
	assert.Equal(t, span.SpanContext().SpanID().String(), logEntry["span_id"])
}

// 6. HTTP Metrics Low-Cardinality Enforcement Tests
func TestMetrics_LowCardinalityAndRouteTemplate(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test-meter")

	metrics, err := NewHTTPMetrics(meter)
	require.NoError(t, err)

	provider := &Provider{
		MeterProvider: mp,
		Metrics:       metrics,
		cfg:           DefaultConfig(),
	}

	r := chi.NewRouter()
	r.Use(Middleware(provider, nil))
	r.Get("/products/{sku}", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/products/SKU-998877", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)
	require.NotEmpty(t, rm.ScopeMetrics)

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "http_requests_total" {
				sum := m.Data.(metricdata.Sum[int64])
				require.NotEmpty(t, sum.DataPoints)
				for _, dp := range sum.DataPoints {
					attrs := dp.Attributes
					val, exists := attrs.Value("route")
					assert.True(t, exists)
					assert.Equal(t, "/products/{sku}", val.AsString(), "route label must be template, not concrete ID")

					methodVal, _ := attrs.Value("method")
					assert.Equal(t, "GET", methodVal.AsString())

					statusVal, _ := attrs.Value("status")
					assert.Equal(t, "200", statusVal.AsString())

					// Ensure no high-cardinality leaks in metric attributes
					assert.False(t, attrs.HasValue("request_id"))
					assert.False(t, attrs.HasValue("trace_id"))
					assert.False(t, attrs.HasValue("user_id"))
					assert.False(t, attrs.HasValue("sku"))
				}
			}
		}
	}
}

// 7. Resilient Failure Behavior Test
func TestOTLPFailure_DoesNotFailHTTPRequest(t *testing.T) {
	// Point to non-existent endpoint
	cfg := Config{
		ServiceName:    "zamk-api",
		Environment:    "local",
		ServiceVersion: "1.0.0",
		OTLPEndpoint:   "127.0.0.1:59999", // dead port
		OTLPInsecure:   true,
		Enabled:        true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	provider, err := Init(ctx, cfg, nil)
	require.NoError(t, err)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer shutdownCancel()
		_ = provider.Shutdown(shutdownCtx)
	}()

	r := chi.NewRouter()
	r.Use(Middleware(provider, nil))
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}
