package main

import (
	"log"
	"net/http"
	"os"

	"github.com/angelmondragon/packfinderz-backend/api"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println(`{"level":"warn","msg":".env file not found, relying on environment"}`)
	}
	log.Printf("ENV PREFIX = %q", config.EnvPrefix)
	log.Printf("PACKFINDERZ_APP_ENV = %q", os.Getenv("PACKFINDERZ_APP_ENV"))
	log.Printf("APP_ENV = %q", os.Getenv("APP_ENV"))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"failed to load config","err":"%v"}`, err)
	}

	addr := ":" + cfg.App.Port
	log.Printf(`{"level":"info","msg":"starting api server","env":"%s","addr":"%s"}`, cfg.App.Env, addr)

	server := &http.Server{
		Addr:    addr,
		Handler: api.NewHandler(cfg),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf(`{"level":"error","msg":"api server stopped unexpectedly","err":"%v"}`, err)
	}
}
