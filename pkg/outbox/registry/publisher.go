package registry

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/google/uuid"
)

// EventDescriptor links an event type to its aggregate/topic/payload schema.
type EventDescriptor struct {
	EventType      enums.OutboxEventType
	AggregateType  enums.OutboxAggregateType
	Topic          string
	PayloadFactory func() interface{}
}

// ResolvedEvent is the result of decoding an outbox row.
type ResolvedEvent struct {
	Descriptor EventDescriptor
	Envelope   outbox.PayloadEnvelope
	Payload    interface{}
}

// EventRegistry maps each supported event type to its descriptor.
type EventRegistry struct {
	entries map[enums.OutboxEventType]EventDescriptor
}

// NonRetryableError signals the dispatcher should stop retrying a row.
type NonRetryableError struct {
	Err error
}

// Error implements error.
func (e NonRetryableError) Error() string {
	if e.Err == nil {
		return "non-retryable error"
	}
	return e.Err.Error()
}

// Unwrap exposes the wrapped error.
func (e NonRetryableError) Unwrap() error {
	return e.Err
}

// NewEventRegistry builds the registry with the configured topic names.
func NewEventRegistry(cfg config.PubSubConfig) (*EventRegistry, error) {
	if cfg.OrdersTopic == "" {
		return nil, fmt.Errorf("orders topic is required")
	}
	if cfg.NotificationTopic == "" {
		return nil, fmt.Errorf("notification topic is required")
	}
	if cfg.BillingTopic == "" {
		return nil, fmt.Errorf("billing topic is required")
	}

	reg := &EventRegistry{entries: make(map[enums.OutboxEventType]EventDescriptor)}
	ordersTopic := cfg.OrdersTopic
	notificationTopic := cfg.NotificationTopic
	billingTopic := cfg.BillingTopic

	reg.register(EventDescriptor{
		EventType:      enums.EventOrderCreated,
		AggregateType:  enums.AggregateCheckoutGroup,
		Topic:          ordersTopic,
		PayloadFactory: func() interface{} { return &payloads.OrderCreatedEvent{} },
	})
	for _, desc := range []EventDescriptor{
		{
			EventType:      enums.EventOrderDecided,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.OrderDecisionEvent{} },
		},
		{
			EventType:      enums.EventOrderReadyForDispatch,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.OrderReadyForDispatchEvent{} },
		},
		{
			EventType:      enums.EventOrderCanceled,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.OrderCanceledEvent{} },
		},
		{
			EventType:      enums.EventCashCollected,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.CashCollectedEvent{} },
		},
		{
			EventType:      enums.EventPaymentFailed,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.PaymentStatusEvent{} },
		},
		{
			EventType:      enums.EventPaymentRejected,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.PaymentStatusEvent{} },
		},
		{
			EventType:      enums.EventOrderPendingNudge,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.OrderPendingNudgeEvent{} },
		},
		{
			EventType:      enums.EventOrderExpired,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.OrderExpiredEvent{} },
		},
		{
			EventType:      enums.EventOrderRetried,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          ordersTopic,
			PayloadFactory: func() interface{} { return &payloads.OrderRetriedEvent{} },
		},
	} {
		reg.register(desc)
	}
	for _, desc := range []EventDescriptor{
		{
			EventType:      enums.EventNotificationRequested,
			AggregateType:  enums.AggregateVendorOrder,
			Topic:          notificationTopic,
			PayloadFactory: func() interface{} { return &payloads.NotificationRequestedEvent{} },
		},
		{
			EventType:      enums.EventLicenseStatusChanged,
			AggregateType:  enums.AggregateLicense,
			Topic:          notificationTopic,
			PayloadFactory: func() interface{} { return &payloads.LicenseStatusChangedEvent{} },
		},
		{
			EventType:      enums.EventLicenseExpiringSoon,
			AggregateType:  enums.AggregateLicense,
			Topic:          notificationTopic,
			PayloadFactory: func() interface{} { return &payloads.LicenseExpiringSoonEvent{} },
		},
		{
			EventType:      enums.EventLicenseExpired,
			AggregateType:  enums.AggregateLicense,
			Topic:          notificationTopic,
			PayloadFactory: func() interface{} { return &payloads.LicenseExpiredEvent{} },
		},
		{
			EventType:      enums.EventCheckoutConverted,
			AggregateType:  enums.AggregateCheckoutGroup,
			Topic:          notificationTopic,
			PayloadFactory: func() interface{} { return &payloads.CheckoutConvertedEvent{} },
		},
	} {
		reg.register(desc)
	}
	reg.register(EventDescriptor{
		EventType:      enums.EventOrderPaid,
		AggregateType:  enums.AggregateVendorOrder,
		Topic:          billingTopic,
		PayloadFactory: func() interface{} { return &payloads.OrderPaidEvent{} },
	})

	return reg, nil
}

func (r *EventRegistry) register(desc EventDescriptor) {
	if desc.PayloadFactory == nil {
		return
	}
	r.entries[desc.EventType] = desc
}

// Resolve validates the row and decodes its typed payload.
func (r *EventRegistry) Resolve(event models.OutboxEvent) (*ResolvedEvent, error) {
	desc, ok := r.entries[event.EventType]
	if !ok {
		return nil, NewNonRetryableError(fmt.Errorf("unsupported event type %s", event.EventType))
	}
	if desc.AggregateType != event.AggregateType {
		return nil, NewNonRetryableError(fmt.Errorf("aggregate mismatch: expected %s got %s", desc.AggregateType, event.AggregateType))
	}
	if event.AggregateID == uuid.Nil {
		return nil, NewNonRetryableError(fmt.Errorf("missing aggregate_id"))
	}

	var envelope outbox.PayloadEnvelope
	if err := json.Unmarshal(event.Payload, &envelope); err != nil {
		return nil, NewNonRetryableError(fmt.Errorf("decode envelope: %w", err))
	}

	trimmed := bytes.TrimSpace(envelope.Data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, NewNonRetryableError(fmt.Errorf("payload missing for %s", event.EventType))
	}

	payload := desc.PayloadFactory()
	if payload == nil {
		return nil, NewNonRetryableError(fmt.Errorf("payload factory not configured for %s", event.EventType))
	}
	if err := json.Unmarshal(envelope.Data, payload); err != nil {
		return nil, NewNonRetryableError(fmt.Errorf("decode %s payload: %w", event.EventType, err))
	}

	return &ResolvedEvent{
		Descriptor: desc,
		Envelope:   envelope,
		Payload:    payload,
	}, nil
}

// NewNonRetryableError wraps an error to signal no retries.
func NewNonRetryableError(err error) NonRetryableError {
	return NonRetryableError{Err: err}
}
