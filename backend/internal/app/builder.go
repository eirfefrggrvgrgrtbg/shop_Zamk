package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/addresses"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/admin/dashboard"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/admin/reports"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auctions"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/audit"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/delivery"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/favorites"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/router"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/ratelimit"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/reviews"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/selleranalytics"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testlab"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

func BuildRouter(ctx context.Context, cfg *config.Config, pgClient *postgres.Client, redisClient *redis.Client, logger *slog.Logger) (*chi.Mux, func()) {
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

	devEmailSender := notifications.NewDevEmailSender(logger)
	notificationsRepo := notifications.NewRepository(pgClient)
	notificationsService := notifications.NewService(notificationsRepo, userRepo, devEmailSender)
	notificationsHandler := notifications.NewHandler(notificationsService, logger)

	sellersService := sellers.NewService(sellersRepo, userRepo, pgClient, notificationsService)
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
	ordersService := orders.NewService(ordersRepo, cartRepo, inventoryService, pgClient, cfg)
	ordersHandler := orders.NewHandler(ordersService)

	reviewsRepo := reviews.NewRepository(pgClient)
	reviewsService := reviews.NewService(reviewsRepo, ordersRepo, sellersRepo, pgClient, notificationsService)
	reviewsHandler := reviews.NewHandler(reviewsService)

	productsRepo := products.NewRepository(pgClient.Pool)
	productsService := products.NewService(productsRepo, sellersRepo, pgClient, reviewsService, notificationsService).WithRedis(redisClient)
	productsHandler := products.NewHandler(productsService, sellersService)

	tbankProvider := payments.NewTBankProvider(
		cfg.TBank.TerminalKey,
		cfg.TBank.Password,
		cfg.TBank.APIBaseURL,
		cfg.TBank.SuccessURL,
		cfg.TBank.FailURL,
		cfg.TBank.TPayEnabled,
		cfg.TBank.PayType,
		cfg.TBank.TPayMode,
	)

	paymentsRepo := payments.NewRepository(pgClient.Pool)
	paymentsService := payments.NewService(paymentsRepo, ordersRepo, inventoryService, tbankProvider, pgClient, notificationsService, cfg)
	paymentsHandler := payments.NewHandler(paymentsService, cfg.App.Env)

	returnsRepo := returns.NewRepository(pgClient.Pool)

	payoutsRepo := payouts.NewRepository(pgClient.Pool)
	payoutsService := payouts.NewService(payoutsRepo, pgClient, returnsRepo, ordersRepo, cfg, notificationsService)
	payoutsHandler := payouts.NewHandler(payoutsService)

	fulfillmentRepo := fulfillment.NewRepository(pgClient.Pool)
	fulfillmentService := fulfillment.NewService(fulfillmentRepo, ordersRepo, pgClient, payoutsService, notificationsService)
	fulfillmentHandler := fulfillment.NewHandler(fulfillmentService)

	deliveryRepo := delivery.NewRepository(pgClient)
	deliveryService := delivery.NewService(deliveryRepo)
	deliveryHandler := delivery.NewHandler(deliveryService)

	suppliesRepo := supplies.NewRepository(pgClient.Pool)
	suppliesService := supplies.NewService(pgClient.Pool, suppliesRepo)
	suppliesHandler := supplies.NewHandler(suppliesService, logger)

	// In test mode without S3 configured, this might fail, so check err or configure dummy
	var storageHandler *storage.Handler
	storageProvider, err := storage.NewS3Client(&cfg.S3)
	if err == nil {
		storageService := storage.NewService(storageProvider, productsRepo, catalogRepo, sellersRepo, pgClient)
		storageHandler = storage.NewHandler(storageService, &cfg.S3)
	} else {
		// Log error if it happens in prod, but allow missing S3 in tests?
		logger.Warn("failed to create storage provider, continuing without storage handler", "error", err)
	}

	returnsService := returns.NewService(returnsRepo, ordersRepo, inventoryService, pgClient, payoutsService, paymentsService, cfg.Worker.ReturnWindowDays, notificationsService, storageProvider)
	returnsHandler := returns.NewHandler(returnsService)


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

	favoritesRepo := favorites.NewRepository(pgClient.Pool)
	favoritesService := favorites.NewService(favoritesRepo)
	favoritesHandler := favorites.NewHandler(favoritesService)

	usersHandler := users.NewHandler(userRepo)

	addressesRepo := addresses.NewRepository(pgClient)
	addressesService := addresses.NewService(addressesRepo)
	addressesHandler := addresses.NewHandler(addressesService)

	var auctionsLimiter *ratelimit.Limiter
	if redisClient != nil && redisClient.Client != nil {
		auctionsLimiter = ratelimit.New(redisClient.Client)
	}
	auctionsRepo := auctions.NewRepository(pgClient.Pool)
	auctionsHub := auctions.NewSSEHub()
	auctionsService := auctions.NewService(auctionsRepo, notificationsService, auctionsLimiter, auctionsHub)
	auctionsAdminHandler := auctions.NewAdminHandler(auctionsRepo, auctionsService, logger)
	auctionsPublicHandler := auctions.NewPublicHandler(auctionsRepo, auctionsService, logger)
	auctionsCustomerHandler := auctions.NewCustomerHandler(auctionsRepo, auctionsService, logger)

	dashboardRepo := dashboard.NewRepository(pgClient.Pool)
	dashboardService := dashboard.NewService(dashboardRepo)
	dashboardHandler := dashboard.NewHandler(dashboardService)

	reportsHandler := reports.NewHandler(dashboardService, logger)

	auditLogRepo := audit.NewRepository(pgClient.Pool)
	auditLogService := audit.NewService(auditLogRepo, logger)
	auditLogHandler := audit.NewHandler(auditLogService, logger)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	if cfg.Worker.AuctionMaintenanceEnabled {
		go auctions.StartMaintenanceWorker(
			workerCtx,
			auctionsService,
			time.Duration(cfg.Worker.AuctionMaintenanceIntervalSecs)*time.Second,
			cfg.Worker.AuctionMaintenanceBatchLimit,
			logger,
		)
	}

	analyticsRepo := selleranalytics.NewRepository(pgClient.Pool)
	analyticsSvc := selleranalytics.NewService(analyticsRepo)
	analyticsHandler := selleranalytics.NewHandler(analyticsSvc, analyticsRepo)

	var testLabHandler *testlab.Handler
	if cfg.App.Env == "local" || cfg.App.Env == "test" {
		testLabRepo := testlab.NewRepository(pgClient.Pool)
		testLabCalc := testlab.NewCalculator()
		testLabEngine := testlab.NewScenarioEngine(testlab.EngineDeps{
			UserRepo:   userRepo,
			SellerSvc:  sellersService,
			ProductSvc: productsService,
			InvSvc:     inventoryService,
			CartSvc:    cartService,
			OrderSvc:   ordersService,
			PayoutSvc:  payoutsService,
			TestRepo:   testLabRepo,
			Calc:       testLabCalc,
		})
		testLabSvc := testlab.NewService(testLabRepo, testLabEngine)
		testLabHandler = testlab.NewHandler(testLabSvc, cfg.App.Env)
	}

	r := router.New(cfg, pgClient, redisClient, logger, authHandler, tokenService, sellersHandler, catalogHandler, productsHandler, inventoryHandler, cartHandler, ordersHandler, paymentsHandler, fulfillmentHandler, returnsHandler, payoutsHandler, reviewsHandler, storageHandler, staffHandler, staffAuditRepo, staffService, favoritesHandler, usersHandler, addressesHandler, notificationsHandler, auctionsAdminHandler, auctionsPublicHandler, auctionsCustomerHandler, dashboardHandler, reportsHandler, auditLogHandler, deliveryHandler, suppliesHandler, analyticsHandler, testLabHandler)

	return r, cancelWorkers
}
