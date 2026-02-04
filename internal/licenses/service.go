package licenses

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
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

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type outboxPublisher interface {
	Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error
}

type storesRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error)
	Update(ctx context.Context, store *models.Store) error
	FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error)
	UpdateWithTx(tx *gorm.DB, store *models.Store) error
	UpdateStatusWithTx(tx *gorm.DB, storeID uuid.UUID, newStatus enums.KYCStatus) error
}

type licensesRepository interface {
	Create(ctx context.Context, license *models.License) (*models.License, error)
	List(ctx context.Context, opts listQuery) ([]models.License, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.License, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteWithTx(tx *gorm.DB, id uuid.UUID) error
	CountValidLicenses(ctx context.Context, storeID uuid.UUID) (int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status enums.LicenseStatus) error
	CreateWithTx(tx *gorm.DB, license *models.License) (*models.License, error)
	FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.License, error)
	UpdateStatusWithTx(tx *gorm.DB, id uuid.UUID, status enums.LicenseStatus) error
	ListStatusesWithTx(tx *gorm.DB, storeID uuid.UUID) ([]enums.LicenseStatus, error)
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
	attachments  media.AttachmentReconciler
	gcs          gcsClient
	bucket       string
	downloadTTL  time.Duration
	storeRepo    storesRepository
	allowedRoles []enums.MemberRole
	tx           txRunner
	publisher    outboxPublisher
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
func NewService(repo licensesRepository, mediaRepo mediasRepository, memberships membershipsRepository, attachments media.AttachmentReconciler, gcs gcsClient, bucket string, downloadTTL time.Duration, storeRepo storesRepository, tx txRunner, publisher outboxPublisher) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("license repository required")
	}
	if mediaRepo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	if memberships == nil {
		return nil, fmt.Errorf("memberships repository required")
	}
	if attachments == nil {
		return nil, fmt.Errorf("attachment reconciler required")
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
	if tx == nil {
		return nil, fmt.Errorf("transaction runner required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("outbox publisher required")
	}
	return &service{
		repo:        repo,
		mediaRepo:   mediaRepo,
		memberships: memberships,
		attachments: attachments,
		gcs:         gcs,
		bucket:      bucket,
		downloadTTL: downloadTTL,
		storeRepo:   storeRepo,
		tx:          tx,
		publisher:   publisher,
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

	var created *models.License
	if err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		stored, err := s.repo.CreateWithTx(tx, license)
		if err != nil {
			return err
		}
		if err := s.attachments.Reconcile(ctx, tx, models.AttachmentEntityLicense, stored.ID, stored.StoreID, nil, []uuid.UUID{stored.MediaID}); err != nil {
			return err
		}
		created = stored
		storeRef := storeID
		return s.emitLicenseStatusEvent(ctx, tx, stored, stored.Status, "", &outbox.ActorRef{
			UserID:  userID,
			StoreID: &storeRef,
		})
	}); err != nil {
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

	if err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.attachments.Reconcile(ctx, tx, models.AttachmentEntityLicense, license.ID, license.StoreID, []uuid.UUID{license.MediaID}, nil); err != nil {
			return err
		}
		if err := s.repo.DeleteWithTx(tx, licenseID); err != nil {
			return err
		}
		return nil
	}); err != nil {
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

func (s *service) emitLicenseStatusEvent(ctx context.Context, tx *gorm.DB, license *models.License, status enums.LicenseStatus, reason string, actor *outbox.ActorRef) error {
	if license == nil {
		return fmt.Errorf("license is required for outbox event")
	}
	trimmedReason := strings.TrimSpace(reason)
	payload := payloads.LicenseStatusChangedEvent{
		LicenseID: license.ID,
		StoreID:   license.StoreID,
		Status:    status,
		Reason:    trimmedReason,
	}
	event := outbox.DomainEvent{
		EventType:     enums.EventLicenseStatusChanged,
		AggregateType: enums.AggregateLicense,
		AggregateID:   license.ID,
		Actor:         actor,
		Data:          payload,
		Version:       1,
	}
	return s.publisher.Emit(ctx, tx, event)
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

func (s *service) reconcileStoreKYC(ctx context.Context, tx *gorm.DB, storeID uuid.UUID) error {
	statuses, err := s.repo.ListStatusesWithTx(tx, storeID)
	if err != nil {
		return err
	}
	newStatus := DetermineStoreKYCStatus(statuses)
	if newStatus == enums.KYCStatusPendingVerification {
		return nil
	}
	store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
	if err != nil {
		return err
	}
	if store.KYCStatus == newStatus {
		return nil
	}

	if err := s.storeRepo.UpdateStatusWithTx(tx, storeID, newStatus); err != nil {
		return err
	}

	return nil
}

func DetermineStoreKYCStatus(statuses []enums.LicenseStatus) enums.KYCStatus {
	hasExpired := false
	hasRejected := false
	for _, status := range statuses {
		switch status {
		case enums.LicenseStatusVerified:
			return enums.KYCStatusVerified
		case enums.LicenseStatusExpired:
			hasExpired = true
		case enums.LicenseStatusRejected:
			hasRejected = true
		}
	}
	if hasExpired && !hasRejected {
		return enums.KYCStatusExpired
	}
	if hasRejected {
		return enums.KYCStatusRejected
	}
	return enums.KYCStatusPendingVerification
}
func (s *service) VerifyLicense(ctx context.Context, licenseID uuid.UUID, decision enums.LicenseStatus, reason string) (*models.License, error) {

	if licenseID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "license id is required")
	}
	if decision != enums.LicenseStatusVerified && decision != enums.LicenseStatusRejected {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "invalid decision")
	}

	var updated *models.License

	err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {

		license, err := s.repo.FindByIDWithTx(tx, licenseID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return pkgerrors.New(pkgerrors.CodeNotFound, "license not found")
			}
			return err
		}

		if license.Status != enums.LicenseStatusPending {
			return pkgerrors.New(pkgerrors.CodeConflict, "license already finalized")
		}

		if err := s.repo.UpdateStatusWithTx(tx, licenseID, decision); err != nil {
			return err
		}

		license.Status = decision

		if err := s.reconcileStoreKYC(ctx, tx, license.StoreID); err != nil {
			return err
		}

		updated = license

		if err := s.emitLicenseStatusEvent(ctx, tx, license, decision, reason, nil); err != nil {
			return err
		}

		if tx != nil && tx.Error != nil {
			fmt.Printf("[VerifyLicense] tx.state tx.Error=%T %v\n", tx.Error, tx.Error)
		}

		return nil
	})

	if err != nil {
		if pkgErr := pkgerrors.As(err); pkgErr != nil {
			return nil, pkgErr
		}

		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update license status")
	}

	return updated, nil
}
