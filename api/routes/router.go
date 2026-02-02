package routes

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/controllers"
	analysiscontrollers "github.com/angelmondragon/packfinderz-backend/api/controllers/analytics"
	billingcontrollers "github.com/angelmondragon/packfinderz-backend/api/controllers/billing"
	cartcontrollers "github.com/angelmondragon/packfinderz-backend/api/controllers/cart"
	ordercontrollers "github.com/angelmondragon/packfinderz-backend/api/controllers/orders"
	subscriptionControllers "github.com/angelmondragon/packfinderz-backend/api/controllers/subscriptions"
	webhookcontrollers "github.com/angelmondragon/packfinderz-backend/api/controllers/webhooks"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	checkoutsvc "github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	subscriptionsvc "github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	stripewebhook "github.com/angelmondragon/packfinderz-backend/internal/webhooks/stripe"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
	"github.com/angelmondragon/packfinderz-backend/pkg/stripe"
)

type sessionManager interface {
	session.AccessSessionChecker
	Rotate(context.Context, string, string) (string, string, error)
	Revoke(context.Context, string) error
}

func NewRouter(
	cfg *config.Config,
	logg *logger.Logger,
	dbP db.Pinger,
	redisClient *redis.Client,
	gcsClient gcs.Pinger,
	bigqueryClient bigquery.Pinger,
	sessionManager sessionManager,
	analyticsService analytics.Service,
	authService auth.Service,
	registerService auth.RegisterService,
	adminRegisterService auth.AdminRegisterService,
	switchService auth.SwitchStoreService,
	storeService stores.Service,
	mediaService media.Service,
	licenseService licenses.Service,
	productService products.Service,
	checkoutService checkoutsvc.Service,
	cartService cart.Service,
	notificationsService notifications.Service,
	ordersRepo orders.Repository,
	ordersSvc orders.Service,
	subscriptionsService subscriptionsvc.Service,
	billingService billingcontrollers.ChargesService,
	stripeClient *stripe.Client,
	stripeWebhookService *stripewebhook.Service,
	stripeWebhookGuard *stripewebhook.IdempotencyGuard,
) http.Handler {
	r := chi.NewRouter()
	// if stripeClient != nil && logg != nil {
	// 	ctx := logg.WithField(context.Background(), "stripe_env", stripeClient.Environment())
	// 	logg.Info(ctx, "stripe client wired to API routes")
	// }
	r.Use(
		middleware.Recoverer(logg),
		middleware.RequestID(logg),
		middleware.Logging(logg),
	)

	loginPolicy := middleware.NewAuthRateLimitPolicy(
		"login",
		cfg.AuthRateLimit.LoginWindow,
		cfg.AuthRateLimit.LoginIPLimit,
		cfg.AuthRateLimit.LoginEmailLimit,
	)
	registerPolicy := middleware.NewAuthRateLimitPolicy(
		"register",
		cfg.AuthRateLimit.RegisterWindow,
		cfg.AuthRateLimit.RegisterIPLimit,
		cfg.AuthRateLimit.RegisterEmailLimit,
	)

	r.Route("/health", func(r chi.Router) {
		r.Get("/live", controllers.HealthLive(cfg))
		r.Get("/ready", controllers.HealthReady(cfg, logg, dbP, redisClient, gcsClient, bigqueryClient))
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Get("/ping", controllers.PublicPing())
		r.Post("/validate", controllers.PublicValidate(logg))
	})

	r.Route("/api/v1/webhooks", func(r chi.Router) {
		r.Post("/stripe", webhookcontrollers.StripeWebhook(stripeWebhookService, stripeClient, stripeWebhookGuard, logg))
	})

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.With(middleware.AuthRateLimit(loginPolicy, redisClient, logg)).Post("/login", controllers.AuthLogin(authService, logg))
		r.With(middleware.AuthRateLimit(registerPolicy, redisClient, logg)).Post("/register", controllers.AuthRegister(registerService, authService, logg))
		r.Post("/logout", controllers.AuthLogout(sessionManager, cfg.JWT, logg))
		r.Post("/refresh", controllers.AuthRefresh(sessionManager, cfg.JWT, logg))
		r.Post("/switch-store", controllers.AuthSwitchStore(switchService, cfg.JWT, logg))
	})

	r.Route("/api/admin/v1/auth", func(r chi.Router) {
		if !cfg.App.IsProd() {
			r.Post("/register", controllers.AdminAuthRegister(adminRegisterService, authService, cfg, logg))
		}
		r.With(middleware.AuthRateLimit(loginPolicy, redisClient, logg)).Post("/login", controllers.AdminAuthLogin(authService, logg))
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.Idempotency(redisClient, logg))
		r.Use(middleware.RateLimit())

		r.Group(func(r chi.Router) {
			r.Use(middleware.StoreContext(logg))
			r.Get("/ping", controllers.PrivatePing())

			r.Route("/v1/vendor", func(r chi.Router) {
				r.Post("/products", controllers.VendorCreateProduct(productService, logg))
				r.Patch("/products/{productId}", controllers.VendorUpdateProduct(productService, logg))
				r.Delete("/products/{productId}", controllers.VendorDeleteProduct(productService, logg))
				r.Get("/billing/charges", billingcontrollers.VendorBillingCharges(billingService, logg))
				r.Post("/orders/{orderId}/decision", ordercontrollers.VendorOrderDecision(ordersSvc, logg))
				r.Post("/orders/{orderId}/line-items/decision", ordercontrollers.VendorLineItemDecision(ordersSvc, logg))
				r.Route("/subscriptions", func(r chi.Router) {
					r.Post("/", subscriptionControllers.VendorSubscriptionCreate(subscriptionsService, logg))
					r.Post("/cancel", subscriptionControllers.VendorSubscriptionCancel(subscriptionsService, logg))
					r.Get("/", subscriptionControllers.VendorSubscriptionFetch(subscriptionsService, logg))
				})
			})

			r.Route("/v1/analytics", func(r chi.Router) {
				r.Get("/marketplace", analysiscontrollers.MarketplaceAnalytics(analyticsService, logg))
			})

			r.Route("/v1/stores", func(r chi.Router) {
				r.Get("/me", controllers.StoreProfile(storeService, logg))
				r.Put("/me", controllers.StoreUpdate(storeService, logg))
				r.Get("/me/users", controllers.StoreUsers(storeService, logg))
				r.Post("/me/users/invite", controllers.StoreInvite(storeService, logg))
				r.Delete("/me/users/{userId}", controllers.StoreRemoveUser(storeService, logg))
			})
			r.Route("/v1/media", func(r chi.Router) {
				r.Get("/", controllers.MediaList(mediaService, logg))
				r.Post("/presign", controllers.MediaPresign(mediaService, logg))
				r.Delete("/{mediaId}", controllers.MediaDelete(mediaService, logg))
			})
			r.Route("/v1/licenses", func(r chi.Router) {
				r.Get("/", controllers.LicenseList(licenseService, logg))
				r.Post("/", controllers.LicenseCreate(licenseService, logg))
				r.Delete("/{licenseId}", controllers.LicenseDelete(licenseService, logg))
			})

			r.Route("/v1/notifications", func(r chi.Router) {
				r.Get("/", controllers.ListNotifications(notificationsService, logg))
				r.Post("/{notificationId}/read", controllers.MarkNotificationRead(notificationsService, logg))
				r.Post("/read-all", controllers.MarkAllNotificationsRead(notificationsService, logg))
			})

			r.Route("/v1/cart", func(r chi.Router) {
				r.Get("/", cartcontrollers.CartFetch(cartService, logg))
				r.Post("/", cartcontrollers.CartQuote(cartService, logg))
			})
			r.Route("/v1/orders", func(r chi.Router) {
				r.Get("/", ordercontrollers.List(ordersRepo, logg))
				r.Get("/{orderId}", ordercontrollers.Detail(ordersRepo, logg))
				r.Post("/{orderId}/cancel", ordercontrollers.CancelOrder(ordersSvc, logg))
				r.Post("/{orderId}/nudge", ordercontrollers.NudgeVendor(ordersSvc, logg))
				r.Post("/{orderId}/retry", ordercontrollers.RetryOrder(ordersSvc, logg))
			})
			r.Post("/v1/checkout", controllers.Checkout(checkoutService, storeService, logg))
		})

		r.Route("/v1/agent", func(r chi.Router) {
			r.Use(middleware.RequireRole("agent", logg))
			r.Get("/ping", controllers.AgentPing())
			r.Route("/orders", func(r chi.Router) {
				r.Get("/", controllers.AgentAssignedOrders(ordersRepo, logg))
				r.Get("/queue", controllers.AgentOrderQueue(ordersRepo, logg))
				r.Get("/{orderId}", controllers.AgentAssignedOrderDetail(ordersRepo, logg))
				r.Post("/{orderId}/pickup", controllers.AgentPickupOrder(ordersSvc, logg))
				r.Post("/{orderId}/deliver", controllers.AgentDeliverOrder(ordersSvc, logg))
			})
		})
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.RequireRole("admin", logg))
		r.Use(middleware.Idempotency(redisClient, logg))
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AdminPing())
		r.Route("/v1/licenses", func(r chi.Router) {
			r.Post("/{licenseId}/verify", controllers.AdminLicenseVerify(licenseService, logg))
		})
		r.Route("/v1/orders", func(r chi.Router) {
			r.Route("/payouts", func(r chi.Router) {
				r.Get("/", controllers.AdminPayoutOrders(ordersRepo, logg))
				r.Get("/{orderId}", controllers.AdminPayoutOrderDetail(ordersRepo, logg))
			})
			r.Post("/{orderId}/confirm-payout", controllers.AdminConfirmPayout(ordersSvc, logg))
		})
	})

	r.Route("/api/agent", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.RequireRole("agent", logg))
		r.Use(middleware.Idempotency(redisClient, logg))
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AgentPing())
	})

	return r
}
