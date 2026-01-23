package licenses

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	pkgpagination "github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type mediasRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
}

type membershipsRepository interface {
	UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error)
}

type storesRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error)
	Update(ctx context.Context, store *models.Store) error
}

type licensesRepository interface {
	Create(ctx context.Context, license *models.License) (*models.License, error)
	List(ctx context.Context, opts listQuery) ([]models.License, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.License, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CountValidLicenses(ctx context.Context, storeID uuid.UUID) (int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status enums.LicenseStatus) error
}

type gcsClient interface {
	SignedReadURL(bucket, object string, expires time.Duration) (string, error)
}

// Service exposes license creation, listing, verification, and deletion semantics.
type Service interface {
	CreateLicense(ctx context.Context, userID, storeID uuid.UUID, input CreateLicenseInput) (*models.License, error)
	ListLicenses(ctx context.Context, params ListParams) (*ListResult, error)
	DeleteLicense(ctx context.Context, userID, storeID, licenseID uuid.UUID) error
	VerifyLicense(ctx context.Context, licenseID uuid.UUID, decision enums.LicenseStatus, reason string) (*models.License, error)
}

type service struct {
	repo         licensesRepository
	mediaRepo    mediasRepository
	memberships  membershipsRepository
	gcs          gcsClient
	bucket       string
	downloadTTL  time.Duration
	storeRepo    storesRepository
	allowedRoles []enums.MemberRole
}

// CreateLicenseInput holds the metadata required to create a license.
type CreateLicenseInput struct {
	MediaID        uuid.UUID
	IssuingState   string
	IssueDate      *time.Time
	ExpirationDate *time.Time
	Type           enums.LicenseType
	Number         string
}

// NewService builds a license service backed by the provided repositories and GCS signer.
func NewService(repo licensesRepository, mediaRepo mediasRepository, memberships membershipsRepository, gcs gcsClient, bucket string, downloadTTL time.Duration, storeRepo storesRepository) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("license repository required")
	}
	if mediaRepo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	if memberships == nil {
		return nil, fmt.Errorf("memberships repository required")
	}
	if gcs == nil {
		return nil, fmt.Errorf("gcs client required")
	}
	if bucket == "" {
		return nil, fmt.Errorf("gcs bucket required")
	}
	if downloadTTL <= 0 {
		return nil, fmt.Errorf("download ttl must be positive")
	}
	if storeRepo == nil {
		return nil, fmt.Errorf("store repository required")
	}
	return &service{
		repo:        repo,
		mediaRepo:   mediaRepo,
		memberships: memberships,
		gcs:         gcs,
		bucket:      bucket,
		downloadTTL: downloadTTL,
		storeRepo:   storeRepo,
		allowedRoles: []enums.MemberRole{
			enums.MemberRoleOwner,
			enums.MemberRoleAdmin,
			enums.MemberRoleManager,
			enums.MemberRoleStaff,
			enums.MemberRoleOps,
		},
	}, nil
}

func (s *service) CreateLicense(ctx context.Context, userID, storeID uuid.UUID, input CreateLicenseInput) (*models.License, error) {
	if userID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "user identity missing")
	}
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store identity missing")
	}
	if input.MediaID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "media_id is required")
	}
	if strings.TrimSpace(input.IssuingState) == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "issuing_state is required")
	}
	if strings.TrimSpace(input.Number) == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "number is required")
	}
	if !input.Type.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "invalid license type")
	}

	ok, err := s.memberships.UserHasRole(ctx, userID, storeID, s.allowedRoles...)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership role")
	}
	if !ok {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}

	mediaRow, err := s.mediaRepo.FindByID(ctx, input.MediaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "media not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup media")
	}

	if mediaRow.StoreID != storeID {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "media does not belong to active store")
	}
	if mediaRow.Kind != enums.MediaKindLicenseDoc {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "media must be a license document")
	}
	if !isLicenseMediaStatus(mediaRow.Status) {
		return nil, pkgerrors.New(pkgerrors.CodeConflict, "media not ready")
	}
	if !isAllowedLicenseMime(mediaRow.MimeType) {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "media mime_type must be pdf or image")
	}

	license := &models.License{
		StoreID:        storeID,
		UserID:         userID,
		Status:         enums.LicenseStatusPending,
		MediaID:        mediaRow.ID,
		GCSKey:         mediaRow.GCSKey,
		IssuingState:   strings.TrimSpace(input.IssuingState),
		IssueDate:      input.IssueDate,
		ExpirationDate: input.ExpirationDate,
		Type:           input.Type,
		Number:         strings.TrimSpace(input.Number),
	}

	created, err := s.repo.Create(ctx, license)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create license")
	}
	return created, nil
}

