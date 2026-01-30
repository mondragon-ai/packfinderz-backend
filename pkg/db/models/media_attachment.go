package models

import (
	"time"

	"github.com/google/uuid"
)

type MediaAttachment struct {
	ID         uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	MediaID    uuid.UUID `gorm:"column:media_id;type:uuid;not null"`
	EntityType string    `gorm:"column:entity_type;not null"` // protects license/ad usage; see ProtectedAttachmentEntities
	EntityID   uuid.UUID `gorm:"column:entity_id;type:uuid;not null"`
	StoreID    uuid.UUID `gorm:"column:store_id;type:uuid;not null"`
	GCSKey     string    `gorm:"column:gcs_key;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

var ProtectedAttachmentEntities = map[string]struct{}{
	AttachmentEntityLicense: {},
	AttachmentEntityAd:      {},
}

const (
	AttachmentEntityLicense = "license"
	AttachmentEntityAd      = "ad"
)
