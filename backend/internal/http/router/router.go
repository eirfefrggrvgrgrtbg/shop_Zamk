package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/health"
	appMiddleware "github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/middleware"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/ratelimit"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/reviews"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/favorites"
)

func New(
	cfg *config.Config,
	pg *postgres.Client,
	rdb *redis.Client,
	logger *slog.Logger,
	authHandler *auth.Handler,
	tokenService *auth.TokenService,
	sellersHandler *sellers.Handler,
	catalogHandler *catalog.Handler,
	productsHandler *products.Handler,
	inventoryHandler *inventory.Handler,
	cartHandler *cart.Handler,
	ordersHandler *orders.Handler,
	paymentsHandler *payments.Handler,
	fulfillmentHandler *fulfillment.Handler,
	returnsHandler *returns.Handler,
	payoutsHandler *payouts.Handler,
	reviewsHandler *reviews.Handler,
	storageHandler *storage.Handler,
	staffHandler *staff.Handler,
	auditRepo *staff.AuditRepository,
	staffSvc *staff.Service,
	favoritesHandler *favorites.Handler,
) *chi.Mux {
	r := chi.NewRouter()
	rateLimiter := ratelimit.NewMiddleware(
		ratelimit.New(rdb.Client),
		cfg.RateLimit.Enabled,
		cfg.RateLimit.FailOpenOnRedisError,
		logger,
	)
	loginLimit := rateLimiter.Limit(ratelimit.Rule{Group: "auth_login", Limit: cfg.RateLimit.AuthLoginLimitPerMinute, Window: time.Minute, Key: ratelimit.LoginKey})
	registerLimit := rateLimiter.Limit(ratelimit.Rule{Group: "auth_register", Limit: cfg.RateLimit.AuthRegisterLimitPerHour, Window: time.Hour, Key: ratelimit.RegisterKey})
	refreshLimit := rateLimiter.Limit(ratelimit.Rule{Group: "auth_refresh", Limit: cfg.RateLimit.AuthRefreshLimitPerMinute, Window: time.Minute, Key: ratelimit.RefreshKey})
	changePasswordLimit := rateLimiter.Limit(ratelimit.Rule{Group: "auth_change_password", Limit: cfg.RateLimit.AuthChangePasswordLimitPerHour, Window: time.Hour, Key: ratelimit.UserIPKey("auth_change_password")})
	uploadLimit := rateLimiter.Limit(ratelimit.Rule{Group: "upload", Limit: cfg.RateLimit.UploadLimitPerMinute, Window: time.Minute, Key: ratelimit.UserIPKey("upload")})
	webhookLimit := rateLimiter.Limit(ratelimit.Rule{Group: "payment_webhook", Limit: cfg.RateLimit.WebhookLimitPerMinute, Window: time.Minute, Key: ratelimit.IPKey("payment_webhook")})
	adminDangerousLimit := rateLimiter.Limit(ratelimit.Rule{Group: "admin_dangerous", Limit: cfg.RateLimit.AdminDangerousLimitPerMinute, Window: time.Minute, Key: ratelimit.UserIPKey("admin_dangerous")})

	// Shorthand permission middleware builder
	perm := func(p string) func(http.Handler) http.Handler {
		return appMiddleware.RequirePermission(staffSvc, p)
	}

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")
			allowed := false
			for _, o := range cfg.CORS.AllowedOrigins {
				if o != "*" && o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}

			if req.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, req)
		})
	})

	healthHandler := health.NewHandler(pg, rdb)

	r.Get("/api/health", healthHandler.HealthCheck)
	r.Get("/api/ready", healthHandler.ReadinessCheck)

	r.Route("/api/auth", func(r chi.Router) {
		r.With(registerLimit).Post("/register", authHandler.Register)
		r.With(loginLimit).Post("/login", authHandler.Login)
		r.With(refreshLimit).Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)

		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.AuthMiddleware(tokenService))
			r.Get("/me", authHandler.Me)
			r.With(changePasswordLimit).Post("/change-password", authHandler.ChangePassword)
		})
	})

	r.Route("/api/admin/sellers", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleAdmin))
		r.With(perm("sellers.create_access")).Post("/", sellersHandler.CreateSellerByAdmin)
		r.With(perm("sellers.read")).Get("/", sellersHandler.ListSellers)
		r.With(perm("sellers.read")).Get("/{id}", sellersHandler.GetAdminSellerDetail)
		r.With(perm("sellers.update_status")).Patch("/{id}/status", sellersHandler.UpdateSellerStatus)
		r.With(perm("sellers.verify")).Post("/{id}/verify", sellersHandler.VerifySeller)
		r.With(perm("sellers.read")).Get("/{id}/status-history", sellersHandler.GetSellerStatusHistory)
		r.With(perm("sellers.read")).Get("/{id}/warnings", sellersHandler.ListSellerWarnings)
		r.With(perm("sellers.warn")).Post("/{id}/warnings", sellersHandler.CreateSellerWarning)
		r.With(perm("sellers.warn")).Patch("/{id}/warnings/{warningId}/resolve", sellersHandler.ResolveSellerWarning)
		r.With(perm("sellers.warn")).Patch("/{id}/warnings/{warningId}/cancel", sellersHandler.CancelSellerWarning)
		r.With(perm("sellers.read")).Get("/{id}/violations", sellersHandler.ListSellerViolations)
		r.With(perm("sellers.warn")).Post("/{id}/violations", sellersHandler.CreateSellerViolation)
		r.With(perm("sellers.warn")).Patch("/{id}/violations/{violationId}/resolve", sellersHandler.ResolveSellerViolation)
		r.With(perm("sellers.warn")).Patch("/{id}/violations/{violationId}/cancel", sellersHandler.CancelSellerViolation)
	})

	r.Route("/api/seller/me", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))
		r.Get("/", sellersHandler.GetSellerMe)
		r.Patch("/", sellersHandler.UpdateSellerProfile)
		r.With(uploadLimit).Post("/logo/upload", storageHandler.UploadSellerProfileImage)
	})

	r.Route("/api/seller/warnings", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))
		r.Get("/", sellersHandler.GetMyWarnings)
	})

	r.Route("/api/seller/violations", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))
		r.Get("/", sellersHandler.GetMyViolations)
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Get("/categories", catalogHandler.ListCategories)
		r.Get("/brands", catalogHandler.ListBrands)
		r.Get("/products", productsHandler.ListPublicProducts)
		r.Get("/products/{idOrSlug}", productsHandler.GetPublicProduct)
		r.Get("/sellers/{idOrSlug}", productsHandler.GetPublicSellerStore)
	})

	r.Route("/api/customer", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleCustomer))

		r.Get("/cart", cartHandler.GetCart)
		r.Post("/cart/items", cartHandler.AddItem)
		r.Patch("/cart/items/{id}", cartHandler.UpdateItem)
		r.Delete("/cart/items/{id}", cartHandler.RemoveItem)
		r.Delete("/cart", cartHandler.ClearCart)

		r.Post("/orders", ordersHandler.CreateOrder)
		r.Get("/orders", ordersHandler.ListCustomerOrders)
		r.Get("/orders/{id}", ordersHandler.GetCustomerOrder)
		r.Post("/orders/{id}/cancel", ordersHandler.CancelCustomerOrder)
		r.Post("/orders/{id}/payment", paymentsHandler.CreatePayment)
		r.Get("/orders/{id}/shipment", fulfillmentHandler.GetCustomerShipment)
		r.Post("/orders/{id}/returns", returnsHandler.CreateCustomerReturn)
		r.Get("/returns", returnsHandler.ListCustomerReturns)
		r.Get("/returns/{id}", returnsHandler.GetCustomerReturn)

		r.Get("/favorites", favoritesHandler.ListFavorites)
		r.Post("/favorites/{productId}", favoritesHandler.AddFavorite)
		r.Delete("/favorites/{productId}", favoritesHandler.RemoveFavorite)
	})

	r.With(webhookLimit).Post("/api/payments/tbank/webhook", paymentsHandler.HandleTBankWebhook)

	r.Route("/api/seller/products", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))

		r.Get("/", productsHandler.ListSellerProducts)
		r.Post("/", productsHandler.CreateProduct)
		r.Get("/{id}", productsHandler.GetSellerProduct)
		r.Patch("/{id}", productsHandler.UpdateProduct)
		r.Delete("/{id}", productsHandler.DeleteDraftProduct)
		r.Post("/{id}/submit-moderation", productsHandler.SubmitForModeration)
		r.Get("/{id}/moderation-history", productsHandler.GetModerationHistory)
		r.With(uploadLimit).Post("/{id}/images/upload", storageHandler.UploadSellerProductImage)
	})

	r.Route("/api/seller/inventory", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))

		r.Get("/", inventoryHandler.ListSellerInventory)
		r.Get("/{id}", inventoryHandler.GetSellerInventoryItem)
		r.Get("/{id}/movements", inventoryHandler.ListSellerMovements)
	})

	r.Route("/api/seller/orders", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))

		r.Get("/", ordersHandler.ListSellerOrders)
		r.Get("/{id}", ordersHandler.GetSellerOrder)
		r.Get("/{id}/shipment", fulfillmentHandler.GetSellerShipment)
	})

	r.Route("/api/seller/returns", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))

		r.Get("/", returnsHandler.ListSellerReturns)
		r.Get("/{id}", returnsHandler.GetSellerReturn)
	})

	r.Route("/api/seller/balance", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))

		r.Get("/", payoutsHandler.GetSellerBalance)
	})

	r.Route("/api/seller/payouts", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))

		r.Get("/", payoutsHandler.ListSellerPayouts)
		r.Post("/request", payoutsHandler.RequestPayout)
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleAdmin))

		// /api/admin/me — no fine-grained permission required, role=admin is enough
		r.Get("/me", staffHandler.GetAdminMe)

		// Staff RBAC endpoints
		r.With(perm("roles.read")).Get("/staff/roles", staffHandler.GetStaffRoles)
		r.With(perm("audit.read")).Get("/audit-logs", staffHandler.GetAuditLogs)
		r.With(perm("staff.read")).Get("/staff/members", staffHandler.ListStaffMembers)
		r.With(perm("staff.create")).Post("/staff/members", staffHandler.CreateStaffMember)
		r.With(perm("staff.update")).Patch("/staff/members/{userId}/role", staffHandler.UpdateStaffRole)
		r.With(perm("staff.block")).Patch("/staff/members/{userId}/status", staffHandler.UpdateStaffStatus)
		r.With(perm("staff.update")).Post("/staff/members/{userId}/reset-password", staffHandler.ResetStaffPassword)

		// Catalog
		r.With(perm("categories.read")).Get("/categories", catalogHandler.ListCategories)
		r.With(perm("categories.create")).Post("/categories", catalogHandler.CreateCategory)
		r.With(perm("brands.read")).Get("/brands", catalogHandler.ListBrands)
		r.With(perm("brands.create")).Post("/brands", catalogHandler.CreateBrand)
		r.With(uploadLimit, perm("brands.update")).Post("/brands/{id}/logo/upload", storageHandler.UploadAdminBrandLogo)

		// Products
		r.With(perm("products.read")).Get("/products", productsHandler.ListAdminProducts)
		r.With(uploadLimit, perm("products.moderate")).Post("/products/{id}/images/upload", storageHandler.UploadAdminProductImage)
		r.With(perm("products.moderate")).Get("/moderation/products", productsHandler.ListModerationProducts)
		r.With(adminDangerousLimit, perm("products.approve")).Post("/moderation/products/{id}/approve", productsHandler.AdminApproveProduct)
		r.With(adminDangerousLimit, perm("products.reject")).Post("/moderation/products/{id}/reject", productsHandler.AdminRejectProduct)
		r.With(adminDangerousLimit, perm("products.publish")).Post("/moderation/products/{id}/publish", productsHandler.AdminPublishProduct)
		r.With(adminDangerousLimit, perm("products.hide")).Post("/moderation/products/{id}/hide", productsHandler.AdminHideProduct)
		r.With(adminDangerousLimit, perm("products.block")).Post("/moderation/products/{id}/block", productsHandler.AdminBlockProduct)

		// Inventory
		r.With(perm("inventory.read")).Get("/inventory", inventoryHandler.ListAdminInventory)
		r.With(perm("inventory.read")).Get("/inventory/{id}", inventoryHandler.GetAdminInventoryItem)
		r.With(perm("inventory.movements.read")).Get("/inventory/{id}/movements", inventoryHandler.ListMovements)
		r.With(adminDangerousLimit, perm("inventory.receipt")).Post("/inventory/receipts", inventoryHandler.ReceiveStock)
		r.With(adminDangerousLimit, perm("inventory.adjust")).Post("/inventory/adjustments", inventoryHandler.AdjustStock)
		r.With(adminDangerousLimit, perm("inventory.write_off")).Post("/inventory/write-offs", inventoryHandler.WriteOffStock)

		// Orders
		r.With(perm("orders.read")).Get("/orders", ordersHandler.ListAdminOrders)
		r.With(perm("orders.read")).Get("/orders/{id}", ordersHandler.GetAdminOrder)
		r.With(perm("orders.update_status")).Patch("/orders/{id}/status", ordersHandler.UpdateOrderStatus)
		r.With(perm("shipments.create")).Post("/orders/{id}/shipment", fulfillmentHandler.CreateShipment)

		// Payments
		r.With(perm("payments.read")).Get("/payments", paymentsHandler.ListAdminPayments)
		r.With(perm("payments.read")).Get("/payments/{id}", paymentsHandler.GetAdminPayment)

		// Shipments
		r.With(perm("shipments.read")).Get("/shipments", fulfillmentHandler.ListAdminShipments)
		r.With(perm("shipments.read")).Get("/shipments/{id}", fulfillmentHandler.GetAdminShipment)
		r.With(perm("shipments.update_status")).Patch("/shipments/{id}/status", fulfillmentHandler.UpdateShipmentStatus)

		// Returns
		r.With(perm("returns.read")).Get("/returns", returnsHandler.ListAdminReturns)
		r.With(perm("returns.read")).Get("/returns/{id}", returnsHandler.GetAdminReturn)
		r.With(perm("returns.update_status")).Patch("/returns/{id}/status", returnsHandler.UpdateAdminReturnStatus)
		r.With(adminDangerousLimit, perm("refunds.create")).Post("/returns/{id}/refund", returnsHandler.CreateAdminRefund)

		// Refunds
		r.With(perm("refunds.read")).Get("/refunds", returnsHandler.ListAdminRefunds)
		r.With(perm("refunds.read")).Get("/refunds/{id}", returnsHandler.GetAdminRefund)

		// Payouts — UpdateAdminPayoutStatus uses handler-level dynamic permission check
		r.With(perm("payouts.read")).Get("/payouts", payoutsHandler.ListAdminPayouts)
		r.With(perm("payouts.read")).Get("/payouts/{id}", payoutsHandler.GetAdminPayout)
		r.With(adminDangerousLimit).Patch("/payouts/{id}/status", payoutsHandler.UpdateAdminPayoutStatus)
		r.With(perm("payouts.read")).Post("/payouts/trigger-availability", payoutsHandler.TriggerAvailability)
	})

	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleAdmin))

		r.With(perm("reviews.read")).Get("/api/admin/reviews", reviewsHandler.GetAdminReviews)
		r.With(perm("reviews.read")).Get("/api/admin/reviews/{id}", reviewsHandler.GetAdminReview)
		// ModerateReview uses handler-level dynamic permission (approve/reject/hide/block)
		r.With(adminDangerousLimit).Post("/api/admin/reviews/{id}/{action}", reviewsHandler.ModerateReview)
	})

	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleCustomer))

		r.Post("/api/customer/orders/{orderId}/items/{orderItemId}/review", reviewsHandler.CreateCustomerReview)
		r.Get("/api/customer/reviews", reviewsHandler.GetCustomerReviews)
		r.Get("/api/customer/reviews/{id}", reviewsHandler.GetCustomerReview)
	})

	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))

		r.Get("/api/seller/reviews", reviewsHandler.GetSellerReviews)
		r.Get("/api/seller/reviews/{id}", reviewsHandler.GetSellerReview)
	})

	r.Get("/api/public/products/{idOrSlug}/reviews", reviewsHandler.GetPublicProductReviews)
	r.Get("/api/public/products/{idOrSlug}/rating-summary", reviewsHandler.GetPublicRatingSummary)

	_ = auditRepo

	return r
}
