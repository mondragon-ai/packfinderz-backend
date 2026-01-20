package controllers

import (
	"errors"
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
)

func HealthLive(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		responses.WriteSuccess(w, map[string]string{"status": "live"})
	}
}

func HealthReady(cfg *config.Config, logg *logger.Logger, dbP db.Pinger, redisP redis.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)

		failures := map[string]string{}

		// ---- Postgres
		if dbP == nil {
			if cfg.App.Env != "test" {
				failures["postgres"] = "not configured"
			}
		} else if err := dbP.Ping(ctx); err != nil {
			failures["postgres"] = err.Error()
		}

		// ---- Redis
		if redisP == nil {
			if cfg.App.Env != "test" {
				failures["redis"] = "not configured"
			}
		} else if err := redisP.Ping(ctx); err != nil {
			failures["redis"] = err.Error()
		}

		if len(failures) > 0 {
			err := pkgerrors.
				New(pkgerrors.CodeDependency, "dependencies unavailable").
				WithDetails(failures)

			if logg != nil {
				logg.Error(ctx, "readiness failed", errors.New("dependency check failed"))
			}

			responses.WriteError(ctx, logg, w, err)
			return
		}

		responses.WriteSuccess(w, map[string]string{"status": "ready"})
	}
}
