package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/joho/godotenv"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/registry"
	"github.com/angelmondragon/packfinderz-backend/pkg/pubsub"
)

func main() {
	ctx := context.Background()
	logg := logger.New(logger.Options{ServiceName: "outbox-publisher"})

	_ = godotenv.Load()

	cfg, err := config.Load()
	requireResource(ctx, logg, "config", err)

	cfg.Service.Kind = "outbox-publisher"

	logg = logger.New(logger.Options{
		ServiceName: "outbox-publisher",
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

	pubsubClient, err := pubsub.NewClient(context.Background(), cfg.GCP, cfg.PubSub, logg)
	requireResource(ctx, logg, "pubsub", err)
	defer func() {
		if err := pubsubClient.Close(); err != nil {
			logg.Error(ctx, "failed to close pubsub client", err)
		}
	}()

	repo := outbox.NewRepository(dbClient.DB())
	dlqRepo := outbox.NewDLQRepository(dbClient.DB())
	eventRegistry, err := registry.NewEventRegistry(cfg.PubSub)
	requireResource(ctx, logg, "event registry", err)
	service, err := NewService(ServiceParams{
		Config:        cfg,
		Logger:        logg,
		DB:            dbClient,
		PubSub:        pubsubClient,
		Repository:    repo,
		Registry:      eventRegistry,
		DLQRepository: dlqRepo,
	})
	requireResource(ctx, logg, "outbox publisher service", err)

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runCtx = logg.WithFields(runCtx, map[string]any{
		"env":         cfg.App.Env,
		"serviceKind": "outbox-publisher",
	})
	logg.Info(runCtx, "outbox publisher ready")

	if err := service.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(runCtx, "outbox publisher not working", err)
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
