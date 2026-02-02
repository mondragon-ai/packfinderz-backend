package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	analyticspayloads "github.com/angelmondragon/packfinderz-backend/internal/analytics/payloads"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	outboxpayloads "github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
)

var ErrUnsupportedEventType = errors.New("unsupported analytics event type")

// Writer delivers BigQuery rows produced by analytics handlers.
type Writer interface {
	InsertMarketplace(ctx context.Context, row types.MarketplaceEventRow) error
	InsertAdFact(ctx context.Context, row types.AdEventFactRow) error
}

// Handler receives an envelope plus a decoded event payload.
type Handler interface {
	Handle(ctx context.Context, envelope types.Envelope, payload any) error
}

type handlerEntry struct {
	factory func() any
	handler Handler
}

// Router dispatches analytics envelopes to the configured handler per event type.
type Router struct {
	handlers map[enums.AnalyticsEventType]handlerEntry
	logg     *logger.Logger
}

// NewRouter wires the default handlers and allows overrides for specific events.
func NewRouter(writer Writer, logg *logger.Logger, overrides map[enums.AnalyticsEventType]Handler) (*Router, error) {
	if writer == nil {
		return nil, errors.New("writer is required")
	}
	if logg == nil {
		return nil, errors.New("logger is required")
	}

	entries := map[enums.AnalyticsEventType]handlerEntry{
		enums.AnalyticsEventOrderCreated: {
			factory: func() any { return &analyticspayloads.OrderCreatedEvent{} },
			handler: newOrderCreatedHandler(writer, logg),
		},
		enums.AnalyticsEventOrderPaid: {
			factory: func() any { return &outboxpayloads.OrderPaidEvent{} },
			handler: newOrderPaidHandler(writer, logg),
		},
		enums.AnalyticsEventCashCollected: {
			factory: func() any { return &analyticspayloads.CashCollectedEvent{} },
			handler: newCashCollectedHandler(writer, logg),
		},
		enums.AnalyticsEventOrderCanceled: {
			factory: func() any { return &outboxpayloads.OrderCanceledEvent{} },
			handler: newOrderCanceledHandler(writer, logg),
		},
		enums.AnalyticsEventOrderExpired: {
			factory: func() any { return &outboxpayloads.OrderExpiredEvent{} },
			handler: newOrderExpiredHandler(writer, logg),
		},
		enums.AnalyticsEventAdImpression: {
			factory: func() any { return &analyticspayloads.AdImpressionEvent{} },
			handler: newAdImpressionHandler(writer, logg),
		},
		enums.AnalyticsEventAdClick: {
			factory: func() any { return &analyticspayloads.AdClickEvent{} },
			handler: newAdClickHandler(writer, logg),
		},
		enums.AnalyticsEventAdDailyChargeRecorded: {
			factory: func() any { return &analyticspayloads.AdDailyChargeRecordedEvent{} },
			handler: newAdDailyChargeHandler(writer, logg),
		},
	}

	for event, custom := range overrides {
		entry, ok := entries[event]
		if !ok || custom == nil {
			continue
		}
		entry.handler = custom
		entries[event] = entry
	}

	return &Router{
		handlers: entries,
		logg:     logg,
	}, nil
}

// Handle dispatches the incoming envelope to the configured handler.
func (r *Router) Handle(ctx context.Context, envelope types.Envelope) error {
	entry, ok := r.handlers[envelope.EventType]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedEventType, envelope.EventType)
	}
	payload := entry.factory()
	if len(envelope.Payload) == 0 {
		return fmt.Errorf("empty payload for %s", envelope.EventType)
	}
	if err := json.Unmarshal(envelope.Payload, payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", envelope.EventType, err)
	}

	return entry.handler.Handle(ctx, envelope, payload)
}
