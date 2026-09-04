package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Provider bundles initialized OpenTelemetry providers.
type Provider struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
	Metrics        *HTTPMetrics
	DBMetrics      *DBMetrics
	RedisMetrics   *RedisMetrics
	Tracer         trace.Tracer
	logger         *slog.Logger
	cfg            Config
}

// Init initializes OpenTelemetry traces, metrics, and logs with OTLP exporters.
// If the OTLP endpoint is unavailable or initialization encounters an error,
// it gracefully logs a warning and returns a safe fallback provider so the API
// never crashes or blocks request processing.
func Init(ctx context.Context, cfg Config, logger *slog.Logger) (*Provider, error) {
	if !cfg.Enabled {
		if logger != nil {
			logger.Info("observability is disabled by configuration, running with no-op telemetry")
		}
		return &Provider{
			Tracer: noop.NewTracerProvider().Tracer(cfg.ServiceName),
			cfg:    cfg,
		}, nil
	}

	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to create otel resource, falling back to default", "error", err)
		}
		res = resource.Default()
	}

	// 1. Trace Exporter & Provider
	var traceOpts []otlptracegrpc.Option
	traceOpts = append(traceOpts, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint))
	if cfg.OTLPInsecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}

	traceExp, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to initialize OTLP trace exporter", "error", err)
		}
	}

	var tp *sdktrace.TracerProvider
	if traceExp != nil {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
	} else {
		otel.SetTracerProvider(noop.NewTracerProvider())
	}

	// Set global W3C text map propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 2. Metric Exporter & Provider
	var metricOpts []otlpmetricgrpc.Option
	metricOpts = append(metricOpts, otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint))
	if cfg.OTLPInsecure {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}

	metricExp, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to initialize OTLP metric exporter", "error", err)
		}
	}

	var mp *sdkmetric.MeterProvider
	if metricExp != nil {
		// Periodic reader with 2s interval for local dev and prompt telemetry ingestion
		reader := sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(2*time.Second))
		mp = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
	}

	meter := otel.GetMeterProvider().Meter(cfg.ServiceName)
	httpMetrics, err := NewHTTPMetrics(meter)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to initialize HTTP metrics", "error", err)
		}
	}

	dbMetrics, err := NewDBMetrics(meter)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to initialize DB metrics", "error", err)
		}
	}

	redisMetrics, err := NewRedisMetrics(meter)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to initialize Redis metrics", "error", err)
		}
	}

	// 3. Log Exporter & Provider
	var logOpts []otlploggrpc.Option
	logOpts = append(logOpts, otlploggrpc.WithEndpoint(cfg.OTLPEndpoint))
	if cfg.OTLPInsecure {
		logOpts = append(logOpts, otlploggrpc.WithInsecure())
	}

	logExp, err := otlploggrpc.New(ctx, logOpts...)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to initialize OTLP log exporter", "error", err)
		}
	}

	var lp *sdklog.LoggerProvider
	if logExp != nil {
		lp = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
			sdklog.WithResource(res),
		)
	}

	tracer := otel.GetTracerProvider().Tracer(cfg.ServiceName)

	return &Provider{
		TracerProvider: tp,
		MeterProvider:  mp,
		LoggerProvider: lp,
		Metrics:        httpMetrics,
		DBMetrics:      dbMetrics,
		RedisMetrics:   redisMetrics,
		Tracer:         tracer,
		logger:         logger,
		cfg:            cfg,
	}, nil
}

// NewPgTracer creates a centrally-configured PgTracer using the provider's tracer and metrics.
func (p *Provider) NewPgTracer(host, port, dbName string, slowThreshold ...time.Duration) *PgTracer {
	var tracer trace.Tracer
	var logger *slog.Logger
	var metrics *DBMetrics
	threshold := 250 * time.Millisecond
	if p != nil {
		tracer = p.Tracer
		logger = p.logger
		metrics = p.DBMetrics
		if p.cfg.DBSlowQueryThresholdMs > 0 {
			threshold = time.Duration(p.cfg.DBSlowQueryThresholdMs) * time.Millisecond
		}
	}
	if len(slowThreshold) > 0 && slowThreshold[0] > 0 {
		threshold = slowThreshold[0]
	}
	return NewPgTracer(tracer, logger, metrics, host, port, dbName, threshold)
}

// NewRedisHook creates a centrally-configured RedisHook using the provider's tracer and metrics.
func (p *Provider) NewRedisHook(addr string, slowThreshold ...time.Duration) *RedisHook {
	var tracer trace.Tracer
	var logger *slog.Logger
	var metrics *RedisMetrics
	threshold := 50 * time.Millisecond
	if p != nil {
		tracer = p.Tracer
		logger = p.logger
		metrics = p.RedisMetrics
		if p.cfg.RedisSlowOpThresholdMs > 0 {
			threshold = time.Duration(p.cfg.RedisSlowOpThresholdMs) * time.Millisecond
		}
	}
	if len(slowThreshold) > 0 && slowThreshold[0] > 0 {
		threshold = slowThreshold[0]
	}
	return NewRedisHook(tracer, logger, metrics, addr, threshold)
}

// RegisterPoolMetrics registers asynchronous gauge metrics for the given pgxpool.Pool.
func (p *Provider) RegisterPoolMetrics(pool *pgxpool.Pool) error {
	if p == nil || pool == nil {
		return nil
	}
	meter := otel.GetMeterProvider().Meter(p.cfg.ServiceName)
	_, err := RegisterPoolMetrics(meter, pool)
	return err
}

// Shutdown flushes in-flight telemetry and gracefully shuts down providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}

	var errs []error

	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
	}

	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	if p.LoggerProvider != nil {
		if err := p.LoggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("logger provider shutdown: %w", err))
		}
	}

	return errors.Join(errs...)
}
