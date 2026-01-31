package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultBatchSize      = 50
	defaultPollMs         = 500
	defaultPublishTimeout = 15 * time.Second
	maxBackoff            = 10 * time.Second
	jitterWindow          = 250 * time.Millisecond
)

var jitterSource = rand.New(rand.NewSource(time.Now().UnixNano()))

type dbClient interface {
	Ping(context.Context) error
	WithTx(context.Context, func(tx *gorm.DB) error) error
}

type pubSubClient interface {
	Ping(context.Context) error
	DomainPublisher() *gcppubsub.Publisher
}

type outboxRepository interface {
	FetchUnpublishedForPublish(tx *gorm.DB, limit, maxAttempts int) ([]models.OutboxEvent, error)
	MarkPublishedTx(tx *gorm.DB, id uuid.UUID) error
	MarkFailedTx(tx *gorm.DB, id uuid.UUID, err error) error
}

type publisher interface {
	Publish(context.Context, *gcppubsub.Message) publishResult
}

type publishResult interface {
	Get(context.Context) (string, error)
}

type ServiceParams struct {
	Config     *config.Config
	Logger     *logger.Logger
	DB         dbClient
	PubSub     pubSubClient
	Repository outboxRepository
	Publisher  publisher
}

type Service struct {
	cfg          *config.Config
	logg         *logger.Logger
	db           dbClient
	repo         outboxRepository
	pubsub       pubSubClient
	publisher    publisher
	batchSize    int
	maxAttempts  int
	pollInterval time.Duration
}

func NewService(params ServiceParams) (*Service, error) {
	if params.Config == nil {
		return nil, errors.New("config is required")
	}
	if params.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if params.DB == nil {
		return nil, errors.New("database client is required")
	}
	if params.PubSub == nil {
		return nil, errors.New("pubsub client is required")
	}
	if params.Repository == nil {
		return nil, errors.New("outbox repository is required")
	}

	var publisher publisher
	if params.Publisher != nil {
		publisher = params.Publisher
	} else {
		domainPublisher := params.PubSub.DomainPublisher()
		if domainPublisher == nil {
			return nil, errors.New("domain pubsub topic is not configured")
		}
		publisher = newGCPPubPublisher(domainPublisher)
	}

	batch := params.Config.Outbox.BatchSize
	if batch <= 0 {
		batch = defaultBatchSize
	}
	pollMs := params.Config.Outbox.PollIntervalMS
	if pollMs <= 0 {
		pollMs = defaultPollMs
	}
	maxAttempts := params.Config.Outbox.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = math.MaxInt32
	}

	return &Service{
		cfg:          params.Config,
		logg:         params.Logger,
		db:           params.DB,
		repo:         params.Repository,
		pubsub:       params.PubSub,
		publisher:    publisher,
		batchSize:    batch,
		maxAttempts:  maxAttempts,
		pollInterval: time.Duration(pollMs) * time.Millisecond,
	}, nil
}

func (s *Service) ensureReadiness(ctx context.Context) error {
	if err := pingDependency(ctx, s.logg, "database", s.db.Ping); err != nil {
		return err
	}
	if err := pingDependency(ctx, s.logg, "pubsub", s.pubsub.Ping); err != nil {
		return err
	}
	s.logg.Info(ctx, "all outbox dependencies are ready")
	return nil
}

func pingDependency(ctx context.Context, logg *logger.Logger, name string, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		logg.Error(ctx, fmt.Sprintf("%s ping failed", name), err)
		return fmt.Errorf("%s ping failed: %w", name, err)
	}
	logg.Info(ctx, fmt.Sprintf("%s ping succeeded", name))
	return nil
}

func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.ensureReadiness(ctx); err != nil {
		return err
	}

	interval := s.pollInterval
	if interval <= 0 {
		interval = time.Duration(defaultPollMs) * time.Millisecond
	}
	backoff := interval

	for {
		select {
		case <-ctx.Done():
			s.logg.Info(ctx, "outbox publisher context canceled")
			return ctx.Err()
		default:
		}

		processed, err := s.processBatch(ctx)
		if err != nil {
			s.logg.Error(ctx, "outbox publisher batch error", err)
			backoff = nextBackoff(backoff, interval, maxBackoff)
			if err := s.sleep(ctx, withJitter(backoff)); err != nil {
				return err
			}
			continue
		}

		backoff = interval

		if processed {
			continue
		}

		if err := s.sleep(ctx, withJitter(interval)); err != nil {
			return err
		}
	}
}

