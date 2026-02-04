package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/joho/godotenv"

	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/media/consumer"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	schedulers "github.com/angelmondragon/packfinderz-backend/internal/schedulers/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/idempotency"
	"github.com/angelmondragon/packfinderz-backend/pkg/pubsub"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/square"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
)

func main() {
	ctx := context.Background()
	logg := logger.New(logger.Options{ServiceName: "worker"})

	_ = godotenv.Load()

	cfg, err := config.Load()
	requireResource(ctx, logg, "config", err)

	cfg.Service.Kind = "worker"

	logg = logger.New(logger.Options{
		ServiceName: "worker",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	squareClient, err := square.NewClient(context.Background(), cfg.Square, logg)
	requireResource(ctx, logg, "square client", err)

	dbClient, err := db.New(context.Background(), cfg.DB, logg)
	requireResource(ctx, logg, "database", err)
	defer func() {
		if err := dbClient.Close(); err != nil {
			logg.Error(ctx, "failed to close database client", err)
		}
	}()
	requireResource(ctx, logg, "migrations", migrate.MaybeRunDev(context.Background(), cfg, logg, dbClient))

	redisClient, err := redis.New(context.Background(), cfg.Redis, logg)
	requireResource(ctx, logg, "redis", err)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logg.Error(ctx, "failed to close redis client", err)
		}
	}()

	pubsubClient, err := pubsub.NewClient(context.Background(), cfg.GCP, cfg.PubSub, logg)
	requireResource(ctx, logg, "pubsub", err)
	defer func() {
		if err := pubsubClient.Close(); err != nil {
			logg.Error(ctx, "failed to close pubsub client", err)
		}
	}()

	gcsClient, err := gcs.NewClient(context.Background(), cfg.GCS, cfg.GCP, logg)
	requireResource(ctx, logg, "gcs", err)
	defer func() {
		if err := gcsClient.Close(); err != nil {
			logg.Error(ctx, "failed to close gcs client", err)
		}
	}()

	bqClient, err := bigquery.NewClient(context.Background(), cfg.GCP, cfg.BigQuery, logg)
	requireResource(ctx, logg, "bigquery", err)
	defer func() {
		if err := bqClient.Close(); err != nil {
			logg.Error(ctx, "failed to close bigquery client", err)
		}
	}()

	mediaRepo := media.NewRepository(dbClient.DB())
	mediaConsumer, err := consumer.NewConsumer(mediaRepo, pubsubClient.MediaSubscription(), logg)
	requireResource(ctx, logg, "media consumer", err)

	idempotencyManager, err := idempotency.NewManager(redisClient, cfg.Eventing.OutboxIdempotencyTTL)
	requireResource(ctx, logg, "idempotency manager", err)

	notificationRepo := notifications.NewRepository(dbClient.DB())
	notificationConsumer, err := notifications.NewConsumer(notificationRepo, pubsubClient.NotificationSubscription(), idempotencyManager, logg)
	requireResource(ctx, logg, "notifications consumer", err)

	licenseRepo := licenses.NewRepository(dbClient.DB())
	storeRepo := stores.NewRepository(dbClient.DB())
	outboxRepo := outbox.NewRepository(dbClient.DB())
	outboxSvc := outbox.NewService(outboxRepo, logg)
	licenseScheduler, err := schedulers.NewService(schedulers.ServiceParams{
		Logger:    logg,
		DB:        dbClient,
		Repo:      licenseRepo,
		StoreRepo: storeRepo,
		Outbox:    outboxSvc,
	})
	requireResource(ctx, logg, "license scheduler", err)

	service, err := NewService(ServiceParams{
		Config:               cfg,
		Logger:               logg,
		DB:                   dbClient,
		Redis:                redisClient,
		PubSub:               pubsubClient,
		MediaConsumer:        mediaConsumer,
		NotificationConsumer: notificationConsumer,
		LicenseScheduler:     licenseScheduler,
		GCS:                  gcsClient,
		BigQuery:             bqClient,
		Square:               squareClient,
	})
	requireResource(ctx, logg, "worker service", err)

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runCtx = logg.WithFields(runCtx, map[string]any{
		"env":         cfg.App.Env,
		"serviceKind": cfg.Service.Kind,
	})
	logg.Info(runCtx, "worker ready")

	if err := service.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(runCtx, "worker not working", err)
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
