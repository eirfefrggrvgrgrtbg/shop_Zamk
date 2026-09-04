package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	Tracer         trace.Tracer
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
		Tracer:         tracer,
		cfg:            cfg,
	}, nil
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
