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
)

func TestOrderCreatedHandlerInsertsMarketplaceRow(t *testing.T) {
	writer := &fakeWriter{}
	handler := newOrderCreatedHandler(writer, logger.New(logger.Options{ServiceName: "router-order-created-test"}))
	now := time.Now().UTC()
	event := &analyticspayloads.OrderCreatedEvent{
		CheckoutGroupID:   "checkout-group",
		CartID:            "cart-id",
		OrderID:           "order-id",
		BuyerStoreID:      "buyer-id",
		VendorStoreID:     "vendor-id",
		Currency:          "USD",
		SubtotalCents:     1000,
		DiscountsCents:    100,
		TaxCents:          50,
		TransportFeeCents: 25,
		TotalCents:        975,
		ShippingAddress: &analyticspayloads.ShippingAddress{
			PostalCode: "73112",
			Lat:        35.5,
			Lng:        -97.5,
		},
		Items: []analyticspayloads.OrderCreatedItem{
			{
				ProductID:         "product-1",
				Title:             "Product 1",
				Category:          "Flower",
				Classification:    "Indica",
				Qty:               5,
				Moq:               1,
				UnitPriceCents:    200,
				LineSubtotalCents: 1000,
				LineTotalCents:    1000,
				DiscountCents:     0,
				Status:            "ok",
				Warnings:          []string{},
			},
		},
	}

	envelope := types.Envelope{
		EventID:    "event-id",
		EventType:  enums.AnalyticsEventOrderCreated,
		OccurredAt: now,
		Payload:    []byte("{}"),
	}

	if err := handler.Handle(context.Background(), envelope, event); err != nil {
		t.Fatalf("handle order_created: %v", err)
	}

	if len(writer.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(writer.inserted))
	}

	row := writer.inserted[0]
	if row.EventID != envelope.EventID {
		t.Fatalf("unexpected event id: %s", row.EventID)
	}
	if row.OrderID == nil || *row.OrderID != event.OrderID {
		t.Fatalf("order id mismatch: got %v", row.OrderID)
	}
	if row.BuyerZip == nil || *row.BuyerZip != event.ShippingAddress.PostalCode {
		t.Fatalf("buyer zip mismatch: %v", row.BuyerZip)
	}
	if row.GrossRevenueCents == nil || *row.GrossRevenueCents != event.TotalCents {
		t.Fatalf("gross revenue mismatch: %v", row.GrossRevenueCents)
	}
	if row.NetRevenueCents == nil || *row.NetRevenueCents != event.TotalCents {
		t.Fatalf("net revenue mismatch: %v", row.NetRevenueCents)
	}
	if row.RefundCents == nil || *row.RefundCents != 0 {
		t.Fatalf("expected refund zero, got %v", row.RefundCents)
	}

	if !row.Items.Valid {
		t.Fatal("items json not valid")
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(row.Items.JSONVal), &items); err != nil {
		t.Fatalf("unmarshal items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["product_id"] != event.Items[0].ProductID {
		t.Fatalf("item product mismatch: %v", items[0]["product_id"])
	}

	if !row.Payload.Valid {
		t.Fatal("payload json not valid")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(row.Payload.JSONVal), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["order_id"] != event.OrderID {
		t.Fatalf("payload order id mismatch: %v", payload["order_id"])
	}
}

type fakeWriter struct {
	inserted []types.MarketplaceEventRow
}

func (f *fakeWriter) InsertMarketplace(_ context.Context, row types.MarketplaceEventRow) error {
	f.inserted = append(f.inserted, row)
	return nil
}

func (f *fakeWriter) InsertAdFact(_ context.Context, _ types.AdEventFactRow) error {
	return nil
}
