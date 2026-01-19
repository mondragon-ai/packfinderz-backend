package api

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/handlers"
	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// NewHandler returns the HTTP handler that cmd/api wires into its server.
func NewHandler(cfg *config.Config, logg *logger.Logger) http.Handler {
	mux := http.NewServeMux()

	// Routes
	mux.HandleFunc("/healthz", handlers.Healthz(cfg, logg))
	mux.HandleFunc("/demo-error", handlers.DemoError(logg))

	// Middleware chain (outermost first)
	h := middleware.RequestID(logg)(mux)
	h = middleware.Logging(logg)(h)

	return h
}
