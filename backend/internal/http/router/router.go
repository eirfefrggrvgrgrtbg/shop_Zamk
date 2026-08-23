package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/health"
	appMiddleware "github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/middleware"
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
	usersHandler *users.Handler,
	addressesHandler *addresses.Handler,
	notificationsHandler *notifications.Handler,
	auctionsAdminHandler *auctions.AdminHandler,
	auctionsPublicHandler *auctions.PublicHandler,
	auctionsCustomerHandler *auctions.CustomerHandler,
	dashboardHandler *dashboard.Handler,
	reportsHandler *reports.Handler,
	auditHandler *audit.Handler,
	deliveryHandler *delivery.Handler,
	suppliesHandler *supplies.Handler,
	sellerAnalyticsHandler *selleranalytics.Handler,
	testLabHandler *testlab.Handler,
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
		r.Post("/forgot-password", authHandler.ForgotPassword)

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
		r.With(perm("sellers.create_access")).Post("/invite", sellersHandler.InviteSeller)
		r.With(perm("sellers.read")).Get("/", sellersHandler.ListSellers)
		r.With(perm("sellers.read")).Get("/{id}", sellersHandler.GetAdminSellerDetail)
		r.With(perm("sellers.read")).Get("/{id}/overview", sellersHandler.GetAdminSellerOverview)
		r.With(perm("sellers.update_status")).Patch("/{id}/status", sellersHandler.UpdateSellerStatus)
		r.With(perm("sellers.create_access")).Post("/{id}/reset-owner-password", sellersHandler.ResetOwnerPassword)
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

		// Notes & Improvement Plans
		r.With(perm("sellers.read")).Get("/{id}/notes", sellersHandler.ListSellerNotes)
		r.With(perm("sellers.warn")).Post("/{id}/notes", sellersHandler.CreateSellerNote)
		r.With(perm("sellers.warn")).Patch("/{id}/notes/{noteId}/archive", sellersHandler.ArchiveSellerNote)

		r.With(perm("sellers.read")).Get("/{id}/improvement-plans", sellersHandler.ListImprovementPlans)
		r.With(perm("sellers.warn")).Post("/{id}/improvement-plans", sellersHandler.CreateImprovementPlan)
		r.With(perm("sellers.warn")).Patch("/{id}/improvement-plans/{planId}/status", sellersHandler.UpdateImprovementPlanStatus)

		// Commission
		r.With(perm("sellers.read")).Get("/{id}/commission", payoutsHandler.GetAdminSellerCommissionHistory)
		r.With(adminDangerousLimit, perm("commission.manage")).Post("/{id}/commission", payoutsHandler.SetAdminSellerCommission)
	})

	r.Route("/api/admin/seller-onboarding", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleAdmin))
		r.With(perm("sellers.read")).Get("/", sellersHandler.ListOnboardingApplications)
		r.With(perm("sellers.read")).Get("/{id}", sellersHandler.GetAdminOnboardingApplication)
		r.With(perm("sellers.update_status")).Post("/{id}/request-changes", sellersHandler.RequestChangesOnboarding)
		r.With(perm("sellers.update_status")).Post("/{id}/reject", sellersHandler.RejectOnboarding)
		r.With(perm("sellers.update_status")).Post("/{id}/approve", sellersHandler.ApproveOnboarding)
	})

	r.Route("/api/seller/me", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess())
		r.Get("/", sellersHandler.GetSellerMe)
		r.Patch("/", sellersHandler.UpdateSellerProfile)
		r.With(uploadLimit).Post("/logo/upload", storageHandler.UploadSellerProfileImage)
	})

	r.Route("/api/seller/onboarding", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess())
		r.Get("/", sellersHandler.GetOnboardingApplication)
		r.Put("/steps/{step}", sellersHandler.UpdateOnboardingStep)
		r.Post("/submit", sellersHandler.SubmitOnboarding)
		r.Post("/complete", sellersHandler.CompleteOnboarding)
	})

	r.Route("/api/seller/warnings", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess())
		r.Get("/", sellersHandler.GetMyWarnings)
	})

	r.Route("/api/seller/violations", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess())
		r.Get("/", sellersHandler.GetMyViolations)
	})

	r.Route("/api/seller/supplies", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess())
		r.Get("/", suppliesHandler.ListSupplies)
		r.Post("/", suppliesHandler.CreateSupply)
		r.Get("/{id}", suppliesHandler.GetSupply)
		r.Post("/{id}/ship", suppliesHandler.MarkShipped)
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Get("/categories", catalogHandler.ListCategories)
		r.Get("/brands", catalogHandler.ListBrands)
		r.Get("/products", productsHandler.ListPublicProducts)
		r.Get("/product-previews/{token}", productsHandler.GetProductPreviewByToken)
		r.Get("/direct-sale", productsHandler.GetDirectSaleProducts)
		r.Get("/products/{idOrSlug}", productsHandler.GetPublicProduct)
		r.Get("/sellers/{idOrSlug}", productsHandler.GetPublicSellerStore)
		r.Get("/delivery-methods", deliveryHandler.GetPublicMethods)

		// Auctions
		r.Get("/auctions/active", auctionsPublicHandler.GetActiveAuctions)
		r.Get("/auctions/homepage", auctionsPublicHandler.GetHomepageAuctions)
		r.Get("/auctions/nav-highlight", auctionsPublicHandler.GetNavHighlightAuctions)
		r.Get("/auctions/{id}/lots", auctionsPublicHandler.GetAuctionLots)
		r.Get("/auctions/{id}/stream", auctionsPublicHandler.StreamAuction)
		r.Get("/auction-lots/{id}", auctionsPublicHandler.GetAuctionLot)
	})

	r.Route("/api/customer", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireCustomerAccess())

		r.Get("/cart", cartHandler.GetCart)
		r.Post("/cart/items", cartHandler.AddItem)
		r.Patch("/cart/items/{id}", cartHandler.UpdateItem)
		r.Delete("/cart/items/{id}", cartHandler.RemoveItem)
		r.Delete("/cart", cartHandler.ClearCart)

		r.Post("/orders", ordersHandler.CreateOrder)
		r.Get("/orders", ordersHandler.ListCustomerOrders)
		r.Get("/orders/{id}", ordersHandler.GetCustomerOrder)
		r.Get("/orders/{orderId}/fulfillments", fulfillmentHandler.GetCustomerOrderFulfillments)
		r.Post("/orders/{id}/cancel", ordersHandler.CancelCustomerOrder)
		r.Post("/orders/{id}/payment", paymentsHandler.CreatePayment)
		r.Get("/orders/{id}/shipment", fulfillmentHandler.GetCustomerShipment)
		r.Post("/orders/{id}/returns", returnsHandler.CreateCustomerReturn)
		r.Get("/returns", returnsHandler.ListCustomerReturns)
		r.Get("/returns/{id}", returnsHandler.GetCustomerReturn)

		r.Get("/favorites", favoritesHandler.ListFavorites)
		r.Post("/favorites/{productId}", favoritesHandler.AddFavorite)
		r.Delete("/favorites/{productId}", favoritesHandler.RemoveFavorite)

		r.Get("/profile", usersHandler.GetProfile)
		r.Patch("/profile", usersHandler.UpdateProfile)

		r.Get("/addresses", addressesHandler.ListAddresses)
		r.Post("/addresses", addressesHandler.CreateAddress)
		r.Patch("/addresses/{id}", addressesHandler.UpdateAddress)
		r.Delete("/addresses/{id}", addressesHandler.DeleteAddress)
		r.Post("/addresses/{id}/default", addressesHandler.SetDefault)

		r.Get("/notifications", notificationsHandler.ListCustomerNotifications)
		r.Get("/notifications/unread-count", notificationsHandler.UnreadCountCustomer)
		r.Post("/notifications/{id}/read", notificationsHandler.ReadCustomerNotification)
		r.Post("/notifications/read-all", notificationsHandler.ReadAllCustomer)

		// Auctions (Customer)
		r.Post("/auction-lots/{id}/bid", auctionsCustomerHandler.PlaceBid)
		r.Get("/auction-wins", auctionsCustomerHandler.GetAuctionWins)
		r.Post("/auction-lots/{id}/create-order", auctionsCustomerHandler.CreateOrderForLot)
	})

	r.With(webhookLimit).Post("/api/payments/tbank/webhook", paymentsHandler.HandleTBankWebhook)

	r.Route("/api/dev/payments/mock", func(r chi.Router) {
		r.Get("/{id}", paymentsHandler.GetDevMockPayment)
		r.Post("/{id}/{action}", paymentsHandler.ProcessDevMockAction)
	})


	r.Route("/api/seller/reference", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)

		r.Get("/categories", catalogHandler.ListCategories)
		r.Get("/categories/{id}/schema", productsHandler.GetCategorySchema)
		r.Get("/colors", productsHandler.ListColors)
		r.Get("/materials", productsHandler.ListMaterials)
		r.Get("/size-systems", productsHandler.ListSizeSystems)
		r.Get("/size-systems/{id}/values", productsHandler.ListSizeValues)
		r.Get("/dictionaries/{id}/values", productsHandler.ListDictionaryValues)
	})

	r.Route("/api/seller/products", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)

		r.Get("/", productsHandler.ListSellerProducts)
		r.Post("/", productsHandler.CreateProduct)
		r.Post("/generate-skus", productsHandler.GenerateSKUs)
		r.Get("/{id}", productsHandler.GetSellerProduct)
		r.Patch("/{id}", productsHandler.UpdateProduct)
		r.Delete("/{id}", productsHandler.DeleteDraftProduct)
		r.Post("/{id}/submit-moderation", productsHandler.SubmitForModeration)
		r.Get("/{id}/moderation-history", productsHandler.GetModerationHistory)
		r.With(uploadLimit).Post("/{id}/images/upload", storageHandler.UploadSellerProductImage)
		r.Delete("/{id}/images/{imageId}", storageHandler.DeleteSellerProductImage)
		r.Put("/{id}/images/reorder", storageHandler.ReorderSellerProductImages)
		r.Post("/{id}/images/{imageId}/crop", storageHandler.CropSellerProductImage)
	})

	r.Route("/api/seller/inventory", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)

		r.Get("/", inventoryHandler.ListSellerInventory)
		r.Get("/{id}", inventoryHandler.GetSellerInventoryItem)
		r.Get("/{id}/movements", inventoryHandler.ListSellerMovements)
	})

	r.Route("/api/seller/orders", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)

		r.Get("/", ordersHandler.ListSellerOrders)
		r.Get("/summary", ordersHandler.GetSellerOrderSummary)
		r.Get("/{id}", ordersHandler.GetSellerOrder)
		r.Get("/{id}/shipment", fulfillmentHandler.GetSellerShipment)
	})

	// /api/seller/fulfillments removed for FBO architecture

	r.Route("/api/seller/returns", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)

		r.Get("/", returnsHandler.ListSellerReturns)
		r.Get("/{id}", returnsHandler.GetSellerReturn)
	})

	r.Route("/api/seller/balance", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)

		r.Get("/", payoutsHandler.GetSellerBalance)
	})

	r.Route("/api/seller/payouts", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)

		r.Get("/", payoutsHandler.ListSellerPayouts)
		r.Get("/ledger", payoutsHandler.ListSellerLedger)
	})

	r.Route("/api/seller/notifications", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess())

		r.Get("/", notificationsHandler.ListSellerNotifications)
		r.Get("/unread-count", notificationsHandler.UnreadCountSeller)
		r.Post("/{id}/read", notificationsHandler.ReadSellerNotification)
		r.Post("/read-all", notificationsHandler.ReadAllSeller)
	})

	r.Route("/api/seller/analytics", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess(), sellersHandler.RequireActiveSeller)
		
		sellerAnalyticsHandler.RegisterRoutes(r)
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireRole(users.RoleAdmin))

		// /api/admin/me — no fine-grained permission required, role=admin is enough
		r.Get("/me", staffHandler.GetAdminMe)
		r.With(perm("dashboard.read")).Get("/dashboard/summary", dashboardHandler.GetDashboardSummary)

		if testLabHandler != nil {
			r.Route("/testing/analytics/scenarios", func(r chi.Router) {
				r.Use(perm("testing.manage"))
				testLabHandler.RegisterRoutes(r)
			})
		}

		r.Get("/notifications", notificationsHandler.ListAdminNotifications)
		r.Get("/notifications/unread-count", notificationsHandler.UnreadCountAdmin)
		r.Post("/notifications/{id}/read", notificationsHandler.ReadAdminNotification)
		r.Post("/notifications/read-all", notificationsHandler.ReadAllAdmin)

		// Users and Staff RBAC endpoints
		r.With(perm("users.read")).Get("/users", usersHandler.ListAdminUsers)
		r.With(perm("roles.read")).Get("/staff/roles", staffHandler.GetStaffRoles)
		r.With(perm("audit.read")).Get("/audit-logs", auditHandler.HandleListLogs)
		r.With(perm("reports.read")).Get("/reports/summary", reportsHandler.HandleGetSummary)
		r.With(perm("staff.read")).Get("/staff/members", staffHandler.ListStaffMembers)
		r.With(perm("staff.create")).Post("/staff/members", staffHandler.CreateStaffMember)
		r.With(perm("staff.update")).Patch("/staff/members/{userId}/role", staffHandler.UpdateStaffRole)
		r.With(perm("staff.block")).Patch("/staff/members/{userId}/status", staffHandler.UpdateStaffStatus)
		r.With(perm("staff.update")).Post("/staff/members/{userId}/reset-password", staffHandler.ResetStaffPassword)

		r.Route("/receiving", func(r chi.Router) {
			r.Use(perm("inventory.receipt"))
			r.Post("/sessions", suppliesHandler.StartSession)
			r.Post("/sessions/{sessionId}/scan", suppliesHandler.RecordScan)
			r.Post("/sessions/{sessionId}/finalize", suppliesHandler.FinalizeSession)
		})

		// Catalog
		r.With(perm("categories.read")).Get("/categories", catalogHandler.ListCategories)
		r.With(perm("categories.create")).Post("/categories", catalogHandler.CreateCategory)
		r.With(perm("brands.read")).Get("/brands", catalogHandler.ListBrands)
		r.With(perm("brands.create")).Post("/brands", catalogHandler.CreateBrand)
		r.With(uploadLimit, perm("brands.update")).Post("/brands/{id}/logo/upload", storageHandler.UploadAdminBrandLogo)

		// Products
		r.With(perm("products.read")).Get("/products", productsHandler.ListAdminProducts)
		r.With(perm("products.read")).Get("/products/{id}", productsHandler.GetAdminProduct)
		r.With(perm("products.moderate")).Patch("/products/{id}", productsHandler.AdminUpdateProduct)
		r.With(perm("products.read")).Post("/products/{id}/preview-link", productsHandler.AdminCreateProductPreviewLink)
		r.With(perm("products.read")).Get("/products/{id}/moderation-logs", productsHandler.GetAdminModerationHistory)
		r.With(uploadLimit, perm("products.moderate")).Post("/products/{id}/images/upload", storageHandler.UploadAdminProductImage)
		r.With(perm("products.moderate")).Get("/moderation/products", productsHandler.ListModerationProducts)
		r.With(perm("products.moderate")).Post("/moderation/products/{id}/start-review", productsHandler.AdminStartProductReview)
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
		r.With(adminDangerousLimit, perm("inventory.adjust")).Post("/inventory/{id}/adjust", inventoryHandler.AdjustStockUnified)
		r.With(perm("inventory.read")).Get("/inventory/{id}/reservations", inventoryHandler.GetAdminInventoryReservations)

		// Orders
		r.With(perm("orders.read")).Get("/orders", ordersHandler.ListAdminOrders)
		r.With(perm("orders.read")).Get("/orders/{id}", ordersHandler.GetAdminOrder)
		r.With(perm("orders.read")).Get("/orders/{orderId}/fulfillments", fulfillmentHandler.GetAdminOrderFulfillments)
		r.With(perm("orders.update_status")).Patch("/orders/{id}/status", ordersHandler.UpdateOrderStatus)
		r.With(perm("orders.update_status")).Patch("/orders/{id}/fulfillment-status", fulfillmentHandler.UpdateAdminOrderFulfillmentStatus)
		r.With(perm("shipments.create")).Post("/orders/{id}/shipment", fulfillmentHandler.CreateShipment)

		// Payments
		r.With(perm("payments.read")).Get("/payments", paymentsHandler.ListAdminPayments)
		r.With(perm("payments.read")).Get("/payments/{id}", paymentsHandler.GetAdminPayment)

		// Shipments
		r.With(perm("shipments.read")).Get("/shipments", fulfillmentHandler.ListAdminShipments)
		r.With(perm("shipments.read")).Get("/shipments/{id}", fulfillmentHandler.GetAdminShipment)
		r.With(perm("shipments.update_status")).Patch("/shipments/{id}/status", fulfillmentHandler.UpdateShipmentStatus)

		// Fulfillments & Receiving
		r.With(perm("orders.read")).Get("/order-fulfillments", fulfillmentHandler.ListAdminFulfillments)
		r.With(perm("orders.read")).Get("/order-fulfillments/{id}", fulfillmentHandler.GetAdminFulfillment)
		r.With(perm("shipments.create")).Post("/fulfillments/{id}/shipment", fulfillmentHandler.CreateShipmentForFulfillment)
		r.With(perm("orders.read")).Post("/fulfillments/resolve-receiving-code", fulfillmentHandler.ResolveReceivingCode)
		r.With(perm("orders.read")).Post("/fulfillments/{id}/receiving/start", fulfillmentHandler.StartReceiving)
		r.With(perm("orders.read")).Post("/fulfillments/{id}/receiving/scan-item", fulfillmentHandler.ScanReceivingItem)
		r.With(perm("shipments.create")).Post("/fulfillments/{id}/receiving/confirm", fulfillmentHandler.ConfirmReceiving)
		r.With(perm("orders.read")).Post("/fulfillments/{id}/receiving/discrepancy", fulfillmentHandler.RecordDiscrepancy)

		// Returns
		r.With(perm("returns.read")).Get("/returns", returnsHandler.ListAdminReturns)
		r.With(perm("returns.read")).Get("/returns/{id}", returnsHandler.GetAdminReturn)
		r.With(perm("returns.update_status")).Patch("/returns/{id}/status", returnsHandler.UpdateAdminReturnStatus)
		r.With(adminDangerousLimit, perm("refunds.create")).Post("/returns/{id}/refund", returnsHandler.CreateAdminRefund)

		// Refunds
		r.With(perm("refunds.read")).Get("/refunds", returnsHandler.ListAdminRefunds)
		r.With(perm("refunds.read")).Get("/refunds/{id}", returnsHandler.GetAdminRefund)

		// Payouts
		r.Route("/payouts", func(r chi.Router) {
			r.With(perm("payouts.read")).Get("/summary", payoutsHandler.GetAdminPayoutSummary)
			r.With(adminDangerousLimit, perm("payouts.create")).Post("/batch", payoutsHandler.CreatePayoutBatch)
			r.With(adminDangerousLimit, perm("payouts.update")).Post("/{id}/process", payoutsHandler.ProcessPayoutBatch)
			r.With(adminDangerousLimit, perm("payouts.update")).Post("/{id}/hold", payoutsHandler.HoldPayoutBatch)
		})



		// Auctions
		r.With(perm("auctions.read")).Get("/auctions", auctionsAdminHandler.GetAuctions)
		r.With(perm("auctions.update")).Post("/auctions/expire-unpaid", auctionsAdminHandler.ExpireUnpaidLots)
		r.With(perm("auctions.read")).Get("/auctions/{id}", auctionsAdminHandler.GetAuction)
		r.With(perm("auctions.create")).Post("/auctions", auctionsAdminHandler.CreateAuction)
		r.With(perm("auctions.update")).Patch("/auctions/{id}", auctionsAdminHandler.UpdateAuction)
		r.With(perm("auctions.publish")).Post("/auctions/{id}/publish", auctionsAdminHandler.PublishAuction)
		r.With(perm("auctions.pause")).Post("/auctions/{id}/pause", auctionsAdminHandler.PauseAuction)
		r.With(perm("auctions.resume")).Post("/auctions/{id}/resume", auctionsAdminHandler.ResumeAuction)
		r.With(perm("auctions.cancel")).Post("/auctions/{id}/cancel", auctionsAdminHandler.CancelAuction)
		r.With(perm("auctions.finalize")).Post("/auctions/{id}/finalize", auctionsAdminHandler.FinalizeAuction)

		r.With(perm("auctions.update")).Post("/auctions/{id}/lots", auctionsAdminHandler.CreateLot)
		r.With(perm("auctions.update")).Patch("/auction-lots/{id}", auctionsAdminHandler.UpdateLot)
		r.With(perm("auctions.read")).Get("/auction-lots/{id}/bids", auctionsAdminHandler.GetLotBids)
		r.With(perm("auctions.update")).Post("/auction-lots/{id}/mark-unpaid-review", auctionsAdminHandler.MarkLotUnpaid)
		r.With(perm("auctions.move_to_direct_sale")).Post("/auction-lots/{id}/move-to-direct-sale", auctionsAdminHandler.MoveToDirectSale)

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
		r.Use(appMiddleware.RequireCustomerAccess())

		r.Post("/api/customer/orders/{orderId}/items/{orderItemId}/review", reviewsHandler.CreateCustomerReview)
		r.Get("/api/customer/reviews", reviewsHandler.GetCustomerReviews)
		r.Get("/api/customer/reviews/{id}", reviewsHandler.GetCustomerReview)
	})

	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware(tokenService))
		r.Use(appMiddleware.RequireSellerAccess())

		r.Get("/api/seller/reviews", reviewsHandler.GetSellerReviews)
		r.Get("/api/seller/reviews/{id}", reviewsHandler.GetSellerReview)
	})

	r.Get("/api/public/products/{idOrSlug}/reviews", reviewsHandler.GetPublicProductReviews)
	r.Get("/api/public/products/{idOrSlug}/rating-summary", reviewsHandler.GetPublicRatingSummary)

	_ = auditRepo

	return r
}
