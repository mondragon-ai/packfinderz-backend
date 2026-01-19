package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// NewHandler returns the HTTP handler that cmd/api wires into its server.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	return mux
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "ok"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf(`{"level":"error","msg":"failed to write health response","err":"%v"}`, err)
		http.Error(w, `{"status":"error"}`, http.StatusInternalServerError)
	}
}