func (s *service) ListLicenses(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "active store id required")
	}

	limit := pkgpagination.NormalizeLimit(params.Limit)
	query := listQuery{
		storeID: params.StoreID,
		limit:   pkgpagination.LimitWithBuffer(params.Limit),
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
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list licenses")
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
	for i, row := range rows {
		url, err := s.buildSignedURL(row.GCSKey)
		if err != nil {
			return nil, err
		}
		items[i] = toListItem(row)
		items[i].SignedURL = url
	}

	return &ListResult{
		Items:  items,
		Cursor: nextCursor,
	}, nil
}

func (s *service) DeleteLicense(ctx context.Context, userID, storeID, licenseID uuid.UUID) error {
	if userID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "user identity missing")
	}
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store identity missing")
	}
	if licenseID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "license id is required")
	}

	ok, err := s.memberships.UserHasRole(ctx, userID, storeID, enums.MemberRoleOwner, enums.MemberRoleManager)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership role")
	}
	if !ok {
		return pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role")
	}

	license, err := s.repo.FindByID(ctx, licenseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.New(pkgerrors.CodeNotFound, "license not found")
		}
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup license")
	}
	if license.StoreID != storeID {
		return pkgerrors.New(pkgerrors.CodeForbidden, "license does not belong to active store")
	}
	if !isDeletableLicenseStatus(license.Status) {
		return pkgerrors.New(pkgerrors.CodeConflict, "only rejected or expired licenses can be deleted")
	}

	if err := s.repo.Delete(ctx, licenseID); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "delete license")
	}

	validCount, err := s.repo.CountValidLicenses(ctx, storeID)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "count licenses")
	}
	if validCount == 0 {
		store, err := s.storeRepo.FindByID(ctx, storeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup store")
		}
		if store.KYCStatus != enums.KYCStatusPendingVerification {
			store.KYCStatus = enums.KYCStatusPendingVerification
			if err := s.storeRepo.Update(ctx, store); err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store kyc")
			}
		}
	}
	return nil
}

func (s *service) buildSignedURL(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	url, err := s.gcs.SignedReadURL(s.bucket, key, s.downloadTTL)
	if err != nil {
		return "", pkgerrors.Wrap(pkgerrors.CodeDependency, err, "generate signed read url")
	}
	return url, nil
}

func isLicenseMediaStatus(status enums.MediaStatus) bool {
	return status == enums.MediaStatusUploaded || status == enums.MediaStatusReady
}

func isAllowedLicenseMime(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	lowered := strings.ToLower(strings.TrimSpace(mimeType))
	if lowered == "application/pdf" {
		return true
	}
	return strings.HasPrefix(lowered, "image/")
}

func isDeletableLicenseStatus(status enums.LicenseStatus) bool {
	return status == enums.LicenseStatusExpired || status == enums.LicenseStatusRejected
}

func (s *service) VerifyLicense(ctx context.Context, licenseID uuid.UUID, decision enums.LicenseStatus, reason string) (*models.License, error) {
	if licenseID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "license id is required")
	}
	if decision != enums.LicenseStatusVerified && decision != enums.LicenseStatusRejected {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "invalid decision")
	}

	license, err := s.repo.FindByID(ctx, licenseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "license not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup license")
	}

	if license.Status != enums.LicenseStatusPending {
		return nil, pkgerrors.New(pkgerrors.CodeConflict, "license already finalized")
	}

	if err := s.repo.UpdateStatus(ctx, licenseID, decision); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update license status")
	}

	license.Status = decision
	_ = reason // currently unused, accepted for future use
	return license, nil
}
