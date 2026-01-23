package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
)

// Manager tracks processed event IDs per consumer using Redis SETNX with a TTL.
// Keys follow the `pf:idempotency:evt:processed:<consumer>:<event_id>` pattern.
type Manager struct {
	store redis.IdempotencyStore
	ttl   time.Duration
}

// NewManager builds an idempotency guard that marks events as processed for the given TTL.
func NewManager(store redis.IdempotencyStore, ttl time.Duration) (*Manager, error) {
	if store == nil {
		return nil, errors.New("idempotency store is required")
	}
	if ttl < 0 {
		return nil, errors.New("ttl must be non-negative")
	}
	return &Manager{
		store: store,
		ttl:   ttl,
	}, nil
}

// CheckAndMarkProcessed returns true if the event has already been processed and
// otherwise marks it as processed with the configured TTL.
func (m *Manager) CheckAndMarkProcessed(ctx context.Context, consumer string, eventID uuid.UUID) (bool, error) {
	if consumer == "" {
		return false, errors.New("consumer name is required")
	}
	if eventID == uuid.Nil {
		return false, errors.New("event id is required")
	}
	scope := fmt.Sprintf("evt:processed:%s", consumer)
	key := m.store.IdempotencyKey(scope, eventID.String())
	set, err := m.store.SetNX(ctx, key, "1", m.ttl)
	if err != nil {
		return false, err
	}
	return !set, nil
}
