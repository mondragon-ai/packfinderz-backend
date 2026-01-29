package models

import (
	"time"

	dbtypes "github.com/angelmondragon/packfinderz-backend/pkg/db/types"
	"github.com/google/uuid"
)

// User represents the canonical identity entity.
type User struct {
	ID           uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email        string            `gorm:"type:text;not null;uniqueIndex"`
	PasswordHash string            `gorm:"column:password_hash;not null"`
	FirstName    string            `gorm:"column:first_name;not null"`
	LastName     string            `gorm:"column:last_name;not null"`
	Phone        *string           `gorm:"column:phone"`
	IsActive     bool              `gorm:"column:is_active;not null;default:true"`
	LastLoginAt  *time.Time        `gorm:"column:last_login_at"`
	SystemRole   *string           `gorm:"column:system_role"`
	StoreIDs     dbtypes.UUIDArray `gorm:"type:uuid[];column:store_ids;not null;default:ARRAY[]::uuid[]"`
	CreatedAt    time.Time         `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time         `gorm:"column:updated_at;autoUpdateTime"`
}
