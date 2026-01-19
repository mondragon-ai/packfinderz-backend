package main

import (
	"context"
	"net/http"
	"os"

	"github.com/angelmondragon/packfinderz-backend/api/routes"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/instance"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
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

	logg = logger.New(logger.Options{
		ServiceName: "api",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	addr := ":" + cfg.App.Port
	ctx := logg.WithFields(context.Background(), map[string]any{
		"env":      cfg.App.Env,
		"addr":     addr,
		"instance": instance.GetID(),
	})
	logg.Info(ctx, "starting api server")

	server := &http.Server{
		Addr:    addr,
		Handler: routes.NewRouter(cfg, logg, dbClient),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Error(ctx, "api server stopped unexpectedly", err)
		os.Exit(1)
	}
}
