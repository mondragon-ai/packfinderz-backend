package users

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	dbtypes "github.com/angelmondragon/packfinderz-backend/pkg/db/types"
)

// UserDTO is the transport shape that omits sensitive credentials.
type UserDTO struct {
	ID          uuid.UUID   `json:"id"`
	Email       string      `json:"email"`
	FirstName   string      `json:"first_name"`
	LastName    string      `json:"last_name"`
	Phone       *string     `json:"phone,omitempty"`
	IsActive    bool        `json:"is_active"`
	LastLoginAt *time.Time  `json:"last_login_at,omitempty"`
	SystemRole  *string     `json:"system_role,omitempty"`
	StoreIDs    []uuid.UUID `json:"store_ids"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// CreateUserDTO holds the data required by the repo to persist a new user.
type CreateUserDTO struct {
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	Phone        *string
	SystemRole   *string
	StoreIDs     []uuid.UUID
	IsActive     *bool
}

func FromModel(u *models.User) *UserDTO {
	if u == nil {
		return nil
	}

	return &UserDTO{
		ID:          u.ID,
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Phone:       u.Phone,
		IsActive:    u.IsActive,
		LastLoginAt: u.LastLoginAt,
		SystemRole:  u.SystemRole,
		StoreIDs:    append([]uuid.UUID(nil), []uuid.UUID(u.StoreIDs)...),
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func (c CreateUserDTO) ToModel() *models.User {
	isActive := true
	if c.IsActive != nil {
		isActive = *c.IsActive
	}

	storeIDs := c.StoreIDs
	if storeIDs == nil {
		storeIDs = []uuid.UUID{}
	} else {
		storeIDs = append([]uuid.UUID(nil), storeIDs...)
	}

	return &models.User{
		Email:        c.Email,
		PasswordHash: c.PasswordHash,
		FirstName:    c.FirstName,
		LastName:     c.LastName,
		Phone:        c.Phone,
		IsActive:     isActive,
		SystemRole:   c.SystemRole,
		StoreIDs:     dbtypes.UUIDArray(storeIDs),
	}
}
