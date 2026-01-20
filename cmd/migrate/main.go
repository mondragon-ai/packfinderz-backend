package main

import (
	"context"
	"flag"
	"os"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/joho/godotenv"
)

func main() {
	// bootstrap logger early (then re-init after config load)
	logg := logger.New(logger.Options{ServiceName: "migrate"})

	if err := godotenv.Load(); err != nil {
		logg.Warn(context.Background(), ".env file not found, relying on environment")
	}

	// Flags
	cmd := flag.String("cmd", "up", "migration command: up|down|status|version|create|validate")
	dir := flag.String("dir", migrate.DefaultDir, "goose migrations directory")

	// Command-specific flags
	name := flag.String("name", "", "migration name (for create)")
	version := flag.String("version", "", "target version (YYYYMMDDHHMMSS) for -cmd=version")

	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		logg.Error(context.Background(), "failed to load config", err)
		os.Exit(1)
	}

	logg = logger.New(logger.Options{
		ServiceName: "migrate",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	ctx := logg.WithFields(context.Background(), map[string]any{
		"env": cfg.App.Env,
		"cmd": *cmd,
		"dir": *dir,
	})

	// Commands that do NOT require DB
	switch *cmd {
	case "create":
		if *name == "" {
			logg.Error(ctx, "missing -name for create", nil)
			os.Exit(1)
		}
		path, err := migrate.CreateSQLMigration(*dir, *name)
		if err != nil {
			logg.Error(ctx, "failed to create migration", err)
			os.Exit(1)
		}
		logg.Info(ctx, "created migration: "+path)
		return

	case "validate":
		if err := migrate.ValidateDir(*dir); err != nil {
			logg.Error(ctx, "migration validation failed", err)
			os.Exit(1)
		}
		logg.Info(ctx, "migration validation passed")
		return
	}

	// Everything else needs DB
	dbClient, err := db.New(context.Background(), cfg.DB, logg)
	if err != nil {
		logg.Error(ctx, "failed to bootstrap database", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := dbClient.Close(); cerr != nil {
			logg.Error(ctx, "failed to close database", cerr)
		}
	}()

	sqlDB, err := dbClient.DB().DB()
	if err != nil {
		logg.Error(ctx, "failed to access sql DB", err)
		os.Exit(1)
	}

	logg.Info(ctx, "running migrations")

	switch *cmd {
	case "up":
		if err := migrate.Run(ctx, sqlDB, *dir, "up"); err != nil {
			logg.Error(ctx, "goose up failed", err)
			os.Exit(1)
		}

	case "down":
		if err := migrate.Run(ctx, sqlDB, *dir, "down"); err != nil {
			logg.Error(ctx, "goose down failed", err)
			os.Exit(1)
		}

	case "status":
		if err := migrate.Run(ctx, sqlDB, *dir, "status"); err != nil {
			logg.Error(ctx, "goose status failed", err)
			os.Exit(1)
		}

	case "version":
		if *version == "" {
			logg.Error(ctx, "missing -version for version command", nil)
			os.Exit(1)
		}
		if err := migrate.MigrateToVersion(ctx, sqlDB, *dir, *version); err != nil {
			logg.Error(ctx, "goose version migrate failed", err)
			os.Exit(1)
		}

	default:
		logg.Error(ctx, "unknown -cmd value: "+*cmd, nil)
		os.Exit(1)
	}

	logg.Info(ctx, "migrations completed")
}
