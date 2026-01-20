package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	redislib "github.com/redis/go-redis/v9"
)

type mockStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newMockStore() *mockStore {
	return &mockStore{data: make(map[string]string)}
}

func (m *mockStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = fmt.Sprint(value)
	return nil
}

func (m *mockStore) Get(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.data[key]
	if !ok {
		return "", redislib.Nil
	}
	return val, nil
}

func (m *mockStore) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *mockStore) AccessSessionKey(accessID string) string {
	return fmt.Sprintf("sess:%s", accessID)
}

func TestManagerGenerateAndRotate(t *testing.T) {
	store := newMockStore()
	manager := &Manager{
		store: store,
		keyer: store,
		ttl:   time.Hour,
	}

	ctx := context.Background()
	accessID := "access-123"
	token, err := manager.Generate(ctx, accessID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if stored := store.data[store.AccessSessionKey(accessID)]; stored != token {
		t.Fatalf("expected stored token %q, got %q", token, stored)
	}

	if _, _, err := manager.Rotate(ctx, accessID, "wrong"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected invalid refresh token error, got %v", err)
	}

	newAccessID, newToken, err := manager.Rotate(ctx, accessID, token)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, exists := store.data[store.AccessSessionKey(accessID)]; exists {
		t.Fatalf("old access key left behind")
	}
	if stored := store.data[store.AccessSessionKey(newAccessID)]; stored != newToken {
		t.Fatalf("expected new token stored, got %q", stored)
	}
}
