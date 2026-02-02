package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/registry"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultBatchSize      = 50
	defaultPollMs         = 500
	defaultPublishTimeout = 15 * time.Second
	defaultMaxAttempts    = 10
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
	Publisher(name string) *gcppubsub.Publisher
}

type outboxRepository interface {
	FetchUnpublishedForPublish(tx *gorm.DB, limit, maxAttempts int) ([]models.OutboxEvent, error)
	MarkPublishedTx(tx *gorm.DB, id uuid.UUID) error
	MarkFailedTx(tx *gorm.DB, id uuid.UUID, err error) error
	MarkTerminalTx(tx *gorm.DB, id uuid.UUID, err error, terminalAttempts int) error
}

type dlqRepository interface {
	InsertTx(tx *gorm.DB, entry models.OutboxDLQ) error
}

type registryResolver interface {
	Resolve(models.OutboxEvent) (*registry.ResolvedEvent, error)
}

type publisherFactory func(topic string) publisher

type publisher interface {
	Publish(context.Context, *gcppubsub.Message) publishResult
}

type publishResult interface {
	Get(context.Context) (string, error)
}

type ServiceParams struct {
	Config           *config.Config
	Logger           *logger.Logger
	DB               dbClient
	PubSub           pubSubClient
	Repository       outboxRepository
	Registry         registryResolver
	PublisherFactory publisherFactory
	DLQRepository    dlqRepository
}

type Service struct {
	cfg              *config.Config
	logg             *logger.Logger
	db               dbClient
	repo             outboxRepository
	pubsub           pubSubClient
	registry         registryResolver
	dlq              dlqRepository
	publisherFactory publisherFactory
	batchSize        int
	maxAttempts      int
	pollInterval     time.Duration
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
	if params.Registry == nil {
		return nil, errors.New("event registry is required")
	}
	if params.DLQRepository == nil {
		return nil, errors.New("dlq repository is required")
	}

	factory := params.PublisherFactory
	if factory == nil {
		factory = func(topic string) publisher {
			publisher := params.PubSub.Publisher(topic)
			if publisher == nil {
				return nil
			}
			return newGCPPubPublisher(publisher)
		}
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
		maxAttempts = defaultMaxAttempts
	}

	return &Service{
		cfg:              params.Config,
		logg:             params.Logger,
		db:               params.DB,
		repo:             params.Repository,
		pubsub:           params.PubSub,
		registry:         params.Registry,
		dlq:              params.DLQRepository,
		publisherFactory: factory,
		batchSize:        batch,
		maxAttempts:      maxAttempts,
		pollInterval:     time.Duration(pollMs) * time.Millisecond,
	}, nil
}

func (s *Service) ensureReadiness(ctx context.Context) error {
	if err := pingDependency(ctx, s.logg, "database", s.db.Ping); err != nil {
		return err
	}
	if err := pingDependency(ctx, s.logg, "pubsub", s.pubsub.Ping); err != nil {
		return err
	}
	// s.logg.Info(ctx, "all outbox dependencies are ready")
	return nil
}

func pingDependency(ctx context.Context, logg *logger.Logger, name string, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		logg.Error(ctx, fmt.Sprintf("%s ping failed", name), err)
		return fmt.Errorf("%s ping failed: %w", name, err)
	}
	// logg.Info(ctx, fmt.Sprintf("%s ping succeeded", name))
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
			resolved, err := s.registry.Resolve(event)
			if err != nil {
				if markErr := s.handleTerminal(ctx, tx, event, enums.OutboxDLQReasonNonRetryable, err, "", nil); markErr != nil {
					return markErr
				}
				continue
			}

			fields := s.eventFields(event, resolved.Envelope, resolved.Descriptor.Topic)
			if err := s.publishResolved(ctx, event, resolved); err != nil {
				var nonRetry registry.NonRetryableError
				if errors.As(err, &nonRetry) {
					if markErr := s.handleTerminal(ctx, tx, event, enums.OutboxDLQReasonNonRetryable, err, resolved.Descriptor.Topic, fields); markErr != nil {
						return markErr
					}
					continue
				}

				nextAttempt := event.AttemptCount + 1
				fields["attempt_count"] = nextAttempt

				if nextAttempt >= s.maxAttempts {
					fields["terminal_reason"] = "max_attempts"
					terminalErr := fmt.Errorf("max publish attempts reached: %w", err)
					if markErr := s.handleTerminal(ctx, tx, event, enums.OutboxDLQReasonMaxAttempts, terminalErr, resolved.Descriptor.Topic, fields); markErr != nil {
						return markErr
					}
					continue
				}

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

func (s *Service) handleTerminal(ctx context.Context, tx *gorm.DB, event models.OutboxEvent, reason enums.OutboxDLQErrorReason, err error, topic string, fields map[string]any) error {
	if fields == nil {
		fields = s.eventFields(event, outbox.PayloadEnvelope{}, topic)
	}
	fields["error_reason"] = reason
	ctxWithFields := s.logg.WithFields(ctx, fields)
	ctxWithFields = s.logg.WithField(ctxWithFields, "error", err.Error())
	s.logg.Warn(ctxWithFields, "outbox event will not be retried")

	dlqEntry := models.OutboxDLQ{
		EventID:       event.ID,
		EventType:     event.EventType,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		Payload:       event.Payload,
		ErrorReason:   reason,
		ErrorMessage:  dlqErrorMessage(err),
		AttemptCount:  event.AttemptCount,
		FailedAt:      time.Now().UTC(),
	}
	if dlqErr := s.dlq.InsertTx(tx, dlqEntry); dlqErr != nil {
		return fmt.Errorf("insert dlq %s: %w", event.ID, dlqErr)
	}
	if markErr := s.repo.MarkTerminalTx(tx, event.ID, err, s.maxAttempts); markErr != nil {
		return fmt.Errorf("mark terminal %s: %w", event.ID, markErr)
	}
	return nil
}

func dlqErrorMessage(err error) *string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	return &msg
}

func (s *Service) publishResolved(ctx context.Context, event models.OutboxEvent, resolved *registry.ResolvedEvent) error {
	topic := resolved.Descriptor.Topic
	pub := s.publisherFactory(topic)
	if pub == nil {
		return registry.NewNonRetryableError(fmt.Errorf("publisher not configured for topic %s", topic))
	}

	msg := &gcppubsub.Message{
		Data: event.Payload,
		Attributes: map[string]string{
			"event_id":       resolved.Envelope.EventID,
			"event_type":     string(event.EventType),
			"aggregate_type": string(event.AggregateType),
			"aggregate_id":   event.AggregateID.String(),
			"created_at":     event.CreatedAt.Format(time.RFC3339Nano),
		},
	}

	publishCtx, cancel := context.WithTimeout(ctx, defaultPublishTimeout)
	defer cancel()
	result := pub.Publish(publishCtx, msg)
	if result == nil {
		return registry.NewNonRetryableError(fmt.Errorf("publisher returned nil for topic %s", topic))
	}
	if _, err := result.Get(publishCtx); err != nil {
		return err
	}
	return nil
}

func (s *Service) eventFields(event models.OutboxEvent, envelope outbox.PayloadEnvelope, topic string) map[string]any {
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
	if topic != "" {
		fields["topic"] = topic
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
