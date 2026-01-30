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
	logg := logger.New(logger.Options{ServiceName: "cron-worker"})

	if err := godotenv.Load(); err != nil {
		logg.Warn(context.Background(), ".env file not found, relying on environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logg.Error(context.Background(), "failed to load config", err)
		os.Exit(1)
	}

	cfg.Service.Kind = "cron-worker"

	logg = logger.New(logger.Options{
		ServiceName: "cron-worker",
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

	licenseRepo := licenses.NewRepository(dbClient.DB())
	storeRepo := stores.NewRepository(dbClient.DB())
	mediaRepo := media.NewRepository(dbClient.DB())
	attachmentRepo := media.NewMediaAttachmentRepository(dbClient.DB())
	outboxRepo := outbox.NewRepository(dbClient.DB())
	outboxSvc := outbox.NewService(outboxRepo, logg)
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

	metricsCollector := metrics.NewCronJobMetrics(prometheus.DefaultRegisterer)
	lock, err := cron.NewRedisLock(redisClient, lockKey(cfg.App.Env), 0)
	if err != nil {
		logg.Error(context.Background(), "failed to create cron lock", err)
		os.Exit(1)
	}

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
	if err != nil {
		logg.Error(context.Background(), "failed to create license lifecycle job", err)
		os.Exit(1)
	}
	registry.Register(licenseJob)
	service, err := cron.NewService(cron.ServiceParams{
		Logger:   logg,
		Registry: registry,
		Lock:     lock,
		Metrics:  metricsCollector,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create cron service", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx = logg.WithFields(ctx, map[string]any{
		"env":         cfg.App.Env,
		"serviceKind": cfg.Service.Kind,
	})
	logg.Info(ctx, "starting cron worker")

	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(ctx, "cron worker stopped unexpectedly", err)
		os.Exit(1)
	}

	logg.Info(ctx, "cron worker shutting down gracefully")
}

func lockKey(env string) string {
	if env == "" {
		env = "local"
	}
	return fmt.Sprintf(lockKeyFormat, env)
}
