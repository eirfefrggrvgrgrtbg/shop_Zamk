package observability

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics manages operational HTTP metrics using low-cardinality labels.
type HTTPMetrics struct {
	requestsTotal   metric.Int64Counter
	requestDuration metric.Float64Histogram
}

// NewHTTPMetrics initializes HTTP metrics registered with the given MeterProvider.
func NewHTTPMetrics(meter metric.Meter) (*HTTPMetrics, error) {
	requestsTotal, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total count of processed HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	return &HTTPMetrics{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
	}, nil
}

// Record records request count and latency for an HTTP request.
// Only bounded, low-cardinality labels (method, route template, status code) are permitted.
func (m *HTTPMetrics) Record(ctx context.Context, method, route string, statusCode int, duration time.Duration) {
	if m == nil {
		return
	}

	if route == "" {
		route = "unmatched"
	}

	attrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("route", route),
		attribute.String("status", strconv.Itoa(statusCode)),
	)

	m.requestsTotal.Add(ctx, 1, attrs)
	m.requestDuration.Record(ctx, duration.Seconds(), attrs)
}

// DBMetrics manages operational metrics for database operations.
type DBMetrics struct {
	operationsTotal   metric.Int64Counter
	operationDuration metric.Float64Histogram
	errorsTotal       metric.Int64Counter
}

// NewDBMetrics initializes database metrics registered with the given meter.
func NewDBMetrics(meter metric.Meter) (*DBMetrics, error) {
	operationsTotal, err := meter.Int64Counter(
		"db_operations_total",
		metric.WithDescription("Total count of database operations"),
	)
	if err != nil {
		return nil, err
	}

	operationDuration, err := meter.Float64Histogram(
		"db_operation_duration_seconds",
		metric.WithDescription("Duration of database operations in seconds"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5),
	)
	if err != nil {
		return nil, err
	}

	errorsTotal, err := meter.Int64Counter(
		"db_errors_total",
		metric.WithDescription("Total count of database errors by SQLSTATE"),
	)
	if err != nil {
		return nil, err
	}

	return &DBMetrics{
		operationsTotal:   operationsTotal,
		operationDuration: operationDuration,
		errorsTotal:       errorsTotal,
	}, nil
}

// RecordOperation records operation count and duration using strictly bounded labels.
func (m *DBMetrics) RecordOperation(ctx context.Context, system, operation, status string, duration time.Duration) {
	if m == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("db_system", system),
		attribute.String("operation", operation),
		attribute.String("status", status),
	)
	m.operationsTotal.Add(ctx, 1, attrs)
	m.operationDuration.Record(ctx, duration.Seconds(), attrs)
}

// RecordError records database error count classified by sql_state.
func (m *DBMetrics) RecordError(ctx context.Context, system, sqlState string) {
	if m == nil {
		return
	}
	if sqlState == "" {
		sqlState = "UNKNOWN"
	}
	attrs := metric.WithAttributes(
		attribute.String("db_system", system),
		attribute.String("sql_state", sqlState),
	)
	m.errorsTotal.Add(ctx, 1, attrs)
}

// RedisMetrics manages operational metrics for Redis operations.
type RedisMetrics struct {
	operationsTotal   metric.Int64Counter
	operationDuration metric.Float64Histogram
	errorsTotal       metric.Int64Counter
}

// NewRedisMetrics initializes Redis metrics registered with the given meter.
func NewRedisMetrics(meter metric.Meter) (*RedisMetrics, error) {
	operationsTotal, err := meter.Int64Counter(
		"redis_operations_total",
		metric.WithDescription("Total count of Redis operations"),
	)
	if err != nil {
		return nil, err
	}

	operationDuration, err := meter.Float64Histogram(
		"redis_operation_duration_seconds",
		metric.WithDescription("Duration of Redis operations in seconds"),
		metric.WithExplicitBucketBoundaries(0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5),
	)
	if err != nil {
		return nil, err
	}

	errorsTotal, err := meter.Int64Counter(
		"redis_errors_total",
		metric.WithDescription("Total count of unexpected Redis errors"),
	)
	if err != nil {
		return nil, err
	}

	return &RedisMetrics{
		operationsTotal:   operationsTotal,
		operationDuration: operationDuration,
		errorsTotal:       errorsTotal,
	}, nil
}

// RecordOperation records Redis command count and duration.
func (m *RedisMetrics) RecordOperation(ctx context.Context, operation, status string, duration time.Duration) {
	if m == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("db_system", "redis"),
		attribute.String("operation", operation),
		attribute.String("status", status),
	)
	m.operationsTotal.Add(ctx, 1, attrs)
	m.operationDuration.Record(ctx, duration.Seconds(), attrs)
}

// RecordError records unexpected Redis errors.
func (m *RedisMetrics) RecordError(ctx context.Context, operation string) {
	if m == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("db_system", "redis"),
		attribute.String("operation", operation),
	)
	m.errorsTotal.Add(ctx, 1, attrs)
}

