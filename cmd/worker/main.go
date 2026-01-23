package main

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/joho/godotenv"

	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/media/consumer"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/idempotency"
	"github.com/angelmondragon/packfinderz-backend/pkg/pubsub"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
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

	cfg.Service.Kind = "worker"

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

	pubsubClient, err := pubsub.NewClient(context.Background(), cfg.GCP, cfg.PubSub, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap pubsub", err)
		os.Exit(1)
	}
	defer func() {
		if err := pubsubClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing pubsub client", err)
		}
	}()

	gcsClient, err := gcs.NewClient(context.Background(), cfg.GCS, cfg.GCP, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap gcs", err)
		os.Exit(1)
	}
	defer func() {
		if err := gcsClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing gcs client", err)
		}
	}()

	mediaRepo := media.NewRepository(dbClient.DB())
	mediaConsumer, err := consumer.NewConsumer(mediaRepo, pubsubClient.MediaSubscription(), logg)
	if err != nil {
		logg.Error(context.Background(), "failed to create media consumer", err)
		os.Exit(1)
	}

	idempotencyManager, err := idempotency.NewManager(redisClient, cfg.Eventing.OutboxIdempotencyTTL)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap idempotency manager", err)
		os.Exit(1)
	}

	notificationRepo := notifications.NewRepository(dbClient.DB())
	notificationConsumer, err := notifications.NewConsumer(notificationRepo, pubsubClient.DomainSubscription(), idempotencyManager, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to create notifications consumer", err)
		os.Exit(1)
	}

	service, err := NewService(ServiceParams{
		Config:               cfg,
		Logger:               logg,
		DB:                   dbClient,
		Redis:                redisClient,
		PubSub:               pubsubClient,
		MediaConsumer:        mediaConsumer,
		NotificationConsumer: notificationConsumer,
		GCS:                  gcsClient,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create worker service", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx = logg.WithFields(ctx, map[string]any{
		"env":         cfg.App.Env,
		"serviceKind": cfg.Service.Kind,
	})
	logg.Info(ctx, "starting worker")

	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(ctx, "worker stopped unexpectedly", err)
		os.Exit(1)
	}

	logg.Info(ctx, "worker shutting down gracefully")
}
