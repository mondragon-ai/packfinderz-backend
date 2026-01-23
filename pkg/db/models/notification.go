package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// Notification stores in-app notification payloads scoped to stores.
type Notification struct {
	ID        uuid.UUID              `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID   uuid.UUID              `gorm:"type:uuid;not null"`
	Type      enums.NotificationType `gorm:"type:notification_type;not null"`
	Title     string                 `gorm:"type:text;not null"`
	Message   string                 `gorm:"type:text;not null"`
	Link      *string                `gorm:"type:text"`
	ReadAt    *time.Time             `gorm:"type:timestamptz"`
	CreatedAt time.Time              `gorm:"type:timestamptz;default:now()"`
}
