package router

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
)

type orderCreatedHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newOrderCreatedHandler(writer Writer, logg *logger.Logger) Handler {
	return &orderCreatedHandler{writer: writer, logg: logg}
}

func (h *orderCreatedHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*payloads.OrderCreatedEvent)
	if !ok {
		return fmt.Errorf("invalid payload for order_created")
	}
	fields := map[string]any{
		"event_type":       envelope.EventType,
		"checkout_group":   event.CheckoutGroupID,
		"vendor_order_ids": event.VendorOrderIDs,
	}
	logCtx := h.logg.WithFields(ctx, fields)
	h.logg.Info(logCtx, "order_created handler stub")
	return nil
}
