package models

import (
	"time"

	"github.com/google/uuid"
)

// ProductMedia stores ordered media entries for products.
type ProductMedia struct {
	ID        uuid.UUID  `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	ProductID uuid.UUID  `gorm:"column:product_id;type:uuid;not null"`
	URL       *string    `gorm:"column:url"`
	GCSKey    string     `gorm:"column:gcs_key;not null"`
	MediaID   *uuid.UUID `gorm:"column:media_id;type:uuid"`
	Position  int        `gorm:"column:position;not null;default:0"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}
