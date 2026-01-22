package pagination

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultLimit is the standard page size when a limit is not provided.
	DefaultLimit = 25
	// MaxLimit caps how many rows any cursor query can request.
	MaxLimit = 100
)

// Params holds cursor pagination inputs from controllers or services.
type Params struct {
	Limit  int
	Cursor string
}

// Cursor represents the pagination cursor components.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// NormalizeLimit enforces the configured default and maximum limits.
func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

// LimitWithBuffer returns the normalization result plus one to detect the next page.
func LimitWithBuffer(limit int) int {
	return NormalizeLimit(limit) + 1
}

// EncodeCursor builds a base64 cursor string from the provided values.
func EncodeCursor(cursor Cursor) string {
	payload := fmt.Sprintf("%s|%s", cursor.CreatedAt.UTC().Format(time.RFC3339Nano), cursor.ID.String())
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

// ParseCursor decodes the cursor string back into its components.
func ParseCursor(value string) (*Cursor, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor format")
	}

	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid cursor timestamp: %w", err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid cursor id: %w", err)
	}
	return &Cursor{
		CreatedAt: t,
		ID:        id,
	}, nil
}
