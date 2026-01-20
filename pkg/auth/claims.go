package auth

import (
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessTokenPayload captures the data available when minting a JWT.
type AccessTokenPayload struct {
	UserID        uuid.UUID
	ActiveStoreID *uuid.UUID
	Role          enums.MemberRole
	StoreType     *enums.StoreType
	KYCStatus     *enums.KYCStatus
}

// AccessTokenClaims represents the typed JWT issued to clients.
type AccessTokenClaims struct {
	UserID        uuid.UUID        `json:"user_id"`
	ActiveStoreID *uuid.UUID       `json:"active_store_id,omitempty"`
	Role          enums.MemberRole `json:"role"`
	StoreType     *enums.StoreType `json:"store_type,omitempty"`
	KYCStatus     *enums.KYCStatus `json:"kyc_status,omitempty"`
	jwt.RegisteredClaims
}
