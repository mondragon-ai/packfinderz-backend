package main

import (
	"context"
	"net/http"
	"os"

	"github.com/angelmondragon/packfinderz-backend/api/routes"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
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
		Handler: routes.NewRouter(cfg, logg),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Error(ctx, "api server stopped unexpectedly", err)
		os.Exit(1)
	}
}
