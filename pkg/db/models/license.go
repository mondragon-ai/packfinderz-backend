package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// License captures compliance metadata tied to a stored media document.
type License struct {
	ID             uuid.UUID           `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID        uuid.UUID           `gorm:"column:store_id;type:uuid;not null"`
	UserID         uuid.UUID           `gorm:"column:user_id;type:uuid;not null"`
	Status         enums.LicenseStatus `gorm:"column:status;type:license_status;not null;default:'pending'"`
	MediaID        uuid.UUID           `gorm:"column:media_id;type:uuid;not null"`
	IssuingState   string              `gorm:"column:issuing_state;not null"`
	IssueDate      *time.Time          `gorm:"column:issue_date"`
	ExpirationDate *time.Time          `gorm:"column:expiration_date"`
	Type           enums.LicenseType   `gorm:"column:type;type:license_type;not null"`
	Number         string              `gorm:"column:number;not null;unique"`
	CreatedAt      time.Time           `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time           `gorm:"column:updated_at;autoUpdateTime"`
}
