package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/controllers"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func NewRouter(cfg *config.Config, logg *logger.Logger, pinger db.Pinger) http.Handler {
	r := chi.NewRouter()
	r.Use(
		middleware.Recoverer(logg),
		middleware.RequestID(logg),
		middleware.Logging(logg),
	)

	r.Route("/health", func(r chi.Router) {
		r.Get("/live", controllers.HealthLive(cfg))
		r.Get("/ready", controllers.HealthReady(cfg, logg, pinger))
	})

	r.Route("/api/public", func(r chi.Router) {
		r.Get("/ping", controllers.PublicPing())
		r.Post("/validate", controllers.PublicValidate(logg))
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.Idempotency())
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.PrivatePing())
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(middleware.Auth(logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.RequireRole("admin", logg))
		r.Use(middleware.Idempotency())
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AdminPing())
	})

	r.Route("/api/agent", func(r chi.Router) {
		r.Use(middleware.Auth(logg))
		r.Use(middleware.StoreContext(logg))
		r.Use(middleware.RequireRole("agent", logg))
		r.Use(middleware.Idempotency())
		r.Use(middleware.RateLimit())
		r.Get("/ping", controllers.AgentPing())
	})

	return r
}
