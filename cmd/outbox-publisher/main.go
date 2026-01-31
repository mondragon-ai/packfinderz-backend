package main

import (
	"context"
	"errors"
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
	logg := logger.New(logger.Options{ServiceName: "outbox-publisher"})

	if err := godotenv.Load(); err != nil {
		logg.Warn(context.Background(), ".env file not found, relying on environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logg.Error(context.Background(), "failed to load config", err)
		os.Exit(1)
	}

	cfg.Service.Kind = "outbox-publisher"

	logg = logger.New(logger.Options{
		ServiceName: "outbox-publisher",
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

	repo := outbox.NewRepository(dbClient.DB())
	eventRegistry, err := registry.NewEventRegistry(cfg.PubSub)
	if err != nil {
		logg.Error(context.Background(), "failed to build event registry", err)
		os.Exit(1)
	}
	service, err := NewService(ServiceParams{
		Config:     cfg,
		Logger:     logg,
		DB:         dbClient,
		PubSub:     pubsubClient,
		Repository: repo,
		Registry:   eventRegistry,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create outbox publisher", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx = logg.WithFields(ctx, map[string]any{
		"env":         cfg.App.Env,
		"serviceKind": "outbox-publisher",
	})
	logg.Info(ctx, "starting outbox publisher")

	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(ctx, "outbox publisher stopped unexpectedly", err)
		os.Exit(1)
	}

	logg.Info(ctx, "outbox publisher shutting down gracefully")
}
