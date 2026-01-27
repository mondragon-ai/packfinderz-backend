package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
)

func TestAnalyticsConsumerProcessesOrderCreated(t *testing.T) {
	inserter := &fakeInserter{}
	manager := fakeIdempotency{
		check: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			return false, nil
		},
		deleteFn: func(_ context.Context, _ string, _ uuid.UUID) error {
			return nil
		},
	}
	consumer := mustConsumer(t, inserter, manager)

	checkoutID := uuid.New()
	envelope := buildEnvelope(t, uuid.New(), map[string]any{
		"checkout_group_id": checkoutID.String(),
		"vendor_order_ids":  []string{uuid.NewString()},
	})

	if err := consumer.Process(context.Background(), enums.EventOrderCreated, envelope); err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	if len(inserter.rows) != 1 {
		t.Fatalf("expected 1 row inserted, got %d", len(inserter.rows))
	}
	row, ok := inserter.rows[0].(*marketplaceEventRow)
	if !ok {
		t.Fatalf("expected marketplaceEventRow, got %T", inserter.rows[0])
	}
	if row.EventType != string(enums.EventOrderCreated) {
		t.Fatalf("unexpected event type: %s", row.EventType)
	}
	if row.CheckoutGroupID == nil || *row.CheckoutGroupID != checkoutID.String() {
		t.Fatalf("checkout group id mismatch")
	}
	if row.OrderID != nil {
		t.Fatalf("order id should be nil for checkout-level event")
	}
	if !row.Payload.Valid {
		t.Fatalf("payload should be valid json")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(row.Payload.JSONVal), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["vendor_order_ids"]; !ok {
		t.Fatalf("payload missing vendor_order_ids")
	}
}

func TestAnalyticsConsumerIsIdempotent(t *testing.T) {
	inserter := &fakeInserter{}
	manager := fakeIdempotency{
		check: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			return true, nil
		},
		deleteFn: func(_ context.Context, _ string, _ uuid.UUID) error {
			return nil
		},
	}
	consumer := mustConsumer(t, inserter, manager)

	envelope := buildEnvelope(t, uuid.New(), map[string]any{})
	if err := consumer.Process(context.Background(), enums.EventOrderCreated, envelope); err != nil {
		t.Fatalf("Process() error: %v", err)
	}
	if len(inserter.rows) != 0 {
		t.Fatalf("expected no rows inserted when idempotent")
	}
}

func TestAnalyticsConsumerDeletesOnInsertFailure(t *testing.T) {
	inserter := &fakeInserter{err: errors.New("bigquery down")}
	deleted := false
	manager := fakeIdempotency{
		check: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			return false, nil
		},
		deleteFn: func(_ context.Context, _ string, _ uuid.UUID) error {
			deleted = true
			return nil
		},
	}
	consumer := mustConsumer(t, inserter, manager)

	envelope := buildEnvelope(t, uuid.New(), map[string]any{
		"checkout_group_id": uuid.NewString(),
	})
	if err := consumer.Process(context.Background(), enums.EventOrderCreated, envelope); err == nil {
		t.Fatalf("expected error when insert fails")
	}
	if !deleted {
		t.Fatalf("expected idempotency key deletion on failure")
	}
}

func TestAnalyticsConsumerDeletesOnPayloadDecodeFailure(t *testing.T) {
	inserter := &fakeInserter{}
	deleted := false
	manager := fakeIdempotency{
		check: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			return false, nil
		},
		deleteFn: func(_ context.Context, _ string, _ uuid.UUID) error {
			deleted = true
			return nil
		},
	}
	consumer := mustConsumer(t, inserter, manager)

	envelope := outbox.PayloadEnvelope{
		Version:    1,
		EventID:    uuid.NewString(),
		OccurredAt: time.Now(),
		Data:       []byte("{invalid json"),
	}
	if err := consumer.Process(context.Background(), enums.EventOrderCreated, envelope); err == nil {
		t.Fatalf("expected error for bad payload")
	}
	if !deleted {
		t.Fatalf("expected idempotency key deletion on payload error")
	}
	if len(inserter.rows) != 0 {
		t.Fatalf("expected no rows inserted on payload failure")
	}
}

type fakeInserter struct {
	rows []any
	err  error
}

func (f *fakeInserter) InsertRows(ctx context.Context, table string, rows []any) error {
	f.rows = append(f.rows, rows...)
	return f.err
}

type fakeIdempotency struct {
	check    func(ctx context.Context, consumer string, eventID uuid.UUID) (bool, error)
	deleteFn func(ctx context.Context, consumer string, eventID uuid.UUID) error
}

func (f fakeIdempotency) CheckAndMarkProcessed(ctx context.Context, consumer string, eventID uuid.UUID) (bool, error) {
	return f.check(ctx, consumer, eventID)
}

func (f fakeIdempotency) Delete(ctx context.Context, consumer string, eventID uuid.UUID) error {
	return f.deleteFn(ctx, consumer, eventID)
}

func mustConsumer(t *testing.T, inserter *fakeInserter, manager fakeIdempotency) *Consumer {
	t.Helper()
	consumer, err := NewConsumer(inserter, "marketplace_events", manager, logger.New(logger.Options{
		ServiceName: "analytics-test",
		Level:       logger.ParseLevel("debug"),
		Output:      io.Discard,
	}))
	if err != nil {
		t.Fatalf("failed to build consumer: %v", err)
	}
	return consumer
}

func buildEnvelope(t *testing.T, eventID uuid.UUID, payload any) outbox.PayloadEnvelope {
	t.Helper()
	bytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return outbox.PayloadEnvelope{
		Version:    1,
		EventID:    eventID.String(),
		OccurredAt: time.Now(),
		Data:       bytes,
	}
}
