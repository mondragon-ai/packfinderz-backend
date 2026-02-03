package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/angelmondragon/packfinderz-backend/internal/cron"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/metrics"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
)

const lockKeyFormat = "pf:cron-worker:lock:%s"

func main() {
	ctx := context.Background()
	logg := logger.New(logger.Options{ServiceName: "cron-worker"})

	_ = godotenv.Load()

	cfg, err := config.Load()
	requireResource(ctx, logg, "config", err)

	cfg.Service.Kind = "cron-worker"

	logg = logger.New(logger.Options{
		ServiceName: "cron-worker",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

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

	licenseRepo := licenses.NewRepository(dbClient.DB())
	storeRepo := stores.NewRepository(dbClient.DB())
	mediaRepo := media.NewRepository(dbClient.DB())
	attachmentRepo := media.NewMediaAttachmentRepository(dbClient.DB())
	outboxRepo := outbox.NewRepository(dbClient.DB())
	outboxSvc := outbox.NewService(outboxRepo, logg)
	gcsClient, err := gcs.NewClient(context.Background(), cfg.GCS, cfg.GCP, logg)
	requireResource(ctx, logg, "gcs", err)
	defer func() {
		if err := gcsClient.Close(); err != nil {
			logg.Error(ctx, "failed to close gcs client", err)
		}
	}()

	metricsCollector := metrics.NewCronJobMetrics(prometheus.DefaultRegisterer)
	lock, err := cron.NewRedisLock(redisClient, lockKey(cfg.App.Env), 0)
	requireResource(ctx, logg, "cron lock", err)

	registry := cron.NewRegistry()
	licenseJob, err := cron.NewLicenseLifecycleJob(cron.LicenseLifecycleJobParams{
		Logger:         logg,
		DB:             dbClient,
		LicenseRepo:    licenseRepo,
		StoreRepo:      storeRepo,
		MediaRepo:      mediaRepo,
		AttachmentRepo: attachmentRepo,
		Outbox:         outboxSvc,
		OutboxRepo:     outboxRepo,
		GCS:            gcsClient,
		GCSBucket:      cfg.GCS.BucketName,
	})
	requireResource(ctx, logg, "license job", err)
	registry.Register(licenseJob)

	ordersRepo := orders.NewRepository(dbClient.DB())
	orderTTLJob, err := cron.NewOrderTTLJob(cron.OrderTTLJobParams{
		Logger:        logg,
		DB:            dbClient,
		PendingReader: ordersRepo,
		Inventory:     orders.NewInventoryReleaser(),
		Outbox:        outboxSvc,
		OutboxRepo:    outboxRepo,
	})
	requireResource(ctx, logg, "order ttl job", err)
	registry.Register(orderTTLJob)
	notificationRepo := notifications.NewRepository(dbClient.DB())
	notificationCleanupJob, err := cron.NewNotificationCleanupJob(cron.NotificationCleanupJobParams{
		Logger:     logg,
		DB:         dbClient,
		Repository: notificationRepo,
	})
	requireResource(ctx, logg, "notification cleanup job", err)
	registry.Register(notificationCleanupJob)

	pendingMediaCleanupJob, err := cron.NewPendingMediaCleanupJob(cron.PendingMediaCleanupJobParams{
		Logger:         logg,
		DB:             dbClient,
		MediaRepo:      mediaRepo,
		AttachmentRepo: attachmentRepo,
	})
	requireResource(ctx, logg, "pending media cleanup job", err)
	registry.Register(pendingMediaCleanupJob)

	outboxRetentionJob, err := cron.NewOutboxRetentionJob(cron.OutboxRetentionJobParams{
		Logger:     logg,
		DB:         dbClient,
		Repository: outboxRepo,
	})
	requireResource(ctx, logg, "outbox retention job", err)
	registry.Register(outboxRetentionJob)
	service, err := cron.NewService(cron.ServiceParams{
		Logger:   logg,
		Registry: registry,
		Lock:     lock,
		Metrics:  metricsCollector,
	})
	requireResource(ctx, logg, "cron service", err)

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runCtx = logg.WithFields(runCtx, map[string]any{
		"env":         cfg.App.Env,
		"serviceKind": cfg.Service.Kind,
	})
	logg.Info(runCtx, "cron worker ready")

	if err := service.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(runCtx, "cron worker not working", err)
		os.Exit(1)
	}
}

func lockKey(env string) string {
	if env == "" {
		env = "local"
	}
	return fmt.Sprintf(lockKeyFormat, env)
}

func requireResource(ctx context.Context, logg *logger.Logger, resource string, err error) {
	if err == nil {
		return
	}
	logg.Error(ctx, fmt.Sprintf("resource not working: %s", resource), err)
	os.Exit(1)
}
