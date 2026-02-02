package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/redis/go-redis/v9"
)

const (
	keyNamespace      = "pf"
	idempotencyPrefix = "idempotency"
	rateLimitPrefix   = "rate_limit"
	counterPrefix     = "counter"
	sessionPrefix     = "session"
)

type cmdable interface {
	Ping(context.Context) *redis.StatusCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Get(context.Context, string) *redis.StringCmd
	SetNX(context.Context, string, any, time.Duration) *redis.BoolCmd
	Incr(context.Context, string) *redis.IntCmd
	Expire(context.Context, string, time.Duration) *redis.BoolCmd
	Del(context.Context, ...string) *redis.IntCmd
}

// Client wraps the redis connection helpers needed by the platform.
type Client struct {
	store cmdable
	raw   *redis.Client
}

// Pinger exposes the health-check surface.
type Pinger interface {
	Ping(context.Context) error
}

// IdempotencyStore exposes minimal operations used by idempotency helpers.
type IdempotencyStore interface {
	Get(context.Context, string) (string, error)
	SetNX(context.Context, string, any, time.Duration) (bool, error)
	IdempotencyKey(scope, id string) string
	Del(context.Context, ...string) error
}

// New bootstraps a Redis client with pooling/timeouts and verifies connectivity.
func New(ctx context.Context, cfg config.RedisConfig, logg *logger.Logger) (*Client, error) {
	opts, err := optionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	raw := redis.NewClient(opts)
	if err := raw.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	// if logg != nil {
	// 	logg.Info(ctx, "redis connection established")
	// }
	return &Client{store: raw, raw: raw}, nil
}

func optionsFromConfig(cfg config.RedisConfig) (*redis.Options, error) {
	if cfg.URL == "" && cfg.Address == "" {
		return nil, errors.New("redis url or address is required")
	}
	var opts *redis.Options
	if cfg.URL != "" {
		parsed, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("parsing redis url: %w", err)
		}
		opts = parsed
	} else {
		opts = &redis.Options{
			Addr:     cfg.Address,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}
	if opts.DB == 0 {
		opts.DB = cfg.DB
	}
	if opts.PoolSize == 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if opts.MinIdleConns == 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}
	if opts.DialTimeout == 0 {
		opts.DialTimeout = cfg.DialTimeout
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = cfg.ReadTimeout
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = cfg.WriteTimeout
	}
	return opts, nil
}

// Set stores a string value with an optional TTL.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if c.store == nil {
		return errors.New("redis client not initialized")
	}
	return c.store.Set(ctx, key, value, ttl).Err()
}

// Get returns a string value stored at key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if c.store == nil {
		return "", errors.New("redis client not initialized")
	}
	return c.store.Get(ctx, key).Result()
}

// SetNX sets a value only if the key does not exist yet.
func (c *Client) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	if c.store == nil {
		return false, errors.New("redis client not initialized")
	}
	return c.store.SetNX(ctx, key, value, ttl).Result()
}

// Incr increments the counter stored at key.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if c.store == nil {
		return 0, errors.New("redis client not initialized")
	}
	return c.store.Incr(ctx, key).Result()
}

// IncrWithTTL increments and ensures the key has the supplied TTL on the first increment.
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	count, err := c.Incr(ctx, key)
	if err != nil {
		return 0, err
	}
	if ttl > 0 && count == 1 {
		if _, expErr := c.store.Expire(ctx, key, ttl).Result(); expErr != nil {
			return count, expErr
		}
	}
	return count, nil
}

// FixedWindowAllow applies a simple fixed-window rate limit.
func (c *Client) FixedWindowAllow(ctx context.Context, scope string, limit int64, window time.Duration) (bool, int64, error) {
	key := c.RateLimitKey(scope)
	count, err := c.IncrWithTTL(ctx, key, window)
	if err != nil {
		return false, 0, err
	}
	return count <= limit, count, nil
}

// IdempotencyKey returns a namespaced key for idempotency storage.
func (c *Client) IdempotencyKey(scope, id string) string {
	return c.buildKey(idempotencyPrefix, scope, id)
}

// RateLimitKey returns a namespaced key for rate limit counters.
func (c *Client) RateLimitKey(scope string) string {
	return c.buildKey(rateLimitPrefix, scope)
}

// CounterKey returns a namespaced key for counters.
func (c *Client) CounterKey(name string) string {
	return c.buildKey(counterPrefix, name)
}

// RefreshTokenKey returns a namespaced key for user/session tokens.
func (c *Client) RefreshTokenKey(userID, storeID string) string {
	if storeID == "" {
		return c.buildKey(sessionPrefix, userID)
	}
	return c.buildKey(sessionPrefix, userID, storeID)
}

// AccessSessionKey builds a namespaced key for access-token-based sessions.
func (c *Client) AccessSessionKey(accessID string) string {
	return c.buildKey(sessionPrefix, "access", accessID)
}

// StoreRefreshToken writes a refresh token with the provided TTL.
func (c *Client) StoreRefreshToken(ctx context.Context, userID, storeID, token string, ttl time.Duration) error {
	key := c.RefreshTokenKey(userID, storeID)
	return c.Set(ctx, key, token, ttl)
}

// GetRefreshToken pulls the refresh token for the given user/store.
func (c *Client) GetRefreshToken(ctx context.Context, userID, storeID string) (string, error) {
	return c.Get(ctx, c.RefreshTokenKey(userID, storeID))
}

// RevokeRefreshToken deletes the stored refresh token.
func (c *Client) RevokeRefreshToken(ctx context.Context, userID, storeID string) error {
	if c.store == nil {
		return errors.New("redis client not initialized")
	}
	_, err := c.store.Del(ctx, c.RefreshTokenKey(userID, storeID)).Result()
	return err
}

// Del removes the provided keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if c.store == nil {
		return errors.New("redis client not initialized")
	}
	return c.store.Del(ctx, keys...).Err()
}

// Ping verifies the connection.
func (c *Client) Ping(ctx context.Context) error {
	if c.store == nil {
		return errors.New("redis client not initialized")
	}
	return c.store.Ping(ctx).Err()
}

// Close shuts down the underlying client if available.
func (c *Client) Close() error {
	if c.raw == nil {
		return nil
	}
	return c.raw.Close()
}

func (c *Client) buildKey(parts ...string) string {
	if len(parts) == 0 {
		return keyNamespace
	}
	clean := []string{keyNamespace}
	for _, part := range parts {
		if part == "" {
			continue
		}
		clean = append(clean, strings.TrimSpace(part))
	}
	return strings.Join(clean, ":")
}
