package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := ":" + getEnv("HTTP_PORT", "8080")
	log.Printf(`{"level":"info","msg":"starting api server","addr":"%s"}`, addr)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf(`{"level":"error","msg":"api server stopped unexpectedly","err":"%v"}`, err)
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "ok"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf(`{"level":"error","msg":"failed to write health response","err":"%v"}`, err)
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
