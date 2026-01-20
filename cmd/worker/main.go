package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
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

	dbClient, err := db.New(context.Background(), cfg.DB, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap database", err)
		os.Exit(1)
	}
	defer func() {
		if err := dbClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing database", err)
		}
	}()

	if err := migrate.MaybeRunDev(context.Background(), cfg, logg, dbClient); err != nil {
		logg.Error(context.Background(), "failed to run dev migrations", err)
		os.Exit(1)
	}

	redisClient, err := redis.New(context.Background(), cfg.Redis, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap redis", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing redis", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctx = logg.WithFields(ctx, map[string]any{
		// "instance":    instance.GetID(),
		"env":         cfg.App.Env,
		"serviceKind": cfg.Service.Kind,
	})
	logg.Info(ctx, "starting worker")

	runWorker(ctx, cfg, logg, dbClient, redisClient)
	logg.Info(ctx, "worker shutting down gracefully")
}

func runWorker(ctx context.Context, cfg *config.Config, logg *logger.Logger, dbClient *db.Client, redisClient *redis.Client) {
	ctx = logg.WithFields(ctx, map[string]any{
		"job":          "heartbeat",
		"pubsub_media": cfg.PubSub.MediaTopic,
	})
	if err := dbClient.Ping(ctx); err != nil {
		logg.Error(ctx, "database ping failed", err)
	} else {
		logg.Info(ctx, "database ping succeeded")
	}
	if err := redisClient.Ping(ctx); err != nil {
		logg.Error(ctx, "redis ping failed", err)
	} else {
		logg.Info(ctx, "redis ping succeeded")
	}
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
