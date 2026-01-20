package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// StoreMembership links a user with a store and captures their role/status.
type StoreMembership struct {
	ID              uuid.UUID              `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID         uuid.UUID              `gorm:"column:store_id;type:uuid;not null"`
	UserID          uuid.UUID              `gorm:"column:user_id;type:uuid;not null"`
	Role            enums.MemberRole       `gorm:"column:role;type:member_role;not null"`
	Status          enums.MembershipStatus `gorm:"column:status;type:membership_status;not null"`
	InvitedByUserID *uuid.UUID             `gorm:"column:invited_by_user_id;type:uuid"`
	CreatedAt       time.Time              `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time              `gorm:"column:updated_at;autoUpdateTime"`
}
