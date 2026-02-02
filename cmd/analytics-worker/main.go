package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/joho/godotenv"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/router"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/worker"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/idempotency"
	"github.com/angelmondragon/packfinderz-backend/pkg/pubsub"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
)

func main() {
	ctx := context.Background()
	logg := logger.New(logger.Options{ServiceName: "analytics-worker"})

	_ = godotenv.Load()

	cfg, err := config.Load()
	requireResource(ctx, logg, "config", err)

	cfg.Service.Kind = "analytics-worker"

	logg = logger.New(logger.Options{
		ServiceName: "analytics-worker",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

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

	subscription := pubsubClient.AnalyticsSubscription()
	if subscription == nil {
		requireResource(ctx, logg, "analytics subscription", errors.New("subscription not configured"))
	}

	manager, err := idempotency.NewManager(redisClient, cfg.Eventing.OutboxIdempotencyTTL)
	requireResource(ctx, logg, "idempotency manager", err)

	routingHandler, err := router.NewRouter(noopWriter{}, logg, nil)
	requireResource(ctx, logg, "analytics router", err)

	service, err := worker.NewService(subscription, routingHandler, manager, logg)
	requireResource(ctx, logg, "analytics worker service", err)

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runCtx = logg.WithFields(runCtx, map[string]any{
		"env":         cfg.App.Env,
		"serviceKind": cfg.Service.Kind,
	})
	logg.Info(runCtx, "analytics worker ready")

	if err := service.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error(runCtx, "analytics worker failed", err)
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

type noopWriter struct{}

func (noopWriter) InsertMarketplace(ctx context.Context, row types.MarketplaceEventRow) error {
	return nil
}

func (noopWriter) InsertAdFact(ctx context.Context, row types.AdEventFactRow) error {
	return nil
}
