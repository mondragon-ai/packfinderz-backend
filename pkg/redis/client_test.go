package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestFixedWindowAllow(t *testing.T) {
	ctx := context.Background()
	mock := newMockCmdable()
	client := &Client{store: mock}

	allowed, count, err := client.FixedWindowAllow(ctx, "test-scope", 2, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed on first request")
	}
	if count != 1 {
		t.Fatalf("expected counter 1 got %d", count)
	}
	if len(mock.expireCalls) != 1 {
		t.Fatalf("expected expire for first increment")
	}

	allowed, count, err = client.FixedWindowAllow(ctx, "test-scope", 2, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed || count != 2 {
		t.Fatalf("unexpected second call state allowed=%v count=%d", allowed, count)
	}
	if len(mock.expireCalls) != 1 {
		t.Fatalf("expire should not be set again")
	}

	allowed, _, err = client.FixedWindowAllow(ctx, "test-scope", 2, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("expected limit reached")
	}
}

func TestRefreshTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	mock := newMockCmdable()
	client := &Client{store: mock}

	if err := client.StoreRefreshToken(ctx, "user-1", "store-a", "token-value", 10*time.Minute); err != nil {
		t.Fatalf("store failed: %v", err)
	}
	token, err := client.GetRefreshToken(ctx, "user-1", "store-a")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if token != "token-value" {
		t.Fatalf("expected stored token, got %q", token)
	}

	if err := client.RevokeRefreshToken(ctx, "user-1", "store-a"); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if _, err := client.GetRefreshToken(ctx, "user-1", "store-a"); err == nil || err != redis.Nil {
		t.Fatalf("expected redis.Nil after revoke, got %v", err)
	}
}

func TestKeyBuilders(t *testing.T) {
	client := &Client{}
	if got := client.IdempotencyKey("scope", "id"); got != "pf:idempotency:scope:id" {
		t.Fatalf("unexpected idempotency key %s", got)
	}
	if got := client.RateLimitKey("scope"); got != "pf:rate_limit:scope" {
		t.Fatalf("unexpected rate limit key %s", got)
	}
	if got := client.CounterKey("hits"); got != "pf:counter:hits" {
		t.Fatalf("unexpected counter key %s", got)
	}
	if got := client.RefreshTokenKey("user", "store"); got != "pf:session:user:store" {
		t.Fatalf("unexpected refresh key %s", got)
	}
	if got := client.RefreshTokenKey("user", ""); got != "pf:session:user" {
		t.Fatalf("store-less refresh key should skip empty parts, got %s", got)
	}
}

type mockCmdable struct {
	data        map[string]string
	incr        map[string]int64
	expireCalls []expireCall
}

type expireCall struct {
	key string
	ttl time.Duration
}

func newMockCmdable() *mockCmdable {
	return &mockCmdable{
		data: make(map[string]string),
		incr: make(map[string]int64),
	}
}

func (m *mockCmdable) Ping(context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func (m *mockCmdable) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	m.data[key] = fmt.Sprint(value)
	return redis.NewStatusResult("OK", nil)
}

func (m *mockCmdable) Get(ctx context.Context, key string) *redis.StringCmd {
	v, ok := m.data[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(v, nil)
}

func (m *mockCmdable) SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd {
	if _, exists := m.data[key]; exists {
		return redis.NewBoolResult(false, nil)
	}
	m.data[key] = fmt.Sprint(value)
	return redis.NewBoolResult(true, nil)
}

func (m *mockCmdable) Incr(ctx context.Context, key string) *redis.IntCmd {
	m.incr[key]++
	return redis.NewIntResult(m.incr[key], nil)
}

func (m *mockCmdable) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	m.expireCalls = append(m.expireCalls, expireCall{key: key, ttl: expiration})
	return redis.NewBoolResult(true, nil)
}

func (m *mockCmdable) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	for _, key := range keys {
		delete(m.data, key)
	}
	return redis.NewIntResult(int64(len(keys)), nil)
}
