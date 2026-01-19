package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/instance"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println(`{"level":"warn","msg":".env file not found, relying on environment"}`)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"failed to load config","err":"%v"}`, err)
	}

	log.Printf(`{"level":"info","msg":"starting worker","instance":"%s","env":"%s","serviceKind":"%s"}`, instance.GetID(), cfg.App.Env, cfg.Service.Kind)

	runWorker(ctx, cfg)
	log.Printf(`{"level":"info","msg":"worker shutting down gracefully"}`)
}

func runWorker(ctx context.Context, cfg *config.Config) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf(`{"level":"debug","msg":"worker heartbeat","env":"%s","pubsubMedia":"%s"}`, cfg.App.Env, cfg.PubSub.MediaTopic)
		}
	}
}
