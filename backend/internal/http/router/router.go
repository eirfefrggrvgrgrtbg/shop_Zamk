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
		r.Post("/", sellersHandler.CreateSellerByAdmin)
		r.Get("/", sellersHandler.ListSellers)
		r.Patch("/{id}/status", sellersHandler.UpdateSellerStatus)
	})

	r.Route("/api/seller/me", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleSeller))
		r.Get("/", sellersHandler.GetSellerMe)
		r.Patch("/", sellersHandler.UpdateSellerProfile)
		r.With(uploadLimit).Post("/logo/upload", storageHandler.UploadSellerProfileImage)
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Get("/categories", catalogHandler.ListCategories)
		r.Get("/brands", catalogHandler.ListBrands)
		r.Get("/products", productsHandler.ListPublicProducts)
		r.Get("/products/{idOrSlug}", productsHandler.GetPublicProduct)
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

		// Staff RBAC endpoints (Phase B/C) — auth+role guard only, no RequirePermission yet
		r.Get("/me", staffHandler.GetAdminMe)
		r.Get("/staff/roles", staffHandler.GetStaffRoles)
		r.Get("/audit-logs", staffHandler.GetAuditLogs)

		// Staff member management (Phase C)
		r.Get("/staff/members", staffHandler.ListStaffMembers)
		r.Post("/staff/members", staffHandler.CreateStaffMember)
		r.Patch("/staff/members/{userId}/role", staffHandler.UpdateStaffRole)
		r.Patch("/staff/members/{userId}/status", staffHandler.UpdateStaffStatus)
		r.Post("/staff/members/{userId}/reset-password", staffHandler.ResetStaffPassword)

		r.Get("/categories", catalogHandler.ListCategories)
		r.Post("/categories", catalogHandler.CreateCategory)
		r.Get("/brands", catalogHandler.ListBrands)
		r.Post("/brands", catalogHandler.CreateBrand)
		r.With(uploadLimit).Post("/brands/{id}/logo/upload", storageHandler.UploadAdminBrandLogo)

		r.Get("/products", productsHandler.ListAdminProducts)
		r.With(uploadLimit).Post("/products/{id}/images/upload", storageHandler.UploadAdminProductImage)
		r.Get("/moderation/products", productsHandler.ListModerationProducts)
		r.With(adminDangerousLimit).Post("/moderation/products/{id}/approve", productsHandler.AdminApproveProduct)
		r.With(adminDangerousLimit).Post("/moderation/products/{id}/reject", productsHandler.AdminRejectProduct)
		r.With(adminDangerousLimit).Post("/moderation/products/{id}/publish", productsHandler.AdminPublishProduct)
		r.With(adminDangerousLimit).Post("/moderation/products/{id}/hide", productsHandler.AdminHideProduct)
		r.With(adminDangerousLimit).Post("/moderation/products/{id}/block", productsHandler.AdminBlockProduct)

		r.Get("/inventory", inventoryHandler.ListAdminInventory)
		r.Get("/inventory/{id}", inventoryHandler.GetAdminInventoryItem)
		r.Get("/inventory/{id}/movements", inventoryHandler.ListMovements)
		r.With(adminDangerousLimit).Post("/inventory/receipts", inventoryHandler.ReceiveStock)
		r.With(adminDangerousLimit).Post("/inventory/adjustments", inventoryHandler.AdjustStock)
		r.With(adminDangerousLimit).Post("/inventory/write-offs", inventoryHandler.WriteOffStock)

		r.Get("/orders", ordersHandler.ListAdminOrders)
		r.Get("/orders/{id}", ordersHandler.GetAdminOrder)
		r.Patch("/orders/{id}/status", ordersHandler.UpdateOrderStatus)
		r.Post("/orders/{id}/shipment", fulfillmentHandler.CreateShipment)

		r.Get("/payments", paymentsHandler.ListAdminPayments)
		r.Get("/payments/{id}", paymentsHandler.GetAdminPayment)

		r.Get("/shipments", fulfillmentHandler.ListAdminShipments)
		r.Get("/shipments/{id}", fulfillmentHandler.GetAdminShipment)
		r.Patch("/shipments/{id}/status", fulfillmentHandler.UpdateShipmentStatus)

		r.Get("/returns", returnsHandler.ListAdminReturns)
		r.Get("/returns/{id}", returnsHandler.GetAdminReturn)
		r.Patch("/returns/{id}/status", returnsHandler.UpdateAdminReturnStatus)
		r.With(adminDangerousLimit).Post("/returns/{id}/refund", returnsHandler.CreateAdminRefund)

		r.Get("/refunds", returnsHandler.ListAdminRefunds)
		r.Get("/refunds/{id}", returnsHandler.GetAdminRefund)

		r.Get("/payouts", payoutsHandler.ListAdminPayouts)
		r.Get("/payouts/{id}", payoutsHandler.GetAdminPayout)
		r.With(adminDangerousLimit).Patch("/payouts/{id}/status", payoutsHandler.UpdateAdminPayoutStatus)
		r.Post("/payouts/trigger-availability", payoutsHandler.TriggerAvailability)
	})

	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleAdmin))

		r.Get("/api/admin/reviews", reviewsHandler.GetAdminReviews)
		r.Get("/api/admin/reviews/{id}", reviewsHandler.GetAdminReview)
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
