package router

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
)

type orderCanceledHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newOrderCanceledHandler(writer Writer, logg *logger.Logger) Handler {
	return &orderCanceledHandler{writer: writer, logg: logg}
}

func (h *orderCanceledHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*payloads.OrderCanceledEvent)
	if !ok {
		return fmt.Errorf("invalid payload for order_canceled")
	}
	fields := map[string]any{
		"event_type":   envelope.EventType,
		"order_id":     event.OrderID,
		"vendor_store": event.VendorStoreID,
	}
	logCtx := h.logg.WithFields(ctx, fields)
	h.logg.Info(logCtx, "order_canceled handler stub")
	return nil
}
