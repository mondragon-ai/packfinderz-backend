package idempotency

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type exampleStore struct {
	values []bool
	index  int
}

func (s *exampleStore) Get(context.Context, string) (string, error) {
	return "", nil
}

func (s *exampleStore) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	result := false
	if s.index < len(s.values) {
		result = s.values[s.index]
	}
	s.index++
	return result, nil
}

func (s *exampleStore) IdempotencyKey(scope, id string) string {
	return "pf:idempotency:" + scope + ":" + id
}

type exampleConsumer struct {
	name    string
	manager *Manager
}

func (c *exampleConsumer) handle(ctx context.Context, eventID uuid.UUID) string {
	alreadyProcessed, _ := c.manager.CheckAndMarkProcessed(ctx, c.name, eventID)
	if alreadyProcessed {
		return "already processed"
	}
	return "processing event"
}

func ExampleManager_CheckAndMarkProcessed() {
	ctx := context.Background()
	store := &exampleStore{values: []bool{true, false}}
	manager, _ := NewManager(store, 7*24*time.Hour)
	consumer := &exampleConsumer{name: "orders-worker", manager: manager}
	eventID := uuid.MustParse("f47ac10b-58cc-4372-a567-0e02b2c3d479")

	fmt.Println(consumer.handle(ctx, eventID))
	fmt.Println(consumer.handle(ctx, eventID))
	// Output:
	// processing event
	// already processed
}
