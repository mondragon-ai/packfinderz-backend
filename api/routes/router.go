// api/routes/router.go
package routes

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/controllers"
	ordercontrollers "github.com/angelmondragon/packfinderz-backend/api/controllers/orders"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	checkoutsvc "github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
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
	sessionManager sessionManager,
	authService auth.Service,
	registerService auth.RegisterService,
	switchService auth.SwitchStoreService,
	storeService stores.Service,
	mediaService media.Service,
	licenseService licenses.Service,
	productService products.Service,
	checkoutService checkoutsvc.Service,
	cartService cart.Service,
	ordersRepo orders.Repository,
	ordersSvc orders.Service,
) http.Handler {
	r := chi.NewRouter()
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
		r.Get("/ready", controllers.HealthReady(cfg, logg, dbP, redisClient, gcsClient))
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Get("/ping", controllers.PublicPing())
		r.Post("/validate", controllers.PublicValidate(logg))
	})

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.With(middleware.AuthRateLimit(loginPolicy, redisClient, logg)).Post("/login", controllers.AuthLogin(authService, logg))
		r.With(middleware.AuthRateLimit(registerPolicy, redisClient, logg)).Post("/register", controllers.AuthRegister(registerService, authService, logg))
		r.Post("/logout", controllers.AuthLogout(sessionManager, cfg.JWT, logg))
		r.Post("/refresh", controllers.AuthRefresh(sessionManager, cfg.JWT, logg))
		r.Post("/switch-store", controllers.AuthSwitchStore(switchService, cfg.JWT, logg))
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.Idempotency(redisClient, logg))
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.PrivatePing())

		r.Route("/v1/vendor", func(r chi.Router) {
			r.Post("/products", controllers.VendorCreateProduct(productService, logg))
			r.Patch("/products/{productId}", controllers.VendorUpdateProduct(productService, logg))
			r.Delete("/products/{productId}", controllers.VendorDeleteProduct(productService, logg))
			r.Post("/orders/{orderId}/decision", ordercontrollers.VendorOrderDecision(ordersSvc, logg))
			r.Post("/orders/{orderId}/line-items/decision", ordercontrollers.VendorLineItemDecision(ordersSvc, logg))
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

		r.Route("/v1/cart", func(r chi.Router) {
			r.Get("/", controllers.CartFetch(cartService, logg))
			r.Put("/", controllers.CartUpsert(cartService, logg))
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

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.RequireRole("admin", logg))
		r.Use(middleware.Idempotency(redisClient, logg))
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AdminPing())
		r.Route("/v1/licenses", func(r chi.Router) {
			r.Post("/{licenseId}/verify", controllers.AdminLicenseVerify(licenseService, logg))
		})
	})

	r.Route("/api/agent", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.RequireRole("agent", logg))
		r.Use(middleware.Idempotency(redisClient, logg))
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AgentPing())
	})

	return r
}
