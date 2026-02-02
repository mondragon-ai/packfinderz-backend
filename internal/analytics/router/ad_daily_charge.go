package router

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/payloads"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type adDailyChargeHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newAdDailyChargeHandler(writer Writer, logg *logger.Logger) Handler {
	return &adDailyChargeHandler{writer: writer, logg: logg}
}

func (h *adDailyChargeHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*payloads.AdDailyChargeRecordedEvent)
	if !ok {
		return fmt.Errorf("invalid payload for ad_daily_charge_recorded")
	}
	fields := map[string]any{
		"event_type": envelope.EventType,
		"ad_id":      event.AdID,
		"cost_cents": event.CostCents,
	}
	logCtx := h.logg.WithFields(ctx, fields)
	h.logg.Info(logCtx, "ad_daily_charge_recorded handler stub")
	return nil
}
