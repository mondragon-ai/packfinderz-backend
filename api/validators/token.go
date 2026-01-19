package validators

import (
	"errors"
	"strings"
)

var ErrInvalidToken = errors.New("invalid auth token")

// Claims holds the minimal data we track from JWTs.
type Claims struct {
	UserID  string
	Role    string
	StoreID string
}

// ParseAuthToken accepts Authorization header values like "Bearer user|role|store".
func ParseAuthToken(raw string) (Claims, error) {
	if raw == "" {
		return Claims{}, ErrInvalidToken
	}
	token := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	parts := strings.Split(token, "|")
	if len(parts) < 3 {
		return Claims{}, ErrInvalidToken
	}
	return Claims{UserID: parts[0], Role: parts[1], StoreID: parts[2]}, nil
}
