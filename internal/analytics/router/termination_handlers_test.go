package router

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/google/uuid"
)

func TestOrderCanceledHandlerInsertsRow(t *testing.T) {
	writer := &fakeWriter{}
	handler := newOrderCanceledHandler(writer, logger.New(logger.Options{ServiceName: "router-order-canceled-test"}))
	now := time.Now().UTC()
	event := &payloads.OrderCanceledEvent{
		OrderID:       uuid.New(),
		BuyerStoreID:  uuid.New(),
		VendorStoreID: uuid.New(),
		CanceledAt:    now,
		Reason:        "buyer_cancel",
	}

	envelope := types.Envelope{
		EventID:    "cancel-event",
		EventType:  enums.AnalyticsEventOrderCanceled,
		OccurredAt: now.Add(-time.Hour),
	}

	if err := handler.Handle(context.Background(), envelope, event); err != nil {
		t.Fatalf("handle order_canceled: %v", err)
	}

	if len(writer.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(writer.inserted))
	}

	row := writer.inserted[0]
	if row.EventType != string(envelope.EventType) {
		t.Fatalf("unexpected event type: %s", row.EventType)
	}
	if row.OccurredAt != now {
		t.Fatalf("expected occurred_at from canceled_at, got %s", row.OccurredAt)
	}
	if !row.Payload.Valid {
		t.Fatal("payload json not valid")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(row.Payload.JSONVal), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["reason"] != event.Reason {
		t.Fatalf("payload reason mismatch: %v", payload["reason"])
	}
}

func TestOrderExpiredHandlerInsertsRow(t *testing.T) {
	writer := &fakeWriter{}
	handler := newOrderExpiredHandler(writer, logger.New(logger.Options{ServiceName: "router-order-expired-test"}))
	now := time.Now().UTC()
	ttl := 10
	event := &payloads.OrderExpiredEvent{
		OrderID:       uuid.New(),
		BuyerStoreID:  uuid.New(),
		VendorStoreID: uuid.New(),
		ExpiredAt:     now,
		TTLDays:       &ttl,
	}

	envelope := types.Envelope{
		EventID:    "expired-event",
		EventType:  enums.AnalyticsEventOrderExpired,
		OccurredAt: now.Add(-time.Hour),
	}

	if err := handler.Handle(context.Background(), envelope, event); err != nil {
		t.Fatalf("handle order_expired: %v", err)
	}

	if len(writer.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(writer.inserted))
	}

	row := writer.inserted[0]
	if row.EventType != string(envelope.EventType) {
		t.Fatalf("unexpected event type: %s", row.EventType)
	}
	if row.OccurredAt != now {
		t.Fatalf("expected occurred_at from expired_at, got %s", row.OccurredAt)
	}
	if !row.Payload.Valid {
		t.Fatal("payload json not valid")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(row.Payload.JSONVal), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if int(payload["ttl_days"].(float64)) != ttl {
		t.Fatalf("ttl mismatch: %v", payload["ttl_days"])
	}
}
