package controllers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
)

func HealthLive(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		responses.WriteSuccess(w, map[string]string{"status": "live"})
	}
}

func HealthReady(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		responses.WriteSuccess(w, map[string]string{"status": "ready"})
	}
}
