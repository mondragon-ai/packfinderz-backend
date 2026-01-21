package memberships

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// MembershipDTO is the transport shape for a raw membership record.
type MembershipDTO struct {
	ID              uuid.UUID              `json:"id"`
	StoreID         uuid.UUID              `json:"store_id"`
	UserID          uuid.UUID              `json:"user_id"`
	Role            enums.MemberRole       `json:"role"`
	Status          enums.MembershipStatus `json:"status"`
	InvitedByUserID *uuid.UUID             `json:"invited_by_user_id,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// MembershipWithStore includes basic store metadata + membership info.
type MembershipWithStore struct {
	MembershipID    uuid.UUID              `json:"membership_id"`
	StoreID         uuid.UUID              `json:"store_id"`
	UserID          uuid.UUID              `json:"user_id"`
	StoreName       string                 `json:"store_name"`
	StoreType       enums.StoreType        `json:"store_type"`
	Role            enums.MemberRole       `json:"role"`
	Status          enums.MembershipStatus `json:"status"`
	InvitedByUserID *uuid.UUID             `json:"invited_by_user_id,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// StoreUserDTO mixes membership metadata with the associated user profile for store admins.
type StoreUserDTO struct {
	MembershipID uuid.UUID              `json:"membership_id"`
	StoreID      uuid.UUID              `json:"store_id"`
	UserID       uuid.UUID              `json:"user_id"`
	Email        string                 `json:"email"`
	FirstName    string                 `json:"first_name"`
	LastName     string                 `json:"last_name"`
	Role         enums.MemberRole       `json:"role"`
	Status       enums.MembershipStatus `json:"membership_status"`
	CreatedAt    time.Time              `json:"created_at"`
	LastLoginAt  *time.Time             `json:"last_login_at,omitempty"`
}

// ToDTO converts a model to the external DTO.
func ToDTO(m *models.StoreMembership) *MembershipDTO {
	if m == nil {
		return nil
	}

	return &MembershipDTO{
		ID:              m.ID,
		StoreID:         m.StoreID,
		UserID:          m.UserID,
		Role:            m.Role,
		Status:          m.Status,
		InvitedByUserID: copyUUIDPointer(m.InvitedByUserID),
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func copyUUIDPointer(src *uuid.UUID) *uuid.UUID {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}
