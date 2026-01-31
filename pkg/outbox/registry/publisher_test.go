package registry

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/google/uuid"
)

func TestEventRegistryResolveSuccess(t *testing.T) {
	reg := newTestEventRegistry(t)

	vendorOrderID := uuid.New()
	payloadBytes := mustMarshal(t, payloads.OrderCreatedEvent{
		CheckoutGroupID: uuid.New(),
		VendorOrderIDs:  []uuid.UUID{vendorOrderID},
	})

	event := models.OutboxEvent{
		EventType:     enums.EventOrderCreated,
		AggregateType: enums.AggregateCheckoutGroup,
		AggregateID:   uuid.New(),
		Payload:       mustEnvelope(t, payloadBytes),
	}

	resolved, err := reg.Resolve(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Descriptor.Topic != "orders-topic" {
		t.Fatalf("unexpected topic %q", resolved.Descriptor.Topic)
	}
	if resolved.Descriptor.EventType != enums.EventOrderCreated {
		t.Fatalf("unexpected event type %s", resolved.Descriptor.EventType)
	}
	payload, ok := resolved.Payload.(*payloads.OrderCreatedEvent)
	if !ok {
		t.Fatalf("unexpected payload type %T", resolved.Payload)
	}
	if len(payload.VendorOrderIDs) != 1 || payload.VendorOrderIDs[0] != vendorOrderID {
		t.Fatalf("payload mismatch %+v", payload)
	}
	if resolved.Envelope.EventID == "" {
		t.Fatalf("envelope missing event id")
	}
	if resolved.Envelope.OccurredAt.IsZero() {
		t.Fatalf("envelope missing occurred_at")
	}
}

func TestEventRegistryResolveUnknownEvent(t *testing.T) {
	reg := newTestEventRegistry(t)

	event := models.OutboxEvent{
		EventType:     enums.EventReservationReleased,
		AggregateType: enums.AggregateAd,
		AggregateID:   uuid.New(),
		Payload:       mustEnvelope(t, []byte(`{"reason":"none"}`)),
	}

	_, err := reg.Resolve(event)
	if err == nil {
		t.Fatalf("expected error")
	}
	var nonRetry NonRetryableError
	if !errors.As(err, &nonRetry) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
}

func TestEventRegistryResolveAggregateMismatch(t *testing.T) {
	reg := newTestEventRegistry(t)

	event := models.OutboxEvent{
		EventType:     enums.EventOrderCreated,
		AggregateType: enums.AggregateVendorOrder,
		AggregateID:   uuid.New(),
		Payload:       mustEnvelope(t, []byte(`{"checkout_group_id":"00000000-0000-0000-0000-000000000000","vendor_order_ids":[]}`)),
	}

	_, err := reg.Resolve(event)
	if err == nil {
		t.Fatalf("expected error")
	}
	var nonRetry NonRetryableError
	if !errors.As(err, &nonRetry) {
		t.Fatalf("expected non-retryable error")
	}
}

func TestEventRegistryResolveMissingAggregateID(t *testing.T) {
	reg := newTestEventRegistry(t)

	event := models.OutboxEvent{
		EventType:     enums.EventOrderCreated,
		AggregateType: enums.AggregateCheckoutGroup,
		AggregateID:   uuid.Nil,
		Payload:       mustEnvelope(t, []byte(`{}`)),
	}

	_, err := reg.Resolve(event)
	if err == nil {
		t.Fatalf("expected error")
	}
	var nonRetry NonRetryableError
	if !errors.As(err, &nonRetry) {
		t.Fatalf("expected non-retryable error")
	}
}

func TestEventRegistryResolveNullPayload(t *testing.T) {
	reg := newTestEventRegistry(t)

	event := models.OutboxEvent{
		EventType:     enums.EventOrderCreated,
		AggregateType: enums.AggregateCheckoutGroup,
		AggregateID:   uuid.New(),
		Payload:       mustEnvelope(t, []byte("null")),
	}

	_, err := reg.Resolve(event)
	if err == nil {
		t.Fatalf("expected error")
	}
	var nonRetry NonRetryableError
	if !errors.As(err, &nonRetry) {
		t.Fatalf("expected non-retryable error")
	}
}

func newTestEventRegistry(t *testing.T) *EventRegistry {
	t.Helper()
	cfg := config.PubSubConfig{
		OrdersTopic:       "orders-topic",
		BillingTopic:      "billing-topic",
		NotificationTopic: "notification-topic",
	}
	reg, err := NewEventRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	return reg
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

func mustEnvelope(t *testing.T, payload []byte) json.RawMessage {
	t.Helper()
	envelope := outbox.PayloadEnvelope{
		Version:    1,
		EventID:    uuid.NewString(),
		OccurredAt: time.Now().UTC(),
		Data:       payload,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return data
}
