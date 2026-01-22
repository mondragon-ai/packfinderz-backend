package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pubsub"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
)

type ServiceParams struct {
	Config *config.Config
	Logger *logger.Logger
	DB     *db.Client
	Redis  *redis.Client
	PubSub *pubsub.Client
	GCS    *gcs.Client
}

type Service struct {
	cfg    *config.Config
	logg   *logger.Logger
	db     *db.Client
	redis  *redis.Client
	pubsub *pubsub.Client
	gcs    *gcs.Client
}

func NewService(params ServiceParams) (*Service, error) {
	if params.Config == nil {
		return nil, errors.New("config is required")
	}
	if params.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if params.DB == nil {
		return nil, errors.New("database client is required")
	}
	if params.Redis == nil {
		return nil, errors.New("redis client is required")
	}
	if params.PubSub == nil {
		return nil, errors.New("pubsub client is required")
	}
	if params.GCS == nil {
		return nil, errors.New("gcs client is required")
	}

	return &Service{
		cfg:    params.Config,
		logg:   params.Logger,
		db:     params.DB,
		redis:  params.Redis,
		pubsub: params.PubSub,
		gcs:    params.GCS,
	}, nil
}

func (s *Service) ensureReadiness(ctx context.Context) error {
	if err := pingDependency(ctx, s.logg, "database", s.db.Ping); err != nil {
		return err
	}
	if err := pingDependency(ctx, s.logg, "redis", s.redis.Ping); err != nil {
		return err
	}
	if err := pingDependency(ctx, s.logg, "pubsub", s.pubsub.Ping); err != nil {
		return err
	}
	if err := pingDependency(ctx, s.logg, "gcs", s.gcs.Ping); err != nil {
		return err
	}
	s.logg.Info(ctx, "all worker dependencies are ready")
	return nil
}

func pingDependency(ctx context.Context, logg *logger.Logger, name string, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		logg.Error(ctx, fmt.Sprintf("%s ping failed", name), err)
		return fmt.Errorf("%s ping failed: %w", name, err)
	}
	logg.Info(ctx, fmt.Sprintf("%s ping succeeded", name))
	return nil
}

func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.ensureReadiness(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logg.Info(ctx, "worker context canceled")
			return ctx.Err()
		case <-ticker.C:
			s.logg.Info(ctx, "worker.heartbeat")
		}
	}
}
