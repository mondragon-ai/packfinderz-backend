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
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type mediasRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
}

type membershipsRepository interface {
	UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error)
}

type licensesRepository interface {
	Create(ctx context.Context, license *models.License) (*models.License, error)
}

// Service exposes license creation semantics.
type Service interface {
	CreateLicense(ctx context.Context, userID, storeID uuid.UUID, input CreateLicenseInput) (*models.License, error)
}

type service struct {
	repo         licensesRepository
	mediaRepo    mediasRepository
	memberships  membershipsRepository
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

// NewService builds a license service backed by the provided repositories.
func NewService(repo licensesRepository, mediaRepo mediasRepository, memberships membershipsRepository) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("license repository required")
	}
	if mediaRepo == nil {
		return nil, fmt.Errorf("media repository required")
	}
	if memberships == nil {
		return nil, fmt.Errorf("memberships repository required")
	}
	return &service{
		repo:        repo,
		mediaRepo:   mediaRepo,
		memberships: memberships,
		allowedRoles: []enums.MemberRole{
			enums.MemberRoleOwner,
			enums.MemberRoleAdmin,
			enums.MemberRoleManager,
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
