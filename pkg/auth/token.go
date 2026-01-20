package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var jwtSigningMethod = jwt.SigningMethodHS256

// MintAccessToken issues a signed JWT for the provided payload using the configured TTL.
func MintAccessToken(cfg config.JWTConfig, now time.Time, payload AccessTokenPayload) (string, error) {
	if cfg.Secret == "" {
		return "", fmt.Errorf("jwt secret is required")
	}
	if cfg.Issuer == "" {
		return "", fmt.Errorf("jwt issuer is required")
	}
	if cfg.ExpirationMinutes <= 0 {
		return "", fmt.Errorf("jwt expiration minutes must be positive")
	}
	if !payload.Role.IsValid() {
		return "", fmt.Errorf("invalid member role %q", payload.Role)
	}
	if payload.StoreType != nil && !payload.StoreType.IsValid() {
		return "", fmt.Errorf("invalid store type %q", payload.StoreType)
	}
	if payload.KYCStatus != nil && !payload.KYCStatus.IsValid() {
		return "", fmt.Errorf("invalid kyc status %q", payload.KYCStatus)
	}

	issuedAt := jwt.NewNumericDate(now)
	expiry := jwt.NewNumericDate(now.Add(time.Duration(cfg.ExpirationMinutes) * time.Minute))

	jti := strings.TrimSpace(payload.JTI)
	if jti == "" {
		jti = uuid.NewString()
	}

	claims := AccessTokenClaims{
		UserID:        payload.UserID,
		ActiveStoreID: payload.ActiveStoreID,
		Role:          payload.Role,
		StoreType:     payload.StoreType,
		KYCStatus:     payload.KYCStatus,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			IssuedAt:  issuedAt,
			ExpiresAt: expiry,
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwtSigningMethod, claims)
	signed, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("signing jwt: %w", err)
	}
	return signed, nil
}

// ParseAccessToken validates the JWT string and returns typed claims.
func ParseAccessToken(cfg config.JWTConfig, tokenString string) (*AccessTokenClaims, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}

	claims := &AccessTokenClaims{}
	_, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwtSigningMethod {
				return nil, fmt.Errorf("unexpected signing method %s", token.Header["alg"])
			}
			return []byte(cfg.Secret), nil
		},
		jwt.WithValidMethods([]string{jwtSigningMethod.Alg()}),
		jwt.WithIssuer(cfg.Issuer),
	)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// ParseAccessTokenAllowExpired parses the JWT without validating exp/nbf so refresh can inspect jti.
func ParseAccessTokenAllowExpired(cfg config.JWTConfig, tokenString string) (*AccessTokenClaims, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}

	claims := &AccessTokenClaims{}
	parser := jwt.NewParser(
		jwt.WithoutClaimsValidation(),
		jwt.WithValidMethods([]string{jwtSigningMethod.Alg()}),
		jwt.WithIssuer(cfg.Issuer),
	)
	_, err := parser.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwtSigningMethod {
				return nil, fmt.Errorf("unexpected signing method %s", token.Header["alg"])
			}
			return []byte(cfg.Secret), nil
		},
	)
	if err != nil {
		return nil, err
	}

	return claims, nil
}
