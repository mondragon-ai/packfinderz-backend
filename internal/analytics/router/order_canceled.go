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
	row, err := buildTerminationRow(
		envelope,
		event.CanceledAt,
		event.OrderID.String(),
		event.BuyerStoreID.String(),
		event.VendorStoreID.String(),
		event,
	)
	if err != nil {
		h.logg.Error(logCtx, "failed to build termination row", err)
		return err
	}

	if err := h.writer.InsertMarketplace(logCtx, row); err != nil {
		h.logg.Error(logCtx, "failed to insert marketplace row", err)
		return err
	}

	h.logg.Info(logCtx, "order_canceled handler inserted marketplace row")
	return nil
}
