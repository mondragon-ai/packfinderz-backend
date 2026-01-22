package licenses

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubLicenseRepo struct {
	created *models.License
	err     error
}

func (s *stubLicenseRepo) Create(ctx context.Context, license *models.License) (*models.License, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.created = license
	return license, nil
}

type stubMediaRepo struct {
	media *models.Media
	err   error
}

func (s *stubMediaRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.media == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.media, nil
}

type stubMemberships struct {
	ok    bool
	err   error
	roles []enums.MemberRole
}

func (s *stubMemberships) UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	s.roles = roles
	return s.ok, nil
}

func newServiceForTests(media *models.Media, members *stubMemberships) (Service, *stubLicenseRepo) {
	repo := &stubLicenseRepo{}
	if media != nil && media.ID == uuid.Nil {
		media.ID = uuid.New()
	}
	if members == nil {
		members = &stubMemberships{ok: true}
	}
	mediaRepo := &stubMediaRepo{media: media}
	svc, err := NewService(repo, mediaRepo, members)
	if err != nil {
		panic(err)
	}
	return svc, repo
}

func TestCreateLicenseSuccess(t *testing.T) {
	now := time.Now()
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:        uuid.New(),
		StoreID:   storeID,
		Status:    enums.MediaStatusUploaded,
		GCSKey:    "media/license",
		MimeType:  "application/pdf",
		Kind:      enums.MediaKindLicenseDoc,
		CreatedAt: now,
	}

	members := &stubMemberships{ok: true}
	svc, repo := newServiceForTests(mediaRow, members)

	input := CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "OK",
		IssueDate:    &now,
		Type:         enums.LicenseTypeProducer,
		Number:       " 123 ",
	}
	license, err := svc.CreateLicense(context.Background(), userID, storeID, input)
	if err != nil {
		t.Fatalf("CreateLicense returned error: %v", err)
	}
	if license.Number != "123" {
		t.Fatalf("expected trimmed number, got %q", license.Number)
	}
	if license.Status != enums.LicenseStatusPending {
		t.Fatalf("expected status pending, got %s", license.Status)
	}
	if repo.created == nil {
		t.Fatalf("expected license created")
	}
}

func TestCreateLicenseRequiresMembership(t *testing.T) {
	storeID := uuid.New()
	members := &stubMemberships{ok: false}
	svc, _ := newServiceForTests(&models.Media{StoreID: storeID, Status: enums.MediaStatusUploaded, Kind: enums.MediaKindLicenseDoc, MimeType: "application/pdf"}, members)

	if _, err := svc.CreateLicense(context.Background(), uuid.New(), storeID, CreateLicenseInput{
		MediaID:      uuid.New(),
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected forbidden error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", err)
	}
}

func TestCreateLicenseMediaNotFound(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRepo := &stubMediaRepo{}
	repo := &stubLicenseRepo{}
	svc, _ := NewService(repo, mediaRepo, &stubMemberships{ok: true})

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      uuid.New(),
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected not found error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("expected not found code, got %v", err)
	}
}

func TestCreateLicenseMediaStoreMismatch(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  uuid.New(),
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/pdf",
	}
	svc, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true})

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected forbidden error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", err)
	}
}

func TestCreateLicenseInvalidStatus(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusPending,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/pdf",
	}
	svc, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true})

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected conflict error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func TestCreateLicenseInvalidMime(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/octet-stream",
	}
	svc, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true})

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected validation error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation code, got %v", err)
	}
}

func TestCreateLicenseInvalidKind(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindProduct,
		MimeType: "image/png",
	}
	svc, _ := newServiceForTests(mediaRow, &stubMemberships{ok: true})

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected validation error")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation code, got %v", err)
	}
}

func TestCreateLicenseChecksAllMemberRoles(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	mediaRow := &models.Media{
		ID:       uuid.New(),
		StoreID:  storeID,
		Status:   enums.MediaStatusUploaded,
		Kind:     enums.MediaKindLicenseDoc,
		MimeType: "application/pdf",
	}
	members := &stubMemberships{ok: false}
	svc, _ := newServiceForTests(mediaRow, members)

	if _, err := svc.CreateLicense(context.Background(), userID, storeID, CreateLicenseInput{
		MediaID:      mediaRow.ID,
		IssuingState: "state",
		Type:         enums.LicenseTypeProducer,
		Number:       "123",
	}); err == nil {
		t.Fatal("expected forbidden error")
	}

	expectedRoles := []enums.MemberRole{
		enums.MemberRoleOwner,
		enums.MemberRoleAdmin,
		enums.MemberRoleManager,
		enums.MemberRoleStaff,
		enums.MemberRoleOps,
	}
	if len(members.roles) != len(expectedRoles) {
		t.Fatalf("expected %d roles, got %d", len(expectedRoles), len(members.roles))
	}
	for _, expected := range expectedRoles {
		if !containsRole(members.roles, expected) {
			t.Fatalf("expected role %s in membership check", expected)
		}
	}
}

func containsRole(list []enums.MemberRole, target enums.MemberRole) bool {
	for _, candidate := range list {
		if candidate == target {
			return true
		}
	}
	return false
}
