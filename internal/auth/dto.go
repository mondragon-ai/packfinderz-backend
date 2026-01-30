package auth

import (
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// LoginRequest captures the user credentials sent to the login endpoint.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// StoreSummary describes the store metadata returned after login.
type StoreSummary struct {
	ID      uuid.UUID       `json:"id"`
	Name    string          `json:"name"`
	Type    enums.StoreType `json:"type"`
	LogoURL *string         `json:"logo_url,omitempty"`
}

// LoginResponse contains the tokens, user, and store list produced by a successful login.
type LoginResponse struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	Stores       []StoreSummary `json:"stores"`
	User         *users.UserDTO `json:"user"`
}

// AdminLoginResponse mirrors LoginResponse while exposing the admin user.
type AdminLoginResponse struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	User         *users.UserDTO `json:"user"`
}
