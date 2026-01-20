package session

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	redisclient "github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/google/uuid"
	redislib "github.com/redis/go-redis/v9"
)

const refreshTokenBytes = 32

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type sessionStore interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
}

type sessionKeyer interface {
	AccessSessionKey(accessID string) string
}

// Manager handles refresh token creation, storage, and rotation.
type Manager struct {
	store sessionStore
	keyer sessionKeyer
	ttl   time.Duration
}

// AccessSessionChecker exposes the read-only surface needed by middleware.
type AccessSessionChecker interface {
	HasSession(ctx context.Context, accessID string) (bool, error)
}

// NewManager constructs a session manager backed by Redis.
func NewManager(client *redisclient.Client, cfg config.JWTConfig) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	ttl := cfg.RefreshTokenTTL()
	if ttl <= 0 {
		return nil, fmt.Errorf("refresh token ttl must be positive")
	}
	accessTTL := time.Duration(cfg.ExpirationMinutes) * time.Minute
	if ttl <= accessTTL {
		return nil, fmt.Errorf("refresh token ttl (%s) must exceed access token ttl (%s)", ttl, accessTTL)
	}

	return &Manager{
		store: client,
		keyer: client,
		ttl:   ttl,
	}, nil
}

// Generate creates a refresh token for the provided access ID and stores it in Redis.
func (m *Manager) Generate(ctx context.Context, accessID string) (string, error) {
	if strings.TrimSpace(accessID) == "" {
		return "", fmt.Errorf("access id is required")
	}
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	if err := m.store.Set(ctx, m.keyer.AccessSessionKey(accessID), token, m.ttl); err != nil {
		return "", err
	}
	return token, nil
}

// Rotate validates the provided refresh token, invalidates the prior session, and issues a new access/refresh pair.
func (m *Manager) Rotate(ctx context.Context, oldAccessID, provided string) (string, string, error) {
	if strings.TrimSpace(oldAccessID) == "" || strings.TrimSpace(provided) == "" {
		return "", "", ErrInvalidRefreshToken
	}

	key := m.keyer.AccessSessionKey(oldAccessID)
	stored, err := m.store.Get(ctx, key)
	if err != nil {
		return "", "", wrapNotFound(err)
	}

	if subtle.ConstantTimeCompare([]byte(stored), []byte(provided)) != 1 {
		return "", "", ErrInvalidRefreshToken
	}

	newAccessID := NewAccessID()
	newToken, err := generateRefreshToken()
	if err != nil {
		return "", "", err
	}
	if err := m.store.Set(ctx, m.keyer.AccessSessionKey(newAccessID), newToken, m.ttl); err != nil {
		return "", "", err
	}

	if err := m.store.Del(ctx, key); err != nil {
		return "", "", err
	}

	return newAccessID, newToken, nil
}

// Revoke deletes the refresh mapping tied to the access identifier.
func (m *Manager) Revoke(ctx context.Context, accessID string) error {
	if strings.TrimSpace(accessID) == "" {
		return fmt.Errorf("access id is required")
	}
	return m.store.Del(ctx, m.keyer.AccessSessionKey(accessID))
}

// NewAccessID produces a stable identifier used as the JWT jti/Redis key.
func NewAccessID() string {
	return uuid.NewString()
}

func generateRefreshToken() (string, error) {
	bytes := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func wrapNotFound(err error) error {
	if errors.Is(err, redislib.Nil) || errors.Is(err, ErrInvalidRefreshToken) {
		return ErrInvalidRefreshToken
	}
	return err
}

// HasSession reports whether the provided access ID still has an active refresh session.
func (m *Manager) HasSession(ctx context.Context, accessID string) (bool, error) {
	if strings.TrimSpace(accessID) == "" {
		return false, fmt.Errorf("access id is required")
	}
	key := m.keyer.AccessSessionKey(accessID)
	if _, err := m.store.Get(ctx, key); err != nil {
		if errors.Is(err, redislib.Nil) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
