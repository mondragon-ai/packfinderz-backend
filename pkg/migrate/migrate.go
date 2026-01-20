package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/pressly/goose/v3"
)

const DefaultDir = "pkg/migrate/migrations"

// Run executes a standard goose command that requires a DB connection.
func Run(ctx context.Context, db *sql.DB, dir string, command string, args ...string) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	if dir == "" {
		return fmt.Errorf("dir is required")
	}

	// PackFinderz is Postgres today
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	// RunContext prints status output to stdout (goose internal)
	if err := goose.RunContext(ctx, command, db, dir, args...); err != nil {
		return fmt.Errorf("goose %s: %w", command, err)
	}
	return nil
}

// MigrateToVersion migrates up/down to the requested version by comparing current DB version.
func MigrateToVersion(ctx context.Context, db *sql.DB, dir string, targetVersion string) error {
	if targetVersion == "" {
		return fmt.Errorf("targetVersion is required")
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	target, err := strconv.ParseInt(targetVersion, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid version %q (expected YYYYMMDDHHMMSS): %w", targetVersion, err)
	}

	current, err := goose.GetDBVersion(db)
	if err != nil {
		return fmt.Errorf("get db version: %w", err)
	}

	switch {
	case current == target:
		return nil

	case current < target:
		// migrate up to target
		if err := goose.UpToContext(ctx, db, dir, target); err != nil {
			return fmt.Errorf("goose up-to %d: %w", target, err)
		}
		return nil

	default:
		// migrate down to target
		if err := goose.DownToContext(ctx, db, dir, target); err != nil {
			return fmt.Errorf("goose down-to %d: %w", target, err)
		}
		return nil
	}
}
