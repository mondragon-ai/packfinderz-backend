package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
)

// NewHandler returns the HTTP handler that cmd/api wires into its server.
func NewHandler(cfg *config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler(cfg))
	return mux
}

func healthzHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-PackFinderz-Env", cfg.App.Env)
		response := map[string]string{"status": "ok"}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf(`{"level":"error","msg":"failed to write health response","err":"%v"}`, err)
			http.Error(w, `{"status":"error"}`, http.StatusInternalServerError)
		}
	}
}
