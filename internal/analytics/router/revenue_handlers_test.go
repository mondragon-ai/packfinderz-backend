package router

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	analyticspayloads "github.com/angelmondragon/packfinderz-backend/internal/analytics/payloads"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/google/uuid"
)

func TestOrderPaidHandlerInsertsRevenueRow(t *testing.T) {
	writer := &fakeWriter{}
	handler := newOrderPaidHandler(writer, logger.New(logger.Options{ServiceName: "router-order-paid-test"}))
	now := time.Now().UTC()
	event := &payloads.OrderPaidEvent{
		OrderID:       uuid.New(),
		BuyerStoreID:  uuid.New(),
		VendorStoreID: uuid.New(),
		AmountCents:   12345,
		VendorPaidAt:  now,
	}

	envelope := types.Envelope{
		EventID:    "paid-event-id",
		EventType:  enums.AnalyticsEventOrderPaid,
		OccurredAt: now.Add(-time.Hour),
	}

	if err := handler.Handle(context.Background(), envelope, event); err != nil {
		t.Fatalf("handle order_paid: %v", err)
	}

	if len(writer.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(writer.inserted))
	}

	row := writer.inserted[0]
	if row.EventType != string(envelope.EventType) {
		t.Fatalf("unexpected event type: %s", row.EventType)
	}
	if row.OccurredAt != now.UTC() {
		t.Fatalf("expected occurred_at from vendor_paid_at, got %s", row.OccurredAt)
	}
	if row.GrossRevenueCents == nil || *row.GrossRevenueCents != int64(event.AmountCents) {
		t.Fatalf("gross revenue mismatch: %v", row.GrossRevenueCents)
	}
	if row.NetRevenueCents == nil || *row.NetRevenueCents != int64(event.AmountCents) {
		t.Fatalf("net revenue mismatch: %v", row.NetRevenueCents)
	}
	if !row.Payload.Valid {
		t.Fatal("payload json not valid")
	}
	var payloadData map[string]any
	if err := json.Unmarshal([]byte(row.Payload.JSONVal), &payloadData); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payloadData["order_id"] != event.OrderID.String() {
		t.Fatalf("payload order id mismatch: %v", payloadData["order_id"])
	}
}

func TestCashCollectedHandlerInsertsRevenueRow(t *testing.T) {
	writer := &fakeWriter{}
	handler := newCashCollectedHandler(writer, logger.New(logger.Options{ServiceName: "router-cash-collected-test"}))
	now := time.Now().UTC()
	event := &analyticspayloads.CashCollectedEvent{
		OrderID:         "order-id",
		BuyerStoreID:    "buyer-id",
		VendorStoreID:   "vendor-id",
		AmountCents:     45678,
		CashCollectedAt: now,
	}

	envelope := types.Envelope{
		EventID:    "cash-event-id",
		EventType:  enums.AnalyticsEventCashCollected,
		OccurredAt: now.Add(-time.Hour),
	}

	if err := handler.Handle(context.Background(), envelope, event); err != nil {
		t.Fatalf("handle cash_collected: %v", err)
	}

	if len(writer.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(writer.inserted))
	}

	row := writer.inserted[0]
	if row.EventType != string(envelope.EventType) {
		t.Fatalf("unexpected event type: %s", row.EventType)
	}
	if row.OccurredAt != now.UTC() {
		t.Fatalf("expected occurred_at from cash_collected_at, got %s", row.OccurredAt)
	}
	if row.GrossRevenueCents == nil || *row.GrossRevenueCents != int64(event.AmountCents) {
		t.Fatalf("gross revenue mismatch: %v", row.GrossRevenueCents)
	}
	if row.NetRevenueCents == nil || *row.NetRevenueCents != int64(event.AmountCents) {
		t.Fatalf("net revenue mismatch: %v", row.NetRevenueCents)
	}
}
