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

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/router"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/reviews"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load .env locally if present
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgClient.Close()
	logger.Info("connected to postgres")

	// Connect to Redis
	redisClient, err := redis.NewClient(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	logger.Info("connected to redis")

	userRepo := users.NewRepository(pgClient.Pool)
	authRepo := auth.NewRepository(pgClient)
	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)
	authService := auth.NewService(authRepo, userRepo, tokenService, cfg.JWT.RefreshTokenTTLDays)
	authHandler := auth.NewHandler(authService, cfg.JWT.RefreshTokenTTLDays, auth.CookieConfig{
		Domain:   cfg.Auth.CookieDomain,
		Secure:   cfg.Auth.CookieSecure,
		SameSite: cfg.Auth.CookieSameSite,
	})

	sellersRepo := sellers.NewRepository(pgClient.Pool)
	sellersService := sellers.NewService(sellersRepo, userRepo, pgClient)
	sellersHandler := sellers.NewHandler(sellersService)

	catalogRepo := catalog.NewRepository(pgClient.Pool)
	catalogService := catalog.NewService(catalogRepo)
	catalogHandler := catalog.NewHandler(catalogService)

	inventoryRepo := inventory.NewRepository(pgClient.Pool)
	inventoryService := inventory.NewService(inventoryRepo, sellersRepo, pgClient)
	inventoryHandler := inventory.NewHandler(inventoryService)

	cartRepo := cart.NewRepository(pgClient.Pool)
	cartService := cart.NewService(cartRepo)
	cartHandler := cart.NewHandler(cartService)

	ordersRepo := orders.NewRepository(pgClient.Pool)
	ordersService := orders.NewService(ordersRepo, cartRepo, inventoryService, pgClient)
	ordersHandler := orders.NewHandler(ordersService)

	reviewsRepo := reviews.NewRepository(pgClient)
	reviewsService := reviews.NewService(reviewsRepo, ordersRepo, sellersRepo, pgClient)
	reviewsHandler := reviews.NewHandler(reviewsService)

	productsRepo := products.NewRepository(pgClient.Pool)
	productsService := products.NewService(productsRepo, sellersRepo, pgClient, reviewsService)
	productsHandler := products.NewHandler(productsService, sellersService)

	tbankProvider := payments.NewTBankProvider(
		cfg.TBank.TerminalKey,
		cfg.TBank.Password,
		cfg.TBank.APIBaseURL,
		cfg.TBank.SuccessURL,
		cfg.TBank.FailURL,
	)
	paymentsRepo := payments.NewRepository(pgClient.Pool)
	paymentsService := payments.NewService(paymentsRepo, ordersRepo, inventoryService, tbankProvider, pgClient)
	paymentsHandler := payments.NewHandler(paymentsService)

	returnsRepo := returns.NewRepository(pgClient.Pool)

	payoutsRepo := payouts.NewRepository(pgClient.Pool)
	payoutsService := payouts.NewService(payoutsRepo, pgClient, returnsRepo, ordersRepo, cfg)
	payoutsHandler := payouts.NewHandler(payoutsService)

	fulfillmentRepo := fulfillment.NewRepository(pgClient.Pool)
	fulfillmentService := fulfillment.NewService(fulfillmentRepo, ordersRepo, pgClient, payoutsService)
	fulfillmentHandler := fulfillment.NewHandler(fulfillmentService)

	returnsService := returns.NewService(returnsRepo, ordersRepo, inventoryService, pgClient, payoutsService, cfg.Worker.ReturnWindowDays)
	returnsHandler := returns.NewHandler(returnsService)

	storageProvider, err := storage.NewS3Client(&cfg.S3)
	if err != nil {
		logger.Error("failed to create storage provider", "error", err)
		os.Exit(1)
	}
	storageService := storage.NewService(storageProvider, productsRepo, catalogRepo, sellersRepo)
	storageHandler := storage.NewHandler(storageService, &cfg.S3)

	// Staff RBAC
	staffRepo := staff.NewRepository(pgClient.Pool)
	staffAuditRepo := staff.NewAuditRepository(pgClient.Pool)
	staffService := staff.NewService(staffRepo, userRepo, pgClient)
	staffHandler := staff.NewHandler(staffService, staffAuditRepo, userRepo)
	sellersHandler = sellersHandler.WithAudit(staffAuditRepo)
	payoutsHandler = payoutsHandler.WithAudit(staffAuditRepo).WithStaffSvc(staffService)
	productsHandler = productsHandler.WithAudit(staffAuditRepo)
	inventoryHandler = inventoryHandler.WithAudit(staffAuditRepo)
	ordersHandler = ordersHandler.WithAudit(staffAuditRepo)
	fulfillmentHandler = fulfillmentHandler.WithAudit(staffAuditRepo)
	returnsHandler = returnsHandler.WithAudit(staffAuditRepo)
	reviewsHandler = reviewsHandler.WithAudit(staffAuditRepo).WithStaffSvc(staffService)

	// Create router
	r := router.New(cfg, pgClient, redisClient, logger, authHandler, tokenService, sellersHandler, catalogHandler, productsHandler, inventoryHandler, cartHandler, ordersHandler, paymentsHandler, fulfillmentHandler, returnsHandler, payoutsHandler, reviewsHandler, storageHandler, staffHandler, staffAuditRepo, staffService)

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
			os.Exit(1)
		}
	case sig := <-shutdown:
		logger.Info("start shutdown", "signal", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown did not complete", "error", err)
			if err := srv.Close(); err != nil {
				logger.Error("could not stop server gracefully", "error", err)
			}
		}
	}

	logger.Info("api server stopped")
}
