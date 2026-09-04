package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func main() {
	// Initialize bootstrap logger
	bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(bootstrapLogger)

	// Load .env locally if present
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		bootstrapLogger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Initialize observability provider (OTEL traces, metrics, logs)
	obsProvider, err := observability.Init(ctx, cfg.Observability, bootstrapLogger)
	if err != nil {
		bootstrapLogger.Warn("observability init reported error, running with fallback", "error", err)
	}

	// Initialize canonical structured logger connected to OTel + stdout
	logger := observability.NewLogger(cfg.Observability, obsProvider.LoggerProvider, os.Stdout)
	slog.SetDefault(logger)

	// Connect to PostgreSQL with central observability
	pgTracer := obsProvider.NewPgTracer(cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Database)
	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN, postgres.WithTracer(pgTracer))
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgClient.Close()
	if err := obsProvider.RegisterPoolMetrics(pgClient.Pool); err != nil {
		logger.Warn("failed to register postgres pool metrics", "error", err)
	}

	// Connect to Redis with central observability
	redisHook := obsProvider.NewRedisHook(cfg.Redis.Addr)
	redisClient, err := redis.NewClient(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, redis.WithHook(redisHook))
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// Startup Database & Schema Diagnostics
	if _, err := observability.RunStartupDiagnostics(ctx, pgClient.Pool, redisClient.Client, cfg.Postgres.Database, cfg.Redis.Addr, cfg.Observability.MigrationsPath, logger); err != nil {
		logger.Warn("startup diagnostics check reported warning", "error", err)
	}

	r, cancelWorkers := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger, obsProvider)
	defer cancelWorkers()

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.App.Port,
		Handler: r,
	}

	// Channel to listen for errors coming from the listener.
	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("starting api server", "port", cfg.App.Port)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for an interrupt or terminate signal from the OS.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			obsShutdownCtx, obsCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer obsCancel()
			_ = obsProvider.Shutdown(obsShutdownCtx)
			os.Exit(1)
		}
	case sig := <-shutdown:
		logger.Info("start shutdown", "signal", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cancelWorkers()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown did not complete", "error", err)
			if err := srv.Close(); err != nil {
				logger.Error("could not stop server gracefully", "error", err)
			}
		}

		obsShutdownCtx, obsCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer obsCancel()
		if err := obsProvider.Shutdown(obsShutdownCtx); err != nil {
			logger.Error("failed to shutdown observability provider cleanly", "error", err)
		}
	}

	logger.Info("api server stopped")
}
