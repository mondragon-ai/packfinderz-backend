package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/router"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
)

func TestBuildEnvelope(t *testing.T) {
	svc := newTestService(t)
	payload := outbox.PayloadEnvelope{
		EventID:    "evt-1",
		OccurredAt: time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC),
		Data:       json.RawMessage(`{"order_id":"ord-1"}`),
	}
	msg := buildMessage(payload, map[string]string{
		"event_type":     "order_created",
		"aggregate_type": "vendor_order",
		"aggregate_id":   "ord-1",
	})

	env, err := svc.buildEnvelope(msg)
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	if env.EventType != enums.AnalyticsEventOrderCreated {
		t.Fatalf("unexpected event type %v", env.EventType)
	}
	if env.AggregateType != enums.AggregateVendorOrder {
		t.Fatalf("unexpected aggregate type %v", env.AggregateType)
	}
	if env.AggregateID != "ord-1" {
		t.Fatalf("unexpected aggregate id %s", env.AggregateID)
	}
	if env.EventID != "evt-1" {
		t.Fatalf("unexpected event id %s", env.EventID)
	}
	if env.OccurredAt != payload.OccurredAt {
		t.Fatalf("unexpected occurred at %v", env.OccurredAt)
	}
}

func TestProcessAlreadyProcessed(t *testing.T) {
	manager := &stubManager{checkResult: true}
	handler := &stubHandler{}
	svc := newTestServiceWithDeps(t, handler, manager)

	msg := buildAnalyticsMessage(t)
	res := svc.process(context.Background(), msg)
	if res.nack {
		t.Fatalf("expected ack, got nack")
	}
	if handler.called {
		t.Fatal("handler should not be invoked when already processed")
	}
	if len(manager.checked) != 1 {
		t.Fatalf("expected check once, got %d", len(manager.checked))
	}
}

func TestProcessHandlerErrorRetries(t *testing.T) {
	manager := &stubManager{}
	handler := &stubHandler{err: errors.New("boom")}
	svc := newTestServiceWithDeps(t, handler, manager)

	msg := buildAnalyticsMessage(t)
	res := svc.process(context.Background(), msg)
	if !res.nack {
		t.Fatalf("expected nack on handler error")
	}
	if !handler.called {
		t.Fatal("handler should be invoked")
	}
	if len(manager.deleted) != 1 {
		t.Fatalf("expected idempotency delete on failure")
	}
}

func TestProcessInvalidEnvelope(t *testing.T) {
	manager := &stubManager{}
	handler := &stubHandler{}
	svc := newTestServiceWithDeps(t, handler, manager)

	msg := &gcppubsub.Message{Data: []byte("invalid json")}
	res := svc.process(context.Background(), msg)
	if res.nack {
		t.Fatalf("invalid envelope should ack")
	}
	if handler.called {
		t.Fatal("handler should not be invoked")
	}
	if len(manager.checked) != 0 {
		t.Fatalf("idempotency manager should not be touched")
	}
}

func TestProcessUnsupportedEvent(t *testing.T) {
	manager := &stubManager{}
	handler := &stubHandler{err: router.ErrUnsupportedEventType}
	svc := newTestServiceWithDeps(t, handler, manager)

	msg := buildAnalyticsMessage(t)
	res := svc.process(context.Background(), msg)
	if res.nack {
		t.Fatalf("unsupported event should ack")
	}
	if len(manager.deleted) != 0 {
		t.Fatalf("idempotency delete should not run")
	}
}

func buildAnalyticsMessage(t *testing.T) *gcppubsub.Message {
	payload := outbox.PayloadEnvelope{
		EventID:    uuid.NewString(),
		OccurredAt: time.Now().UTC(),
		Data:       json.RawMessage(`{"foo":"bar"}`),
	}
	return buildMessage(payload, map[string]string{
		"event_type":     "order_created",
		"aggregate_type": "vendor_order",
		"aggregate_id":   "abc-123",
	})
}

func buildMessage(payload outbox.PayloadEnvelope, attrs map[string]string) *gcppubsub.Message {
	data, _ := json.Marshal(payload)
	return &gcppubsub.Message{
		ID:         "msg-1",
		Data:       data,
		Attributes: attrs,
	}
}

func newTestService(t *testing.T) *Service {
	return newTestServiceWithDeps(t, &stubHandler{}, &stubManager{})
}

func newTestServiceWithDeps(t *testing.T, handler Handler, manager *stubManager) *Service {
	t.Helper()
	return &Service{
		handler: handler,
		manager: manager,
		logg:    logger.New(logger.Options{ServiceName: "analytics-test"}),
	}
}

type stubHandler struct {
	called   bool
	envelope types.Envelope
	err      error
}

func (h *stubHandler) Handle(ctx context.Context, envelope types.Envelope) error {
	h.called = true
	h.envelope = envelope
	return h.err
}

type stubManager struct {
	checkResult bool
	checkErr    error
	deleteErr   error
	checked     []uuid.UUID
	deleted     []uuid.UUID
}

func (s *stubManager) CheckAndMarkProcessed(ctx context.Context, consumer string, eventID uuid.UUID) (bool, error) {
	s.checked = append(s.checked, eventID)
	return s.checkResult, s.checkErr
}

func (s *stubManager) Delete(ctx context.Context, consumer string, eventID uuid.UUID) error {
	s.deleted = append(s.deleted, eventID)
	return s.deleteErr
}
