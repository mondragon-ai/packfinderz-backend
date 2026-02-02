package router

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
)

type orderPaidHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newOrderPaidHandler(writer Writer, logg *logger.Logger) Handler {
	return &orderPaidHandler{writer: writer, logg: logg}
}

func (h *orderPaidHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*payloads.OrderPaidEvent)
	if !ok {
		return fmt.Errorf("invalid payload for order_paid")
	}
	fields := map[string]any{
		"event_type":     envelope.EventType,
		"order_id":       event.OrderID,
		"amount_cents":   event.AmountCents,
		"vendor_paid_at": event.VendorPaidAt,
	}
	logCtx := h.logg.WithFields(ctx, fields)
	h.logg.Info(logCtx, "order_paid handler stub")
	return nil
}
