package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	// bootstrap logger early (then re-init after config load)
	logg := logger.New(logger.Options{ServiceName: "migrate"})

	_ = godotenv.Load()

	// Flags
	cmd := flag.String("cmd", "up", "migration command: up|down|status|version|create|validate")
	dir := flag.String("dir", migrate.DefaultDir, "goose migrations directory")

	// Command-specific flags
	name := flag.String("name", "", "migration name (for create)")
	version := flag.String("version", "", "target version (YYYYMMDDHHMMSS) for -cmd=version")

	flag.Parse()

	cfg, err := config.Load()
	requireResource(ctx, logg, "config", err)

	logg = logger.New(logger.Options{
		ServiceName: "migrate",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	ctx = logg.WithFields(context.Background(), map[string]any{
		"env": cfg.App.Env,
		"cmd": *cmd,
		"dir": *dir,
	})

	// Commands that do NOT require DB
	switch *cmd {
	case "create":
		if *name == "" {
			fmt.Fprintln(os.Stderr, "missing -name for create")
			os.Exit(1)
		}
		logg.Info(ctx, "migrate ready")
		path, err := migrate.CreateSQLMigration(*dir, *name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create migration: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("created migration:", path)
		return

	case "validate":
		logg.Info(ctx, "migrate ready")
		if err := migrate.ValidateDir(*dir); err != nil {
			fmt.Fprintf(os.Stderr, "migration validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migration validation passed")
		return
	}

	// Everything else needs DB
	dbClient, err := db.New(context.Background(), cfg.DB, logg)
	requireResource(ctx, logg, "database", err)
	defer dbClient.Close()

	sqlDB, err := dbClient.DB().DB()
	requireResource(ctx, logg, "sql database", err)

	logg.Info(ctx, "migrate ready")

	switch *cmd {
	case "up":
		if err := migrate.Run(ctx, sqlDB, *dir, "up"); err != nil {
			fmt.Fprintf(os.Stderr, "goose up failed: %v\n", err)
			os.Exit(1)
		}

	case "down":
		if err := migrate.Run(ctx, sqlDB, *dir, "down"); err != nil {
			fmt.Fprintf(os.Stderr, "goose down failed: %v\n", err)
			os.Exit(1)
		}

	case "status":
		if err := migrate.Run(ctx, sqlDB, *dir, "status"); err != nil {
			fmt.Fprintf(os.Stderr, "goose status failed: %v\n", err)
			os.Exit(1)
		}

	case "version":
		if *version == "" {
			fmt.Fprintln(os.Stderr, "missing -version for version command")
			os.Exit(1)
		}
		if err := migrate.MigrateToVersion(ctx, sqlDB, *dir, *version); err != nil {
			fmt.Fprintf(os.Stderr, "goose version migrate failed: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintln(os.Stderr, "unknown -cmd value:", *cmd)
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
