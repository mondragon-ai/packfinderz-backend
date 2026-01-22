package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// OutboxEvent represents an append-only event emitted via the outbox pattern.
type OutboxEvent struct {
	ID            uuid.UUID                 `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	EventType     enums.OutboxEventType     `gorm:"column:event_type;type:event_type_enum;not null"`
	AggregateType enums.OutboxAggregateType `gorm:"column:aggregate_type;type:aggregate_type_enum;not null"`
	AggregateID   uuid.UUID                 `gorm:"column:aggregate_id;type:uuid;not null"`
	Payload       json.RawMessage           `gorm:"column:payload;type:jsonb;not null"`
	CreatedAt     time.Time                 `gorm:"column:created_at;autoCreateTime"`
	PublishedAt   *time.Time                `gorm:"column:published_at"`
	AttemptCount  int                       `gorm:"column:attempt_count;not null;default:0"`
	LastError     *string                   `gorm:"column:last_error"`
}
