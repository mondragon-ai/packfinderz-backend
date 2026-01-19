package main

import (
	"log"
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api"
	"github.com/angelmondragon/packfinderz-backend/pkg/env"
)

func main() {
	addr := ":" + env.Get("HTTP_PORT", "8080")
	log.Printf(`{"level":"info","msg":"starting api server","addr":"%s"}`, addr)

	server := &http.Server{
		Addr:    addr,
		Handler: api.NewHandler(),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf(`{"level":"error","msg":"api server stopped unexpectedly","err":"%v"}`, err)
	}
}