func (s *Service) processBatch(ctx context.Context) (bool, error) {
	processed := false
	err := s.db.WithTx(ctx, func(tx *gorm.DB) error {
		events, err := s.repo.FetchUnpublishedForPublish(tx, s.batchSize, s.maxAttempts)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}

		processed = true
		for _, event := range events {
			envelope, err := s.publishRow(ctx, event)
			fields := s.eventFields(event, envelope)
			if err != nil {
				fields["attempt_count"] = event.AttemptCount + 1
				ctxWithFields := s.logg.WithFields(ctx, fields)
				ctxWithFields = s.logg.WithField(ctxWithFields, "error", err.Error())
				s.logg.Warn(ctxWithFields, "outbox publish failed")
				if markErr := s.repo.MarkFailedTx(tx, event.ID, err); markErr != nil {
					return fmt.Errorf("mark failure %s: %w", event.ID, markErr)
				}
				continue
			}

			if markErr := s.repo.MarkPublishedTx(tx, event.ID); markErr != nil {
				return fmt.Errorf("mark published %s: %w", event.ID, markErr)
			}
			s.logg.Info(s.logg.WithFields(ctx, fields), "outbox event published")
		}
		return nil
	})
	return processed, err
}

func (s *Service) publishRow(ctx context.Context, event models.OutboxEvent) (outbox.PayloadEnvelope, error) {
	var envelope outbox.PayloadEnvelope
	if err := json.Unmarshal(event.Payload, &envelope); err != nil {
		return envelope, fmt.Errorf("decode envelope: %w", err)
	}

	msg := &gcppubsub.Message{
		Data: event.Payload,
		Attributes: map[string]string{
			"event_id":       envelope.EventID,
			"event_type":     string(event.EventType),
			"aggregate_type": string(event.AggregateType),
			"aggregate_id":   event.AggregateID.String(),
			"created_at":     event.CreatedAt.Format(time.RFC3339Nano),
		},
	}

	publishCtx, cancel := context.WithTimeout(ctx, defaultPublishTimeout)
	defer cancel()
	if s.publisher == nil {
		return envelope, errors.New("publisher is not configured")
	}
	result := s.publisher.Publish(publishCtx, msg)
	if result == nil {
		return envelope, errors.New("publisher did not return a result")
	}
	if _, err := result.Get(publishCtx); err != nil {
		return envelope, fmt.Errorf("publish to topic: %w", err)
	}
	return envelope, nil
}

func (s *Service) eventFields(event models.OutboxEvent, envelope outbox.PayloadEnvelope) map[string]any {
	fields := map[string]any{
		"outbox_id":      event.ID.String(),
		"event_type":     event.EventType,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID.String(),
		"batch_size":     s.batchSize,
		"attempt_count":  event.AttemptCount,
	}
	if envelope.EventID != "" {
		fields["event_id"] = envelope.EventID
		fields["occurred_at"] = envelope.OccurredAt.Format(time.RFC3339Nano)
	}
	if event.LastError != nil {
		fields["last_error"] = *event.LastError
	}
	return fields
}

func (s *Service) sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current, base, max time.Duration) time.Duration {
	if current <= 0 {
		current = base
	}
	next := current * 2
	if next > max {
		return max
	}
	return next
}

func withJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	jitter := time.Duration(jitterSource.Int63n(int64(jitterWindow)))
	return d + jitter
}

func newGCPPubPublisher(p *gcppubsub.Publisher) publisher {
	if p == nil {
		return nil
	}
	return &gcpPublisher{Publisher: p}
}

type gcpPublisher struct {
	*gcppubsub.Publisher
}

func (p *gcpPublisher) Publish(ctx context.Context, msg *gcppubsub.Message) publishResult {
	if p == nil || p.Publisher == nil {
		return nil
	}
	return &gcpPublishResult{PublishResult: p.Publisher.Publish(ctx, msg)}
}

type gcpPublishResult struct {
	*gcppubsub.PublishResult
}

func (r *gcpPublishResult) Get(ctx context.Context) (string, error) {
	if r == nil || r.PublishResult == nil {
		return "", errors.New("publish result is nil")
	}
	return r.PublishResult.Get(ctx)
}
