package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
)

const analyticsConsumerName = "analytics"

// Handler defines how to process analytics envelopes.
type Handler interface {
	Handle(ctx context.Context, envelope types.Envelope) error
}

// HandlerFunc adapts functions to the Handler interface.
type HandlerFunc func(ctx context.Context, envelope types.Envelope) error

// Handle calls the underlying function.
func (fn HandlerFunc) Handle(ctx context.Context, envelope types.Envelope) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, envelope)
}

type idempotencyChecker interface {
	CheckAndMarkProcessed(ctx context.Context, consumer string, eventID uuid.UUID) (bool, error)
	Delete(ctx context.Context, consumer string, eventID uuid.UUID) error
}

// Service consumes analytics events from Pub/Sub while honoring Redis idempotency.
type Service struct {
	subscription *gcppubsub.Subscriber
	handler      Handler
	manager      idempotencyChecker
	logg         *logger.Logger
}

// NewService creates a new analytics worker service.
func NewService(subscription *gcppubsub.Subscriber, handler Handler, manager idempotencyChecker, logg *logger.Logger) (*Service, error) {
	if subscription == nil {
		return nil, errors.New("analytics subscription is required")
	}
	if handler == nil {
		return nil, errors.New("analytics handler is required")
	}
	if manager == nil {
		return nil, errors.New("idempotency manager is required")
	}
	if logg == nil {
		return nil, errors.New("logger is required")
	}

	return &Service{
		subscription: subscription,
		handler:      handler,
		manager:      manager,
		logg:         logg,
	}, nil
}

type processResult struct {
	nack bool
}

// Run starts consuming analytics messages until the context is canceled.
func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.subscription.Receive(ctx, func(innerCtx context.Context, msg *gcppubsub.Message) {
		if s.process(innerCtx, msg).nack {
			msg.Nack()
			return
		}
		msg.Ack()
	})
}

func (s *Service) process(ctx context.Context, msg *gcppubsub.Message) processResult {
	fields := s.buildBaseFields(msg)
	logCtx := s.logg.WithFields(ctx, fields)

	envelope, err := s.buildEnvelope(msg)
	if err != nil {
		fields["error"] = err.Error()
		s.logg.Warn(logCtx, "invalid analytics envelope")
		return processResult{}
	}
	fields["event_id"] = envelope.EventID
	fields["event_type"] = envelope.EventType
	fields["aggregate_type"] = envelope.AggregateType
	fields["aggregate_id"] = envelope.AggregateID
	fields["occurred_at"] = envelope.OccurredAt.Format(time.RFC3339Nano)
	logCtx = s.logg.WithFields(ctx, fields)

	eventID, err := uuid.Parse(envelope.EventID)
	if err != nil {
		s.logg.Warn(logCtx, "invalid event id")
		return processResult{}
	}

	already, err := s.manager.CheckAndMarkProcessed(logCtx, analyticsConsumerName, eventID)
	if err != nil {
		s.logg.Error(logCtx, "idempotency check failed", err)
		return processResult{nack: true}
	}
	if already {
		s.logg.Info(logCtx, "event already processed")
		return processResult{}
	}

	if err := s.handler.Handle(logCtx, *envelope); err != nil {
		s.logg.Error(logCtx, "handler error", err)
		_ = s.manager.Delete(logCtx, analyticsConsumerName, eventID)
		return processResult{nack: true}
	}

	s.logg.Info(logCtx, "analytics event handled")
	return processResult{}
}

func (s *Service) buildBaseFields(msg *gcppubsub.Message) map[string]any {
	fields := map[string]any{
		"message_id": msg.ID,
	}
	return fields
}

func (s *Service) buildEnvelope(msg *gcppubsub.Message) (*types.Envelope, error) {
	var stored outbox.PayloadEnvelope
	if err := json.Unmarshal(msg.Data, &stored); err != nil {
		return nil, fmt.Errorf("decode payload envelope: %w", err)
	}

	eventTypeStr := s.attribute(strings.TrimSpace(msg.Attributes["event_type"]))
	eventType, err := enums.ParseAnalyticsEventType(eventTypeStr)
	if err != nil {
		return nil, fmt.Errorf("event_type: %w", err)
	}

	aggregateTypeStr := s.attribute(strings.TrimSpace(msg.Attributes["aggregate_type"]))
	aggregateType, err := enums.ParseOutboxAggregateType(aggregateTypeStr)
	if err != nil {
		return nil, fmt.Errorf("aggregate_type: %w", err)
	}

	aggregateID := s.attribute(strings.TrimSpace(msg.Attributes["aggregate_id"]))
	if aggregateID == "" {
		return nil, errors.New("aggregate_id missing")
	}

	occurredAt := stored.OccurredAt
	if occurredAt.IsZero() {
		if created := s.attribute(strings.TrimSpace(msg.Attributes["created_at"])); created != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, created); err == nil {
				occurredAt = parsed
			}
		}
	}

	eventID := s.attribute(strings.TrimSpace(stored.EventID))
	if eventID == "" {
		eventID = s.attribute(strings.TrimSpace(msg.Attributes["event_id"]))
	}
	if eventID == "" {
		return nil, errors.New("event_id missing")
	}

	payload := stored.Data
	return &types.Envelope{
		EventID:       eventID,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		OccurredAt:    occurredAt.UTC(),
		Payload:       payload,
	}, nil
}

func (s *Service) attribute(value string) string {
	return strings.TrimSpace(value)
}
