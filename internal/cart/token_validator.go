package cart

import (
	"fmt"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

var attributionTokenSigningMethod = jwt.SigningMethodHS256

type attributionTokenValidator interface {
	Validate(token string) bool
}

type jwtAttributionTokenValidator struct {
	cfg config.JWTConfig
}

// NewJWTAttributionTokenValidator builds a validator that verifies JWT signature and expiry.
func NewJWTAttributionTokenValidator(cfg config.JWTConfig) (attributionTokenValidator, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, fmt.Errorf("jwt secret required for attribution token validator")
	}
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, fmt.Errorf("jwt issuer required for attribution token validator")
	}
	return &jwtAttributionTokenValidator{cfg: cfg}, nil
}

func (v *jwtAttributionTokenValidator) Validate(token string) bool {
	if token == "" {
		return false
	}

	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(
		token,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != attributionTokenSigningMethod {
				return nil, fmt.Errorf("unexpected signing method %s", token.Header["alg"])
			}
			return []byte(v.cfg.Secret), nil
		},
		jwt.WithValidMethods([]string{attributionTokenSigningMethod.Alg()}),
		jwt.WithIssuer(v.cfg.Issuer),
	)
	return err == nil
}
