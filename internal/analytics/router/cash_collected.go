package router

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/payloads"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type cashCollectedHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newCashCollectedHandler(writer Writer, logg *logger.Logger) Handler {
	return &cashCollectedHandler{writer: writer, logg: logg}
}

func (h *cashCollectedHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*payloads.CashCollectedEvent)
	if !ok {
		return fmt.Errorf("invalid payload for cash_collected")
	}
	fields := map[string]any{
		"event_type":        envelope.EventType,
		"order_id":          event.OrderID,
		"amount_cents":      event.AmountCents,
		"cash_collected_at": event.CashCollectedAt,
	}
	logCtx := h.logg.WithFields(ctx, fields)
	h.logg.Info(logCtx, "cash_collected handler stub")
	return nil
}
