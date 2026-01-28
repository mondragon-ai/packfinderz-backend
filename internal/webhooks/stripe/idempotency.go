package stripewebhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
)

type IdempotencyGuard struct {
	store redis.IdempotencyStore
	ttl   time.Duration
	scope string
}

func NewIdempotencyGuard(store redis.IdempotencyStore, ttl time.Duration, scope string) (*IdempotencyGuard, error) {
	if store == nil {
		return nil, errors.New("idempotency store is required")
	}
	if ttl < 0 {
		return nil, errors.New("ttl must be non-negative")
	}
	if scope == "" {
		return nil, errors.New("scope is required")
	}
	return &IdempotencyGuard{
		store: store,
		ttl:   ttl,
		scope: scope,
	}, nil
}

func (g *IdempotencyGuard) CheckAndMark(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, errors.New("event id is required")
	}
	key := g.store.IdempotencyKey(g.scope, eventID)
	set, err := g.store.SetNX(ctx, key, "1", g.ttl)
	if err != nil {
		return false, fmt.Errorf("set idempotency key: %w", err)
	}
	return !set, nil
}

func (g *IdempotencyGuard) Delete(ctx context.Context, eventID string) error {
	if eventID == "" {
		return errors.New("event id is required")
	}
	key := g.store.IdempotencyKey(g.scope, eventID)
	return g.store.Del(ctx, key)
}
