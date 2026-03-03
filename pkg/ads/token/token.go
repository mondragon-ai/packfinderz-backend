package token

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

var jwtSigningMethod = jwt.SigningMethodHS256

// Payload defines the fields embedded within each ad attribution token.
type Payload struct {
	TokenID        uuid.UUID             `json:"token_id"`
	AdID           uuid.UUID             `json:"ad_id"`
	CreativeID     uuid.UUID             `json:"creative_id"`
	Placement      enums.AdPlacement     `json:"placement"`
	TargetType     enums.AdTargetType    `json:"target_type"`
	TargetID       uuid.UUID             `json:"target_id"`
	BuyerStoreID   uuid.UUID             `json:"buyer_store_id"`
	EventType      enums.AdEventFactType `json:"event_type"`
	OccurredAt     time.Time             `json:"occurred_at"`
	ExpiresAt      time.Time             `json:"expires_at"`
	RequestID      string                `json:"request_id"`
	BidCents       int64                 `json:"bid_cents"`
	DestinationURL string                `json:"destination_url"`
}

// Claims mirrors the JWT payload stored in ad tokens.
type Claims struct {
	Payload
	jwt.RegisteredClaims
}

// Validate ensures the payload meets the minimum schema requirements.
func (p Payload) Validate() error {
	if p.TokenID == uuid.Nil {
		return fmt.Errorf("token_id is required")
	}
	if p.AdID == uuid.Nil {
		return fmt.Errorf("ad_id is required")
	}
	if p.CreativeID == uuid.Nil {
		return fmt.Errorf("creative_id is required")
	}
	if !p.Placement.IsValid() {
		return fmt.Errorf("invalid placement")
	}
	if !p.TargetType.IsValid() {
		return fmt.Errorf("invalid target type")
	}
	if p.TargetID == uuid.Nil {
		return fmt.Errorf("target_id is required")
	}
	if p.BuyerStoreID == uuid.Nil {
		return fmt.Errorf("buyer_store_id is required")
	}
	if !p.EventType.IsValid() {
		return fmt.Errorf("invalid event type")
	}
	if p.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	if p.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	if !p.ExpiresAt.After(p.OccurredAt) {
		return fmt.Errorf("expires_at must be after occurred_at")
	}
	if strings.TrimSpace(p.RequestID) == "" {
		return fmt.Errorf("request_id is required")
	}
	if p.BidCents < 0 {
		return fmt.Errorf("bid_cents must be non-negative")
	}
	if strings.TrimSpace(p.DestinationURL) == "" {
		return fmt.Errorf("destination_url is required")
	}
	return nil
}

// MintToken returns a signed JWT for the provided payload.
func MintToken(secret string, payload Payload) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("token secret is required")
	}
	if err := payload.Validate(); err != nil {
		return "", err
	}

	claims := Claims{
		Payload: payload,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(payload.OccurredAt),
			ExpiresAt: jwt.NewNumericDate(payload.ExpiresAt),
			ID:        payload.TokenID.String(),
		},
	}

	token := jwt.NewWithClaims(jwtSigningMethod, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

// ParseToken validates the signed string and returns the payload.
func ParseToken(secret string, tokenString string) (Payload, error) {
	if strings.TrimSpace(secret) == "" {
		return Payload{}, fmt.Errorf("token secret is required")
	}
	if strings.TrimSpace(tokenString) == "" {
		return Payload{}, fmt.Errorf("token is required")
	}

	claims := &Claims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwtSigningMethod.Alg()}))
	if _, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwtSigningMethod {
			return nil, fmt.Errorf("unexpected signing method %s", token.Header["alg"])
		}
		return []byte(secret), nil
	}); err != nil {
		return Payload{}, err
	}

	payload := claims.Payload
	// if claims.RegisteredClaims.IssuedAt != nil {
	// 	payload.OccurredAt = claims.RegisteredClaims.IssuedAt.Time
	// }
	if claims.RegisteredClaims.ExpiresAt != nil {
		payload.ExpiresAt = claims.RegisteredClaims.ExpiresAt.Time
	}

	if err := payload.Validate(); err != nil {
		return Payload{}, err
	}

	return payload, nil
}
