package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// Media captures metadata for uploaded objects across the platform.
type Media struct {
	ID           uuid.UUID       `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID      *uuid.UUID      `gorm:"column:store_id;type:uuid"`
	UserID       *uuid.UUID      `gorm:"column:user_id;type:uuid"`
	Kind         enums.MediaKind `gorm:"column:kind;type:media_kind;not null"`
	URL          *string         `gorm:"column:url"`
	GSCKey       string          `gorm:"column:gsc_key;not null;unique"`
	FileName     string          `gorm:"column:file_name;not null"`
	MimeType     string          `gorm:"column:mime_type;not null"`
	OCR          *string         `gorm:"column:ocr"`
	SizeBytes    int64           `gorm:"column:size_bytes;not null"`
	IsCompressed bool            `gorm:"column:is_compressed;not null;default:false"`
	CreatedAt    time.Time       `gorm:"column:created_at;autoCreateTime"`
}
