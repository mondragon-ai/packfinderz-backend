package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	dbpkg "github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type DomainEvent struct {
	EventType     enums.OutboxEventType
	AggregateType enums.OutboxAggregateType
	AggregateID   uuid.UUID
	Actor         *ActorRef
	Data          interface{}
	Version       int
	OccurredAt    time.Time
}

type Service struct {
	repo *Repository
	logg *logger.Logger
}

func NewService(repo *Repository, logg *logger.Logger) *Service {
	return &Service{repo: repo, logg: logg}
}

func (s *Service) Emit(ctx context.Context, tx *gorm.DB, event DomainEvent) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}
	envelope := PayloadEnvelope{
		Version:    event.Version,
		EventID:    uuid.NewString(),
		OccurredAt: event.OccurredAt,
		Actor:      event.Actor,
		Data:       payload,
	}
	payloadJSON, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	row := models.OutboxEvent{
		EventType:     event.EventType,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		Payload:       json.RawMessage(payloadJSON),
	}
	if err := s.repo.Insert(tx, row); err != nil {
		return err
	}
	if s.logg != nil {
		fields := map[string]any{
			"event_id":       envelope.EventID,
			"event_type":     event.EventType,
			"aggregate_id":   event.AggregateID.String(),
			"aggregate_type": event.AggregateType,
		}
		logCtx := s.logg.WithFields(ctx, fields)
		s.logg.Info(logCtx, "outbox event queued")
	}
	return nil
}

func (s *Service) EmitIfNotExists(ctx context.Context, tx *gorm.DB, event DomainEvent) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	exists, err := s.repo.ExistsTx(tx, event.EventType, event.AggregateType, event.AggregateID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := s.Emit(ctx, tx, event); err != nil {
		if dbpkg.IsUniqueViolation(err, "ux_outbox_events_event_aggregate") {
			return nil
		}
		return err
	}
	return nil
}
