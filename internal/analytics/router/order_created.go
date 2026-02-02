package router

import (
	"context"
	"fmt"

	analyticspayloads "github.com/angelmondragon/packfinderz-backend/internal/analytics/payloads"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	analyticswriter "github.com/angelmondragon/packfinderz-backend/internal/analytics/writer"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type orderCreatedHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newOrderCreatedHandler(writer Writer, logg *logger.Logger) Handler {
	return &orderCreatedHandler{writer: writer, logg: logg}
}

func (h *orderCreatedHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*analyticspayloads.OrderCreatedEvent)
	if !ok {
		return fmt.Errorf("invalid payload for order_created")
	}

	fields := map[string]any{
		"event_type":     envelope.EventType,
		"order_id":       event.OrderID,
		"vendor_store":   event.VendorStoreID,
		"buyer_store":    event.BuyerStoreID,
		"checkout_group": event.CheckoutGroupID,
	}
	logCtx := h.logg.WithFields(ctx, fields)

	row, err := buildOrderCreatedRow(envelope, event)
	if err != nil {
		h.logg.Error(logCtx, "failed to build marketplace row", err)
		return err
	}

	if err := h.writer.InsertMarketplace(logCtx, row); err != nil {
		h.logg.Error(logCtx, "failed to insert marketplace row", err)
		return err
	}

	h.logg.Info(logCtx, "order_created handler inserted marketplace row")
	return nil
}

func buildOrderCreatedRow(envelope types.Envelope, event *analyticspayloads.OrderCreatedEvent) (types.MarketplaceEventRow, error) {
	itemsJSON, err := analyticswriter.EncodeJSON(event.Items)
	if err != nil {
		return types.MarketplaceEventRow{}, fmt.Errorf("encode items json: %w", err)
	}
	payloadJSON, err := analyticswriter.EncodeJSON(event)
	if err != nil {
		return types.MarketplaceEventRow{}, fmt.Errorf("encode payload json: %w", err)
	}

	var buyerZip *string
	var buyerLat, buyerLng *float64
	if addr := event.ShippingAddress; addr != nil {
		buyerZip = stringPtr(addr.PostalCode)
		buyerLat = float64Ptr(addr.Lat)
		buyerLng = float64Ptr(addr.Lng)
	}

	return types.MarketplaceEventRow{
		EventID:           envelope.EventID,
		EventType:         string(envelope.EventType),
		OccurredAt:        envelope.OccurredAt,
		CheckoutGroupID:   stringPtr(event.CheckoutGroupID),
		OrderID:           stringPtr(event.OrderID),
		BuyerStoreID:      stringPtr(event.BuyerStoreID),
		VendorStoreID:     stringPtr(event.VendorStoreID),
		BuyerZip:          buyerZip,
		BuyerLat:          buyerLat,
		BuyerLng:          buyerLng,
		SubtotalCents:     int64Ptr(event.SubtotalCents),
		DiscountsCents:    int64Ptr(event.DiscountsCents),
		TaxCents:          int64Ptr(event.TaxCents),
		TransportFeeCents: int64Ptr(event.TransportFeeCents),
		GrossRevenueCents: int64Ptr(event.TotalCents),
		RefundCents:       int64Ptr(0),
		NetRevenueCents:   int64Ptr(event.TotalCents),
		Items:             itemsJSON,
		Payload:           payloadJSON,
	}, nil
}
