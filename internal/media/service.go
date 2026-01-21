package media

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

const maxUploadBytes = 20 * 1024 * 1024

type membershipsRepository interface {
	UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error)
}

type mediaRepository interface {
	Create(ctx context.Context, media *models.Media) (*models.Media, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type gcsClient interface {
	SignedURL(bucket, object, contentType string, expires time.Duration) (string, error)
}

// Service exposes media-presign semantics.
type Service interface {
	PresignUpload(ctx context.Context, userID, storeID uuid.UUID, input PresignInput) (*PresignOutput, error)
}

type service struct {
	repo         mediaRepository
	memberships  membershipsRepository
	gcs          gcsClient
	bucket       string
	uploadTTL    time.Duration
	allowedRoles []enums.MemberRole
}

// NewService constructs a media service backed by the provided repositories and GCS signer.
func NewService(repo mediaRepository, memberships membershipsRepository, gcsClient gcsClient, bucket string, uploadTTL time.Duration) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	if memberships == nil {
		return nil, fmt.Errorf("memberships repository required")
	}
	if gcsClient == nil {
		return nil, fmt.Errorf("gcs client required")
	}
	if bucket == "" {
		return nil, fmt.Errorf("gcs bucket required")
	}
	if uploadTTL <= 0 {
		return nil, fmt.Errorf("upload ttl must be positive")
	}
	return &service{
		repo:        repo,
		memberships: memberships,
		gcs:         gcsClient,
		bucket:      bucket,
		uploadTTL:   uploadTTL,
		allowedRoles: []enums.MemberRole{
			enums.MemberRoleOwner,
			enums.MemberRoleAdmin,
			enums.MemberRoleManager,
			enums.MemberRoleStaff,
			enums.MemberRoleOps,
		},
	}, nil
}

// PresignInput models the payload required to request an upload URL.
type PresignInput struct {
	Kind      enums.MediaKind
	MimeType  string
	FileName  string
	SizeBytes int64
}

// PresignOutput contains the data returned to the client after creating a media record.
type PresignOutput struct {
	MediaID      uuid.UUID `json:"media_id"`
	GCSKey       string    `json:"gcs_key"`
	SignedPUTURL string    `json:"signed_put_url"`
	ContentType  string    `json:"content_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

var mimeTypesByKind = map[enums.MediaKind][]string{
	enums.MediaKindProduct:    {"image/png", "image/jpeg", "image/webp", "image/gif", "video/mp4", "video/webm"},
	enums.MediaKindAds:        {"image/png", "image/jpeg", "image/webp", "image/gif", "video/mp4", "video/webm"},
	enums.MediaKindPDF:        {"application/pdf"},
	enums.MediaKindLicenseDoc: {"application/pdf"},
	enums.MediaKindCOA:        {"application/pdf"},
	enums.MediaKindManifest:   {"application/pdf"},
	enums.MediaKindUser:       {"image/png", "image/jpeg", "image/webp"},
	// MediaKindOther is intentionally absent to allow any mime type.
}

func (s *service) PresignUpload(ctx context.Context, userID, storeID uuid.UUID, input PresignInput) (*PresignOutput, error) {
	if userID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "user identity missing")
	}
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store identity missing")
	}

	if input.Kind == "" || !input.Kind.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "invalid media kind")
	}

	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "file_name is required")
	}

	if input.SizeBytes <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "size_bytes must be positive")
	}
	if input.SizeBytes > maxUploadBytes {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, fmt.Sprintf("size_bytes must be ≤ %d bytes", maxUploadBytes))
	}

	mimeType := strings.TrimSpace(input.MimeType)
	if mimeType == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "mime_type is required")
	}
	if !isAllowedMime(input.Kind, mimeType) {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "mime_type not allowed for media kind")
	}

	ok, err := s.memberships.UserHasRole(ctx, userID, storeID, s.allowedRoles...)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership role")
	}
	if !ok {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}

	mediaID := uuid.New()
	gcsKey := buildGCSKey(input.Kind, mediaID, fileName)

	mediaRow := &models.Media{
		ID:        mediaID,
		StoreID:   storeID,
		UserID:    userID,
		Kind:      input.Kind,
		Status:    enums.MediaStatusPending,
		GCSKey:    gcsKey,
		FileName:  fileName,
		MimeType:  mimeType,
		SizeBytes: input.SizeBytes,
	}

	if _, err := s.repo.Create(ctx, mediaRow); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist media row")
	}

	expiresAt := time.Now().Add(s.uploadTTL)
	signedURL, err := s.gcs.SignedURL(s.bucket, gcsKey, mimeType, s.uploadTTL)
	if err != nil {
		_ = s.repo.Delete(ctx, mediaID)
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "sign upload url")
	}

	return &PresignOutput{
		MediaID:      mediaID,
		GCSKey:       gcsKey,
		SignedPUTURL: signedURL,
		ContentType:  mimeType,
		ExpiresAt:    expiresAt,
	}, nil
}

func isAllowedMime(kind enums.MediaKind, mimeType string) bool {
	if allowed, ok := mimeTypesByKind[kind]; ok && len(allowed) > 0 {
		for _, candidate := range allowed {
			if strings.EqualFold(candidate, mimeType) {
				return true
			}
		}
		return false
	}
	return true
}

func buildGCSKey(kind enums.MediaKind, id uuid.UUID, fileName string) string {
	cleanName := sanitizeFileName(fileName)
	if cleanName == "" {
		cleanName = id.String()
	}
	return fmt.Sprintf("media/%s/%s/%s", kind, id.String(), cleanName)
}

func sanitizeFileName(name string) string {
	if name == "" {
		return ""
	}
	clean := path.Base(strings.TrimSpace(name))
	if clean == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(clean))
	for _, r := range clean {
		switch {
		case r == '/' || r == '\\' || unicode.IsControl(r):
			continue
		case unicode.IsSpace(r):
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-_.")
	return result
}
