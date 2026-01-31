package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/registry"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestServiceProcessBatchContinuesAfterFailure(t *testing.T) {
	repo := &fakeRepo{
		events: []models.OutboxEvent{
			{
				ID:            uuid.New(),
				EventType:     enums.EventOrderCreated,
				AggregateType: enums.AggregateCheckoutGroup,
				AggregateID:   uuid.New(),
				Payload:       mustEnvelopePayload(t, "event-one"),
			},
			{
				ID:            uuid.New(),
				EventType:     enums.EventOrderCreated,
				AggregateType: enums.AggregateCheckoutGroup,
				AggregateID:   uuid.New(),
				Payload:       mustEnvelopePayload(t, "event-two"),
			},
		},
	}
	pub := &fakePublisher{
		results: []publishResult{
			fakePublishResult{err: errors.New("transient")},
			fakePublishResult{},
		},
	}
	resolved := &registry.ResolvedEvent{
		Descriptor: registry.EventDescriptor{
			Topic:         "orders-topic",
			AggregateType: enums.AggregateCheckoutGroup,
		},
		Envelope: outbox.PayloadEnvelope{
			EventID:    uuid.NewString(),
			OccurredAt: time.Now(),
		},
		Payload: &payloads.OrderCreatedEvent{},
	}
	eventRegistry := &fakeRegistry{resolved: resolved}
	service := newTestService(t, repo, pub, eventRegistry)

	processed, err := service.processBatch(context.Background())
	if err != nil {
		t.Fatalf("process batch returned error: %v", err)
	}
	if !processed {
		t.Fatalf("expected batch to report processed")
	}
	if got := len(repo.failed); got != 1 {
		t.Fatalf("unexpected number of failed rows: %d", got)
	}
	if got := len(repo.published); got != 1 {
		t.Fatalf("unexpected number of published rows: %d", got)
	}
	if repo.failed[0] != repo.events[0].ID {
		t.Fatalf("failed row recorded wrong ID")
	}
	if repo.published[0] != repo.events[1].ID {
		t.Fatalf("published row recorded wrong ID")
	}
}

func newTestService(t *testing.T, repo outboxRepository, pub publisher, registry registryResolver) *Service {
	cfg := &config.Config{
		Outbox: config.OutboxConfig{
			BatchSize:      2,
			PollIntervalMS: 100,
			MaxAttempts:    5,
		},
	}
	logg := logger.New(logger.Options{
		ServiceName: "outbox-publisher-test",
		Output:      io.Discard,
	})
	service, err := NewService(ServiceParams{
		Config:           cfg,
		Logger:           logg,
		DB:               &fakeDB{},
		PubSub:           &fakePubSubClient{},
		Repository:       repo,
		Registry:         registry,
		PublisherFactory: func(_ string) publisher { return pub },
	})
	if err != nil {
		t.Fatalf("failed to construct service: %v", err)
	}
	return service
}

func mustEnvelopePayload(tb testing.TB, eventID string) json.RawMessage {
	tb.Helper()
	env := outbox.PayloadEnvelope{
		Version:    1,
		EventID:    eventID,
		OccurredAt: time.Now(),
		Data:       json.RawMessage(`{}`),
	}
	payload, err := json.Marshal(env)
	if err != nil {
		tb.Fatalf("marshal envelope: %v", err)
	}
	return payload
}

type fakeRepo struct {
	events    []models.OutboxEvent
	published []uuid.UUID
	failed    []uuid.UUID
}

func (f *fakeRepo) FetchUnpublishedForPublish(tx *gorm.DB, limit, maxAttempts int) ([]models.OutboxEvent, error) {
	return f.events, nil
}

func (f *fakeRepo) MarkPublishedTx(tx *gorm.DB, id uuid.UUID) error {
	f.published = append(f.published, id)
	return nil
}

func (f *fakeRepo) MarkFailedTx(tx *gorm.DB, id uuid.UUID, err error) error {
	f.failed = append(f.failed, id)
	return nil
}

func (f *fakeRepo) MarkTerminalTx(tx *gorm.DB, id uuid.UUID, err error, terminalAttempts int) error {
	f.failed = append(f.failed, id)
	return nil
}

type fakeDB struct{}

func (f *fakeDB) Ping(context.Context) error {
	return nil
}

func (f *fakeDB) WithTx(_ context.Context, fn func(*gorm.DB) error) error {
	return fn(nil)
}

type fakePubSubClient struct{}

func (f *fakePubSubClient) Ping(context.Context) error {
	return nil
}

func (f *fakePubSubClient) DomainPublisher() *gcppubsub.Publisher {
	return nil
}

func (f *fakePubSubClient) Publisher(name string) *gcppubsub.Publisher {
	return nil
}

type fakePublisher struct {
	results []publishResult
}

func (f *fakePublisher) Publish(context.Context, *gcppubsub.Message) publishResult {
	if len(f.results) == 0 {
		return nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result
}

type fakePublishResult struct {
	err error
}

func (f fakePublishResult) Get(context.Context) (string, error) {
	return "", f.err
}

type fakeRegistry struct {
	resolved *registry.ResolvedEvent
	err      error
}

func (f *fakeRegistry) Resolve(event models.OutboxEvent) (*registry.ResolvedEvent, error) {
	if f.resolved == nil {
		return nil, f.err
	}
	resolved := *f.resolved
	resolved.Descriptor.AggregateType = event.AggregateType
	resolved.Envelope.EventID = event.ID.String()
	resolved.Envelope.OccurredAt = time.Now()
	return &resolved, f.err
}
