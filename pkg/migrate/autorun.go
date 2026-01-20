package migrate

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// MaybeRunDev executes migrations automatically when the app is running in dev mode and
// the feature flag is enabled.
func MaybeRunDev(ctx context.Context, cfg *config.Config, logg *logger.Logger, client *db.Client) error {
	if !cfg.App.IsDev() || !cfg.FeatureFlags.AutoMigrate {
		return nil
	}

	sqlDB, err := client.DB().DB()
	if err != nil {
		return fmt.Errorf("extracting sql.DB: %w", err)
	}

	meta := map[string]any{"env": cfg.App.Env, "dir": DefaultDir}
	ctx = logg.WithFields(ctx, meta)
	logg.Info(ctx, "running Goose migrations (dev auto-run)")

	if err := Run(ctx, sqlDB, DefaultDir, "up"); err != nil {
		return fmt.Errorf("running goose up: %w", err)
	}

	logg.Info(ctx, "Goose migrations completed")
	return nil
}
