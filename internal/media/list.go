package media

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

const (
	defaultMediaListLimit = 25
	maxMediaListLimit     = 100
)

// ListParams configures media listing filters/pagination.
type ListParams struct {
	StoreID   uuid.UUID
	HasKind   bool
	Kind      enums.MediaKind
	HasStatus bool
	Status    enums.MediaStatus
	MimeType  string
	Search    string
	Limit     int
	Cursor    string
}

// ListResult returns paginated media metadata.
type ListResult struct {
	Items  []ListItem `json:"items"`
	Cursor string     `json:"cursor"`
}

// ListItem represents returned media metadata.
type ListItem struct {
	ID         uuid.UUID         `json:"id"`
	StoreID    uuid.UUID         `json:"store_id"`
	UserID     uuid.UUID         `json:"user_id"`
	Kind       enums.MediaKind   `json:"kind"`
	Status     enums.MediaStatus `json:"status"`
	FileName   string            `json:"file_name"`
	MimeType   string            `json:"mime_type"`
	SizeBytes  int64             `json:"size_bytes"`
	CreatedAt  time.Time         `json:"created_at"`
	UploadedAt *time.Time        `json:"uploaded_at"`
	SignedURL  string            `json:"signed_url,omitempty"`
}

type listCursor struct {
	createdAt time.Time
	id        uuid.UUID
}

type listQuery struct {
	storeID  uuid.UUID
	kind     *enums.MediaKind
	status   *enums.MediaStatus
	mimeType string
	search   string
	limit    int
	cursor   *listCursor
}

func (s *service) ListMedia(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "active store id required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultMediaListLimit
	}
	if limit > maxMediaListLimit {
		limit = maxMediaListLimit
	}

	query := listQuery{
		storeID:  params.StoreID,
		limit:    limit + 1,
		mimeType: strings.TrimSpace(params.MimeType),
		search:   strings.TrimSpace(params.Search),
	}
	if params.HasKind {
		query.kind = &params.Kind
	}
	if params.HasStatus {
		query.status = &params.Status
	}
	if params.Cursor != "" {
		cursor, err := parseListCursor(params.Cursor)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid cursor")
		}
		query.cursor = cursor
	}

	rows, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list media")
	}

	nextCursor := ""
	if len(rows) > limit {
		nextCursor = encodeListCursor(&rows[limit])
		rows = rows[:limit]
	}

	items := make([]ListItem, len(rows))
	for i, m := range rows {
		signedURL, err := s.buildReadURL(m)
		if err != nil {
			return nil, err
		}

		items[i] = toListItem(m)
		items[i].SignedURL = signedURL
	}

	return &ListResult{
		Items:  items,
		Cursor: nextCursor,
	}, nil
}

func toListItem(m models.Media) ListItem {
	return ListItem{
		ID:         m.ID,
		StoreID:    m.StoreID,
		UserID:     m.UserID,
		Kind:       m.Kind,
		Status:     m.Status,
		FileName:   m.FileName,
		MimeType:   m.MimeType,
		SizeBytes:  m.SizeBytes,
		CreatedAt:  m.CreatedAt,
		UploadedAt: m.UploadedAt,
	}
}

func encodeListCursor(m *models.Media) string {
	payload := fmt.Sprintf("%s|%s", m.CreatedAt.UTC().Format(time.RFC3339Nano), m.ID.String())
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

func parseListCursor(value string) (*listCursor, error) {
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
	return &listCursor{
		createdAt: t,
		id:        id,
	}, nil
}

func (s *service) buildReadURL(media models.Media) (string, error) {
	if !isReadableStatus(media.Status) {
		return "", nil
	}
	url, err := s.gcs.SignedReadURL(s.bucket, media.GCSKey, s.downloadTTL)
	if err != nil {
		return "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "generate signed read url")
	}
	return url, nil
}
