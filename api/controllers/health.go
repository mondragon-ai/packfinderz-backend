package controllers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func HealthLive(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		responses.WriteSuccess(w, map[string]string{"status": "live"})
	}
}

func HealthReady(cfg *config.Config, logg *logger.Logger, pinger db.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		if err := pinger.Ping(ctx); err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "database connectivity failed"))
			return
		}
		responses.WriteSuccess(w, map[string]string{"status": "ready"})
	}
}
