package main

import (
	"context"
	"net/http"
	"os"

	"github.com/angelmondragon/packfinderz-backend/api/routes"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/joho/godotenv"
)

func main() {
	logg := logger.New(logger.Options{ServiceName: "api"})

	if err := godotenv.Load(); err != nil {
		logg.Warn(context.Background(), ".env file not found, relying on environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logg.Error(context.Background(), "failed to load config", err)
		os.Exit(1)
	}

	logg = logger.New(logger.Options{
		ServiceName: "api",
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

	sessionManager, err := session.NewManager(redisClient, cfg.JWT)
	if err != nil {
		logg.Error(context.Background(), "failed to create session manager", err)
		os.Exit(1)
	}

	authService, err := auth.NewService(auth.ServiceParams{
		UserRepo:        users.NewRepository(dbClient.DB()),
		MembershipsRepo: memberships.NewRepository(dbClient.DB()),
		SessionManager:  sessionManager,
		JWTConfig:       cfg.JWT,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create auth service", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.App.Port
	}
	addr := ":" + port
	id := os.Getenv("DYNO")
	if id == "" {
		id = "local"
	}
	ctx := logg.WithFields(context.Background(), map[string]any{
		"env":      cfg.App.Env,
		"addr":     addr,
		"instance": id,
	})
	logg.Info(ctx, "starting api server")

	server := &http.Server{
		Addr:    addr,
		Handler: routes.NewRouter(cfg, logg, dbClient, redisClient, sessionManager, authService),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Error(ctx, "api server stopped unexpectedly", err)
		os.Exit(1)
	}
}
