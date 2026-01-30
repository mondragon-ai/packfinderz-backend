package main

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/joho/godotenv"

	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/media/consumer"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pubsub"
)

func main() {
	logg := logger.New(logger.Options{ServiceName: "media-deletion-worker"})

	if err := godotenv.Load(); err != nil {
		logg.Warn(context.Background(), ".env file not found, relying on environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logg.Error(context.Background(), "failed to load config", err)
		os.Exit(1)
	}

	cfg.Service.Kind = "media-deletion-worker"

	logg = logger.New(logger.Options{
		ServiceName: "media-deletion-worker",
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

	mediaRepo := media.NewRepository(dbClient.DB())
	attachmentRepo := media.NewMediaAttachmentRepository(dbClient.DB())
	detacher := consumer.NewNoopDetacher(logg)
	deletionConsumer, err := consumer.NewDeletionConsumer(
		mediaRepo,
		attachmentRepo,
		detacher,
		pubsubClient.MediaDeletionSubscription(),
		logg,
	)
	if err != nil {
		logg.Error(context.Background(), "failed to create media deletion consumer", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx = logg.WithFields(ctx, map[string]any{
		"serviceKind": cfg.Service.Kind,
		"env":         cfg.App.Env,
	})
	logg.Info(ctx, "starting media deletion worker")

	if err := deletionConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(ctx, "media deletion worker stopped unexpectedly", err)
		os.Exit(1)
	}

	logg.Info(ctx, "media deletion worker shutting down gracefully")
}