// RegisterPoolMetrics registers an observable gauge for pgxpool connection metrics.
func RegisterPoolMetrics(meter metric.Meter, pool *pgxpool.Pool) (metric.Registration, error) {
	if pool == nil || meter == nil {
		return nil, nil
	}

	gauge, err := meter.Int64ObservableGauge(
		"db_pool_connections",
		metric.WithDescription("PostgreSQL connection pool metrics"),
	)
	if err != nil {
		return nil, err
	}

	reg, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		stat := pool.Stat()
		observer.ObserveInt64(gauge, int64(stat.AcquiredConns()), metric.WithAttributes(attribute.String("state", "acquired")))
		observer.ObserveInt64(gauge, int64(stat.IdleConns()), metric.WithAttributes(attribute.String("state", "idle")))
		observer.ObserveInt64(gauge, int64(stat.TotalConns()), metric.WithAttributes(attribute.String("state", "total")))
		observer.ObserveInt64(gauge, int64(stat.MaxConns()), metric.WithAttributes(attribute.String("state", "max")))
		return nil
	}, gauge)
	return reg, err
}

// WarehouseMetrics manages operational metrics for warehouse operations.
type WarehouseMetrics struct {
	reconciliationResolutionsTotal metric.Int64Counter
	pickingScansTotal              metric.Int64Counter
	inventoryWriteoffsTotal        metric.Int64Counter
}

// NewWarehouseMetrics initializes warehouse metrics registered with the given meter.
func NewWarehouseMetrics(meter metric.Meter) (*WarehouseMetrics, error) {
	if meter == nil {
		return nil, errors.New("meter is required")
	}

	reconciliationResolutionsTotal, err := meter.Int64Counter(
		"warehouse_reconciliation_resolutions_total",
		metric.WithDescription("Total count of inventory reconciliation resolution decisions"),
	)
	if err != nil {
		return nil, err
	}

	pickingScansTotal, err := meter.Int64Counter(
		"warehouse_picking_scans_total",
		metric.WithDescription("Total count of warehouse picking barcode/ZMU scans"),
	)
	if err != nil {
		return nil, err
	}

	inventoryWriteoffsTotal, err := meter.Int64Counter(
		"inventory_writeoffs_total",
		metric.WithDescription("Total count of physical inventory units written off"),
	)
	if err != nil {
		return nil, err
	}

	return &WarehouseMetrics{
		reconciliationResolutionsTotal: reconciliationResolutionsTotal,
		pickingScansTotal:              pickingScansTotal,
		inventoryWriteoffsTotal:        inventoryWriteoffsTotal,
	}, nil
}

// RecordReconciliationResolution records a reconciliation resolution decision with low-cardinality labels.
func (m *WarehouseMetrics) RecordReconciliationResolution(ctx context.Context, action, result string) {
	if m == nil || m.reconciliationResolutionsTotal == nil {
		return
	}
	if action == "" {
		action = "unknown"
	}
	if result == "" {
		result = "unknown"
	}
	attrs := metric.WithAttributes(
		attribute.String("action", action),
		attribute.String("result", result),
	)
	m.reconciliationResolutionsTotal.Add(ctx, 1, attrs)
}

// RecordPickingScan records a picking scan result with low-cardinality label.
func (m *WarehouseMetrics) RecordPickingScan(ctx context.Context, result string) {
	if m == nil || m.pickingScansTotal == nil {
		return
	}
	if result == "" {
		result = "unknown"
	}
	attrs := metric.WithAttributes(
		attribute.String("result", result),
	)
	m.pickingScansTotal.Add(ctx, 1, attrs)
}

// RecordInventoryWriteoff records an inventory write-off with low-cardinality reason category.
func (m *WarehouseMetrics) RecordInventoryWriteoff(ctx context.Context, reasonCategory string) {
	if m == nil || m.inventoryWriteoffsTotal == nil {
		return
	}
	if reasonCategory == "" {
		reasonCategory = "other"
	}
	attrs := metric.WithAttributes(
		attribute.String("reason_category", reasonCategory),
	)
	m.inventoryWriteoffsTotal.Add(ctx, 1, attrs)
}

var (
	globalWarehouseMetricsMu sync.RWMutex
	globalWarehouseMetrics   *WarehouseMetrics
)

// SetGlobalWarehouseMetrics sets the global warehouse metrics instance.
func SetGlobalWarehouseMetrics(m *WarehouseMetrics) {
	globalWarehouseMetricsMu.Lock()
	defer globalWarehouseMetricsMu.Unlock()
	globalWarehouseMetrics = m
}

// GetGlobalWarehouseMetrics returns the global warehouse metrics instance.
func GetGlobalWarehouseMetrics() *WarehouseMetrics {
	globalWarehouseMetricsMu.RLock()
	defer globalWarehouseMetricsMu.RUnlock()
	return globalWarehouseMetrics
}

// RecordReconciliationResolution records a reconciliation resolution metric globally.
func RecordReconciliationResolution(ctx context.Context, action, result string) {
	if m := GetGlobalWarehouseMetrics(); m != nil {
		m.RecordReconciliationResolution(ctx, action, result)
	}
}

// RecordPickingScan records a picking scan metric globally.
func RecordPickingScan(ctx context.Context, result string) {
	if m := GetGlobalWarehouseMetrics(); m != nil {
		m.RecordPickingScan(ctx, result)
	}
}

// RecordInventoryWriteoff records an inventory write-off metric globally.
func RecordInventoryWriteoff(ctx context.Context, reasonCategory string) {
	if m := GetGlobalWarehouseMetrics(); m != nil {
		m.RecordInventoryWriteoff(ctx, reasonCategory)
	}
}
