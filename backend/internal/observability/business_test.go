package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestEmitBusinessEvent_StructuredLogAndSpanEvent(t *testing.T) {
	// Set up in-memory span recorder
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test-tracer")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	event := BusinessEvent{
		EventName: "warehouse.receiving_finalized",
		Domain:    "warehouse",
		Action:    "finalize_receiving",
		Result:    "success",
		ActorID:   "user-123",
		ActorRole: "warehouse_admin",
		Attributes: []slog.Attr{
			slog.String("supply_id", "supp-456"),
			slog.Int("received_units_count", 10),
			slog.Int("damaged_units_count", 1),
		},
	}

	EmitBusinessEvent(ctx, logger, event)
	span.End()

	// 1. Assert structured log in buffer
	var logRecord map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logRecord)
	require.NoError(t, err)

	assert.Equal(t, "warehouse.receiving_finalized", logRecord["event_name"])
	assert.Equal(t, "warehouse", logRecord["domain"])
	assert.Equal(t, "finalize_receiving", logRecord["action"])
	assert.Equal(t, "success", logRecord["result"])
	assert.Equal(t, "user-123", logRecord["actor_id"])
	assert.Equal(t, "warehouse_admin", logRecord["actor_role"])
	assert.Equal(t, "supp-456", logRecord["supply_id"])
	assert.Equal(t, float64(10), logRecord["received_units_count"])
	assert.Equal(t, float64(1), logRecord["damaged_units_count"])

	// 2. Assert span event in OpenTelemetry trace
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Len(t, spans[0].Events, 1)

	spanEv := spans[0].Events[0]
	assert.Equal(t, "warehouse.receiving_finalized", spanEv.Name)

	spanAttrMap := make(map[string]interface{})
	for _, a := range spanEv.Attributes {
		spanAttrMap[string(a.Key)] = a.Value.AsInterface()
	}

	assert.Equal(t, "warehouse.receiving_finalized", spanAttrMap["event.name"])
	assert.Equal(t, "warehouse", spanAttrMap["event.domain"])
	assert.Equal(t, "finalize_receiving", spanAttrMap["event.action"])
	assert.Equal(t, "success", spanAttrMap["event.result"])
	assert.Equal(t, "user-123", spanAttrMap["actor.id"])
	assert.Equal(t, "warehouse_admin", spanAttrMap["actor.role"])
	assert.Equal(t, "supp-456", spanAttrMap["supply_id"])
	assert.Equal(t, int64(10), spanAttrMap["received_units_count"])
	assert.Equal(t, int64(1), spanAttrMap["damaged_units_count"])
}

func TestEmitBusinessEvent_NoActiveSpan(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	event := BusinessEvent{
		EventName: "warehouse.zmu_received",
		Domain:    "warehouse",
		Action:    "zmu_received",
		Result:    "success",
		Attributes: []slog.Attr{
			slog.String("zmu", "ZMU-999"),
		},
	}

	// Must not panic or fail with plain background context
	assert.NotPanics(t, func() {
		EmitBusinessEvent(context.Background(), logger, event)
	})

	var logRecord map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logRecord)
	require.NoError(t, err)
	assert.Equal(t, "warehouse.zmu_received", logRecord["event_name"])
	assert.Equal(t, "ZMU-999", logRecord["zmu"])
}

func TestEmitBusinessEvent_ActorFromContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	ctx := WithActor(context.Background(), "admin-777", "admin")

	event := BusinessEvent{
		EventName: "inventory.stale_allocation_released",
		Domain:    "inventory",
		Action:    "release_stale_allocation",
		Result:    "success",
	}

	EmitBusinessEvent(ctx, logger, event)

	var logRecord map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logRecord)
	require.NoError(t, err)
	assert.Equal(t, "admin-777", logRecord["actor_id"])
	assert.Equal(t, "admin", logRecord["actor_role"])
}

func TestEmitBusinessEvent_SensitiveDataRedacted(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test-tracer")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	event := BusinessEvent{
		EventName: "security.test_event",
		Domain:    "security",
		Action:    "test",
		Result:    "ok",
		Attributes: []slog.Attr{
			slog.String("user_password", "supersecret123"),
			slog.String("auth_token", "jwt.token.here"),
			slog.String("card_number", "1234567812345678"),
			slog.String("public_info", "visible"),
		},
	}

	EmitBusinessEvent(ctx, logger, event)
	span.End()

	var logRecord map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logRecord)
	require.NoError(t, err)
	assert.Equal(t, "[REDACTED]", logRecord["user_password"])
	assert.Equal(t, "[REDACTED]", logRecord["auth_token"])
	assert.Equal(t, "[REDACTED]", logRecord["card_number"])
	assert.Equal(t, "visible", logRecord["public_info"])

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Len(t, spans[0].Events, 1)

	spanAttrMap := make(map[string]interface{})
	for _, a := range spans[0].Events[0].Attributes {
		spanAttrMap[string(a.Key)] = a.Value.AsInterface()
	}
	assert.Equal(t, "[REDACTED]", spanAttrMap["user_password"])
	assert.Equal(t, "[REDACTED]", spanAttrMap["auth_token"])
	assert.Equal(t, "[REDACTED]", spanAttrMap["card_number"])
	assert.Equal(t, "visible", spanAttrMap["public_info"])
}

func TestWarehouseMetrics_RecordsWithoutPanic(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	meter := tp.Tracer("test").(interface{}).(trace.Tracer)
	_ = meter

	// Use Provider with nil/noop safe checks
	assert.NotPanics(t, func() {
		RecordReconciliationResolution(context.Background(), "confirm_missing", "success")
		RecordPickingScan(context.Background(), "ok")
		RecordInventoryWriteoff(context.Background(), "reconciliation_missing")
	})
}
