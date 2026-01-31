package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// OutboxDLQ captures terminal outbox failures for auditing and remediation.
type OutboxDLQ struct {
	ID            uuid.UUID                  `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	EventID       uuid.UUID                  `gorm:"column:event_id;type:uuid;not null"`
	EventType     enums.OutboxEventType      `gorm:"column:event_type;type:event_type_enum;not null"`
	AggregateType enums.OutboxAggregateType  `gorm:"column:aggregate_type;type:aggregate_type_enum;not null"`
	AggregateID   uuid.UUID                  `gorm:"column:aggregate_id;type:uuid;not null"`
	Payload       json.RawMessage            `gorm:"column:payload_json;type:jsonb;not null"`
	ErrorReason   enums.OutboxDLQErrorReason `gorm:"column:error_reason;type:outbox_dlq_error_reason_enum;not null"`
	ErrorMessage  *string                    `gorm:"column:error_message"`
	AttemptCount  int                        `gorm:"column:attempt_count;not null;default:0"`
	FailedAt      time.Time                  `gorm:"column:failed_at;autoCreateTime"`
	CreatedAt     time.Time                  `gorm:"column:created_at;autoCreateTime"`
}
