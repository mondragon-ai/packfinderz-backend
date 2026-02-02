package router

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
)

type orderExpiredHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newOrderExpiredHandler(writer Writer, logg *logger.Logger) Handler {
	return &orderExpiredHandler{writer: writer, logg: logg}
}

func (h *orderExpiredHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*payloads.OrderExpiredEvent)
	if !ok {
		return fmt.Errorf("invalid payload for order_expired")
	}
	fields := map[string]any{
		"event_type": envelope.EventType,
		"order_id":   event.OrderID,
		"expired_at": event.ExpiredAt,
	}
	logCtx := h.logg.WithFields(ctx, fields)
	h.logg.Info(logCtx, "order_expired handler stub")
	return nil
}
