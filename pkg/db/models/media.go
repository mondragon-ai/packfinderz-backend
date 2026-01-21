package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// Media captures metadata for uploaded objects across the platform.
type Media struct {
	ID                  uuid.UUID         `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID             uuid.UUID         `gorm:"column:store_id;type:uuid;not null"`
	UserID              uuid.UUID         `gorm:"column:user_id;type:uuid;not null"`
	Kind                enums.MediaKind   `gorm:"column:kind;type:media_kind;not null"`
	Status              enums.MediaStatus `gorm:"column:status;type:media_status;not null;default:'pending'"`
	GCSKey              string            `gorm:"column:gcs_key;not null;unique"`
	FileName            string            `gorm:"column:file_name;not null"`
	MimeType            string            `gorm:"column:mime_type;not null"`
	OCR                 *string           `gorm:"column:ocr"`
	SizeBytes           int64             `gorm:"column:size_bytes;not null"`
	IsCompressed        bool              `gorm:"column:is_compressed;not null;default:false"`
	CreatedAt           time.Time         `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time         `gorm:"column:updated_at;autoUpdateTime"`
	UploadedAt          *time.Time        `gorm:"column:uploaded_at"`
	VerifiedAt          *time.Time        `gorm:"column:verified_at"`
	ProcessingStartedAt *time.Time        `gorm:"column:processing_started_at"`
	ReadyAt             *time.Time        `gorm:"column:ready_at"`
	FailedAt            *time.Time        `gorm:"column:failed_at"`
	DeletedAt           *time.Time        `gorm:"column:deleted_at"`
}
