package router

import (
	"context"
	"fmt"

	analyticspayloads "github.com/angelmondragon/packfinderz-backend/internal/analytics/payloads"
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
	event, ok := payload.(*analyticspayloads.CashCollectedEvent)
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

	row, err := buildRevenueRow(
		envelope,
		int64(event.AmountCents),
		event.OrderID,
		event.BuyerStoreID,
		event.VendorStoreID,
		event.CashCollectedAt,
		event,
	)
	if err != nil {
		h.logg.Error(logCtx, "failed to build revenue row", err)
		return err
	}

	if err := h.writer.InsertMarketplace(logCtx, row); err != nil {
		h.logg.Error(logCtx, "failed to insert marketplace row", err)
		return err
	}

	h.logg.Info(logCtx, "cash_collected handler inserted marketplace row")
	return nil
}
