package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Client wraps the shared GORM connection.
type Client struct {
	conn *gorm.DB
}

// Pinger exposes the health check surface.
type Pinger interface {
	Ping(ctx context.Context) error
}

// New boots a GORM client using the provided configuration.
func New(ctx context.Context, cfg config.DBConfig, logg *logger.Logger) (*Client, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	dialector := postgres.New(postgres.Config{
		DSN:                  cfg.DSN,
		PreferSimpleProtocol: true,
	})

	gormLogger := gormlogger.New(
		log.New(io.Discard, "", log.LstdFlags),
		gormlogger.Config{LogLevel: gormlogger.Silent},
	)

	gormCfg := &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true,
	}

	conn, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("opening db connection: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("getting sql db handle: %w", err)
	}

	applyPoolSettings(sqlDB, cfg)

	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	if logg != nil {
		logg.Info(ctx, "database connection established")
	}

	return &Client{conn: conn}, nil
}

func applyPoolSettings(sqlDB *sql.DB, cfg config.DBConfig) {
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
}

// DB returns the underlying GORM connection.
func (c *Client) DB() *gorm.DB {
	return c.conn
}

// Ping verifies the datasource is reachable.
func (c *Client) Ping(ctx context.Context) error {
	sqlDB, err := c.conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close shuts down the pooled connections.
func (c *Client) Close() error {
	sqlDB, err := c.conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Exec wraps GORM's Exec with context propagation.
func (c *Client) Exec(ctx context.Context, query string, args ...any) *gorm.DB {
	return c.conn.WithContext(ctx).Exec(query, args...)
}

// Raw wraps GORM's Raw with context propagation.
func (c *Client) Raw(ctx context.Context, query string, args ...any) *gorm.DB {
	return c.conn.WithContext(ctx).Raw(query, args...)
}

// WithTx executes fn inside a transaction, rolling back on error/panic.
func (c *Client) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	tx := c.conn.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
