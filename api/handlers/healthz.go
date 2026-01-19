package handlers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func Healthz(cfg *config.Config, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := logg.WithFields(r.Context(), map[string]any{
			"env":  cfg.App.Env,
			"path": r.URL.Path,
		})
		logg.Info(ctx, "health.check")

		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		responses.WriteSuccess(w, map[string]string{"status": "ok"})
	}
}
