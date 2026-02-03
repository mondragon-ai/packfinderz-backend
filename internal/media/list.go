package media

import (
	"context"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	pkgpagination "github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
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
	pkgpagination.Params
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
	SignedURL  *string           `json:"signed_url,omitempty"`
}

type listQuery struct {
	storeID  uuid.UUID
	kind     *enums.MediaKind
	status   *enums.MediaStatus
	mimeType string
	search   string
	limit    int
	cursor   *pkgpagination.Cursor
}

func (s *service) ListMedia(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "active store id required")
	}

	limit := pkgpagination.NormalizeLimit(params.Limit)
	query := listQuery{
		storeID:  params.StoreID,
		limit:    pkgpagination.LimitWithBuffer(params.Limit),
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
		cursor, err := pkgpagination.ParseCursor(params.Cursor)
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
		nextCursor = pkgpagination.EncodeCursor(pkgpagination.Cursor{
			CreatedAt: rows[limit].CreatedAt,
			ID:        rows[limit].ID,
		})
		rows = rows[:limit]
	}

	items := make([]ListItem, len(rows))
	for i, m := range rows {
		signedURL, err := s.buildReadURL(m)
		if err != nil {
			return nil, err
		}

		items[i] = toListItem(m)
		items[i].SignedURL = stringPtr(signedURL)
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

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
