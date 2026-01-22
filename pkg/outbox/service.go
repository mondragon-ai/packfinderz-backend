package outbox

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
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
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Emit(tx *gorm.DB, event DomainEvent) error {
	if tx == nil {
		return errors.New("transaction required")
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
	return s.repo.Insert(tx, row)
}
