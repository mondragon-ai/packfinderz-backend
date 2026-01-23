package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	setNXResult bool
	setNXError  error
	lastKey     string
	lastTTL     time.Duration
	lastDeleted string
}

func (f *fakeStore) Get(context.Context, string) (string, error) {
	return "", nil
}

func (f *fakeStore) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	f.lastKey = key
	f.lastTTL = ttl
	return f.setNXResult, f.setNXError
}

func (f *fakeStore) IdempotencyKey(scope, id string) string {
	return "pf:idempotency:" + scope + ":" + id
}

func (f *fakeStore) Del(_ context.Context, keys ...string) error {
	if len(keys) > 0 {
		f.lastDeleted = keys[0]
	}
	return nil
}

func TestCheckAndMarkProcessed_FirstTime(t *testing.T) {
	store := &fakeStore{setNXResult: true}
	manager, err := NewManager(store, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	eventID := uuid.New()
	already, err := manager.CheckAndMarkProcessed(context.Background(), "orders-worker", eventID)
	if err != nil {
		t.Fatalf("CheckAndMarkProcessed: %v", err)
	}
	if already {
		t.Fatalf("expected first call to return false, got true")
	}

	expectedKey := "pf:idempotency:evt:processed:orders-worker:" + eventID.String()
	if store.lastKey != expectedKey {
		t.Fatalf("unexpected key: %q", store.lastKey)
	}
	if store.lastTTL != 24*time.Hour {
		t.Fatalf("unexpected ttl: %v", store.lastTTL)
	}
}

func TestCheckAndMarkProcessed_AlreadyProcessed(t *testing.T) {
	store := &fakeStore{setNXResult: false}
	manager, err := NewManager(store, 12*time.Hour)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	eventID := uuid.New()
	already, err := manager.CheckAndMarkProcessed(context.Background(), "orders-worker", eventID)
	if err != nil {
		t.Fatalf("CheckAndMarkProcessed: %v", err)
	}
	if !already {
		t.Fatalf("expected already processed, got false")
	}
}

func TestCheckAndMarkProcessed_Error(t *testing.T) {
	store := &fakeStore{setNXError: errors.New("boom")}
	manager, err := NewManager(store, time.Hour)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, err = manager.CheckAndMarkProcessed(context.Background(), "orders-worker", uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestDeleteProcessed(t *testing.T) {
	store := &fakeStore{}
	manager, err := NewManager(store, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	eventID := uuid.New()
	if err := manager.Delete(context.Background(), "orders-worker", eventID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	expected := "pf:idempotency:evt:processed:orders-worker:" + eventID.String()
	if store.lastDeleted != expected {
		t.Fatalf("unexpected deleted key %q", store.lastDeleted)
	}
}
