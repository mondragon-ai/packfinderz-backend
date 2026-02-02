package router

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/payloads"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type adImpressionHandler struct {
	writer Writer
	logg   *logger.Logger
}

func newAdImpressionHandler(writer Writer, logg *logger.Logger) Handler {
	return &adImpressionHandler{writer: writer, logg: logg}
}

func (h *adImpressionHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	event, ok := payload.(*payloads.AdImpressionEvent)
	if !ok {
		return fmt.Errorf("invalid payload for ad_impression")
	}
	fields := map[string]any{
		"event_type": envelope.EventType,
		"ad_id":      event.AdID,
	}
	logCtx := h.logg.WithFields(ctx, fields)
	h.logg.Info(logCtx, "ad_impression handler stub")
	return nil
}
