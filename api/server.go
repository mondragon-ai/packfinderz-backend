package api

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

// NewHandler returns the HTTP handler that cmd/api wires into its server.
func NewHandler(cfg *config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler(cfg))
	mux.HandleFunc("/demo-error", demoErrorHandler())
	return mux
}

func healthzHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		responses.WriteSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func demoErrorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		demoErr := pkgerrors.New(pkgerrors.CodeValidation, "missing demo payload").WithDetails(map[string]string{"field": "demo"})
		responses.WriteError(w, demoErr)
	}
}
