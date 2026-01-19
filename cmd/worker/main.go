package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/joho/godotenv"
)

func main() {
	logg := logger.New(logger.Options{ServiceName: "worker"})

	if err := godotenv.Load(); err != nil {
		logg.Warn(context.Background(), ".env file not found, relying on environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logg.Error(context.Background(), "failed to load config", err)
		os.Exit(1)
	}

	logg = logger.New(logger.Options{
		ServiceName: "worker",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctx = logg.WithFields(ctx, map[string]any{
		// "instance":    instance.GetID(),
		"env":         cfg.App.Env,
		"serviceKind": cfg.Service.Kind,
	})
	logg.Info(ctx, "starting worker")

	runWorker(ctx, cfg, logg)
	logg.Info(ctx, "worker shutting down gracefully")
}

func runWorker(ctx context.Context, cfg *config.Config, logg *logger.Logger) {
	ctx = logg.WithFields(ctx, map[string]any{
		"job":          "heartbeat",
		"pubsub_media": cfg.PubSub.MediaTopic,
	})
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logg.Info(ctx, "worker.heartbeat")
		}
	}
}
