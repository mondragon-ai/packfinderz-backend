package cron

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const defaultLockTTL = 25 * time.Hour

// Lock coordinates exclusive cron runs.
type Lock interface {
	Acquire(ctx context.Context) (bool, error)
	Release(ctx context.Context) error
}

// redisStore defines the operations used by RedisLock.
type redisStore interface {
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
}

// RedisLock implements Lock using Redis SETNX + TTL.
type RedisLock struct {
	client redisStore
	key    string
	ttl    time.Duration
	owner  string
}

// NewRedisLock constructs a Redis-backed lock.
func NewRedisLock(client redisStore, key string, ttl time.Duration) (*RedisLock, error) {
	if client == nil {
		return nil, errors.New("redis client required for lock")
	}
	if key == "" {
		return nil, errors.New("lock key is required")
	}
	if ttl <= 0 {
		ttl = defaultLockTTL
	}
	return &RedisLock{client: client, key: key, ttl: ttl}, nil
}

// Acquire tries to own the lock for the configured TTL.
func (l *RedisLock) Acquire(ctx context.Context) (bool, error) {
	owner := uuid.NewString()
	ok, err := l.client.SetNX(ctx, l.key, owner, l.ttl)
	if err != nil {
		return false, fmt.Errorf("setnx: %w", err)
	}
	if ok {
		l.owner = owner
	}
	return ok, nil
}

// Release frees the lock only if the owner value still matches.
func (l *RedisLock) Release(ctx context.Context) error {
	if l.owner == "" {
		return nil
	}
	value, err := l.client.Get(ctx, l.key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("read lock owner: %w", err)
	}
	if value != l.owner {
		return nil
	}
	if err := l.client.Del(ctx, l.key); err != nil {
		return fmt.Errorf("delete lock: %w", err)
	}
	l.owner = ""
	return nil
}
