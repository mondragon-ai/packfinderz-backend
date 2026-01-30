package media

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxUploadBytes = 20 * 1024 * 1024

type membershipsRepository interface {
	UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error)
}

type mediaRepository interface {
	Create(ctx context.Context, media *models.Media) (*models.Media, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, opts listQuery) ([]models.Media, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
	MarkDeleted(ctx context.Context, id uuid.UUID, deletedAt time.Time) error
}

type gcsClient interface {
	SignedURL(bucket, object, contentType string, expires time.Duration) (string, error)
	SignedReadURL(bucket, object string, expires time.Duration) (string, error)
	DeleteObject(ctx context.Context, bucket, object string) error
}

type mediaAttachmentLookup interface {
	ListByMediaID(ctx context.Context, mediaID uuid.UUID) ([]models.MediaAttachment, error)
}

// Service exposes media-presign semantics.
type Service interface {
	PresignUpload(ctx context.Context, userID, storeID uuid.UUID, input PresignInput) (*PresignOutput, error)
	ListMedia(ctx context.Context, params ListParams) (*ListResult, error)
	DeleteMedia(ctx context.Context, params DeleteMediaParams) error
	GenerateReadURL(ctx context.Context, params ReadURLParams) (*ReadURLOutput, error)
}

type service struct {
	repo         mediaRepository
	memberships  membershipsRepository
	gcs          gcsClient
	attachments  mediaAttachmentLookup
	bucket       string
	uploadTTL    time.Duration
	downloadTTL  time.Duration
	allowedRoles []enums.MemberRole
}

// NewService constructs a media service backed by the provided repositories and GCS signer.
func NewService(repo mediaRepository, memberships membershipsRepository, attachments mediaAttachmentLookup, gcsClient gcsClient, bucket string, uploadTTL, downloadTTL time.Duration) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	if memberships == nil {
		return nil, fmt.Errorf("memberships repository required")
	}
	if attachments == nil {
		return nil, fmt.Errorf("attachments lookup required")
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
	if downloadTTL <= 0 {
		return nil, fmt.Errorf("download ttl must be positive")
	}
	return &service{
		repo:        repo,
		memberships: memberships,
		gcs:         gcsClient,
		attachments: attachments,
		bucket:      bucket,
		uploadTTL:   uploadTTL,
		downloadTTL: downloadTTL,
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

type ReadURLParams struct {
	StoreID uuid.UUID
	MediaID uuid.UUID
}

type ReadURLOutput struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *service) GenerateReadURL(ctx context.Context, params ReadURLParams) (*ReadURLOutput, error) {
	if params.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "active store id required")
	}
	if params.MediaID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "media id required")
	}

	mediaRow, err := s.repo.FindByID(ctx, params.MediaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "media not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup media")
	}

	if mediaRow.StoreID != params.StoreID {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "media does not belong to active store")
	}

	if !isReadableStatus(mediaRow.Status) {
		return nil, pkgerrors.New(pkgerrors.CodeConflict, "media not available for download")
	}

	url, err := s.gcs.SignedReadURL(s.bucket, mediaRow.GCSKey, s.downloadTTL)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "generate download url")
	}

	return &ReadURLOutput{
		URL:       url,
		ExpiresAt: time.Now().Add(s.downloadTTL),
	}, nil
}

type DeleteMediaParams struct {
	StoreID uuid.UUID
	MediaID uuid.UUID
}

func (s *service) DeleteMedia(ctx context.Context, params DeleteMediaParams) error {
	if params.StoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "active store id required")
	}
	if params.MediaID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "media id required")
	}

	mediaRow, err := s.repo.FindByID(ctx, params.MediaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.New(pkgerrors.CodeNotFound, "media not found")
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup media")
	}

	if mediaRow.StoreID != params.StoreID {
		return pkgerrors.New(pkgerrors.CodeForbidden, "media does not belong to active store")
	}

	if mediaRow.Status == enums.MediaStatusDeleted {
		return nil
	}

	if !isReadableStatus(mediaRow.Status) {
		return pkgerrors.New(pkgerrors.CodeConflict, "media not available for deletion")
	}

	attachments, err := s.attachments.ListByMediaID(ctx, mediaRow.ID)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load media attachments")
	}
	if protected := protectedAttachmentEntities(attachments); len(protected) > 0 {
		return pkgerrors.New(pkgerrors.CodeConflict,
			fmt.Sprintf("media has protected attachments: %s", strings.Join(protected, ", ")))
	}

	if err := s.gcs.DeleteObject(ctx, s.bucket, mediaRow.GCSKey); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "delete gcs object")
	}

	return nil
}

func isReadableStatus(status enums.MediaStatus) bool {
	return status == enums.MediaStatusUploaded || status == enums.MediaStatusReady
}

func protectedAttachmentEntities(attachments []models.MediaAttachment) []string {
	if len(attachments) == 0 {
		return nil
	}
	set := make(map[string]struct{})
	for _, attachment := range attachments {
		if _, ok := models.ProtectedAttachmentEntities[attachment.EntityType]; ok {
			set[attachment.EntityType] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	types := make([]string, 0, len(set))
	for entityType := range set {
		types = append(types, entityType)
	}
	sort.Strings(types)
	return types
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
