package routes

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/controllers"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
)

type sessionManager interface {
	session.AccessSessionChecker
	Rotate(context.Context, string, string) (string, string, error)
	Revoke(context.Context, string) error
}

func NewRouter(cfg *config.Config, logg *logger.Logger, dbP db.Pinger, redisP redis.Pinger, sessionManager sessionManager, authService auth.Service, registerService auth.RegisterService, switchService auth.SwitchStoreService) http.Handler {
	r := chi.NewRouter()
	r.Use(
		middleware.Recoverer(logg),
		middleware.RequestID(logg),
		middleware.Logging(logg),
	)

	r.Route("/health", func(r chi.Router) {
		r.Get("/live", controllers.HealthLive(cfg))
		r.Get("/ready", controllers.HealthReady(cfg, logg, dbP, redisP))
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Get("/ping", controllers.PublicPing())
		r.Post("/validate", controllers.PublicValidate(logg))
	})

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", controllers.AuthLogin(authService, logg))
		r.Post("/register", controllers.AuthRegister(registerService, authService, logg))
		r.Post("/logout", controllers.AuthLogout(sessionManager, cfg.JWT, logg))
		r.Post("/refresh", controllers.AuthRefresh(sessionManager, cfg.JWT, logg))
		r.Post("/switch-store", controllers.AuthSwitchStore(switchService, cfg.JWT, logg))
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.Idempotency())
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.PrivatePing())
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.RequireRole("admin", logg))
		r.Use(middleware.Idempotency())
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AdminPing())
	})

	r.Route("/api/agent", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT, sessionManager, logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.RequireRole("agent", logg))
		r.Use(middleware.Idempotency())
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AgentPing())
	})

	return r
}
