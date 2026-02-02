package main

import (
	"context"
	"errors"
	"fmt"
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
	ctx := context.Background()
	logg := logger.New(logger.Options{ServiceName: "media-deletion-worker"})

	_ = godotenv.Load()

	cfg, err := config.Load()
	requireResource(ctx, logg, "config", err)

	cfg.Service.Kind = "media-deletion-worker"

	logg = logger.New(logger.Options{
		ServiceName: "media-deletion-worker",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	dbClient, err := db.New(context.Background(), cfg.DB, logg)
	requireResource(ctx, logg, "database", err)
	defer dbClient.Close()

	pubsubClient, err := pubsub.NewClient(context.Background(), cfg.GCP, cfg.PubSub, logg)
	requireResource(ctx, logg, "pubsub", err)
	defer pubsubClient.Close()

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
	requireResource(ctx, logg, "media deletion consumer", err)

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runCtx = logg.WithFields(runCtx, map[string]any{
		"serviceKind": cfg.Service.Kind,
		"env":         cfg.App.Env,
	})
	logg.Info(runCtx, "media deletion worker ready")

	if err := deletionConsumer.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(runCtx, "media deletion worker not working", err)
		os.Exit(1)
	}
}

func requireResource(ctx context.Context, logg *logger.Logger, resource string, err error) {
	if err == nil {
		return
	}
	logg.Error(ctx, fmt.Sprintf("resource not working: %s", resource), err)
	os.Exit(1)
}
